// +marmot:name=Glue
// +marmot:description=This plugin discovers jobs, databases, tables and crawlers from AWS Glue.
// +marmot:status=experimental
// +marmot:features=Assets
package glue

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for Glue plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`
	*plugin.AWSConfig `json:",inline"`

	DiscoverJobs      bool `json:"discover_jobs" description:"Whether to discover Glue jobs" default:"true"`
	DiscoverDatabases bool `json:"discover_databases" description:"Whether to discover Glue databases" default:"true"`
	DiscoverTables    bool `json:"discover_tables" description:"Whether to discover Glue tables" default:"true"`
	DiscoverCrawlers  bool `json:"discover_crawlers" description:"Whether to discover Glue crawlers" default:"true"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
credentials:
  region: "us-east-1"
  profile: "production"
  role: "<role>"
tags:
  - "aws"
discover_jobs: true
discover_databases: true
discover_tables: true
discover_crawlers: true
`

type Source struct {
	config *Config
	client *glue.Client
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if _, ok := rawConfig["discover_jobs"]; !ok {
		config.DiscoverJobs = true
	}
	if _, ok := rawConfig["discover_databases"]; !ok {
		config.DiscoverDatabases = true
	}
	if _, ok := rawConfig["discover_tables"]; !ok {
		config.DiscoverTables = true
	}
	if _, ok := rawConfig["discover_crawlers"]; !ok {
		config.DiscoverCrawlers = true
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

	if _, ok := pluginConfig["discover_jobs"]; !ok {
		config.DiscoverJobs = true
	}
	if _, ok := pluginConfig["discover_databases"]; !ok {
		config.DiscoverDatabases = true
	}
	if _, ok := pluginConfig["discover_tables"]; !ok {
		config.DiscoverTables = true
	}
	if _, ok := pluginConfig["discover_crawlers"]; !ok {
		config.DiscoverCrawlers = true
	}

	s.config = config

	awsConfig, err := plugin.ExtractAWSConfig(pluginConfig)
	if err != nil {
		return nil, fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating AWS config: %w", err)
	}

	s.client = glue.NewFromConfig(awsCfg)

	var allAssets []asset.Asset

	if config.DiscoverJobs {
		jobs, err := s.discoverJobs(ctx)
		if err != nil {
			return nil, fmt.Errorf("discovering jobs: %w", err)
		}
		allAssets = append(allAssets, jobs...)
	}

	var dbNames []string
	if config.DiscoverDatabases || config.DiscoverTables {
		databases, names, err := s.discoverDatabases(ctx)
		if err != nil {
			return nil, fmt.Errorf("discovering databases: %w", err)
		}
		allAssets = append(allAssets, databases...)
		dbNames = names
	}

	if config.DiscoverTables {
		tables, err := s.discoverTables(ctx, dbNames)
		if err != nil {
			return nil, fmt.Errorf("discovering tables: %w", err)
		}
		allAssets = append(allAssets, tables...)
	}

	if config.DiscoverCrawlers {
		crawlers, err := s.discoverCrawlers(ctx)
		if err != nil {
			return nil, fmt.Errorf("discovering crawlers: %w", err)
		}
		allAssets = append(allAssets, crawlers...)
	}

	return &plugin.DiscoveryResult{
		Assets: allAssets,
	}, nil
}

func (s *Source) discoverJobs(ctx context.Context) ([]asset.Asset, error) {
	var assets []asset.Asset
	paginator := glue.NewGetJobsPaginator(s.client, &glue.GetJobsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing jobs: %w", err)
		}

		for _, job := range output.Jobs {
			a := s.createJobAsset(job)
			assets = append(assets, a)
		}
	}

	return assets, nil
}

func (s *Source) createJobAsset(job types.Job) asset.Asset {
	metadata := make(map[string]interface{})

	name := ""
	if job.Name != nil {
		name = *job.Name
	}

	if job.Role != nil {
		metadata["role"] = *job.Role
	}
	if job.Command != nil && job.Command.Name != nil {
		metadata["type"] = *job.Command.Name
	}
	if job.GlueVersion != nil {
		metadata["glue_version"] = *job.GlueVersion
	}
	if job.WorkerType != "" {
		metadata["worker_type"] = string(job.WorkerType)
	}
	if job.NumberOfWorkers != nil {
		metadata["number_of_workers"] = *job.NumberOfWorkers
	}
	if job.MaxCapacity != nil {
		metadata["max_capacity"] = *job.MaxCapacity
	}
	if job.Timeout != nil {
		metadata["timeout"] = *job.Timeout
	}
	if job.MaxRetries != 0 {
		metadata["max_retries"] = job.MaxRetries
	}
	if job.Command != nil && job.Command.ScriptLocation != nil {
		metadata["script_location"] = *job.Command.ScriptLocation
	}
	if job.Connections != nil && len(job.Connections.Connections) > 0 {
		metadata["connections"] = strings.Join(job.Connections.Connections, ", ")
	}
	if job.CreatedOn != nil {
		metadata["created_on"] = job.CreatedOn.Format(time.RFC3339)
	}
	if job.LastModifiedOn != nil {
		metadata["last_modified_on"] = job.LastModifiedOn.Format(time.RFC3339)
	}
	if job.SecurityConfiguration != nil && *job.SecurityConfiguration != "" {
		metadata["security_configuration"] = *job.SecurityConfiguration
	}

	var description *string
	if job.Description != nil && *job.Description != "" {
		description = job.Description
	}

	mrnValue := mrn.New("Job", "Glue", name)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Job",
		Providers:   []string{"Glue"},
		Description: description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Glue",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) discoverDatabases(ctx context.Context) ([]asset.Asset, []string, error) {
	var assets []asset.Asset
	var dbNames []string
	paginator := glue.NewGetDatabasesPaginator(s.client, &glue.GetDatabasesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("listing databases: %w", err)
		}

		for _, db := range output.DatabaseList {
			name := ""
			if db.Name != nil {
				name = *db.Name
			}
			dbNames = append(dbNames, name)

			if s.config.DiscoverDatabases {
				a := s.createDatabaseAsset(db)
				assets = append(assets, a)
			}
		}
	}

	return assets, dbNames, nil
}

func (s *Source) createDatabaseAsset(db types.Database) asset.Asset {
	metadata := make(map[string]interface{})

	name := ""
	if db.Name != nil {
		name = *db.Name
	}

	if db.CatalogId != nil {
		metadata["catalog_id"] = *db.CatalogId
	}
	if db.LocationUri != nil && *db.LocationUri != "" {
		metadata["location_uri"] = *db.LocationUri
	}
	if db.Description != nil && *db.Description != "" {
		metadata["description"] = *db.Description
	}
	if db.CreateTime != nil {
		metadata["create_time"] = db.CreateTime.Format(time.RFC3339)
	}
	if len(db.Parameters) > 0 {
		metadata["parameters"] = formatParameters(db.Parameters)
	}

	var description *string
	if db.Description != nil && *db.Description != "" {
		description = db.Description
	}

	mrnValue := mrn.New("Database", "Glue", name)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Database",
		Providers:   []string{"Glue"},
		Description: description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Glue",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) discoverTables(ctx context.Context, dbNames []string) ([]asset.Asset, error) {
	var assets []asset.Asset

	for _, dbName := range dbNames {
		paginator := glue.NewGetTablesPaginator(s.client, &glue.GetTablesInput{
			DatabaseName: &dbName,
		})

		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				log.Warn().Err(err).Str("database", dbName).Msg("Failed to list tables in database")
				break
			}

			for _, table := range output.TableList {
				if isIcebergTable(table) {
					continue
				}
				a := s.createTableAsset(dbName, table)
				assets = append(assets, a)
			}
		}
	}

	return assets, nil
}

func isIcebergTable(table types.Table) bool {
	if table.Parameters == nil {
		return false
	}
	tableType, ok := table.Parameters["table_type"]
	if !ok {
		return false
	}
	return strings.EqualFold(tableType, "ICEBERG")
}

func (s *Source) createTableAsset(dbName string, table types.Table) asset.Asset {
	metadata := make(map[string]interface{})

	tableName := ""
	if table.Name != nil {
		tableName = *table.Name
	}

	metadata["database_name"] = dbName

	if table.TableType != nil && *table.TableType != "" {
		metadata["table_type"] = *table.TableType
	}
	if table.Parameters != nil {
		if classification, ok := table.Parameters["classification"]; ok {
			metadata["classification"] = classification
		}
	}
	if table.Owner != nil && *table.Owner != "" {
		metadata["owner"] = *table.Owner
	}
	if table.StorageDescriptor != nil {
		if table.StorageDescriptor.Location != nil && *table.StorageDescriptor.Location != "" {
			metadata["location"] = *table.StorageDescriptor.Location
		}
		if table.StorageDescriptor.InputFormat != nil && *table.StorageDescriptor.InputFormat != "" {
			metadata["input_format"] = *table.StorageDescriptor.InputFormat
		}
		if table.StorageDescriptor.OutputFormat != nil && *table.StorageDescriptor.OutputFormat != "" {
			metadata["output_format"] = *table.StorageDescriptor.OutputFormat
		}
		if table.StorageDescriptor.SerdeInfo != nil && table.StorageDescriptor.SerdeInfo.SerializationLibrary != nil {
			metadata["serde"] = *table.StorageDescriptor.SerdeInfo.SerializationLibrary
		}
	}
	if len(table.PartitionKeys) > 0 {
		var partKeys []string
		for _, pk := range table.PartitionKeys {
			if pk.Name != nil {
				partKeys = append(partKeys, *pk.Name)
			}
		}
		metadata["partition_keys"] = strings.Join(partKeys, ", ")
	}
	if table.CreateTime != nil {
		metadata["create_time"] = table.CreateTime.Format(time.RFC3339)
	}
	if table.UpdateTime != nil {
		metadata["update_time"] = table.UpdateTime.Format(time.RFC3339)
	}
	if table.Retention != 0 {
		metadata["retention"] = table.Retention
	}

	var description *string
	if table.Description != nil && *table.Description != "" {
		description = table.Description
	}

	var schema map[string]string
	if columns := buildColumnSchema(table); columns != nil {
		jsonBytes, err := json.Marshal(columns)
		if err == nil {
			schema = map[string]string{"columns": string(jsonBytes)}
		}
	}

	qualifiedName := dbName + "." + tableName
	mrnValue := mrn.New("Table", "Glue", qualifiedName)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &tableName,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"Glue"},
		Description: description,
		Metadata:    metadata,
		Schema:      schema,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Glue",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) discoverCrawlers(ctx context.Context) ([]asset.Asset, error) {
	var assets []asset.Asset
	paginator := glue.NewGetCrawlersPaginator(s.client, &glue.GetCrawlersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing crawlers: %w", err)
		}

		for _, crawler := range output.Crawlers {
			a := s.createCrawlerAsset(crawler)
			assets = append(assets, a)
		}
	}

	return assets, nil
}

func (s *Source) createCrawlerAsset(crawler types.Crawler) asset.Asset {
	metadata := make(map[string]interface{})

	name := ""
	if crawler.Name != nil {
		name = *crawler.Name
	}

	if crawler.Role != nil {
		metadata["role"] = *crawler.Role
	}
	if crawler.DatabaseName != nil && *crawler.DatabaseName != "" {
		metadata["database_name"] = *crawler.DatabaseName
	}
	if crawler.State != "" {
		metadata["state"] = string(crawler.State)
	}
	if crawler.Schedule != nil && crawler.Schedule.ScheduleExpression != nil {
		metadata["schedule"] = *crawler.Schedule.ScheduleExpression
	}
	if crawler.Targets != nil {
		metadata["targets"] = summarizeCrawlerTargets(crawler.Targets)
	}
	if crawler.SchemaChangePolicy != nil {
		if crawler.SchemaChangePolicy.UpdateBehavior != "" {
			metadata["schema_update_behavior"] = string(crawler.SchemaChangePolicy.UpdateBehavior)
		}
		if crawler.SchemaChangePolicy.DeleteBehavior != "" {
			metadata["schema_delete_behavior"] = string(crawler.SchemaChangePolicy.DeleteBehavior)
		}
	}
	if crawler.RecrawlPolicy != nil && crawler.RecrawlPolicy.RecrawlBehavior != "" {
		metadata["recrawl_behavior"] = string(crawler.RecrawlPolicy.RecrawlBehavior)
	}
	if crawler.CreationTime != nil {
		metadata["creation_time"] = crawler.CreationTime.Format(time.RFC3339)
	}
	if crawler.LastUpdated != nil {
		metadata["last_updated"] = crawler.LastUpdated.Format(time.RFC3339)
	}
	if crawler.LastCrawl != nil {
		if crawler.LastCrawl.Status != "" {
			metadata["last_crawl_status"] = string(crawler.LastCrawl.Status)
		}
		if crawler.LastCrawl.StartTime != nil {
			metadata["last_crawl_time"] = crawler.LastCrawl.StartTime.Format(time.RFC3339)
		}
		if crawler.LastCrawl.ErrorMessage != nil && *crawler.LastCrawl.ErrorMessage != "" {
			metadata["last_crawl_error"] = *crawler.LastCrawl.ErrorMessage
		}
	}
	if len(crawler.Classifiers) > 0 {
		metadata["classifiers"] = strings.Join(crawler.Classifiers, ", ")
	}

	var description *string
	if crawler.Description != nil && *crawler.Description != "" {
		description = crawler.Description
	}

	mrnValue := mrn.New("Crawler", "Glue", name)
	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        "Crawler",
		Providers:   []string{"Glue"},
		Description: description,
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Glue",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func buildColumnSchema(table types.Table) []map[string]interface{} {
	var columns []map[string]interface{}

	if table.StorageDescriptor != nil {
		for _, col := range table.StorageDescriptor.Columns {
			entry := map[string]interface{}{
				"column_name": safeStr(col.Name),
				"data_type":   safeStr(col.Type),
			}
			if col.Comment != nil && *col.Comment != "" {
				entry["comment"] = *col.Comment
			}
			columns = append(columns, entry)
		}
	}

	for _, pk := range table.PartitionKeys {
		entry := map[string]interface{}{
			"column_name":  safeStr(pk.Name),
			"data_type":    safeStr(pk.Type),
			"is_partition": true,
		}
		if pk.Comment != nil && *pk.Comment != "" {
			entry["comment"] = *pk.Comment
		}
		columns = append(columns, entry)
	}

	if len(columns) == 0 {
		return nil
	}
	return columns
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func summarizeCrawlerTargets(targets *types.CrawlerTargets) string {
	var parts []string

	if len(targets.S3Targets) > 0 {
		parts = append(parts, fmt.Sprintf("%d S3 target(s)", len(targets.S3Targets)))
	}
	if len(targets.JdbcTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d JDBC target(s)", len(targets.JdbcTargets)))
	}
	if len(targets.DynamoDBTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d DynamoDB target(s)", len(targets.DynamoDBTargets)))
	}
	if len(targets.CatalogTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d Catalog target(s)", len(targets.CatalogTargets)))
	}
	if len(targets.DeltaTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d Delta target(s)", len(targets.DeltaTargets)))
	}
	if len(targets.IcebergTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d Iceberg target(s)", len(targets.IcebergTargets)))
	}
	if len(targets.HudiTargets) > 0 {
		parts = append(parts, fmt.Sprintf("%d Hudi target(s)", len(targets.HudiTargets)))
	}

	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func formatParameters(params map[string]string) string {
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "glue",
		Name:        "AWS Glue",
		Description: "Discover jobs, databases, tables and crawlers from AWS Glue",
		Icon:        "glue",
		Category:    "etl",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Glue plugin")
	}
}
