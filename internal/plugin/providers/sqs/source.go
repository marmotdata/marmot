// +marmot:name=SQS
// +marmot:description=This plugin discovers SQS queues from AWS accounts.
// +marmot:status=experimental
package sqs

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for SQS plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline"`

	DiscoverDLQ bool `json:"discover_dlq,omitempty" description:"Discover Dead Letter Queue relationships"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
credentials:
  region: "us-east-1" 
  id: "<aws-secret-id>"
  secret: "<aws-secret-key>"
tags:
  - "sns"
`

type Source struct {
	config *Config
	client *sqs.Client
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

	s.client = sqs.NewFromConfig(awsCfg)

	queues, err := s.discoverQueues(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering queues: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	queueArns := make(map[string]string)

	for _, queueURL := range queues {
		name := extractQueueName(queueURL)

		if config.AWSConfig != nil {
			if !plugin.ShouldIncludeResource(name, config.AWSConfig.Filter) {
				log.Debug().Str("queue", name).Msg("Skipping queue due to filter")
				continue
			}
		}

		asset, arn, err := s.createQueueAsset(ctx, queueURL)
		if err != nil {
			log.Warn().Err(err).Str("queue", queueURL).Msg("Failed to create asset for queue")
			continue
		}
		assets = append(assets, asset)
		queueArns[name] = arn
	}

	if s.config.DiscoverDLQ {
		dlqLineages, err := s.discoverDLQLineage(ctx, queues, queueArns)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover DLQ lineage")
		} else {
			lineages = append(lineages, dlqLineages...)
		}
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) discoverQueues(ctx context.Context) ([]string, error) {
	var queues []string
	var nextToken *string

	for {
		output, err := s.client.ListQueues(ctx, &sqs.ListQueuesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing queues: %w", err)
		}

		queues = append(queues, output.QueueUrls...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return queues, nil
}

func (s *Source) createQueueAsset(ctx context.Context, queueURL string) (asset.Asset, string, error) {
	attrs, err := s.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: &queueURL,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return asset.Asset{}, "", fmt.Errorf("getting queue attributes: %w", err)
	}

	metadata := make(map[string]interface{})
	if s.config.TagsToMetadata {
		tagsOutput, err := s.client.ListQueueTags(ctx, &sqs.ListQueueTagsInput{
			QueueUrl: &queueURL,
		})
		if err != nil {
			log.Warn().Err(err).Str("queue", queueURL).Msg("Failed to get queue tags")
		} else {
			tagMap := make(map[string]string)
			for key, value := range tagsOutput.Tags {
				tagMap[key] = value
			}
			metadata = plugin.ProcessAWSTags(s.config.TagsToMetadata, s.config.IncludeTags, tagMap)
		}
	}

	metadata["queue_arn"] = attrs.Attributes[string(types.QueueAttributeNameQueueArn)]
	metadata["visibility_timeout"] = attrs.Attributes[string(types.QueueAttributeNameVisibilityTimeout)]
	metadata["message_retention_period"] = attrs.Attributes[string(types.QueueAttributeNameMessageRetentionPeriod)]
	metadata["maximum_message_size"] = attrs.Attributes[string(types.QueueAttributeNameMaximumMessageSize)]
	metadata["delay_seconds"] = attrs.Attributes[string(types.QueueAttributeNameDelaySeconds)]
	metadata["receive_message_wait_time_seconds"] = attrs.Attributes[string(types.QueueAttributeNameReceiveMessageWaitTimeSeconds)]

	if fifoQueue, ok := attrs.Attributes[string(types.QueueAttributeNameFifoQueue)]; ok {
		metadata["fifo_queue"] = fifoQueue
		if contentDeduplication, ok := attrs.Attributes[string(types.QueueAttributeNameContentBasedDeduplication)]; ok {
			metadata["content_based_deduplication"] = contentDeduplication
		}
		if deduplicationScope, ok := attrs.Attributes[string(types.QueueAttributeNameDeduplicationScope)]; ok {
			metadata["deduplication_scope"] = deduplicationScope
		}
		if throughputLimit, ok := attrs.Attributes[string(types.QueueAttributeNameFifoThroughputLimit)]; ok {
			metadata["fifo_throughput_limit"] = throughputLimit
		}
	}

	if redrivePolicy, ok := attrs.Attributes[string(types.QueueAttributeNameRedrivePolicy)]; ok {
		metadata["redrive_policy"] = redrivePolicy
	}

	name := extractQueueName(queueURL)
	mrnValue := mrn.New("Queue", "SQS", name)
	description := fmt.Sprintf("SQS queue %s", name)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"SQS"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "SQS",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, attrs.Attributes[string(types.QueueAttributeNameQueueArn)], nil
}

func (s *Source) discoverDLQLineage(ctx context.Context, queues []string, queueArns map[string]string) ([]lineage.LineageEdge, error) {
	var lineages []lineage.LineageEdge

	for _, queueURL := range queues {
		attrs, err := s.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: &queueURL,
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameRedrivePolicy,
			},
		})
		if err != nil {
			log.Warn().Err(err).Str("queue", queueURL).Msg("Failed to get queue redrive policy")
			continue
		}

		if redrivePolicy, ok := attrs.Attributes[string(types.QueueAttributeNameRedrivePolicy)]; ok {
			var policy struct {
				DeadLetterTargetArn string `json:"deadLetterTargetArn"`
			}
			if err := json.Unmarshal([]byte(redrivePolicy), &policy); err != nil {
				log.Warn().Err(err).Str("queue", queueURL).Msg("Failed to parse redrive policy")
				continue
			}

			sourceName := extractQueueName(queueURL)
			targetName := extractQueueNameFromArn(policy.DeadLetterTargetArn)

			if _, ok := queueArns[sourceName]; ok {
				sourceMRN := mrn.New("Queue", "SQS", sourceName)
				targetMRN := mrn.New("Queue", "SQS", targetName)

				lineages = append(lineages, lineage.LineageEdge{
					Source: sourceMRN,
					Target: targetMRN,
					Type:   "DLQ",
				})
			}
		}
	}

	return lineages, nil
}

func extractQueueName(queueURL string) string {
	parts := strings.Split(queueURL, "/")
	return parts[len(parts)-1]
}

func extractQueueNameFromArn(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[len(parts)-1]
}
