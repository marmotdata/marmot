package sqlite

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SQLite has no schema layer, so a table's MRN is the database file name followed by the table.

func TestDatabaseName_IsTheFileNameWithoutItsExtension(t *testing.T) {
	for path, want := range map[string]string{
		"/data/app.db":                           "app",
		"app.sqlite3":                            "app",
		"s3://bucket/backups/users.db":           "users",
		"git::https://x.io/r.git//a.db?ref=main": "a",
		`C:\data\app.db`:                         "app",
	} {
		assert.Equal(t, want, databaseName(path), path)
	}
}

func TestTableMRN_IsQualifiedByTheDatabaseFile(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/sqlite/app.orders", assetMRN(&Config{Path: "/data/app.db"}, "Table", "orders"))
	assert.Equal(t, "mrn://asset/view/sqlite/app.active_users", assetMRN(&Config{Path: "/data/app.db"}, "View", "active_users"))
}

func TestTableMRN_SameTableInTwoFilesGetsDifferentMRNs(t *testing.T) {
	assert.NotEqual(t,
		assetMRN(&Config{Path: "/data/a.db"}, "Table", "orders"),
		assetMRN(&Config{Path: "/data/b.db"}, "Table", "orders"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{Path: "/data/app.db", BaseConfig: pluginsdk.BaseConfig{Zone: "laptop"}}

	got := assetMRN(cfg, "Table", "orders")
	assert.Equal(t, "mrn://asset/table/sqlite@laptop/app.orders", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "laptop", parsed.Zone)
}

func TestMeta_IsAFileIdentityScope(t *testing.T) {
	assert.Equal(t, pluginsdk.IdentityScopeFile, Meta().IdentityScope)
}
