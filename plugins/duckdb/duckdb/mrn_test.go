package duckdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseName_IsTheFileNameWithoutItsExtension(t *testing.T) {
	for path, want := range map[string]string{
		"/data/analytics.duckdb":                            "analytics",
		"analytics.db":                                      "analytics",
		"s3://bucket/warehouse/sales.duckdb":                "sales",
		"git::https://example.com/repo.git//x.ddb?ref=main": "x",
		`C:\data\finance.duckdb`:                            "finance",
		"/data/noextension":                                 "noextension",
	} {
		assert.Equal(t, want, databaseName(path), path)
	}
}

func TestTableMRN_IsQualifiedByDatabaseAndSchema(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "mrn://asset/table/duckdb/analytics.main.orders", assetMRN(cfg, "analytics", "Table", "main", "orders"))
	assert.NotEqual(t,
		assetMRN(cfg, "analytics", "Table", "main", "orders"),
		assetMRN(cfg, "analytics", "Table", "staging", "orders"))
	assert.NotEqual(t,
		assetMRN(cfg, "analytics", "Table", "main", "orders"),
		assetMRN(cfg, "archive", "Table", "main", "orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "laptop"}}

	got := assetMRN(cfg, "analytics", "Table", "main", "orders")
	assert.Equal(t, "mrn://asset/table/duckdb@laptop/analytics.main.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "laptop", parsed.Zone)
}

func TestMeta_IsAFileIdentityScope(t *testing.T) {
	assert.Equal(t, pluginsdk.IdentityScopeFile, Meta().IdentityScope)
}

func TestDiscover_QualifiesTablesByDatabaseAndSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shop.duckdb")
	db, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	for _, stmt := range []string{
		`CREATE TABLE main.orders (id INTEGER)`,
		`CREATE SCHEMA staging`,
		`CREATE TABLE staging.orders (id INTEGER)`,
	} {
		_, err := db.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	require.NoError(t, db.Close())

	result, err := (&Source{}).Discover(t.Context(), pluginsdk.RawConfig{"path": path, "zone": "laptop"})
	require.NoError(t, err)

	byMRN := map[string]pluginsdk.Asset{}
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		byMRN[*a.MRN] = a
	}
	require.Len(t, byMRN, 2, "same-named tables in two schemas must stay two assets")

	main, ok := byMRN["mrn://asset/table/duckdb@laptop/shop.main.orders"]
	require.True(t, ok, "assets: %v", byMRN)
	assert.Equal(t, "orders", *main.Name)
	assert.Equal(t, []string{"mrn://asset/table/duckdb/main.orders"}, main.PreviousMRNs)

	_, ok = byMRN["mrn://asset/table/duckdb@laptop/shop.staging.orders"]
	assert.True(t, ok)
}
