package amundsen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverDashboards_NamesADashboardByItsGroup(t *testing.T) {
	// Two groups in one BI tool routinely hold a dashboard of the same
	// name, so the group has to be part of the identity.
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Dashboard", "finance/revenue")
	assert.Equal(t, "mrn://dashboard/superset/finance-revenue", *asset.MRN)
}

func TestDiscoverDashboards_ProviderComesFromTheProduct(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, []string{"Superset"}, assetNamed(t, c, "Dashboard", "finance/revenue").Providers)
}

func TestDiscoverDashboards_DescriptionComesFromTheDescriptionNode(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Dashboard", "finance/revenue")
	require.NotNil(t, asset.Description)
	assert.Equal(t, "Monthly revenue by region", *asset.Description)
}

func TestDiscoverDashboards_ProvenanceCarriesTheGroupAndProduct(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	provenance := provenanceOf(t, assetNamed(t, c, "Dashboard", "finance/revenue"))
	assert.Equal(t, "superset_dashboard://prod.finance/revenue", provenance["key"])
	assert.Equal(t, "finance", provenance["group"])
	assert.Equal(t, "prod", provenance["cluster"])
	assert.Equal(t, "superset", provenance["product"])
	assert.Equal(t, "Finance reporting", provenance["group_description"])
	assert.Equal(t, "https://superset.marmot.test/dashboard/group/finance", provenance["group_url"])
}

func TestDiscoverDashboards_QueryNamesAreRecorded(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, []string{"revenue_by_region"}, provenanceOf(t, assetNamed(t, c, "Dashboard", "finance/revenue"))["query_names"])
}

func TestDiscoverDashboards_LastSuccessfulRunIsRecordedAsATimestamp(t *testing.T) {
	// Matching the execution by the suffix of its key rather than by the
	// position of a slash is what makes this land at all: a real key has
	// five segments, so the sixth OpenMetadata reads is always null.
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, "2025-09-04T15:53:54Z", provenanceOf(t, assetNamed(t, c, "Dashboard", "finance/revenue"))["last_successful_run"])
}

func TestDiscoverDashboards_TagsAndBadgesAreCarried(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Dashboard", "finance/revenue")
	assert.Equal(t, []string{"finance"}, asset.Tags)
	assert.Equal(t, []string{"verified"}, provenanceOf(t, asset)["badges"])
}

func TestDiscoverDashboards_ChartCountIsRecorded(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, int64(2), assetNamed(t, c, "Dashboard", "finance/revenue").Metadata["chart_count"])
}

func TestDiscoverDashboards_LinksToTheDashboardInItsOwnTool(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	links := assetNamed(t, c, "Dashboard", "finance/revenue").ExternalLinks
	require.NotEmpty(t, links)
	assert.Equal(t, "Open in Superset", links[0].Name)
	assert.Equal(t, "https://superset.marmot.test/dashboard/revenue", links[0].URL)
}

func TestDiscoverDashboards_LinksBackToTheAmundsenPage(t *testing.T) {
	config := testConfig()
	config.AmundsenURL = "https://amundsen.marmot.test"

	c := discover(t, config, seededGraph())

	links := assetNamed(t, c, "Dashboard", "finance/revenue").ExternalLinks
	require.Len(t, links, 2)
	assert.Equal(t, "Open in Amundsen", links[1].Name)
	assert.Equal(t, "https://amundsen.marmot.test/dashboard/superset_dashboard:%2F%2Fprod.finance%2Frevenue", links[1].URL)
}

func TestDiscoverDashboards_UsageBecomesStatistics(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Dashboard", "finance/revenue")
	reads, ok := statisticFor(c, *asset.MRN, metricReadCount)
	require.True(t, ok)
	assert.Equal(t, float64(7), reads)
}

func TestDiscoverDashboards_CanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeDashboards = false
	f := seededGraph()

	c := discover(t, config, f)

	for _, asset := range c.assets {
		assert.NotEqual(t, "Dashboard", asset.Type)
	}
	assert.Equal(t, 0, f.callsFor(dashboardQuery))
}

func TestDiscoverCharts_AreNamedByGroupDashboardAndChart(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Chart", "finance/revenue/Revenue by region")
	assert.Equal(t, "mrn://chart/superset/finance-revenue-revenue-by-region", *asset.MRN)
	assert.Equal(t, []string{"Superset"}, asset.Providers)
}

