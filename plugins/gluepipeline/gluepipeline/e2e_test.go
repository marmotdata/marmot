package gluepipeline_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/smithy-go"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Glue API. Point
// MARMOT_TEST_GLUEPIPELINE_ENDPOINT at one, for example a moto server:
//
//	docker run -d --name marmot-test-gluepipeline -p 15560:5000 motoserver/moto:latest
//	MARMOT_TEST_GLUEPIPELINE_ENDPOINT=http://localhost:15560 go test ./...

const endpointEnv = "MARMOT_TEST_GLUEPIPELINE_ENDPOINT"

var seedOnce sync.Once

func endpoint(t *testing.T) string {
	t.Helper()

	value := os.Getenv(endpointEnv)
	if value == "" {
		t.Skipf("set %s to a Glue endpoint, for example http://localhost:15560 for moto", endpointEnv)
	}
	return value
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func testConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	return pluginsdk.RawConfig{
		"credentials": map[string]interface{}{
			"use_default": false,
			"id":          "test",
			"secret":      "test",
			"region":      "us-east-1",
			"endpoint":    endpoint(t),
		},
	}
}

// glueClient builds a client through the same AWS config helper the plugin
// uses, so the test talks to the endpoint the same way.
func glueClient(t *testing.T) *glue.Client {
	t.Helper()

	awsConfig := &pluginsdk.AWSConfig{Credentials: pluginsdk.AWSCredentials{
		ID:       "test",
		Secret:   "test",
		Region:   "us-east-1",
		Endpoint: endpoint(t),
	}}
	cfg, err := awsConfig.NewAWSConfig(context.Background())
	require.NoError(t, err)

	return glue.NewFromConfig(cfg)
}

// seed creates a shop account: a daily-etl workflow that crawls the orders
// prefix and then runs two jobs, plus the runs the plugin reports. It runs
// once per test binary and tolerates a container that is already seeded.
func seed(t *testing.T) {
	t.Helper()
	endpoint(t)

	seedOnce.Do(func() {
		ctx := context.Background()
		client := glueClient(t)

		created(t, errorOf(client.CreateDatabase(ctx, &glue.CreateDatabaseInput{
			DatabaseInput: &types.DatabaseInput{Name: aws.String("shop"), Description: aws.String("Shop warehouse")},
		})))

		created(t, errorOf(client.CreateTable(ctx, &glue.CreateTableInput{
			DatabaseName: aws.String("shop"),
			TableInput: &types.TableInput{
				Name: aws.String("orders"),
				StorageDescriptor: &types.StorageDescriptor{
					Location: aws.String("s3://marmot-lake/raw/orders/"),
					Columns:  []types.Column{{Name: aws.String("id"), Type: aws.String("bigint")}},
				},
			},
		})))

		for _, job := range []struct{ name, command, script string }{
			{"load-orders", "glueetl", "s3://marmot-lake/scripts/load_orders.py"},
			{"aggregate-orders", "pythonshell", "s3://marmot-lake/scripts/aggregate.py"},
		} {
			created(t, errorOf(client.CreateJob(ctx, &glue.CreateJobInput{
				Name:        aws.String(job.name),
				Description: aws.String("Job " + job.name),
				Role:        aws.String("arn:aws:iam::123456789012:role/GlueRole"),
				Command: &types.JobCommand{
					Name:           aws.String(job.command),
					ScriptLocation: aws.String(job.script),
				},
				GlueVersion:     aws.String("4.0"),
				WorkerType:      types.WorkerTypeG1x,
				NumberOfWorkers: aws.Int32(2),
			})))
		}

		created(t, errorOf(client.CreateCrawler(ctx, &glue.CreateCrawlerInput{
			Name:         aws.String("orders-crawler"),
			Role:         aws.String("arn:aws:iam::123456789012:role/GlueRole"),
			DatabaseName: aws.String("shop"),
			Description:  aws.String("Crawl the orders prefix"),
			Schedule:     aws.String("cron(0 1 * * ? *)"),
			Targets:      &types.CrawlerTargets{S3Targets: []types.S3Target{{Path: aws.String("s3://marmot-lake/raw/orders/")}}},
		})))

		created(t, errorOf(client.CreateWorkflow(ctx, &glue.CreateWorkflowInput{
			Name:                 aws.String("daily-etl"),
			Description:          aws.String("Daily ETL"),
			DefaultRunProperties: map[string]string{"env": "prod"},
			MaxConcurrentRuns:    aws.Int32(1),
		})))

		created(t, errorOf(client.CreateTrigger(ctx, &glue.CreateTriggerInput{
			Name:         aws.String("start-crawler"),
			WorkflowName: aws.String("daily-etl"),
			Type:         types.TriggerTypeScheduled,
			Schedule:     aws.String("cron(0 2 * * ? *)"),
			Description:  aws.String("Kick off the crawler"),
			Actions:      []types.Action{{CrawlerName: aws.String("orders-crawler")}},
		})))

		created(t, errorOf(client.CreateTrigger(ctx, &glue.CreateTriggerInput{
			Name:         aws.String("after-crawl"),
			WorkflowName: aws.String("daily-etl"),
			Type:         types.TriggerTypeConditional,
			Description:  aws.String("Run the jobs after the crawl"),
			Actions:      []types.Action{{JobName: aws.String("load-orders")}, {JobName: aws.String("aggregate-orders")}},
			Predicate: &types.Predicate{
				Logical:    types.LogicalAny,
				Conditions: []types.Condition{{LogicalOperator: types.LogicalOperatorEquals, CrawlerName: aws.String("orders-crawler"), CrawlState: types.CrawlStateSucceeded}},
			},
		})))

		created(t, errorOf(client.TagResource(ctx, &glue.TagResourceInput{
			ResourceArn: aws.String("arn:aws:glue:us-east-1:123456789012:workflow/daily-etl"),
			TagsToAdd:   map[string]string{"team": "data"},
		})))

		_, err := client.StartJobRun(ctx, &glue.StartJobRunInput{JobName: aws.String("load-orders")})
		require.NoError(t, err)
		_, err = client.StartWorkflowRun(ctx, &glue.StartWorkflowRunInput{Name: aws.String("daily-etl")})
		require.NoError(t, err)
	})
}

