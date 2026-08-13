package bigquery

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from the bare table id, not dataset.table, so
// staging.events and prod.events land on one asset.

func TestTableMRN_IsTheBareTableID(t *testing.T) {
	assert.Equal(t, "mrn://table/bigquery/orders",
		mrn.New("Table", "BigQuery", "orders"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := mrn.New("Table", "BigQuery", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}
