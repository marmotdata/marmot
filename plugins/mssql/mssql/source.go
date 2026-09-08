// Package mssql discovers databases, tables, views and routines from
// Microsoft SQL Server and Azure SQL instances.
package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	_ "github.com/microsoft/go-mssqldb" // pure-Go TDS driver, registered as "sqlserver"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "mssql",
		Name:        "SQL Server",
		Description: "Discover databases, tables, views and routines from Microsoft SQL Server instances",
		Icon:        "sql-server",
		Category:    "database",
		Status:      "experimental",
		// Discover emits CONTAINS, FOREIGN_KEY and VIEW_OF edges, so the
		// manifest declares Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the SQL Server plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"SQL Server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"SQL Server port" default:"1433" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Login to authenticate with. Use DOMAIN\\user for Windows authentication" validate:"required"`
	Password string `json:"password" description:"Password for the login" sensitive:"true" validate:"required"`
	Database string `json:"database" description:"Discover only this database. Leave empty to discover every database the login can open"`

	ExcludeDatabases []string `json:"exclude_databases" description:"Databases to skip" default:"[\"master\",\"model\",\"msdb\",\"tempdb\"]"`
	ExcludeSchemas   []string `json:"exclude_schemas" description:"Schemas to skip" default:"[\"sys\",\"INFORMATION_SCHEMA\",\"guest\",\"db_owner\",\"db_accessadmin\",\"db_securityadmin\",\"db_ddladmin\",\"db_backupoperator\",\"db_datareader\",\"db_datawriter\",\"db_denydatareader\",\"db_denydatawriter\"]"`

	Encrypt                bool   `json:"encrypt" description:"Require an encrypted connection" default:"true"`
	TrustServerCertificate bool   `json:"trust_server_certificate" description:"Accept the server certificate without verifying it. Needed for self-signed certificates" default:"false"`
	ConnectTimeoutSeconds  int    `json:"connect_timeout_seconds" description:"Seconds to wait for a connection" default:"30" validate:"omitempty,min=1,max=600"`
	ApplicationIntent      string `json:"application_intent" label:"Application Intent" description:"Connect to a read-only replica with ReadOnly" default:"ReadWrite" validate:"omitempty,oneof=ReadWrite ReadOnly"`

	IncludeColumns      bool `json:"include_columns" description:"Whether to include column information" default:"true"`
	IncludeViews        bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeProcedures   bool `json:"include_procedures" description:"Whether to discover stored procedures and functions" default:"true"`
	DiscoverForeignKeys bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	IncludeStatistics   bool `json:"include_statistics" description:"Whether to collect row counts and table sizes" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "sqlserver.company.com"
port: 1433
user: "marmot_reader"
password: "secure_password_123"
encrypt: true
trust_server_certificate: false
exclude_databases:
  - "master"
  - "model"
  - "msdb"
  - "tempdb"
tags:
  - "sqlserver"
  - "production"
`

// Source represents the SQL Server plugin.
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

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// discovery accumulates one run's output. Passing it around keeps the
// per-database work from having to thread three slices through every call.
type discovery struct {
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// objectNames holds the qualified name of every table and view found so
	// far, keyed by its lowercased form. Foreign key and view reference edges
	// are only emitted when both ends are in here, because the server drops
	// edges pointing at an MRN that does not exist.
	objectNames map[string]string
}

