package superset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// fakeSuperset stands in for a Superset server. The JSON it serves is
// trimmed from what Superset 6.1.0 answered during development, so the
// field names and nesting are the real ones. Tests register objects and
// then run a real discovery against it.
type fakeSuperset struct {
	mu sync.Mutex

	dashboards  []map[string]interface{}
	charts      []map[string]interface{}
	datasets    []map[string]interface{}
	databases   []map[string]interface{}
	details     map[string]map[string]interface{} // "dataset/1", "dashboard/1", "database/1/connection"
	memberships map[int][]map[string]interface{}  // dashboard id -> /dashboard/{id}/charts
	failing     map[string]int                    // path -> status
	rejectLogin bool

	requests []*http.Request
}

func newFakeSuperset() *fakeSuperset {
	return &fakeSuperset{
		details:     make(map[string]map[string]interface{}),
		memberships: make(map[int][]map[string]interface{}),
		failing:     make(map[string]int),
	}
}

func (f *fakeSuperset) withDashboard(d map[string]interface{}, chartIDs ...int) *fakeSuperset {
	f.dashboards = append(f.dashboards, d)
	id := d["id"].(int)
	f.details[fmt.Sprintf("dashboard/%d", id)] = map[string]interface{}{
		"id": id, "dashboard_title": d["dashboard_title"], "position_json": nil, "json_metadata": nil,
	}
	f.memberships[id] = []map[string]interface{}{}
	for _, chartID := range chartIDs {
		f.memberships[id] = append(f.memberships[id], map[string]interface{}{
			"id": chartID, "slice_name": fmt.Sprintf("chart %d", chartID),
			"slice_url": fmt.Sprintf("/explore/?slice_id=%d", chartID),
		})
	}
	return f
}

// withLayout gives a dashboard a position_json naming charts, the shape
// read when the charts endpoint is unavailable.
func (f *fakeSuperset) withLayout(dashboardID int, chartIDs ...int) *fakeSuperset {
	layout := map[string]interface{}{
		"DASHBOARD_VERSION_KEY": "v2",
		"ROOT_ID":               map[string]interface{}{"type": "ROOT", "id": "ROOT_ID", "children": []string{"GRID_ID"}},
		"GRID_ID":               map[string]interface{}{"type": "GRID", "id": "GRID_ID", "children": []string{"ROW-1"}},
		"HEADER_ID":             map[string]interface{}{"type": "HEADER", "id": "HEADER_ID", "meta": map[string]interface{}{"text": "title"}},
		"ROW-1":                 map[string]interface{}{"type": "ROW", "id": "ROW-1", "children": []string{}},
	}
	for _, id := range chartIDs {
		key := fmt.Sprintf("CHART-%d", id)
		layout[key] = map[string]interface{}{
			"type": "CHART", "id": key, "children": []string{},
			"meta": map[string]interface{}{"width": 4, "height": 50, "chartId": id, "sliceName": "x"},
		}
	}
	encoded, _ := json.Marshal(layout)
	f.details[fmt.Sprintf("dashboard/%d", dashboardID)]["position_json"] = string(encoded)
	return f
}

func (f *fakeSuperset) withChart(c map[string]interface{}) *fakeSuperset {
	f.charts = append(f.charts, c)
	return f
}

// withDataset registers a dataset. The listing gets the entry as is; the
// detail endpoint adds what only it carries: columns, the edit URL and
// the database backend.
func (f *fakeSuperset) withDataset(d map[string]interface{}, backend string, columns ...map[string]interface{}) *fakeSuperset {
	f.datasets = append(f.datasets, d)

	detail := make(map[string]interface{}, len(d)+3)
	for k, v := range d {
		detail[k] = v
	}
	delete(detail, "changed_on_utc")
	db := d["database"].(map[string]interface{})
	detail["database"] = map[string]interface{}{
		"id": db["id"], "database_name": db["database_name"], "backend": backend, "allow_multi_catalog": false,
	}
	detail["url"] = fmt.Sprintf("/tablemodelview/edit/%d", d["id"])
	if columns == nil {
		columns = []map[string]interface{}{}
	}
	detail["columns"] = columns
	detail["metrics"] = []map[string]interface{}{{"metric_name": "count", "expression": "COUNT(*)"}}
	f.details[fmt.Sprintf("dataset/%d", d["id"])] = detail
	return f
}

func (f *fakeSuperset) withDatabase(db map[string]interface{}, conn map[string]interface{}) *fakeSuperset {
	f.databases = append(f.databases, db)
	if conn != nil {
		f.details[fmt.Sprintf("database/%d/connection", db["id"])] = conn
	}
	return f
}

// failing makes one path answer with the given status.
func (f *fakeSuperset) withFailure(path string, status int) *fakeSuperset {
	f.failing[path] = status
	return f
}

func (f *fakeSuperset) rejectingLogin() *fakeSuperset {
	f.rejectLogin = true
	return f
}

