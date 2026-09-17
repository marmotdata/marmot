package prefect

// PrefectPipelineFields describes the metadata fields the Prefect plugin
// emits for a flow (Pipeline) asset. It is kept as a documentation-only
// struct so downstream tooling can introspect the shape of the metadata
// map.
type PrefectPipelineFields struct {
	FlowID          string            `json:"flow_id" metadata:"flow_id" description:"Prefect's internal identifier for the flow"`
	Tags            []string          `json:"tags" metadata:"tags" description:"Tags from the flow and its deployments"`
	Labels          map[string]string `json:"labels" metadata:"labels" description:"Labels Prefect attached to the flow"`
	DeploymentCount int               `json:"deployment_count" metadata:"deployment_count" description:"Number of deployments of this flow"`
	Deployments     []string          `json:"deployments" metadata:"deployments" description:"Deployment names, newest first"`
	Schedules       []string          `json:"schedules" metadata:"schedules" description:"Active schedules as cron expressions, intervals or recurrence rules"`
	WorkPools       []string          `json:"work_pools" metadata:"work_pools" description:"Work pools the deployments run on"`
	Entrypoint      string            `json:"entrypoint" metadata:"entrypoint" description:"File and function the newest deployment runs"`
	Paused          bool              `json:"paused" metadata:"paused" description:"Whether the newest deployment is paused"`
	LastRunState    string            `json:"last_run_state" metadata:"last_run_state" description:"State of the most recent run (COMPLETED, FAILED, RUNNING)"`
	LastRunAt       string            `json:"last_run_at" metadata:"last_run_at" description:"Start time of the most recent run"`
	RunCount        int               `json:"run_count" metadata:"run_count" description:"Number of recent runs read"`
	SuccessRate     float64           `json:"success_rate" metadata:"success_rate" description:"Percentage of the recent runs that completed"`
	Created         string            `json:"created" metadata:"created" description:"When the flow was first seen by Prefect"`
	Updated         string            `json:"updated" metadata:"updated" description:"When the flow was last updated"`
	URL             string            `json:"url" metadata:"url" description:"Address of the flow in the Prefect UI"`
}

// PrefectTaskFields describes the metadata fields for a Task asset.
type PrefectTaskFields struct {
	TaskKey   string   `json:"task_key" metadata:"task_key" description:"Prefect's task key, the task name plus a hash of its source"`
	Flow      string   `json:"flow" metadata:"flow" description:"Name of the flow the task belongs to"`
	LastState string   `json:"last_state" metadata:"last_state" description:"State of the task in the most recent run (COMPLETED, FAILED)"`
	LastRunAt string   `json:"last_run_at" metadata:"last_run_at" description:"Start time of the task in the most recent run"`
	RunCount  int      `json:"run_count" metadata:"run_count" description:"How many times the task ran during that flow run"`
	Tags      []string `json:"tags" metadata:"tags" description:"Tags on the task's runs"`
}

// PrefectRunFacetFields describes the facets attached to each run-history
// event of a Pipeline asset.
type PrefectRunFacetFields struct {
	StateName      string  `json:"state_name" metadata:"state_name" description:"Prefect's display name for the run's state"`
	Deployment     string  `json:"deployment" metadata:"deployment" description:"Deployment that started the run, when it was not started by hand"`
	RunName        string  `json:"run_name" metadata:"run_name" description:"Name Prefect generated for the run"`
	TotalRunTime   float64 `json:"total_run_time" metadata:"total_run_time" description:"Seconds the run spent running"`
	ParameterCount int     `json:"parameter_count" metadata:"parameter_count" description:"Number of parameters the run was given"`
}
