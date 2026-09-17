package firehose

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/rs/zerolog/log"
)

// firehoseAPI is the part of the Firehose API this plugin calls. The unit
// tests substitute a fake for it so they need neither AWS nor Docker.
type firehoseAPI interface {
	ListDeliveryStreams(ctx context.Context, in *firehose.ListDeliveryStreamsInput, opts ...func(*firehose.Options)) (*firehose.ListDeliveryStreamsOutput, error)
	DescribeDeliveryStream(ctx context.Context, in *firehose.DescribeDeliveryStreamInput, opts ...func(*firehose.Options)) (*firehose.DescribeDeliveryStreamOutput, error)
	ListTagsForDeliveryStream(ctx context.Context, in *firehose.ListTagsForDeliveryStreamInput, opts ...func(*firehose.Options)) (*firehose.ListTagsForDeliveryStreamOutput, error)
}

// maxPages caps every paginated call. An account holds thousands of
// streams at most, so reaching the cap means the API keeps reporting more
// pages without handing any out, and the loop would otherwise never end.
const maxPages = 200

// listDeliveryStreams returns every delivery stream name in the region.
//
// Firehose pages by name, not by token: the next call resumes after the
// last name of the previous page. Sending both would be meaningless, so
// only the name is carried forward.
func listDeliveryStreams(ctx context.Context, api firehoseAPI) ([]string, error) {
	var names []string
	var startName *string

	for page := 0; page < maxPages; page++ {
		out, err := api.ListDeliveryStreams(ctx, &firehose.ListDeliveryStreamsInput{
			ExclusiveStartDeliveryStreamName: startName,
		})
		if err != nil {
			return nil, fmt.Errorf("listing delivery streams: %w", err)
		}

		names = append(names, out.DeliveryStreamNames...)

		// A page with no names ends the walk whatever the more flag says.
		// Without this the next call would repeat the same request forever.
		if len(out.DeliveryStreamNames) == 0 || !aws.ToBool(out.HasMoreDeliveryStreams) {
			return names, nil
		}

		startName = aws.String(out.DeliveryStreamNames[len(out.DeliveryStreamNames)-1])
	}

	log.Warn().Int("pages", maxPages).Int("streams", len(names)).
		Msg("Stopped listing delivery streams at the page cap, some streams may be missing")
	return names, nil
}

// describeDeliveryStream returns one stream's description with all of its
// destinations, following the destination pages when there are several.
func describeDeliveryStream(ctx context.Context, api firehoseAPI, name string) (*types.DeliveryStreamDescription, error) {
	var combined *types.DeliveryStreamDescription
	var startDestination *string

	for page := 0; page < maxPages; page++ {
		out, err := api.DescribeDeliveryStream(ctx, &firehose.DescribeDeliveryStreamInput{
			DeliveryStreamName:          aws.String(name),
			ExclusiveStartDestinationId: startDestination,
		})
		if err != nil {
			return nil, fmt.Errorf("describing delivery stream %s: %w", name, err)
		}

		current := out.DeliveryStreamDescription
		if current == nil {
			return nil, fmt.Errorf("describing delivery stream %s: no description returned", name)
		}

		if combined == nil {
			combined = current
		} else {
			combined.Destinations = append(combined.Destinations, current.Destinations...)
		}

		if len(current.Destinations) == 0 || !aws.ToBool(current.HasMoreDestinations) {
			return combined, nil
		}

		startDestination = current.Destinations[len(current.Destinations)-1].DestinationId
		// Without a destination id there is nothing to resume from.
		if startDestination == nil {
			return combined, nil
		}
	}

	log.Warn().Str("stream", name).Int("pages", maxPages).
		Msg("Stopped listing destinations at the page cap, some destinations may be missing")
	return combined, nil
}

// listTags returns the AWS tags on one delivery stream.
func listTags(ctx context.Context, api firehoseAPI, name string) (map[string]string, error) {
	tags := make(map[string]string)
	var startKey *string

	for page := 0; page < maxPages; page++ {
		out, err := api.ListTagsForDeliveryStream(ctx, &firehose.ListTagsForDeliveryStreamInput{
			DeliveryStreamName:   aws.String(name),
			ExclusiveStartTagKey: startKey,
		})
		if err != nil {
			return nil, fmt.Errorf("listing tags for delivery stream %s: %w", name, err)
		}

		for _, tag := range out.Tags {
			if tag.Key == nil {
				continue
			}
			tags[*tag.Key] = aws.ToString(tag.Value)
		}

		if len(out.Tags) == 0 || !aws.ToBool(out.HasMoreTags) {
			return tags, nil
		}

		startKey = out.Tags[len(out.Tags)-1].Key
		if startKey == nil {
			return tags, nil
		}
	}

	log.Warn().Str("stream", name).Int("pages", maxPages).
		Msg("Stopped listing tags at the page cap, some tags may be missing")
	return tags, nil
}
