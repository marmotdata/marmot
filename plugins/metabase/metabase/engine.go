package metabase

import (
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

// engine describes how tables behind one Metabase database engine are
// addressed in Marmot.
type engine struct {
	// Provider is the exact provider string of the technology's Marmot
	// plugin. It is used verbatim as the service part of the MRN.
	Provider string

	// Type is the asset type of a table. Empty means Table.
	Type string

	// Name builds the asset name from the Metabase database's details
	// map and the table's schema and name.
	Name func(details map[string]any, schema, table string) string
}

// Name components, named so the table below reads as the naming rule
// each technology uses.
func bare(_ map[string]any, _, table string) string                 { return table }
func schemaQualified(_ map[string]any, schema, table string) string { return join(schema, table) }

// detailQualified prefixes the name with one key of the database's
// details, such as the Snowflake database or the Trino catalog.
func detailQualified(key string) func(map[string]any, string, string) string {
	return func(details map[string]any, schema, table string) string {
		prefix, _ := details[key].(string)
		return join(prefix, schema, table)
	}
}

func join(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ".")
}

// engineMap maps Metabase engine identifiers to their Marmot identity.
// Engines missing from it fall back to the engine name as provider and
// schema.table as name, see engineFor.
var engineMap = map[string]engine{
	"postgres":           {Provider: "PostgreSQL", Name: bare},
	"mysql":              {Provider: "MySQL", Name: bare},
	"mariadb":            {Provider: "MariaDB", Name: bare},
	"bigquery-cloud-sdk": {Provider: "BigQuery", Name: bare},
	"clickhouse":         {Provider: "ClickHouse", Name: bare},
	"sqlite":             {Provider: "SQLite", Name: bare},
	// Athena reads the Glue Data Catalog, which the Glue plugin already
	// catalogues by bare table name.
	"athena":      {Provider: "Glue", Name: bare},
	"snowflake":   {Provider: "Snowflake", Name: detailQualified("db")},
	"redshift":    {Provider: "Redshift", Name: detailQualified("db")},
	"sqlserver":   {Provider: "SQL Server", Name: detailQualified("db")},
	"oracle":      {Provider: "Oracle", Name: schemaQualified},
	"druid":       {Provider: "Druid", Name: schemaQualified},
	"presto-jdbc": {Provider: "Trino", Name: detailQualified("catalog")},
	"starburst":   {Provider: "Trino", Name: detailQualified("catalog")},
	"mongo":       {Provider: "MongoDB", Type: "Collection", Name: bare},
	"databricks":  {Provider: "Databricks", Name: detailQualified("catalog")},
}

// engineFor returns the identity rule for a Metabase engine. An engine
// with no Marmot plugin, such as the h2 behind older sample databases,
// keeps its engine name as the provider with the first letter
// upper-cased, and is named schema.table so two same-named tables in
// different schemas stay apart.
func engineFor(name string) engine {
	if e, ok := engineMap[name]; ok {
		if e.Type == "" {
			e.Type = "Table"
		}
		return e
	}
	return engine{Provider: titleCase(name), Type: "Table", Name: schemaQualified}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// tableMRN is the MRN of a table behind a Metabase database, built the
// way the technology's own plugin builds it.
func tableMRN(db database, t table) string {
	e := engineFor(db.Engine)
	return mrn.New(e.Type, e.Provider, e.Name(db.Details, t.Schema, t.Name))
}
