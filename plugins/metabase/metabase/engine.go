package metabase

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// Metabase is a BI tool, not a data system: the tables its cards read
// live in a warehouse Marmot catalogues with that warehouse's own
// plugin. Lineage from a table to a card therefore has to name the
// table exactly as that plugin does, or the edge points at an asset
// that never exists and the server drops it.
//
// engineMap turns a Metabase database engine into the Marmot provider
// for that technology and the naming rule its plugin uses. It agrees
// with plugins/openmetadata/openmetadata/projection.go and the Trino
// plugin's connectorMap for every technology they share.

// engine describes how tables behind one Metabase database engine are addressed in Marmot.
type engine struct {
	// Provider is the exact provider string of the technology's Marmot plugin. It is used verbatim as the provider part of the MRN.
	Provider string

	// Type is the asset type of a table. Empty means Table.
	Type string

	// Path builds the levels of the table's MRN from the Metabase database's details map and the table's schema and name.
	Path func(details map[string]any, schema, table string) []string

	// Port is the port the engine listens on when the details name a host without one. Empty for hosted services, whose address says nothing a Marmot plugin could match.
	Port string
}

func levels(parts ...string) []string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return kept
}

func detail(details map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, _ := details[key].(string); v != "" {
			return v
		}
	}
	return ""
}

// tableOnly names a table by itself.
func tableOnly(_ map[string]any, _, table string) []string { return levels(table) }

// schemaTable names a table schema.table.
func schemaTable(_ map[string]any, schema, table string) []string { return levels(schema, table) }

// detailTable names a table database.table with the database taken from one of the details, for engines whose Metabase schema is empty.
func detailTable(keys ...string) func(map[string]any, string, string) []string {
	return func(details map[string]any, schema, table string) []string {
		return levels(detail(details, keys...), table)
	}
}

// detailSchemaTable names a table database.schema.table with the first level taken from one of the details, such as the Snowflake database or the Trino catalog.
func detailSchemaTable(keys ...string) func(map[string]any, string, string) []string {
	return func(details map[string]any, schema, table string) []string {
		return levels(detail(details, keys...), schema, table)
	}
}

// engineMap maps Metabase engine identifiers to their Marmot identity. Engines missing from it fall back to the engine name as provider and schema.table as the path, see engineFor.
var engineMap = map[string]engine{
	"postgres":           {Provider: "PostgreSQL", Path: detailSchemaTable("dbname", "db"), Port: "5432"},
	"mysql":              {Provider: "MySQL", Path: detailTable("dbname", "db"), Port: "3306"},
	"mariadb":            {Provider: "MariaDB", Path: detailTable("dbname", "db"), Port: "3306"},
	"bigquery-cloud-sdk": {Provider: "BigQuery", Path: detailSchemaTable("project-id")},
	"clickhouse":         {Provider: "ClickHouse", Path: detailTable("dbname", "db"), Port: "8123"},
	"sqlite":             {Provider: "SQLite", Path: sqlitePath},
	// Athena reads the Glue Data Catalog, which the Glue plugin names database.table.
	"athena":      {Provider: "Glue", Path: schemaTable},
	"snowflake":   {Provider: "Snowflake", Path: detailSchemaTable("db")},
	"redshift":    {Provider: "Redshift", Path: detailSchemaTable("db"), Port: "5439"},
	"sqlserver":   {Provider: "SQL Server", Path: detailSchemaTable("db"), Port: "1433"},
	"oracle":      {Provider: "Oracle", Path: detailSchemaTable("service-name", "sid"), Port: "1521"},
	"druid":       {Provider: "Druid", Path: schemaTable},
	"presto-jdbc": {Provider: "Trino", Path: detailSchemaTable("catalog"), Port: "8080"},
	"starburst":   {Provider: "Trino", Path: detailSchemaTable("catalog"), Port: "8080"},
	"mongo":       {Provider: "MongoDB", Type: "Collection", Path: detailTable("dbname", "db"), Port: "27017"},
	"databricks":  {Provider: "Databricks", Path: detailSchemaTable("catalog")},
}

// sqlitePath names a table by the database file without its extension, and the table.
func sqlitePath(details map[string]any, _, table string) []string {
	file := filepath.Base(detail(details, "db"))
	if file == "." || file == "/" {
		file = ""
	}
	return levels(strings.TrimSuffix(file, filepath.Ext(file)), table)
}

// engineFor returns the identity rule for a Metabase engine. An engine with no Marmot plugin, such as the h2 behind older sample databases, keeps its engine name as the provider with the first letter upper-cased, and is named schema.table so two same-named tables in different schemas stay apart.
func engineFor(name string) engine {
	if e, ok := engineMap[name]; ok {
		if e.Type == "" {
			e.Type = "Table"
		}
		return e
	}
	return engine{Provider: titleCase(name), Type: "Table", Path: schemaTable}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// tableMRN is the MRN of a table behind a Metabase database, built the way the technology's own plugin builds it. The database's host and port become a hint, so Marmot picks the zone that owns that server.
func tableMRN(db database, t table) string {
	e := engineFor(db.Engine)
	return mrn.Asset(e.Type, e.Provider, e.Path(db.Details, t.Schema, t.Name)...).WithHost(databaseHost(e, db.Details)).String()
}

// databaseHost is the host:port a database's details name, with the engine's default port when it names none. It is empty for an engine with no host to match.
func databaseHost(e engine, details map[string]any) string {
	host := strings.ToLower(detail(details, "host"))
	if host == "" || e.Port == "" {
		return ""
	}
	port := e.Port
	switch p := details["port"].(type) {
	case float64:
		port = strconv.Itoa(int(p))
	case string:
		if p != "" {
			port = p
		}
	}
	return net.JoinHostPort(host, port)
}
