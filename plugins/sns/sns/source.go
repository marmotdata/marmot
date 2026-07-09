// Package sns discovers SNS topics from AWS accounts.
package sns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "sns",
		Name:        "AWS SNS",
		Description: "Discover SNS topics from AWS accounts",
		Icon:        "sns",
		Category:    "messaging",
		Status:      "experimental",
		Features:    []string{"Assets"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for SNS plugin
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`
}

// Example configuration for the plugin
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

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	awsConfig, err := pluginsdk.ExtractAWSConfig(pluginConfig)
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

	var assets []pluginsdk.Asset
	for _, topic := range topics {
		asset, err := s.createTopicAsset(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", *topic.TopicArn).Msg("Failed to create asset for topic")
			continue
		}
		assets = append(assets, asset)
	}

	return &pluginsdk.DiscoveryResult{
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

func (s *Source) createTopicAsset(ctx context.Context, topic types.Topic) (pluginsdk.Asset, error) {
	metadata := make(map[string]interface{})

	attrs, err := s.client.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{
		TopicArn: topic.TopicArn,
	})
	if err != nil {
		return pluginsdk.Asset{}, fmt.Errorf("getting topic attributes: %w", err)
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
			metadata = pluginsdk.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
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

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Topic",
		Providers: []string{"SNS"},
		Metadata:  metadata,
		Tags:      processedTags,
		Sources: []pluginsdk.AssetSource{{
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
