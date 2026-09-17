package grafana_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Grafana. They expect a
// Grafana seeded with: a PostgreSQL data source called Shop (the
// default, database shop with public.orders, public.customers and the
// view public.customer_totals), a TestData data source, a Sales folder
// holding the dashboard "Orders overview" (panels Revenue, a collapsed
// Details row holding Top customers, a Notes text panel and Random),
// "Ops home" in the General folder (an untitled gauge and the Customer
// totals table filtering on ${customer}) and "Library test" holding one
// library panel called Shared revenue.

func grafanaEnv(t *testing.T) (host, token string) {
	t.Helper()

	host = os.Getenv("MARMOT_TEST_GRAFANA_URL")
	token = os.Getenv("MARMOT_TEST_GRAFANA_TOKEN")
	if host == "" || token == "" {
		t.Skip("set MARMOT_TEST_GRAFANA_URL and MARMOT_TEST_GRAFANA_TOKEN to run the Grafana e2e tests")
	}
	return host, token
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	host, token := grafanaEnv(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"host": host, "api_key": token})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findByName(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	grafanaEnv(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "grafana", meta.ID)
	assert.Equal(t, "Grafana", meta.Name)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	_, token := grafanaEnv(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"api_key": token})
	require.Error(t, err)
}

func TestE2E_ValidateMissingTokenFails(t *testing.T) {
	host, _ := grafanaEnv(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{"host": host})
	require.Error(t, err)
}

func TestE2E_DiscoverFailsOnAWrongToken(t *testing.T) {
	host, _ := grafanaEnv(t)
	bin := buildBinary(t)

	_, err := bin.Discover(t.Context(), pluginsdk.RawConfig{"host": host, "api_key": "glsa_wrong"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestE2E_DiscoversTheSeededDashboards(t *testing.T) {
	result := discoverE2E(t)

	orders := findByName(result, "Dashboard", "Sales/Orders overview")
	require.NotNil(t, orders, "the dashboard in the Sales folder")
	assert.Equal(t, "mrn://dashboard/grafana/sales-orders-overview", *orders.MRN)
	assert.Equal(t, "orders-overview", orders.Metadata["uid"])
	assert.Equal(t, "Sales", orders.Metadata["folder"])
	assert.Equal(t, 3, int(orders.Metadata["panel_count"].(float64)))
	assert.Contains(t, orders.Tags, "sales")
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Revenue and top customers", *orders.Description)

	ops := findByName(result, "Dashboard", "Ops home")
	require.NotNil(t, ops, "the dashboard in the General folder is named by title alone")
	assert.Equal(t, "mrn://dashboard/grafana/ops-home", *ops.MRN)
}

func TestE2E_DiscoversCharts(t *testing.T) {
	result := discoverE2E(t)

	revenue := findByName(result, "Chart", "Sales/Orders overview/Revenue")
	require.NotNil(t, revenue)
	assert.Equal(t, "timeseries", revenue.Metadata["panel_type"])
	assert.Equal(t, "Line", revenue.Metadata["chart_type"])
	assert.Equal(t, "Shop", revenue.Metadata["datasource"])
	assert.Equal(t, "grafana-postgresql-datasource", revenue.Metadata["datasource_type"])
	require.NotNil(t, revenue.QueryLanguage)
	assert.Equal(t, "SQL", *revenue.QueryLanguage)

	assert.NotNil(t, findByName(result, "Chart", "Sales/Orders overview/Top customers"),
		"the panel inside the collapsed row is hoisted")
	assert.Nil(t, findByName(result, "Chart", "Sales/Orders overview/Details"), "the row is not a chart")
	assert.Nil(t, findByName(result, "Chart", "Sales/Orders overview/Notes"), "the text panel is skipped")
	assert.NotNil(t, findByName(result, "Chart", "Sales/Orders overview/Random"))

	assert.NotNil(t, findByName(result, "Chart", "Ops home/panel-1"), "an untitled panel is named by id")
	totals := findByName(result, "Chart", "Ops home/Customer totals")
	require.NotNil(t, totals)
	assert.Equal(t, "Shop", totals.Metadata["datasource"], "no data source on the panel means the default one")
}

func TestE2E_DiscoversDataSources(t *testing.T) {
	result := discoverE2E(t)

	shop := findByName(result, "DataSource", "Shop")
	require.NotNil(t, shop)
	assert.Equal(t, "grafana-postgresql-datasource", shop.Metadata["type"])
	assert.Equal(t, "PostgreSQL", shop.Metadata["type_name"])
	assert.Equal(t, "shop", shop.Metadata["database"])
	assert.Equal(t, true, shop.Metadata["is_default"])
	assert.NotContains(t, shop.Metadata, "password")
	assert.NotContains(t, shop.Metadata, "user")

	assert.NotNil(t, findByName(result, "DataSource", "TestData"))
}

func TestE2E_Lineage(t *testing.T) {
	result := discoverE2E(t)

	dash := "mrn://dashboard/grafana/sales-orders-overview"
	revenue := "mrn://chart/grafana/sales-orders-overview-revenue"
	topCustomers := "mrn://chart/grafana/sales-orders-overview-top-customers"
	random := "mrn://chart/grafana/sales-orders-overview-random"

	assert.True(t, hasEdge(result, dash, revenue, "CONTAINS"))
	assert.True(t, hasEdge(result, dash, topCustomers, "CONTAINS"))
	assert.True(t, hasEdge(result, dash, random, "CONTAINS"))

	assert.True(t, hasEdge(result, "mrn://datasource/grafana/shop", revenue, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://datasource/grafana/testdata", random, "FEEDS"))

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", revenue, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", revenue, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", dash, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", topCustomers, "FEEDS"))

	// The Ops home table panel filters on a template variable; that
	// yields the real table and nothing for the variable.
	totals := "mrn://chart/grafana/ops-home-customer-totals"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customer_totals", totals, "FEEDS"))
	for _, e := range result.Lineage {
		assert.NotContains(t, e.Source, "$")
	}
}

func TestE2E_HonoursIncludeToggles(t *testing.T) {
	host, token := grafanaEnv(t)
	bin := buildBinary(t)

	result, err := bin.Discover(t.Context(), pluginsdk.RawConfig{
		"host": host, "api_key": token,
		"include_panels": false, "include_datasources": false, "discover_lineage": false,
	})
	require.NoError(t, err)

	for _, a := range result.Assets {
		assert.Equal(t, "Dashboard", a.Type)
	}
	assert.Empty(t, result.Lineage)
}

func TestE2E_ResolvesLibraryPanels(t *testing.T) {
	result := discoverE2E(t)

	chart := findByName(result, "Chart", "Library test/Shared revenue")
	require.NotNil(t, chart, "the dashboard holding only a library panel reference")
	assert.Equal(t, "barchart", chart.Metadata["panel_type"])
	assert.Equal(t, "Shop", chart.Metadata["datasource"])
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", *chart.MRN, "FEEDS"))
}
