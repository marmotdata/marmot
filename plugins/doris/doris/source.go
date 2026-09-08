// Package doris discovers databases, tables, views and materialized views
// from Apache Doris clusters.
package doris

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql" // Doris frontends speak the MySQL protocol
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Doris"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "doris",
		Name:        "Doris",
		Description: "Discover databases, tables, views and materialized views from Apache Doris clusters",
		Icon:        "doris",
		Category:    "data-warehouse",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Doris plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host             string   `json:"host" description:"Doris frontend hostname or IP address" validate:"required"`
	Port             int      `json:"port" description:"Doris frontend MySQL protocol port" default:"9030" validate:"omitempty,min=1,max=65535"`
	User             string   `json:"user" description:"Username for authentication" validate:"required"`
	Password         string   `json:"password" description:"Password for authentication" sensitive:"true"`
	Databases        []string `json:"databases,omitempty" description:"Databases to discover (every database except exclude_databases when empty)"`
	ExcludeDatabases []string `json:"exclude_databases,omitempty" description:"Databases to skip when databases is empty" default:"[\"information_schema\",\"mysql\",\"__internal_schema\"]"`
	Catalog          string   `json:"catalog" description:"Doris catalog to discover" default:"internal"`
	TLS              string   `json:"tls" description:"TLS configuration (false, true, skip-verify, preferred)" default:"false" validate:"omitempty,oneof=false true skip-verify preferred"`

	IncludeColumns           bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews             bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeMaterializedViews bool `json:"include_materialized_views" description:"Whether to discover async materialized views" default:"true"`
	IncludeStatistics        bool `json:"include_statistics" description:"Whether to include row counts, data sizes and column counts" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "doris-fe.internal"
port: 9030
user: "marmot_reader"
password: "${DORIS_PASSWORD}"
databases:
  - "shop"
  - "analytics"
tags:
  - "doris"
  - "warehouse"
`

// Source represents the Doris plugin.
type Source struct {
	config *Config
	db     *sql.DB
	// conn pins every query to one session. SWITCH <catalog> is session
	// state, so a pooled connection could silently run a later query
	// against the internal catalog.
	conn *sql.Conn
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Doris databases, tables, views and materialized views.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx); err != nil {
		return nil, fmt.Errorf("initialising database connection: %w", err)
	}
	defer s.closeConnection()

	version := s.serverVersion(ctx)

	databases, err := s.listDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Str("catalog", s.config.Catalog).
		Strs("databases", databases).
		Msg("Starting Doris discovery")

	var (
		assets     []pluginsdk.Asset
		lineages   []pluginsdk.LineageEdge
		statistics []pluginsdk.Statistic
		pending    []pendingEdge
	)

	// Objects are keyed by database.name so edges that point across
	// databases resolve once every database has been discovered.
	objectMRNs := make(map[string]string)

	for _, dbName := range databases {
		result := s.discoverDatabase(ctx, dbName, version)
		assets = append(assets, result.assets...)
		lineages = append(lineages, result.lineages...)
		statistics = append(statistics, result.statistics...)
		pending = append(pending, result.pending...)
		for key, m := range result.objectMRNs {
			objectMRNs[key] = m
		}
	}

	lineages = append(lineages, resolvePendingEdges(pending, objectMRNs)...)

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Doris discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", s.dsn())
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(15 * time.Minute)

	conn, err := db.Conn(timeoutCtx)
	if err != nil {
		db.Close()
		return fmt.Errorf("connecting: %w", err)
	}

	if err := conn.PingContext(timeoutCtx); err != nil {
		conn.Close()
		db.Close()
		return fmt.Errorf("pinging database: %w", err)
	}

	if !strings.EqualFold(s.config.Catalog, "internal") {
		if _, err := conn.ExecContext(timeoutCtx, "SWITCH "+quoteIdentifier(s.config.Catalog)); err != nil {
			conn.Close()
			db.Close()
			return fmt.Errorf("switching to catalog %s: %w", s.config.Catalog, err)
		}
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Str("catalog", s.config.Catalog).
		Msg("Successfully connected to Doris")

	s.db = db
	s.conn = conn
	return nil
}

