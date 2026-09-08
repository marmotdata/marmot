package cassandra_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Cassandra reachable at
// MARMOT_TEST_CASSANDRA_HOST (host:port). They seed a small shop keyspace
// themselves, so the only requirement is a cluster with materialized views
// enabled.

const hostEnv = "MARMOT_TEST_CASSANDRA_HOST"

func cassandraHost(t *testing.T) string {
	t.Helper()
	host := os.Getenv(hostEnv)
	if host == "" {
		t.Skipf("%s not set, skipping Cassandra e2e tests", hostEnv)
	}
	return host
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func e2eConfig(host string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"hosts":     []interface{}{host},
		"keyspaces": []interface{}{"shop"},
	}
}

// seedStatements build the shop keyspace: a user type, a table with a
// map and a frozen UDT, a table with a clustering key, collections, a
// static column, a comment and a secondary index, a counter table, and a
// materialized view. Everything is IF NOT EXISTS so reruns are harmless.
var seedStatements = []string{
	`CREATE KEYSPACE IF NOT EXISTS shop WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}`,
	`CREATE TYPE IF NOT EXISTS shop.address (street text, city text, postcode text)`,
	`CREATE TABLE IF NOT EXISTS shop.customers (
		id uuid PRIMARY KEY,
		name text,
		email text,
		attributes map<text, text>,
		home frozen<address>,
		created_at timestamp
	)`,
	`CREATE TABLE IF NOT EXISTS shop.orders (
		customer_id uuid,
		order_id timeuuid,
		status text,
		total decimal,
		items list<text>,
		tags set<text>,
		loyalty_tier text STATIC,
		placed_at timestamp,
		PRIMARY KEY (customer_id, order_id)
	) WITH CLUSTERING ORDER BY (order_id DESC)
	  AND comment = 'Customer orders'
	  AND default_time_to_live = 0`,
	`CREATE INDEX IF NOT EXISTS orders_status_idx ON shop.orders (status)`,
	`CREATE TABLE IF NOT EXISTS shop.page_views (page text PRIMARY KEY, views counter)`,
	`CREATE MATERIALIZED VIEW IF NOT EXISTS shop.orders_by_status AS
		SELECT customer_id, order_id, status, total, placed_at FROM shop.orders
		WHERE status IS NOT NULL AND customer_id IS NOT NULL AND order_id IS NOT NULL
		PRIMARY KEY (status, customer_id, order_id)`,
	`INSERT INTO shop.customers (id, name, email, attributes, home, created_at)
		VALUES (11111111-1111-1111-1111-111111111111, 'Alice', 'alice@example.com', {'plan': 'gold'},
		        {street: '1 Main St', city: 'London', postcode: 'N1'}, '2026-01-02T03:04:05Z')`,
	`INSERT INTO shop.customers (id, name, email, created_at)
		VALUES (22222222-2222-2222-2222-222222222222, 'Bob', 'bob@example.com', '2026-02-03T04:05:06Z')`,
	`INSERT INTO shop.orders (customer_id, order_id, status, total, items, tags, loyalty_tier, placed_at)
		VALUES (11111111-1111-1111-1111-111111111111, 6ba7b810-9dad-11d1-80b4-00c04fd430c8, 'shipped', 42.50,
		        ['book', 'pen'], {'gift'}, 'gold', '2026-03-04T05:06:07Z')`,
	`INSERT INTO shop.orders (customer_id, order_id, status, total, items, tags, placed_at)
		VALUES (11111111-1111-1111-1111-111111111111, 6ba7b811-9dad-11d1-80b4-00c04fd430c8, 'pending', 7.25,
		        ['mug'], {'promo', 'bulk'}, '2026-03-05T05:06:07Z')`,
	`UPDATE shop.page_views SET views = views + 1 WHERE page = '/home'`,
}

