package grafana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// The fixtures below are trimmed copies of what Grafana 13.2.1 returned
// for the dashboards and data sources the e2e tests seed.

const ordersOverviewJSON = `{
  "meta": {
    "type": "db", "canSave": true, "canEdit": true, "canAdmin": true, "canStar": true, "canDelete": true,
    "slug": "orders-overview",
    "url": "/d/orders-overview/orders-overview",
    "expires": "0001-01-01T00:00:00Z",
    "created": "2026-09-07T21:10:31Z",
    "updated": "2026-09-07T21:10:31Z",
    "updatedBy": "Anonymous",
    "createdBy": "Anonymous",
    "version": 1,
    "hasAcl": false, "isFolder": false, "apiVersion": "v0alpha1",
    "folderId": 2896197908852736,
    "folderUid": "dfxkla5oojcw0c",
    "folderTitle": "Sales",
    "folderUrl": "/dashboards/f/dfxkla5oojcw0c/sales",
    "provisioned": false,
    "provisionedExternalId": ""
  },
  "dashboard": {
    "description": "Revenue and top customers",
    "id": 2896204983521280,
    "panels": [
      {
        "datasource": {"type": "grafana-postgresql-datasource", "uid": "ffxkla6ojkvlsf"},
        "description": "Daily revenue",
        "id": 1,
        "targets": [
          {"format": "table", "rawSql": "SELECT o.created_at AS time, sum(o.total) FROM public.orders o JOIN public.customers c ON c.id = o.customer_id WHERE $__timeFilter(o.created_at) GROUP BY 1", "refId": "A"}
        ],
        "title": "Revenue",
        "type": "timeseries"
      },
      {
        "collapsed": true,
        "id": 2,
        "panels": [
          {
            "datasource": {"type": "grafana-postgresql-datasource", "uid": "ffxkla6ojkvlsf"},
            "id": 3,
            "targets": [{"format": "table", "rawSql": "SELECT * FROM customers", "refId": "A"}],
            "title": "Top customers",
            "type": "table"
          }
        ],
        "title": "Details",
        "type": "row"
      },
      {"id": 4, "options": {"content": "hello"}, "title": "Notes", "type": "text"},
      {
        "datasource": {"type": "grafana-testdata-datasource", "uid": "efxkla7l5962oe"},
        "id": 5,
        "targets": [{"refId": "A", "scenarioId": "random_walk"}],
        "title": "Random",
        "type": "stat"
      }
    ],
    "refresh": "5m",
    "tags": ["sales"],
    "time": {"from": "now-7d", "to": "now"},
    "title": "Orders overview",
    "uid": "orders-overview",
    "version": 1
  }
}`

const opsHomeJSON = `{
  "meta": {
    "type": "db",
    "slug": "ops-home",
    "url": "/d/ops-home/ops-home",
    "created": "2026-09-07T21:10:32Z",
    "updated": "2026-09-07T21:10:32Z",
    "updatedBy": "Anonymous",
    "createdBy": "Anonymous",
    "version": 1,
    "folderId": 0,
    "folderUid": "",
    "folderTitle": "General",
    "folderUrl": "",
    "provisioned": false
  },
  "dashboard": {
    "id": 2896205918851072,
    "panels": [
      {
        "datasource": {"type": "grafana-testdata-datasource", "uid": "efxkla7l5962oe"},
        "id": 1,
        "targets": [{"refId": "A", "scenarioId": "random_walk"}],
        "title": "",
        "type": "gauge"
      },
      {
        "id": 2,
        "targets": [{"format": "table", "rawSql": "SELECT name, total FROM public.customer_totals WHERE name = '${customer}' ORDER BY total DESC", "refId": "A"}],
        "title": "Customer totals",
        "type": "table"
      }
    ],
    "tags": [],
    "title": "Ops home",
    "uid": "ops-home",
    "version": 1
  }
}`

