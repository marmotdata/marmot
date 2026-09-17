package redash

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// fakeRedash stands in for a Redash instance. Every payload below is the
// shape a real Redash 25.1.0 returned, trimmed to the fields the plugin
// reads plus a few it deliberately ignores, so the tests fail if the plugin
// starts depending on something Redash does not send.
type fakeRedash struct {
	version string

	dataSources      []map[string]any
	dataSourceDetail map[int]map[string]any
	queries          []map[string]any
	archivedQueries  []map[string]any
	dashboards       []map[string]any
	dashboardDetail  map[string]map[string]any

	// sessionFails makes the version probe answer 500, the way a Redash
	// behind a proxy that blocks /api/session does.
	sessionFails bool
	// dashboardsFail makes the dashboard list answer 500.
	dashboardsFail bool
	// detailByIDFails makes the dashboard detail endpoint reject a numeric
	// key, the way Redash 8 does.
	detailByIDFails bool

	requests []string
}

func newFakeRedash() *fakeRedash {
	return &fakeRedash{
		version:          "25.1.0",
		dataSourceDetail: make(map[int]map[string]any),
		dashboardDetail:  make(map[string]map[string]any),
	}
}

// start serves the fake over HTTP and returns its base URL.
func (f *fakeRedash) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.Path)

		if key := r.Header.Get("Authorization"); key != "Key test-key" {
			// Redash answers an unauthenticated call with a 404, not a 401.
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Couldn't find resource. Please login and try again."})
			return
		}

		path := r.URL.Path
		switch {
		case path == "/api/session":
			if f.sessionFails {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "Internal Server Error"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"user":          map[string]any{"id": 1, "name": "Marmot Admin", "email": "marmot@example.com"},
				"org_slug":      "default",
				"client_config": map[string]any{"version": f.version, "pageSize": 20},
			})

		case path == "/api/data_sources":
			writeJSON(w, http.StatusOK, f.dataSources)

		case strings.HasPrefix(path, "/api/data_sources/"):
			id, _ := strconv.Atoi(strings.TrimPrefix(path, "/api/data_sources/"))
			detail, ok := f.dataSourceDetail[id]
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "Couldn't find resource."})
				return
			}
			writeJSON(w, http.StatusOK, detail)

		case path == "/api/queries":
			f.writePage(w, r, f.queries)

		case path == "/api/queries/archive":
			f.writePage(w, r, f.archivedQueries)

		case path == "/api/dashboards":
			if f.dashboardsFail {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "Internal Server Error"})
				return
			}
			f.writePage(w, r, f.dashboards)

		case strings.HasPrefix(path, "/api/dashboards/"):
			key := strings.TrimPrefix(path, "/api/dashboards/")
			if f.detailByIDFails {
				if _, err := strconv.Atoi(key); err == nil {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "Internal Server Error"})
					return
				}
			}
			detail, ok := f.dashboardDetail[key]
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "Couldn't find resource."})
				return
			}
			writeJSON(w, http.StatusOK, detail)

		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Couldn't find resource."})
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// writePage answers like a Redash list endpoint: a count/page/page_size
// envelope, a page size capped at 250, and a 400 rather than an empty page
// once the caller runs off the end.
func (f *fakeRedash) writePage(w http.ResponseWriter, r *http.Request, all []map[string]any) {
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if pageNum < 1 {
		pageNum = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if size < 1 {
		size = 25
	}
	if size > 250 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Page size is out of range (1-250)."})
		return
	}

	start := (pageNum - 1) * size
	if start >= len(all) && len(all) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Page is out of range."})
		return
	}
	end := min(start+size, len(all))

	results := []map[string]any{}
	if start < len(all) {
		results = all[start:end]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":     len(all),
		"page":      pageNum,
		"page_size": size,
		"results":   results,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// withDataSource registers a data source on both the list and the detail
// endpoint. Redash omits options from the list and redacts the password in
// the detail, and the fake does the same.
func (f *fakeRedash) withDataSource(id int, name, dsType, syntax string, options map[string]any) *fakeRedash {
	f.dataSources = append(f.dataSources, map[string]any{
		"id": id, "name": name, "type": dsType, "syntax": syntax,
		"paused": 0, "pause_reason": nil, "supports_auto_limit": true, "view_only": false,
	})

	detail := map[string]any{
		"id": id, "name": name, "type": dsType, "syntax": syntax,
		"paused": 0, "pause_reason": nil, "supports_auto_limit": true, "view_only": false,
		"queue_name": "queries", "scheduled_queue_name": "scheduled_queries",
		"groups":  map[string]any{"2": false},
		"options": options,
	}
	f.dataSourceDetail[id] = detail
	return f
}

// queryPayload builds the query shape Redash returns, which is the same on
// the query list and nested inside a dashboard widget.
func queryPayload(id int, name, description, sql string, dataSourceID int) map[string]any {
	return map[string]any{
		"id": id, "name": name, "description": description, "query": sql,
		"query_hash": "c49c06e13ceb56d55568b780d5f9c5f0",
		"schedule":   nil,
		// Redash returns a per-query api key. The plugin must never record
		// it, so the fake sends one.
		"api_key":              "aCtrbcSVjggRc7BitHVvl1M5GtBwt2tT8XVEK2r5",
		"is_archived":          false,
		"is_draft":             false,
		"updated_at":           "2026-09-08T08:48:20.482Z",
		"created_at":           "2026-09-08T08:47:25.403Z",
		"data_source_id":       dataSourceID,
		"options":              map[string]any{},
		"version":              1,
		"tags":                 []string{},
		"is_safe":              true,
		"latest_query_data_id": nil,
		"runtime":              nil,
		"retrieved_at":         nil,
		"is_favorite":          false,
		"last_modified_by_id":  1,
		"user":                 map[string]any{"id": 1, "name": "Marmot Admin", "email": "marmot@example.com"},
	}
}

// dashboardPayload builds the dashboard shape the list endpoint returns.
// Redash leaves widgets null there and fills it in on the detail endpoint.
func dashboardPayload(id int, name, slug string) map[string]any {
	return map[string]any{
		"id": id, "slug": slug, "name": name, "user_id": 1,
		"user":                      map[string]any{"id": 1, "name": "Marmot Admin", "email": "marmot@example.com"},
		"layout":                    []any{},
		"dashboard_filters_enabled": false,
		"widgets":                   nil,
		"options":                   map[string]any{},
		"is_archived":               false,
		"is_draft":                  false,
		"tags":                      []string{},
		"updated_at":                "2026-09-08T08:48:20.396Z",
		"created_at":                "2026-09-08T08:47:54.122Z",
		"version":                   3,
		"is_favorite":               false,
	}
}

func visualizationWidget(widgetID, visualizationID int, vizType, vizName, vizDescription string, options map[string]any, query map[string]any) map[string]any {
	return map[string]any{
		"id": widgetID, "width": 1, "dashboard_id": 1, "text": nil,
		"options": map[string]any{
			"isHidden": false,
			"position": map[string]any{"col": 0, "row": 0, "sizeX": 3, "sizeY": 8},
		},
		"created_at": "2026-09-08T08:48:03.782Z",
		"updated_at": "2026-09-08T08:48:03.782Z",
		"visualization": map[string]any{
			"id": visualizationID, "type": vizType, "name": vizName, "description": vizDescription,
			"options":    options,
			"created_at": "2026-09-08T08:47:46.960Z",
			"updated_at": "2026-09-08T08:48:03.782Z",
			"query":      query,
		},
	}
}

// textWidget is a dashboard note. Redash leaves the visualization key out of
// the payload entirely, which is the shape the OpenMetadata connector
// crashes on.
func textWidget(widgetID int, text string) map[string]any {
	return map[string]any{
		"id": widgetID, "width": 1, "dashboard_id": 1, "text": text,
		"options": map[string]any{
			"position": map[string]any{"col": 0, "row": 8, "sizeX": 6, "sizeY": 3},
		},
		"created_at": "2026-09-08T08:48:20.247Z",
		"updated_at": "2026-09-08T08:48:20.247Z",
	}
}

// withDashboard registers a dashboard on the list endpoint and its detail
// under both the numeric id and the slug, the two keys Redash accepts across
// releases.
func (f *fakeRedash) withDashboard(dashboard map[string]any, widgets ...map[string]any) *fakeRedash {
	f.dashboards = append(f.dashboards, dashboard)

	detail := map[string]any{}
	for k, v := range dashboard {
		detail[k] = v
	}
	detail["widgets"] = widgets

	f.dashboardDetail[strconv.Itoa(dashboard["id"].(int))] = detail
	f.dashboardDetail[dashboard["slug"].(string)] = detail
	return f
}

func (f *fakeRedash) withQuery(query map[string]any) *fakeRedash {
	f.queries = append(f.queries, query)
	return f
}

func (f *fakeRedash) withArchivedQuery(query map[string]any) *fakeRedash {
	f.archivedQueries = append(f.archivedQueries, query)
	return f
}

// discoverAgainst runs a real discovery against the fake, with the given
// config overrides layered on top of the required fields.
func discoverAgainst(t *testing.T, fake *fakeRedash, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := tryDiscoverAgainst(t, fake, overrides)
	require.NoError(t, err)
	return result
}

func tryDiscoverAgainst(t *testing.T, fake *fakeRedash, overrides pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	t.Helper()

	config := pluginsdk.RawConfig{
		"host":    fake.start(t),
		"api_key": "test-key",
	}
	for key, value := range overrides {
		config[key] = value
	}

	source := &Source{}
	return source.Discover(t.Context(), config)
}

// assetNamed finds one asset by type and name.
func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	var found []string
	for _, asset := range result.Assets {
		found = append(found, asset.Type+" "+*asset.Name)
	}
	require.Failf(t, "asset not found", "no %s named %q, discovered: %v", assetType, name, found)
	return pluginsdk.Asset{}
}

func assetsOfType(result *pluginsdk.DiscoveryResult, assetType string) []pluginsdk.Asset {
	var out []pluginsdk.Asset
	for _, asset := range result.Assets {
		if asset.Type == assetType {
			out = append(out, asset)
		}
	}
	return out
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}
