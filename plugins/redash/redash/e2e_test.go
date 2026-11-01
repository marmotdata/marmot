package redash_test

import (
	"os"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Redash. plugintest.Build
// compiles the main package and every call spawns the process, runs one RPC
// and kills it again.
//
// Run them with a Redash to point at:
//
//	MARMOT_TEST_REDASH_URL=http://localhost:15000 \
//	MARMOT_TEST_REDASH_API_KEY=<user api key> go test ./redash/ -run E2E
//
// The seeded fixture the assertions expect is a data source "shop" (pg), a
// published query "Orders by day" reading public.orders and public.customers,
// a draft query "Customer totals", an archived query "Old report", a
// published dashboard "Revenue" with a column chart, a counter and two text
// widgets, and a draft dashboard "Scratch".

func e2eConfig(t *testing.T) pluginsdk.RawConfig {
	t.Helper()

	host := os.Getenv("MARMOT_TEST_REDASH_URL")
	if host == "" {
		t.Skip("MARMOT_TEST_REDASH_URL is not set, skipping the Redash end to end tests")
	}

	return pluginsdk.RawConfig{
		"host":    host,
		"api_key": os.Getenv("MARMOT_TEST_REDASH_API_KEY"),
	}
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func e2eDiscover(t *testing.T, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	config := e2eConfig(t)
	for key, value := range overrides {
		config[key] = value
	}

	result, err := buildBinary(t).Discover(t.Context(), config)
	require.NoError(t, err)
	return result
}

func findAsset(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return asset
		}
	}

	var found []string
	for _, asset := range result.Assets {
		found = append(found, asset.Type+" "+*asset.Name)
	}
	require.Failf(t, "asset not found", "no %s named %q, discovered: %v", assetType, name, found)
	return pluginsdk.Asset{}
}

func findsAsset(result *pluginsdk.DiscoveryResult, assetType, name string) bool {
	for _, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil && *asset.Name == name {
			return true
		}
	}
	return false
}

func findsEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	e2eConfig(t)

	meta, err := buildBinary(t).Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "redash", meta.ID)
	assert.Equal(t, "Redash", meta.Name)
	assert.Equal(t, "dashboard", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
}

func TestE2E_ValidateRejectsAMissingHost(t *testing.T) {
	e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"api_key": "k"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "host is required")
}

