package validator

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ernestns/daily-docs/internal/db"
	"github.com/ernestns/daily-docs/internal/seed"
)

func TestValidateLinksRecordsSuccessAndRedirect(t *testing.T) {
	ctx := context.Background()
	conn := openValidatorTestDB(t, ctx)
	defer conn.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	importValidatorTopic(t, ctx, conn, server.URL+"/old")

	result, err := ValidateLinks(ctx, conn, server.Client(), 3)
	if err != nil {
		t.Fatalf("validate links: %v", err)
	}
	if result.Checked != 1 || result.Healthy != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	var finalURL string
	var failures int
	if err := conn.QueryRowContext(ctx, "SELECT url, verification_failures FROM pages").Scan(&finalURL, &failures); err != nil {
		t.Fatalf("read page: %v", err)
	}
	if finalURL != server.URL+"/new" {
		t.Fatalf("expected redirected URL, got %q", finalURL)
	}
	if failures != 0 {
		t.Fatalf("expected failures reset to 0, got %d", failures)
	}
}

func TestValidateLinksFallsBackToGetForHead405(t *testing.T) {
	ctx := context.Background()
	conn := openValidatorTestDB(t, ctx)
	defer conn.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	importValidatorTopic(t, ctx, conn, server.URL)

	result, err := ValidateLinks(ctx, conn, server.Client(), 3)
	if err != nil {
		t.Fatalf("validate links: %v", err)
	}
	if result.Healthy != 1 {
		t.Fatalf("expected healthy fallback result, got %+v", result)
	}
}

func TestValidateLinksDisablesAfterRepeatedFailures(t *testing.T) {
	ctx := context.Background()
	conn := openValidatorTestDB(t, ctx)
	defer conn.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	importValidatorTopic(t, ctx, conn, server.URL)

	for i := 1; i <= 2; i++ {
		result, err := ValidateLinks(ctx, conn, server.Client(), 3)
		if err != nil {
			t.Fatalf("validate links attempt %d: %v", i, err)
		}
		if result.Disabled != 0 {
			t.Fatalf("did not expect disable on attempt %d: %+v", i, result)
		}
	}

	result, err := ValidateLinks(ctx, conn, server.Client(), 3)
	if err != nil {
		t.Fatalf("validate links third attempt: %v", err)
	}
	if result.Disabled != 1 {
		t.Fatalf("expected disabled link, got %+v", result)
	}

	var active int
	var failures int
	if err := conn.QueryRowContext(ctx, "SELECT active, verification_failures FROM pages").Scan(&active, &failures); err != nil {
		t.Fatalf("read page state: %v", err)
	}
	if active != 0 || failures != 3 {
		t.Fatalf("expected inactive with 3 failures, got active=%d failures=%d", active, failures)
	}
}

func openValidatorTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "dailydocs.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return conn
}

func importValidatorTopic(t *testing.T, ctx context.Context, conn *sql.DB, pageURL string) {
	t.Helper()

	if _, err := seed.ImportTopic(ctx, conn, seed.TopicFile{
		Topic: "http",
		Name:  "HTTP",
		Pages: []seed.PageFile{
			{Title: "Reference", URL: pageURL},
		},
	}); err != nil {
		t.Fatalf("import topic: %v", err)
	}
}
