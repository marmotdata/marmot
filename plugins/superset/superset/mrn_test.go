package superset

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Superset objects are named by the titles people gave them, not by the
// numeric ids OpenMetadata uses. A dataset is the one kind qualified by
// its schema, because two datasets on same-named tables in different
// schemas are different datasets.

func TestDashboardMRN_IsTheTitle(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/superset/shop-overview", assetMRN("Dashboard", "Shop Overview"))
}

func TestChartMRN_IsTheSliceName(t *testing.T) {
	assert.Equal(t, "mrn://chart/superset/orders-by-status", assetMRN("Chart", "Orders by Status"))
}

func TestDatasetMRN_IsSchemaDotTable(t *testing.T) {
	// The type loses its spaces: mrn.New sanitises the type the same way
	// it sanitises the name, and the server derives the same string.
	assert.Equal(t, "mrn://data-model-object/superset/public.orders", assetMRN("Data Model Object", "public.orders"))
}

func TestDatasetMRN_WithoutASchemaIsTheTable(t *testing.T) {
	assert.Equal(t, "mrn://data-model-object/superset/events", assetMRN("Data Model Object", datasetName("", "events")))
}

func TestDataSourceMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://datasource/superset/shop", assetMRN("DataSource", "Shop"))
}

func TestMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	for _, original := range []string{
		assetMRN("Dashboard", "Shop Overview"),
		assetMRN("Chart", "Orders by Status"),
		assetMRN("Data Model Object", "public.orders"),
		assetMRN("DataSource", "Shop"),
	} {
		parsed, err := mrn.Parse(original)
		require.NoError(t, err)
		assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
	}
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name); the
	// MRN a plugin declares must be exactly that or the asset lands twice.
	result := discover(t, shop(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %s", *a.Name)
	}
}

func TestDatasetMRN_IsNotAPrefixOfTheDatabaseThatHoldsIt(t *testing.T) {
	// The Contents tree is built from the FEEDS and CONTAINS edges the
	// run emits, not by matching MRN prefixes.
	assert.NotContains(t, assetMRN("Data Model Object", "public.orders"), "shop")
}

// Edges to tables other plugins own are pinned to those plugins' exact
// MRNs, because the server only keeps an edge whose both ends exist.

func TestNativeTableMRN_MatchesThePostgreSQLPlugin(t *testing.T) {
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "orders"), nativeTableMRN("postgresql", "shop", "public", "orders"))
	assert.Equal(t, "mrn://table/postgresql/orders", nativeTableMRN("postgresql", "shop", "public", "orders"))
}

func TestNativeTableMRN_MatchesTheSnowflakeProjection(t *testing.T) {
	assert.Equal(t, mrn.New("Table", "Snowflake", "SALES.PUBLIC.ORDERS"), nativeTableMRN("snowflake", "SALES", "PUBLIC", "ORDERS"))
}

func TestNativeTableMRN_KeepsTheSpaceInSQLServer(t *testing.T) {
	// "SQL Server" is the provider string with a space; mrn.New keeps it
	// in the service component, so the two plugins land on one asset.
	assert.Equal(t, "mrn://table/sql-server/shop.dbo.orders", nativeTableMRN("mssql", "shop", "dbo", "orders"))
}