// Discover discovers SQL Server databases, tables, views and routines.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// A SQL Server session is bound to one database, so discovery starts on
	// whichever database the config points at and reconnects per database.
	entryDatabase := masterDatabase
	if s.config.Database != "" {
		entryDatabase = s.config.Database
	}

	if err := s.connect(ctx, entryDatabase); err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", entryDatabase, err)
	}
	defer s.close()

	server, err := s.queryServerInfo(ctx)
	if err != nil {
		// An instance that will not report its version is still worth reading,
		// so this only costs the server_version and edition metadata.
		log.Warn().Err(err).Msg("Failed to read server properties")
	}

	databases, err := s.queryDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}

	databases = s.selectDatabases(databases)
	if len(databases) == 0 {
		log.Warn().Msg("No databases matched the configuration")
	}

	result := &discovery{objectNames: make(map[string]string)}

	for _, database := range databases {
		if err := s.discoverDatabase(ctx, database, server, result); err != nil {
			log.Warn().Err(err).Str("database", database.Name).Msg("Failed to discover database")
		}
	}

	log.Info().
		Int("assets", len(result.assets)).
		Int("lineages", len(result.lineage)).
		Int("statistics", len(result.statistics)).
		Msg("SQL Server discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     result.assets,
		Lineage:    result.lineage,
		Statistics: result.statistics,
	}, nil
}

// selectDatabases narrows the instance's databases to the ones to discover.
// A configured database wins over the exclusion list, so naming a system
// database explicitly still works.
func (s *Source) selectDatabases(databases []databaseInfo) []databaseInfo {
	if s.config.Database != "" {
		for _, database := range databases {
			if strings.EqualFold(database.Name, s.config.Database) {
				return []databaseInfo{database}
			}
		}
		log.Warn().Str("database", s.config.Database).Msg("Configured database was not found or is not accessible")
		return nil
	}

	excluded := lowercaseSet(s.config.ExcludeDatabases)

	var selected []databaseInfo
	for _, database := range databases {
		if excluded[strings.ToLower(database.Name)] {
			continue
		}
		selected = append(selected, database)
	}
	return selected
}

