package timescale

// TimescaleFields represents the metadata every discovered object carries.
// +marmot:metadata
type TimescaleFields struct {
	Host            string `json:"host" metadata:"host" description:"TimescaleDB server hostname"`
	Port            int    `json:"port" metadata:"port" description:"TimescaleDB server port"`
	Database        string `json:"database" metadata:"database" description:"Database name"`
	Schema          string `json:"schema" metadata:"schema" description:"Schema name"`
	TableName       string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType      string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view)"`
	Owner           string `json:"owner" metadata:"owner" description:"Object owner"`
	Comment         string `json:"comment" metadata:"comment" description:"Object comment"`
	Encoding        string `json:"encoding" metadata:"encoding" description:"Database encoding"`
	Collate         string `json:"collate" metadata:"collate" description:"Database collation"`
	CType           string `json:"ctype" metadata:"ctype" description:"Database character classification"`
	ConnectionLimit int    `json:"connection_limit" metadata:"connection_limit" description:"Maximum allowed connections to the database"`
	Size            int64  `json:"size" metadata:"size" description:"Database size in bytes"`
}

// TimescaleHypertableFields represents the metadata a table carries when
// TimescaleDB partitions it into chunks.
// +marmot:metadata
type TimescaleHypertableFields struct {
	Hypertable           bool     `json:"hypertable" metadata:"hypertable" description:"Whether the table is a hypertable"`
	TimescaleDBVersion   string   `json:"timescaledb_version" metadata:"timescaledb_version" description:"Version of the timescaledb extension"`
	TimeColumn           string   `json:"time_column" metadata:"time_column" description:"Column the hypertable is partitioned by time on"`
	TimeColumnType       string   `json:"time_column_type" metadata:"time_column_type" description:"Data type of the time column"`
	TimeInterval         string   `json:"time_interval" metadata:"time_interval" description:"Time span covered by one chunk"`
	TimeIntervalSeconds  int64    `json:"time_interval_seconds" metadata:"time_interval_seconds" description:"Time span covered by one chunk, in seconds"`
	IntegerInterval      int64    `json:"integer_interval" metadata:"integer_interval" description:"Chunk interval when the hypertable is partitioned on an integer column"`
	IntegerNowFunc       string   `json:"integer_now_func" metadata:"integer_now_func" description:"Function that returns the current value of an integer time column"`
	NumDimensions        int      `json:"num_dimensions" metadata:"num_dimensions" description:"Number of partitioning dimensions"`
	NumChunks            int64    `json:"num_chunks" metadata:"num_chunks" description:"Number of chunks the hypertable is split into"`
	SpacePartitions      []string `json:"space_partitions" metadata:"space_partitions" description:"Partitioning columns beyond time, with their partition counts"`
	ChunkRange           []string `json:"chunk_range" metadata:"chunk_range" description:"Oldest and newest chunk boundary, when include_chunks is on"`
	CompressionEnabled   bool     `json:"compression_enabled" metadata:"compression_enabled" description:"Whether compression is enabled"`
	CompressionSegmentBy []string `json:"compression_segment_by" metadata:"compression_segment_by" description:"Columns compressed batches are grouped by"`
	CompressionOrderBy   []string `json:"compression_order_by" metadata:"compression_order_by" description:"Columns rows are ordered by inside a compressed batch"`
	Policies             []string `json:"policies" metadata:"policies" description:"Background policies attached to the object (compression, retention, refresh)"`
	IsDistributed        bool     `json:"is_distributed" metadata:"is_distributed" description:"Whether the hypertable is distributed, on TimescaleDB versions that support it"`
	Tablespaces          []string `json:"tablespaces" metadata:"tablespaces" description:"Tablespaces the hypertable's chunks are placed in"`
}

// TimescaleAggregateFields represents the metadata a view carries when it is
// a continuous aggregate.
// +marmot:metadata
type TimescaleAggregateFields struct {
	ContinuousAggregate       bool     `json:"continuous_aggregate" metadata:"continuous_aggregate" description:"Whether the view is a continuous aggregate"`
	MaterializedOnly          bool     `json:"materialized_only" metadata:"materialized_only" description:"Whether the view reads only materialized data, without the most recent rows"`
	Finalized                 bool     `json:"finalized" metadata:"finalized" description:"Whether the aggregate uses the finalized form, on TimescaleDB versions that report it"`
	SourceHypertable          string   `json:"source_hypertable" metadata:"source_hypertable" description:"Hypertable the aggregate reads from"`
	MaterializationHypertable string   `json:"materialization_hypertable" metadata:"materialization_hypertable" description:"Internal hypertable the results are stored in"`
	RefreshPolicy             []string `json:"refresh_policy" metadata:"refresh_policy" description:"Policy that refreshes the aggregate"`
}

// TimescaleColumnFields represents the per-column metadata attached to an
// asset's schema.
// +marmot:metadata
type TimescaleColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Data type"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value expression"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
	Identity          string `json:"identity" metadata:"identity" description:"Identity kind: a for always, d for by default"`
	IsGenerated       bool   `json:"is_generated" metadata:"is_generated" description:"Whether the column is generated from other columns"`
}
