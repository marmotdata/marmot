// Package starrocks discovers databases, tables, views and materialized
// views from StarRocks clusters.
package starrocks

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const (
	provider       = "StarRocks"
	defaultCatalog = "default_catalog"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "starrocks",
		Name:        "StarRocks",
		Description: "Discover databases, tables, views and materialized views from StarRocks clusters",
		Icon:        "starrocks",
		Category:    "data-warehouse",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the StarRocks plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"Frontend (FE) hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"Frontend MySQL protocol port" default:"9030" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" validate:"required"`
	Password string `json:"password" description:"Password for authentication" sensitive:"true"`
	Catalog  string `json:"catalog" description:"Catalog to discover; an external catalog name switches discovery to it" default:"default_catalog"`
	TLS      string `json:"tls" description:"TLS mode (false, true, skip-verify, preferred)" default:"false" validate:"omitempty,oneof=false true skip-verify preferred"`

	Databases        []string `json:"databases" description:"Databases to discover; all when empty"`
	ExcludeDatabases []string `json:"exclude_databases" description:"Databases to skip" default:"[\"information_schema\",\"_statistics_\",\"sys\"]"`

	IncludeColumns           bool `json:"include_columns" description:"Whether to include column information" default:"true"`
	IncludeViews             bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeMaterializedViews bool `json:"include_materialized_views" description:"Whether to discover asynchronous materialized views" default:"true"`
	IncludeStatistics        bool `json:"include_statistics" description:"Whether to include row counts, sizes and column counts" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "starrocks-fe.internal"
port: 9030
user: "marmot_reader"
password: "${STARROCKS_PASSWORD}"
catalog: "default_catalog"
databases:
  - "shop"
  - "analytics"
include_columns: true
include_views: true
include_materialized_views: true
include_statistics: true
tags:
  - "starrocks"
`

// Source represents the StarRocks plugin.
type Source struct {
	config *Config
	db     *sql.DB
	conn   *sql.Conn
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	pluginsdk.ApplyDefaults(config, rawConfig)

	if config.Catalog == "" {
		config.Catalog = defaultCatalog
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

// discovery accumulates one run's output. Lineage between objects is
// resolved at the end, once every database has been listed, so a view
// in one database can point at a table in another.
type discovery struct {
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic

	// objects maps database -> bare object name -> MRN for everything
	// discovered this run. Edges are only emitted between entries.
	objects map[string]map[string]string
	edges   map[string]struct{}

	viewRefs    []viewReference
	foreignKeys []foreignKeyReference
}

// viewReference is a view (or materialized view) and the names its
// query reads, kept until the run can resolve them.
type viewReference struct {
	database string
	mrn      string
	refs     []string
}

// foreignKeyReference is a table and one table it declares a foreign
// key to.
type foreignKeyReference struct {
	database string
	mrn      string
	ref      string
}

func newDiscovery() *discovery {
	return &discovery{
		objects: make(map[string]map[string]string),
		edges:   make(map[string]struct{}),
	}
}

func (d *discovery) addObject(database, name, mrnValue string) {
	if d.objects[database] == nil {
		d.objects[database] = make(map[string]string)
	}
	d.objects[database][name] = mrnValue
}

func (d *discovery) addEdge(source, target, edgeType string) {
	if source == target {
		return
	}
	key := edgeType + ":" + source + ":" + target
	if _, exists := d.edges[key]; exists {
		return
	}
	d.edges[key] = struct{}{}
	d.lineage = append(d.lineage, pluginsdk.LineageEdge{Source: source, Target: target, Type: edgeType})
}

// Discover discovers StarRocks databases, tables, views and materialized
// views in the configured catalog.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.connect(ctx); err != nil {
		return nil, fmt.Errorf("connecting to StarRocks: %w", err)
	}
	defer s.close()

	catalogType, err := s.catalogType(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.useCatalog(ctx); err != nil {
		return nil, fmt.Errorf("switching to catalog %s: %w", s.config.Catalog, err)
	}

	version, err := s.serverVersion(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read the StarRocks version")
	}

	databases, err := s.listDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}
	databases = s.selectDatabases(databases)
	log.Debug().Strs("databases", databases).Str("catalog", s.config.Catalog).Msg("Starting discovery")

	d := newDiscovery()
	for _, database := range databases {
		s.discoverDatabase(ctx, d, database, catalogType, version)
	}
	s.resolveLineage(d)

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Int("statistics", len(d.statistics)).
		Msg("StarRocks discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     d.assets,
		Lineage:    d.lineage,
		Statistics: d.statistics,
	}, nil
}

// catalogType checks the configured catalog exists and returns its type.
// A missing catalog is a configuration error, not something to warn past.
func (s *Source) catalogType(ctx context.Context) (string, error) {
	catalogs, err := s.listCatalogs(ctx)
	if err != nil {
		return "", fmt.Errorf("listing catalogs: %w", err)
	}
	var names []string
	for _, c := range catalogs {
		if c.Name == s.config.Catalog {
			return c.Type, nil
		}
		names = append(names, c.Name)
	}
	return "", fmt.Errorf("catalog %q not found (available: %s)", s.config.Catalog, strings.Join(names, ", "))
}

// selectDatabases applies the databases and exclude_databases settings.
func (s *Source) selectDatabases(all []string) []string {
	excluded := make(map[string]struct{}, len(s.config.ExcludeDatabases))
	for _, name := range s.config.ExcludeDatabases {
		excluded[name] = struct{}{}
	}
	wanted := make(map[string]struct{}, len(s.config.Databases))
	for _, name := range s.config.Databases {
		wanted[name] = struct{}{}
	}

	var selected []string
	for _, name := range all {
		if _, skip := excluded[name]; skip {
			continue
		}
		if len(wanted) > 0 {
			if _, ok := wanted[name]; !ok {
				continue
			}
		}
		selected = append(selected, name)
	}
	return selected
}

// discoverDatabase emits the Database asset and every object in it. A
// database whose tables cannot be listed still gets its asset; each
// object that fails is logged and skipped.
func (s *Source) discoverDatabase(ctx context.Context, d *discovery, database, catalogType, version string) {
	dbAsset := s.databaseAsset(database, catalogType, version)
	dbIndex := len(d.assets)
	d.assets = append(d.assets, dbAsset)

	tables, err := s.listTables(ctx, database)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Msg("Failed to list tables")
		return
	}

	mvs := map[string]materializedViewInfo{}
	if s.config.IncludeMaterializedViews {
		mvs, err = s.listMaterializedViews(ctx, database)
		if err != nil {
			log.Warn().Err(err).Str("database", database).Msg("Failed to list materialized views")
			mvs = map[string]materializedViewInfo{}
		}
	}

	partitions, err := s.partitionCounts(ctx, database)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Msg("Failed to count partitions")
	}

	tableCount, viewCount := 0, 0
	for _, t := range tables {
		objectType := classifyObject(t.TableType, t.Engine)
		switch objectType {
		case "":
			log.Debug().Str("database", database).Str("name", t.Name).Str("table_type", t.TableType).Str("engine", t.Engine).Msg("Skipping object of unknown kind")
			continue
		case "view":
			if !s.config.IncludeViews {
				continue
			}
		case "materialized_view":
			if !s.config.IncludeMaterializedViews {
				continue
			}
		}

		asset := s.objectAsset(ctx, d, database, t, objectType, mvs[t.Name], partitions[t.Name])
		d.assets = append(d.assets, asset)
		d.addObject(database, t.Name, *asset.MRN)
		d.addEdge(*dbAsset.MRN, *asset.MRN, "CONTAINS")

		if asset.Type == "View" {
			viewCount++
		} else {
			tableCount++
		}
	}

	if fks, err := s.listForeignKeys(ctx, database); err != nil {
		log.Warn().Err(err).Str("database", database).Msg("Failed to list foreign keys")
	} else {
		for _, fk := range fks {
			source, ok := d.objects[database][fk.Table]
			if !ok {
				continue
			}
			ref := fk.ReferencedTable
			if fk.ReferencedSchema != "" {
				ref = fk.ReferencedSchema + "." + fk.ReferencedTable
			}
			d.foreignKeys = append(d.foreignKeys, foreignKeyReference{database: database, mrn: source, ref: ref})
		}
	}

	d.assets[dbIndex].Metadata["table_count"] = tableCount
	d.assets[dbIndex].Metadata["view_count"] = viewCount
	log.Debug().Str("database", database).Int("tables", tableCount).Int("views", viewCount).Msg("Discovered database")
}

// classifyObject maps an information_schema.tables row onto the kinds
// this plugin emits. StarRocks reports its own storage as engine
// "StarRocks" (older versions: "OLAP"), plain views with no engine,
// system views as MEMORY, and external tables by their remote engine.
func classifyObject(tableType, engine string) string {
	tt := strings.ToUpper(strings.TrimSpace(tableType))
	eng := strings.ToUpper(strings.TrimSpace(engine))
	isView := strings.Contains(tt, "VIEW")

	switch eng {
	case "STARROCKS", "OLAP":
		if isView {
			return "materialized_view"
		}
		return "table"
	case "", "MEMORY":
		if isView {
			return "view"
		}
		return "table"
	default:
		if isView {
			return "view"
		}
		return "external_table"
	}
}

// databaseAsset is the container the database's tables and views hang
// from. Its name is the bare database name, matching what an
// OpenMetadata import of a StarRocks service produces.
func (s *Source) databaseAsset(database, catalogType, version string) pluginsdk.Asset {
	name := database
	mrnValue := assetMRN("Database", name)

	metadata := map[string]any{
		"host":     s.config.Host,
		"port":     s.config.Port,
		"catalog":  s.config.Catalog,
		"database": database,
	}
	if catalogType != "" {
		metadata["catalog_type"] = catalogType
	}
	if version != "" {
		metadata["starrocks_version"] = version
	}

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

// objectAsset builds the asset for one table, view or materialized view
// and records the lineage it implies (view references, foreign keys)
// and its statistics on d.
func (s *Source) objectAsset(ctx context.Context, d *discovery, database string, t tableInfo, objectType string, mv materializedViewInfo, partitionCount int) pluginsdk.Asset {
	assetType := "Table"
	if objectType == "view" || objectType == "materialized_view" {
		assetType = "View"
	}

	name := objectName(database, t.Name)
	mrnValue := assetMRN(assetType, name)

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"catalog":     s.config.Catalog,
		"database":    database,
		"table_name":  t.Name,
		"object_type": objectType,
	}
	if t.Engine != "" {
		metadata["engine"] = t.Engine
	}
	if objectType == "external_table" {
		metadata["external_engine"] = t.Engine
	}
	if t.Comment != "" {
		metadata["comment"] = t.Comment
	}
	if t.Created.Valid {
		metadata["created"] = t.Created.Time.Format("2006-01-02 15:04:05")
	}
	if t.Updated.Valid {
		metadata["updated"] = t.Updated.Time.Format("2006-01-02 15:04:05")
	}

	ddl := s.objectDDL(ctx, database, t.Name, objectType)
	if ddl == "" && objectType == "materialized_view" {
		ddl = mv.Definition
	}
	if ddl != "" {
		metadata["ddl"] = redactDDL(ddl)
	}

	var parsed tableDDL
	if objectType != "view" {
		parsed = parseTableDDL(ddl)
		addTableLayout(metadata, parsed, partitionCount)
	}

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{provider},
		Metadata:  metadata,
		Schema:    make(map[string]string),
	}
	if t.Comment != "" {
		description := t.Comment
		asset.Description = &description
	}

	if assetType == "View" {
		query := viewQuery(ddl)
		if query != "" {
			language := "SQL"
			asset.Query = &query
			asset.QueryLanguage = &language
		}
		refs := queryReferences(query)
		if objectType == "materialized_view" {
			addMaterializedViewMetadata(metadata, mv, parsed)
			refs = append(refs, mv.BaseTables...)
		}
		d.viewRefs = append(d.viewRefs, viewReference{database: database, mrn: mrnValue, refs: refs})
	}

	for _, fk := range parsed.ForeignKeys {
		ref := fk.ReferencedTable
		if fk.ReferencedDB != "" {
			ref = fk.ReferencedDB + "." + ref
		}
		if fk.ReferencedCatalog != "" {
			ref = fk.ReferencedCatalog + "." + ref
		}
		d.foreignKeys = append(d.foreignKeys, foreignKeyReference{database: database, mrn: mrnValue, ref: ref})
	}

	columnCount := -1
	if s.config.IncludeColumns {
		columns, err := s.showColumns(ctx, database, t.Name)
		if err != nil {
			log.Warn().Err(err).Str("database", database).Str("object", t.Name).Msg("Failed to read columns")
		} else {
			if err := pluginsdk.SetColumns(&asset, buildColumns(columns, parsed)); err != nil {
				log.Warn().Err(err).Str("object", name).Msg("Failed to serialise columns")
			}
			columnCount = len(columns)
		}
	}

	if s.config.IncludeStatistics {
		d.statistics = append(d.statistics, objectStatistics(mrnValue, objectType, t, columnCount)...)
	}

	asset.Tags = pluginsdk.InterpolateTags(s.config.Tags, metadata)
	asset.Sources = []pluginsdk.AssetSource{{
		Name:       provider,
		LastSyncAt: time.Now(),
		Properties: metadata,
		Priority:   1,
	}}

	log.Debug().Str("database", database).Str("name", t.Name).Str("object_type", objectType).Msg("Found database object")
	return asset
}

// objectDDL fetches the CREATE statement with the SHOW CREATE variant
// that matches the object kind. A failure is logged; the asset is still
// emitted without the layout details the DDL would have supplied.
func (s *Source) objectDDL(ctx context.Context, database, name, objectType string) string {
	kind := "TABLE"
	switch objectType {
	case "view":
		kind = "VIEW"
	case "materialized_view":
		kind = "MATERIALIZED VIEW"
	}
	ddl, err := s.showCreate(ctx, kind, database, name)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Str("object", name).Msg("Failed to read DDL")
		return ""
	}
	return ddl
}

// addTableLayout copies the parsed DDL clauses into metadata, leaving
// out anything the DDL did not declare.
func addTableLayout(metadata map[string]any, ddl tableDDL, partitionCount int) {
	if ddl.KeyModel != "" {
		metadata["key_model"] = ddl.KeyModel
		metadata["key_columns"] = strings.Join(ddl.KeyColumns, ", ")
	}
	if ddl.PartitionType != "" {
		metadata["partition_type"] = ddl.PartitionType
		if len(ddl.PartitionColumns) > 0 {
			metadata["partition_columns"] = strings.Join(ddl.PartitionColumns, ", ")
		}
		if ddl.PartitionExpression != "" {
			metadata["partition_expression"] = ddl.PartitionExpression
		}
		if partitionCount > 0 {
			metadata["partition_count"] = partitionCount
		}
	}
	if ddl.Distribution != "" {
		metadata["distribution"] = ddl.Distribution
		if len(ddl.DistributionColumns) > 0 {
			metadata["distribution_columns"] = strings.Join(ddl.DistributionColumns, ", ")
		}
		if ddl.Buckets > 0 {
			metadata["buckets"] = ddl.Buckets
		}
	}
	if len(ddl.OrderBy) > 0 {
		metadata["order_by"] = strings.Join(ddl.OrderBy, ", ")
	}
	if v, err := strconv.Atoi(ddl.Properties["replication_num"]); err == nil {
		metadata["replication_num"] = v
	}
	if v := ddl.Properties["storage_volume"]; v != "" {
		metadata["storage_volume"] = v
	}
}

// addMaterializedViewMetadata records the refresh state of an
// asynchronous materialized view. StarRocks prints the literal "null"
// for a refresh that has not happened yet, which is not a state.
func addMaterializedViewMetadata(metadata map[string]any, mv materializedViewInfo, ddl tableDDL) {
	metadata["materialized"] = true
	if mv.RefreshType != "" {
		metadata["refresh_type"] = mv.RefreshType
	}
	if mv.IsActive != "" {
		metadata["is_active"] = strings.EqualFold(mv.IsActive, "true")
	}
	if ddl.PartitionType == "" && mv.PartitionType != "" && !strings.EqualFold(mv.PartitionType, "UNPARTITIONED") {
		metadata["partition_type"] = mv.PartitionType
	}
	if state := mv.LastRefreshState; state != "" && !strings.EqualFold(state, "null") {
		metadata["last_refresh_state"] = state
	}
	if mv.LastRefreshStart.Valid {
		metadata["last_refresh_start_time"] = mv.LastRefreshStart.Time.Format("2006-01-02 15:04:05")
	}
	if mv.TaskName != "" {
		metadata["task_name"] = mv.TaskName
	}
}

// column is the per-column shape serialised into an asset's schema. The
// SDK fields render in the schema view; the extras are what StarRocks
// adds on top of a plain SQL column.
type column struct {
	pluginsdk.Column
	IsKey           bool   `json:"is_key"`
	AggregationType string `json:"aggregation_type,omitempty"`
	IsAutoIncrement bool   `json:"is_auto_increment,omitempty"`
}

// aggregationTypes are the values SHOW FULL COLUMNS puts in Extra for the
// value columns of an AGGREGATE KEY table.
var aggregationTypes = map[string]struct{}{
	"SUM": {}, "MAX": {}, "MIN": {}, "REPLACE": {}, "REPLACE_IF_NOT_NULL": {},
	"HLL_UNION": {}, "BITMAP_UNION": {}, "PERCENTILE_UNION": {}, "AGG_STATE_UNION": {},
}

// buildColumns merges SHOW FULL COLUMNS with what the DDL knows about
// keys. A PRIMARY or UNIQUE key identifies a row, so those columns are
// primary keys; DUPLICATE and AGGREGATE keys only order the data, so
// those columns are sorting keys.
func buildColumns(columns []columnInfo, ddl tableDDL) []column {
	keyColumns := make(map[string]struct{}, len(ddl.KeyColumns))
	for _, c := range ddl.KeyColumns {
		keyColumns[c] = struct{}{}
	}
	orderBy := make(map[string]struct{}, len(ddl.OrderBy))
	for _, c := range ddl.OrderBy {
		orderBy[c] = struct{}{}
	}
	autoIncrement := make(map[string]struct{}, len(ddl.AutoIncrementColumns))
	for _, c := range ddl.AutoIncrementColumns {
		autoIncrement[c] = struct{}{}
	}
	uniqueModel := ddl.KeyModel == "PRIMARY" || ddl.KeyModel == "UNIQUE"

	out := make([]column, 0, len(columns))
	for _, c := range columns {
		_, inKey := keyColumns[c.Field]
		_, inOrderBy := orderBy[c.Field]
		_, isAutoIncrement := autoIncrement[c.Field]

		col := column{
			Column: pluginsdk.Column{
				Name:        c.Field,
				DataType:    c.Type,
				Nullable:    strings.EqualFold(c.Null, "YES"),
				PrimaryKey:  inKey && uniqueModel,
				SortingKey:  (inKey && !uniqueModel) || inOrderBy,
				Description: c.Comment,
			},
			IsKey:           strings.EqualFold(c.Key, "YES") || strings.EqualFold(c.Key, "PRI"),
			IsAutoIncrement: isAutoIncrement || strings.Contains(strings.ToUpper(c.Extra), "AUTO_INCREMENT"),
		}
		if c.Default.Valid {
			col.Default = c.Default.String
		}
		if _, ok := aggregationTypes[strings.ToUpper(c.Extra)]; ok {
			col.AggregationType = strings.ToUpper(c.Extra)
		}
		out = append(out, col)
	}
	return out
}

// objectStatistics reports what information_schema.tables knows. Row and
// size figures come from backend tablet reports and only exist for
// StarRocks-managed storage; plain views and external tables have none.
func objectStatistics(assetMRN, objectType string, t tableInfo, columnCount int) []pluginsdk.Statistic {
	var stats []pluginsdk.Statistic
	if columnCount >= 0 {
		stats = append(stats, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: "asset.column_count", Value: float64(columnCount)})
	}
	if objectType != "table" && objectType != "materialized_view" {
		return stats
	}
	if t.Rows.Valid {
		stats = append(stats, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: "asset.row_count", Value: float64(t.Rows.Int64)})
	}
	if t.DataLength.Valid {
		stats = append(stats, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: "asset.size_bytes", Value: float64(t.DataLength.Int64)})
	}
	return stats
}

// resolveLineage turns the recorded view references and foreign keys
// into edges, keeping only those whose other end was discovered.
func (s *Source) resolveLineage(d *discovery) {
	for _, v := range d.viewRefs {
		for _, ref := range v.refs {
			target, ok := s.lookupObject(d, v.database, ref)
			if !ok {
				log.Debug().Str("view", v.mrn).Str("reference", ref).Msg("Skipping VIEW_OF edge, referenced object not discovered")
				continue
			}
			d.addEdge(target, v.mrn, "VIEW_OF")
		}
	}
	for _, fk := range d.foreignKeys {
		target, ok := s.lookupObject(d, fk.database, fk.ref)
		if !ok {
			log.Debug().Str("table", fk.mrn).Str("reference", fk.ref).Msg("Skipping FOREIGN_KEY edge, referenced table not discovered")
			continue
		}
		d.addEdge(fk.mrn, target, "FOREIGN_KEY")
	}
}

func (s *Source) lookupObject(d *discovery, currentDB, ref string) (string, bool) {
	database, object, ok := resolveReference(ref, s.config.Catalog, currentDB)
	if !ok {
		return "", false
	}
	mrnValue, found := d.objects[database][object]
	return mrnValue, found
}

// FetchSampleData implements pluginsdk.DataFetcher: the first rows of a
// table or view, read through the same catalog discovery used.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}
	if _, err := s.Validate(config); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	database, _ := a.Metadata["database"].(string)
	table, _ := a.Metadata["table_name"].(string)
	if database == "" || table == "" {
		return nil, nil, fmt.Errorf("asset metadata has no database and table_name")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	if err := s.connect(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("connecting to StarRocks: %w", err)
	}
	defer s.close()

	if err := s.useCatalog(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("switching to catalog %s: %w", s.config.Catalog, err)
	}

	//nolint:gosec // G201: identifiers are quoted by quoteIdentifier
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 20", quoteIdentifier(database), quoteIdentifier(table))

	rows, err := s.conn.QueryContext(fetchCtx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying %s.%s: %w", database, table, err)
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("reading column names: %w", err)
	}

	var data [][]any
	for rows.Next() {
		values := make([]any, len(columnNames))
		ptrs := make([]any, len(columnNames))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.Warn().Err(err).Msg("Failed to scan sample row, skipping")
			continue
		}
		for i, v := range values {
			values[i] = convertValue(v)
		}
		data = append(data, values)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	return columnNames, data, nil
}

// convertValue makes driver values JSON-friendly: text comes back as
// bytes, and binary data is shown as hex.
func convertValue(v any) any {
	switch val := v.(type) {
	case []byte:
		if utf8.Valid(val) {
			return string(val)
		}
		return fmt.Sprintf("0x%x", val)
	case time.Time:
		return val.Format(time.RFC3339)
	default:
		return v
	}
}

// objectName is the Name of a table or view: database.object, the same
// shape an OpenMetadata import of a StarRocks service produces, so both
// land on one asset.
func objectName(database, object string) string {
	return database + "." + object
}

// assetMRN is the single place a StarRocks MRN is built. Assets, CONTAINS
// edges, view lineage and foreign keys all go through it so they can
// never address the same object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
