package starrocks

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StarRocks table and view names are qualified with their database in the MRN, because a StarRocks cluster holds many databases and the same table name appears in several of them. The database itself is the bare name.

func TestTableMRN_IsTheDatabaseQualifiedName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/starrocks/shop.orders", assetMRN(&Config{}, "Table", "shop", "orders"))
}

func TestDatabaseMRN_IsTheBareDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/starrocks/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestViewMRN_UsesTheViewTypeForBothKindsOfView(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/view/starrocks/shop.daily_sales", assetMRN(cfg, "View", "shop", "daily_sales"))
	assert.Equal(t, "mrn://asset/view/starrocks/shop.mv_sales", assetMRN(cfg, "View", "shop", "mv_sales"))
}

func TestTableMRN_KeepsSameNamedTablesInTwoDatabasesApart(t *testing.T) {
	cfg := &Config{}

	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "orders"), assetMRN(cfg, "Table", "archive", "orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/starrocks@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestDatabaseAsset_MRNIsTheDatabaseName(t *testing.T) {
	s := &Source{config: &Config{Host: "fe.internal", Port: 9030, Catalog: defaultCatalog}}

	a := s.databaseAsset("shop", "Internal", "4.1.4-4a9848e")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "mrn://asset/database/starrocks/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"StarRocks"}, a.Providers)
	assert.Equal(t, "shop", *a.Name)
}
