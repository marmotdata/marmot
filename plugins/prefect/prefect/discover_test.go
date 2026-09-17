package prefect

import (
	"encoding/json"
	"fmt"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seededPrefect is the fake loaded with the whole captured workspace: two
// flows, a deployment each, one run each and the nightly run's tasks.
func seededPrefect(t *testing.T) *fakePrefect {
	t.Helper()

	return newFakePrefect().
		withFlows(t, flowsFixture).
		withDeployments(t, nightlyFlowID, nightlyDeploymentsFixture).
		withDeployments(t, brokenFlowID, brokenDeploymentsFixture).
		withFlowRuns(t, nightlyFlowID, nightlyRunsFixture).
		withFlowRuns(t, brokenFlowID, brokenRunsFixture).
		withTaskRuns(t, nightlyRunID, nightlyTaskRunsFixture)
}

func discover(t *testing.T, fake *fakePrefect, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := pluginsdk.RawConfig{"host": fake.start(t)}
	for key, value := range overrides {
		config[key] = value
	}

	result, err := (&Source{}).Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

// assetByName finds one discovered asset, failing the test when it is
// absent so the assertion that follows reads cleanly.
func assetByName(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	t.Fatalf("no asset named %q in %d discovered assets", name, len(result.Assets))
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

func TestDiscover_CreatesAPipelinePerFlow(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, "Pipeline", assetByName(t, result, "nightly-etl").Type)
	assert.Equal(t, "Pipeline", assetByName(t, result, "broken-report").Type)
}

func TestDiscover_PipelineIsProvidedByPrefect(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, []string{"Prefect"}, assetByName(t, result, "nightly-etl").Providers)
}

func TestDiscover_PipelineCarriesTheFlowID(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, nightlyFlowID, assetByName(t, result, "nightly-etl").Metadata["flow_id"])
}

func TestDiscover_PipelineDescriptionComesFromTheNewestDeployment(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	description := assetByName(t, result, "nightly-etl").Description
	require.NotNil(t, description)
	assert.Equal(t, "Nightly ETL that refreshes the order rollups.", *description)
}

func TestDiscover_PipelineHasNoDescriptionWithoutADeployment(t *testing.T) {
	fake := newFakePrefect().
		withFlows(t, flowsFixture).
		withFlowRuns(t, nightlyFlowID, nightlyRunsFixture)

	result := discover(t, fake, nil)

	assert.Nil(t, assetByName(t, result, "nightly-etl").Description)
}

func TestDiscover_PipelineCarriesTheCronSchedule(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, []string{"0 2 * * *"}, assetByName(t, result, "nightly-etl").Metadata["schedules"])
}

func TestDiscover_PipelineWritesAnIntervalScheduleInSeconds(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, []string{"every 3600s"}, assetByName(t, result, "broken-report").Metadata["schedules"])
}

func TestDiscover_PipelineListsItsDeployments(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	pipeline := assetByName(t, result, "nightly-etl")
	assert.Equal(t, []string{"nightly"}, pipeline.Metadata["deployments"])
	assert.Equal(t, 1, pipeline.Metadata["deployment_count"])
}

func TestDiscover_PipelineRecordsTheEntrypoint(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, "flows.py:nightly_etl", assetByName(t, result, "nightly-etl").Metadata["entrypoint"])
}

func TestDiscover_PipelineTakesItsTagsFromTheDeployment(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	pipeline := assetByName(t, result, "nightly-etl")
	assert.Equal(t, []string{"etl", "om-source:shop.public.orders"}, pipeline.Metadata["tags"])
	assert.Contains(t, pipeline.Tags, "etl")
}

func TestDiscover_PipelineKeepsTheTagsFromTheIngestConfig(t *testing.T) {
	result := discover(t, seededPrefect(t), pluginsdk.RawConfig{"tags": []string{"orchestration"}})

	assert.Contains(t, assetByName(t, result, "nightly-etl").Tags, "orchestration")
}

func TestDiscover_PipelineRecordsTheLatestRunState(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	pipeline := assetByName(t, result, "nightly-etl")
	assert.Equal(t, "COMPLETED", pipeline.Metadata["last_run_state"])
	assert.Equal(t, "2026-09-08T07:40:29.400518Z", pipeline.Metadata["last_run_at"])
}

func TestDiscover_PipelineSuccessRateCountsOnlyCompletedRuns(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, float64(100), assetByName(t, result, "nightly-etl").Metadata["success_rate"])
	assert.Equal(t, float64(0), assetByName(t, result, "broken-report").Metadata["success_rate"])
}

func TestDiscover_PipelineLinksBackToThePrefectUI(t *testing.T) {
	fake := seededPrefect(t)
	host := fake.start(t)

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": host})
	require.NoError(t, err)

	pipeline := assetByName(t, result, "nightly-etl")
	expected := host + "/flows/flow/" + nightlyFlowID
	assert.Equal(t, expected, pipeline.Metadata["url"])
	require.Len(t, pipeline.ExternalLinks, 1)
	assert.Equal(t, "Open in Prefect", pipeline.ExternalLinks[0].Name)
	assert.Equal(t, expected, pipeline.ExternalLinks[0].URL)
}

