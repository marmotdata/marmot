package kafkaconnect

import (
	"net/url"
	"regexp"
	"strings"
)

// A connector's config says which database tables, collections or buckets
// it moves data between and which Kafka topics it uses, but every
// connector family spells that differently. A resolver reads one family's
// config and turns it into the dataset identities Marmot's own plugins
// give those objects, so the lineage lands on the same assets.

// dataset is one table, collection, bucket or container outside Kafka.
// Provider and Name follow the naming rule of the Marmot plugin that
// catalogues the technology, so the edge reaches the asset it made.
type dataset struct {
	Type     string
	Provider string
	Name     string
}

// resolver reads one connector family's config.
type resolver struct {
	// sources are the datasets a source connector reads from.
	sources func(cfg connectorConfig) []dataset
	// topics are the topics a source connector writes to, as its config
	// names them before any RegexRouter transform.
	topics func(cfg connectorConfig) []string
	// targets are the datasets a sink connector writes for the topics it
	// reads, after transforms.
	targets func(cfg connectorConfig, topics []string) []dataset
}

var resolvers = map[string]resolver{}

// register files a resolver under each class name and under the simple
// name after the last dot, so a Debezium class matches by its full name
// and a Confluent Cloud connector by its short one.
func register(r resolver, classes ...string) {
	for _, class := range classes {
		resolvers[class] = r
		resolvers[lastSegment(class)] = r
	}
}

// resolverFor returns the resolver for a connector class, matching the
// fully qualified name first and the simple name second.
func resolverFor(class string) (resolver, bool) {
	if r, ok := resolvers[class]; ok {
		return r, true
	}
	r, ok := resolvers[lastSegment(class)]
	return r, ok
}

