package pipeline

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverURLReturnsScopedCandidateURLs(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Docs Home", "Docs Home", []string{
			`<a href="/docs/guide">Guide</a>`,
			`<a href="/docs/book/">Book</a>`,
			`<a href="/blog/out-of-scope">Blog</a>`,
		})
	})
	mux.HandleFunc("/docs/book/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Book Home", "Book Home", []string{
			`<a href="/docs/book/chapter-1">Chapter 1</a>`,
		})
	})
	mux.HandleFunc("/docs/book/chapter-1", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Chapter 1", "Chapter 1", nil)
	})
	mux.HandleFunc("/docs/guide", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Guide", "Guide", nil)
	})
	mux.HandleFunc("/blog/out-of-scope", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Blog", "Blog", nil)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs/", Options{Client: server.Client(), MaxPages: 10})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	if result.NormalizedURL != server.URL+"/docs" {
		t.Fatalf("expected normalized base URL, got %q", result.NormalizedURL)
	}
	if result.DiscoveredCount != 4 {
		t.Fatalf("expected 4 discovered URLs, got %+v", result)
	}
	for _, expected := range []string{server.URL + "/docs", server.URL + "/docs/book", server.URL + "/docs/book/chapter-1", server.URL + "/docs/guide"} {
		if !containsString(result.URLs, expected) {
			t.Fatalf("expected discovered URL %q, got %v", expected, result.URLs)
		}
	}
	if containsString(result.URLs, server.URL+"/blog/out-of-scope") {
		t.Fatalf("did not expect out-of-scope URL, got %v", result.URLs)
	}
}

func TestDiscoverURLScopesFileSourceToParentDirectory(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Docs Index", "Docs Index", []string{
			`<a href="/wal.html">WAL</a>`,
			`<a href="/queryplanner.html">Query Planner</a>`,
			`<a href="/news.html">News</a>`,
		})
	})
	mux.HandleFunc("/wal.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "WAL", "WAL", nil)
	})
	mux.HandleFunc("/queryplanner.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Query Planner", "Query Planner", nil)
	})
	mux.HandleFunc("/news.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "News", "News", nil)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs.html", Options{Client: server.Client(), MaxPages: 10})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	for _, expected := range []string{server.URL + "/docs.html", server.URL + "/wal.html", server.URL + "/queryplanner.html"} {
		if !containsString(result.URLs, expected) {
			t.Fatalf("expected discovered URL %q, got %v", expected, result.URLs)
		}
	}
}

func TestDiscoverURLSkipsAssetLinks(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Docs Home", "Docs Home", []string{
			`<a href="/docs/guide.html">Guide</a>`,
			`<a href="/docs/gopher.jpg">Gopher</a>`,
			`<a href="/docs/app.js">Script</a>`,
		})
	})
	mux.HandleFunc("/docs/guide.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Guide", "Guide", nil)
	})
	mux.HandleFunc("/docs/gopher.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpg"))
	})
	mux.HandleFunc("/docs/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte("js"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs/", Options{Client: server.Client(), MaxPages: 10})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	if containsString(result.URLs, server.URL+"/docs/gopher.jpg") || containsString(result.URLs, server.URL+"/docs/app.js") {
		t.Fatalf("did not expect asset URLs, got %v", result.URLs)
	}
	if !containsString(result.URLs, server.URL+"/docs/guide.html") {
		t.Fatalf("expected guide URL, got %v", result.URLs)
	}
}

func TestDiscoverURLFollowsSitemapIndex(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs/concepts/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Concepts", "Concepts", nil)
	})
	mux.HandleFunc("/docs/concepts/workloads/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Workloads", "Workloads", nil)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>%s/docs-sitemap.xml</loc></sitemap>
