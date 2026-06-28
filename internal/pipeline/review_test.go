package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIReviewerSkipsEnrichmentBelowGateThreshold(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeOpenAIResponse(t, w, map[string]any{
			"dailydocs_score": 30,
			"page_type":       "index_page",
		})
	}))
	defer server.Close()

	reviewer := openAIReviewer{
		client:   server.Client(),
		apiKey:   "test-key",
		model:    "gpt-5-nano",
		endpoint: server.URL,
	}

	review, err := reviewer.ReviewPage(ctx, document{
		Title:         "Docs Home",
		NormalizedURL: server.URL + "/docs",
		H1:            "Docs Home",
		Headings:      []string{"Start"},
		WordCount:     500,
	})
	if err != nil {
		t.Fatalf("review page: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one gate call, got %d", calls)
	}
	if review.Decision != "exclude" {
		t.Fatalf("expected exclude decision, got %+v", review)
	}
	if review.GateScore != 30 || review.RejectStage != "ai_gate" {
		t.Fatalf("expected gate rejection metadata, got %+v", review)
	}
	if review.GateUsage.InputTokens != 620 || review.GateUsage.OutputTokens != 84 || review.GateUsage.ReasoningTokens != 128 || review.GateUsage.TotalTokens != 704 {
		t.Fatalf("expected gate token usage, got %+v", review.GateUsage)
	}
	if review.EnrichmentUsage.TotalTokens != 0 {
		t.Fatalf("did not expect enrichment usage below gate threshold, got %+v", review.EnrichmentUsage)
	}
}

func TestOpenAIReviewerEnrichesAboveGateThreshold(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeOpenAIResponse(t, w, map[string]any{
				"dailydocs_score": 80,
				"page_type":       "concept",
			})
			return
		}
		writeOpenAIResponse(t, w, map[string]any{
			"decision":          "include",
			"classification":    "Concept",
			"confidence":        0.91,
			"estimated_minutes": 7,
			"rationale":         "Standalone concept page.",
			"rejection_reason":  "",
		})
	}))
	defer server.Close()

	reviewer := openAIReviewer{
		client:   server.Client(),
		apiKey:   "test-key",
		model:    "gpt-5-nano",
		endpoint: server.URL,
	}

	review, err := reviewer.ReviewPage(ctx, document{
		Title:         "Ownership",
		NormalizedURL: server.URL + "/docs/ownership",
		H1:            "Ownership",
		Headings:      []string{"Introduction", "Borrowing"},
		Text:          repeatedWords(200),
		WordCount:     200,
	})
	if err != nil {
		t.Fatalf("review page: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected gate and enrichment calls, got %d", calls)
	}
	if review.Decision != "include" || review.Classification != "Concept" {
		t.Fatalf("expected enriched include review, got %+v", review)
	}
	if review.GateScore != 80 || review.GatePageType != "concept" {
		t.Fatalf("expected gate metadata, got %+v", review)
	}
	if review.GateUsage.InputTokens != 620 || review.GateUsage.OutputTokens != 84 || review.GateUsage.ReasoningTokens != 128 || review.GateUsage.TotalTokens != 704 {
		t.Fatalf("expected gate token usage, got %+v", review.GateUsage)
	}
	if review.EnrichmentUsage.InputTokens != 620 || review.EnrichmentUsage.OutputTokens != 84 || review.EnrichmentUsage.ReasoningTokens != 128 || review.EnrichmentUsage.TotalTokens != 704 {
		t.Fatalf("expected enrichment token usage, got %+v", review.EnrichmentUsage)
	}
}
