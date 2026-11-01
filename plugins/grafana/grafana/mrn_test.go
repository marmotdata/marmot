package grafana

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A dashboard is named by its folder and title, a chart by its
// dashboard and panel title, a data source by its own name. mrn.New
// turns the slashes and spaces in those names into hyphens.

func TestDashboardMRN_IsFolderAndTitle(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/grafana/sales-orders-overview",
		assetMRN("Dashboard", "Sales/Orders overview"))
}

func TestDashboardMRN_InTheGeneralFolderIsTheTitleAlone(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/grafana/ops-home", assetMRN("Dashboard", "Ops home"))
}

func TestChartMRN_IsDashboardAndPanel(t *testing.T) {
	assert.Equal(t, "mrn://chart/grafana/sales-orders-overview-revenue",
		assetMRN("Chart", "Sales/Orders overview/Revenue"))
}

func TestDataSourceMRN_IsTheDataSourceName(t *testing.T) {
	assert.Equal(t, "mrn://datasource/grafana/shop", assetMRN("DataSource", "Shop"))
}

func TestDashboardMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that
	// unchanged or the asset becomes unreachable from the UI.
	original := assetMRN("Dashboard", "Sales/Orders overview")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestChartMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Chart", "Sales/Orders overview/Top customers")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so
	// the MRN an asset declares must be exactly that or the asset lands
	// twice.
	result := discover(t, seeded(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %q", *a.Name)
		assert.Equal(t, "Grafana", a.Providers[0])
	}
}

// Edges to tables use the identity the table's own Marmot plugin
// produces, or the server drops them for pointing at nothing.

func TestTableEdge_PinsThePostgreSQLPluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://table/postgresql/orders"},
		sqlTableMRNs("grafana-postgresql-datasource", "shop", "SELECT * FROM public.orders"))
}

func TestTableEdge_PinsTheMySQLPluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://table/mysql/orders"},
		sqlTableMRNs("mysql", "shop", "SELECT * FROM shop.orders"))
}

func TestTableEdge_PinsTheSQLServerPluginsMRN(t *testing.T) {
	// The SQL Server provider has a space in it, which mrn.New dashes,
	// landing on the same MRN plugins/openmetadata produces.
	assert.Equal(t, []string{"mrn://table/sql-server/sales.dbo.orders"},
		sqlTableMRNs("mssql", "sales", "SELECT * FROM orders"))
}

func TestTableEdge_PinsTheClickHousePluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://table/clickhouse/events"},
		sqlTableMRNs("grafana-clickhouse-datasource", "default", "SELECT count() FROM default.events"))
}
