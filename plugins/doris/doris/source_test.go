package doris

import (
	"database/sql"
	"testing"

	"github.com/go-sql-driver/mysql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta_DescribesTheDorisPlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "doris", meta.ID)
	assert.Equal(t, "Apache Doris", meta.Name)
	assert.Equal(t, "doris", meta.Icon)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotNil(t, meta.ConfigSpec)
}

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"user": "root"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_MissingUserFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user")
}

func TestValidate_DefaultsPortCatalogAndTLS(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root"})
	require.NoError(t, err)

	assert.Equal(t, 9030, s.config.Port)
	assert.Equal(t, "internal", s.config.Catalog)
	assert.Equal(t, "false", s.config.TLS)
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root"})
	require.NoError(t, err)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeMaterializedViews)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":                       "fe",
		"user":                       "root",
		"include_views":              false,
		"include_materialized_views": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeViews)
	assert.False(t, s.config.IncludeMaterializedViews)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeStatistics)
}

func TestValidate_DefaultsTheExcludedDatabases(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root"})
	require.NoError(t, err)

	assert.Equal(t, []string{"information_schema", "mysql", "__internal_schema"}, s.config.ExcludeDatabases)
}

func TestValidate_KeepsConfiguredDatabaseLists(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":              "fe",
		"user":              "root",
		"databases":         []any{"shop"},
		"exclude_databases": []any{"scratch"},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"shop"}, s.config.Databases)
	assert.Equal(t, []string{"scratch"}, s.config.ExcludeDatabases)
}

func TestValidate_RejectsAnUnknownTLSMode(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root", "tls": "maybe"})
	require.Error(t, err)
}

func TestValidate_RejectsAnOutOfRangePort(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "fe", "user": "root", "port": 70000})
	require.Error(t, err)
}

func TestDSN_SurvivesPasswordsWithSpecialCharacters(t *testing.T) {
	// A hand-built user:pass@tcp(...) string breaks on @ / and : in the
	// password; the driver's own formatter escapes them.
	s := &Source{config: &Config{Host: "fe", Port: 9030, User: "reader", Password: "p@ss/w:rd", TLS: "skip-verify"}}

	cfg, err := mysql.ParseDSN(s.dsn())
	require.NoError(t, err)

	assert.Equal(t, "reader", cfg.User)
	assert.Equal(t, "p@ss/w:rd", cfg.Passwd)
	assert.Equal(t, "fe:9030", cfg.Addr)
	assert.Equal(t, "skip-verify", cfg.TLSConfig)
}

func TestDSN_NamesNoDatabaseAndInterpolatesParameters(t *testing.T) {
	// Doris only prepares point queries server side, so every other
	// parameterised query has to be interpolated by the driver.
	s := &Source{config: &Config{Host: "fe", Port: 9030, User: "reader", TLS: "false"}}

	cfg, err := mysql.ParseDSN(s.dsn())
	require.NoError(t, err)

	assert.Empty(t, cfg.DBName)
	assert.True(t, cfg.InterpolateParams)
	assert.False(t, cfg.ParseTime)
}

func TestSelectDatabases_ExcludesTheSystemDatabases(t *testing.T) {
	existing := []string{"__internal_schema", "information_schema", "mysql", "shop"}
	excluded := []string{"information_schema", "mysql", "__internal_schema"}

	assert.Equal(t, []string{"shop"}, selectDatabases(existing, nil, excluded))
}

func TestSelectDatabases_ExclusionIsCaseInsensitive(t *testing.T) {
	assert.Equal(t, []string{"shop"}, selectDatabases([]string{"Scratch", "shop"}, nil, []string{"scratch"}))
}

func TestSelectDatabases_KeepsOnlyConfiguredDatabasesThatExist(t *testing.T) {
	// A typo in the config must not produce an empty Database asset.
	existing := []string{"information_schema", "shop"}

	assert.Equal(t, []string{"shop"}, selectDatabases(existing, []string{"shop", "shoppe"}, nil))
}

func TestSelectDatabases_ConfiguredListIgnoresExclusions(t *testing.T) {
	existing := []string{"information_schema", "shop"}

	assert.Equal(t, []string{"information_schema"}, selectDatabases(existing, []string{"information_schema"}, []string{"information_schema"}))
}

func nullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

func TestClassify_AnOLAPTableIsATable(t *testing.T) {
	row := tableRow{name: "events", tableType: "BASE TABLE", engine: nullString("Doris")}

	assert.Equal(t, "table", classify(row, nil))
}

func TestClassify_TheViewEngineIsAView(t *testing.T) {
	row := tableRow{name: "daily_sales", tableType: "VIEW", engine: nullString("View")}

	assert.Equal(t, "view", classify(row, nil))
}

func TestClassify_AViewWithoutAnEngineIsAMaterializedView(t *testing.T) {
	// This is how Doris 2.1 lists async materialized views in
	// information_schema.TABLES.
	row := tableRow{name: "mv_sales", tableType: "VIEW"}

	assert.Equal(t, "materialized_view", classify(row, nil))
}

