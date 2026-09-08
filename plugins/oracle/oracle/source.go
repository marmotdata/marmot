// Package oracle discovers schemas, tables, views, materialized views and
// stored procedures from Oracle databases.
package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// provider is the exact provider string shared with the OpenMetadata
// projection and the Trino connector map, so an Oracle table catalogued by
// any of the three lands on one asset.
const provider = "Oracle"

// Config for the Oracle plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host        string `json:"host" description:"Oracle listener hostname or IP address" validate:"required"`
	Port        int    `json:"port" description:"Oracle listener port" default:"1521" validate:"omitempty,min=1,max=65535"`
	User        string `json:"user" description:"Username for authentication" validate:"required"`
	Password    string `json:"password" description:"Password for authentication" sensitive:"true" validate:"required"`
	ServiceName string `json:"service_name" description:"Service name to connect to, for example FREEPDB1 (required unless sid is set)"`
	SID         string `json:"sid" label:"SID" description:"System identifier to connect to instead of a service name"`

	Schemas        []string `json:"schemas" description:"Schemas to discover (every schema not maintained by Oracle when empty)"`
	ExcludeSchemas []string `json:"exclude_schemas" description:"Schemas to skip" default:"[\"SYS\",\"SYSTEM\",\"CTXSYS\",\"DBSNMP\",\"OUTLN\",\"XDB\",\"MDSYS\",\"ORDSYS\",\"OLAPSYS\",\"WMSYS\",\"APEX_PUBLIC_USER\",\"AUDSYS\",\"DVSYS\",\"GSMADMIN_INTERNAL\",\"LBACSYS\",\"OJVMSYS\",\"DBSFWUSER\",\"GGSYS\",\"APPQOSSYS\",\"REMOTE_SCHEDULER_AGENT\",\"SYS$UMF\",\"DIP\",\"ORACLE_OCM\",\"ANONYMOUS\",\"XS$NULL\",\"FLOWS_FILES\"]"`
	UseDBAViews    bool     `json:"use_dba_views" label:"Use DBA Views" description:"Read DBA_* dictionary views instead of ALL_* (needs SELECT ANY DICTIONARY)" default:"false"`

	IncludeColumns           bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews             bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeMaterializedViews bool `json:"include_materialized_views" description:"Whether to discover materialized views" default:"true"`
	IncludeProcedures        bool `json:"include_procedures" description:"Whether to discover procedures, functions and packages" default:"true"`
	DiscoverForeignKeys      bool `json:"discover_foreign_keys" description:"Whether to discover foreign key relationships" default:"true"`
	IncludeStatistics        bool `json:"include_statistics" description:"Whether to include row counts and sizes from optimizer statistics" default:"true"`

	SSL        bool   `json:"ssl" label:"SSL" description:"Connect over TCPS (TLS)" default:"false"`
	SSLVerify  bool   `json:"ssl_verify" label:"SSL Verify" description:"Verify the server certificate when ssl is enabled" default:"true"`
	WalletPath string `json:"wallet_path" description:"Path to an Oracle wallet directory (TCPS certificates or wallet authentication)"`
}

// Example configuration for the plugin
var _ = `
host: "oracle-prod.internal"
port: 1521
user: "marmot_reader"
password: "secure_password"
service_name: "ORCLPDB1"
schemas:
  - "HR"
  - "SALES"
tags:
  - "oracle"
  - "production"
`

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "oracle",
		Name:        "Oracle Database",
		Description: "Discover schemas, tables, views, materialized views and stored procedures from Oracle databases",
		Icon:        "oracle",
		Category:    "database",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Source represents the Oracle plugin.
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

	config.ServiceName = strings.TrimSpace(config.ServiceName)
	config.SID = strings.TrimSpace(config.SID)
	if config.ServiceName == "" && config.SID == "" {
		return nil, fmt.Errorf("one of service_name or sid is required")
	}
	if config.ServiceName != "" && config.SID != "" {
		return nil, fmt.Errorf("service_name and sid are mutually exclusive, set only one")
	}

	config.Schemas = normaliseSchemaNames(config.Schemas)
	config.ExcludeSchemas = normaliseSchemaNames(config.ExcludeSchemas)

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Oracle schemas and the objects they own.
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

	info := s.serverInfo(ctx)

	schemas, err := s.listSchemas(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing schemas: %w", err)
	}
	log.Debug().Int("count", len(schemas)).Msg("Selected schemas for discovery")

	d := &discovery{
		source:  s,
		info:    info,
		objects: make(map[objectRef]string),
		edges:   make(map[string]struct{}),
	}

	for _, schema := range schemas {
		if err := d.discoverSchema(ctx, schema); err != nil {
			log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to discover schema")
		}
	}

	if s.config.DiscoverForeignKeys {
		d.addForeignKeyLineage()
	}
	d.addViewLineage()

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Int("statistics", len(d.statistics)).
		Msg("Oracle discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     d.assets,
		Lineage:    d.lineage,
		Statistics: d.statistics,
	}, nil
}

