package metabase

import (
	"strings"
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Metabase asset's MRN has one level per collection holding it, then its own name.

// assetMRN builds the MRN of a Metabase asset from its slash-joined collection path and name, the way the plugin does from its levels.
func assetMRN(assetType, path string) string {
	return mrn.Asset(assetType, provider, strings.Split(path, "/")...).String()
}

func TestDashboardMRN_IsTheCollectionPathAndName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/metabase/marmot.finance.finance-overview",
		assetMRN("Dashboard", "Marmot/Finance/Finance overview"))
}

func TestRootItemMRN_IsTheBareName(t *testing.T) {
	assert.Equal(t, "mrn://asset/dashboard/metabase/root-board", assetMRN("Dashboard", "Root board"))
}

func TestMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "prod"
	assert.Equal(t, "mrn://asset/dashboard/metabase@prod/finance.board", cfg.AssetMRN("Dashboard", provider, "Finance", "Board"))
}

func TestEveryAsset_MRNIsKindedAndMatchesItsTypeAndProvider(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	require.NotEmpty(t, result.Assets)
	for _, a := range result.Assets {
		require.NotNil(t, a.MRN)
		parsed, err := mrn.ParseMRN(*a.MRN)
		require.NoError(t, err)
		assert.False(t, parsed.Legacy, *a.MRN)
		assert.Equal(t, strings.ReplaceAll(strings.ToLower(a.Type), " ", "-"), parsed.Type)
		assert.Equal(t, "metabase", parsed.Provider)
		assert.NotContains(t, *a.Name, "/", "the name is the item's own name, not its path")
	}
}

func TestAssetInACollection_ListsItsOldMRN(t *testing.T) {
	result := discover(t, shopFixture(), nil)

	found := false
	for _, a := range result.Assets {
		if len(a.PreviousMRNs) > 0 {
			found = true
			assert.Contains(t, a.PreviousMRNs[0], "-", "the old MRN folded the collection path into one slug")
		}
	}
	assert.True(t, found)
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
			assert.Containsf(t, edge.Source, "mrn://asset/table/postgresql", "edge source %s is neither emitted nor a native table", edge.Source)
		}
	}
}

func TestTableMRN_MatchesThePostgreSQLPlugin(t *testing.T) {
	db := database{ID: 2, Engine: "postgres", Details: map[string]any{"dbname": "shop", "host": "DB.internal", "port": float64(5433)}}

	assert.Equal(t, "mrn://asset/table/postgresql@host=db.internal:5433/shop.public.orders", tableMRN(db, table{Schema: "public", Name: "orders"}))
}

func TestTableMRN_TheHostGetsTheDefaultPort(t *testing.T) {
	db := database{Engine: "postgres", Details: map[string]any{"dbname": "shop", "host": "db"}}

	assert.Equal(t, "mrn://asset/table/postgresql@host=db:5432/shop.public.orders", tableMRN(db, table{Schema: "public", Name: "orders"}))
}

func TestTableMRN_SameTableInTwoSchemasDiffers(t *testing.T) {
	db := database{Engine: "postgres", Details: map[string]any{"dbname": "shop"}}

	assert.NotEqual(t, tableMRN(db, table{Schema: "public", Name: "orders"}), tableMRN(db, table{Schema: "archive", Name: "orders"}))
}
