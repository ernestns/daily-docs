package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/ernestns/daily-docs/internal/activation"
	"github.com/ernestns/daily-docs/internal/db"
	"github.com/ernestns/daily-docs/internal/inspect"
	"github.com/ernestns/daily-docs/internal/pipeline"
	"github.com/ernestns/daily-docs/internal/queue"
	"github.com/ernestns/daily-docs/internal/seed"
	"github.com/ernestns/daily-docs/internal/topicsource"
	"github.com/ernestns/daily-docs/internal/validator"
)

func runCommand(ctx context.Context, args []string) error {
	switch args[0] {
	case "import-file":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs import-file path/to/topic.yaml")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := seed.ImportFile(ctx, conn, args[1])
		if err != nil {
			return err
		}

		log.Printf("imported topic=%s pages_found=%d pages_imported=%d", result.TopicSlug, result.PagesFound, result.PagesImported)
		return nil
	case "validate-links":
		if len(args) != 1 {
			return fmt.Errorf("usage: dailydocs validate-links")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := validator.ValidateLinks(ctx, conn, nil, validator.DefaultFailureThreshold)
		if err != nil {
			return err
		}

		log.Printf("validated links checked=%d healthy=%d failed=%d disabled=%d", result.Checked, result.Healthy, result.Failed, result.Disabled)
		return nil
	case "process-submission":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs process-submission submission-id")
		}
		submissionID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || submissionID < 1 {
			return fmt.Errorf("submission-id must be a positive integer")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := pipeline.ProcessSubmission(ctx, conn, submissionID, pipeline.Options{})
		if err != nil {
			return err
		}

		log.Printf("processed submission id=%d run_id=%d discovered=%d crawled=%d eligible=%d rejected=%d failed=%d", result.SubmissionID, result.PipelineRunID, result.DiscoveredCount, result.CrawledCount, result.EligibleCount, result.RejectedCount, result.FailureCount)
		return nil
	case "create-source-from-submission":
		if len(args) < 3 || len(args) > 4 {
			return fmt.Errorf("usage: dailydocs create-source-from-submission submission-id topic-slug [topic-name]")
		}
		submissionID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || submissionID < 1 {
			return fmt.Errorf("submission-id must be a positive integer")
		}
		topicName := ""
		if len(args) == 4 {
			topicName = args[3]
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		source, err := topicsource.CreateFromSubmission(ctx, conn, topicsource.CreateFromSubmissionInput{
			SubmissionID: submissionID,
			TopicSlug:    args[2],
			TopicName:    topicName,
		})
		if err != nil {
			return err
		}
		log.Printf("created topic source id=%d topic=%s url=%s", source.ID, source.TopicSlug, source.NormalizedURL)
		return nil
	case "list-sources":
		if len(args) != 1 {
			return fmt.Errorf("usage: dailydocs list-sources")
		}
		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()
		return topicsource.WriteList(ctx, conn, os.Stdout)
	case "process-source":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs process-source source-id")
		}
		sourceID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || sourceID < 1 {
			return fmt.Errorf("source-id must be a positive integer")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := pipeline.ProcessSource(ctx, conn, sourceID, pipeline.Options{})
		if err != nil {
			return err
		}

		log.Printf("processed source id=%d submission_id=%d run_id=%d discovered=%d crawled=%d eligible=%d rejected=%d failed=%d", result.TopicSourceID, result.SubmissionID, result.PipelineRunID, result.DiscoveredCount, result.CrawledCount, result.EligibleCount, result.RejectedCount, result.FailureCount)
		return nil
	case "inspect-url":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs inspect-url url")
		}
		result, err := pipeline.InspectURL(ctx, args[1], pipeline.Options{})
		if err != nil {
			return err
		}
		return writePrettyJSON(os.Stdout, result)
	case "discover-url":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs discover-url url")
		}
		result, err := pipeline.DiscoverURL(ctx, args[1], pipeline.Options{})
		if err != nil {
			return err
		}
		return writePrettyJSON(os.Stdout, result)
	case "gate-url":
		showRequest := false
		showResponse := false
		var rawURL string
		for _, arg := range args[1:] {
			switch arg {
			case "--show-request":
				showRequest = true
			case "--show-response":
				showResponse = true
			default:
				if rawURL != "" {
					return fmt.Errorf("usage: dailydocs gate-url [--show-request] [--show-response] url")
				}
				rawURL = arg
			}
		}
		if rawURL == "" {
			return fmt.Errorf("usage: dailydocs gate-url [--show-request] [--show-response] url")
		}
		result, err := pipeline.GateURL(ctx, rawURL, pipeline.Options{}, showResponse)
		if err != nil {
			if showRequest && result.Request != nil {
				_ = writePrettyJSON(os.Stdout, map[string]any{"request": result.Request})
			}
			return err
		}
		output := map[string]any{
			"review": result.Review,
		}
		if showRequest {
			output["request"] = result.Request
		}
		if showResponse {
			var raw any
			if len(result.Response) > 0 && json.Unmarshal(result.Response, &raw) == nil {
				output["response"] = raw
			} else {
				output["response"] = string(result.Response)
			}
		}
		return writePrettyJSON(os.Stdout, output)
	case "activate-candidates":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs activate-candidates submission-id")
		}
		submissionID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || submissionID < 1 {
			return fmt.Errorf("submission-id must be a positive integer")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := activation.ActivateCandidates(ctx, conn, submissionID)
		if err != nil {
			return err
		}

		log.Printf("activated candidates submission_id=%d topic=%s pages=%d", result.SubmissionID, result.TopicSlug, result.Activated)
		return nil
	case "process-pending-submissions":
		limit := 5
		if len(args) > 3 {
			return fmt.Errorf("usage: dailydocs process-pending-submissions [--limit N]")
		}
		if len(args) == 3 {
			if args[1] != "--limit" {
				return fmt.Errorf("usage: dailydocs process-pending-submissions [--limit N]")
			}
			parsedLimit, err := strconv.Atoi(args[2])
			if err != nil || parsedLimit < 1 {
				return fmt.Errorf("limit must be a positive integer")
			}
			limit = parsedLimit
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		result, err := queue.ProcessPending(ctx, conn, queue.Options{Limit: limit})
		if err != nil {
			return err
		}

		log.Printf("processed pending submissions claimed=%d processed=%d failed=%d", result.Claimed, result.Processed, result.Failed)
		return nil
	case "list-submissions":
		if len(args) != 1 {
			return fmt.Errorf("usage: dailydocs list-submissions")
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		return inspect.WriteSubmissions(ctx, conn, os.Stdout)
	case "show-submission":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs show-submission submission-id")
		}
		submissionID, err := parsePositiveID(args[1], "submission-id")
		if err != nil {
			return err
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		return inspect.WriteSubmission(ctx, conn, os.Stdout, submissionID)
	case "list-runs":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs list-runs submission-id")
		}
		submissionID, err := parsePositiveID(args[1], "submission-id")
		if err != nil {
			return err
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		return inspect.WriteRuns(ctx, conn, os.Stdout, submissionID)
	case "list-candidates":
		if len(args) != 2 {
			return fmt.Errorf("usage: dailydocs list-candidates submission-id")
		}
		submissionID, err := parsePositiveID(args[1], "submission-id")
		if err != nil {
			return err
		}

		conn, err := db.Open(ctx, os.Getenv("DB_PATH"))
		if err != nil {
			return err
		}
		defer conn.Close()

		return inspect.WriteCandidates(ctx, conn, os.Stdout, submissionID)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parsePositiveID(value string, name string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return id, nil
}

func slugFromTopicName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	previousDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			previousDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			previousDash = false
		default:
			if builder.Len() > 0 && !previousDash {
				builder.WriteByte('-')
				previousDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func writePrettyJSON(out *os.File, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
