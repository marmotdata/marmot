package flink

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Pipeline is named after the Flink job, a Task after
// "<pipeline>/<vertex>". mrn.New folds spaces and slashes into hyphens
// and lowercases the rest, so the readable name and the MRN differ, but
// the name is the only thing the identity is derived from.

func testSource(zone string) *Source {
	cfg := &Config{}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestPipelineMRN_IsTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/flink/wordcount", testSource("").pipelineMRN("WordCount"))
}

func TestPipelineMRN_FoldsSpacesInTheJobName(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/flink/state-machine-job", testSource("").pipelineMRN("State machine job"))
}

func TestPipelineMRN_OfADuplicateJobCarriesItsID(t *testing.T) {
	// When two jobs share a name, the newest keeps the bare name and the others get the id appended (see pipelineNames).
	assert.Equal(t, "mrn://asset/pipeline/flink/wordcount-(3cea68fa0ce6b9f13db4e9b8aabf45ba)",
		testSource("").pipelineMRN("WordCount (3cea68fa0ce6b9f13db4e9b8aabf45ba)"))
}

func TestTaskMRN_IsThePipelineAndVertexName(t *testing.T) {
	assert.Equal(t, "mrn://asset/task/flink/wordcount.counter-->-sink:-print-sink",
		testSource("").taskMRN("WordCount", "counter -> Sink: print-sink"))
}

func TestTaskMRN_SameVertexNameInDifferentJobsDiffers(t *testing.T) {
	s := testSource("")
	assert.NotEqual(t, s.taskMRN("A", "Source"), s.taskMRN("B", "Source"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	assert.Equal(t, "mrn://asset/pipeline/flink@prod/wordcount", testSource("prod").pipelineMRN("WordCount"))
	assert.Equal(t, "mrn://asset/task/flink@prod/wordcount.source", testSource("prod").taskMRN("WordCount", "Source"))
}

func TestEveryAsset_MRNIsKindedAndTasksKeepTheirOldMRN(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob(), finishedJob(), canceledJob(), failedJob()), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, "flink", parsed.Provider)
		if a.Type == "Task" {
			require.Len(t, a.PreviousMRNs, 1)
			assert.NotEqual(t, *a.MRN, a.PreviousMRNs[0])
		}
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
