package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/core/asset"
)

type KafkaTopic struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields        // Embedding SharedFields
	KafkaFields         // Embedding KafkaFields
}

func (s *Source) createKafkaTopic(spec *asyncapi2.Document, channelName string, binding *kafka.ChannelBinding) asset.Asset {
	name := binding.Topic
	if name == "" {
		name = channelName
	}

	description := fmt.Sprintf("Kafka topic for channel %s", channelName)
	mrnValue := mrn.New("Topic", "Kafka", name)

	// Set shared fields
	sharedFields := SharedFields{
		ServiceName:    spec.Info.Title,
		ServiceVersion: spec.Info.Version,
		Description:    description,
	}

	// Set Kafka fields
	kafkaFields := KafkaFields{
		Partitions: binding.Partitions,
		Replicas:   binding.Replicas,
	}

	if binding.TopicConfiguration != nil {
		kafkaFields.CleanupPolicies = binding.TopicConfiguration.CleanupPolicy
		kafkaFields.RetentionMs = binding.TopicConfiguration.RetentionMs
		kafkaFields.RetentionBytes = binding.TopicConfiguration.RetentionBytes
		kafkaFields.DeleteRetentionMs = binding.TopicConfiguration.DeleteRetentionMs
		kafkaFields.MaxMessageBytes = binding.TopicConfiguration.MaxMessageBytes
	}

	// Combine metadata
	metadata := plugin.MapToMetadata(sharedFields)
	for k, v := range plugin.MapToMetadata(kafkaFields) {
		metadata[k] = v
	}

	// Process tags with interpolation
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Kafka"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec": map[string]interface{}{
					"version": spec.AsyncAPI,
					"info":    spec.Info,
				},
				"metadata": metadata,
			},
			Priority: 1,
		}},
	}
}
