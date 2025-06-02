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

type AMQPQueue struct {
	ResourceName string `json:"resourceName"`
	DisplayName  string `json:"displayName"`
	SharedFields
	AMQPFields
}

func (s *Source) createAMQPQueue(spec *asyncapi2.Document, channelName string, binding *amqp.ChannelBinding) asset.Asset {
	name := ""
	if binding.Exchange != nil && binding.Exchange.Name != "" {
		name = binding.Exchange.Name
	} else if binding.Queue != nil && binding.Queue.Name != "" {
		name = binding.Queue.Name
	}

	// TODO: get desc from asyncapi
	description := fmt.Sprintf("AMQP queue for channel %s", channelName)
	mrnValue := mrn.New("Queue", "AMQP", name)

	sharedFields := SharedFields{
		ServiceName:    spec.Info.Title,
		ServiceVersion: spec.Info.Version,
		Description:    description,
	}

	amqpFields := AMQPFields{
		BindingIs: binding.Is,
	}

	if binding.Exchange != nil {
		amqpFields.ExchangeName = binding.Exchange.Name
		amqpFields.ExchangeType = binding.Exchange.Type
		amqpFields.ExchangeDurable = binding.Exchange.Durable
		amqpFields.ExchangeAutoDelete = binding.Exchange.AutoDelete
		amqpFields.QueueVHost = binding.Exchange.VHost
	}

	if binding.Queue != nil {
		amqpFields.QueueName = binding.Queue.Name
		amqpFields.QueueDurable = binding.Queue.Durable
		amqpFields.QueueExclusive = binding.Queue.Exclusive
		amqpFields.QueueAutoDelete = binding.Queue.AutoDelete
		amqpFields.QueueVHost = binding.Queue.VHost
	}

	metadata := plugin.MapToMetadata(sharedFields)
	for k, v := range plugin.MapToMetadata(amqpFields) {
		metadata[k] = v
	}

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
