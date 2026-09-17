package redash

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// maxPages caps every paged endpoint. Redash reports a total count and
// rejects a page past the end, so this only trips if the count keeps growing
// while discovery runs.
const maxPages = 1000

// Dashboard is a Redash dashboard. The list endpoint leaves Widgets null and
// the detail endpoint fills it in, so one type serves both.
type Dashboard struct {
	ID         int      `json:"id"`
	Slug       string   `json:"slug"`
	Name       string   `json:"name"`
	User       *User    `json:"user"`
	Tags       []string `json:"tags"`
	IsArchived bool     `json:"is_archived"`
	IsDraft    bool     `json:"is_draft"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	Version    int      `json:"version"`
	Widgets    []Widget `json:"widgets"`
}

// User is the subset of a Redash user the plugin records. Email is read but
// never published, so an owner never leaks a personal address into metadata.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Widget is one tile on a dashboard. A text widget has no visualization at
// all, which is the case the OpenMetadata connector crashes on.
type Widget struct {
	ID            int            `json:"id"`
	Width         int            `json:"width"`
	Text          string         `json:"text"`
	Options       WidgetOptions  `json:"options"`
	Visualization *Visualization `json:"visualization"`
}

// WidgetOptions holds a widget's placement on the dashboard grid.
type WidgetOptions struct {
	IsHidden bool           `json:"isHidden"`
	Position map[string]any `json:"position"`
}

// Visualization is one rendering of a query. Options carries the renderer
// settings; for a CHART it holds globalSeriesType, the field that says what
// the chart actually draws.
type Visualization struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Options     map[string]any `json:"options"`
	Query       *Query         `json:"query"`
}

// Query is a saved Redash query. The list endpoint already returns Options,
// so parameters need no extra request per query.
type Query struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Query        string       `json:"query"`
	DataSourceID int          `json:"data_source_id"`
	IsArchived   bool         `json:"is_archived"`
	IsDraft      bool         `json:"is_draft"`
	Schedule     *Schedule    `json:"schedule"`
	Tags         []string     `json:"tags"`
	User         *User        `json:"user"`
	CreatedAt    string       `json:"created_at"`
	UpdatedAt    string       `json:"updated_at"`
	Runtime      *float64     `json:"runtime"`
	Options      QueryOptions `json:"options"`
}

// Schedule is a query's refresh schedule.
type Schedule struct {
	Interval  int    `json:"interval"`
	Time      string `json:"time"`
	DayOfWeek string `json:"day_of_week"`
	Until     string `json:"until"`
}

// QueryOptions holds a query's user-facing parameters.
type QueryOptions struct {
	Parameters []QueryParameter `json:"parameters"`
}

// QueryParameter is one {{placeholder}} a query prompts for.
type QueryParameter struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// DataSource is a Redash connection. The list endpoint omits Options; only
// the detail endpoint returns them, and Redash redacts the password there.
type DataSource struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Syntax      string         `json:"syntax"`
	ViewOnly    bool           `json:"view_only"`
	Paused      flexBool       `json:"paused"`
	PauseReason string         `json:"pause_reason"`
	Options     map[string]any `json:"options"`
}

// flexBool decodes a field Redash reports inconsistently. Current versions
// build "paused" from a Redis key check, which serialises as 0 or 1; older
// ones stored a real boolean.
type flexBool bool

func (f *flexBool) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*f = flexBool(b)
		return nil
	}

	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = n != 0
		return nil
	}

	// A null or an unexpected shape means "not paused" rather than a
	// discovery failure.
	*f = false
	return nil
}

// Session is the caller's own session. It is the reachability probe and the
// only place Redash 25 reports its version.
type Session struct {
	User         *User `json:"user"`
	ClientConfig struct {
		Version string `json:"version"`
	} `json:"client_config"`
}

// page is the envelope every paged Redash list endpoint returns.
type page[T any] struct {
	Count    int `json:"count"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Results  []T `json:"results"`
}

