package firehose

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

// redacted replaces any value that could carry a credential.
const redacted = "REDACTED"

// lineageTarget is an asset another Marmot plugin owns. Firehose never
// creates these assets, it only points at them, so the type, provider and
// name have to match that plugin exactly or the server drops the edge.
type lineageTarget struct {
	Type     string
	Provider string
	Name     string
}

// destination is one Firehose destination reduced to what the catalog
// needs: which kind of system it writes to, the settings worth recording,
// and the assets it writes into.
type destination struct {
	Kind     string
	Settings map[string]any
	Targets  []lineageTarget
}

// classifyDestination works out which system a destination writes to.
//
// The description carries one field per destination kind and only the
// matching one is set. Extended S3 is the exception: it is a superset of
// plain S3 and some Firehose implementations fill both, so it is checked
// first and the plain S3 field is only used when it stands alone.
func classifyDestination(d types.DestinationDescription) (destination, bool) {
	switch {
	case d.RedshiftDestinationDescription != nil:
		return redshiftDestination(d.RedshiftDestinationDescription), true
	case d.AmazonopensearchserviceDestinationDescription != nil:
		return openSearchDestination(d.AmazonopensearchserviceDestinationDescription), true
	case d.ElasticsearchDestinationDescription != nil:
		return elasticsearchDestination(d.ElasticsearchDestinationDescription), true
	case d.AmazonOpenSearchServerlessDestinationDescription != nil:
		return openSearchServerlessDestination(d.AmazonOpenSearchServerlessDestinationDescription), true
	case d.SplunkDestinationDescription != nil:
		return splunkDestination(d.SplunkDestinationDescription), true
	case d.HttpEndpointDestinationDescription != nil:
		return httpEndpointDestination(d.HttpEndpointDestinationDescription), true
	case d.SnowflakeDestinationDescription != nil:
		return snowflakeDestination(d.SnowflakeDestinationDescription), true
	case d.IcebergDestinationDescription != nil:
		return icebergDestination(d.IcebergDestinationDescription), true
	case d.ExtendedS3DestinationDescription != nil:
		return extendedS3Destination(d.ExtendedS3DestinationDescription), true
	case d.S3DestinationDescription != nil:
		return s3Destination(d.S3DestinationDescription), true
	}

	return destination{}, false
}

func s3Destination(d *types.S3DestinationDescription) destination {
	settings := map[string]any{}
	bucket := bucketFromARN(aws.ToString(d.BucketARN))
	putString(settings, "bucket", bucket)
	putString(settings, "prefix", aws.ToString(d.Prefix))
	putString(settings, "error_output_prefix", aws.ToString(d.ErrorOutputPrefix))
	putString(settings, "compression_format", string(d.CompressionFormat))
	addBufferingHints(settings, d.BufferingHints)

	dest := destination{Kind: "s3", Settings: settings}
	if bucket != "" {
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Bucket", Provider: "S3", Name: bucket})
	}
	return dest
}

func extendedS3Destination(d *types.ExtendedS3DestinationDescription) destination {
	settings := map[string]any{}
	bucket := bucketFromARN(aws.ToString(d.BucketARN))
	putString(settings, "bucket", bucket)
	putString(settings, "prefix", aws.ToString(d.Prefix))
	putString(settings, "error_output_prefix", aws.ToString(d.ErrorOutputPrefix))
	putString(settings, "compression_format", string(d.CompressionFormat))
	putString(settings, "file_extension", aws.ToString(d.FileExtension))
	putString(settings, "s3_backup_mode", string(d.S3BackupMode))
	addBufferingHints(settings, d.BufferingHints)

	dest := destination{Kind: "extended_s3", Settings: settings}
	if bucket != "" {
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Bucket", Provider: "S3", Name: bucket})
	}

	glueTable := addFormatConversion(settings, d.DataFormatConversionConfiguration)
	if glueTable != "" {
		// Format conversion reads the column types from a Glue catalog
		// table, so the stream writes data shaped by that table.
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Table", Provider: "Glue", Name: glueTable})
	}

	return dest
}

