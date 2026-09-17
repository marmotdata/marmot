package gluepipeline

// PipelineFields describes the metadata a workflow Pipeline asset carries.
// It is a documentation-only struct so downstream tooling can introspect the
// shape of the metadata map.
type PipelineFields struct {
	Description          string `json:"description" metadata:"description" description:"Workflow description"`
	DefaultRunProperties string `json:"default_run_properties" metadata:"default_run_properties" description:"Run properties every run starts with, as key=value pairs"`
	CreatedOn            string `json:"created_on" metadata:"created_on" description:"When the workflow was created"`
	LastModifiedOn       string `json:"last_modified_on" metadata:"last_modified_on" description:"When the workflow was last changed"`
	MaxConcurrentRuns    int32  `json:"max_concurrent_runs" metadata:"max_concurrent_runs" description:"How many runs may overlap"`
	NodeCount            int    `json:"node_count" metadata:"node_count" description:"Number of steps in the workflow"`
	JobCount             int    `json:"job_count" metadata:"job_count" description:"Number of job steps"`
	CrawlerCount         int    `json:"crawler_count" metadata:"crawler_count" description:"Number of crawler steps"`
	TriggerCount         int    `json:"trigger_count" metadata:"trigger_count" description:"Number of trigger steps"`
	LastRunID            string `json:"last_run_id" metadata:"last_run_id" description:"Identifier of the most recent run"`
	LastRunStatus        string `json:"last_run_status" metadata:"last_run_status" description:"Status of the most recent run"`
	LastRunStarted       string `json:"last_run_started" metadata:"last_run_started" description:"When the most recent run started"`
	LastRunCompleted     string `json:"last_run_completed" metadata:"last_run_completed" description:"When the most recent run finished"`
	LastRunStatistics    string `json:"last_run_statistics" metadata:"last_run_statistics" description:"Action counters of the most recent run, as key=value pairs"`
	Region               string `json:"region" metadata:"region" description:"AWS region the workflow lives in"`
	URL                  string `json:"url" metadata:"url" description:"AWS console link to the workflow"`
}

// TaskFields describes the metadata a workflow step Task asset carries.
type TaskFields struct {
	Workflow          string `json:"workflow" metadata:"workflow" description:"Workflow the step belongs to"`
	NodeName          string `json:"node_name" metadata:"node_name" description:"Step name inside the workflow"`
	NodeType          string `json:"node_type" metadata:"node_type" description:"Step kind (job, crawler, trigger)"`
	UniqueID          string `json:"unique_id" metadata:"unique_id" description:"Identifier AWS gives the step in the run graph"`
	Job               string `json:"job" metadata:"job" description:"Glue job the step runs"`
	JobType           string `json:"job_type" metadata:"job_type" description:"Job command (glueetl, pythonshell, gluestreaming)"`
	JobScriptLocation string `json:"job_script_location" metadata:"job_script_location" description:"Location of the job script"`
	Crawler           string `json:"crawler" metadata:"crawler" description:"Glue crawler the step runs"`
	TriggerType       string `json:"trigger_type" metadata:"trigger_type" description:"Trigger kind (SCHEDULED, CONDITIONAL, ON_DEMAND, EVENT)"`
	TriggerSchedule   string `json:"trigger_schedule" metadata:"trigger_schedule" description:"Cron expression of a scheduled trigger"`
	TriggerState      string `json:"trigger_state" metadata:"trigger_state" description:"Trigger state reported by AWS"`
	TriggerPredicate  string `json:"trigger_predicate" metadata:"trigger_predicate" description:"What a conditional trigger waits for"`
	TriggerActions    string `json:"trigger_actions" metadata:"trigger_actions" description:"Jobs and crawlers the trigger starts"`
}

// RunFacetFields describes the facets attached to run history events.
type RunFacetFields struct {
	State                string  `json:"state" metadata:"state" description:"Job or crawler run state reported by AWS"`
	Status               string  `json:"status" metadata:"status" description:"Workflow run status reported by AWS"`
	Statistics           string  `json:"statistics" metadata:"statistics" description:"Action counters of a workflow run"`
	ExecutionTimeSeconds int32   `json:"execution_time_seconds" metadata:"execution_time_seconds" description:"How long a job run took"`
	Attempt              int32   `json:"attempt" metadata:"attempt" description:"Retry attempt number of a job run"`
	Trigger              string  `json:"trigger" metadata:"trigger" description:"Trigger that started the job run"`
	WorkerType           string  `json:"worker_type" metadata:"worker_type" description:"Worker size the job run used"`
	NumberOfWorkers      int32   `json:"number_of_workers" metadata:"number_of_workers" description:"Workers the job run used"`
	DPUHour              float64 `json:"dpu_hour" metadata:"dpu_hour" description:"DPU hours a crawl consumed"`
	Summary              string  `json:"summary" metadata:"summary" description:"What a crawl changed"`
	LogGroup             string  `json:"log_group" metadata:"log_group" description:"CloudWatch log group of a crawl"`
	ErrorMessage         string  `json:"error_message" metadata:"error_message" description:"Error reported by a failed run"`
}
