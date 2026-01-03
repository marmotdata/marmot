package airflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DAG represents an Airflow DAG from the REST API
type DAG struct {
	DagID            string            `json:"dag_id"`
	DagDisplayName   string            `json:"dag_display_name,omitempty"`
	Description      *string           `json:"description"`
	FileToken        string            `json:"file_token"`
	Fileloc          string            `json:"fileloc"`
	IsPaused         bool              `json:"is_paused"`
	IsActive         bool              `json:"is_active"`
	IsSubdag         bool              `json:"is_subdag"`
	LastParsedTime   *string           `json:"last_parsed_time"`
	LastPickled      *string           `json:"last_pickled"`
	LastExpiredTime  *string           `json:"last_expired"`
	SchedulerLock    *string           `json:"scheduler_lock"`
	PickleID         *string           `json:"pickle_id"`
	DefaultView      string            `json:"default_view"`
	Owners           []string          `json:"owners"`
	Tags             []Tag             `json:"tags"`
	ScheduleInterval *ScheduleInterval `json:"schedule_interval"`
	TimetableDesc    *string           `json:"timetable_description"`
	NextDagRun       *string           `json:"next_dagrun"`
	NextDagRunTime   *string           `json:"next_dagrun_data_interval_start"`
	MaxActiveRuns    int               `json:"max_active_runs"`
	MaxActiveTasks   int               `json:"max_active_tasks"`
	HasTaskConcur    bool              `json:"has_task_concurrency_limits"`
	HasImportErrors  bool              `json:"has_import_errors"`
}

// Tag represents a DAG tag
type Tag struct {
	Name string `json:"name"`
}

// ScheduleInterval represents a DAG's schedule interval
type ScheduleInterval struct {
	Type  string `json:"__type"`
	Value string `json:"value"`
}

// DAGCollection represents the API response for listing DAGs
type DAGCollection struct {
	DAGs       []DAG `json:"dags"`
	TotalCount int   `json:"total_entries"`
}

// Task represents an Airflow task from the REST API
type Task struct {
	TaskID              string                 `json:"task_id"`
	TaskDisplayName     string                 `json:"task_display_name,omitempty"`
	OperatorName        string                 `json:"operator_name"`
	ClassName           string                 `json:"class_ref,omitempty"`
	Pool                string                 `json:"pool"`
	PoolSlots           int                    `json:"pool_slots"`
	ExecutionTimeout    *string                `json:"execution_timeout"`
	TriggerRule         string                 `json:"trigger_rule"`
	Retries             int                    `json:"retries"`
	RetryDelay          *RetryDelay            `json:"retry_delay"`
	RetryExponentialBac bool                   `json:"retry_exponential_backoff"`
	PriorityWeight      int                    `json:"priority_weight"`
	Weight              string                 `json:"weight_rule"`
	Queue               string                 `json:"queue"`
	DownstreamTaskIDs   []string               `json:"downstream_task_ids"`
	UpstreamTaskIDs     []string               `json:"upstream_task_ids"`
	DependsOnPast       bool                   `json:"depends_on_past"`
	WaitForDownstream   bool                   `json:"wait_for_downstream"`
	StartDate           *string                `json:"start_date"`
	EndDate             *string                `json:"end_date"`
	UIColor             string                 `json:"ui_color"`
	UIFgcolor           string                 `json:"ui_fgcolor"`
	TemplateFields      []string               `json:"template_fields"`
	ExtraLinks          []ExtraLink            `json:"extra_links"`
	SubDag              *SubDAG                `json:"sub_dag,omitempty"`
	Params              map[string]interface{} `json:"params,omitempty"`
}

// RetryDelay represents a task's retry delay configuration
type RetryDelay struct {
	Type    string `json:"__type"`
	Days    int    `json:"days"`
	Seconds int    `json:"seconds"`
	Micros  int    `json:"microseconds"`
}

// ExtraLink represents an extra link on a task
type ExtraLink struct {
	ClassRef string `json:"class_ref"`
}

// SubDAG represents a sub-DAG reference
type SubDAG struct {
	DagID string `json:"dag_id"`
}

