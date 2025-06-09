// +marmot:name=BigQuery
// +marmot:description=This plugin discovers datasets and tables from Google BigQuery projects.
// +marmot:status=experimental
package bigquery

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	ProjectID             string `json:"project_id" description:"Google Cloud Project ID"`
	CredentialsPath       string `json:"credentials_path,omitempty" description:"Path to service account credentials JSON file"`
	CredentialsJSON       string `json:"credentials_json,omitempty" description:"Service account credentials JSON content"`
	UseDefaultCredentials bool   `json:"use_default_credentials" description:"Use default Google Cloud credentials" default:"false"`

	IncludeDatasets       bool           `json:"include_datasets" description:"Whether to discover datasets" default:"true"`
	IncludeTableStats     bool           `json:"include_table_stats" description:"Whether to include table statistics (row count, size)" default:"true"`
	IncludeViews          bool           `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeExternalTables bool           `json:"include_external_tables" description:"Whether to discover external tables" default:"true"`
	DatasetFilter         *plugin.Filter `json:"dataset_filter,omitempty" description:"Filter configuration for datasets"`
	TableFilter           *plugin.Filter `json:"table_filter,omitempty" description:"Filter configuration for tables"`
	ExcludeSystemDatasets bool           `json:"exclude_system_datasets" description:"Whether to exclude system datasets (_script, _analytics, etc.)" default:"true"`
	MaxConcurrentRequests int            `json:"max_concurrent_requests" description:"Maximum number of concurrent API requests" default:"10"`
}

// +marmot:example-config
var _ = `
project_id: "company-data-warehouse"
credentials_path: "/etc/marmot/bq-service-account.json"
tags:
  - "bigquery"
  - "data-warehouse"
