package firehose

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSource wires a Source to the fake API, skipping the AWS client
// construction Discover does.
func newTestSource(t *testing.T, api firehoseAPI, raw pluginsdk.RawConfig) *Source {
	t.Helper()

	source := &Source{}
	_, err := source.Validate(raw)
	require.NoError(t, err)

	source.client = api
	source.region = "us-east-1"
	return source
}

func assetByName(t *testing.T, assets []pluginsdk.Asset, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range assets {
		if aws.ToString(asset.Name) == name {
			return asset
		}
	}

	t.Fatalf("no asset named %s", name)
	return pluginsdk.Asset{}
}

func TestMeta_DescribesThePlugin(t *testing.T) {
	meta := Meta()

	assert.Equal(t, "firehose", meta.ID)
	assert.Equal(t, "AWS Firehose", meta.Name)
	assert.Equal(t, "messaging", meta.Category)
	assert.Equal(t, "experimental", meta.Status)
	assert.Equal(t, []string{"Assets", "Lineage"}, meta.Features)
	assert.NotEmpty(t, meta.ConfigSpec)
}

func TestValidate_DefaultsLineageAndDestinationConfigOn(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	assert.True(t, source.config.DiscoverLineage)
	assert.True(t, source.config.IncludeDestinationConfig)
}

func TestValidate_KeepsLineageOffWhenAskedTo(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{"discover_lineage": false})

	require.NoError(t, err)
	assert.False(t, source.config.DiscoverLineage)
}

// Everything AWS is optional: with no credentials block the plugin falls
// back to the ambient AWS environment, like the other AWS plugins.
func TestValidate_AcceptsAnEmptyConfig(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{})

	require.NoError(t, err)
	assert.Nil(t, source.config.AWSConfig)
}

func TestValidate_RejectsAnEndpointThatIsNotAURL(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"credentials": map[string]any{"endpoint": "not a url"},
	})

	require.Error(t, err)
}

func TestValidate_AcceptsACustomEndpoint(t *testing.T) {
	source := &Source{}

	_, err := source.Validate(pluginsdk.RawConfig{
		"credentials": map[string]any{
			"region":   "us-east-1",
			"endpoint": "http://localhost:15559",
			"id":       "test",
			"secret":   "test",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, source.config.AWSConfig)
	assert.Equal(t, "http://localhost:15559", source.config.Credentials.Endpoint)
}

func TestDiscover_CreatesOneAssetPerDeliveryStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())
	api.add(logsToOpenSearch())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())

	require.NoError(t, err)
	require.Len(t, result.Assets, 3)
	for _, asset := range result.Assets {
		assert.Equal(t, "DeliveryStream", asset.Type)
		assert.Equal(t, []string{"Firehose"}, asset.Providers)
	}
}

func TestDiscover_RecordsTheStreamMetadata(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "orders-to-s3").Metadata
	assert.Equal(t, "arn:aws:firehose:us-east-1:123456789012:deliverystream/orders-to-s3", metadata["arn"])
	assert.Equal(t, "ACTIVE", metadata["status"])
	assert.Equal(t, "KinesisStreamAsSource", metadata["stream_type"])
	assert.Equal(t, "1", metadata["version_id"])
	assert.Equal(t, "us-east-1", metadata["region"])
	assert.Equal(t, "kinesis", metadata["source_type"])
	assert.Equal(t, "orders", metadata["source_kinesis_stream"])
	assert.Equal(t, "extended_s3", metadata["destination_type"])
	assert.Equal(t, 1, metadata["destination_count"])
}

// The ARN says where the stream really lives, which matters when a role
// points at a region other than the configured one.
func TestDiscover_TakesTheRegionFromTheStreamARN(t *testing.T) {
	api := newFakeFirehose()
	api.add(logsToOpenSearch())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "eu-west-1", assetByName(t, result.Assets, "logs-to-opensearch").Metadata["region"])
}

func TestDiscover_RecordsTheMSKSource(t *testing.T) {
	api := newFakeFirehose()
	api.add(eventsFromMSK())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "events-from-msk").Metadata
	assert.Equal(t, "msk", metadata["source_type"])
	assert.Equal(t, "events-cluster", metadata["source_msk_cluster"])
	assert.Equal(t, "events", metadata["source_msk_topic"])
}

func TestDiscover_ADirectPutStreamHasNoSourceAsset(t *testing.T) {
	api := newFakeFirehose()
	api.add(clicksToRedshift())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "clicks-to-redshift").Metadata
	assert.Equal(t, "direct_put", metadata["source_type"])
	assert.NotContains(t, metadata, "source_kinesis_stream")
}

