package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/amqp"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createAMQPAssets(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *amqp.ChannelBinding) []asset.Asset {
	var assets []asset.Asset

	if binding.Exchange != nil && binding.Exchange.Name != "" {
		exchangeAsset := s.createAMQPExchange(doc, channelName, channel, binding)
		assets = append(assets, exchangeAsset)
	}

	if binding.Queue != nil && binding.Queue.Name != "" {
		queueAsset := s.createAMQPQueue(doc, channelName, channel, binding)
		assets = append(assets, queueAsset)
	}

	return assets
}

func (s *Source) createAMQPQueue(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *amqp.ChannelBinding) asset.Asset {
	name := binding.Queue.Name

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("AMQP queue for channel %s", channelName)
	}

	mrnValue := mrn.New("Queue", "AMQP", name)

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

	if binding.Is != "" {
		metadata["binding_is"] = binding.Is
	}

	q := binding.Queue
	metadata["queue_durable"] = q.Durable
	metadata["queue_exclusive"] = q.Exclusive
	metadata["queue_auto_delete"] = q.AutoDelete

	if q.VHost != "" {
		metadata["queue_vhost"] = q.VHost
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"AMQP"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "amqp",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createAMQPExchange(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *amqp.ChannelBinding) asset.Asset {
	name := binding.Exchange.Name

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("AMQP exchange for channel %s", channelName)
	}

	mrnValue := mrn.New("Exchange", "AMQP", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"exchange_name":    name,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding.Is != "" {
		metadata["binding_is"] = binding.Is
	}

	e := binding.Exchange
	if e.Type != "" {
		metadata["exchange_type"] = e.Type
	}
	metadata["exchange_durable"] = e.Durable
	metadata["exchange_auto_delete"] = e.AutoDelete

	if e.VHost != "" {
		metadata["exchange_vhost"] = e.VHost
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Exchange",
		Providers:   []string{"AMQP"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "amqp",
			},
			Priority: 1,
		}},
	}
}