func TestClassify_MvInfosOverridesTheTableListing(t *testing.T) {
	row := tableRow{name: "mv_sales", tableType: "VIEW", engine: nullString("View")}
	mvs := map[string]materializedView{"mv_sales": {name: "mv_sales"}}

	assert.Equal(t, "materialized_view", classify(row, mvs))
}

func TestClassify_AForeignEngineIsAnExternalTable(t *testing.T) {
	row := tableRow{name: "remote", tableType: "BASE TABLE", engine: nullString("MYSQL")}

	assert.Equal(t, "external_table", classify(row, nil))
}

func TestAddDDLMetadata_RecordsKeyPartitionAndDistribution(t *testing.T) {
	metadata := map[string]any{"engine": "Doris"}

	addDDLMetadata(metadata, parseTableDDL(eventsDDL))

	assert.Equal(t, "DUPLICATE", metadata["key_model"])
	assert.Equal(t, []string{"event_date", "event_id"}, metadata["key_columns"])
	assert.Equal(t, "RANGE", metadata["partition_type"])
	assert.Equal(t, []string{"event_date"}, metadata["partition_columns"])
	assert.Equal(t, "HASH", metadata["distribution_type"])
	assert.Equal(t, []string{"event_id"}, metadata["distribution_columns"])
	assert.Equal(t, 4, metadata["buckets"])
	assert.Equal(t, "tag.location.default: 1", metadata["replication"])
	assert.Equal(t, "hdd", metadata["storage_medium"])
}

func TestAddDDLMetadata_SaysNoneWhenThereIsNoKeyOrPartition(t *testing.T) {
	metadata := map[string]any{}

	addDDLMetadata(metadata, parseTableDDL(mvSalesDDL))

	assert.Equal(t, "none", metadata["key_model"])
	assert.Equal(t, "none", metadata["partition_type"])
	assert.NotContains(t, metadata, "key_columns")
	assert.NotContains(t, metadata, "partition_columns")
}

func TestAddDDLMetadata_RecordsAutoBucketsInsteadOfACount(t *testing.T) {
	metadata := map[string]any{}

	addDDLMetadata(metadata, parseTableDDL(ordersDDL))

	assert.Equal(t, true, metadata["auto_bucket"])
	assert.NotContains(t, metadata, "buckets")
	assert.NotContains(t, metadata, "distribution_columns")
}

func TestAddDDLMetadata_KeepsTheEngineInformationSchemaGave(t *testing.T) {
	metadata := map[string]any{"engine": "Doris"}

	addDDLMetadata(metadata, parseTableDDL(eventsDDL))

	assert.Equal(t, "Doris", metadata["engine"])
}

func TestAddDDLMetadata_FallsBackToTheDDLEngine(t *testing.T) {
	// information_schema leaves ENGINE empty for materialized views.
	metadata := map[string]any{}

	addDDLMetadata(metadata, parseTableDDL(mvSalesDDL))

	assert.Equal(t, "MATERIALIZED_VIEW", metadata["engine"])
}

func TestAddDDLMetadata_FallsBackToReplicationNum(t *testing.T) {
	metadata := map[string]any{}

	addDDLMetadata(metadata, tableDDL{Properties: map[string]string{"replication_num": "3"}})

	assert.Equal(t, "3", metadata["replication"])
}

func nullInt(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func statsByName(stats []pluginsdk.Statistic) map[string]float64 {
	out := make(map[string]float64, len(stats))
	for _, st := range stats {
		out[st.MetricName] = st.Value
	}
	return out
}

func TestObjectStatistics_ATableReportsRowsSizeAndColumns(t *testing.T) {
	row := tableRow{rowCount: nullInt(4), dataLength: nullInt(7853)}

	stats := statsByName(objectStatistics("mrn://table/doris/shop.events", "table", row, 8))

	assert.Equal(t, float64(4), stats["asset.row_count"])
	assert.Equal(t, float64(7853), stats["asset.size_bytes"])
	assert.Equal(t, float64(8), stats["asset.column_count"])
}

func TestObjectStatistics_AViewOnlyReportsColumns(t *testing.T) {
	row := tableRow{rowCount: nullInt(0), dataLength: nullInt(0)}

	stats := statsByName(objectStatistics("mrn://view/doris/shop.daily_sales", "view", row, 4))

	assert.Equal(t, map[string]float64{"asset.column_count": 4}, stats)
}

func TestObjectStatistics_AMaterializedViewReportsRows(t *testing.T) {
	row := tableRow{rowCount: nullInt(4), dataLength: nullInt(1331)}

	stats := statsByName(objectStatistics("mrn://view/doris/shop.mv_sales", "materialized_view", row, 3))

	assert.Equal(t, float64(4), stats["asset.row_count"])
	assert.Equal(t, float64(1331), stats["asset.size_bytes"])
}

func TestObjectStatistics_SkipsColumnCountWhenColumnsWereNotRead(t *testing.T) {
	row := tableRow{rowCount: nullInt(4), dataLength: nullInt(7853)}

	stats := statsByName(objectStatistics("mrn://table/doris/shop.events", "table", row, 0))

	assert.NotContains(t, stats, "asset.column_count")
}

func TestObjectStatistics_EveryMetricCarriesTheAssetMRN(t *testing.T) {
	row := tableRow{rowCount: nullInt(1), dataLength: nullInt(1)}

	for _, st := range objectStatistics("mrn://table/doris/shop.orders", "table", row, 3) {
		assert.Equal(t, "mrn://table/doris/shop.orders", st.AssetMRN)
	}
}

func TestResolvePendingEdges_ViewOfPointsFromTheBaseTableToTheView(t *testing.T) {
	objects := map[string]string{objectKey("shop", "events"): "mrn://table/doris/shop.events"}
	pending := []pendingEdge{{
		ref:         tableRef{Database: "shop", Table: "events"},
		otherMRN:    "mrn://view/doris/shop.daily_sales",
		edgeType:    "VIEW_OF",
		refIsSource: true,
	}}

	edges := resolvePendingEdges(pending, objects)

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://table/doris/shop.events",
		Target: "mrn://view/doris/shop.daily_sales",
		Type:   "VIEW_OF",
	}}, edges)
}

