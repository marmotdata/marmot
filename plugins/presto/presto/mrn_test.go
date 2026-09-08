package presto

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A catalog is Presto's own object, so it lives under the Presto
// provider by its bare name. A table is named by its full Presto path
// unless its connector maps to a technology another Marmot plugin
// catalogues, in which case it takes that plugin's provider and name
// shape and the two runs land on one asset.

func TestCatalogMRN_IsTheBareCatalogName(t *testing.T) {
	assert.Equal(t, "mrn://catalog/presto/tpch", assetMRN("Catalog", "Presto", "tpch"))
}

func TestTableMRN_PrestoNativeCatalogUsesTheFullPath(t *testing.T) {
	assert.Equal(t, "mrn://table/presto/memory.default.orders", assetMRN("Table", "Presto", "memory.default.orders"))
}

func TestTableMRN_MappedConnectorUsesTheNativePluginsIdentity(t *testing.T) {
	info := connectorInfoForName("postgresql")
	assert.Equal(t, "mrn://table/postgresql/orders", assetMRN("Table", info.Provider, info.MRNName("pg", "public", "orders")))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/presto/memory.shop.big_orders", assetMRN("View", "Presto", "memory.shop.big_orders"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical,
	// dots included.
	original := assetMRN("Table", "Presto", "memory.shop.orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestCatalogAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	a := s.createCatalogAsset("tpch", "tpch", "0.299", 1, nil)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableAsset_MRNAgreesWithItsOwnFieldsForEveryConnector(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}

	connectors := []string{"memory", "tpch", "unknown_connector"}
	for connector := range connectorMap {
		connectors = append(connectors, connector)
	}

	for _, connector := range connectors {
		a := s.createTableAsset("cat", "sch", "orders", "BASE TABLE", connector, connectorInfoForName(connector))

		require.NotNil(t, a.MRN, connector)
		require.NotNil(t, a.Name, connector)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "%s: MRN and Name must agree", connector)
	}
}

func TestCatalogAsset_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// The Contents tree is built from the CONTAINS edges Discover emits,
	// not by matching MRN prefixes, so a PostgreSQL table reached through
	// a Presto catalog keeps its bare PostgreSQL name.
	catalog := assetMRN("Catalog", "Presto", "pg")
	table := assetMRN("Table", "PostgreSQL", "orders")

	assert.Equal(t, "mrn://catalog/presto/pg", catalog)
	assert.Equal(t, "mrn://table/postgresql/orders", table)
	assert.NotContains(t, table, "pg")
}

func TestTableMRN_ProviderWithASpaceLandsInTheMRN(t *testing.T) {
	// mrn.New sanitises the name but not the service, so "Delta Lake"
	// puts a literal space in the MRN. That is what the Trino plugin
	// already emits for the same table, and matching it is the point.
	s := &Source{config: &Config{}}

	a := s.createTableAsset("dl", "default", "events", "BASE TABLE", "delta", connectorInfoForName("delta"))

	assert.Equal(t, "mrn://table/delta lake/dl.default.events", *a.MRN)
	assert.Equal(t, []string{"Delta Lake"}, a.Providers)
}

func TestLineage_ContainsEdgeUsesTheSameMRNsAsTheAssets(t *testing.T) {
	s := &Source{config: &Config{Host: "localhost", Port: 8080}}
	table := s.createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory", connectorInfoForName("memory"))
	catalog := s.createCatalogAsset("memory", "memory", "", 1, []pluginsdk.Asset{table})

	assert.Equal(t, "mrn://catalog/presto/memory", *catalog.MRN)
	assert.Equal(t, "mrn://table/presto/memory.shop.orders", *table.MRN)
}
