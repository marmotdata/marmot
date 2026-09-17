package flink

import (
	"strings"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_CreatesAPipelinePerJob(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob(), finishedJob()), nil)

	running := findAsset(result, "Pipeline", "State machine job")
	require.NotNil(t, running)
	assert.Equal(t, []string{"Flink"}, running.Providers)
	assert.Equal(t, "RUNNING", running.Metadata["state"])

	finished := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, finished)
	assert.Equal(t, "FINISHED", finished.Metadata["state"])
}

func TestDiscover_PipelineCarriesJobMetadata(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	pipeline := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, pipeline)

	assert.Equal(t, "3cea68fa0ce6b9f13db4e9b8aabf45ba", pipeline.Metadata["jid"])
	assert.Equal(t, "2026-09-07T21:12:45Z", pipeline.Metadata["start_time"])
	assert.Equal(t, "2026-09-07T21:12:46Z", pipeline.Metadata["end_time"])
	assert.Equal(t, int64(490), pipeline.Metadata["duration_ms"])
	assert.Equal(t, false, pipeline.Metadata["is_stoppable"])
	assert.Equal(t, 1, pipeline.Metadata["parallelism"])
	assert.Equal(t, "Cluster level default restart strategy", pipeline.Metadata["restart_strategy"])
	assert.Equal(t, 2, pipeline.Metadata["vertex_count"])
	assert.Equal(t, "2.3.0", pipeline.Metadata["flink_version"])

	counts, ok := pipeline.Metadata["task_counts"].(map[string]interface{})
	require.True(t, ok, "task_counts should be the per-state map from the overview")
	assert.Equal(t, 2, counts["total"])
	assert.Equal(t, 2, counts["finished"])
	assert.Equal(t, 0, counts["running"])
}

func TestDiscover_OmitsWhatFlinkHasNotSet(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob()), nil)

	pipeline := findAsset(result, "Pipeline", "State machine job")
	require.NotNil(t, pipeline)

	// A running job reports end-time -1 and maxParallelism -1; Flink 2.x
	// no longer reports an execution mode at all.
	assert.NotContains(t, pipeline.Metadata, "end_time")
	assert.NotContains(t, pipeline.Metadata, "max_parallelism")
	assert.NotContains(t, pipeline.Metadata, "execution_mode")
	assert.NotContains(t, pipeline.Metadata, "error")
}

func TestDiscover_LinksToTheJobPageInTheWebUI(t *testing.T) {
	f := newFakeJobManager().with(runningJob(), finishedJob())
	result := discover(t, f, nil)

	running := findAsset(result, "Pipeline", "State machine job")
	require.NotNil(t, running)
	require.Len(t, running.ExternalLinks, 1)
	assert.Equal(t, "Open in Flink", running.ExternalLinks[0].Name)
	assert.True(t, strings.HasSuffix(running.ExternalLinks[0].URL, "/#/job/running/e4ca10917efb90dc4579d7baad113fac/overview"), running.ExternalLinks[0].URL)
	assert.Equal(t, running.ExternalLinks[0].URL, running.Metadata["url"])

	finished := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, finished)
	assert.True(t, strings.HasSuffix(finished.ExternalLinks[0].URL, "/#/job/completed/3cea68fa0ce6b9f13db4e9b8aabf45ba/overview"), finished.ExternalLinks[0].URL)
}

func TestDiscover_RecordsWhyAJobFailed(t *testing.T) {
	result := discover(t, newFakeJobManager().with(failedJob()), nil)

	pipeline := findAsset(result, "Pipeline", "Socket Window WordCount")
	require.NotNil(t, pipeline)

	cause, ok := pipeline.Metadata["error"].(string)
	require.True(t, ok, "a failed job should carry its root cause")
	assert.True(t, strings.HasPrefix(cause, "org.apache.flink.runtime.JobException: Recovery is suppressed"), cause)
}

func TestDiscover_TrimsALongRootCause(t *testing.T) {
	job := failedJob()
	entries := job.exceptions["exceptionHistory"].(map[string]interface{})["entries"].([]interface{})
	entries[0].(map[string]interface{})["stacktrace"] = strings.Repeat("x", 2000)

	result := discover(t, newFakeJobManager().with(job), nil)

	pipeline := findAsset(result, "Pipeline", "Socket Window WordCount")
	require.NotNil(t, pipeline)
	assert.Len(t, pipeline.Metadata["error"], errorLimit)
}

