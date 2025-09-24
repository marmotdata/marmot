// +marmot:name=AsyncAPI
// +marmot:description=This plugin enables fetching data from AsyncAPI specifications.
// +marmot:status=experimental
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

	"github.com/charlie-haley/asyncapi-go"
	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/amqp"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/charlie-haley/asyncapi-go/spec"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig   `json:",inline"`
	SpecPath            string `json:"spec_path"`
	ResolveExternalDocs bool   `json:"resolve_external_docs,omitempty"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
spec_path: "/app/api-specs"
resolve_external_docs: true
tags:
  - "asyncapi"
  - "specifications"
`

type Source struct {
	config *Config
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if config.SpecPath == "" {
		return nil, fmt.Errorf("spec_path is required")
	}

	if _, err := os.Stat(config.SpecPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("spec path does not exist: %s", config.SpecPath)
	}

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

		data, err := os.ReadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to read AsyncAPI file")
			return nil
		}

		if !isAsyncAPI2x(data, path) {
			return nil
		}

		var doc spec.Document
		opts := asyncapi.ParseOptions{FilePath: config.SpecPath}
		if filepath.Ext(path) == ".json" {
			doc, err = asyncapi.ParseFromJSON(data, opts)
		} else {
			doc, err = asyncapi.ParseFromYAML(data, opts)
		}
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to parse AsyncAPI file")
			return nil
		}

		spec, ok := doc.(*asyncapi2.Document)
		if !ok {
			log.Warn().Str("path", path).Msg("Document is not AsyncAPI 2.x")
			return nil
		}

		serviceAsset := s.createServiceAsset(spec)
		serviceMRN := *serviceAsset.MRN
		s.addUniqueAsset(&assets, serviceAsset, seenAssets)

		for channelName, channel := range spec.Channels {
			if channel.Bindings == nil {
				continue
			}

			if kafkaBinding, err := asyncapi.ParseBindings[kafka.ChannelBinding](channel.Bindings, "kafka"); err == nil {
				kafkaAsset := s.createKafkaTopic(spec, channelName, kafkaBinding)
				kafkaMRN := *kafkaAsset.MRN
				s.addUniqueAsset(&assets, kafkaAsset, seenAssets)
				s.createLineageEdge(serviceMRN, kafkaMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
			}

			if snsBinding, err := asyncapi.ParseBindings[sns.ChannelBinding](channel.Bindings, "sns"); err == nil {
				snsAsset := s.createSNSTopic(spec, channelName, snsBinding)
				snsMRN := *snsAsset.MRN
				s.addUniqueAsset(&assets, snsAsset, seenAssets)
				s.createLineageEdge(serviceMRN, snsMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
			}

			if sqsBinding, err := asyncapi.ParseBindings[sqs.ChannelBinding](channel.Bindings, "sqs"); err == nil {
				if sqsBinding.Queue != nil {
					sqsAsset := s.createSQSQueue(spec, channelName, sqsBinding)
					sqsMRN := *sqsAsset.MRN
					s.addUniqueAsset(&assets, sqsAsset, seenAssets)
					s.createLineageEdge(serviceMRN, sqsMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
				}
			}

			if amqpBinding, err := asyncapi.ParseBindings[amqp.ChannelBinding](channel.Bindings, "amqp"); err == nil {
				var exchangeMRN string

				if amqpBinding.Exchange != nil {
					exchangeAsset := s.createAMQPExchange(spec, channelName, amqpBinding)
					exchangeMRN = *exchangeAsset.MRN
					s.addUniqueAsset(&assets, exchangeAsset, seenAssets)
					s.createLineageEdge(serviceMRN, exchangeMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
				}

				if amqpBinding.Queue != nil {
					queueAsset := s.createAMQPQueue(spec, channelName, amqpBinding)
					queueMRN := *queueAsset.MRN
					s.addUniqueAsset(&assets, queueAsset, seenAssets)

					if amqpBinding.Exchange == nil {
						s.createLineageEdge(serviceMRN, queueMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
					}

					if amqpBinding.Exchange != nil && exchangeMRN != "" {
						s.createLineageEdge(exchangeMRN, queueMRN, "ROUTES", seenAssets, seenEdges, &lineages)
					}
				}
			}
			if channel.Publish != nil && channel.Publish.Message != nil {
				s.attachSchemasToChannelAssets(spec, channelName, channel.Publish.Message, "PUBLISH", &assets, seenAssets)
			}

			if channel.Subscribe != nil && channel.Subscribe.Message != nil {
				s.attachSchemasToChannelAssets(spec, channelName, channel.Subscribe.Message, "SUBSCRIBE", &assets, seenAssets)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking spec path: %w", err)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func isAsyncAPI2x(data []byte, path string) bool {
	var doc map[string]interface{}
	var err error

	if filepath.Ext(path) == ".json" {
		err = json.Unmarshal(data, &doc)
	} else {
		err = yaml.Unmarshal(data, &doc)
	}

	if err != nil {
		return false
	}

	version, ok := doc["asyncapi"].(string)
	return ok && strings.HasPrefix(version, "2.")
}

func (s *Source) attachSchemasToChannelAssets(spec *asyncapi2.Document, channelName string, message *asyncapi2.Message, operationType string, assets *[]asset.Asset, seenAssets map[string]struct{}) {
	schemas := s.extractMessageSchemas(message)
	if len(schemas) == 0 {
		return
	}

	for i := range *assets {
		assetPtr := &(*assets)[i]
		if s.isChannelRelatedAsset(assetPtr, channelName) {
			s.attachSchemasToAsset(assetPtr, schemas, channelName, operationType)
		}
	}
}

func (s *Source) extractMessageSchemas(message *asyncapi2.Message) map[string]string {
	schemas := make(map[string]string)

	if message.Payload != nil {
		if payloadStr, err := s.convertSchemaToString(message.Payload); err == nil {
			schemas["payload"] = payloadStr
		}
	}

	if message.Headers != nil {
		if headersStr, err := s.convertSchemaToString(message.Headers); err == nil {
			schemas["headers"] = headersStr
		}
	}

	return schemas
}

func (s *Source) convertSchemaToString(schema interface{}) (string, error) {
	switch v := schema.(type) {
	case string:
		return v, nil
	case map[string]interface{}, []interface{}:
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes), nil
		}
	}
	return "", fmt.Errorf("unable to convert schema to string")
}

func (s *Source) attachSchemasToAsset(asset *asset.Asset, schemas map[string]string, channelName, operationType string) {
	if asset.Schema == nil {
		asset.Schema = make(map[string]string)
	}

	for schemaType, schemaContent := range schemas {
		key := schemaType
		if schemaType == "payload" {
			key = "message"
		}
		asset.Schema[key] = schemaContent
	}
}

func (s *Source) isChannelRelatedAsset(asset *asset.Asset, channelName string) bool {
	if asset.Metadata == nil {
		return false
	}

	if metaChannelName, exists := asset.Metadata["channel_name"]; exists {
		if str, ok := metaChannelName.(string); ok && str == channelName {
			return true
		}
	}

	if asset.Name != nil && strings.Contains(*asset.Name, channelName) {
		return true
	}

	return false
}

func (s *Source) createServiceAsset(spec *asyncapi2.Document) asset.Asset {
	serviceName := spec.Info.Title
	description := spec.Info.Description
	if description == "" {
		description = fmt.Sprintf("AsyncAPI service: %s", serviceName)
	}

	mrnValue := mrn.New("service", "asyncapi", serviceName)

	metadata := map[string]interface{}{
		"asyncapi_version": spec.AsyncAPI,
		"service_name":     serviceName,
		"service_version":  spec.Info.Version,
		"description":      description,
	}

	if spec.Servers != nil {
		for serverName, server := range spec.Servers {
			if server.Bindings != nil {
				for protocol, binding := range server.Bindings {
					key := fmt.Sprintf("server_%s_%s_binding", serverName, protocol)
					metadata[key] = binding
				}
			}
		}
	}

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &serviceName,
		MRN:         &mrnValue,
		Type:        "Service",
		Providers:   []string{"AsyncAPI"},
		Description: &description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "AsyncAPI",
			LastSyncAt: time.Now(),
			Properties: map[string]interface{}{
				"spec": map[string]interface{}{
					"version": spec.AsyncAPI,
					"info":    spec.Info,
				},
				"metadata": metadata,
			},
			Priority: 1,
		}},
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
			Str("service", newAsset.Providers[0]).
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

func determineEdgeType(channel *asyncapi2.Channel) string {
	if channel.Subscribe != nil {
		return "PRODUCES"
	}
	if channel.Publish != nil {
		return "CONSUMES"
	}
	return "UNKNOWN"
}

func isAsyncAPIFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}
