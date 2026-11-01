package firehose

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

// fakeFirehose is a stand-in for the Firehose API. The responses mirror
// what a real Firehose (and moto, which the e2e tests run against) returns,
// so the unit tests need neither AWS nor Docker.
type fakeFirehose struct {
	streams map[string]*types.DeliveryStreamDescription
	tags    map[string][]types.Tag

	// pageSize splits list responses when set, so tests can exercise
	// pagination. Zero returns everything in one page.
	pageSize int

	// alwaysMore makes every page claim there is more to come, which is
	// what moto does when a limit is given. Without a stop condition on
	// empty pages this would loop forever.
	alwaysMore bool

	// describeErrors names streams whose DescribeDeliveryStream call
	// fails, so tests can check discovery keeps going.
	describeErrors map[string]bool

	listCalls int
}

func newFakeFirehose() *fakeFirehose {
	return &fakeFirehose{
		streams:        map[string]*types.DeliveryStreamDescription{},
		tags:           map[string][]types.Tag{},
		describeErrors: map[string]bool{},
	}
}

func (f *fakeFirehose) add(d *types.DeliveryStreamDescription) {
	f.streams[aws.ToString(d.DeliveryStreamName)] = d
}

func (f *fakeFirehose) names() []string {
	names := make([]string, 0, len(f.streams))
	for name := range f.streams {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *fakeFirehose) ListDeliveryStreams(_ context.Context, in *firehose.ListDeliveryStreamsInput, _ ...func(*firehose.Options)) (*firehose.ListDeliveryStreamsOutput, error) {
	f.listCalls++

	names := f.names()
	if start := aws.ToString(in.ExclusiveStartDeliveryStreamName); start != "" {
		names = after(names, start)
	}

	hasMore := f.alwaysMore
	if f.pageSize > 0 && len(names) > f.pageSize {
		names = names[:f.pageSize]
		hasMore = true
	}

	return &firehose.ListDeliveryStreamsOutput{
		DeliveryStreamNames:    names,
		HasMoreDeliveryStreams: aws.Bool(hasMore),
	}, nil
}

func (f *fakeFirehose) DescribeDeliveryStream(_ context.Context, in *firehose.DescribeDeliveryStreamInput, _ ...func(*firehose.Options)) (*firehose.DescribeDeliveryStreamOutput, error) {
	name := aws.ToString(in.DeliveryStreamName)
	if f.describeErrors[name] {
		return nil, fmt.Errorf("stream %s is not available", name)
	}

	stream, ok := f.streams[name]
	if !ok {
		return nil, fmt.Errorf("stream %s not found", name)
	}

	page := *stream
	// A fresh slice per call, like a real decoded response, so the caller
	// appending pages cannot write back into the stored fixture.
	destinations := append([]types.DestinationDescription(nil), stream.Destinations...)
	if start := aws.ToString(in.ExclusiveStartDestinationId); start != "" {
		destinations = afterDestination(destinations, start)
	}

	hasMore := false
	if f.pageSize > 0 && len(destinations) > f.pageSize {
		destinations = destinations[:f.pageSize]
		hasMore = true
	}

	page.Destinations = destinations
	page.HasMoreDestinations = aws.Bool(hasMore)

	return &firehose.DescribeDeliveryStreamOutput{DeliveryStreamDescription: &page}, nil
}

func (f *fakeFirehose) ListTagsForDeliveryStream(_ context.Context, in *firehose.ListTagsForDeliveryStreamInput, _ ...func(*firehose.Options)) (*firehose.ListTagsForDeliveryStreamOutput, error) {
	name := aws.ToString(in.DeliveryStreamName)

	tags := f.tags[name]
	if start := aws.ToString(in.ExclusiveStartTagKey); start != "" {
		tags = afterTag(tags, start)
	}

	hasMore := f.alwaysMore
	if f.pageSize > 0 && len(tags) > f.pageSize {
		tags = tags[:f.pageSize]
		hasMore = true
	}

	return &firehose.ListTagsForDeliveryStreamOutput{
		Tags:        tags,
		HasMoreTags: aws.Bool(hasMore),
	}, nil
}

func after(names []string, start string) []string {
	for i, name := range names {
		if name == start {
			return names[i+1:]
		}
	}
	return nil
}

func afterDestination(destinations []types.DestinationDescription, start string) []types.DestinationDescription {
	for i, d := range destinations {
		if aws.ToString(d.DestinationId) == start {
			return destinations[i+1:]
		}
	}
	return nil
}

func afterTag(tags []types.Tag, start string) []types.Tag {
	for i, tag := range tags {
		if aws.ToString(tag.Key) == start {
			return tags[i+1:]
		}
	}
	return nil
}

// ordersToS3 is the extended S3 stream the e2e fixture also creates: a
// Kinesis source, a bucket destination and format conversion against a
// Glue table. A real Firehose fills only the extended field, moto fills
// both, so the fixture fills both too.
func ordersToS3() *types.DeliveryStreamDescription {
	return &types.DeliveryStreamDescription{
		DeliveryStreamName:   aws.String("orders-to-s3"),
		DeliveryStreamARN:    aws.String("arn:aws:firehose:us-east-1:123456789012:deliverystream/orders-to-s3"),
		DeliveryStreamStatus: types.DeliveryStreamStatusActive,
		DeliveryStreamType:   types.DeliveryStreamTypeKinesisStreamAsSource,
		VersionId:            aws.String("1"),
		Source: &types.SourceDescription{
			KinesisStreamSourceDescription: &types.KinesisStreamSourceDescription{
				KinesisStreamARN: aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/orders"),
				RoleARN:          aws.String("arn:aws:iam::123456789012:role/firehose"),
			},
		},
		Destinations: []types.DestinationDescription{{
			DestinationId: aws.String("destinationId-000000000001"),
			S3DestinationDescription: &types.S3DestinationDescription{
				BucketARN: aws.String("arn:aws:s3:::marmot-lake"),
				Prefix:    aws.String("orders/"),
			},
			ExtendedS3DestinationDescription: &types.ExtendedS3DestinationDescription{
				BucketARN:         aws.String("arn:aws:s3:::marmot-lake"),
				Prefix:            aws.String("orders/"),
				ErrorOutputPrefix: aws.String("errors/"),
				CompressionFormat: types.CompressionFormatGzip,
				BufferingHints:    &types.BufferingHints{SizeInMBs: aws.Int32(64), IntervalInSeconds: aws.Int32(300)},
				DataFormatConversionConfiguration: &types.DataFormatConversionConfiguration{
					Enabled: aws.Bool(true),
					SchemaConfiguration: &types.SchemaConfiguration{
						CatalogId:    aws.String("123456789012"),
						DatabaseName: aws.String("shop"),
						TableName:    aws.String("orders"),
						Region:       aws.String("us-east-1"),
					},
					InputFormatConfiguration:  &types.InputFormatConfiguration{Deserializer: &types.Deserializer{OpenXJsonSerDe: &types.OpenXJsonSerDe{}}},
					OutputFormatConfiguration: &types.OutputFormatConfiguration{Serializer: &types.Serializer{ParquetSerDe: &types.ParquetSerDe{}}},
				},
			},
		}},
	}
}

func clicksToRedshift() *types.DeliveryStreamDescription {
	return &types.DeliveryStreamDescription{
		DeliveryStreamName:   aws.String("clicks-to-redshift"),
		DeliveryStreamARN:    aws.String("arn:aws:firehose:us-east-1:123456789012:deliverystream/clicks-to-redshift"),
		DeliveryStreamStatus: types.DeliveryStreamStatusActive,
		DeliveryStreamType:   types.DeliveryStreamTypeDirectPut,
		VersionId:            aws.String("1"),
		Destinations: []types.DestinationDescription{{
			DestinationId: aws.String("destinationId-000000000001"),
			RedshiftDestinationDescription: &types.RedshiftDestinationDescription{
				ClusterJDBCURL: aws.String("jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439/analytics"),
				CopyCommand:    &types.CopyCommand{DataTableName: aws.String("public.clicks"), CopyOptions: aws.String("json 'auto'")},
				Username:       aws.String("admin"),
			},
		}},
	}
}

func logsToOpenSearch() *types.DeliveryStreamDescription {
	return &types.DeliveryStreamDescription{
		DeliveryStreamName:   aws.String("logs-to-opensearch"),
		DeliveryStreamARN:    aws.String("arn:aws:firehose:eu-west-1:123456789012:deliverystream/logs-to-opensearch"),
		DeliveryStreamStatus: types.DeliveryStreamStatusActive,
		DeliveryStreamType:   types.DeliveryStreamTypeDirectPut,
		VersionId:            aws.String("2"),
		Destinations: []types.DestinationDescription{{
			DestinationId: aws.String("destinationId-000000000001"),
			AmazonopensearchserviceDestinationDescription: &types.AmazonopensearchserviceDestinationDescription{
				DomainARN:           aws.String("arn:aws:es:eu-west-1:123456789012:domain/marmot-logs"),
				IndexName:           aws.String("orders-index"),
				IndexRotationPeriod: types.AmazonopensearchserviceIndexRotationPeriodOneDay,
			},
		}},
	}
}

func eventsFromMSK() *types.DeliveryStreamDescription {
	return &types.DeliveryStreamDescription{
		DeliveryStreamName:   aws.String("events-from-msk"),
		DeliveryStreamARN:    aws.String("arn:aws:firehose:us-east-1:123456789012:deliverystream/events-from-msk"),
		DeliveryStreamStatus: types.DeliveryStreamStatusActive,
		DeliveryStreamType:   types.DeliveryStreamTypeMSKAsSource,
		VersionId:            aws.String("1"),
		Source: &types.SourceDescription{
			MSKSourceDescription: &types.MSKSourceDescription{
				MSKClusterARN: aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/events-cluster/1a2b3c4d-1"),
				TopicName:     aws.String("events"),
			},
		},
		Destinations: []types.DestinationDescription{{
			DestinationId: aws.String("destinationId-000000000001"),
			S3DestinationDescription: &types.S3DestinationDescription{
				BucketARN:         aws.String("arn:aws:s3:::marmot-lake"),
				CompressionFormat: types.CompressionFormatUncompressed,
			},
		}},
	}
}
