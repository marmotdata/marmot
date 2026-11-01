package mlflow

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// apiPrefix is the REST root shared by the tracking server and the model
// registry. The registry lives on the same server, so one client covers both.
const apiPrefix = "/api/2.0/mlflow"

// pageSize is the largest page MLflow accepts on its search endpoints.
const pageSize = 100

// maxPages bounds every paged listing so a server that keeps handing out
// page tokens cannot make discovery run forever.
const maxPages = 100

// keyValue is MLflow's shape for tags and params.
type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type registeredModel struct {
	Name                 string         `json:"name"`
	CreationTimestamp    int64          `json:"creation_timestamp"`
	LastUpdatedTimestamp int64          `json:"last_updated_timestamp"`
	Description          string         `json:"description"`
	LatestVersions       []modelVersion `json:"latest_versions"`
	Tags                 []keyValue     `json:"tags"`
	Aliases              []modelAlias   `json:"aliases"`
}

type modelAlias struct {
	Alias   string `json:"alias"`
	Version string `json:"version"`
}

type modelVersion struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	CurrentStage string `json:"current_stage"`
	Source       string `json:"source"`
	RunID        string `json:"run_id"`
	Status       string `json:"status"`
}

// versionNumber is the version as an integer. MLflow numbers versions from
// 1; anything unparsable sorts below every real version.
func (v modelVersion) versionNumber() int {
	n, err := strconv.Atoi(v.Version)
	if err != nil {
		return 0
	}
	return n
}

type run struct {
	Info   runInfo   `json:"info"`
	Data   runData   `json:"data"`
	Inputs runInputs `json:"inputs"`
}

type runInfo struct {
	RunID        string `json:"run_id"`
	RunName      string `json:"run_name"`
	ExperimentID string `json:"experiment_id"`
	ArtifactURI  string `json:"artifact_uri"`
}

type runData struct {
	Params  []keyValue `json:"params"`
	Metrics []metric   `json:"metrics"`
	Tags    []keyValue `json:"tags"`
}

// tag returns the value of a run tag, or "" when the run does not carry it.
func (d runData) tag(key string) string {
	for _, t := range d.Tags {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}

// metric is the latest value of one metric; runs/get returns one entry
// per key, at its highest step.
type metric struct {
	Key   string      `json:"key"`
	Value metricValue `json:"value"`
}

// metricValue is a metric's number. MLflow writes a NaN metric to the wire
// as the string "NaN" (JSON has no NaN), so a plain float64 field would
// fail to decode the whole run.
type metricValue float64

func (m *metricValue) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		*m = metricValue(f)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("metric value %s is neither a number nor a string", data)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("metric value %q: %w", s, err)
	}
	*m = metricValue(f)
	return nil
}

