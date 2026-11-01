package mariadb

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from the bare object name, not database.table,
// which is the shape the OpenMetadata projection and the Trino connector
// map already use for MariaDB tables.

func TestTableMRN_IsTheBareObjectName(t *testing.T) {
	assert.Equal(t, "mrn://table/mariadb/orders", assetMRN("Table", "orders"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/mariadb/customer_totals", assetMRN("View", "customer_totals"))
}

func TestSequenceMRN_UsesTheSequenceType(t *testing.T) {
	assert.Equal(t, "mrn://sequence/mariadb/order_seq", assetMRN("Sequence", "order_seq"))
}

func TestTableMRN_AgreesWithTheTrinoConnectorMap(t *testing.T) {
	// The Trino plugin names a table reached through its mariadb connector
	// mrn.New("Table", "MariaDB", table). The two have to agree or the same
	// table shows up twice, once per plugin.
	assert.Equal(t, mrn.New("Table", "MariaDB", "orders"), assetMRN("Table", "orders"))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN set on the asset must be exactly mrn.New over those three.
	s := &Source{config: &Config{Database: "shop", Host: "localhost", Port: 3306}}

	a := s.databaseAsset()

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Table", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseAsset_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin already creates mrn://database/mariadb/<db>
	// for a MariaDB service. This plugin has to produce the same MRN or the
	// day it takes over, that asset is stranded and a second one appears.
	s := &Source{config: &Config{Database: "shop", Host: "localhost", Port: 3306}}

	a := s.databaseAsset()

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://database/mariadb/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"MariaDB"}, a.Providers)
	require.NotNil(t, a.Name)
	assert.Equal(t, "shop", *a.Name, "the name people read is the database's own name")
}

func TestDatabaseAsset_IsNotAPrefixOfTheTablesItHolds(t *testing.T) {
	// Table MRNs are bare, so the container's MRN is not a prefix of its
	// contents. The Contents tree is built from the CONTAINS edges Discover
	// emits, not by matching MRN prefixes.
	s := &Source{config: &Config{Database: "shop"}}

	db := s.databaseAsset()
	table := assetMRN("Table", "orders")

	assert.Equal(t, "mrn://database/mariadb/shop", *db.MRN)
	assert.Equal(t, "mrn://table/mariadb/orders", table)
	assert.NotContains(t, table, "shop")
}

func TestAssetMRN_LowercasesAndSanitisesTheName(t *testing.T) {
	assert.Equal(t, "mrn://table/mariadb/order-lines", assetMRN("Table", "Order Lines"))
}
