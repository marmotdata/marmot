package clickhouse

// ClickHouseDatabaseFields represents ClickHouse database-specific metadata fields.
// +marmot:metadata
type ClickHouseDatabaseFields struct {
	Database string `json:"database" metadata:"database" description:"Database name"`
	Engine   string `json:"engine" metadata:"engine" description:"Database engine type"`
	Comment  string `json:"comment" metadata:"comment" description:"Database comment/description"`
}

// ClickHouseTableFields represents ClickHouse table-specific metadata fields.
// +marmot:metadata
type ClickHouseTableFields struct {
	Database  string `json:"database" metadata:"database" description:"Parent database name"`
	TableName string `json:"table_name" metadata:"table_name" description:"Table name"`
	Engine    string `json:"engine" metadata:"engine" description:"Table engine (MergeTree, ReplacingMergeTree, etc.)"`
	RowCount  int64  `json:"row_count" metadata:"row_count" description:"Estimated row count"`
	SizeBytes int64  `json:"size_bytes" metadata:"size_bytes" description:"Table size in bytes"`
	Comment   string `json:"comment" metadata:"comment" description:"Table comment/description"`
}

// ClickHouseColumnFields represents ClickHouse column-specific metadata fields.
// +marmot:metadata
type ClickHouseColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Column data type"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether column is part of primary key"`
	IsSortingKey      bool   `json:"is_sorting_key" metadata:"is_sorting_key" description:"Whether column is part of sorting key"`
	DefaultKind       string `json:"default_kind" metadata:"default_kind" description:"Default value kind (DEFAULT, MATERIALIZED, ALIAS)"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value expression"`
	Comment           string `json:"comment" metadata:"comment" description:"Column comment/description"`
}
