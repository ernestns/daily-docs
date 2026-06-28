package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
)

func openAIReviewerFromEnv(client *http.Client) openAIReviewer {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_REVIEW_PROVIDER")))
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	zaiKey := strings.TrimSpace(os.Getenv("ZAI_API_KEY"))
	if provider == "" {
		provider = "openai"
		if openAIKey == "" {
			return openAIReviewer{}
		}
	}

	switch provider {
	case "openai":
		model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
		if openAIKey == "" {
			return openAIReviewer{}
		}
		if model == "" {
			model = defaultOpenAIModel
		}
		endpoint := strings.TrimSpace(os.Getenv("OPENAI_ENDPOINT"))
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/responses"
		}
		return openAIReviewer{
			client:   client,
			apiKey:   openAIKey,
			model:    model,
			endpoint: endpoint,
			provider: "openai",
		}
	case "zai", "z.ai":
		model := strings.TrimSpace(os.Getenv("ZAI_MODEL"))
		if zaiKey == "" {
			return openAIReviewer{}
		}
		if model == "" {
			model = defaultZAIModel
		}
		endpoint := firstNonEmpty(strings.TrimSpace(os.Getenv("ZAI_ENDPOINT")), strings.TrimSpace(os.Getenv("ZAI_BASE_URL")))
		if endpoint == "" {
			endpoint = "https://api.z.ai/api/paas/v4/chat/completions"
		}
		return openAIReviewer{
			client:   client,
			apiKey:   zaiKey,
			model:    model,
			endpoint: endpoint,
			provider: "zai",
		}
	default:
		return openAIReviewer{}
	}
}

type openAIReviewer struct {
	client   *http.Client
	apiKey   string
	model    string
	endpoint string
	provider string
}

type gateReview struct {
	DailyDocsScore int    `json:"dailydocs_score"`
	PageType       string `json:"page_type"`
}

func gateSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"dailydocs_score": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"maximum":     100,
				"description": "0 means definitely reject; 100 means among the best pages in the documentation set for daily reading.",
			},
			"page_type": map[string]any{
				"type": "string",
				"enum": []string{"tutorial", "guide", "concept", "reference_concept", "api_reference", "index_page", "navigation_page", "print_page", "release_notes", "changelog", "exercise_or_quiz", "playground", "product_page", "too_thin", "mostly_links", "other"},
			},
		},
		"required": []string{"dailydocs_score", "page_type"},
	}
}

func gatePrompt() string {
	return `You are the editor of DailyDocs.

DailyDocs recommends one documentation page each day to software engineers.

Your job is to determine how suitable this page is for that purpose.

A DailyDocs score of 100 means this is among the best pages in the documentation set for daily reading.
A score of 75 means it is worthwhile but not exceptional.
A score below 40 means it should almost never be shown.

DailyDocs wants one focused page, not an entire documentation set in one page.
All-in-one print pages, whole-book pages, pages with hundreds of headings, and pages with tens of thousands of words should usually score below 40 even when the underlying documentation is good.
A focused chapter or guide can still be suitable when it is several thousand words, as long as it is self-contained and readable in one sitting.

Consider:

* educational value
* evergreen content
* standalone readability
* conceptual depth
* practical usefulness
* whether it is self-contained
* whether it teaches a concept
* whether it can be read in one sitting
* whether an experienced engineer would recommend reading it

Do not favor API indexes, release notes, navigation pages, print pages, all-in-one pages, or generated reference material.

Return only JSON matching the schema.`
}

func enrichmentSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"decision": map[string]any{
				"type": "string",
				"enum": []string{"include", "exclude", "needs_review"},
			},
			"classification": map[string]any{
				"type": "string",
				"enum": []string{"Tutorial", "Guide", "Concept", "Reference", "API Reference", "Index", "Release Notes", "Exercise", "Playground", "Product Page", "Other"},
			},
			"confidence": map[string]any{
				"type": "number",
			},
			"estimated_minutes": map[string]any{
				"type": "integer",
			},
			"rationale": map[string]any{
				"type": "string",
			},
			"rejection_reason": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"decision", "classification", "confidence", "estimated_minutes", "rationale", "rejection_reason"},
	}
}

