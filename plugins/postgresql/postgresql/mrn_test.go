package postgresql

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table's MRN is built from the bare object name, not
// database.schema.table, so public.users and staging.users land on one
// asset.

func TestTableMRN_IsTheBareObjectName(t *testing.T) {
	objectName := "orders"

	assert.Equal(t, "mrn://table/postgresql/orders",
		mrn.New("Table", "PostgreSQL", objectName))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so an MRN has to survive that unchanged
	// or the asset becomes unreachable from the UI.
	original := mrn.New("Table", "PostgreSQL", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}
