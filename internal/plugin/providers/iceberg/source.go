// +marmot:name=Iceberg
// +marmot:description=This plugin discovers namespaces, tables, and views from Iceberg REST catalogs.
// +marmot:status=experimental
// +marmot:features=Assets,Lineage
package iceberg

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/iceberg-go/catalog"
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

	URI        string            `json:"uri" description:"REST catalog URI" validate:"required,url"`
	Warehouse  string            `json:"warehouse" description:"Warehouse identifier"`
	Credential string            `json:"credential" description:"Credential for OAuth2 client credentials authentication" sensitive:"true"`
	Token      string            `json:"token" description:"Bearer token for authentication" sensitive:"true"`
	Properties map[string]string `json:"properties" description:"Additional catalog properties"`
	Prefix     string            `json:"prefix" description:"Optional prefix for the REST catalog"`

	IncludeNamespaces bool `json:"include_namespaces" description:"Whether to discover namespaces as assets" default:"true"`
	IncludeViews      bool `json:"include_views" description:"Whether to discover views" default:"true"`
}

// +marmot:example-config
var _ = `
uri: "http://localhost:8181"
warehouse: "my-warehouse"
credential: "client-id:client-secret"
tags:
  - "iceberg"
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

	if _, ok := rawConfig["include_namespaces"]; !ok {
		config.IncludeNamespaces = true
	}
	if _, ok := rawConfig["include_views"]; !ok {
		config.IncludeViews = true
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if _, ok := pluginConfig["include_namespaces"]; !ok {
		config.IncludeNamespaces = true
	}
	if _, ok := pluginConfig["include_views"]; !ok {
		config.IncludeViews = true
	}

	s.config = config

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
	meta := plugin.PluginMeta{
		ID:          "iceberg",
		Name:        "Apache Iceberg",
		Description: "Discover namespaces, tables, and views from Iceberg REST catalogs",
		Icon:        "iceberg",
		Category:    "data-lake",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Iceberg plugin")
	}
}
