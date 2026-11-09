package superset

import (
	"net"
	"net/url"
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
	// Path builds the levels of the table's MRN from the database (or catalog), schema and table.
	Path func(database, schema, table string) []string
	// Port is the port the backend listens on when the connection names a host without one. Empty for hosted services, whose address says nothing a Marmot plugin could match.
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

func tableOnly(_, _, t string) []string { return levels(t) }

// databaseTable names a table database.table, the way MySQL and ClickHouse do. Superset's schema is the database for these backends, so it is preferred over the connection's default.
func databaseTable(db, schema, t string) []string {
	if schema != "" {
		return levels(schema, t)
	}
	return levels(db, t)
}
func schemaTable(_, schema, t string) []string { return levels(schema, t) }
func fullPath(db, schema, t string) []string   { return levels(db, schema, t) }

func nonEmpty(parts ...string) []string { return levels(parts...) }

// backendMap maps the SQLAlchemy dialect name Superset reports as a database's backend to the naming of the matching Marmot plugin. The rules mirror the naming table in plugins/openmetadata/openmetadata/projection.go.
var backendMap = map[string]nativeNaming{
	"postgresql":   {Provider: "PostgreSQL", Path: fullPath, Port: "5432"},
	"mysql":        {Provider: "MySQL", Path: databaseTable, Port: "3306"},
	"mariadb":      {Provider: "MariaDB", Path: databaseTable, Port: "3306"},
	"bigquery":     {Provider: "BigQuery", Path: fullPath},
	"clickhouse":   {Provider: "ClickHouse", Path: databaseTable, Port: "8123"},
	"clickhousedb": {Provider: "ClickHouse", Path: databaseTable, Port: "8123"},
	"sqlite":       {Provider: "SQLite", Path: fullPath},
	"duckdb":       {Provider: "DuckDB", Path: fullPath},
	// Athena reads the Glue Data Catalog, which the Glue plugin names database.table.
	"awsathena":   {Provider: "Glue", Path: schemaTable},
	"athena":      {Provider: "Glue", Path: schemaTable},
	"snowflake":   {Provider: "Snowflake", Path: fullPath},
	"redshift":    {Provider: "Redshift", Path: fullPath, Port: "5439"},
	"mssql":       {Provider: "SQL Server", Path: fullPath, Port: "1433"},
	"cockroachdb": {Provider: "CockroachDB", Path: fullPath, Port: "26257"},
	"oracle":      {Provider: "Oracle", Path: fullPath, Port: "1521"},
	"druid":       {Provider: "Druid", Path: schemaTable},
	"trino":       {Provider: "Trino", Path: fullPath, Port: "8080"},
	"databricks":  {Provider: "Databricks", Path: fullPath},
}

// nativeTableMRN returns the MRN the owning plugin gives a table, or "" when Superset's backend is one Marmot has no naming rule for. The backend arrives as Superset reports it, which is lowercase already; lowering it again costs nothing and guards a hand-typed value. host is the connection's host:port and becomes a hint, so Marmot picks the zone that owns that server.
func nativeTableMRN(backend, host, database, schema, table string) string {
	naming, ok := backendMap[strings.ToLower(backend)]
	if !ok || table == "" {
		return ""
	}
	path := naming.Path(database, schema, table)
	if len(path) == 0 {
		return ""
	}
	return mrn.Asset("Table", naming.Provider, path...).WithHost(host).String()
}

// connectionHost reads the host:port of a connection, from its parameters or else its SQLAlchemy URI, with the backend's default port when there is none. It is empty for a backend with no host to match.
func connectionHost(backend string, conn *databaseConnection) string {
	naming, ok := backendMap[strings.ToLower(backend)]
	if !ok || naming.Port == "" {
		return ""
	}
	host, port := strings.ToLower(strings.TrimSpace(conn.Parameters.Host)), conn.Parameters.Port.String()
	if host == "" {
		if u, err := url.Parse(conn.SQLAlchemyURI); err == nil {
			host, port = strings.ToLower(u.Hostname()), u.Port()
		}
	}
	if host == "" {
		return ""
	}
	if port == "" {
		port = naming.Port
	}
	return net.JoinHostPort(host, port)
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
