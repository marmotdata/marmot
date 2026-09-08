// Package firehose discovers Amazon Data Firehose delivery streams from
// AWS accounts.
package firehose

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact service string Marmot stores for Firehose assets.
// It matches the OpenMetadata projection, so a Firehose service catalogued
// through OpenMetadata lands on the same assets as this plugin.
const provider = "Firehose"

// assetType is the only asset kind this plugin creates. Everything a
// stream reads from or writes to belongs to another plugin.
const assetType = "DeliveryStream"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "firehose",
		Name:        "AWS Firehose",
		Description: "Discover Amazon Data Firehose delivery streams from AWS accounts",
		Icon:        "firehose",
		Category:    "messaging",
		Status:      "experimental",
		// Discover links each stream to the Kinesis stream or Kafka topic
		// it reads and to the tables and buckets it writes, so the manifest
		// declares Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Firehose plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

	DiscoverLineage          bool `json:"discover_lineage" description:"Link each stream to the systems it reads from and writes to" default:"true"`
	IncludeDestinationConfig bool `json:"include_destination_config" description:"Record the destination settings in metadata" default:"true"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
discover_lineage: true
include_destination_config: true
tags_to_metadata: true
tags:
  - "aws"
  - "firehose"
`

// Source represents the Firehose plugin.
type Source struct {
	config *Config
	client firehoseAPI
	region string
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Firehose delivery streams and the lineage between
// them and the systems they are wired to.
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

	s.client = firehose.NewFromConfig(awsCfg)
	s.region = awsCfg.Region

	return s.discover(ctx)
}

// discover holds the discovery logic that does not depend on how the API
// client was built, so the unit tests can drive it with a fake API.
func (s *Source) discover(ctx context.Context) (*pluginsdk.DiscoveryResult, error) {
	names, err := listDeliveryStreams(ctx, s.client)
	if err != nil {
		return nil, fmt.Errorf("listing delivery streams: %w", err)
	}

	log.Debug().Int("count", len(names)).Msg("Listed delivery streams")

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge

	for _, name := range names {
		description, err := describeDeliveryStream(ctx, s.client, name)
		if err != nil {
			log.Warn().Err(err).Str("stream", name).Msg("Failed to describe delivery stream")
			continue
		}

		asset := s.buildAsset(ctx, description)
		assets = append(assets, asset)

		if s.config.DiscoverLineage {
			lineages = append(lineages, streamLineage(description)...)
		}
	}

	log.Info().Int("assets", len(assets)).Int("lineage_edges", len(lineages)).
		Msg("Firehose discovery complete")

	return &pluginsdk.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

// buildAsset turns one delivery stream description into a catalog asset.
func (s *Source) buildAsset(ctx context.Context, d *types.DeliveryStreamDescription) pluginsdk.Asset {
	name := aws.ToString(d.DeliveryStreamName)
	streamARN := aws.ToString(d.DeliveryStreamARN)

	metadata := s.streamTags(ctx, name)

	// The ARN carries the region the stream really lives in, which beats
	// the configured region when a role points at another one.
	region := arnRegion(streamARN)
	if region == "" {
		region = s.region
	}

	putString(metadata, "arn", streamARN)
	putString(metadata, "status", string(d.DeliveryStreamStatus))
	putString(metadata, "stream_type", string(d.DeliveryStreamType))
	putString(metadata, "version_id", aws.ToString(d.VersionId))
	putString(metadata, "region", region)

	if d.CreateTimestamp != nil {
		metadata["created_at"] = d.CreateTimestamp.UTC().Format(time.RFC3339)
	}
	if d.LastUpdateTimestamp != nil {
		metadata["last_updated_at"] = d.LastUpdateTimestamp.UTC().Format(time.RFC3339)
	}
	if d.DeliveryStreamEncryptionConfiguration != nil {
		putString(metadata, "encryption_status", string(d.DeliveryStreamEncryptionConfiguration.Status))
		putString(metadata, "encryption_key_type", string(d.DeliveryStreamEncryptionConfiguration.KeyType))
	}

	addSourceMetadata(metadata, d)

	metadata["destination_count"] = len(d.Destinations)
	if dest, ok := firstDestination(d); ok {
		metadata["destination_type"] = dest.Kind
		if s.config.IncludeDestinationConfig && len(dest.Settings) > 0 {
			metadata["destination"] = maskSecrets(dest.Settings)
		}
	}

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

	if region != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{
			Name: "Open in AWS Console",
			URL:  consoleURL(region, name),
		}}
	}

	return asset
}

