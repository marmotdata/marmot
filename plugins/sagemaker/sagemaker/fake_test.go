package sagemaker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/require"
)

// fakeAPI answers with canned responses in the shapes the real SageMaker
// API returns, observed from a live moto server. Setting an entry in errs
// makes that operation fail, which is how the tests cover a call the
// account has no permission for.
type fakeAPI struct {
	models             []types.ModelSummary
	modelDetails       map[string]*sagemaker.DescribeModelOutput
	tags               map[string][]types.Tag
	endpoints          []types.EndpointSummary
	endpointDetails    map[string]*sagemaker.DescribeEndpointOutput
	endpointConfigs    map[string]*sagemaker.DescribeEndpointConfigOutput
	packageGroups      []types.ModelPackageGroupSummary
	packageGroupDetail map[string]*sagemaker.DescribeModelPackageGroupOutput
	packages           map[string][]types.ModelPackageSummary
	packageDetails     map[string]*sagemaker.DescribeModelPackageOutput
	featureGroups      []types.FeatureGroupSummary
	featureDetails     map[string]*sagemaker.DescribeFeatureGroupOutput
	trainingJobs       []types.TrainingJobSummary
	trainingDetails    map[string]*sagemaker.DescribeTrainingJobOutput

	// modelPages, when set, is served instead of models, one page per
	// entry, so a test can prove pagination is followed to the end.
	modelPages [][]types.ModelSummary

	errs map[string]error
}

func (f *fakeAPI) fail(op string) error {
	if f.errs == nil {
		return nil
	}
	return f.errs[op]
}

func (f *fakeAPI) ListModels(ctx context.Context, params *sagemaker.ListModelsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelsOutput, error) {
	if err := f.fail("ListModels"); err != nil {
		return nil, err
	}

	if len(f.modelPages) == 0 {
		return &sagemaker.ListModelsOutput{Models: f.models}, nil
	}

	page := 0
	if params.NextToken != nil {
		if _, err := fmt.Sscanf(*params.NextToken, "page-%d", &page); err != nil {
			return nil, err
		}
	}
	out := &sagemaker.ListModelsOutput{Models: f.modelPages[page]}
	if page+1 < len(f.modelPages) {
		out.NextToken = aws.String(fmt.Sprintf("page-%d", page+1))
	}
	return out, nil
}

func (f *fakeAPI) DescribeModel(ctx context.Context, params *sagemaker.DescribeModelInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelOutput, error) {
	if err := f.fail("DescribeModel"); err != nil {
		return nil, err
	}
	out, ok := f.modelDetails[aws.ToString(params.ModelName)]
	if !ok {
		return nil, fmt.Errorf("model %q not found", aws.ToString(params.ModelName))
	}
	return out, nil
}

func (f *fakeAPI) ListTags(ctx context.Context, params *sagemaker.ListTagsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListTagsOutput, error) {
	if err := f.fail("ListTags"); err != nil {
		return nil, err
	}
	return &sagemaker.ListTagsOutput{Tags: f.tags[aws.ToString(params.ResourceArn)]}, nil
}

func (f *fakeAPI) ListEndpoints(ctx context.Context, params *sagemaker.ListEndpointsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListEndpointsOutput, error) {
	if err := f.fail("ListEndpoints"); err != nil {
		return nil, err
	}
	return &sagemaker.ListEndpointsOutput{Endpoints: f.endpoints}, nil
}

func (f *fakeAPI) DescribeEndpoint(ctx context.Context, params *sagemaker.DescribeEndpointInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeEndpointOutput, error) {
	if err := f.fail("DescribeEndpoint"); err != nil {
		return nil, err
	}
	out, ok := f.endpointDetails[aws.ToString(params.EndpointName)]
	if !ok {
		return nil, fmt.Errorf("endpoint %q not found", aws.ToString(params.EndpointName))
	}
	return out, nil
}

func (f *fakeAPI) DescribeEndpointConfig(ctx context.Context, params *sagemaker.DescribeEndpointConfigInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeEndpointConfigOutput, error) {
	if err := f.fail("DescribeEndpointConfig"); err != nil {
		return nil, err
	}
	out, ok := f.endpointConfigs[aws.ToString(params.EndpointConfigName)]
	if !ok {
		return nil, fmt.Errorf("endpoint config %q not found", aws.ToString(params.EndpointConfigName))
	}
	return out, nil
}

