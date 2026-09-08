package firehose

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every lineage edge is built from a name parsed out of an ARN, so a
// parser that returns the wrong segment points the edge at an asset that
// does not exist and the server drops it.

func TestBucketFromARN_ReturnsTheBucketName(t *testing.T) {
	assert.Equal(t, "marmot-lake", bucketFromARN("arn:aws:s3:::marmot-lake"))
}

func TestBucketFromARN_DropsAKeySuffix(t *testing.T) {
	assert.Equal(t, "marmot-lake", bucketFromARN("arn:aws:s3:::marmot-lake/orders/2026"))
}

func TestBucketFromARN_RejectsSomethingThatIsNotAnARN(t *testing.T) {
	assert.Equal(t, "", bucketFromARN("marmot-lake"))
}

func TestKinesisStreamFromARN_ReturnsTheStreamName(t *testing.T) {
	assert.Equal(t, "orders", kinesisStreamFromARN("arn:aws:kinesis:us-east-1:123456789012:stream/orders"))
}

func TestKinesisStreamFromARN_RejectsAnotherResourceKind(t *testing.T) {
	assert.Equal(t, "", kinesisStreamFromARN("arn:aws:kinesis:us-east-1:123456789012:consumer/orders"))
}

func TestMSKClusterFromARN_ReturnsTheClusterName(t *testing.T) {
	assert.Equal(t, "events-cluster", mskClusterFromARN("arn:aws:kafka:us-east-1:123456789012:cluster/events-cluster/1a2b3c4d-1"))
}

func TestArnRegion_ReturnsTheRegion(t *testing.T) {
	assert.Equal(t, "eu-west-1", arnRegion("arn:aws:firehose:eu-west-1:123456789012:deliverystream/logs"))
}

func TestArnRegion_IsEmptyForSomethingThatIsNotAnARN(t *testing.T) {
	assert.Equal(t, "", arnRegion("logs-to-opensearch"))
}

func TestParseRedshiftJDBCURL_SplitsHostAndDatabase(t *testing.T) {
	host, database := parseRedshiftJDBCURL("jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439/analytics")

	assert.Equal(t, "cluster.abc.us-east-1.redshift.amazonaws.com", host)
	assert.Equal(t, "analytics", database)
}

func TestParseRedshiftJDBCURL_DropsConnectionProperties(t *testing.T) {
	_, database := parseRedshiftJDBCURL("jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439/analytics?ssl=true")

	assert.Equal(t, "analytics", database)
}

func TestParseRedshiftJDBCURL_WithoutADatabase(t *testing.T) {
	host, database := parseRedshiftJDBCURL("jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439")

	assert.Equal(t, "cluster.abc.us-east-1.redshift.amazonaws.com", host)
	assert.Equal(t, "", database)
}

func TestParseRedshiftJDBCURL_RejectsGarbage(t *testing.T) {
	host, database := parseRedshiftJDBCURL("not a url")

	assert.Equal(t, "", host)
	assert.Equal(t, "", database)
}

func TestRedshiftTableName_QualifiesWithTheDatabase(t *testing.T) {
	name, ok := redshiftTableName("analytics", "public.clicks")

	require.True(t, ok)
	assert.Equal(t, "analytics.public.clicks", name)
}

func TestRedshiftTableName_KeepsAnAlreadyQualifiedName(t *testing.T) {
	name, ok := redshiftTableName("analytics", "warehouse.public.clicks")

	require.True(t, ok)
	assert.Equal(t, "warehouse.public.clicks", name)
}

// Redshift resolves a bare table name through the connection search path,
// which the API never shows, so there is no name to point an edge at.
func TestRedshiftTableName_RefusesABareTableName(t *testing.T) {
	_, ok := redshiftTableName("analytics", "clicks")

	assert.False(t, ok)
}

func TestRedshiftTableName_RefusesAnUnparseableJDBCURL(t *testing.T) {
	_, ok := redshiftTableName("", "public.clicks")

	assert.False(t, ok)
}

