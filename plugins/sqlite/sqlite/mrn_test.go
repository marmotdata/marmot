package sqlite

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SQLite has no schema layer, so a table's MRN is built from its bare name,
// matching the other database plugins.

func TestTableMRN_IsTheBareTableName(t *testing.T) {
	assert.Equal(t, "mrn://table/sqlite/orders", assetMRN("Table", "orders"))
}

func TestViewMRN_UsesTheViewType(t *testing.T) {
	assert.Equal(t, "mrn://view/sqlite/active_users", assetMRN("View", "active_users"))
}

func TestTableMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Table", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}
