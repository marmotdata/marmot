package athena

// AthenaTableFields describes the metadata the plugin records for a table or
// view. The fields live under the `athena` key of the asset metadata, because
// the asset itself is owned by the Glue provider and shared with the Glue
// plugin. It is a documentation-only struct so downstream tooling can
// introspect the shape of the metadata map.
// +marmot:metadata
type AthenaTableFields struct {
	Catalog             string `json:"catalog" metadata:"catalog" description:"Data catalog holding the table"`
	CatalogType         string `json:"catalog_type" metadata:"catalog_type" description:"Data catalog type (GLUE, HIVE, LAMBDA, FEDERATED)"`
	Database            string `json:"database" metadata:"database" description:"Database holding the table"`
	TableType           string `json:"table_type" metadata:"table_type" description:"Table type (EXTERNAL_TABLE, VIRTUAL_VIEW, ...)"`
	Location            string `json:"location" metadata:"location" description:"Storage location of the table data"`
	InputFormat         string `json:"input_format" metadata:"input_format" description:"Hadoop input format class"`
	OutputFormat        string `json:"output_format" metadata:"output_format" description:"Hadoop output format class"`
	Serde               string `json:"serde" metadata:"serde" description:"Serialization library"`
	Compression         string `json:"compression" metadata:"compression" description:"Compression codec of the table data"`
	Classification      string `json:"classification" metadata:"classification" description:"Data format of the table (parquet, csv, json, ...)"`
	PartitionKeys       string `json:"partition_keys" metadata:"partition_keys" description:"Partition key columns"`
	PartitionProjection bool   `json:"partition_projection" metadata:"partition_projection" description:"Whether partition projection is enabled"`
	Created             string `json:"created" metadata:"created" description:"Date and time the table was created"`
	LastAccess          string `json:"last_access" metadata:"last_access" description:"Date and time the table was last accessed"`
	Comment             string `json:"comment" metadata:"comment" description:"Table comment"`
}

// AthenaDatabaseFields describes the metadata recorded for a database, under
// the `athena` key of the asset metadata.
// +marmot:metadata
type AthenaDatabaseFields struct {
	Catalog     string            `json:"catalog" metadata:"catalog" description:"Data catalog holding the database"`
	CatalogType string            `json:"catalog_type" metadata:"catalog_type" description:"Data catalog type (GLUE, HIVE, LAMBDA, FEDERATED)"`
	Description string            `json:"description" metadata:"description" description:"Description of the database"`
	LocationURI string            `json:"location_uri" metadata:"location_uri" description:"Storage location of the database"`
	Parameters  map[string]string `json:"parameters" metadata:"parameters" description:"Database parameters"`
}

// AthenaColumnFields describes the per-column fields embedded in a table
// asset's schema.
type AthenaColumnFields struct {
	ColumnName     string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType       string `json:"data_type" metadata:"data_type" description:"Hive type of the column, recorded verbatim"`
	IsNullable     bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed, always true in Athena"`
	IsPartitionKey bool   `json:"is_partition_key" metadata:"is_partition_key" description:"Whether the column is a partition key"`
	Description    string `json:"description" metadata:"description" description:"Column comment"`
}

// AthenaWorkGroupFields describes the metadata recorded for a workgroup.
// +marmot:metadata
type AthenaWorkGroupFields struct {
	State                 string `json:"state" metadata:"state" description:"Workgroup state (ENABLED, DISABLED)"`
	Description           string `json:"description" metadata:"description" description:"Description of the workgroup"`
	EngineVersion         string `json:"engine_version" metadata:"engine_version" description:"Effective Athena engine version"`
	SelectedEngineVersion string `json:"selected_engine_version" metadata:"selected_engine_version" description:"Engine version the workgroup requested"`
	OutputLocation        string `json:"output_location" metadata:"output_location" description:"S3 location query results are written to"`
	Encryption            string `json:"encryption" metadata:"encryption" description:"Encryption option for query results (SSE_S3, SSE_KMS, CSE_KMS)"`
	EncryptionKMSKey      string `json:"encryption_kms_key" metadata:"encryption_kms_key" description:"KMS key used to encrypt query results"`
	BytesScannedCutoff    int64  `json:"bytes_scanned_cutoff" metadata:"bytes_scanned_cutoff" description:"Per-query limit on bytes scanned"`
	EnforceConfiguration  bool   `json:"enforce_configuration" metadata:"enforce_configuration" description:"Whether the workgroup settings override client settings"`
	PublishMetrics        bool   `json:"publish_metrics" metadata:"publish_metrics" description:"Whether query metrics are published to CloudWatch"`
	RequesterPays         bool   `json:"requester_pays" metadata:"requester_pays" description:"Whether queries may read requester pays buckets"`
	Created               string `json:"created" metadata:"created" description:"Date and time the workgroup was created"`
	Region                string `json:"region" metadata:"region" description:"AWS region of the workgroup"`
	URL                   string `json:"url" metadata:"url" description:"Link to the workgroup in the Athena console"`
}

// AthenaSavedQueryFields describes the metadata recorded for a saved query.
// +marmot:metadata
type AthenaSavedQueryFields struct {
	NamedQueryID string `json:"named_query_id" metadata:"named_query_id" description:"Athena identifier of the saved query"`
	Database     string `json:"database" metadata:"database" description:"Database the query runs against by default"`
	WorkGroup    string `json:"workgroup" metadata:"workgroup" description:"Workgroup holding the saved query"`
	Description  string `json:"description" metadata:"description" description:"Description of the saved query"`
}

// AthenaCatalogFields describes the metadata recorded for a data catalog.
// +marmot:metadata
type AthenaCatalogFields struct {
	Type        string            `json:"type" metadata:"type" description:"Catalog type (GLUE, HIVE, LAMBDA, FEDERATED)"`
	Description string            `json:"description" metadata:"description" description:"Description of the catalog"`
	Parameters  map[string]string `json:"parameters" metadata:"parameters" description:"Catalog parameters, such as the Lambda function backing a federated catalog"`
}