// addFormatConversion records the record format conversion settings and
// returns the Glue table the conversion reads its schema from.
func addFormatConversion(settings map[string]any, c *types.DataFormatConversionConfiguration) string {
	if c == nil || !aws.ToBool(c.Enabled) {
		return ""
	}

	settings["format_conversion_enabled"] = true
	putString(settings, "input_format", deserializerName(c.InputFormatConfiguration))
	putString(settings, "output_format", serializerName(c.OutputFormatConfiguration))

	if c.SchemaConfiguration == nil {
		return ""
	}
	putString(settings, "glue_catalog_id", aws.ToString(c.SchemaConfiguration.CatalogId))
	putString(settings, "glue_database", aws.ToString(c.SchemaConfiguration.DatabaseName))
	putString(settings, "glue_table", aws.ToString(c.SchemaConfiguration.TableName))
	putString(settings, "glue_region", aws.ToString(c.SchemaConfiguration.Region))

	return aws.ToString(c.SchemaConfiguration.TableName)
}

func deserializerName(c *types.InputFormatConfiguration) string {
	if c == nil || c.Deserializer == nil {
		return ""
	}
	switch {
	case c.Deserializer.HiveJsonSerDe != nil:
		return "HiveJsonSerDe"
	case c.Deserializer.OpenXJsonSerDe != nil:
		return "OpenXJsonSerDe"
	}
	return ""
}

func serializerName(c *types.OutputFormatConfiguration) string {
	if c == nil || c.Serializer == nil {
		return ""
	}
	switch {
	case c.Serializer.ParquetSerDe != nil:
		return "ParquetSerDe"
	case c.Serializer.OrcSerDe != nil:
		return "OrcSerDe"
	}
	return ""
}

func redshiftDestination(d *types.RedshiftDestinationDescription) destination {
	settings := map[string]any{}

	host, database := parseRedshiftJDBCURL(aws.ToString(d.ClusterJDBCURL))
	putString(settings, "cluster_endpoint", host)
	putString(settings, "database", database)
	putString(settings, "username", aws.ToString(d.Username))

	var dataTable string
	if d.CopyCommand != nil {
		dataTable = aws.ToString(d.CopyCommand.DataTableName)
		putString(settings, "table", dataTable)
		putString(settings, "copy_options", redactCopyOptions(aws.ToString(d.CopyCommand.CopyOptions)))
		putString(settings, "copy_columns", aws.ToString(d.CopyCommand.DataTableColumns))
	}

	dest := destination{Kind: "redshift", Settings: settings}
	if name, ok := redshiftTableName(database, dataTable); ok {
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Table", Provider: "Redshift", Name: name})
	}
	return dest
}

// redshiftTableName builds the database.schema.table name the Marmot
// Redshift plugin gives its tables.
//
// The copy command normally names the table as schema.table. When it names
// only a table, Redshift resolves the schema through the connection's
// search path, which is not visible here, so there is no name to point at
// and the caller drops the edge rather than guessing a schema.
func redshiftTableName(database, dataTableName string) (string, bool) {
	if database == "" || dataTableName == "" {
		return "", false
	}

	parts := strings.Split(dataTableName, ".")
	switch len(parts) {
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return "", false
		}
		return database + "." + dataTableName, true
	case 3:
		// Already fully qualified, the database in the name wins.
		return dataTableName, true
	}

	return "", false
}

// parseRedshiftJDBCURL pulls the cluster host and the database out of a
// Redshift JDBC URL such as
// jdbc:redshift://cluster.abc.us-east-1.redshift.amazonaws.com:5439/analytics.
func parseRedshiftJDBCURL(url string) (host, database string) {
	_, after, found := strings.Cut(url, "://")
	if !found {
		return "", ""
	}

	hostPort, path, found := strings.Cut(after, "/")
	if !found {
		return stripPort(hostPort), ""
	}

	// Anything after the database name is a JDBC connection property.
	database, _, _ = strings.Cut(path, "?")
	return stripPort(hostPort), database
}

func stripPort(hostPort string) string {
	host, _, found := strings.Cut(hostPort, ":")
	if !found {
		return hostPort
	}
	return host
}

