package gluepipeline

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// fakeGlue answers the Glue calls this plugin makes. The fixtures below
// mirror the responses a real account returns, including the workflow run
// graph that moto does not implement.
type fakeGlue struct {
	workflows    []types.Workflow
	workflowRuns map[string][]types.WorkflowRun
	jobs         []types.Job
	jobRuns      map[string][]types.JobRun
	triggers     []types.Trigger
	crawlers     []types.Crawler
	crawls       map[string][]types.CrawlerHistory
	tags         map[string]map[string]string

	// paginate makes the list calls return one name per page, so tests can
	// prove the plugin follows the token to the end.
	paginate bool

	listWorkflowsErr error
	jobRunsErr       error

	batchedWorkflowNames [][]string
	taggedARNs           []string
}

func (f *fakeGlue) ListWorkflows(ctx context.Context, in *glue.ListWorkflowsInput, opts ...func(*glue.Options)) (*glue.ListWorkflowsOutput, error) {
	if f.listWorkflowsErr != nil {
		return nil, f.listWorkflowsErr
	}

	names := make([]string, 0, len(f.workflows))
	for _, workflow := range f.workflows {
		names = append(names, safeStr(workflow.Name))
	}
	return &glue.ListWorkflowsOutput{Workflows: page(names, in.NextToken, f.paginate), NextToken: nextToken(names, in.NextToken, f.paginate)}, nil
}

func (f *fakeGlue) BatchGetWorkflows(ctx context.Context, in *glue.BatchGetWorkflowsInput, opts ...func(*glue.Options)) (*glue.BatchGetWorkflowsOutput, error) {
	f.batchedWorkflowNames = append(f.batchedWorkflowNames, in.Names)

	var out []types.Workflow
	for _, workflow := range f.workflows {
		if contains(in.Names, safeStr(workflow.Name)) {
			out = append(out, workflow)
		}
	}
	return &glue.BatchGetWorkflowsOutput{Workflows: out}, nil
}

func (f *fakeGlue) GetWorkflowRuns(ctx context.Context, in *glue.GetWorkflowRunsInput, opts ...func(*glue.Options)) (*glue.GetWorkflowRunsOutput, error) {
	return &glue.GetWorkflowRunsOutput{Runs: f.workflowRuns[safeStr(in.Name)]}, nil
}

func (f *fakeGlue) ListJobs(ctx context.Context, in *glue.ListJobsInput, opts ...func(*glue.Options)) (*glue.ListJobsOutput, error) {
	names := make([]string, 0, len(f.jobs))
	for _, job := range f.jobs {
		names = append(names, safeStr(job.Name))
	}
	return &glue.ListJobsOutput{JobNames: page(names, in.NextToken, f.paginate), NextToken: nextToken(names, in.NextToken, f.paginate)}, nil
}

func (f *fakeGlue) BatchGetJobs(ctx context.Context, in *glue.BatchGetJobsInput, opts ...func(*glue.Options)) (*glue.BatchGetJobsOutput, error) {
	var out []types.Job
	for _, job := range f.jobs {
		if contains(in.JobNames, safeStr(job.Name)) {
			out = append(out, job)
		}
	}
	return &glue.BatchGetJobsOutput{Jobs: out}, nil
}

func (f *fakeGlue) GetJobRuns(ctx context.Context, in *glue.GetJobRunsInput, opts ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	if f.jobRunsErr != nil {
		return nil, f.jobRunsErr
	}
	return &glue.GetJobRunsOutput{JobRuns: f.jobRuns[safeStr(in.JobName)]}, nil
}

func (f *fakeGlue) ListTriggers(ctx context.Context, in *glue.ListTriggersInput, opts ...func(*glue.Options)) (*glue.ListTriggersOutput, error) {
	names := make([]string, 0, len(f.triggers))
	for _, trigger := range f.triggers {
		names = append(names, safeStr(trigger.Name))
	}
	return &glue.ListTriggersOutput{TriggerNames: page(names, in.NextToken, f.paginate), NextToken: nextToken(names, in.NextToken, f.paginate)}, nil
}

func (f *fakeGlue) BatchGetTriggers(ctx context.Context, in *glue.BatchGetTriggersInput, opts ...func(*glue.Options)) (*glue.BatchGetTriggersOutput, error) {
	var out []types.Trigger
	for _, trigger := range f.triggers {
		if contains(in.TriggerNames, safeStr(trigger.Name)) {
			out = append(out, trigger)
		}
	}
	return &glue.BatchGetTriggersOutput{Triggers: out}, nil
}

