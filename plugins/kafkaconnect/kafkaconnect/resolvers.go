package kafkaconnect

import (
	"net"
	"net/url"
	"regexp"
	"strings"
)

// A connector's config says which database tables, collections or buckets
// it moves data between and which Kafka topics it uses, but every
// connector family spells that differently. A resolver reads one family's
// config and turns it into the dataset identities Marmot's own plugins
// give those objects, so the lineage lands on the same assets.

// dataset is one table, collection, bucket or container outside Kafka. Provider and Path follow the naming table of the Marmot plugin that catalogues the technology, so the edge reaches the asset it made. Host is the host:port the connector was configured with, empty when the config names no server; Marmot resolves it to the zone that owns the address.
type dataset struct {
	Type     string
	Provider string
	Path     []string
	Host     string
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
	register(debeziumResolver("PostgreSQL", "Table", "table.include.list", "table.whitelist", "5432", pathPostgres, debeziumTopicIdentifier),
		"io.debezium.connector.postgresql.PostgresConnector",
		"PostgresCdcSource", "PostgresCdcSourceV2", "PostgresSourceConnector")

	register(debeziumResolver("MySQL", "Table", "table.include.list", "table.whitelist", "3306", pathMySQL, debeziumTopicIdentifier),
		"io.debezium.connector.mysql.MySqlConnector",
		"MySqlCdcSource", "MySqlCdcSourceV2")

	register(debeziumResolver("SQL Server", "Table", "table.include.list", "table.whitelist", "1433", pathSQLServer, sqlServerTopicIdentifier),
		"io.debezium.connector.sqlserver.SqlServerConnector",
		"SqlServerCdcSource", "SqlServerCdcSourceV2")

	register(debeziumResolver("MongoDB", "Collection", "collection.include.list", "collection.whitelist", "27017", pathMongo, debeziumTopicIdentifier),
		"io.debezium.connector.mongodb.MongoDbConnector",
		"MongoDbCdcSource", "MongoDbCdcSourceV2")

	register(debeziumResolver("Oracle", "Table", "table.include.list", "table.whitelist", "1521", pathOracle, debeziumTopicIdentifier),
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

// debeziumResolver builds the resolver for one Debezium connector. The include list names each captured object as a dotted identifier; path decides how the identifier and the connector's database settings make up the dataset path and topicID how it appears in the topic name. defaultPort fills in a connector that names a host without a port.
func debeziumResolver(provider, assetType, includeKey, legacyKey, defaultPort string,
	path func(cfg connectorConfig, parts []string) []string,
	topicID func(cfg connectorConfig, parts []string) string) resolver {
	return resolver{
		sources: func(cfg connectorConfig) []dataset {
			host := debeziumHost(cfg, defaultPort)
			var out []dataset
			for _, entry := range cfg.list(includeKey, legacyKey) {
				id, ok := literalIdentifier(entry)
				if !ok {
					continue
				}
				if p := path(cfg, strings.Split(id, ".")); len(p) > 0 {
					out = append(out, dataset{Type: assetType, Provider: provider, Path: p, Host: host})
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

// debeziumHost is the database server a Debezium connector reads, as host:port.
func debeziumHost(cfg connectorConfig, defaultPort string) string {
	if host := cfg.get("database.hostname", "hostname"); host != "" {
		return hostPort(host, cfg.get("database.port", "port"), defaultPort)
	}
	// Debezium MongoDB names its servers in a connection string or a replica set list instead.
	if conn := cfg.get("mongodb.connection.string"); conn != "" {
		return mongoHost(conn)
	}
	if hosts := cfg.get("mongodb.hosts"); hosts != "" {
		_, list, found := strings.Cut(hosts, "/")
		if !found {
			list = hosts
		}
		first, _, _ := strings.Cut(list, ",")
		return hostPort(first, "", defaultPort)
	}
	return ""
}

// hostPort joins a host and port into a lowercase host:port, taking a port already on the host first and the default last. It returns "" for an empty host.
func hostPort(host, port, defaultPort string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		return net.JoinHostPort(h, p)
	}
	host = strings.Trim(host, "[]")
	if port = strings.TrimSpace(port); port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port)
}

// mongoHost reads the first host of a mongodb:// or mongodb+srv:// connection string. The default port applies unless the string names one.
func mongoHost(conn string) string {
	rest := conn
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		rest = rest[at+1:]
	}
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	first, _, _ := strings.Cut(rest, ",")
	return hostPort(first, "", "27017")
}

// pathPostgres is database.schema.table, with the database taken from the connector because Debezium lists tables as schema.table.
func pathPostgres(cfg connectorConfig, parts []string) []string {
	db := debeziumDatabase(cfg)
	if db == "" || len(parts) < 2 {
		return nil
	}
	return []string{db, parts[len(parts)-2], parts[len(parts)-1]}
}

// pathMySQL is database.table. Debezium MySQL lists tables as database.table.
func pathMySQL(_ connectorConfig, parts []string) []string {
	if len(parts) < 2 {
		return nil
	}
	return parts[len(parts)-2:]
}

// pathSQLServer is database.schema.table, with the database taken from the connector.
func pathSQLServer(cfg connectorConfig, parts []string) []string {
	return pathPostgres(cfg, parts)
}

// pathMongo is database.collection.
func pathMongo(_ connectorConfig, parts []string) []string {
	if len(parts) < 2 {
		return nil
	}
	return parts[len(parts)-2:]
}

// pathOracle is service.schema.table. The service is the pluggable database when the connector names one and the container database otherwise.
func pathOracle(cfg connectorConfig, parts []string) []string {
	service := cfg.get("database.pdb.name", "database.dbname")
	if service == "" || len(parts) < 2 {
		return nil
	}
	return []string{service, parts[len(parts)-2], parts[len(parts)-1]}
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

// jdbcDialect is how one database behind a JDBC URL names its tables in Marmot.
type jdbcDialect struct {
	provider string
	// defaultPort is the port the driver uses when the URL names none. It is empty for hosted warehouses, whose address says nothing a Marmot plugin could match.
	defaultPort string
	// path builds the dataset path from the dotted parts of a table identifier, using the connection for whatever the identifier lacks.
	path func(conn jdbcURL, cfg connectorConfig, parts []string) []string
}

// jdbcURL is the part of a JDBC connection URL the dialects read.
type jdbcURL struct {
	scheme   string
	host     string
	port     string
	database string
	params   map[string]string
}

var jdbcDialects = map[string]jdbcDialect{
	"postgresql": {provider: "PostgreSQL", defaultPort: "5432", path: jdbcQualified("public")},
	"mysql":      {provider: "MySQL", defaultPort: "3306", path: jdbcDatabaseTable},
	"mariadb":    {provider: "MariaDB", defaultPort: "3306", path: jdbcDatabaseTable},
	"sqlserver":  {provider: "SQL Server", defaultPort: "1433", path: jdbcQualified("dbo")},
	"oracle":     {provider: "Oracle", defaultPort: "1521", path: jdbcOracle},
	"snowflake":  {provider: "Snowflake", path: jdbcQualified("PUBLIC")},
	"redshift":   {provider: "Redshift", defaultPort: "5439", path: jdbcQualified("public")},
}

// jdbcDatabaseTable names a table database.table, filling the database from the connection when the identifier is bare.
func jdbcDatabaseTable(conn jdbcURL, _ connectorConfig, parts []string) []string {
	table := parts[len(parts)-1]
	db := conn.database
	if len(parts) >= 2 {
		db = parts[len(parts)-2]
	}
	if db == "" {
		return nil
	}
	return []string{db, table}
}

// jdbcQualified names a table database.schema.table, filling the schema from the identifier or the default and the database from the connection.
func jdbcQualified(defaultSchema string) func(jdbcURL, connectorConfig, []string) []string {
	return func(conn jdbcURL, _ connectorConfig, parts []string) []string {
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
			return nil
		}
		return []string{db, schema, table}
	}
}

// jdbcOracle names a table service.schema.table. A bare table lives in the connecting user's schema, which Oracle stores in upper case, and the service is the one the URL connects to.
func jdbcOracle(conn jdbcURL, cfg connectorConfig, parts []string) []string {
	if conn.database == "" {
		return nil
	}
	if len(parts) >= 2 {
		return []string{conn.database, parts[len(parts)-2], parts[len(parts)-1]}
	}
	user := cfg.get("connection.user")
	if user == "" {
		return nil
	}
	return []string{conn.database, strings.ToUpper(user), parts[0]}
}

// parseJDBCURL reads the scheme, server, database and parameters out of the URL forms the supported drivers use:
//
//	jdbc:postgresql://host:5432/db?sslmode=require
//	jdbc:sqlserver://host:1433;databaseName=db;encrypt=true
//	jdbc:oracle:thin:@//host:1521/service
//	jdbc:oracle:thin:@host:1521:SID
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
				conn.host, conn.port = splitHostPort(strings.TrimPrefix(part, "//"))
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

	// Oracle's SID form is host:port:SID, which no URL parser reads.
	if conn.scheme == "oracle" {
		if lead, sid, ok := oracleSID(tail); ok {
			conn.host, conn.port = splitHostPort(lead)
			conn.database = sid
			return conn, true
		}
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
	conn.host, conn.port = parsed.Hostname(), parsed.Port()
	conn.database = strings.Trim(parsed.Path, "/")
	if conn.scheme == "snowflake" {
		conn.database = conn.params["db"]
	}
	return conn, true
}

// oracleSID reads the host:port:SID tail of a thin URL that has no slash.
func oracleSID(tail string) (string, string, bool) {
	tail = strings.TrimPrefix(tail, "thin:@")
	tail = strings.TrimPrefix(tail, "@")
	if strings.Contains(tail, "/") || strings.HasPrefix(tail, "(") {
		return "", "", false
	}
	parts := strings.Split(tail, ":")
	if len(parts) != 3 {
		return "", "", false
	}
	return parts[0] + ":" + parts[1], parts[2], true
}

// splitHostPort splits host and port, which may be missing, from an address that may carry an instance name after a backslash.
func splitHostPort(address string) (string, string) {
	address, _, _ = strings.Cut(strings.TrimSpace(address), `\`)
	if h, p, err := net.SplitHostPort(address); err == nil {
		return h, p
	}
	return strings.Trim(address, "[]"), ""
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

// jdbcTable names one table for the dialect, returning the Snowflake schema from the URL when the identifier does not carry one.
func jdbcTable(dialect jdbcDialect, conn jdbcURL, cfg connectorConfig, identifier string) (dataset, bool) {
	parts := strings.Split(identifier, ".")
	if conn.scheme == "snowflake" && len(parts) == 1 && conn.params["schema"] != "" {
		parts = []string{conn.params["schema"], parts[0]}
	}
	path := dialect.path(conn, cfg, parts)
	if len(path) == 0 {
		return dataset{}, false
	}
	d := dataset{Type: "Table", Provider: dialect.provider, Path: path}
	if dialect.defaultPort != "" {
		d.Host = hostPort(conn.host, conn.port, dialect.defaultPort)
	}
	return d, true
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

// bucketTargets makes every topic of a storage sink land in its one bucket or container.
func bucketTargets(provider, assetType, key string) func(connectorConfig, []string) []dataset {
	return func(cfg connectorConfig, _ []string) []dataset {
		name := cfg.get(key)
		if name == "" {
			return nil
		}
		return []dataset{{Type: assetType, Provider: provider, Path: []string{name}}}
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
		out = append(out, dataset{Type: "Table", Provider: "Snowflake", Path: []string{db, schema, table}})
	}
	return out
}

var bigQueryUnsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// bigQuerySinkTargets names each table project.dataset.table. A connector that routes topics to datasets by pattern names no single dataset, so it resolves to nothing.
func bigQuerySinkTargets(cfg connectorConfig, topics []string) []dataset {
	project := cfg.get("project")
	bqDataset := cfg.get("defaultDataset")
	if project == "" || bqDataset == "" {
		return nil
	}
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
		out = append(out, dataset{Type: "Table", Provider: "BigQuery", Path: []string{project, bqDataset, table}})
	}
	return out
}

// elasticsearchSinkTargets names the index each topic is written to. Marmot's Elasticsearch plugin catalogues an index as a Table.
func elasticsearchSinkTargets(cfg connectorConfig, topics []string) []dataset {
	host := ""
	if first := cfg.list("connection.url"); len(first) > 0 {
		host = urlHost(first[0], "9200")
	}
	var out []dataset
	for _, topic := range topics {
		out = append(out, dataset{Type: "Table", Provider: "Elasticsearch", Path: []string{topic}, Host: host})
	}
	return out
}

// urlHost reads the host:port of a URL, using the default port when it names none.
func urlHost(raw, defaultPort string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return hostPort(u.Hostname(), u.Port(), defaultPort)
}

// MongoDB

// mongoSinkTargets names each collection database.collection. The sink writes to the configured collection, or to one named after each topic.
func mongoSinkTargets(cfg connectorConfig, topics []string) []dataset {
	db := cfg.get("database")
	if db == "" {
		return nil
	}
	host := mongoHost(cfg.get("connection.uri"))
	if collection := cfg.get("collection"); collection != "" {
		return []dataset{{Type: "Collection", Provider: "MongoDB", Path: []string{db, collection}, Host: host}}
	}
	var out []dataset
	for _, topic := range topics {
		out = append(out, dataset{Type: "Collection", Provider: "MongoDB", Path: []string{db, topic}, Host: host})
	}
	return out
}

func mongoSourceDatasets(cfg connectorConfig) []dataset {
	db, collection := cfg.get("database"), cfg.get("collection")
	if db == "" || collection == "" {
		return nil
	}
	return []dataset{{Type: "Collection", Provider: "MongoDB", Path: []string{db, collection}, Host: mongoHost(cfg.get("connection.uri"))}}
}

// mongoSourceTopics names the topic the MongoDB source connector writes to: the database and collection, behind the prefix when one is set.
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