func seed(host string) error {
	cluster := gocql.NewCluster(host)
	cluster.ConnectTimeout = 30 * time.Second
	cluster.Timeout = 30 * time.Second
	cluster.Consistency = gocql.LocalOne

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", host, err)
	}
	defer session.Close()

	for _, stmt := range seedStatements {
		if err := session.Query(stmt).Exec(); err != nil {
			return fmt.Errorf("seeding: %w\n%s", err, stmt)
		}
	}
	return nil
}

// Discovery is run once and shared: every test below reads the same
// result, and one process spawn per test would only repeat the same work.
var (
	discoverOnce   sync.Once
	discoverResult *pluginsdk.DiscoveryResult
	discoverErr    error
)

func discoverShop(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()
	host := cassandraHost(t)

	discoverOnce.Do(func() {
		if err := seed(host); err != nil {
			discoverErr = err
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		discoverResult, discoverErr = buildBinary(t).Discover(ctx, e2eConfig(host))
	})

	require.NoError(t, discoverErr)
	require.NotNil(t, discoverResult)
	return discoverResult
}

func findAsset(result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].Name != nil && *result.Assets[i].Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func requireAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) *pluginsdk.Asset {
	t.Helper()
	a := findAsset(result, name)
	require.NotNilf(t, a, "expected asset %s", name)
	return a
}

// stringList reads a list metadata value whichever way it crossed the wire.
func stringList(v interface{}) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, fmt.Sprint(e))
		}
		return out
	}
	return nil
}

func decodeColumns(t *testing.T, a *pluginsdk.Asset) map[string]map[string]interface{} {
	t.Helper()
	raw, ok := a.Schema["columns"]
	require.Truef(t, ok, "expected a column schema on %s", *a.Name)

	var cols []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &cols))

	byName := make(map[string]map[string]interface{}, len(cols))
	for _, c := range cols {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func columnOrder(t *testing.T, a *pluginsdk.Asset) []string {
	t.Helper()
	var cols []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(a.Schema["columns"]), &cols))

	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c["column_name"].(string))
	}
	return names
}

func TestE2E_Meta(t *testing.T) {
	cassandraHost(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "cassandra", meta.ID)
	assert.Equal(t, "Apache Cassandra", meta.Name)
	assert.Equal(t, "database", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.True(t, meta.SupportsDataPreview, "Serve marks a DataFetcher as previewable")
}

func TestE2E_ValidateMissingHostsFails(t *testing.T) {
	cassandraHost(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hosts")
}

func TestE2E_ValidateAcceptsAReachableConfig(t *testing.T) {
	host := cassandraHost(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), e2eConfig(host))
	require.NoError(t, err)
}

func TestE2E_DiscoverFindsKeyspaceTablesAndView(t *testing.T) {
	result := discoverShop(t)

	types := make(map[string]string)
	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		types[*a.Name] = a.Type
		assert.Equal(t, []string{"Cassandra"}, a.Providers)
	}

	assert.Equal(t, "Keyspace", types["shop"])
	assert.Equal(t, "Table", types["shop.customers"])
	assert.Equal(t, "Table", types["shop.orders"])
	assert.Equal(t, "Table", types["shop.page_views"])
	assert.Equal(t, "View", types["shop.orders_by_status"])
	assert.Len(t, result.Assets, 5, "only the configured keyspace is discovered")
}

func TestE2E_KeyspaceAssetCarriesReplicationAndUserTypes(t *testing.T) {
	ks := requireAsset(t, discoverShop(t), "shop")

	assert.Equal(t, "mrn://keyspace/cassandra/shop", *ks.MRN)
	assert.Equal(t, "SimpleStrategy", ks.Metadata["replication_class"])
	assert.EqualValues(t, 1, ks.Metadata["replication_factor"])
	assert.Equal(t, true, ks.Metadata["durable_writes"])
	assert.EqualValues(t, 3, ks.Metadata["table_count"])
	assert.EqualValues(t, 1, ks.Metadata["view_count"])
	assert.Equal(t, []string{"address"}, stringList(ks.Metadata["user_types"]))
	assert.NotEmpty(t, ks.Metadata["cluster_name"])
	assert.NotEmpty(t, ks.Metadata["cassandra_version"])
	assert.NotEmpty(t, ks.Metadata["datacenter"])
}

