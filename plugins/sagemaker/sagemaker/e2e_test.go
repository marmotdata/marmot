package sagemaker_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the compiled plugin binary over the same gRPC wire
// protocol the Marmot host uses, against a real SageMaker API. Point
// MARMOT_TEST_SAGEMAKER_ENDPOINT at one, for example a moto server:
//
//	docker run -d --name marmot-test-sagemaker -p 15556:5000 motoserver/moto:latest
//	MARMOT_TEST_SAGEMAKER_ENDPOINT=http://localhost:15556 go test ./...
const endpointEnv = "MARMOT_TEST_SAGEMAKER_ENDPOINT"

func requireEndpoint(t *testing.T) string {
	t.Helper()

	endpoint := os.Getenv(endpointEnv)
	if endpoint == "" {
		t.Skipf("set %s to a SageMaker API endpoint to run the end to end tests", endpointEnv)
	}

	return endpoint
}

func buildBinary(t *testing.T) plugintest.Binary {
	t.Helper()
	// ".." is the plugin main package, one level up from this subpackage.
	return plugintest.Build(t, "..")
}

func testConfig(endpoint string) pluginsdk.RawConfig {
	return pluginsdk.RawConfig{
		"credentials": map[string]any{
			"region":   "us-east-1",
			"id":       "test",
			"secret":   "test",
			"endpoint": endpoint,
		},
		"include_training_jobs": true,
	}
}

// trainedModel is a second model whose artifact is the one the seeded
// training job wrote, which is how the plugin connects a job to a model.
const trainedModel = "churn-xgb-trained"

var (
	seedOnce   sync.Once
	seedResult *pluginsdk.DiscoveryResult
	seedErr    error
)

// discoverSeeded seeds the SageMaker API once and returns the plugin's
// discovery of it. Seeding is expensive, and every test looks at the same
// account.
func discoverSeeded(t *testing.T) *pluginsdk.DiscoveryResult {
	t.Helper()

	endpoint := requireEndpoint(t)
	bin := buildBinary(t)

	seedOnce.Do(func() {
		seed(t, endpoint)
		seedResult, seedErr = bin.Discover(context.Background(), testConfig(endpoint))
	})

	require.NoError(t, seedErr)
	require.NotNil(t, seedResult)

	return seedResult
}

