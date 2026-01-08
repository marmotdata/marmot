package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/solace"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createSolaceAssets(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, opBinding *solace.OperationBinding) []asset.Asset {
	var assets []asset.Asset

	// Create assets based on operation binding destinations
	if opBinding != nil && len(opBinding.Destinations) > 0 {
		for _, dest := range opBinding.Destinations {
			if dest.Queue != nil && dest.Queue.Name != "" {
				assets = append(assets, s.createSolaceQueue(doc, channelName, channel, dest.Queue, opBinding))
			}
			// Topic destinations don't have a name field, they have topic subscriptions
			// Create a topic asset based on channel address when destination type is topic
			if dest.DestinationType == "topic" && dest.Topic != nil {
				assets = append(assets, s.createSolaceTopicFromDest(doc, channelName, channel, dest, opBinding))
			}
		}
	}

	// If no destinations, create a generic topic asset
	if len(assets) == 0 {
		assets = append(assets, s.createSolaceGenericTopic(doc, channelName, channel, opBinding))
	}

	return assets
}

func (s *Source) createSolaceQueue(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, queue *solace.QueueDestination, opBinding *solace.OperationBinding) asset.Asset {
	name := queue.Name

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Solace queue for channel %s", channelName)
	}

	mrnValue := mrn.New("Queue", "Solace", name)

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

	if len(queue.TopicSubscriptions) > 0 {
		metadata["topic_subscriptions"] = queue.TopicSubscriptions
	}

	if queue.AccessType != "" {
		metadata["access_type"] = queue.AccessType
	}

	if queue.MaxMsgSpoolSize > 0 {
		metadata["max_msg_spool_size"] = queue.MaxMsgSpoolSize
	}

	if queue.MaxTtl > 0 {
		metadata["max_ttl"] = queue.MaxTtl
	}

	if opBinding != nil && opBinding.BindingVersion != "" {
		metadata["binding_version"] = opBinding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Queue",
		Providers:   []string{"Solace"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "solace",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createSolaceTopicFromDest(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, dest *solace.Destination, opBinding *solace.OperationBinding) asset.Asset {
	// Use channel address or name as topic identifier
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Solace topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "Solace", name)

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

	if dest.Topic != nil && len(dest.Topic.TopicSubscriptions) > 0 {
		metadata["topic_subscriptions"] = dest.Topic.TopicSubscriptions
	}

	if dest.DeliveryMode != "" {
		metadata["delivery_mode"] = dest.DeliveryMode
	}

	if opBinding != nil && opBinding.BindingVersion != "" {
		metadata["binding_version"] = opBinding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Solace"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "solace",
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createSolaceGenericTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, opBinding *solace.OperationBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Solace topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "Solace", name)

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

	if opBinding != nil {
		// TimeToLive and Priority are interface{} types (can be Schema Object or Reference Object)
		if opBinding.TimeToLive != nil {
			metadata["time_to_live"] = opBinding.TimeToLive
		}
		if opBinding.Priority != nil {
			metadata["priority"] = opBinding.Priority
		}
		if opBinding.DMQEligible {
			metadata["dmq_eligible"] = opBinding.DMQEligible
		}
		if opBinding.BindingVersion != "" {
			metadata["binding_version"] = opBinding.BindingVersion
		}
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Solace"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "solace",
			},
			Priority: 1,
		}},
	}
}
