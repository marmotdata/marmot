// Package mariadb discovers a database and its tables, views and sequences
// from MariaDB servers.
package mariadb

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string shared with the OpenMetadata and
// Trino plugins, so all three address a MariaDB table by the same MRN.
const provider = "MariaDB"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "mariadb",
		Name:        "MariaDB",
		Description: "Discover databases, tables, views and sequences from MariaDB servers",
		Icon:        "mariadb",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the MariaDB plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"MariaDB server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"MariaDB server port" default:"3306" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" validate:"required"`
	Password string `json:"password" description:"Password for authentication" sensitive:"true"`
	Database string `json:"database" description:"Database to discover" validate:"required"`
	TLS      string `json:"tls" label:"TLS" description:"TLS mode (false, true, skip-verify, preferred)" default:"false" validate:"omitempty,oneof=false true skip-verify preferred"`

	IncludeColumns      bool `json:"include_columns" description:"Whether to include column information in table and view schemas" default:"true"`
	IncludeViews        bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeSequences    bool `json:"include_sequences" description:"Whether to discover sequences" default:"true"`
	IncludeRowCounts    bool `json:"include_row_counts" description:"Whether to include approximate row counts in table metadata" default:"true"`
	IncludeStatistics   bool `json:"include_statistics" description:"Whether to emit row count, size and column count statistics" default:"true"`
	DiscoverForeignKeys bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "mariadb-prod.internal"
port: 3306
user: "marmot_reader"
password: "mariadb_secure_pass"
database: "shop"
tls: "true"
include_views: true
include_sequences: true
tags:
  - "mariadb"
  - "shop"
`

// Source represents the MariaDB plugin.
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

	pluginsdk.ApplyDefaults(config, rawConfig)

	// An explicit port of 0 is not a usable port either.
	if config.Port == 0 {
		config.Port = 3306
	}
	if config.TLS == "" {
		config.TLS = "false"
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers the configured database with its tables, views and
// sequences, plus foreign key and view lineage.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx, s.config.Database); err != nil {
		return nil, fmt.Errorf("initialising database connection: %w", err)
	}
	defer s.closeConnection()

	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	// The database is an asset in its own right, like it is for the MySQL
	// and PostgreSQL plugins. An OpenMetadata import already creates the
	// same asset for a MariaDB service, so the two land on one record.
	dbAsset := s.databaseAsset()
	s.describeServer(ctx, &dbAsset)
	assets := []pluginsdk.Asset{dbAsset}

	log.Debug().Str("database", s.config.Database).Msg("Starting table, view and sequence discovery")
	objects, objectStats, err := s.discoverObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering tables: %w", err)
	}
	log.Debug().Int("count", len(objects)).Msg("Discovered tables, views and sequences")

	for _, object := range objects {
		lineages = append(lineages, pluginsdk.LineageEdge{
			Source: *dbAsset.MRN,
			Target: *object.MRN,
			Type:   "CONTAINS",
		})
	}

	if s.config.IncludeColumns {
		if err := s.attachColumns(ctx, objects); err != nil {
			log.Warn().Err(err).Msg("Failed to get column information")
		}
	}

	if s.config.IncludeViews {
		viewLineages, err := s.attachViewDefinitions(ctx, objects)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get view definitions")
		} else {
			lineages = append(lineages, viewLineages...)
			log.Debug().Int("count", len(viewLineages)).Msg("Discovered view dependencies")
		}
	}

	if s.config.IncludeSequences {
		s.attachSequenceDetails(ctx, objects)
	}

	if s.config.DiscoverForeignKeys {
		log.Debug().Str("database", s.config.Database).Msg("Starting foreign key discovery")
		fkLineages, err := s.discoverForeignKeys(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover foreign key relationships")
		} else {
			lineages = append(lineages, fkLineages...)
			log.Debug().Int("count", len(fkLineages)).Msg("Discovered foreign key relationships")
		}
	}

	if s.config.IncludeStatistics {
		statistics = append(statistics, objectStats...)
		statistics = append(statistics, s.columnCountStatistics(ctx, objects)...)
	}

	assets = append(assets, objects...)

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("MariaDB discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context, database string) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", buildDSN(s.config, database))
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
		Msg("Successfully connected to MariaDB")

	s.db = db
	return nil
}

