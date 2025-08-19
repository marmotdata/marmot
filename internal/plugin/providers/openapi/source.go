// +marmot:name=OpenAPI
// +marmot:description=This plugin discovers OpenAPI v3 specifications.
// +marmot:status=experimental
package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	SpecPath		string `json:"spec_path" description:"Path to the directory containing the OpenAPI specifications"`
}

const (
	typeEndpoint = "Endpoint"
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

		openapiVersion := doc.GetSpecInfo().VersionNumeric
		if openapiVersion < 3 {
			log.Warn().Str("path", path).Msg(fmt.Sprintf("Unsupported OpenAPI version %f found", openapiVersion))
			return nil
		}

		spec, parseErrors := doc.BuildV3Model()
		if parseErrors != nil {
			log.Warn().Err(err).Str("path", path).Msg("Failed to build OpenAPI v3 model")
			return nil
		}

		serviceAsset := s.createServiceAsset(spec, config)
		addUniqueAsset(&assets, serviceAsset, seenAssets)

		endpointAssets := s.createEndpointAssets(spec, config)
		for _, asset := range endpointAssets {
			addUniqueAsset(&assets, asset, seenAssets)
		}

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

	mrnValue := serviceMrnValue(spec)
	openapiVersion := spec.Model.Version

	var servers []string
	if spec.Model.Servers != nil {
		for _, server := range spec.Model.Servers {
			servers = append(servers, server.URL)
		}
	}

	numEndpoints := 0
	numDeprecatedEndpoints := 0
	for _, item := range spec.Model.Paths.PathItems.FromOldest() {
		for _, op := range item.GetOperations().FromOldest() {
			if op.Deprecated != nil && *op.Deprecated {
				numDeprecatedEndpoints++ 
			}
		}
		numEndpoints += item.GetOperations().Len()
	}

	serviceFields := OpenAPIFields{
		Description: description,
		NumDeprecatedEndpoints: numDeprecatedEndpoints,
		NumEndpoints: numEndpoints,
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

func (s *Source) createEndpointAssets(spec*libopenapi.DocumentModel[v3.Document], config *Config) []asset.Asset {
	assets := []asset.Asset{}
	parentMrn := serviceMrnValue(spec)
	serviceName := spec.Model.Info.Title

	for path, item := range spec.Model.Paths.PathItems.FromOldest() {
		for httpMethod, op := range item.GetOperations().FromOldest() {
			pathWithMethod := fmt.Sprintf("%s %s", strings.ToUpper(httpMethod), path)
			mrnValue := mrn.New(typeEndpoint, serviceName, pathWithMethod)
			description := op.Summary
			if len(description) == 0 {
				description = op.Description
			}

			statusCodes := []string{}
			for code := range op.Responses.Codes.FromOldest() {
				statusCodes = append(statusCodes, code)
			}

			endpointField := EndpointField{
				Description: op.Description,
				HTTPMethod: strings.ToUpper(httpMethod),
				OperationID: op.OperationId,
				Path: path,
				StatusCodes: statusCodes,
				Summary: op.Summary,
			}
			if op.Deprecated != nil {
				endpointField.Deprecated = *op.Deprecated
			}
			if len(endpointField.Summary) == 0 {
				endpointField.Summary = item.Summary
			}
			if len(endpointField.Description) == 0 {
				endpointField.Description = item.Description
			}
			metadata := plugin.MapToMetadata(endpointField)
			processedTags := plugin.InterpolateTags(s.config.Tags, metadata)
			processedTags = append(processedTags, serviceName)
			processedTags = append(processedTags, op.Tags...)
			if op.Deprecated != nil && *op.Deprecated {
				processedTags = append(processedTags, "deprecated")
			}

			externalLinks := []asset.ExternalLink{}
			if op.ExternalDocs != nil {
				name := op.ExternalDocs.Description
				if len(name) == 0 {
					name = op.ExternalDocs.URL
				}
				externalLinks = append(externalLinks, asset.ExternalLink{
					Name: name,
					URL: op.ExternalDocs.URL,
				})
			}

			schema := make(map[string]string)
			for code, response := range op.Responses.Codes.FromOldest() {
				for content, mediaType := range response.Content.FromOldest() {
					jsonSchema, err := NewJsonSchemaFromOpenAPISchema(mediaType.Schema)
					if err != nil {
						log.Warn().Err(err).Msg("Failed to convert OpenAPI schema to json schema")
						continue
					}
					jsonStr, err := json.Marshal(jsonSchema)
					if err != nil {
						log.Warn().Err(err).Msg("Failed to marshal json schema")
						continue
					}
					schema[code+":"+content] = string(jsonStr)
				}
			}

			asset := asset.Asset{
				Name: &pathWithMethod,
				MRN: &mrnValue,
				ParentMRN: &parentMrn,
				Type: typeEndpoint,
				Providers: []string{openapiProvider},
				Description: &description,
				Metadata: metadata,
				Tags: processedTags,
				Sources: []asset.AssetSource{},
				ExternalLinks: externalLinks,
				Schema: schema,
			}
			assets = append(assets, asset)
		}
	}

	return assets
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

func serviceMrnValue(spec *libopenapi.DocumentModel[v3.Document]) string {
	serviceName := spec.Model.Info.Title
	return mrn.New(typeService, "openapi", serviceName)
}

func isJSON(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".json"
}

func isYAML(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml"
}