func (f *fakeGlue) GetCrawlers(ctx context.Context, in *glue.GetCrawlersInput, opts ...func(*glue.Options)) (*glue.GetCrawlersOutput, error) {
	return &glue.GetCrawlersOutput{Crawlers: f.crawlers}, nil
}

func (f *fakeGlue) ListCrawls(ctx context.Context, in *glue.ListCrawlsInput, opts ...func(*glue.Options)) (*glue.ListCrawlsOutput, error) {
	return &glue.ListCrawlsOutput{Crawls: f.crawls[safeStr(in.CrawlerName)]}, nil
}

func (f *fakeGlue) GetTags(ctx context.Context, in *glue.GetTagsInput, opts ...func(*glue.Options)) (*glue.GetTagsOutput, error) {
	arn := safeStr(in.ResourceArn)
	f.taggedARNs = append(f.taggedARNs, arn)
	return &glue.GetTagsOutput{Tags: f.tags[arn]}, nil
}

// page returns one name at a time when pagination is on, otherwise all of
// them. The token is the index of the next name.
func page(names []string, token *string, paginate bool) []string {
	if !paginate {
		return names
	}
	start := tokenIndex(token)
	if start >= len(names) {
		return nil
	}
	return names[start : start+1]
}

func nextToken(names []string, token *string, paginate bool) *string {
	if !paginate {
		return nil
	}
	next := tokenIndex(token) + 1
	if next >= len(names) {
		return nil
	}
	return aws.String(string(rune('0' + next)))
}

func tokenIndex(token *string) int {
	if token == nil || *token == "" {
		return 0
	}
	return int((*token)[0] - '0')
}

func contains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// newSource wires a Source to a fake client with every feature on.
func newSource(client glueAPI) *Source {
	return &Source{
		config: &Config{
			AWSConfig:         &pluginsdk.AWSConfig{},
			IncludeWorkflows:  true,
			IncludeTriggers:   true,
			IncludeRunHistory: true,
			RunHistoryLimit:   20,
			IncludeCrawlers:   true,
		},
		client:  client,
		region:  "us-east-1",
		account: "123456789012",
	}
}

var (
	created   = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	runStart  = time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	runFinish = time.Date(2026, 9, 7, 2, 12, 0, 0, time.UTC)
)