func TestE2E_ValidateRejectsAMissingAPIKey(t *testing.T) {
	e2eConfig(t)

	_, err := buildBinary(t).Validate(t.Context(), pluginsdk.RawConfig{"host": "http://localhost:15000"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestE2E_ValidateAcceptsTheRealConfig(t *testing.T) {
	_, err := buildBinary(t).Validate(t.Context(), e2eConfig(t))

	require.NoError(t, err)
}

func TestE2E_DiscoversThePublishedDashboard(t *testing.T) {
	result := e2eDiscover(t, nil)

	dashboard := findAsset(t, result, "Dashboard", "Revenue")

	assert.Equal(t, "mrn://dashboard/redash/revenue", *dashboard.MRN)
	assert.Equal(t, []string{"Redash"}, dashboard.Providers)
	assert.Equal(t, false, dashboard.Metadata["is_draft"])
	assert.Equal(t, "revenue", dashboard.Metadata["slug"])
	assert.Equal(t, "Marmot Admin", dashboard.Metadata["owner"])
}

func TestE2E_DashboardCountsItsWidgetsAndQueries(t *testing.T) {
	result := e2eDiscover(t, nil)

	dashboard := findAsset(t, result, "Dashboard", "Revenue")

	// A column chart, a counter and two text widgets, reading two queries.
	// Metadata crosses the plugin wire as JSON, so a number arrives as a
	// float64 and the assertions compare by value rather than by type.
	assert.EqualValues(t, 4, dashboard.Metadata["widget_count"])
	assert.EqualValues(t, 2, dashboard.Metadata["query_count"])
}

func TestE2E_DashboardNotesComeFromTheTextWidgets(t *testing.T) {
	result := e2eDiscover(t, nil)

	dashboard := findAsset(t, result, "Dashboard", "Revenue")

	assert.Equal(t, "Revenue overview for the shop database.\n\nUpdated nightly.", dashboard.Metadata["notes"])
}

func TestE2E_DashboardCarriesItsRedashTags(t *testing.T) {
	result := e2eDiscover(t, nil)

	dashboard := findAsset(t, result, "Dashboard", "Revenue")

	assert.Contains(t, dashboard.Tags, "finance")
	assert.Contains(t, dashboard.Tags, "daily")
}

func TestE2E_DashboardURLUsesTheIDAndSlugForm(t *testing.T) {
	result := e2eDiscover(t, nil)

	dashboard := findAsset(t, result, "Dashboard", "Revenue")

	assert.Equal(t, os.Getenv("MARMOT_TEST_REDASH_URL")+"/dashboards/1-revenue", dashboard.Metadata["url"])
}

func TestE2E_DiscoversTheColumnChartWithItsNormalisedType(t *testing.T) {
	result := e2eDiscover(t, nil)

	chart := findAsset(t, result, "Chart", "Revenue/Orders by day")

	assert.Equal(t, "mrn://chart/redash/revenue-orders-by-day", *chart.MRN)
	assert.Equal(t, "CHART", chart.Metadata["visualization_type"])
	// Read out of options.globalSeriesType, which is where Redash keeps the
	// real shape.
	assert.Equal(t, "Bar", chart.Metadata["chart_type"])
	assert.Equal(t, "shop", chart.Metadata["data_source"])
}

func TestE2E_DiscoversTheCounterAsAText(t *testing.T) {
	result := e2eDiscover(t, nil)

	chart := findAsset(t, result, "Chart", "Revenue/Total revenue")

	assert.Equal(t, "COUNTER", chart.Metadata["visualization_type"])
	assert.Equal(t, "Text", chart.Metadata["chart_type"])
}

func TestE2E_TextWidgetsDoNotBecomeCharts(t *testing.T) {
	result := e2eDiscover(t, nil)

	charts := 0
	for _, asset := range result.Assets {
		if asset.Type == "Chart" {
			charts++
		}
	}

	assert.Equal(t, 2, charts)
}

func TestE2E_DiscoversThePublishedQueryWithItsSQL(t *testing.T) {
	result := e2eDiscover(t, nil)

	query := findAsset(t, result, "Data Model Object", "Orders by day")

	assert.Equal(t, "mrn://data-model-object/redash/orders-by-day", *query.MRN)
	require.NotNil(t, query.Query)
	assert.Contains(t, *query.Query, "FROM public.orders o JOIN public.customers c")
	require.NotNil(t, query.QueryLanguage)
	assert.Equal(t, "SQL", *query.QueryLanguage)
	assert.Equal(t, "shop", query.Metadata["data_source"])
}

func TestE2E_QueryRecordsItsParameters(t *testing.T) {
	result := e2eDiscover(t, nil)

	query := findAsset(t, result, "Data Model Object", "Orders by day")

	parameters, ok := query.Metadata["parameters"].([]any)
	require.True(t, ok, "parameters is %T", query.Metadata["parameters"])
	assert.Equal(t, []any{map[string]any{"name": "min_total", "title": "Min total", "type": "number"}}, parameters)
}

func TestE2E_QueryRecordsItsSchedule(t *testing.T) {
	result := e2eDiscover(t, nil)

	query := findAsset(t, result, "Data Model Object", "Customer totals")

	schedule, ok := query.Metadata["schedule"].(map[string]any)
	require.True(t, ok, "schedule is %T", query.Metadata["schedule"])
	assert.EqualValues(t, 86400, schedule["interval"])
	assert.Equal(t, "03:00", schedule["time"])
}

func TestE2E_QueryNeverRecordsItsAPIKey(t *testing.T) {
	// Redash returns a per-query api key on every query payload.
	result := e2eDiscover(t, nil)

	query := findAsset(t, result, "Data Model Object", "Orders by day")

	for key := range query.Metadata {
		assert.NotContains(t, key, "api_key")
	}
}

func TestE2E_DiscoversTheDataSourceWithItsConnectionDetails(t *testing.T) {
	result := e2eDiscover(t, nil)

	source := findAsset(t, result, "DataSource", "shop")

	assert.Equal(t, "mrn://datasource/redash/shop", *source.MRN)
	assert.Equal(t, "pg", source.Metadata["type"])
	assert.Equal(t, "sql", source.Metadata["syntax"])
	assert.Equal(t, "marmot-test-redash-source", source.Metadata["host"])
	assert.Equal(t, "shop", source.Metadata["dbname"])
	assert.Equal(t, "marmot", source.Metadata["user"])
	assert.Equal(t, false, source.Metadata["paused"])
}

func TestE2E_DataSourceNeverRecordsAPassword(t *testing.T) {
	result := e2eDiscover(t, nil)

	source := findAsset(t, result, "DataSource", "shop")

	for key, value := range source.Metadata {
		assert.NotContains(t, key, "password")
		assert.NotEqual(t, "--------", value)
		assert.NotEqual(t, "marmot", key)
	}
}

func TestE2E_DashboardContainsItsCharts(t *testing.T) {
	result := e2eDiscover(t, nil)

	assert.True(t, findsEdge(result, "mrn://dashboard/redash/revenue", "mrn://chart/redash/revenue-orders-by-day", "CONTAINS"))
	assert.True(t, findsEdge(result, "mrn://dashboard/redash/revenue", "mrn://chart/redash/revenue-total-revenue", "CONTAINS"))
}

func TestE2E_QueryFeedsTheChartThatRendersIt(t *testing.T) {
	result := e2eDiscover(t, nil)

	assert.True(t, findsEdge(result, "mrn://data-model-object/redash/orders-by-day", "mrn://chart/redash/revenue-orders-by-day", "FEEDS"))
}

func TestE2E_DataSourceFeedsTheQuery(t *testing.T) {
	result := e2eDiscover(t, nil)

	assert.True(t, findsEdge(result, "mrn://datasource/redash/shop", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
}

func TestE2E_PostgresTablesFeedTheQueryThatReadsThem(t *testing.T) {
	// The table MRNs are the ones plugins/postgresql produces, so the edge
	// merges with that plugin's assets rather than minting new ones.
	result := e2eDiscover(t, nil)

	assert.True(t, findsEdge(result, "mrn://table/postgresql/orders", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
	assert.True(t, findsEdge(result, "mrn://table/postgresql/customers", "mrn://data-model-object/redash/orders-by-day", "FEEDS"))
}

func TestE2E_IncludesDraftsByDefault(t *testing.T) {
	result := e2eDiscover(t, nil)

	assert.True(t, findsAsset(result, "Dashboard", "Scratch"))
	assert.True(t, findsAsset(result, "Data Model Object", "Customer totals"))
}

func TestE2E_ExcludesDraftsWhenTurnedOff(t *testing.T) {
	result := e2eDiscover(t, pluginsdk.RawConfig{"include_drafts": false})

	assert.False(t, findsAsset(result, "Dashboard", "Scratch"))
	assert.False(t, findsAsset(result, "Data Model Object", "Customer totals"))
	assert.True(t, findsAsset(result, "Dashboard", "Revenue"))
}

func TestE2E_ExcludesArchivedQueriesByDefault(t *testing.T) {
	result := e2eDiscover(t, nil)

	assert.False(t, findsAsset(result, "Data Model Object", "Old report"))
}

func TestE2E_IncludesArchivedQueriesWhenAsked(t *testing.T) {
	// Redash serves archived queries from /api/queries/archive, not from
	// the main query list.
	result := e2eDiscover(t, pluginsdk.RawConfig{"include_archived": true})

	assert.True(t, findsAsset(result, "Data Model Object", "Old report"))
}

func TestE2E_OmitsQueriesWhenTurnedOff(t *testing.T) {
	result := e2eDiscover(t, pluginsdk.RawConfig{"include_queries": false})

	assert.False(t, findsAsset(result, "Data Model Object", "Orders by day"))
	assert.True(t, findsAsset(result, "Dashboard", "Revenue"))
}

func TestE2E_OmitsDataSourcesWhenTurnedOff(t *testing.T) {
	result := e2eDiscover(t, pluginsdk.RawConfig{"include_data_sources": false})

	assert.False(t, findsAsset(result, "DataSource", "shop"))
	// The data source is still read, so the query still names it.
	assert.Equal(t, "shop", findAsset(t, result, "Data Model Object", "Orders by day").Metadata["data_source"])
}

func TestE2E_EmitsNoLineageWhenTurnedOff(t *testing.T) {
	result := e2eDiscover(t, pluginsdk.RawConfig{"discover_lineage": false})

	assert.Empty(t, result.Lineage)
}

func TestE2E_PagesThroughEveryDashboard(t *testing.T) {
	small := e2eDiscover(t, pluginsdk.RawConfig{"page_size": 1})
	large := e2eDiscover(t, pluginsdk.RawConfig{"page_size": 250})

	assert.Equal(t, len(large.Assets), len(small.Assets))
	assert.True(t, findsAsset(small, "Dashboard", "Revenue"))
}

func TestE2E_FailsOnABadAPIKey(t *testing.T) {
	config := e2eConfig(t)
	config["api_key"] = "0000000000000000000000000000000000000000"

	_, err := buildBinary(t).Discover(t.Context(), config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check the api_key")
}

func TestE2E_EveryAssetMRNEqualsTheServersDerivation(t *testing.T) {
	result := e2eDiscover(t, nil)

	require.NotEmpty(t, result.Assets)
	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)
		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}
