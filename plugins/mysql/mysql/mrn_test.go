package mysql

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from the bare object name, not database.table,
// so the same table name in two databases lands on one asset.

func TestTableMRN_IsTheBareObjectName(t *testing.T) {
	assert.Equal(t, "mrn://table/mysql/orders",
		mrn.New("Table", "MySQL", "orders"))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The one asset this plugin builds without a live connection, so the
	// one place the agreement can be checked against real output: the
	// MRN must be exactly mrn.New over the asset's own Type, Providers[0]
	// and Name.
	s := &Source{config: &Config{Database: "posts_db", Host: "localhost", Port: 3306}}

	a := s.databaseAsset()

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := mrn.New("Table", "MySQL", "orders")

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

func TestDatabaseAsset_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// Worth stating outright, because it is the thing a reader expects and
	// does not get: table MRNs are bare, so the container's MRN is NOT a
	// prefix of its contents. The Contents tree is built from the CONTAINS
	// lineage edges Discover emits, not by matching MRN prefixes.
	s := &Source{config: &Config{Database: "posts_db"}}

	db := s.databaseAsset()
	table := mrn.New("Table", "MySQL", "comments")

	assert.Equal(t, "mrn://database/mysql/posts_db", *db.MRN)
	assert.Equal(t, "mrn://table/mysql/comments", table)
	assert.NotContains(t, table, "posts_db")
}
