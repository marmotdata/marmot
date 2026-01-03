package airflow

// AirflowDAGFields represents Airflow DAG-specific metadata fields
// +marmot:metadata
type AirflowDAGFields struct {
	DagID            string  `json:"dag_id" metadata:"dag_id" description:"Unique DAG identifier"`
	Description      string  `json:"description" metadata:"description" description:"DAG description"`
	FilePath         string  `json:"file_path" metadata:"file_path" description:"Path to DAG definition file"`
	ScheduleInterval string  `json:"schedule_interval" metadata:"schedule_interval" description:"DAG schedule (cron expression or preset)"`
	IsPaused         bool    `json:"is_paused" metadata:"is_paused" description:"Whether DAG is paused"`
	IsActive         bool    `json:"is_active" metadata:"is_active" description:"Whether DAG is active"`
	Owners           string  `json:"owners" metadata:"owners" description:"DAG owners (comma-separated)"`
	LastRunState     string  `json:"last_run_state" metadata:"last_run_state" description:"State of the last DAG run (success, failed, running)"`
	LastRunID        string  `json:"last_run_id" metadata:"last_run_id" description:"ID of the last DAG run"`
	LastRunDate      string  `json:"last_run_date" metadata:"last_run_date" description:"Execution date of the last DAG run"`
	NextRunDate      string  `json:"next_run_date" metadata:"next_run_date" description:"Next scheduled run date"`
	LastParsedTime   string  `json:"last_parsed_time" metadata:"last_parsed_time" description:"Last time the DAG file was parsed"`
	SuccessRate      float64 `json:"success_rate" metadata:"success_rate" description:"Success rate percentage over the lookback period"`
	RunCount         int     `json:"run_count" metadata:"run_count" description:"Number of runs in the lookback period"`
}

// AirflowTaskFields represents Airflow task-specific metadata fields
// +marmot:metadata
type AirflowTaskFields struct {
	TaskID          string   `json:"task_id" metadata:"task_id" description:"Task identifier within the DAG"`
	DagID           string   `json:"dag_id" metadata:"dag_id" description:"Parent DAG ID"`
	OperatorName    string   `json:"operator_name" metadata:"operator_name" description:"Airflow operator class name (e.g., BashOperator, PythonOperator)"`
	TriggerRule     string   `json:"trigger_rule" metadata:"trigger_rule" description:"Task trigger rule (e.g., all_success, one_success)"`
	Retries         int      `json:"retries" metadata:"retries" description:"Number of retries configured for the task"`
	Pool            string   `json:"pool" metadata:"pool" description:"Execution pool for the task"`
	DownstreamTasks []string `json:"downstream_tasks" metadata:"downstream_tasks" description:"List of downstream task IDs"`
}

// AirflowDatasetFields represents Airflow Dataset-specific metadata fields
// +marmot:metadata
type AirflowDatasetFields struct {
	URI           string `json:"uri" metadata:"uri" description:"Dataset URI identifier"`
	CreatedAt     string `json:"created_at" metadata:"created_at" description:"Dataset creation timestamp"`
	UpdatedAt     string `json:"updated_at" metadata:"updated_at" description:"Dataset last update timestamp"`
	ProducerCount int    `json:"producer_count" metadata:"producer_count" description:"Number of tasks that produce this dataset"`
	ConsumerCount int    `json:"consumer_count" metadata:"consumer_count" description:"Number of DAGs that consume this dataset"`
}

// AirflowDAGRunFields represents Airflow DAG run-specific metadata fields
// +marmot:metadata
type AirflowDAGRunFields struct {
	DagRunID      string `json:"dag_run_id" metadata:"dag_run_id" description:"Unique identifier for the DAG run"`
	State         string `json:"state" metadata:"state" description:"Run state (queued, running, success, failed)"`
	ExecutionDate string `json:"execution_date" metadata:"execution_date" description:"Logical execution date"`
	StartDate     string `json:"start_date" metadata:"start_date" description:"Actual start time of the run"`
	EndDate       string `json:"end_date" metadata:"end_date" description:"End time of the run"`
	RunType       string `json:"run_type" metadata:"run_type" description:"Type of run (scheduled, manual, backfill)"`
}
