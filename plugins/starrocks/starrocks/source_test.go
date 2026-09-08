package starrocks

import (
	"database/sql"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesThePlugin(t *testing.T) {
	m := Meta()

	assert.Equal(t, "starrocks", m.ID)
	assert.Equal(t, "StarRocks", m.Name)
	assert.Equal(t, "starrocks", m.Icon)
	assert.Equal(t, "data-warehouse", m.Category)
	assert.Equal(t, "experimental", m.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, m.Features)
	assert.NotEmpty(t, m.ConfigSpec)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}

// Validate

func TestValidate_AppliesDefaults(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal", "user": "root"})

	require.NoError(t, err)
	assert.Equal(t, 9030, s.config.Port)
	assert.Equal(t, defaultCatalog, s.config.Catalog)
	assert.Equal(t, "false", s.config.TLS)
	assert.Equal(t, []string{"information_schema", "_statistics_", "sys"}, s.config.ExcludeDatabases)
}

func TestValidate_DefaultsTheBooleansOn(t *testing.T) {
	// A config that names none of them should still discover columns,
	// views, materialized views and statistics.
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal", "user": "root"})

	require.NoError(t, err)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeMaterializedViews)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_KeepsAnExplicitFalse(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "fe.internal", "user": "root", "include_statistics": false,
	})

	require.NoError(t, err)
	assert.False(t, s.config.IncludeStatistics)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"user": "root"})

	require.Error(t, err)
}

func TestValidate_MissingUserFails(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal"})

	require.Error(t, err)
}

func TestValidate_RejectsAnOutOfRangePort(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal", "user": "root", "port": 70000})

	require.Error(t, err)
}

func TestValidate_RejectsAnUnknownTLSMode(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal", "user": "root", "tls": "maybe"})

	require.Error(t, err)
}

func TestValidate_AcceptsSkipVerifyTLS(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe.internal", "user": "root", "tls": "skip-verify"})

	require.NoError(t, err)
	assert.Equal(t, "skip-verify", s.config.TLS)
}

// classifyObject. The engine and table type strings are the ones
// StarRocks 4.1.4 reports in information_schema.tables.

func TestClassifyObject_StarRocksBaseTableIsATable(t *testing.T) {
	assert.Equal(t, "table", classifyObject("BASE TABLE", "StarRocks"))
}

func TestClassifyObject_OlapEngineIsStillATable(t *testing.T) {
	// Releases before 3.x reported the engine as OLAP.
	assert.Equal(t, "table", classifyObject("TABLE", "OLAP"))
}

func TestClassifyObject_StarRocksViewIsAMaterializedView(t *testing.T) {
	// A view backed by StarRocks storage is a materialized view; that
	// pairing is the only signal information_schema.tables gives.
	assert.Equal(t, "materialized_view", classifyObject("VIEW", "StarRocks"))
}

func TestClassifyObject_ViewWithNoEngineIsAPlainView(t *testing.T) {
	assert.Equal(t, "view", classifyObject("VIEW", ""))
}

func TestClassifyObject_SystemViewIsAPlainView(t *testing.T) {
	assert.Equal(t, "view", classifyObject("SYSTEM VIEW", "MEMORY"))
}

func TestClassifyObject_MySQLEngineIsAnExternalTable(t *testing.T) {
	assert.Equal(t, "external_table", classifyObject("BASE TABLE", "MYSQL"))
}

func TestClassifyObject_IcebergEngineIsAnExternalTable(t *testing.T) {
	assert.Equal(t, "external_table", classifyObject("BASE TABLE", "ICEBERG"))
}

func TestClassifyObject_HiveEngineIsAnExternalTable(t *testing.T) {
	assert.Equal(t, "external_table", classifyObject("BASE TABLE", "HIVE"))
}

// selectDatabases

func TestSelectDatabases_ExcludesSystemDatabasesByDefault(t *testing.T) {
	s := &Source{config: &Config{ExcludeDatabases: []string{"information_schema", "_statistics_", "sys"}}}

	assert.Equal(t, []string{"shop"},
		s.selectDatabases([]string{"information_schema", "shop", "_statistics_", "sys"}))
}

func TestSelectDatabases_AnAllowListWins(t *testing.T) {
	s := &Source{config: &Config{Databases: []string{"shop"}}}

	assert.Equal(t, []string{"shop"}, s.selectDatabases([]string{"shop", "analytics"}))
}

