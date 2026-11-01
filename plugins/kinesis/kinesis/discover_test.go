package kinesis

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Listing streams

func TestListStreamNames_FollowsNextToken(t *testing.T) {
	f := &fakeAPI{listStreams: func(in *kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		if in.NextToken == nil {
			return &kinesis.ListStreamsOutput{StreamNames: []string{"a"}, NextToken: aws.String("t1")}, nil
		}
		return &kinesis.ListStreamsOutput{StreamNames: []string{"b"}}, nil
	}}

	names, err := listStreamNames(t.Context(), f)
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, names)
	require.Len(t, f.listStreamsCalls, 2)
	assert.Equal(t, "t1", *f.listStreamsCalls[1].NextToken)
	assert.Nil(t, f.listStreamsCalls[1].ExclusiveStartStreamName, "a token page must not also name a start stream")
}

func TestListStreamNames_FallsBackToExclusiveStartStreamName(t *testing.T) {
	// Older API versions send no token, only HasMoreStreams, and expect
	// the last name back as the cursor.
	f := &fakeAPI{listStreams: func(in *kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		if in.ExclusiveStartStreamName == nil {
			return &kinesis.ListStreamsOutput{StreamNames: []string{"a", "b"}, HasMoreStreams: aws.Bool(true)}, nil
		}
		return &kinesis.ListStreamsOutput{StreamNames: []string{"c"}, HasMoreStreams: aws.Bool(false)}, nil
	}}

	names, err := listStreamNames(t.Context(), f)
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b", "c"}, names)
	require.Len(t, f.listStreamsCalls, 2)
	assert.Equal(t, "b", *f.listStreamsCalls[1].ExclusiveStartStreamName)
}

func TestListStreamNames_StopsWhenMoreIsPromisedButNothingIsReturned(t *testing.T) {
	// HasMoreStreams with an empty page gives no cursor to continue from.
	// Re-sending the same request would spin forever.
	f := &fakeAPI{listStreams: func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		return &kinesis.ListStreamsOutput{HasMoreStreams: aws.Bool(true)}, nil
	}}

	names, err := listStreamNames(t.Context(), f)
	require.NoError(t, err)

	assert.Empty(t, names)
	assert.Len(t, f.listStreamsCalls, 1)
}

func TestListStreamNames_StopsAtThePageCap(t *testing.T) {
	f := &fakeAPI{listStreams: func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		return &kinesis.ListStreamsOutput{StreamNames: []string{"x"}, NextToken: aws.String("again")}, nil
	}}

	names, err := listStreamNames(t.Context(), f)
	require.NoError(t, err)

	assert.Len(t, f.listStreamsCalls, maxPages)
	assert.Len(t, names, maxPages, "what was read before the cap is kept")
}

func TestListStreamNames_StopsOnTheFirstError(t *testing.T) {
	f := &fakeAPI{listStreams: func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		return nil, errors.New("AccessDeniedException")
	}}

	_, err := listStreamNames(t.Context(), f)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "listing streams")
	assert.Len(t, f.listStreamsCalls, 1, "an error is not retried")
}

// Listing shards

