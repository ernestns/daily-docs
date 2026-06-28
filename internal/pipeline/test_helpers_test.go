package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ernestns/daily-docs/internal/db"
)

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type usageReviewer struct{}

func (usageReviewer) ReviewPage(_ context.Context, doc document) (Review, error) {
	return Review{
		Decision:         "include",
		Classification:   "Concept",
		Confidence:       0.9,
		EstimatedMinutes: 1,
		Rationale:        "Included by test reviewer.",
		GateScore:        90,
		GatePageType:     "concept",
		Model:            "test-reviewer",
		PromptVersion:    reviewPromptVersion,
		InputHash:        reviewInputHash(doc),
		GateUsage: TokenUsage{
			InputTokens:     620,
			OutputTokens:    84,
			ReasoningTokens: 128,
			TotalTokens:     704,
		},
		EnrichmentUsage: TokenUsage{
			InputTokens:     980,
			OutputTokens:    210,
			ReasoningTokens: 256,
			TotalTokens:     1190,
		},
	}, nil
}

func writeOpenAIResponse(t *testing.T, w http.ResponseWriter, value map[string]any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response value: %v", err)
	}
	response := map[string]any{
		"output": []map[string]any{
			{
				"content": []map[string]any{
					{
						"type": "output_text",
						"text": string(encoded),
					},
				},
			},
		},
		"usage": map[string]any{
			"input_tokens":  620,
			"output_tokens": 84,
			"total_tokens":  704,
			"output_tokens_details": map[string]any{
				"reasoning_tokens": 128,
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func openPipelineTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()

	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "dailydocs.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return conn
}

func docsServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Sitemap: http://%s/sitemap.xml\n", r.Host)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset>
  <url><loc>http://%s/docs/concepts/overview</loc></url>
  <url><loc>http://%s/docs/releases</loc></url>
  <url><loc>http://%s/blog/out-of-scope</loc></url>
</urlset>`, r.Host, r.Host, r.Host)
	})
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Docs Home", "Docs Home", []string{
			`<a href="/docs/guide">Guide</a>`,
			`<a href="/docs/releases">Release Notes</a>`,
			`<a href="/blog/out-of-scope">Blog</a>`,
		})
	})
	mux.HandleFunc("/docs/guide", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Guide Page", "Guide Page", nil)
	})
	mux.HandleFunc("/docs/concepts/overview", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Concept Overview", "Concept Overview", nil)
	})
	mux.HandleFunc("/docs/releases", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Release Notes", "Release Notes", nil)
	})
	mux.HandleFunc("/blog/out-of-scope", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Out of Scope", "Out of Scope", nil)
	})
	return httptest.NewServer(mux)
}

func serverHost(r *http.Request) string {
	return "http://" + r.Host
}

func writeDoc(w http.ResponseWriter, title string, h1 string, links []string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head>
  <title>%s</title>
  <meta name="description" content="%s">
</head>
<body>
  <nav>%s</nav>
  <main>
    <h1>%s</h1>
    <h2>First section</h2>
    <p>%s</p>
    <h2>Second section</h2>
    <p>%s</p>
  </main>
</body>
</html>`, title, title, strings.Join(links, "\n"), h1, repeatedWords(80), repeatedWords(80))
}

func repeatedWords(count int) string {
	words := make([]string, 0, count)
	for i := 0; i < count; i++ {
		words = append(words, "documentation")
	}
	return strings.Join(words, " ")
}