func TestSelectDatabases_ExclusionBeatsTheAllowList(t *testing.T) {
	s := &Source{config: &Config{Databases: []string{"shop"}, ExcludeDatabases: []string{"shop"}}}

	assert.Empty(t, s.selectDatabases([]string{"shop", "analytics"}))
}

func TestSelectDatabases_EmptyAllowListKeepsEverything(t *testing.T) {
	s := &Source{config: &Config{}}

	assert.Equal(t, []string{"shop", "analytics"}, s.selectDatabases([]string{"shop", "analytics"}))
}

// buildColumns

func TestBuildColumns_MarksPrimaryKeyColumnsOfAPrimaryKeyTable(t *testing.T) {
	ddl := tableDDL{KeyModel: "PRIMARY", KeyColumns: []string{"customer_id"}}
	cols := buildColumns([]columnInfo{
		{Field: "customer_id", Type: "bigint", Null: "NO", Key: "YES", Comment: "Customer identifier"},
		{Field: "email", Type: "varchar(255)", Null: "NO", Key: "NO"},
	}, ddl)

	require.Len(t, cols, 2)
	assert.True(t, cols[0].PrimaryKey)
	assert.False(t, cols[0].SortingKey)
	assert.False(t, cols[1].PrimaryKey)
}

func TestBuildColumns_DuplicateKeyColumnsSortRatherThanIdentify(t *testing.T) {
	// A DUPLICATE key does not identify a row, so its columns are
	// sorting keys, not primary keys.
	ddl := tableDDL{KeyModel: "DUPLICATE", KeyColumns: []string{"event_date", "event_id"}}
	cols := buildColumns([]columnInfo{
		{Field: "event_date", Type: "date", Null: "NO", Key: "YES"},
		{Field: "user_id", Type: "bigint", Null: "YES", Key: "NO"},
	}, ddl)

	require.Len(t, cols, 2)
	assert.False(t, cols[0].PrimaryKey)
	assert.True(t, cols[0].SortingKey)
	assert.False(t, cols[1].SortingKey)
}

func TestBuildColumns_UniqueKeyColumnsArePrimaryKeys(t *testing.T) {
	ddl := tableDDL{KeyModel: "UNIQUE", KeyColumns: []string{"order_id", "order_date"}}
	cols := buildColumns([]columnInfo{
		{Field: "order_id", Type: "bigint", Null: "NO", Key: "YES"},
	}, ddl)

	require.Len(t, cols, 1)
	assert.True(t, cols[0].PrimaryKey)
}

func TestBuildColumns_OrderByColumnsAreSortingKeys(t *testing.T) {
	ddl := tableDDL{KeyModel: "UNIQUE", KeyColumns: []string{"order_id"}, OrderBy: []string{"order_date"}}
	cols := buildColumns([]columnInfo{
		{Field: "order_date", Type: "date", Null: "NO", Key: "NO"},
	}, ddl)

	require.Len(t, cols, 1)
	assert.True(t, cols[0].SortingKey)
}

func TestBuildColumns_ReadsAggregationTypeFromExtra(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "order_total", Type: "decimal(38,2)", Null: "YES", Key: "NO", Extra: "SUM"},
		{Field: "max_order", Type: "decimal(10,2)", Null: "YES", Key: "NO", Extra: "MAX"},
		{Field: "last_channel", Type: "varchar(64)", Null: "YES", Key: "NO", Extra: "REPLACE"},
	}, tableDDL{KeyModel: "AGGREGATE", KeyColumns: []string{"stat_date"}})

	require.Len(t, cols, 3)
	assert.Equal(t, "SUM", cols[0].AggregationType)
	assert.Equal(t, "MAX", cols[1].AggregationType)
	assert.Equal(t, "REPLACE", cols[2].AggregationType)
}

func TestBuildColumns_LeavesAggregationTypeEmptyForAnUnknownExtra(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "id", Type: "bigint", Null: "NO", Key: "YES", Extra: "AUTO_INCREMENT"},
	}, tableDDL{})

	require.Len(t, cols, 1)
	assert.Empty(t, cols[0].AggregationType)
}

func TestBuildColumns_MarksAutoIncrementFromExtra(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "id", Type: "bigint", Null: "NO", Key: "YES", Extra: "AUTO_INCREMENT"},
	}, tableDDL{})

	require.Len(t, cols, 1)
	assert.True(t, cols[0].IsAutoIncrement)
}

