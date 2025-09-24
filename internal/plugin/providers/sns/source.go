// +marmot:name=SNS
// +marmot:description=This plugin discovers SNS topics from AWS accounts.
// +marmot:status=experimental
package sns

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for SNS plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
`

type Source struct {
	config *Config
	client *sns.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	s.config = config

	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	awsConfig, err := plugin.ExtractAWSConfig(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = sns.NewFromConfig(awsCfg)

	topics, err := s.discoverTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering topics: %w", err)
	}

	var assets []asset.Asset
	for _, topic := range topics {
		name := extractTopicName(*topic.TopicArn)

		if s.config.AWSConfig != nil && !plugin.ShouldIncludeResource(name, s.config.AWSConfig.Filter) {
			log.Debug().Str("topic", name).Msg("Skipping topic due to filter")
			continue
		}

		asset, err := s.createTopicAsset(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", *topic.TopicArn).Msg("Failed to create asset for topic")
			continue
		}
		assets = append(assets, asset)
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) discoverTopics(ctx context.Context) ([]types.Topic, error) {
	var topics []types.Topic
	paginator := sns.NewListTopicsPaginator(s.client, &sns.ListTopicsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing topics: %w", err)
		}
		topics = append(topics, output.Topics...)
	}

	return topics, nil
}

func (s *Source) createTopicAsset(ctx context.Context, topic types.Topic) (asset.Asset, error) {
	metadata := make(map[string]interface{})

	attrs, err := s.client.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: topic.TopicArn,
	})
	if err != nil {
		return asset.Asset{}, fmt.Errorf("getting topic attributes: %w", err)
	}

	if s.config.AWSConfig != nil && s.config.TagsToMetadata {
		tagsOutput, err := s.client.ListTagsForResource(ctx, &sns.ListTagsForResourceInput{
			ResourceArn: topic.TopicArn,
		})
		if err != nil {
			log.Warn().Err(err).Str("topic", *topic.TopicArn).Msg("Failed to get topic tags")
		} else {
			tagMap := make(map[string]string)
			for _, tag := range tagsOutput.Tags {
				tagMap[*tag.Key] = *tag.Value
			}
			metadata = plugin.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
		}
	}

	metadata["topic_arn"] = attrs.Attributes["TopicArn"]
	metadata["owner"] = attrs.Attributes["Owner"]
	metadata["policy"] = attrs.Attributes["Policy"]

	if displayName, ok := attrs.Attributes["DisplayName"]; ok {
		metadata["display_name"] = displayName
	}
	if subscriptionsPending, ok := attrs.Attributes["SubscriptionsPending"]; ok {
		metadata["subscriptions_pending"] = subscriptionsPending
	}
	if subscriptionsConfirmed, ok := attrs.Attributes["SubscriptionsConfirmed"]; ok {
		metadata["subscriptions_confirmed"] = subscriptionsConfirmed
	}

	name := extractTopicName(*topic.TopicArn)
	mrnValue := mrn.New("Topic", "SNS", name)
	description := fmt.Sprintf("SNS topic %s", name)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"SNS"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "SNS",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

func extractTopicName(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[len(parts)-1]
}
