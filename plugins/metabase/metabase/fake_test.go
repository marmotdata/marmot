package metabase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// fakeMetabase is a stand-in for a Metabase server. Tests register the
// collections, databases, cards and dashboards it should serve and
// then run a real discovery against it. The JSON shapes mirror what
// Metabase 0.63 returns, trimmed to the fields the plugin reads.
type fakeMetabase struct {
	apiKey   string
	username string
	password string

	collections []map[string]any
	// flatCollectionsFail makes /collection answer 500, the way Metabase
	// 0.63 does for API key principals, leaving only /collection/tree.
	flatCollectionsFail bool
	tree                []map[string]any

	databases []map[string]any
	// bareDatabaseList serves /database as a plain array instead of the
	// {"data": [...]} envelope current releases use.
	bareDatabaseList bool
	tables           map[int][]map[string]any

	cards      []map[string]any
	dashboards []map[string]any
	// orderedCards names the placement list ordered_cards, as Metabase
	// did before 0.48.
	orderedCards bool
	// failDashboardDetail makes /dashboard/{id} answer 500 for these ids.
	failDashboardDetail map[int]bool

	mu       sync.Mutex
	requests []string
}

func newFakeMetabase() *fakeMetabase {
	return &fakeMetabase{
		apiKey: "test-key",
		tables: make(map[int][]map[string]any),
		collections: []map[string]any{
			{"id": "root", "name": "Our analytics", "location": nil, "archived": nil},
		},
		failDashboardDetail: make(map[int]bool),
	}
}

func (f *fakeMetabase) withCollections(entries ...map[string]any) *fakeMetabase {
	f.collections = append(f.collections, entries...)
	return f
}

func (f *fakeMetabase) withDatabases(entries ...map[string]any) *fakeMetabase {
	f.databases = append(f.databases, entries...)
	return f
}

func (f *fakeMetabase) withTables(databaseID int, entries ...map[string]any) *fakeMetabase {
	f.tables[databaseID] = append(f.tables[databaseID], entries...)
	return f
}

func (f *fakeMetabase) withCards(entries ...map[string]any) *fakeMetabase {
	f.cards = append(f.cards, entries...)
	return f
}

func (f *fakeMetabase) withDashboards(entries ...map[string]any) *fakeMetabase {
	f.dashboards = append(f.dashboards, entries...)
	return f
}