func (r openAIReviewer) ReviewPage(ctx context.Context, doc document) (Review, error) {
	gateResult, err := r.GatePage(ctx, doc, false)
	if err != nil {
		return Review{}, err
	}
	if gateResult.Review.Decision == "exclude" {
		return gateResult.Review, nil
	}

	input := reviewInput(doc)
	var review Review
	enrichmentUsage, err := r.callAIJSON(ctx, "daily_docs_page_enrichment", enrichmentSchema(), "You review documentation page metadata for DailyDocs. Include standalone tutorials, guides, and concept pages. Exclude landing pages, indexes, generated API references, release notes, changelogs, quizzes, playgrounds, and product pages. Return only valid JSON matching the schema.", input, &review, nil)
	if err != nil {
		return Review{}, err
	}
	review.GateScore = gateResult.Review.GateScore
	review.GatePageType = gateResult.Review.GatePageType
	review.GateUsage = gateResult.Review.GateUsage
	review.EnrichmentUsage = enrichmentUsage
	if review.Decision != "include" {
		review.RejectStage = "ai_enrichment"
	}
	review.Model = r.model
	review.PromptVersion = reviewPromptVersion
	review.InputHash = reviewInputHash(doc)
	return review, nil
}

func (r openAIReviewer) GatePage(ctx context.Context, doc document, includeRawResponse bool) (GateDebugResult, error) {
	var gate gateReview
	var raw json.RawMessage
	var rawPtr *json.RawMessage
	if includeRawResponse {
		rawPtr = &raw
	}
	request := r.requestBody("daily_docs_page_gate", gateSchema(), gatePrompt(), gateInput(doc))
	usage, err := r.callAIJSON(ctx, "daily_docs_page_gate", gateSchema(), gatePrompt(), gateInput(doc), &gate, rawPtr)
	if err != nil {
		return GateDebugResult{Request: request}, err
	}

	estimated := int(math.Ceil(float64(doc.WordCount) / 200.0))
	if estimated < 1 {
		estimated = 1
	}
	review := Review{
		Decision:         "include",
		Classification:   "Other",
		Confidence:       float64(gate.DailyDocsScore) / 100,
		EstimatedMinutes: estimated,
		Rationale:        fmt.Sprintf("AI gate score %d met threshold.", gate.DailyDocsScore),
		GateScore:        gate.DailyDocsScore,
		GatePageType:     gate.PageType,
		Model:            r.model,
		PromptVersion:    reviewPromptVersion,
		InputHash:        reviewInputHash(doc),
		GateUsage:        usage,
	}
	if gate.DailyDocsScore < DefaultGateThreshold {
		review.Decision = "exclude"
		review.Rationale = fmt.Sprintf("AI gate score %d below threshold.", gate.DailyDocsScore)
		review.RejectReason = review.Rationale
		review.RejectStage = "ai_gate"
	}
	return GateDebugResult{
		Request:  request,
		Response: raw,
		Review:   review,
	}, nil
}

func (r openAIReviewer) requestBody(schemaName string, schema map[string]any, systemPrompt string, input map[string]any) map[string]any {
	if r.providerName() == "zai" {
		return map[string]any{
			"model": r.model,
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": systemPrompt,
				},
				{
					"role":    "user",
					"content": mustJSON(input),
				},
			},
			"response_format": map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   schemaName,
					"schema": schema,
					"strict": true,
				},
			},
		}
	}
	body := map[string]any{
		"model": r.model,
		"reasoning": map[string]any{
			"effort": "low",
		},
		"input": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": mustJSON(input),
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   schemaName,
				"schema": schema,
				"strict": true,
			},
		},
	}
	return body
}

