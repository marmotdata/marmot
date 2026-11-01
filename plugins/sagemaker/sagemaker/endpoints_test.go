package sagemaker

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverEndpoints_CatalogsTheEndpoint(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, []string{"SageMaker"}, endpoint.Providers)
	assert.Equal(t, "InService", endpoint.Metadata["status"])
}

func TestDiscoverEndpoints_RecordsTheEndpointConfigurationName(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "churn-cfg", endpoint.Metadata["endpoint_config"])
}

func TestDiscoverEndpoints_RecordsTheVariantsFromTheConfiguration(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")
	variants, ok := endpoint.Metadata["variants"].([]map[string]any)
	require.True(t, ok, "expected a variant list")
	require.Len(t, variants, 1)

	assert.Equal(t, "AllTraffic", variants[0]["name"])
	assert.Equal(t, "churn-xgb", variants[0]["model"])
	assert.Equal(t, "ml.m5.large", variants[0]["instance_type"])
	assert.Equal(t, int32(1), variants[0]["instance_count"])
	assert.Equal(t, float32(1), variants[0]["weight"])
}

func TestDiscoverEndpoints_RecordsTheDataCaptureLocation(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "s3://ml-capture/churn", endpoint.Metadata["data_capture_s3_uri"])
}

func TestDiscoverEndpoints_RecordsTheKMSKey(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "arn:aws:kms:us-east-1:123456789012:key/abc", endpoint.Metadata["kms_key_id"])
}

func TestDiscoverEndpoints_RecordsTheTimestamps(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "2026-09-01T10:00:00Z", endpoint.Metadata["created_at"])
	assert.Equal(t, "2026-09-01T10:00:00Z", endpoint.Metadata["last_modified_at"])
}

func TestDiscoverEndpoints_LinksToTheAWSConsole(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	require.Len(t, endpoint.ExternalLinks, 1)
	assert.Equal(t,
		"https://us-east-1.console.aws.amazon.com/sagemaker/home?region=us-east-1#/endpoints/churn-prod",
		endpoint.ExternalLinks[0].URL)
}

func TestDiscoverEndpoints_LinksTheModelToTheEndpointItServes(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://model/sagemaker/churn-xgb", "mrn://endpoint/sagemaker/churn-prod", "FEEDS"))
}

func TestDiscoverEndpoints_SkipsTheEdgeForAModelFromAnotherAccount(t *testing.T) {
	// A variant can name a model this run never saw, and the server drops
	// an edge whose endpoint has no asset behind it.
	fake := fullFake()
	fake.endpointConfigs["churn-cfg"].ProductionVariants[0].ModelName = aws.String("elsewhere")

	result := discoverWith(t, fake, nil)

	assert.False(t, hasEdge(result, "mrn://model/sagemaker/elsewhere", "mrn://endpoint/sagemaker/churn-prod", "FEEDS"))
}

func TestDiscoverEndpoints_KeepsTheEndpointWhenItsConfigurationIsGone(t *testing.T) {
	fake := fullFake()
	fake.errs = map[string]error{"DescribeEndpointConfig": errors.New("not found")}

	result := discoverWith(t, fake, nil)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")
	assert.NotContains(t, endpoint.Metadata, "variants")
}

func TestDiscoverEndpoints_SkipsAnEndpointItCannotDescribe(t *testing.T) {
	fake := fullFake()
	fake.endpoints = append(fake.endpoints, types.EndpointSummary{EndpointName: aws.String("ghost")})

	result := discoverWith(t, fake, nil)

	for _, a := range result.Assets {
		assert.NotEqual(t, "ghost", *a.Name)
	}
	assetNamed(t, result, "Endpoint", "churn-prod")
}

func TestDiscoverEndpoints_AreSkippedWhenTurnedOff(t *testing.T) {
	result := discoverWith(t, fullFake(), pluginsdk.RawConfig{"include_endpoints": false})

	for _, a := range result.Assets {
		assert.NotEqual(t, "Endpoint", a.Type)
	}
}

func TestDiscoverEndpoints_FailureKeepsTheRestOfTheRun(t *testing.T) {
	fake := fullFake()
	fake.errs = map[string]error{"ListEndpoints": errors.New("access denied")}

	result := discoverWith(t, fake, nil)

	assetNamed(t, result, "Model", "churn-xgb")
}

func TestVariantList_MarksAServerlessVariant(t *testing.T) {
	// A serverless variant has no instance type or count, so the flag is
	// what explains their absence.
	variants := variantList([]types.ProductionVariant{{
		VariantName:      aws.String("AllTraffic"),
		ModelName:        aws.String("churn-xgb"),
		ServerlessConfig: &types.ProductionVariantServerlessConfig{MemorySizeInMB: aws.Int32(2048)},
	}})

	require.Len(t, variants, 1)
	assert.Equal(t, true, variants[0]["serverless"])
	assert.NotContains(t, variants[0], "instance_type")
}
