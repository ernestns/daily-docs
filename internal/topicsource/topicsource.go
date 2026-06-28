package topicsource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/ernestns/daily-docs/internal/submission"
)

type Source struct {
	ID                      int64
	TopicID                 int64
	TopicSlug               string
	TopicName               string
	BaseURL                 string
	NormalizedURL           string
	SourceHost              string
	SourceType              string
	Status                  string
	CreatedFromSubmissionID sql.NullInt64
	LastProcessedAt         sql.NullString
	LastError               string
}

type CreateFromSubmissionInput struct {
	SubmissionID int64
	TopicSlug    string
	TopicName    string
	SourceType   string
}

func CreateFromSubmission(ctx context.Context, conn *sql.DB, input CreateFromSubmissionInput) (Source, error) {
	if input.SubmissionID < 1 {
		return Source{}, errors.New("submission id must be positive")
	}
	topicSlug := slugify(input.TopicSlug)
	if topicSlug == "" {
		return Source{}, errors.New("topic slug is required")
	}
	topicName := strings.TrimSpace(input.TopicName)
	if topicName == "" {
		topicName = topicSlug
	}
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" {
		sourceType = "documentation"
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, fmt.Errorf("begin topic source create: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var submittedURL string
	var normalizedURL string
	var sourceHost string
	if err := tx.QueryRowContext(ctx, `
		SELECT submitted_url, normalized_url, source_host
		FROM documentation_submissions
		WHERE id = ?
	`, input.SubmissionID).Scan(&submittedURL, &normalizedURL, &sourceHost); err != nil {
		return Source{}, fmt.Errorf("load submission for source: %w", err)
	}

	topicID, err := upsertTopic(ctx, tx, topicSlug, topicName)
	if err != nil {
		return Source{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO topic_sources (
			topic_id,
			base_url,
			normalized_url,
			source_host,
			source_type,
			status,
			created_from_submission_id,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, 'active', ?, datetime('now'))
		ON CONFLICT(topic_id, normalized_url) DO UPDATE SET
			base_url = excluded.base_url,
			source_host = excluded.source_host,
			source_type = excluded.source_type,
			status = 'active',
			created_from_submission_id = COALESCE(topic_sources.created_from_submission_id, excluded.created_from_submission_id),
			updated_at = datetime('now')
	`, topicID, submittedURL, normalizedURL, sourceHost, sourceType, input.SubmissionID)
	if err != nil {
		return Source{}, fmt.Errorf("upsert topic source: %w", err)
	}

	var sourceID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM topic_sources
		WHERE topic_id = ? AND normalized_url = ?
	`, topicID, normalizedURL).Scan(&sourceID); err != nil {
		return Source{}, fmt.Errorf("read topic source id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE documentation_submissions
		SET status = 'active',
			last_error = ''
		WHERE id = ?
	`, input.SubmissionID); err != nil {
		return Source{}, fmt.Errorf("mark source submission active: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Source{}, fmt.Errorf("commit topic source create: %w", err)
	}

	return Load(ctx, conn, sourceID)
}

func Load(ctx context.Context, conn *sql.DB, id int64) (Source, error) {
	var src Source
	err := conn.QueryRowContext(ctx, `
		SELECT
			ts.id,
			ts.topic_id,
			t.slug,
			t.name,
			ts.base_url,
			ts.normalized_url,
			ts.source_host,
			ts.source_type,
			ts.status,
			ts.created_from_submission_id,
			ts.last_processed_at,
			ts.last_error
		FROM topic_sources ts
		JOIN topics t ON t.id = ts.topic_id
		WHERE ts.id = ?
	`, id).Scan(&src.ID, &src.TopicID, &src.TopicSlug, &src.TopicName, &src.BaseURL, &src.NormalizedURL, &src.SourceHost, &src.SourceType, &src.Status, &src.CreatedFromSubmissionID, &src.LastProcessedAt, &src.LastError)
	if err != nil {
		return Source{}, fmt.Errorf("load topic source: %w", err)
	}
	return src, nil
}

func WriteList(ctx context.Context, conn *sql.DB, out io.Writer) error {
	rows, err := conn.QueryContext(ctx, `
		SELECT
			ts.id,
			t.slug,
			ts.status,
			ts.source_type,
			ts.normalized_url,
			COALESCE(ts.last_processed_at, ''),
			ts.last_error
		FROM topic_sources ts
		JOIN topics t ON t.id = ts.topic_id
		ORDER BY t.slug ASC, ts.normalized_url ASC
	`)
	if err != nil {
		return fmt.Errorf("query topic sources: %w", err)
	}
	defer rows.Close()

	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "ID\tTOPIC\tSTATUS\tTYPE\tURL\tLAST_PROCESSED\tERROR")
	for rows.Next() {
		var id int64
		var topic string
		var status string
		var sourceType string
		var url string
		var lastProcessed string
		var lastError string
		if err := rows.Scan(&id, &topic, &status, &sourceType, &url, &lastProcessed, &lastError); err != nil {
			return fmt.Errorf("scan topic source: %w", err)
		}
		if lastProcessed == "" {
			lastProcessed = "-"
		}
		if lastError == "" {
			lastError = "-"
		}
		_, _ = fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n", id, topic, status, sourceType, url, lastProcessed, shorten(lastError, 120))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate topic sources: %w", err)
	}
	return writer.Flush()
}

func shorten(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
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
		return 0, fmt.Errorf("upsert source topic: %w", err)
	}

	var topicID int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM topics WHERE slug = ?", slug).Scan(&topicID); err != nil {
		return 0, fmt.Errorf("read source topic: %w", err)
	}
	return topicID, nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func NormalizeURL(raw string) (string, string, error) {
	return submission.NormalizeURL(raw)
}
