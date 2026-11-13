package mssql

import (
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One SQL Server instance holds many databases and two of them can hold the
// same schema and table name, so a table is addressed by its fully qualified
// database.schema.table name. This is the same shape the OpenMetadata plugin
// projects for an Mssql or AzureSQL service, so the two agree on identity.

func testSource() *Source {
	return &Source{config: &Config{
		Host:              "sqlserver.company.com",
		Port:              1433,
		User:              "marmot_reader",
		IncludeColumns:    true,
		IncludeViews:      true,
		IncludeProcedures: true,
	}}
}

func testDatabase() databaseInfo {
	return databaseInfo{
		Name:          "shop",
		DatabaseID:    5,
		CreateDate:    time.Date(2026, 9, 8, 8, 16, 57, 0, time.UTC),
		Collation:     "SQL_Latin1_General_CP1_CI_AS",
		State:         "ONLINE",
		RecoveryModel: "SIMPLE",
		Owner:         "sa",
	}
}

func TestTableMRN_IsTheFullyQualifiedPath(t *testing.T) {
	cfg := testSource().config

	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.orders", assetMRN(cfg, "Table", "shop", "dbo", "orders"))
	assert.Equal(t, "mrn://asset/view/sql-server/shop.dbo.order_totals", assetMRN(cfg, "View", "shop", "dbo", "order_totals"))
	assert.Equal(t, "mrn://asset/function/sql-server/shop.dbo.get_customer", assetMRN(cfg, "Function", "shop", "dbo", "get_customer"))
}

func TestDatabaseMRN_IsTheBareDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/sql-server/shop", assetMRN(testSource().config, "Database", "shop"))
}

func TestSameTableInTwoSchemasOrDatabasesGetsDifferentMRNs(t *testing.T) {
	cfg := testSource().config

	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "dbo", "users"), assetMRN(cfg, "Table", "shop", "sales", "users"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "dbo", "users"), assetMRN(cfg, "Table", "crm", "dbo", "users"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "dbo", "orders")
	assert.Equal(t, "mrn://asset/table/sql-server@prod-eu/shop.dbo.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestZoneBinding_NamesTheServerAddress(t *testing.T) {
	s := testSource()
	s.config.Zone = "prod-eu"

	assert.Equal(t, pluginsdk.ZoneBinding{Provider: "SQL Server", Host: "sqlserver.company.com:1433", Zone: "prod-eu"}, s.zoneBinding())
}

func TestObjectAsset_KeepsItsOwnNameAndListsTheOldMRNWhenAnZoneIsSet(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	plain := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)
	assert.Equal(t, "orders", *plain.Name)
	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.orders", *plain.MRN)
	assert.Empty(t, plain.PreviousMRNs, "an unchanged MRN needs no previous entry")

	s := testSource()
	s.config.Zone = "prod-eu"
	withZone := s.objectAsset(testDatabase(), object, "Table", "", "", nil, nil)
	assert.Equal(t, []string{"mrn://asset/table/sql-server/shop.dbo.orders"}, withZone.PreviousMRNs)
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	asset := testSource().databaseAsset(testDatabase(), serverInfo{}, 2, 3, 2)

	require.NotNil(t, asset.MRN)
	require.NotNil(t, asset.Name)
	assert.Equal(t, "mrn://asset/database/sql-server/shop", *asset.MRN)
	assert.Equal(t, "shop", *asset.Name)
}

func TestTableAsset_NameIsTheObjectsOwnName(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "customers", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Equal(t, "customers", *asset.Name)
	assert.Equal(t, "mrn://asset/table/sql-server/shop.dbo.customers", *asset.MRN)
	assert.Equal(t, []string{"SQL Server"}, asset.Providers)
}

func TestFunctionAsset_NameIsTheRoutinesOwnName(t *testing.T) {
	routine := routineInfo{Schema: "dbo", Name: "get_customer", TypeDesc: "SQL_STORED_PROCEDURE"}

	asset := testSource().routineAsset(testDatabase(), routine)

	assert.Equal(t, "get_customer", *asset.Name)
	assert.Equal(t, "mrn://asset/function/sql-server/shop.dbo.get_customer", *asset.MRN)
}

func TestTableAsset_MRNSurvivesASchemaNameWithASpace(t *testing.T) {
	object := objectInfo{Schema: "order archive", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	parsed, err := mrn.ParseMRN(*asset.MRN)
	require.NoError(t, err)
	assert.Equal(t, *asset.MRN, parsed.String())
}