var risonRe = regexp.MustCompile(`\(page:(\d+),page_size:(\d+)\)`)

func (f *fakeSuperset) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r)
		f.mu.Unlock()

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		w.Header().Set("Content-Type", "application/json")

		if path == "security/login" {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.rejectLogin || body["username"] != "admin" || body["password"] != "admin" {
				http.Error(w, `{"message":"Not authorized"}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "fake-access", "refresh_token": "fake-refresh"})
			return
		}

		if r.Header.Get("Authorization") != "Bearer fake-access" {
			http.Error(w, `{"msg":"Missing Authorization Header"}`, http.StatusUnauthorized)
			return
		}

		if status, ok := f.failing[path]; ok {
			http.Error(w, `{"message":"Fatal error"}`, status)
			return
		}

		if strings.HasSuffix(path, "/charts") {
			id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(path, "dashboard/"), "/charts"))
			charts, ok := f.memberships[id]
			if !ok {
				http.Error(w, `{"message":"Not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": charts})
			return
		}

		if detail, ok := f.details[path]; ok {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": detail["id"], "result": detail})
			return
		}

		var entries []map[string]interface{}
		switch path {
		case "dashboard/":
			entries = f.dashboards
		case "chart/":
			entries = f.charts
		case "dataset/":
			entries = f.datasets
		case "database/":
			entries = f.databases
		default:
			http.Error(w, `{"message":"Not found"}`, http.StatusNotFound)
			return
		}
		writePage(w, r, entries)
	}))

	t.Cleanup(server.Close)
	return server
}

// writePage answers a list request the way Superset does: the total
// count plus the slice of entries the Rison page selects.
func writePage(w http.ResponseWriter, r *http.Request, entries []map[string]interface{}) {
	page, pageSize := 0, 100
	if m := risonRe.FindStringSubmatch(r.URL.RawQuery); m != nil {
		page, _ = strconv.Atoi(m[1])
		pageSize, _ = strconv.Atoi(m[2])
	}

	start := page * pageSize
	if start > len(entries) {
		start = len(entries)
	}
	end := start + pageSize
	if end > len(entries) {
		end = len(entries)
	}

	result := entries[start:end]
	if result == nil {
		result = []map[string]interface{}{}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count":  len(entries),
		"result": result,
	})
}

// paths returns every request path the fake has served, in order.
func (f *fakeSuperset) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, r.URL.Path)
	}
	return out
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeSuperset, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL, "username": "admin", "password": "admin"}
	for key, value := range overrides {
		config[key] = value
	}

	source := &Source{}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

// findAsset returns the asset of one type whose name lands on the given
// MRN name, or nil.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	want := mrn.New(assetType, provider, name)
	for i, asset := range result.Assets {
		if asset.MRN != nil && *asset.MRN == want {
			return &result.Assets[i]
		}
	}
	return nil
}

// hasEdge reports whether the result contains an edge between two MRNs.
func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

// schemaColumns decodes the column list stored on an asset.
func schemaColumns(t *testing.T, asset *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()

	raw, ok := asset.Schema["columns"]
	require.True(t, ok, "expected columns on %s", *asset.Name)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]interface{}, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

var adminOwner = map[string]interface{}{"email": "admin@example.com", "first_name": "A", "id": 1, "last_name": "B"}

// Fixtures below mirror the list entries Superset 6.1.0 returned for the
// objects seeded during development, trimmed to the fields that matter.

func dashboardEntry(id int, title, slug string, published bool) map[string]interface{} {
	status := "draft"
	if published {
		status = "published"
	}
	return map[string]interface{}{
		"changed_by":            map[string]interface{}{"first_name": "A", "id": 1, "last_name": "B"},
		"changed_on_utc":        "2026-09-07T21:12:01.731766+0000",
		"dashboard_title":       title,
		"id":                    id,
		"owners":                []map[string]interface{}{adminOwner},
		"published":             published,
		"roles":                 []interface{}{},
		"slug":                  slug,
		"status":                status,
		"tags":                  []interface{}{},
		"url":                   "/superset/dashboard/" + slug + "/",
		"is_managed_externally": false,
	}
}

func chartEntry(id int, name, vizType string, datasetID int, datasetName string, dashboards ...map[string]interface{}) map[string]interface{} {
	if dashboards == nil {
		dashboards = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"changed_on_utc":       "2026-09-07T21:12:01.991148+0000",
		"dashboards":           dashboards,
		"datasource_id":        datasetID,
		"datasource_name_text": datasetName,
		"datasource_type":      "table",
		"datasource_url":       fmt.Sprintf("/explore/?datasource_type=table&datasource_id=%d", datasetID),
		"description":          nil,
		"edit_url":             fmt.Sprintf("/chart/edit/%d", id),
		"id":                   id,
		"owners":               []map[string]interface{}{adminOwner},
		"slice_name":           name,
		"slice_url":            fmt.Sprintf("/explore/?slice_id=%d&form_data=%%7B%%22slice_id%%22%%3A%%20%d%%7D", id, id),
		"tags":                 []interface{}{},
		"url":                  fmt.Sprintf("/explore/?slice_id=%d", id),
		"viz_type":             vizType,
	}
}

