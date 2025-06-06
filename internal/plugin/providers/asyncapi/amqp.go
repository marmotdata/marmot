package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/amqp"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createAMQPQueue(spec *asyncapi2.Document, channelName string, binding *amqp.ChannelBinding) asset.Asset {
	name := binding.Queue.Name

	// TODO: get desc from asyncapi
	description := fmt.Sprintf("AMQP queue for channel %s", channelName)
	mrnValue := mrn.New("Queue", "AMQP", name)

	sharedFields := SharedFields{
		ServiceName:    spec.Info.Title,
		ServiceVersion: spec.Info.Version,
		Description:    description,
	}

	amqpFields := AMQPFields{
		BindingIs:       binding.Is,
		QueueName:       binding.Queue.Name,
		QueueDurable:    binding.Queue.Durable,
		QueueExclusive:  binding.Queue.Exclusive,
		QueueAutoDelete: binding.Queue.AutoDelete,
	}

	if binding.Queue.VHost != "" {
		amqpFields.QueueVHost = binding.Queue.VHost
	}

	metadata := plugin.MapToMetadata(sharedFields)
	for k, v := range plugin.MapToMetadata(amqpFields) {
		if str, ok := v.(string); !ok || str != "" {
			metadata[k] = v
		}
	}
	metadata["binding_version"] = binding.BindingVersion

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	sourceProps := map[string]interface{}{
		"spec": map[string]interface{}{
			"version": spec.AsyncAPI,
		},
		"metadata": metadata,
	}

	if spec.Info != nil {
		sourceProps["spec"].(map[string]interface{})["info"] = spec.Info
	}

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"AMQP"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: sourceProps,
			Priority:   1,
		}},
	}
}

func (s *Source) createAMQPExchange(spec *asyncapi2.Document, channelName string, binding *amqp.ChannelBinding) asset.Asset {
	name := binding.Exchange.Name

	description := fmt.Sprintf("AMQP exchange for channel %s", channelName)
	mrnValue := mrn.New("Exchange", "AMQP", name)

	sharedFields := SharedFields{
		ServiceName:    spec.Info.Title,
		ServiceVersion: spec.Info.Version,
		Description:    description,
	}

	amqpFields := AMQPFields{
		BindingIs:          binding.Is,
		ExchangeName:       binding.Exchange.Name,
		ExchangeType:       binding.Exchange.Type,
		ExchangeDurable:    binding.Exchange.Durable,
		ExchangeAutoDelete: binding.Exchange.AutoDelete,
	}

	if binding.Exchange.VHost != "" {
		amqpFields.ExchangeVHost = binding.Exchange.VHost
	}

	metadata := plugin.MapToMetadata(sharedFields)
	for k, v := range plugin.MapToMetadata(amqpFields) {
		if str, ok := v.(string); !ok || str != "" {
			metadata[k] = v
		}
	}
	metadata["binding_version"] = binding.BindingVersion

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	sourceProps := map[string]interface{}{
		"spec": map[string]interface{}{
			"version": spec.AsyncAPI,
		},
		"metadata": metadata,
	}

	if spec.Info != nil {
		sourceProps["spec"].(map[string]interface{})["info"] = spec.Info
	}

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Exchange",
		Providers:   []string{"AMQP"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: sourceProps,
			Priority:   1,
		}},
	}
}