const datasourcesJSON = `[
  {
    "id": 1, "uid": "ffxkla6ojkvlsf", "orgId": 1, "name": "Shop",
    "type": "grafana-postgresql-datasource", "typeName": "PostgreSQL",
    "typeLogoUrl": "public/plugins/grafana-postgresql-datasource/img/postgresql_logo.svg",
    "access": "proxy", "url": "marmot-test-grafana-pg:5432", "user": "marmot", "database": "shop",
    "basicAuth": false, "isDefault": true,
    "jsonData": {"database": "shop", "sslmode": "disable"},
    "readOnly": false
  },
  {
    "id": 2, "uid": "efxkla7l5962oe", "orgId": 1, "name": "TestData",
    "type": "grafana-testdata-datasource", "typeName": "TestData",
    "typeLogoUrl": "public/plugins/grafana-testdata-datasource/img/testdata.svg",
    "access": "proxy", "url": "", "user": "", "database": "",
    "basicAuth": false, "isDefault": false, "jsonData": {}, "readOnly": false
  }
]`

const fakeToken = "glsa_test_token"

// fakeGrafana is a stand-in for a Grafana server. Tests register the
// dashboards and data sources it should serve and run a real discovery
// against it.
type fakeGrafana struct {
	dashboards  []map[string]any
	datasources []map[string]any
	library     map[string]map[string]any
	unreadable  map[string]bool
	dsStatus    int
	requests    []string
}

func newFakeGrafana() *fakeGrafana {
	return &fakeGrafana{
		library:    make(map[string]map[string]any),
		unreadable: make(map[string]bool),
		dsStatus:   http.StatusOK,
	}
}

// withJSON registers dashboards and data sources from raw fixtures.
func (f *fakeGrafana) withJSON(dashboards []string, datasources string) *fakeGrafana {
	for _, doc := range dashboards {
		f.withDashboard(parseJSON(doc))
	}
	if datasources != "" {
		var list []map[string]any
		if err := json.Unmarshal([]byte(datasources), &list); err != nil {
			panic(err)
		}
		f.datasources = append(f.datasources, list...)
	}
	return f
}

// withDashboard registers one dashboard document ({dashboard, meta}).
func (f *fakeGrafana) withDashboard(doc map[string]any) *fakeGrafana {
	f.dashboards = append(f.dashboards, doc)
	return f
}

// withDatasource registers one data source.
func (f *fakeGrafana) withDatasource(ds map[string]any) *fakeGrafana {
	f.datasources = append(f.datasources, ds)
	return f
}

// withLibraryPanel registers the shared panel behind a library
// reference.
func (f *fakeGrafana) withLibraryPanel(uid string, model map[string]any) *fakeGrafana {
	f.library[uid] = model
	return f
}

// broken makes a dashboard appear in search but fail to load.
func (f *fakeGrafana) broken(uid string) *fakeGrafana {
	f.unreadable[uid] = true
	return f
}

// withoutDatasourceAccess makes the data source listing answer the way
// Grafana does for a token that lacks datasources:read.
func (f *fakeGrafana) withoutDatasourceAccess() *fakeGrafana {
	f.dsStatus = http.StatusForbidden
	return f
}

func (f *fakeGrafana) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")

		if r.Header.Get("Authorization") != "Bearer "+fakeToken {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"message": "Invalid API key", "messageId": "api-key.invalid", "statusCode": 401,
			})
			return
		}

		path := r.URL.Path
		switch {
		case path == "/api/search":
			f.serveSearch(w, r)
		case strings.HasPrefix(path, "/api/dashboards/uid/"):
			f.serveDashboard(w, strings.TrimPrefix(path, "/api/dashboards/uid/"))
		case path == "/api/datasources":
			if f.dsStatus != http.StatusOK {
				writeJSON(w, f.dsStatus, map[string]any{
					"message": "You'll need additional permissions to perform this action. Permissions needed: datasources:read",
				})
				return
			}
			list := f.datasources
			if list == nil {
				list = []map[string]any{}
			}
			writeJSON(w, http.StatusOK, list)
		case strings.HasPrefix(path, "/api/library-elements/"):
			uid := strings.TrimPrefix(path, "/api/library-elements/")
			model, ok := f.library[uid]
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "library element could not be found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"result": map[string]any{"uid": uid, "model": model}})
		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Not found"})
		}
	}))

	t.Cleanup(server.Close)
	return server
}

