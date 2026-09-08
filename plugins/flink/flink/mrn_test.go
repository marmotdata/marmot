package flink

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Pipeline is named after the Flink job, a Task after
// "<pipeline>/<vertex>". mrn.New folds spaces and slashes into hyphens
// and lowercases the rest, so the readable name and the MRN differ, but
// the name is the only thing the identity is derived from.

func TestPipelineMRN_IsTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/flink/wordcount", assetMRN("Pipeline", "WordCount"))
}

func TestPipelineMRN_FoldsSpacesInTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://pipeline/flink/state-machine-job", assetMRN("Pipeline", "State machine job"))
}

func TestPipelineMRN_OfADuplicateJobCarriesItsID(t *testing.T) {
	// When two jobs share a name, the newest keeps the bare name and the
	// others get the id appended (see pipelineNames), which lands in the
	// MRN as-is.
	assert.Equal(t, "mrn://pipeline/flink/wordcount-(3cea68fa0ce6b9f13db4e9b8aabf45ba)",
		assetMRN("Pipeline", "WordCount (3cea68fa0ce6b9f13db4e9b8aabf45ba)"))
}

func TestTaskMRN_IsThePipelineAndVertexName(t *testing.T) {
	assert.Equal(t, "mrn://task/flink/wordcount-counter-->-sink:-print-sink",
		assetMRN("Task", "WordCount/counter -> Sink: print-sink"))
}

func TestTaskMRN_IsNotAPrefixMatchOnThePipeline(t *testing.T) {
	// Worth stating outright: the Pipeline's MRN is not a prefix of its
	// Tasks'. The Contents tree is built from the CONTAINS edges Discover
	// emits, not by matching MRN prefixes.
	pipeline := assetMRN("Pipeline", "WordCount")
	task := assetMRN("Task", "WordCount/counter -> Sink: print-sink")

	assert.Equal(t, "mrn://pipeline/flink/wordcount", pipeline)
	assert.NotContains(t, task, pipeline)
}

func TestPipelineMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := assetMRN("Pipeline", "State machine job")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTaskMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The slash in the task name is folded into a hyphen, so the parsed
	// MRN still has exactly three parts.
	original := assetMRN("Task", "WordCount/Source: in-memory-input -> tokenizer")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN an asset carries must be exactly that.
	result := discover(t, newFakeJobManager().with(runningJob(), finishedJob(), canceledJob(), failedJob()), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestEveryEdge_PointsAtAnAssetThisRunCreated(t *testing.T) {
	// The server drops edges whose endpoint does not exist, so every edge
	// must be between assets emitted in the same run.
	result := discover(t, newFakeJobManager().with(runningJob(), finishedJob(), canceledJob(), failedJob()), nil)
	require.NotEmpty(t, result.Lineage)

	created := make(map[string]struct{}, len(result.Assets))
	for _, a := range result.Assets {
		created[*a.MRN] = struct{}{}
	}

	for _, edge := range result.Lineage {
		assert.Contains(t, created, edge.Source)
		assert.Contains(t, created, edge.Target)
	}
}

func TestEveryRun_PointsAtAPipelineThisRunCreated(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob(), finishedJob()), nil)
	require.NotEmpty(t, result.RunHistory)

	pipelines := make(map[string]struct{})
	for _, a := range result.Assets {
		if a.Type == "Pipeline" {
			pipelines[*a.MRN] = struct{}{}
		}
	}

	for _, history := range result.RunHistory {
		assert.Contains(t, pipelines, history.AssetMRN)
	}
}
