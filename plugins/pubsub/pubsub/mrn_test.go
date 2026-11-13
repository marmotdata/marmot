package pubsub

import (
	"testing"

	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Pub/Sub id is unique within a project, so an asset is addressed by its bare id and the project is the zone.

func TestTopicMRN_IsTheBareTopicID(t *testing.T) {
	s := newTestSource(newFakeProject())
	assert.Equal(t, "mrn://asset/topic/googlepubsub/orders", s.assetMRN("Topic", "orders"))
}

func TestSubscriptionMRN_IsTheBareSubscriptionID(t *testing.T) {
	s := newTestSource(newFakeProject())
	assert.Equal(t, "mrn://asset/subscription/googlepubsub/orders-sub", s.assetMRN("Subscription", "orders-sub"))
}

func TestAssetMRN_CarriesTheConfiguredZone(t *testing.T) {
	s := newTestSource(newFakeProject())
	s.config.Zone = "prod"
	assert.Equal(t, "mrn://asset/topic/googlepubsub@prod/orders", s.assetMRN("Topic", "orders"))
}

func TestAssetMRN_IsStableUnderParseAndFormat(t *testing.T) {
	s := newTestSource(newFakeProject())
	s.config.Zone = "prod"

	for _, original := range []string{s.assetMRN("Topic", "orders"), s.assetMRN("Subscription", "orders-sub")} {
		parsed, err := mrn.ParseMRN(original)
		require.NoError(t, err)
		assert.Equal(t, original, parsed.String())
	}
}

// Edges point at assets other plugins own, so their MRNs are pinned here too: a drift in either plugin's naming silently drops the edge on the server.

func TestBigQueryTableMRN_MatchesTheBigQueryPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/bigquery/test-project.analytics.orders_raw",
		mrn.Asset("Table", "BigQuery", bigQueryTablePath("test-project.analytics.orders_raw")...).String())
}

func TestGCSBucketMRN_MatchesTheGCSPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/gcs/order-archive", mrn.Asset("Bucket", "GCS", "order-archive").String())
}

func TestKinesisStreamMRN_MatchesTheKinesisPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/stream/kinesis/orders-stream",
		mrn.Asset("Stream", "Kinesis", kinesisStreamName("arn:aws:kinesis:us-east-1:111122223333:stream/orders-stream")).String())
}

func TestEveryAssetMRN_IsTheAssetNameUnderTheProvider(t *testing.T) {
	fake := newFakeProject()
	result := discoverFake(t, newTestSource(fake), fake)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.NotEmpty(t, a.Providers)

		assert.Equal(t, mrn.Asset(a.Type, a.Providers[0], *a.Name).String(), *a.MRN, "asset %s", *a.Name)
	}
}
