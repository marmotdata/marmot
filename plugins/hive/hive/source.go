// Package hive discovers databases, tables and views from Apache Hive
// through HiveServer2.
package hive

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

const provider = "Hive"

// maxPartitionValues caps how many partition specs land in metadata; the
// count is always recorded.
const maxPartitionValues = 20

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "hive",
		Name:        "Apache Hive",
		Description: "Discover databases, tables and views from Apache Hive through HiveServer2",
		Icon:        "hive",
		Category:    "data-warehouse",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(HiveDatabaseFields{}, "Database",
				"The metadata fields the Hive plugin emits for database assets."),
			pluginsdk.AssetSchemaOf(HiveTableFields{}, "Table",
				"The metadata fields the Hive plugin emits for table and view assets."),
			pluginsdk.AssetSchemaOf(HiveColumnFields{}, "Column",
				"The per-column fields embedded in an asset's schema."),
		},
	}
}

// Config for the Hive plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host                string `json:"host" description:"HiveServer2 hostname or IP address" validate:"required"`
	Port                int    `json:"port" description:"HiveServer2 port" default:"10000" validate:"omitempty,min=1,max=65535"`
	Auth                string `json:"auth" description:"Authentication mechanism (NONE, NOSASL, LDAP, KERBEROS)" default:"NONE" validate:"omitempty,oneof=NONE NOSASL LDAP KERBEROS"`
	Username            string `json:"username" description:"Username (hive when empty)" validate:"required_if=Auth LDAP"`
	Password            string `json:"password" description:"Password, required for LDAP" sensitive:"true" validate:"required_if=Auth LDAP"`
	KerberosServiceName string `json:"kerberos_service_name" label:"Kerberos Service Name" description:"Service part of the HiveServer2 Kerberos principal" default:"hive"`
	Transport           string `json:"transport" description:"Thrift transport (binary or http)" default:"binary" validate:"omitempty,oneof=binary http"`
	HTTPPath            string `json:"http_path" label:"HTTP Path" description:"Endpoint path for the http transport" default:"cliservice"`
	SSL                 bool   `json:"ssl" label:"SSL" description:"Connect over TLS" default:"false"`
	SSLSkipVerify       bool   `json:"ssl_skip_verify" label:"SSL Skip Verify" description:"Skip TLS certificate verification" default:"false"`

	Databases        []string `json:"databases" description:"Databases to discover (all when empty)"`
	ExcludeDatabases []string `json:"exclude_databases" description:"Databases to skip" default:"[\"sys\",\"information_schema\"]"`

	IncludeColumns    bool `json:"include_columns" description:"Include column information" default:"true"`
	IncludeViews      bool `json:"include_views" description:"Include views and materialized views" default:"true"`
	IncludeStatistics bool `json:"include_statistics" description:"Include row counts and sizes from table statistics" default:"true"`
	IncludePartitions bool `json:"include_partitions" description:"Count the partitions of partitioned tables" default:"true"`
	IncludeDDL        bool `json:"include_ddl" label:"Include DDL" description:"Store the SHOW CREATE TABLE output in metadata" default:"false"`
}

// Example configuration for the plugin
var _ = `
host: "hive.internal"
port: 10000
auth: "LDAP"
username: "marmot_reader"
password: "secret"
databases:
  - "sales"
  - "marketing"
include_ddl: false
tags:
  - "hive"
  - "warehouse"
`

// Source represents the Hive plugin.
type Source struct {
	config *Config
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	config.Host = strings.TrimSpace(config.Host)
	config.Auth = strings.ToUpper(strings.TrimSpace(config.Auth))
	config.Transport = strings.ToLower(strings.TrimSpace(config.Transport))
	config.HTTPPath = strings.Trim(strings.TrimSpace(config.HTTPPath), "/")

	// Keys present in the raw config with an empty value bypass
	// ApplyDefaults, so the fallbacks are repeated here.
	if config.Port == 0 {
		config.Port = 10000
	}
	if config.Auth == "" {
		config.Auth = "NONE"
	}
	if config.Transport == "" {
		config.Transport = "binary"
	}
	if config.HTTPPath == "" {
		config.HTTPPath = "cliservice"
	}
	if config.KerberosServiceName == "" {
		config.KerberosServiceName = "hive"
	}
	// HiveServer2 without authentication still records a user for every
	// session and file it writes; hive is the conventional one.
	if config.Username == "" && (config.Auth == "NONE" || config.Auth == "NOSASL") {
		config.Username = "hive"
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers Hive databases, tables and views.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	c, err := connect(s.config)
	if err != nil {
		return nil, fmt.Errorf("connecting to HiveServer2 at %s:%d: %w", s.config.Host, s.config.Port, err)
	}
	defer c.close()

	log.Debug().Str("host", s.config.Host).Int("port", s.config.Port).Msg("Connected to HiveServer2")

	d := &discovery{
		source: s,
		client: c,
		known:  make(map[string]string),
	}

	d.version, err = c.version(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read the Hive version")
	}

	databases, err := c.databases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing databases: %w", err)
	}

	for _, database := range s.selectDatabases(databases) {
		d.discoverDatabase(ctx, database)
	}
	d.resolvePendingEdges()

	log.Info().
		Int("assets", len(d.assets)).
		Int("lineages", len(d.lineage)).
		Int("statistics", len(d.statistics)).
		Msg("Hive discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     d.assets,
		Lineage:    d.lineage,
		Statistics: d.statistics,
	}, nil
}

