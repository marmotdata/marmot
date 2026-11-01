package gluepipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog/log"
)

// glueAPI is the part of the AWS Glue client this plugin calls. Depending on
// an interface keeps the discovery code testable without AWS.
type glueAPI interface {
	ListWorkflows(ctx context.Context, in *glue.ListWorkflowsInput, opts ...func(*glue.Options)) (*glue.ListWorkflowsOutput, error)
	BatchGetWorkflows(ctx context.Context, in *glue.BatchGetWorkflowsInput, opts ...func(*glue.Options)) (*glue.BatchGetWorkflowsOutput, error)
	GetWorkflowRuns(ctx context.Context, in *glue.GetWorkflowRunsInput, opts ...func(*glue.Options)) (*glue.GetWorkflowRunsOutput, error)
	ListJobs(ctx context.Context, in *glue.ListJobsInput, opts ...func(*glue.Options)) (*glue.ListJobsOutput, error)
	BatchGetJobs(ctx context.Context, in *glue.BatchGetJobsInput, opts ...func(*glue.Options)) (*glue.BatchGetJobsOutput, error)
	GetJobRuns(ctx context.Context, in *glue.GetJobRunsInput, opts ...func(*glue.Options)) (*glue.GetJobRunsOutput, error)
	ListTriggers(ctx context.Context, in *glue.ListTriggersInput, opts ...func(*glue.Options)) (*glue.ListTriggersOutput, error)
	BatchGetTriggers(ctx context.Context, in *glue.BatchGetTriggersInput, opts ...func(*glue.Options)) (*glue.BatchGetTriggersOutput, error)
	GetCrawlers(ctx context.Context, in *glue.GetCrawlersInput, opts ...func(*glue.Options)) (*glue.GetCrawlersOutput, error)
	ListCrawls(ctx context.Context, in *glue.ListCrawlsInput, opts ...func(*glue.Options)) (*glue.ListCrawlsOutput, error)
	GetTags(ctx context.Context, in *glue.GetTagsInput, opts ...func(*glue.Options)) (*glue.GetTagsOutput, error)
}