func TestDiscover_OnlyAsksForExceptionsOfFailedJobs(t *testing.T) {
	f := newFakeJobManager().with(finishedJob()).failing("/jobs/3cea68fa0ce6b9f13db4e9b8aabf45ba/exceptions")

	result := discover(t, f, nil)

	// The endpoint is broken, and a finished job never needed it.
	assert.NotNil(t, findAsset(result, "Pipeline", "WordCount"))
}

func TestDiscover_CreatesATaskPerVertex(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	source := findAsset(result, "Task", "WordCount/Source: in-memory-input -> tokenizer")
	require.NotNil(t, source)
	assert.Equal(t, []string{"Flink"}, source.Providers)

	assert.NotNil(t, findAsset(result, "Task", "WordCount/counter -> Sink: print-sink"))
}

func TestDiscover_TaskCarriesVertexMetadata(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	task := findAsset(result, "Task", "WordCount/counter -> Sink: print-sink")
	require.NotNil(t, task)

	assert.Equal(t, "90bea66de1c231edf33913ecd54406c1", task.Metadata["vertex_id"])
	assert.Equal(t, "WordCount", task.Metadata["pipeline"])
	assert.Equal(t, "3cea68fa0ce6b9f13db4e9b8aabf45ba", task.Metadata["jid"])
	assert.Equal(t, "FINISHED", task.Metadata["status"])
	assert.Equal(t, 1, task.Metadata["parallelism"])
	assert.Equal(t, 128, task.Metadata["max_parallelism"])
	assert.Equal(t, "2026-09-07T21:12:46Z", task.Metadata["start_time"])
	assert.Equal(t, "2026-09-07T21:12:46Z", task.Metadata["end_time"])
	assert.Equal(t, int64(194), task.Metadata["duration_ms"])
	assert.Equal(t, int64(287), task.Metadata["read_records"])
	assert.Equal(t, int64(0), task.Metadata["write_records"])
	assert.Equal(t, int64(6356), task.Metadata["read_bytes"])
	assert.Equal(t, int64(0), task.Metadata["write_bytes"])
	assert.Equal(t, "counter +- Sink: print-sink", task.Metadata["description"])

	// Flink 2.x leaves the plan's operator field empty.
	assert.NotContains(t, task.Metadata, "operator")
}

func TestDiscover_TaskOfARunningJobHasNoEndTime(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob()), nil)

	task := findAsset(result, "Task", "State machine job/Source: Events Generator Source")
	require.NotNil(t, task)
	assert.Equal(t, "RUNNING", task.Metadata["status"])
	assert.NotContains(t, task.Metadata, "end_time")
}

func TestDiscover_TaskLinksToTheJobPage(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	task := findAsset(result, "Task", "WordCount/counter -> Sink: print-sink")
	require.NotNil(t, task)
	require.Len(t, task.ExternalLinks, 1)
	assert.True(t, strings.HasSuffix(task.ExternalLinks[0].URL, "/#/job/completed/3cea68fa0ce6b9f13db4e9b8aabf45ba/overview"))
}

func TestDiscover_LinksThePipelineToItsTasks(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	assert.True(t, hasEdge(result,
		pipelineMRN("WordCount"),
		taskMRN("WordCount/Source: in-memory-input -> tokenizer"),
		"CONTAINS"))
	assert.True(t, hasEdge(result,
		pipelineMRN("WordCount"),
		taskMRN("WordCount/counter -> Sink: print-sink"),
		"CONTAINS"))
}

func TestDiscover_LinksTasksAlongThePlan(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	// The plan says the counter vertex reads from the source vertex, so
	// the edge runs upstream to downstream, the way the Airflow plugin
	// links a task to the one after it.
	assert.True(t, hasEdge(result,
		taskMRN("WordCount/Source: in-memory-input -> tokenizer"),
		taskMRN("WordCount/counter -> Sink: print-sink"),
		"DEPENDS_ON"))
	assert.False(t, hasEdge(result,
		taskMRN("WordCount/counter -> Sink: print-sink"),
		taskMRN("WordCount/Source: in-memory-input -> tokenizer"),
		"DEPENDS_ON"))
}

func TestDiscover_IgnoresPlanInputsThatNameNoVertex(t *testing.T) {
	job := finishedJob()
	nodes := job.details["plan"].(map[string]interface{})["nodes"].([]interface{})
	inputs := nodes[0].(map[string]interface{})["inputs"].([]interface{})
	inputs[0].(map[string]interface{})["id"] = "does-not-exist"

	result := discover(t, newFakeJobManager().with(job), nil)

	for _, edge := range result.Lineage {
		assert.NotEqual(t, "DEPENDS_ON", edge.Type, "an input with no vertex must not become an edge to nowhere")
	}
}

