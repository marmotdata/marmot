package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/anypointmq"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createAnypointMQDestination(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *anypointmq.ChannelBinding) asset.Asset {
	name := channelName
	if binding.Destination != "" {
		name = binding.Destination
	} else if channel.Address != "" {
		name = channel.Address
	}

	assetType := "Queue"
	if binding.DestinationType == "exchange" {
		assetType = "Exchange"
	} else if binding.DestinationType == "fifo-queue" {
		assetType = "FIFOQueue"
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Anypoint MQ %s for channel %s", assetType, channelName)
	}

	mrnValue := mrn.New(assetType, "AnypointMQ", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"destination_name": name,
		"destination_type": binding.DestinationType,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        assetType,
		Providers:   []string{"AnypointMQ"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "anypointmq",
			},
			Priority: 1,
		}},
	}
}
