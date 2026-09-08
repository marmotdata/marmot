package firehose_test

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	fhtypes "github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real Firehose API. Set
// MARMOT_TEST_FIREHOSE_ENDPOINT to run them, for example:
//
//	docker run -d --name marmot-test-firehose -p 15559:5000 motoserver/moto:latest
//	MARMOT_TEST_FIREHOSE_ENDPOINT=http://localhost:15559 go test ./firehose/

const (
	e2eRegion = "us-east-1"
	roleARN   = "arn:aws:iam::123456789012:role/firehose"
	bucket    = "marmot-lake"
	bucketARN = "arn:aws:s3:::" + bucket
)

func endpoint(t *testing.T) string {
	t.Helper()

	url := os.Getenv("MARMOT_TEST_FIREHOSE_ENDPOINT")
	if url == "" {
		t.Skip("set MARMOT_TEST_FIREHOSE_ENDPOINT (for example http://localhost:15559) to run the Firehose e2e tests")
	}
	return url
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

// e2eConfig is the ingest config the plugin under test receives.
func e2eConfig(url string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"credentials": map[string]any{
			"region":      e2eRegion,
			"id":          "test",
			"secret":      "test",
			"use_default": false,
			"endpoint":    url,
		},
		"tags_to_metadata": true,
	}
}

func awsConfig(t *testing.T, url string) aws.Config {
	t.Helper()

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(e2eRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)
	cfg.BaseEndpoint = aws.String(url)

	return cfg
}

// seed builds the fixture the assertions below expect. Creating anything
// that already exists is not an error worth failing on, the delivery
// streams are recreated from scratch so a rerun sees the same shape.
func seed(t *testing.T, url string) {
	t.Helper()

	ctx := context.Background()
	cfg := awsConfig(t, url)

	// A real Kinesis stream, so the source ARN the plugin parses is the
	// one the service itself produces rather than a handwritten string.
	kinesisClient := kinesis.NewFromConfig(cfg)
	_, err := kinesisClient.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String("orders"),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Logf("creating the kinesis stream: %v (continuing, it may already exist)", err)
	}

	summary, err := kinesisClient.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{
		StreamName: aws.String("orders"),
	})
	require.NoError(t, err, "the kinesis source stream must exist")
	kinesisARN := aws.ToString(summary.StreamDescriptionSummary.StreamARN)

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Logf("creating the s3 bucket: %v (continuing, it may already exist)", err)
	}

	firehoseClient := firehose.NewFromConfig(cfg)
	backup := &fhtypes.S3DestinationConfiguration{
		RoleARN:           aws.String(roleARN),
		BucketARN:         aws.String(bucketARN),
		Prefix:            aws.String("backup/"),
		CompressionFormat: fhtypes.CompressionFormatUncompressed,
		BufferingHints:    &fhtypes.BufferingHints{SizeInMBs: aws.Int32(5), IntervalInSeconds: aws.Int32(300)},
	}

	streams := []*firehose.CreateDeliveryStreamInput{
		{
			DeliveryStreamName: aws.String("orders-to-s3"),
			DeliveryStreamType: fhtypes.DeliveryStreamTypeKinesisStreamAsSource,
			KinesisStreamSourceConfiguration: &fhtypes.KinesisStreamSourceConfiguration{
				KinesisStreamARN: aws.String(kinesisARN),
				RoleARN:          aws.String(roleARN),
			},
			ExtendedS3DestinationConfiguration: &fhtypes.ExtendedS3DestinationConfiguration{
				RoleARN:           aws.String(roleARN),
				BucketARN:         aws.String(bucketARN),
				Prefix:            aws.String("orders/"),
				ErrorOutputPrefix: aws.String("errors/"),
				CompressionFormat: fhtypes.CompressionFormatGzip,
				BufferingHints:    &fhtypes.BufferingHints{SizeInMBs: aws.Int32(64), IntervalInSeconds: aws.Int32(300)},
				DataFormatConversionConfiguration: &fhtypes.DataFormatConversionConfiguration{
					Enabled: aws.Bool(true),
					SchemaConfiguration: &fhtypes.SchemaConfiguration{
						CatalogId:    aws.String("123456789012"),
						DatabaseName: aws.String("shop"),
						TableName:    aws.String("orders"),
						Region:       aws.String(e2eRegion),
						RoleARN:      aws.String(roleARN),
					},
					InputFormatConfiguration:  &fhtypes.InputFormatConfiguration{Deserializer: &fhtypes.Deserializer{OpenXJsonSerDe: &fhtypes.OpenXJsonSerDe{}}},
					OutputFormatConfiguration: &fhtypes.OutputFormatConfiguration{Serializer: &fhtypes.Serializer{ParquetSerDe: &fhtypes.ParquetSerDe{}}},
				},
			},
		},
		{
			DeliveryStreamName: aws.String("clicks-to-redshift"),
			DeliveryStreamType: fhtypes.DeliveryStreamTypeDirectPut,
			RedshiftDestinationConfiguration: &fhtypes.RedshiftDestinationConfiguration{
				RoleARN:         aws.String(roleARN),
				ClusterJDBCURL:  aws.String("jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439/analytics"),
				CopyCommand:     &fhtypes.CopyCommand{DataTableName: aws.String("public.clicks"), CopyOptions: aws.String("json 'auto'")},
				Username:        aws.String("admin"),
				Password:        aws.String("hunter2hunter2"),
				S3Configuration: backup,
			},
		},
		{
			// moto rejects AmazonopensearchserviceDestinationConfiguration
			// with "Exactly one destination configuration is supported for
			// a Firehose", so the fixture uses the Elasticsearch shape,
			// which it does accept. The OpenSearch path is covered by the
			// unit tests.
			DeliveryStreamName: aws.String("logs-to-opensearch"),
			DeliveryStreamType: fhtypes.DeliveryStreamTypeDirectPut,
			ElasticsearchDestinationConfiguration: &fhtypes.ElasticsearchDestinationConfiguration{
				RoleARN:             aws.String(roleARN),
				DomainARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/marmot-logs"),
				IndexName:           aws.String("logs"),
				IndexRotationPeriod: fhtypes.ElasticsearchIndexRotationPeriodOneDay,
				S3Configuration:     backup,
			},
		},
	}

	for _, stream := range streams {
		name := aws.ToString(stream.DeliveryStreamName)
		if _, err := firehoseClient.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
			DeliveryStreamName: aws.String(name),
		}); err != nil {
			t.Logf("deleting %s before recreating it: %v (continuing, it may not exist yet)", name, err)
		}

		_, err := firehoseClient.CreateDeliveryStream(ctx, stream)
		require.NoErrorf(t, err, "creating delivery stream %s", name)
	}

	_, err = firehoseClient.TagDeliveryStream(ctx, &firehose.TagDeliveryStreamInput{
		DeliveryStreamName: aws.String("orders-to-s3"),
		Tags: []fhtypes.Tag{
			{Key: aws.String("env"), Value: aws.String("prod")},
			{Key: aws.String("team"), Value: aws.String("data")},
		},
	})
	require.NoError(t, err, "tagging orders-to-s3")
}

