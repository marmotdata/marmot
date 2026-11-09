package clickhouse

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableMRN_IsQualifiedByDatabase(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/clickhouse/shop.orders", assetMRN(cfg, "Table", "shop", "orders"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "orders"), assetMRN(cfg, "Table", "archive", "orders"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/clickhouse/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "orders")
	assert.Equal(t, "mrn://asset/table/clickhouse@prod-eu/shop.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}