func dashboardRefEntry(id int, title string) map[string]interface{} {
	return map[string]interface{}{"dashboard_title": title, "id": id}
}

func datasetEntry(id int, schema, table, kind, sql string, databaseID int, databaseName string) map[string]interface{} {
	var sqlValue interface{}
	if sql != "" {
		sqlValue = sql
	}
	return map[string]interface{}{
		"catalog":         "shop",
		"changed_on_utc":  "2026-09-07T21:12:25.530269+0000",
		"database":        map[string]interface{}{"database_name": databaseName, "id": databaseID},
		"datasource_type": "table",
		"description":     nil,
		"id":              id,
		"kind":            kind,
		"owners":          []map[string]interface{}{{"first_name": "A", "id": 1, "last_name": "B"}},
		"schema":          schema,
		"sql":             sqlValue,
		"table_name":      table,
	}
}

func columnEntry(name, dataType string) map[string]interface{} {
	return map[string]interface{}{
		"advanced_data_type": nil, "column_name": name, "description": nil, "expression": nil,
		"filterable": true, "groupby": true, "is_active": true, "is_dttm": false, "type": dataType,
	}
}

func databaseEntry(id int, name, backend string) map[string]interface{} {
	return map[string]interface{}{
		"allow_ctas": false, "allow_cvas": false, "allow_dml": false, "allow_file_upload": false,
		"backend": backend, "database_name": name, "expose_in_sqllab": true, "id": id,
		"engine_information": map[string]interface{}{"supports_file_upload": true},
	}
}

func connectionEntry(id int, name, backend, driver, uri, host string, port int, database string) map[string]interface{} {
	return map[string]interface{}{
		"backend": backend, "database_name": name, "driver": driver, "expose_in_sqllab": true, "allow_dml": false,
		"id": id, "sqlalchemy_uri": uri,
		"parameters": map[string]interface{}{
			"database": database, "encryption": false, "host": host, "password": "XXXXXXXXXX",
			"port": port, "query": map[string]interface{}{}, "username": "marmot",
		},
	}
}

// shop is a fake holding the objects seeded during development: one
// Postgres connection, physical and virtual datasets, charts on a
// published dashboard, and a draft dashboard.
func shop() *fakeSuperset {
	overview := dashboardRefEntry(1, "Shop Overview")
	draft := dashboardRefEntry(2, "Draft Scratchpad")

	return newFakeSuperset().
		withDatabase(databaseEntry(1, "Shop", "postgresql"),
			connectionEntry(1, "Shop", "postgresql", "psycopg2", "postgresql://marmot:XXXXXXXXXX@marmot-test-superset-pg:5432/shop", "marmot-test-superset-pg", 5432, "shop")).
		withDataset(datasetEntry(1, "public", "orders", "physical", "", 1, "Shop"), "postgresql",
			columnEntry("id", "INTEGER"),
			columnEntry("total", "NUMERIC(10, 2)"),
			map[string]interface{}{"column_name": "status", "type": "TEXT", "is_dttm": false, "description": "Order lifecycle state", "expression": nil},
			map[string]interface{}{"column_name": "ordered_at", "type": "TIMESTAMP WITH TIME ZONE", "is_dttm": true, "description": nil, "expression": nil},
			map[string]interface{}{"column_name": "total_with_tax", "type": "NUMERIC", "is_dttm": false, "description": "Order total including 21% VAT", "expression": "total * 1.21"}).
		withDataset(datasetEntry(2, "public", "customers", "physical", "", 1, "Shop"), "postgresql",
			columnEntry("id", "INTEGER"), columnEntry("name", "TEXT")).
		withDataset(datasetEntry(4, "public", "order_totals", "virtual",
			"SELECT c.id AS customer_id, c.name, SUM(o.total) AS total_spent FROM public.orders o JOIN public.customers c ON c.id = o.customer_id GROUP BY c.id, c.name",
			1, "Shop"), "postgresql",
			columnEntry("customer_id", "INTEGER"), columnEntry("name", "STRING"), columnEntry("total_spent", "DECIMAL")).
		withChart(chartEntry(1, "Orders by Status", "table", 1, "public.orders", overview)).
		withChart(chartEntry(2, "Spend per Customer", "pie", 4, "public.order_totals", overview)).
		withChart(chartEntry(3, "Product Prices", "echarts_timeseries_bar", 2, "public.customers", draft)).
		withDashboard(dashboardEntry(1, "Shop Overview", "shop-overview", true), 1, 2).
		withDashboard(dashboardEntry(2, "Draft Scratchpad", "draft-scratchpad", false), 3)
}
