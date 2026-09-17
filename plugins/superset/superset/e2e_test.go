package superset_test

import (
	"encoding/json"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Superset. They expect the
// objects seeded during development: a "Shop" Postgres connection,
// physical datasets public.orders (with a total_with_tax calculated
// column), public.customers and inventory.products, a virtual dataset
// public.order_totals joining orders and customers, charts "Orders by
// Status" (table, on Shop Overview), "Spend per Customer" (pie, on Shop
// Overview) and "Product Prices" (on the draft), the published dashboard
// "Shop Overview" tagged "finance" and the draft "Draft Scratchpad".

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_SUPERSET_URL")
	if host == "" {
		t.Skip("MARMOT_TEST_SUPERSET_URL not set; skipping Superset e2e tests")
	}

	return pluginsdk.RawConfig{
		"host":     host,
		"username": os.Getenv("MARMOT_TEST_SUPERSET_USERNAME"),
		"password": os.Getenv("MARMOT_TEST_SUPERSET_PASSWORD"),
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	for k, v := range overrides {
		config[k] = v
	}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func findByMRN(result *pluginsdk.DiscoveryResult, mrn string) *pluginsdk.Asset {
	for i := range result.Assets {
		if result.Assets[i].MRN != nil && *result.Assets[i].MRN == mrn {
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
	e2eConfig(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "superset", meta.ID)
	assert.Equal(t, "Apache Superset", meta.Name)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "host")

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_ValidateMissingPasswordFails(t *testing.T) {
	config := e2eConfig(t)
	delete(config, "password")

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.Error(t, err)
}

func TestE2E_DiscoverFailsWithWrongCredentials(t *testing.T) {
	config := e2eConfig(t)
	config["password"] = "definitely-wrong"

	_, err := buildBinary(t).Discover(t.Context(), config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "logging in")
}

func TestE2E_DiscoverDashboards(t *testing.T) {
	result := discoverE2E(t, nil)

	overview := findByMRN(result, "mrn://dashboard/superset/shop-overview")
	require.NotNil(t, overview)
	assert.Equal(t, "Dashboard", overview.Type)
	assert.Equal(t, []string{"Superset"}, overview.Providers)
	assert.Equal(t, "Shop Overview", *overview.Name)
	assert.Equal(t, "shop-overview", overview.Metadata["slug"])
	assert.Equal(t, true, overview.Metadata["published"])
	assert.Equal(t, "published", overview.Metadata["status"])
	assert.Equal(t, float64(2), overview.Metadata["chart_count"], "two charts sit on the dashboard")
	assert.Contains(t, overview.Metadata["url"], "/superset/dashboard/shop-overview/")
	assert.Contains(t, overview.Tags, "finance")
	require.Len(t, overview.ExternalLinks, 1)
	assert.Equal(t, "Open in Superset", overview.ExternalLinks[0].Name)

	draft := findByMRN(result, "mrn://dashboard/superset/draft-scratchpad")
	require.NotNil(t, draft)
	assert.Equal(t, false, draft.Metadata["published"])
}

func TestE2E_SkipsDraftsWhenConfigured(t *testing.T) {
	result := discoverE2E(t, pluginsdk.RawConfig{"include_draft": false})

	assert.NotNil(t, findByMRN(result, "mrn://dashboard/superset/shop-overview"))
	assert.Nil(t, findByMRN(result, "mrn://dashboard/superset/draft-scratchpad"))
}

func TestE2E_DiscoverCharts(t *testing.T) {
	result := discoverE2E(t, nil)

	orders := findByMRN(result, "mrn://chart/superset/orders-by-status")
	require.NotNil(t, orders)
	assert.Equal(t, "Chart", orders.Type)
	assert.Equal(t, "table", orders.Metadata["viz_type"])
	assert.Equal(t, "Table", orders.Metadata["chart_type"])
	assert.Equal(t, "public.orders", orders.Metadata["dataset"])
	assert.Equal(t, []interface{}{"Shop Overview"}, orders.Metadata["dashboards"])
	require.NotNil(t, orders.Description)
	assert.Equal(t, "Count of orders per status", *orders.Description)
	assert.Contains(t, orders.Metadata["url"], "/explore/?slice_id=")

	spend := findByMRN(result, "mrn://chart/superset/spend-per-customer")
	require.NotNil(t, spend)
	assert.Equal(t, "Pie", spend.Metadata["chart_type"])

	prices := findByMRN(result, "mrn://chart/superset/product-prices")
	require.NotNil(t, prices)
	assert.Equal(t, "Bar", prices.Metadata["chart_type"])
}

func TestE2E_DiscoverPhysicalDatasetWithColumns(t *testing.T) {
	result := discoverE2E(t, nil)

	orders := findByMRN(result, "mrn://data-model-object/superset/public.orders")
	require.NotNil(t, orders)
	assert.Equal(t, "Data Model Object", orders.Type)
	assert.Equal(t, "public.orders", *orders.Name)
	assert.Equal(t, "physical", orders.Metadata["kind"])
	assert.Equal(t, "Shop", orders.Metadata["database"])
	assert.Equal(t, "postgresql", orders.Metadata["backend"])
	assert.Equal(t, "public", orders.Metadata["schema"])
	assert.Equal(t, "orders", orders.Metadata["table_name"])
	assert.Nil(t, orders.Query)
	require.NotNil(t, orders.Description)
	assert.Equal(t, "One row per customer order", *orders.Description)

	var columns []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(orders.Schema["columns"]), &columns))
	byName := make(map[string]map[string]interface{})
	for _, c := range columns {
		byName[c["column_name"].(string)] = c
	}
	require.Contains(t, byName, "total")
	require.Contains(t, byName, "status")
	require.Contains(t, byName, "total_with_tax")
	assert.Equal(t, "NUMERIC(10, 2)", byName["total"]["data_type"])
	assert.Equal(t, "Order lifecycle state", byName["status"]["description"])
	assert.Equal(t, "total * 1.21", byName["total_with_tax"]["expression"])

	products := findByMRN(result, "mrn://data-model-object/superset/inventory.products")
	require.NotNil(t, products)
	assert.Equal(t, "inventory", products.Metadata["schema"])
}

func TestE2E_DiscoverVirtualDatasetWithQuery(t *testing.T) {
	result := discoverE2E(t, nil)

	totals := findByMRN(result, "mrn://data-model-object/superset/public.order_totals")
	require.NotNil(t, totals)
	assert.Equal(t, "virtual", totals.Metadata["kind"])
	require.NotNil(t, totals.Query)
	assert.Contains(t, *totals.Query, "FROM public.orders o JOIN public.customers c")
	require.NotNil(t, totals.QueryLanguage)
	assert.Equal(t, "SQL", *totals.QueryLanguage)
}

func TestE2E_DiscoverDatabaseConnection(t *testing.T) {
	result := discoverE2E(t, nil)

	shop := findByMRN(result, "mrn://datasource/superset/shop")
	require.NotNil(t, shop)
	assert.Equal(t, "DataSource", shop.Type)
	assert.Equal(t, "Shop", *shop.Name)
	assert.Equal(t, "postgresql", shop.Metadata["backend"])
	assert.Equal(t, "psycopg2", shop.Metadata["driver"])
	assert.Equal(t, "marmot-test-superset-pg", shop.Metadata["host"])
	assert.Equal(t, "5432", shop.Metadata["port"])
	assert.Equal(t, "shop", shop.Metadata["database"])
	assert.Equal(t, true, shop.Metadata["expose_in_sqllab"])
	assert.NotContains(t, shop.Metadata["sqlalchemy_uri"], "marmot:marmot", "the password never reaches the catalog")
}

func TestE2E_Lineage(t *testing.T) {
	result := discoverE2E(t, nil)

	// Dashboard -> charts
	assert.True(t, hasEdge(result, "mrn://dashboard/superset/shop-overview", "mrn://chart/superset/orders-by-status", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://dashboard/superset/shop-overview", "mrn://chart/superset/spend-per-customer", "CONTAINS"))
	assert.True(t, hasEdge(result, "mrn://dashboard/superset/draft-scratchpad", "mrn://chart/superset/product-prices", "CONTAINS"))

	// Dataset -> chart
	assert.True(t, hasEdge(result, "mrn://data-model-object/superset/public.orders", "mrn://chart/superset/orders-by-status", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://data-model-object/superset/public.order_totals", "mrn://chart/superset/spend-per-customer", "FEEDS"))

	// Connection -> dataset
	assert.True(t, hasEdge(result, "mrn://datasource/superset/shop", "mrn://data-model-object/superset/public.orders", "FEEDS"))

	// Postgres table -> dataset, physical and through the virtual SQL
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/superset/public.orders", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/products", "mrn://data-model-object/superset/inventory.products", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/superset/public.order_totals", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", "mrn://data-model-object/superset/public.order_totals", "FEEDS"))
}
