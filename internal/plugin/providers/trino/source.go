// +marmot:name=Trino
// +marmot:description=Discovers catalogs, schemas, tables, and views from Trino clusters with optional AI enrichment.
// +marmot:status=experimental
// +marmot:features=Assets, Lineage
package trino

//go:generate go run ../../../docgen/cmd/main.go

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
	_ "github.com/trinodb/trino-go-client/trino"
)

// connectorInfo describes how to map a Trino connector's assets to native
// provider MRNs so that assets merge with those discovered by native plugins.
type connectorInfo struct {
	Provider string
	// MRNName formats the name component of the MRN to match the native plugin.
	MRNName func(catalog, schema, table string) string
}

// defaultMRNName returns catalog.schema.table — used as the default for
// connectors without a specific native MRN format.
func defaultMRNName(catalog, schema, table string) string {
	return catalog + "." + schema + "." + table
}

// internalConnectors lists Trino-native connectors that don't represent
// external data sources. Catalogs using these are skipped entirely.
var internalConnectors = map[string]bool{
	"memory":    true,
	"tpch":      true,
	"tpcds":     true,
	"blackhole": true,
	"localfile": true,
}

// connectorMap maps known Trino connector_name values to their native provider
// info. MRN names match the native plugin's format so assets merge correctly.
// Connectors absent from this map (and not internal) use a default mapping.
var connectorMap = map[string]connectorInfo{
	// Relational databases
	"postgresql": {Provider: "PostgreSQL", MRNName: func(_, _, table string) string { return table }},
	"mysql":      {Provider: "MySQL", MRNName: func(_, _, table string) string { return table }},
	"mariadb":    {Provider: "MariaDB", MRNName: func(_, _, table string) string { return table }},
	"sqlserver":  {Provider: "SQL Server", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"oracle":     {Provider: "Oracle", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"clickhouse": {Provider: "ClickHouse", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"singlestore": {Provider: "SingleStore", MRNName: func(_, _, table string) string { return table }},
	"redshift":    {Provider: "Redshift", MRNName: func(_, schema, table string) string { return schema + "." + table }},

	// Cloud warehouses / lakehouses
	"snowflake":  {Provider: "Snowflake", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"bigquery":   {Provider: "BigQuery", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"iceberg":    {Provider: "Iceberg", MRNName: defaultMRNName},
	"delta_lake": {Provider: "Delta Lake", MRNName: defaultMRNName},
	"hive":       {Provider: "Hive", MRNName: defaultMRNName},
	"hudi":       {Provider: "Hudi", MRNName: defaultMRNName},

	// NoSQL / document / key-value
	"mongodb":       {Provider: "MongoDB", MRNName: func(_, _, table string) string { return table }},
	"cassandra":     {Provider: "Cassandra", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"redis":         {Provider: "Redis", MRNName: func(_, _, table string) string { return table }},
	"accumulo":      {Provider: "Accumulo", MRNName: func(_, schema, table string) string { return schema + "." + table }},

	// Search / analytics engines
	"elasticsearch": {Provider: "Elasticsearch", MRNName: func(_, _, table string) string { return table }},
	"opensearch":    {Provider: "OpenSearch", MRNName: func(_, _, table string) string { return table }},
	"druid":         {Provider: "Druid", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"pinot":         {Provider: "Pinot", MRNName: func(_, _, table string) string { return table }},

	// Streaming
	"kafka":   {Provider: "Kafka", MRNName: func(_, _, table string) string { return table }},
	"kinesis": {Provider: "Kinesis", MRNName: func(_, _, table string) string { return table }},

	// Other
	"prometheus":    {Provider: "Prometheus", MRNName: func(_, _, table string) string { return table }},
	"google_sheets": {Provider: "Google Sheets", MRNName: func(_, _, table string) string { return table }},
	"phoenix":       {Provider: "Phoenix", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"ignite":        {Provider: "Ignite", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"kudu":          {Provider: "Kudu", MRNName: func(_, schema, table string) string { return schema + "." + table }},
}

// connectorInfoForCatalog returns the connectorInfo for the given catalog.
// Returns false for internal connectors that should be skipped.
func connectorInfoForName(connector string) (connectorInfo, bool) {
	if internalConnectors[connector] {
		return connectorInfo{}, false
	}
	if info, ok := connectorMap[connector]; ok {
		return info, true
	}
	// Unknown external connector — use connector name as provider with default MRN naming.
	return connectorInfo{
		Provider: connector,
		MRNName:  defaultMRNName,
	}, true
}

// Config for Trino plugin
// +marmot:config
type Config struct {
	plugin.BaseConfig `json:",inline"`

	// Connection
	Host        string `json:"host" validate:"required" description:"Trino coordinator hostname"`
	Port        int    `json:"port" default:"8080" validate:"omitempty,min=1,max=65535" description:"Trino coordinator port"`
	User        string `json:"user" validate:"required" description:"Username for authentication"`
	Password    string `json:"password,omitempty" sensitive:"true" description:"Password (requires HTTPS)"`
	Secure      bool   `json:"secure,omitempty" default:"false" description:"Use HTTPS"`
	SSLCertPath string `json:"ssl_cert_path,omitempty" description:"Path to TLS certificate file"`
	AccessToken string `json:"access_token,omitempty" sensitive:"true" description:"JWT bearer token"`

	// Scope
	Catalog         string   `json:"catalog,omitempty" description:"Specific catalog to discover (all if empty)"`
	ExcludeCatalogs []string `json:"exclude_catalogs,omitempty" default:"[\"system\",\"jmx\"]" description:"Catalogs to skip"`

	// Discovery options
	IncludeCatalogs bool `json:"include_catalogs" default:"true" description:"Create catalog-level assets"`
	IncludeColumns  bool `json:"include_columns" default:"true" description:"Include column info in table metadata"`
	IncludeStats    bool `json:"include_stats,omitempty" default:"false" description:"Collect table statistics (can be slow)"`

	// AI enrichment (requires Trino AI connector)
	AICatalog              string   `json:"ai_catalog,omitempty" description:"Name of the AI connector catalog (empty = disabled)"`
	AIGenerateDescriptions bool     `json:"ai_generate_descriptions,omitempty" default:"false" description:"Auto-generate descriptions for undocumented tables"`
	AIClassifyTables       bool     `json:"ai_classify_tables,omitempty" default:"false" description:"Auto-classify tables into categories"`
	AIClassifyLabels       []string `json:"ai_classify_labels,omitempty" description:"Custom classification labels" default:"[\"analytics\",\"operational\",\"pii\",\"financial\",\"logs\",\"reference\"]"`
	AIMaxEnrichments       int      `json:"ai_max_enrichments,omitempty" default:"100" description:"Max tables to enrich with AI (0 = unlimited)"`
}

// Example configuration for the plugin
// +marmot:example-config
var _ = `
host: "trino.company.com"
port: 8080
user: "marmot_reader"
secure: false
exclude_catalogs:
  - "system"
  - "jmx"
tags:
  - "trino"
  - "production"
`

// Source represents the Trino plugin
type Source struct {
	config            *Config
	db                *sql.DB
	catalogConnectors map[string]string // catalog name → connector name
}

func (s *Source) Validate(rawConfig plugin.RawPluginConfig) (plugin.RawPluginConfig, error) {
	config, err := plugin.UnmarshalPluginConfig[Config](rawConfig)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if config.Port == 0 {
		config.Port = 8080
	}

	if config.ExcludeCatalogs == nil {
		config.ExcludeCatalogs = []string{"system", "jmx"}
	}

	// Default bool fields that should be true unless explicitly set to false
	if _, ok := rawConfig["include_catalogs"]; !ok {
		config.IncludeCatalogs = true
	}
	if _, ok := rawConfig["include_columns"]; !ok {
		config.IncludeColumns = true
	}

	if config.AIMaxEnrichments == 0 {
		if _, ok := rawConfig["ai_max_enrichments"]; !ok {
			config.AIMaxEnrichments = 100
		}
	}

	if config.AIClassifyLabels == nil && config.AIClassifyTables {
		config.AIClassifyLabels = []string{"analytics", "operational", "pii", "financial", "logs", "reference"}
	}

	if err := plugin.ValidateStruct(config); err != nil {
		return nil, err
	}

	s.config = config
	return rawConfig, nil
}

func (s *Source) Discover(ctx context.Context, pluginConfig plugin.RawPluginConfig) (*plugin.DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := s.initConnection(ctx); err != nil {
		return nil, fmt.Errorf("initializing connection: %w", err)
	}
	defer s.closeConnection()

	var assets []asset.Asset
	var lineages []lineage.LineageEdge

	catalogs, err := s.discoverCatalogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering catalogs: %w", err)
	}

	log.Debug().Int("count", len(catalogs)).Msg("Discovered catalogs")

	for _, catalogName := range catalogs {
		info, _ := connectorInfoForName(s.catalogConnectors[catalogName])

		// Create catalog asset
		if s.config.IncludeCatalogs {
			catalogAsset := s.createCatalogAsset(catalogName)
			assets = append(assets, catalogAsset)
		}

		schemas, err := s.discoverSchemas(ctx, catalogName)
		if err != nil {
			log.Warn().Err(err).Str("catalog", catalogName).Msg("Failed to discover schemas")
			continue
		}

		log.Debug().Str("catalog", catalogName).Int("count", len(schemas)).Msg("Discovered schemas")

		for _, schemaName := range schemas {
			tableAssets, err := s.discoverTables(ctx, catalogName, schemaName, info)
			if err != nil {
				log.Warn().Err(err).Str("catalog", catalogName).Str("schema", schemaName).Msg("Failed to discover tables")
				continue
			}

			log.Debug().Str("catalog", catalogName).Str("schema", schemaName).Int("count", len(tableAssets)).Msg("Discovered tables")

			if s.config.IncludeColumns && len(tableAssets) > 0 {
				s.attachColumns(ctx, catalogName, schemaName, tableAssets)
			}

			s.attachDDL(ctx, catalogName, schemaName, tableAssets)

			// Catalog -> Table/View lineage
			if s.config.IncludeCatalogs {
				for i := range tableAssets {
					lineages = append(lineages, lineage.LineageEdge{
						Source: mrn.New("Catalog", "Trino", catalogName),
						Target: *tableAssets[i].MRN,
						Type:   "CONTAINS",
					})
				}
			}

			assets = append(assets, tableAssets...)
		}

		s.attachTableComments(ctx, catalogName, assets)
	}

	if s.config.IncludeStats {
		s.collectStats(ctx, assets)
	}

	if s.config.AICatalog != "" {
		s.enrichWithAI(ctx, assets)
	}

	return &plugin.DiscoveryResult{
		Assets:  assets,
		Lineage: lineages,
	}, nil
}

func (s *Source) initConnection(ctx context.Context) error {
	s.closeConnection()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	scheme := "http"
	if s.config.Secure {
		scheme = "https"
	}

	dsn := fmt.Sprintf("%s://%s@%s:%d", scheme, s.config.User, s.config.Host, s.config.Port)

	params := []string{}
	if s.config.SSLCertPath != "" {
		params = append(params, "SSLCertPath="+s.config.SSLCertPath)
	}
	if s.config.AccessToken != "" {
		params = append(params, "accessToken="+s.config.AccessToken)
	}
	if s.config.Password != "" {
		params = append(params, "custom_auth="+s.config.User+":"+s.config.Password)
	}
	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	db, err := sql.Open("trino", dsn)
	if err != nil {
		return fmt.Errorf("opening connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return fmt.Errorf("pinging Trino: %w", err)
	}

	log.Debug().
		Str("host", s.config.Host).
		Int("port", s.config.Port).
		Msg("Successfully connected to Trino")

	s.db = db
	return nil
}

func (s *Source) closeConnection() {
	if s.db != nil {
		s.db.Close()
		s.db = nil
	}
}

func (s *Source) discoverCatalogs(ctx context.Context) ([]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(queryCtx, "SELECT catalog_name, connector_name FROM system.metadata.catalogs")
	if err != nil {
		return nil, fmt.Errorf("querying catalogs: %w", err)
	}
	defer rows.Close()

	s.catalogConnectors = make(map[string]string)
	var catalogs []string
	for rows.Next() {
		var name, connector string
		if err := rows.Scan(&name, &connector); err != nil {
			log.Warn().Err(err).Msg("Failed to scan catalog row")
			continue
		}

		if s.config.Catalog != "" && name != s.config.Catalog {
			continue
		}

		if s.isExcludedCatalog(name) {
			log.Debug().Str("catalog", name).Msg("Skipping excluded catalog")
			continue
		}

		// Skip Trino-internal connectors (memory, tpch, etc.)
		if internalConnectors[connector] {
			log.Debug().Str("catalog", name).Str("connector", connector).Msg("Skipping internal connector")
			continue
		}

		s.catalogConnectors[name] = connector
		catalogs = append(catalogs, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating catalog rows: %w", err)
	}

	return catalogs, nil
}

func (s *Source) isExcludedCatalog(name string) bool {
	for _, excluded := range s.config.ExcludeCatalogs {
		if strings.EqualFold(name, excluded) {
			return true
		}
	}
	return false
}

func (s *Source) discoverSchemas(ctx context.Context, catalog string) ([]string, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT schema_name FROM %s.information_schema.schemata WHERE schema_name NOT IN ('information_schema', 'system', 'pg_catalog')",
		quoteIdentifier(catalog),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
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
		schemas = append(schemas, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schema rows: %w", err)
	}

	return schemas, nil
}

func (s *Source) discoverTables(ctx context.Context, catalog, schema string, info connectorInfo) ([]asset.Asset, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT table_name, table_type FROM %s.information_schema.tables WHERE table_schema = '%s'",
		quoteIdentifier(catalog),
		escapeString(schema),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("querying tables in %s.%s: %w", catalog, schema, err)
	}
	defer rows.Close()

	var assets []asset.Asset
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			log.Warn().Err(err).Str("catalog", catalog).Str("schema", schema).Msg("Failed to scan table row")
			continue
		}

		a := s.createTableAsset(catalog, schema, tableName, tableType, info)
		assets = append(assets, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table rows: %w", err)
	}

	return assets, nil
}

func (s *Source) attachColumns(ctx context.Context, catalog, schema string, assets []asset.Asset) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf(
		`SELECT table_name, column_name, data_type, is_nullable, ordinal_position
		 FROM %s.information_schema.columns
		 WHERE table_schema = '%s'
		 ORDER BY table_name, ordinal_position`,
		quoteIdentifier(catalog),
		escapeString(schema),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Warn().Err(err).Str("catalog", catalog).Str("schema", schema).Msg("Failed to query columns")
		return
	}
	defer rows.Close()

	columnMap := make(map[string][]interface{})
	for rows.Next() {
		var tableName, columnName, dataType, isNullable string
		var ordinalPosition int
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &ordinalPosition); err != nil {
			log.Warn().Err(err).Msg("Failed to scan column row")
			continue
		}

		col := map[string]interface{}{
			"column_name":      columnName,
			"data_type":        dataType,
			"is_nullable":      isNullable,
			"ordinal_position": ordinalPosition,
		}
		columnMap[tableName] = append(columnMap[tableName], col)
	}

	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Msg("Error iterating column rows")
	}

	for i := range assets {
		tName, ok := assets[i].Metadata["table_name"].(string)
		if !ok {
			continue
		}
		if cols, exists := columnMap[tName]; exists {
			jsonBytes, err := json.Marshal(cols)
			if err != nil {
				log.Warn().Err(err).Str("table", tName).Msg("Failed to marshal columns")
				continue
			}
			if assets[i].Schema == nil {
				assets[i].Schema = make(map[string]string)
			}
			assets[i].Schema["columns"] = string(jsonBytes)
		}
	}
}

func (s *Source) attachTableComments(ctx context.Context, catalog string, assets []asset.Asset) {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT schema_name, table_name, comment FROM system.metadata.table_comments WHERE catalog_name = '%s'",
		escapeString(catalog),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Str("catalog", catalog).Msg("Failed to query table comments (may not be supported)")
		return
	}
	defer rows.Close()

	type commentKey struct {
		schema string
		table  string
	}
	comments := make(map[commentKey]string)

	for rows.Next() {
		var schemaName, tableName string
		var comment sql.NullString
		if err := rows.Scan(&schemaName, &tableName, &comment); err != nil {
			continue
		}
		if comment.Valid && comment.String != "" {
			comments[commentKey{schema: schemaName, table: tableName}] = comment.String
		}
	}

	for i := range assets {
		schemaVal, _ := assets[i].Metadata["schema"].(string)
		tableVal, _ := assets[i].Metadata["table_name"].(string)
		if schemaVal == "" || tableVal == "" {
			continue
		}
		catalogVal, _ := assets[i].Metadata["catalog"].(string)
		if catalogVal != catalog {
			continue
		}
		if c, ok := comments[commentKey{schema: schemaVal, table: tableVal}]; ok {
			assets[i].Metadata["comment"] = c
			desc := c
			assets[i].Description = &desc
		}
	}
}

func (s *Source) collectStats(ctx context.Context, assets []asset.Asset) {
	for i := range assets {
		if assets[i].Type != "Table" {
			continue
		}

		catalogVal, _ := assets[i].Metadata["catalog"].(string)
		schemaVal, _ := assets[i].Metadata["schema"].(string)
		tableVal, _ := assets[i].Metadata["table_name"].(string)
		if catalogVal == "" || schemaVal == "" || tableVal == "" {
			continue
		}

		rowCount := s.getTableRowCount(ctx, catalogVal, schemaVal, tableVal)
		if rowCount >= 0 {
			assets[i].Metadata["row_count"] = rowCount
		}
	}
}

func (s *Source) getTableRowCount(ctx context.Context, catalog, schema, table string) int64 {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := fmt.Sprintf("SHOW STATS FOR %s.%s.%s",
		quoteIdentifier(catalog),
		quoteIdentifier(schema),
		quoteIdentifier(table),
	)

	rows, err := s.db.QueryContext(queryCtx, query)
	if err != nil {
		log.Debug().Err(err).Str("table", catalog+"."+schema+"."+table).Msg("Failed to get table stats")
		return -1
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return -1
	}

	// SHOW STATS returns rows per column + a summary row.
	// The summary row has column_name = NULL and row_count set.
	colIdx := -1
	rowCountIdx := -1
	for i, c := range cols {
		if c == "column_name" {
			colIdx = i
		}
		if c == "row_count" {
			rowCountIdx = i
		}
	}
	if colIdx < 0 || rowCountIdx < 0 {
		return -1
	}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		// Summary row has NULL column_name
		if values[colIdx] != nil {
			continue
		}

		if values[rowCountIdx] != nil {
			switch v := values[rowCountIdx].(type) {
			case float64:
				return int64(v)
			case int64:
				return v
			}
		}
	}

	return -1
}

// enrichWithAI performs AI-powered enrichment on discovered assets
func (s *Source) enrichWithAI(ctx context.Context, assets []asset.Asset) {
	if !s.config.AIGenerateDescriptions && !s.config.AIClassifyTables {
		return
	}

	enriched := 0
	limit := s.config.AIMaxEnrichments

	for i := range assets {
		if assets[i].Type != "Table" && assets[i].Type != "View" {
			continue
		}

		if limit > 0 && enriched >= limit {
			log.Debug().Int("limit", limit).Msg("Reached AI enrichment limit")
			break
		}

		columnSummary := buildColumnSummary(assets[i].Schema)
		schemaVal, _ := assets[i].Metadata["schema"].(string)
		tableVal, _ := assets[i].Metadata["table_name"].(string)

		if s.config.AIGenerateDescriptions {
			// Only enrich tables without existing descriptions
			if assets[i].Description == nil || *assets[i].Description == "" {
				desc := s.aiGenerateDescription(ctx, schemaVal, tableVal, columnSummary)
				if desc != "" {
					assets[i].Description = &desc
					assets[i].Tags = append(assets[i].Tags, "ai-generated-description")
					enriched++
				}
			}
		}

		if s.config.AIClassifyTables {
			category := s.aiClassifyTable(ctx, tableVal, columnSummary)
			if category != "" {
				assets[i].Tags = append(assets[i].Tags, "ai-category:"+category)
				enriched++
			}
		}
	}

	log.Debug().Int("enriched", enriched).Msg("AI enrichment complete")
}

func (s *Source) aiGenerateDescription(ctx context.Context, schema, table, columnSummary string) string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(
		`Write a brief one-sentence description for a database table named "%s" in schema "%s" with columns: %s`,
		escapeString(table), escapeString(schema), escapeString(columnSummary),
	)

	query := fmt.Sprintf(
		"SELECT %s.ai.ai_gen('%s')",
		quoteIdentifier(s.config.AICatalog),
		escapeString(prompt),
	)

	var result sql.NullString
	err := s.db.QueryRowContext(queryCtx, query).Scan(&result)
	if err != nil {
		log.Warn().Err(err).Str("table", schema+"."+table).Msg("AI description generation failed")
		return ""
	}

	if result.Valid {
		return strings.TrimSpace(result.String)
	}
	return ""
}

func (s *Source) aiClassifyTable(ctx context.Context, table, columnSummary string) string {
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	labels := make([]string, len(s.config.AIClassifyLabels))
	for i, l := range s.config.AIClassifyLabels {
		labels[i] = fmt.Sprintf("'%s'", escapeString(l))
	}
	arrayLiteral := "ARRAY[" + strings.Join(labels, ", ") + "]"

	prompt := fmt.Sprintf(
		`Table "%s" with columns: %s`,
		escapeString(table), escapeString(columnSummary),
	)

	query := fmt.Sprintf(
		"SELECT %s.ai.ai_classify('%s', %s)",
		quoteIdentifier(s.config.AICatalog),
		escapeString(prompt),
		arrayLiteral,
	)

	var result sql.NullString
	err := s.db.QueryRowContext(queryCtx, query).Scan(&result)
	if err != nil {
		log.Warn().Err(err).Str("table", table).Msg("AI classification failed")
		return ""
	}

	if result.Valid {
		return strings.TrimSpace(strings.ToLower(result.String))
	}
	return ""
}

func (s *Source) createCatalogAsset(catalogName string) asset.Asset {
	metadata := map[string]interface{}{
		"host":         s.config.Host,
		"port":         s.config.Port,
		"catalog_name": catalogName,
	}

	mrnValue := mrn.New("Catalog", "Trino", catalogName)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &catalogName,
		MRN:         &mrnValue,
		Type:        "Catalog",
		Providers:   []string{"Trino"},
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Trino",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

func (s *Source) attachDDL(ctx context.Context, catalog, schema string, assets []asset.Asset) {
	for i := range assets {
		tName, ok := assets[i].Metadata["table_name"].(string)
		if !ok {
			continue
		}

		queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		query := fmt.Sprintf("SHOW CREATE TABLE %s.%s.%s",
			quoteIdentifier(catalog),
			quoteIdentifier(schema),
			quoteIdentifier(tName),
		)

		var ddl string
		err := s.db.QueryRowContext(queryCtx, query).Scan(&ddl)
		cancel()
		if err != nil {
			log.Debug().Err(err).Str("table", catalog+"."+schema+"."+tName).Msg("Failed to get DDL")
			continue
		}

		if ddl != "" {
			lang := "sql"
			assets[i].Query = &ddl
			assets[i].QueryLanguage = &lang
		}
	}
}

func (s *Source) createTableAsset(catalog, schema, tableName, tableType string, info connectorInfo) asset.Asset {
	metadata := map[string]interface{}{
		"host":       s.config.Host,
		"port":       s.config.Port,
		"catalog":    catalog,
		"schema":     schema,
		"table_name": tableName,
		"table_type": tableType,
	}

	assetType := "Table"
	if strings.EqualFold(tableType, "VIEW") {
		assetType = "View"
	}

	name := info.MRNName(catalog, schema, tableName)
	mrnValue := mrn.New(assetType, info.Provider, name)

	processedTags := plugin.InterpolateTags(s.config.Tags, metadata)

	return asset.Asset{
		Name:        &name,
		MRN:         &mrnValue,
		Type:        assetType,
		Providers:   []string{info.Provider},
		Metadata:    metadata,
		Tags:        processedTags,
		Sources: []asset.AssetSource{{
			Name:       "Trino",
			LastSyncAt: time.Now(),
			Properties: metadata,
			Priority:   1,
		}},
	}
}

// buildColumnSummary creates a concise column list for AI prompts
func buildColumnSummary(schema map[string]string) string {
	raw, ok := schema["columns"]
	if !ok || raw == "" {
		return "(no column info)"
	}

	var cols []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cols); err != nil {
		return "(no column info)"
	}

	var parts []string
	for _, colMap := range cols {
		name, _ := colMap["column_name"].(string)
		dtype, _ := colMap["data_type"].(string)
		if name != "" && dtype != "" {
			parts = append(parts, name+" "+dtype)
		}
	}

	if len(parts) == 0 {
		return "(no column info)"
	}
	return strings.Join(parts, ", ")
}

// quoteIdentifier wraps an identifier in double quotes for Trino SQL
func quoteIdentifier(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

// escapeString escapes single quotes in strings for SQL literals
func escapeString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func init() {
	meta := plugin.PluginMeta{
		ID:          "trino",
		Name:        "Trino",
		Description: "Discover catalogs, schemas, and tables from Trino clusters",
		Icon:        "trino",
		Category:    "database",
		ConfigSpec:  plugin.GenerateConfigSpec(Config{}),
	}

	if err := plugin.GetRegistry().Register(meta, &Source{}); err != nil {
		log.Fatal().Err(err).Msg("Failed to register Trino plugin")
	}
}
