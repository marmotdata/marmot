// +marmot:name=AsyncAPI
// +marmot:description=This plugin ingests metadata from AsyncAPI v3 specifications, discovering services, channels, and message schemas.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package asyncapi

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	asyncapi "github.com/charlie-haley/asyncapi-go"
	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/charlie-haley/asyncapi-go/bindings/amqp"
	"github.com/charlie-haley/asyncapi-go/bindings/anypointmq"
	"github.com/charlie-haley/asyncapi-go/bindings/googlepubsub"
	"github.com/charlie-haley/asyncapi-go/bindings/http"
	"github.com/charlie-haley/asyncapi-go/bindings/ibmmq"
	"github.com/charlie-haley/asyncapi-go/bindings/jms"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"
	"github.com/charlie-haley/asyncapi-go/bindings/mqtt"
	"github.com/charlie-haley/asyncapi-go/bindings/nats"
	"github.com/charlie-haley/asyncapi-go/bindings/pulsar"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/solace"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/charlie-haley/asyncapi-go/bindings/websockets"
	"github.com/charlie-haley/asyncapi-go/spec"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// Config for AsyncAPI plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	SpecPath    string `json:"spec_path" validate:"required" description:"Path to AsyncAPI spec file or directory containing specs"`
	Environment string `json:"environment,omitempty" description:"Environment name (e.g., production, staging)" default:"production"`

	DiscoverServices bool `json:"discover_services" description:"Create service assets from AsyncAPI info" default:"true"`
	DiscoverChannels bool `json:"discover_channels" description:"Create channel/topic assets from channels and bindings" default:"true"`
	DiscoverMessages bool `json:"discover_messages" description:"Attach message schemas to channel assets" default:"true"`

}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
spec_path: "/app/asyncapi-specs"
environment: "production"
discover_services: true
discover_channels: true
discover_messages: true
tags:
  - "asyncapi"
  - "event-driven"
filter:
  include:
    - "orders.*"
    - "users.*"
