package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (a app) adminRunHandler(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runID, err := parsePositiveID(strings.TrimPrefix(path, "/admin/runs/"), "run-id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	filters := adminCandidateFiltersFromQuery(r.URL.Query())
	detail, err := adminGetRun(r.Context(), a.db, runID, filters)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("admin show run failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, adminRunDetailTemplate, struct {
		Run adminRunDetail
	}{Run: detail})
}

func adminGetRun(ctx context.Context, conn *sql.DB, runID int64, filters adminCandidateFilters) (adminRunDetail, error) {
	var detail adminRunDetail
	var completed sql.NullString
	var sourceID sql.NullInt64
	err := conn.QueryRowContext(ctx, `
		SELECT
			pr.id,
			pr.documentation_submission_id,
			pr.topic_source_id,
			COALESCE(t.slug, ''),
			COALESCE(t.name, ''),
			COALESCE(ts.normalized_url, ds.normalized_url),
			pr.status,
			pr.started_at,
			pr.completed_at,
			pr.discovered_count,
			pr.crawled_count,
			pr.eligible_count,
			pr.rejected_count,
			pr.failure_count,
			pr.error
		FROM pipeline_runs pr
		JOIN documentation_submissions ds ON ds.id = pr.documentation_submission_id
		LEFT JOIN topic_sources ts ON ts.id = pr.topic_source_id
		LEFT JOIN topics t ON t.id = ts.topic_id
		WHERE pr.id = ?
	`, runID).Scan(
		&detail.ID,
		&detail.SubmissionID,
		&sourceID,
		&detail.TopicSlug,
		&detail.TopicName,
		&detail.SourceURL,
		&detail.Status,
		&detail.StartedAt,
		&completed,
		&detail.DiscoveredCount,
		&detail.CrawledCount,
		&detail.EligibleCount,
		&detail.RejectedCount,
		&detail.FailureCount,
		&detail.Error,
	)
	if err != nil {
		return adminRunDetail{}, err
	}
	if sourceID.Valid {
		detail.SourceID = sourceID.Int64
	}
	detail.CompletedAt = "-"
	if completed.Valid && completed.String != "" {
		detail.CompletedAt = completed.String
	}
	candidates, err := adminListRunCandidates(ctx, conn, runID, filters)
	if err != nil {
		return adminRunDetail{}, err
	}
	detail.Candidates = candidates
	detail.CandidateFilters = filters
	detail.CandidateFilterQS = filters.queryString()
	return detail, nil
}

func adminListRuns(ctx context.Context, conn *sql.DB, submissionID int64) ([]adminRunRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, status, started_at, completed_at, discovered_count, crawled_count, eligible_count, rejected_count, failure_count, error
		FROM pipeline_runs
		WHERE documentation_submission_id = ?
		ORDER BY id DESC
	`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("query admin runs: %w", err)
	}
	defer rows.Close()
	return scanAdminRuns(rows)
}

func scanAdminRuns(rows *sql.Rows) ([]adminRunRow, error) {
	var runs []adminRunRow
	for rows.Next() {
		var run adminRunRow
		var completed sql.NullString
		if err := rows.Scan(&run.ID, &run.Status, &run.StartedAt, &completed, &run.DiscoveredCount, &run.CrawledCount, &run.EligibleCount, &run.RejectedCount, &run.FailureCount, &run.Error); err != nil {
			return nil, fmt.Errorf("scan admin run: %w", err)
		}
		run.CompletedAt = "-"
		if completed.Valid && completed.String != "" {
			run.CompletedAt = completed.String
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin runs: %w", err)
	}
	return runs, nil
}