// dsn builds the driver connection string. No database is named: every
// query is database qualified, and the user may have no default database
// to land in. Doris only prepares point queries server side, so the driver
// has to interpolate parameters itself. Dates and timestamps are left as
// the text Doris sends, so sample rows and metadata show them unchanged.
func (s *Source) dsn() string {
	cfg := mysql.NewConfig()
	cfg.User = s.config.User
	cfg.Passwd = s.config.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	cfg.TLSConfig = s.config.TLS
	cfg.InterpolateParams = true
	cfg.Timeout = 15 * time.Second
	cfg.ReadTimeout = 30 * time.Second
	return cfg.FormatDSN()
}

func (s *Source) closeConnection() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

// serverVersion returns the Doris build, for example
// doris-2.1.0-rc11-91efb6a43d. SELECT VERSION() only reports the MySQL
// protocol version the frontend imitates.
func (s *Source) serverVersion(ctx context.Context) string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.conn.QueryContext(queryCtx, "SHOW VARIABLES LIKE 'version_comment'")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read the Doris version")
		return ""
	}
	defer rows.Close()

	// Doris 2.x adds Default_Value and Changed columns after Value; scan
	// by position into as many columns as the server returns.
	columns, err := rows.Columns()
	if err != nil {
		return ""
	}
	if !rows.Next() {
		return ""
	}
	values := make([]sql.NullString, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil || len(values) < 2 {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(values[1].String, "Doris version"))
}

// listDatabases returns the configured databases that exist in the catalog,
// or every database minus the excluded ones when none are configured. A
// configured database that does not exist is logged and skipped rather
// than catalogued as an empty container.
func (s *Source) listDatabases(ctx context.Context) ([]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.conn.QueryContext(queryCtx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("querying databases: %w", err)
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Warn().Err(err).Msg("Failed to scan database row")
			continue
		}
		existing = append(existing, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating database rows: %w", err)
	}

	return selectDatabases(existing, s.config.Databases, s.config.ExcludeDatabases), nil
}

// selectDatabases picks the databases to discover out of the ones the
// catalog has. Doris compares database names case-sensitively, so the
// configured names are matched exactly.
func selectDatabases(existing, wanted, excluded []string) []string {
	if len(wanted) > 0 {
		have := make(map[string]struct{}, len(existing))
		for _, name := range existing {
			have[name] = struct{}{}
		}

		var databases []string
		for _, name := range wanted {
			if _, ok := have[name]; !ok {
				log.Warn().Str("database", name).Msg("Configured database not found, skipping")
				continue
			}
			databases = append(databases, name)
		}
		return databases
	}

	skip := make(map[string]struct{}, len(excluded))
	for _, name := range excluded {
		skip[strings.ToLower(name)] = struct{}{}
	}

	var databases []string
	for _, name := range existing {
		if _, ok := skip[strings.ToLower(name)]; ok {
			continue
		}
		databases = append(databases, name)
	}
	return databases
}

// databaseResult is everything one database contributes to the run.
type databaseResult struct {
	assets     []pluginsdk.Asset
	lineages   []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic
	pending    []pendingEdge
	objectMRNs map[string]string
}

// pendingEdge is a lineage edge whose other end is named in SQL rather
// than discovered directly: a view's base table or a foreign key target.
// It is resolved against the discovered objects once every database is in.
type pendingEdge struct {
	ref      tableRef
	otherMRN string
	edgeType string
	// refIsSource says which end the named object is: the base table of a
	// VIEW_OF edge (source) or the referenced table of a FOREIGN_KEY edge
	// (target).
	refIsSource bool
}

// tableRow is one row of information_schema.TABLES.
type tableRow struct {
	name       string
	tableType  string
	engine     sql.NullString
	comment    sql.NullString
	created    sql.NullString
	updated    sql.NullString
	rowCount   sql.NullInt64
	dataLength sql.NullInt64
}