// TaskCollection represents the API response for listing tasks
type TaskCollection struct {
	Tasks      []Task `json:"tasks"`
	TotalCount int    `json:"total_entries"`
}

// DAGRun represents an Airflow DAG run from the REST API
type DAGRun struct {
	DagRunID          string                 `json:"dag_run_id"`
	DagID             string                 `json:"dag_id"`
	LogicalDate       string                 `json:"logical_date"`
	ExecutionDate     string                 `json:"execution_date"`
	StartDate         *string                `json:"start_date"`
	EndDate           *string                `json:"end_date"`
	DataIntervalStart *string                `json:"data_interval_start"`
	DataIntervalEnd   *string                `json:"data_interval_end"`
	LastSchedulingDec *string                `json:"last_scheduling_decision"`
	RunType           string                 `json:"run_type"`
	State             string                 `json:"state"`
	ExternalTrigger   bool                   `json:"external_trigger"`
	Conf              map[string]interface{} `json:"conf"`
	Note              *string                `json:"note"`
}

// DAGRunCollection represents the API response for listing DAG runs
type DAGRunCollection struct {
	DagRuns    []DAGRun `json:"dag_runs"`
	TotalCount int      `json:"total_entries"`
}

// Dataset represents an Airflow Dataset from the REST API (Airflow 2.4+)
type Dataset struct {
	ID             int                    `json:"id"`
	URI            string                 `json:"uri"`
	Extra          map[string]interface{} `json:"extra"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	ConsumingDags  []DagRef               `json:"consuming_dags"`
	ProducingTasks []TaskRef              `json:"producing_tasks"`
}

// DagRef represents a reference to a DAG
type DagRef struct {
	DagID string `json:"dag_id"`
}

// TaskRef represents a reference to a task
type TaskRef struct {
	DagID  string `json:"dag_id"`
	TaskID string `json:"task_id"`
}

// DatasetCollection represents the API response for listing datasets
type DatasetCollection struct {
	Datasets   []Dataset `json:"datasets"`
	TotalCount int       `json:"total_entries"`
}

// DatasetEvent represents a dataset event from the REST API
type DatasetEvent struct {
	ID             int                    `json:"id"`
	DatasetID      int                    `json:"dataset_id"`
	DatasetURI     string                 `json:"dataset_uri"`
	SourceDagID    *string                `json:"source_dag_id"`
	SourceTaskID   *string                `json:"source_task_id"`
	SourceRunID    *string                `json:"source_run_id"`
	SourceMapIndex int                    `json:"source_map_index"`
	CreatedDagruns []DagRef               `json:"created_dagruns"`
	Timestamp      string                 `json:"timestamp"`
	Extra          map[string]interface{} `json:"extra"`
}

// DatasetEventCollection represents the API response for listing dataset events
type DatasetEventCollection struct {
	DatasetEvents []DatasetEvent `json:"dataset_events"`
	TotalCount    int            `json:"total_entries"`
}

// APIError represents an error response from the Airflow API
type APIError struct {
	Detail string `json:"detail"`
	Status int    `json:"status"`
	Title  string `json:"title"`
	Type   string `json:"type"`
}

// ClientConfig holds configuration for the Airflow API client
type ClientConfig struct {
	BaseURL  string
	Username string
	Password string
	APIToken string
	Timeout  time.Duration
}

// Client is an Airflow REST API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
	apiToken   string
}

// NewClient creates a new Airflow API client
func NewClient(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		username: config.Username,
		password: config.Password,
		apiToken: config.APIToken,
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values) ([]byte, error) {
	reqURL := fmt.Sprintf("%s%s", c.baseURL, path)
	if len(query) > 0 {
		reqURL = fmt.Sprintf("%s?%s", reqURL, query.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set authentication
	if c.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Detail != "" {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Detail)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ListDAGs returns all DAGs from Airflow
func (c *Client) ListDAGs(ctx context.Context, onlyActive bool) ([]DAG, error) {
	var allDAGs []DAG
	limit := 100
	offset := 0

	for {
		query := url.Values{}
		query.Set("limit", fmt.Sprintf("%d", limit))
		query.Set("offset", fmt.Sprintf("%d", offset))

		if onlyActive {
			query.Set("only_active", "true")
		}

		body, err := c.doRequest(ctx, http.MethodGet, "/api/v1/dags", query)
		if err != nil {
			return nil, err
		}

		var collection DAGCollection
		if err := json.Unmarshal(body, &collection); err != nil {
			return nil, fmt.Errorf("parsing DAGs response: %w", err)
		}

		allDAGs = append(allDAGs, collection.DAGs...)

		// Check if we have more pages
		if len(collection.DAGs) < limit {
			break
		}
		offset += limit
	}

	return allDAGs, nil
}

// GetDAG returns a specific DAG by ID
func (c *Client) GetDAG(ctx context.Context, dagID string) (*DAG, error) {
	path := fmt.Sprintf("/api/v1/dags/%s", url.PathEscape(dagID))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var dag DAG
	if err := json.Unmarshal(body, &dag); err != nil {
		return nil, fmt.Errorf("parsing DAG response: %w", err)
	}

	return &dag, nil
}

// ListTasks returns all tasks for a specific DAG
func (c *Client) ListTasks(ctx context.Context, dagID string) ([]Task, error) {
	path := fmt.Sprintf("/api/v1/dags/%s/tasks", url.PathEscape(dagID))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var collection TaskCollection
	if err := json.Unmarshal(body, &collection); err != nil {
		return nil, fmt.Errorf("parsing tasks response: %w", err)
	}

	return collection.Tasks, nil
}

// ListDAGRuns returns DAG runs for a specific DAG within the specified number of days
func (c *Client) ListDAGRuns(ctx context.Context, dagID string, days int) ([]DAGRun, error) {
	path := fmt.Sprintf("/api/v1/dags/%s/dagRuns", url.PathEscape(dagID))

	query := url.Values{}
	query.Set("limit", "100")
	query.Set("order_by", "-execution_date")

	// Filter by execution date if days is specified
	if days > 0 {
		startDate := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
		query.Set("execution_date_gte", startDate)
	}

	body, err := c.doRequest(ctx, http.MethodGet, path, query)
	if err != nil {
		return nil, err
	}

	var collection DAGRunCollection
	if err := json.Unmarshal(body, &collection); err != nil {
		return nil, fmt.Errorf("parsing DAG runs response: %w", err)
	}

	return collection.DagRuns, nil
}

// ListDatasets returns all datasets from Airflow (requires Airflow 2.4+)
func (c *Client) ListDatasets(ctx context.Context) ([]Dataset, error) {
	var allDatasets []Dataset
	limit := 100
	offset := 0

	for {
		query := url.Values{}
		query.Set("limit", fmt.Sprintf("%d", limit))
		query.Set("offset", fmt.Sprintf("%d", offset))

		body, err := c.doRequest(ctx, http.MethodGet, "/api/v1/datasets", query)
		if err != nil {
			return nil, err
		}

		var collection DatasetCollection
		if err := json.Unmarshal(body, &collection); err != nil {
			return nil, fmt.Errorf("parsing datasets response: %w", err)
		}

		allDatasets = append(allDatasets, collection.Datasets...)

		// Check if we have more pages
		if len(collection.Datasets) < limit {
			break
		}
		offset += limit
	}

	return allDatasets, nil
}

// GetDatasetEvents returns events for a specific dataset
func (c *Client) GetDatasetEvents(ctx context.Context, datasetURI string) ([]DatasetEvent, error) {
	query := url.Values{}
	query.Set("dataset_uri", datasetURI)
	query.Set("limit", "100")
	query.Set("order_by", "-timestamp")

	body, err := c.doRequest(ctx, http.MethodGet, "/api/v1/datasetEvents", query)
	if err != nil {
		return nil, err
	}

	var collection DatasetEventCollection
	if err := json.Unmarshal(body, &collection); err != nil {
		return nil, fmt.Errorf("parsing dataset events response: %w", err)
	}

	return collection.DatasetEvents, nil
}
