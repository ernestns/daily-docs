package activation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNoEligibleCandidates = errors.New("no eligible candidates")

type Result struct {
	SubmissionID int64
	TopicID      int64
	TopicSlug    string
	Activated    int
}

type candidate struct {
	ID                int64
	PipelineRunID     int64
	ProposedTopicSlug string
	ProposedTopicName string
	Title             string
	URL               string
	Source            string
	Official          bool
	EstimatedMinutes  sql.NullInt64
	Score             sql.NullInt64
}

func ActivateCandidates(ctx context.Context, conn *sql.DB, submissionID int64) (Result, error) {
	candidates, err := eligibleCandidates(ctx, conn, submissionID)
	if err != nil {
		return Result{}, err
	}
	if len(candidates) == 0 {
		return Result{}, ErrNoEligibleCandidates
	}

	topicSlug := strings.TrimSpace(candidates[0].ProposedTopicSlug)
	topicName := strings.TrimSpace(candidates[0].ProposedTopicName)
	if topicSlug == "" {
		return Result{}, errors.New("candidate is missing proposed topic slug")
	}
	if topicName == "" {
		topicName = topicSlug
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin activation: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	topicID, err := upsertTopic(ctx, tx, topicSlug, topicName)
	if err != nil {
		return Result{}, err
	}

	nextOrder, err := nextReadingOrder(ctx, tx, topicID)
	if err != nil {
		return Result{}, err
	}

	activated := 0
	for _, cand := range candidates {
		pageID, created, err := upsertPage(ctx, tx, topicID, cand, nextOrder)
		if err != nil {
			return Result{}, err
		}
		if created {
			nextOrder++
		}
		if err := markCandidateActivated(ctx, tx, cand.ID, pageID, topicID); err != nil {
			return Result{}, err
		}
		activated++
	}

	if err := markSubmissionActive(ctx, tx, submissionID); err != nil {
		return Result{}, err
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit activation: %w", err)
	}

	return Result{
		SubmissionID: submissionID,
		TopicID:      topicID,
		TopicSlug:    topicSlug,
		Activated:    activated,
	}, nil
}

func eligibleCandidates(ctx context.Context, conn *sql.DB, submissionID int64) ([]candidate, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT
			id,
			pipeline_run_id,
			proposed_topic_slug,
			proposed_topic_name,
			title,
			url,
			source,
			official,
			estimated_minutes,
			score
		FROM page_candidates
		WHERE documentation_submission_id = ?
			AND status IN ('eligible', 'activated')
		ORDER BY title ASC, id ASC
	`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("query eligible candidates: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var cand candidate
		var official int
		if err := rows.Scan(&cand.ID, &cand.PipelineRunID, &cand.ProposedTopicSlug, &cand.ProposedTopicName, &cand.Title, &cand.URL, &cand.Source, &official, &cand.EstimatedMinutes, &cand.Score); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		cand.Official = official == 1
		candidates = append(candidates, cand)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidates: %w", err)
	}
	return candidates, nil
}

func upsertTopic(ctx context.Context, tx *sql.Tx, slug string, name string) (int64, error) {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO topics (slug, name, status)
		VALUES (?, ?, 'active')
		ON CONFLICT(slug) DO UPDATE SET
			name = excluded.name,
			status = 'active'
	`, slug, name)
	if err != nil {
		return 0, fmt.Errorf("upsert activation topic: %w", err)
	}

	var topicID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM topics WHERE slug = ?", slug).Scan(&topicID); err != nil {
		return 0, fmt.Errorf("read activation topic: %w", err)
	}
	return topicID, nil
}

func nextReadingOrder(ctx context.Context, tx *sql.Tx, topicID int64) (int, error) {
	var maxOrder sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT MAX(reading_order) FROM pages WHERE topic_id = ?", topicID).Scan(&maxOrder); err != nil {
		return 0, fmt.Errorf("read max reading order: %w", err)
	}
	if !maxOrder.Valid {
		return 1, nil
	}
	return int(maxOrder.Int64) + 1, nil
}

func upsertPage(ctx context.Context, tx *sql.Tx, topicID int64, cand candidate, readingOrder int) (int64, bool, error) {
	var existingID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM pages WHERE topic_id = ? AND url = ?", topicID, cand.URL).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, fmt.Errorf("lookup existing page: %w", err)
	}

	official := 0
	if cand.Official {
		official = 1
	}
	var estimated any
	if cand.EstimatedMinutes.Valid {
		estimated = cand.EstimatedMinutes.Int64
	}
	var score any
	if cand.Score.Valid {
		score = cand.Score.Int64
	}

	if existingID != 0 {
		_, err := tx.ExecContext(ctx, `
			UPDATE pages
			SET title = ?,
				source = ?,
				official = ?,
				estimated_minutes = ?,
				evergreen_score = ?,
				active = 1,
				page_candidate_id = ?,
				activated_from_pipeline_run_id = ?,
				activation_reason = 'candidate activation',
				updated_at = datetime('now')
			WHERE id = ?
		`, cand.Title, cand.Source, official, estimated, score, cand.ID, cand.PipelineRunID, existingID)
		if err != nil {
			return 0, false, fmt.Errorf("update activated page: %w", err)
		}
		return existingID, false, nil
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO pages (
			topic_id,
			title,
			url,
			source,
			official,
			estimated_minutes,
			evergreen_score,
			reading_order,
			active,
			page_candidate_id,
			activated_from_pipeline_run_id,
			activation_reason,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, 'candidate activation', datetime('now'))
	`, topicID, cand.Title, cand.URL, cand.Source, official, estimated, score, readingOrder, cand.ID, cand.PipelineRunID)
	if err != nil {
		return 0, false, fmt.Errorf("insert activated page: %w", err)
	}
	pageID, err := result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("read activated page id: %w", err)
	}
	return pageID, true, nil
}

func markCandidateActivated(ctx context.Context, tx *sql.Tx, candidateID int64, pageID int64, topicID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE page_candidates
		SET status = 'activated',
			topic_id = ?,
			reviewed_at = datetime('now')
		WHERE id = ?
	`, topicID, candidateID)
	if err != nil {
		return fmt.Errorf("mark candidate %d activated for page %d: %w", candidateID, pageID, err)
	}
	return nil
}

func markSubmissionActive(ctx context.Context, tx *sql.Tx, submissionID int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE documentation_submissions SET status = 'active', last_error = '' WHERE id = ?", submissionID)
	if err != nil {
		return fmt.Errorf("mark submission active: %w", err)
	}
	return nil
}
