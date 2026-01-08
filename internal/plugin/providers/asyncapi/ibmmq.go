package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/ibmmq"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createIBMMQAssets(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *ibmmq.ChannelBinding) []asset.Asset {
	var assets []asset.Asset

	// Create queue asset if queue binding is present
	if binding.Queue != nil && binding.Queue.ObjectName != "" {
		assets = append(assets, s.createIBMMQQueue(doc, channelName, channel, binding))
	}

	// Create topic asset if topic binding is present
	if binding.Topic != nil && (binding.Topic.String != "" || binding.Topic.ObjectName != "") {
		assets = append(assets, s.createIBMMQTopic(doc, channelName, channel, binding))
	}

	// If neither queue nor topic specified, create a generic queue based on destination type
	if len(assets) == 0 {
		if binding.DestinationType == "topic" {
			assets = append(assets, s.createIBMMQGenericTopic(doc, channelName, channel, binding))
		} else {
			assets = append(assets, s.createIBMMQGenericQueue(doc, channelName, channel, binding))
		}
	}

	return assets
}

func (s *Source) createIBMMQQueue(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *ibmmq.ChannelBinding) asset.Asset {
	name := binding.Queue.ObjectName

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("IBM MQ queue for channel %s", channelName)
	}

	mrnValue := mrn.New("Queue", "IBMMQ", name)

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

	if binding.Queue.IsPartitioned {
		metadata["is_partitioned"] = binding.Queue.IsPartitioned
	}

	if binding.Queue.Exclusive {
		metadata["exclusive"] = binding.Queue.Exclusive
	}

	if binding.MaxMsgLength > 0 {
		metadata["max_msg_length"] = binding.MaxMsgLength
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"IBMMQ"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "ibmmq",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createIBMMQTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *ibmmq.ChannelBinding) asset.Asset {
	name := binding.Topic.String
	if name == "" {
		name = binding.Topic.ObjectName
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("IBM MQ topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "IBMMQ", name)

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

	if binding.Topic.ObjectName != "" {
		metadata["topic_object_name"] = binding.Topic.ObjectName
	}

	if binding.Topic.DurablePermitted {
		metadata["durable_permitted"] = binding.Topic.DurablePermitted
	}

	if binding.Topic.LastMsgRetained {
		metadata["last_msg_retained"] = binding.Topic.LastMsgRetained
	}

	if binding.MaxMsgLength > 0 {
		metadata["max_msg_length"] = binding.MaxMsgLength
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"IBMMQ"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "ibmmq",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createIBMMQGenericQueue(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *ibmmq.ChannelBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("IBM MQ queue for channel %s", channelName)
	}

	mrnValue := mrn.New("Queue", "IBMMQ", name)

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

	if binding.MaxMsgLength > 0 {
		metadata["max_msg_length"] = binding.MaxMsgLength
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"IBMMQ"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "ibmmq",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createIBMMQGenericTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *ibmmq.ChannelBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("IBM MQ topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "IBMMQ", name)

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

	if binding.MaxMsgLength > 0 {
		metadata["max_msg_length"] = binding.MaxMsgLength
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"IBMMQ"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "ibmmq",
			},
			Priority: 1,
		}},
	}
}
