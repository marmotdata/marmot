package amundsen_test

import (
	"encoding/json"
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC
// wire protocol the Marmot host uses, against a real Neo4j holding an
// Amundsen shaped graph: plugintest.Build compiles the main package and
// every call spawns the process, runs one RPC and kills it again.
//
// Bring the graph up with:
//
//	docker run -d --name marmot-test-amundsen -p 17687:7687 -p 17474:7474 \
//	  -e NEO4J_AUTH=neo4j/marmotpass -e NEO4J_PLUGINS='[]' neo4j:5-community
//	docker cp seed.cypher marmot-test-amundsen:/tmp/seed.cypher
//	docker exec marmot-test-amundsen cypher-shell -u neo4j -p marmotpass -f /tmp/seed.cypher
//
// then run with MARMOT_TEST_AMUNDSEN_URI=bolt://localhost:17687,
// MARMOT_TEST_AMUNDSEN_USER=neo4j and MARMOT_TEST_AMUNDSEN_PASSWORD set.
// The seed is the graph in plugins/amundsen/testdata/seed.cypher.

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// graphConfig is the connection to the seeded Amundsen graph, or a skip
// when the test environment does not have one.
func graphConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	uri := os.Getenv("MARMOT_TEST_AMUNDSEN_URI")
	if uri == "" {
		t.Skip("set MARMOT_TEST_AMUNDSEN_URI (for example bolt://localhost:17687) to run the Amundsen end to end tests")
	}

	return pluginsdk.RawConfig{
		"uri":          uri,
		"username":     os.Getenv("MARMOT_TEST_AMUNDSEN_USER"),
		"password":     os.Getenv("MARMOT_TEST_AMUNDSEN_PASSWORD"),
		"amundsen_url": "https://amundsen.marmot.test",
	}
}

// discoverGraph runs one discovery over the wire.
func discoverGraph(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := graphConfig(t)
	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func assetWithMRN(t *testing.T, result *pluginsdk.DiscoveryResult, mrn string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.MRN != nil && *asset.MRN == mrn {
			return asset
		}
	}
	t.Fatalf("no asset with MRN %s", mrn)
	return pluginsdk.Asset{}
}

func hasLineage(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func statistic(result *pluginsdk.DiscoveryResult, mrn, metric string) (float64, bool) {
	for _, s := range result.Statistics {
		if s.AssetMRN == mrn && s.MetricName == metric {
			return s.Value, true
		}
	}
	return 0, false
}

func TestE2E_Meta(t *testing.T) {
	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "amundsen", meta.ID)
	assert.Equal(t, "Amundsen", meta.Name)
	assert.Equal(t, "catalog", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

func TestE2E_ValidateMissingURIFails(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"username": "neo4j", "password": "marmotpass"})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheGraphConfig(t *testing.T) {
	config := graphConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), config)

	require.NoError(t, err)
}

func TestE2E_DiscoverWithBadCredentialsFails(t *testing.T) {
	config := graphConfig(t)
	config["password"] = "not-the-password"

	_, err := buildBinary(t).Discover(t.Context(), config)

	require.Error(t, err)
}

func TestE2E_PostgresTableIsProjectedOntoThePostgreSQLPlugin(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://table/postgresql/orders")
	assert.Equal(t, "Table", asset.Type)
	assert.Equal(t, []string{"PostgreSQL"}, asset.Providers)
	require.NotNil(t, asset.Description)
	assert.Equal(t, "One row per placed order", *asset.Description)
}

func TestE2E_HiveTableIsProjectedOntoASchemaQualifiedName(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://table/hive/sales.orders")
	assert.Equal(t, []string{"Hive"}, asset.Providers)
	assert.Equal(t, "sales.orders", *asset.Name)
}

func TestE2E_HiveViewIsTypedAsAView(t *testing.T) {
	result := discoverGraph(t)

	assert.Equal(t, "View", assetWithMRN(t, result, "mrn://view/hive/sales.orders_view").Type)
}

func TestE2E_ColumnsArriveInSortOrderWithTheirDescriptions(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://table/postgresql/orders")
	encoded, ok := asset.Schema["columns"]
	require.True(t, ok, "expected a column list")

	var columns []map[string]any
	require.NoError(t, json.Unmarshal([]byte(encoded), &columns))
	require.Len(t, columns, 3)

	assert.Equal(t, "id", columns[0]["column_name"])
	assert.Equal(t, "integer", columns[0]["data_type"])
	assert.Equal(t, "Surrogate order key", columns[0]["description"])

	assert.Equal(t, "customer_id", columns[1]["column_name"])
	assert.Equal(t, "References customers.id", columns[1]["description"])

	assert.Equal(t, "total", columns[2]["column_name"])
	assert.Equal(t, "numeric", columns[2]["data_type"])
}

