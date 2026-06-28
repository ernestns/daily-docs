package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func adminCandidateFiltersFromQuery(values url.Values) adminCandidateFilters {
	filters := adminCandidateFilters{
		Status:   strings.TrimSpace(values.Get("status")),
		PageType: strings.TrimSpace(values.Get("page_type")),
		MinScore: strings.TrimSpace(values.Get("min_score")),
	}
	switch filters.Status {
	case "", "eligible", "rejected", "activated":
	default:
		filters.Status = ""
	}
	if _, err := strconv.Atoi(filters.MinScore); filters.MinScore != "" && err != nil {
		filters.MinScore = ""
	}
	return filters
}

func (f adminCandidateFilters) queryString() string {
	values := url.Values{}
	if f.Status != "" {
		values.Set("status", f.Status)
	}
	if f.PageType != "" {
		values.Set("page_type", f.PageType)
	}
	if f.MinScore != "" {
		values.Set("min_score", f.MinScore)
	}
	return values.Encode()
}

func (f adminCandidateFilters) apply(sqlQuery string, args []any) (string, []any) {
	if f.Status != "" {
		sqlQuery += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.PageType != "" {
		sqlQuery += " AND gate_page_type = ?"
		args = append(args, f.PageType)
	}
	if f.MinScore != "" {
		if minScore, err := strconv.Atoi(f.MinScore); err == nil {
			sqlQuery += " AND score >= ?"
			args = append(args, minScore)
		}
	}
	return sqlQuery, args
}

func adminListCandidates(ctx context.Context, conn *sql.DB, submissionID int64) ([]adminCandidateRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, title, url, primary_classification, score, gate_score, gate_page_type, reject_stage, status, estimated_minutes,
			CASE WHEN status = 'rejected' THEN reject_reason ELSE reason END,
			gate_input_tokens,
			gate_output_tokens,
			gate_reasoning_tokens,
			gate_total_tokens,
			enrichment_total_tokens,
			review_model,
			review_confidence,
			review_rationale
		FROM page_candidates
		WHERE documentation_submission_id = ?
		ORDER BY score DESC, title ASC
	`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("query admin candidates: %w", err)
	}
	defer rows.Close()
	return scanAdminCandidates(rows)
}

func adminListSourceCandidates(ctx context.Context, conn *sql.DB, sourceID int64, filters adminCandidateFilters) ([]adminCandidateRow, error) {
	sqlQuery := `
		SELECT id, title, url, primary_classification, score, gate_score, gate_page_type, reject_stage, status, estimated_minutes,
			CASE WHEN status = 'rejected' THEN reject_reason ELSE reason END,
			gate_input_tokens,
			gate_output_tokens,
			gate_reasoning_tokens,
			gate_total_tokens,
			enrichment_total_tokens,
			review_model,
			review_confidence,
			review_rationale
		FROM page_candidates
		WHERE topic_source_id = ?`
	args := []any{sourceID}
	sqlQuery, args = filters.apply(sqlQuery, args)
	sqlQuery += " ORDER BY score DESC, title ASC"
	rows, err := conn.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query admin source candidates: %w", err)
	}
	defer rows.Close()
	return scanAdminCandidates(rows)
}

func adminListRunCandidates(ctx context.Context, conn *sql.DB, runID int64, filters adminCandidateFilters) ([]adminCandidateRow, error) {
	sqlQuery := `
		SELECT id, title, url, primary_classification, score, gate_score, gate_page_type, reject_stage, status, estimated_minutes,
			CASE WHEN status = 'rejected' THEN reject_reason ELSE reason END,
			gate_input_tokens,
			gate_output_tokens,
			gate_reasoning_tokens,
			gate_total_tokens,
			enrichment_total_tokens,
			review_model,
			review_confidence,
			review_rationale
		FROM page_candidates
		WHERE pipeline_run_id = ?`
	args := []any{runID}
	sqlQuery, args = filters.apply(sqlQuery, args)
	sqlQuery += " ORDER BY status ASC, score DESC, title ASC"
	rows, err := conn.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query admin run candidates: %w", err)
	}
	defer rows.Close()
	return scanAdminCandidates(rows)
}

func scanAdminCandidates(rows *sql.Rows) ([]adminCandidateRow, error) {
	var candidates []adminCandidateRow
	for rows.Next() {
		var cand adminCandidateRow
		var estimated sql.NullInt64
		var gateScore sql.NullInt64
		var gateReason string
		var reviewConfidence float64
		var gateInput int
		var gateOutput int
		var gateReasoning int
		var gateTotal int
		var enrichmentTotal int
		if err := rows.Scan(&cand.ID, &cand.Title, &cand.URL, &cand.Classification, &cand.Score, &gateScore, &gateReason, &cand.RejectStage, &cand.Status, &estimated, &cand.Reason, &gateInput, &gateOutput, &gateReasoning, &gateTotal, &enrichmentTotal, &cand.ReviewModel, &reviewConfidence, &cand.Rationale); err != nil {
			return nil, fmt.Errorf("scan admin candidate: %w", err)
		}
		cand.Gate = "-"
		if gateScore.Valid {
			cand.Gate = strconv.FormatInt(gateScore.Int64, 10)
		}
		if gateReason != "" {
			cand.Gate += "/" + gateReason
		}
		if cand.RejectStage == "" {
			cand.RejectStage = "-"
		}
		cand.EstimatedMinutes = "-"
		if estimated.Valid {
			cand.EstimatedMinutes = strconv.FormatInt(estimated.Int64, 10)
		}
		cand.TokenSummary = fmt.Sprintf("gate %d/%d/%d/%d enrich %d", gateInput, gateOutput, gateReasoning, gateTotal, enrichmentTotal)
		cand.Confidence = fmt.Sprintf("%.2f", reviewConfidence)
		if cand.ReviewModel == "" {
			cand.ReviewModel = "-"
		}
		if cand.Rationale == "" {
			cand.Rationale = "-"
		}
		candidates = append(candidates, cand)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin candidates: %w", err)
	}
	return candidates, nil
}
