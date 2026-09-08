package mssql

import (
	"encoding/json"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func minimalConfig() pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"host":     "sqlserver.company.com",
		"user":     "marmot_reader",
		"password": "secret",
	}
}

// Meta.

func TestMeta_UsesTheProviderIconKey(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "mssql", meta.ID)
	assert.Equal(t, "SQL Server", meta.Name)
	assert.Equal(t, "sql-server", meta.Icon)
	assert.Equal(t, "database", meta.Category)
}

func TestMeta_DeclaresLineageBecauseDiscoverEmitsEdges(t *testing.T) {
	assert.Equal(t, []string{"Assets", "Lineage"}, Meta().Features)
}

func TestMeta_ExposesTheConfigForm(t *testing.T) {
	assert.NotEmpty(t, Meta().ConfigSpec)
}

// Validate.

func TestValidate_RejectsAMissingHost(t *testing.T) {
	raw := minimalConfig()
	delete(raw, "host")

	_, err := (&Source{}).Validate(raw)

	assert.Error(t, err)
}

func TestValidate_RejectsAMissingUser(t *testing.T) {
	raw := minimalConfig()
	delete(raw, "user")

	_, err := (&Source{}).Validate(raw)

	assert.Error(t, err)
}

func TestValidate_RejectsAMissingPassword(t *testing.T) {
	raw := minimalConfig()
	delete(raw, "password")

	_, err := (&Source{}).Validate(raw)

	assert.Error(t, err)
}

func TestValidate_RejectsAnUnknownApplicationIntent(t *testing.T) {
	raw := minimalConfig()
	raw["application_intent"] = "WriteOnly"

	_, err := (&Source{}).Validate(raw)

	assert.Error(t, err)
}

func TestValidate_RejectsAPortOutsideTheValidRange(t *testing.T) {
	raw := minimalConfig()
	raw["port"] = 70000

	_, err := (&Source{}).Validate(raw)

	assert.Error(t, err)
}

func TestValidate_DefaultsThePort(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.Equal(t, 1433, s.config.Port)
}

func TestValidate_DefaultsTheSystemDatabaseExclusions(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.Equal(t, []string{"master", "model", "msdb", "tempdb"}, s.config.ExcludeDatabases)
}

func TestValidate_DefaultsTheSystemSchemaExclusions(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.Contains(t, s.config.ExcludeSchemas, "sys")
	assert.Contains(t, s.config.ExcludeSchemas, "INFORMATION_SCHEMA")
	assert.Contains(t, s.config.ExcludeSchemas, "db_datareader")
}

func TestValidate_DefaultsTheDiscoveryTogglesToOn(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.True(t, s.config.IncludeColumns)
	assert.True(t, s.config.IncludeViews)
	assert.True(t, s.config.IncludeProcedures)
	assert.True(t, s.config.DiscoverForeignKeys)
	assert.True(t, s.config.IncludeStatistics)
	assert.True(t, s.config.Encrypt)
}

func TestValidate_KeepsAToggleTheUserTurnedOff(t *testing.T) {
	// The tricky case for defaults: false is also a bool's zero value, so a
	// naive default would overwrite it.
	raw := minimalConfig()
	raw["include_views"] = false
	s := &Source{}

	_, err := s.Validate(raw)

	require.NoError(t, err)
	assert.False(t, s.config.IncludeViews)
}

func TestValidate_DefaultsApplicationIntentToReadWrite(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.Equal(t, "ReadWrite", s.config.ApplicationIntent)
}

func TestValidate_KeepsAnEmptyDatabaseMeaningEveryDatabase(t *testing.T) {
	s := &Source{}

	_, err := s.Validate(minimalConfig())

	require.NoError(t, err)
	assert.Equal(t, "", s.config.Database)
}

// Connection string.

func TestDSN_RequiresTLSWhenEncryptIsOn(t *testing.T) {
	s := &Source{}
	require.NoError(t, mustValidate(s, minimalConfig()))

	assert.Contains(t, s.dsn("shop"), "encrypt=true")
}

