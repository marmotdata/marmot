// Package presto discovers catalogs, schemas, tables and views from
// Presto clusters.
package presto

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

// provider is the Marmot provider for what exists only inside Presto:
// catalogs, and the tables of connectors no other plugin covers.
const provider = "Presto"

// Meta describes the plugin to the Marmot host.
func Meta() pluginsdk.Meta {
	return pluginsdk.Meta{
		ID:          "presto",
		Name:        "Presto",
		Description: "Discover catalogs, schemas, tables and views from Presto clusters",
		Icon:        "presto",
		Category:    "data-warehouse",
		Status:      "experimental",
		Features:    []string{"Assets", "Lineage"},
		ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
	}
}

// Config for the Presto plugin.
type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	// Connection
	Host        string `json:"host" validate:"required" description:"Presto coordinator hostname"`
	Port        int    `json:"port" default:"8080" validate:"omitempty,min=1,max=65535" description:"Presto coordinator port"`
	User        string `json:"user" validate:"required" description:"Username for authentication"`
	Password    string `json:"password,omitempty" sensitive:"true" description:"Password (requires HTTPS)"`
	Secure      bool   `json:"secure,omitempty" default:"false" description:"Use HTTPS"`
	SSLCertPath string `json:"ssl_cert_path,omitempty" label:"SSL Cert Path" description:"Path to TLS certificate file"`

	// Scope
	Catalog         string   `json:"catalog,omitempty" description:"Specific catalog to discover (all if empty)"`
	ExcludeCatalogs []string `json:"exclude_catalogs,omitempty" default:"[\"system\",\"jmx\"]" description:"Catalogs to skip"`
	ExcludeSchemas  []string `json:"exclude_schemas,omitempty" default:"[]" description:"Schemas to skip in every catalog"`

	// Discovery options
	IncludeCatalogs bool `json:"include_catalogs" default:"true" description:"Create catalog-level assets"`
	IncludeViews    bool `json:"include_views" default:"true" description:"Discover views and their definitions"`
	IncludeColumns  bool `json:"include_columns" default:"true" description:"Include column info in table metadata"`
	IncludeStats    bool `json:"include_stats,omitempty" default:"false" description:"Collect row counts with SHOW STATS (can be slow)"`
}

// Example configuration for the plugin.
var _ = `
host: "presto.company.com"
port: 8080
user: "marmot_reader"
secure: false
exclude_catalogs:
  - "system"
  - "jmx"
exclude_schemas:
  - "scratch"
include_stats: false
tags:
  - "presto"
  - "production"
`

// Source is the Presto plugin.
type Source struct {
	config *Config
	db     *sql.DB
	// tableCommentsUnavailable remembers that this cluster has no
	// system.metadata.table_comments, so the lookup fails once per run
	// instead of once per catalog.
	tableCommentsUnavailable bool
}

