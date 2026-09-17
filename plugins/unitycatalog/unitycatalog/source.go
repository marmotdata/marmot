// Package unitycatalog discovers catalogs, tables, views, volumes,
// functions and registered models from a Unity Catalog server.
package unitycatalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact string every asset carries in Providers and the
// service component of every MRN. It has a space in it, which mrn.New
// dashes: mrn://table/unity-catalog/shop.sales.orders.
const provider = "Unity Catalog"

// Config for the Unity Catalog plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host  string `json:"host" description:"Unity Catalog server URL, for example http://localhost:8080" validate:"required,url"`
	Token string `json:"token" description:"Bearer token, when the server requires one" sensitive:"true"`

	Catalogs        []string `json:"catalogs" description:"Catalogs to discover (all when empty)"`
	ExcludeCatalogs []string `json:"exclude_catalogs" description:"Catalogs to skip" default:"[\"system\",\"__databricks_internal\"]"`

	IncludeColumns   bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeVolumes   bool `json:"include_volumes" description:"Whether to discover volumes" default:"true"`
	IncludeFunctions bool `json:"include_functions" description:"Whether to discover functions" default:"true"`
	IncludeModels    bool `json:"include_models" description:"Whether to discover registered models" default:"true"`

	VerifySSL bool `json:"verify_ssl" label:"Verify SSL" description:"Whether to verify the server's TLS certificate" default:"true"`
	PageSize  int  `json:"page_size" description:"Objects requested per API page" default:"100" validate:"omitempty,min=1,max=1000"`
}