func init() {
	register(debeziumResolver("PostgreSQL", "Table", "table.include.list", "table.whitelist", nameBare, debeziumTopicIdentifier),
		"io.debezium.connector.postgresql.PostgresConnector",
		"PostgresCdcSource", "PostgresCdcSourceV2", "PostgresSourceConnector")

	register(debeziumResolver("MySQL", "Table", "table.include.list", "table.whitelist", nameBare, debeziumTopicIdentifier),
		"io.debezium.connector.mysql.MySqlConnector",
		"MySqlCdcSource", "MySqlCdcSourceV2")

	register(debeziumResolver("SQL Server", "Table", "table.include.list", "table.whitelist", nameWithDatabase, sqlServerTopicIdentifier),
		"io.debezium.connector.sqlserver.SqlServerConnector",
		"SqlServerCdcSource", "SqlServerCdcSourceV2")

	register(debeziumResolver("MongoDB", "Collection", "collection.include.list", "collection.whitelist", nameBare, debeziumTopicIdentifier),
		"io.debezium.connector.mongodb.MongoDbConnector",
		"MongoDbCdcSource", "MongoDbCdcSourceV2")

	register(debeziumResolver("Oracle", "Table", "table.include.list", "table.whitelist", nameLastTwo, debeziumTopicIdentifier),
		"io.debezium.connector.oracle.OracleConnector",
		"OracleCdcSource")

	register(resolver{sources: jdbcSourceDatasets, topics: jdbcSourceTopics},
		"io.confluent.connect.jdbc.JdbcSourceConnector")

	register(resolver{targets: jdbcSinkTargets},
		"io.confluent.connect.jdbc.JdbcSinkConnector")

	register(resolver{targets: bucketTargets("S3", "Bucket", "s3.bucket.name")},
		"io.confluent.connect.s3.S3SinkConnector")

	register(resolver{targets: bucketTargets("GCS", "Bucket", "gcs.bucket.name")},
		"io.confluent.connect.gcs.GcsSinkConnector")

	register(resolver{targets: bucketTargets("AzureBlob", "Container", "azblob.container.name")},
		"io.confluent.connect.azure.blob.AzureBlobStorageSinkConnector")

	register(resolver{targets: snowflakeSinkTargets},
		"com.snowflake.kafka.connector.SnowflakeSinkConnector")

	register(resolver{targets: bigQuerySinkTargets},
		"com.wepay.kafka.connect.bigquery.BigQuerySinkConnector")

	register(resolver{targets: elasticsearchSinkTargets},
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector")

	register(resolver{targets: mongoSinkTargets},
		"com.mongodb.kafka.connect.MongoSinkConnector")

	register(resolver{sources: mongoSourceDatasets, topics: mongoSourceTopics},
		"com.mongodb.kafka.connect.MongoSourceConnector")
}

// Debezium

// debeziumPrefix is the topic prefix Debezium puts before every topic it
// writes. Debezium 2 calls it topic.prefix, Debezium 1 called it the
// server name.
func debeziumPrefix(cfg connectorConfig) string {
	return cfg.get("topic.prefix", "database.server.name")
}

// debeziumResolver builds the resolver for one Debezium connector. The
// include list names each captured object as a dotted identifier; name
// decides how much of it goes into the dataset name and topicID how it
// appears in the topic name.
func debeziumResolver(provider, assetType, includeKey, legacyKey string,
	name func(cfg connectorConfig, parts []string) string,
	topicID func(cfg connectorConfig, parts []string) string) resolver {
	return resolver{
		sources: func(cfg connectorConfig) []dataset {
			var out []dataset
			for _, entry := range cfg.list(includeKey, legacyKey) {
				id, ok := literalIdentifier(entry)
				if !ok {
					continue
				}
				if n := name(cfg, strings.Split(id, ".")); n != "" {
					out = append(out, dataset{Type: assetType, Provider: provider, Name: n})
				}
			}
			return out
		},
		topics: func(cfg connectorConfig) []string {
			prefix := debeziumPrefix(cfg)
			if prefix == "" {
				return nil
			}
			var out []string
			for _, entry := range cfg.list(includeKey, legacyKey) {
				id, ok := literalIdentifier(entry)
				if !ok {
					continue
				}
				if t := topicID(cfg, strings.Split(id, ".")); t != "" {
					out = append(out, prefix+"."+t)
				}
			}
			return out
		},
	}
}

// nameBare names the object by its last identifier part, the way the
// PostgreSQL, MySQL and MongoDB plugins do.
func nameBare(_ connectorConfig, parts []string) string {
	return parts[len(parts)-1]
}

// nameLastTwo names the object schema.table, the way the Oracle plugin
// does.
func nameLastTwo(_ connectorConfig, parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// nameWithDatabase names the object database.schema.table, the way the
// SQL Server plugin does. Debezium keeps the database in its own setting
// and lists tables as schema.table.
func nameWithDatabase(cfg connectorConfig, parts []string) string {
	db := debeziumDatabase(cfg)
	if db == "" || len(parts) < 2 {
		return ""
	}
	return db + "." + strings.Join(parts[len(parts)-2:], ".")
}

func debeziumDatabase(cfg connectorConfig) string {
	if names := cfg.list("database.names"); len(names) > 0 {
		return names[0]
	}
	return cfg.get("database.dbname")
}

// debeziumTopicIdentifier is the include-list entry as written, which is
// how most Debezium connectors spell the topic below the prefix.
func debeziumTopicIdentifier(_ connectorConfig, parts []string) string {
	return strings.Join(parts, ".")
}

// sqlServerTopicIdentifier adds the database, which Debezium's SQL Server
// connector puts in the topic name but not in the include list.
func sqlServerTopicIdentifier(cfg connectorConfig, parts []string) string {
	db := debeziumDatabase(cfg)
	if db == "" || len(parts) < 2 {
		return ""
	}
	return db + "." + strings.Join(parts[len(parts)-2:], ".")
}

// JDBC

// jdbcDialect is how one database behind a JDBC URL names its tables in
// Marmot.
type jdbcDialect struct {
	provider string
	// name builds the dataset name from the dotted parts of a table
	// identifier, using the connection for whatever the identifier lacks.
	name func(conn jdbcURL, cfg connectorConfig, parts []string) string
}

// jdbcURL is the part of a JDBC connection URL the dialects read.
type jdbcURL struct {
	scheme   string
	database string
	params   map[string]string
}

var jdbcDialects = map[string]jdbcDialect{
	"postgresql": {provider: "PostgreSQL", name: jdbcBare},
	"mysql":      {provider: "MySQL", name: jdbcBare},
	"mariadb":    {provider: "MariaDB", name: jdbcBare},
	"sqlserver":  {provider: "SQL Server", name: jdbcQualified("dbo")},
	"oracle":     {provider: "Oracle", name: jdbcOracle},
	"snowflake":  {provider: "Snowflake", name: jdbcQualified("PUBLIC")},
	"redshift":   {provider: "Redshift", name: jdbcQualified("public")},
}

func jdbcBare(_ jdbcURL, _ connectorConfig, parts []string) string {
	return parts[len(parts)-1]
}

// jdbcQualified names a table database.schema.table, filling the schema
// from the identifier or the default and the database from the
// connection.
func jdbcQualified(defaultSchema string) func(jdbcURL, connectorConfig, []string) string {
	return func(conn jdbcURL, _ connectorConfig, parts []string) string {
		table := parts[len(parts)-1]
		schema := defaultSchema
		if len(parts) >= 2 {
			schema = parts[len(parts)-2]
		}
		db := conn.database
		if len(parts) >= 3 {
			db = parts[len(parts)-3]
		}
		if db == "" {
			return ""
		}
		return db + "." + schema + "." + table
	}
}

// jdbcOracle names a table schema.table. A bare table lives in the
// connecting user's schema, which Oracle stores in upper case.
func jdbcOracle(_ jdbcURL, cfg connectorConfig, parts []string) string {
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	user := cfg.get("connection.user")
	if user == "" {
		return ""
	}
	return strings.ToUpper(user) + "." + parts[0]
}

// parseJDBCURL reads the scheme, database and parameters out of the URL
// forms the supported drivers use:
//
//	jdbc:postgresql://host:5432/db?sslmode=require
//	jdbc:sqlserver://host:1433;databaseName=db;encrypt=true
//	jdbc:oracle:thin:@//host:1521/service
//	jdbc:snowflake://acct.snowflakecomputing.com/?db=DB&schema=S
func parseJDBCURL(raw string) (jdbcURL, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "jdbc:") {
		return jdbcURL{}, false
	}
	rest := raw[len("jdbc:"):]
	scheme, tail, ok := strings.Cut(rest, ":")
	if !ok {
		return jdbcURL{}, false
	}
	conn := jdbcURL{scheme: strings.ToLower(scheme), params: map[string]string{}}

	// SQL Server separates its settings with semicolons.
	if conn.scheme == "sqlserver" {
		for i, part := range strings.Split(tail, ";") {
			if i == 0 {
				continue
			}
			if key, value, ok := strings.Cut(part, "="); ok {
				conn.params[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
		conn.database = conn.params["databasename"]
		if conn.database == "" {
			conn.database = conn.params["database"]
		}
		return conn, true
	}

	// The rest are URL shaped once the driver-specific lead-in is gone.
	tail = strings.TrimPrefix(tail, "thin:@")
	tail = strings.TrimPrefix(tail, "@")
	if !strings.HasPrefix(tail, "//") {
		tail = "//" + tail
	}
	parsed, err := url.Parse("jdbc:" + tail)
	if err != nil {
		return conn, true
	}
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			conn.params[strings.ToLower(key)] = values[0]
		}
	}
	conn.database = strings.Trim(parsed.Path, "/")
	if conn.scheme == "snowflake" {
		conn.database = conn.params["db"]
	}
	if conn.scheme == "oracle" {
		// The path is a service name, not a schema, so it is no use here.
		conn.database = ""
	}
	return conn, true
}

// jdbcDialectFor picks the dialect for a connector's connection.url.
func jdbcDialectFor(cfg connectorConfig) (jdbcDialect, jdbcURL, bool) {
	conn, ok := parseJDBCURL(cfg.get("connection.url"))
	if !ok {
		return jdbcDialect{}, conn, false
	}
	dialect, ok := jdbcDialects[conn.scheme]
	return dialect, conn, ok
}

// jdbcTable names one table for the dialect, returning the Snowflake
// schema from the URL when the identifier does not carry one.
func jdbcTable(dialect jdbcDialect, conn jdbcURL, cfg connectorConfig, identifier string) (dataset, bool) {
	parts := strings.Split(identifier, ".")
	if conn.scheme == "snowflake" && len(parts) == 1 && conn.params["schema"] != "" {
		parts = []string{conn.params["schema"], parts[0]}
	}
	name := dialect.name(conn, cfg, parts)
	if name == "" {
		return dataset{}, false
	}
	return dataset{Type: "Table", Provider: dialect.provider, Name: name}, true
}

func jdbcSourceTables(cfg connectorConfig) []string {
	return cfg.list("table.whitelist", "tables", "table.include.list")
}

func jdbcSourceDatasets(cfg connectorConfig) []dataset {
	dialect, conn, ok := jdbcDialectFor(cfg)
	if !ok {
		return nil
	}
	var out []dataset
	for _, table := range jdbcSourceTables(cfg) {
		if d, ok := jdbcTable(dialect, conn, cfg, table); ok {
			out = append(out, d)
		}
	}
	return out
}

// jdbcSourceTopics names the topics the JDBC source writes: the prefix
// followed by the bare table name, or in query mode the prefix alone.
func jdbcSourceTopics(cfg connectorConfig) []string {
	prefix := cfg.get("topic.prefix")
	tables := jdbcSourceTables(cfg)
	if len(tables) == 0 {
		if cfg.get("query") != "" && prefix != "" {
			return []string{prefix}
		}
		return nil
	}
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		out = append(out, prefix+lastSegment(table))
	}
	return out
}

// jdbcSinkTargets names the table each topic lands in. table.name.format
// defaults to the topic name and may qualify it with a schema.
func jdbcSinkTargets(cfg connectorConfig, topics []string) []dataset {
	dialect, conn, ok := jdbcDialectFor(cfg)
	if !ok {
		return nil
	}
	format := cfg.get("table.name.format")
	if format == "" {
		format = "${topic}"
	}
	var out []dataset
	for _, topic := range topics {
		table := strings.ReplaceAll(format, "${topic}", topic)
		if d, ok := jdbcTable(dialect, conn, cfg, table); ok {
			out = append(out, d)
		}
	}
	return out
}

// Object storage

// bucketTargets makes every topic of a storage sink land in its one
// bucket or container.
func bucketTargets(provider, assetType, key string) func(connectorConfig, []string) []dataset {
	return func(cfg connectorConfig, _ []string) []dataset {
		name := cfg.get(key)
		if name == "" {
			return nil
		}
		return []dataset{{Type: assetType, Provider: provider, Name: name}}
	}
}

// Warehouses and search

func snowflakeSinkTargets(cfg connectorConfig, topics []string) []dataset {
	db := cfg.get("snowflake.database.name")
	schema := cfg.get("snowflake.schema.name")
	if db == "" || schema == "" {
		return nil
	}
	tableFor := pairs(cfg.get("snowflake.topic2table.map"))
	var out []dataset
	for _, topic := range topics {
		table := tableFor[topic]
		if table == "" {
			table = topic
		}
		out = append(out, dataset{Type: "Table", Provider: "Snowflake", Name: db + "." + schema + "." + table})
	}
	return out
}

var bigQueryUnsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)