// Doris fills TABLE_COMMENT with the object kind when no comment was set;
// those placeholders are not descriptions.
var placeholderComments = map[string]struct{}{
	"OLAP": {}, "VIEW": {}, "MATERIALIZED_VIEW": {},
}

func (s *Source) discoverDatabase(ctx context.Context, dbName, version string) databaseResult {
	result := databaseResult{objectMRNs: make(map[string]string)}

	tables, err := s.listTables(ctx, dbName)
	if err != nil {
		log.Warn().Err(err).Str("database", dbName).Msg("Failed to list tables")
	}

	// Async materialized views are listed as plain views in
	// information_schema.TABLES; mv_infos tells the two apart and carries
	// the refresh settings and the defining query.
	var mvs map[string]materializedView
	if s.config.IncludeMaterializedViews {
		mvs = s.listMaterializedViews(ctx, dbName)
	}

	var objects []pluginsdk.Asset
	for _, row := range tables {
		objectType := classify(row, mvs)
		switch objectType {
		case "view":
			if !s.config.IncludeViews {
				continue
			}
		case "materialized_view":
			if !s.config.IncludeMaterializedViews {
				continue
			}
		}

		asset, edges, stats := s.buildObject(ctx, dbName, row, objectType, mvs[row.name])
		objects = append(objects, asset)
		result.pending = append(result.pending, edges...)
		result.statistics = append(result.statistics, stats...)
		result.objectMRNs[objectKey(dbName, row.name)] = *asset.MRN
	}

	tableCount, viewCount := 0, 0
	for _, a := range objects {
		if a.Type == "View" {
			viewCount++
		} else {
			tableCount++
		}
	}

	dbAsset := s.databaseAsset(dbName, version, tableCount, viewCount)
	result.assets = append(result.assets, dbAsset)
	result.assets = append(result.assets, objects...)

	for _, a := range objects {
		result.lineages = append(result.lineages, pluginsdk.LineageEdge{
			Source: *dbAsset.MRN,
			Target: *a.MRN,
			Type:   "CONTAINS",
		})
	}

	log.Debug().
		Str("database", dbName).
		Int("tables", tableCount).
		Int("views", viewCount).
		Msg("Discovered database")

	return result
}

func (s *Source) listTables(ctx context.Context, dbName string) ([]tableRow, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `
		SELECT TABLE_NAME, TABLE_TYPE, ENGINE, TABLE_COMMENT,
		       CREATE_TIME, UPDATE_TIME, TABLE_ROWS, DATA_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`

	rows, err := s.conn.QueryContext(queryCtx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var tables []tableRow
	for rows.Next() {
		var row tableRow
		if err := rows.Scan(&row.name, &row.tableType, &row.engine, &row.comment,
			&row.created, &row.updated, &row.rowCount, &row.dataLength); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to scan table row")
			continue
		}
		tables = append(tables, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	return tables, nil
}

// materializedView is one row of mv_infos.
type materializedView struct {
	name          string
	jobName       string
	state         string
	refreshState  string
	refreshInfo   string
	querySQL      string
	partitionInfo string
}

// listMaterializedViews reads the async materialized views of a database.
// Older frontends and external catalogs have no mv_infos; those log and
// return nothing, and the views fall back to being catalogued as views.
func (s *Source) listMaterializedViews(ctx context.Context, dbName string) map[string]materializedView {
	mvs := make(map[string]materializedView)

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf(`SELECT Name, JobName, State, RefreshState, RefreshInfo, QuerySql, MvPartitionInfo FROM mv_infos("database"=%s)`, quoteString(dbName))

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Str("database", dbName).Msg("Materialized view listing unavailable")
		return mvs
	}
	defer rows.Close()

	for rows.Next() {
		var mv materializedView
		var jobName, state, refreshState, refreshInfo, querySQL, partitionInfo sql.NullString
		if err := rows.Scan(&mv.name, &jobName, &state, &refreshState, &refreshInfo, &querySQL, &partitionInfo); err != nil {
			log.Warn().Err(err).Str("database", dbName).Msg("Failed to scan materialized view row")
			continue
		}
		mv.jobName = jobName.String
		mv.state = state.String
		mv.refreshState = refreshState.String
		mv.refreshInfo = refreshInfo.String
		mv.querySQL = querySQL.String
		mv.partitionInfo = partitionInfo.String
		mvs[mv.name] = mv
	}

	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Str("database", dbName).Msg("Failed to iterate materialized view rows")
	}

	return mvs
}