// selectDatabases applies the databases and exclude_databases config to
// the list the server returned. An explicit databases list is taken as is,
// so a user can still read one of the excluded system databases by naming
// it.
func (s *Source) selectDatabases(all []string) []string {
	if len(s.config.Databases) > 0 {
		wanted := make(map[string]bool, len(s.config.Databases))
		for _, name := range s.config.Databases {
			wanted[strings.ToLower(name)] = true
		}
		var selected []string
		for _, name := range all {
			if wanted[strings.ToLower(name)] {
				selected = append(selected, name)
			}
		}
		return selected
	}

	excluded := make(map[string]bool, len(s.config.ExcludeDatabases))
	for _, name := range s.config.ExcludeDatabases {
		excluded[strings.ToLower(name)] = true
	}
	var selected []string
	for _, name := range all {
		if !excluded[strings.ToLower(name)] {
			selected = append(selected, name)
		}
	}
	return selected
}

// discovery accumulates one run's output. View and foreign key edges are
// kept pending until every database has been read, because a view may read
// a table in another database and the server drops an edge whose endpoint
// was never created.
type discovery struct {
	source     *Source
	client     *client
	version    string
	assets     []pluginsdk.Asset
	lineage    []pluginsdk.LineageEdge
	statistics []pluginsdk.Statistic
	// known maps the lowercased db.table of every discovered table and
	// view to its MRN.
	known   map[string]string
	pending []pendingEdge
}

// pendingEdge is a lineage edge with one endpoint still named by db.table
// rather than by MRN.
type pendingEdge struct {
	// ref is the db.table the edge must be resolved against.
	ref string
	// mrn is the endpoint already known.
	mrn string
	// refIsSource says whether ref is the Source (VIEW_OF: base table ->
	// view) or the Target (FOREIGN_KEY: table -> referenced table).
	refIsSource bool
	edgeType    string
}

func (d *discovery) discoverDatabase(ctx context.Context, database string) {
	log.Debug().Str("database", database).Msg("Discovering database")

	info, err := d.client.describeDatabase(ctx, database)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Msg("Failed to describe database")
		info = &databaseInfo{}
	}

	names, kinds, err := d.listObjects(ctx, database)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Msg("Failed to list tables")
	}

	dbAsset := d.databaseAsset(database, info)
	dbIndex := len(d.assets)
	d.assets = append(d.assets, dbAsset)

	tableCount, viewCount := 0, 0
	for _, name := range names {
		if !d.source.config.IncludeViews && kinds[name] != "" {
			continue
		}
		asset, ok := d.discoverObject(ctx, database, name)
		if !ok {
			continue
		}
		if asset.Type == "View" {
			viewCount++
		} else {
			tableCount++
		}
		d.lineage = append(d.lineage, pluginsdk.LineageEdge{
			Source: *dbAsset.MRN,
			Target: *asset.MRN,
			Type:   "CONTAINS",
		})
	}

	d.assets[dbIndex].Metadata["table_count"] = tableCount
	d.assets[dbIndex].Metadata["view_count"] = viewCount

	log.Debug().Str("database", database).Int("tables", tableCount).Int("views", viewCount).Msg("Discovered database")
}

