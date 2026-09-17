package metabase_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC
// wire protocol the Marmot host uses, against a real Metabase. They
// expect the content seeded by the Docker recipe in the README: a
// Postgres database "Shop" with public.customers, public.orders, the
// view public.recent_orders and sales.regions, a "Marmot" collection
// holding a "Finance" sub-collection, and the cards and dashboards
// asserted on below.

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_METABASE_URL")
	apiKey := os.Getenv("MARMOT_TEST_METABASE_API_KEY")
	if host == "" || apiKey == "" {
		t.Skip("set MARMOT_TEST_METABASE_URL and MARMOT_TEST_METABASE_API_KEY to run the Metabase e2e tests")
	}
	return pluginsdk.RawConfig{"host": host, "api_key": apiKey}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func discoverE2E(t *testing.T, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	for key, value := range overrides {
		config[key] = value
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

	assert.Equal(t, "metabase", meta.ID)
	assert.Equal(t, "Metabase", meta.Name)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingHostFails(t *testing.T) {
	e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"api_key": "k"})
	require.Error(t, err)
}

func TestE2E_ValidateWithoutCredentialsFails(t *testing.T) {
	config := e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"host": config["host"]})
	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheRealConfig(t *testing.T) {
	config := e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), config)
	require.NoError(t, err)
}

func TestE2E_DiscoverNamesDashboardsByCollectionPath(t *testing.T) {
	result := discoverE2E(t, nil)

	finance := findByMRN(result, "mrn://dashboard/metabase/marmot-finance-finance-overview")
	require.NotNil(t, finance, "dashboard in the Marmot/Finance collection")
	assert.Equal(t, "Marmot/Finance/Finance overview", *finance.Name)
	assert.Equal(t, "Dashboard", finance.Type)
	assert.Equal(t, []string{"Metabase"}, finance.Providers)
	assert.Equal(t, "Revenue and orders at a glance", *finance.Description)
	assert.Equal(t, "Marmot/Finance", finance.Metadata["collection"])
	assert.EqualValues(t, 4, finance.Metadata["card_count"], "numbers arrive as float64 over the wire")
	assert.Equal(t, false, finance.Metadata["archived"])

	root := findByMRN(result, "mrn://dashboard/metabase/root-board")
	require.NotNil(t, root, "dashboard in the root collection keeps its bare name")
	assert.Equal(t, "Root board", *root.Name)
	assert.Nil(t, root.Metadata["collection"])
}

func TestE2E_DiscoverLinksDashboardsToTheirCards(t *testing.T) {
	result := discoverE2E(t, nil)

	dash := "mrn://dashboard/metabase/marmot-finance-finance-overview"
	assert.True(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-finance-revenue-by-customer", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-finance-totals-from-model", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://chart/metabase/marmot-finance-recent-order-count", "CONTAINS"))
	assert.True(t, hasEdge(result, dash, "mrn://data-model-object/metabase/marmot-finance-customer-orders", "CONTAINS"))
}

func TestE2E_DiscoverCataloguesANativeQuestionAsAChartWithItsSQL(t *testing.T) {
	result := discoverE2E(t, nil)

	chart := findByMRN(result, "mrn://chart/metabase/marmot-finance-revenue-by-customer")
	require.NotNil(t, chart)
	assert.Equal(t, "Chart", chart.Type)
	assert.Equal(t, "Bar", chart.Metadata["chart_type"])
	assert.Equal(t, "bar", chart.Metadata["display"])
	assert.Equal(t, "question", chart.Metadata["card_type"])
	assert.Equal(t, "native", chart.Metadata["query_type"])
	assert.Equal(t, "Shop", chart.Metadata["database"])
	assert.Equal(t, "Total order value per customer", *chart.Description)
	require.NotNil(t, chart.Query)
	assert.Contains(t, *chart.Query, "FROM public.customers c")
	assert.Equal(t, "SQL", *chart.QueryLanguage)
	require.Len(t, chart.ExternalLinks, 1)
	assert.Equal(t, "Open in Metabase", chart.ExternalLinks[0].Name)
	assert.Contains(t, chart.ExternalLinks[0].URL, "/question/")
}

func TestE2E_DiscoverCataloguesAModelAsADataModelObject(t *testing.T) {
	result := discoverE2E(t, nil)

	model := findByMRN(result, "mrn://data-model-object/metabase/marmot-finance-customer-orders")
	require.NotNil(t, model)
	assert.Equal(t, "Data Model Object", model.Type)
	assert.Equal(t, "model", model.Metadata["card_type"])
	require.NotNil(t, model.Query)
	assert.Contains(t, *model.Query, "JOIN public.orders o")
}

func TestE2E_DiscoverLinksNativeCardsToTheirPostgresTables(t *testing.T) {
	result := discoverE2E(t, nil)

	chart := "mrn://chart/metabase/marmot-finance-revenue-by-customer"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", chart, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", chart, "FEEDS"))

	// The same tables feed the dashboard the card is on.
	dash := "mrn://dashboard/metabase/marmot-finance-finance-overview"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/customers", dash, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", dash, "FEEDS"))
}

