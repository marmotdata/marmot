package dagster

// DagsterPipelineFields describes the metadata fields the Dagster plugin
// emits for Pipeline assets, one per job. It is a documentation-only struct
// so downstream tooling can introspect the shape of the metadata map.
type DagsterPipelineFields struct {
	JobID         string  `json:"job_id" metadata:"job_id" description:"Dagster's internal identifier for the job"`
	Repository    string  `json:"repository" metadata:"repository" description:"Repository the job is defined in"`
	CodeLocation  string  `json:"code_location" metadata:"code_location" description:"Code location serving the job"`
	Description   string  `json:"description" metadata:"description" description:"Job description"`
	IsAssetJob    bool    `json:"is_asset_job" metadata:"is_asset_job" description:"Whether the job exists to materialise assets"`
	Tags          string  `json:"tags" metadata:"tags" description:"Job tags, keyed by tag name"`
	Schedules     string  `json:"schedules" metadata:"schedules" description:"Schedules that launch the job, with cron expression and status"`
	Sensors       string  `json:"sensors" metadata:"sensors" description:"Sensors that launch the job, with status"`
	OpCount       int     `json:"op_count" metadata:"op_count" description:"Number of ops in the job"`
	LastRunStatus string  `json:"last_run_status" metadata:"last_run_status" description:"Status of the most recent run"`
	LastRunAt     string  `json:"last_run_at" metadata:"last_run_at" description:"Start time of the most recent run"`
	RunCount      int     `json:"run_count" metadata:"run_count" description:"Number of runs read for this job"`
	SuccessRate   float64 `json:"success_rate" metadata:"success_rate" description:"Percentage of finished runs that succeeded"`
	URL           string  `json:"url" metadata:"url" description:"Link to the job in the Dagster UI"`
}

// DagsterTaskFields describes the metadata fields emitted for Task assets,
// one per op in a job.
type DagsterTaskFields struct {
	Op          string `json:"op" metadata:"op" description:"Name of the op definition"`
	HandleID    string `json:"handle_id" metadata:"handle_id" description:"Path to the op within the job graph"`
	Job         string `json:"job" metadata:"job" description:"Job the op belongs to"`
	Description string `json:"description" metadata:"description" description:"Op description"`
	InputCount  int    `json:"input_count" metadata:"input_count" description:"Number of inputs the op declares"`
	OutputCount int    `json:"output_count" metadata:"output_count" description:"Number of outputs the op declares"`
	URL         string `json:"url" metadata:"url" description:"Link to the op in the Dagster UI"`
}

// DagsterDatasetFields describes the metadata fields emitted for Dataset
// assets, one per software-defined asset. Any metadata a user attaches to the
// asset in Dagster is flattened alongside these, keyed by its label.
type DagsterDatasetFields struct {
	AssetKey               string `json:"asset_key" metadata:"asset_key" description:"Asset key path as declared in Dagster"`
	Description            string `json:"description" metadata:"description" description:"Asset description"`
	ComputeKind            string `json:"compute_kind" metadata:"compute_kind" description:"Technology that computes the asset, for example duckdb"`
	Group                  string `json:"group" metadata:"group" description:"Asset group the asset belongs to"`
	CodeLocation           string `json:"code_location" metadata:"code_location" description:"Code location serving the asset"`
	Repository             string `json:"repository" metadata:"repository" description:"Repository the asset is defined in"`
	OpNames                string `json:"op_names" metadata:"op_names" description:"Ops that compute the asset"`
	JobNames               string `json:"job_names" metadata:"job_names" description:"Jobs that can materialise the asset"`
	IsPartitioned          bool   `json:"is_partitioned" metadata:"is_partitioned" description:"Whether the asset is partitioned"`
	LastMaterializedAt     string `json:"last_materialized_at" metadata:"last_materialized_at" description:"Time of the most recent materialization"`
	LastMaterializationRun string `json:"last_materialization_run" metadata:"last_materialization_run" description:"Run that produced the most recent materialization"`
	URL                    string `json:"url" metadata:"url" description:"Link to the asset in the Dagster UI"`
}

// DagsterColumnFields describes the per-column fields taken from an asset's
// TableSchema metadata entry and embedded in the asset's schema.
type DagsterColumnFields struct {
	ColumnName  string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType    string `json:"data_type" metadata:"data_type" description:"Column data type as declared in the table schema"`
	IsNullable  bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	Description string `json:"description" metadata:"description" description:"Column description"`
}
