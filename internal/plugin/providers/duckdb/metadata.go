package duckdb

// DuckDBFields represents DuckDB-specific metadata fields.
// +marmot:metadata
type DuckDBFields struct {
	Path       string `json:"path" metadata:"path" description:"Path to the DuckDB database file"`
	Schema     string `json:"schema" metadata:"schema" description:"Schema name"`
	TableName  string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType string `json:"object_type" metadata:"object_type" description:"Object type (BASE TABLE, VIEW)"`
	RowCount   int64  `json:"row_count" metadata:"row_count" description:"Estimated row count"`
	Size       int64  `json:"size" metadata:"size" description:"Estimated size in bytes"`
	Comment    string `json:"comment" metadata:"comment" description:"Object comment/description"`
}

// DuckDBColumnFields represents DuckDB column-specific metadata fields.
// +marmot:metadata
type DuckDBColumnFields struct {
	ColumnName    string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType      string `json:"data_type" metadata:"data_type" description:"Column data type"`
	IsNullable    bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	ColumnDefault string `json:"column_default" metadata:"column_default" description:"Default value expression"`
}

// DuckDBForeignKeyFields represents DuckDB foreign key relationship fields.
// +marmot:metadata
type DuckDBForeignKeyFields struct {
	ConstraintName string `json:"constraint_name" metadata:"constraint_name" description:"Foreign key constraint name"`
	SourceSchema   string `json:"source_schema" metadata:"source_schema" description:"Schema of the referencing table"`
	SourceTable    string `json:"source_table" metadata:"source_table" description:"Name of the referencing table"`
	SourceColumn   string `json:"source_column" metadata:"source_column" description:"Column in the referencing table"`
	TargetSchema   string `json:"target_schema" metadata:"target_schema" description:"Schema of the referenced table"`
	TargetTable    string `json:"target_table" metadata:"target_table" description:"Name of the referenced table"`
	TargetColumn   string `json:"target_column" metadata:"target_column" description:"Column in the referenced table"`
}
