package amundsen

import "strings"

// Amundsen is a catalog, not a data system: every node in it describes
// something that lives somewhere else. A table under Amundsen's
// "postgres" database is a Postgres table, so Marmot catalogues it as
// one.
//
// projectionFor turns an Amundsen Database.name (which holds the
// technology, not a database) into the Marmot provider for that
// technology and the MRN naming rule its native Marmot plugin uses.
// Assets imported from Amundsen then land on the same MRNs the native
// plugin would produce and merge with them instead of duplicating them.
// This mirrors plugins/openmetadata/openmetadata/projection.go; the two
// tables agree for every technology they share.

// projection describes how one Amundsen database maps onto Marmot's
// vocabulary.
type projection struct {
	// Provider is the Marmot provider string for the technology. It also
	// becomes the service component of every MRN built here, used
	// verbatim: it has to be the exact word the technology's own plugin
	// puts in its Providers, or the two runs file the same table under
	// two assets. A provider with a space in it lands that space in the
	// MRN.
	Provider string

	// TableName builds the MRN name for a table from the parts of its
	// Amundsen key. nil means schema.table.
	//
	// Where a native plugin exists this must match the Name that plugin
	// sets, so both runs land on one asset.
	TableName func(database, cluster, schema, table string) string
}

// Name components below the technology, by shape. Named so the table
// below reads as the MRN format each technology uses.
//
// Amundsen's hierarchy is Database -> Cluster -> Schema -> Table, one
// level deeper than Marmot's. The cluster only enters the name where the
// technology's own naming has room for it, which is the leading slot of
// a three part name. Everywhere else it means something Marmot does not
// address by, such as an environment label ("gold", "master"), and it is
// recorded in metadata instead.
func nameOnly(_, _, _, table string) string                { return table }
func schemaQualified(_, _, schema, table string) string    { return join(schema, table) }
func clusterQualified(_, cluster, schema, t string) string { return join(cluster, schema, t) }

func join(parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ".")
}

// projections maps an Amundsen Database.name to its Marmot projection.
// The keys are the strings Amundsen's own extractors write, lowercased.
//
// The naming rule follows the same two rules the OpenMetadata plugin
// does, in order:
//
//  1. Where Marmot has a native plugin for the technology, copy the
//     identity that plugin lands on so the two runs reach one asset.
//     Those plugins name a table by its bare name, so Postgres, MySQL,
//     BigQuery, ClickHouse, Glue and Delta Lake tables are nameOnly, and
//     Athena is catalogued as Glue.
//
//     That means an Amundsen import cannot tell public.orders from
//     staging.orders for those technologies: the two collide into one
//     asset. Matching the plugin matters more, because a mismatch does
//     not merge at all and leaves two half populated assets.
//
//  2. Otherwise use every level the engine actually has, so two same
//     named tables in different databases stay apart. Snowflake tables
//     are database.schema.table; Hive tables, whose Amundsen cluster is
//     an environment label rather than a catalog, are not.
var projections = map[string]projection{
	// Relational databases
	"postgres":   {Provider: "PostgreSQL", TableName: nameOnly},
	"postgresql": {Provider: "PostgreSQL", TableName: nameOnly},
	"mysql":      {Provider: "MySQL", TableName: nameOnly},
	"mariadb":    {Provider: "MariaDB", TableName: nameOnly},
	"clickhouse": {Provider: "ClickHouse", TableName: nameOnly},
	"mssql":      {Provider: "SQL Server", TableName: clusterQualified},
	"oracle":     {Provider: "Oracle", TableName: schemaQualified},
	"db2":        {Provider: "Db2", TableName: clusterQualified},
	"vertica":    {Provider: "Vertica", TableName: clusterQualified},
	"teradata":   {Provider: "Teradata", TableName: schemaQualified},

	// Warehouses and lakehouses
	"snowflake": {Provider: "Snowflake", TableName: clusterQualified},
	"redshift":  {Provider: "Redshift", TableName: clusterQualified},
	"bigquery":  {Provider: "BigQuery", TableName: nameOnly},
	"hive":      {Provider: "Hive", TableName: schemaQualified},
	"presto":    {Provider: "Presto", TableName: clusterQualified},
	"trino":     {Provider: "Trino", TableName: clusterQualified},
	"dremio":    {Provider: "Dremio", TableName: clusterQualified},
	"delta":     {Provider: "Delta Lake", TableName: nameOnly},
	"deltalake": {Provider: "Delta Lake", TableName: nameOnly},
	// Athena keeps no catalog of its own: it reads the Glue Data
	// Catalog, which plugins/glue already catalogues under the Glue
	// provider, naming each table by its bare name. Naming it Athena
	// would file one table twice, once per route.
	"athena":     {Provider: "Glue", TableName: nameOnly},
	"glue":       {Provider: "Glue", TableName: nameOnly},
	"databricks": {Provider: "Databricks", TableName: clusterQualified},
	"druid":      {Provider: "Druid", TableName: schemaQualified},

	// Wide column stores
	"cassandra": {Provider: "Cassandra", TableName: schemaQualified},

	// Search
	"elasticsearch": {Provider: "Elasticsearch", TableName: nameOnly},

	// SaaS
	"salesforce": {Provider: "Salesforce", TableName: nameOnly},
}

// projectionFor returns the projection for an Amundsen database name.
// Unknown technologies keep the Amundsen name as the provider, with its
// first letter upper cased so it reads as a name rather than a key, and
// use schema qualified MRN names.
func projectionFor(database string) projection {
	p, ok := projections[strings.ToLower(strings.TrimSpace(database))]
	if !ok {
		p = projection{Provider: titleFirst(database)}
	}
	if p.TableName == nil {
		p.TableName = schemaQualified
	}
	return p
}

// dashboardProviders maps the product prefix of an Amundsen dashboard
// key ("superset_dashboard://...") to the Marmot provider for that BI
// tool, using the same provider strings the OpenMetadata projection
// does so a dashboard imported by either route is one asset.
var dashboardProviders = map[string]string{
	"superset":   "Superset",
	"mode":       "Mode",
	"redash":     "Redash",
	"tableau":    "Tableau",
	"looker":     "Looker",
	"metabase":   "Metabase",
	"powerbi":    "PowerBI",
	"grafana":    "Grafana",
	"databricks": "Databricks",
}

// dashboardProviderFor returns the Marmot provider for a dashboard
// product. Unknown products keep their own name, first letter upper
// cased, the same fallback projectionFor applies to tables.
func dashboardProviderFor(product string) string {
	if provider, ok := dashboardProviders[strings.ToLower(strings.TrimSpace(product))]; ok {
		return provider
	}
	return titleFirst(product)
}

// titleFirst upper cases the first character and leaves the rest alone,
// so an unrecognised "questdb" reads as "Questdb" without mangling a
// name that already has capitals.
func titleFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	runes := []rune(s)
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}
