package amundsen

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the exact strings the Marmot server will store. An asset
// imported from Amundsen has to land on the MRN the technology's own
// plugin produces, or the two runs leave two half populated assets.

func tableMRN(database, cluster, schema, table, assetType string) string {
	p := projectionFor(database)
	return assetMRN(assetType, p.Provider, p.tablePath(cluster, schema, table)...)
}

func TestTableMRN_PostgresTableIsDatabaseSchemaTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/postgresql/prod.public.orders", tableMRN("postgres", "prod", "public", "orders", "Table"))
}

func TestTableMRN_MySQLTableIsDatabaseTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/mysql/shop.orders", tableMRN("mysql", "gold", "shop", "orders", "Table"))
}

func TestTableMRN_HiveTableIsSchemaQualified(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/hive/sales.orders", tableMRN("hive", "gold", "sales", "orders", "Table"))
}

func TestTableMRN_SnowflakeTableKeepsEveryLevel(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/snowflake/analytics.public.orders", tableMRN("snowflake", "analytics", "public", "orders", "Table"))
}

func TestTableMRN_BigQueryTableIsProjectDatasetTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/bigquery/acme.sales.orders", tableMRN("bigquery", "acme", "sales", "orders", "Table"))
}

func TestTableMRN_AthenaIsCataloguedAsGlueByDatabaseAndTable(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/glue/sales.orders", tableMRN("athena", "awsdatacatalog", "sales", "orders", "Table"))
}

func TestTableMRN_SameTableNameInTwoSchemasDiffers(t *testing.T) {
	assert.NotEqual(t, tableMRN("postgres", "prod", "public", "orders", "Table"), tableMRN("postgres", "prod", "archive", "orders", "Table"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://asset/view/hive/sales.orders_view", tableMRN("hive", "gold", "sales", "orders_view", "View"))
}

func TestDashboardMRN_IsTheGroupAndTheDashboard(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/superset/finance-revenue",
		assetMRN("Dashboard", dashboardProviderFor("superset"), dashboardName("finance", "revenue")))
}

func TestTableMRN_DeltaLakeProviderLosesItsSpace(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/delta-lake/orders", tableMRN("delta", "prod", "sales", "orders", "Table"))
}

func TestAssets_EveryMRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())
	require.NotEmpty(t, c.assets)

	for _, asset := range c.assets {
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)

		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *asset.MRN)
		assert.Equal(t, strings.ToLower(asset.Type), parsed.Type)
		assert.Equal(t, strings.ReplaceAll(strings.ToLower(asset.Providers[0]), " ", "-"), parsed.Provider)
	}
}

func TestAssets_TablesKeepTheirOwnNameAndListTheirOldMRN(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	found := false
	for _, asset := range c.assets {
		if asset.Type != "Table" || asset.Providers[0] != "PostgreSQL" {
			continue
		}
		found = true
		assert.NotContains(t, *asset.Name, ".")
		assert.Contains(t, asset.PreviousMRNs, "mrn://asset/table/postgresql/"+*asset.Name)
	}
	assert.True(t, found)
}

func TestLineage_EndpointsArePinnedToTheProjectedMRNs(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.True(t, hasEdge(c, "mrn://asset/table/postgresql/prod.public.customers", "mrn://asset/table/postgresql/prod.public.orders", "FEEDS"),
		"customers is upstream of orders")
	assert.True(t, hasEdge(c, "mrn://asset/table/postgresql/prod.public.orders", "mrn://asset/dashboard/superset/finance-revenue", "FEEDS"),
		"orders feeds the revenue dashboard")
	assert.True(t, hasEdge(c, "mrn://asset/dashboard/superset/finance-revenue", "mrn://asset/chart/superset/finance-revenue.revenue-by-region", "CONTAINS"),
		"the dashboard contains its chart")
}
