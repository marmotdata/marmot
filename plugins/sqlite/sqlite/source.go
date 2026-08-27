// Package sqlite discovers tables, views and foreign key relationships
// from SQLite database files.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/filesource"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// Config for the SQLite plugin.
type Config struct {
	pluginsdk.BaseConfig         `json:",inline"`
	*filesource.FileSourceConfig `json:",inline"`

	Path string `json:"path" description:"Path to the SQLite database file (local path, s3://bucket/key or git::url)" validate:"required"`

	IncludeColumns      bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	EnableMetrics       bool `json:"enable_metrics" description:"Whether to include table metrics (row and column counts)" default:"true"`
	DiscoverForeignKeys bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	ExcludeSystemTables bool `json:"exclude_system_tables" description:"Whether to exclude SQLite internal tables (sqlite_*)" default:"true"`
}

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "sqlite",
		Name:        "SQLite",
		Description: "Discover tables, views and foreign key relationships from SQLite database files",
		Icon:        "sqlite",
		Category:    "database",
		Status:      "experimental",
		// Discover emits FOREIGN_KEY lineage edges, so the manifest declares
		// Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the SQLite plugin.
type Source struct {
	config *Config
	db     *sql.DB
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	// Default bool fields that should be true unless explicitly set to false.
	if _, ok := rawConfig["include_columns"]; !ok {
		config.IncludeColumns = true
	}
	if _, ok := rawConfig["enable_metrics"]; !ok {
		config.EnableMetrics = true
	}
	if _, ok := rawConfig["discover_foreign_keys"]; !ok {
		config.DiscoverForeignKeys = true
	}
	if _, ok := rawConfig["exclude_system_tables"]; !ok {
		config.ExcludeSystemTables = true
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers SQLite tables, views and foreign key relationships.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// filesource may download an S3 object or clone a git repository to a
	// temp file. Connect to that local copy, but keep the original path (which
	// may be an s3:// or git:: URL) for the asset metadata so the recorded
	// source stays meaningful and reproducible.
	localPath, cleanup, err := filesource.ResolveFilePath(ctx, s.config.FileSourceConfig, s.config.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving file path: %w", err)
	}
	defer cleanup()

	if err := s.initConnection(ctx, localPath); err != nil {
		return nil, fmt.Errorf("initialising database connection: %w", err)
	}
	defer s.closeConnection()

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	log.Debug().Str("path", s.config.Path).Msg("Starting SQLite discovery")

	tableAssets, err := s.discoverTablesAndViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables and views: %w", err)
	}
	assets = append(assets, tableAssets...)
	log.Debug().Int("count", len(tableAssets)).Msg("Discovered tables and views")

	if s.config.EnableMetrics {
		statistics = append(statistics, s.collectTableStatistics(ctx, tableAssets)...)
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
		Msg("SQLite discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context, path string) error {
	s.closeConnection()

	db, err := sql.Open("sqlite", readOnlyDSN(path))
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	// A SQLite file is a single writer, and read-only discovery has no need
	// for more than one connection.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	log.Debug().Str("path", path).Msg("Successfully connected to SQLite")

	s.db = db
	return nil
}

// readOnlyDSN builds a modernc.org/sqlite connection string that opens the
// file read-only. The path is wrapped in a file: URI (rather than
// interpolated directly) so names containing spaces or other characters the
// query parser would misread are percent-encoded. busy_timeout keeps a
// database being written concurrently from failing the metadata reads
// immediately.
func readOnlyDSN(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = "mode=ro&_pragma=busy_timeout(5000)"
	return u.String()
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *Source) discoverTablesAndViews(ctx context.Context) ([]pluginsdk.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT name, type
		FROM sqlite_master
		WHERE type IN ('table', 'view')
	`
	if s.config.ExcludeSystemTables {
		query += ` AND name NOT LIKE 'sqlite_%'`
	}
	query += ` ORDER BY name`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var assets []pluginsdk.Asset

	for rows.Next() {
		var name, objectType string
		if err := rows.Scan(&name, &objectType); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table row")
			continue
		}

		log.Debug().Str("name", name).Str("type", objectType).Msg("Found database object")

		var assetType string
		switch objectType {
		case "table":
			assetType = "Table"
		case "view":
			assetType = "View"
		default:
			continue
		}

		metadata := map[string]interface{}{
			"path":        s.config.Path,
			"table_name":  name,
			"object_type": objectType,
		}

		mrnValue := assetMRN(assetType, name)
		processedTags := pluginsdk.InterpolateTags(s.config.Tags, metadata)

		assets = append(assets, pluginsdk.Asset{
			Name:      &name,
			MRN:       &mrnValue,
			Type:      assetType,
			Providers: []string{"SQLite"},
			Metadata:  metadata,
			Schema:    make(map[string]string),
			Tags:      processedTags,
			Sources: []pluginsdk.AssetSource{{
				Name:       "SQLite",
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	if s.config.IncludeColumns && len(assets) > 0 {
		columnInfoMap, err := s.getBulkColumnInfo(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get column information")
		} else {
			for i := range assets {
				name, ok := assets[i].Metadata["table_name"].(string)
				if !ok {
					continue
				}
				columns, exists := columnInfoMap[name]
				if !exists {
					continue
				}
				jsonBytes, err := json.Marshal(columns)
				if err != nil {
					log.Warn().Err(err).Str("table", name).Msg("Failed to marshal columns")
					continue
				}
				assets[i].Schema["columns"] = string(jsonBytes)
			}
		}
	}

	return assets, nil
}

// columnInfo is the per-column shape serialised into an asset's Schema. The
// default is a pointer so "no default" (null) is distinguishable from a
// default of the empty string, and omitted from the JSON when absent.
type columnInfo struct {
	ColumnName    string  `json:"column_name"`
	DataType      string  `json:"data_type"`
	IsNullable    bool    `json:"is_nullable"`
	IsPrimaryKey  bool    `json:"is_primary_key"`
	ColumnDefault *string `json:"column_default,omitempty"`
}

// getBulkColumnInfo reads every table and view's columns in one pass by
// joining sqlite_master against the pragma_table_info table-valued function,
// keyed by the object name.
func (s *Source) getBulkColumnInfo(ctx context.Context) (map[string][]columnInfo, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT m.name AS table_name,
		       ti.name AS column_name,
		       ti.type AS data_type,
		       ti."notnull" AS not_null,
		       ti.dflt_value AS column_default,
		       ti.pk AS pk
		FROM sqlite_master m
		JOIN pragma_table_info(m.name) ti
		WHERE m.type IN ('table', 'view')
	`
	if s.config.ExcludeSystemTables {
		query += ` AND m.name NOT LIKE 'sqlite_%'`
	}
	query += ` ORDER BY m.name, ti.cid`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]columnInfo)

	for rows.Next() {
		var (
			tableName     string
			dataType      sql.NullString
			notNull       int
			columnDefault sql.NullString
			pk            int
			column        columnInfo
		)

		if err := rows.Scan(&tableName, &column.ColumnName, &dataType, &notNull, &columnDefault, &pk); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		column.DataType = dataType.String
		column.IsNullable = notNull == 0
		column.IsPrimaryKey = pk > 0
		if columnDefault.Valid {
			column.ColumnDefault = &columnDefault.String
		}

		result[tableName] = append(result[tableName], column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return result, nil
}

// discoverForeignKeys reads every table's foreign keys in one pass by joining
// sqlite_master against the pragma_foreign_key_list table-valued function.
// Each key becomes an edge from the referencing table to the referenced one.
func (s *Source) discoverForeignKeys(ctx context.Context) ([]pluginsdk.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT m.name AS source_table,
		       fk."table" AS target_table,
		       fk."from" AS source_column,
		       fk."to" AS target_column
		FROM sqlite_master m
		JOIN pragma_foreign_key_list(m.name) fk
		WHERE m.type = 'table'
	`
	if s.config.ExcludeSystemTables {
		query += ` AND m.name NOT LIKE 'sqlite_%'`
	}
	query += ` LIMIT 1000`

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var lineages []pluginsdk.LineageEdge
	uniqueRelations := make(map[string]struct{})

	for rows.Next() {
		var sourceTable, targetTable, sourceColumn, targetColumn sql.NullString

		if err := rows.Scan(&sourceTable, &targetTable, &sourceColumn, &targetColumn); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		if !sourceTable.Valid || !targetTable.Valid {
			continue
		}

		log.Debug().
			Str("source", fmt.Sprintf("%s.%s", sourceTable.String, sourceColumn.String)).
			Str("target", fmt.Sprintf("%s.%s", targetTable.String, targetColumn.String)).
			Msg("Found foreign key relationship")

		sourceMRN := assetMRN("Table", sourceTable.String)
		targetMRN := assetMRN("Table", targetTable.String)

		if sourceMRN == targetMRN {
			continue
		}

		relationKey := fmt.Sprintf("%s:%s", sourceMRN, targetMRN)
		if _, exists := uniqueRelations[relationKey]; exists {
			continue
		}
		uniqueRelations[relationKey] = struct{}{}

		lineages = append(lineages, pluginsdk.LineageEdge{
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

// collectTableStatistics emits a column count for every object and an exact
// row count for tables. SQLite keeps no row estimate, so the count is a real
// COUNT(*); views are skipped to avoid re-running their underlying query.
func (s *Source) collectTableStatistics(ctx context.Context, assets []pluginsdk.Asset) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	columnCounts := s.columnCounts(ctx)

	for _, a := range assets {
		name, ok := a.Metadata["table_name"].(string)
		if !ok {
			continue
		}

		if count, exists := columnCounts[name]; exists {
			statistics = append(statistics, pluginsdk.Statistic{
				AssetMRN:   *a.MRN,
				MetricName: "asset.column_count",
				Value:      float64(count),
			})
		}

		if a.Type != "Table" {
			continue
		}

		count, err := s.tableRowCount(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("table", name).Msg("Failed to count rows")
			continue
		}
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *a.MRN,
			MetricName: "asset.row_count",
			Value:      float64(count),
		})
	}

	return statistics
}

func (s *Source) columnCounts(ctx context.Context) map[string]int64 {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT m.name, COUNT(*)
		FROM sqlite_master m
		JOIN pragma_table_info(m.name) ti
		WHERE m.type IN ('table', 'view')
	`
	if s.config.ExcludeSystemTables {
		query += ` AND m.name NOT LIKE 'sqlite_%'`
	}
	query += ` GROUP BY m.name`

	counts := make(map[string]int64)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to collect column counts")
		return counts
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			continue
		}
		counts[name] = count
	}

	return counts
}

func (s *Source) tableRowCount(ctx context.Context, table string) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table))
	if err := s.db.QueryRowContext(queryCtx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// quoteIdent double-quotes a SQLite identifier so table names containing
// spaces, keywords or punctuation interpolate safely. A COUNT(*) query cannot
// bind an identifier as a parameter, so the name has to be quoted by hand.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// assetMRN is the single place a SQLite MRN is built. The table pass and the
// foreign key pass both go through it so the two can never drift into
// addressing the same table differently. SQLite has no schema layer, so a
// table is addressed by its bare name, matching the other database plugins.
func assetMRN(assetType, table string) string {
	return mrn.New(assetType, "SQLite", table)
}
