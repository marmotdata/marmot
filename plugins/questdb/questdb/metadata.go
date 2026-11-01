package questdb

// QuestDBFields describes the metadata fields the QuestDB plugin emits for
// table and view assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type QuestDBFields struct {
	Host                string `json:"host" metadata:"host" description:"QuestDB server hostname"`
	Port                int    `json:"port" metadata:"port" description:"PostgreSQL wire protocol port"`
	TableName           string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType          string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view)"`
	DesignatedTimestamp string `json:"designated_timestamp" metadata:"designated_timestamp" description:"Column the table is ordered and partitioned by"`
	PartitionBy         string `json:"partition_by" metadata:"partition_by" description:"Partition interval (NONE, HOUR, DAY, WEEK, MONTH, YEAR)"`
	PartitionCount      int64  `json:"partition_count" metadata:"partition_count" description:"Number of partitions on disk"`
	WALEnabled          bool   `json:"wal_enabled" metadata:"wal_enabled" description:"Whether writes go through the write-ahead log"`
	Dedup               bool   `json:"dedup" metadata:"dedup" description:"Whether deduplication on upsert keys is enabled"`
	TTL                 string `json:"ttl" metadata:"ttl" description:"Retention period, for example 1 WEEK"`
	MaxUncommittedRows  int64  `json:"max_uncommitted_rows" metadata:"max_uncommitted_rows" description:"Rows buffered before an out-of-order commit"`
	O3MaxLag            int64  `json:"o3_max_lag" metadata:"o3_max_lag" description:"Out-of-order commit lag in microseconds"`
	Materialized        bool   `json:"materialized" metadata:"materialized" description:"Whether a view is materialized"`
	BaseTable           string `json:"base_table" metadata:"base_table" description:"Table a materialized view is built from"`
	RefreshType         string `json:"refresh_type" metadata:"refresh_type" description:"Materialized view refresh type (immediate, timer, manual, period)"`
	RefreshPeriod       string `json:"refresh_period" metadata:"refresh_period" description:"Materialized view refresh interval, for example 1 HOUR"`
	LastRefresh         string `json:"last_refresh" metadata:"last_refresh" description:"When the materialized view last started refreshing"`
}

// QuestDBColumnFields describes the per-column fields embedded in an asset's
// schema.
type QuestDBColumnFields struct {
	ColumnName          string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType            string `json:"data_type" metadata:"data_type" description:"QuestDB data type as reported, for example SYMBOL or GEOHASH(8c)"`
	IsNullable          bool   `json:"is_nullable" metadata:"is_nullable" description:"Always true, QuestDB columns are nullable"`
	DesignatedTimestamp bool   `json:"designated_timestamp" metadata:"designated_timestamp" description:"Whether this is the designated timestamp column"`
	Indexed             bool   `json:"indexed" metadata:"indexed" description:"Whether the symbol column has a bitmap index"`
	SymbolCapacity      int64  `json:"symbol_capacity" metadata:"symbol_capacity" description:"Distinct values a symbol column is sized for"`
	SymbolCached        bool   `json:"symbol_cached" metadata:"symbol_cached" description:"Whether the symbol table is cached in memory"`
	UpsertKey           bool   `json:"upsert_key" metadata:"upsert_key" description:"Whether the column is a deduplication upsert key"`
}
