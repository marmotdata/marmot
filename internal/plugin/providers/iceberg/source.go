// +marmot:name=Iceberg
// +marmot:description=This plugin discovers Apache Iceberg tables from various catalog implementations.
// +marmot:status=experimental
package iceberg

import (
	"context"
	"fmt"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for Iceberg plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Catalog configuration
	CatalogType string `json:"catalog_type" yaml:"catalog_type" description:"Type of catalog to use: rest, glue"`

	// Catalog-specific configurations
	REST *RESTConfig `json:"rest,omitempty" yaml:"rest,omitempty" description:"REST catalog configuration"`
	Glue *GlueConfig `json:"glue,omitempty" yaml:"glue,omitempty" description:"AWS Glue catalog configuration"`

	// Metadata collection options
	IncludeSchemaInfo    bool `json:"include_schema_info" yaml:"include_schema_info" description:"Whether to include schema information in metadata"`
	IncludePartitionInfo bool `json:"include_partition_info" yaml:"include_partition_info" description:"Whether to include partition information in metadata"`
	IncludeSnapshotInfo  bool `json:"include_snapshot_info" yaml:"include_snapshot_info" description:"Whether to include snapshot information in metadata"`
	IncludeProperties    bool `json:"include_properties" yaml:"include_properties" description:"Whether to include table properties in metadata"`
	IncludeStatistics    bool `json:"include_statistics" yaml:"include_statistics" description:"Whether to include table statistics in metadata"`

	// Filter configuration
	TableFilter     *plugin.Filter `json:"table_filter,omitempty" yaml:"table_filter,omitempty" description:"Filter configuration for tables"`
	NamespaceFilter *plugin.Filter `json:"namespace_filter,omitempty" yaml:"namespace_filter,omitempty" description:"Filter configuration for namespaces"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
catalog_type: "rest"  # Options: "rest", "glue"

# REST catalog configuration
rest:
  uri: "http://localhost:8181"
  auth:
    type: "none"  # Options: "none", "basic", "oauth2", "bearer"
    username: ""
    password: ""
    token: ""
    client_id: ""
    client_secret: ""
    token_url: ""
    cert_path: ""

# AWS Glue catalog configuration
# glue:
#   region: "us-west-2"
#   database: "default"  # Optional: limit discovery to a single database
#   access_key: ""  # Optional: use environment or instance profile if not provided
#   secret_key: ""
#   credentials_profile: ""  # Optional: use named AWS profile
#   assume_role_arn: ""  # Optional: assume role ARN

# Metadata collection options
include_schema_info: true
include_partition_info: true
include_snapshot_info: true
include_properties: true
include_statistics: true

# Filter configuration
table_filter:
  include:
    - "^prod-.*"
    - "^staging-.*"
  exclude:
    - ".*-test$"
    - ".*-dev$"

namespace_filter:
  include:
    - "^analytics.*"
    - "^data.*"
  exclude:
    - ".*_temp$"

# Common tag configuration
tags:
  - "iceberg"
  - "data-lake"
`

// Source represents the Iceberg plugin source
type Source struct {
	config *Config
	client interface{} // Can be REST client, Glue client, etc. based on catalog type
}

// IcebergMetadata represents the metadata for an Iceberg table
type IcebergMetadata struct {
	// Table identity
	Identifier string `json:"identifier"`
	Namespace  string `json:"namespace"`
	TableName  string `json:"table_name"`
	Location   string `json:"location"`

	// Format and version information
	FormatVersion int    `json:"format_version"`
	UUID          string `json:"uuid"`

	// Schema information
	CurrentSchemaID int    `json:"current_schema_id"`
	SchemaJSON      string `json:"schema_json,omitempty"`
	PartitionSpec   string `json:"partition_spec,omitempty"`

	// Snapshot information
	CurrentSnapshotID int64 `json:"current_snapshot_id"`
	LastUpdatedMs     int64 `json:"last_updated_ms"`
	NumSnapshots      int   `json:"num_snapshots,omitempty"`

	// Statistics
	NumRows        int64 `json:"num_rows,omitempty"`
	FileSizeBytes  int64 `json:"file_size_bytes,omitempty"`
	NumDataFiles   int   `json:"num_data_files,omitempty"`
	NumDeleteFiles int   `json:"num_delete_files,omitempty"`

	// Partition information
	NumPartitions         int    `json:"num_partitions,omitempty"`
	PartitionTransformers string `json:"partition_transformers,omitempty"`

	// Properties
	Properties map[string]string `json:"properties,omitempty"`

	// Catalog information
	CatalogType string `json:"catalog_type"`
	CatalogName string `json:"catalog_name,omitempty"`

	// Sort order
	SortOrderJSON string `json:"sort_order_json,omitempty"`
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	log.Debug().Interface("raw_config", rawConfig).Msg("Starting Iceberg config validation")

	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	switch config.CatalogType {
	case "rest":
		if config.REST == nil {
			return fmt.Errorf("REST configuration is required when catalog_type is 'rest'")
		}
		if config.REST.URI == "" {
			return fmt.Errorf("REST URI is required when catalog_type is 'rest'")
		}
	case "glue":
		if config.Glue == nil {
			return fmt.Errorf("Glue configuration is required when catalog_type is 'glue'")
		}
		if config.Glue.Region == "" {
			return fmt.Errorf("AWS region is required when catalog_type is 'glue'")
		}
	default:
		return fmt.Errorf("unsupported catalog type: %s (supported types: rest, glue)", config.CatalogType)
	}

	s.config = config
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if err := s.initClient(ctx); err != nil {
		return nil, fmt.Errorf("initializing %s client: %w", s.config.CatalogType, err)
	}

	namespaces, err := s.discoverNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering namespaces: %w", err)
	}

	var assets []asset.Asset
	for _, namespace := range namespaces {
		if s.config.NamespaceFilter != nil && !plugin.ShouldIncludeResource(namespace, *s.config.NamespaceFilter) {
			log.Debug().Str("namespace", namespace).Msg("Skipping namespace due to filter")
			continue
		}

		tables, err := s.discoverTables(ctx, namespace)
		if err != nil {
			log.Warn().Err(err).Str("namespace", namespace).Msg("Failed to discover tables in namespace")
			continue
		}

		for _, table := range tables {
			if s.config.TableFilter != nil && !plugin.ShouldIncludeResource(table, *s.config.TableFilter) {
				log.Debug().Str("table", table).Msg("Skipping table due to filter")
				continue
			}

			asset, err := s.createTableAsset(ctx, namespace, table)
			if err != nil {
				log.Warn().Err(err).Str("namespace", namespace).Str("table", table).Msg("Failed to create asset for table")
				continue
			}
			assets = append(assets, asset)
		}
	}

	return &plugin.DiscoveryResult{
		Assets: assets,
	}, nil
}

