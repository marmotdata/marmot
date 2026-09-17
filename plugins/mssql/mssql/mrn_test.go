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

func TestTableMRN_IsTheFullyQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders",
		assetMRN("Table", qualifiedName("shop", "dbo", "orders")))
}

func TestDatabaseMRN_IsTheBareDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/sql-server/shop", assetMRN("Database", "shop"))
}

func TestViewMRN_IsTheFullyQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://view/sql-server/shop.dbo.order_totals",
		assetMRN("View", qualifiedName("shop", "dbo", "order_totals")))
}

func TestFunctionMRN_IsTheFullyQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://function/sql-server/shop.dbo.get_customer",
		assetMRN("Function", qualifiedName("shop", "dbo", "get_customer")))
}

func TestProviderName_LosesItsSpaceInTheMRN(t *testing.T) {
	// The provider keeps its space where it is displayed, but mrn.New dashes
	// it so the MRN needs no escaping in a URL or a lineage edge.
	assert.Equal(t, providerName, "SQL Server")
	assert.Contains(t, assetMRN("Table", "shop.dbo.orders"), "/sql-server/")
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Table", "shop.dbo.orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Database", "shop")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so an
	// asset whose MRN says something else lands on a second, orphan asset.
	asset := testSource().databaseAsset(testDatabase(), serverInfo{}, 2, 3, 2)

	requireMRNAgreesWithFields(t, asset)
	assert.Equal(t, "mrn://database/sql-server/shop", *asset.MRN)
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	requireMRNAgreesWithFields(t, asset)
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", *asset.MRN)
	assert.Equal(t, "shop.dbo.orders", *asset.Name)
}

func TestViewAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	object := objectInfo{Schema: "dbo", Name: "order_totals", TypeDesc: "VIEW"}

	asset := testSource().objectAsset(testDatabase(), object, "View", "", "SELECT 1", nil, nil)

	requireMRNAgreesWithFields(t, asset)
	assert.Equal(t, "mrn://view/sql-server/shop.dbo.order_totals", *asset.MRN)
}

func TestFunctionAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	routine := routineInfo{Schema: "dbo", Name: "get_customer", TypeDesc: "SQL_STORED_PROCEDURE"}

	asset := testSource().routineAsset(testDatabase(), routine)

	requireMRNAgreesWithFields(t, asset)
	assert.Equal(t, "mrn://function/sql-server/shop.dbo.get_customer", *asset.MRN)
}

func TestTableAsset_MRNMatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects an Mssql service's tables with a fully
	// qualified name under the provider "SQL Server". This plugin has to
	// produce the same MRN, or the day it takes over, that asset is stranded
	// and a second one appears alongside it.
	object := objectInfo{Schema: "dbo", Name: "customers", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Equal(t, "mrn://table/sql-server/shop.dbo.customers", *asset.MRN)
	assert.Equal(t, []string{"SQL Server"}, asset.Providers)
}

func TestTableAsset_MRNSurvivesASchemaNameWithASpace(t *testing.T) {
	// mrn.New turns a space in the name into a hyphen. That is fine as long
	// as it does it the same way every time, because the edges are built from
	// the same helper.
	object := objectInfo{Schema: "order archive", Name: "orders", TypeDesc: "USER_TABLE"}

	asset := testSource().objectAsset(testDatabase(), object, "Table", "", "", nil, nil)

	assert.Equal(t, "mrn://table/sql-server/shop.order-archive.orders", *asset.MRN)

	parsed, err := mrn.Parse(*asset.MRN)
	require.NoError(t, err)
	assert.Equal(t, *asset.MRN, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestForeignKeyEdge_PointsAtMRNsThisRunCreates(t *testing.T) {
	// An edge whose endpoint does not exist is dropped by the server, so both
	// ends have to be built with the same helper the assets used.
	source := assetMRN("Table", qualifiedName("shop", "dbo", "orders"))
	target := assetMRN("Table", qualifiedName("shop", "dbo", "customers"))

	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", source)
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.customers", target)
}

func TestContainsEdge_LinksTheDatabaseToATableWhoseNameIsNotItsPrefix(t *testing.T) {
	// The Contents tree is built from CONTAINS edges, not by matching MRN
	// prefixes. Here the database MRN happens to be a prefix of the table
	// name, but the edge is what makes the link.
	database := assetMRN("Database", "shop")
	table := assetMRN("Table", qualifiedName("shop", "dbo", "orders"))

	assert.Equal(t, "mrn://database/sql-server/shop", database)
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", table)
	assert.NotEqual(t, database, table)
}

func requireMRNAgreesWithFields(t *testing.T, asset pluginsdk.Asset) {
	t.Helper()

	require.NotNil(t, asset.MRN)
	require.NotNil(t, asset.Name)
	require.NotEmpty(t, asset.Providers)
	assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
}
