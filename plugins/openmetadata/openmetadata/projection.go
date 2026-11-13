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
	// Provider is the Marmot provider string for the technology. It also
	// becomes the service component of every MRN built here, used
	// verbatim: it has to be the exact word the technology's own plugin
	// puts in its Providers, not a prettier synonym and not a slugged
	// version, or the two runs file the same table under two assets. A
	// provider with a space in it lands that space in the MRN.
	Provider string

	// TableName is the single string earlier releases built the MRN of a
	// table, view or stored procedure from. It is kept only to work out
	// the PreviousMRNs of an asset whose MRN has since changed. nil means
	// database.schema.name.
	TableName func(database, schema, name string) string

	// Path is how many levels of the OpenMetadata path make up the MRN of
	// a table, view or stored procedure. It follows the naming table of
	// the technology's own Marmot plugin so both runs land on one asset.
	// Zero value is pathFull.
	Path pathShape

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

	// TopicType is the asset type for a messaging topic. Empty means
	// Topic; Marmot's Kinesis plugin catalogues a stream as a Stream, so
	// that service follows suit.
	TopicType string
}

// groupLevel names the OpenMetadata level a technology's tables are
// grouped under.
type groupLevel int

const (
	groupDatabase groupLevel = iota
	groupSchema
	groupNone
)

// pathShape picks which of a table's OpenMetadata levels go into its MRN.
type pathShape int

const (
	// pathFull is database, schema and table.
	pathFull pathShape = iota
	// pathSchemaTable is schema and table, for engines whose OpenMetadata
	// database level is a placeholder.
	pathSchemaTable
	// pathDatabaseTable is database and table.
	pathDatabaseTable
	// pathTable is the table alone, for services named by account or
	// project rather than by a database.
	pathTable
	// pathBucketKey is a bucket and the object key inside it.
	pathBucketKey
)

// tablePath returns the MRN path of a table, view or stored procedure.
func (p projection) tablePath(database, schema, name string) []string {
	switch p.Path {
	case pathSchemaTable:
		return nonEmptyLevels(schema, name)
	case pathDatabaseTable:
		return nonEmptyLevels(database, name)
	case pathTable:
		return nonEmptyLevels(name)
	case pathBucketKey:
		return nonEmptyLevels(schema, name)
	default:
		return nonEmptyLevels(database, schema, name)
	}
}

// groupPath returns the MRN path of the asset a service's tables hang off:
// the database for a technology grouped by database, otherwise the table
// path without the table.
func (p projection) groupPath(parts []string) []string {
	if len(parts) == 0 {
		return nil
	}
	if p.TableGroup == groupDatabase {
		return []string{parts[0]}
	}
	database, schema := parts[0], ""
	if len(parts) > 1 {
		schema = parts[1]
	}
	path := p.tablePath(database, schema, "")
	if len(path) == 0 {
		return []string{parts[len(parts)-1]}
	}
	return path
}

func nonEmptyLevels(levels ...string) []string {
	kept := make([]string, 0, len(levels))
	for _, l := range levels {
		if l != "" {
			kept = append(kept, l)
		}
	}
	return kept
}

// Name components below the service, by shape. Named so the table below
// reads as the MRN format each technology uses.
func nameOnly(_, _, name string) string          { return name }
func schemaQualified(_, schema, n string) string { return join(schema, n) }
func fullyQualified(db, schema, n string) string { return join(db, schema, n) }

