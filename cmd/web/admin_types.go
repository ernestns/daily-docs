package main

import "time"

const adminSessionCookie = "dailydocs_admin"
const adminSessionTTL = 12 * time.Hour

type adminSubmissionRow struct {
	ID             int64
	SuggestedTopic string
	SourceHost     string
	Status         string
	RequestCount   int
	LastSubmitted  string
	LastError      string
}

type adminSubmissionDetail struct {
	ID             int64
	SuggestedTopic string
	SuggestedSlug  string
	SourceHost     string
	Status         string
	RequestCount   int
	SubmittedURL   string
	NormalizedURL  string
	LastSubmitted  string
	LastError      string
	Sources        []adminSourceRow
	Runs           []adminRunRow
	Candidates     []adminCandidateRow
}

type adminSourceRow struct {
	ID               int64
	SubmissionID     int64
	TopicID          int64
	TopicSlug        string
	TopicName        string
	Status           string
	SourceType       string
	BaseURL          string
	NormalizedURL    string
	LastProcessedAt  string
	LastError        string
	LastDiscoveredAt string
	DiscoveryCount   int
	DiscoverySample  []string
	DiscoveryError   string
	DiscoveryStatus  string
	WorkflowStatus   string
	NextAction       string
}

type adminSourceDetail struct {
	adminSourceRow
	Runs             []adminRunRow
	DiscoveryRuns    []adminDiscoveryRunRow
	Candidates       []adminCandidateRow
	CandidateFilters adminCandidateFilters
	CanDiscover      bool
	CanProcess       bool
	CanActivate      bool
	CanCreateSource  bool
}

type adminRunDetail struct {
	adminRunRow
	SubmissionID      int64
	SourceID          int64
	TopicSlug         string
	TopicName         string
	SourceURL         string
	Candidates        []adminCandidateRow
	CandidateFilters  adminCandidateFilters
	CandidateFilterQS string
}

type adminRunRow struct {
	ID              int64
	Status          string
	StartedAt       string
	CompletedAt     string
	DiscoveredCount int
	CrawledCount    int
	EligibleCount   int
	RejectedCount   int
	FailureCount    int
	Error           string
}

type adminCandidateRow struct {
	ID               int64
	Title            string
	URL              string
	Classification   string
	Score            int
	Gate             string
	RejectStage      string
	Status           string
	EstimatedMinutes string
	Reason           string
	TokenSummary     string
	ReviewModel      string
	Confidence       string
	Rationale        string
}

type adminDiscoveryRunRow struct {
	ID              int64
	Status          string
	CreatedAt       string
	DiscoveredCount int
	DiscoverySample []string
	DiscoveryError  string
}

type adminSourceActionState struct {
	Status          string
	DiscoveryCount  int
	EligibleCount   int
	ActivatedCount  int
	ProcessingCount int
}

type adminCandidateFilters struct {
	Status   string
	PageType string
	MinScore string
}
