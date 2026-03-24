package deltalake

// DeltaLakeTableFields represents Delta Lake table metadata fields
// +marmot:metadata
type DeltaLakeTableFields struct {
	TableID          string `json:"table_id" metadata:"table_id" description:"Delta table unique identifier"`
	Location         string `json:"location" metadata:"location" description:"Table directory path"`
	Format           string `json:"format" metadata:"format" description:"Data format (e.g. parquet)"`
	MinReaderVersion int    `json:"min_reader_version" metadata:"min_reader_version" description:"Minimum reader protocol version"`
	MinWriterVersion int    `json:"min_writer_version" metadata:"min_writer_version" description:"Minimum writer protocol version"`
	PartitionColumns string `json:"partition_columns" metadata:"partition_columns" description:"Comma-separated partition column names"`
	SchemaFieldCount int    `json:"schema_field_count" metadata:"schema_field_count" description:"Number of schema fields"`
	CreatedTime      int64  `json:"created_time" metadata:"created_time" description:"Table creation timestamp in milliseconds"`
	NumFiles         int    `json:"num_files" metadata:"num_files" description:"Number of active data files"`
	TotalSize        int64  `json:"total_size" metadata:"total_size" description:"Total size of active data files in bytes"`
	CurrentVersion   int64  `json:"current_version" metadata:"current_version" description:"Current Delta log version"`
}
