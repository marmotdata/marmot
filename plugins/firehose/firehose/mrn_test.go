package firehose

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A delivery stream name is unique within a region, so the bare name is
// the identity and nothing has to be folded into it.

func TestDeliveryStreamMRN_IsTheBareStreamName(t *testing.T) {
	s := &Source{config: &Config{}}
	assert.Equal(t, "mrn://asset/deliverystream/firehose/orders-to-s3", s.assetMRN(assetType, "orders-to-s3"))
}

func TestDeliveryStreamMRN_CarriesTheZone(t *testing.T) {
	s := &Source{config: &Config{}}
	s.config.Zone = "prod-eu"
	assert.Equal(t, "mrn://asset/deliverystream/firehose@prod-eu/orders-to-s3", s.assetMRN(assetType, "orders-to-s3"))
}

func TestDeliveryStreamMRN_IsStableUnderParseAndFormat(t *testing.T) {
	s := &Source{config: &Config{}}
	s.config.Zone = "prod-eu"
	original := s.assetMRN(assetType, "orders-to-s3")

	parsed, err := mrn.ParseMRN(original)
	require.NoError(t, err)

	assert.Equal(t, original, parsed.String())
}

// Every lineage endpoint belongs to another plugin. These are the exact
// MRNs those plugins produce; if one drifts, the edge points at nothing
// and the server drops it.

func TestKinesisSourceMRN_MatchesTheKinesisPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/stream/kinesis/orders", mrn.Asset("Stream", "Kinesis", "orders").String())
}

func TestKafkaSourceMRN_MatchesTheKafkaPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/topic/kafka/events", mrn.Asset("Topic", "Kafka", "events").String())
}

func TestS3DestinationMRN_MatchesTheS3Plugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/bucket/s3/marmot-lake", mrn.Asset("Bucket", "S3", "marmot-lake").String())
}

func TestRedshiftDestinationMRN_MatchesTheRedshiftNameShape(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/redshift/analytics.public.orders", mrn.Asset("Table", "Redshift", "analytics", "public", "orders").String())
}

func TestOpenSearchDestinationMRN_MatchesTheOpenSearchPlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/opensearch/orders-index", mrn.Asset("Table", "OpenSearch", "orders-index").String())
}

func TestGlueDestinationMRN_MatchesTheGluePlugin(t *testing.T) {
	assert.Equal(t, "mrn://asset/table/glue/shop.orders", mrn.Asset("Table", "Glue", "shop", "orders").String())
}

// The server rebuilds identity from an asset's own Type, first provider
// and Name. An MRN that does not match that is unreachable.
func TestEveryAssetMRN_MatchesItsOwnTypeProviderAndName(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())
	api.add(logsToOpenSearch())
	api.add(eventsFromMSK())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)
	require.Len(t, result.Assets, 4)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		require.NotNil(t, asset.MRN)
		require.Len(t, asset.Providers, 1)

		assert.Equal(t, mrn.Asset(asset.Type, asset.Providers[0], *asset.Name).String(), *asset.MRN)
	}
}

// A lineage edge whose endpoint is not a well formed MRN is silently
// dropped by the server, so every edge is checked for the shape.
func TestEveryLineageEndpoint_IsAWellFormedMRN(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())
	api.add(logsToOpenSearch())
	api.add(eventsFromMSK())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, result.Lineage)

	for _, edge := range result.Lineage {
		for _, endpoint := range []string{edge.Source, edge.Target} {
			parsed, err := mrn.ParseMRN(endpoint)
			require.NoError(t, err, endpoint)
			assert.Equal(t, endpoint, parsed.String())
		}
	}
}

// A stream name with a space or a slash is legal in Firehose neither, but
// mrn.New sanitises anyway, and the asset MRN has to keep matching what
// the server derives from the same name.
func TestDeliveryStreamMRN_SurvivesASanitisedName(t *testing.T) {
	description := ordersToS3()
	description.DeliveryStreamName = aws.String("orders to s3")

	api := newFakeFirehose()
	api.add(description)

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)
	require.Len(t, result.Assets, 1)

	asset := result.Assets[0]
	assert.Equal(t, "mrn://asset/deliverystream/firehose/orders-to-s3", *asset.MRN)
	assert.Equal(t, mrn.Asset(asset.Type, asset.Providers[0], *asset.Name).String(), *asset.MRN)
}
