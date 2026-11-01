package grafana

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

// maxSearchPages bounds the dashboard listing so a server that keeps
// answering full pages cannot hold discovery open forever.
const maxSearchPages = 1000

// client talks to the Grafana HTTP API. Grafana authenticates machine
// clients with a service account token sent as a bearer token.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(host, token string, verifySSL bool) *client {
	transport := http.DefaultTransport
	if !verifySSL {
		// Clone rather than replace, so proxy settings and connection
		// pooling behave the same with verification turned off.
		insecure := http.DefaultTransport.(*http.Transport).Clone()
		insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		transport = insecure
	}

	return &client{
		baseURL: host,
		token:   token,
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// get performs a GET against a path below the Grafana root and decodes
// the JSON response into out.
func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return newAPIError(path, resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

// apiError is a non-200 response from Grafana. Grafana wraps most
// failures as {"message": "..."}; the message is kept because it is
// what tells a wrong token ("Invalid API key") from a missing permission.
type apiError struct {
	Path    string
	Status  int
	Message string
}

func newAPIError(path string, resp *http.Response) *apiError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	var payload struct {
		Message string `json:"message"`
	}
	message := ""
	if json.Unmarshal(body, &payload) == nil && payload.Message != "" {
		message = payload.Message
	} else {
		message = strings.TrimSpace(string(body))
	}

	return &apiError{Path: path, Status: resp.StatusCode, Message: message}
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s: %s", e.Path, http.StatusText(e.Status))
	}
	return fmt.Sprintf("%s: %s: %s", e.Path, http.StatusText(e.Status), e.Message)
}

// searchHit is one dashboard in a /api/search listing. Only the uid is
// needed: everything else comes from the dashboard endpoint.
type searchHit struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
}

// searchDashboards lists every dashboard. Grafana pages the search
// endpoint with 1-based page numbers and ends the listing with a short
// page.
func (c *client) searchDashboards(ctx context.Context, pageSize int) ([]searchHit, error) {
	var all []searchHit

	for page := 1; page <= maxSearchPages; page++ {
		query := url.Values{}
		query.Set("type", "dash-db")
		query.Set("page", strconv.Itoa(page))
		query.Set("limit", strconv.Itoa(pageSize))

		var hits []searchHit
		if err := c.get(ctx, "/api/search", query, &hits); err != nil {
			return nil, err
		}

		all = append(all, hits...)
		if len(hits) < pageSize {
			return all, nil
		}
	}

	log.Warn().Int("pages", maxSearchPages).Int("dashboards", len(all)).
		Msg("Stopped listing dashboards at the page cap")
	return all, nil
}

// dashboardResponse is /api/dashboards/uid/{uid}: the dashboard's own
// JSON model plus the metadata Grafana keeps about it.
type dashboardResponse struct {
	Dashboard dashboard     `json:"dashboard"`
	Meta      dashboardMeta `json:"meta"`
}

type dashboard struct {
	ID            int64       `json:"id"`
	UID           string      `json:"uid"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Tags          []string    `json:"tags"`
	Panels        []panel     `json:"panels"`
	Rows          []legacyRow `json:"rows"`
	SchemaVersion int         `json:"schemaVersion"`
	Version       int         `json:"version"`
	Refresh       flexString  `json:"refresh"`
	Time          timeRange   `json:"time"`
}

// legacyRow is the row layout dashboards used before schema version 16.
// Grafana upgrades those in the browser but the API returns them as
// stored, so their panels have to be lifted out here.
type legacyRow struct {
	Panels []panel `json:"panels"`
}

type timeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type dashboardMeta struct {
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	FolderID    int64  `json:"folderId"`
	FolderUID   string `json:"folderUid"`
	FolderTitle string `json:"folderTitle"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	CreatedBy   string `json:"createdBy"`
	UpdatedBy   string `json:"updatedBy"`
	Version     int    `json:"version"`
	Provisioned bool   `json:"provisioned"`
}

func (c *client) dashboard(ctx context.Context, uid string) (*dashboardResponse, error) {
	var resp dashboardResponse
	if err := c.get(ctx, "/api/dashboards/uid/"+url.PathEscape(uid), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// panel is one panel of a dashboard. A row panel carries its panels
// inside it while collapsed; a library panel reference carries only the
// uid of the shared panel it stands for.
type panel struct {
	ID           int64          `json:"id"`
	Type         string         `json:"type"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Collapsed    bool           `json:"collapsed"`
	Panels       []panel        `json:"panels"`
	Datasource   *datasourceRef `json:"datasource"`
	Targets      []target       `json:"targets"`
	LibraryPanel *libraryRef    `json:"libraryPanel"`
}

type libraryRef struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

// target is one query of a panel. Which field holds the query text
// depends on the data source plugin: SQL sources write rawSql, the
// community ClickHouse source writes query, Prometheus and Loki write
// expr.
type target struct {
	RefID      string         `json:"refId"`
	Datasource *datasourceRef `json:"datasource"`
	RawSQL     flexString     `json:"rawSql"`
	Query      flexString     `json:"query"`
	Expr       flexString     `json:"expr"`
}

// datasourceRef is how a panel or target names its data source. Grafana
// 8.3 and later write {type, uid}; older dashboards write a bare string
// holding either the data source's name or its uid.
type datasourceRef struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

func (d *datasourceRef) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case string:
		*d = datasourceRef{UID: v}
	case map[string]any:
		uid, _ := v["uid"].(string)
		typ, _ := v["type"].(string)
		*d = datasourceRef{Type: typ, UID: uid}
	default:
		// null or a shape no Grafana version writes: treat as unset
		// rather than fail the whole dashboard.
		*d = datasourceRef{}
	}
	return nil
}

// flexString decodes a field that Grafana has written as a string in
// some versions and as another JSON type in others: refresh is false
// when auto refresh is off, and query is an object for some data
// source plugins. Anything that is not a string, number or true reads
// as empty.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case string:
		*f = flexString(v)
	case bool:
		if v {
			*f = "true"
		} else {
			*f = ""
		}
	case float64:
		*f = flexString(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		*f = ""
	}
	return nil
}

// libraryElementResponse is /api/library-elements/{uid}. The model is
// the full panel the dashboard reference stands for.
type libraryElementResponse struct {
	Result struct {
		Model panel `json:"model"`
	} `json:"result"`
}

func (c *client) libraryPanel(ctx context.Context, uid string) (*panel, error) {
	var resp libraryElementResponse
	if err := c.get(ctx, "/api/library-elements/"+url.PathEscape(uid), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Result.Model, nil
}

// datasource is one entry of /api/datasources. Only the descriptive
// fields are decoded: the listing never includes secrets, and this
// struct makes sure none are picked up should that ever change.
type datasource struct {
	ID        int64  `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	TypeName  string `json:"typeName"`
	URL       string `json:"url"`
	Database  string `json:"database"`
	IsDefault bool   `json:"isDefault"`
	ReadOnly  bool   `json:"readOnly"`
	Access    string `json:"access"`
	JSONData  struct {
		Database string `json:"database"`
	} `json:"jsonData"`
}

// databaseName is the database a data source connects to. Newer Grafana
// releases keep it in jsonData and leave the top-level field empty.
func (d *datasource) databaseName() string {
	if d.Database != "" {
		return d.Database
	}
	return d.JSONData.Database
}

func (c *client) datasources(ctx context.Context) ([]datasource, error) {
	var list []datasource
	if err := c.get(ctx, "/api/datasources", nil, &list); err != nil {
		return nil, err
	}
	return list, nil
}
