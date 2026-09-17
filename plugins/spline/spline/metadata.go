package spline

// SplineFields describes the metadata fields the Spline plugin emits for
// Pipeline assets. It is kept as a documentation-only struct so downstream
// tooling can introspect the shape of the metadata map.
type SplineFields struct {
	Framework        string   `json:"framework" metadata:"framework" description:"Framework that ran the application, for example spark 3.5.0"`
	ExecutionCount   int      `json:"execution_count" metadata:"execution_count" description:"Number of runs seen in the ingest window"`
	LastExecutionAt  string   `json:"last_execution_at" metadata:"last_execution_at" description:"When the most recent run finished"`
	LastExecutionID  string   `json:"last_execution_id" metadata:"last_execution_id" description:"Spline execution event id of the most recent run"`
	LastDurationMs   int64    `json:"last_duration_ms" metadata:"last_duration_ms" description:"Duration of the most recent run in milliseconds"`
	LastError        string   `json:"last_error" metadata:"last_error" description:"Error message of the most recent run, when it failed"`
	ApplicationIDs   []string `json:"application_ids" metadata:"application_ids" description:"Recent Spark application ids, newest first"`
	ExecutionPlanIDs []string `json:"execution_plan_ids" metadata:"execution_plan_ids" description:"Recent Spline execution plan ids, newest first"`
	SystemName       string   `json:"system_name" metadata:"system_name" description:"System that produced the execution plan, for example spark"`
	SystemVersion    string   `json:"system_version" metadata:"system_version" description:"Version of that system"`
	AgentName        string   `json:"agent_name" metadata:"agent_name" description:"Spline agent that captured the run"`
	AgentVersion     string   `json:"agent_version" metadata:"agent_version" description:"Version of the Spline agent"`
	Inputs           []string `json:"inputs" metadata:"inputs" description:"Raw data source URIs the application read"`
	Outputs          []string `json:"outputs" metadata:"outputs" description:"Raw data source URIs the application wrote"`
	ColumnLineage    string   `json:"column_lineage" metadata:"column_lineage" description:"Column level lineage as JSON, keyed by the MRN of the table produced"`
	URL              string   `json:"url" metadata:"url" description:"Link to the most recent run in the Spline UI"`
}

// SplineRunFacetFields describes the facets attached to each run history event.
type SplineRunFacetFields struct {
	ApplicationID   string `json:"application_id" metadata:"application_id" description:"Spark application id of the run"`
	DurationMs      int64  `json:"duration_ms" metadata:"duration_ms" description:"Duration of the run in milliseconds"`
	ExecutionPlanID string `json:"execution_plan_id" metadata:"execution_plan_id" description:"Spline execution plan the run used"`
	Output          string `json:"output" metadata:"output" description:"Data source URI the run wrote to"`
	Append          bool   `json:"append" metadata:"append" description:"Whether the run appended to the output instead of overwriting it"`
}