// discovery accumulates one run's output. Foreign keys and view references
// can point across schemas, so they are resolved only after every schema has
// been read and the objects index is complete.
type discovery struct {
	source *Source
	info   serverInfo

	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// objects maps every table, view and materialized view emitted in this
	// run to its MRN, so an edge is only ever built to an asset that exists.
	objects map[objectRef]string
	// texts holds the SQL behind every view and materialized view, resolved
	// into VIEW_OF edges at the end.
	texts []viewText
	// constraints from every schema, resolved into FOREIGN_KEY edges at the end.
	constraints []constraintRow
	// edges deduplicates lineage by source, target and type.
	edges map[string]struct{}
}

type viewText struct {
	ref  objectRef
	text string
}

func (d *discovery) discoverSchema(ctx context.Context, schema schemaRow) error {
	s := d.source

	allTables, err := s.listTables(ctx, schema.Name)
	if err != nil {
		return fmt.Errorf("listing tables: %w", err)
	}

	// A materialized view is backed by a table of the same name that shows
	// up in ALL_TABLES. The list of materialized views is always read so the
	// backing table is never catalogued as a plain table, even when the
	// materialized views themselves are not wanted.
	mviews, err := s.listMaterializedViews(ctx, schema.Name)
	if err != nil {
		log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to list materialized views")
	}
	mviewNames := make(map[string]struct{}, len(mviews))
	for _, mv := range mviews {
		mviewNames[mv.Name] = struct{}{}
	}
	if !s.config.IncludeMaterializedViews {
		mviews = nil
	}

	var tables []tableRow
	mviewStorage := make(map[string]tableRow)
	for _, t := range allTables {
		if _, isMView := mviewNames[t.Name]; isMView {
			mviewStorage[t.Name] = t
			continue
		}
		tables = append(tables, t)
	}

	var views []viewRow
	if s.config.IncludeViews {
		views, err = s.listViews(ctx, schema.Name)
		if err != nil {
			log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to list views")
		}
	}

	if len(tables)+len(views)+len(mviews) == 0 {
		log.Debug().Str("schema", schema.Name).Msg("Schema owns no tables or views, skipping")
		return nil
	}

	tableComments, columnComments, err := s.listComments(ctx, schema.Name)
	if err != nil {
		log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to read comments")
	}

	var constraints []constraintRow
	if s.config.IncludeColumns || s.config.DiscoverForeignKeys {
		constraints, err = s.listConstraints(ctx, schema.Name)
		if err != nil {
			log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to list constraints")
		}
		d.constraints = append(d.constraints, constraints...)
	}
	primaryKeys := primaryKeyColumns(constraints)

	columns := make(map[string][]columnRow)
	if s.config.IncludeColumns {
		columns, err = s.listColumns(ctx, schema.Name)
		if err != nil {
			log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to list columns")
		}
	}

	dbAsset := d.databaseAsset(schema, len(tables), len(views), len(mviews))
	d.assets = append(d.assets, dbAsset)

	for _, t := range tables {
		metadata := d.tableMetadata(t, "table", tableComments[t.Name])
		asset := d.objectAsset("Table", t.Owner, t.Name, metadata, tableComments[t.Name])
		d.setColumns(&asset, t.Name, columns[t.Name], columnComments[t.Name], primaryKeys[t.Name])
		d.addObject(asset, *dbAsset.MRN)
		d.addStatistics(asset, t, len(columns[t.Name]))
	}

	for _, v := range views {
		metadata := map[string]any{
			"host":        s.config.Host,
			"port":        s.config.Port,
			"schema":      v.Owner,
			"table_name":  v.Name,
			"object_type": "view",
		}
		if v.TextLength > 0 {
			metadata["text_length"] = v.TextLength
		}
		if comment := tableComments[v.Name]; comment != "" {
			metadata["comment"] = comment
		}
		asset := d.objectAsset("View", v.Owner, v.Name, metadata, tableComments[v.Name])
		setQuery(&asset, v.Text)
		d.setColumns(&asset, v.Name, columns[v.Name], columnComments[v.Name], nil)
		d.addObject(asset, *dbAsset.MRN)
		d.addColumnCount(asset, len(columns[v.Name]))
		d.texts = append(d.texts, viewText{ref: objectRef{Owner: v.Owner, Name: v.Name}, text: v.Text})
	}

	for _, mv := range mviews {
		storage, hasStorage := mviewStorage[mv.Name]
		metadata := map[string]any{
			"host":         s.config.Host,
			"port":         s.config.Port,
			"schema":       mv.Owner,
			"table_name":   mv.Name,
			"object_type":  "materialized_view",
			"materialized": true,
		}
		setIfValid(metadata, "refresh_mode", mv.RefreshMode)
		setIfValid(metadata, "refresh_method", mv.RefreshMethod)
		setIfValid(metadata, "build_mode", mv.BuildMode)
		setIfValid(metadata, "staleness", mv.Staleness)
		if mv.LastRefresh.Valid {
			metadata["last_refresh"] = mv.LastRefresh.Time.Format(time.RFC3339)
		}
		if hasStorage {
			setIfValid(metadata, "tablespace", storage.Tablespace)
			if storage.NumRows.Valid {
				metadata["num_rows"] = storage.NumRows.Int64
			}
			if storage.LastAnalyzed.Valid {
				metadata["last_analyzed"] = storage.LastAnalyzed.Time.Format(time.RFC3339)
			}
		}
		if comment := tableComments[mv.Name]; comment != "" {
			metadata["comment"] = comment
		}
		asset := d.objectAsset("View", mv.Owner, mv.Name, metadata, tableComments[mv.Name])
		setQuery(&asset, mv.Query)
		d.setColumns(&asset, mv.Name, columns[mv.Name], columnComments[mv.Name], nil)
		d.addObject(asset, *dbAsset.MRN)
		if hasStorage {
			d.addStatistics(asset, storage, len(columns[mv.Name]))
		} else {
			d.addColumnCount(asset, len(columns[mv.Name]))
		}
		d.texts = append(d.texts, viewText{ref: objectRef{Owner: mv.Owner, Name: mv.Name}, text: mv.Query})
	}

	if s.config.IncludeProcedures {
		procedures, err := s.listProcedures(ctx, schema.Name)
		if err != nil {
			log.Warn().Err(err).Str("schema", schema.Name).Msg("Failed to list procedures")
		}
		for _, p := range procedures {
			asset := d.procedureAsset(p)
			d.assets = append(d.assets, asset)
			d.addEdge(*dbAsset.MRN, *asset.MRN, "CONTAINS")
		}
	}

	log.Debug().
		Str("schema", schema.Name).
		Int("tables", len(tables)).
		Int("views", len(views)).
		Int("materialized_views", len(mviews)).
		Msg("Discovered schema")

	return nil
}