// buildDSN builds the driver connection string with the driver's own
// formatter, so a database name containing characters that mean something
// in a DSN is escaped the way the driver's parser expects.
func buildDSN(config *Config, database string) string {
	c := mysql.NewConfig()
	c.User = config.User
	c.Passwd = config.Password
	c.Net = "tcp"
	c.Addr = net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	c.DBName = database
	c.TLSConfig = config.TLS
	c.ParseTime = true
	c.Timeout = 15 * time.Second
	return c.FormatDSN()
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// databaseAsset is the container the tables, views and sequences hang from.
// It needs no connection, so its identity can be checked in a unit test.
func (s *Source) databaseAsset() pluginsdk.Asset {
	name := s.config.Database

	metadata := map[string]interface{}{
		"host":     s.config.Host,
		"port":     s.config.Port,
		"database": name,
	}

	mrnValue := assetMRN("Database", name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Database",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// describeServer adds the server version and the database's character set
// to the database asset. Both are best effort: a failed lookup leaves them
// out rather than failing the run.
func (s *Source) describeServer(ctx context.Context, a *pluginsdk.Asset) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var version string
	if err := s.db.QueryRowContext(queryCtx, "SELECT VERSION()").Scan(&version); err != nil {
		log.Warn().Err(err).Msg("Failed to read server version")
	} else {
		a.Metadata["server_version"] = version
	}

	var charset, collation sql.NullString
	err := s.db.QueryRowContext(queryCtx, `
		SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
		FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME = ?
	`, s.config.Database).Scan(&charset, &collation)
	if err != nil {
		log.Warn().Err(err).Str("database", s.config.Database).Msg("Failed to read database character set")
		return
	}
	if charset.Valid {
		a.Metadata["character_set"] = charset.String
	}
	if collation.Valid {
		a.Metadata["collation"] = collation.String
	}
}

// objectKind maps an information_schema.TABLES.TABLE_TYPE onto the asset
// type Marmot shows and the object_type recorded in metadata. A
// system-versioned table is still a table; the flag is kept separately.
// Unknown types (for example the current session's temporary tables)
// return ok=false and are skipped.
func objectKind(tableType string) (assetType, kind string, systemVersioned, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(tableType)) {
	case "BASE TABLE":
		return "Table", "table", false, true
	case "SYSTEM VERSIONED":
		return "Table", "table", true, true
	case "VIEW":
		return "View", "view", false, true
	case "SEQUENCE":
		return "Sequence", "sequence", false, true
	default:
		return "", "", false, false
	}
}

// tablesQuery lists the configured database's objects. Older MariaDB
// servers have no TEMPORARY column, so they get a NULL placeholder in its
// place and the scan stays the same.
func tablesQuery(hasTemporary bool) string {
	temporary := "NULL"
	if hasTemporary {
		temporary = "TEMPORARY"
	}
	return fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			TABLE_TYPE,
			ENGINE,
			TABLE_ROWS,
			DATA_LENGTH,
			INDEX_LENGTH,
			AUTO_INCREMENT,
			TABLE_COLLATION,
			CREATE_TIME,
			UPDATE_TIME,
			TABLE_COMMENT,
			%s AS temporary
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`, temporary)
}

func (s *Source) hasTemporaryColumn(ctx context.Context) bool {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var count int
	err := s.db.QueryRowContext(queryCtx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = 'information_schema'
		  AND TABLE_NAME = 'TABLES'
		  AND COLUMN_NAME = 'TEMPORARY'
	`).Scan(&count)
	if err != nil {
		log.Debug().Err(err).Msg("Could not check for the TEMPORARY column, assuming an older server")
		return false
	}
	return count > 0
}

