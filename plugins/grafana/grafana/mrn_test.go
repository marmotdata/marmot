package grafana

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A dashboard is named by its folder and title, a chart by its
// dashboard and panel title, a data source by its own name. mrn.New
// turns the slashes and spaces in those names into hyphens.

func assetMRN(assetType, name string) string {
	return mrn.Asset(assetType, providerName, name).String()
}

func TestMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod"
	d := &discovery{config: cfg}

	assert.Equal(t, "mrn://asset/dashboard/grafana@prod/ops-home", d.assetMRN("Dashboard", "Ops home"))
}

func TestDashboardMRN_IsFolderAndTitle(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/grafana/sales-orders-overview",
		assetMRN("Dashboard", "Sales/Orders overview"))
}

func TestDashboardMRN_InTheGeneralFolderIsTheTitleAlone(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/grafana/ops-home", assetMRN("Dashboard", "Ops home"))
}

func TestChartMRN_IsDashboardAndPanel(t *testing.T) {
	assert.Equal(t, "mrn://asset/chart/grafana/sales-orders-overview-revenue",
		assetMRN("Chart", "Sales/Orders overview/Revenue"))
}

func TestDataSourceMRN_IsTheDataSourceName(t *testing.T) {
	assert.Equal(t, "mrn://asset/datasource/grafana/shop", assetMRN("DataSource", "Shop"))
}

func TestEveryAsset_MRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, seeded(), nil)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *a.MRN)
		assert.Equal(t, strings.ToLower(a.Type), parsed.Type)
		assert.Equal(t, "grafana", parsed.Provider)
		assert.Equal(t, "Grafana", a.Providers[0])
	}
}

// Edges to tables use the identity the table's own Marmot plugin
// produces, or the server drops them for pointing at nothing.

func TestTableEdge_PinsThePostgreSQLPluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://asset/table/postgresql@host=pg:5432/shop.public.orders"},
		sqlTableMRNs("grafana-postgresql-datasource", "shop", "pg", "SELECT * FROM public.orders"))
}

func TestTableEdge_PinsTheMySQLPluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://asset/table/mysql@host=my:3306/shop.orders"},
		sqlTableMRNs("mysql", "shop", "my", "SELECT * FROM shop.orders"))
}

func TestTableEdge_PinsTheSQLServerPluginsMRN(t *testing.T) {
	// The SQL Server provider has a space in it, which mrn.New dashes,
	// landing on the same MRN plugins/openmetadata produces.
	assert.Equal(t, []string{"mrn://asset/table/sql-server/sales.dbo.orders"},
		sqlTableMRNs("mssql", "sales", "", "SELECT * FROM orders"))
}

func TestTableEdge_PinsTheClickHousePluginsMRN(t *testing.T) {
	assert.Equal(t, []string{"mrn://asset/table/clickhouse@host=ch:9000/default.events"},
		sqlTableMRNs("grafana-clickhouse-datasource", "default", "ch", "SELECT count() FROM default.events"))
}