// discoverDatabase reads one database over its own connection and appends
// everything it finds to result.
func (s *Source) discoverDatabase(ctx context.Context, database databaseInfo, server serverInfo, result *discovery) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := s.connect(dbCtx, database.Name); err != nil {
		return fmt.Errorf("connecting: %w", err)
	}

	log.Debug().Str("database", database.Name).Msg("Starting database discovery")

	schemas, err := s.querySchemas(dbCtx)
	if err != nil {
		return fmt.Errorf("listing schemas: %w", err)
	}

	excludedSchemas := lowercaseSet(s.config.ExcludeSchemas)
	schemaComments := make(map[string]string)
	schemaCount := 0
	for _, schema := range schemas {
		if excludedSchemas[strings.ToLower(schema.Name)] {
			continue
		}
		schemaCount++
		if schema.Comment != "" {
			schemaComments[schema.Name] = schema.Comment
		}
	}

	objects, err := s.queryObjects(dbCtx)
	if err != nil {
		return fmt.Errorf("listing tables and views: %w", err)
	}

	viewDefinitions := map[string]string{}
	if s.config.IncludeViews {
		viewDefinitions, err = s.queryViewDefinitions(dbCtx)
		if err != nil {
			log.Warn().Err(err).Str("database", database.Name).Msg("Failed to read view definitions")
			viewDefinitions = map[string]string{}
		}
	}

	columnsByObject := map[string][]columnInfo{}
	primaryKeys := map[string]bool{}
	if s.config.IncludeColumns {
		columns, err := s.queryColumns(dbCtx)
		if err != nil {
			log.Warn().Err(err).Str("database", database.Name).Msg("Failed to read columns")
		} else {
			for _, column := range columns {
				key := column.Schema + "." + column.Object
				columnsByObject[key] = append(columnsByObject[key], column)
			}
		}

		primaryKeys, err = s.queryPrimaryKeyColumns(dbCtx)
		if err != nil {
			log.Warn().Err(err).Str("database", database.Name).Msg("Failed to read primary keys")
			primaryKeys = map[string]bool{}
		}
	}

	// The database asset is built after its objects are counted, so its
	// metadata can report how much it holds.
	var objectAssets []pluginsdk.Asset
	tableCount, viewCount := 0, 0

	for _, object := range objects {
		if excludedSchemas[strings.ToLower(object.Schema)] {
			continue
		}

		assetType := "Table"
		if object.TypeDesc == "VIEW" {
			if !s.config.IncludeViews {
				continue
			}
			assetType = "View"
			viewCount++
		} else {
			tableCount++
		}

		key := object.Schema + "." + object.Name
		asset := s.objectAsset(database, object, assetType,
			schemaComments[object.Schema],
			viewDefinitions[key],
			columnsByObject[key],
			primaryKeys)

		objectAssets = append(objectAssets, asset)
		result.objectNames[strings.ToLower(*asset.Name)] = *asset.Name
	}

	var routineAssets []pluginsdk.Asset
	if s.config.IncludeProcedures {
		routines, err := s.queryRoutines(dbCtx)
		if err != nil {
			log.Warn().Err(err).Str("database", database.Name).Msg("Failed to read stored procedures and functions")
		} else {
			for _, routine := range routines {
				if excludedSchemas[strings.ToLower(routine.Schema)] {
					continue
				}
				routineAssets = append(routineAssets, s.routineAsset(database, routine))
			}
		}
	}

	databaseAsset := s.databaseAsset(database, server, schemaCount, tableCount, viewCount)
	result.assets = append(result.assets, databaseAsset)
	result.assets = append(result.assets, objectAssets...)
	result.assets = append(result.assets, routineAssets...)

	for _, asset := range append(append([]pluginsdk.Asset{}, objectAssets...), routineAssets...) {
		result.lineage = append(result.lineage, pluginsdk.LineageEdge{
			Source: *databaseAsset.MRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
	}

	if s.config.DiscoverForeignKeys {
		result.lineage = append(result.lineage, s.foreignKeyEdges(dbCtx, database.Name, excludedSchemas, result.objectNames)...)
	}

	if s.config.IncludeViews {
		result.lineage = append(result.lineage, viewOfEdges(database.Name, objectAssets, viewDefinitions, result.objectNames)...)
	}

	if s.config.IncludeStatistics {
		result.statistics = append(result.statistics, s.tableStatistics(dbCtx, database.Name, objectAssets, columnsByObject)...)
	}

	return nil
}

func (s *Source) databaseAsset(database databaseInfo, server serverInfo, schemaCount, tableCount, viewCount int) pluginsdk.Asset {
	name := database.Name
	mrnValue := assetMRN("Database", name)

	metadata := map[string]any{
		"host":         s.config.Host,
		"port":         s.config.Port,
		"database":     name,
		"database_id":  database.DatabaseID,
		"created":      database.CreateDate.Format(time.RFC3339),
		"schema_count": schemaCount,
		"table_count":  tableCount,
		"view_count":   viewCount,
	}
	setIfNotEmpty(metadata, "collation", database.Collation)
	setIfNotEmpty(metadata, "state", database.State)
	setIfNotEmpty(metadata, "recovery_model", database.RecoveryModel)
	setIfNotEmpty(metadata, "owner", database.Owner)
	setIfNotEmpty(metadata, "server_version", server.ProductVersion)
	setIfNotEmpty(metadata, "edition", server.Edition)
	if server.EngineEdition != 0 {
		metadata["engine_edition"] = server.EngineEdition
	}

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Database",
		Providers: []string{providerName},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       providerName,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) objectAsset(database databaseInfo, object objectInfo, assetType, schemaComment, definition string, columns []columnInfo, primaryKeys map[string]bool) pluginsdk.Asset {
	name := qualifiedName(database.Name, object.Schema, object.Name)
	mrnValue := assetMRN(assetType, name)

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"database":    database.Name,
		"schema":      object.Schema,
		"table_name":  object.Name,
		"object_type": strings.ToLower(object.TypeDesc),
		"created":     object.CreateDate.Format(time.RFC3339),
		"modified":    object.ModifyDate.Format(time.RFC3339),
	}
	setIfNotEmpty(metadata, "comment", object.Comment)
	setIfNotEmpty(metadata, "schema_comment", schemaComment)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{providerName},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       providerName,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if object.Comment != "" {
		description := object.Comment
		asset.Description = &description
	}

	if definition != "" {
		query := definition
		language := "SQL"
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	if len(columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, buildColumns(object.Schema, object.Name, columns, primaryKeys)); err != nil {
			log.Warn().Err(err).Str("object", name).Msg("Failed to set columns")
		}
	}

	return asset
}

