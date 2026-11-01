package spline

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// ExecutionEvent is one Spark run recorded by Spline, as returned by
// GET /consumer/execution-events. Error is free-form: Spline stores whatever
// the agent reported, usually an object with a message field.
type ExecutionEvent struct {
	ExecutionEventID string         `json:"executionEventId"`
	ExecutionPlanID  string         `json:"executionPlanId"`
	FrameworkName    string         `json:"frameworkName"`
	ApplicationName  string         `json:"applicationName"`
	ApplicationID    string         `json:"applicationId"`
	Timestamp        int64          `json:"timestamp"`
	DurationNs       *int64         `json:"durationNs"`
	DataSourceURI    string         `json:"dataSourceUri"`
	DataSourceName   string         `json:"dataSourceName"`
	DataSourceType   string         `json:"dataSourceType"`
	Append           bool           `json:"append"`
	Error            any            `json:"error"`
	Extra            map[string]any `json:"extra"`
}

// ExecutionEventsPage is one page of GET /consumer/execution-events.
type ExecutionEventsPage struct {
	Items      []ExecutionEvent `json:"items"`
	TotalCount int64            `json:"totalCount"`
	PageNum    int              `json:"pageNum"`
	PageSize   int              `json:"pageSize"`
}

// DataSourceInfo is a read or write target of an execution plan.
type DataSourceInfo struct {
	Source     string `json:"source"`
	SourceType string `json:"sourceType"`
}

// NameAndVersion identifies the Spark version or the Spline agent version.
type NameAndVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PlanAttribute is one column produced by an execution plan. The id is
// prefixed with the plan id, which is what attribute lineage is keyed by.
type PlanAttribute struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DataTypeID string `json:"dataTypeId"`
}

// ExecutionPlanExtra is the free-form section of an execution plan. Spline
// versions differ in what they put here, so only the two fields the plugin
// needs are typed.
type ExecutionPlanExtra struct {
	AppName    string          `json:"appName"`
	Attributes []PlanAttribute `json:"attributes"`
}

// ExecutionPlanInfo describes what a Spark job read and wrote.
type ExecutionPlanInfo struct {
	ID         string             `json:"_id"`
	Name       string             `json:"name"`
	SystemInfo NameAndVersion     `json:"systemInfo"`
	AgentInfo  NameAndVersion     `json:"agentInfo"`
	Extra      ExecutionPlanExtra `json:"extra"`
	Inputs     []DataSourceInfo   `json:"inputs"`
	Output     *DataSourceInfo    `json:"output"`
}

// LineageDetailed is the response of GET /consumer/lineage-detailed. The
// operation graph is not read: the plugin only needs the plan's inputs,
// output and attributes.
type LineageDetailed struct {
	ExecutionPlan ExecutionPlanInfo `json:"executionPlan"`
}

// AttributeNode is one column in an attribute dependency graph.
type AttributeNode struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

// AttributeEdge points from a column to a column it was derived from.
type AttributeEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// AttributeGraph is a set of columns and the dependencies between them.
type AttributeGraph struct {
	Nodes []AttributeNode `json:"nodes"`
	Edges []AttributeEdge `json:"edges"`
}

// AttributeLineageAndImpact is the response of
// GET /consumer/attribute-lineage-and-impact. Only the lineage half is used;
// impact points the other way and would duplicate edges the plugin already has.
type AttributeLineageAndImpact struct {
	Lineage AttributeGraph `json:"lineage"`
	Impact  AttributeGraph `json:"impact"`
}

// versionResponse is the response of GET /about/version.
type versionResponse struct {
	Build struct {
		Version string `json:"version"`
	} `json:"build"`
}

// ClientConfig holds the settings for a Spline REST gateway client.
type ClientConfig struct {
	BaseURL   string
	Username  string
	Password  string
	Token     string
	VerifySSL bool
	Timeout   time.Duration
}

// Client talks to the Spline REST gateway. BaseURL is the gateway root, so
// consumer calls go to {BaseURL}/consumer.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
	token      string
}

