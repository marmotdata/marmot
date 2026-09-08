package gluepipeline

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discover(t *testing.T, source *Source) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := source.discover(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// assetByMRN returns the asset with that MRN, failing the test when it is
// missing.
func assetByMRN(t *testing.T, result *pluginsdk.DiscoveryResult, value string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		if *asset.MRN == value {
			return asset
		}
	}
	require.FailNowf(t, "asset not found", "no asset with MRN %s", value)
	return pluginsdk.Asset{}
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func runsFor(result *pluginsdk.DiscoveryResult, assetMRN string) []pluginsdk.RunHistoryEvent {
	for _, history := range result.RunHistory {
		if history.AssetMRN == assetMRN {
			return history.Runs
		}
	}
	return nil
}

func TestDiscover_CreatesOnePipelinePerWorkflow(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "Pipeline", pipeline.Type)
	assert.Equal(t, []string{"Glue"}, pipeline.Providers)
	assert.Equal(t, "daily-etl", *pipeline.Name)
}

func TestDiscover_PipelineDescriptionComesFromTheWorkflow(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	require.NotNil(t, pipeline.Description)
	assert.Equal(t, "Daily ETL", *pipeline.Description)
}

func TestDiscover_PipelineCountsItsSteps(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, 5, pipeline.Metadata["node_count"])
	assert.Equal(t, 2, pipeline.Metadata["job_count"])
	assert.Equal(t, 1, pipeline.Metadata["crawler_count"])
	assert.Equal(t, 2, pipeline.Metadata["trigger_count"])
}

func TestDiscover_PipelineRecordsTheLastRun(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "wr_1", pipeline.Metadata["last_run_id"])
	assert.Equal(t, "COMPLETED", pipeline.Metadata["last_run_status"])
	assert.Equal(t, "2026-09-07T02:00:00Z", pipeline.Metadata["last_run_started"])
	assert.Equal(t, "2026-09-07T02:12:00Z", pipeline.Metadata["last_run_completed"])
	assert.Contains(t, pipeline.Metadata["last_run_statistics"], "succeeded_actions=3")
}

func TestDiscover_PipelineFlattensDefaultRunProperties(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "env=prod, team=data", pipeline.Metadata["default_run_properties"])
}

func TestDiscover_PipelineLinksToTheAWSConsole(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	require.Len(t, pipeline.ExternalLinks, 1)
	assert.Equal(t, "Open in AWS Console", pipeline.ExternalLinks[0].Name)
	assert.Equal(t, "https://us-east-1.console.aws.amazon.com/glue/home?region=us-east-1#/v2/etl-configuration/workflows/view/daily-etl", pipeline.ExternalLinks[0].URL)
}

func TestDiscover_CreatesATaskPerGraphNode(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	var tasks []string
	for _, asset := range result.Assets {
		if asset.Type == "Task" {
			tasks = append(tasks, *asset.Name)
		}
	}
	assert.ElementsMatch(t, []string{
		"daily-etl/start-crawler",
		"daily-etl/orders-crawler",
		"daily-etl/after-crawl",
		"daily-etl/load-orders",
		"daily-etl/aggregate-orders",
	}, tasks)
}

func TestDiscover_TaskRecordsItsNodeType(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-load-orders")
	assert.Equal(t, "job", task.Metadata["node_type"])
	assert.Equal(t, "daily-etl", task.Metadata["workflow"])
	assert.Equal(t, "load-orders", task.Metadata["node_name"])
	assert.Equal(t, "n4", task.Metadata["unique_id"])
}

func TestDiscover_JobTaskRecordsTheJobCommand(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-load-orders")
	assert.Equal(t, "load-orders", task.Metadata["job"])
	assert.Equal(t, "glueetl", task.Metadata["job_type"])
	assert.Equal(t, "s3://marmot-lake/scripts/load_orders.py", task.Metadata["job_script_location"])
}

func TestDiscover_CrawlerTaskRecordsTheCrawler(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-orders-crawler")
	assert.Equal(t, "crawler", task.Metadata["node_type"])
	assert.Equal(t, "orders-crawler", task.Metadata["crawler"])
}

func TestDiscover_TriggerTaskRecordsItsSchedule(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-start-crawler")
	assert.Equal(t, "trigger", task.Metadata["node_type"])
	assert.Equal(t, "SCHEDULED", task.Metadata["trigger_type"])
	assert.Equal(t, "cron(0 2 * * ? *)", task.Metadata["trigger_schedule"])
	assert.Equal(t, "ACTIVATED", task.Metadata["trigger_state"])
	assert.Equal(t, "crawler orders-crawler", task.Metadata["trigger_actions"])
}