func TestListShards_FollowUpPageCarriesOnlyTheToken(t *testing.T) {
	// The API rejects a request that has both NextToken and StreamName.
	f := &fakeAPI{listShards: func(in *kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
		if in.NextToken == nil {
			return &kinesis.ListShardsOutput{Shards: []types.Shard{openShard("s0")}, NextToken: aws.String("t1")}, nil
		}
		return &kinesis.ListShardsOutput{Shards: []types.Shard{openShard("s1")}}, nil
	}}

	shards, err := listShards(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Len(t, shards, 2)
	require.Len(t, f.listShardsCalls, 2)
	assert.Equal(t, "orders", *f.listShardsCalls[0].StreamName)
	assert.Equal(t, "t1", *f.listShardsCalls[1].NextToken)
	assert.Nil(t, f.listShardsCalls[1].StreamName)
}

func TestListShards_StopsAtThePageCap(t *testing.T) {
	f := &fakeAPI{listShards: func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
		return &kinesis.ListShardsOutput{NextToken: aws.String("again")}, nil
	}}

	_, err := listShards(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Len(t, f.listShardsCalls, maxPages)
}

// Listing consumers

func TestListConsumers_KeepsTheStreamARNOnFollowUpPages(t *testing.T) {
	// Unlike ListShards, ListStreamConsumers requires the ARN on every
	// request.
	const arn = "arn:aws:kinesis:us-east-1:123456789012:stream/orders"
	f := &fakeAPI{listStreamConsumers: func(in *kinesis.ListStreamConsumersInput) (*kinesis.ListStreamConsumersOutput, error) {
		if in.NextToken == nil {
			return &kinesis.ListStreamConsumersOutput{Consumers: []types.Consumer{consumer("a")}, NextToken: aws.String("t1")}, nil
		}
		return &kinesis.ListStreamConsumersOutput{Consumers: []types.Consumer{consumer("b")}}, nil
	}}

	consumers, err := listConsumers(t.Context(), f, arn)
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, consumerNames(consumers))
	require.Len(t, f.listStreamConsumersCalls, 2)
	assert.Equal(t, arn, *f.listStreamConsumersCalls[1].StreamARN)
	assert.Equal(t, "t1", *f.listStreamConsumersCalls[1].NextToken)
}

// Listing tags

func TestListTags_PagesByTheLastTagKey(t *testing.T) {
	f := &fakeAPI{listTagsForStream: func(in *kinesis.ListTagsForStreamInput) (*kinesis.ListTagsForStreamOutput, error) {
		if in.ExclusiveStartTagKey == nil {
			return &kinesis.ListTagsForStreamOutput{
				Tags:        []types.Tag{{Key: aws.String("env"), Value: aws.String("test")}},
				HasMoreTags: aws.Bool(true),
			}, nil
		}
		return &kinesis.ListTagsForStreamOutput{
			Tags:        []types.Tag{{Key: aws.String("team"), Value: aws.String("data")}},
			HasMoreTags: aws.Bool(false),
		}, nil
	}}

	tags, err := listTags(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"env": "test", "team": "data"}, tags)
	require.Len(t, f.listTagsForStreamCalls, 2)
	assert.Equal(t, "env", *f.listTagsForStreamCalls[1].ExclusiveStartTagKey)
	assert.Equal(t, "orders", *f.listTagsForStreamCalls[1].StreamName)
}