func TestDSN_TurnsTLSOffEntirelyWhenEncryptIsOff(t *testing.T) {
	// The driver's own "false" still runs a TLS handshake for the login
	// packet, so encrypt: false has to become "disable" to mean no TLS.
	raw := minimalConfig()
	raw["encrypt"] = false
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	assert.Contains(t, s.dsn("shop"), "encrypt=disable")
}

func TestDSN_NamesTheDatabaseToConnectTo(t *testing.T) {
	s := &Source{}
	require.NoError(t, mustValidate(s, minimalConfig()))

	assert.Contains(t, s.dsn("shop"), "database=shop")
}

func TestDSN_EscapesAPasswordThatWouldBreakTheURL(t *testing.T) {
	raw := minimalConfig()
	raw["password"] = "p@ss/word?x"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	dsn := s.dsn("shop")

	assert.Contains(t, dsn, "p%40ss%2Fword%3Fx")
	assert.NotContains(t, dsn, "p@ss/word?x")
}

func TestDSN_CarriesApplicationIntent(t *testing.T) {
	raw := minimalConfig()
	raw["application_intent"] = "ReadOnly"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	assert.Contains(t, s.dsn("shop"), "applicationintent=ReadOnly")
}

// Database selection.

func allDatabases() []databaseInfo {
	return []databaseInfo{
		{Name: "master"}, {Name: "model"}, {Name: "msdb"}, {Name: "tempdb"},
		{Name: "shop"}, {Name: "analytics"},
	}
}

func TestSelectDatabases_DropsTheSystemDatabases(t *testing.T) {
	s := &Source{}
	require.NoError(t, mustValidate(s, minimalConfig()))

	selected := s.selectDatabases(allDatabases())

	assert.Equal(t, []string{"shop", "analytics"}, databaseNames(selected))
}

func TestSelectDatabases_KeepsOnlyTheConfiguredDatabase(t *testing.T) {
	raw := minimalConfig()
	raw["database"] = "shop"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	selected := s.selectDatabases(allDatabases())

	assert.Equal(t, []string{"shop"}, databaseNames(selected))
}

func TestSelectDatabases_LetsAConfiguredDatabaseBeatTheExclusionList(t *testing.T) {
	// Naming msdb explicitly is a deliberate choice, so it should not be
	// silently dropped by the default exclusions.
	raw := minimalConfig()
	raw["database"] = "msdb"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	selected := s.selectDatabases(allDatabases())

	assert.Equal(t, []string{"msdb"}, databaseNames(selected))
}

func TestSelectDatabases_MatchesTheConfiguredDatabaseCaseInsensitively(t *testing.T) {
	raw := minimalConfig()
	raw["database"] = "SHOP"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	selected := s.selectDatabases(allDatabases())

	assert.Equal(t, []string{"shop"}, databaseNames(selected))
}

func TestSelectDatabases_ReturnsNothingWhenTheConfiguredDatabaseIsAbsent(t *testing.T) {
	raw := minimalConfig()
	raw["database"] = "warehouse"
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	assert.Empty(t, s.selectDatabases(allDatabases()))
}

func TestSelectDatabases_HonoursACustomExclusionList(t *testing.T) {
	raw := minimalConfig()
	raw["exclude_databases"] = []any{"analytics"}
	s := &Source{}
	require.NoError(t, mustValidate(s, raw))

	selected := s.selectDatabases(allDatabases())

	assert.NotContains(t, databaseNames(selected), "analytics")
	assert.Contains(t, databaseNames(selected), "shop")
	assert.Contains(t, databaseNames(selected), "master", "an empty exclusion list means nothing is excluded")
}

// Asset building.