func TestBuildColumns_MarksAutoIncrementFromTheDDLWhenExtraIsSilent(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "id", Type: "bigint", Null: "NO", Key: "YES"},
	}, tableDDL{AutoIncrementColumns: []string{"id"}})

	require.Len(t, cols, 1)
	assert.True(t, cols[0].IsAutoIncrement)
}

func TestBuildColumns_ReadsNullabilityFromTheNullColumn(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "a", Type: "int", Null: "YES"},
		{Field: "b", Type: "int", Null: "NO"},
	}, tableDDL{})

	require.Len(t, cols, 2)
	assert.True(t, cols[0].Nullable)
	assert.False(t, cols[1].Nullable)
}

func TestBuildColumns_KeepsTheDeclaredTypeVerbatim(t *testing.T) {
	// Complex types must survive as written; a catalog that rewrites
	// array<int> to "array" loses the element type.
	cols := buildColumns([]columnInfo{
		{Field: "tag_ids", Type: "array<int>", Null: "YES"},
		{Field: "payload", Type: "json", Null: "YES"},
		{Field: "amount", Type: "decimal(10,2)", Null: "YES"},
	}, tableDDL{})

	require.Len(t, cols, 3)
	assert.Equal(t, "array<int>", cols[0].DataType)
	assert.Equal(t, "json", cols[1].DataType)
	assert.Equal(t, "decimal(10,2)", cols[2].DataType)
}

func TestBuildColumns_CarriesTheCommentAsDescription(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "email", Type: "varchar(255)", Null: "NO", Comment: "Customer email address"},
	}, tableDDL{})

	require.Len(t, cols, 1)
	assert.Equal(t, "Customer email address", cols[0].Description)
}

func TestBuildColumns_ReadsADefaultValue(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "last_channel", Type: "varchar(64)", Null: "YES", Default: sql.NullString{String: "web", Valid: true}},
	}, tableDDL{})

	require.Len(t, cols, 1)
	assert.Equal(t, "web", cols[0].Default)
}

func TestBuildColumns_LeavesDefaultUnsetWhenThereIsNone(t *testing.T) {
	cols := buildColumns([]columnInfo{
		{Field: "user_id", Type: "bigint", Null: "YES", Default: sql.NullString{}},
	}, tableDDL{})

	require.Len(t, cols, 1)
	assert.Nil(t, cols[0].Default)
}

// objectStatistics

func TestObjectStatistics_ReportsRowCountSizeAndColumnCount(t *testing.T) {
	stats := objectStatistics("mrn://table/starrocks/shop.orders", "table", tableInfo{
		Rows:       sql.NullInt64{Int64: 42, Valid: true},
		DataLength: sql.NullInt64{Int64: 4096, Valid: true},
	}, 4)

	require.Len(t, stats, 3)
	assert.Equal(t, "asset.column_count", stats[0].MetricName)
	assert.Equal(t, float64(4), stats[0].Value)
	assert.Equal(t, "asset.row_count", stats[1].MetricName)
	assert.Equal(t, float64(42), stats[1].Value)
	assert.Equal(t, "asset.size_bytes", stats[2].MetricName)
	assert.Equal(t, float64(4096), stats[2].Value)
}

func TestObjectStatistics_APlainViewHasNoRowsOrBytes(t *testing.T) {
	// A view stores nothing, so information_schema reports NULL and
	// there is no figure to publish.
	stats := objectStatistics("mrn://view/starrocks/shop.daily_sales", "view", tableInfo{}, 3)

	require.Len(t, stats, 1)
	assert.Equal(t, "asset.column_count", stats[0].MetricName)
}

func TestObjectStatistics_AMaterializedViewHasRowsAndBytes(t *testing.T) {
	stats := objectStatistics("mrn://view/starrocks/shop.mv_sales", "materialized_view", tableInfo{
		Rows:       sql.NullInt64{Int64: 2, Valid: true},
		DataLength: sql.NullInt64{Int64: 128, Valid: true},
	}, 2)

	require.Len(t, stats, 3)
}

func TestObjectStatistics_SkipsColumnCountWhenColumnsWereNotRead(t *testing.T) {
	stats := objectStatistics("mrn://table/starrocks/shop.orders", "table", tableInfo{}, -1)

	assert.Empty(t, stats)
}

// metadata assembly

