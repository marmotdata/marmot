package hive

import (
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validate(t *testing.T, raw pluginsdk.RawConfig) *Config {
	t.Helper()
	s := &Source{}
	_, err := s.Validate(raw)
	require.NoError(t, err)
	require.NotNil(t, s.config)
	return s.config
}

func TestValidate_MissingHostFails(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host")
}

func TestValidate_AppliesDefaults(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "hive.internal"})

	assert.Equal(t, 10000, config.Port)
	assert.Equal(t, "NONE", config.Auth)
	assert.Equal(t, "hive", config.Username)
	assert.Equal(t, "hive", config.KerberosServiceName)
	assert.Equal(t, "binary", config.Transport)
	assert.Equal(t, "cliservice", config.HTTPPath)
	assert.False(t, config.SSL)
	assert.False(t, config.SSLSkipVerify)
	assert.Equal(t, []string{"sys", "information_schema"}, config.ExcludeDatabases)
	assert.True(t, config.IncludeColumns)
	assert.True(t, config.IncludeViews)
	assert.True(t, config.IncludeStatistics)
	assert.True(t, config.IncludePartitions)
	assert.False(t, config.IncludeDDL)
}

func TestValidate_RespectsExplicitFalse(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{
		"host":               "hive.internal",
		"include_views":      false,
		"include_statistics": false,
	})

	assert.False(t, config.IncludeViews)
	assert.False(t, config.IncludeStatistics)
	assert.True(t, config.IncludeColumns)
	assert.True(t, config.IncludePartitions)
}

func TestValidate_NormalisesAuthAndTransportCase(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{
		"host":      "hive.internal",
		"auth":      "ldap",
		"username":  "reader",
		"password":  "secret",
		"transport": "HTTP",
		"http_path": "/cliservice/",
	})

	assert.Equal(t, "LDAP", config.Auth)
	assert.Equal(t, "http", config.Transport)
	assert.Equal(t, "cliservice", config.HTTPPath)
}

func TestValidate_RejectsUnknownAuth(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "hive.internal", "auth": "TOKEN"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth")
}

func TestValidate_RejectsUnknownTransport(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "hive.internal", "transport": "thrift"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport")
}

func TestValidate_LDAPNeedsAPassword(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "hive.internal", "auth": "LDAP", "username": "reader"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestValidate_LDAPNeedsAUsername(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "hive.internal", "auth": "LDAP", "password": "secret"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
}

func TestValidate_LDAPWithCredentialsPasses(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "hive.internal", "auth": "LDAP", "username": "reader", "password": "secret"})

	assert.Equal(t, "reader", config.Username)
}

func TestValidate_KerberosLeavesTheUsernameEmpty(t *testing.T) {
	// The ticket cache decides the principal; a made-up username would be
	// misleading.
	config := validate(t, pluginsdk.RawConfig{"host": "hive.internal", "auth": "KERBEROS"})

	assert.Equal(t, "", config.Username)
}