// Client talks to one Redash instance.
type Client struct {
	baseURL    string
	apiKey     string
	pageSize   int
	httpClient *http.Client
}

// ClientConfig configures a Client.
type ClientConfig struct {
	BaseURL   string
	APIKey    string
	PageSize  int
	VerifySSL bool
	Timeout   time.Duration
}

// NewClient creates a Redash API client.
func NewClient(config ClientConfig) *Client {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	httpClient := &http.Client{Timeout: timeout}
	if !config.VerifySSL {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &Client{
		baseURL:    strings.TrimSuffix(config.BaseURL, "/"),
		apiKey:     config.APIKey,
		pageSize:   config.PageSize,
		httpClient: httpClient,
	}
}

// do performs one GET against the Redash API and decodes the body into out.
func (c *Client) do(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Key "+c.apiKey)
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
		// Redash answers an unauthenticated call with the same 404 it uses
		// for a missing object, so say so rather than leaving the user
		// hunting for a dashboard that does exist.
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("requesting %s: status %d, check the api_key and that the host serves this Redash organisation", path, resp.StatusCode)
		}
		return fmt.Errorf("requesting %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing %s response: %w", path, err)
	}
	return nil
}

// listPaged walks a paged endpoint to the end. Redash rejects a page past the
// last one with a 400, so the loop stops on the count it reports rather than
// probing for an empty page.
func listPaged[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T

	for pageNum := 1; pageNum <= maxPages; pageNum++ {
		query := url.Values{}
		query.Set("page", strconv.Itoa(pageNum))
		query.Set("page_size", strconv.Itoa(c.pageSize))

		var p page[T]
		if err := c.do(ctx, path, query, &p); err != nil {
			return nil, err
		}

		all = append(all, p.Results...)

		size := p.PageSize
		if size <= 0 {
			size = c.pageSize
		}
		if len(p.Results) == 0 || pageNum*size >= p.Count {
			return all, nil
		}
	}

	log.Warn().Str("path", path).Int("max_pages", maxPages).Msg("Stopped paging at the page cap, results may be incomplete")
	return all, nil
}

// GetSession returns the calling user's session. It doubles as the
// reachability probe.
func (c *Client) GetSession(ctx context.Context) (*Session, error) {
	var session Session
	if err := c.do(ctx, "/api/session", nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListDashboards returns every dashboard the API key can see. Redash never
// includes archived dashboards here and offers no archived-dashboard
// endpoint, so archived dashboards are simply invisible to the API.
func (c *Client) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	return listPaged[Dashboard](ctx, c, "/api/dashboards")
}

// GetDashboard returns one dashboard with its widgets. Redash 10 and newer
// key this endpoint on the numeric id; older versions key it on the slug.
func (c *Client) GetDashboard(ctx context.Context, key string) (*Dashboard, error) {
	var dashboard Dashboard
	if err := c.do(ctx, "/api/dashboards/"+url.PathEscape(key), nil, &dashboard); err != nil {
		return nil, err
	}
	return &dashboard, nil
}

// ListQueries returns every non-archived query.
func (c *Client) ListQueries(ctx context.Context) ([]Query, error) {
	return listPaged[Query](ctx, c, "/api/queries")
}

// ListArchivedQueries returns archived queries, which the main query list
// leaves out.
func (c *Client) ListArchivedQueries(ctx context.Context) ([]Query, error) {
	return listPaged[Query](ctx, c, "/api/queries/archive")
}

// ListDataSources returns every data source. This endpoint answers with a
// bare array and is not paged.
func (c *Client) ListDataSources(ctx context.Context) ([]DataSource, error) {
	var sources []DataSource
	if err := c.do(ctx, "/api/data_sources", nil, &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetDataSource returns one data source with its connection options.
func (c *Client) GetDataSource(ctx context.Context, id int) (*DataSource, error) {
	var source DataSource
	if err := c.do(ctx, "/api/data_sources/"+strconv.Itoa(id), nil, &source); err != nil {
		return nil, err
	}
	return &source, nil
}
