// +marmot:name=Iceberg
// +marmot:description=This plugin discovers namespaces, tables and views from Iceberg catalogs (REST and AWS Glue).
// +marmot:status=experimental
// +marmot:features=Assets,Lineage
package iceberg

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/iceberg-go/catalog"
	gluecat "github.com/apache/iceberg-go/catalog/glue"
	"github.com/apache/iceberg-go/catalog/rest"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline"`

	CatalogType string `json:"catalog_type" description:"Catalog backend type" default:"rest" validate:"omitempty,oneof=rest glue"`

	// REST catalog fields
	URI        string            `json:"uri" description:"REST catalog URI (required for catalog_type=rest)" show_when:"catalog_type:rest"`
	Warehouse  string            `json:"warehouse" description:"Warehouse identifier" show_when:"catalog_type:rest"`
	Credential string            `json:"credential" description:"Credential for OAuth2 client credentials authentication" sensitive:"true" show_when:"catalog_type:rest"`
	Token      string            `json:"token" description:"Bearer token for authentication" sensitive:"true" show_when:"catalog_type:rest"`
	Properties map[string]string `json:"properties" description:"Additional catalog properties" show_when:"catalog_type:rest"`
	Prefix     string            `json:"prefix" description:"Optional prefix for the REST catalog" show_when:"catalog_type:rest"`

	// Glue catalog fields
	GlueCatalogID string `json:"glue_catalog_id" description:"AWS Glue Data Catalog ID (defaults to caller's account)" show_when:"catalog_type:glue"`

	IncludeNamespaces bool `json:"include_namespaces" description:"Whether to discover namespaces as assets" default:"true"`
	IncludeViews      bool `json:"include_views" description:"Whether to discover views" default:"true"`
}

// +marmot:example-config
var _ = `
# REST catalog (default)
uri: "http://localhost:8181"
warehouse: "my-warehouse"
credential: "client-id:client-secret"
tags:
  - "iceberg"

# Glue catalog:
# catalog_type: "glue"
# credentials:
#   region: "us-east-1"
# glue_catalog_id: "123456789012"  # optional, defaults to caller's account
`

type Source struct {
	config *Config
	cat    catalog.Catalog
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if _, ok := rawConfig["catalog_type"]; !ok {
		config.CatalogType = "rest"
	}
	if _, ok := rawConfig["include_namespaces"]; !ok {
		config.IncludeNamespaces = true
	}
	if _, ok := rawConfig["include_views"]; !ok {
		config.IncludeViews = true
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	if config.CatalogType == "rest" {
		if config.URI == "" {
			return nil, fmt.Errorf("uri is required when catalog_type is rest")
		}
		u, err := url.ParseRequestURI(config.URI)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("uri must be a valid URL")
		}
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if _, ok := pluginConfig["catalog_type"]; !ok {
		config.CatalogType = "rest"
	}
	if _, ok := pluginConfig["include_namespaces"]; !ok {
		config.IncludeNamespaces = true
	}
	if _, ok := pluginConfig["include_views"]; !ok {
		config.IncludeViews = true
	}

	s.config = config

	switch config.CatalogType {
	case "rest":
		var opts []rest.Option
		if config.Credential != "" {
			opts = append(opts, rest.WithCredential(config.Credential))
		}
		if config.Token != "" {
			opts = append(opts, rest.WithOAuthToken(config.Token))
		}
		if config.Warehouse != "" {
			opts = append(opts, rest.WithWarehouseLocation(config.Warehouse))
		}
		if config.Prefix != "" {
			opts = append(opts, rest.WithPrefix(config.Prefix))
		}
		if len(config.Properties) > 0 {
			opts = append(opts, rest.WithAdditionalProps(config.Properties))
		}

		cat, err := rest.NewCatalog(ctx, "rest", config.URI, opts...)
		if err != nil {
			return nil, fmt.Errorf("creating REST catalog: %w", err)
		}
		s.cat = cat

		// Disable pagination to avoid compatibility issues with REST catalog
		// servers that don't support the pageSize parameter (e.g. reference impl <= 1.6.x)
		ctx = cat.SetPageSize(ctx, -1)

	case "glue":
		awsConfig, err := plugin.ExtractAWSConfig(pluginConfig)
		if err != nil {
			return nil, fmt.Errorf("extracting AWS config: %w", err)
		}

		awsCfg, err := awsConfig.NewAWSConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("creating AWS config: %w", err)
		}

		glueOpts := []gluecat.Option{gluecat.WithAwsConfig(awsCfg)}
		if config.GlueCatalogID != "" {
			glueOpts = append(glueOpts, gluecat.WithAwsProperties(gluecat.AwsProperties{
				gluecat.CatalogIdKey: config.GlueCatalogID,
			}))
		}
		s.cat = gluecat.NewCatalog(glueOpts...)
	}

	nsAssets, namespaces, err := s.discoverNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering namespaces: %w", err)
	}

	tableAssets, err := s.discoverTables(ctx, namespaces)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}

	var viewAssets []asset.Asset
	if config.IncludeViews {
		viewAssets, err = s.discoverViews(ctx, namespaces)
		if err != nil {
			return nil, fmt.Errorf("discovering views: %w", err)
		}
	}

	var allAssets []asset.Asset
	allAssets = append(allAssets, nsAssets...)
	allAssets = append(allAssets, tableAssets...)
	allAssets = append(allAssets, viewAssets...)

	var lineages []lineage.LineageEdge
	if config.IncludeNamespaces {
		lineages = buildContainsLineage(tableAssets, viewAssets)
	}

	result := &plugin.DiscoveryResult{
		Assets:  allAssets,
		Lineage: lineages,
	}

	plugin.FilterDiscoveryResult(result, pluginConfig)

	return result, nil
}

