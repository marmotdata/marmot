// +marmot:name=PostgreSQL
// +marmot:description=This plugin discovers databases and tables from PostgreSQL instances.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package postgresql

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for PostgreSQL plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection configuration
	Host     string `json:"host" description:"PostgreSQL server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"PostgreSQL server port" default:"5432" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" validate:"required"`
	Password string `json:"password" description:"Password for authentication" sensitive:"true"`
	SSLMode  string `json:"ssl_mode" description:"SSL mode (disable, require, verify-ca, verify-full)" default:"disable" validate:"omitempty,oneof=disable require verify-ca verify-full"`

	// Discovery configuration
	IncludeDatabases     bool           `json:"include_databases" description:"Whether to discover databases" default:"true"`
	IncludeColumns       bool           `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	EnableMetrics        bool           `json:"enable_metrics" description:"Whether to include table metrics" default:"true"`
	DiscoverForeignKeys  bool           `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	SchemaFilter         *plugin.Filter `json:"schema_filter,omitempty" description:"Filter configuration for schemas"`
	TableFilter          *plugin.Filter `json:"table_filter,omitempty" description:"Filter configuration for tables"`
	DatabaseFilter       *plugin.Filter `json:"database_filter,omitempty" description:"Filter configuration for databases"`
	ExcludeSystemSchemas bool           `json:"exclude_system_schemas" description:"Whether to exclude system schemas (pg_*)" default:"true"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "prod-postgres.company.com"
port: 5432
user: "marmot_reader"
password: "secure_password_123"
ssl_mode: "require"
tags:
  - "postgres"
  - "production"
`

