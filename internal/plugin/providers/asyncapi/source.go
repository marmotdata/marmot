// +marmot:name=AsyncAPI
// +marmot:description=This plugin enables fetching data from AsyncAPI specifications.
// +marmot:status=experimental
package asyncapi

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charlie-haley/asyncapi-go"
	"github.com/charlie-haley/asyncapi-go/asyncapi2"
	"github.com/charlie-haley/asyncapi-go/bindings/kafka"
	"github.com/charlie-haley/asyncapi-go/bindings/sns"
	"github.com/charlie-haley/asyncapi-go/bindings/sqs"
	"github.com/charlie-haley/asyncapi-go/spec"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/services/lineage"
	"github.com/rs/zerolog/log"
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
spec_path: "./specs"
tags:
  - "asyncapi"
  - "api"
resolve_external_docs: true
`

type Source struct {
	config *Config
}

// TODO: use YAML
func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	if config.SpecPath == "" {
		return fmt.Errorf("spec_path is required")
	}

	if _, err := os.Stat(config.SpecPath); os.IsNotExist(err) {
		return fmt.Errorf("spec path does not exist: %s", config.SpecPath)
	}

	return nil
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

		var doc spec.Document
		if filepath.Ext(path) == ".json" {
			doc, err = asyncapi.ParseFromJSON(data)
		} else {
			doc, err = asyncapi.ParseFromYAML(data)
		}
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to parse AsyncAPI file")
			return nil
		}

		// Type assert to asyncapi2.Document
		spec, ok := doc.(*asyncapi2.Document)
		if !ok {
			log.Warn().Str("path", path).Msg("Document is not AsyncAPI 2.x")
			return nil
		}

		// Create service asset
		serviceAsset := s.createServiceAsset(spec)
		serviceMRN := *serviceAsset.MRN
		s.addUniqueAsset(&assets, serviceAsset, seenAssets)

		// Process channels
		for channelName, channel := range spec.Channels {
			if channel.Bindings == nil {
				continue
			}

			// Process Kafka binding
			if kafkaBinding, err := asyncapi.ParseBindings[kafka.ChannelBinding](channel.Bindings, "kafka"); err == nil {
				kafkaAsset := s.createKafkaTopic(spec, channelName, kafkaBinding)
				kafkaMRN := *kafkaAsset.MRN
				s.addUniqueAsset(&assets, kafkaAsset, seenAssets)
				s.createLineageEdge(serviceMRN, kafkaMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
			}

			// Process SNS binding
			if snsBinding, err := asyncapi.ParseBindings[sns.ChannelBinding](channel.Bindings, "sns"); err == nil {
				snsAsset := s.createSNSTopic(spec, channelName, snsBinding)
				snsMRN := *snsAsset.MRN
				s.addUniqueAsset(&assets, snsAsset, seenAssets)
				s.createLineageEdge(serviceMRN, snsMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
			}

			// Process SQS binding
			if sqsBinding, err := asyncapi.ParseBindings[sqs.ChannelBinding](channel.Bindings, "sqs"); err == nil {
				if sqsBinding.Queue != nil {
					sqsAsset := s.createSQSQueue(spec, channelName, sqsBinding)
					sqsMRN := *sqsAsset.MRN
					s.addUniqueAsset(&assets, sqsAsset, seenAssets)
					s.createLineageEdge(serviceMRN, sqsMRN, determineEdgeType(channel), seenAssets, seenEdges, &lineages)
				}
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

	componentsMap := make(map[string]interface{})
	if spec.Components != nil {
		if spec.Components.Messages != nil {
			componentsMap["messages"] = spec.Components.Messages
		}
		if spec.Components.Schemas != nil {
			componentsMap["schemas"] = spec.Components.Schemas
		}
		if spec.Components.Servers != nil {
			componentsMap["servers"] = spec.Components.Servers
		}
	}

	// Process tags with interpolation
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &serviceName,
		MRN:         &mrnValue,
		Type:        "Service",
		Providers:   []string{"AsyncAPI"},
		Description: &description,
		Schema:      componentsMap,
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
