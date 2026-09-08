package starrocks

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StarRocks table and view names are qualified with their database,
// because a StarRocks cluster holds many databases and the same table
// name appears in several of them. The database itself is the bare name.

func TestTableMRN_IsTheDatabaseQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://table/starrocks/shop.orders",
		assetMRN("Table", objectName("shop", "orders")))
}

func TestDatabaseMRN_IsTheBareDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/starrocks/shop", assetMRN("Database", "shop"))
}

func TestViewMRN_IsTheDatabaseQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://view/starrocks/shop.daily_sales",
		assetMRN("View", objectName("shop", "daily_sales")))
}

func TestMaterializedViewMRN_UsesTheViewTypeLikeAPlainView(t *testing.T) {
	// Marmot has no materialized view type; the distinction lives in
	// metadata, so both kinds share one MRN shape.
	assert.Equal(t, "mrn://view/starrocks/shop.mv_sales",
		assetMRN("View", objectName("shop", "mv_sales")))
}

func TestAssetMRN_LowercasesTheProvider(t *testing.T) {
	// The provider string is "StarRocks" but mrn.New lowercases the
	// service, so lookups by service are case-insensitive in practice.
	assert.Equal(t, "mrn://table/starrocks/shop.orders", assetMRN("Table", "shop.orders"))
	assert.Equal(t, "StarRocks", provider)
}

func TestAssetMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a StarRocks service onto provider
	// "StarRocks" with <database>.<table> names. This plugin has to land
	// on the same MRN or the day it takes over, a second asset appears.
	assert.Equal(t, mrn.New("Table", "StarRocks", "shop.orders"),
		assetMRN("Table", objectName("shop", "orders")))
	assert.Equal(t, mrn.New("Database", "StarRocks", "shop"),
		assetMRN("Database", "shop"))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so a
	// disagreement here strands the asset under a second MRN.
	s := &Source{config: &Config{Host: "fe.internal", Port: 9030, Catalog: defaultCatalog}}

	a := s.databaseAsset("shop", "Internal", "4.1.4-4a9848e")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://database/starrocks/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"StarRocks"}, a.Providers)
	assert.Equal(t, "shop", *a.Name)
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// /assets/lookup splits an MRN and feeds the parts back through
	// mrn.New, so it has to survive byte-identical, dot included.
	original := assetMRN("Table", objectName("shop", "orders"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
	assert.Equal(t, "shop.orders", parsed.Name)
}

func TestDatabaseMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Database", "shop")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestAssetMRN_SurvivesADatabaseNameWithAnUnderscore(t *testing.T) {
	original := assetMRN("Table", objectName("web_analytics", "page_views"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, "mrn://table/starrocks/web_analytics.page_views", original)
	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseMRN_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// The Contents tree is built from the CONTAINS edges Discover emits,
	// not by matching MRN prefixes, and the two shapes differ.
	db := assetMRN("Database", "shop")
	table := assetMRN("Table", objectName("shop", "orders"))

	assert.Equal(t, "mrn://database/starrocks/shop", db)
	assert.Equal(t, "mrn://table/starrocks/shop.orders", table)
	assert.NotContains(t, table, db)
}