func TestAddTableLayout_CopiesTheParsedClauses(t *testing.T) {
	metadata := map[string]any{}

	addTableLayout(metadata, parseTableDDL(eventsDDL), 2)

	assert.Equal(t, "DUPLICATE", metadata["key_model"])
	assert.Equal(t, "event_date, event_id", metadata["key_columns"])
	assert.Equal(t, "RANGE", metadata["partition_type"])
	assert.Equal(t, "event_date", metadata["partition_columns"])
	assert.Equal(t, 2, metadata["partition_count"])
	assert.Equal(t, "HASH", metadata["distribution"])
	assert.Equal(t, "event_id", metadata["distribution_columns"])
	assert.Equal(t, 4, metadata["buckets"])
	assert.Equal(t, 1, metadata["replication_num"])
}

func TestAddTableLayout_LeavesOutClausesTheTableDoesNotHave(t *testing.T) {
	// An unpartitioned table must not carry an empty partition_type.
	metadata := map[string]any{}

	addTableLayout(metadata, parseTableDDL("PRIMARY KEY(`id`)\nDISTRIBUTED BY HASH(`id`) BUCKETS 3"), 0)

	assert.NotContains(t, metadata, "partition_type")
	assert.NotContains(t, metadata, "order_by")
	assert.NotContains(t, metadata, "storage_volume")
}

func TestAddMaterializedViewMetadata_RecordsRefreshState(t *testing.T) {
	metadata := map[string]any{}

	addMaterializedViewMetadata(metadata, materializedViewInfo{
		RefreshType: "ASYNC", IsActive: "true", PartitionType: "UNPARTITIONED",
		TaskName: "mv-10359", LastRefreshState: "SUCCESS",
	}, tableDDL{})

	assert.Equal(t, true, metadata["materialized"])
	assert.Equal(t, "ASYNC", metadata["refresh_type"])
	assert.Equal(t, true, metadata["is_active"])
	assert.Equal(t, "SUCCESS", metadata["last_refresh_state"])
	assert.Equal(t, "mv-10359", metadata["task_name"])
}

func TestAddMaterializedViewMetadata_UnpartitionedIsNotAPartitionType(t *testing.T) {
	metadata := map[string]any{}

	addMaterializedViewMetadata(metadata, materializedViewInfo{PartitionType: "UNPARTITIONED"}, tableDDL{})

	assert.NotContains(t, metadata, "partition_type")
}

func TestAddMaterializedViewMetadata_IgnoresTheLiteralNullRefreshState(t *testing.T) {
	// StarRocks prints "null" for a refresh that has not happened yet,
	// which is not a state worth showing.
	metadata := map[string]any{}

	addMaterializedViewMetadata(metadata, materializedViewInfo{LastRefreshState: "null"}, tableDDL{})

	assert.NotContains(t, metadata, "last_refresh_state")
}

func TestParseBaseTables_ReadsTheQualifiedNamesStarRocksTracks(t *testing.T) {
	names := parseBaseTables(`{"default_catalog.shop.orders":"2026-09-08 07:41:27"}`)

	assert.Equal(t, []string{"default_catalog.shop.orders"}, names)
}

func TestParseBaseTables_EmptyValueYieldsNothing(t *testing.T) {
	assert.Empty(t, parseBaseTables(""))
	assert.Empty(t, parseBaseTables("not json"))
}

// lineage bookkeeping

func TestAddEdge_RecordsAnEdgeOnce(t *testing.T) {
	d := newDiscovery()

	d.addEdge("a", "b", "CONTAINS")
	d.addEdge("a", "b", "CONTAINS")

	assert.Len(t, d.lineage, 1)
}

func TestAddEdge_KeepsDifferentEdgeTypesBetweenTheSamePair(t *testing.T) {
	d := newDiscovery()

	d.addEdge("a", "b", "CONTAINS")
	d.addEdge("a", "b", "VIEW_OF")

	assert.Len(t, d.lineage, 2)
}

func TestAddEdge_RefusesToPointAnAssetAtItself(t *testing.T) {
	d := newDiscovery()

	d.addEdge("a", "a", "VIEW_OF")

	assert.Empty(t, d.lineage)
}

