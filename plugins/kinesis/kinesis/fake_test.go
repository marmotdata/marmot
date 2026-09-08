package kinesis

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// fakeAPI stands in for the Kinesis client. Each method delegates to the
// matching function field and records the request it was given, so a
// test can script responses and inspect what the plugin asked for.
type fakeAPI struct {
	listStreams           func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error)
	describeStreamSummary func(*kinesis.DescribeStreamSummaryInput) (*kinesis.DescribeStreamSummaryOutput, error)
	listShards            func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error)
	listStreamConsumers   func(*kinesis.ListStreamConsumersInput) (*kinesis.ListStreamConsumersOutput, error)
	listTagsForStream     func(*kinesis.ListTagsForStreamInput) (*kinesis.ListTagsForStreamOutput, error)
	getShardIterator      func(*kinesis.GetShardIteratorInput) (*kinesis.GetShardIteratorOutput, error)
	getRecords            func(*kinesis.GetRecordsInput) (*kinesis.GetRecordsOutput, error)

	listStreamsCalls         []*kinesis.ListStreamsInput
	listShardsCalls          []*kinesis.ListShardsInput
	listStreamConsumersCalls []*kinesis.ListStreamConsumersInput
	listTagsForStreamCalls   []*kinesis.ListTagsForStreamInput
	getShardIteratorCalls    []*kinesis.GetShardIteratorInput
	getRecordsCalls          []*kinesis.GetRecordsInput
}

var errNotScripted = errors.New("fake: call not scripted")

func (f *fakeAPI) ListStreams(_ context.Context, in *kinesis.ListStreamsInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error) {
	f.listStreamsCalls = append(f.listStreamsCalls, in)
	if f.listStreams == nil {
		return nil, errNotScripted
	}
	return f.listStreams(in)
}

func (f *fakeAPI) DescribeStreamSummary(_ context.Context, in *kinesis.DescribeStreamSummaryInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error) {
	if f.describeStreamSummary == nil {
		return nil, errNotScripted
	}
	return f.describeStreamSummary(in)
}

func (f *fakeAPI) ListShards(_ context.Context, in *kinesis.ListShardsInput, _ ...func(*kinesis.Options)) (*kinesis.ListShardsOutput, error) {
	f.listShardsCalls = append(f.listShardsCalls, in)
	if f.listShards == nil {
		return nil, errNotScripted
	}
	return f.listShards(in)
}

func (f *fakeAPI) ListStreamConsumers(_ context.Context, in *kinesis.ListStreamConsumersInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamConsumersOutput, error) {
	f.listStreamConsumersCalls = append(f.listStreamConsumersCalls, in)
	if f.listStreamConsumers == nil {
		return nil, errNotScripted
	}
	return f.listStreamConsumers(in)
}

func (f *fakeAPI) ListTagsForStream(_ context.Context, in *kinesis.ListTagsForStreamInput, _ ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error) {
	f.listTagsForStreamCalls = append(f.listTagsForStreamCalls, in)
	if f.listTagsForStream == nil {
		return nil, errNotScripted
	}
	return f.listTagsForStream(in)
}

func (f *fakeAPI) GetShardIterator(_ context.Context, in *kinesis.GetShardIteratorInput, _ ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error) {
	f.getShardIteratorCalls = append(f.getShardIteratorCalls, in)
	if f.getShardIterator == nil {
		return nil, errNotScripted
	}
	return f.getShardIterator(in)
}

func (f *fakeAPI) GetRecords(_ context.Context, in *kinesis.GetRecordsInput, _ ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error) {
	f.getRecordsCalls = append(f.getRecordsCalls, in)
	if f.getRecords == nil {
		return nil, errNotScripted
	}
	return f.getRecords(in)
}

// Fixtures below mirror what moto and the real API return, trimmed to
// the fields the plugin reads.

var ordersCreated = time.Date(2026, 9, 7, 21, 12, 22, 0, time.UTC)

func ordersSummary() *types.StreamDescriptionSummary {
	return &types.StreamDescriptionSummary{
		StreamName:              aws.String("orders"),
		StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/orders"),
		StreamStatus:            types.StreamStatusActive,
		StreamModeDetails:       &types.StreamModeDetails{StreamMode: types.StreamModeProvisioned},
		RetentionPeriodHours:    aws.Int32(48),
		StreamCreationTimestamp: aws.Time(ordersCreated),
		EnhancedMonitoring:      []types.EnhancedMetrics{{ShardLevelMetrics: []types.MetricsName{}}},
		EncryptionType:          types.EncryptionTypeKms,
		KeyId:                   aws.String("alias/aws/kinesis"),
		OpenShardCount:          aws.Int32(2),
	}
}

func openShard(id string) types.Shard {
	return types.Shard{
		ShardId:             aws.String(id),
		SequenceNumberRange: &types.SequenceNumberRange{StartingSequenceNumber: aws.String("1")},
	}
}

func closedShard(id string) types.Shard {
	return types.Shard{
		ShardId: aws.String(id),
		SequenceNumberRange: &types.SequenceNumberRange{
			StartingSequenceNumber: aws.String("1"),
			EndingSequenceNumber:   aws.String("99"),
		},
	}
}

func consumer(name string) types.Consumer {
	return types.Consumer{
		ConsumerName:   aws.String(name),
		ConsumerARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/orders/consumer/" + name),
		ConsumerStatus: types.ConsumerStatusActive,
	}
}

// ordersAPI scripts the happy path for one stream named orders: two open
// shards, one consumer and two tags.
func ordersAPI() *fakeAPI {
	return &fakeAPI{
		listStreams: func(*kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
			return &kinesis.ListStreamsOutput{StreamNames: []string{"orders"}, HasMoreStreams: aws.Bool(false)}, nil
		},
		describeStreamSummary: func(*kinesis.DescribeStreamSummaryInput) (*kinesis.DescribeStreamSummaryOutput, error) {
			return &kinesis.DescribeStreamSummaryOutput{StreamDescriptionSummary: ordersSummary()}, nil
		},
		listShards: func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
			return &kinesis.ListShardsOutput{Shards: []types.Shard{openShard("shardId-000000000000"), openShard("shardId-000000000001")}}, nil
		},
		listStreamConsumers: func(*kinesis.ListStreamConsumersInput) (*kinesis.ListStreamConsumersOutput, error) {
			return &kinesis.ListStreamConsumersOutput{Consumers: []types.Consumer{consumer("orders-consumer")}}, nil
		},
		listTagsForStream: func(*kinesis.ListTagsForStreamInput) (*kinesis.ListTagsForStreamOutput, error) {
			return &kinesis.ListTagsForStreamOutput{
				Tags:        []types.Tag{{Key: aws.String("env"), Value: aws.String("test")}, {Key: aws.String("team"), Value: aws.String("data")}},
				HasMoreTags: aws.Bool(false),
			}, nil
		},
	}
}

// newSource wires a Source to a fake with every option on, the way
// Validate leaves a config that names none of them.
func newSource(client api) *Source {
	return &Source{
		config: &Config{
			AWSConfig:        &pluginsdk.AWSConfig{TagsToMetadata: true},
			IncludeConsumers: true,
			IncludeShards:    true,
		},
		client: client,
		region: "us-east-1",
	}
}
