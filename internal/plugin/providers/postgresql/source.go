// +marmot:name=PostgreSQL
// +marmot:description=This plugin discovers databases and tables from PostgreSQL instances.
// +marmot:status=experimental
package postgresql

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/rs/zerolog/log"
)

// Config for PostgreSQL plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection configuration
	Host     string `json:"host" yaml:"host" description:"PostgreSQL server hostname or IP address"`
	Port     int    `json:"port" yaml:"port" description:"PostgreSQL server port (default: 5432)"`
	User     string `json:"user" yaml:"user" description:"Username for authentication"`
	Password string `json:"password" yaml:"password" description:"Password for authentication"`
	SSLMode  string `json:"ssl_mode" yaml:"ssl_mode" description:"SSL mode (disable, require, verify-ca, verify-full)"`

	// Discovery configuration
	IncludeDatabases     bool           `json:"include_databases" yaml:"include_databases" description:"Whether to discover databases" default:"true"`
	IncludeColumns       bool           `json:"include_columns" yaml:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeRowCounts     bool           `json:"include_row_counts" yaml:"include_row_counts" description:"Whether to include approximate row counts (requires analyze)" default:"true"`
	DiscoverForeignKeys  bool           `json:"discover_foreign_keys" yaml:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	SchemaFilter         *plugin.Filter `json:"schema_filter,omitempty" yaml:"schema_filter,omitempty" description:"Filter configuration for schemas"`
	TableFilter          *plugin.Filter `json:"table_filter,omitempty" yaml:"table_filter,omitempty" description:"Filter configuration for tables"`
	DatabaseFilter       *plugin.Filter `json:"database_filter,omitempty" yaml:"database_filter,omitempty" description:"Filter configuration for databases"`
	ExcludeSystemSchemas bool           `json:"exclude_system_schemas" yaml:"exclude_system_schemas" description:"Whether to exclude system schemas (pg_*)" default:"true"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "localhost"
port: 5432
user: "postgres"
password: "mysecretpassword"
ssl_mode: "disable"
include_databases: true
include_columns: true
include_row_counts: true
discover_foreign_keys: true
exclude_system_schemas: true
schema_filter:
  include:
    - "^public$"
    - "^app_.*"
  exclude:
    - "^test_.*"
table_filter:
  include:
    - ".*"
  exclude:
    - "^temp_.*"
database_filter:
  include:
    - ".*"
  exclude:
    - "^template.*"
tags:
  - "postgres"
  - "database"
`

