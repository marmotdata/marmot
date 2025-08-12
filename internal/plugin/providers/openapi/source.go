// +marmot:name=OpenAPI
// +marmot:description=This plugin discovers OpenAPI v3.0 specifications.
// +marmot:status=experimental
package openapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/rs/zerolog/log"
)

//go:generate go run ../../../docgen/cmd/main.go

type Source struct {
	config *Config
}

// Config for OpenAPI plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig 	`json:",inline"`
	IncludeOpenAPITags	bool `json:"include_openapi_tags" description:"Inlcude tags from OpenAPI specification" default:"false"`	
	SpecPath		string `json:"spec_path" description:"Path to the directory containing the OpenAPI specifications"`
}

const (
	typeService = "Service"
	openapiProvider = "OpenAPI"
)

// Example configuration for the plugin
// +marmot:example-config
var _ = `
spec_path: "/app/openapi-specs"
tags:
  - "openapi"
  - "specifications"
`

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

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	s.config = config

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	seenAssets := make(map[string]bool)

	err = filepath.Walk(config.SpecPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		if !isJSON(path) && !isYAML(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to read OpenAPI file")
			return nil
		}

		docConfig := datamodel.NewDocumentConfiguration()
		docConfig.IgnoreArrayCircularReferences = true
		docConfig.IgnorePolymorphicCircularReferences = true
		doc, err := libopenapi.NewDocumentWithConfiguration(data, docConfig)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to parse OpenAPI file")
			return nil
		}

		// TO-DO check if it is v3
		spec, parseErrors := doc.BuildV3Model()
		if parseErrors != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to build OpenAPI v3 model")
			return nil
		}

		serviceAsset := s.createServiceAsset(spec, config)
		addUniqueAsset(&assets, serviceAsset, seenAssets)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking spec path: %w", err)
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) createServiceAsset(spec *libopenapi.DocumentModel[v3.Document], config *Config) asset.Asset {
	serviceName := spec.Model.Info.Title
	description := spec.Model.Info.Description
	if description == "" {
		description = fmt.Sprintf("OpenAPI service: %s", serviceName)
	}

	mrnValue := mrn.New(typeService, "openapi", serviceName)
	openapiVersion := spec.Model.Version

	var servers []string
	if spec.Model.Servers != nil {
		for _, server := range spec.Model.Servers {
			servers = append(servers, server.URL)
		}
	}

	serviceFields := OpenAPIFields{
		Description: description,
		NumEndpoints: spec.Model.Paths.PathItems.Len(),
		OpenAPIVersion: openapiVersion,
		Servers: servers,
		ServiceName: serviceName,
		ServiceVersion: spec.Model.Info.Version,
		TermsOfService: spec.Model.Info.TermsOfService,
	}
	if spec.Model.Info.Contact != nil {
		serviceFields.ContactEmail = spec.Model.Info.Contact.Email
		serviceFields.ContactName = spec.Model.Info.Contact.Name
		serviceFields.ContactURL = spec.Model.Info.Contact.URL
	}
	if spec.Model.Info.License != nil {
		serviceFields.LicenseIdentifier = spec.Model.Info.License.Identifier
		serviceFields.LicenseName = spec.Model.Info.License.Name
		serviceFields.LicenseURL = spec.Model.Info.License.URL
	}
	if spec.Model.ExternalDocs != nil {
		serviceFields.ExternalDocs = spec.Model.ExternalDocs.URL
	}
	metadata := plugin.MapToMetadata(serviceFields)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)
	if config.IncludeOpenAPITags && spec.Model.Tags != nil {
		for _, tag := range spec.Model.Tags {
			processedTags = append(processedTags, tag.Name)
		}
	}

	externalLinks := []asset.ExternalLink{}
	if spec.Model.ExternalDocs != nil {
		name := spec.Model.ExternalDocs.Description
		if len(name) == 0 {
			name = spec.Model.ExternalDocs.URL
		}
		externalLinks = append(externalLinks, asset.ExternalLink{
			Name: name,
			URL: spec.Model.ExternalDocs.URL,
		})
	}

	return asset.Asset{
		Name: 		&serviceName,
		MRN: 		&mrnValue,
		Type:		typeService,
		Providers: 	[]string{openapiProvider},
		Description: 	&description,
		Metadata: 	metadata,
		Tags: 		processedTags,
		Sources: 	[]asset.AssetSource{},
		ExternalLinks: 	externalLinks,
	}
}

func addUniqueAsset(assets *[]asset.Asset, newAsset asset.Asset, seen map[string]bool) {
	if newAsset.MRN == nil {
		log.Warn().Interface("asset", newAsset).Msg("Asset has no MRN, skipping")
		return
	}

	if _, exists := seen[*newAsset.MRN]; exists {
		log.Warn().Interface("asset", newAsset).Msg("Asset already exists, skipping")
		return
	}

	*assets = append(*assets, newAsset)
	seen[*newAsset.MRN] = true
	log.Info().
		Str("mrn", *newAsset.MRN).
		Str("type", newAsset.Type).
		Str("service", newAsset.Providers[0]).
		Msg("Added new asset")
}

func isJSON(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".json"
}

func isYAML(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}