// seed creates the account the tests assert on. Only a feature group
// refuses to be created twice, so re-running the suite against the same
// container is safe.
func seed(t *testing.T, endpoint string) {
	t.Helper()

	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)
	cfg.BaseEndpoint = aws.String(endpoint)

	client := sagemaker.NewFromConfig(cfg)

	_, err = client.CreateTrainingJob(ctx, &sagemaker.CreateTrainingJobInput{
		TrainingJobName: aws.String("churn-train-1"),
		AlgorithmSpecification: &types.AlgorithmSpecification{
			TrainingImage:     aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
			TrainingInputMode: types.TrainingInputModeFile,
		},
		HyperParameters: map[string]string{"max_depth": "5", "eta": "0.2"},
		InputDataConfig: []types.Channel{{
			ChannelName: aws.String("train"),
			DataSource: &types.DataSource{S3DataSource: &types.S3DataSource{
				S3DataType: types.S3DataTypeS3Prefix,
				S3Uri:      aws.String("s3://ml-data/train"),
			}},
		}},
		OutputDataConfig: &types.OutputDataConfig{S3OutputPath: aws.String("s3://ml-artifacts/churn")},
		ResourceConfig: &types.ResourceConfig{
			InstanceType:   types.TrainingInstanceTypeMlM5Large,
			InstanceCount:  aws.Int32(1),
			VolumeSizeInGB: aws.Int32(10),
		},
		RoleArn:           aws.String("arn:aws:iam::123456789012:role/SageMakerRole"),
		StoppingCondition: &types.StoppingCondition{MaxRuntimeInSeconds: aws.Int32(3600)},
		Tags:              []types.Tag{{Key: aws.String("team"), Value: aws.String("growth")}},
	})
	require.NoError(t, err)

	job, err := client.DescribeTrainingJob(ctx, &sagemaker.DescribeTrainingJobInput{
		TrainingJobName: aws.String("churn-train-1"),
	})
	require.NoError(t, err)
	require.NotNil(t, job.ModelArtifacts)

	_, err = client.CreateModel(ctx, &sagemaker.CreateModelInput{
		ModelName: aws.String("churn-xgb"),
		PrimaryContainer: &types.ContainerDefinition{
			Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
			ModelDataUrl: aws.String("s3://ml-artifacts/churn/model.tar.gz"),
			Environment:  map[string]string{"LOG_LEVEL": "info", "API_SECRET": "hunter2"},
		},
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/SageMakerRole"),
		Tags:             []types.Tag{{Key: aws.String("team"), Value: aws.String("growth")}},
	})
	require.NoError(t, err)

	_, err = client.CreateModel(ctx, &sagemaker.CreateModelInput{
		ModelName: aws.String(trainedModel),
		PrimaryContainer: &types.ContainerDefinition{
			Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
			ModelDataUrl: job.ModelArtifacts.S3ModelArtifacts,
		},
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/SageMakerRole"),
	})
	require.NoError(t, err)

	_, err = client.CreateEndpointConfig(ctx, &sagemaker.CreateEndpointConfigInput{
		EndpointConfigName: aws.String("churn-cfg"),
		ProductionVariants: []types.ProductionVariant{{
			VariantName:          aws.String("AllTraffic"),
			ModelName:            aws.String("churn-xgb"),
			InstanceType:         types.ProductionVariantInstanceTypeMlM5Large,
			InitialInstanceCount: aws.Int32(1),
			InitialVariantWeight: aws.Float32(1),
		}},
		DataCaptureConfig: &types.DataCaptureConfig{
			InitialSamplingPercentage: aws.Int32(100),
			DestinationS3Uri:          aws.String("s3://ml-capture/churn"),
			CaptureOptions:            []types.CaptureOption{{CaptureMode: types.CaptureModeInput}},
		},
		KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc"),
	})
	require.NoError(t, err)

	_, err = client.CreateEndpoint(ctx, &sagemaker.CreateEndpointInput{
		EndpointName:       aws.String("churn-prod"),
		EndpointConfigName: aws.String("churn-cfg"),
		Tags:               []types.Tag{{Key: aws.String("env"), Value: aws.String("prod")}},
	})
	require.NoError(t, err)

	_, err = client.CreateModelPackageGroup(ctx, &sagemaker.CreateModelPackageGroupInput{
		ModelPackageGroupName:        aws.String("churn-registry"),
		ModelPackageGroupDescription: aws.String("Churn model registry"),
		Tags:                         []types.Tag{{Key: aws.String("team"), Value: aws.String("growth")}},
	})
	require.NoError(t, err)

	_, err = client.CreateModelPackage(ctx, &sagemaker.CreateModelPackageInput{
		ModelPackageGroupName: aws.String("churn-registry"),
		ModelApprovalStatus:   types.ModelApprovalStatusApproved,
		Domain:                aws.String("MACHINE_LEARNING"),
		Task:                  aws.String("CLASSIFICATION"),
		SamplePayloadUrl:      aws.String("s3://ml-artifacts/churn/sample.json"),
		InferenceSpecification: &types.InferenceSpecification{
			Containers: []types.ModelPackageContainerDefinition{{
				Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
				ModelDataUrl: aws.String("s3://ml-artifacts/churn/model.tar.gz"),
			}},
			SupportedContentTypes:      []string{"text/csv"},
			SupportedResponseMIMETypes: []string{"text/csv"},
		},
	})
	require.NoError(t, err)

	_, err = client.CreateFeatureGroup(ctx, &sagemaker.CreateFeatureGroupInput{
		FeatureGroupName:            aws.String("customer-features"),
		RecordIdentifierFeatureName: aws.String("age"),
		EventTimeFeatureName:        aws.String("event_time"),
		Description:                 aws.String("Customer features for churn"),
		FeatureDefinitions: []types.FeatureDefinition{
			{FeatureName: aws.String("age"), FeatureType: types.FeatureTypeIntegral},
			{FeatureName: aws.String("plan"), FeatureType: types.FeatureTypeString},
			{FeatureName: aws.String("event_time"), FeatureType: types.FeatureTypeFractional},
		},
		OnlineStoreConfig: &types.OnlineStoreConfig{EnableOnlineStore: aws.Bool(true)},
		OfflineStoreConfig: &types.OfflineStoreConfig{
			S3StorageConfig: &types.S3StorageConfig{S3Uri: aws.String("s3://ml-features/customer-features")},
		},
		RoleArn: aws.String("arn:aws:iam::123456789012:role/SageMakerRole"),
	})
	if err != nil && !strings.Contains(err.Error(), "ResourceInUse") {
		require.NoError(t, err)
	}
}