// discoverObjects reads every table, view and sequence of the configured
// database in one pass. Row count and size statistics come from the same
// rows, so they are collected here rather than with a second query.
func (s *Source) discoverObjects(ctx context.Context) ([]pluginsdk.Asset, []pluginsdk.Statistic, error) {
	hasTemporary := s.hasTemporaryColumn(ctx)

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, tablesQuery(hasTemporary), s.config.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var assets []pluginsdk.Asset
	var statistics []pluginsdk.Statistic

	for rows.Next() {
		var (
			schemaName    string
			objectName    string
			tableType     string
			engine        sql.NullString
			rowCount      sql.NullInt64
			dataLength    sql.NullInt64
			indexLength   sql.NullInt64
			autoIncrement sql.NullInt64
			collation     sql.NullString
			created       sql.NullTime
			updated       sql.NullTime
			comment       sql.NullString
			temporary     sql.NullString
		)

		if err := rows.Scan(
			&schemaName, &objectName, &tableType, &engine, &rowCount,
			&dataLength, &indexLength, &autoIncrement, &collation,
			&created, &updated, &comment, &temporary,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan table row")
			continue
		}

		assetType, kind, systemVersioned, ok := objectKind(tableType)
		if !ok {
			log.Debug().Str("name", objectName).Str("type", tableType).Msg("Skipping object of unsupported type")
			continue
		}
		if assetType == "View" && !s.config.IncludeViews {
			continue
		}
		if assetType == "Sequence" && !s.config.IncludeSequences {
			continue
		}

		log.Debug().
			Str("name", objectName).
			Str("type", tableType).
			Str("engine", engine.String).
			Msg("Found database object")

		metadata := map[string]interface{}{
			"host":        s.config.Host,
			"port":        s.config.Port,
			"database":    s.config.Database,
			"schema":      schemaName,
			"table_name":  objectName,
			"object_type": kind,
		}

		if engine.Valid {
			metadata["engine"] = engine.String
		}
		if collation.Valid {
			metadata["collation"] = collation.String
		}
		if rowCount.Valid && s.config.IncludeRowCounts {
			metadata["row_count"] = rowCount.Int64
		}
		if dataLength.Valid {
			metadata["data_length"] = dataLength.Int64
		}
		if indexLength.Valid {
			metadata["index_length"] = indexLength.Int64
		}
		if autoIncrement.Valid {
			metadata["auto_increment"] = autoIncrement.Int64
		}
		if created.Valid {
			metadata["created"] = created.Time.Format("2006-01-02 15:04:05")
		}
		if updated.Valid {
			metadata["updated"] = updated.Time.Format("2006-01-02 15:04:05")
		}
		if temporary.Valid {
			metadata["temporary"] = strings.EqualFold(temporary.String, "Y")
		}

		// information_schema reports the literal "VIEW" as a view's comment,
		// which is not a description anyone wrote.
		var description *string
		if assetType != "View" && comment.Valid && comment.String != "" {
			metadata["comment"] = comment.String
			desc := comment.String
			description = &desc
		}

		if assetType == "Table" {
			metadata["system_versioned"] = systemVersioned
		}

		mrnValue := assetMRN(assetType, objectName)

		assets = append(assets, pluginsdk.Asset{
			Name:        &objectName,
			MRN:         &mrnValue,
			Type:        assetType,
			Providers:   []string{provider},
			Description: description,
			Metadata:    metadata,
			Schema:      make(map[string]string),
			Tags:        pluginsdk.InterpolateTags(s.config.Tags, metadata),
			Sources: []pluginsdk.AssetSource{{
				Name:       provider,
				LastSyncAt: time.Now(),
				Properties: metadata,
				Priority:   1,
			}},
		})

		if assetType == "Table" && s.config.IncludeStatistics {
			statistics = append(statistics, tableStatistics(mrnValue, rowCount, dataLength, indexLength)...)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating table rows: %w", err)
	}

	return assets, statistics, nil
}

// tableStatistics maps one information_schema.TABLES row onto the row count
// and size metrics. TABLE_ROWS is InnoDB's estimate, refreshed by ANALYZE
// TABLE, so it is reported as is rather than replaced by a COUNT(*).
func tableStatistics(assetMRN string, rowCount, dataLength, indexLength sql.NullInt64) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	if rowCount.Valid {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   assetMRN,
			MetricName: "asset.row_count",
			Value:      float64(rowCount.Int64),
		})
	}

	if dataLength.Valid || indexLength.Valid {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   assetMRN,
			MetricName: "asset.size_bytes",
			Value:      float64(dataLength.Int64 + indexLength.Int64),
		})
	}

	return statistics
}