// Validate validates and normalises the plugin configuration.
func (s *Source) Validate(rawConfig pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	pluginsdk.ApplyDefaults(config, rawConfig)

	// The driver only sends the password over HTTPS and silently drops
	// it on plain HTTP, which would look like a working anonymous login.
	if config.Password != "" && !config.Secure {
		return nil, fmt.Errorf("password requires secure: true")
	}

	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

// Discover discovers catalogs, schemas, tables and views.
func (s *Source) Discover(ctx context.Context, rawConfig pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// The host spawns a fresh plugin process per call, so Discover cannot
	// rely on state set by an earlier Validate call.
	if _, err := s.Validate(rawConfig); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx); err != nil {
		return nil, fmt.Errorf("initialising connection: %w", err)
	}
	defer s.closeConnection()

	catalogs, err := s.discoverCatalogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering catalogs: %w", err)
	}
	log.Debug().Int("count", len(catalogs)).Msg("Discovered catalogs")

	connectors := s.catalogConnectors(ctx)
	version := s.prestoVersion(ctx)

	var assets []pluginsdk.Asset
	var lineages []pluginsdk.LineageEdge
	var statistics []pluginsdk.Statistic

	for _, catalog := range catalogs {
		info := connectorInfoForName(connectors[catalog])

		schemas, err := s.discoverSchemas(ctx, catalog)
		if err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to discover schemas")
			continue
		}

		tableAssets, err := s.discoverTables(ctx, catalog, connectors[catalog], info)
		if err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to discover tables")
			continue
		}
		log.Debug().Str("catalog", catalog).Int("schemas", len(schemas)).Int("tables", len(tableAssets)).Msg("Discovered tables")

		if s.config.IncludeViews {
			s.attachViewDefinitions(ctx, catalog, tableAssets)
		}
		if s.config.IncludeColumns {
			statistics = append(statistics, s.attachColumns(ctx, catalog, tableAssets)...)
		}
		s.attachTableComments(ctx, catalog, tableAssets)
		if s.config.IncludeStats {
			statistics = append(statistics, s.collectRowCounts(ctx, tableAssets)...)
		}

		if s.config.IncludeCatalogs {
			catalogAsset := s.createCatalogAsset(catalog, connectors[catalog], version, len(schemas), tableAssets)
			assets = append(assets, catalogAsset)
			for i := range tableAssets {
				lineages = append(lineages, pluginsdk.LineageEdge{
					Source: *catalogAsset.MRN,
					Target: *tableAssets[i].MRN,
					Type:   "CONTAINS",
				})
			}
		}

		assets = append(assets, tableAssets...)
	}

	// Views can read tables in other catalogs, so base tables are
	// resolved once every catalog is in.
	lineages = append(lineages, viewLineage(assets)...)

	log.Info().
		Int("assets", len(assets)).
		Int("lineages", len(lineages)).
		Int("statistics", len(statistics)).
		Msg("Presto discovery completed")

	return &pluginsdk.DiscoveryResult{
		Assets:     assets,
		Lineage:    lineages,
		Statistics: statistics,
	}, nil
}

// discoverCatalogs lists the catalogs to discover. SHOW CATALOGS needs
// no access to the system catalog, so it works for the most locked-down
// users.
func (s *Source) discoverCatalogs(ctx context.Context) ([]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, "SHOW CATALOGS")
	if err != nil {
		return nil, fmt.Errorf("querying catalogs: %w", err)
	}
	defer rows.Close()

	var catalogs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Warn().Err(err).Msg("Failed to scan catalog row")
			continue
		}

		if s.config.Catalog != "" && name != s.config.Catalog {
			continue
		}
		if containsFold(s.config.ExcludeCatalogs, name) {
			log.Debug().Str("catalog", name).Msg("Skipping excluded catalog")
			continue
		}

		catalogs = append(catalogs, name)
	}

	if err := rowsErr(rows); err != nil {
		return nil, fmt.Errorf("iterating catalog rows: %w", err)
	}

	return catalogs, nil
}

// catalogConnectors maps each catalog to its connector name. The map is
// empty when the system catalog cannot be read; every catalog then
// falls back to the Presto provider.
func (s *Source) catalogConnectors(ctx context.Context) map[string]string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	connectors := make(map[string]string)

	rows, err := s.db.QueryContext(queryCtx, "SELECT catalog_name, connector_name FROM system.metadata.catalogs")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to query connector names, assets will use the Presto provider")
		return connectors
	}
	defer rows.Close()

	for rows.Next() {
		var catalog, connector string
		if err := rows.Scan(&catalog, &connector); err != nil {
			log.Warn().Err(err).Msg("Failed to scan connector row")
			continue
		}
		connectors[catalog] = connector
	}

	if err := rowsErr(rows); err != nil {
		log.Warn().Err(err).Msg("Error iterating connector rows")
	}

	return connectors
}

// prestoVersion reads the coordinator's version. Presto has no version()
// function, so it comes from the node table; empty when unreadable.
func (s *Source) prestoVersion(ctx context.Context) string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, "SELECT node_version FROM system.runtime.nodes WHERE coordinator = true")
	if err != nil {
		log.Debug().Err(err).Msg("Failed to query Presto version")
		return ""
	}
	defer rows.Close()

	var version string
	for rows.Next() {
		if err := rows.Scan(&version); err != nil {
			log.Debug().Err(err).Msg("Failed to scan version row")
		}
		break
	}
	return version
}

