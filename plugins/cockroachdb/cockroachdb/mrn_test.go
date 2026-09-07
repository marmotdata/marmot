package cockroachdb

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from database.schema.table. One cluster holds many
// databases, so a bare name would fold shop.public.orders and
// archive.public.orders into one asset.

func TestTableMRN_IsFullyQualified(t *testing.T) {
	assert.Equal(t, "mrn://table/cockroachdb/shop.public.orders",
		assetMRN("Table", qualifiedName("shop", "public", "orders")))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/cockroachdb/shop.public.customer_totals",
		assetMRN("View", qualifiedName("shop", "public", "customer_totals")))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/cockroachdb/shop", assetMRN("Database", "shop"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical, dots
	// included.
	original := assetMRN("Table", qualifiedName("shop", "public", "orders"))

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDatabaseAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from Type, Providers[0] and Name, so the
	// MRN the plugin sets has to be exactly mrn.New over those.
	a := newSource().databaseAsset(database{Name: "shop", Owner: "root"}, "")

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestViewAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := newSource().relationAsset("shop", relation{Schema: "public", Name: "order_summary", Kind: relkindMaterializedView}, relationDetails{})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestTableAsset_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// The OpenMetadata plugin projects a Cockroach service's tables onto
	// provider CockroachDB with database.schema.table names. This plugin has
	// to land on the same MRN or the two runs file one table twice.
	a := newSource().relationAsset("shop", relation{Schema: "public", Name: "orders", Kind: relkindTable}, relationDetails{})

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://table/cockroachdb/shop.public.orders", *a.MRN)
	assert.Equal(t, []string{"CockroachDB"}, a.Providers)
}

func TestDatabaseAsset_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	a := newSource().databaseAsset(database{Name: "shop"}, "")

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://database/cockroachdb/shop", *a.MRN)
	assert.Equal(t, "Database", a.Type)
}

func TestTableMRN_UppercaseNamesAreLowercased(t *testing.T) {
	// mrn.New lowercases the name, so a quoted mixed-case table still gets
	// one stable MRN; the readable Name keeps its case.
	a := newSource().relationAsset("shop", relation{Schema: "Sales", Name: "Regions", Kind: relkindTable}, relationDetails{})

	assert.Equal(t, "shop.Sales.Regions", *a.Name)
	assert.Equal(t, "mrn://table/cockroachdb/shop.sales.regions", *a.MRN)
}
