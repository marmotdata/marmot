package sagemaker

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModels_CatalogsTheModel(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, []string{"SageMaker"}, model.Providers)
	assert.Equal(t, "model", model.Metadata["kind"])
}

func TestDiscoverModels_RecordsTheServingImage(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", model.Metadata["image"])
}

func TestDiscoverModels_RecordsTheArtifactLocation(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "s3://ml-artifacts/churn/model.tar.gz", model.Metadata["model_data_url"])
}

func TestDiscoverModels_RecordsTheExecutionRole(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "arn:aws:iam::123456789012:role/SageMakerRole", model.Metadata["execution_role_arn"])
}

func TestDiscoverModels_RecordsTheCreationTimeAsRFC3339(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "2026-09-01T10:00:00Z", model.Metadata["created_at"])
}

func TestDiscoverModels_MasksSecretLookingEnvironmentValues(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")
	env, ok := model.Metadata["environment"].(map[string]any)
	require.True(t, ok, "expected an environment map")

	assert.Equal(t, "info", env["LOG_LEVEL"])
	assert.Equal(t, maskedValue, env["API_SECRET"])
}

func TestDiscoverModels_TurnsAWSTagsIntoMetadata(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.Equal(t, "growth", model.Metadata["tag_team"])
}

func TestDiscoverModels_SkipsTagsWhenTheConfigTurnsThemOff(t *testing.T) {
	result := discoverWith(t, fullFake(), pluginsdk.RawConfig{"tags_to_metadata": false})

	model := assetNamed(t, result, "Model", "churn-xgb")

	assert.NotContains(t, model.Metadata, "tag_team")
}

func TestDiscoverModels_KeepsTheModelWhenTaggingFails(t *testing.T) {
	// A missing sagemaker:ListTags permission should cost the tags, not
	// the asset.
	fake := fullFake()
	fake.errs = map[string]error{"ListTags": errors.New("access denied")}

	result := discoverWith(t, fake, nil)

	model := assetNamed(t, result, "Model", "churn-xgb")
	assert.NotContains(t, model.Metadata, "tag_team")
}

func TestDiscoverModels_LinksToTheAWSConsole(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	model := assetNamed(t, result, "Model", "churn-xgb")

	require.Len(t, model.ExternalLinks, 1)
	assert.Equal(t, "Open in AWS Console", model.ExternalLinks[0].Name)
	assert.Equal(t,
		"https://us-east-1.console.aws.amazon.com/sagemaker/home?region=us-east-1#/models/churn-xgb",
		model.ExternalLinks[0].URL)
}

func TestDiscoverModels_RecordsTheInferencePipelineContainers(t *testing.T) {
	fake := fullFake()
	fake.modelDetails["churn-xgb"].Containers = []types.ContainerDefinition{{
		Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/preprocess:1"),
		ModelDataUrl: aws.String("s3://ml-artifacts/churn/preprocess.tar.gz"),
	}}

	result := discoverWith(t, fake, nil)

	model := assetNamed(t, result, "Model", "churn-xgb")
	containers, ok := model.Metadata["containers"].([]map[string]any)
	require.True(t, ok, "expected a container list")
	require.Len(t, containers, 1)
	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/preprocess:1", containers[0]["image"])
}

func TestDiscoverModels_OmitsTheVpcCountForAModelOutsideAVpc(t *testing.T) {
	// The API answers with an empty VpcConfig rather than none at all.
	fake := fullFake()
	fake.modelDetails["churn-xgb"].VpcConfig = &types.VpcConfig{}

	result := discoverWith(t, fake, nil)

	model := assetNamed(t, result, "Model", "churn-xgb")
	assert.NotContains(t, model.Metadata, "vpc_subnet_count")
}

func TestDiscoverModels_CountsTheVpcSubnets(t *testing.T) {
	fake := fullFake()
	fake.modelDetails["churn-xgb"].VpcConfig = &types.VpcConfig{Subnets: []string{"subnet-a", "subnet-b"}}

	result := discoverWith(t, fake, nil)

	model := assetNamed(t, result, "Model", "churn-xgb")
	assert.Equal(t, 2, model.Metadata["vpc_subnet_count"])
}

func TestDiscoverModels_SkipsAModelItCannotDescribe(t *testing.T) {
	fake := fullFake()
	fake.models = append(fake.models, types.ModelSummary{ModelName: aws.String("ghost")})

	result := discoverWith(t, fake, nil)

	for _, a := range result.Assets {
		assert.NotEqual(t, "ghost", *a.Name)
	}
	assetNamed(t, result, "Model", "churn-xgb")
}

func TestDiscoverModels_LinksTheArtifactBucketToTheModel(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-artifacts", "mrn://model/sagemaker/churn-xgb", "FEEDS"))
}

func TestDiscoverModels_FollowsPaginationToTheEnd(t *testing.T) {
	fake := fullFake()
	fake.modelPages = [][]types.ModelSummary{
		{{ModelName: aws.String("churn-xgb")}},
		{{ModelName: aws.String("churn-lgbm")}},
	}
	fake.modelDetails["churn-lgbm"] = &sagemaker.DescribeModelOutput{
		PrimaryContainer: &types.ContainerDefinition{Image: aws.String("lgbm:1")},
	}

	result := discoverWith(t, fake, nil)

	assetNamed(t, result, "Model", "churn-xgb")
	assetNamed(t, result, "Model", "churn-lgbm")
}

