package kinesis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// sampleLimit is how many records one preview reads.
const sampleLimit = 20

// sampleColumns are the preview columns, in order.
var sampleColumns = []string{"sequence_number", "partition_key", "arrival_time", "data"}

// FetchSampleData implements pluginsdk.DataFetcher. It reads the oldest
// retained records from the stream's first open shard.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil || a.Name == nil || *a.Name == "" {
		return nil, nil, fmt.Errorf("asset has no stream name")
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	if err := s.initClient(ctx, rawConfig); err != nil {
		return nil, nil, err
	}

	log.Debug().Str("stream", *a.Name).Msg("Fetching sample data")

	return sampleRecords(ctx, s.client, *a.Name)
}

// sampleRecords reads up to sampleLimit records from the first open
// shard, starting at the oldest one still retained. One shard and one
// call keep a preview cheap on a busy stream.
func sampleRecords(ctx context.Context, client api, streamName string) ([]string, [][]interface{}, error) {
	shards, err := listShards(ctx, client, streamName)
	if err != nil {
		return nil, nil, err
	}

	shard, ok := firstOpenShard(shards)
	if !ok {
		return nil, nil, fmt.Errorf("stream %s has no open shards", streamName)
	}

	iterator, err := client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
		StreamName:        &streamName,
		ShardId:           shard.ShardId,
		ShardIteratorType: types.ShardIteratorTypeTrimHorizon,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("getting shard iterator: %w", err)
	}

	out, err := client.GetRecords(ctx, &kinesis.GetRecordsInput{
		ShardIterator: iterator.ShardIterator,
		Limit:         aws.Int32(sampleLimit),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("getting records: %w", err)
	}

	rows := make([][]interface{}, 0, len(out.Records))
	for _, record := range out.Records {
		rows = append(rows, recordRow(record))
	}

	return sampleColumns, rows, nil
}

func firstOpenShard(shards []types.Shard) (types.Shard, bool) {
	for _, shard := range shards {
		if isOpenShard(shard) && shard.ShardId != nil {
			return shard, true
		}
	}
	return types.Shard{}, false
}

// recordRow lays one record out in sampleColumns order.
func recordRow(record types.Record) []interface{} {
	var arrival interface{}
	if record.ApproximateArrivalTimestamp != nil {
		arrival = record.ApproximateArrivalTimestamp.UTC().Format(time.RFC3339)
	}

	return []interface{}{
		stringValue(record.SequenceNumber),
		stringValue(record.PartitionKey),
		arrival,
		decodeRecordData(record.Data),
	}
}

// decodeRecordData renders a record payload for a preview table. Text
// is shown as text, with JSON compacted onto one line; anything else is
// base64 so binary payloads stay copyable.
func decodeRecordData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	if json.Valid(data) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, data); err == nil {
			return compact.String()
		}
	}

	if utf8.Valid(data) {
		return string(data)
	}

	return base64.StdEncoding.EncodeToString(data)
}
