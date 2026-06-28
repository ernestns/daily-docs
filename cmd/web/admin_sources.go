package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ernestns/daily-docs/internal/activation"
	"github.com/ernestns/daily-docs/internal/pipeline"
	"github.com/ernestns/daily-docs/internal/topicsource"
)

func (a app) adminSourceHandler(w http.ResponseWriter, r *http.Request, token string, path string) {
	rest := strings.TrimPrefix(path, "/admin/sources/")
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || len(parts) > 2 {
		http.NotFound(w, r)
		return
	}
	sourceID, err := parsePositiveID(parts[0], "source-id")
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
		a.adminSourceDetailHandler(w, r, token, sourceID)
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
	case "discover":
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "discover"); err != nil {
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		count, err := a.discoverSourcePreview(r.Context(), sourceID)
		if err != nil {
			log.Printf("admin discover source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		redirectAdminSource(w, r, sourceID, fmt.Sprintf("discovered %d candidate URLs", count), "")
	case "process":
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "process"); err != nil {
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		result, err := pipeline.ProcessSource(r.Context(), a.db, sourceID, pipeline.Options{})
		if err != nil {
			log.Printf("admin process source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		redirectAdminSource(w, r, sourceID, fmt.Sprintf("processed source %d: %d eligible", sourceID, result.EligibleCount), "")
	case "activate":
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "activate"); err != nil {
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		result, err := activation.ActivateSourceCandidates(r.Context(), a.db, sourceID)
		if err != nil {
			log.Printf("admin activate source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		redirectAdminSource(w, r, sourceID, fmt.Sprintf("activated %d candidates", result.Activated), "")
	case "create-source":
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "create-source"); err != nil {
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		current, err := topicsource.Load(r.Context(), a.db, sourceID)
		if err != nil {
			log.Printf("admin load source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		created, err := topicsource.CreateForTopic(r.Context(), a.db, topicsource.CreateForTopicInput{
			TopicID: current.TopicID,
			URL:     r.Form.Get("url"),
		})
		if err != nil {
			log.Printf("admin create sibling source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSource(w, r, sourceID, "", err.Error())
			return
		}
		redirectAdminSource(w, r, created.ID, fmt.Sprintf("created source %d", created.ID), "")
	default:
		http.NotFound(w, r)
	}
}

func (a app) adminSourcesHandler(w http.ResponseWriter, r *http.Request, token string) {
	switch r.Method {
	case http.MethodGet:
		sources, err := adminListAllSources(r.Context(), a.db)
		if err != nil {
			log.Printf("admin list sources failed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		renderTemplate(w, adminSourcesTemplate, struct {
			Sources []adminSourceRow
			CSRF    string
			Notice  string
			Error   string
		}{
			Sources: sources,
			CSRF:    csrfToken(r, token),
			Notice:  r.URL.Query().Get("notice"),
			Error:   r.URL.Query().Get("error"),
		})
	case http.MethodPost:
		if !validCSRF(r, token) {
			http.Error(w, "invalid csrf token", http.StatusForbidden)
			return
		}
		sourceID, err := parsePositiveID(r.Form.Get("source_id"), "source-id")
		if err != nil {
			redirectAdminSources(w, r, "", err.Error())
			return
		}
		if err := adminEnsureSourceAction(r.Context(), a.db, sourceID, "process"); err != nil {
			redirectAdminSources(w, r, "", err.Error())
			return
		}
		result, err := pipeline.ProcessSource(r.Context(), a.db, sourceID, pipeline.Options{})
		if err != nil {
			log.Printf("admin process source failed source_id=%d error=%v", sourceID, err)
			redirectAdminSources(w, r, "", err.Error())
			return
		}
		redirectAdminSources(w, r, fmt.Sprintf("processed source %d: %d eligible", sourceID, result.EligibleCount), "")
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a app) adminSourceDetailHandler(w http.ResponseWriter, r *http.Request, token string, sourceID int64) {
	filters := adminCandidateFiltersFromQuery(r.URL.Query())
	detail, err := adminGetSource(r.Context(), a.db, sourceID, filters)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("admin show source failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, adminSourceDetailTemplate, struct {
		Source adminSourceDetail
		CSRF   string
		Notice string
		Error  string
	}{
		Source: detail,
		CSRF:   csrfToken(r, token),
		Notice: r.URL.Query().Get("notice"),
		Error:  r.URL.Query().Get("error"),
	})
}

func (a app) discoverSourcePreview(ctx context.Context, sourceID int64) (int, error) {
	source, err := topicsource.Load(ctx, a.db, sourceID)
	if err != nil {
		return 0, err
	}
	if err := adminClaimSourceProcessing(ctx, a.db, sourceID); err != nil {
		return 0, err
	}
	discovery, err := pipeline.DiscoverURL(ctx, source.NormalizedURL, pipeline.Options{MaxPages: 50, MaxDepth: 2})
	preview := topicsource.DiscoveryPreview{}
	if err != nil {
		preview.Error = err.Error()
		var tooBroad pipeline.DiscoveryTooBroadError
		if errors.As(err, &tooBroad) {
			preview.Count = tooBroad.Count
			preview.NeedsScope = true
		}
		if recordErr := topicsource.RecordDiscoveryPreview(ctx, a.db, sourceID, preview); recordErr != nil {
			log.Printf("record discovery preview failed source_id=%d error=%v", sourceID, recordErr)
		}
		return preview.Count, err
	}
	preview.Count = discovery.DiscoveredCount
	preview.Sample = discovery.URLs
	if err := topicsource.RecordDiscoveryPreview(ctx, a.db, sourceID, preview); err != nil {
		return 0, err
	}
	return discovery.DiscoveredCount, nil
}

func adminClaimSourceProcessing(ctx context.Context, conn *sql.DB, sourceID int64) error {
	result, err := conn.ExecContext(ctx, `
		UPDATE topic_sources
		SET status = 'processing',
			last_error = '',
			updated_at = datetime('now')
		WHERE id = ?
			AND status NOT IN ('disabled', 'processing')
	`, sourceID)
	if err != nil {
		return fmt.Errorf("claim source processing: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read source processing claim: %w", err)
	}
	if affected == 0 {
		return pipeline.ErrSourceAlreadyProcessing
	}
	return nil
}

func sourceDiscoveryStatus(source adminSourceRow) string {
	switch source.Status {
	case "needs_scope", "discovery_failed", "ready_to_process", "pending_discovery":
		return source.Status
	default:
		if source.DiscoveryError != "" {
			return "discovery_failed"
		}
		if source.DiscoveryCount > 0 {
			return "ready_to_process"
		}
		return "not_discovered"
	}
}

func sourceWorkflowStatus(source adminSourceRow, runs []adminRunRow, candidates []adminCandidateRow) (string, string) {
	switch source.Status {
	case "pending_discovery":
		return "Submission -> Source", "Discover"
	case "ready_to_process":
		return "Submission -> Source -> Discovery", "Process"
	case "processing":
		return "Submission -> Source -> Discovery -> Processing", "Wait for processing"
	case "candidates_ready":
		if hasEligibleCandidates(candidates) {
			return "Submission -> Source -> Discovery -> Process -> Review", "Activate candidates"
		}
		return "Submission -> Source -> Discovery -> Process", "Review rejected candidates"
	case "needs_scope":
		return "Submission -> Source -> Discovery", "Add narrower source"
	case "discovery_failed":
		return "Submission -> Source", "Fix URL or discover again"
	case "disabled":
		return "Disabled", "No action"
	default:
		if len(runs) > 0 {
			return "Submission -> Source -> Process", "Review latest run"
		}
		return "Submission -> Source", "Discover"
	}
}

func hasEligibleCandidates(candidates []adminCandidateRow) bool {
	for _, candidate := range candidates {
		if candidate.Status == "eligible" {
			return true
		}
	}
	return false
}

func adminEnsureSourceAction(ctx context.Context, conn *sql.DB, sourceID int64, action string) error {
	state, err := adminGetSourceActionState(ctx, conn, sourceID)
	if err != nil {
		return err
	}

	switch action {
	case "discover":
		if !state.canDiscover() {
			return fmt.Errorf("source cannot be discovered while status is %s", state.Status)
		}
	case "process":
		if !state.canProcess() {
			return fmt.Errorf("source must be discovered before processing")
		}
	case "activate":
		if !state.canActivate() {
			return fmt.Errorf("source has no eligible candidates to activate")
		}
	case "create-source":
		if !state.canCreateSource() {
			return fmt.Errorf("source must need narrower scope before adding a narrower source")
		}
	default:
		return fmt.Errorf("unknown source action %q", action)
	}
	return nil
}

func adminGetSourceActionState(ctx context.Context, conn *sql.DB, sourceID int64) (adminSourceActionState, error) {
	var state adminSourceActionState
	err := conn.QueryRowContext(ctx, `
		SELECT
			status,
			discovery_count,
			(SELECT COUNT(*) FROM page_candidates WHERE topic_source_id = topic_sources.id AND status = 'eligible'),
			(SELECT COUNT(*) FROM page_candidates WHERE topic_source_id = topic_sources.id AND status = 'activated'),
			(SELECT COUNT(*) FROM pipeline_runs WHERE topic_source_id = topic_sources.id AND status = 'processing')
		FROM topic_sources
		WHERE id = ?
	`, sourceID).Scan(&state.Status, &state.DiscoveryCount, &state.EligibleCount, &state.ActivatedCount, &state.ProcessingCount)
	if err != nil {
		return adminSourceActionState{}, fmt.Errorf("load source action state: %w", err)
	}
	return state, nil
}

func (s adminSourceActionState) canDiscover() bool {
	return s.Status != "disabled" && s.Status != "processing" && s.ProcessingCount == 0
}

func (s adminSourceActionState) canProcess() bool {
	return s.ProcessingCount == 0 && s.DiscoveryCount > 0 && (s.Status == "ready_to_process" || s.Status == "candidates_ready")
}

func (s adminSourceActionState) canActivate() bool {
	return s.ProcessingCount == 0 && s.EligibleCount > 0 && (s.Status == "candidates_ready" || s.Status == "ready_to_process")
}

func (s adminSourceActionState) canCreateSource() bool {
	return s.Status == "needs_scope"
}

func adminListAllSources(ctx context.Context, conn *sql.DB) ([]adminSourceRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT
			ts.id,
			COALESCE(ts.created_from_submission_id, 0),
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
		ORDER BY ts.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query admin all sources: %w", err)
	}
	defer rows.Close()

	var sources []adminSourceRow
	for rows.Next() {
		var source adminSourceRow
		if err := rows.Scan(&source.ID, &source.SubmissionID, &source.TopicID, &source.TopicSlug, &source.TopicName, &source.Status, &source.SourceType, &source.BaseURL, &source.NormalizedURL, &source.LastProcessedAt, &source.LastError); err != nil {
			return nil, fmt.Errorf("scan admin all source: %w", err)
		}
		if source.LastProcessedAt == "" {
			source.LastProcessedAt = "-"
		}
		if source.LastError == "" {
			source.LastError = "-"
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin all sources: %w", err)
	}
	return sources, nil
}

func adminGetSource(ctx context.Context, conn *sql.DB, sourceID int64, filters adminCandidateFilters) (adminSourceDetail, error) {
	var detail adminSourceDetail
	var discoverySample string
	err := conn.QueryRowContext(ctx, `
		SELECT
			ts.id,
			COALESCE(ts.created_from_submission_id, 0),
			ts.topic_id,
			t.slug,
			t.name,
			ts.status,
			ts.source_type,
			ts.base_url,
			ts.normalized_url,
			COALESCE(ts.last_processed_at, ''),
			ts.last_error,
			COALESCE(ts.last_discovered_at, ''),
			ts.discovery_count,
			ts.discovery_sample,
			ts.discovery_error
		FROM topic_sources ts
		JOIN topics t ON t.id = ts.topic_id
		WHERE ts.id = ?
	`, sourceID).Scan(&detail.ID, &detail.SubmissionID, &detail.TopicID, &detail.TopicSlug, &detail.TopicName, &detail.Status, &detail.SourceType, &detail.BaseURL, &detail.NormalizedURL, &detail.LastProcessedAt, &detail.LastError, &detail.LastDiscoveredAt, &detail.DiscoveryCount, &discoverySample, &detail.DiscoveryError)
	if err != nil {
		return adminSourceDetail{}, err
	}
	if detail.LastProcessedAt == "" {
		detail.LastProcessedAt = "-"
	}
	if detail.LastError == "" {
		detail.LastError = "-"
	}
	if detail.LastDiscoveredAt == "" {
		detail.LastDiscoveredAt = "-"
	}
	if discoverySample != "" {
		_ = json.Unmarshal([]byte(discoverySample), &detail.DiscoverySample)
	}
	runs, err := adminListSourceRuns(ctx, conn, sourceID)
	if err != nil {
		return adminSourceDetail{}, err
	}
	discoveryRuns, err := adminListSourceDiscoveryRuns(ctx, conn, sourceID)
	if err != nil {
		return adminSourceDetail{}, err
	}
	candidates, err := adminListSourceCandidates(ctx, conn, sourceID, filters)
	if err != nil {
		return adminSourceDetail{}, err
	}
	actionState, err := adminGetSourceActionState(ctx, conn, sourceID)
	if err != nil {
		return adminSourceDetail{}, err
	}
	detail.Runs = runs
	detail.DiscoveryRuns = discoveryRuns
	detail.Candidates = candidates
	detail.CandidateFilters = filters
	detail.CanDiscover = actionState.canDiscover()
	detail.CanProcess = actionState.canProcess()
	detail.CanActivate = actionState.canActivate()
	detail.CanCreateSource = actionState.canCreateSource()
	detail.DiscoveryStatus = sourceDiscoveryStatus(detail.adminSourceRow)
	detail.WorkflowStatus, detail.NextAction = sourceWorkflowStatus(detail.adminSourceRow, runs, candidates)
	return detail, nil
}

func adminListSourceDiscoveryRuns(ctx context.Context, conn *sql.DB, sourceID int64) ([]adminDiscoveryRunRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, status, created_at, discovered_count, discovery_sample, discovery_error
		FROM source_discovery_runs
		WHERE topic_source_id = ?
		ORDER BY id DESC
		LIMIT 20
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("query admin source discovery runs: %w", err)
	}
	defer rows.Close()

	var runs []adminDiscoveryRunRow
	for rows.Next() {
		var run adminDiscoveryRunRow
		var sample string
		if err := rows.Scan(&run.ID, &run.Status, &run.CreatedAt, &run.DiscoveredCount, &sample, &run.DiscoveryError); err != nil {
			return nil, fmt.Errorf("scan admin source discovery run: %w", err)
		}
		if sample != "" {
			_ = json.Unmarshal([]byte(sample), &run.DiscoverySample)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin source discovery runs: %w", err)
	}
	return runs, nil
}

func adminListSourceRuns(ctx context.Context, conn *sql.DB, sourceID int64) ([]adminRunRow, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT id, status, started_at, completed_at, discovered_count, crawled_count, eligible_count, rejected_count, failure_count, error
		FROM pipeline_runs
		WHERE topic_source_id = ?
		ORDER BY id DESC
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("query admin source runs: %w", err)
	}
	defer rows.Close()
	return scanAdminRuns(rows)
}