func (r openAIReviewer) callAIJSON(ctx context.Context, schemaName string, schema map[string]any, systemPrompt string, input map[string]any, output any, rawResponse *json.RawMessage) (TokenUsage, error) {
	body := r.requestBody(schemaName, schema, systemPrompt, input)
	encoded, err := json.Marshal(body)
	if err != nil {
		return TokenUsage{}, err
	}
	endpoint := r.endpoint
	if endpoint == "" {
		if r.providerName() == "zai" {
			endpoint = "https://api.z.ai/api/paas/v4/chat/completions"
		} else {
			endpoint = "https://api.openai.com/v1/responses"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return TokenUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return TokenUsage{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return TokenUsage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenUsage{}, fmt.Errorf("%s review status %d: %s", r.providerName(), resp.StatusCode, shortenText(string(respBody), 500))
	}
	if rawResponse != nil {
		*rawResponse = append((*rawResponse)[:0], respBody...)
	}

	text := responseOutputText(respBody)
	if text == "" {
		return TokenUsage{}, fmt.Errorf("%s review missing output text", r.providerName())
	}
	if err := json.Unmarshal([]byte(text), output); err != nil {
		return TokenUsage{}, fmt.Errorf("decode %s review: %w", r.providerName(), err)
	}
	return responseTokenUsage(respBody), nil
}

func (r openAIReviewer) providerName() string {
	if strings.TrimSpace(r.provider) != "" {
		return strings.TrimSpace(r.provider)
	}
	return "openai"
}

func reviewInput(doc document) map[string]any {
	return map[string]any{
		"title":            truncateText(doc.Title, enrichmentMaxTitleChars),
		"url":              doc.NormalizedURL,
		"canonical_url":    doc.CanonicalURL,
		"description":      truncateText(doc.MetaDescription, enrichmentMaxDescriptionChars),
		"h1":               truncateText(doc.H1, enrichmentMaxTitleChars),
		"breadcrumbs":      sampleStrings(doc.Breadcrumbs, enrichmentMaxBreadcrumbs, enrichmentMaxBreadcrumbChars),
		"headings_sample":  sampleStrings(doc.Headings, enrichmentMaxHeadings, enrichmentMaxHeadingChars),
		"heading_count":    len(doc.Headings),
		"first_paragraph":  truncateText(doc.FirstParagraph, enrichmentMaxFirstParagraphChars),
		"word_count":       doc.WordCount,
		"paragraph_count":  doc.ParagraphCount,
		"link_count":       doc.LinkCount,
		"code_block_count": doc.CodeBlockCount,
		"code_ratio":       doc.CodeRatio,
		"link_density":     doc.LinkDensity,
		"text_excerpt":     truncateText(doc.Text, enrichmentMaxExcerptChars),
	}
}

func gateInput(doc document) map[string]any {
	return map[string]any{
		"title":           truncateText(doc.Title, gateMaxTitleChars),
		"url":             doc.NormalizedURL,
		"canonical_url":   doc.CanonicalURL,
		"description":     truncateText(doc.MetaDescription, gateMaxDescriptionChars),
		"h1":              truncateText(doc.H1, gateMaxTitleChars),
		"breadcrumbs":     sampleStrings(doc.Breadcrumbs, gateMaxBreadcrumbs, gateMaxBreadcrumbChars),
		"headings_sample": sampleStrings(doc.Headings, gateMaxHeadings, gateMaxHeadingChars),
		"heading_count":   len(doc.Headings),
		"first_paragraph": truncateText(doc.FirstParagraph, gateMaxFirstParagraphChars),
		"word_count":      doc.WordCount,
		"paragraph_count": doc.ParagraphCount,
		"link_count":      doc.LinkCount,
		"code_ratio":      doc.CodeRatio,
		"link_density":    doc.LinkDensity,
	}
}

func sampleStrings(values []string, limit int, maxChars int) []string {
	if limit < 0 {
		limit = 0
	}
	if len(values) < limit {
		limit = len(values)
	}
	sample := make([]string, 0, limit)
	for _, value := range values[:limit] {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		sample = append(sample, truncateText(value, maxChars))
	}
	return sample
}

func truncateText(value string, maxChars int) string {
	value = strings.TrimSpace(value)
	if maxChars < 0 || len(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return value[:maxChars]
	}
	return value[:maxChars-3] + "..."
}

func reviewInputHash(doc document) string {
	sum := sha256.Sum256([]byte(mustJSON(reviewInput(doc))))
	return fmt.Sprintf("%x", sum[:])
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func responseOutputText(body []byte) string {
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if parsed.OutputText != "" {
		return parsed.OutputText
	}
	for _, output := range parsed.Output {
		for _, content := range output.Content {
			if content.Text != "" {
				return content.Text
			}
		}
	}
	for _, choice := range parsed.Choices {
		if choice.Message.Content != "" {
			return choice.Message.Content
		}
	}
	return ""
}

func responseTokenUsage(body []byte) TokenUsage {
	var parsed struct {
		Usage struct {
			InputTokens         int `json:"input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			TotalTokens         int `json:"total_tokens"`
			OutputTokensDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
			CompletionTokensDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return TokenUsage{}
	}
	inputTokens := parsed.Usage.InputTokens
	if inputTokens == 0 {
		inputTokens = parsed.Usage.PromptTokens
	}
	outputTokens := parsed.Usage.OutputTokens
	if outputTokens == 0 {
		outputTokens = parsed.Usage.CompletionTokens
	}
	reasoningTokens := parsed.Usage.OutputTokensDetails.ReasoningTokens
	if reasoningTokens == 0 {
		reasoningTokens = parsed.Usage.CompletionTokensDetails.ReasoningTokens
	}
	return TokenUsage{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		ReasoningTokens: reasoningTokens,
		TotalTokens:     parsed.Usage.TotalTokens,
	}
}

func shortenText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
