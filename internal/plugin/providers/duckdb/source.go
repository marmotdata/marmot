// +marmot:name=DuckDB
// +marmot:description=Discovers schemas, tables, views and foreign key relationships from DuckDB database files.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package duckdb

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for DuckDB plugin.
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Path string `json:"path" description:"Path to the DuckDB database file" validate:"required"`

	IncludeColumns       bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	EnableMetrics        bool `json:"enable_metrics" description:"Whether to include table metrics (row counts and sizes)" default:"true"`
	DiscoverForeignKeys  bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	ExcludeSystemSchemas bool `json:"exclude_system_schemas" description:"Whether to exclude system schemas (information_schema, pg_catalog)" default:"true"`
}

// +marmot:example-config
var _ = `
path: "/data/analytics.duckdb"
include_columns: true
enable_metrics: true
discover_foreign_keys: true
exclude_system_schemas: true
filter:
  include:
    - "^main\\..*"
  exclude:
    - ".*_temp$"
tags:
  - "duckdb"
  - "analytics"
`

// Source represents the DuckDB plugin.
type Source struct {
	config *Config
	db     *sql.DB
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Default bool fields that should be true unless explicitly set to false
	if _, ok := rawConfig["include_columns"]; !ok {
		config.IncludeColumns = true
	}
	if _, ok := rawConfig["enable_metrics"]; !ok {
		config.EnableMetrics = true
	}
	if _, ok := rawConfig["discover_foreign_keys"]; !ok {
		config.DiscoverForeignKeys = true
	}
	if _, ok := rawConfig["exclude_system_schemas"]; !ok {
		config.ExcludeSystemSchemas = true
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers DuckDB tables, views and foreign key relationships.
func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx); err != nil {
		return nil, fmt.Errorf("initialising database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var statistics []plugin.Statistic

	log.Debug().Str("path", s.config.Path).Msg("Starting DuckDB discovery")

	tableAssets, err := s.discoverTablesAndViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables and views: %w", err)
	}
	assets = append(assets, tableAssets...)
	log.Debug().Int("count", len(tableAssets)).Msg("Discovered tables and views")

	if s.config.EnableMetrics {
		tableStats := s.collectTableStatistics(ctx, tableAssets)
		statistics = append(statistics, tableStats...)
	}

	if s.config.DiscoverForeignKeys {
		log.Debug().Msg("Starting foreign key discovery")
		fkLineages, err := s.discoverForeignKeys(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover foreign key relationships")
		} else {
			lineages = append(lineages, fkLineages...)
			log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("DuckDB discovery completed")

	return &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	dsn := s.config.Path + "?access_mode=read_only"

	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("path", s.config.Path).
		Msg("Successfully connected to DuckDB")

	s.db = db
	return nil
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *Source) discoverTablesAndViews(ctx context.Context) ([]asset.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			table_schema,
			table_name,
			table_type
		FROM information_schema.tables
		WHERE 1=1
	`

	if s.config.ExcludeSystemSchemas {
		query += ` AND table_schema NOT IN ('information_schema', 'pg_catalog')`
	}

	query += ` ORDER BY table_schema, table_name`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var assets []asset.Asset
	var schemaTables []struct {
		schema string
		table  string
	}

	for rows.Next() {
		var schemaName, tableName, tableType string

		if err := rows.Scan(&schemaName, &tableName, &tableType); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table row")
			continue
		}

		log.Debug().
			Str("schema", schemaName).
			Str("name", tableName).
			Str("type", tableType).
			Msg("Found database object")

		var assetType string
		switch tableType {
		case "BASE TABLE":
			assetType = "Table"
		case "VIEW":
			assetType = "View"
		default:
			continue
		}

		qualifiedName := fmt.Sprintf("%s.%s", schemaName, tableName)

		metadata := map[string]interface{}{
			"path":        s.config.Path,
			"schema":      schemaName,
			"table_name":  tableName,
			"object_type": tableType,
		}

		mrnValue := mrn.New(assetType, "DuckDB", qualifiedName)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		a := asset.Asset{
			Name:      &tableName,
			MRN:       &mrnValue,
			Type:      assetType,
			Providers: []string{"DuckDB"},
			Metadata:  metadata,
			Schema:    make(map[string]string),
			Tags:      processedTags,
			Sources: []asset.AssetSource{{
				Name:       "DuckDB",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		}

		schemaTables = append(schemaTables, struct {
			schema string
			table  string
		}{
			schema: schemaName,
			table:  tableName,
		})

		assets = append(assets, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	if s.config.IncludeColumns && len(schemaTables) > 0 {
		columnInfoMap, err := s.getBulkColumnInfo(ctx, schemaTables)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get column information")
		} else {
			for i := range assets {
				schemaName, ok := assets[i].Metadata["schema"].(string)
				if !ok {
					continue
				}

				tableName, ok := assets[i].Metadata["table_name"].(string)
				if !ok {
					continue
				}

				key := schemaName + "." + tableName
				if columns, exists := columnInfoMap[key]; exists {
					jsonBytes, err := json.Marshal(columns)
					if err != nil {
						log.Warn().Err(err).Str("table", key).Msg("Failed to marshal columns")
						continue
					}
					assets[i].Schema["columns"] = string(jsonBytes)
				}
			}
		}
	}

	return assets, nil
}

func (s *Source) getBulkColumnInfo(ctx context.Context, schemaTables []struct {
	schema string
	table  string
}) (map[string][]interface{}, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			table_schema,
			table_name,
			column_name,
			data_type,
			is_nullable,
			column_default,
			ordinal_position
		FROM information_schema.columns
		WHERE 1=1
	`

	if s.config.ExcludeSystemSchemas {
		query += ` AND table_schema NOT IN ('information_schema', 'pg_catalog')`
	}

	query += ` ORDER BY table_schema, table_name, ordinal_position`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	tableSet := make(map[string]bool)
	for _, st := range schemaTables {
		tableSet[st.schema+"."+st.table] = true
	}

	result := make(map[string][]interface{})

	for rows.Next() {
		var (
			schemaName    string
			tableName     string
			columnName    string
			dataType      string
			isNullable    string
			columnDefault sql.NullString
			ordinalPos    int
		)

		if err := rows.Scan(
			&schemaName, &tableName, &columnName, &dataType,
			&isNullable, &columnDefault, &ordinalPos,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		key := schemaName + "." + tableName
		if !tableSet[key] {
			continue
		}

		column := map[string]interface{}{
			"column_name": columnName,
			"data_type":   dataType,
			"is_nullable": isNullable == "YES",
		}

		if columnDefault.Valid {
			column["column_default"] = columnDefault.String
		}

		result[key] = append(result[key], column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return result, nil
}

func (s *Source) discoverForeignKeys(ctx context.Context) ([]lineage.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			kcu.table_schema AS source_schema,
			kcu.table_name AS source_table,
			kcu.column_name AS source_column,
			ccu.table_schema AS target_schema,
			ccu.table_name AS target_table,
			ccu.column_name AS target_column,
			tc.constraint_name
		FROM
			information_schema.table_constraints AS tc
			JOIN information_schema.key_column_usage AS kcu
				ON tc.constraint_name = kcu.constraint_name
				AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage AS ccu
				ON ccu.constraint_name = tc.constraint_name
				AND ccu.table_schema = tc.table_schema
		WHERE
			tc.constraint_type = 'FOREIGN KEY'
	`

	if s.config.ExcludeSystemSchemas {
		query += ` AND kcu.table_schema NOT IN ('information_schema', 'pg_catalog')`
	}

	query += ` LIMIT 1000`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var lineages []lineage.LineageEdge
	uniqueRelations := make(map[string]struct{})

	for rows.Next() {
		var (
			sourceSchema   string
			sourceTable    string
			sourceColumn   string
			targetSchema   string
			targetTable    string
			targetColumn   string
			constraintName string
		)

		if err := rows.Scan(
			&sourceSchema, &sourceTable, &sourceColumn,
			&targetSchema, &targetTable, &targetColumn,
			&constraintName,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		log.Debug().
			Str("source", fmt.Sprintf("%s.%s.%s", sourceSchema, sourceTable, sourceColumn)).
			Str("target", fmt.Sprintf("%s.%s.%s", targetSchema, targetTable, targetColumn)).
			Str("constraint", constraintName).
			Msg("Found foreign key relationship")

		sourceMRN := mrn.New("Table", "DuckDB", fmt.Sprintf("%s.%s", sourceSchema, sourceTable))
		targetMRN := mrn.New("Table", "DuckDB", fmt.Sprintf("%s.%s", targetSchema, targetTable))

		if sourceMRN == targetMRN {
			continue
		}

		relationKey := fmt.Sprintf("%s:%s", sourceMRN, targetMRN)
		if _, exists := uniqueRelations[relationKey]; exists {
			continue
		}
		uniqueRelations[relationKey] = struct{}{}

		lineages = append(lineages, lineage.LineageEdge{
			Source: sourceMRN,
			Target: targetMRN,
			Type:   "FOREIGN_KEY",
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foreign key rows: %w", err)
	}

	return lineages, nil
}

func (s *Source) collectTableStatistics(ctx context.Context, assets []asset.Asset) []plugin.Statistic {
	var statistics []plugin.Statistic

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			schema_name,
			table_name,
			estimated_size,
			column_count
		FROM duckdb_tables()
	`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to collect table statistics")
		return statistics
	}
	defer rows.Close()

	assetMap := make(map[string]string)
	for _, a := range assets {
		schemaName, _ := a.Metadata["schema"].(string)
		tableName, _ := a.Metadata["table_name"].(string)
		key := schemaName + "." + tableName
		assetMap[key] = *a.MRN
	}

	for rows.Next() {
		var schemaName, tableName string
		var estimatedSize, columnCount int64

		if err := rows.Scan(&schemaName, &tableName, &estimatedSize, &columnCount); err != nil {
			continue
		}

		key := schemaName + "." + tableName
		assetMRN, ok := assetMap[key]
		if !ok {
			continue
		}

		statistics = append(statistics,
			plugin.Statistic{
				AssetMRN:   assetMRN,
				MetricName: "asset.size_bytes",
				Value:      float64(estimatedSize),
			},
			plugin.Statistic{
				AssetMRN:   assetMRN,
				MetricName: "asset.column_count",
				Value:      float64(columnCount),
			},
		)
	}

	return statistics
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "duckdb",
		Name:        "DuckDB",
		Description: "Discover schemas, tables, views and foreign key relationships from DuckDB database files",
		Icon:        "duckdb",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register DuckDB plugin")
	}
}
