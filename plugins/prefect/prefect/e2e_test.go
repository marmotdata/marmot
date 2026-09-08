package prefect_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Prefect server.
//
// Bring one up and seed it with:
//
//	docker run -d --name marmot-test-prefect -p 14200:4200 \
//	  prefecthq/prefect:3-latest prefect server start --host 0.0.0.0
//
// then run two flows against it, one that succeeds with three tasks and
// one that fails, and give each a deployment. Point
// MARMOT_TEST_PREFECT_URL at http://localhost:14200/api.

func prefectURL(t *testing.T) string {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_PREFECT_URL")
	if host == "" {
		t.Skip("set MARMOT_TEST_PREFECT_URL to a seeded Prefect API, for example http://localhost:14200/api")
	}

	return host
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := buildBinary(t).Discover(t.Context(), pluginsdk.RawConfig{"host": prefectURL(t)})
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	t.Fatalf("no asset named %q in %d discovered assets", name, len(result.Assets))
	return pluginsdk.Asset{}
}

func hasE2EEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "prefect", meta.ID)
	assert.Equal(t, "Prefect", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAHostWithoutTheAPIPath(t *testing.T) {
	host := prefectURL(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"host": host})

	require.NoError(t, err)
}

func TestE2E_DiscoverFindsBothSeededFlows(t *testing.T) {
	result := discoverE2E(t)

	assert.Equal(t, "Pipeline", assetNamed(t, result, "nightly-etl").Type)
	assert.Equal(t, "Pipeline", assetNamed(t, result, "broken-report").Type)
}

func TestE2E_PipelineCarriesItsDeployment(t *testing.T) {
	result := discoverE2E(t)

	pipeline := assetNamed(t, result, "nightly-etl")
	assert.Equal(t, []string{"nightly"}, metadataList(t, pipeline, "deployments"))
	assert.Equal(t, []string{"0 2 * * *"}, metadataList(t, pipeline, "schedules"))

	require.NotNil(t, pipeline.Description)
	assert.Equal(t, "Nightly ETL that refreshes the order rollups.", *pipeline.Description)
}

func TestE2E_PipelineCarriesTheDeploymentTags(t *testing.T) {
	result := discoverE2E(t)

	assert.Contains(t, assetNamed(t, result, "nightly-etl").Tags, "etl")
	assert.Contains(t, assetNamed(t, result, "broken-report").Tags, "reporting")
}

func TestE2E_PipelineRecordsTheLatestRunState(t *testing.T) {
	result := discoverE2E(t)

	assert.Equal(t, "COMPLETED", assetNamed(t, result, "nightly-etl").Metadata["last_run_state"])
	assert.Equal(t, "FAILED", assetNamed(t, result, "broken-report").Metadata["last_run_state"])
}

func TestE2E_PipelineIgnoresRunsThatHaveNotStartedYet(t *testing.T) {
	// Both deployments have an active schedule, so the server has created
	// SCHEDULED runs with start times in the future. Those must not become
	// the latest run.
	result := discoverE2E(t)

	assert.NotEqual(t, "SCHEDULED", assetNamed(t, result, "nightly-etl").Metadata["last_run_state"])
}

func TestE2E_DiscoverFindsTheTasksOfTheLatestRun(t *testing.T) {
	result := discoverE2E(t)

	var tasks []string
	for _, asset := range result.Assets {
		if asset.Type == "Task" {
			tasks = append(tasks, *asset.Name)
		}
	}

	assert.Contains(t, tasks, "nightly-etl/extract")
	assert.Contains(t, tasks, "nightly-etl/transform")
	assert.Contains(t, tasks, "nightly-etl/load")
}

func TestE2E_TaskNameDropsThePrefectHashSuffix(t *testing.T) {
	result := discoverE2E(t)

	task := assetNamed(t, result, "nightly-etl/extract")
	key, ok := task.Metadata["task_key"].(string)
	require.True(t, ok)
	assert.Regexp(t, `^extract-[0-9a-f]{8}$`, key, "the raw key keeps the hash the name drops")
}