func bigQuerySinkTargets(cfg connectorConfig, topics []string) []dataset {
	tableFor := pairs(cfg.get("topic2TableMap"))
	sanitise := cfg.isTrue("sanitizeTopics")
	var out []dataset
	for _, topic := range topics {
		table := tableFor[topic]
		if table == "" {
			table = topic
			if sanitise {
				table = bigQueryUnsafe.ReplaceAllString(table, "_")
			}
		}
		out = append(out, dataset{Type: "Table", Provider: "BigQuery", Name: table})
	}
	return out
}

// elasticsearchSinkTargets names the index each topic is written to.
// Marmot's Elasticsearch plugin catalogues an index as a Table.
func elasticsearchSinkTargets(_ connectorConfig, topics []string) []dataset {
	var out []dataset
	for _, topic := range topics {
		out = append(out, dataset{Type: "Table", Provider: "Elasticsearch", Name: topic})
	}
	return out
}

// MongoDB

func mongoSinkTargets(cfg connectorConfig, topics []string) []dataset {
	if collection := cfg.get("collection"); collection != "" {
		return []dataset{{Type: "Collection", Provider: "MongoDB", Name: collection}}
	}
	var out []dataset
	for _, topic := range topics {
		out = append(out, dataset{Type: "Collection", Provider: "MongoDB", Name: topic})
	}
	return out
}

func mongoSourceDatasets(cfg connectorConfig) []dataset {
	collection := cfg.get("collection")
	if collection == "" {
		return nil
	}
	return []dataset{{Type: "Collection", Provider: "MongoDB", Name: collection}}
}

// mongoSourceTopics names the topic the MongoDB source connector writes
// to: the database and collection, behind the prefix when one is set.
func mongoSourceTopics(cfg connectorConfig) []string {
	db, collection := cfg.get("database"), cfg.get("collection")
	if db == "" || collection == "" {
		return nil
	}
	topic := db + "." + collection
	if prefix := cfg.get("topic.prefix"); prefix != "" {
		topic = prefix + "." + topic
	}
	return []string{topic}
}