func buildContainsLineage(tableAssets, viewAssets []asset.Asset) []lineage.LineageEdge {
	var edges []lineage.LineageEdge

	for i := range tableAssets {
		nsMRN := namespaceFromAssetMRN(tableAssets[i])
		if nsMRN == "" {
			continue
		}
		edges = append(edges, lineage.LineageEdge{
			Source: nsMRN,
			Target: *tableAssets[i].MRN,
			Type:   "CONTAINS",
		})
	}

	for i := range viewAssets {
		nsMRN := namespaceFromAssetMRN(viewAssets[i])
		if nsMRN == "" {
			continue
		}
		edges = append(edges, lineage.LineageEdge{
			Source: nsMRN,
			Target: *viewAssets[i].MRN,
			Type:   "CONTAINS",
		})
	}

	return edges
}

// namespaceFromAssetMRN derives the parent namespace MRN from a table/view MRN.
func namespaceFromAssetMRN(a asset.Asset) string {
	if a.MRN == nil || a.Metadata == nil {
		return ""
	}

	mrnStr := *a.MRN
	parts := strings.SplitN(mrnStr, "/iceberg/", 2)
	if len(parts) != 2 {
		return ""
	}

	fullName := parts[1]
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot < 0 {
		return ""
	}

	nsPath := fullName[:lastDot]
	return mrn.New("Namespace", "Iceberg", nsPath)
}

func init() {
	spec := plugin.GenerateConfigSpec(Config{})

	// Set show_when on inlined AWSConfig fields so they only appear for Glue catalogs
	glueShowWhen := &plugin.ShowWhen{Field: "catalog_type", Value: "glue"}
	for i := range spec {
		switch spec[i].Name {
		case "credentials", "tags_to_metadata", "include_tags":
			spec[i].ShowWhen = glueShowWhen
		}
	}

	meta := plugin.PluginMeta{
		ID:          "iceberg",
		Name:        "Apache Iceberg",
		Description: "Discover namespaces, tables and views from Iceberg catalogs (REST and AWS Glue)",
		Icon:        "iceberg",
		Category:    "data-lake",
		ConfigSpec:  spec,
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Iceberg plugin")
	}
}