func discover(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	url := endpoint(t)
	seed(t, url)

	result, err := buildBinary(t).Discover(t.Context(), e2eConfig(url))
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

func e2eAsset(t *testing.T, result *pluginsdk.DiscoveryResult, name string) pluginsdk.Asset {
	t.Helper()

	for _, asset := range result.Assets {
		if aws.ToString(asset.Name) == name {
			return asset
		}
	}

	t.Fatalf("no asset named %s", name)
	return pluginsdk.Asset{}
}

func TestE2E_Meta(t *testing.T) {
	endpoint(t)
	bin := buildBinary(t)

	meta, err := bin.Meta(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "firehose", meta.ID)
	assert.Equal(t, "Firehose", meta.Name)
	assert.Equal(t, "messaging", meta.Category)
	assert.Equal(t, "firehose", meta.Icon)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
}

// The plugin has no required field: with no credentials it falls back to
// the ambient AWS environment. A malformed endpoint is the one thing
// Validate can reject outright.
func TestE2E_ValidateRejectsAMalformedEndpoint(t *testing.T) {
	endpoint(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), pluginsdk.RawConfig{
		"credentials": map[string]any{"endpoint": "not a url"},
	})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsTheTestConfig(t *testing.T) {
	url := endpoint(t)
	bin := buildBinary(t)

	_, err := bin.Validate(t.Context(), e2eConfig(url))

	require.NoError(t, err)
}

func TestE2E_DiscoversEveryDeliveryStream(t *testing.T) {
	result := discover(t)

	names := make(map[string]string) // name -> type
	for _, asset := range result.Assets {
		require.NotNil(t, asset.Name)
		names[*asset.Name] = asset.Type
	}

	assert.Equal(t, "DeliveryStream", names["orders-to-s3"])
	assert.Equal(t, "DeliveryStream", names["clicks-to-redshift"])
	assert.Equal(t, "DeliveryStream", names["logs-to-opensearch"])
}

func TestE2E_RecordsTheKinesisSourcedStream(t *testing.T) {
	metadata := e2eAsset(t, discover(t), "orders-to-s3").Metadata

	assert.Equal(t, "arn:aws:firehose:us-east-1:123456789012:deliverystream/orders-to-s3", metadata["arn"])
	assert.Equal(t, "ACTIVE", metadata["status"])
	assert.Equal(t, "KinesisStreamAsSource", metadata["stream_type"])
	assert.Equal(t, "kinesis", metadata["source_type"])
	assert.Equal(t, "orders", metadata["source_kinesis_stream"])
	assert.Equal(t, "extended_s3", metadata["destination_type"])
	assert.Equal(t, "us-east-1", metadata["region"])
}

func TestE2E_RecordsTheDestinationSettings(t *testing.T) {
	metadata := e2eAsset(t, discover(t), "orders-to-s3").Metadata

	settings, ok := metadata["destination"].(map[string]any)
	require.True(t, ok, "destination should be a sub-map, got %T", metadata["destination"])
	assert.Equal(t, "marmot-lake", settings["bucket"])
	assert.Equal(t, "orders/", settings["prefix"])
	assert.Equal(t, "GZIP", settings["compression_format"])
	assert.Equal(t, "shop", settings["glue_database"])
	assert.Equal(t, "orders", settings["glue_table"])
	assert.Equal(t, "ParquetSerDe", settings["output_format"])
}

func TestE2E_RecordsTheRedshiftDestination(t *testing.T) {
	metadata := e2eAsset(t, discover(t), "clicks-to-redshift").Metadata

	assert.Equal(t, "direct_put", metadata["source_type"])
	assert.Equal(t, "redshift", metadata["destination_type"])

	settings, ok := metadata["destination"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "cluster.abc.us-east-1.redshift.amazonaws.com", settings["cluster_endpoint"])
	assert.Equal(t, "analytics", settings["database"])
	assert.Equal(t, "public.clicks", settings["table"])
}

// The Firehose API never echoes the Redshift password back, and nothing in
// the plugin puts it in the catalog either.
func TestE2E_NeverRecordsTheRedshiftPassword(t *testing.T) {
	metadata := e2eAsset(t, discover(t), "clicks-to-redshift").Metadata

	settings, ok := metadata["destination"].(map[string]any)
	require.True(t, ok)
	for key, value := range settings {
		assert.NotEqual(t, "hunter2hunter2", value, "the password leaked into %s", key)
	}
}

func TestE2E_TurnsAWSTagsIntoMetadata(t *testing.T) {
	metadata := e2eAsset(t, discover(t), "orders-to-s3").Metadata

	assert.Equal(t, "prod", metadata["tag_env"])
	assert.Equal(t, "data", metadata["tag_team"])
}

func TestE2E_LinksTheAWSConsole(t *testing.T) {
	links := e2eAsset(t, discover(t), "orders-to-s3").ExternalLinks

	require.Len(t, links, 1)
	assert.Equal(t, "Open in AWS Console", links[0].Name)
	assert.Equal(t, "https://us-east-1.console.aws.amazon.com/firehose/home?region=us-east-1#/details/orders-to-s3", links[0].URL)
}

func TestE2E_EveryAssetMRNMatchesItsIdentity(t *testing.T) {
	result := discover(t)

	for _, asset := range result.Assets {
		require.NotNil(t, asset.MRN)
		assert.Equal(t, "mrn://deliverystream/firehose/"+aws.ToString(asset.Name), *asset.MRN)
	}
}

func TestE2E_KinesisStreamFeedsTheDeliveryStream(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://stream/kinesis/orders",
		Target: "mrn://deliverystream/firehose/orders-to-s3",
		Type:   "FEEDS",
	})
}