type Source struct {
	config *Config
	pool   *pgxpool.Pool
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	log.Debug().Interface("raw_config", rawConfig).Msg("Starting PostgreSQL config validation")

	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Host == "" {
		return fmt.Errorf("host is required")
	}

	if config.Port == 0 {
		config.Port = 5432
	}

	if config.User == "" {
		return fmt.Errorf("user is required")
	}

	if config.SSLMode == "" {
		config.SSLMode = "disable" // Default to disabled for simplicity
	}

	// Set defaults for discovery options to true if not explicitly set
	if !config.IncludeDatabases {
		config.IncludeDatabases = true
	}

	if !config.IncludeColumns {
		config.IncludeColumns = true
	}

	if !config.IncludeRowCounts {
		config.IncludeRowCounts = true
	}

	if !config.DiscoverForeignKeys {
		config.DiscoverForeignKeys = true
	}

	s.config = config
	return nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	// Initialize connection to the postgres database to get list of all databases
	if err := s.initConnection(ctx, "postgres"); err != nil {
		return nil, fmt.Errorf("initializing database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	// Always discover databases
	log.Debug().Msg("Starting database discovery")
	databaseAssets, err := s.discoverDatabases(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover databases")
	} else {
		assets = append(assets, databaseAssets...)
		log.Debug().Int("count", len(databaseAssets)).Msg("Discovered databases")
	}

	// For each database, discover tables, views, and foreign keys
	for _, dbAsset := range databaseAssets {
		if dbAsset.Type != "Database" {
			continue
		}

		dbName := *dbAsset.Name
		// Skip system databases
		if dbName == "postgres" || dbName == "template0" || dbName == "template1" {
			continue
		}

		// Apply database filter if configured
		if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(dbName, *s.config.DatabaseFilter) {
			log.Debug().Str("database", dbName).Msg("Skipping database due to filter")
			continue
		}

		// Close existing connection and connect to this database
		s.closeConnection()
		if err := s.initConnection(ctx, dbName); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to connect to database")
			continue
		}

		// Discover tables and views
		log.Debug().Str("database", dbName).Msg("Starting table and view discovery")
		objectAssets, err := s.discoverTablesAndViews(ctx, dbName)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover tables and views")
		} else {
			assets = append(assets, objectAssets...)
			log.Debug().Int("count", len(objectAssets)).Msg("Discovered tables and views")
		}

		// Discover foreign key relationships
		if s.config.DiscoverForeignKeys {
			log.Debug().Str("database", dbName).Msg("Starting foreign key discovery")
			fkLineages, err := s.discoverForeignKeys(ctx, dbName)
			if err != nil {
				log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover foreign key relationships")
			} else {
				lineages = append(lineages, fkLineages...)
				log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
			}
		}

		// Close this database connection before moving to the next
		s.closeConnection()
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) initConnection(ctx context.Context, database string) error {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		s.config.User,
		s.config.Password,
		s.config.Host,
		s.config.Port,
		database,
		s.config.SSLMode,
	)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parsing connection string: %w", err)
	}

	// Set reasonable connection pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", database).
		Msg("Successfully connected to PostgreSQL")

	s.pool = pool
	return nil
}

func (s *Source) closeConnection() {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
}