// databaseAsset is the container the schema's objects hang from. Oracle has
// no database level below the instance that users address objects by: a
// schema is what an Oracle user calls the database, and it is the level the
// OpenMetadata projection names the Database asset after too.
func (d *discovery) databaseAsset(schema schemaRow, tableCount, viewCount, mviewCount int) pluginsdk.Asset {
	s := d.source
	name := schema.Name

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"schema":      name,
		"table_count": tableCount,
		"view_count":  viewCount,
	}
	if mviewCount > 0 {
		metadata["materialized_view_count"] = mviewCount
	}
	if s.config.ServiceName != "" {
		metadata["service_name"] = s.config.ServiceName
	} else if d.info.ServiceName != "" {
		metadata["service_name"] = d.info.ServiceName
	}
	if s.config.SID != "" {
		metadata["sid"] = s.config.SID
	}
	if d.info.DBName != "" {
		metadata["db_name"] = d.info.DBName
	}
	if d.info.Container != "" {
		metadata["container"] = d.info.Container
	}
	if d.info.Version != "" {
		metadata["oracle_version"] = d.info.Version
	}
	if schema.Created.Valid {
		metadata["created"] = schema.Created.Time.Format(time.RFC3339)
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

func (d *discovery) tableMetadata(t tableRow, objectType, comment string) map[string]any {
	s := d.source
	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"schema":      t.Owner,
		"table_name":  t.Name,
		"object_type": objectType,
		"partitioned": t.Partitioned.String == "YES",
		"temporary":   t.Temporary.String == "Y",
		"iot":         t.IOTType.Valid,
	}
	setIfValid(metadata, "tablespace", t.Tablespace)
	setIfValid(metadata, "compression", t.Compression)
	if t.NumRows.Valid {
		metadata["num_rows"] = t.NumRows.Int64
	}
	if t.LastAnalyzed.Valid {
		metadata["last_analyzed"] = t.LastAnalyzed.Time.Format(time.RFC3339)
	}
	if comment != "" {
		metadata["comment"] = comment
	}
	return metadata
}