func elasticsearchDestination(d *types.ElasticsearchDestinationDescription) destination {
	settings := map[string]any{}
	index := aws.ToString(d.IndexName)
	putString(settings, "domain_arn", aws.ToString(d.DomainARN))
	putString(settings, "cluster_endpoint", aws.ToString(d.ClusterEndpoint))
	putString(settings, "index_name", index)
	putString(settings, "type_name", aws.ToString(d.TypeName))
	putString(settings, "index_rotation_period", string(d.IndexRotationPeriod))

	dest := destination{Kind: "elasticsearch", Settings: settings}
	if index != "" {
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Table", Provider: "Elasticsearch", Name: index})
	}
	return dest
}

func openSearchDestination(d *types.AmazonopensearchserviceDestinationDescription) destination {
	settings := map[string]any{}
	index := aws.ToString(d.IndexName)
	putString(settings, "domain_arn", aws.ToString(d.DomainARN))
	putString(settings, "cluster_endpoint", aws.ToString(d.ClusterEndpoint))
	putString(settings, "index_name", index)
	putString(settings, "type_name", aws.ToString(d.TypeName))
	putString(settings, "index_rotation_period", string(d.IndexRotationPeriod))

	dest := destination{Kind: "opensearch", Settings: settings}
	if index != "" {
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Table", Provider: "OpenSearch", Name: index})
	}
	return dest
}

// openSearchServerlessDestination records the collection it writes to but
// creates no edge: Marmot has no plugin that catalogs serverless
// collections, so there is no asset to point at.
func openSearchServerlessDestination(d *types.AmazonOpenSearchServerlessDestinationDescription) destination {
	settings := map[string]any{}
	putString(settings, "collection_endpoint", aws.ToString(d.CollectionEndpoint))
	putString(settings, "index_name", aws.ToString(d.IndexName))

	return destination{Kind: "opensearch_serverless", Settings: settings}
}

// splunkDestination records the HEC endpoint. The HEC token is a
// credential and never leaves the account.
func splunkDestination(d *types.SplunkDestinationDescription) destination {
	settings := map[string]any{}
	putString(settings, "hec_endpoint", aws.ToString(d.HECEndpoint))
	putString(settings, "hec_endpoint_type", string(d.HECEndpointType))

	return destination{Kind: "splunk", Settings: settings}
}

// httpEndpointDestination records the endpoint URL and name. Marmot has no
// asset for an arbitrary HTTP endpoint, so there is no edge.
func httpEndpointDestination(d *types.HttpEndpointDestinationDescription) destination {
	settings := map[string]any{}
	if d.EndpointConfiguration != nil {
		putString(settings, "url", aws.ToString(d.EndpointConfiguration.Url))
		putString(settings, "name", aws.ToString(d.EndpointConfiguration.Name))
	}

	return destination{Kind: "http_endpoint", Settings: settings}
}

func snowflakeDestination(d *types.SnowflakeDestinationDescription) destination {
	settings := map[string]any{}
	database := aws.ToString(d.Database)
	schema := aws.ToString(d.Schema)
	table := aws.ToString(d.Table)

	putString(settings, "account_url", aws.ToString(d.AccountUrl))
	putString(settings, "database", database)
	putString(settings, "schema", schema)
	putString(settings, "table", table)
	putString(settings, "user", aws.ToString(d.User))

	dest := destination{Kind: "snowflake", Settings: settings}
	if database != "" && schema != "" && table != "" {
		dest.Targets = append(dest.Targets, lineageTarget{
			Type:     "Table",
			Provider: "Snowflake",
			Name:     database + "." + schema + "." + table,
		})
	}
	return dest
}