func assetNamed(t *testing.T, result *pluginsdk.DiscoveryResult, assetType, name string) pluginsdk.Asset {
	t.Helper()

	for _, a := range result.Assets {
		if a.Type == assetType && a.Name != nil && *a.Name == name {
			return a
		}
	}

	t.Fatalf("no %s asset named %q in %d discovered assets", assetType, name, len(result.Assets))
	return pluginsdk.Asset{}
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}

func TestE2E_Meta(t *testing.T) {
	bin := buildBinary(t)

	meta, err := bin.Meta(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "sagemaker", meta.ID)
	assert.Equal(t, "SageMaker", meta.Name)
	assert.Equal(t, "ml", meta.Category)
	assert.Contains(t, meta.Features, "Assets")
	assert.Contains(t, meta.Features, "Lineage")
	assert.Contains(t, meta.Features, "Run History")
}

func TestE2E_ValidateRejectsAnEndpointThatIsNotAURL(t *testing.T) {
	// Nothing in this plugin's config is required, so an invalid value is
	// what a rejected config looks like.
	bin := buildBinary(t)

	_, err := bin.Validate(context.Background(), pluginsdk.RawConfig{
		"credentials": map[string]any{"endpoint": "not a url"},
	})

	require.Error(t, err)
}

func TestE2E_ValidateAcceptsAnEmptyConfig(t *testing.T) {
	bin := buildBinary(t)

	_, err := bin.Validate(context.Background(), pluginsdk.RawConfig{})

	require.NoError(t, err)
}

func TestE2E_DiscoversTheModel(t *testing.T) {
	result := discoverSeeded(t)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, []string{"SageMaker"}, model.Providers)
	assert.Equal(t, "mrn://model/sagemaker/churn-xgb", *model.MRN)
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", model.Metadata["image"])
	assert.Equal(t, "s3://ml-artifacts/churn/model.tar.gz", model.Metadata["model_data_url"])
}

func TestE2E_CarriesTheModelsAWSTags(t *testing.T) {
	result := discoverSeeded(t)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "growth", model.Metadata["tag_team"])
}

func TestE2E_MasksTheModelsSecretEnvironmentValues(t *testing.T) {
	result := discoverSeeded(t)

	model := assetNamed(t, result, "Model", "churn-xgb")
	env, ok := model.Metadata["environment"].(map[string]any)
	require.True(t, ok, "expected an environment map")

	assert.Equal(t, "info", env["LOG_LEVEL"])
	assert.Equal(t, "***", env["API_SECRET"])
}

func TestE2E_DiscoversTheEndpointAndItsVariant(t *testing.T) {
	result := discoverSeeded(t)

	endpoint := assetNamed(t, result, "Endpoint", "churn-prod")

	assert.Equal(t, "mrn://endpoint/sagemaker/churn-prod", *endpoint.MRN)
	assert.Equal(t, "InService", endpoint.Metadata["status"])
	assert.Equal(t, "churn-cfg", endpoint.Metadata["endpoint_config"])
	assert.Equal(t, "s3://ml-capture/churn", endpoint.Metadata["data_capture_s3_uri"])

	variants, ok := endpoint.Metadata["variants"].([]any)
	require.True(t, ok, "expected a variant list")
	require.Len(t, variants, 1)

	variant, ok := variants[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "AllTraffic", variant["name"])
	assert.Equal(t, "churn-xgb", variant["model"])
	assert.Equal(t, "ml.m5.large", variant["instance_type"])
}