func TestDiscover_TriggerTaskRecordsItsPredicate(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-after-crawl")
	assert.Equal(t, "ANY: crawler orders-crawler SUCCEEDED", task.Metadata["trigger_predicate"])
}

func TestDiscover_TriggerTaskTakesItsDescription(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	task := assetByMRN(t, result, "mrn://task/glue/daily-etl-start-crawler")
	require.NotNil(t, task.Description)
	assert.Equal(t, "Kick off the crawler", *task.Description)
}

func TestDiscover_DoesNotCreateJobCrawlerOrDatabaseAssets(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Job", asset.Type, "the Glue plugin owns Job assets")
		assert.NotEqual(t, "Crawler", asset.Type, "the Glue plugin owns Crawler assets")
		assert.NotEqual(t, "Database", asset.Type, "the Glue plugin owns Database assets")
		assert.NotEqual(t, "Bucket", asset.Type, "the S3 plugin owns Bucket assets")
	}
}

func TestDiscover_PipelineContainsItsTasks(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://pipeline/glue/daily-etl", "mrn://task/glue/daily-etl-load-orders", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://pipeline/glue/daily-etl", "mrn://task/glue/daily-etl-orders-crawler", "CONTAINS"))
}

func TestDiscover_GraphEdgesBecomeTaskDependencies(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-start-crawler", "mrn://task/glue/daily-etl-orders-crawler", "DEPENDS_ON"))
	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-load-orders", "mrn://task/glue/daily-etl-aggregate-orders", "DEPENDS_ON"))
}

func TestDiscover_JobTaskDependsOnTheGlueJobAsset(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-load-orders", "mrn://job/glue/load-orders", "DEPENDS_ON"))
}

func TestDiscover_CrawlerTaskDependsOnTheGlueCrawlerAsset(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-orders-crawler", "mrn://crawler/glue/orders-crawler", "DEPENDS_ON"))
}

func TestDiscover_TriggerLinksToTheJobItStarts(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-after-crawl", "mrn://job/glue/load-orders", "DEPENDS_ON"))
}

func TestDiscover_TriggerLinksFromWhatItWaitsFor(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://crawler/glue/orders-crawler", "mrn://task/glue/daily-etl-after-crawl", "DEPENDS_ON"))
}

func TestDiscover_CrawlerReadsItsBucket(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://bucket/s3/marmot-lake", "mrn://crawler/glue/orders-crawler", "FEEDS"))
}

func TestDiscover_CrawlerProducesItsDatabase(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	assert.True(t, hasEdge(result, "mrn://crawler/glue/orders-crawler", "mrn://database/glue/shop", "PRODUCES"))
}

// Without the run graph, which some Glue endpoints and moto do not return,
// the workflow's own triggers still describe its steps.
func TestDiscover_FallsBackToTriggersWhenTheGraphIsMissing(t *testing.T) {
	client := seededGlue()
	client.workflows[0].Graph = nil

	result := discover(t, newSource(client))

	var tasks []string
	for _, asset := range result.Assets {
		if asset.Type == "Task" {
			tasks = append(tasks, *asset.Name)
		}
	}
	assert.ElementsMatch(t, []string{"daily-etl/start-crawler", "daily-etl/after-crawl"}, tasks)
}

func TestDiscover_TriggerFallbackStillLinksTheJob(t *testing.T) {
	client := seededGlue()
	client.workflows[0].Graph = nil

	result := discover(t, newSource(client))

	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-after-crawl", "mrn://job/glue/load-orders", "DEPENDS_ON"))
	assert.True(t, hasEdge(result, "mrn://task/glue/daily-etl-start-crawler", "mrn://crawler/glue/orders-crawler", "DEPENDS_ON"))
}

func TestDiscover_TriggerFallbackIsEmptyWithoutTriggers(t *testing.T) {
	client := seededGlue()
	client.workflows[0].Graph = nil

	source := newSource(client)
	source.config.IncludeTriggers = false
	result := discover(t, source)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Task", asset.Type)
	}
}

func TestDiscover_AttachesJobRunsToTheGlueJobAsset(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	runs := runsFor(result, "mrn://job/glue/load-orders")
	require.Len(t, runs, 4, "two events per run")
	assert.Equal(t, "jr_1", runs[0].RunID)
	assert.Equal(t, "glue", runs[0].JobNamespace)
	assert.Equal(t, "load-orders", runs[0].JobName)
	assert.Equal(t, "START", runs[0].EventType)
	assert.Equal(t, "COMPLETE", runs[1].EventType)
	assert.Equal(t, "FAIL", runs[3].EventType)
}

func TestDiscover_AttachesWorkflowRunsToThePipeline(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	runs := runsFor(result, "mrn://pipeline/glue/daily-etl")
	require.Len(t, runs, 2)
	assert.Equal(t, "wr_1", runs[0].RunID)
	assert.Equal(t, "daily-etl", runs[0].JobName)
	assert.Equal(t, "START", runs[0].EventType)
	assert.Equal(t, "COMPLETE", runs[1].EventType)
}

