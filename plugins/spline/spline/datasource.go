package spline

import (
	"net/url"
	"strings"
)

// dataSourceRef is a table or bucket that another Marmot plugin owns. Spline
// only records the URI Spark read from or wrote to, so the plugin has to
// resolve that back to the identity the owning plugin publishes, otherwise the
// lineage edge would point at an asset that does not exist.
type dataSourceRef struct {
	Type     string
	Provider string
	Name     string
}

// objectStoreSchemes maps a bucket-like URI scheme to the provider and asset
// type of the plugin that owns the bucket.
var objectStoreSchemes = map[string]dataSourceRef{
	"s3":    {Type: "Bucket", Provider: "S3"},
	"s3a":   {Type: "Bucket", Provider: "S3"},
	"s3n":   {Type: "Bucket", Provider: "S3"},
	"gs":    {Type: "Bucket", Provider: "GCS"},
	"abfs":  {Type: "Container", Provider: "AzureBlob"},
	"abfss": {Type: "Container", Provider: "AzureBlob"},
}

// parseDataSourceURI resolves a Spline data source URI to the asset another
// Marmot plugin publishes for it. It reports false when the URI names
// something Marmot has no asset for (a plain file path, HDFS) or a scheme this
// plugin does not know, in which case no lineage edge is emitted.
func parseDataSourceURI(uri string) (dataSourceRef, bool) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return dataSourceRef{}, false
	}

	scheme, rest, hasScheme := strings.Cut(uri, ":")
	if !hasScheme {
		// Spark's Hive catalog reports tables as a bare "db.table".
		return hiveRef(uri)
	}
	scheme = strings.ToLower(scheme)

	if scheme == "jdbc" {
		return parseJDBCURI(rest)
	}

	if ref, ok := objectStoreSchemes[scheme]; ok {
		bucket := authorityOf(rest)
		if bucket == "" {
			return dataSourceRef{}, false
		}
		// An abfss URI is container@account.dfs.core.windows.net; the
		// container is what Marmot's Azure Blob plugin names its assets after.
		if at := strings.Index(bucket, "@"); at > 0 {
			bucket = bucket[:at]
		}
		ref.Name = bucket
		return ref, true
	}

	switch scheme {
	case "hive":
		return hiveRef(stripAuthoritySlashes(rest))
	case "delta":
		// A Delta table is addressed by its bare name, matching the Delta Lake
		// plugin, and the name is the last segment of the table's path.
		path := strings.Trim(stripAuthoritySlashes(rest), "/")
		if path == "" {
			return dataSourceRef{}, false
		}
		segments := strings.Split(path, "/")
		return dataSourceRef{Type: "Table", Provider: "Delta Lake", Name: segments[len(segments)-1]}, true
	}

	// hdfs, file, dbfs and anything else: Marmot has no asset to point at.
	return dataSourceRef{}, false
}

// hiveRef turns "db.table", "db/table" or a bare "table" into a Hive table
// reference. The Hive plugin names its tables "database.table".
func hiveRef(path string) (dataSourceRef, bool) {
	path = strings.Trim(path, "/")
	if path == "" {
		return dataSourceRef{}, false
	}

	var segments []string
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '.' }) {
		if part != "" {
			segments = append(segments, part)
		}
	}
	if len(segments) == 0 {
		return dataSourceRef{}, false
	}

	name := segments[len(segments)-1]
	if len(segments) >= 2 {
		name = segments[len(segments)-2] + "." + name
	}
	return dataSourceRef{Type: "Table", Provider: "Hive", Name: name}, true
}

// jdbcEngine describes how one JDBC engine's table identity is spelled by the
// Marmot plugin that owns it.
type jdbcEngine struct {
	Provider string
	// DefaultSchema is used when the URI names a table without a schema.
	DefaultSchema string
	// Name builds the asset name from the database, schema and table.
	Name func(database, schema, table string) string
}

func bareTable(_, _, table string) string        { return table }
func schemaTable(_, schema, table string) string { return schema + "." + table }
func dbSchemaTable(database, schema, table string) string {
	return database + "." + schema + "." + table
}

// jdbcEngines maps a JDBC sub-protocol to the identity its Marmot plugin uses.
// Engines missing from this map produce no lineage edge, because guessing the
// name shape would create an edge that points at nothing.
var jdbcEngines = map[string]jdbcEngine{
	"postgresql": {Provider: "PostgreSQL", DefaultSchema: "public", Name: bareTable},
	"mysql":      {Provider: "MySQL", Name: bareTable},
	"mariadb":    {Provider: "MariaDB", Name: bareTable},
	"sqlserver":  {Provider: "SQL Server", DefaultSchema: "dbo", Name: dbSchemaTable},
	"oracle":     {Provider: "Oracle", Name: schemaTable},
	"redshift":   {Provider: "Redshift", DefaultSchema: "public", Name: dbSchemaTable},
	"snowflake":  {Provider: "Snowflake", DefaultSchema: "public", Name: dbSchemaTable},
}

