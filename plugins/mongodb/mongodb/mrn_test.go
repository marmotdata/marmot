package mongodb

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A collection's MRN is built from the bare collection name, not
// database.collection, so an "events" collection in two databases lands
// on one asset.

func TestCollectionMRN_IsTheBareCollectionName(t *testing.T) {
	assert.Equal(t, "mrn://collection/mongodb/orders",
		mrn.New("Collection", "MongoDB", "orders"))
}

func TestCollectionMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := mrn.New("Collection", "MongoDB", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}
