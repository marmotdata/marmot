// Package questdb discovers tables, views and materialized views from
// QuestDB instances.
package questdb

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "questdb",
		Name:        "QuestDB",
		Description: "Discover tables, views and materialized views from QuestDB instances",
		Icon:        "questdb",
		Category:    "database",
		Status:      "experimental",
		// Discover emits VIEW_OF lineage edges, so the manifest declares
		// Lineage alongside Assets.
		Features:   []string{"Assets", "Lineage"},
		ConfigSpec: pluginsdk.GenerateConfigSpec(Config{}),
		AssetSchemas: []pluginsdk.AssetSchema{
			pluginsdk.AssetSchemaOf(QuestDBFields{}, "QuestDB",
				"The metadata fields the QuestDB plugin emits for table and view assets."),
			pluginsdk.AssetSchemaOf(QuestDBColumnFields{}, "Column",
				"The per-column fields embedded in an asset's schema."),
		},
	}
}

// Config for the QuestDB plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	Host     string `json:"host" description:"QuestDB server hostname or IP address" validate:"required"`
	Port     int    `json:"port" description:"PostgreSQL wire protocol port" default:"8812" validate:"omitempty,min=1,max=65535"`
	User     string `json:"user" description:"Username for authentication" default:"admin"`
	Password string `json:"password" description:"Password for authentication (QuestDB ships with quest)" sensitive:"true"`
	Database string `json:"database" description:"Database name, QuestDB has only one" default:"qdb"`
	SSLMode  string `json:"ssl_mode" label:"SSL Mode" description:"SSL mode (disable, require)" default:"disable" validate:"omitempty,oneof=disable require"`

	IncludeColumns           bool `json:"include_columns" description:"Whether to include column information in table metadata" default:"true"`
	IncludeViews             bool `json:"include_views" description:"Whether to discover views" default:"true"`
	IncludeMaterializedViews bool `json:"include_materialized_views" description:"Whether to discover materialized views" default:"true"`
	IncludeStatistics        bool `json:"include_statistics" description:"Whether to include row counts, column counts and sizes" default:"true"`
	ExcludeSystemTables      bool `json:"exclude_system_tables" description:"Whether to exclude QuestDB internal tables (sys.*, _*, telemetry)" default:"true"`
}

// Example configuration for the plugin
var _ = `
host: "questdb.internal"
port: 8812
user: "admin"
password: "quest"
include_statistics: true
tags:
  - "questdb"
  - "timeseries"
`