func TestDatabaseAsset_CountsWhatItHolds(t *testing.T) {
	asset := testSource().databaseAsset(testDatabase(), serverInfo{
		ProductVersion: "15.0.2000.1574",
		Edition:        "Azure SQL Edge Developer (64-bit)",
	}, 2, 3, 2)

	assert.Equal(t, 2, asset.Metadata["schema_count"])
	assert.Equal(t, 3, asset.Metadata["table_count"])
	assert.Equal(t, 2, asset.Metadata["view_count"])
	assert.Equal(t, "15.0.2000.1574", asset.Metadata["server_version"])
	assert.Equal(t, "SQL_Latin1_General_CP1_CI_AS", asset.Metadata["collation"])
}

func TestDatabaseAsset_LeavesOutAnUnknownCollation(t *testing.T) {
	database := testDatabase()
	database.Collation = ""

	asset := testSource().databaseAsset(database, serverInfo{}, 0, 0, 0)

	assert.NotContains(t, asset.Metadata, "collation")
}

func TestTableAsset_UsesTheExtendedPropertyAsItsDescription(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "customers", TypeDesc: "USER_TABLE", Comment: "People who buy things"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	require.NotNil(t, asset.Description)
	assert.Equal(t, "People who buy things", *asset.Description)
	assert.Equal(t, "People who buy things", asset.Metadata["comment"])
}

func TestTableAsset_HasNoDescriptionWithoutAnExtendedProperty(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Nil(t, asset.Description)
	assert.NotContains(t, asset.Metadata, "comment")
}

func TestTableAsset_CarriesTheSchemaComment(t *testing.T) {
	object := objectInfo{Schema: "sales", Name: "regions", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "Sales reference data", "", nil, nil)

	assert.Equal(t, "Sales reference data", asset.Metadata["schema_comment"])
}

func TestTableAsset_RecordsTheObjectTypeInLowercase(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Equal(t, "user_table", asset.Metadata["object_type"])
}

func TestTableAsset_KeepsTheBareObjectNameInMetadata(t *testing.T) {
	// The Name is fully qualified, so the bare name has to live somewhere for
	// FetchSampleData to build its query from.
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Equal(t, "shop", asset.Metadata["database"])
	assert.Equal(t, "dbo", asset.Metadata["schema"])
	assert.Equal(t, "orders", asset.Metadata["table_name"])
}

func TestViewAsset_CarriesItsDefinitionAsAQuery(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "order_totals", TypeDesc: "VIEW"}
	definition := "CREATE VIEW dbo.order_totals AS SELECT 1 AS one"

	asset := testSource().objectAsset(testDatabase(), object, "View", "", definition, nil, nil)

	require.NotNil(t, asset.Query)
	require.NotNil(t, asset.QueryLanguage)
	assert.Equal(t, definition, *asset.Query)
	assert.Equal(t, "SQL", *asset.QueryLanguage)
}

func TestTableAsset_HasNoQuery(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Nil(t, asset.Query)
}

func TestFunctionAsset_CarriesItsBodyAsAQuery(t *testing.T) {
	routine := routineInfo{
		Schema:     "dbo",
		Name:       "get_customer",
		TypeDesc:   "SQL_STORED_PROCEDURE",
		Definition: "CREATE PROCEDURE dbo.get_customer @id INT AS BEGIN SELECT 1 END",
	}

	asset := testSource().routineAsset(testDatabase(), routine)

	require.NotNil(t, asset.Query)
	assert.Equal(t, routine.Definition, *asset.Query)
	assert.Equal(t, "stored_procedure", asset.Metadata["object_type"])
	assert.Equal(t, false, asset.Metadata["encrypted"])
}

func TestFunctionAsset_HasNoQueryWhenTheModuleIsEncrypted(t *testing.T) {
	// An encrypted module has no readable body, so the asset says so rather
	// than showing an empty query.
	routine := routineInfo{Schema: "dbo", Name: "secret_proc", TypeDesc: "SQL_STORED_PROCEDURE", IsEncrypted: true}

	asset := testSource().routineAsset(testDatabase(), routine)

	assert.Nil(t, asset.Query)
	assert.Equal(t, true, asset.Metadata["encrypted"])
}

