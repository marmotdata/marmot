package openmetadata

import "strings"

// OpenMetadata is a catalog, not a data system: every entity in it
// describes something that lives somewhere else. A table under a
// Postgres service is a Postgres table, so Marmot catalogues it as one.
//
// projectionFor turns an OpenMetadata serviceType into the Marmot
// provider for that technology and the MRN naming rule its native
// Marmot plugin uses. Assets discovered through OpenMetadata then land
// on the same MRNs the native plugin would produce and merge with them
// instead of duplicating them. This mirrors what the Trino plugin does
// for its connectors (plugins/trino/trino/source.go, connectorMap); the
// two tables agree on shape for every technology they share.

// projection describes how one OpenMetadata service type maps onto
// Marmot's vocabulary.
type projection struct {
	// Provider is the Marmot provider string for the technology. It is
	// also the service component of every MRN: Marmot rebuilds an
	// asset's MRN from its type, first provider and name when the run
	// reaches the server, so a plugin that addresses an asset under
	// anything else emits lineage pointing at MRNs that do not exist.
	Provider string

	// TableName builds the MRN name for a table, view or stored
	// procedure from the parts of its OpenMetadata FQN below the
	// service. nil means database.schema.name.
	//
	// Note this must match the Name the native plugin sets, not the MRN
	// it builds: Marmot rebuilds the MRN from the name on ingest, so a
	// native plugin that sets a bare name and a qualified MRN is
	// effectively addressing its assets by the bare name.
	TableName func(database, schema, name string) string

	// TableTypes overrides the Marmot asset type for an OpenMetadata
	// tableType, for technologies whose Marmot plugin calls the same
	// thing something else.
	TableTypes map[string]string

	// TableGroup selects which OpenMetadata level becomes the asset that
	// contains a service's tables. OpenMetadata always has both a
	// database and a schema level and fills the one an engine does not
	// have with "default", so the level that carries meaning differs per
	// technology: a Postgres database, a MySQL schema, a BigQuery
	// dataset. Zero value is groupDatabase.
	TableGroup groupLevel

	// TableGroupType is that asset's type. Empty means Database.
	TableGroupType string

	// ContainerType is the asset type for a top-level storage container.
	// Empty means Container; object stores call them buckets.
	ContainerType string

	// IndexType is the asset type for a search index. Empty means Index;
	// Marmot's Elasticsearch and OpenSearch plugins catalogue indices as
	// tables, so those services follow suit.
	IndexType string
}

// groupLevel names the OpenMetadata level a technology's tables are
// grouped under.
type groupLevel int

const (
	groupDatabase groupLevel = iota
	groupSchema
	groupNone
)

// Name components below the service, by shape. Named so the table below
// reads as the MRN format each technology uses.
func nameOnly(_, _, name string) string          { return name }
func schemaQualified(_, schema, n string) string { return join(schema, n) }
func fullyQualified(db, schema, n string) string { return join(db, schema, n) }

func join(parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ".")
}