// Source represents the QuestDB plugin.
type Source struct {
	config *Config
	pool   *pgxpool.Pool
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

// discoveredObject is one tables() row together with what the other table
// functions said about it, gathered before the asset is built.
type discoveredObject struct {
	table   tableInfo
	view    *viewInfo // nil for tables
	columns []columnInfo
	// partitionCount and sizeBytes are -1 when table_partitions() was not
	// consulted or failed.
	partitionCount int64
	sizeBytes      int64
	mrn            string
}

// Discover discovers QuestDB tables, views and materialized views.
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

	s.logVersion(ctx)

	tables, err := s.listTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	log.Debug().Int("count", len(tables)).Msg("Listed tables")

	views := make(map[string]viewInfo)
	if s.config.IncludeViews {
		views = s.listViews(ctx)
	}
	if s.config.IncludeMaterializedViews {
		for name, v := range s.listMaterializedViews(ctx) {
			views[name] = v
		}
	}

	var objects []discoveredObject
	for _, t := range tables {
		if s.config.ExcludeSystemTables && isSystemTable(t.Name) {
			log.Debug().Str("table", t.Name).Msg("Skipping system table")
			continue
		}

		// The view functions are authoritative for what is a view: they
		// exist on builds whose tables() has no table_type column.
		var view *viewInfo
		if v, ok := views[strings.ToLower(t.Name)]; ok {
			view = &v
			t.Kind = kindView
			if v.Materialized {
				t.Kind = kindMaterializedView
			}
		}
		if (t.Kind == kindView && !s.config.IncludeViews) ||
			(t.Kind == kindMaterializedView && !s.config.IncludeMaterializedViews) {
			continue
		}

		objects = append(objects, s.inspect(ctx, t, view))
	}

	var (
		assets     []pluginsdk.Asset
		statistics []pluginsdk.Statistic
	)
	for i := range objects {
		asset := s.buildAsset(objects[i])
		objects[i].mrn = *asset.MRN
		assets = append(assets, asset)

		if s.config.IncludeStatistics {
			statistics = append(statistics, s.objectStatistics(ctx, objects[i])...)
		}
	}

	lineages := viewLineage(objects)

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("QuestDB discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

func (s *Source) logVersion(ctx context.Context) {
	rows, err := s.queryRows(ctx, "SELECT build()")
	if err != nil || len(rows) != 1 {
		log.Debug().Err(err).Msg("Could not read QuestDB build information")
		return
	}
	log.Info().Str("build", rows[0].str("build()")).Msg("Connected to QuestDB")
}

func (s *Source) listTables(ctx context.Context) ([]tableInfo, error) {
	rows, err := s.queryRows(ctx, "SELECT * FROM tables()")
	if err != nil {
		return nil, fmt.Errorf("querying tables(): %w", err)
	}

	tables := make([]tableInfo, 0, len(rows))
	for _, r := range rows {
		t, ok := parseTableRow(r)
		if !ok {
			log.Warn().Msg("Skipping tables() row without a table_name")
			continue
		}
		log.Debug().Str("name", t.Name).Str("kind", string(t.Kind)).Msg("Found database object")
		tables = append(tables, t)
	}

	return tables, nil
}

// listViews returns plain views keyed by lowercased name. A build without
// views() has no views, so that error is not one to surface.
func (s *Source) listViews(ctx context.Context) map[string]viewInfo {
	return s.listViewsWith(ctx, "views()", parseViewRow)
}

func (s *Source) listMaterializedViews(ctx context.Context) map[string]viewInfo {
	return s.listViewsWith(ctx, "materialized_views()", parseMaterializedViewRow)
}

func (s *Source) listViewsWith(ctx context.Context, function string, parse func(row) (viewInfo, bool)) map[string]viewInfo {
	views := make(map[string]viewInfo)

	rows, err := s.queryRows(ctx, "SELECT * FROM "+function)
	if err != nil {
		if isUnsupportedFunction(err) {
			log.Debug().Str("function", function).Msg("This QuestDB build has no such table function, skipping")
		} else {
			log.Warn().Err(err).Str("function", function).Msg("Failed to list views")
		}
		return views
	}

	for _, r := range rows {
		v, ok := parse(r)
		if !ok {
			continue
		}
		views[strings.ToLower(v.Name)] = v
	}

	return views
}

// inspect gathers columns and partition facts for one object. Each lookup
// is its own query and its own warning, so one odd table does not stop the
// rest of the run.
func (s *Source) inspect(ctx context.Context, t tableInfo, view *viewInfo) discoveredObject {
	obj := discoveredObject{table: t, view: view, partitionCount: -1, sizeBytes: -1}

	if s.config.IncludeColumns || s.config.IncludeStatistics {
		columns, err := s.tableColumns(ctx, t.Name)
		if err != nil {
			log.Warn().Err(err).Str("table", t.Name).Msg("Failed to read columns")
		} else {
			obj.columns = columns
		}
	}

	if t.isStored() {
		count, size, err := s.tablePartitions(ctx, t.Name)
		if err != nil {
			log.Warn().Err(err).Str("table", t.Name).Msg("Failed to read partitions")
		} else {
			obj.partitionCount = count
			obj.sizeBytes = size
		}
	}

	return obj
}

func (s *Source) tableColumns(ctx context.Context, table string) ([]columnInfo, error) {
	rows, err := s.queryRows(ctx, "SELECT * FROM table_columns("+quoteLiteral(table)+")")
	if err != nil {
		return nil, fmt.Errorf("querying table_columns(): %w", err)
	}

	columns := make([]columnInfo, 0, len(rows))
	for _, r := range rows {
		if c, ok := parseColumnRow(r); ok {
			columns = append(columns, c)
		}
	}

	return columns, nil
}

// tablePartitions returns how many partitions a table has and their total
// size on disk, summed server-side so a table with years of daily
// partitions does not stream every row back.
func (s *Source) tablePartitions(ctx context.Context, table string) (int64, int64, error) {
	rows, err := s.queryRows(ctx, "SELECT count() AS partitions, sum(diskSize) AS disk_size FROM table_partitions("+quoteLiteral(table)+")")
	if err != nil {
		return 0, 0, fmt.Errorf("querying table_partitions(): %w", err)
	}
	if len(rows) != 1 {
		return 0, 0, fmt.Errorf("expected one aggregate row, got %d", len(rows))
	}

	return rows[0].integer("partitions"), rows[0].integer("disk_size"), nil
}

func (s *Source) rowCount(ctx context.Context, table string) (int64, error) {
	rows, err := s.queryRows(ctx, "SELECT count() AS row_count FROM "+quoteIdent(table))
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("expected one count row, got %d", len(rows))
	}
	return rows[0].integer("row_count"), nil
}

// buildAsset turns a discovered object into the asset Marmot stores. Tables
// and materialized views carry their storage layout; every view carries its
// defining SQL as the asset query.
func (s *Source) buildAsset(obj discoveredObject) pluginsdk.Asset {
	t := obj.table

	metadata := map[string]any{
		"host":        s.config.Host,
		"port":        s.config.Port,
		"table_name":  t.Name,
		"object_type": string(t.Kind),
	}

	if t.isStored() {
		if t.DesignatedTimestamp != "" {
			metadata["designated_timestamp"] = t.DesignatedTimestamp
		}
		if t.PartitionBy != "" {
			metadata["partition_by"] = t.PartitionBy
		}
		metadata["wal_enabled"] = t.WALEnabled
		metadata["dedup"] = t.Dedup
		if ttl := t.ttl(); ttl != "" {
			metadata["ttl"] = ttl
		}
		if t.MaxUncommittedRows > 0 {
			metadata["max_uncommitted_rows"] = t.MaxUncommittedRows
		}
		if t.O3MaxLag >= 0 {
			metadata["o3_max_lag"] = t.O3MaxLag
		}
		if obj.partitionCount >= 0 {
			metadata["partition_count"] = obj.partitionCount
		}
	}

	// The kind comes from tables(), so a view is typed as one even on a
	// build whose views() function is missing and left obj.view nil.
	assetType := "Table"
	if t.Kind != kindTable {
		assetType = "View"
		metadata["materialized"] = t.Kind == kindMaterializedView
	}
	if obj.view != nil {
		if obj.view.BaseTable != "" {
			metadata["base_table"] = obj.view.BaseTable
		}
		if obj.view.RefreshType != "" {
			metadata["refresh_type"] = obj.view.RefreshType
		}
		if obj.view.RefreshPeriod != "" {
			metadata["refresh_period"] = obj.view.RefreshPeriod
		}
		if !obj.view.LastRefresh.IsZero() {
			metadata["last_refresh"] = obj.view.LastRefresh.UTC().Format(time.RFC3339)
		}
	}

	name := t.Name
	mrnValue := assetMRN(assetType, name)

	asset := pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{"QuestDB"},
		Metadata:  metadata,
		Schema:    make(map[string]string),
		Tags:      pluginsdk.InterpolateTags(s.config.Tags, metadata),
		Sources: []pluginsdk.AssetSource{{
			Name:       "QuestDB",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}

	if obj.view != nil && obj.view.SQL != "" {
		query, language := obj.view.SQL, "SQL"
		asset.Query = &query
		asset.QueryLanguage = &language
	}

	if s.config.IncludeColumns && len(obj.columns) > 0 {
		if err := pluginsdk.SetColumns(&asset, obj.columns); err != nil {
			log.Warn().Err(err).Str("table", t.Name).Msg("Failed to marshal columns")
		}
	}

	return asset
}

// objectStatistics emits a column count for every object and, for tables
// and materialized views, an exact row count and the size on disk. Plain
// views are not counted: that would run their query.
func (s *Source) objectStatistics(ctx context.Context, obj discoveredObject) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	if len(obj.columns) > 0 {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   obj.mrn,
			MetricName: "asset.column_count",
			Value:      float64(len(obj.columns)),
		})
	}

	if !obj.table.isStored() {
		return statistics
	}

	if obj.sizeBytes >= 0 {
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   obj.mrn,
			MetricName: "asset.size_bytes",
			Value:      float64(obj.sizeBytes),
		})
	}

	count, err := s.rowCount(ctx, obj.table.Name)
	if err != nil {
		log.Warn().Err(err).Str("table", obj.table.Name).Msg("Failed to count rows")
		return statistics
	}
	statistics = append(statistics, pluginsdk.Statistic{
		AssetMRN:   obj.mrn,
		MetricName: "asset.row_count",
		Value:      float64(count),
	})

	return statistics
}

