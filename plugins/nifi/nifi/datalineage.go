package nifi

import (
	"context"
	"net"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// flowDirection says which way data moves between a processor and the
// object its property names.
type flowDirection int

const (
	// readsFrom: the object feeds the processor (object FEEDS task).
	readsFrom flowDirection = iota
	// writesTo: the processor writes the object (task PRODUCES object).
	writesTo
)

// processorRule describes how one processor type names the object it
// reads or writes. Properties are the property names to try in order,
// which covers the renames between NiFi 1 and NiFi 2.
type processorRule struct {
	Properties []string
	Provider   string
	AssetType  string
	Direction  flowDirection
	// List means the value is a comma-separated list of names.
	List bool
	// JDBC means the provider and name shape come from the connection
	// pool's JDBC URL rather than from this table.
	JDBC bool
}

// processorRules maps a processor's simple class name, with any version
// suffix removed, to the object it touches. Processors whose target is
// only known from a SQL statement (PutSQL, ExecuteSQL) have no rule.
var processorRules = map[string]processorRule{
	"PutS3Object":   {Properties: []string{"Bucket"}, Provider: "S3", AssetType: "Bucket", Direction: writesTo},
	"FetchS3Object": {Properties: []string{"Bucket"}, Provider: "S3", AssetType: "Bucket", Direction: readsFrom},
	"ListS3":        {Properties: []string{"Bucket"}, Provider: "S3", AssetType: "Bucket", Direction: readsFrom},

	"PutGCSObject":   {Properties: []string{"Bucket"}, Provider: "GCS", AssetType: "Bucket", Direction: writesTo},
	"FetchGCSObject": {Properties: []string{"Bucket"}, Provider: "GCS", AssetType: "Bucket", Direction: readsFrom},
	"ListGCSBucket":  {Properties: []string{"Bucket"}, Provider: "GCS", AssetType: "Bucket", Direction: readsFrom},

	"PutAzureBlobStorage":   {Properties: []string{"Container Name"}, Provider: "AzureBlob", AssetType: "Container", Direction: writesTo},
	"FetchAzureBlobStorage": {Properties: []string{"Container Name"}, Provider: "AzureBlob", AssetType: "Container", Direction: readsFrom},

	"PublishKafka":       {Properties: []string{"Topic Name", "topic"}, Provider: "Kafka", AssetType: "Topic", Direction: writesTo},
	"PublishKafkaRecord": {Properties: []string{"Topic Name", "topic"}, Provider: "Kafka", AssetType: "Topic", Direction: writesTo},
	"ConsumeKafka":       {Properties: []string{"Topics", "Topic Name(s)", "topic"}, Provider: "Kafka", AssetType: "Topic", Direction: readsFrom, List: true},
	"ConsumeKafkaRecord": {Properties: []string{"Topics", "Topic Name(s)", "topic"}, Provider: "Kafka", AssetType: "Topic", Direction: readsFrom, List: true},

	"PutDatabaseRecord":        {Properties: []string{"Table Name"}, AssetType: "Table", Direction: writesTo, JDBC: true},
	"QueryDatabaseTable":       {Properties: []string{"Table Name"}, AssetType: "Table", Direction: readsFrom, JDBC: true},
	"QueryDatabaseTableRecord": {Properties: []string{"Table Name"}, AssetType: "Table", Direction: readsFrom, JDBC: true},
	"GenerateTableFetch":       {Properties: []string{"Table Name"}, AssetType: "Table", Direction: readsFrom, JDBC: true},

	"PutElasticsearchRecord": {Properties: []string{"Index"}, Provider: "Elasticsearch", AssetType: "Table", Direction: writesTo},
	"PutElasticsearchJson":   {Properties: []string{"Index"}, Provider: "Elasticsearch", AssetType: "Table", Direction: writesTo},
}

// versionSuffix matches the client version NiFi 1 appended to Kafka
// processors (ConsumeKafka_2_6) and the _v12 of the Azure processors.
var versionSuffix = regexp.MustCompile(`(_\d+_\d+|_v\d+)$`)

// ruleFor finds the rule for a processor's full Java class name.
func ruleFor(processorType string) (processorRule, bool) {
	base := versionSuffix.ReplaceAllString(simpleTypeName(processorType), "")
	rule, ok := processorRules[base]
	return rule, ok
}

// poolProperty names the controller service a database processor reads
// its connection from.
const poolProperty = "Database Connection Pooling Service"

// dataLineage links a processor to the object it reads or writes, when
// its type is in the rule table and the naming property holds a literal
// value. Expression language and parameter references are skipped: the
// value is only known at run time.
func (d *discovery) dataLineage(ctx context.Context, p processorComponent, taskMRN, taskName string) {
	rule, ok := ruleFor(p.Type)
	if !ok {
		return
	}

	value := propertyValue(p.Config, rule.Properties...)
	if value == "" {
		return
	}

	// ConsumeKafka can subscribe by regular expression, which names no
	// topic in particular.
	if rule.List && strings.EqualFold(propertyValue(p.Config, "Topic Format", "topic_type"), "pattern") {
		return
	}

	names := []string{value}
	if rule.List {
		names = splitList(value)
	}

	nativeProvider := rule.Provider
	var host string
	var jdbcPath []string
	switch {
	case rule.JDBC:
		nativeProvider, jdbcPath, host = d.jdbcTable(ctx, p, value)
		if nativeProvider == "" {
			return
		}
		names = []string{value}
	case nativeProvider == "Kafka":
		host = d.kafkaHost(ctx, p)
	}

	for _, name := range names {
		path := []string{name}
		if rule.JDBC {
			path = jdbcPath
		}
		target := nativeMRN(rule.AssetType, nativeProvider, host, path...)
		if rule.Direction == writesTo {
			d.link(taskMRN, target, "PRODUCES")
		} else {
			d.link(target, taskMRN, "FEEDS")
		}

		if nativeProvider == "Kafka" {
			d.recordTopic(target, name, taskName, rule.Direction)
		}
	}
}

func (d *discovery) recordTopic(topicMRN, topic, taskName string, direction flowDirection) {
	usage, ok := d.topics[topicMRN]
	if !ok {
		usage = &topicUsage{name: topic}
		d.topics[topicMRN] = usage
	}
	if direction == writesTo {
		usage.producers = appendUnique(usage.producers, taskName)
	} else {
		usage.consumers = appendUnique(usage.consumers, taskName)
	}
}

// kafkaHost is the first bootstrap server a Kafka processor connects to as host:port, read from the processor or from the connection service NiFi 2 moved it to. It returns "" when neither names one.
func (d *discovery) kafkaHost(ctx context.Context, p processorComponent) string {
	servers := propertyValue(p.Config, "bootstrap.servers", "Kafka Brokers")
	if servers == "" {
		if id := propertyValue(p.Config, "Kafka Connection Service"); id != "" {
			if svc := d.connectionPool(ctx, id); svc != nil {
				servers = stringProperty(svc.Properties, "bootstrap.servers")
			}
		}
	}
	first, _, _ := strings.Cut(servers, ",")
	first = strings.TrimSpace(first)
	if _, rest, ok := strings.Cut(first, "://"); ok {
		first = rest
	}
	return hostWithDefault(first, "9092")
}

// hostWithDefault lowercases an address and adds the port when it has none.
func hostWithDefault(address, defaultPort string) string {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(strings.Trim(address, "[]"), defaultPort)
}

// jdbcTable resolves the provider, path and host:port of the table a database processor touches from the JDBC URL of its connection pool. It returns an empty provider when the pool cannot be read, the database kind is not one Marmot has a plugin for, or the name cannot be qualified the way that plugin names tables.
func (d *discovery) jdbcTable(ctx context.Context, p processorComponent, table string) (string, []string, string) {
	poolID := propertyValue(p.Config, poolProperty)
	if poolID == "" {
		return "", nil, ""
	}

	pool := d.connectionPool(ctx, poolID)
	if pool == nil {
		return "", nil, ""
	}

	jdbcURL := stringProperty(pool.Properties, "Database Connection URL")
	nativeProvider := jdbcProvider(jdbcURL)
	endpoint := parseJDBCURL(jdbcURL)
	host := hostWithDefault(endpoint.host, endpoint.port)
	parts := strings.Split(strings.TrimSpace(table), ".")

	switch nativeProvider {
	case "PostgreSQL":
		// The PostgreSQL plugin names a table database.schema.table and a session's default schema is public.
		schema := propertyValue(p.Config, "Schema Name")
		if schema == "" {
			schema = "public"
		}
		path, ok := qualify(parts, endpoint.database, schema)
		if !ok {
			return "", nil, ""
		}
		return nativeProvider, path, host
	case "MySQL", "MariaDB":
		database := propertyValue(p.Config, "Catalog Name")
		if database == "" {
			database = endpoint.database
		}
		path, ok := qualify(parts, database)
		if !ok {
			return "", nil, ""
		}
		return nativeProvider, path, host
	case "SQL Server":
		database := propertyValue(p.Config, "Database Name")
		if database == "" {
			database = endpoint.database
		}
		schema := propertyValue(p.Config, "Schema Name")
		if schema == "" {
			schema = "dbo"
		}
		path, ok := qualify(parts, database, schema)
		if !ok {
			return "", nil, ""
		}
		return nativeProvider, path, host
	case "Oracle":
		// An Oracle session's default schema is its user, in upper case.
		schema := propertyValue(p.Config, "Schema Name")
		if schema == "" {
			schema = strings.ToUpper(stringProperty(pool.Properties, "Database User"))
		}
		path, ok := qualify(parts, endpoint.database, schema)
		if !ok {
			return "", nil, ""
		}
		return nativeProvider, path, host
	default:
		log.Debug().Str("processor", p.Name).Str("pool", pool.Name).Msg("Connection pool database is not one Marmot catalogues, skipping table lineage")
		return "", nil, ""
	}
}

// connectionPool reads a controller service once per run. A failed read
// is remembered as nil so every processor sharing the pool does not
// repeat the request.
func (d *discovery) connectionPool(ctx context.Context, id string) *controllerServiceComponent {
	if pool, seen := d.services[id]; seen {
		return pool
	}

	entity, err := d.client.controllerService(ctx, id)
	if err != nil {
		log.Warn().Err(err).Str("controller_service", id).Msg("Failed to read connection pool, skipping table lineage")
		d.services[id] = nil
		return nil
	}

	d.services[id] = &entity.Component
	return &entity.Component
}

// jdbcProvider maps a JDBC URL's subprotocol to the Marmot provider that
// catalogues that database, or "" when there is none.
func jdbcProvider(jdbcURL string) string {
	lower := strings.ToLower(strings.TrimSpace(jdbcURL))
	if !strings.HasPrefix(lower, "jdbc:") {
		return ""
	}
	subprotocol := strings.SplitN(strings.TrimPrefix(lower, "jdbc:"), ":", 2)[0]

	switch subprotocol {
	case "postgresql":
		return "PostgreSQL"
	case "mysql":
		return "MySQL"
	case "mariadb":
		return "MariaDB"
	case "sqlserver":
		return "SQL Server"
	case "oracle":
		return "Oracle"
	default:
		return ""
	}
}

// jdbcParameter reads a key=value parameter from a JDBC URL. SQL Server
// separates them with semicolons, most other drivers use a query string.
func jdbcParameter(jdbcURL string, keys ...string) string {
	fields := strings.FieldsFunc(jdbcURL, func(r rune) bool { return r == ';' || r == '?' || r == '&' })
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		for _, want := range keys {
			if strings.EqualFold(strings.TrimSpace(key), want) {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

// qualify pads a table name on the left up to the number of levels the target plugin names a table by. The fillers are the levels above the table, outermost first, and only the ones the name lacks are used. It fails when a filler needed is unknown or the name already has too many levels.
func qualify(segments []string, fillers ...string) ([]string, bool) {
	missing := len(fillers) + 1 - len(segments)
	if missing < 0 {
		return nil, false
	}
	prefix := fillers[:missing]
	for _, filler := range prefix {
		if filler == "" {
			return nil, false
		}
	}
	return append(append([]string{}, prefix...), segments...), true
}

// jdbcEndpoint is the server and database a JDBC URL names. Port is the driver's default when the URL omits it.
type jdbcEndpoint struct {
	host     string
	port     string
	database string
}

// parseJDBCURL reads the first host, the port and the database or service name from a JDBC URL of one of the kinds jdbcProvider knows.
func parseJDBCURL(jdbcURL string) jdbcEndpoint {
	lower := strings.ToLower(strings.TrimSpace(jdbcURL))
	if !strings.HasPrefix(lower, "jdbc:") {
		return jdbcEndpoint{}
	}
	rest := strings.TrimSpace(jdbcURL)[len("jdbc:"):]
	subprotocol, rest, _ := strings.Cut(rest, ":")

	switch strings.ToLower(subprotocol) {
	case "postgresql":
		return parseAuthorityURL(rest, "5432", "")
	case "mysql", "mariadb":
		return parseAuthorityURL(rest, "3306", "")
	case "sqlserver":
		return parseAuthorityURL(rest, "1433", "databaseName", "database")
	case "oracle":
		return parseOracleURL(rest)
	}
	return jdbcEndpoint{}
}

// parseAuthorityURL reads //host:port/database?query, taking the database from a named parameter instead when keys are given (SQL Server).
func parseAuthorityURL(rest, defaultPort string, keys ...string) jdbcEndpoint {
	rest = strings.TrimPrefix(rest, "//")
	authority := rest
	if i := strings.IndexAny(authority, "/;?"); i >= 0 {
		authority = authority[:i]
	}
	database := ""
	if len(keys) > 0 && keys[0] != "" {
		database = jdbcParameter(rest, keys...)
	} else if _, after, ok := strings.Cut(rest, "/"); ok {
		database, _, _ = strings.Cut(after, "?")
	}
	first, _, _ := strings.Cut(authority, ",")
	first, _, _ = strings.Cut(first, "\\")
	host, port := splitHostPort(first, defaultPort)
	return jdbcEndpoint{host: host, port: port, database: strings.TrimSpace(database)}
}

// parseOracleURL reads thin:@//host:port/service, thin:@host:port/service and thin:@host:port:SID.
func parseOracleURL(rest string) jdbcEndpoint {
	_, rest, _ = strings.Cut(rest, "@")
	rest = strings.TrimPrefix(rest, "//")
	if strings.HasPrefix(rest, "(") {
		return jdbcEndpoint{}
	}
	if i := strings.Index(rest, "?"); i >= 0 {
		rest = rest[:i]
	}
	if hostPort, service, ok := strings.Cut(rest, "/"); ok {
		host, port := splitHostPort(hostPort, "1521")
		return jdbcEndpoint{host: host, port: port, database: service}
	}
	parts := strings.Split(rest, ":")
	if len(parts) == 3 {
		return jdbcEndpoint{host: parts[0], port: parts[1], database: parts[2]}
	}
	host, port := splitHostPort(rest, "1521")
	return jdbcEndpoint{host: host, port: port}
}

func splitHostPort(address, defaultPort string) (string, string) {
	address = strings.TrimSpace(address)
	if host, port, err := net.SplitHostPort(address); err == nil {
		return host, port
	}
	return strings.Trim(address, "[]"), defaultPort
}

func stringProperty(properties map[string]*string, name string) string {
	if value, ok := properties[name]; ok && value != nil {
		return strings.TrimSpace(*value)
	}
	return ""
}

func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = appendUnique(out, item)
		}
	}
	return out
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
