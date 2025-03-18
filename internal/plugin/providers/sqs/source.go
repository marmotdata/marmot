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
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/services/lineage"
	"github.com/rs/zerolog/log"
)

// Config for SQS plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline" yaml:",inline"`

	// Whether to discover Dead Letter Queue relationships
	DiscoverDLQ bool `json:"discover_dlq,omitempty" yaml:"discover_dlq,omitempty" description:"Discover Dead Letter Queue relationships"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
credentials:
  region: "us-west-2"
  profile: "default"
  # Optional: manual credentials
  id: ""
  secret: ""
  token: ""
  # Optional: role assumption
  role: ""
  role_external_id: ""
tags_to_metadata: true
include_tags:
  - "Environment"
  - "Team"
  - "Cost-Center"
tags:
  - "sqs"
  - "aws"
discover_dlq: true
filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"
`

type Source struct {
	config *Config
	client *sqs.Client
}

// TODO: use YAML
func (s *Source) Validate(pluginConfig plugin.RawPluginConfig) error {
	configBytes, err := json.Marshal(pluginConfig)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if cfg.Credentials.Region == "" {
		return fmt.Errorf("region is required")
	}

	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	// Unmarshal the raw config into the Config struct
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	s.config = config

	// Initialize AWS config
	awsCfg, err := plugin.NewAWSConfig(ctx, pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	// Create SQS client
	s.client = sqs.NewFromConfig(awsCfg)

	// Discover SQS queues
	queues, err := s.discoverQueues(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering queues: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	queueArns := make(map[string]string) // Map of queue name to ARN for DLQ lineage

	// First pass: Create assets and build queue ARN map
	for _, queueURL := range queues {
		// Extract queue name for filtering
		name := extractQueueName(queueURL)

		// Apply filter
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

	// Second pass: Create DLQ lineage if enabled
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
	// Get queue attributes
	attrs, err := s.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: &queueURL,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		return asset.Asset{}, "", fmt.Errorf("getting queue attributes: %w", err)
	}

	// Convert AWS tags to metadata if enabled
	var metadata map[string]interface{}
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

	// Add standard attributes to metadata
	metadata["queue_arn"] = attrs.Attributes[string(types.QueueAttributeNameQueueArn)]
	metadata["visibility_timeout"] = attrs.Attributes[string(types.QueueAttributeNameVisibilityTimeout)]
	metadata["message_retention_period"] = attrs.Attributes[string(types.QueueAttributeNameMessageRetentionPeriod)]
	metadata["maximum_message_size"] = attrs.Attributes[string(types.QueueAttributeNameMaximumMessageSize)]
	metadata["delay_seconds"] = attrs.Attributes[string(types.QueueAttributeNameDelaySeconds)]
	metadata["receive_message_wait_time_seconds"] = attrs.Attributes[string(types.QueueAttributeNameReceiveMessageWaitTimeSeconds)]

	// FIFO queue attributes
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

	// Dead Letter Queue attributes
	if redrivePolicy, ok := attrs.Attributes[string(types.QueueAttributeNameRedrivePolicy)]; ok {
		metadata["redrive_policy"] = redrivePolicy
	}

	// Extract queue name from URL
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
