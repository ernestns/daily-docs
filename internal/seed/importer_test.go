package seed

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ernestns/daily-docs/internal/db"
)

func TestImportFileCreatesTopicAndPages(t *testing.T) {
	ctx := context.Background()
	conn := openTestDB(t, ctx)
	defer conn.Close()

	path := writeSeed(t, `
topic: sqlite
name: SQLite
description: Embedded SQL database
pages:
  - title: Write-Ahead Logging
    url: https://sqlite.org/wal.html
    source: SQLite Documentation
    official: true
    estimated_minutes: 12
  - title: Partial Indexes
    url: https://sqlite.org/partialindex.html
    source: SQLite Documentation
    official: true
    estimated_minutes: 10
`)

	result, err := ImportFile(ctx, conn, path)
	if err != nil {
		t.Fatalf("import file: %v", err)
	}
	if result.TopicSlug != "sqlite" || result.PagesImported != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}

	var topicCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM topics WHERE slug = 'sqlite' AND name = 'SQLite'").Scan(&topicCount); err != nil {
		t.Fatalf("count topics: %v", err)
	}
	if topicCount != 1 {
		t.Fatalf("expected topic count 1, got %d", topicCount)
	}

	var activePages int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE active = 1").Scan(&activePages); err != nil {
		t.Fatalf("count active pages: %v", err)
	}
	if activePages != 2 {
		t.Fatalf("expected 2 active pages, got %d", activePages)
	}

	var imports int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM imports WHERE topic = 'sqlite' AND status = 'completed' AND pages_found = 2 AND pages_imported = 2").Scan(&imports); err != nil {
		t.Fatalf("count imports: %v", err)
	}
	if imports != 1 {
		t.Fatalf("expected completed import record, got %d", imports)
	}
}

func TestImportFileUpdatesPagesAndDeactivatesOmittedPages(t *testing.T) {
	ctx := context.Background()
	conn := openTestDB(t, ctx)
	defer conn.Close()

	first := writeSeed(t, `
topic: go
name: Go
pages:
  - title: Context
    url: https://go.dev/blog/context
    source: Go Blog
    official: true
  - title: Modules
    url: https://go.dev/doc/modules
    source: Go Documentation
    official: true
`)
	if _, err := ImportFile(ctx, conn, first); err != nil {
		t.Fatalf("first import: %v", err)
	}

	second := writeSeed(t, `
topic: go
name: Go
pages:
  - title: Modules Reference
    url: https://go.dev/doc/modules
    source: Go Documentation
    official: true
  - title: Testing
    url: https://go.dev/doc/tutorial/add-a-test
    source: Go Documentation
    official: true
`)
	if _, err := ImportFile(ctx, conn, second); err != nil {
		t.Fatalf("second import: %v", err)
	}

	var activePages int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE active = 1").Scan(&activePages); err != nil {
		t.Fatalf("count active pages: %v", err)
	}
	if activePages != 2 {
		t.Fatalf("expected 2 active pages, got %d", activePages)
	}

	var inactiveContext int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM pages WHERE url = 'https://go.dev/blog/context' AND active = 0").Scan(&inactiveContext); err != nil {
		t.Fatalf("count inactive context page: %v", err)
	}
	if inactiveContext != 1 {
		t.Fatalf("expected omitted page to be inactive, got %d", inactiveContext)
	}

	var title string
	if err := conn.QueryRowContext(ctx, "SELECT title FROM pages WHERE url = 'https://go.dev/doc/modules' AND reading_order = 1").Scan(&title); err != nil {
		t.Fatalf("read updated title: %v", err)
	}
	if title != "Modules Reference" {
		t.Fatalf("expected updated title, got %q", title)
	}
}

func TestImportFileRejectsDuplicateURLs(t *testing.T) {
	ctx := context.Background()
	conn := openTestDB(t, ctx)
	defer conn.Close()

	path := writeSeed(t, `
topic: docker
name: Docker
pages:
  - title: Build
    url: https://docs.docker.com/build/
  - title: Build Again
    url: https://docs.docker.com/build/
`)

	if _, err := ImportFile(ctx, conn, path); err == nil {
		t.Fatal("expected duplicate url error")
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

func writeSeed(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "topic.yaml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	return path
}