func (s *Source) routineAsset(database databaseInfo, routine routineInfo) pluginsdk.Asset {
	name := qualifiedName(database.Name, routine.Schema, routine.Name)
	mrnValue := assetMRN("Function", name)

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"database":    database.Name,
		"schema":      routine.Schema,
		"object_type": routineObjectType(routine.TypeDesc),
		"created":     routine.CreateDate.Format(time.RFC3339),
		"modified":    routine.ModifyDate.Format(time.RFC3339),
		"encrypted":   routine.IsEncrypted,
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Function",
		Providers: []string{providerName},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       providerName,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	// An encrypted module has no readable body, so there is no query to show.
	if routine.Definition != "" {
		query := routine.Definition
		language := "SQL"
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	return asset
}

// routineObjectType turns SQL Server's type_desc into the shorter name used in
// metadata, so a reader does not have to know the catalog's vocabulary.
func routineObjectType(typeDesc string) string {
	switch typeDesc {
	case "SQL_STORED_PROCEDURE":
		return "stored_procedure"
	case "SQL_SCALAR_FUNCTION":
		return "scalar_function"
	case "SQL_INLINE_TABLE_VALUED_FUNCTION":
		return "inline_table_function"
	case "SQL_TABLE_VALUED_FUNCTION":
		return "table_function"
	default:
		return strings.ToLower(typeDesc)
	}
}

// column is a pluginsdk.Column with the extras SQL Server reports that the
// shared shape has no place for.
type column struct {
	pluginsdk.Column
	IsIdentity         bool   `json:"is_identity,omitempty"`
	IdentitySeed       int64  `json:"identity_seed,omitempty"`
	IdentityIncrement  int64  `json:"identity_increment,omitempty"`
	IsComputed         bool   `json:"is_computed,omitempty"`
	IsPersisted        bool   `json:"is_persisted,omitempty"`
	ComputedDefinition string `json:"computed_definition,omitempty"`
	Collation          string `json:"collation,omitempty"`
}

func buildColumns(schemaName, objectName string, columns []columnInfo, primaryKeys map[string]bool) []column {
	built := make([]column, 0, len(columns))
	for _, info := range columns {
		c := column{
			Column: pluginsdk.Column{
				Name:       info.Name,
				DataType:   renderDataType(info.TypeName, info.MaxLength, info.Precision, info.Scale),
				Nullable:   info.Nullable,
				PrimaryKey: primaryKeys[schemaName+"."+objectName+"."+info.Name],
			},
			IsIdentity:         info.IsIdentity,
			IsComputed:         info.ComputedDefinition != "",
			IsPersisted:        info.IsPersisted,
			ComputedDefinition: info.ComputedDefinition,
			Collation:          info.Collation,
		}
		if info.Comment != "" {
			c.Description = info.Comment
		}
		if info.DefaultDefinition != "" {
			c.Default = info.DefaultDefinition
		}
		if info.IsIdentity {
			c.IdentitySeed = info.IdentitySeed
			c.IdentityIncrement = info.IdentityIncrement
		}
		built = append(built, c)
	}
	return built
}

// foreignKeyEdges turns the database's foreign keys into edges from the
// referencing table to the referenced one. Self references and references to
// objects this run did not discover are dropped, the latter because the server
// discards edges whose endpoint does not exist.
func (s *Source) foreignKeyEdges(ctx context.Context, databaseName string, excludedSchemas map[string]bool, objectNames map[string]string) []pluginsdk.LineageEdge {
	keys, err := s.queryForeignKeys(ctx)
	if err != nil {
		log.Warn().Err(err).Str("database", databaseName).Msg("Failed to discover foreign key relationships")
		return nil
	}

	var edges []pluginsdk.LineageEdge
	seen := make(map[string]bool)

	for _, key := range keys {
		if excludedSchemas[strings.ToLower(key.Schema)] || excludedSchemas[strings.ToLower(key.TargetSchema)] {
			continue
		}

		source, sourceFound := objectNames[strings.ToLower(qualifiedName(databaseName, key.Schema, key.Table))]
		target, targetFound := objectNames[strings.ToLower(qualifiedName(databaseName, key.TargetSchema, key.TargetTable))]
		if !sourceFound || !targetFound {
			continue
		}

		sourceMRN := assetMRN("Table", source)
		targetMRN := assetMRN("Table", target)
		if sourceMRN == targetMRN {
			continue
		}

		// A composite key produces one row per column pair; the edge between
		// two tables is the same one every time.
		edgeKey := sourceMRN + ":" + targetMRN
		if seen[edgeKey] {
			continue
		}
		seen[edgeKey] = true

		log.Debug().
			Str("source", source).
			Str("target", target).
			Str("constraint", key.Name).
			Str("on_delete", key.DeleteAction).
			Str("on_update", key.UpdateAction).
			Msg("Found foreign key relationship")

		edges = append(edges, pluginsdk.LineageEdge{
			Source: sourceMRN,
			Target: targetMRN,
			Type:   "FOREIGN_KEY",
		})
	}

	return edges
}

// viewOfEdges links each view to the objects its definition reads, pointing
// from the base object to the view. Only references that resolve to something
// discovered in this run become edges.
func viewOfEdges(databaseName string, objectAssets []pluginsdk.Asset, viewDefinitions map[string]string, objectNames map[string]string) []pluginsdk.LineageEdge {
	var edges []pluginsdk.LineageEdge
	seen := make(map[string]bool)

	for _, asset := range objectAssets {
		if asset.Type != "View" {
			continue
		}

		schemaName, _ := asset.Metadata["schema"].(string)
		viewName, _ := asset.Metadata["table_name"].(string)
		definition, ok := viewDefinitions[schemaName+"."+viewName]
		if !ok {
			continue
		}

		for _, reference := range extractViewReferences(definition, databaseName, schemaName) {
			base, found := objectNames[strings.ToLower(reference)]
			if !found {
				continue
			}
			if strings.EqualFold(base, *asset.Name) {
				continue
			}

			// The base object may be a table or another view, and only one of
			// the two MRNs exists.
			sourceMRN := assetMRN(baseAssetType(base, objectAssets), base)
			edgeKey := sourceMRN + ":" + *asset.MRN
			if seen[edgeKey] {
				continue
			}
			seen[edgeKey] = true

			log.Debug().
				Str("base", base).
				Str("view", *asset.Name).
				Msg("Found view reference")

			edges = append(edges, pluginsdk.LineageEdge{
				Source: sourceMRN,
				Target: *asset.MRN,
				Type:   "VIEW_OF",
			})
		}
	}

	return edges
}

// baseAssetType reports whether a discovered object is a Table or a View, so
// the edge is built with the MRN that object actually has.
func baseAssetType(name string, objectAssets []pluginsdk.Asset) string {
	for _, asset := range objectAssets {
		if asset.Name != nil && strings.EqualFold(*asset.Name, name) {
			return asset.Type
		}
	}
	return "Table"
}

// tableStatistics emits row counts and sizes for tables, and a column count
// for every object whose columns were read.
func (s *Source) tableStatistics(ctx context.Context, databaseName string, objectAssets []pluginsdk.Asset, columnsByObject map[string][]columnInfo) []pluginsdk.Statistic {
	byName := make(map[string]pluginsdk.Asset, len(objectAssets))
	for _, asset := range objectAssets {
		byName[strings.ToLower(*asset.Name)] = asset
	}

	var statistics []pluginsdk.Statistic

	for _, asset := range objectAssets {
		schemaName, _ := asset.Metadata["schema"].(string)
		objectName, _ := asset.Metadata["table_name"].(string)
		columns, ok := columnsByObject[schemaName+"."+objectName]
		if !ok {
			continue
		}
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.column_count",
			Value:      float64(len(columns)),
		})
	}

	stats, err := s.queryTableStats(ctx)
	if err != nil {
		log.Warn().Err(err).Str("database", databaseName).Msg("Failed to collect table statistics")
		return statistics
	}

	for _, stat := range stats {
		asset, ok := byName[strings.ToLower(qualifiedName(databaseName, stat.Schema, stat.Table))]
		if !ok {
			continue
		}
		statistics = append(statistics,
			pluginsdk.Statistic{
				AssetMRN:   *asset.MRN,
				MetricName: "asset.row_count",
				Value:      float64(stat.RowCount),
			},
			pluginsdk.Statistic{
				AssetMRN:   *asset.MRN,
				MetricName: "asset.size_bytes",
				Value:      float64(stat.SizeBytes),
			},
		)
	}

	return statistics
}

