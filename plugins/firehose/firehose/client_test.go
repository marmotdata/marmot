package firehose

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDeliveryStreams_ReturnsEveryStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())

	names, err := listDeliveryStreams(t.Context(), api)

	require.NoError(t, err)
	assert.Equal(t, []string{"clicks-to-redshift", "orders-to-s3"}, names)
}

// Firehose resumes a listing after the last name of the previous page, so
// every page has to be followed or streams go missing.
func TestListDeliveryStreams_FollowsEveryPage(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.add(clicksToRedshift())
	api.add(logsToOpenSearch())
	api.pageSize = 1

	names, err := listDeliveryStreams(t.Context(), api)

	require.NoError(t, err)
	assert.Equal(t, []string{"clicks-to-redshift", "logs-to-opensearch", "orders-to-s3"}, names)
}

// moto keeps reporting more streams even on the last page. Stopping only
// on that flag would ask for the same empty page forever.
func TestListDeliveryStreams_StopsOnAnEmptyPageEvenWhenTheAPISaysThereIsMore(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.pageSize = 1
	api.alwaysMore = true

	names, err := listDeliveryStreams(t.Context(), api)

	require.NoError(t, err)
	assert.Equal(t, []string{"orders-to-s3"}, names)
	assert.Equal(t, 2, api.listCalls, "one page of names then one empty page")
}

func TestListDeliveryStreams_EmptyAccount(t *testing.T) {
	names, err := listDeliveryStreams(t.Context(), newFakeFirehose())

	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestDescribeDeliveryStream_ReturnsTheDescription(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	description, err := describeDeliveryStream(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	assert.Equal(t, "orders-to-s3", aws.ToString(description.DeliveryStreamName))
	assert.Len(t, description.Destinations, 1)
}

// A stream with several destinations pages them, so a single call would
// see only the first and lose the rest of the lineage.
func TestDescribeDeliveryStream_FollowsTheDestinationPages(t *testing.T) {
	description := ordersToS3()
	description.Destinations = append(description.Destinations, types.DestinationDescription{
		DestinationId: aws.String("destinationId-000000000002"),
		S3DestinationDescription: &types.S3DestinationDescription{
			BucketARN: aws.String("arn:aws:s3:::marmot-archive"),
		},
	})

	api := newFakeFirehose()
	api.add(description)
	api.pageSize = 1

	combined, err := describeDeliveryStream(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	require.Len(t, combined.Destinations, 2)
	assert.Equal(t, "destinationId-000000000002", aws.ToString(combined.Destinations[1].DestinationId))
}

func TestDescribeDeliveryStream_ErrorsForAnUnknownStream(t *testing.T) {
	_, err := describeDeliveryStream(t.Context(), newFakeFirehose(), "gone")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gone")
}

func TestListTags_ReturnsTheTags(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String("team"), Value: aws.String("data")},
	}

	tags, err := listTags(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod", "team": "data"}, tags)
}

func TestListTags_FollowsEveryPage(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String("team"), Value: aws.String("data")},
	}
	api.pageSize = 1

	tags, err := listTags(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod", "team": "data"}, tags)
}

func TestListTags_StopsOnAnEmptyPageEvenWhenTheAPISaysThereIsMore(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())
	api.tags["orders-to-s3"] = []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}
	api.pageSize = 1
	api.alwaysMore = true

	tags, err := listTags(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod"}, tags)
}

func TestListTags_UntaggedStream(t *testing.T) {
	api := newFakeFirehose()
	api.add(ordersToS3())

	tags, err := listTags(t.Context(), api, "orders-to-s3")

	require.NoError(t, err)
	assert.Empty(t, tags)
}