func (f *fakeMetabase) start(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+path)
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && path == "/session" {
			var creds map[string]string
			_ = json.NewDecoder(r.Body).Decode(&creds)
			if f.username == "" || creds["username"] != f.username || creds["password"] != f.password {
				http.Error(w, `{"errors":{"password":"did not match stored password"}}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "session-1"})
			return
		}

		if !f.authorised(r) {
			http.Error(w, "Unauthenticated", http.StatusUnauthorized)
			return
		}

		switch {
		case path == "/collection":
			if f.flatCollectionsFail {
				http.Error(w, `{"via":[{"message":"Cannot invoke \"clojure.lang.IFn.invoke(Object)\" because \"this.personal_collection_ids\" is null"}]}`, http.StatusInternalServerError)
				return
			}
			writeJSON(w, f.collections)
		case path == "/collection/tree":
			writeJSON(w, f.tree)
		case path == "/database":
			if f.bareDatabaseList {
				writeJSON(w, f.databases)
				return
			}
			writeJSON(w, map[string]any{"data": f.databases, "total": len(f.databases)})
		case strings.HasPrefix(path, "/database/") && strings.HasSuffix(path, "/metadata"):
			id, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(path, "/database/"), "/metadata"))
			tables, ok := f.tables[id]
			if !ok {
				http.Error(w, "Not found.", http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]any{"id": id, "tables": tables})
		case path == "/card":
			writeJSON(w, filterArchived(f.cards, r.URL.Query().Get("f")))
		case path == "/dashboard":
			var list []map[string]any
			for _, d := range filterArchived(f.dashboards, r.URL.Query().Get("f")) {
				entry := make(map[string]any, len(d))
				for k, v := range d {
					if k != "dashcards" {
						entry[k] = v
					}
				}
				list = append(list, entry)
			}
			writeJSON(w, list)
		case strings.HasPrefix(path, "/dashboard/"):
			id, _ := strconv.Atoi(strings.TrimPrefix(path, "/dashboard/"))
			if f.failDashboardDetail[id] {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			for _, d := range f.dashboards {
				if d["id"] != id {
					continue
				}
				detail := make(map[string]any, len(d))
				for k, v := range d {
					detail[k] = v
				}
				if f.orderedCards {
					detail["ordered_cards"] = detail["dashcards"]
					delete(detail, "dashcards")
				}
				if archived, _ := d["archived"].(bool); archived {
					// Metabase reports the Trash as the collection of an
					// archived dashboard in the detail call only.
					detail["collection_id"] = 1
				}
				writeJSON(w, detail)
				return
			}
			http.Error(w, "Not found.", http.StatusNotFound)
		default:
			http.Error(w, "Not found.", http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)
	return server
}

func (f *fakeMetabase) authorised(r *http.Request) bool {
	if f.apiKey != "" {
		return r.Header.Get("X-API-KEY") == f.apiKey
	}
	if f.username != "" {
		return r.Header.Get("X-Metabase-Session") == "session-1"
	}
	return true
}

// requestCount reports how many requests hit a path below /api.
func (f *fakeMetabase) requestCount(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, req := range f.requests {
		if strings.HasSuffix(req, " "+path) {
			n++
		}
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}

// filterArchived applies Metabase's f query parameter: archived items
// only appear when f=archived, and then only they do.
func filterArchived(items []map[string]any, f string) []map[string]any {
	out := []map[string]any{}
	for _, item := range items {
		archived, _ := item["archived"].(bool)
		if archived == (f == "archived") {
			out = append(out, item)
		}
	}
	return out
}

// discover runs a full discovery against the fake server. Extra config
// keys override the defaults.
func discover(t *testing.T, f *fakeMetabase, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL, "api_key": f.apiKey}
	if f.apiKey == "" {
		delete(config, "api_key")
	}
	for key, value := range overrides {
		config[key] = value
	}

	result, err := (&Source{}).Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

func findAsset(result *pluginsdk.DiscoveryResult, mrnValue string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].MRN != nil && *result.Assets[i].MRN == mrnValue {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

// Fixture builders. Each mirrors one entry of the real Metabase 0.63
// response for its endpoint, trimmed to the fields the plugin reads.

func collectionEntry(id int, name, location string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "location": location, "archived": false,
		"personal_owner_id": nil, "slug": strings.ToLower(name),
	}
}

func databaseEntry(id int, name, engine string, details map[string]any) map[string]any {
	return map[string]any{
		"id": id, "name": name, "engine": engine, "details": details, "is_sample": false,
	}
}

func tableEntry(id int, schema, name string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "schema": schema, "display_name": name,
		"entity_type": "entity/GenericTable", "active": true,
		"fields": []map[string]any{{"name": "id", "base_type": "type/Integer"}},
	}
}

// cardEntry is the common shape of a /card entry. collectionID nil
// means the root collection, as Metabase reports it.
func cardEntry(id int, name string, collectionID any, cardType, display string, databaseID int, datasetQuery map[string]any) map[string]any {
	queryType := "query"
	if stages, ok := datasetQuery["stages"].([]map[string]any); ok && len(stages) > 0 && stages[0]["lib/type"] == "mbql.stage/native" {
		queryType = "native"
	}
	if datasetQuery["type"] == "native" {
		queryType = "native"
	}
	return map[string]any{
		"id": id, "name": name, "description": nil, "display": display, "type": cardType,
		"query_type": queryType, "collection_id": collectionID, "database_id": databaseID,
		"table_id": nil, "source_card_id": nil, "creator_id": 2, "archived": false,
		"created_at": "2026-09-07T21:10:31.876103Z", "updated_at": "2026-09-07T21:10:31.876103Z",
		"dataset_query": datasetQuery,
	}
}

// nativeCardQuery is the pMBQL shape of a native card's dataset_query on
// Metabase 0.57 and newer.
func nativeCardQuery(databaseID int, sql string) map[string]any {
	return map[string]any{
		"lib/type": "mbql/query", "database": databaseID,
		"stages": []map[string]any{{"lib/type": "mbql.stage/native", "native": sql}},
	}
}

// mbqlCardQuery is the pMBQL shape of a query builder card's dataset_query.
// Joins name their source in a nested stage.
func mbqlCardQuery(databaseID int, sourceTable int, joinTables ...int) map[string]any {
	stage := map[string]any{"lib/type": "mbql.stage/mbql", "source-table": sourceTable}
	if len(joinTables) > 0 {
		var joins []map[string]any
		for _, id := range joinTables {
			joins = append(joins, map[string]any{
				"lib/type": "mbql/join", "alias": "Join " + strconv.Itoa(id), "strategy": "left-join",
				"stages": []map[string]any{{"lib/type": "mbql.stage/mbql", "source-table": id}},
			})
		}
		stage["joins"] = joins
	}
	return map[string]any{"lib/type": "mbql/query", "database": databaseID, "stages": []map[string]any{stage}}
}

// mbqlCardQueryOnCard is a query builder card built on a saved question
// or model, pMBQL shape.
func mbqlCardQueryOnCard(databaseID, cardID int) map[string]any {
	return map[string]any{
		"lib/type": "mbql/query", "database": databaseID,
		"stages": []map[string]any{{"lib/type": "mbql.stage/mbql", "source-card": cardID}},
	}
}

// legacyNativeCardQuery is the pre-0.57 shape of a native dataset_query.
func legacyNativeCardQuery(databaseID int, sql string) map[string]any {
	return map[string]any{
		"database": databaseID, "type": "native",
		"native": map[string]any{"query": sql, "template-tags": map[string]any{}},
	}
}

// legacyMBQLCardQuery is the pre-0.57 shape of a query builder
// dataset_query. sourceTable is an int table id or "card__<id>".
func legacyMBQLCardQuery(databaseID int, sourceTable any, joinTables ...any) map[string]any {
	query := map[string]any{"source-table": sourceTable}
	if len(joinTables) > 0 {
		var joins []map[string]any
		for _, id := range joinTables {
			joins = append(joins, map[string]any{"alias": "j", "source-table": id, "strategy": "left-join"})
		}
		query["joins"] = joins
	}
	return map[string]any{"database": databaseID, "type": "query", "query": query}
}

func dashboardEntry(id int, name string, collectionID any, cardIDs ...int) map[string]any {
	dashcards := []map[string]any{}
	for i, cardID := range cardIDs {
		dashcards = append(dashcards, map[string]any{
			"id": 100 + i, "card_id": cardID, "dashboard_id": id, "row": 0, "col": i * 6, "size_x": 6, "size_y": 4,
			"card": map[string]any{"id": cardID},
		})
	}
	return map[string]any{
		"id": id, "name": name, "description": nil, "collection_id": collectionID,
		"creator_id": 2, "archived": false,
		"created_at": "2026-09-07T21:10:35.176551Z", "updated_at": "2026-09-07T21:10:35.176551Z",
		"dashcards": dashcards,
	}
}

// shopFixture is the fake equivalent of the Docker recipe: a Postgres
// database "Shop" with public.customers, public.orders and
// sales.regions, a Marmot collection with a Finance sub-collection, a
// native question, a query builder question, a model, a question on
// the model, and a dashboard holding the first three.
func shopFixture() *fakeMetabase {
	return newFakeMetabase().
		withCollections(
			collectionEntry(4, "Marmot", "/"),
			collectionEntry(5, "Finance", "/4/"),
		).
		withDatabases(databaseEntry(2, "Shop", "postgres", map[string]any{"host": "pg", "port": 5432, "dbname": "shop"})).
		withTables(2,
			tableEntry(9, "public", "customers"),
			tableEntry(10, "public", "orders"),
			tableEntry(12, "sales", "regions"),
		).
		withCards(
			cardEntry(40, "Revenue by customer", 5, "question", "bar", 2,
				nativeCardQuery(2, "SELECT c.name, sum(o.total) FROM public.customers c JOIN public.orders o ON o.customer_id = c.id [[WHERE o.status = {{status}}]] GROUP BY c.name")),
			cardEntry(41, "Orders table", 4, "question", "table", 2, mbqlCardQuery(2, 10)),
			cardEntry(42, "Customer orders", 5, "model", "table", 2,
				nativeCardQuery(2, "SELECT c.name, o.total FROM public.customers c JOIN public.orders o ON o.customer_id = c.id")),
			cardEntry(43, "Totals from model", 5, "question", "line", 2, mbqlCardQueryOnCard(2, 42)),
		).
		withDashboards(
			dashboardEntry(2, "Finance overview", 5, 40, 42, 43),
		)
}
