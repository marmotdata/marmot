package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func (s *Source) discoverTopics(ctx context.Context) ([]string, error) {
	metadata, err := s.admin.ListTopics(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") {
			return nil, fmt.Errorf("connection closed unexpectedly (EOF): this usually indicates an authentication failure, incorrect credentials, or network connectivity issues")
		}
		if strings.Contains(err.Error(), "timed out") {
			return nil, fmt.Errorf("connection timed out: check your network connectivity and firewall settings")
		}
		if strings.Contains(err.Error(), "authentication") {
			return nil, fmt.Errorf("authentication failed: check your username, password, and SASL mechanism")
		}
		return nil, fmt.Errorf("listing topics: %w", err)
	}

	var topics []string
	for topic := range metadata {
		topics = append(topics, topic)
	}

	return topics, nil
}

func (s *Source) createTopicAsset(ctx context.Context, topic string) (asset.Asset, error) {
	metadata := make(map[string]interface{})
	metadata["topic_name"] = topic

	topicDetails, err := s.getTopicDetails(ctx, topic)
	if err != nil {
		return asset.Asset{}, fmt.Errorf("getting topic details: %w", err)
	}

	if s.config.IncludePartitionInfo {
		metadata["partition_count"] = topicDetails.partitionCount
		metadata["replication_factor"] = topicDetails.replicationFactor
	}

	if s.config.IncludeTopicConfig {
		topicConfig, err := s.getTopicConfig(ctx, topic)
		if err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to get topic config")
		} else {
			for key, value := range topicConfig {
				// Convert Kafka config keys to our standardized metadata keys
				metadataKey := key
				if mappedKey, ok := kafkaConfigToMetadataMapping[key]; ok {
					metadataKey = mappedKey
				}
				metadata[metadataKey] = value
			}
		}
	}

	var schema map[string]string
	if s.schemaRegistry != nil {
		var err error
		schema, err = s.enrichWithSchemaRegistry(topic, metadata)
		if err != nil {
			log.Warn().Err(err).Str("topic", topic).Msg("Failed to get schema information")
		}
	}

	description := fmt.Sprintf("Kafka topic %s", topic)
	mrnValue := mrn.New("Topic", "Kafka", topic)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &topic,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Kafka"},
		Description: &description,
		Metadata:    metadata,
		Schema:      schema,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Kafka",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}, nil
}

type TopicDetails struct {
	partitionCount    int
	replicationFactor int
}

func (s *Source) getTopicDetails(ctx context.Context, topic string) (*TopicDetails, error) {
	metadata, err := s.admin.ListTopics(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("describing topic: %w", err)
	}

	topicMetadata, exists := metadata[topic]
	if !exists {
		return nil, fmt.Errorf("topic %s not found in metadata", topic)
	}

	partitionCount := len(topicMetadata.Partitions)

	// Get replication factor from first partition
	var replicationFactor int
	if partitionCount > 0 {
		replicationFactor = len(topicMetadata.Partitions[0].Replicas)
	}

	return &TopicDetails{
		partitionCount:    partitionCount,
		replicationFactor: replicationFactor,
	}, nil
}

func (s *Source) getTopicConfig(ctx context.Context, topic string) (map[string]string, error) {
	configs, err := s.admin.DescribeTopicConfigs(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("describing topic configs: %w", err)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no config found for topic %s", topic)
	}

	configMap := make(map[string]string)
	for _, config := range configs[0].Configs {
		if config.Source == kmsg.ConfigSourceDefaultConfig {
			continue
		}
		if config.Value != nil {
			configMap[config.Key] = *config.Value
		}
	}

	return configMap, nil
}

// Mapping of Kafka configuration keys to standardized metadata keys
var kafkaConfigToMetadataMapping = map[string]string{
	"retention.ms":        "retention_ms",
	"retention.bytes":     "retention_bytes",
	"cleanup.policy":      "cleanup_policy",
	"min.insync.replicas": "min_insync_replicas",
	"max.message.bytes":   "max_message_bytes",
	"segment.bytes":       "segment_bytes",
	"segment.ms":          "segment_ms",
	"delete.retention.ms": "delete_retention_ms",
}