func TestE2E_LinksTheModelToTheEndpoint(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result, "mrn://model/sagemaker/churn-xgb", "mrn://endpoint/sagemaker/churn-prod", "FEEDS"))
}

func TestE2E_LinksTheArtifactBucketToTheModel(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-artifacts", "mrn://model/sagemaker/churn-xgb", "FEEDS"))
}

func TestE2E_DiscoversTheModelPackageGroup(t *testing.T) {
	result := discoverSeeded(t)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, "model_package_group", group.Metadata["kind"])
	require.NotNil(t, group.Description)
	assert.Equal(t, "Churn model registry", *group.Description)
	assert.Equal(t, "CLASSIFICATION", group.Metadata["task"])
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", group.Metadata["latest_image"])
}

func TestE2E_DiscoversTheTrainingJob(t *testing.T) {
	result := discoverSeeded(t)

	job := assetNamed(t, result, "Job", "churn-train-1")

	assert.Equal(t, "mrn://job/sagemaker/churn-train-1", *job.MRN)
	assert.Equal(t, "Completed", job.Metadata["status"])
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", job.Metadata["algorithm_image"])
	assert.Equal(t, "s3://ml-artifacts/churn", job.Metadata["output_s3_path"])

	channels, ok := job.Metadata["input_channels"].(map[string]any)
	require.True(t, ok, "expected an input channel map")
	assert.Equal(t, "s3://ml-data/train", channels["train"])
}

func TestE2E_LinksTheTrainingDataBucketToTheJob(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-data", "mrn://job/sagemaker/churn-train-1", "FEEDS"))
}

func TestE2E_LinksTheTrainingJobToTheModelItProduced(t *testing.T) {
	result := discoverSeeded(t)

	assert.True(t, hasEdge(result,
		"mrn://job/sagemaker/churn-train-1",
		"mrn://model/sagemaker/"+trainedModel,
		"PRODUCES"))
}

func TestE2E_EmitsRunHistoryForTheTrainingJob(t *testing.T) {
	result := discoverSeeded(t)

	var history *pluginsdk.AssetRunHistory
	for i := range result.RunHistory {
		if result.RunHistory[i].AssetMRN == "mrn://job/sagemaker/churn-train-1" {
			history = &result.RunHistory[i]
		}
	}
	require.NotNil(t, history, "expected run history for the training job")

	require.Len(t, history.Runs, 2)
	assert.Equal(t, "START", history.Runs[0].EventType)
	assert.Equal(t, "COMPLETE", history.Runs[1].EventType)
	assert.Equal(t, "churn-train-1", history.Runs[0].RunID)
	assert.Equal(t, "sagemaker", history.Runs[0].JobNamespace)
}

func TestE2E_SurvivesAnAPIWithoutListFeatureGroups(t *testing.T) {
	// moto has not implemented ListFeatureGroups, so this run finds no
	// feature groups and has to carry on with everything else.
	result := discoverSeeded(t)

	assetNamed(t, result, "Model", "churn-xgb")
	assetNamed(t, result, "Endpoint", "churn-prod")
}

func TestE2E_EveryAssetMRNMatchesTheServersDerivation(t *testing.T) {
	result := discoverSeeded(t)
	require.NotEmpty(t, result.Assets)

	for _, a := range result.Assets {
		require.NotNil(t, a.Name)
		require.NotNil(t, a.MRN)
		require.NotEmpty(t, a.Providers)

		assert.Equal(t, strings.ToLower("mrn://"+a.Type+"/SageMaker/"+*a.Name), *a.MRN)
	}
}