// bucketQualified names an object by the bucket holding it. OpenMetadata's
// file-based database services put the bucket at the schema level and a
// label at the database level, and object stores address data as
// bucket/key rather than with dots.
func bucketQualified(_, bucket, key string) string {
	if bucket == "" {
		return key
	}
	return bucket + "/" + key
}

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
//  1. Where Marmot has a native plugin for the technology, copy the
//     identity that plugin lands on so the two runs reach one asset.
//     Every one of those plugins sets Name to the object's own bare
//     name, so a Postgres, MySQL, BigQuery, MongoDB, ClickHouse, Glue
//     or Iceberg table here is nameOnly, and Athena is catalogued as
//     Glue.
//
//     That means an OpenMetadata import cannot tell public.orders from
//     staging.orders for those technologies: the two collide into one
//     asset. Matching the plugin matters more, because a mismatch does
//     not merge at all and leaves two half-populated assets. Qualified
//     naming is available per-run via the naming: qualified config.
//
//  2. Otherwise use every level OpenMetadata knows that the engine
//     actually has, so two same-named tables in different databases stay
//     apart. Snowflake tables are database.schema.table; MySQL tables,
//     whose OpenMetadata database level is a placeholder, are not.
var projections = map[string]projection{
	// Relational databases
	"Postgres":    {Provider: "PostgreSQL", TableName: nameOnly, Path: pathFull},
	"Mysql":       {Provider: "MySQL", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"MariaDB":     {Provider: "MariaDB", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"Mssql":       {Provider: "SQL Server", TableName: fullyQualified, Path: pathFull},
	"Oracle":      {Provider: "Oracle", TableName: schemaQualified, Path: pathFull, TableGroup: groupSchema},
	"Db2":         {Provider: "Db2", TableName: fullyQualified, Path: pathFull},
	"Clickhouse":  {Provider: "ClickHouse", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"SingleStore": {Provider: "SingleStore", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"Vertica":     {Provider: "Vertica", TableName: fullyQualified, Path: pathFull},
	"Teradata":    {Provider: "Teradata", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"Greenplum":   {Provider: "Greenplum", TableName: fullyQualified, Path: pathFull},
	"Cockroach":   {Provider: "CockroachDB", TableName: fullyQualified, Path: pathFull},
	"SQLite":      {Provider: "SQLite", TableName: nameOnly, Path: pathDatabaseTable, TableGroup: groupNone},
	"Exasol":      {Provider: "Exasol", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},

	// Cloud warehouses and lakehouses
	"Snowflake": {Provider: "Snowflake", TableName: fullyQualified, Path: pathFull, TableTypes: map[string]string{"External": "ExternalTable"}},
	"BigQuery":  {Provider: "BigQuery", TableName: nameOnly, Path: pathFull, TableGroup: groupSchema, TableGroupType: "Dataset", TableTypes: map[string]string{"External": "ExternalTable"}},
	"Redshift":  {Provider: "Redshift", TableName: fullyQualified, Path: pathFull, TableTypes: map[string]string{"External": "ExternalTable"}},
	// Athena keeps no catalog of its own: it reads the Glue Data
	// Catalog, which plugins/glue already catalogues under the Glue
	// provider, naming each table by its bare name. Naming it Athena
	// would file one table twice, once per route.
	"Athena":     {Provider: "Glue", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"Databricks": {Provider: "Databricks", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	// A UnityCatalog service is a Unity Catalog server, which is not a
	// Databricks workspace: it can run on its own, so plugins/unitycatalog
	// catalogues it under its own provider and both routes must agree.
	"UnityCatalog": {Provider: "Unity Catalog", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	"AzureSQL":     {Provider: "SQL Server", TableName: fullyQualified, Path: pathFull},
	"Synapse":      {Provider: "Azure Synapse", TableName: fullyQualified, Path: pathFull},
	"Iceberg":      {Provider: "Iceberg", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema, TableGroupType: "Namespace"},
	"DeltaLake":    {Provider: "Delta Lake", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	// plugins/hive names a table database.table under a Database named by
	// the Hive database; OpenMetadata's database level for Hive is a
	// placeholder, so it stays out of the name.
	"Hive":    {Provider: "Hive", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"Impala":  {Provider: "Impala", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"Trino":   {Provider: "Trino", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	"Presto":  {Provider: "Presto", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	"Dremio":  {Provider: "Dremio", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	"Glue":    {Provider: "Glue", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema},
	"Doris":   {Provider: "Doris", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"Druid":   {Provider: "Druid", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"PinotDB": {Provider: "Pinot", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},

	// Document, key-value and wide-column stores
	"MongoDB":   {Provider: "MongoDB", TableName: nameOnly, Path: pathSchemaTable, TableGroup: groupSchema, TableTypes: map[string]string{"Regular": "Collection"}},
	"DynamoDB":  {Provider: "DynamoDB", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	"Cassandra": {Provider: "Cassandra", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema, TableGroupType: "Keyspace"},
	"Couchbase": {Provider: "Couchbase", TableName: fullyQualified, Path: pathFull, TableGroupType: "Bucket", TableTypes: map[string]string{"Regular": "Collection"}},

	// SaaS and application sources
	"Salesforce":   {Provider: "Salesforce", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	"SAS":          {Provider: "SAS", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"DomoDatabase": {Provider: "Domo", TableName: nameOnly, Path: pathTable, TableGroup: groupNone, TableTypes: map[string]string{"Regular": "Dataset"}},

	// Search
	"ElasticSearch": {Provider: "Elasticsearch", IndexType: "Table"},
	"OpenSearch":    {Provider: "OpenSearch", IndexType: "Table"},

	// Messaging
	"Kafka":    {Provider: "Kafka"},
	"Redpanda": {Provider: "Kafka"},
	"Kinesis":  {Provider: "Kinesis", TopicType: "Stream"},

	// Drives
	"GoogleDrive": {Provider: "GoogleDrive"},
	"SharePoint":  {Provider: "SharePoint"},

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
	"Wherescape":         {Provider: "WhereScape"},
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

	// Technologies Marmot has no plugin for yet. Modelled as a native
	// plugin should model them, so that when one is written the two runs
	// land on the same asset rather than two.
	"Timescale":       {Provider: "PostgreSQL", TableName: nameOnly, Path: pathFull},
	"QuestDB":         {Provider: "QuestDB", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	"SapHana":         {Provider: "SAP HANA", TableName: fullyQualified, Path: pathFull},
	"SapErp":          {Provider: "SAP ERP", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	"StarRocks":       {Provider: "StarRocks", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema},
	"Iomete":          {Provider: "IOMETE", TableName: fullyQualified, Path: pathFull, TableGroupType: "Catalog"},
	"MicrosoftFabric": {Provider: "Microsoft Fabric", TableName: fullyQualified, Path: pathFull},
	// OpenMetadata puts the GCP project at the database level and the
	// instance at the schema level. The project stays out of the name for
	// the same reason it does in BigQuery: a container asset is always
	// named by its own bare level, so keeping the project in the table
	// name would leave the Instance asset named by the zone alone and
	// no longer a prefix of its tables. plugins/bigquery settles the
	// question for GCP by taking one required project_id and leaving the
	// project out of the MRN entirely.
	"BigTable": {Provider: "Bigtable", TableName: schemaQualified, Path: pathSchemaTable, TableGroup: groupSchema, TableGroupType: "Instance"},
	"BurstIQ":  {Provider: "BurstIQ", TableName: nameOnly, Path: pathTable, TableGroup: groupNone},
	// A data lake service reads files out of object storage, so its
	// objects are addressed bucket/key. groupNone keeps it from minting a
	// Bucket that would shadow the one the native S3 and GCS plugins make.
	"Datalake": {Provider: "Data Lake", TableName: bucketQualified, Path: pathBucketKey, TableGroup: groupNone},

	// Messaging and drives
	"PubSub": {Provider: "GooglePubSub"},
	"Sftp":   {Provider: "SFTP"},

	// Orchestration and transformation
	"Spark":                   {Provider: "Spark"},
	"OpenLineage":             {Provider: "OpenLineage"},
	"Snowplow":                {Provider: "Snowplow"},
	"KafkaConnect":            {Provider: "Kafka"},
	"KinesisFirehose":         {Provider: "Firehose"},
	"Mulesoft":                {Provider: "MuleSoft"},
	"DataFactory":             {Provider: "Azure Data Factory"},
	"MicrosoftFabricPipeline": {Provider: "Microsoft Fabric"},

	// Dashboards
	"Grafana": {Provider: "Grafana"},
	"Hex":     {Provider: "Hex"},
	"Ssrs":    {Provider: "SSRS"},
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
	if p.TopicType == "" {
		p.TopicType = "Topic"
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
