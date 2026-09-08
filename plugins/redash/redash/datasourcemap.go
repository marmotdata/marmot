package redash

import "strings"

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
	// bareName is the object's own name. Most Marmot database plugins name
	// a table this way, so the schema is dropped.
	bareName nameShape = iota
	// schemaName is schema.table.
	schemaName
	// databaseName is database.schema.table.
	databaseName
)

// dataSourceTarget describes the Marmot identity behind one Redash data
// source type.
type dataSourceTarget struct {
	// Provider is the exact provider string the owning plugin publishes.
	Provider string

	// Shape is how that plugin names a table.
	Shape nameShape

	// DatabaseKeys are the connection option keys that may hold the
	// database or catalog name, tried in order. They fill the top level of
	// a databaseName when the query itself did not spell it out.
	DatabaseKeys []string

	// DefaultSchema fills the schema for an engine that has exactly one and
	// therefore never writes it in a query or a connection option.
	DefaultSchema string
}

// dataSourceTargets maps Redash data source types to Marmot identities. The
// type strings and the option key names were read from a live Redash
// (GET /api/data_sources/types on 25.1.0); the aliases that instance does not
// serve are kept because older releases and forks use them.
var dataSourceTargets = map[string]dataSourceTarget{
	// Postgres and the engines that speak its wire protocol. Marmot's
	// postgresql plugin names a table by its bare name, so the schema is
	// dropped here too.
	"pg":                {Provider: "PostgreSQL", Shape: bareName},
	"postgres":          {Provider: "PostgreSQL", Shape: bareName},
	"redshift_postgres": {Provider: "PostgreSQL", Shape: bareName},

	"mysql":     {Provider: "MySQL", Shape: bareName},
	"rds_mysql": {Provider: "MySQL", Shape: bareName},
	"mysql_rds": {Provider: "MySQL", Shape: bareName},
	"mariadb":   {Provider: "MariaDB", Shape: bareName},

	"clickhouse": {Provider: "ClickHouse", Shape: bareName},
	"sqlite":     {Provider: "SQLite", Shape: bareName},

	// BigQuery datasets and Glue databases are containers of their own, and
	// both plugins name a table bare.
	"bigquery": {Provider: "BigQuery", Shape: bareName},
	// Athena has no catalog of its own: it reads the Glue Data Catalog,
	// which plugins/glue owns.
	"athena": {Provider: "Glue", Shape: bareName},

	// Warehouses whose plugins keep the full path in the name.
	"snowflake":     {Provider: "Snowflake", Shape: databaseName, DatabaseKeys: []string{"database"}},
	"redshift":      {Provider: "Redshift", Shape: databaseName, DatabaseKeys: []string{"dbname"}},
	"redshift_iam":  {Provider: "Redshift", Shape: databaseName, DatabaseKeys: []string{"dbname"}},
	"mssql":         {Provider: "SQL Server", Shape: databaseName, DatabaseKeys: []string{"db"}},
	"mssql_odbc":    {Provider: "SQL Server", Shape: databaseName, DatabaseKeys: []string{"db"}},
	"cockroach":     {Provider: "CockroachDB", Shape: databaseName, DatabaseKeys: []string{"dbname"}},
	"trino":         {Provider: "Trino", Shape: databaseName, DatabaseKeys: []string{"catalog"}},
	"presto":        {Provider: "Presto", Shape: databaseName, DatabaseKeys: []string{"catalog"}},
	"databricks":    {Provider: "Databricks", Shape: databaseName, DatabaseKeys: []string{"catalog", "database"}},
	"oracle":        {Provider: "Oracle", Shape: schemaName},
	"druid":         {Provider: "Druid", Shape: schemaName, DefaultSchema: "druid"},
	"elasticsearch": {Provider: "Elasticsearch", Shape: bareName},
}

// targetForDataSource looks up the Marmot identity for a Redash data source
// type. Redash type strings are lowercase apart from a couple of legacy ones,
// so the lookup is case-insensitive.
func targetForDataSource(dsType string) (dataSourceTarget, bool) {
	target, ok := dataSourceTargets[strings.ToLower(strings.TrimSpace(dsType))]
	return target, ok
}

// assetName builds the Marmot asset name for a table reference read out of a
// query. Levels the reference left out are taken from the data source's
// connection options. It reports false when a level the shape needs is
// nowhere to be found, because a half-qualified name would address a
// different asset than the owning plugin created.
func (t dataSourceTarget) assetName(options map[string]any, ref tableRef) (string, bool) {
	table := ref.Table()
	if table == "" {
		return "", false
	}

	switch t.Shape {
	case bareName:
		return table, true

	case schemaName:
		schema := t.resolveSchema(options, ref)
		if schema == "" {
			return "", false
		}
		return schema + "." + table, true

	case databaseName:
		schema := t.resolveSchema(options, ref)
		database := ref.Database()
		if database == "" {
			database = firstOption(options, t.DatabaseKeys)
		}
		if schema == "" || database == "" {
			return "", false
		}
		return database + "." + schema + "." + table, true
	}

	return "", false
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
