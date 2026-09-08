package metabase

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Metabase asset is named by the path of the collection holding it
// and its own name, joined with "/". mrn.New folds the slashes and
// spaces to hyphens and lowercases, so the MRN is a flat slug while
// Name keeps the readable path.

func TestDashboardMRN_IsTheCollectionPathAndName(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/metabase/marmot-finance-finance-overview",
		assetMRN("Dashboard", "Marmot/Finance/Finance overview"))
}

func TestChartMRN_IsTheCollectionPathAndName(t *testing.T) {
	assert.Equal(t, "mrn://chart/metabase/marmot-finance-revenue-by-customer",
		assetMRN("Chart", "Marmot/Finance/Revenue by customer"))
}

func TestDataModelObjectMRN_KeepsTheSpacedType(t *testing.T) {
	// The type is used as-is by mrn.New, which is also how the
	// OpenMetadata plugin addresses a Metabase model, so the two merge.
	assert.Equal(t, "mrn://data model object/metabase/marmot-finance-customer-orders",
		assetMRN("Data Model Object", "Marmot/Finance/Customer orders"))
}

func TestRootItemMRN_IsTheBareName(t *testing.T) {
	assert.Equal(t, "mrn://dashboard/metabase/root-board", assetMRN("Dashboard", "Root board"))
}

func TestMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that
	// unchanged or the asset becomes unreachable from the UI.
	for _, original := range []string{
		assetMRN("Dashboard", "Marmot/Finance/Finance overview"),
		assetMRN("Chart", "Examples/Orders + People"),
		assetMRN("Data Model Object", "Marmot/Finance/Customer orders"),
	} {
		parsed, err := mrn.Parse(original)
		require.NoError(t, err)
		assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
	}
}

func TestEveryAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so
	// the MRN each asset carries must be exactly that.
	result := discover(t, shopFixture(), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		require.NotNil(t, a.Name)
		require.NotEmpty(t, a.Providers)
		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	}
}

func TestEveryEdge_PointsAtAnEmittedAssetOrANativeTable(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	emitted := make(map[string]struct{})
	for _, a := range result.Assets {
		emitted[*a.MRN] = struct{}{}
	}
	for _, edge := range result.Lineage {
		_, sourceEmitted := emitted[edge.Source]
		_, targetEmitted := emitted[edge.Target]
		assert.Truef(t, targetEmitted, "edge target %s is not an emitted asset", edge.Target)
		if !sourceEmitted {
			assert.Containsf(t, edge.Source, "mrn://table/postgresql/", "edge source %s is neither emitted nor a native table", edge.Source)
		}
	}
}

func TestTableMRN_MatchesThePostgreSQLPlugin(t *testing.T) {
	// The PostgreSQL plugin names a table by its bare name, so the
	// lineage edge must land on mrn://table/postgresql/<table>.
	db := database{ID: 2, Engine: "postgres", Details: map[string]any{"dbname": "shop"}}

	assert.Equal(t, "mrn://table/postgresql/orders", tableMRN(db, table{Schema: "public", Name: "orders"}))
	assert.Equal(t, mrn.New("Table", "PostgreSQL", "orders"), tableMRN(db, table{Schema: "public", Name: "orders"}))
}
