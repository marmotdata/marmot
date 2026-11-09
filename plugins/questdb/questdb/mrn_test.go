package questdb

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QuestDB has one database and no schemas, so an object is addressed by its bare name.

func TestTableMRN_IsTheBareTableName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/questdb/trades", assetMRN(&Config{}, "Table", "trades"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://asset/view/questdb/trades_1h", assetMRN(&Config{}, "View", "trades_1h"))
}

func TestMRN_KeepsADottedSystemTableName(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/questdb/sys.text_import_log", assetMRN(&Config{}, "Table", "sys.text_import_log"))
}

func TestAssetMRN_CarriesTheZone(t *testing.T) {
	cfg := &Config{BaseConfig: pluginsdk.BaseConfig{Zone: "prod-eu"}}

	got := assetMRN(cfg, "Table", "trades")
	assert.Equal(t, "mrn://asset/table/questdb@prod-eu/trades", got)

	parsed, err := mrn.ParseMRN(got)
	require.NoError(t, err)
	assert.Equal(t, "prod-eu", parsed.Zone)
}

func TestTableAsset_NameAndMRN(t *testing.T) {
	a := testSource().buildAsset(discoveredObject{table: tradesTable(), partitionCount: -1, sizeBytes: -1})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	assert.Equal(t, "trades", *a.Name)
	assert.Equal(t, "mrn://asset/table/questdb/trades", *a.MRN)
}

func TestViewAsset_NameAndMRN(t *testing.T) {
	a := testSource().buildAsset(discoveredObject{
		table:          tableInfo{Name: "trades_1h", Kind: kindMaterializedView},
		view:           &viewInfo{Name: "trades_1h", Materialized: true, BaseTable: "trades"},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	require.NotNil(t, a.MRN)
	assert.Equal(t, "mrn://asset/view/questdb/trades_1h", *a.MRN)
}

func TestViewLineage_AddressesTheBaseTableTheWayTheTablePassDoes(t *testing.T) {
	// Both passes go through assetMRN, so the edge's Source is byte for byte
	// the MRN the table asset was given. Otherwise the server drops the edge.
	s := testSource()
	table := s.buildAsset(discoveredObject{table: tradesTable(), partitionCount: -1, sizeBytes: -1})

	edges := viewLineage([]discoveredObject{
		{table: tradesTable(), mrn: *table.MRN},
		{
			table: tableInfo{Name: "trades_1h", Kind: kindMaterializedView},
			view:  &viewInfo{Name: "trades_1h", Materialized: true, BaseTable: "trades"},
			mrn:   assetMRN(s.config, "View", "trades_1h"),
		},
	})

	require.Len(t, edges, 1)
	assert.Equal(t, *table.MRN, edges[0].Source)
	assert.Equal(t, "VIEW_OF", edges[0].Type)
}
