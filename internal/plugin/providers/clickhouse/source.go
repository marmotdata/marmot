// +marmot:name=ClickHouse
// +marmot:description=Discovers databases, tables, and views from ClickHouse instances.
// +marmot:status=experimental
// +marmot:features=Assets
package clickhouse

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for ClickHouse plugin.
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"ClickHouse server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"ClickHouse native protocol port" default:"9000" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" validate:"required"`
	Password string `json:"password" description:"Password for authentication" sensitive:"true"`
	Database string `json:"database" description:"Default database to connect to" default:"default"`
	Secure   bool   `json:"secure" description:"Use TLS/SSL connection" default:"false"`

	IncludeDatabases     bool           `json:"include_databases" description:"Whether to discover databases" default:"true"`
	IncludeColumns       bool           `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	EnableMetrics        bool           `json:"enable_metrics" description:"Whether to include table metrics (row counts, sizes)" default:"true"`
	DatabaseFilter       *plugin.Filter `json:"database_filter,omitempty" description:"Filter configuration for databases"`
	TableFilter          *plugin.Filter `json:"table_filter,omitempty" description:"Filter configuration for tables"`
	ExcludeSystemTables  bool           `json:"exclude_system_tables" description:"Whether to exclude system tables" default:"true"`
}

// +marmot:example-config
var _ = `
host: "clickhouse.company.com"
port: 9000
user: "default"
password: "${CLICKHOUSE_PASSWORD}"
database: "default"
secure: false
include_databases: true
include_columns: true
enable_metrics: true
exclude_system_tables: true
database_filter:
  include:
    - "^analytics.*"
  exclude:
    - ".*_temp$"
tags:
  - "clickhouse"
  - "analytics"
`

// Source represents the ClickHouse plugin.
type Source struct {
	config *Config
	conn   clickhouse.Conn
}

// Validate validates and normalizes the plugin configuration.
func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Port == 0 {
		config.Port = 9000
	}

	if config.Database == "" {
		config.Database = "default"
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers ClickHouse databases, tables, and views.
func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx); err != nil {
		return nil, fmt.Errorf("initializing database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var statistics []plugin.Statistic

	log.Debug().Msg("Starting ClickHouse discovery")

	databases, err := s.discoverDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering databases: %w", err)
	}
	assets = append(assets, databases...)
	log.Debug().Int("count", len(databases)).Msg("Discovered databases")

	for _, dbAsset := range databases {
		if dbAsset.Type != "Database" {
			continue
		}
		dbName := *dbAsset.Name

		tableAssets, err := s.discoverTables(ctx, dbName)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover tables")
			continue
		}
		assets = append(assets, tableAssets...)
		log.Debug().Str("database", dbName).Int("count", len(tableAssets)).Msg("Discovered tables")

		if s.config.EnableMetrics {
			tableStats := s.collectTableStatistics(ctx, dbName, tableAssets)
			statistics = append(statistics, tableStats...)
		}

		for _, tableAsset := range tableAssets {
			lineages = append(lineages, lineage.LineageEdge{
				Source: *dbAsset.MRN,
				Target: *tableAsset.MRN,
				Type:   "CONTAINS",
			})
		}
	}

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("ClickHouse discovery completed")

	return &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)},
		Auth: clickhouse.Auth{
			Database: s.config.Database,
			Username: s.config.User,
			Password: s.config.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 2 * time.Minute,
	}

	if s.config.Secure {
		options.TLS = &tls.Config{
			InsecureSkipVerify: false,
		}
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", s.config.Database).
		Msg("Successfully connected to ClickHouse")

	s.conn = conn
	return nil
}

func (s *Source) closeConnection() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
}

