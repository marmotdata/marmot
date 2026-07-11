package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/nats"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

func (s *Source) createNATSSubject(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *nats.OperationBinding) pluginsdk.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("NATS subject for channel %s", channelName)
	}

	mrnValue := mrn.New("Subject", "NATS", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"subject_name":     name,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding != nil && binding.Queue != "" {
		metadata["queue_group"] = binding.Queue
	}

	if binding != nil && binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

	return pluginsdk.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Subject",
		Providers:   []string{"NATS"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []pluginsdk.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "nats",
			},
			Priority: 1,
		}},
	}
}