// column is the per-column shape serialised into an asset's schema. It
// extends the SDK's canonical column with the MariaDB-specific flags.
type column struct {
	pluginsdk.Column
	IsAutoIncrement bool   `json:"is_auto_increment,omitempty"`
	IsGenerated     bool   `json:"is_generated,omitempty"`
	IsInvisible     bool   `json:"is_invisible,omitempty"`
	CharacterSet    string `json:"character_set,omitempty"`
	Collation       string `json:"collation,omitempty"`
}

// columnFlags reads the flags MariaDB packs into information_schema.COLUMNS
// EXTRA, for example "auto_increment", "VIRTUAL GENERATED" or
// "STORED GENERATED, INVISIBLE".
func columnFlags(extra string) (autoIncrement, generated, invisible bool) {
	upper := strings.ToUpper(extra)
	return strings.Contains(upper, "AUTO_INCREMENT"),
		strings.Contains(upper, "GENERATED"),
		strings.Contains(upper, "INVISIBLE")
}

// attachColumns reads every column of the database in one query and sets
// the schema of each table and view. Sequences have internal columns too,
// but those describe the sequence's bookkeeping, not data, so they are
// left out.
func (s *Source) attachColumns(ctx context.Context, assets []pluginsdk.Asset) error {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			TABLE_NAME,
			COLUMN_NAME,
			COLUMN_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_KEY,
			EXTRA,
			CHARACTER_SET_NAME,
			COLLATION_NAME,
			COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION
	`

	rows, err := s.db.QueryContext(queryCtx, query, s.config.Database)
	if err != nil {
		return fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	columns := make(map[string][]column)

	for rows.Next() {
		var (
			tableName     string
			columnName    string
			columnType    string
			isNullable    string
			columnDefault sql.NullString
			columnKey     sql.NullString
			extra         sql.NullString
			charset       sql.NullString
			collation     sql.NullString
			comment       sql.NullString
		)

		if err := rows.Scan(
			&tableName, &columnName, &columnType, &isNullable, &columnDefault,
			&columnKey, &extra, &charset, &collation, &comment,
		); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		autoIncrement, generated, invisible := columnFlags(extra.String)

		c := column{
			Column: pluginsdk.Column{
				Name:        columnName,
				DataType:    columnType,
				Nullable:    strings.EqualFold(isNullable, "YES"),
				PrimaryKey:  columnKey.String == "PRI",
				Description: comment.String,
			},
			IsAutoIncrement: autoIncrement,
			IsGenerated:     generated,
			IsInvisible:     invisible,
			CharacterSet:    charset.String,
			Collation:       collation.String,
		}
		if columnDefault.Valid {
			c.Default = columnDefault.String
		}

		columns[tableName] = append(columns[tableName], c)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating column rows: %w", err)
	}

	for i := range assets {
		if assets[i].Type != "Table" && assets[i].Type != "View" {
			continue
		}
		cols, ok := columns[*assets[i].Name]
		if !ok {
			continue
		}
		if err := pluginsdk.SetColumns(&assets[i], cols); err != nil {
			log.Warn().Err(err).Str("table", *assets[i].Name).Msg("Failed to set columns")
		}
	}

	return nil
}

// viewReferencePattern finds the object named right after FROM or JOIN.
// MariaDB stores view definitions normalised: names are backticked and
// qualified with the database, comma joins are rewritten as JOIN, and a
// derived table opens with "(select", so the first token after FROM may be
// a parenthesis.
var viewReferencePattern = regexp.MustCompile(
	"(?i)\\b(?:from|join)\\s*\\(*\\s*(`(?:[^`]|``)+`|[A-Za-z0-9_$]+)(?:\\s*\\.\\s*(`(?:[^`]|``)+`|[A-Za-z0-9_$]+))?",
)

// viewReferences extracts the names of the objects a view definition reads
// from, keeping only references to the given database. The caller still has
// to check each name against the objects it discovered: the match is
// syntactic, so a keyword such as "select" or "dual" can come back too.
func viewReferences(definition, database string) []string {
	var names []string
	seen := make(map[string]struct{})

	for _, match := range viewReferencePattern.FindAllStringSubmatch(definition, -1) {
		qualifier, name := "", unquoteIdentifier(match[1])
		if match[2] != "" {
			qualifier, name = name, unquoteIdentifier(match[2])
		}
		if qualifier != "" && qualifier != database {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}

// unquoteIdentifier strips the backticks around an identifier and undoes the
// doubling that escapes a literal backtick inside one.
func unquoteIdentifier(id string) string {
	if len(id) >= 2 && strings.HasPrefix(id, "`") && strings.HasSuffix(id, "`") {
		return strings.ReplaceAll(id[1:len(id)-1], "``", "`")
	}
	return id
}

