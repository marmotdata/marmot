package cassandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ordersColumns is the shop.orders table as system_schema.columns returns
// it: alphabetical by column name, which is not the order a reader wants.
func ordersColumns() []columnInfo {
	return []columnInfo{
		{Table: "orders", Name: "customer_id", ClusteringOrder: "none", Kind: kindPartitionKey, Position: 0, Type: "uuid"},
		{Table: "orders", Name: "items", ClusteringOrder: "none", Kind: kindRegular, Position: -1, Type: "list<text>"},
		{Table: "orders", Name: "loyalty_tier", ClusteringOrder: "none", Kind: kindStatic, Position: -1, Type: "text"},
		{Table: "orders", Name: "order_id", ClusteringOrder: "desc", Kind: kindClustering, Position: 0, Type: "timeuuid"},
		{Table: "orders", Name: "placed_at", ClusteringOrder: "none", Kind: kindRegular, Position: -1, Type: "timestamp"},
		{Table: "orders", Name: "status", ClusteringOrder: "none", Kind: kindRegular, Position: -1, Type: "text"},
		{Table: "orders", Name: "total", ClusteringOrder: "none", Kind: kindRegular, Position: -1, Type: "decimal"},
	}
}

func columnNames(cols []columnInfo) []string {
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name)
	}
	return names
}

func TestSortColumns_KeysFirstThenAlphabetical(t *testing.T) {
	cols := ordersColumns()

	sortColumns(cols)

	assert.Equal(t, []string{"customer_id", "order_id", "items", "loyalty_tier", "placed_at", "status", "total"}, columnNames(cols))
}

func TestSortColumns_OrdersCompositeKeysByPosition(t *testing.T) {
	cols := []columnInfo{
		{Name: "b", Kind: kindClustering, Position: 1},
		{Name: "z", Kind: kindPartitionKey, Position: 1},
		{Name: "a", Kind: kindClustering, Position: 0},
		{Name: "y", Kind: kindPartitionKey, Position: 0},
	}

	sortColumns(cols)

	assert.Equal(t, []string{"y", "z", "a", "b"}, columnNames(cols))
}

func TestKeyColumns_SplitsPartitionAndClusteringKeys(t *testing.T) {
	cols := ordersColumns()
	sortColumns(cols)

	partition, clustering, order := keyColumns(cols)

	assert.Equal(t, []string{"customer_id"}, partition)
	assert.Equal(t, []string{"order_id"}, clustering)
	assert.Equal(t, map[string]string{"order_id": "desc"}, order)
}

func TestKeyColumns_NoClusteringLeavesOrderNil(t *testing.T) {
	partition, clustering, order := keyColumns([]columnInfo{
		{Name: "id", Kind: kindPartitionKey, Position: 0},
		{Name: "name", Kind: kindRegular, Position: -1},
	})

	assert.Equal(t, []string{"id"}, partition)
	assert.Empty(t, clustering)
	assert.Nil(t, order)
}

func TestSchemaColumns_KeepsTheStoredCQLType(t *testing.T) {
	cols := schemaColumns([]columnInfo{
		{Name: "attributes", Kind: kindRegular, Position: -1, Type: "map<text, text>"},
		{Name: "home", Kind: kindRegular, Position: -1, Type: "frozen<address>"},
	})

	require.Len(t, cols, 2)
	assert.Equal(t, "map<text, text>", cols[0].DataType)
	assert.Equal(t, "frozen<address>", cols[1].DataType)
}

func TestSchemaColumns_KeyColumnsArePrimaryAndNotNullable(t *testing.T) {
	cols := schemaColumns([]columnInfo{
		{Name: "customer_id", Kind: kindPartitionKey, Position: 0, ClusteringOrder: "none", Type: "uuid"},
		{Name: "order_id", Kind: kindClustering, Position: 0, ClusteringOrder: "desc", Type: "timeuuid"},
	})

	require.Len(t, cols, 2)
	for _, c := range cols {
		assert.True(t, c.PrimaryKey, c.Name)
		assert.False(t, c.Nullable, c.Name)
	}
	assert.Equal(t, kindPartitionKey, cols[0].Kind)
	assert.Equal(t, 0, cols[0].Position)
	assert.Empty(t, cols[0].ClusteringOrder, "a partition key has no sort direction")
	assert.Equal(t, kindClustering, cols[1].Kind)
	assert.Equal(t, "desc", cols[1].ClusteringOrder)
}

func TestSchemaColumns_RegularAndStaticColumnsAreNullable(t *testing.T) {
	cols := schemaColumns([]columnInfo{
		{Name: "status", Kind: kindRegular, Position: -1, ClusteringOrder: "none", Type: "text"},
		{Name: "loyalty_tier", Kind: kindStatic, Position: -1, ClusteringOrder: "none", Type: "text"},
	})

	require.Len(t, cols, 2)
	for _, c := range cols {
		assert.False(t, c.PrimaryKey, c.Name)
		assert.True(t, c.Nullable, c.Name)
		assert.Equal(t, -1, c.Position, c.Name)
		assert.Empty(t, c.ClusteringOrder, c.Name)
	}
	assert.Equal(t, kindStatic, cols[1].Kind)
}

