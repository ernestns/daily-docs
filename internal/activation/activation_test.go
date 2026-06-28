package activation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ernestns/daily-docs/internal/db"
)

func TestActivateCandidatesCreatesTopicAndPages(t *testing.T) {
	ctx := context.Background()
	conn := openActivationTestDB(t, ctx)
	defer conn.Close()

	submissionID, runID := insertActivationSubmission(t, ctx, conn)
	insertCandidate(t, ctx, conn, submissionID, runID, "Alpha Page", "https://example.com/docs/alpha")
	insertCandidate(t, ctx, conn, submissionID, runID, "Beta Page", "https://example.com/docs/beta")

	result, err := ActivateCandidates(ctx, conn, submissionID)
	if err != nil {
		t.Fatalf("activate candidates: %v", err)
	}
	if result.Activated != 2 {
		t.Fatalf("expected 2 activated candidates, got %d", result.Activated)
	}
	if result.TopicSlug != "example-docs" {
		t.Fatalf("expected topic slug example-docs, got %q", result.TopicSlug)
	}

	var pages int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE topic_id = ? AND active = 1", result.TopicID).Scan(&pages); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	if pages != 2 {
		t.Fatalf("expected 2 active pages, got %d", pages)
	}

	var activeCandidates int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM page_candidates WHERE status = 'activated'").Scan(&activeCandidates); err != nil {
		t.Fatalf("count activated candidates: %v", err)
	}
	if activeCandidates != 2 {
		t.Fatalf("expected 2 activated candidates, got %d", activeCandidates)
	}

	var status string
	if err := conn.QueryRowContext(ctx, "SELECT status FROM documentation_submissions WHERE id = ?", submissionID).Scan(&status); err != nil {
		t.Fatalf("read submission status: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected active submission, got %q", status)
	}
}

func TestActivateCandidatesIsIdempotent(t *testing.T) {
	ctx := context.Background()
	conn := openActivationTestDB(t, ctx)
	defer conn.Close()

	submissionID, runID := insertActivationSubmission(t, ctx, conn)
	insertCandidate(t, ctx, conn, submissionID, runID, "Alpha Page", "https://example.com/docs/alpha")

	first, err := ActivateCandidates(ctx, conn, submissionID)
	if err != nil {
		t.Fatalf("activate first time: %v", err)
	}
	second, err := ActivateCandidates(ctx, conn, submissionID)
	if err != nil {
		t.Fatalf("activate second time: %v", err)
	}
	if second.TopicID != first.TopicID {
		t.Fatalf("expected same topic id, got %d and %d", first.TopicID, second.TopicID)
	}

	var pages int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE topic_id = ?", first.TopicID).Scan(&pages); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	if pages != 1 {
		t.Fatalf("expected one page after rerun, got %d", pages)
	}
}

func TestActivateCandidatesRequiresEligibleCandidates(t *testing.T) {
	ctx := context.Background()
	conn := openActivationTestDB(t, ctx)
	defer conn.Close()

	submissionID, _ := insertActivationSubmission(t, ctx, conn)
	_, err := ActivateCandidates(ctx, conn, submissionID)
	if !errors.Is(err, ErrNoEligibleCandidates) {
		t.Fatalf("expected ErrNoEligibleCandidates, got %v", err)
	}
}

func TestActivateSourceCandidatesCreatesPages(t *testing.T) {
	ctx := context.Background()
	conn := openActivationTestDB(t, ctx)
	defer conn.Close()

	submissionID, runID := insertActivationSubmission(t, ctx, conn)
	sourceID := insertActivationSource(t, ctx, conn, submissionID)
	insertSourceCandidate(t, ctx, conn, submissionID, sourceID, runID, "Source Page", "https://example.com/docs/source")

	result, err := ActivateSourceCandidates(ctx, conn, sourceID)
	if err != nil {
		t.Fatalf("activate source candidates: %v", err)
	}
	if result.TopicSourceID != sourceID {
		t.Fatalf("expected source id %d, got %+v", sourceID, result)
	}
	if result.Activated != 1 {
		t.Fatalf("expected one activated candidate, got %d", result.Activated)
	}

	var pages int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE topic_id = ? AND active = 1", result.TopicID).Scan(&pages); err != nil {
		t.Fatalf("count source pages: %v", err)
	}
	if pages != 1 {
		t.Fatalf("expected one active page, got %d", pages)
	}
}

func openActivationTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "dailydocs.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return conn
}

func insertActivationSource(t *testing.T, ctx context.Context, conn *sql.DB, submissionID int64) int64 {
	t.Helper()

	var topicID int64
	if err := conn.QueryRowContext(ctx, "SELECT id FROM topics WHERE slug = 'example-docs'").Scan(&topicID); errors.Is(err, sql.ErrNoRows) {
		result, insertErr := conn.ExecContext(ctx, "INSERT INTO topics (slug, name, status) VALUES ('example-docs', 'Example Docs', 'active')")
		if insertErr != nil {
			t.Fatalf("insert activation topic: %v", insertErr)
		}
		topicID, insertErr = result.LastInsertId()
		if insertErr != nil {
			t.Fatalf("read activation topic id: %v", insertErr)
		}
	} else if err != nil {
		t.Fatalf("read activation topic: %v", err)
	}

	result, err := conn.ExecContext(ctx, `
		INSERT INTO topic_sources (
			topic_id,
			base_url,
			normalized_url,
			source_host,
			created_from_submission_id
		)
		VALUES (?, 'https://example.com/docs', 'https://example.com/docs', 'example.com', ?)
	`, topicID, submissionID)
	if err != nil {
		t.Fatalf("insert activation source: %v", err)
	}
	sourceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read activation source id: %v", err)
	}
	return sourceID
}

func insertActivationSubmission(t *testing.T, ctx context.Context, conn *sql.DB) (int64, int64) {
	t.Helper()

	result, err := conn.ExecContext(ctx, `
		INSERT INTO documentation_submissions (
			submitted_url,
			normalized_url,
			source_host,
			suggested_topic,
			status
		)
		VALUES ('https://example.com/docs', 'https://example.com/docs', 'example.com', 'Example Docs', 'candidates_ready')
	`)
	if err != nil {
		t.Fatalf("insert submission: %v", err)
	}
	submissionID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read submission id: %v", err)
	}

	result, err = conn.ExecContext(ctx, `
		INSERT INTO pipeline_runs (documentation_submission_id, status)
		VALUES (?, 'completed')
	`, submissionID)
	if err != nil {
		t.Fatalf("insert pipeline run: %v", err)
	}
	runID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read run id: %v", err)
	}
	return submissionID, runID
}

func insertCandidate(t *testing.T, ctx context.Context, conn *sql.DB, submissionID int64, runID int64, title string, rawURL string) {
	t.Helper()

	_, err := conn.ExecContext(ctx, `
		INSERT INTO page_candidates (
			documentation_submission_id,
			pipeline_run_id,
			proposed_topic_slug,
			proposed_topic_name,
			title,
			url,
			normalized_url,
			source,
			word_count,
			score,
			official,
			estimated_minutes,
			status
		)
		VALUES (?, ?, 'example-docs', 'Example Docs', ?, ?, ?, 'example.com', 800, 95, 1, 4, 'eligible')
	`, submissionID, runID, title, rawURL, rawURL)
	if err != nil {
		t.Fatalf("insert candidate: %v", err)
	}
}

func insertSourceCandidate(t *testing.T, ctx context.Context, conn *sql.DB, submissionID int64, sourceID int64, runID int64, title string, rawURL string) {
	t.Helper()

	_, err := conn.ExecContext(ctx, `
		INSERT INTO page_candidates (
			documentation_submission_id,
			pipeline_run_id,
			topic_source_id,
			proposed_topic_slug,
			proposed_topic_name,
			title,
			url,
			normalized_url,
			source,
			word_count,
			score,
			official,
			estimated_minutes,
			status
		)
		VALUES (?, ?, ?, 'example-docs', 'Example Docs', ?, ?, ?, 'example.com', 800, 95, 1, 4, 'eligible')
	`, submissionID, runID, sourceID, title, rawURL, rawURL)
	if err != nil {
		t.Fatalf("insert source candidate: %v", err)
	}
}
