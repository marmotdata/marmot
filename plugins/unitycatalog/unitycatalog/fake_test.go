package unitycatalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// fakeUC is a stand-in for a Unity Catalog server. Tests register the
// objects each list endpoint should return and then run a real discovery
// against it. Objects are raw JSON so the fixtures can be the responses
// observed from the real server, verbatim.
type fakeUC struct {
	mu sync.Mutex

	catalogs  []json.RawMessage
	schemas   map[string][]json.RawMessage // catalog -> schemas
	tables    map[string][]json.RawMessage // catalog.schema -> tables
	volumes   map[string][]json.RawMessage
	functions map[string][]json.RawMessage
	models    map[string][]json.RawMessage
	versions  map[string][]json.RawMessage // catalog.schema.model -> versions
	// singles holds full table bodies served by GET /tables/<full name>.
	singles map[string]json.RawMessage
	// missing makes an endpoint answer 404, the way a server without
	// that feature does.
	missing map[string]bool
	// status forces one status code on every request, to simulate a
	// broken server.
	status int

	requests []*http.Request
}

func newFakeUC() *fakeUC {
	return &fakeUC{
		schemas:   make(map[string][]json.RawMessage),
		tables:    make(map[string][]json.RawMessage),
		volumes:   make(map[string][]json.RawMessage),
		functions: make(map[string][]json.RawMessage),
		models:    make(map[string][]json.RawMessage),
		versions:  make(map[string][]json.RawMessage),
		singles:   make(map[string]json.RawMessage),
		missing:   make(map[string]bool),
	}
}

func (f *fakeUC) withCatalog(body string) *fakeUC {
	f.catalogs = append(f.catalogs, json.RawMessage(body))
	return f
}

func (f *fakeUC) withSchema(catalog, body string) *fakeUC {
	f.schemas[catalog] = append(f.schemas[catalog], json.RawMessage(body))
	return f
}

func (f *fakeUC) withTable(catalog, schema, body string) *fakeUC {
	key := catalog + "." + schema
	f.tables[key] = append(f.tables[key], json.RawMessage(body))
	return f
}

func (f *fakeUC) withSingleTable(fullName, body string) *fakeUC {
	f.singles[fullName] = json.RawMessage(body)
	return f
}

func (f *fakeUC) withVolume(catalog, schema, body string) *fakeUC {
	key := catalog + "." + schema
	f.volumes[key] = append(f.volumes[key], json.RawMessage(body))
	return f
}

func (f *fakeUC) withFunction(catalog, schema, body string) *fakeUC {
	key := catalog + "." + schema
	f.functions[key] = append(f.functions[key], json.RawMessage(body))
	return f
}

func (f *fakeUC) withModel(catalog, schema, body string) *fakeUC {
	key := catalog + "." + schema
	f.models[key] = append(f.models[key], json.RawMessage(body))
	return f
}

func (f *fakeUC) withVersions(fullName string, bodies ...string) *fakeUC {
	for _, body := range bodies {
		f.versions[fullName] = append(f.versions[fullName], json.RawMessage(body))
	}
	return f
}

func (f *fakeUC) without(paths ...string) *fakeUC {
	for _, path := range paths {
		f.missing[path] = true
	}
	return f
}

func (f *fakeUC) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r)
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error_code":"INTERNAL","message":"boom"}`))
			return
		}

		path := strings.TrimPrefix(r.URL.Path, apiPrefix)
		q := r.URL.Query()
		catalog := q.Get("catalog_name")
		schema := q.Get("schema_name")
		key := catalog + "." + schema

		if f.missing[path] {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error_code":"NOT_FOUND","message":"not found"}`))
			return
		}

		switch {
		case path == "/catalogs":
			f.writePage(w, r, "catalogs", f.catalogs)
		case path == "/schemas":
			f.writePage(w, r, "schemas", f.schemas[catalog])
		case path == "/tables":
			f.writePage(w, r, "tables", f.tables[key])
		case strings.HasPrefix(path, "/tables/"):
			body, ok := f.singles[strings.TrimPrefix(path, "/tables/")]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error_code":"TABLE_NOT_FOUND","message":"Table not found"}`))
				return
			}
			_, _ = w.Write(body)
		case path == "/volumes":
			f.writePage(w, r, "volumes", f.volumes[key])
		case path == "/functions":
			f.writePage(w, r, "functions", f.functions[key])
		case path == "/models":
			f.writePage(w, r, "registered_models", f.models[key])
		case strings.HasPrefix(path, "/models/") && strings.HasSuffix(path, "/versions"):
			name := strings.TrimSuffix(strings.TrimPrefix(path, "/models/"), "/versions")
			versions, ok := f.versions[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error_code":"NOT_FOUND","message":"Registered model not found"}`))
				return
			}
			f.writePage(w, r, "model_versions", versions)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error_code":"NOT_FOUND","message":"no such route"}`))
		}
	}))

	t.Cleanup(server.Close)
	return server
}

// writePage serves one page of a listing. The fake pages by offset: the
// token is the index of the next item, which the client must treat as
// opaque. The real server pages by last name; both end with a null token.
func (f *fakeUC) writePage(w http.ResponseWriter, r *http.Request, key string, items []json.RawMessage) {
	q := r.URL.Query()
	pageSize := len(items)
	if raw := q.Get("max_results"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			pageSize = n
		}
	}
	offset := 0
	if raw := q.Get("page_token"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			offset = n
		}
	}

	end := offset + pageSize
	if end > len(items) || pageSize == 0 {
		end = len(items)
	}
	if offset > end {
		offset = end
	}

	page := items[offset:end]
	if page == nil {
		page = []json.RawMessage{}
	}

	var next any
	if end < len(items) {
		next = strconv.Itoa(end)
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		key:               page,
		"next_page_token": next,
	})
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeUC, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL}
	for key, value := range overrides {
		config[key] = value
	}

	source := &Source{}
	result, err := source.Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

// findAsset looks an asset up by type and name, through the MRN the
// server would rebuild from those parts.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	want := mrn.New(assetType, provider, name)
	for i := range result.Assets {
		if result.Assets[i].MRN != nil && *result.Assets[i].MRN == want {
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

// columnsOf decodes an asset's column list, keyed by column name.
func columnsOf(t *testing.T, a *pluginsdk.Asset) map[string]map[string]any {
	t.Helper()

	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column list on %s", *a.Name)

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &columns))

	byName := make(map[string]map[string]any, len(columns))
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

// A small world used by most tests: the sample objects the OSS image
// ships with, plus a second catalog holding a view and cloud-backed
// tables.
func sampleWorld() *fakeUC {
	return newFakeUC().
		withCatalog(unityCatalogJSON).
		withCatalog(systemCatalogJSON).
		withSchema("unity", `{"name":"default","catalog_name":"unity","comment":"Default schema","full_name":"unity.default"}`).
		withTable("unity", "default", numbersTableJSON).
		withTable("unity", "default", userCountriesTableJSON).
		withVolume("unity", "default", txtFilesVolumeJSON).
		withFunction("unity", "default", sumFunctionJSON).
		withCatalog(`{"name":"shop","comment":"Web shop data","properties":{"team":"commerce"},"owner":null,"created_at":1788835315418,"updated_at":1788835315418,"id":"1bad8c6f-992d-441e-92c4-9a392aeed280"}`).
		withSchema("shop", `{"name":"sales","catalog_name":"shop","comment":"Sales facts","full_name":"shop.sales"}`).
		withTable("shop", "sales", customersTableJSON).
		withTable("shop", "sales", ordersTableJSON).
		withTable("shop", "sales", bigOrdersViewJSON)
}
