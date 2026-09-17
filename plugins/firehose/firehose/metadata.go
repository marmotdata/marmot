package firehose

// FirehoseFields describes the metadata fields the Firehose plugin emits
// for a delivery stream asset. It is a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
// +marmot:metadata
type FirehoseFields struct {
	ARN                 string            `json:"arn" metadata:"arn" description:"ARN of the delivery stream"`
	Status              string            `json:"status" metadata:"status" description:"Delivery stream status (ACTIVE, CREATING, DELETING)"`
	StreamType          string            `json:"stream_type" metadata:"stream_type" description:"Delivery stream type (DirectPut, KinesisStreamAsSource, MSKAsSource, DatabaseAsSource)"`
	VersionID           string            `json:"version_id" metadata:"version_id" description:"Version of the delivery stream configuration"`
	Region              string            `json:"region" metadata:"region" description:"AWS region the delivery stream lives in"`
	CreatedAt           string            `json:"created_at" metadata:"created_at" description:"When the delivery stream was created"`
	LastUpdatedAt       string            `json:"last_updated_at" metadata:"last_updated_at" description:"When the delivery stream was last updated"`
	EncryptionStatus    string            `json:"encryption_status" metadata:"encryption_status" description:"Server-side encryption status"`
	EncryptionKeyType   string            `json:"encryption_key_type" metadata:"encryption_key_type" description:"Server-side encryption key type (AWS_OWNED_CMK, CUSTOMER_MANAGED_CMK)"`
	SourceType          string            `json:"source_type" metadata:"source_type" description:"Where records come from (direct_put, kinesis, msk, database)"`
	SourceKinesisStream string            `json:"source_kinesis_stream" metadata:"source_kinesis_stream" description:"Name of the Kinesis stream feeding the delivery stream"`
	SourceMSKCluster    string            `json:"source_msk_cluster" metadata:"source_msk_cluster" description:"Name of the MSK cluster feeding the delivery stream"`
	SourceMSKTopic      string            `json:"source_msk_topic" metadata:"source_msk_topic" description:"Name of the MSK topic feeding the delivery stream"`
	DestinationType     string            `json:"destination_type" metadata:"destination_type" description:"Where records are written (s3, extended_s3, redshift, elasticsearch, opensearch, opensearch_serverless, splunk, http_endpoint, snowflake, iceberg)"`
	DestinationCount    int               `json:"destination_count" metadata:"destination_count" description:"Number of destinations configured on the delivery stream"`
	Destination         map[string]any    `json:"destination" metadata:"destination" description:"Destination settings, with any value that could carry a credential redacted"`
	Tags                map[string]string `json:"tags" metadata:"tags" description:"AWS resource tags"`
}

// FirehoseDestinationFields describes the fields of the `destination`
// sub-map. Which of them are present depends on the destination type.
type FirehoseDestinationFields struct {
	Bucket                   string   `json:"bucket" metadata:"bucket" description:"S3 bucket name"`
	Prefix                   string   `json:"prefix" metadata:"prefix" description:"S3 key prefix records are written under"`
	ErrorOutputPrefix        string   `json:"error_output_prefix" metadata:"error_output_prefix" description:"S3 key prefix failed records are written under"`
	CompressionFormat        string   `json:"compression_format" metadata:"compression_format" description:"S3 compression format"`
	FileExtension            string   `json:"file_extension" metadata:"file_extension" description:"File extension of the delivered S3 objects"`
	S3BackupMode             string   `json:"s3_backup_mode" metadata:"s3_backup_mode" description:"Whether records are also backed up to S3"`
	BufferingSizeMB          int      `json:"buffering_size_mb" metadata:"buffering_size_mb" description:"Buffer size in MB before delivery"`
	BufferingIntervalSeconds int      `json:"buffering_interval_seconds" metadata:"buffering_interval_seconds" description:"Buffer interval in seconds before delivery"`
	FormatConversionEnabled  bool     `json:"format_conversion_enabled" metadata:"format_conversion_enabled" description:"Whether records are converted to a columnar format"`
	InputFormat              string   `json:"input_format" metadata:"input_format" description:"Deserializer for incoming records (HiveJsonSerDe, OpenXJsonSerDe)"`
	OutputFormat             string   `json:"output_format" metadata:"output_format" description:"Serializer for delivered records (ParquetSerDe, OrcSerDe)"`
	GlueCatalogID            string   `json:"glue_catalog_id" metadata:"glue_catalog_id" description:"Glue catalog the conversion schema is read from"`
	GlueDatabase             string   `json:"glue_database" metadata:"glue_database" description:"Glue database the conversion schema is read from"`
	GlueTable                string   `json:"glue_table" metadata:"glue_table" description:"Glue table the conversion schema is read from"`
	GlueRegion               string   `json:"glue_region" metadata:"glue_region" description:"Region of the Glue catalog"`
	ClusterEndpoint          string   `json:"cluster_endpoint" metadata:"cluster_endpoint" description:"Redshift cluster host, or Elasticsearch and OpenSearch cluster endpoint"`
	Database                 string   `json:"database" metadata:"database" description:"Redshift or Snowflake database"`
	Schema                   string   `json:"schema" metadata:"schema" description:"Snowflake schema"`
	Table                    string   `json:"table" metadata:"table" description:"Redshift or Snowflake table"`
	Username                 string   `json:"username" metadata:"username" description:"Redshift user the copy command runs as"`
	CopyOptions              string   `json:"copy_options" metadata:"copy_options" description:"Redshift copy command options"`
	CopyColumns              string   `json:"copy_columns" metadata:"copy_columns" description:"Redshift columns the copy command targets"`
	DomainARN                string   `json:"domain_arn" metadata:"domain_arn" description:"Elasticsearch or OpenSearch domain ARN"`
	IndexName                string   `json:"index_name" metadata:"index_name" description:"Elasticsearch or OpenSearch index name"`
	TypeName                 string   `json:"type_name" metadata:"type_name" description:"Elasticsearch or OpenSearch type name"`
	IndexRotationPeriod      string   `json:"index_rotation_period" metadata:"index_rotation_period" description:"How often the index name is rotated"`
	CollectionEndpoint       string   `json:"collection_endpoint" metadata:"collection_endpoint" description:"OpenSearch Serverless collection endpoint"`
	HECEndpoint              string   `json:"hec_endpoint" metadata:"hec_endpoint" description:"Splunk HTTP event collector endpoint"`
	HECEndpointType          string   `json:"hec_endpoint_type" metadata:"hec_endpoint_type" description:"Splunk HTTP event collector endpoint type"`
	URL                      string   `json:"url" metadata:"url" description:"HTTP endpoint URL"`
	Name                     string   `json:"name" metadata:"name" description:"HTTP endpoint name"`
	AccountURL               string   `json:"account_url" metadata:"account_url" description:"Snowflake account URL"`
	User                     string   `json:"user" metadata:"user" description:"Snowflake user"`
	CatalogARN               string   `json:"catalog_arn" metadata:"catalog_arn" description:"Iceberg catalog ARN"`
	WarehouseLocation        string   `json:"warehouse_location" metadata:"warehouse_location" description:"Iceberg warehouse location"`
	Tables                   []string `json:"tables" metadata:"tables" description:"Iceberg destination tables"`
}
