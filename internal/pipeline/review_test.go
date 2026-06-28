package pipeline

import (
	"context"
	"encoding/json"
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

func TestZAIReviewerUsesChatCompletionsSchema(t *testing.T) {
	ctx := context.Background()
	var request map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeChatCompletionResponse(t, w, map[string]any{
			"dailydocs_score": 82,
			"page_type":       "guide",
		})
	}))
	defer server.Close()

	reviewer := openAIReviewer{
		client:   server.Client(),
		apiKey:   "test-key",
		model:    defaultZAIModel,
		endpoint: server.URL,
		provider: "zai",
	}
	result, err := reviewer.GatePage(ctx, document{
		Title:         "Guide",
		NormalizedURL: server.URL + "/docs/guide",
		H1:            "Guide",
		WordCount:     400,
	}, false)
	if err != nil {
		t.Fatalf("gate page: %v", err)
	}
	if result.Review.Decision != "include" || result.Review.Model != defaultZAIModel {
		t.Fatalf("expected z.ai include review, got %+v", result.Review)
	}
	if result.Review.GateUsage.InputTokens != 610 || result.Review.GateUsage.OutputTokens != 74 || result.Review.GateUsage.ReasoningTokens != 64 || result.Review.GateUsage.TotalTokens != 684 {
		t.Fatalf("expected chat completion token usage, got %+v", result.Review.GateUsage)
	}

	if request["model"] != defaultZAIModel {
		t.Fatalf("expected z.ai model in request, got %+v", request["model"])
	}
	if _, ok := request["messages"].([]any); !ok {
		t.Fatalf("expected chat messages request, got %+v", request)
	}
	responseFormat, ok := request["response_format"].(map[string]any)
	if !ok || responseFormat["type"] != "json_schema" {
		t.Fatalf("expected json schema response format, got %+v", request["response_format"])
	}
	if _, exists := request["input"]; exists {
		t.Fatalf("did not expect OpenAI responses input field in z.ai request: %+v", request)
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

func TestReviewerFromEnvDefaultsToOpenAIWhenBothKeysAreSet(t *testing.T) {
	t.Setenv("AI_REVIEW_PROVIDER", "")
	t.Setenv("ZAI_API_KEY", "test-zai-key")
	t.Setenv("ZAI_MODEL", "")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENAI_MODEL", "")

	reviewer := openAIReviewerFromEnv(http.DefaultClient)
	if reviewer.provider != "openai" {
		t.Fatalf("expected OpenAI provider, got %+v", reviewer.provider)
	}
	if reviewer.apiKey != "test-openai-key" {
		t.Fatalf("expected OpenAI api key, got %q", reviewer.apiKey)
	}
	if reviewer.model != defaultOpenAIModel {
		t.Fatalf("expected default OpenAI model, got %q", reviewer.model)
	}
}