func TestValidate_RejectsAnOutOfRangePort(t *testing.T) {
	s := &Source{}
	_, err := s.Validate(pluginsdk.RawConfig{"host": "hive.internal", "port": 70000})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestValidate_PortArrivesAsAFloatFromJSON(t *testing.T) {
	config := validate(t, pluginsdk.RawConfig{"host": "hive.internal", "port": float64(20000)})

	assert.Equal(t, 20000, config.Port)
}

func TestValidate_AcceptsFilters(t *testing.T) {
	validate(t, pluginsdk.RawConfig{
		"host": "hive.internal",
		"filter": map[string]any{
			"include": []any{"^sales\\..*"},
			"exclude": []any{".*_tmp$"},
		},
	})
}

func TestDriverAuth_BinaryPassesTheMechanismThrough(t *testing.T) {
	assert.Equal(t, "LDAP", driverAuth(&Config{Transport: "binary", Auth: "LDAP"}))
	assert.Equal(t, "NOSASL", driverAuth(&Config{Transport: "binary", Auth: "NOSASL"}))
	assert.Equal(t, "KERBEROS", driverAuth(&Config{Transport: "binary", Auth: "KERBEROS"}))
}

func TestDriverAuth_HTTPFoldsPasswordMechanismsIntoNone(t *testing.T) {
	// gohive's HTTP transport only knows NONE (basic auth) and KERBEROS.
	assert.Equal(t, "NONE", driverAuth(&Config{Transport: "http", Auth: "LDAP"}))
	assert.Equal(t, "NONE", driverAuth(&Config{Transport: "http", Auth: "NOSASL"}))
	assert.Equal(t, "KERBEROS", driverAuth(&Config{Transport: "http", Auth: "KERBEROS"}))
}

func TestSelectDatabases_ExcludesSystemDatabasesByDefault(t *testing.T) {
	s := &Source{config: &Config{ExcludeDatabases: []string{"sys", "information_schema"}}}

	assert.Equal(t, []string{"default", "sales"},
		s.selectDatabases([]string{"default", "information_schema", "sales", "sys"}))
}

func TestSelectDatabases_ExplicitListWinsOverExclusions(t *testing.T) {
	s := &Source{config: &Config{Databases: []string{"SALES", "sys"}, ExcludeDatabases: []string{"sys"}}}

	assert.Equal(t, []string{"sales", "sys"},
		s.selectDatabases([]string{"default", "sales", "sys"}))
}

func TestSelectDatabases_ExplicitListSkipsDatabasesTheServerDoesNotHave(t *testing.T) {
	s := &Source{config: &Config{Databases: []string{"missing"}}}

	assert.Empty(t, s.selectDatabases([]string{"default", "sales"}))
}

func TestHiveVersion_KeepsTheNumberOnly(t *testing.T) {
	assert.Equal(t, "4.0.1", hiveVersion("4.0.1 r3af4517eb8cfd9407ad34ed78a0b48b57dfaa264"))
	assert.Equal(t, "3.1.3", hiveVersion("3.1.3"))
	assert.Equal(t, "", hiveVersion("  "))
}

func TestParseDatabaseParameters_ReadsTheJavaMapForm(t *testing.T) {
	assert.Equal(t, map[string]string{"team": "data", "tier": "gold"},
		parseDatabaseParameters("{team=data, tier=gold}"))
}

func TestParseDatabaseParameters_EmptyMapIsNil(t *testing.T) {
	assert.Nil(t, parseDatabaseParameters("{}"))
	assert.Nil(t, parseDatabaseParameters(""))
}

func TestQuoteIdent_EscapesBackticks(t *testing.T) {
	assert.Equal(t, "`sales`.`odd``name`", qualifiedIdent("sales", "odd`name"))
}

func newSource() *Source {
	config := &Config{
		Host:              "hive.internal",
		Port:              10000,
		IncludeColumns:    true,
		IncludeStatistics: true,
	}
	config.Tags = pluginsdk.TagsConfig{"hive", "db:${database}"}
	return &Source{config: config}
}

func columnsOf(t *testing.T, a pluginsdk.Asset) map[string]map[string]any {
	t.Helper()
	raw, ok := a.Schema["columns"]
	require.True(t, ok)
	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &cols))
	byName := make(map[string]map[string]any, len(cols))
	for _, c := range cols {
		byName[c["column_name"].(string)] = c
	}
	return byName
}

func TestObjectAsset_TableCarriesTheDescribedMetadata(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := newSource().objectAsset("sales", "orders", info, nil, false, "")

	assert.Equal(t, "Table", a.Type)
	assert.Equal(t, []string{"Hive"}, a.Providers)
	assert.Equal(t, "sales", a.Metadata["database"])
	assert.Equal(t, "orders", a.Metadata["table_name"])
	assert.Equal(t, "external", a.Metadata["object_type"])
	assert.Equal(t, "hive", a.Metadata["owner"])
	assert.Equal(t, "USER", a.Metadata["owner_type"])
	assert.Equal(t, "2026-09-08T02:21:25Z", a.Metadata["created"])
	assert.Equal(t, "2026-09-08T02:21:25Z", a.Metadata["last_ddl"])
	assert.Nil(t, a.Metadata["last_access"], "UNKNOWN is not a time")
	assert.Equal(t, "file:/opt/hive/data/warehouse/sales.db/orders", a.Metadata["location"])
	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcSerde", a.Metadata["serde"])
	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcInputFormat", a.Metadata["input_format"])
	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcOutputFormat", a.Metadata["output_format"])
	assert.Equal(t, false, a.Metadata["compressed"])
	assert.Equal(t, 4, a.Metadata["num_buckets"])
	assert.Equal(t, []string{"customer_id"}, a.Metadata["bucket_columns"])
	assert.Nil(t, a.Metadata["sort_columns"])
	assert.Equal(t, int64(0), a.Metadata["num_files"])
	assert.Equal(t, "Orders", a.Metadata["comment"])
	require.NotNil(t, a.Description)
	assert.Equal(t, "Orders", *a.Description)
	assert.Nil(t, a.Query, "a table has no query")
}