func TestDiscover_SkipsTasksWhenConfigured(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), pluginsdk.RawConfig{"include_tasks": false})

	require.Len(t, result.Assets, 1)
	assert.Equal(t, "Pipeline", result.Assets[0].Type)
	assert.Empty(t, result.Lineage)
}

func TestDiscover_RecordsAFinishedJobAsACompletedRun(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), nil)

	runs := runsOf(result, pipelineMRN("WordCount"))
	require.NotEmpty(t, runs)
	assert.Equal(t, []string{"START", "RUNNING", "COMPLETE"}, eventTypes(runs))

	for _, run := range runs {
		assert.Equal(t, "3cea68fa0ce6b9f13db4e9b8aabf45ba", run.RunID)
		assert.Equal(t, "flink", run.JobNamespace)
		assert.Equal(t, "WordCount", run.JobName)
		assert.Equal(t, "FINISHED", run.RunFacets["state"])
		assert.Equal(t, int64(490), run.RunFacets["duration_ms"])
		assert.Equal(t, 1, run.RunFacets["parallelism"])
	}

	// START is the CREATED timestamp, COMPLETE the FINISHED one.
	assert.Equal(t, time.UnixMilli(1788815565893).UTC(), runs[0].EventTime)
	assert.Equal(t, time.UnixMilli(1788815566322).UTC(), runs[2].EventTime)
}

func TestDiscover_RecordsAFailedJobAsAFailedRun(t *testing.T) {
	result := discover(t, newFakeJobManager().with(failedJob()), nil)

	runs := runsOf(result, pipelineMRN("Socket Window WordCount"))
	assert.Equal(t, []string{"START", "RUNNING", "FAIL"}, eventTypes(runs))
}

func TestDiscover_RecordsACancelledJobAsAnAbortedRun(t *testing.T) {
	result := discover(t, newFakeJobManager().with(canceledJob()), nil)

	runs := runsOf(result, pipelineMRN("CarTopSpeedWindowingExample"))
	assert.Equal(t, []string{"START", "RUNNING", "ABORT"}, eventTypes(runs))
}

func TestDiscover_RecordsARunningJobAsStillRunning(t *testing.T) {
	result := discover(t, newFakeJobManager().with(runningJob()), nil)

	runs := runsOf(result, pipelineMRN("State machine job"))
	assert.Equal(t, []string{"START", "RUNNING"}, eventTypes(runs))
}

func TestDiscover_StartsTheRunAtRunningWhenCreatedIsMissing(t *testing.T) {
	job := finishedJob()
	job.details["timestamps"].(map[string]interface{})["CREATED"] = 0

	result := discover(t, newFakeJobManager().with(job), nil)

	runs := runsOf(result, pipelineMRN("WordCount"))
	require.NotEmpty(t, runs)
	assert.Equal(t, "START", runs[0].EventType)
	assert.Equal(t, time.UnixMilli(1788815565902).UTC(), runs[0].EventTime)
}

func TestDiscover_SkipsRunHistoryWhenConfigured(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), pluginsdk.RawConfig{"include_run_history": false})

	assert.NotNil(t, findAsset(result, "Pipeline", "WordCount"))
	assert.Empty(t, result.RunHistory)
}

func TestDiscover_SkipsCompletedJobsWhenConfigured(t *testing.T) {
	f := newFakeJobManager().with(runningJob(), finishedJob(), canceledJob(), failedJob())

	result := discover(t, f, pluginsdk.RawConfig{"include_completed": false})

	assert.NotNil(t, findAsset(result, "Pipeline", "State machine job"))
	assert.Nil(t, findAsset(result, "Pipeline", "WordCount"))
	assert.Nil(t, findAsset(result, "Pipeline", "CarTopSpeedWindowingExample"))
	assert.Nil(t, findAsset(result, "Pipeline", "Socket Window WordCount"))
}

func TestDiscover_NewestOfTwoJobsSharingANameKeepsItBare(t *testing.T) {
	// Resubmitting a jar gives a second job with the same name. The
	// newest run is the one people mean when they say "WordCount".
	older := finishedJob().withJID("00000000000000000000000000000001").startedAt(1788815000000)
	newer := finishedJob()

	result := discover(t, newFakeJobManager().with(older, newer), nil)

	bare := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, bare)
	assert.Equal(t, "3cea68fa0ce6b9f13db4e9b8aabf45ba", bare.Metadata["jid"])

	suffixed := findAsset(result, "Pipeline", "WordCount (00000000000000000000000000000001)")
	require.NotNil(t, suffixed)
	assert.Equal(t, "00000000000000000000000000000001", suffixed.Metadata["jid"])

	// Tasks follow their pipeline's name, so the two jobs' vertices stay apart.
	assert.NotNil(t, findAsset(result, "Task", "WordCount/counter -> Sink: print-sink"))
	assert.NotNil(t, findAsset(result, "Task", "WordCount (00000000000000000000000000000001)/counter -> Sink: print-sink"))
}