func (s *Source) initClient(ctx context.Context) error {
	switch s.config.CatalogType {
	case "rest":
		return s.initRESTClient(ctx)
	case "glue":
		return s.initGlueClient(ctx)
	default:
		return fmt.Errorf("unsupported catalog type: %s", s.config.CatalogType)
	}
}

// discoverNamespaces discovers all namespaces in the catalog
func (s *Source) discoverNamespaces(ctx context.Context) ([]string, error) {
	switch s.config.CatalogType {
	case "rest":
		return s.discoverRESTNamespaces(ctx)
	case "glue":
		return s.discoverGlueDatabases(ctx)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %s", s.config.CatalogType)
	}
}

// discoverTables discovers all tables in a namespace
func (s *Source) discoverTables(ctx context.Context, namespace string) ([]string, error) {
	switch s.config.CatalogType {
	case "rest":
		return s.discoverRESTTables(ctx, namespace)
	case "glue":
		return s.discoverGlueTables(ctx, namespace)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %s", s.config.CatalogType)
	}
}

// getTableMetadata gets metadata for a specific table
func (s *Source) getTableMetadata(ctx context.Context, namespace, table string) (*IcebergMetadata, error) {
	switch s.config.CatalogType {
	case "rest":
		return s.getRESTTableMetadata(ctx, namespace, table)
	case "glue":
		return s.getGlueTableMetadata(ctx, namespace, table)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %s", s.config.CatalogType)
	}
}