// classify maps an information_schema.TABLES row onto the object kinds this
// plugin reports. Doris lists its own storage as engine Doris (OLAP), views
// as engine View, async materialized views as views with no engine, and
// external tables under the name of the system they read from.
func classify(row tableRow, mvs map[string]materializedView) string {
	if _, ok := mvs[row.name]; ok {
		return "materialized_view"
	}

	engine := strings.ToUpper(row.engine.String)
	if strings.Contains(strings.ToUpper(row.tableType), "VIEW") {
		if engine == "" || engine == "MATERIALIZED_VIEW" {
			return "materialized_view"
		}
		return "view"
	}

	switch engine {
	case "DORIS", "OLAP", "":
		return "table"
	case "VIEW":
		return "view"
	case "MATERIALIZED_VIEW":
		return "materialized_view"
	default:
		return "external_table"
	}
}

func (s *Source) buildObject(ctx context.Context, dbName string, row tableRow, objectType string, mv materializedView) (pluginsdk.Asset, []pendingEdge, []pluginsdk.Statistic) {
	name := dbName + "." + row.name

	assetType := "Table"
	if objectType == "view" || objectType == "materialized_view" {
		assetType = "View"
	}

	metadata := map[string]any{
		"catalog":     s.config.Catalog,
		"database":    dbName,
		"table_name":  row.name,
		"object_type": objectType,
	}
	if row.engine.Valid && row.engine.String != "" {
		metadata["engine"] = row.engine.String
	}
	setIfNotEmpty(metadata, "created", row.created.String)
	setIfNotEmpty(metadata, "updated", row.updated.String)

	var description string
	if row.comment.Valid {
		if _, placeholder := placeholderComments[row.comment.String]; !placeholder && row.comment.String != "" {
			description = row.comment.String
			metadata["comment"] = description
		}
	}

	var (
		query   string
		refs    []tableRef
		pending []pendingEdge
	)

	switch objectType {
	case "view":
		if ddl := s.showCreate(ctx, "VIEW", dbName, row.name); ddl != "" {
			metadata["ddl"] = ddl
			query = extractViewQuery(ddl)
			refs = referencedTables(query)
		}
	case "materialized_view":
		metadata["materialized"] = true
		if ddl := s.showCreate(ctx, "TABLE", dbName, row.name); ddl != "" {
			metadata["ddl"] = ddl
			addDDLMetadata(metadata, parseTableDDL(ddl))
		}
		if count, ok := s.partitionCount(ctx, dbName, row.name); ok {
			metadata["partition_count"] = count
		}
		if mv.name != "" {
			setIfNotEmpty(metadata, "job_name", mv.jobName)
			setIfNotEmpty(metadata, "state", mv.state)
			setIfNotEmpty(metadata, "refresh_state", mv.refreshState)
			setIfNotEmpty(metadata, "refresh_info", mv.refreshInfo)
			setIfNotEmpty(metadata, "mv_partition_info", mv.partitionInfo)
			query = mv.querySQL
			refs = referencedTables(query)
		}
	default:
		if ddl := s.showCreate(ctx, "TABLE", dbName, row.name); ddl != "" {
			metadata["ddl"] = ddl
			addDDLMetadata(metadata, parseTableDDL(ddl))
		}
		if objectType == "table" {
			if count, ok := s.partitionCount(ctx, dbName, row.name); ok {
				metadata["partition_count"] = count
			}
			for _, fk := range s.foreignKeys(ctx, dbName, row.name) {
				ref := fk.ReferencedTable
				if ref.Database == "" {
					ref.Database = dbName
				}
				pending = append(pending, pendingEdge{
					ref:      ref,
					otherMRN: assetMRN(assetType, name),
					edgeType: "FOREIGN_KEY",
				})
			}
		}
	}

	mrnValue := assetMRN(assetType, name)

	for _, ref := range refs {
		if ref.Database == "" {
			ref.Database = dbName
		}
		if ref.Database == dbName && ref.Table == row.name {
			continue
		}
		pending = append(pending, pendingEdge{
			ref:         ref,
			otherMRN:    mrnValue,
			edgeType:    "VIEW_OF",
			refIsSource: true,
		})
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if description != "" {
		asset.Description = &description
	}
	if query != "" {
		lang := "SQL"
		asset.Query = &query
		asset.QueryLanguage = &lang
	}

	var columnCount int
	if s.config.IncludeColumns {
		keyModel, _ := metadata["key_model"].(string)
		columns, err := s.columns(ctx, dbName, row.name, keyModel)
		if err != nil {
			log.Warn().Err(err).Str("table", name).Msg("Failed to read columns")
		} else {
			columnCount = len(columns)
			if err := pluginsdk.SetColumns(&asset, columns); err != nil {
				log.Warn().Err(err).Str("table", name).Msg("Failed to set columns")
			}
		}
	}

	var statistics []pluginsdk.Statistic
	if s.config.IncludeStatistics {
		statistics = objectStatistics(mrnValue, objectType, row, columnCount)
	}

	return asset, pending, statistics
}

// addDDLMetadata copies what parseTableDDL found into the metadata map,
// leaving out anything the DDL did not state. The engine is only taken
// from the DDL when information_schema left it empty, which it does for
// materialized views.
func addDDLMetadata(metadata map[string]any, ddl tableDDL) {
	if _, ok := metadata["engine"]; !ok && ddl.Engine != "" {
		metadata["engine"] = ddl.Engine
	}

	if ddl.KeyModel != "" {
		metadata["key_model"] = ddl.KeyModel
		metadata["key_columns"] = ddl.KeyColumns
	} else {
		metadata["key_model"] = "none"
	}

	if ddl.PartitionType != "" {
		metadata["partition_type"] = ddl.PartitionType
		metadata["partition_columns"] = ddl.PartitionColumns
	} else {
		metadata["partition_type"] = "none"
	}

	if ddl.DistributionType != "" {
		metadata["distribution_type"] = ddl.DistributionType
		if len(ddl.DistributionColumns) > 0 {
			metadata["distribution_columns"] = ddl.DistributionColumns
		}
		if ddl.AutoBucket {
			metadata["auto_bucket"] = true
		} else if ddl.Buckets > 0 {
			metadata["buckets"] = ddl.Buckets
		}
	}

	if replication, ok := ddl.Properties["replication_allocation"]; ok {
		metadata["replication"] = replication
	} else if replication, ok := ddl.Properties["replication_num"]; ok {
		metadata["replication"] = replication
	}
	setIfNotEmpty(metadata, "storage_medium", ddl.Properties["storage_medium"])
}

func setIfNotEmpty(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

// showCreate returns the DDL of a table or view, or "" when Doris will not
// give it (a missing privilege, an object dropped mid-run). SHOW CREATE
// returns the name in the first column and the DDL in the second for
// tables, views and materialized views alike.
func (s *Source) showCreate(ctx context.Context, kind, dbName, objectName string) string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW CREATE %s %s.%s", kind, quoteIdentifier(dbName), quoteIdentifier(objectName))

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Str("object", dbName+"."+objectName).Msg("Failed to read DDL")
		return ""
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil || len(columns) < 2 || !rows.Next() {
		return ""
	}

	values := make([]sql.NullString, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		log.Warn().Err(err).Str("object", dbName+"."+objectName).Msg("Failed to scan DDL")
		return ""
	}

	return values[1].String
}

// partitionCount counts the partitions of an OLAP table. Doris gives an
// unpartitioned table one partition named after the table, so the count
// is never below one for a table that exists.
func (s *Source) partitionCount(ctx context.Context, dbName, tableName string) (int, bool) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW PARTITIONS FROM %s.%s", quoteIdentifier(dbName), quoteIdentifier(tableName))

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Str("table", dbName+"."+tableName).Msg("Failed to read partitions")
		return 0, false
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Str("table", dbName+"."+tableName).Msg("Failed to iterate partitions")
		return 0, false
	}

	return count, true
}

