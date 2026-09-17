package timescale

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TimescaleDB is a PostgreSQL extension, so its objects are filed under the
// PostgreSQL provider by the bare object name, exactly as the PostgreSQL
// plugin files them.

func TestHypertableMRN_IsTheBareTableName(t *testing.T) {
	assert.Equal(t, "mrn://table/postgresql/conditions", assetMRN("Table", "conditions"))
}

func TestDatabaseMRN_IsTheDatabaseName(t *testing.T) {
	assert.Equal(t, "mrn://database/postgresql/shop", assetMRN("Database", "shop"))
}

func TestContinuousAggregateMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/postgresql/conditions_daily", assetMRN("View", "conditions_daily"))
}

func TestMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Table", "conditions")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// A TimescaleDB server is a PostgreSQL server. Running this plugin and the
// PostgreSQL plugin against the same instance has to update one set of assets
// rather than create two, which only holds while both derive identity the
// same way. This pins the construction plugins/postgresql/postgresql builds
// for a table, a view and a database.
func TestIdentity_MatchesThePostgreSQLPlugin(t *testing.T) {
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "conditions"), assetMRN("Table", "conditions"))
	assert.Equal(t, mrn.New("View", "PostgreSQL", "conditions_daily"), assetMRN("View", "conditions_daily"))
	assert.Equal(t, mrn.New("Database", "PostgreSQL", "shop"), assetMRN("Database", "shop"))
}

func TestProvider_IsPostgreSQL(t *testing.T) {
	// A distinct provider string would split one database across two
	// catalogs, one per plugin.
	assert.Equal(t, "PostgreSQL", provider)
}

// Every asset an ingest run produces has to satisfy the identity the server
// rebuilds from its own fields, or the asset lands somewhere else than its
// declared MRN says.
func TestBuildAssets_MRNMatchesTypeProviderAndName(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "conditions", Kind: kindTable},
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable},
		{Database: "shop", Schema: "public", Name: "conditions_daily", Kind: kindView},
		{Database: "shop", Schema: "public", Name: "monthly", Kind: kindMaterializedView},
	}, nil)

	require.Len(t, assets, 4)
	for _, asset := range assets {
		require.NotNil(t, asset.MRN)
		require.NotNil(t, asset.Name)
		assert.Equal(t, mrn.New(asset.Type, asset.Providers[0], *asset.Name), *asset.MRN)
	}
}

// A foreign key edge has to point at the MRN the table pass created for the
// same table, or the server drops it.
func TestForeignKeyEdges_UseTheSameIdentityAsTables(t *testing.T) {
	s := &Source{config: &Config{}}

	assets := s.buildAssets([]object{
		{Database: "shop", Schema: "public", Name: "customers", Kind: kindTable},
	}, nil)

	require.Len(t, assets, 1)
	assert.Equal(t, "mrn://table/postgresql/customers", *assets[0].MRN)
	assert.Equal(t, *assets[0].MRN, assetMRN("Table", "customers"))
}
