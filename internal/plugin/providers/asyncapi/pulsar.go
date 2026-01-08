package asyncapi

import (
	"fmt"
	"time"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/pulsar"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
)

func (s *Source) createPulsarTopic(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel, binding *pulsar.ChannelBinding) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Pulsar topic for channel %s", channelName)
	}

	mrnValue := mrn.New("Topic", "Pulsar", name)

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

	if binding.Namespace != "" {
		metadata["namespace"] = binding.Namespace
	}

	if binding.Persistence != "" {
		metadata["persistence"] = binding.Persistence
	}

	if binding.Compaction > 0 {
		metadata["compaction"] = binding.Compaction
	}

	if len(binding.GeoReplication) > 0 {
		metadata["geo_replication"] = binding.GeoReplication
	}

	if binding.Retention != nil {
		if binding.Retention.Time > 0 {
			metadata["retention_time"] = binding.Retention.Time
		}
		if binding.Retention.Size > 0 {
			metadata["retention_size"] = binding.Retention.Size
		}
	}

	if binding.TTL > 0 {
		metadata["ttl"] = binding.TTL
	}

	if binding.Deduplication {
		metadata["deduplication"] = binding.Deduplication
	}

	if binding.BindingVersion != "" {
		metadata["binding_version"] = binding.BindingVersion
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Topic",
		Providers:   []string{"Pulsar"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
				"binding":      "pulsar",
			},
			Priority: 1,
		}},
	}
}