func TestDiscover_PipelineRecordsTheSourceItCameFrom(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	sources := assetByName(t, result, "nightly-etl").Sources
	require.Len(t, sources, 1)
	assert.Equal(t, "Prefect", sources[0].Name)
	assert.Equal(t, nightlyFlowID, sources[0].Properties["flow_id"])
}

func TestDiscover_CreatesOneTaskPerDistinctTaskKey(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	var tasks []string
	for _, asset := range result.Assets {
		if asset.Type == "Task" {
			tasks = append(tasks, *asset.Name)
		}
	}

	// Four task runs, three task keys: load ran twice.
	assert.ElementsMatch(t,
		[]string{"nightly-etl/load", "nightly-etl/transform", "nightly-etl/extract"},
		tasks)
}

func TestDiscover_TaskNameDropsThePrefectHashSuffix(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	task := assetByName(t, result, "nightly-etl/extract")
	assert.Equal(t, "Task", task.Type)
	assert.Equal(t, "extract-bf522387", task.Metadata["task_key"], "the raw key stays in metadata")
}

func TestDiscover_TaskRecordsItsFlowAndLastState(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	task := assetByName(t, result, "nightly-etl/transform")
	assert.Equal(t, "nightly-etl", task.Metadata["flow"])
	assert.Equal(t, "COMPLETED", task.Metadata["last_state"])
	assert.Equal(t, "2026-09-08T07:40:29.425084Z", task.Metadata["last_run_at"])
}

func TestDiscover_TaskRunCountCountsRepeatedCalls(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.Equal(t, 2, assetByName(t, result, "nightly-etl/load").Metadata["run_count"])
	assert.Equal(t, 1, assetByName(t, result, "nightly-etl/extract").Metadata["run_count"])
}

func TestDiscover_TaskKeepsTheTagsFromItsRuns(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	task := assetByName(t, result, "nightly-etl/transform")
	assert.Equal(t, []string{"shaping"}, task.Metadata["tags"])
	assert.Contains(t, task.Tags, "shaping")
}

func TestDiscover_PipelineContainsItsTasks(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.True(t, hasEdge(result,
		"mrn://pipeline/prefect/nightly-etl",
		"mrn://task/prefect/nightly-etl-extract",
		"CONTAINS"))
}

func TestDiscover_TaskDependsOnTheTaskItConsumed(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	assert.True(t, hasEdge(result,
		"mrn://task/prefect/nightly-etl-extract",
		"mrn://task/prefect/nightly-etl-transform",
		"DEPENDS_ON"))
	assert.True(t, hasEdge(result,
		"mrn://task/prefect/nightly-etl-transform",
		"mrn://task/prefect/nightly-etl-load",
		"DEPENDS_ON"))
}

func TestDiscover_TaskCalledTwiceGivesOneEdgePerUpstream(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	// load ran on extract's rows and on transform's rows, so it depends on
	// both, but each pair is stated once.
	count := 0
	for _, edge := range result.Lineage {
		if edge.Type == "DEPENDS_ON" && edge.Target == "mrn://task/prefect/nightly-etl-load" {
			count++
		}
	}
	assert.Equal(t, 2, count)
}

func TestDiscover_EmitsStartAndCompleteForASuccessfulRun(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	history := runHistoryFor(t, result, "mrn://pipeline/prefect/nightly-etl")
	require.Len(t, history.Runs, 2)

	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "2026-09-08T07:40:29.400518Z", history.Runs[0].EventTime.UTC().Format("2006-01-02T15:04:05.999999Z"))
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
	assert.Equal(t, "2026-09-08T07:40:29.437435Z", history.Runs[1].EventTime.UTC().Format("2006-01-02T15:04:05.999999Z"))
}

func TestDiscover_EmitsFailForAFailedRun(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	history := runHistoryFor(t, result, "mrn://pipeline/prefect/broken-report")
	require.Len(t, history.Runs, 2)
	assert.Equal(t, "FAIL", history.Runs[1].EventType)
}

func TestDiscover_RunHistoryIsFiledUnderThePrefectNamespace(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	event := runHistoryFor(t, result, "mrn://pipeline/prefect/nightly-etl").Runs[0]
	assert.Equal(t, "prefect", event.JobNamespace)
	assert.Equal(t, "nightly-etl", event.JobName)
	assert.Equal(t, nightlyRunID, event.RunID)
}

func TestDiscover_RunFacetsNameTheDeploymentThatStartedTheRun(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	facets := runHistoryFor(t, result, "mrn://pipeline/prefect/nightly-etl").Runs[0].RunFacets
	assert.Equal(t, "nightly", facets["deployment"])
	assert.Equal(t, "papaya-mayfly", facets["run_name"])
	assert.Equal(t, "Completed", facets["state_name"])
	assert.Equal(t, 0.036917, facets["total_run_time"])
	assert.Equal(t, 0, facets["parameter_count"])
}