func (m metricValue) finite() bool {
	f := float64(m)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

type runInputs struct {
	DatasetInputs []datasetInput `json:"dataset_inputs"`
}

type datasetInput struct {
	Dataset dataset    `json:"dataset"`
	Tags    []keyValue `json:"tags"`
}

// dataset is a dataset logged to a run. Source, Schema and Profile are
// JSON documents MLflow stores as strings.
type dataset struct {
	Name       string `json:"name"`
	Digest     string `json:"digest"`
	SourceType string `json:"source_type"`
	Source     string `json:"source"`
	Schema     string `json:"schema"`
	Profile    string `json:"profile"`
}

type experiment struct {
	ExperimentID     string     `json:"experiment_id"`
	Name             string     `json:"name"`
	ArtifactLocation string     `json:"artifact_location"`
	LifecycleStage   string     `json:"lifecycle_stage"`
	CreationTime     int64      `json:"creation_time"`
	LastUpdateTime   int64      `json:"last_update_time"`
	Tags             []keyValue `json:"tags"`
}

// loggedModel is an MLflow 3 model logged independently of a run. A model
// version registered from one has a models:/<model_id> source.
type loggedModel struct {
	Info loggedModelInfo `json:"info"`
}

type loggedModelInfo struct {
	ArtifactURI string `json:"artifact_uri"`
}

// client talks to one MLflow tracking server. Authentication is optional:
// MLflow's basic-auth app takes a username and password, and servers behind
// a proxy or Databricks take a bearer token.
type client struct {
	baseURL  string
	username string
	password string
	token    string
	http     *http.Client
}

func newClient(config *Config) *client {
	transport := http.DefaultTransport
	if !config.VerifySSL {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-configured option
		transport = insecure
	}

	return &client{
		baseURL:  config.TrackingURI,
		username: config.Username,
		password: config.Password,
		token:    config.Token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// get performs a GET against a path below the tracking server root and
// returns the response body. Artifact reads come back as YAML, so decoding
// is left to getJSON.
func (c *client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", path, err)
	}
	switch {
	case c.token != "":
		req.Header.Set("Authorization", "Bearer "+c.token)
	case c.username != "":
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, &apiError{Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return body, nil
}

func (c *client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	body, err := c.get(ctx, path, query)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// apiError is a non-200 response from MLflow.
type apiError struct {
	Path   string
	Status int
	Body   string
}

func (e *apiError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: %s", e.Path, http.StatusText(e.Status))
	}
	return fmt.Sprintf("%s: %s: %s", e.Path, http.StatusText(e.Status), e.Body)
}

// pageThrough calls fetch with each page token until the server stops
// returning one. fetch returns the next token, or "" to stop.
func pageThrough(what string, fetch func(token string) (string, error)) error {
	token := ""
	for page := 0; page < maxPages; page++ {
		next, err := fetch(token)
		if err != nil {
			return err
		}
		// A token that does not move would page forever.
		if next == "" || next == token {
			return nil
		}
		token = next
	}
	log.Warn().Str("listing", what).Int("pages", maxPages).Msg("Stopped at the page cap, results are incomplete")
	return nil
}

// searchRegisteredModels returns every registered model, or the first
// limit of them when limit is positive.
func (c *client) searchRegisteredModels(ctx context.Context, limit int) ([]registeredModel, error) {
	var all []registeredModel

	err := pageThrough("registered models", func(token string) (string, error) {
		query := url.Values{}
		query.Set("max_results", strconv.Itoa(pageSize))
		if token != "" {
			query.Set("page_token", token)
		}

		var resp struct {
			RegisteredModels []registeredModel `json:"registered_models"`
			NextPageToken    string            `json:"next_page_token"`
		}
		if err := c.getJSON(ctx, apiPrefix+"/registered-models/search", query, &resp); err != nil {
			return "", err
		}

		all = append(all, resp.RegisteredModels...)
		if limit > 0 && len(all) >= limit {
			all = all[:limit]
			return "", nil
		}
		return resp.NextPageToken, nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// searchModelVersions returns every version of one registered model,
// newest first as MLflow orders them.
func (c *client) searchModelVersions(ctx context.Context, name string) ([]modelVersion, error) {
	filter, ok := versionFilter(name)
	if !ok {
		return nil, fmt.Errorf("model name %q holds both quote characters, which the MLflow filter grammar cannot express", name)
	}

	var all []modelVersion

	err := pageThrough("model versions of "+name, func(token string) (string, error) {
		query := url.Values{}
		query.Set("filter", filter)
		query.Set("max_results", strconv.Itoa(pageSize))
		if token != "" {
			query.Set("page_token", token)
		}

		var resp struct {
			ModelVersions []modelVersion `json:"model_versions"`
			NextPageToken string         `json:"next_page_token"`
		}
		if err := c.getJSON(ctx, apiPrefix+"/model-versions/search", query, &resp); err != nil {
			return "", err
		}

		all = append(all, resp.ModelVersions...)
		return resp.NextPageToken, nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// versionFilter builds the search filter that selects one model's
// versions. MLflow's filter grammar has no escape sequence for quotes
// (neither a backslash nor a doubled quote works), but a value may be
// wrapped in either kind of quote, so a name holding a single quote is
// wrapped in double quotes instead. A name holding both cannot be
// filtered on.
func versionFilter(name string) (string, bool) {
	switch {
	case !strings.Contains(name, "'"):
		return "name='" + name + "'", true
	case !strings.Contains(name, `"`):
		return `name="` + name + `"`, true
	default:
		return "", false
	}
}

func (c *client) getRun(ctx context.Context, runID string) (*run, error) {
	query := url.Values{}
	query.Set("run_id", runID)

	var resp struct {
		Run run `json:"run"`
	}
	if err := c.getJSON(ctx, apiPrefix+"/runs/get", query, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

// searchExperiments returns every active experiment. Deleted ones stay in
// MLflow's recycle bin and are left out by the server's default view.
func (c *client) searchExperiments(ctx context.Context) ([]experiment, error) {
	var all []experiment

	err := pageThrough("experiments", func(token string) (string, error) {
		query := url.Values{}
		query.Set("max_results", strconv.Itoa(pageSize))
		if token != "" {
			query.Set("page_token", token)
		}

		var resp struct {
			Experiments   []experiment `json:"experiments"`
			NextPageToken string       `json:"next_page_token"`
		}
		if err := c.getJSON(ctx, apiPrefix+"/experiments/search", query, &resp); err != nil {
			return "", err
		}

		all = append(all, resp.Experiments...)
		return resp.NextPageToken, nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func (c *client) getLoggedModel(ctx context.Context, modelID string) (*loggedModel, error) {
	var resp struct {
		Model loggedModel `json:"model"`
	}
	if err := c.getJSON(ctx, apiPrefix+"/logged-models/"+url.PathEscape(modelID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Model, nil
}

// runArtifact reads one file from a run's artifacts through the endpoint
// the MLflow UI uses. It works on every tracking server that stores or
// proxies its own artifacts.
func (c *client) runArtifact(ctx context.Context, runID, path string) ([]byte, error) {
	query := url.Values{}
	query.Set("run_uuid", runID)
	query.Set("path", path)
	return c.get(ctx, "/get-artifact", query)
}

// proxiedArtifact reads one file below the artifact root of a server
// started with --serve-artifacts, addressed by the path an
// mlflow-artifacts:/ URI carries.
func (c *client) proxiedArtifact(ctx context.Context, path string) ([]byte, error) {
	escaped := make([]string, 0, 8)
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		escaped = append(escaped, url.PathEscape(part))
	}
	return c.get(ctx, "/api/2.0/mlflow-artifacts/artifacts/"+strings.Join(escaped, "/"), nil)
}