// foreignKeys reads the FOREIGN KEY constraints declared on a table. Doris
// records them for the planner without enforcing them, and does not
// surface them in information_schema.KEY_COLUMN_USAGE, so SHOW CONSTRAINTS
// is the only place they can be read.
func (s *Source) foreignKeys(ctx context.Context, dbName, tableName string) []foreignKey {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW CONSTRAINTS FROM %s.%s", quoteIdentifier(dbName), quoteIdentifier(tableName))

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Str("table", dbName+"."+tableName).Msg("Constraint listing unavailable")
		return nil
	}
	defer rows.Close()

	var keys []foreignKey
	for rows.Next() {
		var name, kind, definition sql.NullString
		if err := rows.Scan(&name, &kind, &definition); err != nil {
			log.Warn().Err(err).Str("table", dbName+"."+tableName).Msg("Failed to scan constraint row")
			continue
		}
		if !strings.EqualFold(kind.String, "FOREIGN KEY") {
			continue
		}
		if fk, ok := parseForeignKey(definition.String); ok {
			keys = append(keys, fk)
		}
	}

	return keys
}

// column is the per-column shape serialised into an asset's schema. On top
// of the SDK fields it records Doris's key flag and, for AGGREGATE tables,
// how a value column is aggregated.
type column struct {
	pluginsdk.Column
	AggregationType string `json:"aggregation_type,omitempty"`
	IsKey           bool   `json:"is_key,omitempty"`
}