func (s *Source) discoverSchemas(ctx context.Context, catalog string) ([]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, "SHOW SCHEMAS FROM "+quoteIdentifier(catalog))
	if err != nil {
		return nil, fmt.Errorf("querying schemas in catalog %s: %w", catalog, err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to scan schema row")
			continue
		}
		if s.isExcludedSchema(name) {
			continue
		}
		schemas = append(schemas, name)
	}

	if err := rowsErr(rows); err != nil {
		return nil, fmt.Errorf("iterating schema rows: %w", err)
	}

	return schemas, nil
}

// isExcludedSchema reports whether a schema is skipped. information_schema
// describes the catalog rather than holding data, so it always is.
func (s *Source) isExcludedSchema(name string) bool {
	return strings.EqualFold(name, "information_schema") || containsFold(s.config.ExcludeSchemas, name)
}

// excludedSchemaList renders the skipped schemas as a SQL IN list of
// lowercased names, so a query can leave them out server side.
func (s *Source) excludedSchemaList() string {
	names := []string{"'information_schema'"}
	for _, schema := range s.config.ExcludeSchemas {
		names = append(names, "'"+escapeString(strings.ToLower(schema))+"'")
	}
	return strings.Join(names, ", ")
}

func (s *Source) discoverTables(ctx context.Context, catalog, connector string, info connectorInfo) ([]pluginsdk.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitised via quoteIdentifier/escapeString
		`SELECT table_schema, table_name, table_type
		 FROM %s.information_schema.tables
		 WHERE lower(table_schema) NOT IN (%s)
		 ORDER BY table_schema, table_name`,
		quoteIdentifier(catalog),
		s.excludedSchemaList(),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables in catalog %s: %w", catalog, err)
	}
	defer rows.Close()

	var assets []pluginsdk.Asset
	for rows.Next() {
		var schema, table, tableType string
		if err := rows.Scan(&schema, &table, &tableType); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to scan table row")
			continue
		}

		isView := strings.EqualFold(tableType, "VIEW")
		if isView && !s.config.IncludeViews {
			continue
		}

		assets = append(assets, s.createTableAsset(catalog, schema, table, tableType, connector, info))
	}

	if err := rowsErr(rows); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	return assets, nil
}

// attachViewDefinitions sets each view's SQL as its Query.
func (s *Source) attachViewDefinitions(ctx context.Context, catalog string, assets []pluginsdk.Asset) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitised via quoteIdentifier/escapeString
		`SELECT table_schema, table_name, view_definition
		 FROM %s.information_schema.views
		 WHERE lower(table_schema) NOT IN (%s)`,
		quoteIdentifier(catalog),
		s.excludedSchemaList(),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to query view definitions")
		return
	}
	defer rows.Close()

	definitions := make(map[tableKey]string)
	for rows.Next() {
		var schema, table string
		var definition sql.NullString
		if err := rows.Scan(&schema, &table, &definition); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to scan view row")
			continue
		}
		if definition.Valid && definition.String != "" {
			definitions[tableKey{catalog, schema, table}] = definition.String
		}
	}

	if err := rowsErr(rows); err != nil {
		log.Warn().Err(err).Str("catalog", catalog).Msg("Error iterating view rows")
	}

	for i := range assets {
		if assets[i].Type != "View" {
			continue
		}
		if definition, ok := definitions[assetKey(assets[i])]; ok {
			lang := "SQL"
			assets[i].Query = &definition
			assets[i].QueryLanguage = &lang
		}
	}
}

// attachColumns reads a catalog's columns in one query, sets them on the
// matching assets and returns a column count statistic per asset.
func (s *Source) attachColumns(ctx context.Context, catalog string, assets []pluginsdk.Asset) []pluginsdk.Statistic {
	where := fmt.Sprintf("lower(table_schema) NOT IN (%s)", s.excludedSchemaList())

	columns, err := s.fetchColumns(ctx, catalog, where)
	if err != nil {
		log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to query columns")
		return nil
	}

	var statistics []pluginsdk.Statistic
	for i := range assets {
		cols, ok := columns[assetKey(assets[i])]
		if !ok {
			continue
		}
		if err := pluginsdk.SetColumns(&assets[i], cols); err != nil {
			log.Warn().Err(err).Str("table", *assets[i].Name).Msg("Failed to set columns")
			continue
		}
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *assets[i].MRN,
			MetricName: "asset.column_count",
			Value:      float64(len(cols)),
		})
	}

	return statistics
}

