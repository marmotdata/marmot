package bigquery

// +marmot:metadata
type BigQueryDatasetFields struct {
	ProjectID                  string            `json:"project_id" metadata:"project_id" description:"Google Cloud Project ID"`
	DatasetID                  string            `json:"dataset_id" metadata:"dataset_id" description:"Dataset ID"`
	Location                   string            `json:"location" metadata:"location" description:"Geographic location of the dataset"`
	CreationTime               string            `json:"creation_time" metadata:"creation_time" description:"Dataset creation timestamp"`
	LastModified               string            `json:"last_modified" metadata:"last_modified" description:"Last modification timestamp"`
	Description                string            `json:"description" metadata:"description" description:"Dataset description"`
	DefaultTableExpiration     string            `json:"default_table_expiration" metadata:"default_table_expiration" description:"Default table expiration duration"`
	DefaultPartitionExpiration string            `json:"default_partition_expiration" metadata:"default_partition_expiration" description:"Default partition expiration duration"`
	Labels                     map[string]string `json:"labels" metadata:"labels" description:"Dataset labels"`
	AccessEntriesCount         int               `json:"access_entries_count" metadata:"access_entries_count" description:"Number of access control entries"`
}

// +marmot:metadata
type BigQueryTableFields struct {
	ProjectID              string                 `json:"project_id" metadata:"project_id" description:"Google Cloud Project ID"`
	DatasetID              string                 `json:"dataset_id" metadata:"dataset_id" description:"Dataset ID"`
	TableID                string                 `json:"table_id" metadata:"table_id" description:"Table ID"`
	TableType              string                 `json:"table_type" metadata:"table_type" description:"Table type (TABLE, VIEW, EXTERNAL)"`
	CreationTime           string                 `json:"creation_time" metadata:"creation_time" description:"Table creation timestamp"`
	LastModified           string                 `json:"last_modified" metadata:"last_modified" description:"Last modification timestamp"`
	Description            string                 `json:"description" metadata:"description" description:"Table description"`
	ExpirationTime         string                 `json:"expiration_time" metadata:"expiration_time" description:"Table expiration timestamp"`
	Labels                 map[string]string      `json:"labels" metadata:"labels" description:"Table labels"`
	NumRows                uint64                 `json:"num_rows" metadata:"num_rows" description:"Number of rows in the table"`
	NumBytes               int64                  `json:"num_bytes" metadata:"num_bytes" description:"Size of the table in bytes"`
	TimePartitioningType   string                 `json:"time_partitioning_type" metadata:"time_partitioning_type" description:"Time partitioning type"`
	TimePartitioningField  string                 `json:"time_partitioning_field" metadata:"time_partitioning_field" description:"Time partitioning field"`
	PartitionExpiration    string                 `json:"partition_expiration" metadata:"partition_expiration" description:"Partition expiration duration"`
	RangePartitioningField string                 `json:"range_partitioning_field" metadata:"range_partitioning_field" description:"Range partitioning field"`
	ClusteringFields       []string               `json:"clustering_fields" metadata:"clustering_fields" description:"Clustering fields"`
	ViewQuery              string                 `json:"view_query" metadata:"view_query" description:"SQL query for views"`
	ExternalDataConfig     map[string]interface{} `json:"external_data_config" metadata:"external_data_config" description:"External data configuration for external tables"`
}

// +marmot:metadata
type BigQueryColumnFields struct {
	Name         string                   `json:"name" metadata:"name" description:"Column name"`
	Type         string                   `json:"type" metadata:"type" description:"Column data type"`
	Description  string                   `json:"description" metadata:"description" description:"Column description"`
	NestedFields []map[string]interface{} `json:"nested_fields" metadata:"nested_fields" description:"Nested fields for RECORD type columns"`
}

// +marmot:metadata
type BigQueryExternalDataConfig struct {
	SourceFormat string   `json:"source_format" metadata:"source_format" description:"Source data format (CSV, JSON, AVRO, etc.)"`
	SourceURIs   []string `json:"source_uris" metadata:"source_uris" description:"Source URIs for external data"`
}

