package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/websockets"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createWebSocketChannel(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *websockets.ChannelBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("WebSocket channel: %s", channelName)
	}

	mrnValue := mrn.New("Channel", "WebSocket", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if binding.Method != "" {
		metadata["method"] = binding.Method
	}

	if binding.Query != nil {
		metadata["has_query_params"] = true
	}

	if binding.Headers != nil {
		metadata["has_headers"] = true
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Channel",
		Providers:   []string{"WebSocket"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "websockets",
			},
			Priority: 1,
		}},
	}
}