func TestE2E_TableCarriesOptionsAndKeyLayout(t *testing.T) {
	orders := requireAsset(t, discoverShop(t), "shop.orders")

	assert.Equal(t, "mrn://table/cassandra/shop.orders", *orders.MRN)
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Customer orders", *orders.Description)
	assert.Equal(t, "shop", orders.Metadata["keyspace"])
	assert.Equal(t, "orders", orders.Metadata["table_name"])
	assert.Equal(t, "Customer orders", orders.Metadata["comment"])
	assert.EqualValues(t, 0, orders.Metadata["default_ttl"])
	assert.EqualValues(t, 864000, orders.Metadata["gc_grace_seconds"])
	assert.Equal(t, "SizeTieredCompactionStrategy", orders.Metadata["compaction_class"])
	assert.Equal(t, "LZ4Compressor", orders.Metadata["compression_class"])
	assert.NotEmpty(t, orders.Metadata["id"])
	assert.Equal(t, []string{"customer_id"}, stringList(orders.Metadata["partition_key"]))
	assert.Equal(t, []string{"order_id"}, stringList(orders.Metadata["clustering_columns"]))
	assert.Equal(t, []string{"compound"}, stringList(orders.Metadata["flags"]))
	assert.Equal(t, false, orders.Metadata["is_counter"])
}

func TestE2E_TableColumnsCarryKindsAndCQLTypes(t *testing.T) {
	orders := requireAsset(t, discoverShop(t), "shop.orders")

	assert.Equal(t,
		[]string{"customer_id", "order_id", "items", "loyalty_tier", "placed_at", "status", "tags", "total"},
		columnOrder(t, orders), "keys first by position, then the rest alphabetically")

	cols := decodeColumns(t, orders)
	assert.Equal(t, "partition_key", cols["customer_id"]["kind"])
	assert.Equal(t, "uuid", cols["customer_id"]["data_type"])
	assert.Equal(t, true, cols["customer_id"]["is_primary_key"])
	assert.Equal(t, false, cols["customer_id"]["is_nullable"])

	assert.Equal(t, "clustering", cols["order_id"]["kind"])
	assert.Equal(t, "timeuuid", cols["order_id"]["data_type"])
	assert.Equal(t, "desc", cols["order_id"]["clustering_order"])
	assert.Equal(t, true, cols["order_id"]["is_primary_key"])

	assert.Equal(t, "static", cols["loyalty_tier"]["kind"])
	assert.Equal(t, "text", cols["loyalty_tier"]["data_type"])
	assert.Equal(t, true, cols["loyalty_tier"]["is_nullable"])

	assert.Equal(t, "regular", cols["items"]["kind"])
	assert.Equal(t, "list<text>", cols["items"]["data_type"])
	assert.Equal(t, "set<text>", cols["tags"]["data_type"])
	assert.Equal(t, "decimal", cols["total"]["data_type"])
	assert.Equal(t, "timestamp", cols["placed_at"]["data_type"])
}

func TestE2E_CollectionAndUserTypeColumnsKeepTheirSpelling(t *testing.T) {
	customers := requireAsset(t, discoverShop(t), "shop.customers")

	cols := decodeColumns(t, customers)
	assert.Equal(t, "map<text, text>", cols["attributes"]["data_type"])
	assert.Equal(t, "frozen<address>", cols["home"]["data_type"])
	assert.Equal(t, "partition_key", cols["id"]["kind"])
	assert.Empty(t, stringList(customers.Metadata["clustering_columns"]))
}

