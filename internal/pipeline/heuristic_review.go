package pipeline

import (
	"context"
	"math"
)

type heuristicReviewer struct{}

func (heuristicReviewer) ReviewPage(_ context.Context, doc document) (Review, error) {
	classification, _ := classify(doc)
	score, _ := scoreDocument(doc, classification)
	estimated := int(math.Ceil(float64(doc.WordCount) / 200.0))
	if estimated < 1 {
		estimated = 1
	}
	return heuristicReview(doc, classification, score, estimated, ""), nil
}

func prefilterReview(doc document, classification string, estimated int) Review {
	if doc.WordCount >= 50 {
		return Review{}
	}
	return Review{
		Decision:         "exclude",
		Classification:   classification,
		Confidence:       1,
		EstimatedMinutes: estimated,
		Rationale:        "Very little extracted text.",
		RejectReason:     "Very little extracted text.",
		GateScore:        0,
		GatePageType:     "too_thin",
		RejectStage:      "prefilter",
		Model:            "prefilter",
		PromptVersion:    reviewPromptVersion,
		InputHash:        reviewInputHash(doc),
	}
}

func heuristicReview(doc document, classification string, score int, estimated int, fallbackReason string) Review {
	decision := "include"
	rationale := "Standalone documentation page with enough extracted content."
	rejectReason := ""
	confidence := 0.65
	gateScore := 80
	gatePageType := "standalone_doc"
	rejectStage := ""

	switch {
	case classification == "Index":
		decision = "exclude"
		rejectReason = "Documentation index or landing page."
		confidence = 0.85
		gateScore = 20
		gatePageType = "index_page"
		rejectStage = "heuristic_gate"
	case classification == "Release Notes":
		decision = "exclude"
		rejectReason = "Release notes are not durable daily reading material."
		confidence = 0.8
		gateScore = 30
		gatePageType = "release_notes"
		rejectStage = "heuristic_gate"
	case classification == "API":
		decision = "exclude"
		rejectReason = "Generated or API reference shaped page."
		confidence = 0.75
		gateScore = 40
		gatePageType = "api_reference"
		rejectStage = "heuristic_gate"
	case doc.WordCount < 100:
		decision = "exclude"
		rejectReason = "Very little extracted text."
		confidence = 0.7
		gateScore = 20
		gatePageType = "too_thin"
		rejectStage = "heuristic_gate"
	case doc.LinkDensity > 0.5:
		decision = "exclude"
		rejectReason = "Page appears to be mostly links."
		confidence = 0.75
		gateScore = 30
		gatePageType = "mostly_links"
		rejectStage = "heuristic_gate"
	case score < DefaultMinScore:
		decision = "exclude"
		rejectReason = "Quality score below threshold."
		confidence = 0.6
		gateScore = 50
		gatePageType = "other"
		rejectStage = "heuristic_gate"
	}
	if fallbackReason != "" {
		rationale = fallbackReason
		if rejectReason == "" {
			rejectReason = fallbackReason
		}
	}
	if rejectReason != "" {
		rationale = rejectReason
	}
	return Review{
		Decision:         decision,
		Classification:   classification,
		Confidence:       confidence,
		EstimatedMinutes: estimated,
		Rationale:        rationale,
		RejectReason:     rejectReason,
		GateScore:        gateScore,
		GatePageType:     gatePageType,
		RejectStage:      rejectStage,
		Model:            "heuristic",
		PromptVersion:    reviewPromptVersion,
		InputHash:        reviewInputHash(doc),
	}
}
