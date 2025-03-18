package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/services/asset"
)

type SNS struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields        // Embedding SharedFields
	SNSFields           // Embedding SNSFields
}

type SQS struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields        // Embedding SharedFields
	SQSFields           // Embedding SQSFields
}

func (s *Source) createSNSTopic(spec *asyncapi2.Document, channelName string, binding *sns.ChannelBinding) asset.Asset {
	name := binding.Name
	if name == "" {
		name = channelName
	}

	description := fmt.Sprintf("SNS topic for channel %s", channelName)
	mrnValue := mrn.New("Topic", "SNS", name)

	// Initialize metadata map
	metadata := map[string]interface{}{
		"service_name":    spec.Info.Title,
		"service_version": spec.Info.Version,
		"description":     description,
		"topic_arn":       binding.Name,
	}

	if binding.Ordering != nil {
		metadata["ordering_type"] = binding.Ordering.Type
		metadata["content_deduplication"] = binding.Ordering.ContentBasedDeduplication
	}

	if binding.Tags != nil {
		for k, v := range binding.Tags {
			metadata["tag_"+k] = v
		}
	}

	// Process tags with interpolation
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

func (s *Source) createSQSQueue(spec *asyncapi2.Document, channelName string, binding *sqs.ChannelBinding) asset.Asset {
	name := ""
	if binding.Queue != nil {
		name = binding.Queue.Name
	}
	if name == "" {
		name = channelName
	}

	description := fmt.Sprintf("SQS queue for channel %s", channelName)
	mrnValue := mrn.New("Queue", "SQS", name)

	// Initialize metadata map
	metadata := map[string]interface{}{
		"service_name":    spec.Info.Title,
		"service_version": spec.Info.Version,
		"description":     description,
	}

	// Add SQS-specific fields
	if binding.Queue != nil {
		metadata["name"] = binding.Queue.Name
		metadata["fifo_queue"] = binding.Queue.FifoQueue
		metadata["deduplication_scope"] = binding.Queue.DeduplicationScope
		metadata["fifo_throughput_limit"] = binding.Queue.FifoThroughputLimit
		metadata["delivery_delay"] = binding.Queue.DeliveryDelay
		metadata["visibility_timeout"] = binding.Queue.VisibilityTimeout
		metadata["receive_message_wait_time"] = binding.Queue.ReceiveMessageWaitTime
		metadata["message_retention_period"] = binding.Queue.MessageRetentionPeriod

		if binding.Queue.RedrivePolicy != nil {
			metadata["dlq_name"] = binding.Queue.RedrivePolicy.DeadLetterQueue.Name
			if binding.Queue.RedrivePolicy.MaxReceiveCount != nil {
				metadata["max_receive_count"] = *binding.Queue.RedrivePolicy.MaxReceiveCount
			}
		}

		if binding.Queue.Tags != nil {
			for k, v := range binding.Queue.Tags {
				metadata["tag_"+k] = v
			}
		}
	}

	// Process tags with interpolation
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
