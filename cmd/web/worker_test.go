package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ernestns/daily-docs/internal/topicsearch"
)

func TestProcessNextQueuedTopicActivatesOldestQueuedTopic(t *testing.T) {
	ctx := context.Background()
	conn := openWebTestDB(t, ctx)
	defer conn.Close()
	seedQueuedWebTopic(t, ctx, conn, "rust", "Rust")

	app := app{
		db: conn,
		now: func() time.Time {
			return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
		},
		searchProvider: webFakeProvider{
			results: []topicsearch.SearchResult{
				{Title: "Generics", URL: "https://doc.rust-lang.org/stable/book/ch10-00-generics.html"},
			},
		},
	}
	app.processNextQueuedTopic(ctx)

	var status string
	var pageCount int
	if err := conn.QueryRowContext(ctx, "SELECT status FROM topics WHERE slug = 'rust'").Scan(&status); err != nil {
		t.Fatalf("read topic status: %v", err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages").Scan(&pageCount); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	if status != "active" || pageCount != 1 {
		t.Fatalf("expected active topic with page, got status=%q pages=%d", status, pageCount)
	}
}

func TestRunTopicWorkerProcessesQueuedTopic(t *testing.T) {
	ctx := context.Background()
	conn := openWebTestDB(t, ctx)
	defer conn.Close()
	seedQueuedWebTopic(t, ctx, conn, "rust", "Rust")

	app := app{
		db: conn,
		now: func() time.Time {
			return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
		},
		searchProvider: webFakeProvider{
			results: []topicsearch.SearchResult{
				{Title: "Generics", URL: "https://doc.rust-lang.org/stable/book/ch10-00-generics.html"},
			},
		},
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go app.runTopicWorker(workerCtx, 0, 10*time.Millisecond)

	waitForTopicStatus(t, conn, "rust", "active")
}

func seedQueuedWebTopic(t *testing.T, ctx context.Context, conn *sql.DB, slug string, name string) {
	t.Helper()

	if _, err := conn.ExecContext(ctx, "INSERT INTO topics (slug, name, status) VALUES (?, ?, 'queued')", slug, name); err != nil {
		t.Fatalf("seed queued topic: %v", err)
	}
}

func waitForTopicStatus(t *testing.T, conn *sql.DB, slug string, want string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var status string
		err := conn.QueryRow("SELECT status FROM topics WHERE slug = ?", slug).Scan(&status)
		if err == nil && status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("topic %q did not reach status %q", slug, want)
}