// viewLineage links every view to the objects it reads: a materialized view
// to the base table QuestDB records for it, a plain view to each table its
// SQL selects FROM or JOINs. Source is the base object, Target the view.
// Edges to objects not discovered in this run are dropped, since the
// server would drop them anyway.
func viewLineage(objects []discoveredObject) []pluginsdk.LineageEdge {
	mrnByName := make(map[string]string, len(objects))
	for _, obj := range objects {
		mrnByName[strings.ToLower(obj.table.Name)] = obj.mrn
	}

	var lineages []pluginsdk.LineageEdge
	seen := make(map[string]struct{})

	for _, obj := range objects {
		if obj.view == nil {
			continue
		}

		var bases []string
		if obj.view.Materialized {
			if obj.view.BaseTable != "" {
				bases = []string{obj.view.BaseTable}
			}
		} else {
			bases = viewReferences(obj.view.SQL)
		}

		for _, base := range bases {
			sourceMRN, found := mrnByName[strings.ToLower(base)]
			if !found {
				log.Debug().Str("view", obj.table.Name).Str("base", base).
					Msg("Skipping VIEW_OF edge, base object not discovered")
				continue
			}
			if sourceMRN == obj.mrn {
				continue
			}

			key := sourceMRN + ":" + obj.mrn
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}

			lineages = append(lineages, pluginsdk.LineageEdge{
				Source: sourceMRN,
				Target: obj.mrn,
				Type:   "VIEW_OF",
			})
		}
	}

	return lineages
}

