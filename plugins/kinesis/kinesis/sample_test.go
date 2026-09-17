package kinesis

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeRecordData_CompactsJSON(t *testing.T) {
	assert.Equal(t, `{"order_id":1,"amount":10.5}`,
		decodeRecordData([]byte("{\n  \"order_id\": 1,\n  \"amount\": 10.5\n}\n")))
}

func TestDecodeRecordData_KeepsPlainText(t *testing.T) {
	assert.Equal(t, "hello, stream", decodeRecordData([]byte("hello, stream")))
}

func TestDecodeRecordData_KeepsNonJSONUnicode(t *testing.T) {
	assert.Equal(t, "grüße", decodeRecordData([]byte("grüße")))
}

func TestDecodeRecordData_Base64ForBinary(t *testing.T) {
	// 0xff is never valid UTF-8, so the payload is binary.
	assert.Equal(t, "/wD+", decodeRecordData([]byte{0xff, 0x00, 0xfe}))
}

func TestDecodeRecordData_EmptyPayload(t *testing.T) {
	assert.Equal(t, "", decodeRecordData(nil))
}

func TestRecordRow_FollowsTheColumnOrder(t *testing.T) {
	arrived := time.Date(2026, 9, 7, 21, 13, 6, 0, time.UTC)

	row := recordRow(types.Record{
		SequenceNumber:              aws.String("49"),
		PartitionKey:                aws.String("order-1"),
		ApproximateArrivalTimestamp: aws.Time(arrived),
		Data:                        []byte(`{"order_id":1}`),
	})

	require.Len(t, row, len(sampleColumns))
	assert.Equal(t, "49", row[0])
	assert.Equal(t, "order-1", row[1])
	assert.Equal(t, "2026-09-07T21:13:06Z", row[2])
	assert.Equal(t, `{"order_id":1}`, row[3])
}

func TestRecordRow_MissingArrivalTimeIsNull(t *testing.T) {
	row := recordRow(types.Record{Data: []byte("x")})

	assert.Nil(t, row[2])
}

func TestFirstOpenShard_SkipsClosedShards(t *testing.T) {
	shard, ok := firstOpenShard([]types.Shard{closedShard("s0"), openShard("s1"), openShard("s2")})

	require.True(t, ok)
	assert.Equal(t, "s1", *shard.ShardId)
}

func TestFirstOpenShard_NoneOpen(t *testing.T) {
	_, ok := firstOpenShard([]types.Shard{closedShard("s0")})

	assert.False(t, ok)
}

func TestSampleRecords_ReadsTheOldestRecordsOfTheFirstOpenShard(t *testing.T) {
	f := &fakeAPI{
		listShards: func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
			return &kinesis.ListShardsOutput{Shards: []types.Shard{closedShard("s0"), openShard("s1")}}, nil
		},
		getShardIterator: func(*kinesis.GetShardIteratorInput) (*kinesis.GetShardIteratorOutput, error) {
			return &kinesis.GetShardIteratorOutput{ShardIterator: aws.String("iter")}, nil
		},
		getRecords: func(*kinesis.GetRecordsInput) (*kinesis.GetRecordsOutput, error) {
			return &kinesis.GetRecordsOutput{Records: []types.Record{
				{SequenceNumber: aws.String("1"), PartitionKey: aws.String("k"), Data: []byte(`{"a":1}`)},
			}}, nil
		},
	}

	columns, rows, err := sampleRecords(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Equal(t, sampleColumns, columns)
	require.Len(t, rows, 1)
	assert.Equal(t, `{"a":1}`, rows[0][3])

	require.Len(t, f.getShardIteratorCalls, 1)
	assert.Equal(t, "s1", *f.getShardIteratorCalls[0].ShardId)
	assert.Equal(t, types.ShardIteratorTypeTrimHorizon, f.getShardIteratorCalls[0].ShardIteratorType)
	assert.Equal(t, "orders", *f.getShardIteratorCalls[0].StreamName)

	require.Len(t, f.getRecordsCalls, 1, "one call keeps a preview cheap")
	assert.Equal(t, "iter", *f.getRecordsCalls[0].ShardIterator)
	assert.Equal(t, int32(sampleLimit), *f.getRecordsCalls[0].Limit)
}

func TestSampleRecords_FailsWhenNoShardIsOpen(t *testing.T) {
	f := &fakeAPI{listShards: func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
		return &kinesis.ListShardsOutput{Shards: []types.Shard{closedShard("s0")}}, nil
	}}

	_, _, err := sampleRecords(t.Context(), f, "orders")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "no open shards")
	assert.Empty(t, f.getShardIteratorCalls)
}

func TestSampleRecords_EmptyShardGivesNoRows(t *testing.T) {
	f := &fakeAPI{
		listShards: func(*kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
			return &kinesis.ListShardsOutput{Shards: []types.Shard{openShard("s0")}}, nil
		},
		getShardIterator: func(*kinesis.GetShardIteratorInput) (*kinesis.GetShardIteratorOutput, error) {
			return &kinesis.GetShardIteratorOutput{ShardIterator: aws.String("iter")}, nil
		},
		getRecords: func(*kinesis.GetRecordsInput) (*kinesis.GetRecordsOutput, error) {
			return &kinesis.GetRecordsOutput{}, nil
		},
	}

	columns, rows, err := sampleRecords(t.Context(), f, "orders")
	require.NoError(t, err)

	assert.Equal(t, sampleColumns, columns)
	assert.Empty(t, rows)
}

func TestFetchSampleData_RequiresAStreamName(t *testing.T) {
	s := &Source{}

	_, _, err := s.FetchSampleData(t.Context(), pluginsdk.RawConfig{}, &pluginsdk.Asset{})
	require.Error(t, err)

	_, _, err = s.FetchSampleData(t.Context(), pluginsdk.RawConfig{}, nil)
	require.Error(t, err)
}