func TestDiscover_RunFacetsOmitTheDeploymentForAManualRun(t *testing.T) {
	result := discover(t, seededPrefect(t), nil)

	facets := runHistoryFor(t, result, "mrn://pipeline/prefect/broken-report").Runs[0].RunFacets
	assert.NotContains(t, facets, "deployment", "the failed run was started by hand")
}

func TestDiscover_ExcludesRunsThatHaveNotStartedYet(t *testing.T) {
	fake := seededPrefect(t)
	discover(t, fake, nil)

	filters := fake.filters("flow_runs")
	require.NotEmpty(t, filters)

	// An active schedule pre-creates SCHEDULED runs with future start
	// times; under START_TIME_DESC they would hide every real run.
	body, err := json.Marshal(filters[0])
	require.NoError(t, err)
	assert.Contains(t, string(body), `"not_any_":["SCHEDULED"]`)
	assert.Contains(t, string(body), `"sort":"START_TIME_DESC"`)
}

func TestDiscover_AsksForTheConfiguredNumberOfRuns(t *testing.T) {
	fake := seededPrefect(t)
	discover(t, fake, pluginsdk.RawConfig{"run_history_limit": 25})

	filters := fake.filters("flow_runs")
	require.NotEmpty(t, filters)
	assert.Equal(t, float64(25), filters[0]["limit"])
}

func TestDiscover_SkipsTasksWhenIncludeTasksIsFalse(t *testing.T) {
	fake := seededPrefect(t)
	result := discover(t, fake, pluginsdk.RawConfig{"include_tasks": false})

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Task", asset.Type)
	}
	assert.Zero(t, fake.requestCount("task_runs"))
}

func TestDiscover_SkipsDeploymentsWhenIncludeDeploymentsIsFalse(t *testing.T) {
	fake := seededPrefect(t)
	result := discover(t, fake, pluginsdk.RawConfig{"include_deployments": false})

	assert.Zero(t, fake.requestCount("deployments"))
	assert.NotContains(t, assetByName(t, result, "nightly-etl").Metadata, "schedules")
}

func TestDiscover_SkipsRunHistoryWhenIncludeRunHistoryIsFalse(t *testing.T) {
	fake := seededPrefect(t)
	result := discover(t, fake, pluginsdk.RawConfig{"include_run_history": false})

	assert.Empty(t, result.RunHistory)
}

func TestDiscover_StillReadsOneRunForTasksWhenRunHistoryIsOff(t *testing.T) {
	fake := seededPrefect(t)
	result := discover(t, fake, pluginsdk.RawConfig{"include_run_history": false})

	// Tasks come from the latest run, so that one run is still fetched.
	assert.Equal(t, float64(1), fake.filters("flow_runs")[0]["limit"])
	assert.Equal(t, "Task", assetByName(t, result, "nightly-etl/extract").Type)
}

func TestDiscover_ReadsNoRunsWhenTasksAndRunHistoryAreBothOff(t *testing.T) {
	fake := seededPrefect(t)
	result := discover(t, fake, pluginsdk.RawConfig{"include_run_history": false, "include_tasks": false})

	assert.Zero(t, fake.requestCount("flow_runs"))
	assert.NotContains(t, assetByName(t, result, "nightly-etl").Metadata, "last_run_state")
}

func TestDiscover_ContinuesWhenOneFlowsDeploymentsFail(t *testing.T) {
	fake := seededPrefect(t)
	fake.failDeploymentsFor = nightlyFlowID

	result := discover(t, fake, nil)

	// The flow that failed still becomes a Pipeline, just without the
	// deployment detail, and the other flow is untouched.
	assert.NotContains(t, assetByName(t, result, "nightly-etl").Metadata, "schedules")
	assert.Equal(t, []string{"every 3600s"}, assetByName(t, result, "broken-report").Metadata["schedules"])
}

func TestDiscover_FollowsPaginationToTheLastPage(t *testing.T) {
	flows := make([]map[string]any, 0, 250)
	for i := 0; i < 250; i++ {
		flows = append(flows, map[string]any{"id": fmt.Sprintf("flow-%03d", i), "name": fmt.Sprintf("flow-%03d", i)})
	}
	encoded, err := json.Marshal(flows)
	require.NoError(t, err)

	fake := newFakePrefect().withFlows(t, string(encoded))
	result := discover(t, fake, pluginsdk.RawConfig{"include_tasks": false, "include_deployments": false, "include_run_history": false})

	assert.Len(t, result.Assets, 250)
}

func TestDiscover_FailsWhenTheAPIIsUnreachable(t *testing.T) {
	// A port nothing is listening on, so the very first call fails.
	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1/api"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reaching the Prefect API")
}

func runHistoryFor(t *testing.T, result *pluginsdk.DiscoveryResult, assetMRN string) pluginsdk.AssetRunHistory {
	t.Helper()

	for _, history := range result.RunHistory {
		if history.AssetMRN == assetMRN {
			return history
		}
	}

	t.Fatalf("no run history for %s", assetMRN)
	return pluginsdk.AssetRunHistory{}
}