func TestDiscover_AttachesCrawlsToTheGlueCrawlerAsset(t *testing.T) {
	result := discover(t, newSource(seededGlue()))

	runs := runsFor(result, "mrn://crawler/glue/orders-crawler")
	require.Len(t, runs, 2)
	assert.Equal(t, "crawl_1", runs[0].RunID)
	assert.Equal(t, "COMPLETE", runs[1].EventType)
}

func TestDiscover_LimitsRunHistoryToTheConfiguredCount(t *testing.T) {
	source := newSource(seededGlue())
	source.config.RunHistoryLimit = 1

	result := discover(t, source)

	assert.Len(t, runsFor(result, "mrn://job/glue/load-orders"), 2, "one run, two events")
}

func TestDiscover_SkipsRunHistoryWhenDisabled(t *testing.T) {
	source := newSource(seededGlue())
	source.config.IncludeRunHistory = false

	result := discover(t, source)

	assert.Empty(t, result.RunHistory)
}

func TestDiscover_SkipsWorkflowsWhenDisabled(t *testing.T) {
	source := newSource(seededGlue())
	source.config.IncludeWorkflows = false

	result := discover(t, source)

	assert.Empty(t, result.Assets)
}

func TestDiscover_SkipsCrawlerLineageWhenDisabled(t *testing.T) {
	source := newSource(seededGlue())
	source.config.IncludeCrawlers = false

	result := discover(t, source)

	assert.False(t, hasEdge(result, "mrn://crawler/glue/orders-crawler", "mrn://database/glue/shop", "PRODUCES"))
}

// One job whose runs cannot be read must not lose the rest of the run
// history.
func TestDiscover_ContinuesWhenJobRunsCannotBeRead(t *testing.T) {
	client := seededGlue()
	client.jobRunsErr = errUnavailable

	result := discover(t, newSource(client))

	assert.Empty(t, runsFor(result, "mrn://job/glue/load-orders"))
	assert.NotEmpty(t, runsFor(result, "mrn://pipeline/glue/daily-etl"))
}

func TestDiscover_ReturnsErrorWhenGlueIsUnreachable(t *testing.T) {
	client := seededGlue()
	client.listWorkflowsErr = errUnavailable

	_, err := newSource(client).discover(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading workflows")
}

func TestDiscover_FollowsPaginationToTheEnd(t *testing.T) {
	client := seededGlue()
	client.paginate = true
	client.workflows = append(client.workflows, types.Workflow{Name: aws.String("hourly-etl")})

	result := discover(t, newSource(client))

	assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assetByMRN(t, result, "mrn://pipeline/glue/hourly-etl")
}

func TestDiscover_ReadsWorkflowTagsWhenAsked(t *testing.T) {
	client := seededGlue()
	source := newSource(client)
	source.config.TagsToMetadata = true

	result := discover(t, source)

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "data", pipeline.Metadata["tag_team"])
	assert.Equal(t, []string{"arn:aws:glue:us-east-1:123456789012:workflow/daily-etl"}, client.taggedARNs)
}

func TestDiscover_SkipsTagsByDefault(t *testing.T) {
	client := seededGlue()

	result := discover(t, newSource(client))

	pipeline := assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
	assert.NotContains(t, pipeline.Metadata, "tag_team")
	assert.Empty(t, client.taggedARNs)
}

// Without a caller identity there is no account id, so no ARN can be built
// and the tag call has to be skipped rather than sent with a broken ARN.
func TestDiscover_SkipsTagsWithoutAnAccountID(t *testing.T) {
	client := seededGlue()
	source := newSource(client)
	source.config.TagsToMetadata = true
	source.account = ""

	discover(t, source)

	assert.Empty(t, client.taggedARNs)
}

func TestDiscover_BatchesWorkflowNames(t *testing.T) {
	client := seededGlue()
	for i := 0; i < 30; i++ {
		client.workflows = append(client.workflows, types.Workflow{Name: aws.String("filler-" + string(rune('a'+i)))})
	}

	discover(t, newSource(client))

	require.Len(t, client.batchedWorkflowNames, 2)
	assert.Len(t, client.batchedWorkflowNames[0], 25)
	assert.Len(t, client.batchedWorkflowNames[1], 6)
}

func TestDiscover_RunsWithoutAnAWSSectionInTheConfig(t *testing.T) {
	source := &Source{client: seededGlue(), region: "us-east-1", account: "123456789012"}
	_, err := source.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)

	result := discover(t, source)

	assetByMRN(t, result, "mrn://pipeline/glue/daily-etl")
}
