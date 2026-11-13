package doris

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Objects are addressed database.table: the Doris database is what keeps two same named tables apart, and it is the schema level of an OpenMetadata import, so both routes land on one asset.

func TestTableMRN_IsDatabaseQualified(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/doris/shop.orders", assetMRN(&Config{}, "Table", "shop", "orders"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://asset/view/doris/shop.daily_sales", assetMRN(&Config{}, "View", "shop", "daily_sales"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/doris/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestTableMRN_KeepsSameNamedTablesInTwoDatabasesApart(t *testing.T) {
	cfg := &Config{}

	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "orders"), assetMRN(cfg, "Table", "archive", "orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/doris@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestDatabaseAsset_MRNIsTheDatabaseName(t *testing.T) {
	s := &Source{config: &Config{Host: "fe", Port: 9030, Catalog: "internal"}}

	a := s.databaseAsset("shop", "doris-2.1.0", 4, 2)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "mrn://asset/database/doris/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"Doris"}, a.Providers)
	assert.Equal(t, "shop", *a.Name)
}

func TestDatabaseAsset_CarriesTheConnectionAndCounts(t *testing.T) {
	s := &Source{config: &Config{Host: "fe", Port: 9030, Catalog: "internal"}}

	a := s.databaseAsset("shop", "doris-2.1.0-rc11", 4, 2)

	assert.Equal(t, "fe", a.Metadata["host"])
	assert.Equal(t, 9030, a.Metadata["port"])
	assert.Equal(t, "internal", a.Metadata["catalog"])
	assert.Equal(t, "shop", a.Metadata["database"])
	assert.Equal(t, "doris-2.1.0-rc11", a.Metadata["doris_version"])
	assert.Equal(t, 4, a.Metadata["table_count"])
	assert.Equal(t, 2, a.Metadata["view_count"])
}