// objectAsset builds a Table or View asset. The name carries the schema so
// two schemas owning a table of the same name stay two assets, and so the
// identity agrees with the OpenMetadata projection and the Trino plugin.
func (d *discovery) objectAsset(assetType, owner, object string, metadata map[string]any, comment string) pluginsdk.Asset {
	s := d.source
	name := objectName(owner, object)
	mrnValue := assetMRN(assetType, name)

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
	if comment != "" {
		asset.Description = &comment
	}
	return asset
}

func (d *discovery) procedureAsset(p procedureRow) pluginsdk.Asset {
	s := d.source
	name := objectName(p.Owner, p.Name)
	mrnValue := assetMRN("Function", name)

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"schema":      p.Owner,
		"object_name": p.Name,
		"object_type": strings.ToLower(p.Type),
		"status":      p.Status,
	}
	if p.Created.Valid {
		metadata["created"] = p.Created.Time.Format(time.RFC3339)
	}
	if p.LastDDL.Valid {
		metadata["last_ddl_time"] = p.LastDDL.Time.Format(time.RFC3339)
	}

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Function",
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

// column is the per-column shape serialised into an asset's Schema.
type column struct {
	pluginsdk.Column
	IsVirtual  bool `json:"is_virtual,omitempty"`
	IsIdentity bool `json:"is_identity,omitempty"`
}

func (d *discovery) setColumns(asset *pluginsdk.Asset, table string, rows []columnRow, comments map[string]string, primaryKey map[string]bool) {
	if len(rows) == 0 {
		return
	}
	cols := make([]column, 0, len(rows))
	for _, r := range rows {
		c := column{
			Column: pluginsdk.Column{
				Name:        r.Name,
				DataType:    renderDataType(r),
				Nullable:    r.Nullable == "Y",
				PrimaryKey:  primaryKey[r.Name],
				Description: comments[r.Name],
			},
			IsVirtual:  r.Virtual == "YES",
			IsIdentity: r.Identity == "YES",
		}
		if def := strings.TrimSpace(r.Default.String); r.Default.Valid && def != "" {
			c.Default = def
		}
		cols = append(cols, c)
	}
	if err := pluginsdk.SetColumns(asset, cols); err != nil {
		log.Warn().Err(err).Str("table", table).Msg("Failed to set columns")
	}
}

func setQuery(asset *pluginsdk.Asset, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	language := "SQL"
	asset.Query = &text
	asset.QueryLanguage = &language
}

func setIfValid(metadata map[string]any, key string, value sql.NullString) {
	if value.Valid && value.String != "" {
		metadata[key] = value.String
	}
}

// addObject records a table or view asset, its CONTAINS edge from the schema
// and its place in the objects index used to resolve lineage later.
func (d *discovery) addObject(asset pluginsdk.Asset, databaseMRN string) {
	d.assets = append(d.assets, asset)
	d.addEdge(databaseMRN, *asset.MRN, "CONTAINS")

	owner, _ := asset.Metadata["schema"].(string)
	object, _ := asset.Metadata["table_name"].(string)
	d.objects[objectRef{Owner: owner, Name: object}] = *asset.MRN
}

func (d *discovery) addEdge(source, target, edgeType string) {
	if source == target {
		return
	}
	key := source + "|" + target + "|" + edgeType
	if _, exists := d.edges[key]; exists {
		return
	}
	d.edges[key] = struct{}{}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// addStatistics reports what the optimizer statistics know about a table.
// NUM_ROWS and BLOCKS are null until DBMS_STATS has run on the table, so a
// table that was never analyzed reports only its column count.
func (d *discovery) addStatistics(asset pluginsdk.Asset, t tableRow, columnCount int) {
	if !d.source.config.IncludeStatistics {
		return
	}
	if t.NumRows.Valid {
		d.statistics = append(d.statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.row_count",
			Value:      float64(t.NumRows.Int64),
		})
	}
	if t.Blocks.Valid {
		d.statistics = append(d.statistics, pluginsdk.Statistic{
			AssetMRN:   *asset.MRN,
			MetricName: "asset.size_bytes",
			Value:      float64(t.Blocks.Int64 * d.info.BlockSize),
		})
	}
	d.addColumnCount(asset, columnCount)
}

func (d *discovery) addColumnCount(asset pluginsdk.Asset, columnCount int) {
	if !d.source.config.IncludeStatistics || columnCount == 0 {
		return
	}
	d.statistics = append(d.statistics, pluginsdk.Statistic{
		AssetMRN:   *asset.MRN,
		MetricName: "asset.column_count",
		Value:      float64(columnCount),
	})
}

