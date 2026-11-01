package sagemaker

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// modelIndex is what the later passes need to know about the models this run
// found: endpoints link to them by name, training jobs by artifact location.
type modelIndex struct {
	names      map[string]bool
	byArtifact map[string]string
}

func newModelIndex() *modelIndex {
	return &modelIndex{
		names:      map[string]bool{},
		byArtifact: map[string]string{},
	}
}

// discoverModels catalogues every deployed model definition.
func (s *Source) discoverModels(ctx context.Context) ([]pluginsdk.Asset, *modelIndex, error) {
	summaries, err := s.listModels(ctx)
	if err != nil {
		return nil, nil, err
	}

	index := newModelIndex()
	var assets []pluginsdk.Asset

	for _, summary := range summaries {
		name := deref(summary.ModelName)
		if name == "" {
			continue
		}

		detail, err := s.client.DescribeModel(ctx, &sagemaker.DescribeModelInput{ModelName: &name})
		if err != nil {
			log.Warn().Err(err).Str("model", name).Msg("Failed to describe model")
			continue
		}

		asset := s.modelAsset(ctx, name, summary, detail)
		assets = append(assets, asset)

		index.names[name] = true
		if url, ok := asset.Metadata["model_data_url"].(string); ok && url != "" {
			index.byArtifact[url] = name
		}
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered models")
	return assets, index, nil
}

func (s *Source) modelAsset(ctx context.Context, name string, summary types.ModelSummary, detail *sagemaker.DescribeModelOutput) pluginsdk.Asset {
	arn := deref(summary.ModelArn)
	if arn == "" {
		arn = deref(detail.ModelArn)
	}

	metadata := s.resourceTags(ctx, arn)
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadata["kind"] = "model"
	if arn != "" {
		metadata["arn"] = arn
	}
	putString(metadata, "execution_role_arn", detail.ExecutionRoleArn)

	if c := detail.PrimaryContainer; c != nil {
		putString(metadata, "image", c.Image)
		putString(metadata, "model_data_url", c.ModelDataUrl)
		if c.Mode != "" {
			metadata["mode"] = string(c.Mode)
		}
		if env := maskEnvironment(c.Environment); env != nil {
			metadata["environment"] = env
		}
	}

	// Containers is only set for an inference pipeline, where several
	// containers serve one model in sequence.
	if containers := containerList(detail.Containers); len(containers) > 0 {
		metadata["containers"] = containers
	}

	if detail.EnableNetworkIsolation != nil {
		metadata["network_isolation"] = *detail.EnableNetworkIsolation
	}
	if detail.VpcConfig != nil && len(detail.VpcConfig.Subnets) > 0 {
		metadata["vpc_subnet_count"] = len(detail.VpcConfig.Subnets)
	}

	putTime(metadata, "created_at", summary.CreationTime)
	if _, ok := metadata["created_at"]; !ok {
		putTime(metadata, "created_at", detail.CreationTime)
	}

	if s.region != "" {
		metadata["region"] = s.region
	}
	if url := s.consoleURL("models/" + name); url != "" {
		metadata["url"] = url
	}

	return s.newAsset("Model", name, "", metadata)
}

// containerList flattens an inference pipeline's containers into the shape
// the catalog stores them in.
func containerList(containers []types.ContainerDefinition) []map[string]any {
	var out []map[string]any

	for _, c := range containers {
		entry := map[string]any{}
		putString(entry, "image", c.Image)
		putString(entry, "model_data_url", c.ModelDataUrl)
		if c.Mode != "" {
			entry["mode"] = string(c.Mode)
		}
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}

	return out
}

// discoverModelPackageGroups catalogues the model registry. A group is the
// versioned history of one model, so it is filed as a Model too, with a
// kind of "model_package_group" to tell the two apart. A group named the
// same as a deployed model shares its identity and the two merge into one
// asset in the catalog.
func (s *Source) discoverModelPackageGroups(ctx context.Context) ([]pluginsdk.Asset, error) {
	summaries, err := s.listModelPackageGroups(ctx)
	if err != nil {
		return nil, err
	}

	var assets []pluginsdk.Asset

	for _, summary := range summaries {
		name := deref(summary.ModelPackageGroupName)
		if name == "" {
			continue
		}

		asset, err := s.modelPackageGroupAsset(ctx, name, summary)
		if err != nil {
			log.Warn().Err(err).Str("model_package_group", name).Msg("Failed to describe model package group")
			continue
		}
		assets = append(assets, asset)
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered model package groups")
	return assets, nil
}

func (s *Source) modelPackageGroupAsset(ctx context.Context, name string, summary types.ModelPackageGroupSummary) (pluginsdk.Asset, error) {
	detail, err := s.client.DescribeModelPackageGroup(ctx, &sagemaker.DescribeModelPackageGroupInput{
		ModelPackageGroupName: &name,
	})
	if err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("describing model package group: %w", err)
	}

	arn := deref(summary.ModelPackageGroupArn)
	if arn == "" {
		arn = deref(detail.ModelPackageGroupArn)
	}

	metadata := s.resourceTags(ctx, arn)
	if metadata == nil {
		metadata = map[string]any{}
	}

	metadata["kind"] = "model_package_group"
	if arn != "" {
		metadata["arn"] = arn
	}

	description := deref(detail.ModelPackageGroupDescription)
	if description == "" {
		description = deref(summary.ModelPackageGroupDescription)
	}
	if description != "" {
		metadata["description"] = description
	}

	if detail.ModelPackageGroupStatus != "" {
		metadata["status"] = string(detail.ModelPackageGroupStatus)
	}

	putTime(metadata, "created_at", detail.CreationTime)
	if _, ok := metadata["created_at"]; !ok {
		putTime(metadata, "created_at", summary.CreationTime)
	}

	s.addVersionMetadata(ctx, name, metadata)

	if s.region != "" {
		metadata["region"] = s.region
	}
	// No console link: the model registry is only browsable in SageMaker
	// Studio, and the classic console has no confirmed route to a group.

	return s.newAsset("Model", name, description, metadata), nil
}

// addVersionMetadata records how many versions a group holds and what the
// newest one is, then describes the newest approved version to pick up the
// image and artifact a consumer would actually deploy.
func (s *Source) addVersionMetadata(ctx context.Context, group string, metadata map[string]any) {
	packages, err := s.listModelPackages(ctx, group)
	if err != nil {
		log.Warn().Err(err).Str("model_package_group", group).Msg("Failed to list model packages")
		return
	}
	if len(packages) == 0 {
		return
	}

	metadata["version_count"] = len(packages)

	latest := latestPackage(packages, false)
	if latest != nil {
		if latest.ModelPackageVersion != nil {
			metadata["latest_version"] = *latest.ModelPackageVersion
		}
		if latest.ModelApprovalStatus != "" {
			metadata["latest_approval_status"] = string(latest.ModelApprovalStatus)
		}
	}

	approved := latestPackage(packages, true)
	if approved == nil || approved.ModelPackageArn == nil {
		return
	}

	detail, err := s.client.DescribeModelPackage(ctx, &sagemaker.DescribeModelPackageInput{
		ModelPackageName: approved.ModelPackageArn,
	})
	if err != nil {
		log.Warn().Err(err).Str("model_package_group", group).Msg("Failed to describe latest approved model package")
		return
	}

	putString(metadata, "domain", detail.Domain)
	putString(metadata, "task", detail.Task)
	putString(metadata, "sample_payload_url", detail.SamplePayloadUrl)

	if spec := detail.InferenceSpecification; spec != nil {
		if len(spec.Containers) > 0 {
			putString(metadata, "latest_image", spec.Containers[0].Image)
			putString(metadata, "latest_model_data_url", spec.Containers[0].ModelDataUrl)
		}
		if len(spec.SupportedContentTypes) > 0 {
			metadata["supported_content_types"] = spec.SupportedContentTypes
		}
		if len(spec.SupportedResponseMIMETypes) > 0 {
			metadata["supported_response_mime_types"] = spec.SupportedResponseMIMETypes
		}
	}

	if m := detail.ModelMetrics; m != nil && m.ModelQuality != nil && m.ModelQuality.Statistics != nil {
		putString(metadata, "model_quality_statistics_s3_uri", m.ModelQuality.Statistics.S3Uri)
	}
}

// latestPackage returns the highest numbered version in a group, optionally
// restricted to the versions that have been approved for deployment.
func latestPackage(packages []types.ModelPackageSummary, approvedOnly bool) *types.ModelPackageSummary {
	var latest *types.ModelPackageSummary

	for i := range packages {
		p := &packages[i]
		if p.ModelPackageVersion == nil {
			continue
		}
		if approvedOnly && p.ModelApprovalStatus != types.ModelApprovalStatusApproved {
			continue
		}
		if latest == nil || *p.ModelPackageVersion > *latest.ModelPackageVersion {
			latest = p
		}
	}

	return latest
}
