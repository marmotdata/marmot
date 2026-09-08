package prefect

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Prefect rejects a page larger than 200, so that is the page size for
// every list call.
const pageSize = 200

// maxPages stops a paging loop that never sees a short page, so a server
// that ignores the offset cannot hang discovery.
const maxPages = 100

// errNotFound reports a 404. Only the Assets API uses it: that API is
// Prefect Cloud only and a self-hosted server answers 404 for it.
var errNotFound = errors.New("not found")

// Flow is a Prefect flow, the definition a Pipeline asset is built from.
type Flow struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Tags    []string       `json:"tags"`
	Labels  map[string]any `json:"labels"`
	Created string         `json:"created"`
	Updated string         `json:"updated"`
}

// ScheduleDetails holds one of cron, interval or rrule. Prefect fills in
// exactly one of them per schedule.
type ScheduleDetails struct {
	Cron     string  `json:"cron"`
	Interval float64 `json:"interval"`
	RRule    string  `json:"rrule"`
	Timezone string  `json:"timezone"`
}

// DeploymentSchedule is one schedule attached to a deployment.
type DeploymentSchedule struct {
	Active   bool             `json:"active"`
	Schedule *ScheduleDetails `json:"schedule"`
}

// Deployment is a configured way of running a flow: a schedule, a work
// pool and the tags and description a person wrote.
type Deployment struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	FlowID        string               `json:"flow_id"`
	Description   string               `json:"description"`
	Tags          []string             `json:"tags"`
	Paused        bool                 `json:"paused"`
	Status        string               `json:"status"`
	Schedules     []DeploymentSchedule `json:"schedules"`
	WorkPoolName  string               `json:"work_pool_name"`
	WorkQueueName string               `json:"work_queue_name"`
	Entrypoint    string               `json:"entrypoint"`
	Path          string               `json:"path"`
	Version       string               `json:"version"`
	Created       string               `json:"created"`
	Updated       string               `json:"updated"`
}

// RunState is the nested state object on a run. It repeats state_type and
// carries the timestamp of the transition into that state.
type RunState struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
}

// FlowRun is one execution of a flow.
type FlowRun struct {
	ID                string         `json:"id"`
	Name              string         `json:"name"`
	FlowID            string         `json:"flow_id"`
	DeploymentID      string         `json:"deployment_id"`
	StateType         string         `json:"state_type"`
	StateName         string         `json:"state_name"`
	State             *RunState      `json:"state"`
	StartTime         string         `json:"start_time"`
	ExpectedStartTime string         `json:"expected_start_time"`
	EndTime           string         `json:"end_time"`
	TotalRunTime      float64        `json:"total_run_time"`
	RunCount          int            `json:"run_count"`
	Tags              []string       `json:"tags"`
	Parameters        map[string]any `json:"parameters"`
	Created           string         `json:"created"`
}

// TaskInputRef is one entry in a task run's inputs. Only entries with
// input_type "task_run" name another task run; a constant or a flow
// parameter has no id.
type TaskInputRef struct {
	InputType string `json:"input_type"`
	ID        string `json:"id"`
}

// TaskRun is one execution of a task inside a flow run.
type TaskRun struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	FlowRunID         string                    `json:"flow_run_id"`
	TaskKey           string                    `json:"task_key"`
	Tags              []string                  `json:"tags"`
	TaskInputs        map[string][]TaskInputRef `json:"task_inputs"`
	StateType         string                    `json:"state_type"`
	StateName         string                    `json:"state_name"`
	StartTime         string                    `json:"start_time"`
	ExpectedStartTime string                    `json:"expected_start_time"`
	EndTime           string                    `json:"end_time"`
	TotalRunTime      float64                   `json:"total_run_time"`
	RunCount          int                       `json:"run_count"`
}

// AssetMaterialization is one asset a flow run wrote, with the assets it
// read to produce it. Prefect Cloud only.
type AssetMaterialization struct {
	AssetKey       string   `json:"asset_key"`
	UpstreamAssets []string `json:"upstream_assets"`
}

// apiError is the body Prefect returns for a rejected request. A
// validation failure fills exception_message, an unrouted path fills
// detail.
type apiError struct {
	ExceptionMessage string `json:"exception_message"`
	Detail           string `json:"detail"`
}

// ClientConfig configures a Client.
type ClientConfig struct {
	BaseURL    string
	APIKey     string
	AuthString string
	VerifySSL  bool
	Timeout    time.Duration
}

// Client is a Prefect REST API client. Every list endpoint is a POST to
// <resource>/filter with a JSON body, so one helper covers them all.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	authString string
}