`

type Source struct {
	config *Config
	client *bigquery.Client
}

type TableType string

const (
	TableTypeTable    TableType = "TABLE"
	TableTypeView     TableType = "VIEW"
	TableTypeExternal TableType = "EXTERNAL"
)

func (c *Config) ApplyDefaults() {
	if c.MaxConcurrentRequests == 0 {
		c.MaxConcurrentRequests = 10
	}
	c.IncludeDatasets = true
	c.IncludeTableStats = true
	c.IncludeViews = true
	c.IncludeExternalTables = true
	c.ExcludeSystemDatasets = true
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	log.Debug().Interface("raw_config", rawConfig).Msg("Starting BigQuery config validation")

	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	config.ApplyDefaults()

	if config.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	authMethods := 0
	if config.CredentialsPath != "" {
		authMethods++
	}
	if config.CredentialsJSON != "" {
		authMethods++
	}
	if config.UseDefaultCredentials {
		authMethods++
	}

	if authMethods == 0 {
		return fmt.Errorf("at least one authentication method must be provided: credentials_path, credentials_json, or use_default_credentials")
	}
	if authMethods > 1 {
		return fmt.Errorf("only one authentication method should be provided")
	}

	s.config = config
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if err := s.initClient(ctx); err != nil {
		return nil, fmt.Errorf("initializing BigQuery client: %w", err)
	}
	defer s.closeClient()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	if s.config.IncludeDatasets {
		log.Debug().Msg("Starting dataset discovery")
		datasetAssets, err := s.discoverDatasets(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover datasets")
		} else {
			assets = append(assets, datasetAssets...)
			log.Debug().Int("count", len(datasetAssets)).Msg("Discovered datasets")
		}

		for _, datasetAsset := range datasetAssets {
			if datasetAsset.Type != "Dataset" {
				continue
			}

			datasetID := *datasetAsset.Name
			if s.config.DatasetFilter != nil && !plugin.ShouldIncludeResource(datasetID, *s.config.DatasetFilter) {
				log.Debug().Str("dataset", datasetID).Msg("Skipping dataset due to filter")
				continue
			}

			log.Debug().Str("dataset", datasetID).Msg("Starting table discovery")
			tableAssets, err := s.discoverTables(ctx, datasetID)
			if err != nil {
				log.Warn().Err(err).Str("dataset", datasetID).Msg("Failed to discover tables")
				continue
			}

			assets = append(assets, tableAssets...)
			log.Debug().Int("count", len(tableAssets)).Str("dataset", datasetID).Msg("Discovered tables")

			for _, tableAsset := range tableAssets {
				lineages = append(lineages, lineage.LineageEdge{
					Source: *datasetAsset.MRN,
					Target: *tableAsset.MRN,
					Type:   "CONTAINS",
				})
			}
		}
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) initClient(ctx context.Context) error {
	var opts []option.ClientOption

	if emulatorHost := os.Getenv("BIGQUERY_EMULATOR_HOST"); emulatorHost != "" {
		if !strings.HasPrefix(emulatorHost, "http://") {
			emulatorHost = "http://" + emulatorHost
		}
		opts = append(opts, option.WithEndpoint(emulatorHost))
		opts = append(opts, option.WithoutAuthentication())
	} else if s.config.CredentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(s.config.CredentialsPath))
	} else if s.config.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(s.config.CredentialsJSON)))
	}

	client, err := bigquery.NewClient(ctx, s.config.ProjectID, opts...)
	if err != nil {
		return fmt.Errorf("creating BigQuery client: %w", err)
	}

	s.client = client

	it := client.Datasets(ctx)
	it.PageInfo().MaxSize = 1
	_, err = it.Next()
	if err != nil && err != iterator.Done {
		return fmt.Errorf("testing BigQuery connection: %w", err)
	}

	log.Debug().Str("project_id", s.config.ProjectID).Msg("Successfully connected to BigQuery")
	return nil
}

func (s *Source) closeClient() {
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
}

func (s *Source) discoverDatasets(ctx context.Context) ([]asset.Asset, error) {
	log.Debug().Str("project_id", s.config.ProjectID).Msg("Discovering datasets")

	it := s.client.Datasets(ctx)
	var assets []asset.Asset

	for {
		dataset, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing datasets: %w", err)
		}

		datasetID := dataset.DatasetID

		if s.config.ExcludeSystemDatasets && s.isSystemDataset(datasetID) {
			log.Debug().Str("dataset", datasetID).Msg("Skipping system dataset")
			continue
		}

		if s.config.DatasetFilter != nil && !plugin.ShouldIncludeResource(datasetID, *s.config.DatasetFilter) {
			log.Debug().Str("dataset", datasetID).Msg("Skipping dataset due to filter")
			continue
		}

		datasetRef := s.client.Dataset(datasetID)
		metadata, err := datasetRef.Metadata(ctx)
		if err != nil {
			log.Warn().Err(err).Str("dataset", datasetID).Msg("Failed to get dataset metadata")
			continue
		}

		log.Debug().
			Str("dataset", datasetID).
			Str("location", metadata.Location).
			Time("created", metadata.CreationTime).
			Msg("Found dataset")

		assetMetadata := make(map[string]interface{})
		assetMetadata["project_id"] = s.config.ProjectID
		assetMetadata["dataset_id"] = datasetID
		assetMetadata["location"] = metadata.Location
		assetMetadata["creation_time"] = metadata.CreationTime.Format(time.RFC3339)
		assetMetadata["last_modified"] = metadata.LastModifiedTime.Format(time.RFC3339)

		if metadata.Description != "" {
			assetMetadata["description"] = metadata.Description
		}

		if metadata.DefaultTableExpiration > 0 {
			assetMetadata["default_table_expiration"] = metadata.DefaultTableExpiration.String()
		}

		if metadata.DefaultPartitionExpiration > 0 {
			assetMetadata["default_partition_expiration"] = metadata.DefaultPartitionExpiration.String()
		}

		if len(metadata.Labels) > 0 {
			assetMetadata["labels"] = metadata.Labels
		}

		if len(metadata.Access) > 0 {
			assetMetadata["access_entries_count"] = len(metadata.Access)
		}

		mrnValue := mrn.New("Dataset", "BigQuery", datasetID)
		description := fmt.Sprintf("BigQuery dataset %s in project %s", datasetID, s.config.ProjectID)

		processedTags := plugin.InterpolateTags(s.config.Tags, assetMetadata)

		assets = append(assets, asset.Asset{
			Name:        &datasetID,
			MRN:         &mrnValue,
			Type:        "Dataset",
			Providers:   []string{"BigQuery"},
			Description: &description,
			Metadata:    assetMetadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "BigQuery",
				LastSyncAt: time.Now(),
				Properties: assetMetadata,
				Priority:   1,
			}},
		})
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered datasets")
	return assets, nil
}

func (s *Source) discoverTables(ctx context.Context, datasetID string) ([]asset.Asset, error) {
	dataset := s.client.Dataset(datasetID)
	it := dataset.Tables(ctx)

	var assets []asset.Asset

	for {
		table, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("listing tables in dataset %s: %w", datasetID, err)
		}

		tableID := table.TableID

		if s.config.TableFilter != nil && !plugin.ShouldIncludeResource(tableID, *s.config.TableFilter) {
			log.Debug().Str("table", tableID).Msg("Skipping table due to filter")
			continue
		}

		metadata, err := table.Metadata(ctx)
		if err != nil {
			log.Warn().Err(err).Str("table", tableID).Str("dataset", datasetID).Msg("Failed to get table metadata")
			continue
		}

		tableType := s.getTableType(metadata)

		if tableType == TableTypeView && !s.config.IncludeViews {
			continue
		}

		if tableType == TableTypeExternal && !s.config.IncludeExternalTables {
			continue
		}

		log.Debug().
			Str("table", tableID).
			Str("dataset", datasetID).
			Str("type", string(tableType)).
			Msg("Found table")

		assetMetadata := make(map[string]interface{})
		assetMetadata["project_id"] = s.config.ProjectID
		assetMetadata["dataset_id"] = datasetID
		assetMetadata["table_id"] = tableID
		assetMetadata["table_type"] = string(tableType)
		assetMetadata["creation_time"] = metadata.CreationTime.Format(time.RFC3339)
		assetMetadata["last_modified"] = metadata.LastModifiedTime.Format(time.RFC3339)

		if metadata.Description != "" {
			assetMetadata["description"] = metadata.Description
		}

		if !metadata.ExpirationTime.IsZero() {
			assetMetadata["expiration_time"] = metadata.ExpirationTime.Format(time.RFC3339)
		}

		if len(metadata.Labels) > 0 {
			assetMetadata["labels"] = metadata.Labels
		}

		if s.config.IncludeTableStats && tableType == TableTypeTable {
			if metadata.NumRows > 0 {
				assetMetadata["num_rows"] = metadata.NumRows
			}
			if metadata.NumBytes > 0 {
				assetMetadata["num_bytes"] = metadata.NumBytes
			}
		}

		if metadata.TimePartitioning != nil {
			assetMetadata["time_partitioning_type"] = string(metadata.TimePartitioning.Type)
			if metadata.TimePartitioning.Field != "" {
				assetMetadata["time_partitioning_field"] = metadata.TimePartitioning.Field
			}
			if metadata.TimePartitioning.Expiration > 0 {
				assetMetadata["partition_expiration"] = metadata.TimePartitioning.Expiration.String()
			}
		}

		if metadata.RangePartitioning != nil {
			assetMetadata["range_partitioning_field"] = metadata.RangePartitioning.Field
		}

		if metadata.Clustering != nil && len(metadata.Clustering.Fields) > 0 {
			assetMetadata["clustering_fields"] = metadata.Clustering.Fields
		}

		if tableType == TableTypeView && metadata.ViewQuery != "" {
			assetMetadata["view_query"] = metadata.ViewQuery
		}

		if tableType == TableTypeExternal && metadata.ExternalDataConfig != nil {
			externalConfig := make(map[string]interface{})
			externalConfig["source_format"] = string(metadata.ExternalDataConfig.SourceFormat)
			if len(metadata.ExternalDataConfig.SourceURIs) > 0 {
				externalConfig["source_uris"] = metadata.ExternalDataConfig.SourceURIs
			}
			assetMetadata["external_data_config"] = externalConfig
		}

		var assetType string
		var assetDesc string

		switch tableType {
		case TableTypeTable:
			assetType = "Table"
			assetDesc = fmt.Sprintf("BigQuery table %s.%s in project %s", datasetID, tableID, s.config.ProjectID)
		case TableTypeView:
			assetType = "View"
			assetDesc = fmt.Sprintf("BigQuery view %s.%s in project %s", datasetID, tableID, s.config.ProjectID)
		case TableTypeExternal:
			assetType = "ExternalTable"
			assetDesc = fmt.Sprintf("BigQuery external table %s.%s in project %s", datasetID, tableID, s.config.ProjectID)
		default:
			continue
		}

		mrnValue := mrn.New(assetType, "BigQuery", tableID)

		processedTags := plugin.InterpolateTags(s.config.Tags, assetMetadata)

		var schema map[string]string
		if metadata.Schema != nil && (tableType == TableTypeTable || tableType == TableTypeView) {
			jsonSchema := s.generateJSONSchema(metadata.Schema, tableID)
			schemaBytes, _ := json.Marshal(jsonSchema)
			schema = map[string]string{"json_schema": string(schemaBytes)}
		}

		assets = append(assets, asset.Asset{
			Name:        &tableID,
			MRN:         &mrnValue,
			Type:        assetType,
			Providers:   []string{"BigQuery"},
			Description: &assetDesc,
			Metadata:    assetMetadata,
			Schema:      schema,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "BigQuery",
				LastSyncAt: time.Now(),
				Properties: assetMetadata,
				Priority:   1,
			}},
		})
	}

	return assets, nil
}

func (s *Source) getTableType(metadata *bigquery.TableMetadata) TableType {
	if metadata.ViewQuery != "" {
		return TableTypeView
	}
	if metadata.ExternalDataConfig != nil {
		return TableTypeExternal
	}
	return TableTypeTable
}

func (s *Source) isSystemDataset(datasetID string) bool {
	systemPrefixes := []string{
		"_script",
		"_analytics",
		"_bqmetadata",
		"_vt",
		"__REALTIME",
		"_usage",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(datasetID, prefix) {
			return true
		}
	}

	return false
}

func (s *Source) generateJSONSchema(bqSchema bigquery.Schema, tableName string) map[string]interface{} {
	schema := map[string]interface{}{
		"$schema":    "http://json-schema.org/draft-07/schema#",
		"type":       "object",
		"title":      tableName,
		"properties": s.convertFieldsToProperties(bqSchema),
		"required":   s.extractRequiredFields(bqSchema),
	}
	return schema
}

func (s *Source) convertFieldsToProperties(fields bigquery.Schema) map[string]interface{} {
	properties := make(map[string]interface{})

	for _, field := range fields {
		properties[field.Name] = s.convertFieldToProperty(field)
	}

	return properties
}

func (s *Source) convertFieldToProperty(field *bigquery.FieldSchema) map[string]interface{} {
	property := make(map[string]interface{})

	if field.Description != "" {
		property["description"] = field.Description
	}

	if field.Repeated {
		property["type"] = "array"
		property["items"] = s.convertBigQueryTypeToJSONType(field.Type, field.Schema)
		return property
	}

	return s.convertBigQueryTypeToJSONType(field.Type, field.Schema)
}

func (s *Source) convertBigQueryTypeToJSONType(bqType bigquery.FieldType, schema bigquery.Schema) map[string]interface{} {
	switch bqType {
	case bigquery.StringFieldType:
		return map[string]interface{}{"type": "string"}
	case bigquery.IntegerFieldType:
		return map[string]interface{}{"type": "integer"}
	case bigquery.FloatFieldType:
		return map[string]interface{}{"type": "number"}
	case bigquery.BooleanFieldType:
		return map[string]interface{}{"type": "boolean"}
	case bigquery.TimestampFieldType:
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	case bigquery.DateFieldType:
		return map[string]interface{}{
			"type":   "string",
			"format": "date",
		}
	case bigquery.TimeFieldType:
		return map[string]interface{}{
			"type":   "string",
			"format": "time",
		}
	case bigquery.DateTimeFieldType:
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	case bigquery.NumericFieldType, bigquery.BigNumericFieldType:
		return map[string]interface{}{
			"type":        "string",
			"description": "Numeric value as string to preserve precision",
		}
	case bigquery.BytesFieldType:
		return map[string]interface{}{
			"type":            "string",
			"contentEncoding": "base64",
		}
	case bigquery.RecordFieldType:
		return map[string]interface{}{
			"type":       "object",
			"properties": s.convertFieldsToProperties(schema),
			"required":   s.extractRequiredFields(schema),
		}
	case bigquery.GeographyFieldType:
		return map[string]interface{}{
			"type":        "string",
			"description": "Geography in WKT format",
		}
	case bigquery.JSONFieldType:
		return map[string]interface{}{
			"type": "object",
		}
	default:
		return map[string]interface{}{
			"type":        "string",
			"description": fmt.Sprintf("BigQuery type: %s", bqType),
		}
	}
}

func (s *Source) extractRequiredFields(fields bigquery.Schema) []string {
	var required []string
	for _, field := range fields {
		if field.Required {
			required = append(required, field.Name)
		}
	}
	return required
}