func TestE2E_DiscoverResolvesQuotedNamesAndSchemasAndSkipsCTEs(t *testing.T) {
	result := discoverE2E(t, nil)

	chart := "mrn://chart/metabase/marmot-finance-recent-order-count"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/recent_orders", chart, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/regions", chart, "FEEDS"))
	assert.False(t, hasEdge(result, "mrn://table/postgresql/recent", chart, "FEEDS"), "a CTE is not a table")
}

func TestE2E_DiscoverLinksQueryBuilderCardsAndTheirJoins(t *testing.T) {
	result := discoverE2E(t, nil)

	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://chart/metabase/marmot-orders-table", "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", "mrn://dashboard/metabase/root-board", "FEEDS"))

	joined := "mrn://chart/metabase/marmot-orders-with-regions"
	assert.True(t, hasEdge(result, "mrn://table/postgresql/orders", joined, "FEEDS"))
	assert.True(t, hasEdge(result, "mrn://table/postgresql/regions", joined, "FEEDS"))
}

func TestE2E_DiscoverLinksAQuestionToTheModelItBuildsOn(t *testing.T) {
	result := discoverE2E(t, nil)

	assert.True(t, hasEdge(result,
		"mrn://data-model-object/metabase/marmot-finance-customer-orders",
		"mrn://chart/metabase/marmot-finance-totals-from-model", "FEEDS"))
}

func TestE2E_DiscoverAddressesSampleDatabaseTablesByItsEngine(t *testing.T) {
	result := discoverE2E(t, nil)

	// The built-in sample database is SQLite on current releases and H2
	// on older ones; either way the table is named by the engine's rule.
	chart := "mrn://chart/metabase/sample-orders"
	assert.True(t,
		hasEdge(result, "mrn://table/sqlite/orders", chart, "FEEDS") ||
			hasEdge(result, "mrn://table/h2/orders", chart, "FEEDS"),
		"expected the sample ORDERS table to feed the sample card")
}

func TestE2E_DiscoverSkipsArchivedItemsByDefault(t *testing.T) {
	result := discoverE2E(t, nil)

	assert.Nil(t, findByMRN(result, "mrn://chart/metabase/marmot-finance-old-revenue"))
	assert.Nil(t, findByMRN(result, "mrn://dashboard/metabase/marmot-finance-old-board"))
}

func TestE2E_DiscoverIncludesArchivedItemsWhenAsked(t *testing.T) {
	result := discoverE2E(t, pluginsdk.RawConfig{"include_archived": true})

	chart := findByMRN(result, "mrn://chart/metabase/marmot-finance-old-revenue")
	require.NotNil(t, chart)
	assert.Equal(t, true, chart.Metadata["archived"])

	dash := findByMRN(result, "mrn://dashboard/metabase/marmot-finance-old-board")
	require.NotNil(t, dash)
	assert.Equal(t, true, dash.Metadata["archived"])
}

func TestE2E_DiscoverWithSessionLogin(t *testing.T) {
	config := e2eConfig(t)
	username := os.Getenv("MARMOT_TEST_METABASE_USERNAME")
	password := os.Getenv("MARMOT_TEST_METABASE_PASSWORD")
	if username == "" || password == "" {
		t.Skip("set MARMOT_TEST_METABASE_USERNAME and MARMOT_TEST_METABASE_PASSWORD to test session login")
	}

	result, err := buildBinary(t).Discover(t.Context(), pluginsdk.RawConfig{
		"host": config["host"], "username": username, "password": password,
	})
	require.NoError(t, err)
	assert.NotNil(t, findByMRN(result, "mrn://dashboard/metabase/marmot-finance-finance-overview"))
}
