package cassandra

import (
	"encoding/json"
	"testing"

	"github.com/gocql/gocql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The asset builders need no live cluster, so the exact shape of what a
// keyspace, table and view become can be pinned here from the rows
// system_schema returns.

func testSource() *Source {
	return &Source{config: &Config{
		IncludeColumns:    true,
		IncludeViews:      true,
		IncludeIndexes:    true,
		IncludeStatistics: true,
		BaseConfig:        pluginsdk.BaseConfig{Tags: pluginsdk.TagsConfig{"cassandra", "ks:${keyspace}"}},
	}}
}

func ordersTable() tableInfo {
	return tableInfo{
		Name:                "orders",
		ID:                  gocql.UUID{0xb6, 0x53, 0x5e, 0x70, 0xab, 0x01, 0x11, 0xf1, 0x9d, 0x36, 0x3b, 0x32, 0x56, 0x05, 0xc3, 0x64},
		Comment:             "Customer orders",
		Compaction:          map[string]string{"class": "org.apache.cassandra.db.compaction.SizeTieredCompactionStrategy", "max_threshold": "32"},
		Compression:         map[string]string{"chunk_length_in_kb": "16", "class": "org.apache.cassandra.io.compress.LZ4Compressor"},
		Caching:             map[string]string{"keys": "ALL", "rows_per_partition": "NONE"},
		DefaultTTL:          0,
		GCGraceSeconds:      864000,
		BloomFilterFPChance: 0.01,
		Flags:               []string{"compound"},
	}
}

func decodeColumns(t *testing.T, a pluginsdk.Asset) []map[string]interface{} {
	t.Helper()
	raw, ok := a.Schema["columns"]
	require.True(t, ok, "expected a column schema on %s", *a.Name)

	var cols []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &cols))
	return cols
}

func TestTableAsset_IsNamedByKeyspaceAndTable(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	require.NotNil(t, a.Name)
	assert.Equal(t, "shop.orders", *a.Name)
	assert.Equal(t, "Table", a.Type)
	assert.Equal(t, []string{"Cassandra"}, a.Providers)
	assert.Equal(t, "mrn://table/cassandra/shop.orders", *a.MRN)
}

func TestTableAsset_CarriesTableOptions(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	assert.Equal(t, "shop", a.Metadata["keyspace"])
	assert.Equal(t, "orders", a.Metadata["table_name"])
	assert.Equal(t, "table", a.Metadata["object_type"])
	assert.Equal(t, "b6535e70-ab01-11f1-9d36-3b325605c364", a.Metadata["id"])
	assert.Equal(t, "Customer orders", a.Metadata["comment"])
	assert.Equal(t, "SizeTieredCompactionStrategy", a.Metadata["compaction_class"])
	assert.Equal(t, "LZ4Compressor", a.Metadata["compression_class"])
	assert.Equal(t, 0, a.Metadata["default_ttl"])
	assert.Equal(t, 864000, a.Metadata["gc_grace_seconds"])
	assert.Equal(t, 0.01, a.Metadata["bloom_filter_fp_chance"])
	assert.Equal(t, map[string]string{"keys": "ALL", "rows_per_partition": "NONE"}, a.Metadata["caching"])
	assert.Equal(t, []string{"compound"}, a.Metadata["flags"])
	assert.Equal(t, false, a.Metadata["is_counter"])
}

func TestTableAsset_CommentBecomesDescription(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	require.NotNil(t, a.Description)
	assert.Equal(t, "Customer orders", *a.Description)
}

func TestTableAsset_NoCommentMeansNoDescription(t *testing.T) {
	table := ordersTable()
	table.Comment = ""

	a := testSource().tableAsset("shop", table, nil, nil)

	assert.Nil(t, a.Description)
	assert.NotContains(t, a.Metadata, "comment")
}

func TestTableAsset_CounterFlagIsSurfaced(t *testing.T) {
	table := tableInfo{Name: "page_views", Flags: []string{"counter", "compound"}}

	a := testSource().tableAsset("shop", table, nil, nil)

	assert.Equal(t, true, a.Metadata["is_counter"])
	assert.Equal(t, []string{"compound", "counter"}, a.Metadata["flags"], "flags are sorted for stable output")
}

func TestTableAsset_RecordsPrimaryKeyLayout(t *testing.T) {
	cols := ordersColumns()
	sortColumns(cols)

	a := testSource().tableAsset("shop", ordersTable(), cols, nil)

	assert.Equal(t, []string{"customer_id"}, a.Metadata["partition_key"])
	assert.Equal(t, []string{"order_id"}, a.Metadata["clustering_columns"])
	assert.Equal(t, map[string]string{"order_id": "desc"}, a.Metadata["clustering_order"])
}

