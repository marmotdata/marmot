package pinot

// PinotTableFields describes the metadata fields the Pinot plugin emits
// for table assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type PinotTableFields struct {
	TableName            string   `json:"table_name" metadata:"table_name" description:"Logical table name, without the _OFFLINE or _REALTIME suffix"`
	TableTypes           []string `json:"table_types" metadata:"table_types" description:"Table types present: OFFLINE, REALTIME or both for a hybrid table"`
	IngestionType        string   `json:"ingestion_type" metadata:"ingestion_type" description:"How data arrives: batch, stream or hybrid"`
	SchemaName           string   `json:"schema_name" metadata:"schema_name" description:"Name of the Pinot schema the table config points at, when it differs from the table name"`
	TimeColumn           string   `json:"time_column" metadata:"time_column" description:"Column segments are partitioned and retained by"`
	TimeType             string   `json:"time_type" metadata:"time_type" description:"Unit of the time column, for example DAYS or MILLISECONDS"`
	Replication          string   `json:"replication" metadata:"replication" description:"Number of replicas per segment"`
	Retention            string   `json:"retention" metadata:"retention" description:"How long segments are kept, as value and unit (for example 30 DAYS)"`
	BrokerTenant         string   `json:"broker_tenant" metadata:"broker_tenant" description:"Broker tenant serving the table"`
	ServerTenant         string   `json:"server_tenant" metadata:"server_tenant" description:"Server tenant hosting the table"`
	LoadMode             string   `json:"load_mode" metadata:"load_mode" description:"How segments are loaded on servers (MMAP or HEAP)"`
	IsDimTable           bool     `json:"is_dim_table" metadata:"is_dim_table" description:"Whether the table is a dimension table replicated to every server"`
	PrimaryKeyColumns    []string `json:"primary_key_columns" metadata:"primary_key_columns" description:"Primary key columns declared in the schema"`
	SortedColumn         string   `json:"sorted_column" metadata:"sorted_column" description:"Column segments are sorted by"`
	InvertedIndexColumns []string `json:"inverted_index_columns" metadata:"inverted_index_columns" description:"Columns with an inverted index"`
	SegmentCount         int      `json:"segment_count" metadata:"segment_count" description:"Total number of segments across table types"`
	OfflineSegmentCount  int      `json:"offline_segment_count" metadata:"offline_segment_count" description:"Number of offline segments"`
	RealtimeSegmentCount int      `json:"realtime_segment_count" metadata:"realtime_segment_count" description:"Number of realtime segments, including the consuming one"`
	StreamType           string   `json:"stream_type" metadata:"stream_type" description:"Stream type a realtime table consumes from, for example kafka or kinesis"`
	StreamTopic          string   `json:"stream_topic" metadata:"stream_topic" description:"Topic or stream name a realtime table consumes from"`
	StreamBrokers        string   `json:"stream_brokers" metadata:"stream_brokers" description:"Broker list a realtime table consumes from"`
	PinotVersion         string   `json:"pinot_version" metadata:"pinot_version" description:"Pinot release reported by the controller"`
	URL                  string   `json:"url" metadata:"url" description:"Link to the table in the controller UI"`
}

// PinotColumnFields describes the per-column fields embedded in an
// asset's schema.
type PinotColumnFields struct {
	ColumnName       string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType         string `json:"data_type" metadata:"data_type" description:"Pinot data type; multi-value columns carry a [] suffix"`
	IsNullable       bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed (true unless the schema marks the field notNull)"`
	IsPrimaryKey     bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	FieldType        string `json:"field_type" metadata:"field_type" description:"Role of the field in the schema: dimension, metric or datetime"`
	SingleValue      bool   `json:"single_value" metadata:"single_value" description:"Whether each row holds one value rather than an array"`
	Format           string `json:"format" metadata:"format" description:"Date-time format, for example 1:DAYS:EPOCH or TIMESTAMP"`
	Granularity      string `json:"granularity" metadata:"granularity" description:"Date-time granularity, for example 1:SECONDS"`
	DefaultNullValue any    `json:"default_null_value" metadata:"default_null_value" description:"Value stored in place of null"`
}

// PinotStreamFields describes the metadata fields emitted on the Kafka
// topic or Kinesis stream asset that feeds a realtime table.
type PinotStreamFields struct {
	StreamType    string `json:"stream_type" metadata:"stream_type" description:"Stream type: kafka or kinesis"`
	StreamBrokers string `json:"stream_brokers" metadata:"stream_brokers" description:"Broker list the table consumes from"`
}
