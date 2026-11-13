package redash

import (
	"net"
	"strconv"
	"strings"
)

// A Redash data source names a system another Marmot plugin already
// catalogues. To point a query at the tables it reads, the table has to land
// on the MRN that system's own plugin produces, so this file translates a
// Redash data source type into that plugin's provider and naming rule. Get
// either wrong and the edge points at an asset that does not exist, which the
// Marmot server drops.
//
// The shapes here agree with plugins/openmetadata/openmetadata/projection.go
// and plugins/trino/trino/source.go connectorMap.

// nameShape is how much of a table's path the owning plugin puts in its name.
type nameShape int

const (
	// tableOnly is the object's own name, for services named by account or project.
	tableOnly nameShape = iota
	// databaseTable is database.table, for engines where the schema is the database.
	databaseTable
	// schemaName is schema.table.
	schemaName
	// databaseName is database.schema.table.
	databaseName
)

// dataSourceTarget describes the Marmot identity behind one Redash data source type.
type dataSourceTarget struct {
	// Provider is the exact provider string the owning plugin publishes.
	Provider string

	// Shape is how that plugin names a table.
	Shape nameShape

	// DatabaseKeys are the connection option keys that may hold the database, catalog or project name, tried in order. They fill the top level of a databaseName or databaseTable when the query itself did not spell it out.
	DatabaseKeys []string

	// DefaultSchema fills the schema for an engine that has exactly one and therefore never writes it in a query or a connection option.
	DefaultSchema string

	// Port is the port the engine listens on when the connection options name a host without one. Empty for hosted services, whose address says nothing a Marmot plugin could match.
	Port string
}

// dataSourceTargets maps Redash data source types to Marmot identities. The type strings and the option key names were read from a live Redash (GET /api/data_sources/types on 25.1.0); the aliases that instance does not serve are kept because older releases and forks use them.
var dataSourceTargets = map[string]dataSourceTarget{
	// Postgres and the engines that speak its wire protocol.
	"pg":                {Provider: "PostgreSQL", Shape: databaseName, DatabaseKeys: []string{"dbname"}, DefaultSchema: "public", Port: "5432"},
	"postgres":          {Provider: "PostgreSQL", Shape: databaseName, DatabaseKeys: []string{"dbname"}, DefaultSchema: "public", Port: "5432"},
	"redshift_postgres": {Provider: "PostgreSQL", Shape: databaseName, DatabaseKeys: []string{"dbname"}, DefaultSchema: "public", Port: "5439"},

	"mysql":     {Provider: "MySQL", Shape: databaseTable, DatabaseKeys: []string{"db"}, Port: "3306"},
	"rds_mysql": {Provider: "MySQL", Shape: databaseTable, DatabaseKeys: []string{"db"}, Port: "3306"},
	"mysql_rds": {Provider: "MySQL", Shape: databaseTable, DatabaseKeys: []string{"db"}, Port: "3306"},
	"mariadb":   {Provider: "MariaDB", Shape: databaseTable, DatabaseKeys: []string{"db"}, Port: "3306"},

	"clickhouse": {Provider: "ClickHouse", Shape: databaseTable, DatabaseKeys: []string{"dbname"}, Port: "8123"},
	"sqlite":     {Provider: "SQLite", Shape: tableOnly},

	// BigQuery names a table project.dataset.table, the project coming from the connection.
	"bigquery": {Provider: "BigQuery", Shape: databaseName, DatabaseKeys: []string{"projectId"}},
	// Athena has no catalog of its own: it reads the Glue Data Catalog, which plugins/glue owns and names database.table.
	"athena": {Provider: "Glue", Shape: schemaName},

	// Warehouses whose plugins keep the full path in the name.
	"snowflake":     {Provider: "Snowflake", Shape: databaseName, DatabaseKeys: []string{"database"}},
	"redshift":      {Provider: "Redshift", Shape: databaseName, DatabaseKeys: []string{"dbname"}, Port: "5439"},
	"redshift_iam":  {Provider: "Redshift", Shape: databaseName, DatabaseKeys: []string{"dbname"}, Port: "5439"},
	"mssql":         {Provider: "SQL Server", Shape: databaseName, DatabaseKeys: []string{"db"}, Port: "1433"},
	"mssql_odbc":    {Provider: "SQL Server", Shape: databaseName, DatabaseKeys: []string{"db"}, Port: "1433"},
	"cockroach":     {Provider: "CockroachDB", Shape: databaseName, DatabaseKeys: []string{"dbname"}, Port: "26257"},
	"trino":         {Provider: "Trino", Shape: databaseName, DatabaseKeys: []string{"catalog"}, Port: "8080"},
	"presto":        {Provider: "Presto", Shape: databaseName, DatabaseKeys: []string{"catalog"}, Port: "8080"},
	"databricks":    {Provider: "Databricks", Shape: databaseName, DatabaseKeys: []string{"catalog", "database"}},
	"oracle":        {Provider: "Oracle", Shape: databaseName, DatabaseKeys: []string{"servicename", "sid"}, Port: "1521"},
	"druid":         {Provider: "Druid", Shape: schemaName, DefaultSchema: "druid"},
	"elasticsearch": {Provider: "Elasticsearch", Shape: tableOnly, Port: "9200"},
}

