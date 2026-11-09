package trino

import (
	"strings"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// trinoProvider is the provider of everything that exists only inside Trino: catalogs, and the tables of catalogs that are not mapped to a native system.
const trinoProvider = "Trino"

// CatalogMapping says which native system a Trino catalog reads, so its tables get the MRN the native plugin gives them.
type CatalogMapping struct {
	Zone     string `json:"zone,omitempty" description:"Zone of the native plugin that discovers this system"`
	Database string `json:"database,omitempty" description:"Database the catalog points at on the native system. Required for connectors whose native names start with a database, such as PostgreSQL, SQL Server, Oracle, Redshift, Snowflake and BigQuery (the project)"`
}

// connectorInfo describes the native identity of the tables behind one Trino connector.
type connectorInfo struct {
	Provider string
	// NeedsDatabase is true when the native path starts with a database level that a Trino catalog does not carry, so the mapping has to supply it.
	NeedsDatabase bool
	// Path returns the native path levels of a table, given the mapped database and Trino's schema and table.
	Path func(database, schema, table string) []string
}

func dbSchemaTable(database, schema, table string) []string { return []string{database, schema, table} }
func schemaTable(_, schema, table string) []string          { return []string{schema, table} }
func tableOnly(_, _, table string) []string                 { return []string{table} }

// internalConnectors lists Trino-native connectors that don't represent
// external data sources. Catalogs using these are skipped entirely.
var internalConnectors = map[string]bool{
	"memory":    true,
	"tpch":      true,
	"tpcds":     true,
	"blackhole": true,
	"localfile": true,
}

// connectorMap maps Trino connector_name values to the native provider and the path its plugin gives a table. It only applies to catalogs named in catalog_map.
var connectorMap = map[string]connectorInfo{
	// Relational databases
	"postgresql":  {Provider: "PostgreSQL", NeedsDatabase: true, Path: dbSchemaTable},
	"mysql":       {Provider: "MySQL", Path: schemaTable},
	"mariadb":     {Provider: "MariaDB", Path: schemaTable},
	"sqlserver":   {Provider: "SQL Server", NeedsDatabase: true, Path: dbSchemaTable},
	"oracle":      {Provider: "Oracle", NeedsDatabase: true, Path: dbSchemaTable},
	"clickhouse":  {Provider: "ClickHouse", Path: schemaTable},
	"singlestore": {Provider: "SingleStore", Path: schemaTable},
	"redshift":    {Provider: "Redshift", NeedsDatabase: true, Path: dbSchemaTable},

	// Cloud warehouses / lakehouses
	"snowflake":  {Provider: "Snowflake", NeedsDatabase: true, Path: dbSchemaTable},
	"bigquery":   {Provider: "BigQuery", NeedsDatabase: true, Path: dbSchemaTable},
	"iceberg":    {Provider: "Iceberg", Path: schemaTable},
	"delta_lake": {Provider: "Delta Lake", Path: tableOnly},
	"hive":       {Provider: "Hive", Path: schemaTable},
	"hudi":       {Provider: "Hudi", Path: schemaTable},

	// NoSQL / document / key-value
	"mongodb":   {Provider: "MongoDB", Path: schemaTable},
	"cassandra": {Provider: "Cassandra", Path: schemaTable},
	"redis":     {Provider: "Redis", Path: tableOnly},
	"accumulo":  {Provider: "Accumulo", Path: schemaTable},

	// Search / analytics engines
	"elasticsearch": {Provider: "Elasticsearch", Path: tableOnly},
	"opensearch":    {Provider: "OpenSearch", Path: tableOnly},
	"druid":         {Provider: "Druid", Path: schemaTable},
	"pinot":         {Provider: "Pinot", Path: tableOnly},

	// Streaming
	"kafka":   {Provider: "Kafka", Path: tableOnly},
	"kinesis": {Provider: "Kinesis", Path: tableOnly},

	// Other
	"prometheus":    {Provider: "Prometheus", Path: tableOnly},
	"google_sheets": {Provider: "Google Sheets", Path: tableOnly},
	"phoenix":       {Provider: "Phoenix", Path: schemaTable},
	"ignite":        {Provider: "Ignite", Path: schemaTable},
	"kudu":          {Provider: "Kudu", Path: schemaTable},
}

// mappingFor finds the catalog_map entry of a catalog, ignoring case because Trino lower-cases catalog names.
func (c *Config) mappingFor(catalog string) (CatalogMapping, bool) {
	if m, ok := c.CatalogMap[catalog]; ok {
		return m, true
	}
	for name, m := range c.CatalogMap {
		if strings.EqualFold(name, catalog) {
			return m, true
		}
	}
	return CatalogMapping{}, false
}

// tableTarget is where a Trino table lives in Marmot: either a native provider's asset or Trino's own.
type tableTarget struct {
	Provider string
	Zone     string
	Path     []string
	// Native is true when the table is filed under the native plugin's identity.
	Native bool
}

// targetFor decides the identity of a table. A table only gets a native identity when its catalog is mapped to a zone and, for connectors whose native names start with a database, a database; anything else is Trino's own asset, because guessing would merge same-named tables across every server behind Trino.
func (c *Config) targetFor(catalog, connector, schema, table string) tableTarget {
	if info, ok := connectorMap[connector]; ok && !internalConnectors[connector] {
		if m, mapped := c.mappingFor(catalog); mapped && (!info.NeedsDatabase || m.Database != "") {
			return tableTarget{Provider: info.Provider, Zone: m.Zone, Path: info.Path(m.Database, schema, table), Native: true}
		}
	}
	return tableTarget{Provider: trinoProvider, Zone: c.Zone, Path: []string{catalog, schema, table}}
}

// mrn builds the MRN of a table of the given type.
func (t tableTarget) mrn(assetType string) string {
	return mrn.Asset(assetType, t.Provider, t.Path...).WithZone(t.Zone).String()
}

// catalogMRN builds the MRN of a catalog asset, which only Trino owns.
func (c *Config) catalogMRN(catalog string) string {
	return c.AssetMRN("Catalog", trinoProvider, catalog)
}

// unmappedReason explains why a catalog gets Trino's own assets, or returns "" when its tables get native identities.
func (c *Config) unmappedReason(catalog, connector string) string {
	info, known := connectorMap[connector]
	if !known {
		return "its connector " + connector + " has no native plugin"
	}
	m, mapped := c.mappingFor(catalog)
	if !mapped {
		return "it is not in catalog_map"
	}
	if info.NeedsDatabase && m.Database == "" {
		return "its catalog_map entry has no database, which " + connector + " needs"
	}
	return ""
}