// moto, and some Firehose responses, fill both the plain and the extended
// S3 description for one extended destination. Reading the plain one would
// lose the format conversion and the Glue edge with it.
func TestClassifyDestination_PrefersExtendedS3OverPlainS3(t *testing.T) {
	dest, ok := classifyDestination(ordersToS3().Destinations[0])

	require.True(t, ok)
	assert.Equal(t, "extended_s3", dest.Kind)
}

func TestClassifyDestination_ExtendedS3RecordsTheBucketAndPrefix(t *testing.T) {
	dest, _ := classifyDestination(ordersToS3().Destinations[0])

	assert.Equal(t, "marmot-lake", dest.Settings["bucket"])
	assert.Equal(t, "orders/", dest.Settings["prefix"])
	assert.Equal(t, "errors/", dest.Settings["error_output_prefix"])
	assert.Equal(t, "GZIP", dest.Settings["compression_format"])
	assert.Equal(t, 64, dest.Settings["buffering_size_mb"])
	assert.Equal(t, 300, dest.Settings["buffering_interval_seconds"])
}

func TestClassifyDestination_ExtendedS3RecordsTheFormatConversion(t *testing.T) {
	dest, _ := classifyDestination(ordersToS3().Destinations[0])

	assert.Equal(t, true, dest.Settings["format_conversion_enabled"])
	assert.Equal(t, "OpenXJsonSerDe", dest.Settings["input_format"])
	assert.Equal(t, "ParquetSerDe", dest.Settings["output_format"])
	assert.Equal(t, "shop", dest.Settings["glue_database"])
	assert.Equal(t, "orders", dest.Settings["glue_table"])
	assert.Equal(t, "us-east-1", dest.Settings["glue_region"])
	assert.Equal(t, "123456789012", dest.Settings["glue_catalog_id"])
}

func TestClassifyDestination_ExtendedS3TargetsTheBucketAndTheGlueTable(t *testing.T) {
	dest, _ := classifyDestination(ordersToS3().Destinations[0])

	assert.Equal(t, []lineageTarget{
		{Type: "Bucket", Provider: "S3", Name: "marmot-lake"},
		{Type: "Table", Provider: "Glue", Name: "orders"},
	}, dest.Targets)
}

// Conversion that is configured but switched off converts nothing, so
// there is no Glue table shaping the output.
func TestClassifyDestination_DisabledFormatConversionMakesNoGlueEdge(t *testing.T) {
	description := ordersToS3()
	description.Destinations[0].ExtendedS3DestinationDescription.DataFormatConversionConfiguration.Enabled = aws.Bool(false)

	dest, _ := classifyDestination(description.Destinations[0])

	assert.Equal(t, []lineageTarget{{Type: "Bucket", Provider: "S3", Name: "marmot-lake"}}, dest.Targets)
	assert.NotContains(t, dest.Settings, "glue_table")
}

func TestClassifyDestination_PlainS3TargetsTheBucket(t *testing.T) {
	dest, ok := classifyDestination(eventsFromMSK().Destinations[0])

	require.True(t, ok)
	assert.Equal(t, "s3", dest.Kind)
	assert.Equal(t, []lineageTarget{{Type: "Bucket", Provider: "S3", Name: "marmot-lake"}}, dest.Targets)
}

func TestClassifyDestination_RedshiftTargetsTheQualifiedTable(t *testing.T) {
	dest, ok := classifyDestination(clicksToRedshift().Destinations[0])

	require.True(t, ok)
	assert.Equal(t, "redshift", dest.Kind)
	assert.Equal(t, []lineageTarget{{Type: "Table", Provider: "Redshift", Name: "analytics.public.clicks"}}, dest.Targets)
}

