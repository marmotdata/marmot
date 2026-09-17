package kinesis

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/rs/zerolog/log"
)

// api is the slice of the Kinesis client this plugin calls. Discovery is
// written against it so tests can substitute a fake.
type api interface {
	ListStreams(ctx context.Context, in *kinesis.ListStreamsInput, opts ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error)
	DescribeStreamSummary(ctx context.Context, in *kinesis.DescribeStreamSummaryInput, opts ...func(*kinesis.Options)) (*kinesis.DescribeStreamSummaryOutput, error)
	ListShards(ctx context.Context, in *kinesis.ListShardsInput, opts ...func(*kinesis.Options)) (*kinesis.ListShardsOutput, error)
	ListStreamConsumers(ctx context.Context, in *kinesis.ListStreamConsumersInput, opts ...func(*kinesis.Options)) (*kinesis.ListStreamConsumersOutput, error)
	ListTagsForStream(ctx context.Context, in *kinesis.ListTagsForStreamInput, opts ...func(*kinesis.Options)) (*kinesis.ListTagsForStreamOutput, error)
	GetShardIterator(ctx context.Context, in *kinesis.GetShardIteratorInput, opts ...func(*kinesis.Options)) (*kinesis.GetShardIteratorOutput, error)
	GetRecords(ctx context.Context, in *kinesis.GetRecordsInput, opts ...func(*kinesis.Options)) (*kinesis.GetRecordsOutput, error)
}

// maxPages bounds every paginated listing. A server that keeps handing
// back a token would otherwise pin discovery forever.
const maxPages = 1000

// callTimeout bounds one API call.
const callTimeout = 30 * time.Second

// listStreamNames returns every stream name in the account and region.
// Newer API versions page with NextToken; older ones flag HasMoreStreams
// and expect the last name back as ExclusiveStartStreamName. The two are
// mutually exclusive on the request, so each page starts a fresh input.
func listStreamNames(ctx context.Context, client api) ([]string, error) {
	var names []string
	in := &kinesis.ListStreamsInput{}

	for page := 0; page < maxPages; page++ {
		out, err := listStreamsPage(ctx, client, in)
		if err != nil {
			return nil, fmt.Errorf("listing streams: %w", err)
		}
		names = append(names, out.StreamNames...)

		switch {
		case stringValue(out.NextToken) != "":
			in = &kinesis.ListStreamsInput{NextToken: out.NextToken}
		case out.HasMoreStreams != nil && *out.HasMoreStreams && len(out.StreamNames) > 0:
			last := out.StreamNames[len(out.StreamNames)-1]
			in = &kinesis.ListStreamsInput{ExclusiveStartStreamName: &last}
		default:
			return names, nil
		}
	}

	log.Warn().Int("pages", maxPages).Msg("Stopped listing streams at the page cap")
	return names, nil
}

func listStreamsPage(ctx context.Context, client api, in *kinesis.ListStreamsInput) (*kinesis.ListStreamsOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return client.ListStreams(ctx, in)
}

func describeStream(ctx context.Context, client api, name string) (*types.StreamDescriptionSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	out, err := client.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: &name})
	if err != nil {
		return nil, fmt.Errorf("describing stream: %w", err)
	}
	if out.StreamDescriptionSummary == nil {
		return nil, fmt.Errorf("describing stream: empty summary")
	}
	return out.StreamDescriptionSummary, nil
}

// listShards returns every shard of a stream, closed ones included. A
// request that carries NextToken must not carry StreamName, so each
// follow-up page is built from the token alone.
func listShards(ctx context.Context, client api, name string) ([]types.Shard, error) {
	var shards []types.Shard
	in := &kinesis.ListShardsInput{StreamName: &name}

	for page := 0; page < maxPages; page++ {
		out, err := listShardsPage(ctx, client, in)
		if err != nil {
			return nil, fmt.Errorf("listing shards: %w", err)
		}
		shards = append(shards, out.Shards...)

		if stringValue(out.NextToken) == "" {
			return shards, nil
		}
		in = &kinesis.ListShardsInput{NextToken: out.NextToken}
	}

	log.Warn().Str("stream", name).Int("pages", maxPages).Msg("Stopped listing shards at the page cap")
	return shards, nil
}

func listShardsPage(ctx context.Context, client api, in *kinesis.ListShardsInput) (*kinesis.ListShardsOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return client.ListShards(ctx, in)
}

// listConsumers returns the enhanced fan-out consumers registered on a
// stream. The API addresses consumers by stream ARN only.
func listConsumers(ctx context.Context, client api, streamARN string) ([]types.Consumer, error) {
	var consumers []types.Consumer
	in := &kinesis.ListStreamConsumersInput{StreamARN: &streamARN}

	for page := 0; page < maxPages; page++ {
		out, err := listConsumersPage(ctx, client, in)
		if err != nil {
			return nil, fmt.Errorf("listing consumers: %w", err)
		}
		consumers = append(consumers, out.Consumers...)

		if stringValue(out.NextToken) == "" {
			return consumers, nil
		}
		in = &kinesis.ListStreamConsumersInput{StreamARN: &streamARN, NextToken: out.NextToken}
	}

	log.Warn().Str("stream_arn", streamARN).Int("pages", maxPages).Msg("Stopped listing consumers at the page cap")
	return consumers, nil
}

func listConsumersPage(ctx context.Context, client api, in *kinesis.ListStreamConsumersInput) (*kinesis.ListStreamConsumersOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return client.ListStreamConsumers(ctx, in)
}

// listTags returns a stream's tags as a map. The API pages by tag key:
// HasMoreTags asks for the last key back as ExclusiveStartTagKey.
func listTags(ctx context.Context, client api, name string) (map[string]string, error) {
	tags := make(map[string]string)
	in := &kinesis.ListTagsForStreamInput{StreamName: &name}

	for page := 0; page < maxPages; page++ {
		out, err := listTagsPage(ctx, client, in)
		if err != nil {
			return nil, fmt.Errorf("listing tags: %w", err)
		}
		for _, tag := range out.Tags {
			if tag.Key != nil {
				tags[*tag.Key] = stringValue(tag.Value)
			}
		}

		if out.HasMoreTags == nil || !*out.HasMoreTags || len(out.Tags) == 0 {
			return tags, nil
		}
		last := out.Tags[len(out.Tags)-1].Key
		in = &kinesis.ListTagsForStreamInput{StreamName: &name, ExclusiveStartTagKey: last}
	}

	log.Warn().Str("stream", name).Int("pages", maxPages).Msg("Stopped listing tags at the page cap")
	return tags, nil
}

func listTagsPage(ctx context.Context, client api, in *kinesis.ListTagsForStreamInput) (*kinesis.ListTagsForStreamOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return client.ListTagsForStream(ctx, in)
}