func TestObjectAsset_TrimsSurfacedParameters(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	a := newSource().objectAsset("sales", "customers", info, nil, false, "")

	params, ok := a.Metadata["parameters"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "TRUE", params["TRANSLATED_TO_EXTERNAL"])
	assert.Equal(t, "2", params["bucketing_version"])
	for _, surfaced := range []string{"comment", "numRows", "numFiles", "totalSize", "rawDataSize", "transient_lastDdlTime", "COLUMN_STATS_ACCURATE"} {
		_, present := params[surfaced]
		assert.False(t, present, "%s is reported as its own field", surfaced)
	}
}

func TestObjectAsset_InterpolatesTags(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	a := newSource().objectAsset("sales", "customers", info, nil, false, "")

	assert.Equal(t, []string{"hive", "db:sales"}, a.Tags)
}

func TestObjectAsset_ColumnsCarryCommentsKeysAndTypes(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	cols := columnsOf(t, newSource().objectAsset("sales", "customers", info, nil, false, ""))

	require.Len(t, cols, 6)
	assert.Equal(t, "int", cols["id"]["data_type"])
	assert.Equal(t, "Customer id", cols["id"]["description"])
	assert.Equal(t, true, cols["id"]["is_primary_key"])
	assert.Equal(t, true, cols["id"]["is_nullable"], "a primary key without NOT NULL stays nullable in Hive")
	assert.Equal(t, "array<struct<sku:string,qty:int>>", cols["recent_items"]["data_type"])
	assert.Nil(t, cols["email"]["is_primary_key"])
	assert.Nil(t, cols["email"]["description"])
}

func TestObjectAsset_ColumnsMarkNotNullAndDefaults(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock"))

	cols := columnsOf(t, newSource().objectAsset("sales", "stock", info, nil, false, ""))

	assert.Equal(t, false, cols["sku"]["is_nullable"])
	assert.Equal(t, true, cols["sku"]["is_primary_key"])
	assert.Equal(t, true, cols["qty"]["is_nullable"])
	assert.Equal(t, "0", cols["qty"]["default_expression"])
}

func TestObjectAsset_PartitionColumnsComeLastAndAreMarked(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))

	a := newSource().objectAsset("sales", "events", info, []string{"dt=2026-01-01", "dt=2026-01-02"}, true, "")

	assert.Equal(t, []string{"dt"}, a.Metadata["partition_columns"])
	assert.Equal(t, 2, a.Metadata["partition_count"])
	assert.Equal(t, []string{"dt=2026-01-01", "dt=2026-01-02"}, a.Metadata["partition_values"])

	var cols []hiveColumn
	require.NoError(t, json.Unmarshal([]byte(a.Schema["columns"]), &cols))
	require.Len(t, cols, 4)
	assert.Equal(t, "dt", cols[3].Name)
	assert.True(t, cols[3].IsPartitionColumn)
	assert.False(t, cols[0].IsPartitionColumn)
}

func TestObjectAsset_CapsPartitionValuesAtTwenty(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))
	partitions := make([]string, 25)
	for i := range partitions {
		partitions[i] = "dt=" + string(rune('a'+i))
	}

	a := newSource().objectAsset("sales", "events", info, partitions, true, "")

	assert.Equal(t, 25, a.Metadata["partition_count"])
	assert.Len(t, a.Metadata["partition_values"], maxPartitionValues)
}

func TestObjectAsset_PartitionCountIsOmittedWhenTheListingFailed(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))

	a := newSource().objectAsset("sales", "events", info, nil, false, "")

	assert.Nil(t, a.Metadata["partition_count"])
	assert.Nil(t, a.Metadata["partition_values"])
	assert.Equal(t, []string{"dt"}, a.Metadata["partition_columns"], "the columns are known from DESCRIBE regardless")
}

func TestObjectAsset_ViewCarriesItsQuery(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	a := newSource().objectAsset("sales", "daily_totals", info, nil, false, "")

	assert.Equal(t, "View", a.Type)
	assert.Equal(t, "view", a.Metadata["object_type"])
	require.NotNil(t, a.Query)
	assert.Equal(t, "SELECT c.name, SUM(o.amount) AS total\nFROM sales.orders o\nJOIN sales.customers c ON o.customer_id = c.id\nGROUP BY c.name", *a.Query)
	require.NotNil(t, a.QueryLanguage)
	assert.Equal(t, "HiveQL", *a.QueryLanguage)
	assert.Nil(t, a.Metadata["serde"], "Hive prints null for a view's SerDe")
	assert.Nil(t, a.Metadata["location"])
}