func TestClassifyDestination_RedshiftRecordsTheClusterAndCopyCommand(t *testing.T) {
	dest, _ := classifyDestination(clicksToRedshift().Destinations[0])

	assert.Equal(t, "cluster.abc.us-east-1.redshift.amazonaws.com", dest.Settings["cluster_endpoint"])
	assert.Equal(t, "analytics", dest.Settings["database"])
	assert.Equal(t, "public.clicks", dest.Settings["table"])
	assert.Equal(t, "json 'auto'", dest.Settings["copy_options"])
	assert.Equal(t, "admin", dest.Settings["username"])
}

func TestClassifyDestination_OpenSearchTargetsTheIndex(t *testing.T) {
	dest, ok := classifyDestination(logsToOpenSearch().Destinations[0])

	require.True(t, ok)
	assert.Equal(t, "opensearch", dest.Kind)
	assert.Equal(t, []lineageTarget{{Type: "Table", Provider: "OpenSearch", Name: "orders-index"}}, dest.Targets)
	assert.Equal(t, "arn:aws:es:eu-west-1:123456789012:domain/marmot-logs", dest.Settings["domain_arn"])
	assert.Equal(t, "OneDay", dest.Settings["index_rotation_period"])
}

func TestClassifyDestination_ElasticsearchTargetsTheIndex(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		ElasticsearchDestinationDescription: &types.ElasticsearchDestinationDescription{
			DomainARN: aws.String("arn:aws:es:us-east-1:123456789012:domain/marmot-logs"),
			IndexName: aws.String("logs"),
		},
	})

	require.True(t, ok)
	assert.Equal(t, "elasticsearch", dest.Kind)
	assert.Equal(t, []lineageTarget{{Type: "Table", Provider: "Elasticsearch", Name: "logs"}}, dest.Targets)
}

func TestClassifyDestination_SnowflakeTargetsTheThreePartTable(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		SnowflakeDestinationDescription: &types.SnowflakeDestinationDescription{
			AccountUrl: aws.String("https://acme.snowflakecomputing.com"),
			Database:   aws.String("ANALYTICS"),
			Schema:     aws.String("PUBLIC"),
			Table:      aws.String("EVENTS"),
			User:       aws.String("firehose"),
		},
	})

	require.True(t, ok)
	assert.Equal(t, "snowflake", dest.Kind)
	assert.Equal(t, []lineageTarget{{Type: "Table", Provider: "Snowflake", Name: "ANALYTICS.PUBLIC.EVENTS"}}, dest.Targets)
}

func TestClassifyDestination_SnowflakeWithoutASchemaMakesNoEdge(t *testing.T) {
	dest, _ := classifyDestination(types.DestinationDescription{
		SnowflakeDestinationDescription: &types.SnowflakeDestinationDescription{
			Database: aws.String("ANALYTICS"),
			Table:    aws.String("EVENTS"),
		},
	})

	assert.Empty(t, dest.Targets)
}

func TestClassifyDestination_IcebergTargetsEveryDestinationTable(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		IcebergDestinationDescription: &types.IcebergDestinationDescription{
			CatalogConfiguration: &types.CatalogConfiguration{
				CatalogARN: aws.String("arn:aws:glue:us-east-1:123456789012:catalog"),
			},
			DestinationTableConfigurationList: []types.DestinationTableConfiguration{
				{DestinationDatabaseName: aws.String("shop"), DestinationTableName: aws.String("orders")},
				{DestinationDatabaseName: aws.String("shop"), DestinationTableName: aws.String("returns")},
			},
		},
	})

	require.True(t, ok)
	assert.Equal(t, "iceberg", dest.Kind)
	assert.Equal(t, []lineageTarget{
		{Type: "Table", Provider: "Iceberg", Name: "orders"},
		{Type: "Table", Provider: "Iceberg", Name: "returns"},
	}, dest.Targets)
	assert.Equal(t, []string{"shop.orders", "shop.returns"}, dest.Settings["tables"])
	assert.Equal(t, "arn:aws:glue:us-east-1:123456789012:catalog", dest.Settings["catalog_arn"])
}

