package nifi

import (
	"context"
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
	if rule.JDBC {
		var name string
		nativeProvider, name = d.jdbcTable(ctx, p, value)
		if nativeProvider == "" {
			return
		}
		names = []string{name}
	}

	for _, name := range names {
		target := nativeMRN(rule.AssetType, nativeProvider, name)
		if rule.Direction == writesTo {
			d.link(taskMRN, target, "PRODUCES")
		} else {
			d.link(target, taskMRN, "FEEDS")
		}

		if nativeProvider == "Kafka" {
			d.recordTopic(name, taskName, rule.Direction)
		}
	}
}

func (d *discovery) recordTopic(topic, taskName string, direction flowDirection) {
	usage, ok := d.topics[topic]
	if !ok {
		usage = &topicUsage{}
		d.topics[topic] = usage
	}
	if direction == writesTo {
		usage.producers = appendUnique(usage.producers, taskName)
	} else {
		usage.consumers = appendUnique(usage.consumers, taskName)
	}
}

// jdbcTable resolves the provider and table name for a database
// processor from the JDBC URL of its connection pool. It returns an
// empty provider when the pool cannot be read, the database kind is
// not one Marmot has a plugin for, or the name cannot be qualified the
// way that plugin names tables.
func (d *discovery) jdbcTable(ctx context.Context, p processorComponent, table string) (string, string) {
	poolID := propertyValue(p.Config, poolProperty)
	if poolID == "" {
		return "", ""
	}

	pool := d.connectionPool(ctx, poolID)
	if pool == nil {
		return "", ""
	}

	jdbcURL := stringProperty(pool.Properties, "Database Connection URL")
	nativeProvider := jdbcProvider(jdbcURL)

	switch nativeProvider {
	case "PostgreSQL", "MySQL", "MariaDB":
		// These plugins name a table by its bare name.
		parts := strings.Split(table, ".")
		return nativeProvider, parts[len(parts)-1]
	case "SQL Server":
		database := propertyValue(p.Config, "Database Name")
		if database == "" {
			database = jdbcParameter(jdbcURL, "databaseName", "database")
		}
		schema := propertyValue(p.Config, "Schema Name")
		if schema == "" {
			schema = "dbo"
		}
		name, ok := qualify(table, 3, schema, database)
		if !ok {
			return "", ""
		}
		return nativeProvider, name
	case "Oracle":
		// An Oracle session's default schema is its user, in upper case.
		schema := propertyValue(p.Config, "Schema Name")
		if schema == "" {
			schema = strings.ToUpper(stringProperty(pool.Properties, "Database User"))
		}
		name, ok := qualify(table, 2, schema)
		if !ok {
			return "", ""
		}
		return nativeProvider, name
	default:
		log.Debug().Str("processor", p.Name).Str("pool", pool.Name).Msg("Connection pool database is not one Marmot catalogues, skipping table lineage")
		return "", ""
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

// qualify pads a dotted table name on the left up to the number of
// parts the target plugin expects, using the fillers in order from the
// nearest (schema) outwards (database). It fails when a filler needed
// is unknown or the name already has too many parts.
func qualify(table string, parts int, fillers ...string) (string, bool) {
	segments := strings.Split(strings.TrimSpace(table), ".")
	if len(segments) > parts {
		return "", false
	}
	for i := 0; len(segments) < parts; i++ {
		if i >= len(fillers) || fillers[i] == "" {
			return "", false
		}
		segments = append([]string{fillers[i]}, segments...)
	}
	return strings.Join(segments, "."), true
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
