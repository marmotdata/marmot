package cockroachdb

// CockroachDBDatabaseFields describes the metadata fields the plugin emits
// for database assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type CockroachDBDatabaseFields struct {
	Host          string `json:"host" metadata:"host" description:"CockroachDB node hostname"`
	Port          int    `json:"port" metadata:"port" description:"SQL port"`
	Database      string `json:"database" metadata:"database" description:"Database name"`
	Owner         string `json:"owner" metadata:"owner" description:"Database owner"`
	ServerVersion string `json:"server_version" metadata:"server_version" description:"CockroachDB version string"`
}

// CockroachDBTableFields describes the metadata fields the plugin emits for
// table and view assets.
type CockroachDBTableFields struct {
	Database          string `json:"database" metadata:"database" description:"Database name"`
	Schema            string `json:"schema" metadata:"schema" description:"Schema name"`
	TableName         string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType        string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view, foreign_table)"`
	Owner             string `json:"owner" metadata:"owner" description:"Object owner"`
	Comment           string `json:"comment" metadata:"comment" description:"Object comment"`
	Materialized      bool   `json:"materialized" metadata:"materialized" description:"Whether a view is materialized"`
	Partitioned       bool   `json:"partitioned" metadata:"partitioned" description:"Whether the table's primary index is partitioned"`
	PartitionColumns  string `json:"partition_columns" metadata:"partition_columns" description:"Columns the table is partitioned on, comma separated"`
	EstimatedRowCount int64  `json:"estimated_row_count" metadata:"estimated_row_count" description:"Row count estimated from the optimizer's table statistics"`
}

// CockroachDBColumnFields describes the per-column fields embedded in an
// asset's schema.
type CockroachDBColumnFields struct {
	ColumnName           string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType             string `json:"data_type" metadata:"data_type" description:"CockroachDB data type"`
	IsNullable           bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey         bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	Description          string `json:"description" metadata:"description" description:"Column comment"`
	DefaultExpression    string `json:"default_expression" metadata:"default_expression" description:"Default value expression"`
	GenerationExpression string `json:"generation_expression" metadata:"generation_expression" description:"Expression of a computed column"`
}
