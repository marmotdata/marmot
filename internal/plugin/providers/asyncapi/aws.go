package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

type SNS struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields
	SNSFields
}

type SQS struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields
	SQSFields
}

func (s *Source) createSNSTopic(spec *asyncapi2.Document, channelName string, binding *sns.ChannelBinding) asset.Asset {
	name := binding.Name
	if name == "" {
		name = channelName
	}

	description := fmt.Sprintf("SNS topic for channel %s", channelName)
	mrnValue := mrn.New("Topic", "SNS", name)

	metadata := map[string]interface{}{
		"service_name":    spec.Info.Title,
		"service_version": spec.Info.Version,
		"description":     description,
		"binding_version": binding.BindingVersion,
	}

	if binding.Name != "" {
		metadata["topic_arn"] = binding.Name
	}

	if binding.Ordering != nil {
		if binding.Ordering.Type != "" {
			metadata["ordering_type"] = binding.Ordering.Type
		}
		metadata["content_deduplication"] = binding.Ordering.ContentBasedDeduplication
	}

	if binding.Tags != nil {
		for k, v := range binding.Tags {
			if v != "" {
				metadata["tag_"+k] = v
			}
		}
	}

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

	metadata := map[string]interface{}{
		"service_name":    spec.Info.Title,
		"service_version": spec.Info.Version,
		"description":     description,
		"binding_version": binding.BindingVersion,
	}

	if binding.Queue != nil {
		if binding.Queue.Name != "" {
			metadata["name"] = binding.Queue.Name
		}
		if binding.Queue.DeduplicationScope != "" {
			metadata["deduplication_scope"] = binding.Queue.DeduplicationScope
		}
		if binding.Queue.FifoThroughputLimit != "" {
			metadata["fifo_throughput_limit"] = binding.Queue.FifoThroughputLimit
		}

		metadata["fifo_queue"] = binding.Queue.FifoQueue
		metadata["delivery_delay"] = binding.Queue.DeliveryDelay
		metadata["visibility_timeout"] = binding.Queue.VisibilityTimeout
		metadata["receive_message_wait_time"] = binding.Queue.ReceiveMessageWaitTime
		metadata["message_retention_period"] = binding.Queue.MessageRetentionPeriod

		if binding.Queue.RedrivePolicy != nil {
			if binding.Queue.RedrivePolicy.DeadLetterQueue.Name != "" {
				metadata["dlq_name"] = binding.Queue.RedrivePolicy.DeadLetterQueue.Name
			}
			if binding.Queue.RedrivePolicy.MaxReceiveCount != nil {
				metadata["max_receive_count"] = *binding.Queue.RedrivePolicy.MaxReceiveCount
			}
		}

		if binding.Queue.Tags != nil {
			for k, v := range binding.Queue.Tags {
				if v != "" {
					metadata["tag_"+k] = v
				}
			}
		}
	}

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
