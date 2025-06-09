// +marmot:name=MySQL
// +marmot:description=This plugin discovers databases and tables from MySQL instances.
// +marmot:status=experimental
package mysql

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// Config for MySQL plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"MySQL server hostname or IP address"`
	Port     int    `json:"port" description:"MySQL server port (default: 3306)"`
	User     string `json:"user" description:"Username for authentication"`
	Password string `json:"password" description:"Password for authentication"`
	Database string `json:"database" description:"Database name to connect to"`
	TLS      string `json:"tls" description:"TLS configuration (false, true, skip-verify, preferred)"`

	IncludeColumns      bool           `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeRowCounts    bool           `json:"include_row_counts" description:"Whether to include approximate row counts" default:"true"`
	DiscoverForeignKeys bool           `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	TableFilter         *plugin.Filter `json:"table_filter,omitempty" description:"Filter configuration for tables"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "mysql-prod.internal"
port: 3306
user: "marmot_user"
password: "mysql_secure_pass"
database: "ecommerce"
tls: "true"
tags:
  - "mysql"
  - "ecommerce"
`

type Source struct {
	config *Config
	db     *sql.DB
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) error {
	log.Debug().Interface("raw_config", rawConfig).Msg("Starting MySQL config validation")

	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Host == "" {
		return fmt.Errorf("host is required")
	}

	if config.Port == 0 {
		config.Port = 3306
	}

	if config.User == "" {
		return fmt.Errorf("user is required")
	}

	if config.Database == "" {
		return fmt.Errorf("database is required")
	}

	if config.TLS == "" {
		config.TLS = "false"
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.Validate(pluginConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	if err := s.initConnection(ctx, s.config.Database); err != nil {
		return nil, fmt.Errorf("initializing database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	log.Debug().Str("database", s.config.Database).Msg("Starting table and view discovery")
	objectAssets, err := s.discoverTablesAndViews(ctx, s.config.Database)
	if err != nil {
		log.Warn().Err(err).Str("database", s.config.Database).Msg("Failed to discover tables and views")
	} else {
		assets = append(assets, objectAssets...)
		log.Debug().Int("count", len(objectAssets)).Msg("Discovered tables and views")
	}

	if s.config.DiscoverForeignKeys {
		log.Debug().Str("database", s.config.Database).Msg("Starting foreign key discovery")
		fkLineages, err := s.discoverForeignKeys(ctx, s.config.Database)
		if err != nil {
			log.Warn().Err(err).Str("database", s.config.Database).Msg("Failed to discover foreign key relationships")
		} else {
			lineages = append(lineages, fkLineages...)
			log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
		}
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) initConnection(ctx context.Context, database string) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=%s&parseTime=true&timeout=15s",
		s.config.User,
		s.config.Password,
		s.config.Host,
		s.config.Port,
		database,
		s.config.TLS,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("database", database).
		Msg("Successfully connected to MySQL")

	s.db = db
	return nil
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *Source) discoverTablesAndViews(ctx context.Context, dbName string) ([]asset.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT 
			TABLE_SCHEMA as schema_name,
			TABLE_NAME as name,
			TABLE_TYPE as object_type,
			ENGINE as engine,
			TABLE_ROWS as estimated_row_count,
			DATA_LENGTH as data_length,
			INDEX_LENGTH as index_length,
			TABLE_COLLATION as collation,
			CREATE_TIME as created,
			UPDATE_TIME as updated,
			TABLE_COMMENT as description
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`

	rows, err := s.db.QueryContext(queryCtx, query, dbName)
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
			engine        sql.NullString
			estimatedRows sql.NullInt64
			dataLength    sql.NullInt64
			indexLength   sql.NullInt64
			collation     sql.NullString
			created       sql.NullTime
			updated       sql.NullTime
			description   sql.NullString
		)

		if err := rows.Scan(
			&schemaName, &objectName, &objectType, &engine, &estimatedRows,
			&dataLength, &indexLength, &collation, &created, &updated,
			&description,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row")
			continue
		}

		log.Debug().
			Str("schema", schemaName).
			Str("name", objectName).
			Str("type", objectType).
			Str("engine", engine.String).
			Msg("Found database object")

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
		metadata["created"] = time.Now().Format("2006-01-02 15:04:05")
		metadata["object_type"] = strings.ToLower(objectType)

		if engine.Valid {
			metadata["engine"] = engine.String
		}

		if estimatedRows.Valid && s.config.IncludeRowCounts {
			metadata["row_count"] = estimatedRows.Int64
		}

		if dataLength.Valid {
			metadata["data_length"] = dataLength.Int64
		}

		if indexLength.Valid {
			metadata["index_length"] = indexLength.Int64
		}

		if collation.Valid {
			metadata["collation"] = collation.String
		}

		if created.Valid {
			metadata["created"] = created.Time.Format("2006-01-02 15:04:05")
		}

		if updated.Valid {
			metadata["updated"] = updated.Time.Format("2006-01-02 15:04:05")
		}

		if description.Valid {
			metadata["comment"] = description.String
		}

		var assetType string
		var assetDesc string

		if strings.Contains(strings.ToUpper(objectType), "VIEW") {
			assetType = "View"
			assetDesc = fmt.Sprintf("MySQL view %s.%s in database %s", schemaName, objectName, dbName)
		} else {
			assetType = "Table"
			assetDesc = fmt.Sprintf("MySQL table %s.%s in database %s", schemaName, objectName, dbName)
		}

		mrnValue := mrn.New(assetType, "MySQL", objectName)

		processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, asset.Asset{
			Name:        &objectName,
			MRN:         &mrnValue,
			Type:        assetType,
			Providers:   []string{"MySQL"},
			Description: &assetDesc,
			Metadata:    metadata,
			Tags:        processedTags,
			Sources: []asset.AssetSource{{
				Name:       "MySQL",
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

func (s *Source) discoverForeignKeys(ctx context.Context, dbName string) ([]lineage.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			kcu.TABLE_SCHEMA AS source_schema,
			kcu.TABLE_NAME AS source_table,
			kcu.COLUMN_NAME AS source_column,
			kcu.REFERENCED_TABLE_SCHEMA AS target_schema,
			kcu.REFERENCED_TABLE_NAME AS target_table,
			kcu.REFERENCED_COLUMN_NAME AS target_column,
			kcu.CONSTRAINT_NAME AS constraint_name,
			rc.UPDATE_RULE AS update_rule,
			rc.DELETE_RULE AS delete_rule
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		LIMIT 1000
	`

	rows, err := s.db.QueryContext(queryCtx, query, dbName)
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
			targetSchema   sql.NullString
			targetTable    sql.NullString
			targetColumn   sql.NullString
			constraintName string
			updateRule     string
			deleteRule     string
		)

		if err := rows.Scan(
			&sourceSchema, &sourceTable, &sourceColumn,
			&targetSchema, &targetTable, &targetColumn,
			&constraintName, &updateRule, &deleteRule,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		if !targetSchema.Valid || !targetTable.Valid || !targetColumn.Valid {
			continue
		}

		log.Debug().
			Str("source", fmt.Sprintf("%s.%s.%s", sourceSchema, sourceTable, sourceColumn)).
			Str("target", fmt.Sprintf("%s.%s.%s", targetSchema.String, targetTable.String, targetColumn.String)).
			Str("constraint", constraintName).
			Msg("Found foreign key relationship")

		if s.config.TableFilter != nil {
			if !plugin.ShouldIncludeResource(sourceTable, *s.config.TableFilter) ||
				!plugin.ShouldIncludeResource(targetTable.String, *s.config.TableFilter) {
				continue
			}
		}

		sourceMRN := mrn.New("Table", "MySQL", sourceTable)
		targetMRN := mrn.New("Table", "MySQL", targetTable.String)

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