// Columns.

func sampleColumns() []columnInfo {
	return []columnInfo{
		{Schema: "dbo", Object: "customers", Name: "customer_id", TypeName: "int",
			MaxLength: 4, Precision: 10, IsIdentity: true, IdentitySeed: 1, IdentityIncrement: 1},
		{Schema: "dbo", Object: "customers", Name: "first_name", TypeName: "nvarchar",
			MaxLength: 200, Collation: "SQL_Latin1_General_CP1_CI_AS"},
		{Schema: "dbo", Object: "customers", Name: "full_name", TypeName: "nvarchar",
			MaxLength: 402, ComputedDefinition: "(([first_name]+' ')+[last_name])", IsPersisted: true},
		{Schema: "dbo", Object: "customers", Name: "email", TypeName: "varchar",
			MaxLength: 255, Nullable: true, Comment: "Contact email address"},
		{Schema: "dbo", Object: "customers", Name: "credit_limit", TypeName: "decimal",
			MaxLength: 9, Precision: 10, Scale: 2, Nullable: true, DefaultDefinition: "((0.00))"},
	}
}

func TestBuildColumns_MarksThePrimaryKey(t *testing.T) {
	primaryKeys := map[string]bool{"dbo.customers.customer_id": true}

	columns := buildColumns("dbo", "customers", sampleColumns(), primaryKeys)

	assert.True(t, columns[0].PrimaryKey)
	assert.False(t, columns[1].PrimaryKey)
}

func TestBuildColumns_MarksAnIdentityColumnWithItsSeedAndIncrement(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.True(t, columns[0].IsIdentity)
	assert.Equal(t, int64(1), columns[0].IdentitySeed)
	assert.Equal(t, int64(1), columns[0].IdentityIncrement)
}

func TestBuildColumns_MarksAComputedColumnWithItsExpression(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.True(t, columns[2].IsComputed)
	assert.True(t, columns[2].IsPersisted)
	assert.Equal(t, "(([first_name]+' ')+[last_name])", columns[2].ComputedDefinition)
}

func TestBuildColumns_LeavesAnOrdinaryColumnUnmarked(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.False(t, columns[1].IsIdentity)
	assert.False(t, columns[1].IsComputed)
}

func TestBuildColumns_UsesTheExtendedPropertyAsTheColumnDescription(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.Equal(t, "Contact email address", columns[3].Description)
}

func TestBuildColumns_CarriesTheDefaultConstraintExpression(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.Equal(t, "((0.00))", columns[4].Default)
}

func TestBuildColumns_RendersEachTypeTheWaySQLServerDeclaresIt(t *testing.T) {
	columns := buildColumns("dbo", "customers", sampleColumns(), nil)

	assert.Equal(t, "int", columns[0].DataType)
	assert.Equal(t, "nvarchar(100)", columns[1].DataType)
	assert.Equal(t, "nvarchar(201)", columns[2].DataType)
	assert.Equal(t, "varchar(255)", columns[3].DataType)
	assert.Equal(t, "decimal(10,2)", columns[4].DataType)
}

func TestObjectAsset_SerialisesColumnsIntoTheSchema(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "customers", TypeDesc: "USER_TABLE"}
	primaryKeys := map[string]bool{"dbo.customers.customer_id": true}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", sampleColumns(), primaryKeys)

	require.Contains(t, asset.Schema, "columns")

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(asset.Schema["columns"]), &columns))
	require.Len(t, columns, 5)

	assert.Equal(t, "customer_id", columns[0]["column_name"])
	assert.Equal(t, "int", columns[0]["data_type"])
	assert.Equal(t, true, columns[0]["is_primary_key"])
	assert.Equal(t, true, columns[0]["is_identity"])
	assert.Equal(t, false, columns[0]["is_nullable"], "is_nullable is always emitted so false renders as Required")
}