func TestTableAsset_RecordsIndexes(t *testing.T) {
	indexes := []indexInfo{
		{Table: "orders", Name: "orders_total_idx", Kind: "COMPOSITES", Options: map[string]string{"target": "total"}},
		{Table: "orders", Name: "orders_status_idx", Kind: "COMPOSITES", Options: map[string]string{"target": "status"}},
	}

	a := testSource().tableAsset("shop", ordersTable(), nil, indexes)

	assert.Equal(t, []string{"orders_status_idx", "orders_total_idx"}, a.Metadata["indexes"])
	assert.Equal(t, 2, a.Metadata["index_count"])
}

func TestTableAsset_NoIndexesMeansNoIndexMetadata(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	assert.NotContains(t, a.Metadata, "indexes")
	assert.NotContains(t, a.Metadata, "index_count")
}

func TestTableAsset_EmbedsColumnsInSchema(t *testing.T) {
	cols := ordersColumns()
	sortColumns(cols)

	a := testSource().tableAsset("shop", ordersTable(), cols, nil)
	decoded := decodeColumns(t, a)

	require.Len(t, decoded, 7)
	assert.Equal(t, "customer_id", decoded[0]["column_name"])
	assert.Equal(t, "uuid", decoded[0]["data_type"])
	assert.Equal(t, true, decoded[0]["is_primary_key"])
	assert.Equal(t, false, decoded[0]["is_nullable"])
	assert.Equal(t, "partition_key", decoded[0]["kind"])

	assert.Equal(t, "order_id", decoded[1]["column_name"])
	assert.Equal(t, "desc", decoded[1]["clustering_order"])

	assert.Equal(t, "loyalty_tier", decoded[3]["column_name"])
	assert.Equal(t, "static", decoded[3]["kind"])
	assert.Equal(t, true, decoded[3]["is_nullable"])
	assert.NotContains(t, decoded[3], "is_primary_key", "false is omitted, matching the SDK column shape")
	assert.NotContains(t, decoded[3], "clustering_order")
}

func TestTableAsset_SkipsColumnsWhenDisabled(t *testing.T) {
	s := testSource()
	s.config.IncludeColumns = false

	a := s.tableAsset("shop", ordersTable(), ordersColumns(), nil)

	assert.NotContains(t, a.Schema, "columns")
	// The key layout is still known: it comes from the same rows.
	assert.Equal(t, []string{"customer_id"}, a.Metadata["partition_key"])
}

func TestTableAsset_InterpolatesTagsFromMetadata(t *testing.T) {
	a := testSource().tableAsset("shop", ordersTable(), nil, nil)

	assert.Equal(t, []string{"cassandra", "ks:shop"}, a.Tags)
}

func TestViewAsset_IsNamedByKeyspaceAndView(t *testing.T) {
	v := viewInfo{Name: "orders_by_status", BaseTable: "orders", WhereClause: "status IS NOT NULL"}

	a := testSource().viewAsset("shop", v, nil)

	assert.Equal(t, "shop.orders_by_status", *a.Name)
	assert.Equal(t, "View", a.Type)
	assert.Equal(t, "mrn://view/cassandra/shop.orders_by_status", *a.MRN)
}

func TestViewAsset_CarriesTheDefinitionAsCQL(t *testing.T) {
	v := viewInfo{Name: "orders_by_status", BaseTable: "orders", WhereClause: "status IS NOT NULL"}
	cols := []columnInfo{
		{Name: "status", Kind: kindPartitionKey, Position: 0},
		{Name: "customer_id", Kind: kindClustering, Position: 0, ClusteringOrder: "asc"},
	}

	a := testSource().viewAsset("shop", v, cols)

	require.NotNil(t, a.Query)
	assert.Equal(t, "SELECT status, customer_id FROM shop.orders WHERE status IS NOT NULL", *a.Query)
	require.NotNil(t, a.QueryLanguage)
	assert.Equal(t, "CQL", *a.QueryLanguage)

	assert.Equal(t, "view", a.Metadata["object_type"])
	assert.Equal(t, "orders_by_status", a.Metadata["table_name"])
	assert.Equal(t, "orders", a.Metadata["base_table"])
	assert.Equal(t, "status IS NOT NULL", a.Metadata["where_clause"])
	assert.Equal(t, false, a.Metadata["include_all_columns"])
	assert.Equal(t, []string{"status"}, a.Metadata["partition_key"])
	assert.Equal(t, []string{"customer_id"}, a.Metadata["clustering_columns"])
	assert.Len(t, decodeColumns(t, a), 2)
}

