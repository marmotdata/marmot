package questdb

import (
	"math"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "questdb.internal"})
	require.NoError(t, err)
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"port": 8812})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_FillsConnectionDefaults(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "questdb.internal"})
	require.NoError(t, err)
	require.NotNil(t, s.config)

	assert.Equal(t, 8812, s.config.Port)
	assert.Equal(t, "admin", s.config.User)
	assert.Equal(t, "qdb", s.config.Database)
	assert.Equal(t, "disable", s.config.SSLMode)
	assert.Empty(t, s.config.Password, "the default password is documented, never assumed")
}

func TestValidate_DefaultsBooleansToTrue(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "questdb.internal"})
	require.NoError(t, err)

	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeMaterializedViews)
	assert.True(t, s.config.IncludeStatistics)
	assert.True(t, s.config.ExcludeSystemTables)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":                  "questdb.internal",
		"include_statistics":    false,
		"exclude_system_tables": false,
	})
	require.NoError(t, err)

	assert.False(t, s.config.IncludeStatistics)
	assert.False(t, s.config.ExcludeSystemTables)
	// Untouched flags still default to true.
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
}

func TestValidate_KeepsExplicitConnectionValues(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host":     "questdb.internal",
		"port":     18812,
		"user":     "reader",
		"password": "s3cret",
		"ssl_mode": "require",
	})
	require.NoError(t, err)

	assert.Equal(t, 18812, s.config.Port)
	assert.Equal(t, "reader", s.config.User)
	assert.Equal(t, "s3cret", s.config.Password)
	assert.Equal(t, "require", s.config.SSLMode)
}

func TestValidate_RejectsUnknownSSLMode(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "questdb.internal", "ssl_mode": "verify-full"})
	require.Error(t, err)
}

func TestValidate_RejectsPortOutOfRange(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "questdb.internal", "port": 70000})
	require.Error(t, err)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{
		"host": "questdb.internal",
		"filter": map[string]any{
			"include": []any{"^trades.*"},
			"exclude": []any{".*_tmp$"},
		},
	})
	require.NoError(t, err)
}

func TestMeta_DescribesTheQuestDBPlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "questdb", meta.ID)
	assert.Equal(t, "QuestDB", meta.Name)
	assert.Equal(t, "questdb", meta.Icon)
	assert.Equal(t, "database", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}

func testSource() *Source {
	return &Source{config: &Config{Host: "questdb.internal", Port: 8812, IncludeColumns: true}}
}

func tradesTable() tableInfo {
	return tableInfo{
		Name:                "trades",
		Kind:                kindTable,
		DesignatedTimestamp: "ts",
		PartitionBy:         "DAY",
		WALEnabled:          true,
		Dedup:               true,
		MaxUncommittedRows:  500000,
		O3MaxLag:            600000000,
	}
}

func TestBuildAsset_TableCarriesStorageLayout(t *testing.T) {
	asset := testSource().buildAsset(discoveredObject{
		table:          tradesTable(),
		partitionCount: 2,
		sizeBytes:      4096,
	})

	assert.Equal(t, "Table", asset.Type)
	assert.Equal(t, []string{"QuestDB"}, asset.Providers)
	assert.Equal(t, "trades", *asset.Name)
	assert.Equal(t, "questdb.internal", asset.Metadata["host"])
	assert.Equal(t, 8812, asset.Metadata["port"])
	assert.Equal(t, "trades", asset.Metadata["table_name"])
	assert.Equal(t, "table", asset.Metadata["object_type"])
	assert.Equal(t, "ts", asset.Metadata["designated_timestamp"])
	assert.Equal(t, "DAY", asset.Metadata["partition_by"])
	assert.Equal(t, true, asset.Metadata["wal_enabled"])
	assert.Equal(t, true, asset.Metadata["dedup"])
	assert.Equal(t, int64(500000), asset.Metadata["max_uncommitted_rows"])
	assert.Equal(t, int64(600000000), asset.Metadata["o3_max_lag"])
	assert.Equal(t, int64(2), asset.Metadata["partition_count"])
	assert.Nil(t, asset.Query)
	assert.Nil(t, asset.Description)
}

