package flink

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// client talks to the Flink JobManager REST API. The API itself has no
// authentication; the optional basic auth and bearer token are for a
// proxy placed in front of the JobManager.
type client struct {
	baseURL  string
	username string
	password string
	token    string
	http     *http.Client
}

func newClient(host, username, password, token string, verifySSL bool) *client {
	transport := http.DefaultTransport
	if !verifySSL {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // User-configured option
		transport = insecure
	}

	return &client{
		baseURL:  strings.TrimSuffix(host, "/"),
		username: username,
		password: password,
		token:    token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// get performs a GET against a path below the JobManager root and
// decodes the JSON response into out.
func (c *client) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")

	switch {
	case c.token != "":
		req.Header.Set("Authorization", "Bearer "+c.token)
	case c.username != "":
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s: %s", path, http.StatusText(resp.StatusCode), strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// clusterConfig is GET /config.
type clusterConfig struct {
	FlinkVersion  string `json:"flink-version"`
	FlinkRevision string `json:"flink-revision"`
}

func (c *client) clusterConfig(ctx context.Context) (*clusterConfig, error) {
	var out clusterConfig
	if err := c.get(ctx, "/config", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// jobsOverview is GET /jobs/overview. The JobManager keeps listing
// finished, failed and cancelled jobs here until it restarts.
type jobsOverview struct {
	Jobs []jobSummary `json:"jobs"`
}

type jobSummary struct {
	JID       string         `json:"jid"`
	Name      string         `json:"name"`
	State     string         `json:"state"`
	StartTime int64          `json:"start-time"`
	EndTime   int64          `json:"end-time"`
	Duration  int64          `json:"duration"`
	Tasks     map[string]int `json:"tasks"`
}

func (c *client) jobs(ctx context.Context) ([]jobSummary, error) {
	var out jobsOverview
	if err := c.get(ctx, "/jobs/overview", &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

// jobDetails is GET /jobs/{jid}. Times are epoch milliseconds; an
// end-time of -1 and a timestamp of 0 both mean "has not happened".
type jobDetails struct {
	JID            string           `json:"jid"`
	Name           string           `json:"name"`
	IsStoppable    bool             `json:"isStoppable"`
	State          string           `json:"state"`
	StartTime      int64            `json:"start-time"`
	EndTime        int64            `json:"end-time"`
	Duration       int64            `json:"duration"`
	MaxParallelism int              `json:"maxParallelism"`
	Timestamps     map[string]int64 `json:"timestamps"`
	Vertices       []jobVertex      `json:"vertices"`
	Plan           jobPlan          `json:"plan"`
}

type jobVertex struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	MaxParallelism int           `json:"maxParallelism"`
	Parallelism    int           `json:"parallelism"`
	Status         string        `json:"status"`
	StartTime      int64         `json:"start-time"`
	EndTime        int64         `json:"end-time"`
	Duration       int64         `json:"duration"`
	Metrics        vertexMetrics `json:"metrics"`
}

type vertexMetrics struct {
	ReadBytes    int64 `json:"read-bytes"`
	WriteBytes   int64 `json:"write-bytes"`
	ReadRecords  int64 `json:"read-records"`
	WriteRecords int64 `json:"write-records"`
}

// jobPlan is the operator graph. A node's id is the id of the vertex
// that runs it, and its inputs name the vertices it reads from.
type jobPlan struct {
	Nodes []planNode `json:"nodes"`
}

type planNode struct {
	ID          string      `json:"id"`
	Operator    string      `json:"operator"`
	Description string      `json:"description"`
	Inputs      []planInput `json:"inputs"`
}

type planInput struct {
	ID           string `json:"id"`
	ShipStrategy string `json:"ship_strategy"`
	Exchange     string `json:"exchange"`
}

func (c *client) job(ctx context.Context, jid string) (*jobDetails, error) {
	var out jobDetails
	if err := c.get(ctx, "/jobs/"+url.PathEscape(jid), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// jobConfig is GET /jobs/{jid}/config. execution-mode was dropped in
// Flink 2.0, so it is only set for older clusters.
type jobConfig struct {
	ExecutionConfig executionConfig `json:"execution-config"`
}

type executionConfig struct {
	ExecutionMode   string `json:"execution-mode"`
	RestartStrategy string `json:"restart-strategy"`
	JobParallelism  int    `json:"job-parallelism"`
}

func (c *client) config(ctx context.Context, jid string) (*jobConfig, error) {
	var out jobConfig
	if err := c.get(ctx, "/jobs/"+url.PathEscape(jid)+"/config", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// jobExceptions is GET /jobs/{jid}/exceptions. Flink 1.x reports the
// failure in root-exception; Flink 2.x dropped that field and only
// keeps the exception history, newest first.
type jobExceptions struct {
	RootException string           `json:"root-exception"`
	History       exceptionHistory `json:"exceptionHistory"`
}

type exceptionHistory struct {
	Entries []exceptionEntry `json:"entries"`
}

type exceptionEntry struct {
	ExceptionName string `json:"exceptionName"`
	Stacktrace    string `json:"stacktrace"`
	Timestamp     int64  `json:"timestamp"`
}

func (c *client) exceptions(ctx context.Context, jid string) (*jobExceptions, error) {
	var out jobExceptions
	if err := c.get(ctx, "/jobs/"+url.PathEscape(jid)+"/exceptions", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rootCause is the text describing why the job failed, or "" when the
// response carries none.
func (e *jobExceptions) rootCause() string {
	if e.RootException != "" {
		return e.RootException
	}
	if len(e.History.Entries) > 0 {
		return e.History.Entries[0].Stacktrace
	}
	return ""
}
