package kinesis

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A stream name is unique within an account and region, so the MRN is
// built from the bare name. Nothing else about the stream is stable: the
// ARN carries the account, and the account is not part of Marmot's
// identity for any AWS plugin.

func TestStreamMRN_IsTheBareStreamName(t *testing.T) {
	assert.Equal(t, "mrn://stream/kinesis/orders", assetMRN("Stream", "orders"))
}

func TestStreamMRN_KeepsDotsAndHyphens(t *testing.T) {
	// Kinesis allows letters, digits, underscores, dots and hyphens. None
	// of those are rewritten by mrn.New, so the name reads back verbatim.
	assert.Equal(t, "mrn://stream/kinesis/orders.v2-eu", assetMRN("Stream", "orders.v2-eu"))
}

func TestStreamMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the
	// parts back through mrn.New, so it has to survive byte-identical.
	original := assetMRN("Stream", "orders.v2-eu")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestStreamAsset_MRNAgreesWithItsOwnFields(t *testing.T) {
	// The server rebuilds identity from (Type, Providers[0], Name), so the
	// MRN the plugin declares has to be exactly that.
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	require.NotNil(t, a.MRN)
	require.NotNil(t, a.Name)
	require.NotEmpty(t, a.Providers)
	assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN)
}

func TestStreamMRN_MatchesWhatAnOpenMetadataImportProduces(t *testing.T) {
	// plugins/openmetadata projects a Kinesis service onto provider
	// "Kinesis", type "Stream" and the bare stream name. This plugin has
	// to land on the same MRN or the handover leaves two assets.
	assert.Equal(t, mrn.New("Stream", "Kinesis", "orders"), assetMRN("Stream", "orders"))
}
