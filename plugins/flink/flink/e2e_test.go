package flink_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real JobManager. They expect
// the cluster to have been seeded with the bundled streaming examples:
//
//	flink run -d /opt/flink/examples/streaming/StateMachineExample.jar   (runs until cancelled)
//	flink run    /opt/flink/examples/streaming/WordCount.jar             (finishes on its own)
//
// and optionally, for the cancelled and failed states:
//
//	flink run -d /opt/flink/examples/streaming/TopSpeedWindowing.jar; flink cancel <jid>
//	flink run -d /opt/flink/examples/streaming/SocketWindowWordCount.jar --hostname nonexistent.invalid --port 9999
//
// Point MARMOT_TEST_FLINK_URL at the JobManager REST endpoint to run them.

func flinkURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("MARMOT_TEST_FLINK_URL")
	if url == "" {
		t.Skip("MARMOT_TEST_FLINK_URL not set; skipping Flink e2e tests")
	}
	return url
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	// Skip before building, so an unset variable costs nothing.
	host := flinkURL(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"host": host})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func pipelineByState(result *pluginsdk.DiscoveryResult, state string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == "Pipeline" && a.Metadata["state"] == state {
			return &result.Assets[i]
		}
	}
	return nil
}

func TestE2E_Meta(t *testing.T) {
	flinkURL(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "flink", meta.ID)
	assert.Equal(t, "Flink", meta.Name)
	assert.Equal(t, "orchestration", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	flinkURL(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
}

func TestE2E_DiscoversTheSeededJobs(t *testing.T) {
	result := discoverE2E(t)

	running := findE2EAsset(result, "Pipeline", "State machine job")
	require.NotNil(t, running, "StateMachineExample should be listed as a running job")
	assert.Equal(t, "RUNNING", running.Metadata["state"])
	assert.Equal(t, "mrn://pipeline/flink/state-machine-job", *running.MRN)
	assert.NotContains(t, running.Metadata, "end_time")
	assert.NotEmpty(t, running.Metadata["flink_version"])

	finished := findE2EAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, finished, "WordCount should be listed as a finished job")
	assert.Equal(t, "FINISHED", finished.Metadata["state"])
	assert.Equal(t, "mrn://pipeline/flink/wordcount", *finished.MRN)
	assert.NotEmpty(t, finished.Metadata["end_time"])
	assert.Equal(t, float64(2), finished.Metadata["vertex_count"])
}

func TestE2E_DiscoversTasksWithTheirEdges(t *testing.T) {
	result := discoverE2E(t)

	source := findE2EAsset(result, "Task", "WordCount/Source: in-memory-input -> tokenizer")
	require.NotNil(t, source)
	assert.Equal(t, "FINISHED", source.Metadata["status"])
	assert.Equal(t, "Source: in-memory-input +- tokenizer", source.Metadata["description"])

	sink := findE2EAsset(result, "Task", "WordCount/counter -> Sink: print-sink")
	require.NotNil(t, sink)

	assert.True(t, hasE2EEdge(result, "mrn://pipeline/flink/wordcount", *source.MRN, "CONTAINS"))
	assert.True(t, hasE2EEdge(result, "mrn://pipeline/flink/wordcount", *sink.MRN, "CONTAINS"))
	assert.True(t, hasE2EEdge(result, *source.MRN, *sink.MRN, "DEPENDS_ON"),
		"the plan says the counter reads from the source")
}

func TestE2E_RecordsRunHistory(t *testing.T) {
	result := discoverE2E(t)

	var finished, running []string
	for _, history := range result.RunHistory {
		for _, run := range history.Runs {
			switch history.AssetMRN {
			case "mrn://pipeline/flink/wordcount":
				finished = append(finished, run.EventType)
				assert.Equal(t, "flink", run.JobNamespace)
				assert.Equal(t, "WordCount", run.JobName)
			case "mrn://pipeline/flink/state-machine-job":
				running = append(running, run.EventType)
			}
		}
	}

	assert.Equal(t, []string{"START", "RUNNING", "COMPLETE"}, finished)
	assert.Equal(t, []string{"START", "RUNNING"}, running)
}

func TestE2E_RecordsACancelledJobAsAborted(t *testing.T) {
	result := discoverE2E(t)

	pipeline := pipelineByState(result, "CANCELED")
	if pipeline == nil {
		t.Skip("no cancelled job on the cluster; the optional seeding step was not run")
	}

	var events []string
	for _, history := range result.RunHistory {
		if history.AssetMRN == *pipeline.MRN {
			for _, run := range history.Runs {
				events = append(events, run.EventType)
			}
		}
	}
	assert.Equal(t, []string{"START", "RUNNING", "ABORT"}, events)
}

func TestE2E_RecordsAFailedJobWithItsCause(t *testing.T) {
	result := discoverE2E(t)

	pipeline := pipelineByState(result, "FAILED")
	if pipeline == nil {
		t.Skip("no failed job on the cluster; the optional seeding step was not run")
	}

	assert.NotEmpty(t, pipeline.Metadata["error"])

	var events []string
	for _, history := range result.RunHistory {
		if history.AssetMRN == *pipeline.MRN {
			for _, run := range history.Runs {
				events = append(events, run.EventType)
			}
		}
	}
	assert.Equal(t, []string{"START", "RUNNING", "FAIL"}, events)
}

func TestE2E_EveryAssetMRNAgreesWithItsOwnFields(t *testing.T) {
	result := discoverE2E(t)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func findE2EAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasE2EEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}
