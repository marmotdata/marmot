// Package kinesis discovers Kinesis Data Streams from AWS accounts.
package kinesis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact string every Kinesis asset carries in Providers.
// The OpenMetadata plugin projects its Kinesis services onto the same
// string so the two routes land on one asset.
const provider = "Kinesis"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "kinesis",
		Name:        "AWS Kinesis",
		Description: "Discover Kinesis Data Streams from AWS accounts",
		Icon:        "kinesis",
		Category:    "messaging",
		Status:      "experimental",
		Features:    []string{"Assets"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Kinesis plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

	IncludeConsumers bool `json:"include_consumers" description:"Whether to list the enhanced fan-out consumers registered on each stream" default:"true"`
	IncludeShards    bool `json:"include_shards" description:"Whether to list shards to count total and open shards per stream" default:"true"`
}

// Example configuration for the plugin
var _ = `
credentials:
  region: "us-east-1"
  id: "<aws-secret-id>"
  secret: "<aws-secret-key>"
include_consumers: true
include_shards: true
tags:
  - "kinesis"
`

// Source represents the Kinesis plugin.
type Source struct {
	config *Config
	client api
	// region is where the AWS client was configured to call. It only
	// matters for streams whose ARN does not name a region.
	region string
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// The AWS section stays nil when the config names none of its keys,
	// which is the normal shape for the default credential chain.
	if config.AWSConfig == nil {
		config.AWSConfig = &pluginsdk.AWSConfig{}
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	// The shared AWS config leaves tag metadata off unless asked for. A
	// stream's tags are usually the only ownership signal it has, so this
	// plugin turns them on unless explicitly disabled.
	if _, ok := rawConfig["tags_to_metadata"]; !ok {
		config.TagsToMetadata = true
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Kinesis Data Streams.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initClient(ctx, rawConfig); err != nil {
		return nil, err
	}

	log.Debug().Str("region", s.region).Msg("Starting Kinesis discovery")

	assets, err := s.discoverStreams(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering streams: %w", err)
	}

	log.Info().Int("assets", len(assets)).Msg("Kinesis discovery completed")

	return &pluginsdk.DiscoveryResult{Assets: assets}, nil
}

// discoverStreams lists every stream and builds an asset per stream. A
// stream that cannot be described is skipped with a warning; a listing
// that fails outright ends discovery, since nothing can be trusted.
func (s *Source) discoverStreams(ctx context.Context) ([]pluginsdk.Asset, error) {
	names, err := listStreamNames(ctx, s.client)
	if err != nil {
		return nil, err
	}

	var assets []pluginsdk.Asset
	for _, name := range names {
		asset, err := s.discoverStream(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("stream", name).Msg("Failed to create asset for stream")
			continue
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func (s *Source) initClient(ctx context.Context, rawConfig pluginsdk.RawConfig) error {
	awsConfig, err := pluginsdk.ExtractAWSConfig(rawConfig)
	if err != nil {
		return fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = kinesis.NewFromConfig(awsCfg)
	s.region = awsCfg.Region
	return nil
}

// discoverStream reads everything the plugin records about one stream.
// The summary is the one call that has to succeed; shards, consumers and
// tags each degrade to a warning so one denied permission does not hide
// the stream.
func (s *Source) discoverStream(ctx context.Context, name string) (pluginsdk.Asset, error) {
	summary, err := describeStream(ctx, s.client, name)
	if err != nil {
		return pluginsdk.Asset{}, err
	}

	info := streamInfo{summary: summary}

	if s.config.IncludeShards {
		shards, err := listShards(ctx, s.client, name)
		if err != nil {
			log.Warn().Err(err).Str("stream", name).Msg("Failed to list shards")
		} else {
			info.shards = shards
			info.hasShards = true
		}
	}

	if s.config.IncludeConsumers && summary.StreamARN != nil {
		consumers, err := listConsumers(ctx, s.client, *summary.StreamARN)
		if err != nil {
			log.Warn().Err(err).Str("stream", name).Msg("Failed to list consumers")
		} else {
			info.consumers = consumers
			info.hasConsumers = true
		}
	}

	if s.config.TagsToMetadata {
		tags, err := listTags(ctx, s.client, name)
		if err != nil {
			log.Warn().Err(err).Str("stream", name).Msg("Failed to list tags")
		} else {
			info.tags = tags
		}
	}

	return s.streamAsset(info), nil
}

// streamInfo is everything discovery learned about one stream. The has*
// flags separate "listed and found none" from "not listed", so an empty
// list is reported as zero while a skipped one is left out.
type streamInfo struct {
	summary      *types.StreamDescriptionSummary
	shards       []types.Shard
	hasShards    bool
	consumers    []types.Consumer
	hasConsumers bool
	tags         map[string]string
}

// streamAsset assembles the Stream asset for one discovered stream.
func (s *Source) streamAsset(info streamInfo) pluginsdk.Asset {
	name := ""
	if info.summary.StreamName != nil {
		name = *info.summary.StreamName
	}

	metadata := pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, info.tags)

	region := s.region
	if fromARN := regionFromARN(stringValue(info.summary.StreamARN)); fromARN != "" {
		region = fromARN
	}

	for key, value := range streamMetadata(info.summary) {
		metadata[key] = value
	}

	if info.hasShards {
		metadata["shard_count"] = len(info.shards)
		metadata["open_shard_count"] = countOpenShards(info.shards)
	}

	if info.hasConsumers {
		names := consumerNames(info.consumers)
		if len(names) > 0 {
			metadata["consumers"] = names
		}
		// The summary's own count is authoritative when the API returns
		// it; the listed consumers fill in when it does not.
		if _, ok := metadata["consumer_count"]; !ok {
			metadata["consumer_count"] = len(info.consumers)
		}
	}

	if region != "" {
		metadata["region"] = region
	}

	url := consoleURL(region, name)
	if url != "" {
		metadata["url"] = url
	}

	mrnValue := assetMRN("Stream", name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Stream",
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if url != "" {
		asset.ExternalLinks = []pluginsdk.AssetExternalLink{{Name: "Open in AWS Console", URL: url}}
	}

	// Kinesis has no description field of its own. A "description" tag
	// is the convention teams fall back on, so honour it when present.
	if description := strings.TrimSpace(info.tags["description"]); description != "" {
		asset.Description = &description
	}

	return asset
}

// streamMetadata maps the fields of a stream summary onto flat metadata
// keys. Values the API did not return are left out rather than emitted
// as zero, so a missing count is not mistaken for a count of zero.
func streamMetadata(summary *types.StreamDescriptionSummary) map[string]interface{} {
	metadata := make(map[string]interface{})
	if summary == nil {
		return metadata
	}

	if summary.StreamARN != nil {
		metadata["arn"] = *summary.StreamARN
	}
	if summary.StreamStatus != "" {
		metadata["status"] = string(summary.StreamStatus)
	}
	if summary.StreamModeDetails != nil && summary.StreamModeDetails.StreamMode != "" {
		metadata["stream_mode"] = string(summary.StreamModeDetails.StreamMode)
	}
	if summary.RetentionPeriodHours != nil {
		metadata["retention_hours"] = int(*summary.RetentionPeriodHours)
	}
	if summary.OpenShardCount != nil {
		metadata["open_shard_count"] = int(*summary.OpenShardCount)
	}
	if summary.ConsumerCount != nil {
		metadata["consumer_count"] = int(*summary.ConsumerCount)
	}
	if summary.EncryptionType != "" {
		metadata["encryption_type"] = string(summary.EncryptionType)
	}
	if summary.EncryptionType == types.EncryptionTypeKms && summary.KeyId != nil {
		metadata["kms_key_id"] = *summary.KeyId
	}
	if metrics := shardLevelMetrics(summary.EnhancedMonitoring); len(metrics) > 0 {
		metadata["enhanced_monitoring"] = metrics
	}
	if summary.StreamCreationTimestamp != nil {
		metadata["created_at"] = summary.StreamCreationTimestamp.UTC().Format(time.RFC3339)
	}

	return metadata
}

// shardLevelMetrics flattens the enhanced monitoring blocks into the
// metric names that are switched on.
func shardLevelMetrics(monitoring []types.EnhancedMetrics) []string {
	var names []string
	for _, block := range monitoring {
		for _, metric := range block.ShardLevelMetrics {
			names = append(names, string(metric))
		}
	}
	return names
}

// countOpenShards counts shards still accepting writes. A closed shard,
// left behind by a split or merge, has an ending sequence number.
func countOpenShards(shards []types.Shard) int {
	open := 0
	for _, shard := range shards {
		if isOpenShard(shard) {
			open++
		}
	}
	return open
}

func isOpenShard(shard types.Shard) bool {
	return shard.SequenceNumberRange == nil || shard.SequenceNumberRange.EndingSequenceNumber == nil
}

func consumerNames(consumers []types.Consumer) []string {
	var names []string
	for _, consumer := range consumers {
		if consumer.ConsumerName != nil && *consumer.ConsumerName != "" {
			names = append(names, *consumer.ConsumerName)
		}
	}
	return names
}

// regionFromARN reads the region out of a stream ARN
// (arn:aws:kinesis:REGION:ACCOUNT:stream/NAME). It returns "" for
// anything that does not have that shape.
func regionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[3]
}

// consoleURL deep-links to the stream's monitoring page in the AWS
// console. Both parts are needed for the link to resolve.
func consoleURL(region, name string) string {
	if region == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.console.aws.amazon.com/kinesis/home?region=%s#/streams/details/%s/monitoring", region, region, name)
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// assetMRN is the single place a Kinesis MRN is built. Stream names are
// unique per account and region, so the bare name is the identity, the
// same shape the OpenMetadata projection uses for Kinesis.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