// streamTags fetches the AWS tags on a stream and turns them into
// metadata. Missing tags are not worth failing the whole stream over.
func (s *Source) streamTags(ctx context.Context, name string) map[string]any {
	if s.config.AWSConfig == nil || !s.config.TagsToMetadata {
		return map[string]any{}
	}

	tags, err := listTags(ctx, s.client, name)
	if err != nil {
		log.Warn().Err(err).Str("stream", name).Msg("Failed to list delivery stream tags")
		return map[string]any{}
	}

	return pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tags)
}

// addSourceMetadata records where the stream reads its records from.
func addSourceMetadata(metadata map[string]any, d *types.DeliveryStreamDescription) {
	metadata["source_type"] = sourceType(d)

	if d.Source == nil {
		return
	}
	if kinesis := d.Source.KinesisStreamSourceDescription; kinesis != nil {
		putString(metadata, "source_kinesis_stream", kinesisStreamFromARN(aws.ToString(kinesis.KinesisStreamARN)))
	}
	if msk := d.Source.MSKSourceDescription; msk != nil {
		putString(metadata, "source_msk_cluster", mskClusterFromARN(aws.ToString(msk.MSKClusterARN)))
		putString(metadata, "source_msk_topic", aws.ToString(msk.TopicName))
	}
}

// sourceType names the kind of producer feeding the stream. The stream
// type is authoritative; the source description is only filled in for some
// of them.
func sourceType(d *types.DeliveryStreamDescription) string {
	switch d.DeliveryStreamType {
	case types.DeliveryStreamTypeKinesisStreamAsSource:
		return "kinesis"
	case types.DeliveryStreamTypeMSKAsSource:
		return "msk"
	case types.DeliveryStreamTypeDatabaseAsSource:
		return "database"
	default:
		return "direct_put"
	}
}

// firstDestination returns the destination a stream writes to. Firehose
// allows exactly one, but the API returns a list, so the first one that
// can be classified wins.
func firstDestination(d *types.DeliveryStreamDescription) (destination, bool) {
	for _, raw := range d.Destinations {
		if dest, ok := classifyDestination(raw); ok {
			return dest, true
		}
	}
	return destination{}, false
}

// streamLineage builds the edges between a stream and the systems it is
// wired to. Firehose creates none of those assets: every edge points at an
// identity another Marmot plugin produces, and the server drops the edge
// when that plugin has not catalogued the asset.
func streamLineage(d *types.DeliveryStreamDescription) []pluginsdk.LineageEdge {
	streamMRN := assetMRN(assetType, aws.ToString(d.DeliveryStreamName))

	var edges []pluginsdk.LineageEdge

	for _, source := range sourceTargets(d) {
		edges = append(edges, pluginsdk.LineageEdge{
			Source: mrn.New(source.Type, source.Provider, source.Name),
			Target: streamMRN,
			Type:   "FEEDS",
		})
	}

	for _, raw := range d.Destinations {
		dest, ok := classifyDestination(raw)
		if !ok {
			log.Debug().Str("stream", aws.ToString(d.DeliveryStreamName)).
				Msg("Skipping destination of an unrecognised kind")
			continue
		}
		for _, target := range dest.Targets {
			edges = append(edges, pluginsdk.LineageEdge{
				Source: streamMRN,
				Target: mrn.New(target.Type, target.Provider, target.Name),
				Type:   "PRODUCES",
			})
		}
	}

	return edges
}

// sourceTargets lists the assets that feed the stream.
func sourceTargets(d *types.DeliveryStreamDescription) []lineageTarget {
	if d.Source == nil {
		return nil
	}

	var targets []lineageTarget

	if kinesis := d.Source.KinesisStreamSourceDescription; kinesis != nil {
		if name := kinesisStreamFromARN(aws.ToString(kinesis.KinesisStreamARN)); name != "" {
			targets = append(targets, lineageTarget{Type: "Stream", Provider: "Kinesis", Name: name})
		}
	}

	if msk := d.Source.MSKSourceDescription; msk != nil {
		if topic := aws.ToString(msk.TopicName); topic != "" {
			targets = append(targets, lineageTarget{Type: "Topic", Provider: "Kafka", Name: topic})
		}
	}

	return targets
}

// consoleURL is the deep link to a stream in the AWS console.
func consoleURL(region, name string) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/firehose/home?region=%s#/details/%s", region, region, name)
}

// assetMRN is the single place a Firehose MRN is built. The asset pass and
// the lineage pass both go through it so the two can never drift into
// addressing the same stream differently. A delivery stream name is unique
// within a region, so the bare name is the identity.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