// Marmot has no asset for a Splunk index, so the endpoint is recorded and
// no edge is made. The HEC token is a credential and never recorded.
func TestClassifyDestination_SplunkRecordsTheEndpointButMakesNoEdge(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		SplunkDestinationDescription: &types.SplunkDestinationDescription{
			HECEndpoint:     aws.String("https://splunk.example.com:8088"),
			HECEndpointType: types.HECEndpointTypeEvent,
			HECToken:        aws.String("super-secret-token"),
		},
	})

	require.True(t, ok)
	assert.Equal(t, "splunk", dest.Kind)
	assert.Empty(t, dest.Targets)
	assert.Equal(t, "https://splunk.example.com:8088", dest.Settings["hec_endpoint"])
	assert.NotContains(t, dest.Settings, "hec_token")
}

func TestClassifyDestination_HTTPEndpointRecordsTheURLButMakesNoEdge(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		HttpEndpointDestinationDescription: &types.HttpEndpointDestinationDescription{
			EndpointConfiguration: &types.HttpEndpointDescription{
				Name: aws.String("datadog"),
				Url:  aws.String("https://aws-kinesis-http-intake.logs.datadoghq.com/v1/input"),
			},
		},
	})

	require.True(t, ok)
	assert.Equal(t, "http_endpoint", dest.Kind)
	assert.Empty(t, dest.Targets)
	assert.Equal(t, "datadog", dest.Settings["name"])
}

func TestClassifyDestination_OpenSearchServerlessRecordsTheCollectionButMakesNoEdge(t *testing.T) {
	dest, ok := classifyDestination(types.DestinationDescription{
		AmazonOpenSearchServerlessDestinationDescription: &types.AmazonOpenSearchServerlessDestinationDescription{
			CollectionEndpoint: aws.String("https://abc.us-east-1.aoss.amazonaws.com"),
			IndexName:          aws.String("logs"),
		},
	})

	require.True(t, ok)
	assert.Equal(t, "opensearch_serverless", dest.Kind)
	assert.Empty(t, dest.Targets)
}

func TestClassifyDestination_UnknownDestinationIsSkipped(t *testing.T) {
	_, ok := classifyDestination(types.DestinationDescription{DestinationId: aws.String("destinationId-000000000001")})

	assert.False(t, ok)
}

func TestMaskSecrets_BlanksCredentialLookingKeys(t *testing.T) {
	masked := maskSecrets(map[string]any{
		"bucket":     "marmot-lake",
		"password":   "hunter2",
		"hec_token":  "abc",
		"secret_arn": "arn:aws:secretsmanager:us-east-1:1:secret:x",
		"access_key": "AKIA",
	})

	assert.Equal(t, "marmot-lake", masked["bucket"])
	assert.Equal(t, redacted, masked["password"])
	assert.Equal(t, redacted, masked["hec_token"])
	assert.Equal(t, redacted, masked["secret_arn"])
	assert.Equal(t, redacted, masked["access_key"])
}

func TestMaskSecrets_LeavesTheNormalDestinationSettingsAlone(t *testing.T) {
	dest, _ := classifyDestination(ordersToS3().Destinations[0])

	masked := maskSecrets(dest.Settings)

	assert.Equal(t, "marmot-lake", masked["bucket"])
	assert.Equal(t, "orders", masked["glue_table"])
}

// A copy option string is free text and an operator can still put keys in
// it, so the whole option is dropped rather than picked apart.
func TestRedactCopyOptions_DropsInlineCredentials(t *testing.T) {
	assert.Equal(t, redacted, redactCopyOptions("CREDENTIALS 'aws_access_key_id=AKIA;aws_secret_access_key=x'"))
}

func TestRedactCopyOptions_KeepsAHarmlessOption(t *testing.T) {
	assert.Equal(t, "json 'auto' gzip", redactCopyOptions("json 'auto' gzip"))
}
