// +marmot:name=DBT
// +marmot:description=This plugin ingests metadata from DBT (Data Build Tool) projects, including models, tests, and lineage.
// +marmot:status=experimental
package dbt

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for DBT plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Source configuration - either local or S3
	SourceType string `json:"source_type" description:"Source type for DBT artifacts (local or s3)" validate:"required,oneof=local s3"`

	// Local filesystem configuration
	TargetPath string `json:"target_path,omitempty" description:"Path to DBT target directory containing manifest.json, catalog.json, etc." validate:"required_if=SourceType local"`

	// S3 configuration
	*plugin.AWSConfig `json:",inline"`
	S3Bucket          string `json:"s3_bucket,omitempty" description:"S3 bucket containing DBT artifacts" validate:"required_if=SourceType s3"`
	S3Prefix          string `json:"s3_prefix,omitempty" description:"S3 prefix/folder for DBT artifacts (optional)"`

	// DBT project configuration
	ProjectName string `json:"project_name" description:"DBT project name" validate:"required"`
	Environment string `json:"environment,omitempty" description:"Environment name (e.g., production, staging)" default:"production"`

	// Artifact configuration
	IncludeManifest    bool `json:"include_manifest" description:"Include manifest.json for model definitions" default:"true"`
	IncludeCatalog     bool `json:"include_catalog" description:"Include catalog.json for table/column descriptions" default:"true"`
	IncludeRunResults  bool `json:"include_run_results" description:"Include run_results.json for test results" default:"false"`
	IncludeSourcesJSON bool `json:"include_sources_json" description:"Include sources.json for source definitions" default:"false"`

	// Discovery configuration
	DiscoverModels  bool           `json:"discover_models" description:"Discover DBT models" default:"true"`
	DiscoverSources bool           `json:"discover_sources" description:"Discover DBT sources" default:"true"`
	DiscoverTests   bool           `json:"discover_tests" description:"Discover DBT tests" default:"false"`
	ModelFilter     *plugin.Filter `json:"model_filter,omitempty" description:"Filter configuration for models"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
source_type: "local"
target_path: "/path/to/dbt/project/target"
project_name: "analytics"
environment: "production"
tags:
  - "dbt"
  - "analytics"
`

// DBT artifact structures
type DBTManifest struct {
	Metadata     ManifestMetadata        `json:"metadata"`
	Nodes        map[string]ManifestNode `json:"nodes"`
	Sources      map[string]ManifestNode `json:"sources"`
	Macros       map[string]interface{}  `json:"macros"`
	ChildMap     map[string][]string     `json:"child_map"`
	ParentMap    map[string][]string     `json:"parent_map"`
	Exposures    map[string]interface{}  `json:"exposures"`
	Metrics      map[string]interface{}  `json:"metrics"`
	Dependencies map[string]interface{}  `json:"dependencies"`
}

type ManifestMetadata struct {
	DBTVersion   string    `json:"dbt_version"`
	GeneratedAt  time.Time `json:"generated_at"`
	AdapterType  string    `json:"adapter_type"`
	ProjectName  string    `json:"project_name"`
	InvocationID string    `json:"invocation_id"`
}

type ManifestNode struct {
	UniqueID     string                 `json:"unique_id"`
	Name         string                 `json:"name"`
	ResourceType string                 `json:"resource_type"`
	PackageName  string                 `json:"package_name"`
	Path         string                 `json:"path"`
	OriginalPath string                 `json:"original_file_path"`
	Database     string                 `json:"database"`
	Schema       string                 `json:"schema"`
	Alias        string                 `json:"alias"`
	Description  string                 `json:"description"`
	Columns      map[string]NodeColumn  `json:"columns"`
	Tags         []string               `json:"tags"`
	Meta         map[string]interface{} `json:"meta"`
	DependsOn    NodeDependency         `json:"depends_on"`
	Config       map[string]interface{} `json:"config"`
	CompiledSQL  string                 `json:"compiled_sql"`
	CompiledCode string                 `json:"compiled_code"`
	RawSQL       string                 `json:"raw_sql"`
	RawCode      string                 `json:"raw_code"`
	Materialized string                 `json:"materialized"`
}