// NewClient builds a client for one Prefect API base URL.
func NewClient(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport
	if !config.VerifySSL {
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	return &Client{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		apiKey:     config.APIKey,
		authString: config.AuthString,
	}
}

// do sends one request and decodes the JSON response into out. Pass a nil
// body for a GET, and a nil out to discard the response.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	switch {
	case c.apiKey != "":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	case c.authString != "":
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.authString)))
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", path, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", path, errNotFound)
	}

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if err := json.Unmarshal(payload, &apiErr); err == nil {
			if message := apiErr.ExceptionMessage; message != "" {
				return fmt.Errorf("prefect API %s returned %d: %s", path, resp.StatusCode, message)
			}
			if apiErr.Detail != "" {
				return fmt.Errorf("prefect API %s returned %d: %s", path, resp.StatusCode, apiErr.Detail)
			}
		}
		return fmt.Errorf("prefect API %s returned %d", path, resp.StatusCode)
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("parsing %s response: %w", path, err)
	}

	return nil
}

// filterAll pages through a POST <resource>/filter endpoint, merging the
// caller's filter into each page's request body.
func filterAll[T any](ctx context.Context, c *Client, resource string, filter map[string]any) ([]T, error) {
	var all []T

	for page := 0; page < maxPages; page++ {
		body := map[string]any{"limit": pageSize, "offset": page * pageSize}
		for key, value := range filter {
			body[key] = value
		}

		var batch []T
		if err := c.do(ctx, http.MethodPost, "/"+resource+"/filter", body, &batch); err != nil {
			return nil, err
		}

		all = append(all, batch...)

		if len(batch) < pageSize {
			return all, nil
		}
	}

	log.Warn().Str("resource", resource).Int("page_cap", maxPages).
		Msg("Stopped paging at the page cap, some records were not read")

	return all, nil
}

// Ping checks the API is reachable and the credentials work. The flows
// filter is used rather than /health because /health needs no auth, so it
// would pass with a wrong API key.
func (c *Client) Ping(ctx context.Context) error {
	var flows []Flow
	return c.do(ctx, http.MethodPost, "/flows/filter", map[string]any{"limit": 1, "offset": 0}, &flows)
}

// ListFlows returns every flow in the workspace, sorted by name.
func (c *Client) ListFlows(ctx context.Context) ([]Flow, error) {
	return filterAll[Flow](ctx, c, "flows", map[string]any{"sort": "NAME_ASC"})
}

// ListDeployments returns every deployment of one flow, newest first.
func (c *Client) ListDeployments(ctx context.Context, flowID string) ([]Deployment, error) {
	return filterAll[Deployment](ctx, c, "deployments", map[string]any{
		"flows": map[string]any{"id": map[string]any{"any_": []string{flowID}}},
		"sort":  "CREATED_DESC",
	})
}

// ListFlowRuns returns the most recent runs of one flow, newest first.
//
// Runs still in the SCHEDULED state are excluded: a deployment with an
// active schedule pre-creates those with start times in the future, and
// under START_TIME_DESC they would push every real past run out of the
// page.
func (c *Client) ListFlowRuns(ctx context.Context, flowID string, limit int) ([]FlowRun, error) {
	var runs []FlowRun
	err := c.do(ctx, http.MethodPost, "/flow_runs/filter", map[string]any{
		"flows":     map[string]any{"id": map[string]any{"any_": []string{flowID}}},
		"flow_runs": map[string]any{"state": map[string]any{"type": map[string]any{"not_any_": []string{"SCHEDULED"}}}},
		"sort":      "START_TIME_DESC",
		"limit":     limit,
		"offset":    0,
	}, &runs)
	if err != nil {
		return nil, err
	}
	return runs, nil
}

// ListTaskRuns returns every task run of one flow run.
func (c *Client) ListTaskRuns(ctx context.Context, flowRunID string) ([]TaskRun, error) {
	return filterAll[TaskRun](ctx, c, "task_runs", map[string]any{
		"task_runs": map[string]any{"flow_run_id": map[string]any{"any_": []string{flowRunID}}},
	})
}

// ListAssetMaterializations returns the assets one flow run wrote. The
// Assets API exists on Prefect Cloud only, so a self-hosted server
// answers 404 and this returns errNotFound.
func (c *Client) ListAssetMaterializations(ctx context.Context, flowRunID string) ([]AssetMaterialization, error) {
	var materializations []AssetMaterialization
	err := c.do(ctx, http.MethodGet, "/flow_runs/"+flowRunID+"/assets/materializations", nil, &materializations)
	if err != nil {
		return nil, err
	}
	return materializations, nil
}
