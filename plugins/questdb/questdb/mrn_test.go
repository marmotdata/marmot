package questdb

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QuestDB has one database and no schemas, so an object's MRN is built from
// its bare name, matching what the OpenMetadata plugin projects for a
// QuestDB service.

func TestTableMRN_IsTheBareTableName(t *testing.T) {
	assert.Equal(t, "mrn://table/questdb/trades", assetMRN("Table", "trades"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/questdb/trades_1h", assetMRN("View", "trades_1h"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Table", "trades")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestDottedTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// QuestDB allows dots in table names (its own sys.* tables have them).
	original := assetMRN("Table", "sys.text_import_log")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestTableAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN set on an asset must be exactly mrn.New over those three fields.
	a := testSource().buildAsset(discoveredObject{table: tradesTable(), partitionCount: -1, sizeBytes: -1})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://table/questdb/trades", *a.MRN)
}

func TestViewAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	a := testSource().buildAsset(discoveredObject{
		table:          tableInfo{Name: "trades_1h", Kind: kindMaterializedView},
		view:           &viewInfo{Name: "trades_1h", Materialized: true, BaseTable: "trades"},
		partitionCount: -1,
		sizeBytes:      -1,
	})

	require.NotNil(t, a.MRN)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
	assert.Equal(t, "mrn://view/questdb/trades_1h", *a.MRN)
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
			mrn:   assetMRN("View", "trades_1h"),
		},
	})

	require.Len(t, edges, 1)
	assert.Equal(t, *table.MRN, edges[0].Source)
	assert.Equal(t, "VIEW_OF", edges[0].Type)
}
