package pipeline

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
)

func buildCandidate(ctx context.Context, reviewer PageReviewer, sub sourceSubmission, doc document, _ int) candidate {
	classification, tags := classify(doc)
	if isCollectionIndex(sub, doc) {
		classification = "Index"
		tags = append(tags, "index")
	}
	score, components := scoreDocument(doc, classification)

	reason := strings.Join(components, "; ")
	estimated := int(math.Ceil(float64(doc.WordCount) / 200.0))
	if estimated < 1 {
		estimated = 1
	}

	review := prefilterReview(doc, classification, estimated)
	if review.Decision == "" {
		var err error
		review, err = reviewer.ReviewPage(ctx, doc)
		if err != nil {
			review = heuristicReview(doc, classification, score, estimated, fmt.Sprintf("review failed: %v", err))
		}
	}
	if strings.TrimSpace(review.Classification) != "" {
		classification = review.Classification
	}
	if review.EstimatedMinutes > 0 {
		estimated = review.EstimatedMinutes
	}

	status := "rejected"
	rejectReason := review.RejectReason
	if review.Decision == "include" {
		status = "eligible"
		rejectReason = ""
	}
	if review.Model == "heuristic" && score < DefaultMinScore {
		status = "rejected"
		rejectReason = "Quality score below threshold."
		if review.RejectStage == "" {
			review.RejectStage = "heuristic_gate"
		}
	}
	if rejectReason == "" && status == "rejected" {
		rejectReason = firstNonEmpty(review.Rationale, "Review did not include this page.")
	}

	return candidate{
		document:         doc,
		Classification:   classification,
		Tags:             tags,
		Score:            score,
		ScoreComponents:  components,
		EstimatedMinutes: estimated,
		Reason:           reason,
		RejectReason:     rejectReason,
		Status:           status,
		Review:           review,
	}
}

func classify(doc document) (string, []string) {
	value := strings.ToLower(doc.Title + " " + doc.URL)
	switch {
	case strings.Contains(value, "release notes") || strings.Contains(value, "releases"):
		return "Release Notes", []string{"release-notes"}
	case strings.Contains(value, "changelog"):
		return "Release Notes", []string{"changelog"}
	case strings.Contains(value, "archive"):
		return "Archive", []string{"archive"}
	case strings.Contains(value, "api") || isGeneratedAPIReference(doc):
		return "API", []string{"api"}
	case strings.Contains(value, "tutorial"):
		return "Tutorial", []string{"tutorial"}
	case strings.Contains(value, "guide") || strings.Contains(value, "/guide"):
		return "Guide", []string{"guide"}
	case strings.Contains(value, "concept") || strings.Contains(value, "/concept"):
		return "Concept", []string{"concept"}
	case strings.Contains(value, "example"):
		return "Example", []string{"example"}
	case strings.Contains(value, "migration"):
		return "Migration", []string{"migration"}
	case strings.Contains(value, "faq"):
		return "FAQ", []string{"faq"}
	default:
		return "Concept", []string{"concept"}
	}
}

func isGeneratedAPIReference(doc document) bool {
	raw, err := url.Parse(doc.NormalizedURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(raw.Path)
	if strings.Contains(path, "/std/") {
		return true
	}
	generatedMarkers := []string{
		"/struct.",
		"/trait.",
		"/enum.",
		"/fn.",
		"/macro.",
		"/primitive.",
		"/keyword.",
		"/type.",
	}
	for _, marker := range generatedMarkers {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return strings.HasSuffix(path, "/all.html") || strings.HasSuffix(path, "/print.html")
}

func isCollectionIndex(sub sourceSubmission, doc document) bool {
	base, err := url.Parse(sub.NormalizedURL)
	if err != nil {
		return false
	}
	candidate, err := url.Parse(doc.NormalizedURL)
	if err != nil {
		return false
	}
	if !strings.EqualFold(base.Host, candidate.Host) {
		return false
	}

	basePath := strings.TrimSuffix(base.Path, "/")
	candidatePath := strings.TrimSuffix(candidate.Path, "/")
	if candidatePath == basePath {
		return true
	}

	prefix := basePath
	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(candidate.Path, prefix) {
		return false
	}

	relative := strings.Trim(strings.TrimPrefix(candidate.Path, prefix), "/")
	if relative == "" {
		return true
	}
	parts := strings.Split(relative, "/")
	if len(parts) == 1 && strings.HasSuffix(originalPath(doc), "/") {
		return true
	}
	return len(parts) == 2 && parts[1] == "index.html"
}

func originalPath(doc document) string {
	raw, err := url.Parse(doc.URL)
	if err != nil {
		return ""
	}
	return raw.Path
}

func scoreDocument(doc document, classification string) (int, []string) {
	score := 50
	components := []string{"+50 official documentation source"}

	switch classification {
	case "Tutorial", "Guide", "Concept":
		score += 20
		components = append(components, "+20 tutorial/guide/concept")
	case "Release Notes":
		score -= 40
		components = append(components, "-40 release notes")
	case "Migration":
		score -= 40
		components = append(components, "-40 migration")
	case "Archive":
		score -= 30
		components = append(components, "-30 archive")
	case "API":
		score -= 40
		components = append(components, "-40 api reference")
	case "Index":
		score -= 40
		components = append(components, "-40 documentation index")
	}

	if doc.WordCount >= 500 && doc.WordCount <= 3000 {
		score += 15
		components = append(components, "+15 word count")
	}
	if len(doc.Headings) >= 2 {
		score += 10
		components = append(components, "+10 headings")
	}
	if doc.WordCount < 100 {
		score -= 10
		components = append(components, "-10 very short")
	}
	if doc.ParagraphCount >= 3 {
		score += 10
		components = append(components, "+10 paragraphs")
	}
	if doc.LinkDensity > 0.2 {
		score -= 20
		components = append(components, "-20 high link density")
	}
	if doc.LinkDensity > 0.5 {
		score -= 30
		components = append(components, "-30 listing-like link density")
	}
	return score, components
}

func reviewerFromEnv(client *http.Client) PageReviewer {
	reviewer := openAIReviewerFromEnv(client)
	if reviewer.apiKey == "" {
		return heuristicReviewer{}
	}
	return reviewer
}
