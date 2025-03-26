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

// AMQPFields represents AMQP-specific metadata fields
// +marmot:metadata
type AMQPFields struct {
	BindingIs          string `json:"binding_is" description:"AMQP binding type (queue or routingKey)"`
	ExchangeName       string `json:"exchange_name" description:"Exchange name"`
	ExchangeType       string `json:"exchange_type" description:"Exchange type (topic, fanout, direct, etc.)"`
	ExchangeDurable    bool   `json:"exchange_durable" description:"Exchange durability flag"`
	ExchangeAutoDelete bool   `json:"exchange_auto_delete" description:"Exchange auto delete flag"`
	QueueName          string `json:"queue_name" description:"Queue name"`
	QueueVHost         string `json:"queue_vhost" description:"Queue virtual host"`
	QueueDurable       bool   `json:"queue_durable" description:"Queue durability flag"`
	QueueExclusive     bool   `json:"queue_exclusive" description:"Queue exclusivity flag"`
	QueueAutoDelete    bool   `json:"queue_auto_delete" description:"Queue auto delete flag"`
}

func (s *Source) createAMQPQueue(spec *asyncapi2.Document, channelName string, binding *amqp.ChannelBinding) asset.Asset {
	name := channelName
	if binding.Queue != nil && binding.Queue.Name != "" {
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

	// Create metadata map from structs
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
