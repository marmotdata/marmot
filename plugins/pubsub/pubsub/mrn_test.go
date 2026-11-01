package pubsub

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Pub/Sub id is unique within a project, so an asset is addressed by its
// bare id and the project lives in metadata.

func TestTopicMRN_IsTheBareTopicID(t *testing.T) {
	assert.Equal(t, "mrn://topic/googlepubsub/orders", assetMRN("Topic", "orders"))
}

func TestSubscriptionMRN_IsTheBareSubscriptionID(t *testing.T) {
	assert.Equal(t, "mrn://subscription/googlepubsub/orders-sub", assetMRN("Subscription", "orders-sub"))
}

func TestTopicMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	// The UI splits an MRN to build a link and /assets/lookup feeds the parts
	// back through mrn.New, so an MRN has to survive that unchanged or the
	// asset becomes unreachable from the UI.
	original := assetMRN("Topic", "orders")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

func TestSubscriptionMRN_IsStableUnderTheServersRoundTrip(t *testing.T) {
	original := assetMRN("Subscription", "orders-sub")

	parsed, err := mrn.Parse(original)
	require.NoError(t, err)

	assert.Equal(t, original, mrn.New(parsed.Type, parsed.Service, parsed.Name))
}

// Edges point at assets other plugins own, so their MRNs are pinned here too:
// a drift in either plugin's naming silently drops the edge on the server.

func TestBigQueryTableMRN_MatchesTheBigQueryPlugin(t *testing.T) {
	assert.Equal(t, "mrn://table/bigquery/orders_raw",
		mrn.New("Table", "BigQuery", bigQueryTableID("test-project.analytics.orders_raw")))
}

func TestGCSBucketMRN_MatchesTheGCSPlugin(t *testing.T) {
	assert.Equal(t, "mrn://bucket/gcs/order-archive", mrn.New("Bucket", "GCS", "order-archive"))
}

func TestKinesisStreamMRN_MatchesTheKinesisPlugin(t *testing.T) {
	assert.Equal(t, "mrn://stream/kinesis/orders-stream",
		mrn.New("Stream", "Kinesis", kinesisStreamName("arn:aws:kinesis:us-east-1:111122223333:stream/orders-stream")))
}

// The server rebuilds identity from (Type, Providers[0], Name), so every
// asset's MRN has to equal mrn.New over its own fields or the asset is stored
// under a different identity than the one lineage points at.
func TestEveryAssetMRN_EqualsMRNNewOverItsOwnFields(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.NotEmpty(t, a.Providers)

		assert.Equal(t, mrn.New(a.Type, a.Providers[0], *a.Name), *a.MRN, "asset %s", *a.Name)
	}
}
