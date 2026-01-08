package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/http"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createHTTPEndpoint(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, opBinding *http.OperationBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("HTTP endpoint for channel %s", channelName)
	}

	mrnValue := mrn.New("Endpoint", "HTTP", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
		"endpoint":         name,
	}

	if channel.Address != "" {
		metadata["channel_address"] = channel.Address
	}

	if opBinding != nil {
		if opBinding.Method != "" {
			metadata["method"] = opBinding.Method
		}

		if opBinding.Query != nil {
			metadata["has_query_params"] = true
		}

		if opBinding.BindingVersion != "" {
			metadata["binding_version"] = opBinding.BindingVersion
		}
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Endpoint",
		Providers:   []string{"HTTP"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "http",
			},
			Priority: 1,
		}},
	}
}
