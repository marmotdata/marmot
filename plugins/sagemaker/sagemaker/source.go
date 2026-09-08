// Package sagemaker discovers models, inference endpoints, feature groups
// and training jobs from Amazon SageMaker.
package sagemaker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service name every SageMaker asset is filed under.
const provider = "SageMaker"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "sagemaker",
		Name:        "SageMaker",
		Description: "Discover models, endpoints, feature groups and training jobs from Amazon SageMaker",
		Icon:        "sagemaker",
		Category:    "ml",
		Status:      "experimental",
		// Discover emits FEEDS and PRODUCES edges, and run history for
		// training jobs, so the manifest declares all three features.
		Features:   []string{"Assets", "Lineage", "Run History"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the SageMaker plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

	IncludeEndpoints     bool `json:"include_endpoints" description:"Whether to discover inference endpoints" default:"true"`
	IncludeModelPackages bool `json:"include_model_packages" description:"Whether to discover model package groups from the model registry" default:"true"`
	IncludeFeatureGroups bool `json:"include_feature_groups" description:"Whether to discover feature groups from the feature store" default:"true"`
	IncludeTrainingJobs  bool `json:"include_training_jobs" description:"Whether to discover training jobs. Accounts keep a long job history, so this is off by default" default:"false"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
tags_to_metadata: true
include_endpoints: true
include_model_packages: true
include_feature_groups: true
include_training_jobs: false
tags:
  - "aws"
  - "ml"
`

// Source represents the SageMaker plugin.
type Source struct {
	config *Config
	client api
	region string
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// The shared AWS section is only allocated when the config sets one of
	// its keys, and its tag handling defaults to on for this plugin.
	if config.AWSConfig == nil {
		config.AWSConfig = &pluginsdk.AWSConfig{}
	}
	if _, ok := rawConfig["tags_to_metadata"]; !ok {
		config.TagsToMetadata = true
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers SageMaker models, endpoints, feature groups and
// training jobs.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	awsConfig, err := pluginsdk.ExtractAWSConfig(rawConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = sagemaker.NewFromConfig(awsCfg)
	s.region = awsCfg.Region

	return s.discover(ctx)
}

// discover runs the discovery passes against an already configured client.
// It is separate from Discover so tests can drive it with a fake API.
func (s *Source) discover(ctx context.Context) (*pluginsdk.DiscoveryResult, error) {
	result := &pluginsdk.DiscoveryResult{}

	// Models are the anchor of every other pass, so a failure here means
	// the account or endpoint is unreachable and discovery cannot go on.
	modelAssets, index, err := s.discoverModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering models: %w", err)
	}
	result.Assets = append(result.Assets, modelAssets...)
	result.Lineage = append(result.Lineage, artifactEdges(modelAssets)...)

	if s.config.IncludeModelPackages {
		groupAssets, err := s.discoverModelPackageGroups(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover model package groups")
		} else {
			result.Assets = append(result.Assets, groupAssets...)
			result.Lineage = append(result.Lineage, artifactEdges(groupAssets)...)
		}
	}

	if s.config.IncludeEndpoints {
		endpointAssets, edges, err := s.discoverEndpoints(ctx, index)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover endpoints")
		} else {
			result.Assets = append(result.Assets, endpointAssets...)
			result.Lineage = append(result.Lineage, edges...)
		}
	}

	if s.config.IncludeFeatureGroups {
		groupAssets, edges, err := s.discoverFeatureGroups(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover feature groups")
		} else {
			result.Assets = append(result.Assets, groupAssets...)
			result.Lineage = append(result.Lineage, edges...)
		}
	}

	if s.config.IncludeTrainingJobs {
		jobAssets, edges, runs, err := s.discoverTrainingJobs(ctx, index)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover training jobs")
		} else {
			result.Assets = append(result.Assets, jobAssets...)
			result.Lineage = append(result.Lineage, edges...)
			result.RunHistory = append(result.RunHistory, runs...)
		}
	}

	log.Info().
		Int("assets", len(result.Assets)).
		Int("lineages", len(result.Lineage)).
		Int("run_histories", len(result.RunHistory)).
		Msg("SageMaker discovery completed")

	return result, nil
}