// attachViewDefinitions sets each view's query and view metadata, and
// returns a VIEW_OF edge into the view from every discovered table or view
// it reads. The edge follows the data: base table -> view, the direction
// the MongoDB plugin uses for its views.
func (s *Source) attachViewDefinitions(ctx context.Context, assets []pluginsdk.Asset) ([]pluginsdk.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, VIEW_DEFINITION, CHECK_OPTION, IS_UPDATABLE, DEFINER, SECURITY_TYPE
		FROM information_schema.VIEWS
		WHERE TABLE_SCHEMA = ?
	`

	rows, err := s.db.QueryContext(queryCtx, query, s.config.Database)
	if err != nil {
		return nil, fmt.Errorf("querying views: %w", err)
	}
	defer rows.Close()

	type viewInfo struct {
		definition, checkOption, isUpdatable, definer, securityType sql.NullString
	}
	views := make(map[string]viewInfo)

	for rows.Next() {
		var name string
		var v viewInfo
		if err := rows.Scan(&name, &v.definition, &v.checkOption, &v.isUpdatable, &v.definer, &v.securityType); err != nil {
			log.Warn().Err(err).Msg("Failed to scan view row")
			continue
		}
		views[name] = v
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating view rows: %w", err)
	}

	// Only objects discovered in this run can be the target of an edge.
	byName := make(map[string]*pluginsdk.Asset, len(assets))
	for i := range assets {
		byName[*assets[i].Name] = &assets[i]
	}

	var lineages []pluginsdk.LineageEdge
	sqlLanguage := "SQL"

	for i := range assets {
		if assets[i].Type != "View" {
			continue
		}
		v, ok := views[*assets[i].Name]
		if !ok {
			continue
		}

		if v.checkOption.Valid {
			assets[i].Metadata["check_option"] = v.checkOption.String
		}
		if v.isUpdatable.Valid {
			assets[i].Metadata["is_updatable"] = strings.EqualFold(v.isUpdatable.String, "YES")
		}
		if v.definer.Valid {
			assets[i].Metadata["definer"] = v.definer.String
		}
		if v.securityType.Valid {
			assets[i].Metadata["security_type"] = v.securityType.String
		}

		// VIEW_DEFINITION is empty when the user lacks SHOW VIEW.
		if !v.definition.Valid || v.definition.String == "" {
			log.Warn().Str("view", *assets[i].Name).Msg("View definition is empty, grant SHOW VIEW to read it")
			continue
		}

		definition := v.definition.String
		assets[i].Query = &definition
		assets[i].QueryLanguage = &sqlLanguage

		for _, ref := range viewReferences(definition, s.config.Database) {
			base, ok := byName[ref]
			if !ok || base.Type == "Sequence" || *base.MRN == *assets[i].MRN {
				continue
			}
			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: *base.MRN,
				Target: *assets[i].MRN,
				Type:   "VIEW_OF",
			})
		}
	}

	return lineages, nil
}

// sequenceMetadata picks the sequence settings out of a "SELECT * FROM
// <sequence>" row. The row is read by column name because the set of
// columns differs between MariaDB versions.
func sequenceMetadata(columns []string, values []interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})

	for i, name := range columns {
		if i >= len(values) {
			break
		}
		value, ok := toInt64(values[i])
		if !ok {
			continue
		}
		switch strings.ToLower(name) {
		case "start_value", "minimum_value", "maximum_value", "increment", "cache_size":
			metadata[strings.ToLower(name)] = value
		case "cycle_option":
			metadata["cycle_option"] = value != 0
		}
	}

	return metadata
}

// toInt64 reads an integer the driver returned either typed or as the raw
// text of a non-prepared query.
func toInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case uint64:
		return int64(v), true
	case []byte:
		n, err := strconv.ParseInt(string(v), 10, 64)
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

// attachSequenceDetails reads each sequence's settings. A sequence the user
// cannot read keeps its listing metadata and logs a warning.
func (s *Source) attachSequenceDetails(ctx context.Context, assets []pluginsdk.Asset) {
	for i := range assets {
		if assets[i].Type != "Sequence" {
			continue
		}

		metadata, err := s.readSequence(ctx, *assets[i].Name)
		if err != nil {
			log.Warn().Err(err).Str("sequence", *assets[i].Name).Msg("Failed to read sequence settings")
			continue
		}
		for k, v := range metadata {
			assets[i].Metadata[k] = v
		}
	}
}

func (s *Source) readSequence(ctx context.Context, name string) (map[string]interface{}, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	//nolint:gosec // G201: identifiers are quoted by quoteIdentifier
	query := fmt.Sprintf("SELECT * FROM %s.%s", quoteIdentifier(s.config.Database), quoteIdentifier(name))

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying sequence: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading sequence columns: %w", err)
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating sequence row: %w", err)
		}
		return nil, fmt.Errorf("sequence returned no row")
	}

	values := make([]interface{}, len(columns))
	pointers := make([]interface{}, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return nil, fmt.Errorf("scanning sequence row: %w", err)
	}

	return sequenceMetadata(columns, values), nil
}

// discoverForeignKeys emits an edge from each referencing table to the
// table it references. Keys into another database are skipped, since this
// run only creates assets for the configured one.
func (s *Source) discoverForeignKeys(ctx context.Context) ([]pluginsdk.LineageEdge, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT
			kcu.TABLE_NAME AS source_table,
			kcu.COLUMN_NAME AS source_column,
			kcu.REFERENCED_TABLE_SCHEMA AS target_schema,
			kcu.REFERENCED_TABLE_NAME AS target_table,
			kcu.REFERENCED_COLUMN_NAME AS target_column,
			kcu.CONSTRAINT_NAME AS constraint_name
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		LIMIT 1000
	`

	rows, err := s.db.QueryContext(queryCtx, query, s.config.Database)
	if err != nil {
		return nil, fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	var lineages []pluginsdk.LineageEdge
	uniqueRelations := make(map[string]struct{})

	for rows.Next() {
		var (
			sourceTable    string
			sourceColumn   string
			targetSchema   sql.NullString
			targetTable    sql.NullString
			targetColumn   sql.NullString
			constraintName string
		)

		if err := rows.Scan(&sourceTable, &sourceColumn, &targetSchema, &targetTable, &targetColumn, &constraintName); err != nil {
			log.Warn().Err(err).Msg("Failed to scan foreign key row")
			continue
		}

		if !targetSchema.Valid || !targetTable.Valid {
			continue
		}

		log.Debug().
			Str("source", sourceTable+"."+sourceColumn).
			Str("target", targetSchema.String+"."+targetTable.String+"."+targetColumn.String).
			Str("constraint", constraintName).
			Msg("Found foreign key relationship")

		if targetSchema.String != s.config.Database {
			log.Debug().
				Str("constraint", constraintName).
				Str("references", targetSchema.String+"."+targetTable.String).
				Msg("Skipping foreign key to a table outside the configured database")
			continue
		}

		sourceMRN := assetMRN("Table", sourceTable)
		targetMRN := assetMRN("Table", targetTable.String)

		if sourceMRN == targetMRN {
			continue
		}

		relationKey := sourceMRN + ":" + targetMRN
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

// columnCountStatistics emits a column count for every table and view. It
// counts through information_schema so the number is right even when column
// details are turned off.
func (s *Source) columnCountStatistics(ctx context.Context, assets []pluginsdk.Asset) []pluginsdk.Statistic {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, `
		SELECT TABLE_NAME, COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
		GROUP BY TABLE_NAME
	`, s.config.Database)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to collect column counts")
		return nil
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column count row")
			continue
		}
		counts[name] = count
	}
	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Msg("Failed to iterate column counts")
		return nil
	}

	var statistics []pluginsdk.Statistic
	for _, a := range assets {
		if a.Type != "Table" && a.Type != "View" {
			continue
		}
		count, ok := counts[*a.Name]
		if !ok {
			continue
		}
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *a.MRN,
			MetricName: "asset.column_count",
			Value:      float64(count),
		})
	}

	return statistics
}

