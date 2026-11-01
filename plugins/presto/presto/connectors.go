package presto

// connectorInfo describes how the tables of one Presto connector map onto
// the provider and name shape of the Marmot plugin that owns that
// technology, so both runs land on one asset instead of two.
type connectorInfo struct {
	Provider string
	// MRNName builds the asset Name (and so the MRN's name segment) from
	// the Presto path, in the shape the native plugin uses.
	MRNName func(catalog, schema, table string) string
}

// defaultMRNName returns catalog.schema.table, the shape for connectors
// without a native Marmot plugin to agree with.
func defaultMRNName(catalog, schema, table string) string {
	return catalog + "." + schema + "." + table
}

// prestoNativeConnector is the mapping for data that exists only inside
// Presto: its tables belong to Presto itself under their full path.
var prestoNativeConnector = connectorInfo{Provider: provider, MRNName: defaultMRNName}

// internalConnectors lists connectors that hold Presto-native data rather
// than a view onto an external system. Trino skips their catalogs; here
// they are discovered under the Presto provider because a memory or tpch
// table has no other home.
var internalConnectors = map[string]bool{
	"memory":    true,
	"tpch":      true,
	"tpcds":     true,
	"blackhole": true,
	"localfile": true,
}

// connectorMap is copied from the Trino plugin so the two catalogue a
// connector's tables identically. Presto spells a few connectors
// differently from Trino; those rows sit at the end.
var connectorMap = map[string]connectorInfo{
	// Relational databases
	"postgresql":  {Provider: "PostgreSQL", MRNName: func(_, _, table string) string { return table }},
	"mysql":       {Provider: "MySQL", MRNName: func(_, _, table string) string { return table }},
	"mariadb":     {Provider: "MariaDB", MRNName: func(_, _, table string) string { return table }},
	"sqlserver":   {Provider: "SQL Server", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"oracle":      {Provider: "Oracle", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"clickhouse":  {Provider: "ClickHouse", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"singlestore": {Provider: "SingleStore", MRNName: func(_, _, table string) string { return table }},
	"redshift":    {Provider: "Redshift", MRNName: func(_, schema, table string) string { return schema + "." + table }},

	// Cloud warehouses / lakehouses
	"snowflake":  {Provider: "Snowflake", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"bigquery":   {Provider: "BigQuery", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"iceberg":    {Provider: "Iceberg", MRNName: defaultMRNName},
	"delta_lake": {Provider: "Delta Lake", MRNName: defaultMRNName},
	"hive":       {Provider: "Hive", MRNName: defaultMRNName},
	"hudi":       {Provider: "Hudi", MRNName: defaultMRNName},

	// NoSQL / document / key-value
	"mongodb":   {Provider: "MongoDB", MRNName: func(_, _, table string) string { return table }},
	"cassandra": {Provider: "Cassandra", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"redis":     {Provider: "Redis", MRNName: func(_, _, table string) string { return table }},
	"accumulo":  {Provider: "Accumulo", MRNName: func(_, schema, table string) string { return schema + "." + table }},

	// Search / analytics engines
	"elasticsearch": {Provider: "Elasticsearch", MRNName: func(_, _, table string) string { return table }},
	"opensearch":    {Provider: "OpenSearch", MRNName: func(_, _, table string) string { return table }},
	"druid":         {Provider: "Druid", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"pinot":         {Provider: "Pinot", MRNName: func(_, _, table string) string { return table }},

	// Streaming
	"kafka":   {Provider: "Kafka", MRNName: func(_, _, table string) string { return table }},
	"kinesis": {Provider: "Kinesis", MRNName: func(_, _, table string) string { return table }},

	// Other
	"prometheus":    {Provider: "Prometheus", MRNName: func(_, _, table string) string { return table }},
	"google_sheets": {Provider: "Google Sheets", MRNName: func(_, _, table string) string { return table }},
	"phoenix":       {Provider: "Phoenix", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"ignite":        {Provider: "Ignite", MRNName: func(_, schema, table string) string { return schema + "." + table }},
	"kudu":          {Provider: "Kudu", MRNName: func(_, schema, table string) string { return schema + "." + table }},

	// Presto names these connectors differently from Trino.
	"hive-hadoop2": {Provider: "Hive", MRNName: defaultMRNName},
	"delta":        {Provider: "Delta Lake", MRNName: defaultMRNName},
}

// connectorInfoForName picks the mapping for a catalog's connector.
// Internal connectors and connectors no Marmot plugin covers both fall to
// Presto itself: there is no native identity to merge with.
func connectorInfoForName(connector string) connectorInfo {
	if internalConnectors[connector] {
		return prestoNativeConnector
	}
	if info, ok := connectorMap[connector]; ok {
		return info
	}
	return prestoNativeConnector
}
