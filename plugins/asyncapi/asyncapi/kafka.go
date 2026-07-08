package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

func (s *Source) createKafkaTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *kafka.ChannelBinding) pluginsdk.Asset {
	name := channelName
	if binding.Topic != "" {
		name = binding.Topic
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Kafka topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "Kafka", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"topic_name":       name,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding.Partitions > 0 {
		metadata["partitions"] = binding.Partitions
	}
	if binding.Replicas > 0 {
		metadata["replicas"] = binding.Replicas
	}

	if binding.TopicConfiguration != nil {
		tc := binding.TopicConfiguration
		if len(tc.CleanupPolicy) > 0 {
			metadata["cleanup_policy"] = tc.CleanupPolicy
		}
		if tc.RetentionMs > 0 {
			metadata["retention_ms"] = tc.RetentionMs
		}
		if tc.RetentionBytes > 0 {
			metadata["retention_bytes"] = tc.RetentionBytes
		}
		if tc.DeleteRetentionMs > 0 {
			metadata["delete_retention_ms"] = tc.DeleteRetentionMs
		}
		if tc.MaxMessageBytes > 0 {
			metadata["max_message_bytes"] = tc.MaxMessageBytes
		}
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Kafka"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "kafka",
			},
			Priority: 1,
		}},
	}
}