// FetchSampleData implements pluginsdk.DataFetcher, reading the first rows of
// a table or view for the asset preview.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, asset *pluginsdk.Asset) ([]string, [][]any, error) {
	if asset == nil || asset.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}

	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	database, _ := asset.Metadata["database"].(string)
	schemaName, _ := asset.Metadata["schema"].(string)
	objectName, _ := asset.Metadata["table_name"].(string)

	if database == "" {
		return nil, nil, fmt.Errorf("could not determine database from asset metadata")
	}
	if schemaName == "" {
		return nil, nil, fmt.Errorf("could not determine schema from asset metadata")
	}
	if objectName == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	if err := s.connect(fetchCtx, database); err != nil {
		return nil, nil, fmt.Errorf("connecting to database %s: %w", database, err)
	}
	defer s.close()

	// A table name cannot be a bound parameter, so it is bracket quoted.
	query := fmt.Sprintf("SELECT TOP 20 * FROM %s.%s.%s",
		quoteIdent(database), quoteIdent(schemaName), quoteIdent(objectName))

	log.Debug().
		Str("database", database).
		Str("schema", schemaName).
		Str("table", objectName).
		Msg("Fetching sample data")

	rows, err := s.db.QueryContext(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("reading column types: %w", err)
	}

	columnNames := make([]string, len(columnTypes))
	dbTypes := make([]string, len(columnTypes))
	for i, columnType := range columnTypes {
		columnNames[i] = columnType.Name()
		dbTypes[i] = columnType.DatabaseTypeName()
	}

	var dataRows [][]any
	for rows.Next() {
		values := make([]any, len(columnNames))
		pointers := make([]any, len(columnNames))
		for i := range values {
			pointers[i] = &values[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row, skipping")
			continue
		}

		converted := make([]any, len(values))
		for i, value := range values {
			converted[i] = convertValue(value, dbTypes[i])
		}
		dataRows = append(dataRows, converted)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	log.Debug().
		Int("columns", len(columnNames)).
		Int("rows", len(dataRows)).
		Msg("Successfully fetched sample data")

	return columnNames, dataRows, nil
}

// lowercaseSet turns a config exclusion list into a lookup. SQL Server object
// names are case insensitive under the usual collations, so comparisons are
// made on the lowercased name.
func lowercaseSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[strings.ToLower(value)] = true
	}
	return set
}

// setIfNotEmpty keeps blank values out of the metadata map, so the catalog
// never shows an empty field where the source simply had nothing.
func setIfNotEmpty(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}