func TestResolveLineage_ViewOfPointsFromTheBaseTableToTheView(t *testing.T) {
	// The base table is the source and the view is the target, matching
	// the direction the rest of Marmot uses.
	s := &Source{config: &Config{Catalog: defaultCatalog}}
	d := newDiscovery()
	d.addObject("shop", "orders", "mrn://table/starrocks/shop.orders")
	d.viewRefs = append(d.viewRefs, viewReference{
		database: "shop", mrn: "mrn://view/starrocks/shop.daily_sales", refs: []string{"orders"},
	})

	s.resolveLineage(d)

	require.Len(t, d.lineage, 1)
	assert.Equal(t, "mrn://table/starrocks/shop.orders", d.lineage[0].Source)
	assert.Equal(t, "mrn://view/starrocks/shop.daily_sales", d.lineage[0].Target)
	assert.Equal(t, "VIEW_OF", d.lineage[0].Type)
}

func TestResolveLineage_DropsAReferenceToSomethingNotDiscovered(t *testing.T) {
	// The server discards edges whose endpoint does not exist, so the
	// plugin does not emit them in the first place.
	s := &Source{config: &Config{Catalog: defaultCatalog}}
	d := newDiscovery()
	d.viewRefs = append(d.viewRefs, viewReference{
		database: "shop", mrn: "mrn://view/starrocks/shop.daily_sales", refs: []string{"missing"},
	})

	s.resolveLineage(d)

	assert.Empty(t, d.lineage)
}

func TestResolveLineage_ForeignKeyPointsFromTheReferencingTable(t *testing.T) {
	s := &Source{config: &Config{Catalog: defaultCatalog}}
	d := newDiscovery()
	d.addObject("shop", "customers", "mrn://table/starrocks/shop.customers")
	d.foreignKeys = append(d.foreignKeys, foreignKeyReference{
		database: "shop", mrn: "mrn://table/starrocks/shop.orders", ref: "shop.customers",
	})

	s.resolveLineage(d)

	require.Len(t, d.lineage, 1)
	assert.Equal(t, "mrn://table/starrocks/shop.orders", d.lineage[0].Source)
	assert.Equal(t, "mrn://table/starrocks/shop.customers", d.lineage[0].Target)
	assert.Equal(t, "FOREIGN_KEY", d.lineage[0].Type)
}

func TestResolveLineage_ResolvesAViewReferenceInAnotherDatabase(t *testing.T) {
	s := &Source{config: &Config{Catalog: defaultCatalog}}
	d := newDiscovery()
	d.addObject("warehouse", "orders", "mrn://table/starrocks/warehouse.orders")
	d.viewRefs = append(d.viewRefs, viewReference{
		database: "shop", mrn: "mrn://view/starrocks/shop.daily_sales", refs: []string{"warehouse.orders"},
	})

	s.resolveLineage(d)

	require.Len(t, d.lineage, 1)
	assert.Equal(t, "mrn://table/starrocks/warehouse.orders", d.lineage[0].Source)
}

// helpers

func TestObjectName_QualifiesTheObjectWithItsDatabase(t *testing.T) {
	assert.Equal(t, "shop.orders", objectName("shop", "orders"))
}

func TestQuoteIdentifier_WrapsInBackticks(t *testing.T) {
	assert.Equal(t, "`shop`", quoteIdentifier("shop"))
}

func TestQuoteIdentifier_EscapesAnEmbeddedBacktick(t *testing.T) {
	assert.Equal(t, "`we``ird`", quoteIdentifier("we`ird"))
}

func TestShowValue_FindsAColumnByNameRegardlessOfPosition(t *testing.T) {
	// SHOW output gains and loses columns between StarRocks versions,
	// so values are read by name.
	columns := []string{"Field", "Type", "Collation", "Null", "Key", "Default", "Extra", "Privileges", "Comment"}
	row := make([]sql.NullString, len(columns))
	row[8] = sql.NullString{String: "Customer identifier", Valid: true}

	assert.Equal(t, "Customer identifier", showValue(columns, row, "Comment"))
}

func TestShowValue_EmptyWhenTheColumnIsAbsent(t *testing.T) {
	assert.Empty(t, showValue([]string{"Field"}, []sql.NullString{{String: "a", Valid: true}}, "Comment"))
}

func TestConvertValue_TextComesBackAsAString(t *testing.T) {
	assert.Equal(t, "hello", convertValue([]byte("hello")))
}

func TestConvertValue_BinaryComesBackAsHex(t *testing.T) {
	assert.Equal(t, "0xfffe", convertValue([]byte{0xff, 0xfe}))
}
