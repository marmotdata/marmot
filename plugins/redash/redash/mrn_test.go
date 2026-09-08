package redash

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Marmot server rebuilds an asset's identity from its Type, its first
// provider and its Name. These tests pin the exact strings that produces for
// each Redash asset kind, so a change to naming shows up here rather than as
// duplicated assets in a catalog.

func TestDashboardMRN_IsTheDashboardName(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/redash/revenue", assetMRN("Dashboard", "Revenue"))
}

func TestChartMRN_JoinsTheDashboardAndVisualizationNames(t *testing.T) {
	// mrn.New turns the separating slash into a hyphen, so the chart MRN
	// reads as one name rather than a path.
	assert.Equal(t, "mrn://chart/redash/revenue-orders-by-day", assetMRN("Chart", "Revenue/Orders by day"))
}

func TestQueryMRN_DashesTheSpacesInTheDataModelObjectType(t *testing.T) {
	// mrn.New lowercases the type and replaces its spaces the same way it
	// does in the name, so the type reads "data-model-object".
	assert.Equal(t, "mrn://data-model-object/redash/orders-by-day", assetMRN("Data Model Object", "Orders by day"))
}

func TestDataSourceMRN_IsTheDataSourceName(t *testing.T) {
	assert.Equal(t, "mrn://datasource/redash/shop", assetMRN("DataSource", "shop"))
}

func TestTableMRN_MatchesWhatThePostgreSQLPluginProduces(t *testing.T) {
	// A lineage edge points at the asset plugins/postgresql creates, so the
	// provider and the bare name have to match it exactly.
	assert.Equal(t, "mrn://table/postgresql/orders", mrn.New("Table", "PostgreSQL", "orders"))
}

func TestDashboardMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged.
	original := assetMRN("Dashboard", "Revenue")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestQueryMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Data Model Object", "Orders by day")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestChartMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Chart", "Revenue/Orders by day")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDiscover_EveryAssetMRNEqualsMrnNewOverItsOwnFields(t *testing.T) {
	// This is the identity the Marmot server derives. An asset whose MRN
	// disagrees with it is filed twice, once per route.
	result := discoverAgainst(t, fullFake(), nil)

	require.NotEmpty(t, result.Assets)
	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)

		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}

func TestDiscover_ProducesTheExpectedMRNSetForTheFixture(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	var mrns []string
	for _, asset := range result.Assets {
		mrns = append(mrns, *asset.MRN)
	}

	assert.ElementsMatch(t, []string{
		"mrn://datasource/redash/shop",
		"mrn://data-model-object/redash/orders-by-day",
		"mrn://dashboard/redash/revenue",
		"mrn://chart/redash/revenue-orders-by-day",
	}, mrns)
}

func TestDiscover_ProducesTheExpectedLineageForTheFixture(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	type edge struct{ source, target, kind string }
	var edges []edge
	for _, e := range result.Lineage {
		edges = append(edges, edge{e.Source, e.Target, e.Type})
	}

	assert.ElementsMatch(t, []edge{
		{"mrn://datasource/redash/shop", "mrn://data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://table/postgresql/orders", "mrn://data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://table/postgresql/customers", "mrn://data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://dashboard/redash/revenue", "mrn://chart/redash/revenue-orders-by-day", "CONTAINS"},
		{"mrn://data-model-object/redash/orders-by-day", "mrn://chart/redash/revenue-orders-by-day", "FEEDS"},
	}, edges)
}