func TestDiscover_DisambiguatedPipelineKeepsTheFlinkJobNameInItsRun(t *testing.T) {
	older := finishedJob().withJID("00000000000000000000000000000001").startedAt(1788815000000)

	result := discover(t, newFakeJobManager().with(older, finishedJob()), nil)

	runs := runsOf(result, pipelineMRN("WordCount (00000000000000000000000000000001)"))
	require.NotEmpty(t, runs)
	assert.Equal(t, "WordCount", runs[0].JobName)
	assert.Equal(t, "00000000000000000000000000000001", runs[0].RunID)
}

func TestDiscover_ContinuesWhenOneJobCannotBeRead(t *testing.T) {
	f := newFakeJobManager().with(runningJob(), finishedJob()).failing("/jobs/e4ca10917efb90dc4579d7baad113fac")

	result := discover(t, f, nil)

	assert.Nil(t, findAsset(result, "Pipeline", "State machine job"))
	assert.NotNil(t, findAsset(result, "Pipeline", "WordCount"))
}

func TestDiscover_ContinuesWithoutTheJobConfig(t *testing.T) {
	f := newFakeJobManager().with(finishedJob()).failing("/jobs/3cea68fa0ce6b9f13db4e9b8aabf45ba/config")

	result := discover(t, f, nil)

	pipeline := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, pipeline)
	assert.NotContains(t, pipeline.Metadata, "parallelism")
	assert.NotContains(t, pipeline.Metadata, "restart_strategy")
}

func TestDiscover_ContinuesWithoutTheExceptions(t *testing.T) {
	f := newFakeJobManager().with(failedJob()).failing("/jobs/18c9383da628a78870ee3139d2ad756e/exceptions")

	result := discover(t, f, nil)

	pipeline := findAsset(result, "Pipeline", "Socket Window WordCount")
	require.NotNil(t, pipeline)
	assert.NotContains(t, pipeline.Metadata, "error")
}

func TestDiscover_FailsWhenTheJobManagerIsUnreachable(t *testing.T) {
	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": "http://127.0.0.1:1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster config")
}

func TestDiscover_FailsWhenTheJobListCannotBeRead(t *testing.T) {
	server := newFakeJobManager().with(finishedJob()).failing("/jobs/overview").start(t)

	_, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"host": server.URL})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing jobs")
}

func TestDiscover_SendsNoCredentialsByDefault(t *testing.T) {
	f := newFakeJobManager().with(finishedJob())

	discover(t, f, nil)

	for _, headers := range f.requestHeaders() {
		assert.Empty(t, headers.Get("Authorization"))
	}
}

func TestDiscover_SendsBasicAuthWhenConfigured(t *testing.T) {
	f := newFakeJobManager().with(finishedJob())

	discover(t, f, pluginsdk.RawConfig{"username": "marmot", "password": "secret"})

	headers := f.requestHeaders()
	require.NotEmpty(t, headers)
	for _, h := range headers {
		// "marmot:secret" base64-encoded.
		assert.Equal(t, "Basic bWFybW90OnNlY3JldA==", h.Get("Authorization"))
	}
}

func TestDiscover_SendsTheBearerTokenWhenConfigured(t *testing.T) {
	f := newFakeJobManager().with(finishedJob())

	discover(t, f, pluginsdk.RawConfig{"token": "t0k3n"})

	headers := f.requestHeaders()
	require.NotEmpty(t, headers)
	for _, h := range headers {
		assert.Equal(t, "Bearer t0k3n", h.Get("Authorization"))
	}
}

func TestDiscover_InterpolatesTagsFromMetadata(t *testing.T) {
	result := discover(t, newFakeJobManager().with(finishedJob()), pluginsdk.RawConfig{
		"tags": []interface{}{"flink", "state:${state}"},
	})

	pipeline := findAsset(result, "Pipeline", "WordCount")
	require.NotNil(t, pipeline)
	assert.Equal(t, []string{"flink", "state:FINISHED"}, pipeline.Tags)
}

func TestDiscover_TolerantOfAnEmptyCluster(t *testing.T) {
	result := discover(t, newFakeJobManager(), nil)

	assert.Empty(t, result.Assets)
	assert.Empty(t, result.Lineage)
	assert.Empty(t, result.RunHistory)
}
