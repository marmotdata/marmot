package presto

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A catalog is Presto's own object, so it lives under the Presto provider by its name. A table is Presto's own too, named by its full path, unless its catalog is listed in catalog_map, in which case it takes the native plugin's provider, zone and path and the two runs land on one asset.

func newMRNSource(zone string) *Source {
	cfg := &Config{Host: "localhost", Port: 8080}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestCatalogMRN_IsTheBareCatalogName(t *testing.T) {
	assert.Equal(t, "mrn://asset/catalog/presto/tpch", newMRNSource("").config.catalogMRN("tpch"))
}

func TestTableMRN_PrestoCatalogUsesTheFullPath(t *testing.T) {
	a := newMRNSource("").createTableAsset("memory", "default", "orders", "BASE TABLE", "memory")
	assert.Equal(t, "mrn://asset/table/presto/memory.default.orders", *a.MRN)
}

func TestTableMRN_ViewUsesTheViewType(t *testing.T) {
	a := newMRNSource("").createTableAsset("memory", "shop", "big_orders", "VIEW", "memory")
	assert.Equal(t, "mrn://asset/view/presto/memory.shop.big_orders", *a.MRN)
}

func TestTableMRN_CarriesTheZone(t *testing.T) {
	a := newMRNSource("prod-eu").createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory")
	assert.Equal(t, "mrn://asset/table/presto@prod-eu/memory.shop.orders", *a.MRN)
}

func TestTableMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := *newMRNSource("").createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory").MRN

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).String())
}

func TestTableAsset_NameIsTheTableAndTheMRNEndsWithIt(t *testing.T) {
	s := newMRNSource("")
	s.config.CatalogMap = map[string]CatalogMapping{"cat": {Database: "db"}}

	for connector := range connectorMap {
		a := s.createTableAsset("cat", "sch", "orders", "BASE TABLE", connector)

		require.NotNil(t, a.Name, connector)
		assert.Equal(t, "orders", *a.Name, connector)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err, connector)
		assert.Equal(t, "orders", parsed.Path()[len(parsed.Path())-1], connector)
	}
}

func TestLineage_ContainsEdgeUsesTheSameMRNsAsTheAssets(t *testing.T) {
	s := newMRNSource("")
	table := s.createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory")
	catalog := s.createCatalogAsset("memory", "memory", "", 1, []pluginsdk.Asset{table})

	assert.Equal(t, "mrn://asset/catalog/presto/memory", *catalog.MRN)
	assert.Equal(t, "mrn://asset/table/presto/memory.shop.orders", *table.MRN)
}

func TestMeta_Federates(t *testing.T) {
	assert.True(t, Meta().Federates)
}