func (s *Source) discoverDatabases(ctx context.Context) ([]asset.Asset, error) {
	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Msg("Discovering databases")

	// Use a query compatible with Postgres 14+
	query := `
		SELECT
			datname AS database_name,
			pg_catalog.pg_get_userbyid(datdba) AS owner,
			pg_database_size(datname) AS size,
			pg_catalog.shobj_description(d.oid, 'pg_database') AS description,
			pg_catalog.pg_encoding_to_char(d.encoding) AS encoding,
			datcollate AS collate,
			datctype AS ctype,
			datistemplate AS is_template,
			datallowconn AS allow_connections,
			datconnlimit AS connection_limit,
			to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS') AS current_time
		FROM
			pg_catalog.pg_database d
		WHERE
			datistemplate = false
		ORDER BY
			datname
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	var assets []asset.Asset

	for rows.Next() {
		var (
			name            string
			owner           string
			size            int64
			description     sql.NullString
			encoding        string
			collate         string
			ctype           string
			isTemplate      bool
			allowConn       bool
			connectionLimit int
			currentTime     string
		)

		if err := rows.Scan(
			&name, &owner, &size, &description, &encoding,
			&collate, &ctype, &isTemplate, &allowConn, &connectionLimit,
			&currentTime,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}

		log.Debug().
			Str("database", name).
			Str("owner", owner).
			Int64("size", size).
			Msg("Found database")

		// Apply database filter if configured
		if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(name, *s.config.DatabaseFilter) {
			log.Debug().Str("database", name).Msg("Skipping database due to filter")
			continue
		}

		// Initialize metadata map
		metadata := make(map[string]interface{})
		metadata["host"] = s.config.Host
		metadata["port"] = s.config.Port
		metadata["database"] = name
		metadata["owner"] = owner
		metadata["size"] = size
		metadata["encoding"] = encoding
		metadata["collate"] = collate
		metadata["ctype"] = ctype
		metadata["is_template"] = isTemplate
		metadata["allow_connections"] = allowConn
		metadata["connection_limit"] = connectionLimit
		metadata["created"] = currentTime // Using current time as creation time is not directly available

		if description.Valid {
			metadata["comment"] = description.String
		}

		mrnValue := mrn.New("Database", "PostgreSQL", name)
		assetDescription := fmt.Sprintf("PostgreSQL database %s", name)

		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &name,
			MRN:         &mrnValue,
			Type:        "Database",
			Providers:   []string{"PostgreSQL"},
			Description: &assetDescription,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "PostgreSQL",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating database rows: %w", err)
	}

	log.Debug().Int("count", len(assets)).Msg("Discovered databases")
	return assets, nil
}

func (s *Source) discoverTablesAndViews(ctx context.Context, dbName string) ([]asset.Asset, error) {
	// Query for tables and views
	query := `
		SELECT
			n.nspname AS schema_name,
			c.relname AS name,
			CASE 
				WHEN c.relkind = 'r' THEN 'table'
				WHEN c.relkind = 'v' THEN 'view'
				WHEN c.relkind = 'm' THEN 'materialized_view'
			END AS object_type,
			pg_catalog.pg_get_userbyid(c.relowner) AS owner,
			c.reltuples AS estimated_row_count,
			pg_catalog.obj_description(c.oid, 'pg_class') AS description,
			pg_catalog.pg_table_size(c.oid) AS size,
			to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS') AS current_time
		FROM
			pg_catalog.pg_class c
			JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE
			c.relkind IN ('r', 'v', 'm')
			AND (n.nspname NOT LIKE 'pg\\_%' OR NOT $1)
			AND n.nspname != 'information_schema'
		ORDER BY
			n.nspname, c.relname
	`

	rows, err := s.pool.Query(ctx, query, s.config.ExcludeSystemSchemas)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var assets []asset.Asset

	for rows.Next() {
		var (
			schemaName    string
			objectName    string
			objectType    string
			owner         string
			estimatedRows sql.NullFloat64
			description   sql.NullString
			size          sql.NullInt64
			currentTime   string
		)

		if err := rows.Scan(
			&schemaName, &objectName, &objectType, &owner, &estimatedRows,
			&description, &size, &currentTime,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row")
			continue
		}

		log.Debug().
			Str("schema", schemaName).
			Str("name", objectName).
			Str("type", objectType).
			Str("owner", owner).
			Msg("Found database object")

		// Apply schema filter if configured
		if s.config.SchemaFilter != nil && !plugin.ShouldIncludeResource(schemaName, *s.config.SchemaFilter) {
			log.Debug().Str("schema", schemaName).Msg("Skipping schema due to filter")
			continue
		}

		// Apply table filter if configured
		if s.config.TableFilter != nil && !plugin.ShouldIncludeResource(objectName, *s.config.TableFilter) {
			log.Debug().Str("object", objectName).Msg("Skipping object due to filter")
			continue
		}

		// Initialize metadata map
		metadata := make(map[string]interface{})
		metadata["host"] = s.config.Host
		metadata["port"] = s.config.Port
		metadata["database"] = dbName
		metadata["schema"] = schemaName
		metadata["table_name"] = objectName
		metadata["owner"] = owner
		metadata["created"] = currentTime
		metadata["object_type"] = objectType

		if estimatedRows.Valid && s.config.IncludeRowCounts {
			metadata["row_count"] = int64(estimatedRows.Float64)
		}

		if description.Valid {
			metadata["comment"] = description.String
		}

		if size.Valid {
			metadata["size"] = size.Int64
		}

		// Get column information if configured
		if s.config.IncludeColumns {
			columns, err := s.getColumnInfo(ctx, schemaName, objectName)
			if err != nil {
				log.Warn().Err(err).Str("object", objectName).Msg("Failed to get column information")
			} else {
				metadata["columns"] = columns
			}
		}

		// Determine asset type based on the object type
		var assetType string
		var assetDesc string

		switch objectType {
		case "table":
			assetType = "Table"
			assetDesc = fmt.Sprintf("PostgreSQL table %s.%s in database %s", schemaName, objectName, dbName)
		case "view", "materialized_view":
			assetType = "View"
			assetDesc = fmt.Sprintf("PostgreSQL view %s.%s in database %s", schemaName, objectName, dbName)
		default:
			// Skip unknown types
			continue
		}

		objectKey := fmt.Sprintf("%s.%s.%s", dbName, schemaName, objectName)

		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &objectKey,
			Type:        assetType,
			Providers:   []string{"PostgreSQL"},
			Description: &assetDesc,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "PostgreSQL",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	return assets, nil
}

func (s *Source) getColumnInfo(ctx context.Context, schemaName, tableName string) ([]map[string]interface{}, error) {
	query := `
		SELECT
			a.attname AS column_name,
			pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
			CASE WHEN a.attnotnull THEN false ELSE true END AS is_nullable,
			pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS column_default,
			CASE WHEN EXISTS (
				SELECT 1 FROM pg_catalog.pg_constraint c
				WHERE c.conrelid = a.attrelid
				AND a.attnum = ANY(c.conkey)
				AND c.contype = 'p'
			) THEN true ELSE false END AS is_primary_key,
			col_description(a.attrelid, a.attnum) AS comment
		FROM
			pg_catalog.pg_attribute a
			JOIN pg_catalog.pg_class c ON a.attrelid = c.oid
			JOIN pg_catalog.pg_namespace n ON c.relnamespace = n.oid
			LEFT JOIN pg_catalog.pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
		WHERE
			n.nspname = $1
			AND c.relname = $2
			AND a.attnum > 0
			AND NOT a.attisdropped
		ORDER BY
			a.attnum
	`

	rows, err := s.pool.Query(ctx, query, schemaName, tableName)
	if err != nil {
		return nil, fmt.Errorf("querying column information: %w", err)
	}
	defer rows.Close()

	var columns []map[string]interface{}

	for rows.Next() {
		var (
			columnName    string
			dataType      string
			isNullable    bool
			columnDefault sql.NullString
			isPrimaryKey  bool
			comment       sql.NullString
		)

		if err := rows.Scan(
			&columnName, &dataType, &isNullable, &columnDefault,
			&isPrimaryKey, &comment,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		column := map[string]interface{}{
			"column_name":    columnName,
			"data_type":      dataType,
			"is_nullable":    isNullable,
			"is_primary_key": isPrimaryKey,
		}

		if columnDefault.Valid {
			column["column_default"] = columnDefault.String
		}

		if comment.Valid {
			column["comment"] = comment.String
		}

		columns = append(columns, column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return columns, nil
}

func (s *Source) discoverForeignKeys(ctx context.Context, dbName string) ([]lineage.LineageEdge, error) {
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
			AND (kcu.table_schema NOT LIKE 'pg\\_%' OR NOT $1)
	`

	rows, err := s.pool.Query(ctx, query, s.config.ExcludeSystemSchemas)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var lineages []lineage.LineageEdge

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

		// Apply schema and table filters
		if s.config.SchemaFilter != nil {
			if !plugin.ShouldIncludeResource(sourceSchema, *s.config.SchemaFilter) ||
				!plugin.ShouldIncludeResource(targetSchema, *s.config.SchemaFilter) {
				continue
			}
		}

		if s.config.TableFilter != nil {
			if !plugin.ShouldIncludeResource(sourceTable, *s.config.TableFilter) ||
				!plugin.ShouldIncludeResource(targetTable, *s.config.TableFilter) {
				continue
			}
		}

		sourceKey := fmt.Sprintf("%s.%s.%s", dbName, sourceSchema, sourceTable)
		targetKey := fmt.Sprintf("%s.%s.%s", dbName, targetSchema, targetTable)

		sourceMRN := mrn.New("Table", "PostgreSQL", sourceKey)
		targetMRN := mrn.New("Table", "PostgreSQL", targetKey)

		// Create lineage edge
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
