package mysql

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableMRN_IsQualifiedByDatabase(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/mysql/shop.orders", assetMRN(cfg, "Table", "shop", "orders"))
	assert.NotEqual(t,
		assetMRN(cfg, "Table", "shop", "users"),
		assetMRN(cfg, "Table", "crm", "users"))
}

func TestDatabaseAsset_MRNIsTheDatabaseName(t *testing.T) {
	s := &Source{config: &Config{Database: "posts_db", Host: "localhost", Port: 3306}}

	a := s.databaseAsset()

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://asset/database/mysql/posts_db", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"MySQL"}, a.Providers)
	require.NotNil(t, a.Name)
	assert.Equal(t, "posts_db", *a.Name)
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/mysql@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}