func TestResolvePendingEdges_ForeignKeyPointsFromReferencingToReferenced(t *testing.T) {
	objects := map[string]string{objectKey("shop", "customers"): "mrn://table/doris/shop.customers"}
	pending := []pendingEdge{{
		ref:      tableRef{Database: "shop", Table: "customers"},
		otherMRN: "mrn://table/doris/shop.orders",
		edgeType: "FOREIGN_KEY",
	}}

	edges := resolvePendingEdges(pending, objects)

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://table/doris/shop.orders",
		Target: "mrn://table/doris/shop.customers",
		Type:   "FOREIGN_KEY",
	}}, edges)
}

func TestResolvePendingEdges_DropsReferencesToUndiscoveredObjects(t *testing.T) {
	// Aliases, CTE names and tables in databases outside the run all end
	// up here; the server would reject the edge anyway.
	pending := []pendingEdge{{
		ref:         tableRef{Database: "shop", Table: "e"},
		otherMRN:    "mrn://view/doris/shop.daily_sales",
		edgeType:    "VIEW_OF",
		refIsSource: true,
	}}

	assert.Empty(t, resolvePendingEdges(pending, map[string]string{}))
}

func TestResolvePendingEdges_MatchesNamesCaseInsensitively(t *testing.T) {
	objects := map[string]string{objectKey("shop", "events"): "mrn://table/doris/shop.events"}
	pending := []pendingEdge{{
		ref:         tableRef{Database: "SHOP", Table: "Events"},
		otherMRN:    "mrn://view/doris/shop.daily_sales",
		edgeType:    "VIEW_OF",
		refIsSource: true,
	}}

	assert.Len(t, resolvePendingEdges(pending, objects), 1)
}

func TestResolvePendingEdges_DedupesRepeatedEdges(t *testing.T) {
	objects := map[string]string{objectKey("shop", "events"): "mrn://table/doris/shop.events"}
	edge := pendingEdge{
		ref:         tableRef{Database: "shop", Table: "events"},
		otherMRN:    "mrn://view/doris/shop.daily_sales",
		edgeType:    "VIEW_OF",
		refIsSource: true,
	}

	assert.Len(t, resolvePendingEdges([]pendingEdge{edge, edge}, objects), 1)
}

func TestResolvePendingEdges_DropsSelfReferences(t *testing.T) {
	objects := map[string]string{objectKey("shop", "events"): "mrn://table/doris/shop.events"}
	pending := []pendingEdge{{
		ref:      tableRef{Database: "shop", Table: "events"},
		otherMRN: "mrn://table/doris/shop.events",
		edgeType: "FOREIGN_KEY",
	}}

	assert.Empty(t, resolvePendingEdges(pending, objects))
}

func TestQuoteIdentifier_WrapsInBackticksAndEscapes(t *testing.T) {
	assert.Equal(t, "`order`", quoteIdentifier("order"))
	assert.Equal(t, "`a``b`", quoteIdentifier("a`b"))
}

func TestQuoteString_EscapesQuotesAndBackslashes(t *testing.T) {
	assert.Equal(t, `"shop"`, quoteString("shop"))
	assert.Equal(t, `"a\"b\\c"`, quoteString(`a"b\c`))
}

func TestConvertValue_TextBytesBecomeStrings(t *testing.T) {
	assert.Equal(t, "2026-01-05", convertValue([]byte("2026-01-05")))
	assert.Equal(t, "[1, 2]", convertValue([]byte("[1, 2]")))
}

func TestConvertValue_BinaryBytesBecomeHex(t *testing.T) {
	assert.Equal(t, "0xff00", convertValue([]byte{0xff, 0x00}))
}

func TestConvertValue_PassesOtherValuesThrough(t *testing.T) {
	assert.Nil(t, convertValue(nil))
	assert.Equal(t, int64(7), convertValue(int64(7)))
}