type NodeColumn struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Meta        map[string]interface{} `json:"meta"`
	Tags        []string               `json:"tags"`
	DataType    string                 `json:"data_type"`
}

type NodeDependency struct {
	Nodes  []string `json:"nodes"`
	Macros []string `json:"macros"`
}

type DBTCatalog struct {
	Metadata ManifestMetadata         `json:"metadata"`
	Sources  map[string]CatalogNode   `json:"sources"`
	Nodes    map[string]CatalogNode   `json:"nodes"`
	Errors   []map[string]interface{} `json:"errors"`
}

type CatalogNode struct {
	Metadata     CatalogMetadata          `json:"metadata"`
	Columns      map[string]CatalogColumn `json:"columns"`
	Stats        map[string]CatalogStat   `json:"stats"`
	UniqueID     string                   `json:"unique_id"`
	ResourceType string                   `json:"resource_type"`
}

type CatalogMetadata struct {
	Type     string `json:"type"`
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	Comment  string `json:"comment"`
}

type CatalogColumn struct {
	Type    string `json:"type"`
	Comment string `json:"comment"`
	Index   int    `json:"index"`
	Name    string `json:"name"`
}

type CatalogStat struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
	Include     bool        `json:"include"`
}

type DBTRunResults struct {
	Metadata      ManifestMetadata `json:"metadata"`
	Results       []RunResult      `json:"results"`
	ElapsedTime   float64          `json:"elapsed_time"`
	Args          interface{}      `json:"args"`
	GeneratedAt   time.Time        `json:"generated_at"`
	SuccessStatus string           `json:"success"`
}

type RunResult struct {
	UniqueID       string                 `json:"unique_id"`
	Status         string                 `json:"status"`
	ExecutionTime  float64                `json:"execution_time"`
	Message        string                 `json:"message"`
	Failures       int                    `json:"failures"`
	AdapterResponse map[string]interface{} `json:"adapter_response"`
	Thread         string                 `json:"thread_id"`
}