// seededGlue is a shop account: a daily-etl workflow that crawls the orders
// prefix and then runs two jobs, wired by a scheduled and a conditional
// trigger.
func seededGlue() *fakeGlue {
	return &fakeGlue{
		workflows: []types.Workflow{{
			Name:                 aws.String("daily-etl"),
			Description:          aws.String("Daily ETL"),
			DefaultRunProperties: map[string]string{"env": "prod", "team": "data"},
			CreatedOn:            &created,
			LastModifiedOn:       &created,
			MaxConcurrentRuns:    aws.Int32(1),
			LastRun: &types.WorkflowRun{
				WorkflowRunId: aws.String("wr_1"),
				Status:        types.WorkflowRunStatusCompleted,
				StartedOn:     &runStart,
				CompletedOn:   &runFinish,
				Statistics:    &types.WorkflowRunStatistics{TotalActions: 3, SucceededActions: 3},
			},
			Graph: &types.WorkflowGraph{
				Nodes: []types.Node{
					{Name: aws.String("start-crawler"), Type: types.NodeTypeTrigger, UniqueId: aws.String("n1"), TriggerDetails: &types.TriggerNodeDetails{Trigger: &types.Trigger{
						Name:         aws.String("start-crawler"),
						WorkflowName: aws.String("daily-etl"),
						Type:         types.TriggerTypeScheduled,
						State:        types.TriggerStateActivated,
						Schedule:     aws.String("cron(0 2 * * ? *)"),
						Description:  aws.String("Kick off the crawler"),
						Actions:      []types.Action{{CrawlerName: aws.String("orders-crawler")}},
					}}},
					{Name: aws.String("orders-crawler"), Type: types.NodeTypeCrawler, UniqueId: aws.String("n2")},
					{Name: aws.String("after-crawl"), Type: types.NodeTypeTrigger, UniqueId: aws.String("n3"), TriggerDetails: &types.TriggerNodeDetails{Trigger: &types.Trigger{
						Name:         aws.String("after-crawl"),
						WorkflowName: aws.String("daily-etl"),
						Type:         types.TriggerTypeConditional,
						State:        types.TriggerStateActivated,
						Actions:      []types.Action{{JobName: aws.String("load-orders")}},
						Predicate: &types.Predicate{
							Logical:    types.LogicalAny,
							Conditions: []types.Condition{{CrawlerName: aws.String("orders-crawler"), CrawlState: types.CrawlStateSucceeded}},
						},
					}}},
					{Name: aws.String("load-orders"), Type: types.NodeTypeJob, UniqueId: aws.String("n4")},
					{Name: aws.String("aggregate-orders"), Type: types.NodeTypeJob, UniqueId: aws.String("n5")},
				},
				Edges: []types.Edge{
					{SourceId: aws.String("n1"), DestinationId: aws.String("n2")},
					{SourceId: aws.String("n2"), DestinationId: aws.String("n3")},
					{SourceId: aws.String("n3"), DestinationId: aws.String("n4")},
					{SourceId: aws.String("n4"), DestinationId: aws.String("n5")},
				},
			},
		}},
		workflowRuns: map[string][]types.WorkflowRun{
			"daily-etl": {{
				WorkflowRunId: aws.String("wr_1"),
				Status:        types.WorkflowRunStatusCompleted,
				StartedOn:     &runStart,
				CompletedOn:   &runFinish,
				Statistics:    &types.WorkflowRunStatistics{TotalActions: 3, SucceededActions: 3},
			}},
		},
		jobs: []types.Job{
			{
				Name:        aws.String("load-orders"),
				Description: aws.String("Load orders"),
				Command:     &types.JobCommand{Name: aws.String("glueetl"), ScriptLocation: aws.String("s3://marmot-lake/scripts/load_orders.py")},
			},
			{
				Name:    aws.String("aggregate-orders"),
				Command: &types.JobCommand{Name: aws.String("pythonshell"), ScriptLocation: aws.String("s3://marmot-lake/scripts/aggregate.py")},
			},
		},
		jobRuns: map[string][]types.JobRun{
			"load-orders": {
				{
					Id:              aws.String("jr_1"),
					JobName:         aws.String("load-orders"),
					JobRunState:     types.JobRunStateSucceeded,
					StartedOn:       &runStart,
					CompletedOn:     &runFinish,
					ExecutionTime:   720,
					Attempt:         1,
					TriggerName:     aws.String("after-crawl"),
					WorkerType:      types.WorkerTypeG1x,
					NumberOfWorkers: aws.Int32(2),
				},
				{
					Id:           aws.String("jr_2"),
					JobName:      aws.String("load-orders"),
					JobRunState:  types.JobRunStateFailed,
					StartedOn:    &runStart,
					CompletedOn:  &runFinish,
					ErrorMessage: aws.String("OutOfMemoryError"),
				},
			},
		},
		triggers: []types.Trigger{
			{
				Name:         aws.String("start-crawler"),
				WorkflowName: aws.String("daily-etl"),
				Type:         types.TriggerTypeScheduled,
				State:        types.TriggerStateCreated,
				Schedule:     aws.String("cron(0 2 * * ? *)"),
				Description:  aws.String("Kick off the crawler"),
				Actions:      []types.Action{{CrawlerName: aws.String("orders-crawler")}},
			},
			{
				Name:         aws.String("after-crawl"),
				WorkflowName: aws.String("daily-etl"),
				Type:         types.TriggerTypeConditional,
				State:        types.TriggerStateCreated,
				Actions:      []types.Action{{JobName: aws.String("load-orders")}},
				Predicate: &types.Predicate{
					Logical:    types.LogicalAny,
					Conditions: []types.Condition{{CrawlerName: aws.String("orders-crawler"), CrawlState: types.CrawlStateSucceeded}},
				},
			},
		},
		crawlers: []types.Crawler{{
			Name:         aws.String("orders-crawler"),
			DatabaseName: aws.String("shop"),
			Targets:      &types.CrawlerTargets{S3Targets: []types.S3Target{{Path: aws.String("s3://marmot-lake/raw/orders/")}}},
		}},
		crawls: map[string][]types.CrawlerHistory{
			"orders-crawler": {{
				CrawlId:   aws.String("crawl_1"),
				State:     types.CrawlerHistoryStateCompleted,
				StartTime: &runStart,
				EndTime:   &runFinish,
				Summary:   aws.String("Added 1 table"),
			}},
		},
		tags: map[string]map[string]string{
			"arn:aws:glue:us-east-1:123456789012:workflow/daily-etl": {"team": "data"},
		},
	}
}

var errUnavailable = errors.New("glue is unavailable")