func TestDiscoverCharts_AreContainedByTheirDashboard(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	dashboard := assetNamed(t, c, "Dashboard", "finance/revenue")
	for _, name := range []string{"finance/revenue/Revenue by region", "finance/revenue/Revenue trend"} {
		chart := assetNamed(t, c, "Chart", name)
		assert.True(t, hasEdge(c, *dashboard.MRN, *chart.MRN, "CONTAINS"), "missing CONTAINS edge for %s", name)
	}
}

func TestDiscoverCharts_CarryTheirTypeAndURL(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Chart", "finance/revenue/Revenue by region")
	assert.Equal(t, "bar", asset.Metadata["chart_type"])
	assert.Equal(t, "c1", provenanceOf(t, asset)["chart_id"])
	require.NotEmpty(t, asset.ExternalLinks)
	assert.Equal(t, "https://superset.marmot.test/chart/c1", asset.ExternalLinks[0].URL)
}

func TestDiscoverCharts_FallBackToTheirIDWhenUnnamed(t *testing.T) {
	row := revenueDashboardRow()
	row["charts"] = []any{
		map[string]any{"name": nil, "id": "c9", "type": "table", "url": nil},
	}
	f := newFakeReader()
	f.rows[dashboardQuery] = []map[string]any{row}

	c := discover(t, testConfig(), f)

	assert.Equal(t, "mrn://chart/superset/finance-revenue-c9", *assetNamed(t, c, "Chart", "finance/revenue/c9").MRN)
}

func TestDiscoverCharts_ADashboardWithNoChartsMintsNone(t *testing.T) {
	// Neo4j answers the COLLECT with one map of nulls, which must not
	// become a nameless chart.
	row := revenueDashboardRow()
	row["charts"] = []any{
		map[string]any{"name": nil, "id": nil, "type": nil, "url": nil},
	}
	f := newFakeReader()
	f.rows[dashboardQuery] = []map[string]any{row}

	c := discover(t, testConfig(), f)

	require.Len(t, c.assets, 1)
	assert.Equal(t, "Dashboard", c.assets[0].Type)
}

func TestDiscoverDashboardLineage_TableFeedsDashboard(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.True(t, hasEdge(c, "mrn://table/postgresql/orders", "mrn://dashboard/superset/finance-revenue", "FEEDS"))
}

func TestDiscoverDashboardLineage_IgnoresADashboardThisRunDidNotImport(t *testing.T) {
	f := seededGraph()
	f.rows[dashboardTableQuery] = []map[string]any{
		{"dashboard_key": "mode_dashboard://prod.ops/uptime", "table_key": "postgres://prod.public/orders"},
	}

	c := discover(t, testConfig(), f)

	for _, edge := range c.lineage {
		assert.NotEqual(t, "mrn://dashboard/mode/ops-uptime", edge.Target)
	}
}

func TestDiscoverDashboardLineage_IsRecordedOnlyOnce(t *testing.T) {
	// The relationship exists in both directions, so an undirected match
	// over both types can return the same pair twice.
	f := seededGraph()
	f.rows[dashboardTableQuery] = append(f.rows[dashboardTableQuery], map[string]any{
		"dashboard_key": "superset_dashboard://prod.finance/revenue",
		"table_key":     "postgres://prod.public/orders",
	})

	c := discover(t, testConfig(), f)

	edges := 0
	for _, edge := range c.lineage {
		if edge.Target == "mrn://dashboard/superset/finance-revenue" {
			edges++
		}
	}
	assert.Equal(t, 1, edges)
}

func TestDashboardName_DropsTheSlashWhenThereIsNoGroup(t *testing.T) {
	assert.Equal(t, "revenue", dashboardName("", "revenue"))
}

func TestDashboardProduct_ComesFromTheQueryWhenPresent(t *testing.T) {
	assert.Equal(t, "superset", dashboardProduct(map[string]any{"product": "superset", "key": "mode_dashboard://a/b"}))
}

func TestDashboardProduct_FallsBackToTheKeyScheme(t *testing.T) {
	assert.Equal(t, "mode", dashboardProduct(map[string]any{"key": "mode_dashboard://prod.ops/uptime"}))
}

func TestDashboardProduct_IsEmptyForAKeyWithNoScheme(t *testing.T) {
	assert.Equal(t, "", dashboardProduct(map[string]any{"key": "uptime"}))
}
