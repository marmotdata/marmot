package redash

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Marmot server rebuilds an asset's identity from its Type, its first
// provider and its Name. These tests pin the exact strings that produces for
// each Redash asset kind, so a change to naming shows up here rather than as
// duplicated assets in a catalog.

func assetMRN(assetType, name string) string {
	return mrn.Asset(assetType, provider, name).String()
}

func TestMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod"
	c := &collector{config: cfg}

	assert.Equal(t, "mrn://asset/dashboard/redash@prod/revenue", c.assetMRN("Dashboard", "Revenue"))
}

func TestDashboardMRN_IsTheDashboardName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/redash/revenue", assetMRN("Dashboard", "Revenue"))
}

func TestChartMRN_JoinsTheDashboardAndVisualizationNames(t *testing.T) {
	// mrn.New turns the separating slash into a hyphen, so the chart MRN
	// reads as one name rather than a path.
	assert.Equal(t, "mrn://asset/chart/redash/revenue-orders-by-day", assetMRN("Chart", "Revenue/Orders by day"))
}

func TestQueryMRN_DashesTheSpacesInTheDataModelObjectType(t *testing.T) {
	// mrn.New lowercases the type and replaces its spaces the same way it
	// does in the name, so the type reads "data-model-object".
	assert.Equal(t, "mrn://asset/data-model-object/redash/orders-by-day", assetMRN("Data Model Object", "Orders by day"))
}

func TestDataSourceMRN_IsTheDataSourceName(t *testing.T) {
	assert.Equal(t, "mrn://asset/datasource/redash/shop", assetMRN("DataSource", "shop"))
}

func TestDiscover_EveryAssetMRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	require.NotEmpty(t, result.Assets)
	for _, asset := range result.Assets {
		parsed, err := mrn.ParseMRN(*asset.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *asset.MRN)
		assert.Equal(t, strings.ReplaceAll(strings.ToLower(asset.Type), " ", "-"), parsed.Type)
		assert.Equal(t, "redash", parsed.Provider)
	}
}

func TestDiscover_ProducesTheExpectedMRNSetForTheFixture(t *testing.T) {
	result := discoverAgainst(t, fullFake(), nil)

	var mrns []string
	for _, asset := range result.Assets {
		mrns = append(mrns, *asset.MRN)
	}

	assert.ElementsMatch(t, []string{
		"mrn://asset/datasource/redash/shop",
		"mrn://asset/data-model-object/redash/orders-by-day",
		"mrn://asset/dashboard/redash/revenue",
		"mrn://asset/chart/redash/revenue-orders-by-day",
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
		{"mrn://asset/datasource/redash/shop", "mrn://asset/data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://asset/table/postgresql@host=db.internal:5432/shop.public.orders", "mrn://asset/data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://asset/table/postgresql@host=db.internal:5432/shop.public.customers", "mrn://asset/data-model-object/redash/orders-by-day", "FEEDS"},
		{"mrn://asset/dashboard/redash/revenue", "mrn://asset/chart/redash/revenue-orders-by-day", "CONTAINS"},
		{"mrn://asset/data-model-object/redash/orders-by-day", "mrn://asset/chart/redash/revenue-orders-by-day", "FEEDS"},
	}, edges)
}
