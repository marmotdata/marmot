package superset

import (
	"regexp"
	"strings"

	"github.com/marmotdata/plugin-sdk/mrn"
)

// A Superset dataset reads a table that some other system owns. To link
// the two, the plugin has to address that table exactly the way the
// system's own Marmot plugin does: same provider string, same name
// shape. This file holds that rule per Superset backend.

// nativeNaming describes how one technology's Marmot plugin names a table.
type nativeNaming struct {
	// Provider is the exact provider string that plugin sets.
	Provider string
	// Name builds the table name from the database (or catalog), schema
	// and table. Plugins that address tables by bare name ignore the
	// first two.
	Name func(database, schema, table string) string
}

func bare(_, _, table string) string             { return table }
func schemaQualified(_, schema, t string) string { return join(schema, t) }
func fullyQualified(db, schema, t string) string { return join(db, schema, t) }

func join(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ".")
}

// backendMap maps the SQLAlchemy dialect name Superset reports as a
// database's backend to the naming rule of the matching Marmot plugin.
// The rules mirror the table in plugins/openmetadata/openmetadata/projection.go.
var backendMap = map[string]nativeNaming{
	"postgresql":   {Provider: "PostgreSQL", Name: bare},
	"mysql":        {Provider: "MySQL", Name: bare},
	"mariadb":      {Provider: "MariaDB", Name: bare},
	"bigquery":     {Provider: "BigQuery", Name: bare},
	"clickhouse":   {Provider: "ClickHouse", Name: bare},
	"clickhousedb": {Provider: "ClickHouse", Name: bare},
	"sqlite":       {Provider: "SQLite", Name: bare},
	"duckdb":       {Provider: "DuckDB", Name: bare},
	// Athena reads the Glue Data Catalog, which the Glue plugin already
	// catalogues by bare table name.
	"awsathena":   {Provider: "Glue", Name: bare},
	"athena":      {Provider: "Glue", Name: bare},
	"snowflake":   {Provider: "Snowflake", Name: fullyQualified},
	"redshift":    {Provider: "Redshift", Name: fullyQualified},
	"mssql":       {Provider: "SQL Server", Name: fullyQualified},
	"cockroachdb": {Provider: "CockroachDB", Name: fullyQualified},
	"oracle":      {Provider: "Oracle", Name: schemaQualified},
	"druid":       {Provider: "Druid", Name: schemaQualified},
	"trino":       {Provider: "Trino", Name: fullyQualified},
	"databricks":  {Provider: "Databricks", Name: fullyQualified},
}

// nativeTableMRN returns the MRN the owning plugin gives a table, or ""
// when Superset's backend is one Marmot has no naming rule for. The
// backend arrives as Superset reports it, which is lowercase already;
// lowering it again costs nothing and guards a hand-typed value.
func nativeTableMRN(backend, database, schema, table string) string {
	naming, ok := backendMap[strings.ToLower(backend)]
	if !ok || table == "" {
		return ""
	}
	return mrn.New("Table", naming.Provider, naming.Name(database, schema, table))
}

// tableRef is one table a SQL statement reads from.
type tableRef struct {
	Database string
	Schema   string
	Table    string
}

// An identifier is plain, or quoted with double quotes, backticks or
// square brackets, and a reference is one to three of them joined by
// dots. The pattern is deliberately narrow: it only looks right after
// FROM and JOIN, so a virtual dataset gets edges to the tables it
// plainly names and nothing is guessed from the rest of the statement.
const identPattern = `(?:"[^"]+"|` + "`[^`]+`" + `|\[[^\]]+\]|[A-Za-z_][A-Za-z0-9_$]*)`

var (
	tableRefRe = regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+(` + identPattern + `(?:\s*\.\s*` + identPattern + `){0,2})\s*(\()?`)
	cteRe      = regexp.MustCompile(`(?is)(?:\bWITH\b(?:\s+RECURSIVE\b)?|,)\s*(` + identPattern + `)\s*(?:\([^)]*\)\s*)?\bAS\s*\(`)
	commentRe  = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/`)
	identRe    = regexp.MustCompile(identPattern)
)

// tableRefs extracts the tables a SELECT reads from: whatever follows
// FROM or JOIN that is a table name rather than a subquery, a function
// call or a common table expression defined in the same statement.
// References are qualified with defaultSchema when they carry no schema
// of their own, which is how the database resolves them too.
func tableRefs(sql, defaultSchema string) []tableRef {
	sql = commentRe.ReplaceAllString(sql, " ")

	ctes := make(map[string]bool)
	for _, m := range cteRe.FindAllStringSubmatch(sql, -1) {
		ctes[strings.ToLower(unquote(m[1]))] = true
	}

	var refs []tableRef
	seen := make(map[tableRef]bool)
	for _, m := range tableRefRe.FindAllStringSubmatch(sql, -1) {
		if m[2] == "(" {
			// FROM f(...) is a table function, not a table.
			continue
		}

		parts := identRe.FindAllString(m[1], -1)
		for i := range parts {
			parts[i] = unquote(parts[i])
		}

		ref := tableRef{Schema: defaultSchema}
		switch len(parts) {
		case 1:
			if ctes[strings.ToLower(parts[0])] {
				continue
			}
			ref.Table = parts[0]
		case 2:
			ref.Schema, ref.Table = parts[0], parts[1]
		case 3:
			ref.Database, ref.Schema, ref.Table = parts[0], parts[1], parts[2]
		default:
			continue
		}

		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs
}

func unquote(ident string) string {
	if len(ident) < 2 {
		return ident
	}
	first, last := ident[0], ident[len(ident)-1]
	if (first == '"' && last == '"') || (first == '`' && last == '`') || (first == '[' && last == ']') {
		return ident[1 : len(ident)-1]
	}
	return ident
}
