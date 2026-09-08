package amundsen

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the exact strings the Marmot server will store. An asset
// imported from Amundsen has to land on the MRN the technology's own
// plugin produces, or the two runs leave two half populated assets.

func TestTableMRN_PostgresTableIsTheBareName(t *testing.T) {
	p := projectionFor("postgres")

	assert.Equal(t, "mrn://table/postgresql/orders",
		assetMRN("Table", p.Provider, p.TableName("postgres", "prod", "public", "orders")))
}

func TestTableMRN_HiveTableIsSchemaQualified(t *testing.T) {
	p := projectionFor("hive")

	assert.Equal(t, "mrn://table/hive/sales.orders",
		assetMRN("Table", p.Provider, p.TableName("hive", "gold", "sales", "orders")))
}

func TestTableMRN_SnowflakeTableKeepsEveryLevel(t *testing.T) {
	p := projectionFor("snowflake")

	assert.Equal(t, "mrn://table/snowflake/analytics.public.orders",
		assetMRN("Table", p.Provider, p.TableName("snowflake", "analytics", "public", "orders")))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	p := projectionFor("hive")

	assert.Equal(t, "mrn://view/hive/sales.orders_view",
		assetMRN("View", p.Provider, p.TableName("hive", "gold", "sales", "orders_view")))
}

func TestDashboardMRN_IsTheGroupAndTheDashboard(t *testing.T) {
	// mrn.New turns the slash between group and dashboard into a hyphen.
	assert.Equal(t, "mrn://dashboard/superset/finance-revenue",
		assetMRN("Dashboard", dashboardProviderFor("superset"), dashboardName("finance", "revenue")))
}

func TestChartMRN_IsTheGroupDashboardAndChart(t *testing.T) {
	name := dashboardName("finance", "revenue") + "/Revenue by region"

	assert.Equal(t, "mrn://chart/superset/finance-revenue-revenue-by-region",
		assetMRN("Chart", dashboardProviderFor("superset"), name))
}

func TestTableMRN_DeltaLakeProviderKeepsItsSpace(t *testing.T) {
	// A provider with a space lands that space in the MRN, which is
	// where the Delta Lake plugin lands too.
	p := projectionFor("delta")

	assert.Equal(t, "mrn://table/delta lake/orders",
		assetMRN("Table", p.Provider, p.TableName("delta", "prod", "sales", "orders")))
}

func TestMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that
	// unchanged or the asset becomes unreachable from the UI.
	original := assetMRN("Table", "Snowflake", "analytics.public.orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDashboardMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Dashboard", "Superset", dashboardName("finance", "revenue"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestAssets_EveryMRNMatchesItsOwnTypeProviderAndName(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name) and
	// ignores whatever MRN the plugin sent, so any asset where the two
	// disagree is filed somewhere the plugin did not intend.
	c := discover(t, testConfig(), seededGraph())
	require.NotEmpty(t, c.assets)

	for _, asset := range c.assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)

		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}

func TestLineage_EndpointsArePinnedToTheProjectedMRNs(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.True(t, hasEdge(c, "mrn://table/postgresql/customers", "mrn://table/postgresql/orders", "FEEDS"),
		"customers is upstream of orders")
	assert.True(t, hasEdge(c, "mrn://table/postgresql/orders", "mrn://dashboard/superset/finance-revenue", "FEEDS"),
		"orders feeds the revenue dashboard")
	assert.True(t, hasEdge(c, "mrn://dashboard/superset/finance-revenue", "mrn://chart/superset/finance-revenue-revenue-by-region", "CONTAINS"),
		"the dashboard contains its chart")
}