func TestE2E_CounterTableIsFlagged(t *testing.T) {
	pageViews := requireAsset(t, discoverShop(t), "shop.page_views")

	assert.Equal(t, true, pageViews.Metadata["is_counter"])
	assert.ElementsMatch(t, []string{"compound", "counter"}, stringList(pageViews.Metadata["flags"]))
	assert.Equal(t, "counter", decodeColumns(t, pageViews)["views"]["data_type"])
}

func TestE2E_SecondaryIndexIsRecorded(t *testing.T) {
	orders := requireAsset(t, discoverShop(t), "shop.orders")

	assert.Equal(t, []string{"orders_status_idx"}, stringList(orders.Metadata["indexes"]))
	assert.EqualValues(t, 1, orders.Metadata["index_count"])
}

func TestE2E_ViewCarriesItsDefinition(t *testing.T) {
	view := requireAsset(t, discoverShop(t), "shop.orders_by_status")

	assert.Equal(t, "mrn://view/cassandra/shop.orders_by_status", *view.MRN)
	assert.Equal(t, "orders", view.Metadata["base_table"])
	assert.Equal(t, false, view.Metadata["include_all_columns"])
	assert.Equal(t, "status IS NOT NULL AND customer_id IS NOT NULL AND order_id IS NOT NULL", view.Metadata["where_clause"])
	assert.Equal(t, []string{"status"}, stringList(view.Metadata["partition_key"]))
	assert.Equal(t, []string{"customer_id", "order_id"}, stringList(view.Metadata["clustering_columns"]))

	require.NotNil(t, view.Query)
	assert.Equal(t,
		"SELECT status, customer_id, order_id, placed_at, total FROM shop.orders WHERE status IS NOT NULL AND customer_id IS NOT NULL AND order_id IS NOT NULL",
		*view.Query)
	require.NotNil(t, view.QueryLanguage)
	assert.Equal(t, "CQL", *view.QueryLanguage)

	cols := decodeColumns(t, view)
	assert.Equal(t, "partition_key", cols["status"]["kind"])
	assert.Equal(t, "desc", cols["order_id"]["clustering_order"])
}

func TestE2E_LineageLinksKeyspaceToItsObjectsAndViewToBaseTable(t *testing.T) {
	result := discoverShop(t)

	contains := func(target string) pluginsdk.LineageEdge {
		return pluginsdk.LineageEdge{Source: "mrn://keyspace/cassandra/shop", Target: target, Type: "CONTAINS"}
	}
	assert.Contains(t, result.Lineage, contains("mrn://table/cassandra/shop.customers"))
	assert.Contains(t, result.Lineage, contains("mrn://table/cassandra/shop.orders"))
	assert.Contains(t, result.Lineage, contains("mrn://table/cassandra/shop.page_views"))
	assert.Contains(t, result.Lineage, contains("mrn://view/cassandra/shop.orders_by_status"))

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://table/cassandra/shop.orders",
		Target: "mrn://view/cassandra/shop.orders_by_status",
		Type:   "VIEW_OF",
	})
	assert.Len(t, result.Lineage, 5)
}

func TestE2E_StatisticsCountColumnsAndIndexesButNeverRows(t *testing.T) {
	result := discoverShop(t)

	stats := make(map[string]map[string]float64)
	for _, st := range result.Statistics {
		if stats[st.AssetMRN] == nil {
			stats[st.AssetMRN] = make(map[string]float64)
		}
		stats[st.AssetMRN][st.MetricName] = st.Value
		assert.NotEqual(t, "asset.row_count", st.MetricName, "a row count would be a full scan")
	}

	assert.Equal(t, float64(8), stats["mrn://table/cassandra/shop.orders"]["asset.column_count"])
	assert.Equal(t, float64(1), stats["mrn://table/cassandra/shop.orders"]["asset.index_count"])
	assert.Equal(t, float64(6), stats["mrn://table/cassandra/shop.customers"]["asset.column_count"])
	assert.Equal(t, float64(2), stats["mrn://table/cassandra/shop.page_views"]["asset.column_count"])
	assert.Equal(t, float64(5), stats["mrn://view/cassandra/shop.orders_by_status"]["asset.column_count"])
	assert.NotContains(t, stats["mrn://table/cassandra/shop.customers"], "asset.index_count")
}