// listObjects returns every table-like name in a database and, for the ones
// SHOW VIEWS or SHOW MATERIALIZED VIEWS reported, the kind. Older servers
// lack those statements; then every name is treated as a table and DESCRIBE
// FORMATTED settles the type later.
func (d *discovery) listObjects(ctx context.Context, database string) ([]string, map[string]string, error) {
	names, err := d.client.tables(ctx, database)
	if err != nil {
		return nil, nil, err
	}

	kinds := make(map[string]string)
	views, err := d.client.views(ctx, database)
	if err != nil {
		log.Debug().Err(err).Str("database", database).Msg("SHOW VIEWS not supported, treating every object as a table until described")
	}
	for _, name := range views {
		kinds[name] = "view"
	}
	materialized, err := d.client.materializedViews(ctx, database)
	if err != nil {
		log.Debug().Err(err).Str("database", database).Msg("SHOW MATERIALIZED VIEWS not supported")
	}
	for _, name := range materialized {
		kinds[name] = "materialized_view"
	}
	return names, kinds, nil
}

func (d *discovery) databaseAsset(database string, info *databaseInfo) pluginsdk.Asset {
	cfg := d.source.config
	metadata := map[string]any{
		"host":     cfg.Host,
		"port":     cfg.Port,
		"database": database,
	}
	putString(metadata, "comment", info.Comment)
	putString(metadata, "location", info.Location)
	putString(metadata, "owner", info.Owner)
	putString(metadata, "owner_type", info.OwnerType)
	putString(metadata, "hive_version", d.version)
	if len(info.Parameters) > 0 {
		metadata["parameters"] = info.Parameters
	}

	name := database
	mrnValue := assetMRN("Database", name)
	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Database",
		Providers: []string{provider},
		Metadata:  metadata,
		Tags:      pluginsdk.InterpolateTags(cfg.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       provider,
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
	if info.Comment != "" {
		description := info.Comment
		asset.Description = &description
	}
	return asset
}

// discoverObject describes one table or view and records its asset,
// statistics and pending edges. It reports false when the object was
// skipped or could not be described.
func (d *discovery) discoverObject(ctx context.Context, database, table string) (pluginsdk.Asset, bool) {
	rows, err := d.client.describeFormatted(ctx, database, table)
	if err != nil {
		log.Warn().Err(err).Str("database", database).Str("table", table).Msg("Failed to describe table")
		return pluginsdk.Asset{}, false
	}
	info := parseDescribeFormatted(rows)

	if info.isView() && !d.source.config.IncludeViews {
		return pluginsdk.Asset{}, false
	}

	var partitions []string
	partitionsKnown := false
	if d.source.config.IncludePartitions && len(info.PartitionColumns) > 0 {
		partitions, err = d.client.partitions(ctx, database, table)
		if err != nil {
			log.Warn().Err(err).Str("database", database).Str("table", table).Msg("Failed to list partitions")
		} else {
			partitionsKnown = true
		}
	}

	ddl := ""
	if d.source.config.IncludeDDL {
		ddl, err = d.client.showCreate(ctx, database, table)
		if err != nil {
			log.Warn().Err(err).Str("database", database).Str("table", table).Msg("Failed to read DDL")
		}
	}

	asset := d.source.objectAsset(database, table, info, partitions, partitionsKnown, ddl)
	d.assets = append(d.assets, asset)
	d.known[strings.ToLower(qualifiedName(database, table))] = *asset.MRN

	if d.source.config.IncludeStatistics {
		d.statistics = append(d.statistics, objectStatistics(*asset.MRN, info)...)
	}

	if info.isView() {
		for _, ref := range viewSources(info, database) {
			d.pending = append(d.pending, pendingEdge{ref: ref, mrn: *asset.MRN, refIsSource: true, edgeType: "VIEW_OF"})
		}
	}
	for _, fk := range info.ForeignKeys {
		if fk.ParentTable == "" {
			continue
		}
		d.pending = append(d.pending, pendingEdge{ref: fk.ParentTable, mrn: *asset.MRN, refIsSource: false, edgeType: "FOREIGN_KEY"})
	}

	log.Debug().Str("database", database).Str("table", table).Str("type", asset.Type).Msg("Discovered object")
	return asset, true
}

// viewSources names the tables a view reads. Hive records them outright for
// a materialized view; for a plain view they are read out of the query text,
// preferring the expanded form because Hive has already qualified every
// name in it.
func viewSources(info *tableInfo, database string) []string {
	if len(info.SourceTables) > 0 {
		refs := make([]string, 0, len(info.SourceTables))
		for _, ref := range info.SourceTables {
			refs = append(refs, qualify(ref, database))
		}
		return refs
	}
	query := info.ExpandedQuery
	if query == "" {
		query = info.OriginalQuery
	}
	return viewReferences(query, database)
}

// resolvePendingEdges turns the db.table references into edges, dropping
// the ones that point at objects this run did not discover.
func (d *discovery) resolvePendingEdges() {
	seen := make(map[string]bool)
	for _, p := range d.pending {
		target, ok := d.known[strings.ToLower(p.ref)]
		if !ok {
			log.Debug().Str("reference", p.ref).Str("type", p.edgeType).Msg("Skipping edge, referenced object not discovered")
			continue
		}
		edge := pluginsdk.LineageEdge{Source: target, Target: p.mrn, Type: p.edgeType}
		if !p.refIsSource {
			edge = pluginsdk.LineageEdge{Source: p.mrn, Target: target, Type: p.edgeType}
		}
		if edge.Source == edge.Target {
			continue
		}
		key := edge.Type + ":" + edge.Source + ":" + edge.Target
		if seen[key] {
			continue
		}
		seen[key] = true
		d.lineage = append(d.lineage, edge)
	}
}

// hiveColumn is the column shape stored in an asset's schema.
type hiveColumn struct {
	pluginsdk.Column
	IsPartitionColumn bool `json:"is_partition_column,omitempty"`
}

// objectAsset builds the asset for one described table or view.
func (s *Source) objectAsset(database, table string, info *tableInfo, partitions []string, partitionsKnown bool, ddl string) pluginsdk.Asset {
	assetType := "Table"
	if info.isView() {
		assetType = "View"
	}

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"database":    database,
		"table_name":  table,
		"object_type": info.objectType(),
	}
	putString(metadata, "owner", info.Details["Owner"])
	putString(metadata, "owner_type", info.Details["OwnerType"])
	putString(metadata, "created", hiveTime(info.Details["CreateTime"]))
	putString(metadata, "last_access", hiveTime(info.Details["LastAccessTime"]))
	putString(metadata, "location", info.Details["Location"])
	putString(metadata, "input_format", info.Storage["InputFormat"])
	putString(metadata, "output_format", info.Storage["OutputFormat"])
	if serde := info.Storage["SerDe Library"]; serde != "" && serde != "null" {
		metadata["serde"] = serde
	}
	if compressed, ok := info.Storage["Compressed"]; ok {
		metadata["compressed"] = strings.EqualFold(compressed, "Yes")
	}
	if n := info.numBuckets(); n > 0 {
		metadata["num_buckets"] = n
	}
	if cols := info.bucketColumns(); len(cols) > 0 {
		metadata["bucket_columns"] = cols
	}
	if cols := info.sortColumns(); len(cols) > 0 {
		metadata["sort_columns"] = cols
	}
	if transactional, ok := info.paramBool("transactional"); ok {
		metadata["transactional"] = transactional
	}
	if n, ok := info.paramInt("numFiles"); ok {
		metadata["num_files"] = n
	}
	if n, ok := info.paramInt("transient_lastDdlTime"); ok && n > 0 {
		metadata["last_ddl"] = time.Unix(n, 0).UTC().Format(time.RFC3339)
	}
	comment := info.Parameters["comment"]
	putString(metadata, "comment", comment)

	if len(info.PartitionColumns) > 0 {
		names := make([]string, 0, len(info.PartitionColumns))
		for _, col := range info.PartitionColumns {
			names = append(names, col.Name)
		}
		metadata["partition_columns"] = names
	}
	if partitionsKnown {
		metadata["partition_count"] = len(partitions)
		if len(partitions) > 0 {
			metadata["partition_values"] = partitions[:min(len(partitions), maxPartitionValues)]
		}
	}
	if params := trimmedParameters(info.Parameters); len(params) > 0 {
		metadata["parameters"] = params
	}
	putString(metadata, "ddl", ddl)

	name := qualifiedName(database, table)
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
	if info.isView() && info.OriginalQuery != "" {
		query := info.OriginalQuery
		language := "HiveQL"
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	if s.config.IncludeColumns {
		if err := pluginsdk.SetColumns(&asset, columns(info)); err != nil {
			log.Warn().Err(err).Str("table", name).Msg("Failed to marshal columns")
		}
	}

	return asset
}

// columns builds the schema entries, partition columns last, the order
// Hive itself lists them.
func columns(info *tableInfo) []hiveColumn {
	primary := make(map[string]bool, len(info.PrimaryKey))
	for _, col := range info.PrimaryKey {
		primary[strings.ToLower(col)] = true
	}
	notNull := make(map[string]bool, len(info.NotNull))
	for _, col := range info.NotNull {
		notNull[strings.ToLower(col)] = true
	}
	defaults := make(map[string]string, len(info.Defaults))
	for col, value := range info.Defaults {
		defaults[strings.ToLower(col)] = value
	}

	build := func(col columnInfo, partition bool) hiveColumn {
		key := strings.ToLower(col.Name)
		c := hiveColumn{
			Column: pluginsdk.Column{
				Name:        col.Name,
				DataType:    col.DataType,
				Nullable:    !notNull[key],
				PrimaryKey:  primary[key],
				Description: col.Comment,
			},
			IsPartitionColumn: partition,
		}
		if value, ok := defaults[key]; ok {
			c.Default = value
		}
		return c
	}

	cols := make([]hiveColumn, 0, len(info.Columns)+len(info.PartitionColumns))
	for _, col := range info.Columns {
		cols = append(cols, build(col, false))
	}
	for _, col := range info.PartitionColumns {
		cols = append(cols, build(col, true))
	}
	return cols
}

// objectStatistics reads the metastore's basic statistics. They are only
// as fresh as the last ANALYZE TABLE or stats-collecting write, which is
// still the number Hive itself plans with.
func objectStatistics(assetMRN string, info *tableInfo) []pluginsdk.Statistic {
	stats := []pluginsdk.Statistic{{
		AssetMRN:   assetMRN,
		MetricName: "asset.column_count",
		Value:      float64(len(info.Columns) + len(info.PartitionColumns)),
	}}
	if n, ok := info.paramInt("numRows"); ok && n >= 0 {
		stats = append(stats, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: "asset.row_count", Value: float64(n)})
	}
	if n, ok := info.paramInt("totalSize"); ok && n >= 0 {
		stats = append(stats, pluginsdk.Statistic{AssetMRN: assetMRN, MetricName: "asset.size_bytes", Value: float64(n)})
	}
	return stats
}