func (s *Source) discoverDatabases(ctx context.Context) ([]asset.Asset, error) {
	query := `
		SELECT
			name,
			engine,
			comment
		FROM system.databases
		WHERE name NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
		ORDER BY name
	`

	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	var assets []asset.Asset

	for rows.Next() {
		var name, engine, comment string
		if err := rows.Scan(&name, &engine, &comment); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}

		if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(name, *s.config.DatabaseFilter) {
			log.Debug().Str("database", name).Msg("Skipping database due to filter")
			continue
		}

		metadata := map[string]interface{}{
			"host":     s.config.Host,
			"port":     s.config.Port,
			"database": name,
			"engine":   engine,
		}

		if comment != "" {
			metadata["comment"] = comment
		}

		mrnValue := mrn.New("Database", "ClickHouse", name)
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      "Database",
			Providers: []string{"ClickHouse"},
			Metadata:  metadata,
			Tags:      processedTags,
			Sources: []asset.AssetSource{{
				Name:       "ClickHouse",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating database rows: %w", err)
	}

	return assets, nil
}

func (s *Source) discoverTables(ctx context.Context, dbName string) ([]asset.Asset, error) {
	query := `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes,
			comment,
			create_table_query
		FROM system.tables
		WHERE database = ?
	`

	if s.config.ExcludeSystemTables {
		query += ` AND NOT startsWith(name, '.')`
	}

	query += ` ORDER BY name`

	rows, err := s.conn.Query(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var assets []asset.Asset
	var tableNames []string

	for rows.Next() {
		var name, engine, comment, createQuery string
		var totalRows, totalBytes sql.NullInt64

		if err := rows.Scan(&name, &engine, &totalRows, &totalBytes, &comment, &createQuery); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table row")
			continue
		}

		if s.config.TableFilter != nil && !plugin.ShouldIncludeResource(name, *s.config.TableFilter) {
			log.Debug().Str("table", name).Msg("Skipping table due to filter")
			continue
		}

		assetType := "Table"
		if engine == "View" || engine == "MaterializedView" {
			assetType = "View"
		}

		metadata := map[string]interface{}{
			"host":       s.config.Host,
			"port":       s.config.Port,
			"database":   dbName,
			"table_name": name,
			"engine":     engine,
		}

		if totalRows.Valid {
			metadata["row_count"] = totalRows.Int64
		}
		if totalBytes.Valid {
			metadata["size_bytes"] = totalBytes.Int64
		}
		if comment != "" {
			metadata["comment"] = comment
		}

		tableNames = append(tableNames, name)

		mrnValue := mrn.New(assetType, "ClickHouse", fmt.Sprintf("%s.%s", dbName, name))
		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      assetType,
			Providers: []string{"ClickHouse"},
			Metadata:  metadata,
			Tags:      processedTags,
			Sources: []asset.AssetSource{{
				Name:       "ClickHouse",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	if s.config.IncludeColumns && len(tableNames) > 0 {
		columnMap, err := s.getColumnsForTables(ctx, dbName, tableNames)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get column information")
		} else {
			for i := range assets {
				tableName := *assets[i].Name
				if columns, exists := columnMap[tableName]; exists {
					assets[i].Metadata["columns"] = columns
				}
			}
		}
	}

	return assets, nil
}

func (s *Source) getColumnsForTables(ctx context.Context, dbName string, tableNames []string) (map[string][]map[string]interface{}, error) {
	query := `
		SELECT
			table,
			name,
			type,
			default_kind,
			default_expression,
			comment,
			is_in_primary_key,
			is_in_sorting_key
		FROM system.columns
		WHERE database = ?
		ORDER BY table, position
	`

	rows, err := s.conn.Query(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	tableSet := make(map[string]bool)
	for _, name := range tableNames {
		tableSet[name] = true
	}

	result := make(map[string][]map[string]interface{})

	for rows.Next() {
		var tableName, columnName, dataType, defaultKind, defaultExpr, comment string
		var isPrimaryKey, isSortingKey uint8

		if err := rows.Scan(&tableName, &columnName, &dataType, &defaultKind, &defaultExpr, &comment, &isPrimaryKey, &isSortingKey); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		if !tableSet[tableName] {
			continue
		}

		column := map[string]interface{}{
			"column_name":    columnName,
			"data_type":      dataType,
			"is_primary_key": isPrimaryKey == 1,
			"is_sorting_key": isSortingKey == 1,
		}

		if defaultKind != "" {
			column["default_kind"] = defaultKind
			column["default_expression"] = defaultExpr
		}
		if comment != "" {
			column["comment"] = comment
		}

		result[tableName] = append(result[tableName], column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return result, nil
}

func (s *Source) collectTableStatistics(ctx context.Context, dbName string, assets []asset.Asset) []plugin.Statistic {
	var statistics []plugin.Statistic

	for _, a := range assets {
		if a.Type != "Table" {
			continue
		}

		if rowCount, ok := a.Metadata["row_count"].(int64); ok {
			statistics = append(statistics, plugin.Statistic{
				AssetMRN:   *a.MRN,
				MetricName: "asset.row_count",
				Value:      float64(rowCount),
			})
		}

		if sizeBytes, ok := a.Metadata["size_bytes"].(int64); ok {
			statistics = append(statistics, plugin.Statistic{
				AssetMRN:   *a.MRN,
				MetricName: "asset.size_bytes",
				Value:      float64(sizeBytes),
			})
		}

		if columns, ok := a.Metadata["columns"].([]map[string]interface{}); ok {
			statistics = append(statistics, plugin.Statistic{
				AssetMRN:   *a.MRN,
				MetricName: "asset.column_count",
				Value:      float64(len(columns)),
			})
		}
	}

	return statistics
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "clickhouse",
		Name:        "ClickHouse",
		Description: "Discover databases, tables, and views from ClickHouse instances",
		Icon:        "clickhouse",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register ClickHouse plugin")
	}
}