// targetForDataSource looks up the Marmot identity for a Redash data source
// type. Redash type strings are lowercase apart from a couple of legacy ones,
// so the lookup is case-insensitive.
func targetForDataSource(dsType string) (dataSourceTarget, bool) {
	target, ok := dataSourceTargets[strings.ToLower(strings.TrimSpace(dsType))]
	return target, ok
}

// assetPath builds the levels of the Marmot MRN for a table reference read out of a query. Levels the reference left out are taken from the data source's connection options. It reports false when a level the shape needs is nowhere to be found, because a half-qualified path would address a different asset than the owning plugin created.
func (t dataSourceTarget) assetPath(options map[string]any, ref tableRef) ([]string, bool) {
	table := ref.Table()
	if table == "" {
		return nil, false
	}

	switch t.Shape {
	case tableOnly:
		return []string{table}, true

	case databaseTable:
		database := ref.Schema()
		if database == "" {
			database = firstOption(options, t.DatabaseKeys)
		}
		if database == "" {
			return nil, false
		}
		return []string{database, table}, true

	case schemaName:
		schema := t.resolveSchema(options, ref)
		if schema == "" {
			return nil, false
		}
		return []string{schema, table}, true

	case databaseName:
		schema := t.resolveSchema(options, ref)
		database := ref.Database()
		if database == "" {
			database = firstOption(options, t.DatabaseKeys)
		}
		if schema == "" || database == "" {
			return nil, false
		}
		return []string{database, schema, table}, true
	}

	return nil, false
}

// host is the host:port the connection options name, with the engine's default port when they name none. It is empty when the engine has no host to match.
func (t dataSourceTarget) host(options map[string]any) string {
	host := strings.ToLower(firstOption(options, []string{"host"}))
	if host == "" || t.Port == "" {
		return ""
	}
	port := t.Port
	switch p := options["port"].(type) {
	case float64:
		port = strconv.Itoa(int(p))
	case string:
		if p != "" {
			port = p
		}
	}
	return net.JoinHostPort(host, port)
}

func (t dataSourceTarget) resolveSchema(options map[string]any, ref tableRef) string {
	if schema := ref.Schema(); schema != "" {
		return schema
	}
	if schema := firstOption(options, []string{"schema"}); schema != "" {
		return schema
	}
	return t.DefaultSchema
}

// firstOption returns the first non-empty string value among keys.
func firstOption(options map[string]any, keys []string) string {
	for _, key := range keys {
		value, ok := options[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