func TestDiscover_RecordsTheDestinationSettings(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	settings, ok := assetByName(t, result.Assets, "orders-to-s3").Metadata["destination"].(map[string]any)
	require.True(t, ok, "destination should be a sub-map")
	assert.Equal(t, "marmot-lake", settings["bucket"])
	assert.Equal(t, "orders", settings["glue_table"])
}

func TestDiscover_LeavesOutTheDestinationSettingsWhenTurnedOff(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{"include_destination_config": false}).discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "orders-to-s3").Metadata
	assert.NotContains(t, metadata, "destination")
	assert.Equal(t, "extended_s3", metadata["destination_type"], "the destination type is always worth knowing")
}

func TestDiscover_LinksTheConsoleForEachStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	links := assetByName(t, result.Assets, "orders-to-s3").ExternalLinks
	require.Len(t, links, 1)
	assert.Equal(t, "Open in AWS Console", links[0].Name)
	assert.Equal(t, "https://us-east-1.console.aws.amazon.com/firehose/home?region=us-east-1#/details/orders-to-s3", links[0].URL)
}

func TestDiscover_TurnsAWSTagsIntoMetadata(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String("team"), Value: aws.String("data")},
	}

	source := newTestSource(t, api, pluginsdk.RawConfig{"tags_to_metadata": true})
	result, err := source.discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "orders-to-s3").Metadata
	assert.Equal(t, "prod", metadata["tag_env"])
	assert.Equal(t, "data", metadata["tag_team"])
}

func TestDiscover_LeavesOutAWSTagsByDefault(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.NotContains(t, assetByName(t, result.Assets, "orders-to-s3").Metadata, "tag_env")
}

func TestDiscover_KeepsOnlyTheIncludedAWSTags(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String("team"), Value: aws.String("data")},
	}

	source := newTestSource(t, api, pluginsdk.RawConfig{
		"tags_to_metadata": true,
		"include_tags":     []any{"env"},
	})
	result, err := source.discover(t.Context())
	require.NoError(t, err)

	metadata := assetByName(t, result.Assets, "orders-to-s3").Metadata
	assert.Equal(t, "prod", metadata["tag_env"])
	assert.NotContains(t, metadata, "tag_team")
}

// One stream failing to describe is normal in a large account, for example
// while it is being deleted, and must not lose the rest of the run.
func TestDiscover_KeepsGoingWhenOneStreamCannotBeDescribed(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())
	api.describeErrors["clicks-to-redshift"] = true

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())

	require.NoError(t, err)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, "orders-to-s3", aws.ToString(result.Assets[0].Name))
}

func TestDiscover_KinesisSourceFeedsTheStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://stream/kinesis/orders",
		Target: "mrn://deliverystream/firehose/orders-to-s3",
		Type:   "FEEDS",
	})
}

func TestDiscover_MSKTopicFeedsTheStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(eventsFromMSK())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://topic/kafka/events",
		Target: "mrn://deliverystream/firehose/events-from-msk",
		Type:   "FEEDS",
	})
}

func TestDiscover_StreamProducesIntoTheBucketAndTheGlueTable(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/orders-to-s3",
		Target: "mrn://bucket/s3/marmot-lake",
		Type:   "PRODUCES",
	})
	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/orders-to-s3",
		Target: "mrn://table/glue/orders",
		Type:   "PRODUCES",
	})
}

func TestDiscover_StreamProducesIntoTheRedshiftTable(t *testing.T) {
	api := newFakeFirehose()
	api.add(clicksToRedshift())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/clicks-to-redshift",
		Target: "mrn://table/redshift/analytics.public.clicks",
		Type:   "PRODUCES",
	})
}

func TestDiscover_StreamProducesIntoTheOpenSearchIndex(t *testing.T) {
	api := newFakeFirehose()
	api.add(logsToOpenSearch())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())
	require.NoError(t, err)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/logs-to-opensearch",
		Target: "mrn://table/opensearch/orders-index",
		Type:   "PRODUCES",
	})
}

func TestDiscover_MakesNoLineageWhenTurnedOff(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{"discover_lineage": false}).discover(t.Context())

	require.NoError(t, err)
	assert.NotEmpty(t, result.Assets)
	assert.Empty(t, result.Lineage)
}

// Firehose has no throughput to report outside CloudWatch, so the plugin
// deliberately emits no statistics.
func TestDiscover_EmitsNoStatistics(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	result, err := newTestSource(t, api, pluginsdk.RawConfig{}).discover(t.Context())

	require.NoError(t, err)
	assert.Empty(t, result.Statistics)
}

func TestDiscover_EmptyAccountYieldsNothing(t *testing.T) {
	result, err := newTestSource(t, newFakeFirehose(), pluginsdk.RawConfig{}).discover(t.Context())

	require.NoError(t, err)
	assert.Empty(t, result.Assets)
	assert.Empty(t, result.Lineage)
}
