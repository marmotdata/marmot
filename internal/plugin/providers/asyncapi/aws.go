package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createSNSTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *sns.ChannelBinding) asset.Asset {
	name := channelName
	if binding.Name != "" {
		name = binding.Name
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("SNS topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "SNS", name)

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

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"SNS"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "sns",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createSQSQueue(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *sqs.ChannelBinding) asset.Asset {
	name := channelName
	if binding.Queue != nil && binding.Queue.Name != "" {
		name = binding.Queue.Name
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("SQS queue for channel %s", channelName)
	}

	mrnValue := mrn.New("Queue", "SQS", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"queue_name":       name,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding.Queue != nil {
		q := binding.Queue
		metadata["fifo_queue"] = q.FifoQueue

		if q.DeduplicationScope != "" {
			metadata["deduplication_scope"] = q.DeduplicationScope
		}
		if q.FifoThroughputLimit != "" {
			metadata["fifo_throughput_limit"] = q.FifoThroughputLimit
		}
		if q.DeliveryDelay > 0 {
			metadata["delivery_delay"] = q.DeliveryDelay
		}
		if q.VisibilityTimeout > 0 {
			metadata["visibility_timeout"] = q.VisibilityTimeout
		}
		if q.ReceiveMessageWaitTime > 0 {
			metadata["receive_message_wait_time"] = q.ReceiveMessageWaitTime
		}
		if q.MessageRetentionPeriod > 0 {
			metadata["message_retention_period"] = q.MessageRetentionPeriod
		}

		if q.RedrivePolicy != nil {
			if q.RedrivePolicy.DeadLetterQueue.Name != "" {
				metadata["dlq_name"] = q.RedrivePolicy.DeadLetterQueue.Name
			}
			if q.RedrivePolicy.DeadLetterQueue.ARN != "" {
				metadata["dlq_arn"] = q.RedrivePolicy.DeadLetterQueue.ARN
			}
			if q.RedrivePolicy.MaxReceiveCount != nil {
				metadata["max_receive_count"] = *q.RedrivePolicy.MaxReceiveCount
			}
		}

		if q.Tags != nil {
			for k, v := range q.Tags {
				if v != "" {
					metadata["tag_"+k] = v
				}
			}
		}
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"SQS"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "sqs",
			},
			Priority: 1,
		}},
	}
}
