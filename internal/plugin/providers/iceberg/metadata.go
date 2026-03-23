package iceberg

// IcebergNamespaceFields represents Iceberg namespace metadata fields
// +marmot:metadata
type IcebergNamespaceFields struct {
	Namespace string `json:"namespace" metadata:"namespace" description:"Namespace path"`
	Location  string `json:"location" metadata:"location" description:"Default location for tables"`
}

// IcebergTableFields represents Iceberg table metadata fields
// +marmot:metadata
type IcebergTableFields struct {
	TableUUID       string `json:"table_uuid" metadata:"table_uuid" description:"Table UUID"`
	Location        string `json:"location" metadata:"location" description:"Table data location"`
	FormatVersion   int    `json:"format_version" metadata:"format_version" description:"Iceberg format version (1, 2, or 3)"`
	CurrentSnapshot string `json:"current_snapshot_id" metadata:"current_snapshot_id" description:"Current snapshot ID"`
	SnapshotCount   int    `json:"snapshot_count" metadata:"snapshot_count" description:"Number of snapshots"`
	SchemaFields    int    `json:"schema_field_count" metadata:"schema_field_count" description:"Number of schema fields"`
	PartitionSpec   string `json:"partition_spec" metadata:"partition_spec" description:"Partition specification"`
	SortOrder       string `json:"sort_order" metadata:"sort_order" description:"Sort order specification"`
	LastUpdatedMs   int64  `json:"last_updated_ms" metadata:"last_updated_ms" description:"Last update timestamp in milliseconds"`
	TotalRecords    string `json:"total_records" metadata:"total_records" description:"Total record count"`
	TotalDataFiles  string `json:"total_data_files" metadata:"total_data_files" description:"Total data file count"`
	TotalSize       string `json:"total_file_size" metadata:"total_file_size" description:"Total file size in bytes"`
}

// IcebergViewFields represents Iceberg view metadata fields
// +marmot:metadata
type IcebergViewFields struct {
	ViewUUID      string `json:"view_uuid" metadata:"view_uuid" description:"View UUID"`
	Location      string `json:"location" metadata:"location" description:"View metadata location"`
	FormatVersion int    `json:"format_version" metadata:"format_version" description:"View format version"`
	SchemaFields  int    `json:"schema_field_count" metadata:"schema_field_count" description:"Number of schema fields"`
	SQLDialect    string `json:"sql_dialect" metadata:"sql_dialect" description:"SQL dialect of the view definition"`
	SQL           string `json:"sql" metadata:"sql" description:"SQL definition of the view"`
}