// parseJDBCURI resolves the part of a JDBC URI after "jdbc:". Spline appends
// the table to the connection URL, either as a trailing ":table" or as a
// "table" query parameter, depending on which Spark connector wrote it.
func parseJDBCURI(rest string) (dataSourceRef, bool) {
	subProtocol, remainder, ok := strings.Cut(rest, ":")
	if !ok {
		return dataSourceRef{}, false
	}
	engine, known := jdbcEngines[strings.ToLower(subProtocol)]
	if !known {
		return dataSourceRef{}, false
	}

	var query url.Values
	if base, rawQuery, hasQuery := strings.Cut(remainder, "?"); hasQuery {
		remainder = base
		query, _ = url.ParseQuery(rawQuery)
	}

	// Oracle's thin driver prefixes the host with "thin:@"; every engine here
	// then uses the //host[:port][/path] form.
	if i := strings.Index(remainder, "//"); i >= 0 {
		remainder = remainder[i+2:]
	}

	authority, path, _ := strings.Cut(remainder, "/")

	// The table qualifier is whatever follows the last colon of the segment it
	// was appended to. Query-parameter form wins when both are present.
	qualifier := firstQueryValue(query, "table", "dbtable")
	if qualifier == "" {
		if path != "" {
			if head, tail, found := cutLast(path, ":"); found {
				path, qualifier = head, tail
			}
		} else if _, tail, found := cutLast(lastSegment(authority, ";"), ":"); found {
			// A SQL Server URL has no path: its properties hang off the
			// authority, and the table trails the last property.
			authority = strings.TrimSuffix(authority, ":"+tail)
			qualifier = tail
		}
	}
	if qualifier == "" {
		return dataSourceRef{}, false
	}

	database := path
	if strings.EqualFold(subProtocol, "sqlserver") {
		database = propertyValue(authority, "databaseName")
	}
	if database == "" {
		database = firstQueryValue(query, "db", "database", "databaseName")
	}
	database = strings.Trim(database, "/")

	qualDatabase, schema, table := splitQualifier(qualifier)
	if table == "" {
		return dataSourceRef{}, false
	}
	if qualDatabase != "" {
		database = qualDatabase
	}
	if schema == "" {
		schema = firstQueryValue(query, "schema", "currentSchema")
	}
	if schema == "" {
		schema = engine.DefaultSchema
	}

	name := engine.Name(database, schema, table)
	// A name with an empty part would not match the owning plugin's asset, so
	// it is better to emit nothing than an edge that dangles.
	if name == "" || strings.HasPrefix(name, ".") || strings.Contains(name, "..") {
		return dataSourceRef{}, false
	}

	return dataSourceRef{Type: "Table", Provider: engine.Provider, Name: name}, true
}

// splitQualifier splits "table", "schema.table" or "db.schema.table".
func splitQualifier(qualifier string) (database, schema, table string) {
	parts := strings.Split(strings.Trim(qualifier, "."), ".")
	switch len(parts) {
	case 0:
		return "", "", ""
	case 1:
		return "", "", parts[0]
	case 2:
		return "", parts[0], parts[1]
	default:
		n := len(parts)
		return parts[n-3], parts[n-2], parts[n-1]
	}
}

// propertyValue reads a "key=value" property from a semicolon separated JDBC
// property list, matching the key case-insensitively.
func propertyValue(properties, key string) string {
	for _, property := range strings.Split(properties, ";") {
		name, value, ok := strings.Cut(property, "=")
		if ok && strings.EqualFold(strings.TrimSpace(name), key) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstQueryValue(query url.Values, keys ...string) string {
	for _, key := range keys {
		for name, values := range query {
			if strings.EqualFold(name, key) && len(values) > 0 && values[0] != "" {
				return values[0]
			}
		}
	}
	return ""
}

// cutLast splits s at its last occurrence of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

func lastSegment(s, sep string) string {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[i+len(sep):]
	}
	return s
}

// authorityOf returns the host part of a "//authority/path" remainder.
func authorityOf(rest string) string {
	rest = strings.TrimPrefix(rest, "//")
	authority, _, _ := strings.Cut(rest, "/")
	return authority
}

// stripAuthoritySlashes drops the leading slashes of a URI remainder so that
// "//sales/orders" and "/sales/orders" are read the same way.
func stripAuthoritySlashes(rest string) string {
	return strings.TrimPrefix(strings.TrimPrefix(rest, "//"), "/")
}
