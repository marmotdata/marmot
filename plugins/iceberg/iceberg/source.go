// Package iceberg discovers namespaces, tables and views from Iceberg
// catalogs (REST and AWS Glue).
package iceberg

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/iceberg-go/catalog"
	gluecat "github.com/apache/iceberg-go/catalog/glue"
	"github.com/apache/iceberg-go/catalog/rest"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	spec := pluginsdk.GenerateConfigSpec(Config{})

	// Set show_when on inlined AWSConfig fields so they only appear for Glue catalogs
	glueShowWhen := &pluginsdk.ShowWhen{Field: "catalog_type", Value: "glue"}
	for i := range spec {
		switch spec[i].Name {
		case "credentials", "tags_to_metadata", "include_tags":
			spec[i].ShowWhen = glueShowWhen
		}
	}

	return pluginsdk.Meta{
		ID:          "iceberg",
		Name:        "Apache Iceberg",
		Description: "Discover namespaces, tables and views from Iceberg catalogs (REST and AWS Glue)",
		Icon:        "iceberg",
		Category:    "data-lake",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  spec,
	}
}

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`
	*pluginsdk.AWSConfig `json:",inline"`

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

// Example configuration for the plugin
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

func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
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

	if err := pluginsdk.ValidateStruct(config); err != nil {
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

func (s *Source) Discover(ctx context.Context, pluginConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](pluginConfig)
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
		awsConfig, err := pluginsdk.ExtractAWSConfig(pluginConfig)
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

	var viewAssets []pluginsdk.Asset
	if config.IncludeViews {
		viewAssets, err = s.discoverViews(ctx, namespaces)
		if err != nil {
			return nil, fmt.Errorf("discovering views: %w", err)
		}
	}

	var allAssets []pluginsdk.Asset
	allAssets = append(allAssets, nsAssets...)
	allAssets = append(allAssets, tableAssets...)
	allAssets = append(allAssets, viewAssets...)

	var lineages []pluginsdk.LineageEdge
	if config.IncludeNamespaces {
		lineages = buildContainsLineage(tableAssets, viewAssets)
	}

	return &pluginsdk.DiscoveryResult{
		Assets:  allAssets,
		Lineage: lineages,
	}, nil
}

func buildContainsLineage(tableAssets, viewAssets []pluginsdk.Asset) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge

	for i := range tableAssets {
		nsMRN := namespaceFromAssetMRN(tableAssets[i])
		if nsMRN == "" {
			continue
		}
		edges = append(edges, pluginsdk.LineageEdge{
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
		edges = append(edges, pluginsdk.LineageEdge{
			Source: nsMRN,
			Target: *viewAssets[i].MRN,
			Type:   "CONTAINS",
		})
	}

	return edges
}

// namespaceFromAssetMRN derives the parent namespace MRN from a table/view MRN.
func namespaceFromAssetMRN(a pluginsdk.Asset) string {
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
