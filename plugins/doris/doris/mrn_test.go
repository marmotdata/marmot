package doris

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Objects are named database.table: the Doris database is what keeps two
// same named tables apart, and it is the schema level of an OpenMetadata
// import, so both routes land on one asset.

func TestTableMRN_IsDatabaseQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/doris/shop.orders", assetMRN("Table", "shop.orders"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/doris/shop.daily_sales", assetMRN("View", "shop.daily_sales"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/doris/shop", assetMRN("Database", "shop"))
}

func TestTableMRN_KeepsSameNamedTablesInTwoDatabasesApart(t *testing.T) {
	assert.NotEqual(t, assetMRN("Table", "shop.orders"), assetMRN("Table", "archive.orders"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so the dot in the name has to survive
	// that unchanged or the asset becomes unreachable from the UI.
	original := assetMRN("Table", "shop.orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a Doris table as provider Doris
	// with the name <schema>.<table>, where the OpenMetadata schema is the
	// Doris database. This plugin has to produce the same MRN or the day
	// it takes over, that asset is stranded and a second one appears.
	assert.Equal(t, "mrn://table/doris/shop.orders", mrn.New("Table", "Doris", "shop.orders"))
	assert.Equal(t, "mrn://database/doris/shop", mrn.New("Database", "Doris", "shop"))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The one asset this plugin builds without a live connection, so the
	// one place the agreement can be checked against real output: the
	// server rebuilds the MRN from Type, Providers[0] and Name.
	s := &Source{config: &Config{Host: "fe", Port: 9030, Catalog: "internal"}}

	a := s.databaseAsset("shop", "doris-2.1.0", 4, 2)

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://database/doris/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
	assert.Equal(t, []string{"Doris"}, a.Providers)
	assert.Equal(t, "shop", *a.Name)
}

func TestDatabaseAsset_CarriesTheConnectionAndCounts(t *testing.T) {
	s := &Source{config: &Config{Host: "fe", Port: 9030, Catalog: "internal"}}

	a := s.databaseAsset("shop", "doris-2.1.0-rc11", 4, 2)

	assert.Equal(t, "fe", a.Metadata["host"])
	assert.Equal(t, 9030, a.Metadata["port"])
	assert.Equal(t, "internal", a.Metadata["catalog"])
	assert.Equal(t, "shop", a.Metadata["database"])
	assert.Equal(t, "doris-2.1.0-rc11", a.Metadata["doris_version"])
	assert.Equal(t, 4, a.Metadata["table_count"])
	assert.Equal(t, 2, a.Metadata["view_count"])
}
