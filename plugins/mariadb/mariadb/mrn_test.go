package mariadb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableMRN_IsQualifiedByDatabase(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/mariadb/shop.orders", assetMRN(cfg, "Table", "shop", "orders"))
	assert.NotEqual(t,
		assetMRN(cfg, "Table", "shop", "orders"),
		assetMRN(cfg, "Table", "crm", "orders"))
}

func TestViewAndSequenceMRN_UseTheirOwnType(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/view/mariadb/shop.customer_totals", assetMRN(cfg, "View", "shop", "customer_totals"))
	assert.Equal(t, "mrn://asset/sequence/mariadb/shop.order_seq", assetMRN(cfg, "Sequence", "shop", "order_seq"))
}

func TestDatabaseAsset_MRNIsTheDatabaseName(t *testing.T) {
	s := &Source{config: &Config{Database: "shop", Host: "localhost", Port: 3306}}

	a := s.databaseAsset()

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://asset/database/mariadb/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"MariaDB"}, a.Providers)
	require.NotNil(t, a.Name)
	assert.Equal(t, "shop", *a.Name)
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/mariadb@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}