// assetMRN builds the identity of every asset this plugin owns. The Marmot
// server rebuilds an MRN from an asset's type, provider and name, so every
// MRN and every lineage endpoint has to come from here.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// newAsset fills in the parts every SageMaker asset shares.
func (s *Source) newAsset(assetType, name, description string, metadata map[string]any) pluginsdk.Asset {
	mrnValue := assetMRN(assetType, name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if description != "" {
		asset.Description = &description
	}

	if url, ok := metadata["url"].(string); ok && url != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{
			Name: "Open in AWS Console",
			URL:  url,
		}}
	}

	return asset
}

// consoleURL builds a deep link into the SageMaker console. path is the
// console's own hash route, for example "models/churn-xgb".
func (s *Source) consoleURL(path string) string {
	if s.region == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.console.aws.amazon.com/sagemaker/home?region=%s#/%s", s.region, s.region, path)
}

// resourceTags reads a resource's AWS tags. Tagging is best effort: a
// missing permission on one resource should not lose the whole asset.
func (s *Source) resourceTags(ctx context.Context, arn string) map[string]any {
	if !s.config.TagsToMetadata || arn == "" {
		return nil
	}

	out, err := s.client.ListTags(ctx, &sagemaker.ListTagsInput{ResourceArn: &arn})
	if err != nil {
		log.Warn().Err(err).Str("arn", arn).Msg("Failed to list tags")
		return nil
	}

	tags := make(map[string]string, len(out.Tags))
	for _, t := range out.Tags {
		if t.Key == nil {
			continue
		}
		tags[*t.Key] = deref(t.Value)
	}

	return pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tags)
}

// artifactEdges links each model to the S3 bucket its artifact is stored
// in. The bucket asset is owned by the S3 plugin, so only the edge is
// emitted here, and the server drops it when no such bucket is catalogued.
func artifactEdges(assets []pluginsdk.Asset) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge

	for _, a := range assets {
		for _, key := range []string{"model_data_url", "latest_model_data_url"} {
			url, _ := a.Metadata[key].(string)
			bucket := s3Bucket(url)
			if bucket == "" {
				continue
			}
			edges = append(edges, pluginsdk.LineageEdge{
				Source: mrn.New("Bucket", "S3", bucket),
				Target: *a.MRN,
				Type:   "FEEDS",
			})
		}
	}

	return edges
}

// s3Bucket returns the bucket name of an s3:// URI, or "" when the value is
// not one. SageMaker also accepts artifacts referenced by other schemes.
func s3Bucket(uri string) string {
	rest, ok := strings.CutPrefix(uri, "s3://")
	if !ok {
		return ""
	}
	bucket, _, _ := strings.Cut(rest, "/")
	return bucket
}

// sensitiveEnvMarkers are the substrings that make an environment variable
// look like a credential. Their values are replaced rather than dropped, so
// the catalog still shows that the variable is set.
var sensitiveEnvMarkers = []string{"SECRET", "TOKEN", "PASSWORD", "KEY"}

const maskedValue = "***"

// maskEnvironment hides the values of environment variables whose name
// looks like a credential.
func maskEnvironment(env map[string]string) map[string]any {
	if len(env) == 0 {
		return nil
	}

	masked := make(map[string]any, len(env))
	for key, value := range env {
		masked[key] = value
		upper := strings.ToUpper(key)
		for _, marker := range sensitiveEnvMarkers {
			if strings.Contains(upper, marker) {
				masked[key] = maskedValue
				break
			}
		}
	}

	return masked
}

// putString records a value only when it is set, so no empty keys reach
// the catalog.
func putString(m map[string]any, key string, value *string) {
	if value == nil || *value == "" {
		return
	}
	m[key] = *value
}

// putTime records a timestamp in RFC3339, skipping the zero value AWS
// returns for an unset field.
func putTime(m map[string]any, key string, value *time.Time) {
	if value == nil || value.IsZero() {
		return
	}
	m[key] = value.UTC().Format(time.RFC3339)
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
