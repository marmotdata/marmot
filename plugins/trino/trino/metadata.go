package trino

// TrinoCatalogFields represents Trino catalog metadata fields
// +marmot:metadata
type TrinoCatalogFields struct {
	CatalogName string `json:"catalog_name" metadata:"catalog_name" description:"Trino catalog name"`
}

// TrinoSchemaFields represents Trino schema metadata fields
// +marmot:metadata
type TrinoSchemaFields struct {
	Catalog    string `json:"catalog" metadata:"catalog" description:"Parent catalog name"`
	SchemaName string `json:"schema_name" metadata:"schema_name" description:"Schema name"`
}

// TrinoTableFields represents Trino table/view metadata fields
// +marmot:metadata
type TrinoTableFields struct {
	Catalog   string `json:"catalog" metadata:"catalog" description:"Parent catalog name"`
	Schema    string `json:"schema" metadata:"schema" description:"Parent schema name"`
	TableName string `json:"table_name" metadata:"table_name" description:"Table or view name"`
	TableType string `json:"table_type" metadata:"table_type" description:"BASE TABLE or VIEW"`
	Comment   string `json:"comment" metadata:"comment" description:"Table comment"`
	RowCount  int64  `json:"row_count" metadata:"row_count" description:"Estimated row count"`
}

// TrinoColumnFields represents Trino column metadata fields
// +marmot:metadata
type TrinoColumnFields struct {
	ColumnName      string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType        string `json:"data_type" metadata:"data_type" description:"Column data type"`
	IsNullable      string `json:"is_nullable" metadata:"is_nullable" description:"YES or NO"`
	OrdinalPosition int    `json:"ordinal_position" metadata:"ordinal_position" description:"Column position"`
}
