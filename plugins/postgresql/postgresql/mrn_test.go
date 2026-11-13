package postgresql

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableMRN_IsQualifiedByDatabaseAndSchema(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.orders", assetMRN(cfg, "Table", "shop", "public", "orders"))
	assert.NotEqual(t,
		assetMRN(cfg, "Table", "shop", "public", "users"),
		assetMRN(cfg, "Table", "shop", "staging", "users"))
	assert.NotEqual(t,
		assetMRN(cfg, "Table", "shop", "public", "users"),
		assetMRN(cfg, "Table", "crm", "public", "users"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/postgresql/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "public", "orders")
	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/shop.public.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}