`

type Source struct {
	config *Config
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if config.Environment == "" {
		config.Environment = "production"
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	if _, err := os.Stat(config.SpecPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("spec path does not exist: %s", config.SpecPath)
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, rawConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	s.config = config

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	seenAssets := make(map[string]struct{})
	seenEdges := make(map[string]struct{})

	err = filepath.Walk(config.SpecPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if !isAsyncAPIFile(path) {
			return nil
		}

		specDoc, err := asyncapi.ParseFile(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to parse AsyncAPI file")
			return nil
		}

		doc, ok := specDoc.(*asyncapi3.Document)
		if !ok {
			log.Debug().Str("path", path).Str("version", specDoc.GetVersion()).Msg("Skipping non-v3 AsyncAPI file")
			return nil
		}

		log.Info().
			Str("path", path).
			Str("title", doc.Info.Title).
			Str("version", doc.AsyncAPI).
			Int("channels", len(doc.Channels)).
			Int("operations", len(doc.Operations)).
			Msg("Processing AsyncAPI v3 specification")

		var serviceMRN string
		if config.DiscoverServices {
			serviceAsset := s.createServiceAsset(doc)
			serviceMRN = *serviceAsset.MRN
			s.addUniqueAsset(&assets, serviceAsset, seenAssets)
		}

		if config.DiscoverChannels {
			for channelName, channel := range doc.Channels {
				if channel == nil {
					continue
				}

					channelAssets := s.createChannelAssets(doc, channelName, channel)
				for _, channelAsset := range channelAssets {
					s.addUniqueAsset(&assets, channelAsset, seenAssets)
				}
			}
		}

		// Create lineage edges based on operations
		// The ref resolver replaces $ref with resolved content, but the Reference struct
		// only has a Ref field, so we need to work around this by re-reading the raw spec
		// to extract the original channel references from operations.
		if serviceMRN != "" {
			opChannelMap := s.extractOperationChannelMappings(path)
			for opName, op := range doc.Operations {
				if op == nil {
					continue
				}

				// Get the channel name from our extracted mappings
				channelName, ok := opChannelMap[opName]
				if !ok {
					log.Debug().Str("operation", opName).Msg("Could not find channel mapping for operation")
					continue
				}

				channel := doc.Channels[channelName]
				if channel == nil {
					log.Debug().Str("operation", opName).Str("channel", channelName).Msg("Channel not found in document")
					continue
				}

				edgeType := s.determineEdgeType(op.Action)
				channelAssetMRNs := s.getChannelAssetMRNs(channelName, channel)

				for _, targetMRN := range channelAssetMRNs {
					if edgeType == "PRODUCES" {
						s.createLineageEdge(serviceMRN, targetMRN, edgeType, seenAssets, seenEdges, &lineages)
					} else {
						s.createLineageEdge(targetMRN, serviceMRN, edgeType, seenAssets, seenEdges, &lineages)
					}
				}

				log.Debug().
					Str("operation", opName).
					Str("channel", channelName).
					Str("action", string(op.Action)).
					Str("edgeType", edgeType).
					Int("targetMRNs", len(channelAssetMRNs)).
					Msg("Created lineage edges for operation")
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking spec path: %w", err)
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Msg("AsyncAPI discovery completed")

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) createServiceAsset(doc *asyncapi3.Document) asset.Asset {
	serviceName := doc.Info.Title
	description := doc.Info.Description
	if description == "" {
		description = fmt.Sprintf("AsyncAPI service: %s", serviceName)
	}

	mrnValue := mrn.New("Service", "AsyncAPI", serviceName)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     serviceName,
		"service_version":  doc.Info.Version,
		"environment":      s.config.Environment,
	}

	if doc.Info.Description != "" {
		metadata["description"] = doc.Info.Description
	}

	if doc.Info.Contact != nil {
		if doc.Info.Contact.Name != "" {
			metadata["contact_name"] = doc.Info.Contact.Name
		}
		if doc.Info.Contact.Email != "" {
			metadata["contact_email"] = doc.Info.Contact.Email
		}
		if doc.Info.Contact.URL != "" {
			metadata["contact_url"] = doc.Info.Contact.URL
		}
	}

	if doc.Info.License != nil {
		metadata["license"] = doc.Info.License.Name
		if doc.Info.License.URL != "" {
			metadata["license_url"] = doc.Info.License.URL
		}
	}

	if len(doc.Servers) > 0 {
		serverNames := make([]string, 0, len(doc.Servers))
		protocols := make(map[string]struct{})
		for name, server := range doc.Servers {
			serverNames = append(serverNames, name)
			if server.Protocol != "" {
				protocols[server.Protocol] = struct{}{}
			}
		}
		metadata["servers"] = serverNames

		protocolList := make([]string, 0, len(protocols))
		for p := range protocols {
			protocolList = append(protocolList, p)
		}
		if len(protocolList) > 0 {
			metadata["protocols"] = protocolList
		}
	}

	metadata["channel_count"] = len(doc.Channels)
	metadata["operation_count"] = len(doc.Operations)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &serviceName,
		MRN:         &mrnValue,
		Type:        "Service",
		Providers:   []string{"AsyncAPI"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
			},
			Priority: 1,
		}},
	}
}

func (s *Source) createChannelAssets(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel) []asset.Asset {
	var assets []asset.Asset

	if len(channel.Bindings) == 0 {
		asset := s.createGenericChannelAsset(doc, channelName, channel)
		assets = append(assets, asset)
		return assets
	}

	if channel.HasBinding("kafka") {
		binding, err := asyncapi.ParseBindings[kafka.ChannelBinding](channel.Bindings, "kafka")
		if err == nil {
			asset := s.createKafkaTopic(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("amqp") {
		binding, err := asyncapi.ParseBindings[amqp.ChannelBinding](channel.Bindings, "amqp")
		if err == nil {
			amqpAssets := s.createAMQPAssets(doc, channelName, channel, binding)
			for i := range amqpAssets {
				if s.config.DiscoverMessages {
					s.attachMessageSchemas(doc, channel, &amqpAssets[i])
				}
			}
			assets = append(assets, amqpAssets...)
		}
	}

	if channel.HasBinding("sns") {
		binding, err := asyncapi.ParseBindings[sns.ChannelBinding](channel.Bindings, "sns")
		if err == nil {
			asset := s.createSNSTopic(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("sqs") {
		binding, err := asyncapi.ParseBindings[sqs.ChannelBinding](channel.Bindings, "sqs")
		if err == nil {
			asset := s.createSQSQueue(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("googlepubsub") {
		binding, err := asyncapi.ParseBindings[googlepubsub.ChannelBinding](channel.Bindings, "googlepubsub")
		if err == nil {
			asset := s.createGooglePubSubTopic(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("mqtt") {
		binding, err := asyncapi.ParseBindings[mqtt.ChannelBinding](channel.Bindings, "mqtt")
		if err == nil {
			asset := s.createMQTTTopic(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("nats") {
		binding, _ := asyncapi.ParseBindings[nats.OperationBinding](channel.Bindings, "nats")
		asset := s.createNATSSubject(doc, channelName, channel, binding)
		if s.config.DiscoverMessages {
			s.attachMessageSchemas(doc, channel, &asset)
		}
		assets = append(assets, asset)
	}

	if channel.HasBinding("pulsar") {
		binding, err := asyncapi.ParseBindings[pulsar.ChannelBinding](channel.Bindings, "pulsar")
		if err == nil {
			asset := s.createPulsarTopic(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("solace") {
		binding, _ := asyncapi.ParseBindings[solace.OperationBinding](channel.Bindings, "solace")
		solaceAssets := s.createSolaceAssets(doc, channelName, channel, binding)
		for i := range solaceAssets {
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &solaceAssets[i])
			}
		}
		assets = append(assets, solaceAssets...)
	}

	if channel.HasBinding("ibmmq") {
		binding, err := asyncapi.ParseBindings[ibmmq.ChannelBinding](channel.Bindings, "ibmmq")
		if err == nil {
			ibmmqAssets := s.createIBMMQAssets(doc, channelName, channel, binding)
			for i := range ibmmqAssets {
				if s.config.DiscoverMessages {
					s.attachMessageSchemas(doc, channel, &ibmmqAssets[i])
				}
			}
			assets = append(assets, ibmmqAssets...)
		}
	}

	if channel.HasBinding("jms") {
		binding, err := asyncapi.ParseBindings[jms.ChannelBinding](channel.Bindings, "jms")
		if err == nil {
			asset := s.createJMSDestination(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("ws") {
		binding, err := asyncapi.ParseBindings[websockets.ChannelBinding](channel.Bindings, "ws")
		if err == nil {
			asset := s.createWebSocketChannel(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("anypointmq") {
		binding, err := asyncapi.ParseBindings[anypointmq.ChannelBinding](channel.Bindings, "anypointmq")
		if err == nil {
			asset := s.createAnypointMQDestination(doc, channelName, channel, binding)
			if s.config.DiscoverMessages {
				s.attachMessageSchemas(doc, channel, &asset)
			}
			assets = append(assets, asset)
		}
	}

	if channel.HasBinding("http") {
		binding, _ := asyncapi.ParseBindings[http.OperationBinding](channel.Bindings, "http")
		asset := s.createHTTPEndpoint(doc, channelName, channel, binding)
		if s.config.DiscoverMessages {
			s.attachMessageSchemas(doc, channel, &asset)
		}
		assets = append(assets, asset)
	}

	return assets
}

func (s *Source) createGenericChannelAsset(doc *asyncapi3.Document, channelName string, channel *asyncapi3.Channel) asset.Asset {
	name := channelName
	if channel.Address != "" {
		name = channel.Address
	}

	description := channel.Description
	if description == "" {
		description = fmt.Sprintf("Channel: %s", channelName)
	}

	mrnValue := mrn.New("Channel", "AsyncAPI", name)

	metadata := map[string]interface{}{
		"asyncapi_version": doc.AsyncAPI,
		"service_name":     doc.Info.Title,
		"service_version":  doc.Info.Version,
		"channel_name":     channelName,
		"environment":      s.config.Environment,
	}

	if channel.Address != "" {
		metadata["address"] = channel.Address
	}

	if channel.Title != "" {
		metadata["title"] = channel.Title
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	a := asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Channel",
		Providers:   []string{"AsyncAPI"},
		Description: &description,
		Metadata:    s.cleanMetadata(metadata),
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec_version": doc.AsyncAPI,
				"channel":      channelName,
			},
			Priority: 1,
		}},
	}

	if s.config.DiscoverMessages {
		s.attachMessageSchemas(doc, channel, &a)
	}

	return a
}

func (s *Source) attachMessageSchemas(doc *asyncapi3.Document, channel *asyncapi3.Channel, a *asset.Asset) {
	if len(channel.Messages) == 0 {
		return
	}

	schemas := make(map[string]string)

	for msgName, msg := range channel.Messages {
		if msg == nil {
			continue
		}

		if msg.Payload != nil {
			schemaKey := fmt.Sprintf("%s_payload", msgName)
			if data, err := json.Marshal(msg.Payload); err == nil {
				schemas[schemaKey] = string(data)
			}
		}

		if msg.Headers != nil {
			schemaKey := fmt.Sprintf("%s_headers", msgName)
			if data, err := json.Marshal(msg.Headers); err == nil {
				schemas[schemaKey] = string(data)
			}
		}
	}

	if len(schemas) > 0 {
		if a.Schema == nil {
			a.Schema = make(map[string]string)
		}
		for k, v := range schemas {
			a.Schema[k] = v
		}
	}
}

func (s *Source) getChannelAssetMRNs(channelName string, channel *asyncapi3.Channel) []string {
	var mrns []string

	if len(channel.Bindings) == 0 {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Channel", "AsyncAPI", name))
		return mrns
	}

	if channel.HasBinding("kafka") {
		binding, _ := asyncapi.ParseBindings[kafka.ChannelBinding](channel.Bindings, "kafka")
		name := channelName
		if binding != nil && binding.Topic != "" {
			name = binding.Topic
		}
		mrns = append(mrns, mrn.New("Topic", "Kafka", name))
	}

	if channel.HasBinding("amqp") {
		binding, _ := asyncapi.ParseBindings[amqp.ChannelBinding](channel.Bindings, "amqp")
		if binding != nil {
			if binding.Exchange != nil && binding.Exchange.Name != "" {
				mrns = append(mrns, mrn.New("Exchange", "AMQP", binding.Exchange.Name))
			}
			if binding.Queue != nil && binding.Queue.Name != "" {
				mrns = append(mrns, mrn.New("Queue", "AMQP", binding.Queue.Name))
			}
		}
	}

	if channel.HasBinding("sns") {
		binding, _ := asyncapi.ParseBindings[sns.ChannelBinding](channel.Bindings, "sns")
		name := channelName
		if binding != nil && binding.Name != "" {
			name = binding.Name
		}
		mrns = append(mrns, mrn.New("Topic", "SNS", name))
	}

	if channel.HasBinding("sqs") {
		binding, _ := asyncapi.ParseBindings[sqs.ChannelBinding](channel.Bindings, "sqs")
		name := channelName
		if binding != nil && binding.Queue != nil && binding.Queue.Name != "" {
			name = binding.Queue.Name
		}
		mrns = append(mrns, mrn.New("Queue", "SQS", name))
	}

	if channel.HasBinding("googlepubsub") {
		binding, _ := asyncapi.ParseBindings[googlepubsub.ChannelBinding](channel.Bindings, "googlepubsub")
		name := channelName
		if binding != nil && binding.Topic != "" {
			name = binding.Topic
		}
		mrns = append(mrns, mrn.New("Topic", "GooglePubSub", name))
	}

	if channel.HasBinding("mqtt") {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Topic", "MQTT", name))
	}

	if channel.HasBinding("nats") {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Subject", "NATS", name))
	}

	if channel.HasBinding("pulsar") {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Topic", "Pulsar", name))
	}

	if channel.HasBinding("solace") {
		binding, _ := asyncapi.ParseBindings[solace.OperationBinding](channel.Bindings, "solace")
		if binding != nil && len(binding.Destinations) > 0 {
			for _, dest := range binding.Destinations {
				if dest.Queue != nil && dest.Queue.Name != "" {
					mrns = append(mrns, mrn.New("Queue", "Solace", dest.Queue.Name))
				}
				// Topic destinations use channel address as identifier
				if dest.DestinationType == "topic" && dest.Topic != nil {
					name := channelName
					if channel.Address != "" {
						name = channel.Address
					}
					mrns = append(mrns, mrn.New("Topic", "Solace", name))
				}
			}
		} else {
			name := channelName
			if channel.Address != "" {
				name = channel.Address
			}
			mrns = append(mrns, mrn.New("Topic", "Solace", name))
		}
	}

	if channel.HasBinding("ibmmq") {
		binding, _ := asyncapi.ParseBindings[ibmmq.ChannelBinding](channel.Bindings, "ibmmq")
		if binding != nil {
			if binding.Queue != nil && binding.Queue.ObjectName != "" {
				mrns = append(mrns, mrn.New("Queue", "IBMMQ", binding.Queue.ObjectName))
			}
			if binding.Topic != nil && (binding.Topic.String != "" || binding.Topic.ObjectName != "") {
				name := binding.Topic.String
				if name == "" {
					name = binding.Topic.ObjectName
				}
				mrns = append(mrns, mrn.New("Topic", "IBMMQ", name))
			}
			if binding.Queue == nil && binding.Topic == nil {
				name := channelName
				if channel.Address != "" {
					name = channel.Address
				}
				if binding.DestinationType == "topic" {
					mrns = append(mrns, mrn.New("Topic", "IBMMQ", name))
				} else {
					mrns = append(mrns, mrn.New("Queue", "IBMMQ", name))
				}
			}
		}
	}

	if channel.HasBinding("jms") {
		binding, _ := asyncapi.ParseBindings[jms.ChannelBinding](channel.Bindings, "jms")
		name := channelName
		if binding != nil && binding.Destination != "" {
			name = binding.Destination
		} else if channel.Address != "" {
			name = channel.Address
		}
		assetType := "Queue"
		if binding != nil && binding.DestinationType == "topic" {
			assetType = "Topic"
		}
		mrns = append(mrns, mrn.New(assetType, "JMS", name))
	}

	if channel.HasBinding("ws") {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Channel", "WebSocket", name))
	}

	if channel.HasBinding("anypointmq") {
		binding, _ := asyncapi.ParseBindings[anypointmq.ChannelBinding](channel.Bindings, "anypointmq")
		name := channelName
		if binding != nil && binding.Destination != "" {
			name = binding.Destination
		} else if channel.Address != "" {
			name = channel.Address
		}
		assetType := "Queue"
		if binding != nil {
			if binding.DestinationType == "exchange" {
				assetType = "Exchange"
			} else if binding.DestinationType == "fifo-queue" {
				assetType = "FIFOQueue"
			}
		}
		mrns = append(mrns, mrn.New(assetType, "AnypointMQ", name))
	}

	if channel.HasBinding("http") {
		name := channelName
		if channel.Address != "" {
			name = channel.Address
		}
		mrns = append(mrns, mrn.New("Endpoint", "HTTP", name))
	}

	return mrns
}

// extractOperationChannelMappings reads the raw spec file to extract the original
// $ref mappings from operations to channels. This is necessary because the ref resolver
// replaces $ref objects with resolved content, but the Reference struct only has a Ref
// field, so the channel data gets lost during JSON unmarshaling.
func (s *Source) extractOperationChannelMappings(specPath string) map[string]string {
	result := make(map[string]string)

	data, err := os.ReadFile(specPath)
	if err != nil {
		log.Debug().Err(err).Str("path", specPath).Msg("Failed to read spec file for channel mappings")
		return result
	}

	// Parse as generic map to access raw structure
	var rawDoc map[string]interface{}
	if err := json.Unmarshal(data, &rawDoc); err != nil {
		// Try YAML
		yamlData, err := yaml.YAMLToJSON(data)
		if err != nil {
			log.Debug().Err(err).Str("path", specPath).Msg("Failed to parse spec file")
			return result
		}
		if err := json.Unmarshal(yamlData, &rawDoc); err != nil {
			log.Debug().Err(err).Str("path", specPath).Msg("Failed to parse converted YAML")
			return result
		}
	}

	operations, ok := rawDoc["operations"].(map[string]interface{})
	if !ok {
		return result
	}

	for opName, opData := range operations {
		opMap, ok := opData.(map[string]interface{})
		if !ok {
			continue
		}

		channelData, ok := opMap["channel"].(map[string]interface{})
		if !ok {
			continue
		}

		ref, ok := channelData["$ref"].(string)
		if !ok {
			continue
		}

		// Extract channel name from ref like "#/channels/userCreated"
		channelName := extractChannelNameFromRef(ref)
		if channelName != "" {
			result[opName] = channelName
		}
	}

	return result
}

// extractChannelNameFromRef extracts the channel name from a $ref string
func extractChannelNameFromRef(ref string) string {
	const prefix = "#/channels/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	return ref[len(prefix):]
}

func (s *Source) determineEdgeType(action spec.Action) string {
	switch action {
	case spec.Send:
		return "PRODUCES"
	case spec.Receive:
		return "CONSUMES"
	default:
		return "USES"
	}
}

func (s *Source) addUniqueAsset(assets *[]asset.Asset, newAsset asset.Asset, seenAssets map[string]struct{}) {
	if newAsset.MRN == nil {
		log.Warn().Interface("asset", newAsset).Msg("Asset has no MRN, skipping")
		return
	}

	if _, exists := seenAssets[*newAsset.MRN]; !exists {
		*assets = append(*assets, newAsset)
		seenAssets[*newAsset.MRN] = struct{}{}
		log.Debug().
			Str("mrn", *newAsset.MRN).
			Str("type", newAsset.Type).
			Msg("Added new asset")
	}
}

func (s *Source) createLineageEdge(sourceMRN, targetMRN, edgeType string,
	seenAssets, seenEdges map[string]struct{}, lineageEdges *[]lineage.LineageEdge) {

	if _, sourceExists := seenAssets[sourceMRN]; !sourceExists {
		return
	}
	if _, targetExists := seenAssets[targetMRN]; !targetExists {
		return
	}

	edgeKey := fmt.Sprintf("%s->%s", sourceMRN, targetMRN)
	if _, exists := seenEdges[edgeKey]; !exists {
		*lineageEdges = append(*lineageEdges, lineage.LineageEdge{
			Source: sourceMRN,
			Target: targetMRN,
			Type:   edgeType,
		})
		seenEdges[edgeKey] = struct{}{}
		log.Debug().
			Str("source", sourceMRN).
			Str("target", targetMRN).
			Str("type", edgeType).
			Msg("Added new lineage edge")
	}
}

func (s *Source) cleanMetadata(metadata map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range metadata {
		if v == nil {
			continue
		}
		if str, ok := v.(string); ok && str == "" {
			continue
		}
		if slice, ok := v.([]interface{}); ok && len(slice) == 0 {
			continue
		}
		if m, ok := v.(map[string]interface{}); ok && len(m) == 0 {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

func isAsyncAPIFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "asyncapi",
		Name:        "AsyncAPI",
		Description: "Discover metadata from AsyncAPI v3 specifications including services, channels, and message schemas",
		Icon:        "asyncapi",
		Category:    "api",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register AsyncAPI plugin")
	}
}