type Source struct {
	config    *Config
	s3Client  *s3.Client
	manifest  *DBTManifest
	catalog   *DBTCatalog
	runResults *DBTRunResults
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Set defaults
	if config.Environment == "" {
		config.Environment = "production"
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	// Validate source-specific requirements
	if config.SourceType == "local" {
		if config.TargetPath == "" {
			return nil, fmt.Errorf("target_path is required when source_type is local")
		}
	} else if config.SourceType == "s3" {
		if config.S3Bucket == "" {
			return nil, fmt.Errorf("s3_bucket is required when source_type is s3")
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
	s.config = config

	// Initialize S3 client if needed
	if config.SourceType == "s3" {
		if err := s.initS3Client(ctx); err != nil {
			return nil, fmt.Errorf("initializing S3 client: %w", err)
		}
	}

	// Load DBT artifacts
	if err := s.loadArtifacts(ctx); err != nil {
		return nil, fmt.Errorf("loading DBT artifacts: %w", err)
	}

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	// Discover models
	if config.DiscoverModels && s.manifest != nil {
		modelAssets, modelLineages := s.discoverModels()
		assets = append(assets, modelAssets...)
		lineages = append(lineages, modelLineages...)
	}

	// Discover sources
	if config.DiscoverSources && s.manifest != nil {
		sourceAssets := s.discoverSources()
		assets = append(assets, sourceAssets...)
	}

	log.Info().
		Int("models", len(assets)).
		Int("lineages", len(lineages)).
		Msg("DBT discovery completed")

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) initS3Client(ctx context.Context) error {
	awsConfig, err := plugin.ExtractAWSConfig(plugin.RawPluginConfig(map[string]interface{}{
		"credentials": s.config.AWSConfig,
	}))
	if err != nil {
		return fmt.Errorf("extracting AWS config: %w", err)
	}

	awsCfg, err := awsConfig.NewAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("creating AWS config: %w", err)
	}

	s.s3Client = s3.NewFromConfig(awsCfg)
	return nil
}

func (s *Source) loadArtifacts(ctx context.Context) error {
	// Load manifest.json
	if s.config.IncludeManifest {
		manifestData, err := s.readArtifact(ctx, "manifest.json")
		if err != nil {
			return fmt.Errorf("reading manifest.json: %w", err)
		}

		var manifest DBTManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return fmt.Errorf("parsing manifest.json: %w", err)
		}
		s.manifest = &manifest
		log.Debug().Int("nodes", len(manifest.Nodes)).Msg("Loaded manifest.json")
	}

	// Load catalog.json
	if s.config.IncludeCatalog {
		catalogData, err := s.readArtifact(ctx, "catalog.json")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to read catalog.json, continuing without it")
		} else {
			var catalog DBTCatalog
			if err := json.Unmarshal(catalogData, &catalog); err != nil {
				log.Warn().Err(err).Msg("Failed to parse catalog.json")
			} else {
				s.catalog = &catalog
				log.Debug().Int("nodes", len(catalog.Nodes)).Msg("Loaded catalog.json")
			}
		}
	}

	// Load run_results.json
	if s.config.IncludeRunResults {
		runResultsData, err := s.readArtifact(ctx, "run_results.json")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to read run_results.json, continuing without it")
		} else {
			var runResults DBTRunResults
			if err := json.Unmarshal(runResultsData, &runResults); err != nil {
				log.Warn().Err(err).Msg("Failed to parse run_results.json")
			} else {
				s.runResults = &runResults
				log.Debug().Int("results", len(runResults.Results)).Msg("Loaded run_results.json")
			}
		}
	}

	return nil
}

func (s *Source) readArtifact(ctx context.Context, filename string) ([]byte, error) {
	if s.config.SourceType == "local" {
		return s.readLocalArtifact(filename)
	}
	return s.readS3Artifact(ctx, filename)
}