func TestViewQuery_SelectsStarWhenAllColumnsIncluded(t *testing.T) {
	v := viewInfo{Name: "orders_by_status", BaseTable: "orders", IncludeAllColumns: true,
		WhereClause: "status IS NOT NULL AND customer_id IS NOT NULL AND order_id IS NOT NULL"}

	query := viewQuery("shop", v, ordersColumns())

	assert.Equal(t, "SELECT * FROM shop.orders WHERE status IS NOT NULL AND customer_id IS NOT NULL AND order_id IS NOT NULL", query)
}

func TestViewQuery_ListsSelectedColumnsInSchemaOrder(t *testing.T) {
	v := viewInfo{Name: "orders_by_status", BaseTable: "orders", WhereClause: "status IS NOT NULL"}
	cols := []columnInfo{
		{Name: "status", Kind: kindPartitionKey, Position: 0},
		{Name: "customer_id", Kind: kindClustering, Position: 0},
		{Name: "total", Kind: kindRegular, Position: -1},
	}

	query := viewQuery("shop", v, cols)

	assert.Equal(t, "SELECT status, customer_id, total FROM shop.orders WHERE status IS NOT NULL", query)
}

func TestViewQuery_OmitsWhereWhenNoFilter(t *testing.T) {
	v := viewInfo{Name: "v", BaseTable: "orders", IncludeAllColumns: true}

	assert.Equal(t, "SELECT * FROM shop.orders", viewQuery("shop", v, nil))
}

func TestViewQuery_QuotesMixedCaseIdentifiers(t *testing.T) {
	v := viewInfo{Name: "v", BaseTable: "Orders", IncludeAllColumns: false}
	cols := []columnInfo{{Name: "customerId", Kind: kindPartitionKey}}

	assert.Equal(t, `SELECT "customerId" FROM "Shop"."Orders"`, viewQuery("Shop", v, cols))
}

func TestQuoteIdent_EscapesEmbeddedQuotes(t *testing.T) {
	assert.Equal(t, `"we""ird"`, quoteIdent(`we"ird`))
}

func TestParseReplication_SimpleStrategy(t *testing.T) {
	rep := parseReplication(map[string]string{
		"class":              "org.apache.cassandra.locator.SimpleStrategy",
		"replication_factor": "3",
	})

	assert.Equal(t, "SimpleStrategy", rep.Class)
	assert.Equal(t, 3, rep.Factor)
	assert.Nil(t, rep.PerDatacenter)
}

func TestParseReplication_NetworkTopologyPerDatacenter(t *testing.T) {
	rep := parseReplication(map[string]string{
		"class": "org.apache.cassandra.locator.NetworkTopologyStrategy",
		"dc1":   "3",
		"dc2":   "2",
	})

	assert.Equal(t, "NetworkTopologyStrategy", rep.Class)
	assert.Equal(t, 0, rep.Factor)
	assert.Equal(t, map[string]int{"dc1": 3, "dc2": 2}, rep.PerDatacenter)
}

func TestParseReplication_TransientFactorCountsFullReplicas(t *testing.T) {
	rep := parseReplication(map[string]string{
		"class": "org.apache.cassandra.locator.NetworkTopologyStrategy",
		"dc1":   "3/1",
	})

	assert.Equal(t, map[string]int{"dc1": 3}, rep.PerDatacenter)
}

func TestParseReplication_SkipsUnparsableFactor(t *testing.T) {
	rep := parseReplication(map[string]string{
		"class":              "org.apache.cassandra.locator.SimpleStrategy",
		"replication_factor": "many",
	})

	assert.Equal(t, "SimpleStrategy", rep.Class)
	assert.Equal(t, 0, rep.Factor)
}

func TestParseReplication_EmptyMap(t *testing.T) {
	rep := parseReplication(nil)

	assert.Empty(t, rep.Class)
	assert.Equal(t, 0, rep.Factor)
	assert.Nil(t, rep.PerDatacenter)
}

func TestShortClassName_StripsJavaPackage(t *testing.T) {
	assert.Equal(t, "SizeTieredCompactionStrategy",
		shortClassName("org.apache.cassandra.db.compaction.SizeTieredCompactionStrategy"))
	assert.Equal(t, "LZ4Compressor", shortClassName("org.apache.cassandra.io.compress.LZ4Compressor"))
}

func TestShortClassName_LeavesBareNames(t *testing.T) {
	assert.Equal(t, "SimpleStrategy", shortClassName("SimpleStrategy"))
	assert.Empty(t, shortClassName(""))
}