func TestDiscover_FailsWhenModelsCannotBeListed(t *testing.T) {
	// Models are the first call, so a failure here means the account is
	// unreachable and there is nothing to catalog.
	fake := fullFake()
	fake.errs = map[string]error{"ListModels": errors.New("connection refused")}

	source := &Source{}
	_, err := source.Validate(pluginsdk.RawConfig{})
	require.NoError(t, err)
	source.client = fake

	_, err = source.discover(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovering models")
}

func TestDiscoverModelPackageGroups_CatalogsTheGroupAsAModel(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, "model_package_group", group.Metadata["kind"])
}

func TestDiscoverModelPackageGroups_UsesTheGroupDescription(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	require.NotNil(t, group.Description)
	assert.Equal(t, "Churn model registry", *group.Description)
}

func TestDiscoverModelPackageGroups_CountsTheVersions(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, 2, group.Metadata["version_count"])
}

func TestDiscoverModelPackageGroups_ReportsTheNewestVersion(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, int32(2), group.Metadata["latest_version"])
	assert.Equal(t, "PendingManualApproval", group.Metadata["latest_approval_status"])
}

func TestDiscoverModelPackageGroups_DescribesTheNewestApprovedVersion(t *testing.T) {
	// Version 2 is awaiting approval, so what a consumer would deploy is
	// still version 1.
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, "123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1", group.Metadata["latest_image"])
	assert.Equal(t, "s3://ml-registry/churn/1/model.tar.gz", group.Metadata["latest_model_data_url"])
	assert.Equal(t, "CLASSIFICATION", group.Metadata["task"])
	assert.Equal(t, "MACHINE_LEARNING", group.Metadata["domain"])
}

func TestDiscoverModelPackageGroups_RecordsTheModelQualityReport(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Equal(t, "s3://ml-artifacts/churn/quality.json", group.Metadata["model_quality_statistics_s3_uri"])
}

func TestDiscoverModelPackageGroups_LinksTheRegistryBucketToTheGroup(t *testing.T) {
	result := discoverWith(t, fullFake(), nil)

	assert.True(t, hasEdge(result, "mrn://bucket/s3/ml-registry", "mrn://model/sagemaker/churn-registry", "FEEDS"))
}

func TestDiscoverModelPackageGroups_HasNoConsoleLink(t *testing.T) {
	// The model registry has no confirmed route in the classic console, so
	// no link is emitted rather than one that would not resolve.
	result := discoverWith(t, fullFake(), nil)

	group := assetNamed(t, result, "Model", "churn-registry")

	assert.Empty(t, group.ExternalLinks)
	assert.NotContains(t, group.Metadata, "url")
}

func TestDiscoverModelPackageGroups_AreSkippedWhenTurnedOff(t *testing.T) {
	result := discoverWith(t, fullFake(), pluginsdk.RawConfig{"include_model_packages": false})

	for _, a := range result.Assets {
		assert.NotEqual(t, "churn-registry", *a.Name)
	}
}

func TestDiscoverModelPackageGroups_FailureKeepsTheRestOfTheRun(t *testing.T) {
	fake := fullFake()
	fake.errs = map[string]error{"ListModelPackageGroups": errors.New("access denied")}

	result := discoverWith(t, fake, nil)

	assetNamed(t, result, "Model", "churn-xgb")
}

func TestDiscoverModelPackageGroups_KeepsTheGroupWhenVersionsCannotBeListed(t *testing.T) {
	fake := fullFake()
	fake.errs = map[string]error{"ListModelPackages": errors.New("access denied")}

	result := discoverWith(t, fake, nil)

	group := assetNamed(t, result, "Model", "churn-registry")
	assert.NotContains(t, group.Metadata, "version_count")
}

func TestLatestPackage_PicksTheHighestVersion(t *testing.T) {
	packages := []types.ModelPackageSummary{
		{ModelPackageVersion: aws.Int32(1)},
		{ModelPackageVersion: aws.Int32(3)},
		{ModelPackageVersion: aws.Int32(2)},
	}

	latest := latestPackage(packages, false)

	require.NotNil(t, latest)
	assert.Equal(t, int32(3), *latest.ModelPackageVersion)
}

func TestLatestPackage_SkipsVersionsThatAreNotApproved(t *testing.T) {
	packages := []types.ModelPackageSummary{
		{ModelPackageVersion: aws.Int32(1), ModelApprovalStatus: types.ModelApprovalStatusApproved},
		{ModelPackageVersion: aws.Int32(2), ModelApprovalStatus: types.ModelApprovalStatusRejected},
	}

	latest := latestPackage(packages, true)

	require.NotNil(t, latest)
	assert.Equal(t, int32(1), *latest.ModelPackageVersion)
}

func TestLatestPackage_ReturnsNothingWhenNoVersionIsApproved(t *testing.T) {
	packages := []types.ModelPackageSummary{
		{ModelPackageVersion: aws.Int32(1), ModelApprovalStatus: types.ModelApprovalStatusRejected},
	}

	assert.Nil(t, latestPackage(packages, true))
}