// columns reads SHOW FULL COLUMNS, which keeps the declared type verbatim
// (decimalv3(9, 2), array<int>), the defaults and the aggregation type in
// Extra. information_schema.COLUMNS loses all three. Key columns identify a
// row on UNIQUE and AGGREGATE tables, so they are the primary key there; on
// a DUPLICATE table they only order the data and are the sorting key.
func (s *Source) columns(ctx context.Context, dbName, tableName, keyModel string) ([]column, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW FULL COLUMNS FROM %s.%s", quoteIdentifier(dbName), quoteIdentifier(tableName))

	rows, err := s.conn.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	var columns []column
	for rows.Next() {
		var field, dataType, collation, nullable, key, defaultValue, extra, privileges, comment sql.NullString
		if err := rows.Scan(&field, &dataType, &collation, &nullable, &key, &defaultValue, &extra, &privileges, &comment); err != nil {
			return nil, fmt.Errorf("scanning column row: %w", err)
		}

		isKey := strings.EqualFold(key.String, "YES")
		col := column{
			Column: pluginsdk.Column{
				Name:        field.String,
				DataType:    dataType.String,
				Nullable:    strings.EqualFold(nullable.String, "YES"),
				PrimaryKey:  isKey && keyModel != "DUPLICATE",
				SortingKey:  isKey && keyModel == "DUPLICATE",
				Description: comment.String,
			},
			IsKey: isKey,
		}
		// HLL and BITMAP columns report a NUL byte as their default, which
		// is an internal marker rather than a value anyone declared.
		if defaultValue.Valid && strings.Trim(defaultValue.String, "\x00") != "" {
			col.Default = defaultValue.String
		}
		if extra.Valid && extra.String != "" && !strings.EqualFold(extra.String, "NONE") {
			col.AggregationType = extra.String
		}
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return columns, nil
}

// objectStatistics turns the information_schema counters into metrics.
// Row and size counters only mean something for objects Doris stores
// itself; a view has none and an external table's are not maintained.
func objectStatistics(mrnValue, objectType string, row tableRow, columnCount int) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	if columnCount > 0 {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   mrnValue,
			MetricName: "asset.column_count",
			Value:      float64(columnCount),
		})
	}

	if objectType != "table" && objectType != "materialized_view" {
		return statistics
	}

	if row.rowCount.Valid {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   mrnValue,
			MetricName: "asset.row_count",
			Value:      float64(row.rowCount.Int64),
		})
	}
	if row.dataLength.Valid {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   mrnValue,
			MetricName: "asset.size_bytes",
			Value:      float64(row.dataLength.Int64),
		})
	}

	return statistics
}