func TestListTags_StopsWhenMoreIsPromisedButNothingIsReturned(t *testing.T) {
	f := &fakeAPI{listTagsForStream: func(*kinesis.ListTagsForStreamInput) (*kinesis.ListTagsForStreamOutput, error) {
		return &kinesis.ListTagsForStreamOutput{HasMoreTags: aws.Bool(true)}, nil
	}}

	tags, err := listTags(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Empty(t, tags)
	assert.Len(t, f.listTagsForStreamCalls, 1)
}

// Metadata assembly from the summary

func TestStreamMetadata_MapsTheSummaryFields(t *testing.T) {
	summary := ordersSummary()
	summary.ConsumerCount = aws.Int32(1)

	m := streamMetadata(summary)

	assert.Equal(t, "arn:aws:kinesis:us-east-1:123456789012:stream/orders", m["arn"])
	assert.Equal(t, "ACTIVE", m["status"])
	assert.Equal(t, "PROVISIONED", m["stream_mode"])
	assert.Equal(t, 48, m["retention_hours"])
	assert.Equal(t, 2, m["open_shard_count"])
	assert.Equal(t, 1, m["consumer_count"])
	assert.Equal(t, "KMS", m["encryption_type"])
	assert.Equal(t, "alias/aws/kinesis", m["kms_key_id"])
	assert.Equal(t, "2026-09-07T21:12:22Z", m["created_at"])
}

func TestStreamMetadata_LeavesOutWhatTheAPIDidNotReturn(t *testing.T) {
	// A missing count must not read as a count of zero.
	m := streamMetadata(&types.StreamDescriptionSummary{StreamName: aws.String("bare")})

	assert.Empty(t, m)
}

func TestStreamMetadata_KMSKeyOnlyWhenEncrypted(t *testing.T) {
	summary := ordersSummary()
	summary.EncryptionType = types.EncryptionTypeNone

	m := streamMetadata(summary)

	assert.Equal(t, "NONE", m["encryption_type"])
	assert.NotContains(t, m, "kms_key_id")
}

func TestStreamMetadata_FlattensEnhancedMonitoring(t *testing.T) {
	summary := ordersSummary()
	summary.EnhancedMonitoring = []types.EnhancedMetrics{
		{ShardLevelMetrics: []types.MetricsName{types.MetricsNameIncomingBytes, types.MetricsNameIncomingRecords}},
		{ShardLevelMetrics: []types.MetricsName{types.MetricsNameIteratorAgeMilliseconds}},
	}

	m := streamMetadata(summary)

	assert.Equal(t, []string{"IncomingBytes", "IncomingRecords", "IteratorAgeMilliseconds"}, m["enhanced_monitoring"])
}

func TestStreamMetadata_OmitsEmptyEnhancedMonitoring(t *testing.T) {
	// moto and a fresh stream both return one block with no metrics.
	m := streamMetadata(ordersSummary())

	assert.NotContains(t, m, "enhanced_monitoring")
}

// Asset assembly

func TestStreamAsset_IdentityIsTheBareStreamName(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	require.NotNil(t, a.Name)
	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, "Stream", a.Type)
	assert.Equal(t, []string{"Kinesis"}, a.Providers)
	assert.Equal(t, "mrn://stream/kinesis/orders", *a.MRN)
}

func TestStreamAsset_CountsOpenAndClosedShards(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{
		summary:   ordersSummary(),
		shards:    []types.Shard{closedShard("s0"), openShard("s1"), openShard("s2")},
		hasShards: true,
	})

	assert.Equal(t, 3, a.Metadata["shard_count"])
	assert.Equal(t, 2, a.Metadata["open_shard_count"], "listed shards override the summary's count")
}

func TestStreamAsset_KeepsTheSummaryShardCountWhenShardsAreNotListed(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	assert.NotContains(t, a.Metadata, "shard_count")
	assert.Equal(t, 2, a.Metadata["open_shard_count"])
}

func TestStreamAsset_ListsConsumerNames(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{
		summary:      ordersSummary(),
		consumers:    []types.Consumer{consumer("orders-consumer"), consumer("audit")},
		hasConsumers: true,
	})

	assert.Equal(t, []string{"orders-consumer", "audit"}, a.Metadata["consumers"])
	assert.Equal(t, 2, a.Metadata["consumer_count"], "counted from the list when the summary has no count")
}

func TestStreamAsset_SummaryConsumerCountWins(t *testing.T) {
	// The summary counts consumers in every state; the list may lag.
	summary := ordersSummary()
	summary.ConsumerCount = aws.Int32(3)

	a := newSource(nil).streamAsset(streamInfo{
		summary:      summary,
		consumers:    []types.Consumer{consumer("orders-consumer")},
		hasConsumers: true,
	})

	assert.Equal(t, 3, a.Metadata["consumer_count"])
}

func TestStreamAsset_NoConsumersLeavesTheKeyOut(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary(), hasConsumers: true})

	assert.NotContains(t, a.Metadata, "consumers")
	assert.Equal(t, 0, a.Metadata["consumer_count"], "listed and found none is a real zero")
}