// errorOf keeps only the error of an AWS call, so seeding reads as a list of
// statements.
func errorOf[T any](_ T, err error) error {
	return err
}

// created treats "it is already there" as success, so the tests can run twice
// against the same container.
func created(t *testing.T, err error) {
	t.Helper()

	var alreadyExists *types.AlreadyExistsException
	if errors.As(err, &alreadyExists) {
		return
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AlreadyExistsException" {
		return
	}
	require.NoError(t, err)
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	seed(t)

	result, err := buildBinary(t).Discover(t.Context(), testConfig(t))
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, value string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.MRN != nil && *asset.MRN == value {
			return asset
		}
	}
	require.FailNowf(t, "asset not found", "no asset with MRN %s", value)
	return pluginsdk.Asset{}
}

func findEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func findRuns(result *pluginsdk.DiscoveryResult, assetMRN string) []pluginsdk.RunHistoryEvent {
	for _, history := range result.RunHistory {
		if history.AssetMRN == assetMRN {
			return history.Runs
		}
	}
	return nil
}

func TestE2E_Meta(t *testing.T) {
	endpoint(t)
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "gluepipeline", meta.ID)
	assert.Equal(t, "Glue Pipelines", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateAcceptsCredentials(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), testConfig(t))
	require.NoError(t, err)
}

func TestE2E_ValidateRejectsARunHistoryLimitOutOfRange(t *testing.T) {
	config := testConfig(t)
	config["run_history_limit"] = 500

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_DiscoversTheWorkflowAsAPipeline(t *testing.T) {
	result := discoverE2E(t)

	pipeline := findAsset(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "Pipeline", pipeline.Type)
	assert.Equal(t, []string{"Glue"}, pipeline.Providers)
	require.NotNil(t, pipeline.Description)
	assert.Equal(t, "Daily ETL", *pipeline.Description)
	assert.Equal(t, "env=prod", pipeline.Metadata["default_run_properties"])
	assert.Equal(t, "us-east-1", pipeline.Metadata["region"])
}

func TestE2E_PipelineLinksToTheConsole(t *testing.T) {
	result := discoverE2E(t)

	pipeline := findAsset(t, result, "mrn://pipeline/glue/daily-etl")
	require.Len(t, pipeline.ExternalLinks, 1)
	assert.Equal(t, "Open in AWS Console", pipeline.ExternalLinks[0].Name)
}

// The Glue endpoint does not always return the run graph. The workflow's own
// triggers still describe its steps, which is what this asserts.
func TestE2E_DiscoversTheWorkflowSteps(t *testing.T) {
	result := discoverE2E(t)

	scheduled := findAsset(t, result, "mrn://task/glue/daily-etl-start-crawler")
	assert.Equal(t, "Task", scheduled.Type)
	assert.Equal(t, "daily-etl/start-crawler", *scheduled.Name)
	assert.Equal(t, "SCHEDULED", scheduled.Metadata["trigger_type"])
	assert.Equal(t, "cron(0 2 * * ? *)", scheduled.Metadata["trigger_schedule"])

	conditional := findAsset(t, result, "mrn://task/glue/daily-etl-after-crawl")
	assert.Equal(t, "ANY: crawler orders-crawler SUCCEEDED", conditional.Metadata["trigger_predicate"])
	assert.Equal(t, "job load-orders, job aggregate-orders", conditional.Metadata["trigger_actions"])
}

func TestE2E_PipelineContainsItsSteps(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, findEdge(result, "mrn://pipeline/glue/daily-etl", "mrn://task/glue/daily-etl-start-crawler", "CONTAINS"))
	assert.True(t, findEdge(result, "mrn://pipeline/glue/daily-etl", "mrn://task/glue/daily-etl-after-crawl", "CONTAINS"))
}

