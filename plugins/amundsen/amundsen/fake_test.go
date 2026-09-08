package amundsen

import (
	"context"
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// fakeReader replays recorded Neo4j rows so the discovery passes can be
// tested without a graph to read. Rows are keyed by the query constant
// that produced them, and skip and limit are honoured so paging is
// exercised too.
//
// Every fixture below is a real record read from neo4j:5-community
// holding an Amundsen shaped graph, down to the types the Bolt driver
// hands back: int64 for integers, []any for a COLLECT, map[string]any
// for the maps inside one, and nil for a property a node does not have.
type fakeReader struct {
	rows  map[string][]map[string]any
	errs  map[string]error
	calls []fakeCall
}

type fakeCall struct {
	query string
	skip  int
	limit int
}

func newFakeReader() *fakeReader {
	return &fakeReader{
		rows: map[string][]map[string]any{},
		errs: map[string]error{},
	}
}

func (f *fakeReader) read(_ context.Context, cypher string, params map[string]any) ([]map[string]any, error) {
	skip, _ := params["skip"].(int)
	limit, _ := params["limit"].(int)
	f.calls = append(f.calls, fakeCall{query: cypher, skip: skip, limit: limit})

	if err := f.errs[cypher]; err != nil {
		return nil, err
	}

	rows := f.rows[cypher]
	if skip >= len(rows) {
		return nil, nil
	}
	end := skip + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[skip:end], nil
}

// callsFor counts how many times one query was read.
func (f *fakeReader) callsFor(query string) int {
	count := 0
	for _, call := range f.calls {
		if call.query == query {
			count++
		}
	}
	return count
}

// seededGraph is the fake wired up with the whole seeded graph: two
// Postgres tables, a Hive table and a Hive view, owners, usage, table
// lineage and a Superset dashboard with two charts.
func seededGraph() *fakeReader {
	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{hiveOrdersRow(), hiveOrdersViewRow(), customersRow(), ordersRow()}
	f.rows[tableUsageQuery] = []map[string]any{
		{"key": "postgres://prod.public/customers", "total_usage": int64(5), "unique_usage": int64(1)},
		{"key": "postgres://prod.public/orders", "total_usage": int64(42), "unique_usage": int64(2)},
	}
	f.rows[tableOwnerQuery] = []map[string]any{
		{"table_key": "postgres://prod.public/customers", "email": "ann@marmot.test", "full_name": "Ann Ops", "team": "Data Platform"},
		{"table_key": "postgres://prod.public/orders", "email": "ann@marmot.test", "full_name": "Ann Ops", "team": "Data Platform"},
		{"table_key": "postgres://prod.public/orders", "email": "bo@marmot.test", "full_name": "Bo Analyst", "team": "Finance"},
	}
	f.rows[tableLineageQuery] = []map[string]any{
		{"downstream": "postgres://prod.public/orders", "upstream": "postgres://prod.public/customers"},
	}
	f.rows[dashboardQuery] = []map[string]any{revenueDashboardRow()}
	f.rows[dashboardUsageQuery] = []map[string]any{
		{"key": "superset_dashboard://prod.finance/revenue", "total_usage": int64(7), "unique_usage": int64(1)},
	}
	f.rows[dashboardTableQuery] = []map[string]any{
		{"dashboard_key": "superset_dashboard://prod.finance/revenue", "table_key": "postgres://prod.public/orders"},
	}
	return f
}

// ordersRow is postgres://prod.public/orders. Its columns arrive in the
// order a COLLECT happened to return them, which is not sort order.
func ordersRow() map[string]any {
	return map[string]any{
		"database":               "postgres",
		"cluster":                "prod",
		"schema":                 "public",
		"schema_description":     "Customer facing tables",
		"name":                   "orders",
		"key":                    "postgres://prod.public/orders",
		"is_view":                false,
		"description":            "One row per placed order",
		"last_updated_timestamp": int64(1757000000),
		"columns": []any{
			map[string]any{"name": "total", "description": nil, "type": "numeric", "sort_order": int64(2)},
			map[string]any{"name": "customer_id", "description": "References customers.id", "type": "integer", "sort_order": int64(1)},
			map[string]any{"name": "id", "description": "Surrogate order key", "type": "integer", "sort_order": int64(0)},
		},
		"tags":                      []any{"finance", "pii"},
		"badges":                    []any{"certified"},
		"programmatic_descriptions": []any{"Built nightly by dbt model orders"},
	}
}

func customersRow() map[string]any {
	return map[string]any{
		"database":               "postgres",
		"cluster":                "prod",
		"schema":                 "public",
		"schema_description":     "Customer facing tables",
		"name":                   "customers",
		"key":                    "postgres://prod.public/customers",
		"is_view":                false,
		"description":            "One row per customer",
		"last_updated_timestamp": nil,
		"columns": []any{
			map[string]any{"name": "email", "description": nil, "type": "varchar", "sort_order": int64(1)},
			map[string]any{"name": "id", "description": nil, "type": "integer", "sort_order": int64(0)},
		},
		"tags":                      []any{},
		"badges":                    []any{},
		"programmatic_descriptions": []any{},
	}
}

func hiveOrdersRow() map[string]any {
	return map[string]any{
		"database":               "hive",
		"cluster":                "gold",
		"schema":                 "sales",
		"schema_description":     nil,
		"name":                   "orders",
		"key":                    "hive://gold.sales/orders",
		"is_view":                false,
		"description":            nil,
		"last_updated_timestamp": nil,
		"columns": []any{
			map[string]any{"name": "order_id", "description": nil, "type": "bigint", "sort_order": int64(0)},
		},
		"tags":                      []any{},
		"badges":                    []any{},
		"programmatic_descriptions": []any{},
	}
}

func hiveOrdersViewRow() map[string]any {
	return map[string]any{
		"database":               "hive",
		"cluster":                "gold",
		"schema":                 "sales",
		"schema_description":     nil,
		"name":                   "orders_view",
		"key":                    "hive://gold.sales/orders_view",
		"is_view":                true,
		"description":            nil,
		"last_updated_timestamp": nil,
		"columns": []any{
			map[string]any{"name": "order_id", "description": nil, "type": "bigint", "sort_order": int64(0)},
		},
		"tags":                      []any{},
		"badges":                    []any{},
		"programmatic_descriptions": []any{},
	}
}

// emptyTableRow is a table with no columns at all. Neo4j answers the
// COLLECT with one map made entirely of nulls.
func emptyTableRow() map[string]any {
	return map[string]any{
		"database": "postgres",
		"cluster":  "prod",
		"schema":   "public",
		"name":     "empty_tbl",
		"key":      "postgres://prod.public/empty_tbl",
		"is_view":  nil,
		"columns": []any{
			map[string]any{"name": nil, "description": nil, "type": nil, "sort_order": nil},
		},
		"tags":                      []any{},
		"badges":                    []any{},
		"programmatic_descriptions": []any{},
	}
}

func revenueDashboardRow() map[string]any {
	return map[string]any{
		"group_name":                    "finance",
		"name":                          "revenue",
		"cluster":                       "prod",
		"description":                   "Monthly revenue by region",
		"group_description":             "Finance reporting",
		"group_url":                     "https://superset.marmot.test/dashboard/group/finance",
		"url":                           "https://superset.marmot.test/dashboard/revenue",
		"key":                           "superset_dashboard://prod.finance/revenue",
		"product":                       "superset",
		"last_successful_run_timestamp": int64(1757001234),
		"query_names":                   []any{"revenue_by_region"},
		"charts": []any{
			map[string]any{"name": "Revenue by region", "id": "c1", "type": "bar", "url": "https://superset.marmot.test/chart/c1"},
			map[string]any{"name": "Revenue trend", "id": "c2", "type": "line", "url": "https://superset.marmot.test/chart/c2"},
		},
		"tags":   []any{"finance"},
		"badges": []any{"verified"},
	}
}

// testConfig is a run with everything switched on, which is the default.
func testConfig() *Config {
	return &Config{
		URI:                 "bolt://localhost:7687",
		Username:            "neo4j",
		Password:            "secret",
		Database:            "neo4j",
		IncludeUsers:        true,
		IncludeDashboards:   true,
		IncludeTags:         true,
		IncludeDescriptions: true,
		IncludeUsage:        true,
		QueryTimeoutSeconds: 120,
		PageSize:            100,
	}
}

func discover(t *testing.T, config *Config, r reader) *collector {
	t.Helper()

	c := newCollector(config)
	require.NoError(t, c.collect(t.Context(), r))
	return c
}

// assetNamed is the one asset with this name, failing the test when
// there is not exactly one.
func assetNamed(t *testing.T, c *collector, assetType, name string) pluginsdk.Asset {
	t.Helper()

	var found []pluginsdk.Asset
	for _, asset := range c.assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			found = append(found, asset)
		}
	}
	require.Len(t, found, 1, "expected exactly one %s named %q", assetType, name)
	return found[0]
}

// columnsOf decodes the column list an asset carries in its schema.
func columnsOf(t *testing.T, asset pluginsdk.Asset) []map[string]any {
	t.Helper()

	encoded, ok := asset.Schema["columns"]
	require.True(t, ok, "asset has no columns")

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(encoded), &columns))
	return columns
}

// provenanceOf is the amundsen sub-map every asset carries.
func provenanceOf(t *testing.T, asset pluginsdk.Asset) map[string]any {
	t.Helper()

	provenance, ok := asset.Metadata["amundsen"].(map[string]any)
	require.True(t, ok, "asset has no amundsen metadata")
	return provenance
}

// statisticFor is the value of one metric for one asset.
func statisticFor(c *collector, assetMRN, metric string) (float64, bool) {
	for _, statistic := range c.statistics {
		if statistic.AssetMRN == assetMRN && statistic.MetricName == metric {
			return statistic.Value, true
		}
	}
	return 0, false
}

// hasEdge reports whether the run produced this exact lineage edge.
func hasEdge(c *collector, source, target, edgeType string) bool {
	for _, edge := range c.lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}