func TestStreamAsset_TagsBecomeMetadata(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{
		summary: ordersSummary(),
		tags:    map[string]string{"team": "data", "env": "test"},
	})

	assert.Equal(t, "data", a.Metadata["tag_team"])
	assert.Equal(t, "test", a.Metadata["tag_env"])
}

func TestStreamAsset_IncludeTagsFiltersTagMetadata(t *testing.T) {
	s := newSource(nil)
	s.config.IncludeTags = []string{"team"}

	a := s.streamAsset(streamInfo{
		summary: ordersSummary(),
		tags:    map[string]string{"team": "data", "env": "test"},
	})

	assert.Equal(t, "data", a.Metadata["tag_team"])
	assert.NotContains(t, a.Metadata, "tag_env")
}

func TestStreamAsset_TagsToMetadataOffDropsTagMetadata(t *testing.T) {
	s := newSource(nil)
	s.config.TagsToMetadata = false

	a := s.streamAsset(streamInfo{
		summary: ordersSummary(),
		tags:    map[string]string{"team": "data"},
	})

	assert.NotContains(t, a.Metadata, "tag_team")
}

func TestStreamAsset_DescriptionTagBecomesTheDescription(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{
		summary: ordersSummary(),
		tags:    map[string]string{"description": " Orders placed in the shop "},
	})

	require.NotNil(t, a.Description)
	assert.Equal(t, "Orders placed in the shop", *a.Description)
}

func TestStreamAsset_HasNoDescriptionWithoutTheTag(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	assert.Nil(t, a.Description)
}

func TestStreamAsset_RegionComesFromTheARN(t *testing.T) {
	s := newSource(nil)
	s.region = "eu-west-1"

	a := s.streamAsset(streamInfo{summary: ordersSummary()})

	assert.Equal(t, "us-east-1", a.Metadata["region"], "the ARN names where the stream really is")
}

func TestStreamAsset_FallsBackToTheClientRegion(t *testing.T) {
	summary := ordersSummary()
	summary.StreamARN = nil

	a := newSource(nil).streamAsset(streamInfo{summary: summary})

	assert.Equal(t, "us-east-1", a.Metadata["region"])
}

func TestStreamAsset_LinksToTheConsole(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	const want = "https://us-east-1.console.aws.amazon.com/kinesis/home?region=us-east-1#/streams/details/orders/monitoring"
	assert.Equal(t, want, a.Metadata["url"])
	require.Len(t, a.ExternalLinks, 1)
	assert.Equal(t, "Open in AWS Console", a.ExternalLinks[0].Name)
	assert.Equal(t, want, a.ExternalLinks[0].URL)
}

func TestStreamAsset_InterpolatesConfiguredTags(t *testing.T) {
	s := newSource(nil)
	s.config.Tags = []string{"kinesis", "${stream_mode}"}

	a := s.streamAsset(streamInfo{summary: ordersSummary()})

	assert.Equal(t, []string{"kinesis", "PROVISIONED"}, a.Tags)
}

func TestStreamAsset_RecordsItselfAsTheSource(t *testing.T) {
	a := newSource(nil).streamAsset(streamInfo{summary: ordersSummary()})

	require.Len(t, a.Sources, 1)
	assert.Equal(t, "Kinesis", a.Sources[0].Name)
	assert.Equal(t, 1, a.Sources[0].Priority)
}

func TestConsoleURL_NeedsBothParts(t *testing.T) {
	assert.Empty(t, consoleURL("", "orders"))
	assert.Empty(t, consoleURL("us-east-1", ""))
}

func TestRegionFromARN_ReadsTheRegionField(t *testing.T) {
	assert.Equal(t, "eu-central-1", regionFromARN("arn:aws:kinesis:eu-central-1:123456789012:stream/orders"))
}

func TestRegionFromARN_RejectsOtherShapes(t *testing.T) {
	assert.Empty(t, regionFromARN(""))
	assert.Empty(t, regionFromARN("orders"))
	assert.Empty(t, regionFromARN("not:an:arn:at:all:x"))
}

