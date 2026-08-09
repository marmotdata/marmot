package mysql

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An asset's MRN is its identity: two objects sharing one MRN become one
// asset, and the second silently overwrites the first.

func TestAssetMRN_KeepsTheParentInTheIdentity(t *testing.T) {
	// One server holds many databases, each able to hold the same table name.
	assert.NotEqual(t,
		assetMRN("Table", "a", "events"),
		assetMRN("Table", "b", "events"),
		"objects of the same name under different parents must stay apart")
}

func TestAssetMRN_HasTheShapeThePluginDeclares(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/shop.orders", assetMRN("Table", "shop", "orders"))
}

// The UI splits an MRN to build a link and /assets/lookup feeds the parts
// back through mrn.New, so an MRN has to come out of that round trip
// byte-identical or the asset becomes unreachable from the UI.
func TestAssetMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Table", "shop", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseAsset_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin already creates mrn://database/mysql/<db> for
	// a MySQL service. This plugin has to produce the same MRN or the day
	// it takes over, that asset is stranded and a second one appears.
	s := &Source{config: &Config{Database: "posts_db", Host: "localhost", Port: 3306}}

	asset := s.databaseAsset()

	require.NotNil(t, asset.MRN)
	assert.Equal(t, "mrn://database/mysql/posts_db", *asset.MRN)
	assert.Equal(t, "Database", asset.Type)
	assert.Equal(t, []string{"MySQL"}, asset.Providers)
	require.NotNil(t, asset.Name)
	assert.Equal(t, "posts_db", *asset.Name, "the name people read is the database's own name")
}

func TestDatabaseAsset_IsTheParentOfEveryTableItHolds(t *testing.T) {
	// A table's identity is database.table, so the container's MRN is the
	// prefix of everything it contains. That is what makes the Contents
	// tree line up with the identities.
	s := &Source{config: &Config{Database: "posts_db"}}

	db := s.databaseAsset()
	table := assetMRN("Table", "posts_db", "comments")

	assert.Equal(t, "mrn://database/mysql/posts_db", *db.MRN)
	assert.Equal(t, "mrn://table/mysql/posts_db.comments", table)
}