// column is the per-column shape stored in an asset's schema. Presto's
// type text (row(...), array(...), map(...)) is kept verbatim.
type column struct {
	pluginsdk.Column
	OrdinalPosition int64 `json:"ordinal_position"`
}

// columnRow is one information_schema.columns row.
type columnRow struct {
	Schema          string
	Table           string
	Name            string
	OrdinalPosition int64
	Default         sql.NullString
	IsNullable      string
	DataType        string
	Comment         sql.NullString
}

func newColumn(row columnRow) column {
	col := column{
		Column: pluginsdk.Column{
			Name:        row.Name,
			DataType:    row.DataType,
			Nullable:    strings.EqualFold(row.IsNullable, "YES"),
			Description: row.Comment.String,
		},
		OrdinalPosition: row.OrdinalPosition,
	}
	if row.Default.Valid && row.Default.String != "" {
		col.Default = row.Default.String
	}
	return col
}

// fetchColumns reads information_schema.columns for one catalog, narrowed
// by a WHERE clause the caller has already made safe, keyed by table.
func (s *Source) fetchColumns(ctx context.Context, catalog, where string) (map[tableKey][]column, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: inputs sanitised by the caller
		`SELECT table_schema, table_name, column_name, ordinal_position, column_default, is_nullable, data_type, comment
		 FROM %s.information_schema.columns
		 WHERE %s
		 ORDER BY table_schema, table_name, ordinal_position`,
		quoteIdentifier(catalog),
		where,
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying columns in catalog %s: %w", catalog, err)
	}
	defer rows.Close()

	columns := make(map[tableKey][]column)
	for rows.Next() {
		var row columnRow
		if err := rows.Scan(&row.Schema, &row.Table, &row.Name, &row.OrdinalPosition, &row.Default, &row.IsNullable, &row.DataType, &row.Comment); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Msg("Failed to scan column row")
			continue
		}
		key := tableKey{catalog, row.Schema, row.Table}
		columns[key] = append(columns[key], newColumn(row))
	}

	if err := rowsErr(rows); err != nil {
		return nil, fmt.Errorf("iterating column rows: %w", err)
	}

	return columns, nil
}