func TestCountOpenShards_TreatsAMissingRangeAsOpen(t *testing.T) {
	assert.Equal(t, 1, countOpenShards([]types.Shard{{ShardId: aws.String("s0")}}))
}

// Discovery over the fake

func TestDiscoverStreams_BuildsOneAssetPerStream(t *testing.T) {
	s := newSource(ordersAPI())

	assets, err := s.discoverStreams(t.Context())
	require.NoError(t, err)
	require.Len(t, assets, 1)

	a := assets[0]
	assert.Equal(t, "orders", *a.Name)
	assert.Equal(t, 2, a.Metadata["shard_count"])
	assert.Equal(t, []string{"orders-consumer"}, a.Metadata["consumers"])
	assert.Equal(t, "data", a.Metadata["tag_team"])
}

func TestDiscoverStreams_AddressesConsumersByTheStreamARN(t *testing.T) {
	f := ordersAPI()

	_, err := newSource(f).discoverStreams(t.Context())
	require.NoError(t, err)

	require.Len(t, f.listStreamConsumersCalls, 1)
	assert.Equal(t, "arn:aws:kinesis:us-east-1:123456789012:stream/orders", *f.listStreamConsumersCalls[0].StreamARN)
}

func TestDiscoverStreams_SkipsAStreamThatCannotBeDescribed(t *testing.T) {
	f := ordersAPI()
	f.listStreams = func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		return &kinesis.ListStreamsOutput{StreamNames: []string{"orders", "broken"}}, nil
	}
	f.describeStreamSummary = func(in *kinesis.DescribeStreamSummaryInput) (*kinesis.DescribeStreamSummaryOutput, error) {
		if *in.StreamName == "broken" {
			return nil, errors.New("ResourceNotFoundException")
		}
		return &kinesis.DescribeStreamSummaryOutput{StreamDescriptionSummary: ordersSummary()}, nil
	}

	assets, err := newSource(f).discoverStreams(t.Context())
	require.NoError(t, err)

	require.Len(t, assets, 1)
	assert.Equal(t, "orders", *assets[0].Name)
}

func TestDiscoverStreams_KeepsTheStreamWhenShardsCannotBeListed(t *testing.T) {
	f := ordersAPI()
	f.listShards = func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
		return nil, errors.New("AccessDeniedException")
	}

	assets, err := newSource(f).discoverStreams(t.Context())
	require.NoError(t, err)

	require.Len(t, assets, 1)
	assert.NotContains(t, assets[0].Metadata, "shard_count")
	assert.Equal(t, 2, assets[0].Metadata["open_shard_count"], "the summary still says how many are open")
}

func TestDiscoverStreams_FailsWhenListingFails(t *testing.T) {
	f := ordersAPI()
	f.listStreams = func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
		return nil, errors.New("UnrecognizedClientException")
	}

	_, err := newSource(f).discoverStreams(t.Context())
	require.Error(t, err)
}

func TestDiscoverStreams_SkipsShardsAndConsumersWhenSwitchedOff(t *testing.T) {
	f := ordersAPI()
	s := newSource(f)
	s.config.IncludeShards = false
	s.config.IncludeConsumers = false

	assets, err := s.discoverStreams(t.Context())
	require.NoError(t, err)

	assert.Empty(t, f.listShardsCalls)
	assert.Empty(t, f.listStreamConsumersCalls)
	require.Len(t, assets, 1)
	assert.NotContains(t, assets[0].Metadata, "shard_count")
	assert.NotContains(t, assets[0].Metadata, "consumers")
}

func TestDiscoverStreams_SkipsTagsWhenSwitchedOff(t *testing.T) {
	f := ordersAPI()
	s := newSource(f)
	s.config.TagsToMetadata = false

	_, err := s.discoverStreams(t.Context())
	require.NoError(t, err)

	assert.Empty(t, f.listTagsForStreamCalls)
}