</sitemapindex>`, serverHost(r))
	})
	mux.HandleFunc("/docs-sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="utf-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/docs/concepts/workloads/</loc></url>
	<url><loc>%s/blog/out-of-scope/</loc></url>
</urlset>`, serverHost(r), serverHost(r))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs/concepts/", Options{Client: server.Client(), MaxPages: 10})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	if !containsString(result.URLs, server.URL+"/docs/concepts/workloads") {
		t.Fatalf("expected sitemap child URL, got %v", result.URLs)
	}
	if containsString(result.URLs, server.URL+"/blog/out-of-scope") {
		t.Fatalf("did not expect out-of-scope sitemap URL, got %v", result.URLs)
	}
}

func TestDiscoverURLFollowsEmbeddedNavigationFrame(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Docs Home", "Docs Home", []string{
			`<a href="/docs/book/">Book</a>`,
		})
	})
	mux.HandleFunc("/docs/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Book</title></head>
<body>
	<nav><iframe src="toc.html"></iframe></nav>
	<main><h1>Book</h1></main>
</body>
</html>`)
	})
	mux.HandleFunc("/docs/book/toc.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Table of Contents", "Table of Contents", []string{
			`<a href="chapter-1.html">Chapter 1</a>`,
			`<a href="chapter-2.html">Chapter 2</a>`,
		})
	})
	mux.HandleFunc("/docs/book/chapter-1.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Chapter 1", "Chapter 1", nil)
	})
	mux.HandleFunc("/docs/book/chapter-2.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Chapter 2", "Chapter 2", nil)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs/", Options{Client: server.Client(), MaxPages: 10})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	for _, expected := range []string{server.URL + "/docs/book/chapter-1.html", server.URL + "/docs/book/chapter-2.html"} {
		if !containsString(result.URLs, expected) {
			t.Fatalf("expected embedded navigation URL %q, got %v", expected, result.URLs)
		}
	}
	if containsString(result.URLs, server.URL+"/docs/book/toc.html") {
		t.Fatalf("did not expect crawl-only navigation URL, got %v", result.URLs)
	}
}

func TestDiscoverURLPrioritizesCurrentPageChildrenBeforeNoisySiblings(t *testing.T) {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		links := []string{
			`<a href="/docs/book/">Book</a>`,
			`<a href="/docs/reference/">Reference</a>`,
		}
		writeDoc(w, "Docs Home", "Docs Home", links)
	})
	mux.HandleFunc("/docs/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html>
<html>
<head><title>Book</title></head>
<body>
	<nav><iframe src="toc.html"></iframe></nav>
	<main><h1>Book</h1></main>
</body>
</html>`)
	})
	mux.HandleFunc("/docs/book/toc.html", func(w http.ResponseWriter, r *http.Request) {
		writeDoc(w, "Table of Contents", "Table of Contents", []string{
			`<a href="chapter-1.html">Chapter 1</a>`,
			`<a href="chapter-2.html">Chapter 2</a>`,
		})
	})
	mux.HandleFunc("/docs/reference/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/reference/" {
			writeDoc(w, "Reference Item", "Reference Item", nil)
			return
		}
		links := []string{}
		for i := 0; i < 20; i++ {
			links = append(links, fmt.Sprintf(`<a href="/docs/reference/item-%d.html">Item %d</a>`, i, i))
		}
		writeDoc(w, "Reference", "Reference", links)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	result, err := DiscoverURL(ctx, server.URL+"/docs/", Options{Client: server.Client(), MaxPages: 8})
	if err != nil {
		t.Fatalf("discover url: %v", err)
	}
	for _, expected := range []string{server.URL + "/docs/book/chapter-1.html", server.URL + "/docs/book/chapter-2.html"} {
		if !containsString(result.URLs, expected) {
			t.Fatalf("expected current page child URL %q before noisy siblings, got %v", expected, result.URLs)
		}
	}
	if containsString(result.URLs, server.URL+"/docs/book/toc.html") {
		t.Fatalf("did not expect crawl-only navigation URL, got %v", result.URLs)
	}
}
