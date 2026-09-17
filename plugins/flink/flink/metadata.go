package flink

// FlinkPipelineFields describes the metadata fields the Flink plugin emits
// for Pipeline assets (one per job). It is kept as a documentation-only
// struct so downstream tooling can introspect the shape of the metadata
// map.
type FlinkPipelineFields struct {
	JID             string         `json:"jid" metadata:"jid" description:"Flink job id"`
	State           string         `json:"state" metadata:"state" description:"Job state (RUNNING, FINISHED, FAILED, CANCELED, ...)"`
	StartTime       string         `json:"start_time" metadata:"start_time" description:"When the job was submitted (RFC3339)"`
	EndTime         string         `json:"end_time" metadata:"end_time" description:"When the job reached its final state (RFC3339), absent while it runs"`
	DurationMs      int64          `json:"duration_ms" metadata:"duration_ms" description:"Job duration in milliseconds"`
	IsStoppable     bool           `json:"is_stoppable" metadata:"is_stoppable" description:"Whether the job can be stopped with a savepoint"`
	MaxParallelism  int            `json:"max_parallelism" metadata:"max_parallelism" description:"Configured maximum parallelism, absent when unset"`
	Parallelism     int            `json:"parallelism" metadata:"parallelism" description:"Job parallelism from the execution config"`
	ExecutionMode   string         `json:"execution_mode" metadata:"execution_mode" description:"Execution mode (Flink 1.x only)"`
	RestartStrategy string         `json:"restart_strategy" metadata:"restart_strategy" description:"Restart strategy description"`
	TaskCounts      map[string]int `json:"task_counts" metadata:"task_counts" description:"Number of tasks per state (running, finished, failed, ...)"`
	VertexCount     int            `json:"vertex_count" metadata:"vertex_count" description:"Number of vertices in the job graph"`
	FlinkVersion    string         `json:"flink_version" metadata:"flink_version" description:"Version of the Flink cluster"`
	URL             string         `json:"url" metadata:"url" description:"Job page in the Flink web UI"`
	Error           string         `json:"error" metadata:"error" description:"Root cause of a failed job, trimmed to 500 characters"`
}

// FlinkTaskFields describes the metadata fields the Flink plugin emits for
// Task assets (one per job vertex).
type FlinkTaskFields struct {
	JID            string `json:"jid" metadata:"jid" description:"Flink job id"`
	Pipeline       string `json:"pipeline" metadata:"pipeline" description:"Name of the Pipeline the vertex belongs to"`
	VertexID       string `json:"vertex_id" metadata:"vertex_id" description:"Vertex id within the job graph"`
	Status         string `json:"status" metadata:"status" description:"Vertex status (RUNNING, FINISHED, FAILED, CANCELED, ...)"`
	Parallelism    int    `json:"parallelism" metadata:"parallelism" description:"Vertex parallelism"`
	MaxParallelism int    `json:"max_parallelism" metadata:"max_parallelism" description:"Vertex maximum parallelism"`
	StartTime      string `json:"start_time" metadata:"start_time" description:"When the vertex started (RFC3339)"`
	EndTime        string `json:"end_time" metadata:"end_time" description:"When the vertex finished (RFC3339), absent while it runs"`
	DurationMs     int64  `json:"duration_ms" metadata:"duration_ms" description:"Vertex duration in milliseconds"`
	Operator       string `json:"operator" metadata:"operator" description:"Operator name from the job plan, when Flink reports one"`
	Description    string `json:"description" metadata:"description" description:"Operator chain from the job plan"`
	ReadRecords    int64  `json:"read_records" metadata:"read_records" description:"Records read by the vertex"`
	WriteRecords   int64  `json:"write_records" metadata:"write_records" description:"Records written by the vertex"`
	ReadBytes      int64  `json:"read_bytes" metadata:"read_bytes" description:"Bytes read by the vertex"`
	WriteBytes     int64  `json:"write_bytes" metadata:"write_bytes" description:"Bytes written by the vertex"`
}
