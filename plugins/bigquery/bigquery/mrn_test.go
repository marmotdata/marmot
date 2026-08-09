package bigquery

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An asset's MRN is its identity: two objects sharing one MRN become one
// asset, and the second silently overwrites the first.

func TestAssetMRN_KeepsTheParentInTheIdentity(t *testing.T) {
	// A table belongs to a dataset: staging.events is not prod.events.
	assert.NotEqual(t,
		assetMRN("Table", "a", "events"),
		assetMRN("Table", "b", "events"),
		"objects of the same name under different parents must stay apart")
}

func TestAssetMRN_HasTheShapeThePluginDeclares(t *testing.T) {
	assert.Equal(t, "mrn://table/bigquery/analytics.orders", assetMRN("Table", "analytics", "orders"))
}

// The UI splits an MRN to build a link and /assets/lookup feeds the parts
// back through mrn.New, so an MRN has to come out of that round trip
// byte-identical or the asset becomes unreachable from the UI.
func TestAssetMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Table", "analytics", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}