// Source represents the PostgreSQL plugin
type Source struct {
	config *Config
	pool   *pgxpool.Pool
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Port == 0 {
		config.Port = 5432
	}

	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := s.initConnection(ctx, "postgres"); err != nil {
		return nil, fmt.Errorf("initializing database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge
	var statistics []plugin.Statistic

	log.Debug().Msg("Starting database discovery")

	databaseAssets, err := s.discoverDatabases(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover databases")
		return &plugin.DiscoveryResult{
			Assets:     assets,
			Lineage:    lineages,
			Statistics: statistics,
		}, fmt.Errorf("failed to discover databases: %w", err)
	} else {
		assets = append(assets, databaseAssets...)
		log.Debug().Int("count", len(databaseAssets)).Msg("Discovered databases")
	}
	for _, dbAsset := range databaseAssets {
		if dbAsset.Type != "Database" {
			continue
		}
		dbName := *dbAsset.Name
		if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(dbName, *s.config.DatabaseFilter) {
			log.Debug().Str("database", dbName).Msg("Skipping database due to filter")
			continue
		}
		if dbName == "template0" || dbName == "template1" {
			continue
		}
		dbCtx, dbCancel := context.WithTimeout(ctx, 2*time.Minute)
		if err := s.initConnection(dbCtx, dbName); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to connect to database")
			dbCancel()
			continue
		}
		log.Debug().Str("database", dbName).Msg("Starting table and view discovery")
		objectAssets, err := s.discoverTablesAndViews(dbCtx, dbName)
		if err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover tables and views")
		} else {
			assets = append(assets, objectAssets...)
			log.Debug().Int("count", len(objectAssets)).Msg("Discovered tables and views")

			// Collect statistics if enabled
			if s.config.EnableMetrics {
				tableStats := s.collectTableStatistics(dbCtx, dbName, objectAssets)
				statistics = append(statistics, tableStats...)
			}

			// Create lineage between database and its tables/views
			for _, objAsset := range objectAssets {
				lineages = append(lineages, lineage.LineageEdge{
					Source: *dbAsset.MRN,
					Target: *objAsset.MRN,
					Type:   "CONTAINS",
				})
			}
		}
		if s.config.DiscoverForeignKeys {
			log.Debug().Str("database", dbName).Msg("Starting foreign key discovery")
			fkLineages, err := s.discoverForeignKeys(dbCtx, dbName)
			if err != nil {
				log.Warn().Err(err).Str("database", dbName).Msg("Failed to discover foreign key relationships")
			} else {
				lineages = append(lineages, fkLineages...)
				log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
			}
		}
		dbCancel()
	}
	return &plugin.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context, database string) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

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

	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = 2 * time.Minute
	config.MaxConnIdleTime = 30 * time.Second

	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.ConnConfig.RuntimeParams["statement_timeout"] = "30000"

	pool, err := pgxpool.NewWithConfig(timeoutCtx, config)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(timeoutCtx); err != nil {
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

		if s.config.DatabaseFilter != nil && !plugin.ShouldIncludeResource(name, *s.config.DatabaseFilter) {
			log.Debug().Str("database", name).Msg("Skipping database due to filter")
			continue
		}

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
		metadata["created"] = currentTime

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
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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

	rows, err := s.pool.Query(queryCtx, query, s.config.ExcludeSystemSchemas)
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

		if s.config.SchemaFilter != nil && !plugin.ShouldIncludeResource(schemaName, *s.config.SchemaFilter) {
			log.Debug().Str("schema", schemaName).Msg("Skipping schema due to filter")
			continue
		}
		if s.config.TableFilter != nil && !plugin.ShouldIncludeResource(objectName, *s.config.TableFilter) {
			log.Debug().Str("object", objectName).Msg("Skipping object due to filter")
			continue
		}

		metadata := make(map[string]interface{})
		metadata["host"] = s.config.Host
		metadata["port"] = s.config.Port
		metadata["database"] = dbName
		metadata["schema"] = schemaName
		metadata["table_name"] = objectName
		metadata["owner"] = owner
		metadata["created"] = currentTime
		metadata["object_type"] = objectType

		if estimatedRows.Valid && s.config.EnableMetrics {
			metadata["row_count"] = int64(estimatedRows.Float64)
		}

		if description.Valid {
			metadata["comment"] = description.String
		}

		if size.Valid {
			metadata["size"] = size.Int64
		}

		if s.config.IncludeColumns {
			schemaTables = append(schemaTables, struct {
				schema string
				table  string
			}{
				schema: schemaName,
				table:  objectName,
			})
		}

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
			continue
		}

		mrnValue := mrn.New(assetType, "PostgreSQL", objectName)

		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &objectName,
			MRN:         &mrnValue,
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

	if s.config.IncludeColumns && len(schemaTables) > 0 {
		columnInfoMap, err := s.getBulkColumnInfo(ctx, schemaTables)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get bulk column information")
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
					assets[i].Metadata["columns"] = columns
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
        n.nspname AS schema_name,
        c.relname AS table_name,
        a.attname AS column_name,
        pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
        CASE WHEN a.attnotnull THEN false ELSE true END AS is_nullable,
        pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS column_default,
        CASE WHEN EXISTS (
            SELECT 1 FROM pg_catalog.pg_constraint con
            WHERE con.conrelid = a.attrelid
            AND a.attnum = ANY(con.conkey)
            AND con.contype = 'p'
        ) THEN true ELSE false END AS is_primary_key,
        col_description(a.attrelid, a.attnum) AS comment
    FROM
        pg_catalog.pg_attribute a
        JOIN pg_catalog.pg_class c ON a.attrelid = c.oid
        JOIN pg_catalog.pg_namespace n ON c.relnamespace = n.oid
        LEFT JOIN pg_catalog.pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
    WHERE
        a.attnum > 0
        AND NOT a.attisdropped
        AND (n.nspname, c.relname) IN (
    `

	placeholders := make([]string, 0, len(schemaTables))
	params := make([]interface{}, 0, len(schemaTables)*2)

	for i, st := range schemaTables {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		params = append(params, st.schema, st.table)
	}

	query += strings.Join(placeholders, ", ") + ") ORDER BY n.nspname, c.relname, a.attnum"

	rows, err := s.pool.Query(queryCtx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("querying bulk column information: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]interface{})

	for rows.Next() {
		var (
			schemaName    string
			tableName     string
			columnName    string
			dataType      string
			isNullable    bool
			columnDefault sql.NullString
			isPrimaryKey  bool
			comment       sql.NullString
		)

		if err := rows.Scan(
			&schemaName, &tableName, &columnName, &dataType, &isNullable,
			&columnDefault, &isPrimaryKey, &comment,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		key := schemaName + "." + tableName

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

		if _, exists := result[key]; !exists {
			result[key] = make([]interface{}, 0)
		}

		result[key] = append(result[key], column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bulk column rows: %w", err)
	}

	return result, nil
}

func (s *Source) discoverForeignKeys(ctx context.Context, dbName string) ([]lineage.LineageEdge, error) {
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
        AND (kcu.table_schema NOT LIKE 'pg\\_%' OR NOT $1)
        AND kcu.table_schema NOT IN ('information_schema')
    LIMIT 1000
`

	rows, err := s.pool.Query(queryCtx, query, s.config.ExcludeSystemSchemas)
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

		sourceMRN := mrn.New("Table", "PostgreSQL", sourceTable)
		targetMRN := mrn.New("Table", "PostgreSQL", targetTable)

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

func (s *Source) collectTableStatistics(ctx context.Context, dbName string, assets []asset.Asset) []plugin.Statistic {
	var statistics []plugin.Statistic

	query := `
		SELECT 
			n.nspname || '.' || c.relname as table_name,
			c.reltuples::bigint as row_count,
			(SELECT COUNT(*) FROM information_schema.columns 
			 WHERE table_schema = n.nspname AND table_name = c.relname) as column_count,
			pg_total_relation_size(c.oid) as total_size_bytes
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT LIKE 'pg_%'
		  AND n.nspname != 'information_schema'`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to collect table statistics")
		return statistics
	}
	defer rows.Close()

	assetMap := make(map[string]string)
	for _, a := range assets {
		if tableName, ok := a.Metadata["table_name"].(string); ok {
			assetMap[tableName] = *a.MRN
		}
	}

	for rows.Next() {
		var tableName string
		var rowCount, columnCount, sizeBytes int64

		if err := rows.Scan(&tableName, &rowCount, &columnCount, &sizeBytes); err != nil {
			continue
		}

		assetMRN, ok := assetMap[tableName]
		if !ok {
			continue
		}

		statistics = append(statistics,
			plugin.Statistic{
				AssetMRN:   assetMRN,
				MetricName: "asset.row_count",
				Value:      float64(rowCount),
			},
			plugin.Statistic{
				AssetMRN:   assetMRN,
				MetricName: "asset.column_count",
				Value:      float64(columnCount),
			},
			plugin.Statistic{
				AssetMRN:   assetMRN,
				MetricName: "asset.size_bytes",
				Value:      float64(sizeBytes),
			},
		)
	}

	return statistics
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "postgresql",
		Name:        "PostgreSQL",
		Description: "Discover databases, schemas, and tables from PostgreSQL instances",
		Icon:        "postgresql",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register PostgreSQL plugin")
	}
}
