package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/googlepubsub"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createGooglePubSubTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *googlepubsub.ChannelBinding) asset.Asset {
	name := channelName
	if binding.Topic != "" {
		name = binding.Topic
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Google Pub/Sub topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "GooglePubSub", name)

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

	if binding.MessageRetentionDuration != "" {
		metadata["message_retention_duration"] = binding.MessageRetentionDuration
	}

	if binding.MessageStoragePolicy != nil {
		if len(binding.MessageStoragePolicy.AllowedPersistenceRegions) > 0 {
			metadata["allowed_regions"] = binding.MessageStoragePolicy.AllowedPersistenceRegions
		}
	}

	if binding.SchemaSettings != nil {
		if binding.SchemaSettings.Encoding != "" {
			metadata["schema_encoding"] = binding.SchemaSettings.Encoding
		}
		if binding.SchemaSettings.Name != "" {
			metadata["schema_name"] = binding.SchemaSettings.Name
		}
		if binding.SchemaSettings.FirstRevisionID != "" {
			metadata["schema_first_revision"] = binding.SchemaSettings.FirstRevisionID
		}
		if binding.SchemaSettings.LastRevisionID != "" {
			metadata["schema_last_revision"] = binding.SchemaSettings.LastRevisionID
		}
	}

	if binding.Labels != nil {
		for k, v := range binding.Labels {
			if v != "" {
				metadata["label_"+k] = v
			}
		}
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"GooglePubSub"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "googlepubsub",
			},
			Priority: 1,
		}},
	}
}