// projections maps OpenMetadata serviceType values to their Marmot
// projection.
//
// The naming rule follows two rules, in order:
//
//  1. Where Marmot has a native plugin for the technology, copy that
//     plugin's MRN format so the two runs land on the same asset. This
//     is why Postgres tables are named by the bare table name and
//     BigQuery tables by the bare table id.
//  2. Otherwise use every level OpenMetadata knows that the engine
//     actually has, so two same-named tables in different databases stay
//     apart. Snowflake tables are database.schema.table; MySQL tables,
//     whose OpenMetadata database level is a placeholder, are not.
var projections = map[string]projection{
	// Relational databases
	"Postgres":    {Provider: "PostgreSQL", TableName: nameOnly},
	"Mysql":       {Provider: "MySQL", TableName: nameOnly, TableGroup: groupSchema},
	"MariaDB":     {Provider: "MariaDB", TableName: nameOnly, TableGroup: groupSchema},
	"Mssql":       {Provider: "SQL Server", TableName: fullyQualified},
	"Oracle":      {Provider: "Oracle", TableName: fullyQualified},
	"Db2":         {Provider: "Db2", TableName: fullyQualified},
	"Clickhouse":  {Provider: "ClickHouse", TableName: nameOnly, TableGroup: groupSchema},
	"SingleStore": {Provider: "SingleStore", TableName: nameOnly, TableGroup: groupSchema},
	"Vertica":     {Provider: "Vertica", TableName: fullyQualified},
	"Teradata":    {Provider: "Teradata", TableName: fullyQualified},
	"Greenplum":   {Provider: "Greenplum", TableName: fullyQualified},
	"Cockroach":   {Provider: "CockroachDB", TableName: fullyQualified},
	"SQLite":      {Provider: "SQLite", TableName: nameOnly, TableGroup: groupSchema},
	"Exasol":      {Provider: "Exasol", TableName: fullyQualified},

	// Cloud warehouses and lakehouses
	"Snowflake":    {Provider: "Snowflake", TableName: fullyQualified},
	"BigQuery":     {Provider: "BigQuery", TableName: nameOnly, TableGroup: groupSchema, TableGroupType: "Dataset", TableTypes: map[string]string{"External": "ExternalTable"}},
	"Redshift":     {Provider: "Redshift", TableName: fullyQualified},
	"Athena":       {Provider: "Athena", TableName: schemaQualified, TableGroup: groupSchema},
	"Databricks":   {Provider: "Databricks", TableName: fullyQualified, TableGroupType: "Catalog"},
	"UnityCatalog": {Provider: "Databricks", TableName: fullyQualified, TableGroupType: "Catalog"},
	"AzureSQL":     {Provider: "SQL Server", TableName: fullyQualified},
	"Synapse":      {Provider: "Azure Synapse", TableName: fullyQualified},
	"Iceberg":      {Provider: "Iceberg", TableName: nameOnly, TableGroup: groupSchema, TableGroupType: "Namespace"},
	"DeltaLake":    {Provider: "Delta Lake", TableName: nameOnly, TableGroup: groupNone},
	"Hive":         {Provider: "Hive", TableName: fullyQualified, TableGroupType: "Catalog"},
	"Impala":       {Provider: "Impala", TableName: fullyQualified, TableGroupType: "Catalog"},
	"Trino":        {Provider: "Trino", TableName: fullyQualified, TableGroupType: "Catalog"},
	"Presto":       {Provider: "Presto", TableName: fullyQualified, TableGroupType: "Catalog"},
	"Dremio":       {Provider: "Dremio", TableName: fullyQualified, TableGroupType: "Catalog"},
	"Glue":         {Provider: "Glue", TableName: nameOnly, TableGroup: groupSchema},
	"Doris":        {Provider: "Doris", TableName: schemaQualified, TableGroup: groupSchema},
	"Druid":        {Provider: "Druid", TableName: schemaQualified, TableGroup: groupSchema},
	"Pinot":        {Provider: "Pinot", TableName: nameOnly, TableGroup: groupSchema},

	// Document, key-value and wide-column stores
	"MongoDB":   {Provider: "MongoDB", TableName: nameOnly, TableGroup: groupSchema, TableTypes: map[string]string{"Regular": "Collection"}},
	"DynamoDB":  {Provider: "DynamoDB", TableName: nameOnly, TableGroup: groupNone},
	"Cassandra": {Provider: "Cassandra", TableName: schemaQualified, TableGroup: groupSchema, TableGroupType: "Namespace"},
	"Couchbase": {Provider: "Couchbase", TableName: schemaQualified, TableGroup: groupSchema},

	// SaaS and application sources
	"Salesforce": {Provider: "Salesforce", TableName: nameOnly, TableGroup: groupNone},
	"SAS":        {Provider: "SAS", TableName: schemaQualified, TableGroup: groupSchema},
	"Domo":       {Provider: "Domo", TableName: schemaQualified, TableGroup: groupSchema},

	// Search
	"ElasticSearch": {Provider: "Elasticsearch", IndexType: "Table"},
	"OpenSearch":    {Provider: "OpenSearch", IndexType: "Table"},

	// Messaging
	"Kafka":           {Provider: "Kafka"},
	"Redpanda":        {Provider: "Kafka"},
	"Kinesis":         {Provider: "Kinesis"},
	"Pulsar":          {Provider: "Pulsar"},
	"CustomMessaging": {Provider: "Messaging"},

	// Object storage
	"S3":   {Provider: "S3", ContainerType: "Bucket"},
	"GCS":  {Provider: "GCS", ContainerType: "Bucket"},
	"ADLS": {Provider: "AzureBlob"},

	// Orchestration and transformation
	"Airflow":            {Provider: "Airflow"},
	"DBTCloud":           {Provider: "DBT"},
	"Dagster":            {Provider: "Dagster"},
	"Fivetran":           {Provider: "Fivetran"},
	"Airbyte":            {Provider: "Airbyte"},
	"Nifi":               {Provider: "NiFi"},
	"Spline":             {Provider: "Spline"},
	"Flink":              {Provider: "Flink"},
	"Matillion":          {Provider: "Matillion"},
	"Stitch":             {Provider: "Stitch"},
	"GluePipeline":       {Provider: "Glue"},
	"DatabricksPipeline": {Provider: "Databricks"},
	"DomoPipeline":       {Provider: "Domo"},
	"Wherescape":         {Provider: "Wherescape"},
	"SSIS":               {Provider: "SSIS"},

	// Dashboards
	"Looker":              {Provider: "Looker"},
	"Tableau":             {Provider: "Tableau"},
	"PowerBI":             {Provider: "PowerBI"},
	"PowerBIReportServer": {Provider: "PowerBI"},
	"Superset":            {Provider: "Superset"},
	"Metabase":            {Provider: "Metabase"},
	"Redash":              {Provider: "Redash"},
	"Mode":                {Provider: "Mode"},
	"QuickSight":          {Provider: "QuickSight"},
	"Lightdash":           {Provider: "Lightdash"},
	"QlikSense":           {Provider: "Qlik"},
	"QlikCloud":           {Provider: "Qlik"},
	"Sigma":               {Provider: "Sigma"},
	"MicroStrategy":       {Provider: "MicroStrategy"},
	"ThoughtSpot":         {Provider: "ThoughtSpot"},
	"DomoDashboard":       {Provider: "Domo"},

	// Machine learning
	"Mlflow":    {Provider: "MLflow"},
	"SageMaker": {Provider: "SageMaker"},
	"VertexAI":  {Provider: "Vertex AI"},

	// APIs
	"Rest": {Provider: "OpenAPI"},
}

// projectionFor returns the projection for an OpenMetadata serviceType.
// Unknown service types keep their OpenMetadata name as the provider and
// use fully qualified MRN names, the same fallback the Trino plugin
// applies to connectors it does not know.
func projectionFor(serviceType string) projection {
	p, ok := projections[serviceType]
	if !ok {
		p = projection{Provider: serviceType}
	}
	if p.TableName == nil {
		p.TableName = fullyQualified
	}
	if p.TableGroupType == "" {
		p.TableGroupType = "Database"
	}
	if p.ContainerType == "" {
		p.ContainerType = "Container"
	}
	if p.IndexType == "" {
		p.IndexType = "Index"
	}
	return p
}

// assetTypeFor maps an OpenMetadata tableType to a Marmot asset type,
// reusing the type strings Marmot's own plugins already emit for the
// same thing so the assets merge. Marmot's SQL plugins do not
// distinguish a materialized view from a view, so neither does this.
func (p projection) assetTypeFor(tableType string) string {
	if override, ok := p.TableTypes[tableType]; ok {
		return override
	}

	switch tableType {
	case "View", "SecureView", "MaterializedView", "Dynamic":
		return "View"
	default:
		return "Table"
	}
}