func TestObjectAsset_MaterializedViewIsAViewWithStorage(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock_totals"))

	a := newSource().objectAsset("sales", "stock_totals", info, nil, false, "")

	assert.Equal(t, "View", a.Type)
	assert.Equal(t, "materialized_view", a.Metadata["object_type"])
	assert.Equal(t, "file:/opt/hive/data/warehouse/sales.db/stock_totals", a.Metadata["location"])
	require.NotNil(t, a.Query)
}

func TestObjectAsset_ManagedTransactionalTable(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock"))

	a := newSource().objectAsset("sales", "stock", info, nil, false, "")

	assert.Equal(t, "managed", a.Metadata["object_type"])
	assert.Equal(t, true, a.Metadata["transactional"])
}

func TestObjectAsset_DDLIsStoredWhenGiven(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := newSource().objectAsset("sales", "orders", info, nil, false, "CREATE EXTERNAL TABLE `sales`.`orders` (...)")

	assert.Equal(t, "CREATE EXTERNAL TABLE `sales`.`orders` (...)", a.Metadata["ddl"])
}

func TestObjectAsset_ColumnsAreSkippedWhenDisabled(t *testing.T) {
	s := newSource()
	s.config.IncludeColumns = false
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	a := s.objectAsset("sales", "orders", info, nil, false, "")

	_, present := a.Schema["columns"]
	assert.False(t, present)
}

func TestObjectStatistics_ReadsRowsSizeAndColumns(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	stats := make(map[string]float64)
	for _, st := range objectStatistics("mrn://table/hive/sales.customers", info) {
		assert.Equal(t, "mrn://table/hive/sales.customers", st.AssetMRN)
		stats[st.MetricName] = st.Value
	}

	assert.Equal(t, float64(2), stats["asset.row_count"])
	assert.Equal(t, float64(103), stats["asset.size_bytes"])
	assert.Equal(t, float64(6), stats["asset.column_count"])
}

func TestObjectStatistics_CountsPartitionColumns(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))

	for _, st := range objectStatistics("mrn://table/hive/sales.events", info) {
		if st.MetricName == "asset.column_count" {
			assert.Equal(t, float64(4), st.Value)
		}
	}
}

func TestObjectStatistics_ViewWithoutStatsOnlyReportsColumns(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	stats := objectStatistics("mrn://view/hive/sales.daily_totals", info)

	require.Len(t, stats, 1)
	assert.Equal(t, "asset.column_count", stats[0].MetricName)
}

func TestDatabaseAsset_CarriesTheDescribedMetadata(t *testing.T) {
	d := &discovery{source: newSource(), version: "4.0.1"}

	a := d.databaseAsset("sales", &databaseInfo{
		Comment:    "Sales data",
		Location:   "file:/opt/hive/data/warehouse/sales.db",
		Owner:      "hive",
		OwnerType:  "USER",
		Parameters: map[string]string{"team": "data"},
	})

	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, "sales", *a.Name)
	assert.Equal(t, "hive.internal", a.Metadata["host"])
	assert.Equal(t, 10000, a.Metadata["port"])
	assert.Equal(t, "Sales data", a.Metadata["comment"])
	assert.Equal(t, "file:/opt/hive/data/warehouse/sales.db", a.Metadata["location"])
	assert.Equal(t, "hive", a.Metadata["owner"])
	assert.Equal(t, "USER", a.Metadata["owner_type"])
	assert.Equal(t, map[string]string{"team": "data"}, a.Metadata["parameters"])
	assert.Equal(t, "4.0.1", a.Metadata["hive_version"])
	require.NotNil(t, a.Description)
	assert.Equal(t, "Sales data", *a.Description)
	assert.Equal(t, []string{"hive", "db:sales"}, a.Tags)
}

func TestDatabaseAsset_WithoutACommentHasNoDescription(t *testing.T) {
	d := &discovery{source: newSource()}

	a := d.databaseAsset("default", &databaseInfo{})

	assert.Nil(t, a.Description)
	assert.Nil(t, a.Metadata["comment"])
	assert.Nil(t, a.Metadata["parameters"])
}

func TestViewSources_MaterializedViewUsesTheRecordedSources(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock_totals"))

	assert.Equal(t, []string{"sales.stock"}, viewSources(info, "sales"))
}

func TestViewSources_PlainViewReadsTheExpandedQuery(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	assert.Equal(t, []string{"sales.orders", "sales.customers"}, viewSources(info, "sales"))
}