func TestBuildAsset_OmitsWhatTheTableDoesNotHave(t *testing.T) {
	asset := testSource().buildAsset(discoveredObject{
		table:          tableInfo{Name: "venues", Kind: kindTable, PartitionBy: "NONE", O3MaxLag: -1},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	assert.Equal(t, "NONE", asset.Metadata["partition_by"])
	assert.NotContains(t, asset.Metadata, "designated_timestamp")
	assert.NotContains(t, asset.Metadata, "ttl")
	assert.NotContains(t, asset.Metadata, "max_uncommitted_rows")
	assert.NotContains(t, asset.Metadata, "o3_max_lag")
	assert.NotContains(t, asset.Metadata, "partition_count")
	assert.NotContains(t, asset.Metadata, "materialized")
}

func TestBuildAsset_TableRendersTTL(t *testing.T) {
	table := tradesTable()
	table.TTLValue, table.TTLUnit = 1, "WEEK"

	asset := testSource().buildAsset(discoveredObject{table: table, partitionCount: -1, sizeBytes: -1})

	assert.Equal(t, "1 WEEK", asset.Metadata["ttl"])
}

func TestBuildAsset_MaterializedViewCarriesQueryAndRefresh(t *testing.T) {
	refreshed := time.Date(2026, 9, 8, 2, 20, 57, 0, time.UTC)
	asset := testSource().buildAsset(discoveredObject{
		table: tableInfo{Name: "trades_1h", Kind: kindMaterializedView, DesignatedTimestamp: "ts", PartitionBy: "DAY", WALEnabled: true, O3MaxLag: -1},
		view: &viewInfo{
			Name:          "trades_1h",
			SQL:           "SELECT ts, symbol, avg(price) FROM trades SAMPLE BY 1h",
			Materialized:  true,
			BaseTable:     "trades",
			RefreshType:   "timer",
			RefreshPeriod: "1 HOUR",
			LastRefresh:   refreshed,
		},
		partitionCount: 2,
		sizeBytes:      1024,
	})

	assert.Equal(t, "View", asset.Type)
	assert.Equal(t, "materialized_view", asset.Metadata["object_type"])
	assert.Equal(t, true, asset.Metadata["materialized"])
	assert.Equal(t, "trades", asset.Metadata["base_table"])
	assert.Equal(t, "timer", asset.Metadata["refresh_type"])
	assert.Equal(t, "1 HOUR", asset.Metadata["refresh_period"])
	assert.Equal(t, "2026-09-08T02:20:57Z", asset.Metadata["last_refresh"])
	assert.Equal(t, "DAY", asset.Metadata["partition_by"], "a materialized view is stored like a table")
	assert.Equal(t, int64(2), asset.Metadata["partition_count"])
	require.NotNil(t, asset.Query)
	assert.Equal(t, "SELECT ts, symbol, avg(price) FROM trades SAMPLE BY 1h", *asset.Query)
	require.NotNil(t, asset.QueryLanguage)
	assert.Equal(t, "SQL", *asset.QueryLanguage)
}

func TestBuildAsset_PlainViewHasNoStorageLayout(t *testing.T) {
	asset := testSource().buildAsset(discoveredObject{
		table:          tableInfo{Name: "recent_trades", Kind: kindView, DesignatedTimestamp: "ts", PartitionBy: "N/A", WALEnabled: true},
		view:           &viewInfo{Name: "recent_trades", SQL: "SELECT * FROM trades"},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	assert.Equal(t, "View", asset.Type)
	assert.Equal(t, "view", asset.Metadata["object_type"])
	assert.Equal(t, false, asset.Metadata["materialized"])
	assert.NotContains(t, asset.Metadata, "partition_by")
	assert.NotContains(t, asset.Metadata, "designated_timestamp")
	assert.NotContains(t, asset.Metadata, "wal_enabled")
	assert.NotContains(t, asset.Metadata, "base_table")
	assert.NotContains(t, asset.Metadata, "refresh_type")
	require.NotNil(t, asset.Query)
	assert.Equal(t, "SELECT * FROM trades", *asset.Query)
}

func TestBuildAsset_IncludesColumnsInSchema(t *testing.T) {
	asset := testSource().buildAsset(discoveredObject{
		table:          tradesTable(),
		columns:        []columnInfo{{Column: pluginsdk.Column{Name: "ts", DataType: "TIMESTAMP", Nullable: true}, DesignatedTimestamp: true}},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	require.Contains(t, asset.Schema, "columns")
	assert.Contains(t, asset.Schema["columns"], `"column_name":"ts"`)
	assert.Contains(t, asset.Schema["columns"], `"designated_timestamp":true`)
}

func TestBuildAsset_LeavesSchemaEmptyWhenColumnsAreOff(t *testing.T) {
	s := testSource()
	s.config.IncludeColumns = false

	asset := s.buildAsset(discoveredObject{
		table:          tradesTable(),
		columns:        []columnInfo{{Column: pluginsdk.Column{Name: "ts", DataType: "TIMESTAMP"}}},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	assert.NotContains(t, asset.Schema, "columns")
}

func TestBuildAsset_InterpolatesTagsFromMetadata(t *testing.T) {
	s := testSource()
	s.config.Tags = pluginsdk.TagsConfig{"questdb", "partition:${partition_by}"}

	asset := s.buildAsset(discoveredObject{table: tradesTable(), partitionCount: -1, sizeBytes: -1})

	assert.Equal(t, []string{"questdb", "partition:DAY"}, asset.Tags)
}

func discoveredWithMRN(table tableInfo, view *viewInfo) discoveredObject {
	obj := discoveredObject{table: table, view: view}
	assetType := "Table"
	if table.Kind != kindTable {
		assetType = "View"
	}
	obj.mrn = assetMRN(assetType, table.Name)
	return obj
}

func TestBuildAsset_ViewIsTypedByItsKindEvenWithoutViewDetails(t *testing.T) {
	// tables() reports table_type V on builds whose views() function may
	// still be missing; the object is a View either way.
	asset := testSource().buildAsset(discoveredObject{
		table:          tableInfo{Name: "recent_trades", Kind: kindView},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	assert.Equal(t, "View", asset.Type)
	assert.Equal(t, "mrn://view/questdb/recent_trades", *asset.MRN)
	assert.Equal(t, false, asset.Metadata["materialized"])
	assert.Nil(t, asset.Query)
}

func TestViewLineage_MaterializedViewPointsAtItsBaseTable(t *testing.T) {
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tradesTable(), nil),
		discoveredWithMRN(tableInfo{Name: "trades_1h", Kind: kindMaterializedView},
			&viewInfo{Name: "trades_1h", Materialized: true, BaseTable: "trades", SQL: "SELECT ts FROM trades SAMPLE BY 1h"}),
	})

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://table/questdb/trades",
		Target: "mrn://view/questdb/trades_1h",
		Type:   "VIEW_OF",
	}}, edges)
}

func TestViewLineage_PlainViewPointsAtEveryTableItReads(t *testing.T) {
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tradesTable(), nil),
		discoveredWithMRN(tableInfo{Name: "venues", Kind: kindTable}, nil),
		discoveredWithMRN(tableInfo{Name: "recent_trades", Kind: kindView},
			&viewInfo{Name: "recent_trades", SQL: "SELECT t.ts FROM trades t JOIN venues v ON t.venue = v.name"}),
	})

	assert.ElementsMatch(t, []pluginsdk.LineageEdge{
		{Source: "mrn://table/questdb/trades", Target: "mrn://view/questdb/recent_trades", Type: "VIEW_OF"},
		{Source: "mrn://table/questdb/venues", Target: "mrn://view/questdb/recent_trades", Type: "VIEW_OF"},
	}, edges)
}

func TestViewLineage_ResolvesNamesCaseInsensitively(t *testing.T) {
	// QuestDB table names are case-insensitive, so a view written as
	// FROM TRADES still reads the table discovered as trades.
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tradesTable(), nil),
		discoveredWithMRN(tableInfo{Name: "v", Kind: kindView}, &viewInfo{Name: "v", SQL: "SELECT * FROM TRADES"}),
	})

	require.Len(t, edges, 1)
	assert.Equal(t, "mrn://table/questdb/trades", edges[0].Source)
}