// surfacedParameters are the table properties reported as fields of their
// own, so they are left out of the parameters map.
var surfacedParameters = map[string]bool{
	"comment":               true,
	"numFiles":              true,
	"numRows":               true,
	"rawDataSize":           true,
	"totalSize":             true,
	"transient_lastDdlTime": true,
	"transactional":         true,
	"COLUMN_STATS_ACCURATE": true,
}

func trimmedParameters(params map[string]string) map[string]string {
	trimmed := make(map[string]string)
	for key, value := range params {
		if surfacedParameters[key] {
			continue
		}
		trimmed[key] = value
	}
	return trimmed
}

func putString(metadata map[string]any, key, value string) {
	if value != "" {
		metadata[key] = value
	}
}

// FetchSampleData implements pluginsdk.DataFetcher by reading the first
// rows of a table or view.
func (s *Source) FetchSampleData(ctx context.Context, rawConfig pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
	if a == nil || a.Metadata == nil {
		return nil, nil, fmt.Errorf("asset or asset metadata is nil")
	}
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	database, _ := a.Metadata["database"].(string)
	table, _ := a.Metadata["table_name"].(string)
	if database == "" || table == "" {
		return nil, nil, fmt.Errorf("could not determine database and table from asset metadata")
	}

	c, err := connect(s.config)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to HiveServer2 at %s:%d: %w", s.config.Host, s.config.Port, err)
	}
	defer c.close()

	columns, rows, err := c.query(ctx, "SELECT * FROM "+qualifiedIdent(database, table)+" LIMIT 20")
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}

	for i, name := range columns {
		// A server that ignores hive.resultset.use.unique.column.names still
		// prefixes every column with the table name.
		columns[i] = strings.TrimPrefix(name, strings.ToLower(table)+".")
	}
	for _, row := range rows {
		for i, value := range row {
			if b, ok := value.([]byte); ok {
				row[i] = fmt.Sprintf("0x%x", b)
			}
		}
	}

	log.Debug().Str("database", database).Str("table", table).Int("rows", len(rows)).Msg("Fetched sample data")
	return columns, rows, nil
}

// qualifiedName is the Name a table or view asset carries. Hive table names
// are only unique within a database, so the database is part of the
// identity.
func qualifiedName(database, table string) string {
	return database + "." + table
}

// assetMRN is the single place a Hive MRN is built, so the database, table
// and lineage passes can never address one object differently.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, provider, name)
}