// createTableAsset creates an asset for a table
func (s *Source) createTableAsset(ctx context.Context, namespace, table string) (asset.Asset, error) {
	metadata, err := s.getTableMetadata(ctx, namespace, table)
	if err != nil {
		return asset.Asset{}, fmt.Errorf("getting table metadata: %w", err)
	}

	metadataMap := make(map[string]interface{})
	metadataMap["identifier"] = metadata.Identifier
	metadataMap["namespace"] = metadata.Namespace
	metadataMap["table_name"] = metadata.TableName
	metadataMap["location"] = metadata.Location
	metadataMap["format_version"] = metadata.FormatVersion
	metadataMap["uuid"] = metadata.UUID

	if s.config.IncludeSchemaInfo {
		metadataMap["current_schema_id"] = metadata.CurrentSchemaID
		if metadata.SchemaJSON != "" {
			metadataMap["schema_json"] = metadata.SchemaJSON
		}
		if metadata.PartitionSpec != "" {
			metadataMap["partition_spec"] = metadata.PartitionSpec
		}
	}

	if s.config.IncludeSnapshotInfo {
		metadataMap["current_snapshot_id"] = metadata.CurrentSnapshotID
		metadataMap["last_updated_ms"] = metadata.LastUpdatedMs
		metadataMap["num_snapshots"] = metadata.NumSnapshots
	}

	if s.config.IncludeStatistics {
		if metadata.NumRows > 0 {
			metadataMap["num_rows"] = metadata.NumRows
		}
		if metadata.FileSizeBytes > 0 {
			metadataMap["file_size_bytes"] = metadata.FileSizeBytes
		}
		if metadata.NumDataFiles > 0 {
			metadataMap["num_data_files"] = metadata.NumDataFiles
		}
		if metadata.NumDeleteFiles > 0 {
			metadataMap["num_delete_files"] = metadata.NumDeleteFiles
		}
	}

	if s.config.IncludePartitionInfo {
		if metadata.NumPartitions > 0 {
			metadataMap["num_partitions"] = metadata.NumPartitions
		}
		if metadata.PartitionTransformers != "" {
			metadataMap["partition_transformers"] = metadata.PartitionTransformers
		}
	}

	if s.config.IncludeProperties && metadata.Properties != nil {
		for k, v := range metadata.Properties {
			metadataMap[k] = v
		}
	}

	metadataMap["catalog_type"] = metadata.CatalogType
	if metadata.CatalogName != "" {
		metadataMap["catalog_name"] = metadata.CatalogName
	}

	if metadata.SortOrderJSON != "" {
		metadataMap["sort_order_json"] = metadata.SortOrderJSON
	}

	if metadata.LastUpdatedMs > 0 {
		lastCommitTime := time.Unix(0, metadata.LastUpdatedMs*int64(time.Millisecond)).Format(time.RFC3339)
		metadataMap["last_commit_time"] = lastCommitTime
	}

	description := fmt.Sprintf("Iceberg table %s.%s", namespace, table)
	mrnValue := mrn.New("Table", "Iceberg", fmt.Sprintf("%s.%s", namespace, table))

	processedTags := plugin.InterpolateTags(s.config.Tags, metadataMap)

	return asset.Asset{
		Name:        &table,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"Iceberg"},
		Description: &description,
		Metadata:    metadataMap,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Iceberg",
			LastSyncAt: time.Now(),
			Properties: metadataMap,
			Priority:   1,
		}},
	}, nil
}