func sampleRows(t *testing.T, name string) ([]string, [][]interface{}) {
	t.Helper()
	host := cassandraHost(t)
	asset := requireAsset(t, discoverShop(t), name)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	columns, rows, err := buildBinary(t).FetchSampleData(ctx, e2eConfig(host), asset)
	require.NoError(t, err)
	return columns, rows
}

func rowByColumn(columns []string, rows [][]interface{}, column, want string) map[string]interface{} {
	index := -1
	for i, c := range columns {
		if c == column {
			index = i
		}
	}
	if index < 0 {
		return nil
	}
	for _, row := range rows {
		if fmt.Sprint(row[index]) != want {
			continue
		}
		out := make(map[string]interface{}, len(columns))
		for i, c := range columns {
			out[c] = row[i]
		}
		return out
	}
	return nil
}

func TestE2E_FetchSampleDataRendersUUIDsMapsAndUserTypes(t *testing.T) {
	columns, rows := sampleRows(t, "shop.customers")

	assert.ElementsMatch(t, []string{"id", "name", "email", "attributes", "home", "created_at"}, columns)
	require.Len(t, rows, 2)

	alice := rowByColumn(columns, rows, "id", "11111111-1111-1111-1111-111111111111")
	require.NotNil(t, alice, "expected Alice's row keyed by her uuid rendered as a string")
	assert.Equal(t, "Alice", alice["name"])
	assert.Equal(t, "2026-01-02T03:04:05Z", alice["created_at"])
	assert.Equal(t, map[string]interface{}{"plan": "gold"}, alice["attributes"])
	assert.Equal(t, map[string]interface{}{"street": "1 Main St", "city": "London", "postcode": "N1"}, alice["home"])
}

func TestE2E_FetchSampleDataKeepsNullsAsNull(t *testing.T) {
	columns, rows := sampleRows(t, "shop.customers")

	bob := rowByColumn(columns, rows, "id", "22222222-2222-2222-2222-222222222222")
	require.NotNil(t, bob)
	assert.Equal(t, "Bob", bob["name"])
	assert.Nil(t, bob["attributes"], "an unset map is null, not an empty map")
	assert.Nil(t, bob["home"], "an unset user type is null, not a map of zero values")
}

func TestE2E_FetchSampleDataRendersCollectionsDecimalsAndStatics(t *testing.T) {
	columns, rows := sampleRows(t, "shop.orders")

	require.Len(t, rows, 2)
	shipped := rowByColumn(columns, rows, "status", "shipped")
	require.NotNil(t, shipped)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", shipped["customer_id"])
	assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", shipped["order_id"])
	assert.Equal(t, "42.50", shipped["total"])
	assert.Equal(t, []interface{}{"book", "pen"}, shipped["items"])
	assert.Equal(t, []interface{}{"gift"}, shipped["tags"])
	assert.Equal(t, "gold", shipped["loyalty_tier"], "the static column is shared by every row of the partition")
	assert.Equal(t, "2026-03-04T05:06:07Z", shipped["placed_at"])
}

func TestE2E_FetchSampleDataWorksForViews(t *testing.T) {
	columns, rows := sampleRows(t, "shop.orders_by_status")

	assert.ElementsMatch(t, []string{"status", "customer_id", "order_id", "placed_at", "total"}, columns)
	assert.Len(t, rows, 2)
}

func TestE2E_FetchSampleDataRejectsAKeyspaceAsset(t *testing.T) {
	host := cassandraHost(t)
	keyspace := requireAsset(t, discoverShop(t), "shop")

	_, _, err := buildBinary(t).FetchSampleData(t.Context(), e2eConfig(host), keyspace)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "keyspace and table"), err.Error())
}