// addForeignKeyLineage turns every resolved foreign key into an edge from the
// referencing table to the referenced one. Both ends must have been
// discovered in this run; a reference into a schema that was not read is
// dropped rather than pointed at an asset that never gets created.
func (d *discovery) addForeignKeyLineage() {
	for _, fk := range resolveForeignKeys(d.constraints) {
		sourceMRN, ok := d.objects[objectRef{Owner: fk.SourceOwner, Name: fk.SourceTable}]
		if !ok {
			continue
		}
		targetMRN, ok := d.objects[objectRef{Owner: fk.TargetOwner, Name: fk.TargetTable}]
		if !ok {
			log.Debug().
				Str("constraint", fk.Constraint).
				Str("references", objectName(fk.TargetOwner, fk.TargetTable)).
				Msg("Skipping foreign key to a table outside the discovered schemas")
			continue
		}
		log.Debug().
			Str("source", objectName(fk.SourceOwner, fk.SourceTable)).
			Str("target", objectName(fk.TargetOwner, fk.TargetTable)).
			Str("constraint", fk.Constraint).
			Msg("Found foreign key relationship")
		d.addEdge(sourceMRN, targetMRN, "FOREIGN_KEY")
	}
}

// addViewLineage links every view and materialized view to the objects its
// SQL reads from. Edges run in the direction the data flows, base object to
// view, the way the MongoDB plugin draws them. Oracle stores view text with
// references either unqualified or as OWNER.OBJECT, so an unqualified name
// is looked up in the view's own schema.
func (d *discovery) addViewLineage() {
	for _, v := range d.texts {
		viewMRN, ok := d.objects[v.ref]
		if !ok {
			continue
		}
		for _, ref := range extractViewReferences(v.text) {
			if ref.Owner == "" {
				ref.Owner = v.ref.Owner
			}
			baseMRN, ok := d.objects[ref]
			if !ok {
				continue
			}
			d.addEdge(baseMRN, viewMRN, "VIEW_OF")
		}
	}
}

// FetchSampleData implements the DataFetcher interface to retrieve sample
// data from an Oracle table or view.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]interface{}, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	if _, err := s.Validate(config); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	schema, _ := a.Metadata["schema"].(string)
	table, _ := a.Metadata["table_name"].(string)
	if (schema == "" || table == "") && a.Name != nil {
		schema, table = splitObjectName(*a.Name)
	}
	if schema == "" {
		return nil, nil, fmt.Errorf("could not determine schema from asset metadata")
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset metadata")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := s.initConnection(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("connecting to database: %w", err)
	}
	defer s.closeConnection()

	log.Debug().Str("schema", schema).Str("table", table).Msg("Fetching sample data")

	target := quoteIdentifier(schema) + "." + quoteIdentifier(table)
	rows, err := s.db.QueryContext(fetchCtx, "SELECT * FROM "+target+" FETCH FIRST 20 ROWS ONLY")
	if err != nil && isSyntaxError(err) {
		// FETCH FIRST arrived in 12c; older releases only know ROWNUM.
		rows, err = s.db.QueryContext(fetchCtx, "SELECT * FROM "+target+" WHERE ROWNUM <= 20")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("getting column names: %w", err)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("getting column types: %w", err)
	}
	typeNames := make([]string, len(columnTypes))
	for i, ct := range columnTypes {
		typeNames[i] = ct.DatabaseTypeName()
	}

	var dataRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range columnNames {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan row, skipping")
			continue
		}

		converted := make([]interface{}, len(values))
		for i, val := range values {
			converted[i] = convertValue(val, typeNames[i])
		}
		dataRows = append(dataRows, converted)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	log.Debug().Int("columns", len(columnNames)).Int("rows", len(dataRows)).Msg("Successfully fetched sample data")
	return columnNames, dataRows, nil
}

// assetMRN is the single place an Oracle MRN is built. Every asset and every
// lineage edge goes through it so the two can never drift into addressing
// the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}

// objectName qualifies an object by its schema, the same shape the Trino
// plugin's connector map and the OpenMetadata projection use for Oracle.
func objectName(schema, object string) string {
	return schema + "." + object
}

// splitObjectName undoes objectName for assets whose metadata lacks the
// parts. Oracle identifiers cannot contain a dot unless quoted, so the first
// dot is the boundary.
func splitObjectName(name string) (schema, object string) {
	schema, object, found := strings.Cut(name, ".")
	if !found {
		return "", name
	}
	return schema, object
}