func TestViewSources_FallsBackToTheOriginalQuery(t *testing.T) {
	info := &tableInfo{OriginalQuery: "SELECT 1 FROM orders"}

	assert.Equal(t, []string{"sales.orders"}, viewSources(info, "sales"))
}

func TestResolvePendingEdges_ViewOfPointsFromBaseToView(t *testing.T) {
	d := &discovery{
		known: map[string]string{"sales.orders": "mrn://table/hive/sales.orders"},
		pending: []pendingEdge{{
			ref: "sales.orders", mrn: "mrn://view/hive/sales.daily_totals", refIsSource: true, edgeType: "VIEW_OF",
		}},
	}

	d.resolvePendingEdges()

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://table/hive/sales.orders",
		Target: "mrn://view/hive/sales.daily_totals",
		Type:   "VIEW_OF",
	}}, d.lineage)
}

func TestResolvePendingEdges_ForeignKeyPointsFromTableToParent(t *testing.T) {
	d := &discovery{
		known: map[string]string{"sales.customers": "mrn://table/hive/sales.customers"},
		pending: []pendingEdge{{
			ref: "sales.customers", mrn: "mrn://table/hive/sales.orders", refIsSource: false, edgeType: "FOREIGN_KEY",
		}},
	}

	d.resolvePendingEdges()

	assert.Equal(t, []pluginsdk.LineageEdge{{
		Source: "mrn://table/hive/sales.orders",
		Target: "mrn://table/hive/sales.customers",
		Type:   "FOREIGN_KEY",
	}}, d.lineage)
}

func TestResolvePendingEdges_DropsReferencesToUndiscoveredObjects(t *testing.T) {
	d := &discovery{
		known:   map[string]string{},
		pending: []pendingEdge{{ref: "sales.cte_name", mrn: "mrn://view/hive/sales.v", refIsSource: true, edgeType: "VIEW_OF"}},
	}

	d.resolvePendingEdges()

	assert.Empty(t, d.lineage)
}

func TestResolvePendingEdges_DeduplicatesAndSkipsSelfEdges(t *testing.T) {
	d := &discovery{
		known: map[string]string{"sales.a": "mrn://table/hive/sales.a", "sales.b": "mrn://table/hive/sales.b"},
		pending: []pendingEdge{
			{ref: "sales.a", mrn: "mrn://table/hive/sales.b", edgeType: "FOREIGN_KEY"},
			{ref: "sales.a", mrn: "mrn://table/hive/sales.b", edgeType: "FOREIGN_KEY"},
			{ref: "sales.a", mrn: "mrn://table/hive/sales.a", edgeType: "FOREIGN_KEY"},
		},
	}

	d.resolvePendingEdges()

	assert.Len(t, d.lineage, 1)
}

func TestResolvePendingEdges_MatchesReferencesCaseInsensitively(t *testing.T) {
	d := &discovery{
		known:   map[string]string{"sales.orders": "mrn://table/hive/sales.orders"},
		pending: []pendingEdge{{ref: "Sales.ORDERS", mrn: "mrn://view/hive/sales.v", refIsSource: true, edgeType: "VIEW_OF"}},
	}

	d.resolvePendingEdges()

	assert.Len(t, d.lineage, 1)
}

func TestFetchSampleData_RejectsAnAssetWithoutMetadata(t *testing.T) {
	s := &Source{}
	_, _, err := s.FetchSampleData(t.Context(), pluginsdk.RawConfig{"host": "hive.internal"}, &pluginsdk.Asset{})
	require.Error(t, err)
}

func TestFetchSampleData_RejectsAnAssetWithoutATable(t *testing.T) {
	s := &Source{}
	_, _, err := s.FetchSampleData(t.Context(), pluginsdk.RawConfig{"host": "hive.internal"}, &pluginsdk.Asset{
		Metadata: map[string]any{"database": "sales"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table")
}

// The zero-value Config keeps Go's false defaults; only Validate promotes
// the flags to true, so a raw struct must not look pre-configured.
func TestConfig_ZeroValueDefaultsAreFalse(t *testing.T) {
	config := &Config{Host: "hive.internal"}

	assert.False(t, config.IncludeColumns)
	assert.False(t, config.IncludeViews)
	assert.False(t, config.IncludeStatistics)
	assert.False(t, config.IncludePartitions)
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "hive", meta.ID)
	assert.Equal(t, "Hive", meta.Name)
	assert.Equal(t, "hive", meta.Icon)
	assert.Equal(t, "data-warehouse", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestSource_ImplementsDataFetcher(t *testing.T) {
	var _ pluginsdk.DataFetcher = &Source{}
}