func (f *fakeAPI) ListModelPackageGroups(ctx context.Context, params *sagemaker.ListModelPackageGroupsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelPackageGroupsOutput, error) {
	if err := f.fail("ListModelPackageGroups"); err != nil {
		return nil, err
	}
	return &sagemaker.ListModelPackageGroupsOutput{ModelPackageGroupSummaryList: f.packageGroups}, nil
}

func (f *fakeAPI) DescribeModelPackageGroup(ctx context.Context, params *sagemaker.DescribeModelPackageGroupInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelPackageGroupOutput, error) {
	if err := f.fail("DescribeModelPackageGroup"); err != nil {
		return nil, err
	}
	out, ok := f.packageGroupDetail[aws.ToString(params.ModelPackageGroupName)]
	if !ok {
		return nil, fmt.Errorf("model package group %q not found", aws.ToString(params.ModelPackageGroupName))
	}
	return out, nil
}

func (f *fakeAPI) ListModelPackages(ctx context.Context, params *sagemaker.ListModelPackagesInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelPackagesOutput, error) {
	if err := f.fail("ListModelPackages"); err != nil {
		return nil, err
	}
	return &sagemaker.ListModelPackagesOutput{ModelPackageSummaryList: f.packages[aws.ToString(params.ModelPackageGroupName)]}, nil
}

func (f *fakeAPI) DescribeModelPackage(ctx context.Context, params *sagemaker.DescribeModelPackageInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelPackageOutput, error) {
	if err := f.fail("DescribeModelPackage"); err != nil {
		return nil, err
	}
	out, ok := f.packageDetails[aws.ToString(params.ModelPackageName)]
	if !ok {
		return nil, fmt.Errorf("model package %q not found", aws.ToString(params.ModelPackageName))
	}
	return out, nil
}

func (f *fakeAPI) ListFeatureGroups(ctx context.Context, params *sagemaker.ListFeatureGroupsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListFeatureGroupsOutput, error) {
	if err := f.fail("ListFeatureGroups"); err != nil {
		return nil, err
	}
	return &sagemaker.ListFeatureGroupsOutput{FeatureGroupSummaries: f.featureGroups}, nil
}

func (f *fakeAPI) DescribeFeatureGroup(ctx context.Context, params *sagemaker.DescribeFeatureGroupInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeFeatureGroupOutput, error) {
	if err := f.fail("DescribeFeatureGroup"); err != nil {
		return nil, err
	}
	out, ok := f.featureDetails[aws.ToString(params.FeatureGroupName)]
	if !ok {
		return nil, fmt.Errorf("feature group %q not found", aws.ToString(params.FeatureGroupName))
	}
	return out, nil
}

func (f *fakeAPI) ListTrainingJobs(ctx context.Context, params *sagemaker.ListTrainingJobsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListTrainingJobsOutput, error) {
	if err := f.fail("ListTrainingJobs"); err != nil {
		return nil, err
	}
	return &sagemaker.ListTrainingJobsOutput{TrainingJobSummaries: f.trainingJobs}, nil
}

func (f *fakeAPI) DescribeTrainingJob(ctx context.Context, params *sagemaker.DescribeTrainingJobInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeTrainingJobOutput, error) {
	if err := f.fail("DescribeTrainingJob"); err != nil {
		return nil, err
	}
	out, ok := f.trainingDetails[aws.ToString(params.TrainingJobName)]
	if !ok {
		return nil, fmt.Errorf("training job %q not found", aws.ToString(params.TrainingJobName))
	}
	return out, nil
}