// NewClient creates a Spline REST gateway client.
func NewClient(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport
	if !config.VerifySSL {
		// Self-signed certificates are common on internal Spline gateways.
		transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	return &Client{
		baseURL:    config.BaseURL,
		httpClient: &http.Client{Timeout: timeout, Transport: transport},
		username:   config.Username,
		password:   config.Password,
		token:      config.Token,
	}
}

// do performs one GET against the gateway and decodes the JSON body into out.
func (c *Client) do(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", path, err)
	}

	if resp.StatusCode >= 400 {
		// The body can be an HTML error page, so only a short prefix is kept.
		return fmt.Errorf("spline returned %d for %s: %s", resp.StatusCode, path, truncate(string(body), 200))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing %s response: %w", path, err)
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ServerVersion returns the Spline server version. The endpoint moved between
// releases, so both known locations are tried and an empty string is returned
// when neither answers.
func (c *Client) ServerVersion(ctx context.Context) string {
	for _, path := range []string{"/about/version", "/consumer/about/version"} {
		var v versionResponse
		if err := c.do(ctx, path, nil, &v); err == nil && v.Build.Version != "" {
			return v.Build.Version
		}
	}
	return ""
}

// maxEventPages caps paging so a gateway that keeps returning full pages
// cannot spin forever.
const maxEventPages = 1000

// ListExecutionEvents returns execution events between start and end, newest
// first, stopping at maxEvents. asAtTime pins the whole scan to a single point
// in time so a run finishing mid-scan cannot shift the pages underneath it.
func (c *Client) ListExecutionEvents(ctx context.Context, start, end time.Time, pageSize, maxEvents int) ([]ExecutionEvent, error) {
	asAtTime := end.UnixMilli()

	var events []ExecutionEvent
	seen := make(map[string]struct{})

	for pageNum := 1; pageNum <= maxEventPages; pageNum++ {
		query := url.Values{}
		query.Set("asAtTime", strconv.FormatInt(asAtTime, 10))
		query.Set("timestampStart", strconv.FormatInt(start.UnixMilli(), 10))
		query.Set("timestampEnd", strconv.FormatInt(end.UnixMilli(), 10))
		query.Set("pageNum", strconv.Itoa(pageNum))
		query.Set("pageSize", strconv.Itoa(pageSize))
		query.Set("sortField", "timestamp")
		query.Set("sortOrder", "desc")

		var page ExecutionEventsPage
		if err := c.do(ctx, "/consumer/execution-events", query, &page); err != nil {
			return nil, fmt.Errorf("listing execution events: %w", err)
		}

		if len(page.Items) == 0 {
			return events, nil
		}

		// Some gateway versions ignore pageNum and keep serving page one.
		// Tracking the ids already seen stops that turning into a loop.
		fresh := 0
		for _, event := range page.Items {
			if _, ok := seen[event.ExecutionEventID]; ok {
				continue
			}
			seen[event.ExecutionEventID] = struct{}{}
			events = append(events, event)
			fresh++

			if len(events) >= maxEvents {
				log.Warn().Int("max_events", maxEvents).Msg("Reached max_events, older Spark runs are not ingested")
				return events, nil
			}
		}

		if fresh == 0 {
			log.Warn().Int("page_num", pageNum).Msg("Spline returned a page it had already served, stopping")
			return events, nil
		}

		if int64(pageNum)*int64(page.PageSize) >= page.TotalCount {
			return events, nil
		}

		if pageNum == maxEventPages {
			log.Warn().Int("pages", maxEventPages).Msg("Reached the execution event page cap")
		}
	}

	return events, nil
}

// GetLineageDetailed returns the execution plan behind an execution event.
func (c *Client) GetLineageDetailed(ctx context.Context, planID string) (*LineageDetailed, error) {
	query := url.Values{}
	query.Set("execId", planID)

	var detailed LineageDetailed
	if err := c.do(ctx, "/consumer/lineage-detailed", query, &detailed); err != nil {
		return nil, fmt.Errorf("getting lineage for plan %s: %w", planID, err)
	}
	return &detailed, nil
}

// GetAttributeLineage returns the columns one column was derived from.
func (c *Client) GetAttributeLineage(ctx context.Context, attributeID string) (*AttributeLineageAndImpact, error) {
	query := url.Values{}
	query.Set("attributeId", attributeID)

	var result AttributeLineageAndImpact
	if err := c.do(ctx, "/consumer/attribute-lineage-and-impact", query, &result); err != nil {
		return nil, fmt.Errorf("getting attribute lineage for %s: %w", attributeID, err)
	}
	return &result, nil
}
