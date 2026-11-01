package superset

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// maxPages bounds a paginated listing so a server that keeps answering
// with the same page cannot spin discovery forever.
const maxPages = 1000

// client talks to the Superset REST API. Superset issues a short-lived
// JWT on login, which every later call sends as a bearer token.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(host string, timeout time.Duration, verifySSL bool) *client {
	transport := http.DefaultTransport
	if !verifySSL {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user opted out of certificate verification
		transport = insecure
	}

	return &client{
		baseURL: strings.TrimSuffix(strings.TrimSpace(host), "/"),
		http: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}

// login exchanges the credentials for an access token.
func (c *client) login(ctx context.Context, username, password, provider string) error {
	body := map[string]interface{}{
		"username": username,
		"password": password,
		"provider": provider,
		"refresh":  true,
	}

	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/security/login", "", body, &resp); err != nil {
		return err
	}
	if resp.AccessToken == "" {
		return fmt.Errorf("login returned no access token")
	}

	c.token = resp.AccessToken
	return nil
}

// do performs one request against a path below the host. rawQuery is
// appended to the URL as-is: Superset filters and pages with a Rison
// expression such as (page:0,page_size:100), whose parentheses, colons
// and commas are valid query characters and are sent literally.
func (c *client) do(ctx context.Context, method, path, rawQuery string, body, out interface{}) error {
	endpoint := c.baseURL + path
	if rawQuery != "" {
		endpoint += "?" + rawQuery
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request for %s: %w", path, err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body of an error is short JSON. It is included so a wrong
		// password or a missing permission is readable; it never echoes
		// the request, so the token cannot leak through it.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &apiError{Path: path, Status: resp.StatusCode, Body: strings.TrimSpace(string(snippet))}
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// apiError is a non-2xx response from Superset.
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

// listResponse is the envelope every Superset list endpoint returns.
type listResponse[T any] struct {
	Count  int `json:"count"`
	Result []T `json:"result"`
}

// itemResponse is the envelope every Superset single-object endpoint
// returns.
type itemResponse[T any] struct {
	Result T `json:"result"`
}

// listAll pages through a Superset list endpoint. Pages are zero-based
// and the server may clamp page_size below what was asked for, so the
// loop counts what actually arrived rather than trusting the page size.
func listAll[T any](ctx context.Context, c *client, path string, pageSize int) ([]T, error) {
	var all []T

	for page := 0; page < maxPages; page++ {
		var resp listResponse[T]
		query := fmt.Sprintf("q=(page:%d,page_size:%d)", page, pageSize)
		if err := c.do(ctx, http.MethodGet, path, query, nil, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Result...)
		if len(resp.Result) == 0 || len(all) >= resp.Count {
			return all, nil
		}
	}

	log.Warn().Str("path", path).Int("pages", maxPages).Msg("Stopped paging before the end of the listing")
	return all, nil
}

// get fetches one object by id and unwraps the result envelope.
func get[T any](ctx context.Context, c *client, path string) (*T, error) {
	var resp itemResponse[T]
	if err := c.do(ctx, http.MethodGet, path, "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

// owner is a Superset user reference. Only the name is recorded, so an
// email address never lands in asset metadata.
type owner struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// tag is a Superset tag. Superset files ownership and favourites as
// tags too, with a type other than 1; only type 1 is a tag a person
// typed in.
type tag struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

const userTagType = 1

type dashboard struct {
	ID             int     `json:"id"`
	DashboardTitle string  `json:"dashboard_title"`
	Slug           string  `json:"slug"`
	URL            string  `json:"url"`
	Published      bool    `json:"published"`
	Status         string  `json:"status"`
	ChangedOnUTC   string  `json:"changed_on_utc"`
	Owners         []owner `json:"owners"`
	Tags           []tag   `json:"tags"`
}

// dashboardDetail is the part of GET /dashboard/{id} used when the
// charts endpoint is unavailable: the layout, as a JSON string.
type dashboardDetail struct {
	PositionJSON string `json:"position_json"`
}

// dashboardChart is one entry of GET /dashboard/{id}/charts.
type dashboardChart struct {
	ID int `json:"id"`
}

type dashboardRef struct {
	ID             int    `json:"id"`
	DashboardTitle string `json:"dashboard_title"`
}

type chart struct {
	ID                 int            `json:"id"`
	SliceName          string         `json:"slice_name"`
	Description        string         `json:"description"`
	VizType            string         `json:"viz_type"`
	DatasourceID       int            `json:"datasource_id"`
	DatasourceType     string         `json:"datasource_type"`
	DatasourceNameText string         `json:"datasource_name_text"`
	URL                string         `json:"url"`
	ChangedOnUTC       string         `json:"changed_on_utc"`
	Owners             []owner        `json:"owners"`
	Tags               []tag          `json:"tags"`
	Dashboards         []dashboardRef `json:"dashboards"`
}

type databaseRef struct {
	ID           int    `json:"id"`
	DatabaseName string `json:"database_name"`
	Backend      string `json:"backend"`
}

type datasetColumn struct {
	ColumnName  string `json:"column_name"`
	Type        string `json:"type"`
	IsDttm      bool   `json:"is_dttm"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
}

type dataset struct {
	ID           int             `json:"id"`
	TableName    string          `json:"table_name"`
	Schema       string          `json:"schema"`
	Kind         string          `json:"kind"`
	SQL          string          `json:"sql"`
	Description  string          `json:"description"`
	URL          string          `json:"url"`
	ChangedOnUTC string          `json:"changed_on_utc"`
	Database     databaseRef     `json:"database"`
	Columns      []datasetColumn `json:"columns"`
	Owners       []owner         `json:"owners"`
}

type database struct {
	ID             int    `json:"id"`
	DatabaseName   string `json:"database_name"`
	Backend        string `json:"backend"`
	ExposeInSQLLab bool   `json:"expose_in_sqllab"`
	AllowDML       bool   `json:"allow_dml"`
}

// databaseConnection is what GET /database/{id}/connection returns.
// Superset masks the password in both the URI and the parameters.
type databaseConnection struct {
	Driver        string `json:"driver"`
	SQLAlchemyURI string `json:"sqlalchemy_uri"`
	Parameters    struct {
		Host     string      `json:"host"`
		Port     json.Number `json:"port"`
		Database string      `json:"database"`
	} `json:"parameters"`
}

func (c *client) dashboards(ctx context.Context, pageSize int) ([]dashboard, error) {
	return listAll[dashboard](ctx, c, "/api/v1/dashboard/", pageSize)
}

func (c *client) charts(ctx context.Context, pageSize int) ([]chart, error) {
	return listAll[chart](ctx, c, "/api/v1/chart/", pageSize)
}

func (c *client) datasets(ctx context.Context, pageSize int) ([]dataset, error) {
	return listAll[dataset](ctx, c, "/api/v1/dataset/", pageSize)
}

func (c *client) databases(ctx context.Context, pageSize int) ([]database, error) {
	return listAll[database](ctx, c, "/api/v1/database/", pageSize)
}

// dataset fetches one dataset in full. The listing leaves out columns,
// the edit URL and the database backend, so every dataset needs this.
func (c *client) dataset(ctx context.Context, id int) (*dataset, error) {
	return get[dataset](ctx, c, fmt.Sprintf("/api/v1/dataset/%d", id))
}

// dashboardCharts returns the ids of the charts placed on a dashboard.
func (c *client) dashboardCharts(ctx context.Context, id int) ([]int, error) {
	var resp listResponse[dashboardChart]
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/dashboard/%d/charts", id), "", nil, &resp); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(resp.Result))
	for _, ch := range resp.Result {
		ids = append(ids, ch.ID)
	}
	return ids, nil
}

// dashboardLayoutCharts reads the chart ids out of a dashboard's
// position_json, for servers where the charts endpoint is unavailable.
func (c *client) dashboardLayoutCharts(ctx context.Context, id int) ([]int, error) {
	detail, err := get[dashboardDetail](ctx, c, fmt.Sprintf("/api/v1/dashboard/%d", id))
	if err != nil {
		return nil, err
	}
	return chartIDsFromLayout(detail.PositionJSON)
}

// chartIDsFromLayout parses a dashboard layout. The layout is a map of
// components keyed by id; the ones whose key starts with CHART- carry
// the chart id under meta.chartId.
func chartIDsFromLayout(positionJSON string) ([]int, error) {
	if strings.TrimSpace(positionJSON) == "" {
		return nil, nil
	}

	// Values are decoded lazily: the map also holds plain strings such as
	// DASHBOARD_VERSION_KEY, which are not components.
	var layout map[string]json.RawMessage
	if err := json.Unmarshal([]byte(positionJSON), &layout); err != nil {
		return nil, fmt.Errorf("parsing position_json: %w", err)
	}

	var ids []int
	for key, raw := range layout {
		if !strings.HasPrefix(key, "CHART-") {
			continue
		}
		var component struct {
			Meta struct {
				ChartID int `json:"chartId"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(raw, &component); err != nil {
			log.Warn().Err(err).Str("component", key).Msg("Skipping unreadable chart component in dashboard layout")
			continue
		}
		if component.Meta.ChartID != 0 {
			ids = append(ids, component.Meta.ChartID)
		}
	}
	return ids, nil
}

// connection fetches a database's connection details. The dedicated
// endpoint is the one that carries them on current releases; older
// ones returned the same fields from the plain detail endpoint, so
// that is the fallback.
func (c *client) connection(ctx context.Context, id int) (*databaseConnection, error) {
	conn, err := get[databaseConnection](ctx, c, fmt.Sprintf("/api/v1/database/%d/connection", id))
	if err == nil {
		return conn, nil
	}

	log.Debug().Err(err).Int("database_id", id).Msg("Connection endpoint unavailable, reading the database detail instead")
	return get[databaseConnection](ctx, c, fmt.Sprintf("/api/v1/database/%d", id))
}

// redactURI strips the password out of a SQLAlchemy URI. Superset masks
// it before answering, but the plugin does not rely on that.
func redactURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "xxx")
		}
	}
	return u.String()
}

// uriDatabase returns the first path segment of a SQLAlchemy URI, which
// is the database (or catalog) the connection opens by default.
func uriDatabase(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