// FetchSampleData implements pluginsdk.DataFetcher: it returns the first
// rows of a table, view or sequence for the asset preview.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}

	if _, err := s.Validate(config); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	database, _ := a.Metadata["database"].(string)
	table, _ := a.Metadata["table_name"].(string)

	if database == "" {
		return nil, nil, fmt.Errorf("could not determine database from asset metadata")
	}
	if table == "" && a.Name != nil {
		table = *a.Name
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.initConnection(fetchCtx, database); err != nil {
		return nil, nil, fmt.Errorf("connecting to database %s: %w", database, err)
	}
	defer s.closeConnection()

	//nolint:gosec // G201: identifiers are quoted by quoteIdentifier
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 20", quoteIdentifier(database), quoteIdentifier(table))

	log.Debug().Str("database", database).Str("table", table).Msg("Fetching sample data")

	rows, err := s.db.QueryContext(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("getting column names: %w", err)
	}

	var dataRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		pointers := make([]interface{}, len(columnNames))
		for i := range columnNames {
			pointers[i] = &values[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row, skipping")
			continue
		}

		converted := make([]interface{}, len(values))
		for i, val := range values {
			converted[i] = convertValue(val)
		}
		dataRows = append(dataRows, converted)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	log.Debug().Int("columns", len(columnNames)).Int("rows", len(dataRows)).Msg("Successfully fetched sample data")

	return columnNames, dataRows, nil
}

// quoteIdentifier wraps an identifier in backticks. A SELECT cannot bind an
// identifier as a parameter, so the name has to be quoted by hand.
func quoteIdentifier(id string) string {
	id = strings.ReplaceAll(id, "\x00", "")
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}

// convertValue turns the driver's raw values into JSON-friendly ones. Text
// comes back as bytes from a non-prepared query; genuine binary data is
// rendered as hex.
func convertValue(val interface{}) interface{} {
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("0x%x", v)
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return val
	}
}

// assetMRN is the single place a MariaDB MRN is built, so the object pass,
// the lineage passes and the statistics can never address one object
// differently. Names are bare: the database is a separate asset joined by
// CONTAINS edges, which is also the shape the OpenMetadata and Trino
// plugins use for MariaDB tables.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