func (f *fakeGrafana) serveSearch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("type") != "dash-db" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "unexpected search type"})
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 || limit < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "bad page or limit"})
		return
	}

	hits := []map[string]any{}
	start := (page - 1) * limit
	for i := start; i < len(f.dashboards) && i < start+limit; i++ {
		hits = append(hits, searchHitFor(f.dashboards[i]))
	}
	writeJSON(w, http.StatusOK, hits)
}

func (f *fakeGrafana) serveDashboard(w http.ResponseWriter, uid string) {
	if f.unreadable[uid] {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "Dashboard could not be loaded"})
		return
	}
	for _, doc := range f.dashboards {
		if dashboardOf(doc)["uid"] == uid {
			writeJSON(w, http.StatusOK, doc)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"message": "Dashboard not found"})
}

// searchHitFor builds the /api/search entry Grafana lists for a
// dashboard document.
func searchHitFor(doc map[string]any) map[string]any {
	dash := dashboardOf(doc)
	meta, _ := doc["meta"].(map[string]any)

	hit := map[string]any{
		"id": dash["id"], "uid": dash["uid"], "orgId": 1, "title": dash["title"],
		"uri": "db/" + fmt.Sprint(meta["slug"]), "url": meta["url"], "slug": "", "type": "dash-db",
		"tags": dash["tags"], "isStarred": false, "sortMeta": 0, "isDeleted": false,
	}
	if uid, _ := meta["folderUid"].(string); uid != "" {
		hit["folderId"] = meta["folderId"]
		hit["folderUid"] = uid
		hit["folderTitle"] = meta["folderTitle"]
		hit["folderUrl"] = meta["folderUrl"]
	}
	return hit
}

func dashboardOf(doc map[string]any) map[string]any {
	dash, _ := doc["dashboard"].(map[string]any)
	return dash
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func parseJSON(doc string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		panic(err)
	}
	return m
}

// seeded is a fake serving the same content the e2e tests seed into a
// real Grafana.
func seeded() *fakeGrafana {
	return newFakeGrafana().withJSON([]string{opsHomeJSON, ordersOverviewJSON}, datasourcesJSON)
}

// dashboardDoc builds a dashboard document in the General folder.
func dashboardDoc(uid, title string, panels ...map[string]any) map[string]any {
	if panels == nil {
		panels = []map[string]any{}
	}
	return map[string]any{
		"meta": map[string]any{
			"slug": uid, "url": "/d/" + uid + "/" + uid,
			"created": "2026-09-07T21:10:32Z", "updated": "2026-09-07T21:10:32Z",
			"createdBy": "admin", "updatedBy": "admin", "version": 1,
			"folderId": 0, "folderUid": "", "folderTitle": "General", "folderUrl": "", "provisioned": false,
		},
		"dashboard": map[string]any{
			"id": 100, "uid": uid, "title": title, "tags": []string{}, "panels": panels, "version": 1,
		},
	}
}

// inFolder moves a dashboard document into a folder.
func inFolder(doc map[string]any, folderUID, folderTitle string) map[string]any {
	meta := doc["meta"].(map[string]any)
	meta["folderId"] = 7
	meta["folderUid"] = folderUID
	meta["folderTitle"] = folderTitle
	meta["folderUrl"] = "/dashboards/f/" + folderUID + "/" + strings.ToLower(folderTitle)
	return doc
}

// sqlPanel builds a panel running one SQL query against a data source.
func sqlPanel(id int, panelType, title string, datasource any, sql string) map[string]any {
	return map[string]any{
		"id": id, "type": panelType, "title": title, "datasource": datasource,
		"targets": []map[string]any{{"refId": "A", "format": "table", "rawSql": sql}},
	}
}

func dsRef(dsType, uid string) map[string]any {
	return map[string]any{"type": dsType, "uid": uid}
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeGrafana, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL, "api_key": fakeToken}
	for key, value := range overrides {
		config[key] = value
	}

	source := &Source{}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

// findAsset returns the asset of a type with the given name.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
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

// countRequests counts the requests the fake served for a path prefix.
func (f *fakeGrafana) countRequests(prefix string) int {
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}
