package sagemaker

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/rs/zerolog/log"
)

// api is the part of the SageMaker API this plugin calls. The generated AWS
// client satisfies it, and tests swap in their own implementation.
type api interface {
	ListModels(ctx context.Context, params *sagemaker.ListModelsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelsOutput, error)
	DescribeModel(ctx context.Context, params *sagemaker.DescribeModelInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelOutput, error)
	ListTags(ctx context.Context, params *sagemaker.ListTagsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListTagsOutput, error)
	ListEndpoints(ctx context.Context, params *sagemaker.ListEndpointsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListEndpointsOutput, error)
	DescribeEndpoint(ctx context.Context, params *sagemaker.DescribeEndpointInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeEndpointOutput, error)
	DescribeEndpointConfig(ctx context.Context, params *sagemaker.DescribeEndpointConfigInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeEndpointConfigOutput, error)
	ListModelPackageGroups(ctx context.Context, params *sagemaker.ListModelPackageGroupsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelPackageGroupsOutput, error)
	DescribeModelPackageGroup(ctx context.Context, params *sagemaker.DescribeModelPackageGroupInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelPackageGroupOutput, error)
	ListModelPackages(ctx context.Context, params *sagemaker.ListModelPackagesInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListModelPackagesOutput, error)
	DescribeModelPackage(ctx context.Context, params *sagemaker.DescribeModelPackageInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeModelPackageOutput, error)
	ListFeatureGroups(ctx context.Context, params *sagemaker.ListFeatureGroupsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListFeatureGroupsOutput, error)
	DescribeFeatureGroup(ctx context.Context, params *sagemaker.DescribeFeatureGroupInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeFeatureGroupOutput, error)
	ListTrainingJobs(ctx context.Context, params *sagemaker.ListTrainingJobsInput, optFns ...func(*sagemaker.Options)) (*sagemaker.ListTrainingJobsOutput, error)
	DescribeTrainingJob(ctx context.Context, params *sagemaker.DescribeTrainingJobInput, optFns ...func(*sagemaker.Options)) (*sagemaker.DescribeTrainingJobOutput, error)
}

// maxPages stops discovery from walking an unbounded result set. At the AWS
// maximum page size this still covers ten thousand resources of one kind.
const maxPages = 100

// pageLimitReached logs the one warning that tells an operator some
// resources were left out.
func pageLimitReached(kind string) {
	log.Warn().Str("resource", kind).Int("max_pages", maxPages).
		Msg("Stopped paginating at the page limit, some resources were not discovered")
}

func (s *Source) listModels(ctx context.Context) ([]types.ModelSummary, error) {
	paginator := sagemaker.NewListModelsPaginator(s.client, &sagemaker.ListModelsInput{})

	var summaries []types.ModelSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("models")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing models: %w", err)
		}
		summaries = append(summaries, page.Models...)
	}

	return summaries, nil
}

func (s *Source) listEndpoints(ctx context.Context) ([]types.EndpointSummary, error) {
	paginator := sagemaker.NewListEndpointsPaginator(s.client, &sagemaker.ListEndpointsInput{})

	var summaries []types.EndpointSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("endpoints")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing endpoints: %w", err)
		}
		summaries = append(summaries, page.Endpoints...)
	}

	return summaries, nil
}

func (s *Source) listModelPackageGroups(ctx context.Context) ([]types.ModelPackageGroupSummary, error) {
	paginator := sagemaker.NewListModelPackageGroupsPaginator(s.client, &sagemaker.ListModelPackageGroupsInput{})

	var summaries []types.ModelPackageGroupSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("model package groups")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing model package groups: %w", err)
		}
		summaries = append(summaries, page.ModelPackageGroupSummaryList...)
	}

	return summaries, nil
}

func (s *Source) listModelPackages(ctx context.Context, group string) ([]types.ModelPackageSummary, error) {
	paginator := sagemaker.NewListModelPackagesPaginator(s.client, &sagemaker.ListModelPackagesInput{
		ModelPackageGroupName: &group,
	})

	var summaries []types.ModelPackageSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("model packages")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing model packages: %w", err)
		}
		summaries = append(summaries, page.ModelPackageSummaryList...)
	}

	return summaries, nil
}

func (s *Source) listFeatureGroups(ctx context.Context) ([]types.FeatureGroupSummary, error) {
	paginator := sagemaker.NewListFeatureGroupsPaginator(s.client, &sagemaker.ListFeatureGroupsInput{})

	var summaries []types.FeatureGroupSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("feature groups")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing feature groups: %w", err)
		}
		summaries = append(summaries, page.FeatureGroupSummaries...)
	}

	return summaries, nil
}

func (s *Source) listTrainingJobs(ctx context.Context) ([]types.TrainingJobSummary, error) {
	paginator := sagemaker.NewListTrainingJobsPaginator(s.client, &sagemaker.ListTrainingJobsInput{})

	var summaries []types.TrainingJobSummary
	for pages := 0; paginator.HasMorePages(); pages++ {
		if pages == maxPages {
			pageLimitReached("training jobs")
			break
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing training jobs: %w", err)
		}
		summaries = append(summaries, page.TrainingJobSummaries...)
	}

	return summaries, nil
}