// FetchSampleData implements pluginsdk.DataFetcher: the first 20 rows of a
// table or view, for the asset preview.
func (s *Source) FetchSampleData(ctx context.Context, config pluginsdk.RawConfig, a *pluginsdk.Asset) ([]string, [][]any, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	if _, err := s.Validate(config); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	table, _ := a.Metadata["table_name"].(string)
	if table == "" && a.Name != nil {
		table = *a.Name
	}
	if table == "" {
		return nil, nil, fmt.Errorf("could not determine table name from asset")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	if err := s.initConnection(fetchCtx); err != nil {
		return nil, nil, fmt.Errorf("initialising database connection: %w", err)
	}
	defer s.closeConnection()

	log.Debug().Str("table", table).Msg("Fetching sample data")

	rows, err := s.pool.Query(fetchCtx, "SELECT * FROM "+quoteIdent(table)+" LIMIT 20")
	if err != nil {
		return nil, nil, fmt.Errorf("querying table: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columnNames := make([]string, len(fields))
	for i, field := range fields {
		columnNames[i] = field.Name
	}

	var dataRows [][]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to read row, skipping")
			continue
		}
		converted := make([]any, len(values))
		for i, v := range values {
			converted[i] = convertValue(v)
		}
		dataRows = append(dataRows, converted)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterating rows: %w", err)
	}

	return columnNames, dataRows, nil
}

// convertValue turns the Go values pgx decodes into ones that survive JSON:
// timestamps as RFC 3339 strings, UUIDs in their usual text form, and the
// NaN QuestDB uses for a null DOUBLE as null.
func convertValue(v any) any {
	switch v := v.(type) {
	case nil:
		return nil
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16])
	case []byte:
		return fmt.Sprintf("\\x%x", v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return v
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil
		}
		return v
	case []any:
		converted := make([]any, len(v))
		for i, item := range v {
			converted[i] = convertValue(item)
		}
		return converted
	default:
		return v
	}
}

// assetMRN is the single place a QuestDB MRN is built, so the asset pass
// and the lineage pass can never address the same object differently.
// QuestDB has one database and no schemas, so an object is addressed by
// its bare name, matching the OpenMetadata projection for QuestDB.
func assetMRN(assetType, name string) string {
	return mrn.New(assetType, "QuestDB", name)
}