func TestKeyspaceAsset_IsNamedByKeyspace(t *testing.T) {
	ks := keyspaceInfo{Name: "shop", DurableWrites: true,
		Replication: map[string]string{"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"}}

	a := testSource().keyspaceAsset(ks, clusterInfo{}, 0, 0, nil)

	assert.Equal(t, "shop", *a.Name)
	assert.Equal(t, "Keyspace", a.Type)
	assert.Equal(t, "mrn://keyspace/cassandra/shop", *a.MRN)
}

func TestKeyspaceAsset_CarriesReplicationAndCounts(t *testing.T) {
	ks := keyspaceInfo{Name: "shop", DurableWrites: true,
		Replication: map[string]string{"class": "org.apache.cassandra.locator.SimpleStrategy", "replication_factor": "1"}}
	cluster := clusterInfo{Version: "5.0.9", Name: "marmot", Datacenter: "datacenter1"}
	types := []userType{{Name: "money"}, {Name: "address"}}

	a := testSource().keyspaceAsset(ks, cluster, 3, 1, types)

	assert.Equal(t, "shop", a.Metadata["keyspace"])
	assert.Equal(t, "SimpleStrategy", a.Metadata["replication_class"])
	assert.Equal(t, 1, a.Metadata["replication_factor"])
	assert.NotContains(t, a.Metadata, "replication_factors")
	assert.Equal(t, true, a.Metadata["durable_writes"])
	assert.Equal(t, 3, a.Metadata["table_count"])
	assert.Equal(t, 1, a.Metadata["view_count"])
	assert.Equal(t, []string{"address", "money"}, a.Metadata["user_types"])
	assert.Equal(t, "marmot", a.Metadata["cluster_name"])
	assert.Equal(t, "5.0.9", a.Metadata["cassandra_version"])
	assert.Equal(t, "datacenter1", a.Metadata["datacenter"])
}

func TestKeyspaceAsset_PerDatacenterReplication(t *testing.T) {
	ks := keyspaceInfo{Name: "shop",
		Replication: map[string]string{"class": "org.apache.cassandra.locator.NetworkTopologyStrategy", "dc1": "3", "dc2": "2"}}

	a := testSource().keyspaceAsset(ks, clusterInfo{}, 0, 0, nil)

	assert.Equal(t, "NetworkTopologyStrategy", a.Metadata["replication_class"])
	assert.NotContains(t, a.Metadata, "replication_factor")
	assert.Equal(t, map[string]int{"dc1": 3, "dc2": 2}, a.Metadata["replication_factors"])
}

func TestKeyspaceAsset_OmitsWhatItDoesNotKnow(t *testing.T) {
	a := testSource().keyspaceAsset(keyspaceInfo{Name: "shop"}, clusterInfo{}, 0, 0, nil)

	assert.NotContains(t, a.Metadata, "user_types")
	assert.NotContains(t, a.Metadata, "cluster_name")
	assert.NotContains(t, a.Metadata, "cassandra_version")
	assert.NotContains(t, a.Metadata, "datacenter")
}

func TestObjectStatistics_ColumnCountAndIndexCount(t *testing.T) {
	stats := objectStatistics("mrn://table/cassandra/shop.orders", ordersColumns(), true, 1)

	assert.Equal(t, []pluginsdk.Statistic{
		{AssetMRN: "mrn://table/cassandra/shop.orders", MetricName: "asset.column_count", Value: 7},
		{AssetMRN: "mrn://table/cassandra/shop.orders", MetricName: "asset.index_count", Value: 1},
	}, stats)
}

func TestObjectStatistics_NoColumnsKnownMeansNoColumnCount(t *testing.T) {
	// When the columns query failed the count is unknown, not zero.
	stats := objectStatistics("mrn://table/cassandra/shop.orders", nil, false, 0)

	assert.Empty(t, stats)
}

func TestObjectStatistics_NeverEmitsARowCount(t *testing.T) {
	stats := objectStatistics("mrn://table/cassandra/shop.orders", ordersColumns(), true, 0)

	for _, st := range stats {
		assert.NotEqual(t, "asset.row_count", st.MetricName)
	}
}
