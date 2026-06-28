package topicsource

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ernestns/daily-docs/internal/db"
	"github.com/ernestns/daily-docs/internal/submission"
)

func TestCreateFromSubmissionCreatesTopicSource(t *testing.T) {
	ctx := context.Background()
	conn := openTestDB(t, ctx)
	defer conn.Close()

	sub, err := submission.Create(ctx, conn, submission.CreateInput{
		URL:            "https://doc.rust-lang.org/stable/book/",
		SuggestedTopic: "Rust",
	})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}

	source, err := CreateFromSubmission(ctx, conn, CreateFromSubmissionInput{
		SubmissionID: sub.ID,
		TopicSlug:    "rust",
		TopicName:    "Rust",
	})
	if err != nil {
		t.Fatalf("create topic source: %v", err)
	}
	if source.TopicSlug != "rust" {
		t.Fatalf("expected rust topic, got %+v", source)
	}
	if source.NormalizedURL != "https://doc.rust-lang.org/stable/book" {
		t.Fatalf("expected normalized source URL, got %q", source.NormalizedURL)
	}
	if !source.CreatedFromSubmissionID.Valid || source.CreatedFromSubmissionID.Int64 != sub.ID {
		t.Fatalf("expected source to retain submission id, got %+v", source.CreatedFromSubmissionID)
	}

	var status string
	if err := conn.QueryRowContext(ctx, "SELECT status FROM documentation_submissions WHERE id = ?", sub.ID).Scan(&status); err != nil {
		t.Fatalf("read submission status: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected active submission, got %q", status)
	}
}

func TestRecordDiscoveryPreviewStoresLatestAndHistory(t *testing.T) {
	ctx := context.Background()
	conn := openTestDB(t, ctx)
	defer conn.Close()

	sub, err := submission.Create(ctx, conn, submission.CreateInput{
		URL:            "https://doc.rust-lang.org/stable/book/",
		SuggestedTopic: "Rust",
	})
	if err != nil {
		t.Fatalf("create submission: %v", err)
	}

	source, err := CreateFromSubmission(ctx, conn, CreateFromSubmissionInput{
		SubmissionID: sub.ID,
		TopicSlug:    "rust",
		TopicName:    "Rust",
	})
	if err != nil {
		t.Fatalf("create topic source: %v", err)
	}

	if err := RecordDiscoveryPreview(ctx, conn, source.ID, DiscoveryPreview{
		Count:  2,
		Sample: []string{"https://doc.rust-lang.org/stable/book/ch04-01-what-is-ownership.html"},
	}); err != nil {
		t.Fatalf("record discovery preview: %v", err)
	}

	var sourceCount int
	var historyCount int
	var status string
	if err := conn.QueryRowContext(ctx, "SELECT status, discovery_count FROM topic_sources WHERE id = ?", source.ID).Scan(&status, &sourceCount); err != nil {
		t.Fatalf("read source discovery preview: %v", err)
	}
	if status != "ready_to_process" || sourceCount != 2 {
		t.Fatalf("expected latest preview on source, got status=%q count=%d", status, sourceCount)
	}
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM source_discovery_runs WHERE topic_source_id = ?", source.ID).Scan(&historyCount); err != nil {
		t.Fatalf("count discovery history: %v", err)
	}
	if historyCount != 1 {
		t.Fatalf("expected one history row, got %d", historyCount)
	}
}

func openTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "dailydocs.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return conn
}