// attachTableComments sets table comments as descriptions where the
// cluster exposes system.metadata.table_comments. PrestoDB 0.299 does
// not, so this is expected to fall through on most clusters.
func (s *Source) attachTableComments(ctx context.Context, catalog string, assets []pluginsdk.Asset) {
	if s.tableCommentsUnavailable {
		return
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf( //nolint:gosec // G201: input sanitised via escapeString
		"SELECT schema_name, table_name, comment FROM system.metadata.table_comments WHERE catalog_name = '%s'",
		escapeString(catalog),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Msg("Table comments are not available on this cluster")
		s.tableCommentsUnavailable = true
		return
	}
	defer rows.Close()

	comments := make(map[tableKey]string)
	for rows.Next() {
		var schema, table string
		var comment sql.NullString
		if err := rows.Scan(&schema, &table, &comment); err != nil {
			continue
		}
		if comment.Valid && comment.String != "" {
			comments[tableKey{catalog, schema, table}] = comment.String
		}
	}

	if err := rowsErr(rows); err != nil {
		log.Debug().Err(err).Str("catalog", catalog).Msg("Error iterating table comment rows")
	}

	for i := range assets {
		if comment, ok := comments[assetKey(assets[i])]; ok {
			assets[i].Metadata["comment"] = comment
			desc := comment
			assets[i].Description = &desc
		}
	}
}

// collectRowCounts runs SHOW STATS per table. Views are skipped: SHOW
// STATS rejects them, and connectors without statistics return no row
// count, which is silently absent rather than zero.
func (s *Source) collectRowCounts(ctx context.Context, assets []pluginsdk.Asset) []pluginsdk.Statistic {
	var statistics []pluginsdk.Statistic

	for i := range assets {
		if assets[i].Type != "Table" {
			continue
		}
		key := assetKey(assets[i])

		rowCount, ok := s.tableRowCount(ctx, key)
		if !ok {
			continue
		}
		statistics = append(statistics, pluginsdk.Statistic{
			AssetMRN:   *assets[i].MRN,
			MetricName: "asset.row_count",
			Value:      rowCount,
		})
	}

	return statistics
}

// tableRowCount reads the row_count from the summary row of SHOW STATS,
// the one whose column_name is NULL.
func (s *Source) tableRowCount(ctx context.Context, key tableKey) (float64, bool) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := "SHOW STATS FOR " + qualifiedName(key.Catalog, key.Schema, key.Table)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Str("table", key.String()).Msg("Failed to get table stats")
		return 0, false
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, false
	}
	columnNameIdx, rowCountIdx := -1, -1
	for i, c := range cols {
		switch c {
		case "column_name":
			columnNameIdx = i
		case "row_count":
			rowCountIdx = i
		}
	}
	if columnNameIdx < 0 || rowCountIdx < 0 {
		return 0, false
	}

	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			continue
		}
		if values[columnNameIdx] != nil {
			continue
		}
		if count, ok := values[rowCountIdx].(float64); ok {
			return count, true
		}
	}

	return 0, false
}

func (s *Source) createCatalogAsset(catalog, connector, version string, schemaCount int, tables []pluginsdk.Asset) pluginsdk.Asset {
	tableCount, viewCount := 0, 0
	for _, t := range tables {
		if t.Type == "View" {
			viewCount++
		} else {
			tableCount++
		}
	}

	metadata := map[string]any{
		"catalog_name": catalog,
		"schema_count": schemaCount,
		"table_count":  tableCount,
		"view_count":   viewCount,
		"host":         s.config.Host,
		"port":         s.config.Port,
	}
	if connector != "" {
		metadata["connector_name"] = connector
	}
	if version != "" {
		metadata["presto_version"] = version
	}

	name := catalog
	mrnValue := assetMRN("Catalog", provider, name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      "Catalog",
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

func (s *Source) createTableAsset(catalog, schema, table, tableType, connector string, info connectorInfo) pluginsdk.Asset {
	metadata := map[string]any{
		"catalog":    catalog,
		"schema":     schema,
		"table_name": table,
		"table_type": tableType,
	}
	if connector != "" {
		metadata["connector_name"] = connector
	}

	assetType := "Table"
	if strings.EqualFold(tableType, "VIEW") {
		assetType = "View"
	}

	// Name doubles as the MRN's name segment, so the two must carry the
	// same string.
	name := info.MRNName(catalog, schema, table)
	mrnValue := assetMRN(assetType, info.Provider, name)

	return pluginsdk.Asset{
		Name:      &name,
		MRN:       &mrnValue,
		Type:      assetType,
		Providers: []string{info.Provider},
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

// assetMRN is the single place a Presto MRN is built, so assets and the
// lineage that points at them can never drift apart. The service is a
// parameter because tables of a mapped connector belong to that
// connector's native provider, not to Presto.
func assetMRN(assetType, service, name string) string {
	return mrn.New(assetType, service, name)
}

// tableKey addresses one table or view by its Presto path.
type tableKey struct {
	Catalog string
	Schema  string
	Table   string
}

func (k tableKey) String() string {
	return k.Catalog + "." + k.Schema + "." + k.Table
}

// assetKey rebuilds a table asset's Presto path from its metadata.
func assetKey(a pluginsdk.Asset) tableKey {
	catalog, _ := a.Metadata["catalog"].(string)
	schema, _ := a.Metadata["schema"].(string)
	table, _ := a.Metadata["table_name"].(string)
	return tableKey{catalog, schema, table}
}

// containsFold reports whether names holds name, ignoring case.
func containsFold(names []string, name string) bool {
	for _, n := range names {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}