// stsAPI is the one call used to resolve the account id for ARNs.
type stsAPI interface {
	GetCallerIdentity(ctx context.Context, in *sts.GetCallerIdentityInput, opts ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

const (
	// batchSize is the largest number of names the BatchGet calls accept.
	batchSize = 25
	// maxPages stops a broken or endless pagination token from looping.
	maxPages = 100
)

// callerAccount returns the AWS account id, or an empty string when it
// cannot be read. Only the tag lookup needs it, so a failure is not fatal.
func callerAccount(ctx context.Context, api stsAPI) string {
	out, err := api.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil || out.Account == nil {
		log.Debug().Err(err).Msg("Could not read the AWS account id, skipping Glue tags")
		return ""
	}
	return *out.Account
}

// glueARN builds the ARN of a Glue resource, for example
// arn:aws:glue:us-east-1:123456789012:workflow/daily-etl. It returns an
// empty string when the region or account is unknown.
func glueARN(region, account, resource, name string) string {
	if region == "" || account == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("arn:aws:glue:%s:%s:%s/%s", region, account, resource, name)
}

// workflowURL is the AWS console page for a workflow.
func workflowURL(region, name string) string {
	if region == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.console.aws.amazon.com/glue/home?region=%s#/v2/etl-configuration/workflows/view/%s", region, region, name)
}

// fetchWorkflows lists every workflow and reads its definition, including
// the run graph.
func (s *Source) fetchWorkflows(ctx context.Context) ([]types.Workflow, error) {
	var names []string
	var token *string

	for page := 0; ; page++ {
		if page >= maxPages {
			log.Warn().Int("pages", maxPages).Msg("Stopped listing workflows at the page limit")
			break
		}

		out, err := s.client.ListWorkflows(ctx, &glue.ListWorkflowsInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing workflows: %w", err)
		}
		names = append(names, out.Workflows...)

		token = out.NextToken
		if token == nil {
			break
		}
	}

	var workflows []types.Workflow
	for _, batch := range chunk(names, batchSize) {
		out, err := s.client.BatchGetWorkflows(ctx, &glue.BatchGetWorkflowsInput{
			Names:        batch,
			IncludeGraph: aws.Bool(true),
		})
		if err != nil {
			log.Warn().Err(err).Strs("workflows", batch).Msg("Failed to read workflow definitions")
			continue
		}
		workflows = append(workflows, out.Workflows...)
	}

	return workflows, nil
}

// fetchJobs lists every job with its definition, keyed by job name.
func (s *Source) fetchJobs(ctx context.Context) (map[string]types.Job, error) {
	var names []string
	var token *string

	for page := 0; ; page++ {
		if page >= maxPages {
			log.Warn().Int("pages", maxPages).Msg("Stopped listing jobs at the page limit")
			break
		}

		out, err := s.client.ListJobs(ctx, &glue.ListJobsInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing jobs: %w", err)
		}
		names = append(names, out.JobNames...)

		token = out.NextToken
		if token == nil {
			break
		}
	}

	jobs := make(map[string]types.Job, len(names))
	for _, batch := range chunk(names, batchSize) {
		out, err := s.client.BatchGetJobs(ctx, &glue.BatchGetJobsInput{JobNames: batch})
		if err != nil {
			log.Warn().Err(err).Strs("jobs", batch).Msg("Failed to read job definitions")
			continue
		}
		for _, job := range out.Jobs {
			if name := safeStr(job.Name); name != "" {
				jobs[name] = job
			}
		}
	}

	return jobs, nil
}

// fetchTriggersByWorkflow reads every trigger and groups the ones that
// belong to a workflow by that workflow's name.
func (s *Source) fetchTriggersByWorkflow(ctx context.Context) (map[string][]types.Trigger, error) {
	var names []string
	var token *string

	for page := 0; ; page++ {
		if page >= maxPages {
			log.Warn().Int("pages", maxPages).Msg("Stopped listing triggers at the page limit")
			break
		}

		out, err := s.client.ListTriggers(ctx, &glue.ListTriggersInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing triggers: %w", err)
		}
		names = append(names, out.TriggerNames...)

		token = out.NextToken
		if token == nil {
			break
		}
	}

	byWorkflow := make(map[string][]types.Trigger)
	for _, batch := range chunk(names, batchSize) {
		out, err := s.client.BatchGetTriggers(ctx, &glue.BatchGetTriggersInput{TriggerNames: batch})
		if err != nil {
			log.Warn().Err(err).Strs("triggers", batch).Msg("Failed to read trigger definitions")
			continue
		}
		for _, trigger := range out.Triggers {
			workflow := safeStr(trigger.WorkflowName)
			if workflow == "" {
				continue
			}
			byWorkflow[workflow] = append(byWorkflow[workflow], trigger)
		}
	}

	return byWorkflow, nil
}

// fetchCrawlers reads every crawler definition.
func (s *Source) fetchCrawlers(ctx context.Context) ([]types.Crawler, error) {
	var crawlers []types.Crawler
	var token *string

	for page := 0; ; page++ {
		if page >= maxPages {
			log.Warn().Int("pages", maxPages).Msg("Stopped listing crawlers at the page limit")
			break
		}

		out, err := s.client.GetCrawlers(ctx, &glue.GetCrawlersInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("listing crawlers: %w", err)
		}
		crawlers = append(crawlers, out.Crawlers...)

		token = out.NextToken
		if token == nil {
			break
		}
	}

	return crawlers, nil
}

// fetchWorkflowRuns reads the most recent runs of one workflow.
func (s *Source) fetchWorkflowRuns(ctx context.Context, name string) ([]types.WorkflowRun, error) {
	out, err := s.client.GetWorkflowRuns(ctx, &glue.GetWorkflowRunsInput{
		Name:         aws.String(name),
		IncludeGraph: aws.Bool(false),
		MaxResults:   aws.Int32(int32(s.config.RunHistoryLimit)),
	})
	if err != nil {
		return nil, fmt.Errorf("getting workflow runs: %w", err)
	}
	return truncate(out.Runs, s.config.RunHistoryLimit), nil
}

// fetchJobRuns reads the most recent runs of one job.
func (s *Source) fetchJobRuns(ctx context.Context, name string) ([]types.JobRun, error) {
	out, err := s.client.GetJobRuns(ctx, &glue.GetJobRunsInput{
		JobName:    aws.String(name),
		MaxResults: aws.Int32(int32(s.config.RunHistoryLimit)),
	})
	if err != nil {
		return nil, fmt.Errorf("getting job runs: %w", err)
	}
	return truncate(out.JobRuns, s.config.RunHistoryLimit), nil
}

// fetchCrawls reads the most recent runs of one crawler.
func (s *Source) fetchCrawls(ctx context.Context, name string) ([]types.CrawlerHistory, error) {
	out, err := s.client.ListCrawls(ctx, &glue.ListCrawlsInput{
		CrawlerName: aws.String(name),
		MaxResults:  aws.Int32(int32(s.config.RunHistoryLimit)),
	})
	if err != nil {
		return nil, fmt.Errorf("listing crawls: %w", err)
	}
	return truncate(out.Crawls, s.config.RunHistoryLimit), nil
}

// fetchTags reads the tags of a Glue resource. Tag lookups are optional, so
// a missing account id or a failed call just yields no tags.
func (s *Source) fetchTags(ctx context.Context, resource, name string) map[string]string {
	if !s.config.TagsToMetadata {
		return nil
	}

	arn := glueARN(s.region, s.account, resource, name)
	if arn == "" {
		return nil
	}

	out, err := s.client.GetTags(ctx, &glue.GetTagsInput{ResourceArn: aws.String(arn)})
	if err != nil {
		log.Warn().Err(err).Str("arn", arn).Msg("Failed to read tags")
		return nil
	}
	return out.Tags
}

// chunk splits names into slices of at most size entries.
func chunk(names []string, size int) [][]string {
	var batches [][]string
	for start := 0; start < len(names); start += size {
		end := min(start+size, len(names))
		batches = append(batches, names[start:end])
	}
	return batches
}

// truncate keeps at most limit entries, for APIs that ignore MaxResults.
func truncate[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// bucketFromS3Path returns the bucket of an s3:// path, so a crawler target
// can be linked to the bucket asset the S3 plugin owns.
func bucketFromS3Path(path string) string {
	trimmed := strings.TrimPrefix(path, "s3://")
	if trimmed == path {
		return ""
	}
	bucket, _, _ := strings.Cut(trimmed, "/")
	return bucket
}