func (s *Source) readLocalArtifact(filename string) ([]byte, error) {
	path := filepath.Join(s.config.TargetPath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return data, nil
}

func (s *Source) readS3Artifact(ctx context.Context, filename string) ([]byte, error) {
	key := filename
	if s.config.S3Prefix != "" {
		key = filepath.Join(s.config.S3Prefix, filename)
	}

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.config.S3Bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("getting S3 object s3://%s/%s: %w", s.config.S3Bucket, key, err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("reading S3 object body: %w", err)
	}

	return data, nil
}

func (s *Source) discoverModels() ([]asset.Asset, []lineage.LineageEdge) {
	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	for nodeID, node := range s.manifest.Nodes {
		// Only process models
		if node.ResourceType != "model" {
			continue
		}

		// Apply model filter
		if s.config.ModelFilter != nil {
			if !plugin.ShouldIncludeResource(node.Name, *s.config.ModelFilter) {
				log.Debug().Str("model", node.Name).Msg("Skipping model due to filter")
				continue
			}
		}

		asset := s.createModelAsset(node, nodeID)
		assets = append(assets, asset)

		// Create lineage from dependencies
		modelLineages := s.createModelLineage(node)
		lineages = append(lineages, modelLineages...)
	}

	return assets, lineages
}

func (s *Source) createModelAsset(node ManifestNode, nodeID string) asset.Asset {
	// Determine the full table name
	tableName := node.Name
	if node.Alias != "" {
		tableName = node.Alias
	}

	fullyQualifiedName := fmt.Sprintf("%s.%s.%s", node.Database, node.Schema, tableName)

	// Build metadata
	metadata := make(map[string]interface{})
	metadata["dbt_unique_id"] = node.UniqueID
	metadata["dbt_package"] = node.PackageName
	metadata["dbt_path"] = node.Path
	metadata["dbt_original_path"] = node.OriginalPath
	metadata["dbt_materialized"] = node.Materialized
	metadata["database"] = node.Database
	metadata["schema"] = node.Schema
	metadata["table_name"] = tableName
	metadata["fully_qualified_name"] = fullyQualifiedName
	metadata["project_name"] = s.config.ProjectName
	metadata["environment"] = s.config.Environment

	if s.manifest.Metadata.AdapterType != "" {
		metadata["adapter_type"] = s.manifest.Metadata.AdapterType
	}
	if s.manifest.Metadata.DBTVersion != "" {
		metadata["dbt_version"] = s.manifest.Metadata.DBTVersion
	}

	// Add config metadata
	for k, v := range node.Config {
		metadata[fmt.Sprintf("config_%s", k)] = v
	}

	// Add custom meta
	for k, v := range node.Meta {
		metadata[fmt.Sprintf("meta_%s", k)] = v
	}

	// Merge catalog metadata if available
	if s.catalog != nil {
		if catalogNode, exists := s.catalog.Nodes[nodeID]; exists {
			if catalogNode.Metadata.Owner != "" {
				metadata["owner"] = catalogNode.Metadata.Owner
			}
			if catalogNode.Metadata.Comment != "" {
				metadata["catalog_comment"] = catalogNode.Metadata.Comment
			}

			// Add table stats
			for statKey, stat := range catalogNode.Stats {
				if stat.Include {
					metadata[fmt.Sprintf("stat_%s", statKey)] = stat.Value
				}
			}
		}
	}

	// Add test results if available
	if s.runResults != nil {
		for _, result := range s.runResults.Results {
			if result.UniqueID == node.UniqueID {
				metadata["last_run_status"] = result.Status
				metadata["last_run_execution_time"] = result.ExecutionTime
				if result.Message != "" {
					metadata["last_run_message"] = result.Message
				}
				if result.Failures > 0 {
					metadata["last_run_failures"] = result.Failures
				}
				break
			}
		}
	}

	// Build schema information as map[string]string (column name -> type)
	schema := make(map[string]string)
	if len(node.Columns) > 0 {
		for _, col := range node.Columns {
			if col.DataType != "" {
				schema[col.Name] = col.DataType
			}
			// Store column description in metadata
			if col.Description != "" {
				metadata[fmt.Sprintf("column_%s_description", col.Name)] = col.Description
			}
			// Store column tags in metadata
			if len(col.Tags) > 0 {
				metadata[fmt.Sprintf("column_%s_tags", col.Name)] = col.Tags
			}
		}
	}

	// Merge catalog column information
	if s.catalog != nil {
		if catalogNode, exists := s.catalog.Nodes[nodeID]; exists {
			// Add or update with catalog columns
			for _, catalogCol := range catalogNode.Columns {
				if catalogCol.Type != "" {
					schema[catalogCol.Name] = catalogCol.Type
				}
				// Store catalog comment in metadata if description not already set
				if catalogCol.Comment != "" {
					key := fmt.Sprintf("column_%s_description", catalogCol.Name)
					if _, exists := metadata[key]; !exists {
						metadata[key] = catalogCol.Comment
					}
				}
			}
		}
	}

	// Combine tags from config
	allTags := append([]string{}, node.Tags...)
	allTags = append(allTags, s.config.Tags...)

	// Create MRN
	mrnValue := mrn.New("Table", "DBT", fullyQualifiedName)

	description := node.Description
	if description == "" {
		description = fmt.Sprintf("DBT model: %s", tableName)
	}

	return asset.Asset{
		Name:        &tableName,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"DBT"},
		Description: &description,
		Metadata:    metadata,
		Tags:        allTags,
		Schema:      schema,
		Sources: []asset.AssetSource{{
			Name:       "DBT",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) createModelLineage(node ManifestNode) []lineage.LineageEdge {
	var lineages []lineage.LineageEdge

	// Get fully qualified name for this model
	modelName := node.Name
	if node.Alias != "" {
		modelName = node.Alias
	}
	targetFQN := fmt.Sprintf("%s.%s.%s", node.Database, node.Schema, modelName)
	targetMRN := mrn.New("Table", "DBT", targetFQN)

	// Create lineage edges for each dependency
	for _, depNodeID := range node.DependsOn.Nodes {
		// Look up the dependency node
		var depNode ManifestNode
		var found bool

		// Check in nodes (models)
		if n, exists := s.manifest.Nodes[depNodeID]; exists {
			depNode = n
			found = true
		}

		// Check in sources
		if !found {
			if n, exists := s.manifest.Sources[depNodeID]; exists {
				depNode = n
				found = true
			}
		}

		if !found {
			log.Debug().Str("dep_id", depNodeID).Msg("Dependency node not found")
			continue
		}

		// Build source MRN
		depName := depNode.Name
		if depNode.Alias != "" {
			depName = depNode.Alias
		}
		sourceFQN := fmt.Sprintf("%s.%s.%s", depNode.Database, depNode.Schema, depName)
		sourceMRN := mrn.New("Table", "DBT", sourceFQN)

		lineages = append(lineages, lineage.LineageEdge{
			Source: sourceMRN,
			Target: targetMRN,
			Type:   "DEPENDS_ON",
		})
	}

	return lineages
}

func (s *Source) discoverSources() []asset.Asset {
	var assets []asset.Asset

	for _, sourceNode := range s.manifest.Sources {
		asset := s.createSourceAsset(sourceNode)
		assets = append(assets, asset)
	}

	return assets
}

func (s *Source) createSourceAsset(node ManifestNode) asset.Asset {
	tableName := node.Name
	if node.Alias != "" {
		tableName = node.Alias
	}

	fullyQualifiedName := fmt.Sprintf("%s.%s.%s", node.Database, node.Schema, tableName)

	metadata := make(map[string]interface{})
	metadata["dbt_unique_id"] = node.UniqueID
	metadata["dbt_package"] = node.PackageName
	metadata["database"] = node.Database
	metadata["schema"] = node.Schema
	metadata["table_name"] = tableName
	metadata["fully_qualified_name"] = fullyQualifiedName
	metadata["project_name"] = s.config.ProjectName
	metadata["environment"] = s.config.Environment
	metadata["resource_type"] = "source"

	// Add custom meta
	for k, v := range node.Meta {
		metadata[fmt.Sprintf("meta_%s", k)] = v
	}

	// Build schema as map[string]string (column name -> type)
	schema := make(map[string]string)
	for _, col := range node.Columns {
		if col.DataType != "" {
			schema[col.Name] = col.DataType
		}
		// Store column description in metadata
		if col.Description != "" {
			metadata[fmt.Sprintf("column_%s_description", col.Name)] = col.Description
		}
		// Store column tags in metadata
		if len(col.Tags) > 0 {
			metadata[fmt.Sprintf("column_%s_tags", col.Name)] = col.Tags
		}
	}

	allTags := append([]string{}, node.Tags...)
	allTags = append(allTags, s.config.Tags...)
	allTags = append(allTags, "dbt-source")

	mrnValue := mrn.New("Table", "DBT", fullyQualifiedName)
	description := node.Description
	if description == "" {
		description = fmt.Sprintf("DBT source: %s", tableName)
	}

	return asset.Asset{
		Name:        &tableName,
		MRN:         &mrnValue,
		Type:        "Table",
		Providers:   []string{"DBT"},
		Description: &description,
		Metadata:    metadata,
		Tags:        allTags,
		Schema:      schema,
		Sources: []asset.AssetSource{{
			Name:       "DBT",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "dbt",
		Name:        "DBT",
		Description: "Ingest metadata from DBT (Data Build Tool) projects including models, tests, and lineage",
		Icon:        "dbt",
		Category:    "transformation",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register DBT plugin")
	}
}