func TestViewLineage_DropsEdgesToUndiscoveredObjects(t *testing.T) {
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tableInfo{Name: "v", Kind: kindView}, &viewInfo{Name: "v", SQL: "SELECT * FROM elsewhere"}),
	})

	assert.Empty(t, edges)
}

func TestViewLineage_ViewOnAViewLinksTheTwoViews(t *testing.T) {
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tableInfo{Name: "base_view", Kind: kindView}, &viewInfo{Name: "base_view", SQL: "SELECT 1"}),
		discoveredWithMRN(tableInfo{Name: "top_view", Kind: kindView}, &viewInfo{Name: "top_view", SQL: "SELECT * FROM base_view"}),
	})

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://view/questdb/base_view",
		Target: "mrn://view/questdb/top_view",
		Type:   "VIEW_OF",
	}}, edges)
}

func TestViewLineage_NeverLinksAViewToItself(t *testing.T) {
	edges := viewLineage([]discoveredObject{
		discoveredWithMRN(tableInfo{Name: "v", Kind: kindView}, &viewInfo{Name: "v", SQL: "SELECT * FROM v"}),
	})

	assert.Empty(t, edges)
}

func TestConvertValue_FormatsTimestampsAsRFC3339(t *testing.T) {
	ts := time.Date(2026, 9, 1, 10, 0, 0, 500, time.UTC)

	assert.Equal(t, "2026-09-01T10:00:00.0000005Z", convertValue(ts))
}

func TestConvertValue_FormatsUUIDsAsText(t *testing.T) {
	uid := [16]byte{0xa0, 0xee, 0xbc, 0x99, 0x9c, 0x0b, 0x4e, 0xf8, 0xbb, 0x6d, 0x6b, 0xb9, 0xbd, 0x38, 0x0a, 0x11}

	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", convertValue(uid))
}

func TestConvertValue_TurnsNaNIntoNull(t *testing.T) {
	// QuestDB represents a null DOUBLE as NaN, which JSON cannot carry.
	assert.Nil(t, convertValue(math.NaN()))
	assert.Nil(t, convertValue(math.Inf(1)))
	assert.Equal(t, 1.5, convertValue(1.5))
}

func TestConvertValue_PassesPlainValuesThrough(t *testing.T) {
	assert.Nil(t, convertValue(nil))
	assert.Equal(t, int64(3), convertValue(int64(3)))
	assert.Equal(t, "BTC-USD", convertValue("BTC-USD"))
	assert.Equal(t, []any{"a", nil}, convertValue([]any{"a", math.NaN()}))
}
