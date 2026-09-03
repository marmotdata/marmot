package sqlite

// SQLiteFields describes the metadata fields the SQLite plugin emits for
// table and view assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type SQLiteFields struct {
	Path       string `json:"path" metadata:"path" description:"Path to the SQLite database file"`
	TableName  string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	ObjectType string `json:"object_type" metadata:"object_type" description:"Object type (table, view)"`
}

// SQLiteColumnFields describes the per-column fields embedded in an asset's
// schema.
type SQLiteColumnFields struct {
	ColumnName    string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType      string `json:"data_type" metadata:"data_type" description:"Declared column data type"`
	IsNullable    bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey  bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	ColumnDefault string `json:"column_default" metadata:"column_default" description:"Default value expression"`
}

// SQLiteForeignKeyFields describes the fields of a discovered foreign key
// relationship.
type SQLiteForeignKeyFields struct {
	SourceTable  string `json:"source_table" metadata:"source_table" description:"Name of the referencing table"`
	SourceColumn string `json:"source_column" metadata:"source_column" description:"Column in the referencing table"`
	TargetTable  string `json:"target_table" metadata:"target_table" description:"Name of the referenced table"`
	TargetColumn string `json:"target_column" metadata:"target_column" description:"Column in the referenced table"`
}