func TestObjectAsset_HasAnEmptySchemaWhenColumnsAreOff(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.NotContains(t, asset.Schema, "columns")
}

// View lineage.

func viewAssets(t *testing.T) []pluginsdk.Asset {
	t.Helper()
	s := testSource()
	database := testDatabase()
	return []pluginsdk.Asset{
		s.objectAsset(database, objectInfo{Schema: "dbo", Name: "customers", TypeDesc: "USER_TABLE"}, "Table", "", "", nil, nil),
		s.objectAsset(database, objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}, "Table", "", "", nil, nil),
		s.objectAsset(database, objectInfo{Schema: "dbo", Name: "order_totals", TypeDesc: "VIEW"}, "View", "", "x", nil, nil),
	}
}

func discoveredNames() map[string]string {
	return map[string]string{
		"shop.dbo.customers":    "shop.dbo.customers",
		"shop.dbo.orders":       "shop.dbo.orders",
		"shop.dbo.order_totals": "shop.dbo.order_totals",
	}
}

func TestViewOfEdges_PointFromTheBaseTableToTheView(t *testing.T) {
	// The direction matters: data flows from the table into the view, so the
	// table is the Source and the view is the Target.
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT * FROM dbo.customers c JOIN dbo.orders o ON o.customer_id = c.customer_id",
	}

	edges := viewOfEdges("shop", viewAssets(t), definitions, discoveredNames())

	require.Len(t, edges, 2)
	for _, edge := range edges {
		assert.Equal(t, "VIEW_OF", edge.Type)
		assert.Equal(t, "mrn://view/sql-server/shop.dbo.order_totals", edge.Target)
	}
	assert.ElementsMatch(t,
		[]string{"mrn://table/sql-server/shop.dbo.customers", "mrn://table/sql-server/shop.dbo.orders"},
		[]string{edges[0].Source, edges[1].Source})
}

func TestViewOfEdges_SkipAnObjectThisRunDidNotDiscover(t *testing.T) {
	// The server drops an edge whose endpoint does not exist, so a reference
	// to an excluded or unreachable table has to be dropped here.
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT * FROM dbo.audit_log",
	}

	edges := viewOfEdges("shop", viewAssets(t), definitions, discoveredNames())

	assert.Empty(t, edges)
}

func TestViewOfEdges_SkipAViewReferencingItself(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT * FROM dbo.order_totals",
	}

	edges := viewOfEdges("shop", viewAssets(t), definitions, discoveredNames())

	assert.Empty(t, edges)
}

func TestViewOfEdges_SkipATableWithNoDefinition(t *testing.T) {
	edges := viewOfEdges("shop", viewAssets(t), map[string]string{}, discoveredNames())

	assert.Empty(t, edges)
}

func TestViewOfEdges_EmitOneEdgePerBaseObject(t *testing.T) {
	definitions := map[string]string{
		"dbo.order_totals": "CREATE VIEW dbo.order_totals AS SELECT * FROM dbo.orders UNION ALL SELECT * FROM dbo.orders",
	}

	edges := viewOfEdges("shop", viewAssets(t), definitions, discoveredNames())

	assert.Len(t, edges, 1)
}

func TestBaseAssetType_ReportsAViewBaseAsAView(t *testing.T) {
	// A view built on another view needs the View MRN, because that is the
	// only MRN the base object has.
	assert.Equal(t, "View", baseAssetType("shop.dbo.order_totals", viewAssets(t)))
}

func TestBaseAssetType_ReportsATableBaseAsATable(t *testing.T) {
	assert.Equal(t, "Table", baseAssetType("shop.dbo.orders", viewAssets(t)))
}

func mustValidate(s *Source, raw pluginsdk.RawConfig) error {
	_, err := s.Validate(raw)
	return err
}

func databaseNames(databases []databaseInfo) []string {
	names := make([]string, 0, len(databases))
	for _, database := range databases {
		names = append(names, database.Name)
	}
	return names
}
