package bigquery

import (
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMRNSource(zone string) *Source {
	cfg := &Config{ProjectID: "proj"}
	cfg.Zone = zone
	return &Source{config: cfg}
}

func TestTableMRN_IncludesProjectAndDataset(t *testing.T) {
	s := newMRNSource("")
	assert.Equal(t, "mrn://asset/table/bigquery/proj.prod.orders", s.tableMRN("Table", "prod", "orders"))
	assert.Equal(t, "mrn://asset/dataset/bigquery/proj.prod", s.datasetMRN("prod"))
}

func TestTableMRN_SameTableInTwoDatasetsDiffers(t *testing.T) {
	s := newMRNSource("")
	assert.NotEqual(t, s.tableMRN("Table", "staging", "orders"), s.tableMRN("Table", "prod", "orders"))
}

func TestTableMRN_CarriesZone(t *testing.T) {
	s := newMRNSource("eu")
	assert.Equal(t, "mrn://asset/table/bigquery@eu/proj.prod.orders", s.tableMRN("Table", "prod", "orders"))
}

func TestTableMRN_IsStableUnderRoundTrip(t *testing.T) {
	original := newMRNSource("eu").tableMRN("Table", "prod", "orders")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.Asset(parsed.Type, parsed.Provider, parsed.Path()...).WithZone(parsed.Zone).String())
}

func TestMeta_IdentityScope(t *testing.T) {
	assert.Equal(t, pluginsdk.IdentityScopeAccount, Meta().IdentityScope)
}
