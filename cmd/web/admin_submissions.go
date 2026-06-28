package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ernestns/daily-docs/internal/activation"
	"github.com/ernestns/daily-docs/internal/pipeline"
	"github.com/ernestns/daily-docs/internal/topicsource"
)

func (a app) adminSubmissionsHandler(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	submissions, err := adminListSubmissions(r.Context(), a.db)
	if err != nil {
		log.Printf("admin list submissions failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, adminSubmissionsTemplate, struct {
		Submissions []adminSubmissionRow
		CSRF        string
		Notice      string
		Error       string
	}{
		Submissions: submissions,
		CSRF:        csrfToken(r, token),
		Notice:      r.URL.Query().Get("notice"),
		Error:       r.URL.Query().Get("error"),
	})
}

func (a app) adminSubmissionHandler(w http.ResponseWriter, r *http.Request, token string, path string) {
	rest := strings.TrimPrefix(path, "/admin/submissions/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	submissionID, err := parsePositiveID(parts[0], "submission-id")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.adminSubmissionDetailHandler(w, r, token, submissionID)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !validCSRF(r, token) {
		http.Error(w, "invalid csrf token", http.StatusForbidden)
		return
	}

	switch parts[1] {
	case "process":
		_, err := pipeline.ProcessSubmission(r.Context(), a.db, submissionID, pipeline.Options{})
		if err != nil {
			log.Printf("admin process submission failed id=%d error=%v", submissionID, err)
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		redirectAdminSubmission(w, r, submissionID, "processed", "")
	case "activate":
		_, err := activation.ActivateCandidates(r.Context(), a.db, submissionID)
		if err != nil {
			log.Printf("admin activate candidates failed id=%d error=%v", submissionID, err)
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		redirectAdminSubmission(w, r, submissionID, "activated", "")
	case "create-source":
		source, err := topicsource.CreateFromSubmission(r.Context(), a.db, topicsource.CreateFromSubmissionInput{
			SubmissionID: submissionID,
			TopicSlug:    r.Form.Get("topic_slug"),
			TopicName:    r.Form.Get("topic_name"),
		})
		if err != nil {
			log.Printf("admin create source failed submission_id=%d error=%v", submissionID, err)
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		redirectAdminSubmission(w, r, submissionID, fmt.Sprintf("created source %d", source.ID), "")
	case "process-source":
		sourceID, err := parsePositiveID(r.Form.Get("source_id"), "source-id")
		if err != nil {
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		source, err := topicsource.Load(r.Context(), a.db, sourceID)
		if err != nil {
			log.Printf("admin load source failed submission_id=%d source_id=%d error=%v", submissionID, sourceID, err)
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		if !source.CreatedFromSubmissionID.Valid || source.CreatedFromSubmissionID.Int64 != submissionID {
			redirectAdminSubmission(w, r, submissionID, "", "source does not belong to submission")
			return
		}
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "process"); err != nil {
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		result, err := pipeline.ProcessSource(r.Context(), a.db, sourceID, pipeline.Options{})
		if err != nil {
			log.Printf("admin process source failed submission_id=%d source_id=%d error=%v", submissionID, sourceID, err)
			redirectAdminSubmission(w, r, submissionID, "", err.Error())
			return
		}
		redirectAdminSubmission(w, r, submissionID, fmt.Sprintf("processed source %d: %d eligible", sourceID, result.EligibleCount), "")
	default:
		http.NotFound(w, r)
	}
}

func (a app) adminSubmissionDetailHandler(w http.ResponseWriter, r *http.Request, token string, submissionID int64) {
	detail, err := adminGetSubmission(r.Context(), a.db, submissionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("admin show submission failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, adminSubmissionDetailTemplate, struct {
		Submission adminSubmissionDetail
		CSRF       string
		Notice     string
		Error      string
	}{
		Submission: detail,
		CSRF:       csrfToken(r, token),
		Notice:     r.URL.Query().Get("notice"),
		Error:      r.URL.Query().Get("error"),
	})
}

func adminListSubmissions(ctx context.Context, conn *sql.DB) ([]adminSubmissionRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, suggested_topic, source_host, status, request_count, last_submitted_at, last_error
		FROM documentation_submissions
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query admin submissions: %w", err)
	}
	defer rows.Close()

	var submissions []adminSubmissionRow
	for rows.Next() {
		var sub adminSubmissionRow
		if err := rows.Scan(&sub.ID, &sub.SuggestedTopic, &sub.SourceHost, &sub.Status, &sub.RequestCount, &sub.LastSubmitted, &sub.LastError); err != nil {
			return nil, fmt.Errorf("scan admin submission: %w", err)
		}
		submissions = append(submissions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin submissions: %w", err)
	}
	return submissions, nil
}

func adminGetSubmission(ctx context.Context, conn *sql.DB, submissionID int64) (adminSubmissionDetail, error) {
	var detail adminSubmissionDetail
	err := conn.QueryRowContext(ctx, `
		SELECT id, suggested_topic, source_host, status, request_count, submitted_url, normalized_url, last_submitted_at, last_error
		FROM documentation_submissions
		WHERE id = ?
	`, submissionID).Scan(&detail.ID, &detail.SuggestedTopic, &detail.SourceHost, &detail.Status, &detail.RequestCount, &detail.SubmittedURL, &detail.NormalizedURL, &detail.LastSubmitted, &detail.LastError)
	if err != nil {
		return adminSubmissionDetail{}, err
	}
	detail.SuggestedSlug = slugFromTopicName(detail.SuggestedTopic)

	sources, err := adminListSources(ctx, conn, submissionID)
	if err != nil {
		return adminSubmissionDetail{}, err
	}
	runs, err := adminListRuns(ctx, conn, submissionID)
	if err != nil {
		return adminSubmissionDetail{}, err
	}
	candidates, err := adminListCandidates(ctx, conn, submissionID)
	if err != nil {
		return adminSubmissionDetail{}, err
	}
	detail.Sources = sources
	detail.Runs = runs
	detail.Candidates = candidates
	return detail, nil
}

func adminListSources(ctx context.Context, conn *sql.DB, submissionID int64) ([]adminSourceRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT
			ts.id,
			ts.topic_id,
			t.slug,
			t.name,
			ts.status,
			ts.source_type,
			ts.base_url,
			ts.normalized_url,
			COALESCE(ts.last_processed_at, ''),
			ts.last_error
		FROM topic_sources ts
		JOIN topics t ON t.id = ts.topic_id
		WHERE ts.created_from_submission_id = ?
		ORDER BY ts.id DESC
	`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("query admin sources: %w", err)
	}
	defer rows.Close()

	var sources []adminSourceRow
	for rows.Next() {
		var source adminSourceRow
		if err := rows.Scan(&source.ID, &source.TopicID, &source.TopicSlug, &source.TopicName, &source.Status, &source.SourceType, &source.BaseURL, &source.NormalizedURL, &source.LastProcessedAt, &source.LastError); err != nil {
			return nil, fmt.Errorf("scan admin source: %w", err)
		}
		source.SubmissionID = submissionID
		if source.LastProcessedAt == "" {
			source.LastProcessedAt = "-"
		}
		if source.LastError == "" {
			source.LastError = "-"
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin sources: %w", err)
	}
	return sources, nil
}