// seedTime is the fixed creation time every fixture uses.
var seedTime = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// fullFake is the account the tests discover: one model served by one
// endpoint, one registry group, one feature group and one training job that
// produced the model's artifact.
func fullFake() *fakeAPI {
	return &fakeAPI{
		models: []types.ModelSummary{{
			ModelName:    aws.String("churn-xgb"),
			ModelArn:     aws.String("arn:aws:sagemaker:us-east-1:123456789012:model/churn-xgb"),
			CreationTime: aws.Time(seedTime),
		}},
		modelDetails: map[string]*sagemaker.DescribeModelOutput{
			"churn-xgb": {
				ModelArn: aws.String("arn:aws:sagemaker:us-east-1:123456789012:model/churn-xgb"),
				PrimaryContainer: &types.ContainerDefinition{
					Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
					ModelDataUrl: aws.String("s3://ml-artifacts/churn/model.tar.gz"),
					Mode:         types.ContainerModeSingleModel,
					Environment:  map[string]string{"LOG_LEVEL": "info", "API_SECRET": "hunter2"},
				},
				ExecutionRoleArn:       aws.String("arn:aws:iam::123456789012:role/SageMakerRole"),
				EnableNetworkIsolation: aws.Bool(false),
				CreationTime:           aws.Time(seedTime),
			},
		},
		tags: map[string][]types.Tag{
			"arn:aws:sagemaker:us-east-1:123456789012:model/churn-xgb": {
				{Key: aws.String("team"), Value: aws.String("growth")},
			},
		},
		endpoints: []types.EndpointSummary{{
			EndpointName: aws.String("churn-prod"),
			EndpointArn:  aws.String("arn:aws:sagemaker:us-east-1:123456789012:endpoint/churn-prod"),
		}},
		endpointDetails: map[string]*sagemaker.DescribeEndpointOutput{
			"churn-prod": {
				EndpointArn:        aws.String("arn:aws:sagemaker:us-east-1:123456789012:endpoint/churn-prod"),
				EndpointConfigName: aws.String("churn-cfg"),
				EndpointStatus:     types.EndpointStatusInService,
				CreationTime:       aws.Time(seedTime),
				LastModifiedTime:   aws.Time(seedTime),
			},
		},
		endpointConfigs: map[string]*sagemaker.DescribeEndpointConfigOutput{
			"churn-cfg": {
				ProductionVariants: []types.ProductionVariant{{
					VariantName:          aws.String("AllTraffic"),
					ModelName:            aws.String("churn-xgb"),
					InstanceType:         types.ProductionVariantInstanceTypeMlM5Large,
					InitialInstanceCount: aws.Int32(1),
					InitialVariantWeight: aws.Float32(1),
				}},
				DataCaptureConfig: &types.DataCaptureConfig{
					DestinationS3Uri: aws.String("s3://ml-capture/churn"),
				},
				KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc"),
			},
		},
		packageGroups: []types.ModelPackageGroupSummary{{
			ModelPackageGroupName: aws.String("churn-registry"),
			ModelPackageGroupArn:  aws.String("arn:aws:sagemaker:us-east-1:123456789012:model-package-group/churn-registry"),
		}},
		packageGroupDetail: map[string]*sagemaker.DescribeModelPackageGroupOutput{
			"churn-registry": {
				ModelPackageGroupArn:         aws.String("arn:aws:sagemaker:us-east-1:123456789012:model-package-group/churn-registry"),
				ModelPackageGroupDescription: aws.String("Churn model registry"),
				ModelPackageGroupStatus:      types.ModelPackageGroupStatusCompleted,
				CreationTime:                 aws.Time(seedTime),
			},
		},
		packages: map[string][]types.ModelPackageSummary{
			"churn-registry": {
				{
					ModelPackageArn:     aws.String("arn:aws:sagemaker:us-east-1:123456789012:model-package/churn-registry/1"),
					ModelPackageVersion: aws.Int32(1),
					ModelApprovalStatus: types.ModelApprovalStatusApproved,
				},
				{
					ModelPackageArn:     aws.String("arn:aws:sagemaker:us-east-1:123456789012:model-package/churn-registry/2"),
					ModelPackageVersion: aws.Int32(2),
					ModelApprovalStatus: types.ModelApprovalStatusPendingManualApproval,
				},
			},
		},
		packageDetails: map[string]*sagemaker.DescribeModelPackageOutput{
			"arn:aws:sagemaker:us-east-1:123456789012:model-package/churn-registry/1": {
				ModelPackageVersion: aws.Int32(1),
				Domain:              aws.String("MACHINE_LEARNING"),
				Task:                aws.String("CLASSIFICATION"),
				SamplePayloadUrl:    aws.String("s3://ml-artifacts/churn/sample.json"),
				InferenceSpecification: &types.InferenceSpecification{
					Containers: []types.ModelPackageContainerDefinition{{
						Image:        aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
						ModelDataUrl: aws.String("s3://ml-registry/churn/1/model.tar.gz"),
					}},
					SupportedContentTypes:      []string{"text/csv"},
					SupportedResponseMIMETypes: []string{"text/csv"},
				},
				ModelMetrics: &types.ModelMetrics{
					ModelQuality: &types.ModelQuality{
						Statistics: &types.MetricsSource{S3Uri: aws.String("s3://ml-artifacts/churn/quality.json")},
					},
				},
			},
		},
		featureGroups: []types.FeatureGroupSummary{{
			FeatureGroupName: aws.String("customer-features"),
			FeatureGroupArn:  aws.String("arn:aws:sagemaker:us-east-1:123456789012:feature-group/customer-features"),
		}},
		featureDetails: map[string]*sagemaker.DescribeFeatureGroupOutput{
			"customer-features": {
				FeatureGroupArn:             aws.String("arn:aws:sagemaker:us-east-1:123456789012:feature-group/customer-features"),
				RecordIdentifierFeatureName: aws.String("age"),
				EventTimeFeatureName:        aws.String("event_time"),
				Description:                 aws.String("Customer features for churn"),
				FeatureGroupStatus:          types.FeatureGroupStatusCreated,
				CreationTime:                aws.Time(seedTime),
				FeatureDefinitions: []types.FeatureDefinition{
					{FeatureName: aws.String("age"), FeatureType: types.FeatureTypeIntegral},
					{FeatureName: aws.String("plan"), FeatureType: types.FeatureTypeString},
					{FeatureName: aws.String("event_time"), FeatureType: types.FeatureTypeFractional},
				},
				OnlineStoreConfig: &types.OnlineStoreConfig{EnableOnlineStore: aws.Bool(true)},
				OfflineStoreConfig: &types.OfflineStoreConfig{
					S3StorageConfig: &types.S3StorageConfig{S3Uri: aws.String("s3://ml-features/customer-features")},
					DataCatalogConfig: &types.DataCatalogConfig{
						Database:  aws.String("sagemaker_featurestore"),
						TableName: aws.String("customer_features"),
						Catalog:   aws.String("AwsDataCatalog"),
					},
				},
			},
		},
		trainingJobs: []types.TrainingJobSummary{{
			TrainingJobName: aws.String("churn-train-2026-09-01"),
		}},
		trainingDetails: map[string]*sagemaker.DescribeTrainingJobOutput{
			"churn-train-2026-09-01": {
				TrainingJobArn:    aws.String("arn:aws:sagemaker:us-east-1:123456789012:training-job/churn-train-2026-09-01"),
				TrainingJobStatus: types.TrainingJobStatusCompleted,
				AlgorithmSpecification: &types.AlgorithmSpecification{
					TrainingImage: aws.String("123.dkr.ecr.us-east-1.amazonaws.com/xgboost:1"),
				},
				HyperParameters: map[string]string{"max_depth": "5"},
				InputDataConfig: []types.Channel{{
					ChannelName: aws.String("train"),
					DataSource: &types.DataSource{S3DataSource: &types.S3DataSource{
						S3Uri: aws.String("s3://ml-data/train"),
					}},
				}},
				OutputDataConfig: &types.OutputDataConfig{S3OutputPath: aws.String("s3://ml-artifacts/churn")},
				ResourceConfig: &types.ResourceConfig{
					InstanceType:  types.TrainingInstanceTypeMlM5Large,
					InstanceCount: aws.Int32(1),
				},
				ModelArtifacts: &types.ModelArtifacts{
					S3ModelArtifacts: aws.String("s3://ml-artifacts/churn/model.tar.gz"),
				},
				FinalMetricDataList: []types.MetricData{{
					MetricName: aws.String("validation:auc"),
					Value:      aws.Float32(0.91),
				}},
				TrainingStartTime: aws.Time(seedTime),
				TrainingEndTime:   aws.Time(seedTime.Add(30 * time.Minute)),
				CreationTime:      aws.Time(seedTime),
			},
		},
	}
}

// discoverWith runs discovery against a fake API with the given config
// overlaid on the defaults.
func discoverWith(t *testing.T, fake *fakeAPI, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	raw := pluginsdk.RawConfig{"credentials": map[string]any{"region": "us-east-1"}}
	for key, value := range overrides {
		raw[key] = value
	}

	source := &Source{}
	_, err := source.Validate(raw)
	require.NoError(t, err)

	source.client = fake
	source.region = "us-east-1"

	result, err := source.discover(t.Context())
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
}

// assetNamed returns the discovered asset with the given type and name.
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

// hasEdge reports whether the result contains exactly this lineage edge.
func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, e := range result.Lineage {
		if e.Source == source && e.Target == target && e.Type == edgeType {
			return true
		}
	}
	return false
}