// Example configuration for the plugin
var _ = `
host: "http://localhost:8080"
token: "dapi-your-token"
catalogs:
  - "unity"
  - "shop"
exclude_catalogs:
  - "system"
  - "__databricks_internal"
include_columns: true
include_volumes: true
include_functions: true
include_models: true
tags:
  - "unity-catalog"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "unitycatalog",
		Name:        "Unity Catalog",
		Description: "Discover catalogs, tables, views, volumes, functions and models from Unity Catalog servers",
		Icon:        "databricks",
		Category:    "catalog",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(UnityCatalogCatalogFields{}, "Catalog",
				"The metadata fields emitted for catalog assets."),
			pluginsdk.AssetSchemaOf(UnityCatalogTableFields{}, "Table",
				"The metadata fields emitted for table and view assets."),
			pluginsdk.AssetSchemaOf(UnityCatalogColumnFields{}, "Column",
				"The per-column fields embedded in an asset's schema."),
			pluginsdk.AssetSchemaOf(UnityCatalogVolumeFields{}, "Volume",
				"The metadata fields emitted for volume assets."),
			pluginsdk.AssetSchemaOf(UnityCatalogFunctionFields{}, "Function",
				"The metadata fields emitted for function assets."),
			pluginsdk.AssetSchemaOf(UnityCatalogModelFields{}, "Model",
				"The metadata fields emitted for registered model assets."),
		},
	}
}

// Source represents the Unity Catalog plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimSuffix(config.Host, "/")

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover walks every catalog and schema the server exposes and turns
// what it finds into assets, lineage and statistics.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	d := &discovery{
		config:    s.config,
		client:    newClient(s.config.Host, s.config.Token, s.config.PageSize, s.config.VerifySSL),
		tableMRNs: make(map[string]string),
		edges:     make(map[string]struct{}),
	}

	log.Debug().Str("host", s.config.Host).Msg("Starting Unity Catalog discovery")

	catalogs, err := d.client.listCatalogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing catalogs: %w", err)
	}

	for _, catalog := range catalogs {
		if !s.wantsCatalog(catalog.Name) {
			log.Debug().Str("catalog", catalog.Name).Msg("Skipping catalog")
			continue
		}
		d.discoverCatalog(ctx, catalog)
	}

	// Every table is known by now, so view and foreign key references can
	// be resolved against the run's own assets.
	d.buildReferenceLineage()

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Int("statistics", len(d.statistics)).
		Msg("Unity Catalog discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     d.assets,
		Lineage:    d.lineage,
		Statistics: d.statistics,
	}, nil
}

// wantsCatalog applies the catalogs allow list and the exclude list.
func (s *Source) wantsCatalog(name string) bool {
	for _, excluded := range s.config.ExcludeCatalogs {
		if strings.EqualFold(excluded, name) {
			return false
		}
	}
	if len(s.config.Catalogs) == 0 {
		return true
	}
	for _, wanted := range s.config.Catalogs {
		if strings.EqualFold(wanted, name) {
			return true
		}
	}
	return false
}

// discovery accumulates the result of one Discover call.
type discovery struct {
	config *Config
	client *client

	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// tableMRNs maps a lower-cased three-part table name to the MRN this
	// run gave it, so view definitions and foreign keys can be resolved
	// without caring about the case the SQL was written in.
	tableMRNs map[string]string
	// tables keeps every table for the reference pass at the end.
	tables []tableInfo
	// edges dedupes lineage: the same edge can be reached from a view's
	// SQL and from its recorded dependencies.
	edges map[string]struct{}
}

// discoverCatalog lists the catalog's schemas and everything under them.
// A schema that fails is logged and skipped so one bad schema does not
// hide the rest of the server.
func (d *discovery) discoverCatalog(ctx context.Context, catalog catalogInfo) {
	schemas, err := d.client.listSchemas(ctx, catalog.Name)
	if err != nil {
		log.Warn().Err(err).Str("catalog", catalog.Name).Msg("Failed to list schemas")
		d.addAsset(d.catalogAsset(catalog, 0, 0))
		return
	}

	tableCount := 0
	catalogMRN := assetMRN("Catalog", catalog.Name)

	for _, schema := range schemas {
		tables, err := d.client.listTables(ctx, catalog.Name, schema.Name)
		if err != nil {
			log.Warn().Err(err).Str("catalog", catalog.Name).Str("schema", schema.Name).Msg("Failed to list tables")
		}
		for _, table := range tables {
			table = d.completeTable(ctx, table)
			asset := d.tableAsset(table)
			d.addAsset(asset)
			d.addEdge(catalogMRN, *asset.MRN, "CONTAINS")
			d.addStorageEdge(table.StorageLocation, *asset.MRN)
			d.statistics = append(d.statistics, pluginsdk.Statistic{
				AssetMRN:   *asset.MRN,
				MetricName: "asset.column_count",
				Value:      float64(len(table.Columns)),
			})
			d.tableMRNs[strings.ToLower(fullName(table.CatalogName, table.SchemaName, table.Name))] = *asset.MRN
			d.tables = append(d.tables, table)
			tableCount++
		}

		if d.config.IncludeVolumes {
			volumes, err := d.client.listVolumes(ctx, catalog.Name, schema.Name)
			if err != nil {
				log.Warn().Err(err).Str("catalog", catalog.Name).Str("schema", schema.Name).Msg("Failed to list volumes")
			}
			for _, volume := range volumes {
				asset := d.volumeAsset(volume)
				d.addAsset(asset)
				d.addEdge(catalogMRN, *asset.MRN, "CONTAINS")
				d.addStorageEdge(volume.StorageLocation, *asset.MRN)
			}
		}

		if d.config.IncludeFunctions {
			functions, err := d.client.listFunctions(ctx, catalog.Name, schema.Name)
			if err != nil {
				log.Warn().Err(err).Str("catalog", catalog.Name).Str("schema", schema.Name).Msg("Failed to list functions")
			}
			for _, function := range functions {
				asset := d.functionAsset(function)
				d.addAsset(asset)
				d.addEdge(catalogMRN, *asset.MRN, "CONTAINS")
			}
		}

		if d.config.IncludeModels {
			d.discoverModels(ctx, catalog.Name, schema.Name, catalogMRN)
		}
	}

	d.addAsset(d.catalogAsset(catalog, len(schemas), tableCount))
}

// completeTable fetches the table on its own when the listing left the
// columns out, which some servers do for large schemas.
func (d *discovery) completeTable(ctx context.Context, table tableInfo) tableInfo {
	if !d.config.IncludeColumns || table.Columns != nil {
		return table
	}

	name := fullName(table.CatalogName, table.SchemaName, table.Name)
	full, err := d.client.getTable(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("table", name).Msg("Failed to load table columns")
		return table
	}
	return *full
}

// discoverModels lists registered models and their versions. A server
// without the models API answers 404, which is not an error worth
// reporting: the feature simply is not there.
func (d *discovery) discoverModels(ctx context.Context, catalog, schema, catalogMRN string) {
	models, err := d.client.listModels(ctx, catalog, schema)
	if err != nil {
		if isNotFound(err) {
			log.Debug().Str("catalog", catalog).Str("schema", schema).Msg("Server has no registered models API")
			return
		}
		log.Warn().Err(err).Str("catalog", catalog).Str("schema", schema).Msg("Failed to list models")
		return
	}

	for _, model := range models {
		name := fullName(model.CatalogName, model.SchemaName, model.Name)
		versions, err := d.client.listModelVersions(ctx, name)
		if err != nil {
			if isNotFound(err) {
				log.Debug().Str("model", name).Msg("Server has no model versions API")
			} else {
				log.Warn().Err(err).Str("model", name).Msg("Failed to list model versions")
			}
		}
		asset := d.modelAsset(model, versions)
		d.addAsset(asset)
		d.addEdge(catalogMRN, *asset.MRN, "CONTAINS")
	}
}

func (d *discovery) addAsset(a pluginsdk.Asset) {
	d.assets = append(d.assets, a)
}

// addEdge records an edge once, however many times it is found.
func (d *discovery) addEdge(source, target, edgeType string) {
	if source == "" || target == "" || source == target {
		return
	}
	key := source + "|" + target + "|" + edgeType
	if _, seen := d.edges[key]; seen {
		return
	}
	d.edges[key] = struct{}{}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// assetMRN is the single place a Unity Catalog MRN is built. Assets and
// lineage both go through it so the two can never drift into addressing
// the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// fullName is the three-part name Unity Catalog uses everywhere.
func fullName(catalog, schema, name string) string {
	return catalog + "." + schema + "." + name
}
