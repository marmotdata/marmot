package timescale

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TimescaleDB is a PostgreSQL extension, so its objects are filed under the
// PostgreSQL provider by the bare object name, exactly as the PostgreSQL
// plugin files them.

func TestHypertableMRN_IsQualifiedByDatabaseAndSchema(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.conditions", assetMRN(cfg, "Table", "shop", "public", "conditions"))
	assert.Equal(t, "mrn://asset/view/postgresql/shop.public.conditions_daily", assetMRN(cfg, "View", "shop", "public", "conditions_daily"))
	assert.NotEqual(t, assetMRN(cfg, "Table", "shop", "public", "conditions"), assetMRN(cfg, "Table", "shop", "metrics", "conditions"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://asset/database/postgresql/shop", assetMRN(&Config{}, "Database", "shop"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "shop", "public", "conditions")
	assert.Equal(t, "mrn://asset/table/postgresql@prod-eu/shop.public.conditions", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

// A TimescaleDB server is a PostgreSQL server. Running this plugin and the PostgreSQL plugin against the same instance has to update one set of assets rather than create two, which only holds while both derive identity the same way.
func TestIdentity_MatchesThePostgreSQLPlugin(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, mrn.Asset("Table", "PostgreSQL", "shop", "public", "conditions").String(), assetMRN(cfg, "Table", "shop", "public", "conditions"))
	assert.Equal(t, mrn.Asset("Database", "PostgreSQL", "shop").String(), assetMRN(cfg, "Database", "shop"))
}

func TestProvider_IsPostgreSQL(t *testing.T) {
	// A distinct provider string would split one database across two
	// catalogs, one per plugin.
	assert.Equal(t, "PostgreSQL", provider)
}

// A foreign key edge has to point at the MRN the table pass created for the same table, or the server drops it.
func TestForeignKeyEdges_UseTheSameIdentityAsTables(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "mrn://asset/table/postgresql/shop.public.customers", *assets[0].MRN)
	assert.Equal(t, *assets[0].MRN, assetMRN(s.config, "Table", "shop", "public", "customers"))
	assert.Equal(t, "customers", *assets[0].Name)
}