func TestE2E_TaskCalledTwiceIsOneAssetWithTwoRuns(t *testing.T) {
	// The seeded flow calls load twice, on two different upstreams.
	result := discoverE2E(t)

	assert.EqualValues(t, 2, assetNamed(t, result, "nightly-etl/load").Metadata["run_count"])
}

func TestE2E_PipelineContainsItsTasks(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, hasE2EEdge(result,
		"mrn://pipeline/prefect/nightly-etl",
		"mrn://task/prefect/nightly-etl-extract",
		"CONTAINS"))
}

func TestE2E_TaskDependsOnTheTaskItConsumed(t *testing.T) {
	result := discoverE2E(t)

	assert.True(t, hasE2EEdge(result,
		"mrn://task/prefect/nightly-etl-extract",
		"mrn://task/prefect/nightly-etl-transform",
		"DEPENDS_ON"))
	assert.True(t, hasE2EEdge(result,
		"mrn://task/prefect/nightly-etl-transform",
		"mrn://task/prefect/nightly-etl-load",
		"DEPENDS_ON"))
}

func TestE2E_RunHistoryRecordsACompletedRun(t *testing.T) {
	result := discoverE2E(t)

	events := runEventTypes(t, result, "mrn://pipeline/prefect/nightly-etl")
	assert.Contains(t, events, "START")
	assert.Contains(t, events, "COMPLETE")
}

func TestE2E_RunHistoryRecordsAFailedRun(t *testing.T) {
	result := discoverE2E(t)

	events := runEventTypes(t, result, "mrn://pipeline/prefect/broken-report")
	assert.Contains(t, events, "START")
	assert.Contains(t, events, "FAIL")
}

func TestE2E_RunHistoryIsFiledUnderTheFlowName(t *testing.T) {
	result := discoverE2E(t)

	for _, history := range result.RunHistory {
		if history.AssetMRN != "mrn://pipeline/prefect/nightly-etl" {
			continue
		}
		require.NotEmpty(t, history.Runs)
		assert.Equal(t, "prefect", history.Runs[0].JobNamespace)
		assert.Equal(t, "nightly-etl", history.Runs[0].JobName)
		assert.False(t, history.Runs[0].EventTime.IsZero())
		return
	}

	t.Fatal("no run history for nightly-etl")
}

func TestE2E_DiscoverEmitsNoLineageIntoAMissingAsset(t *testing.T) {
	// A self-hosted server has no Assets API, so there is nothing to build
	// table lineage from and every edge has to stay inside this run.
	result := discoverE2E(t)

	created := make(map[string]struct{}, len(result.Assets))
	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		created[*asset.MRN] = struct{}{}
	}

	for _, edge := range result.Lineage {
		assert.Contains(t, created, edge.Source)
		assert.Contains(t, created, edge.Target)
	}
}

func TestE2E_EveryAssetMRNAgreesWithItsOwnFields(t *testing.T) {
	result := discoverE2E(t)
	require.NotEmpty(t, result.Assets)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		require.NotEmpty(t, asset.Providers)
		assert.Equal(t, "Prefect", asset.Providers[0])
	}
}

// metadataList reads a list out of an asset's metadata. Crossing the wire
// turns a []string into a []interface{}, which is the shape the Marmot
// host receives, so a test on the far side has to read it back that way.
func metadataList(t *testing.T, asset pluginsdk.Asset, key string) []string {
	t.Helper()

	raw, ok := asset.Metadata[key].([]interface{})
	require.True(t, ok, "metadata %q is not a list: %#v", key, asset.Metadata[key])

	values := make([]string, 0, len(raw))
	for _, item := range raw {
		value, ok := item.(string)
		require.True(t, ok, "metadata %q holds a non-string: %#v", key, item)
		values = append(values, value)
	}

	return values
}

func runEventTypes(t *testing.T, result *pluginsdk.DiscoveryResult, assetMRN string) []string {
	t.Helper()

	for _, history := range result.RunHistory {
		if history.AssetMRN != assetMRN {
			continue
		}
		types := make([]string, 0, len(history.Runs))
		for _, event := range history.Runs {
			types = append(types, event.EventType)
		}
		return types
	}

	t.Fatalf("no run history for %s", assetMRN)
	return nil
}