func TestE2E_DeliveryStreamProducesIntoTheBucket(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/orders-to-s3",
		Target: "mrn://bucket/s3/marmot-lake",
		Type:   "PRODUCES",
	})
}

func TestE2E_FormatConversionProducesIntoTheGlueTable(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/orders-to-s3",
		Target: "mrn://table/glue/orders",
		Type:   "PRODUCES",
	})
}

func TestE2E_DeliveryStreamProducesIntoTheRedshiftTable(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/clicks-to-redshift",
		Target: "mrn://table/redshift/analytics.public.clicks",
		Type:   "PRODUCES",
	})
}

// moto only accepts the Elasticsearch destination shape, so the index edge
// comes out under the Elasticsearch provider here. Against a real AWS
// account the same stream would be an OpenSearch destination.
func TestE2E_DeliveryStreamProducesIntoTheSearchIndex(t *testing.T) {
	result := discover(t)

	assert.Contains(t, result.Lineage, pluginsdk.LineageEdge{
		Source: "mrn://deliverystream/firehose/logs-to-opensearch",
		Target: "mrn://table/elasticsearch/logs",
		Type:   "PRODUCES",
	})
}

func TestE2E_EmitsNoStatistics(t *testing.T) {
	assert.Empty(t, discover(t).Statistics)
}