// resolvePendingEdges turns SQL-level references into edges between
// discovered objects. A reference that matches nothing (an alias, a CTE, a
// table in an undiscovered database) is dropped: the server would drop an
// edge to a missing asset anyway.
func resolvePendingEdges(pending []pendingEdge, objectMRNs map[string]string) []pluginsdk.LineageEdge {
	var lineages []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, p := range pending {
		refMRN, ok := objectMRNs[objectKey(p.ref.Database, p.ref.Table)]
		if !ok {
			log.Debug().
				Str("reference", p.ref.Database+"."+p.ref.Table).
				Str("edge", p.edgeType).
				Msg("Skipping edge, referenced object not discovered")
			continue
		}

		edge := pluginsdk.LineageEdge{Source: p.otherMRN, Target: refMRN, Type: p.edgeType}
		if p.refIsSource {
			edge.Source, edge.Target = refMRN, p.otherMRN
		}
		if edge.Source == edge.Target {
			continue
		}

		key := edge.Type + ":" + edge.Source + ":" + edge.Target
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		lineages = append(lineages, edge)
	}

	return lineages
}

// objectKey is how discovered objects are looked up when resolving
// references. Doris matches table names case-insensitively by default.
func objectKey(dbName, tableName string) string {
	return strings.ToLower(dbName) + "." + strings.ToLower(tableName)
}

// databaseAsset is the container the discovered objects hang from. Its
// identity matches what an OpenMetadata import produces for a Doris
// service, where the Doris database is the schema level.
func (s *Source) databaseAsset(dbName, version string, tableCount, viewCount int) pluginsdk.Asset {
	name := dbName
	mrnValue := assetMRN("Database", name)

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"catalog":     s.config.Catalog,
		"database":    dbName,
		"table_count": tableCount,
		"view_count":  viewCount,
	}
	setIfNotEmpty(metadata, "doris_version", version)

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

// FetchSampleData implements the DataFetcher interface to retrieve sample
// rows from a Doris table or view.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
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
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.initConnection(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("connecting to Doris: %w", err)
	}
	defer s.closeConnection()

	//nolint:gosec // G201: inputs sanitised via quoteIdentifier
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 20", quoteIdentifier(database), quoteIdentifier(table))

	log.Debug().Str("database", database).Str("table", table).Msg("Fetching sample data")

	rows, err := s.conn.QueryContext(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("getting column names: %w", err)
	}

	var dataRows [][]any
	for rows.Next() {
		values := make([]any, len(columnNames))
		ptrs := make([]any, len(columnNames))
		for i := range columnNames {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row, skipping")
			continue
		}

		converted := make([]any, len(values))
		for i, val := range values {
			converted[i] = convertValue(val)
		}
		dataRows = append(dataRows, converted)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	return columnNames, dataRows, nil
}

// convertValue turns driver values into JSON-friendly ones. Doris sends
// decimals, dates and complex types (arrays, maps, structs) as text.
func convertValue(val any) any {
	switch v := val.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("0x%x", v)
	default:
		return val
	}
}

// quoteIdentifier wraps an identifier in backticks. Doris reserves more
// words than MySQL, so every name is quoted rather than only the risky ones.
func quoteIdentifier(id string) string {
	id = strings.ReplaceAll(id, "\x00", "")
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}

// quoteString wraps a value in double quotes for the table-valued function
// arguments Doris takes as "key"="value" pairs, which cannot be bound.
func quoteString(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, `\`, `\\`)
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

// assetMRN is the single place a Doris MRN is built. Objects are named
// database.table: the Doris database is the level that keeps two same
// named tables apart, and it is the schema level of an OpenMetadata
// import, so both routes land on one asset.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