func TestE2E_OwnersTagsAndBadgesSurviveTheWire(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://table/postgresql/orders")
	assert.ElementsMatch(t, []string{"finance", "pii"}, asset.Tags)
	assert.ElementsMatch(t, []any{"Ann Ops", "Bo Analyst"}, asset.Metadata["owners"])

	provenance, ok := asset.Metadata["amundsen"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "postgres://prod.public/orders", provenance["key"])
	assert.Equal(t, "prod", provenance["cluster"])
	assert.ElementsMatch(t, []any{"certified"}, provenance["badges"])
	assert.ElementsMatch(t, []any{"Built nightly by dbt model orders"}, provenance["programmatic_descriptions"])
	assert.Equal(t, "Customer facing tables", asset.Metadata["schema_description"])
}

func TestE2E_UsageCountsAreNotInflatedByTheOtherJoins(t *testing.T) {
	// The graph holds two reads of orders, of 30 and 12. A single query
	// that also matched tags, badges and columns would report a multiple
	// of 42 instead.
	result := discoverGraph(t)

	reads, ok := statistic(result, "mrn://table/postgresql/orders", "asset.read_count")
	require.True(t, ok)
	assert.Equal(t, float64(42), reads)

	readers, ok := statistic(result, "mrn://table/postgresql/orders", "asset.unique_readers")
	require.True(t, ok)
	assert.Equal(t, float64(2), readers)
}

func TestE2E_TableFeedsTableLineage(t *testing.T) {
	result := discoverGraph(t)

	assert.True(t, hasLineage(result, "mrn://table/postgresql/customers", "mrn://table/postgresql/orders", "FEEDS"),
		"customers is the upstream of orders")
}

func TestE2E_DashboardIsNamedByItsGroup(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://dashboard/superset/finance-revenue")
	assert.Equal(t, "Dashboard", asset.Type)
	assert.Equal(t, []string{"Superset"}, asset.Providers)
	assert.Equal(t, "finance/revenue", *asset.Name)
}

func TestE2E_DashboardLastSuccessfulRunIsRead(t *testing.T) {
	// Matching the execution node by the suffix of its key is what makes
	// this land: the positional filter Amundsen and OpenMetadata use
	// reads a sixth key segment that does not exist.
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://dashboard/superset/finance-revenue")
	provenance, ok := asset.Metadata["amundsen"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2025-09-04T15:53:54Z", provenance["last_successful_run"])
}

func TestE2E_DashboardContainsItsCharts(t *testing.T) {
	result := discoverGraph(t)

	for _, chart := range []string{
		"mrn://chart/superset/finance-revenue-revenue-by-region",
		"mrn://chart/superset/finance-revenue-revenue-trend",
	} {
		assert.Equal(t, "Chart", assetWithMRN(t, result, chart).Type)
		assert.True(t, hasLineage(result, "mrn://dashboard/superset/finance-revenue", chart, "CONTAINS"),
			"missing CONTAINS edge for %s", chart)
	}
}

func TestE2E_TableFeedsDashboardLineage(t *testing.T) {
	result := discoverGraph(t)

	assert.True(t, hasLineage(result, "mrn://table/postgresql/orders", "mrn://dashboard/superset/finance-revenue", "FEEDS"))
}

func TestE2E_DashboardUsageBecomesAStatistic(t *testing.T) {
	result := discoverGraph(t)

	reads, ok := statistic(result, "mrn://dashboard/superset/finance-revenue", "asset.read_count")
	require.True(t, ok)
	assert.Equal(t, float64(7), reads)
}

func TestE2E_AssetsLinkBackToAmundsen(t *testing.T) {
	result := discoverGraph(t)

	asset := assetWithMRN(t, result, "mrn://table/postgresql/orders")
	require.NotEmpty(t, asset.ExternalLinks)
	assert.Equal(t, "https://amundsen.marmot.test/table_detail/prod/postgres/public/orders", asset.ExternalLinks[0].URL)
}

func TestE2E_DashboardsCanBeSwitchedOff(t *testing.T) {
	config := graphConfig(t)
	config["include_dashboards"] = false

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	for _, asset := range result.Assets {
		assert.NotEqual(t, "Dashboard", asset.Type)
		assert.NotEqual(t, "Chart", asset.Type)
	}
}

func TestE2E_PagingReadsTheWholeGraph(t *testing.T) {
	// A page size of one forces every query through several round trips,
	// which is where an unstable ORDER BY would drop or repeat records.
	config := graphConfig(t)
	config["page_size"] = 1

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)

	seen := map[string]int{}
	for _, asset := range result.Assets {
		seen[*asset.MRN]++
	}
	for mrn, count := range seen {
		assert.Equal(t, 1, count, "%s was discovered more than once", mrn)
	}

	assert.Contains(t, seen, "mrn://table/postgresql/orders")
	assert.Contains(t, seen, "mrn://table/postgresql/customers")
	assert.Contains(t, seen, "mrn://table/hive/sales.orders")
	assert.Contains(t, seen, "mrn://view/hive/sales.orders_view")
	assert.Contains(t, seen, "mrn://dashboard/superset/finance-revenue")
}
