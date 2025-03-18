package iceberg

// IcebergTableFields represents Iceberg table-specific metadata fields
// +marmot:metadata
type IcebergTableFields struct {
	// Table identity
	Identifier string `json:"identifier" metadata:"identifier" description:"Full identifier of the table (namespace.table_name)"`
	Namespace  string `json:"namespace" metadata:"namespace" description:"Namespace of the table"`
	TableName  string `json:"table_name" metadata:"table_name" description:"Name of the table"`
	Location   string `json:"location" metadata:"location" description:"Base location URI of the table data"`

	// Format and version information
	FormatVersion int    `json:"format_version" metadata:"format_version" description:"Iceberg table format version"`
	UUID          string `json:"uuid" metadata:"uuid" description:"UUID of the table"`

	// Schema information
	CurrentSchemaID int    `json:"current_schema_id" metadata:"current_schema_id" description:"ID of the current schema"`
	SchemaJSON      string `json:"schema_json" metadata:"schema_json" description:"JSON representation of the current schema"`
	PartitionSpec   string `json:"partition_spec" metadata:"partition_spec" description:"JSON representation of the partition specification"`

	// Snapshot information
	CurrentSnapshotID int64  `json:"current_snapshot_id" metadata:"current_snapshot_id" description:"ID of the current snapshot"`
	LastUpdatedMs     int64  `json:"last_updated_ms" metadata:"last_updated_ms" description:"Timestamp when the table was last updated in milliseconds since epoch"`
	LastCommitTime    string `json:"last_commit_time" metadata:"last_commit_time" description:"Human-readable timestamp when the table was last updated"`
	NumSnapshots      int    `json:"num_snapshots" metadata:"num_snapshots" description:"Number of snapshots in table history"`

	// Statistics
	NumRows        int64 `json:"num_rows" metadata:"num_rows" description:"Number of rows in the table"`
	FileSizeBytes  int64 `json:"file_size_bytes" metadata:"file_size_bytes" description:"Total size of data files in bytes"`
	NumDataFiles   int   `json:"num_data_files" metadata:"num_data_files" description:"Number of data files"`
	NumDeleteFiles int   `json:"num_delete_files" metadata:"num_delete_files" description:"Number of delete files"`

	// Partition information
	NumPartitions         int    `json:"num_partitions" metadata:"num_partitions" description:"Number of partitions"`
	PartitionTransformers string `json:"partition_transformers" metadata:"partition_transformers" description:"List of partition transformers used (identity, bucket, truncate, etc.)"`

	// Properties
	Properties map[string]string `json:"properties" metadata:"properties" description:"Table properties"`

	// Catalog information
	CatalogType string `json:"catalog_type" metadata:"catalog_type" description:"Type of catalog used (rest, hive, s3, adls, local)"`
	CatalogName string `json:"catalog_name" metadata:"catalog_name" description:"Name of the catalog"`

	// Sort order
	SortOrderJSON string `json:"sort_order_json" metadata:"sort_order_json" description:"JSON representation of the sort order"`

	// Maintenance
	OrphanFilesSizeBytes int64 `json:"orphan_files_size_bytes" metadata:"orphan_files_size_bytes" description:"Size of orphan files in bytes, if available"`
	MaintenanceLastRun   int64 `json:"maintenance_last_run" metadata:"maintenance_last_run" description:"Timestamp of last maintenance run in milliseconds since epoch, if available"`
}