func icebergDestination(d *types.IcebergDestinationDescription) destination {
	settings := map[string]any{}
	if d.CatalogConfiguration != nil {
		putString(settings, "catalog_arn", aws.ToString(d.CatalogConfiguration.CatalogARN))
		putString(settings, "warehouse_location", aws.ToString(d.CatalogConfiguration.WarehouseLocation))
	}

	dest := destination{Kind: "iceberg", Settings: settings}

	var tables []string
	for _, table := range d.DestinationTableConfigurationList {
		name := aws.ToString(table.DestinationTableName)
		if name == "" {
			continue
		}
		if database := aws.ToString(table.DestinationDatabaseName); database != "" {
			tables = append(tables, database+"."+name)
		} else {
			tables = append(tables, name)
		}
		dest.Targets = append(dest.Targets, lineageTarget{Type: "Table", Provider: "Iceberg", Name: name})
	}
	if len(tables) > 0 {
		settings["tables"] = tables
	}

	return dest
}

func addBufferingHints(settings map[string]any, hints *types.BufferingHints) {
	if hints == nil {
		return
	}
	if hints.SizeInMBs != nil {
		settings["buffering_size_mb"] = int(*hints.SizeInMBs)
	}
	if hints.IntervalInSeconds != nil {
		settings["buffering_interval_seconds"] = int(*hints.IntervalInSeconds)
	}
}

// bucketFromARN returns the bucket name from an S3 bucket ARN such as
// arn:aws:s3:::marmot-lake. A key suffix, which Firehose does not use but
// an ARN may carry, is dropped.
func bucketFromARN(arn string) string {
	resource := arnResource(arn)
	if resource == "" {
		return ""
	}
	bucket, _, _ := strings.Cut(resource, "/")
	return bucket
}

// kinesisStreamFromARN returns the stream name from a Kinesis stream ARN
// such as arn:aws:kinesis:us-east-1:123456789012:stream/orders.
func kinesisStreamFromARN(arn string) string {
	return resourceNameAfter(arnResource(arn), "stream")
}

// mskClusterFromARN returns the cluster name from an MSK cluster ARN such
// as arn:aws:kafka:us-east-1:123456789012:cluster/orders-cluster/uuid-1.
// The trailing uuid distinguishes recreated clusters and is not part of
// the name.
func mskClusterFromARN(arn string) string {
	return resourceNameAfter(arnResource(arn), "cluster")
}

// resourceNameAfter returns the name segment of an ARN resource of the
// form "<kind>/<name>" or "<kind>:<name>", and an empty string when the
// resource is not of that kind.
func resourceNameAfter(resource, kind string) string {
	if resource == "" {
		return ""
	}

	separator := strings.IndexAny(resource, "/:")
	if separator < 0 || resource[:separator] != kind {
		return ""
	}

	name := resource[separator+1:]
	// MSK appends a uuid after the cluster name.
	name, _, _ = strings.Cut(name, "/")
	return name
}

// arnResource returns the resource part of an ARN, everything after the
// fifth colon of arn:partition:service:region:account:resource. An empty
// string means the value was not an ARN.
func arnResource(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[5]
}

// arnRegion returns the region of an ARN, or an empty string when the
// value is not an ARN.
func arnRegion(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 6 || parts[0] != "arn" {
		return ""
	}
	return parts[3]
}

// putString adds a setting only when it has a value, so the catalog is not
// filled with empty fields.
func putString(settings map[string]any, key, value string) {
	if value == "" {
		return
	}
	settings[key] = value
}

// maskSecrets blanks the values of settings whose name suggests a
// credential. Firehose already withholds the ones it knows about, so this
// is a guard against a future field carrying a secret into the catalog.
func maskSecrets(settings map[string]any) map[string]any {
	for key := range settings {
		if isSecretKey(key) {
			settings[key] = redacted
		}
	}
	return settings
}

func isSecretKey(name string) bool {
	lower := strings.ToLower(name)
	for _, word := range []string{"password", "secret", "token", "key", "credential"} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// redactCopyOptions blanks a Redshift copy command that names credentials
// inline. Firehose copies with an IAM role, but the option string is free
// text and an operator can still put keys in it.
func redactCopyOptions(options string) string {
	lower := strings.ToLower(options)
	for _, word := range []string{"credentials", "access_key_id", "secret_access_key", "session_token", "password"} {
		if strings.Contains(lower, word) {
			return redacted
		}
	}
	return options
}