func TestE2E_StepsLinkToTheJobsAndCrawlersTheyRun(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, findEdge(result, "mrn://task/glue/daily-etl-start-crawler", "mrn://crawler/glue/orders-crawler", "DEPENDS_ON"))
	assert.True(t, findEdge(result, "mrn://task/glue/daily-etl-after-crawl", "mrn://job/glue/load-orders", "DEPENDS_ON"))
	assert.True(t, findEdge(result, "mrn://crawler/glue/orders-crawler", "mrn://task/glue/daily-etl-after-crawl", "DEPENDS_ON"))
}

func TestE2E_CrawlerLinksItsBucketAndDatabase(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, findEdge(result, "mrn://bucket/s3/marmot-lake", "mrn://crawler/glue/orders-crawler", "FEEDS"))
	assert.True(t, findEdge(result, "mrn://crawler/glue/orders-crawler", "mrn://database/glue/shop", "PRODUCES"))
}

func TestE2E_DoesNotDuplicateAssetsTheGluePluginOwns(t *testing.T) {
	result := discoverE2E(t)

	for _, asset := range result.Assets {
		assert.Contains(t, []string{"Pipeline", "Task"}, asset.Type)
	}
}

// Job run history is what Marmot lacks for Glue today, so this is the
// assertion that matters most.
func TestE2E_AttachesJobRunsToTheGlueJobAsset(t *testing.T) {
	result := discoverE2E(t)

	runs := findRuns(result, "mrn://job/glue/load-orders")
	require.NotEmpty(t, runs, "expected run history on the job asset the Glue plugin owns")

	assert.Equal(t, "glue", runs[0].JobNamespace)
	assert.Equal(t, "load-orders", runs[0].JobName)
	assert.Equal(t, "START", runs[0].EventType)
	assert.NotEmpty(t, runs[0].RunID)
	assert.False(t, runs[0].EventTime.IsZero())
	assert.Equal(t, "COMPLETE", runs[1].EventType)
	assert.Equal(t, runs[0].RunID, runs[1].RunID)
	assert.Equal(t, "SUCCEEDED", runs[1].RunFacets["state"])
}

func TestE2E_AttachesWorkflowRunsToThePipeline(t *testing.T) {
	result := discoverE2E(t)

	runs := findRuns(result, "mrn://pipeline/glue/daily-etl")
	require.NotEmpty(t, runs)

	assert.Equal(t, "daily-etl", runs[0].JobName)
	assert.Equal(t, "START", runs[0].EventType)
	assert.Equal(t, "RUNNING", runs[1].EventType, "moto leaves a started workflow run in RUNNING")
}

func TestE2E_ReadsWorkflowTags(t *testing.T) {
	config := testConfig(t)
	config["tags_to_metadata"] = true
	seed(t)

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	pipeline := findAsset(t, result, "mrn://pipeline/glue/daily-etl")
	assert.Equal(t, "data", pipeline.Metadata["tag_team"])
}

func TestE2E_SkipsRunHistoryWhenDisabled(t *testing.T) {
	config := testConfig(t)
	config["include_run_history"] = false
	seed(t)

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	assert.Empty(t, result.RunHistory)
	assert.NotEmpty(t, result.Assets, "assets are still discovered")
}
