package postgresql

// PostgresFields represents PostgreSQL-specific metadata fields
// +marmot:metadata
type PostgresFields struct {
	Host             string `json:"host" metadata:"host" description:"PostgreSQL server hostname"`
	Port             int    `json:"port" metadata:"port" description:"PostgreSQL server port"`
	Database         string `json:"database" metadata:"database" description:"Database name"`
	Schema           string `json:"schema" metadata:"schema" description:"Schema name"`
	TableName        string `json:"table_name" metadata:"table_name" description:"Object name"`
	ObjectType       string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view)"`
	Owner            string `json:"owner" metadata:"owner" description:"Object owner"`
	Size             int64  `json:"size" metadata:"size" description:"Object size in bytes"`
	RowCount         int64  `json:"row_count" metadata:"row_count" description:"Approximate row count"`
	Created          string `json:"created" metadata:"created" description:"Creation timestamp"`
	Comment          string `json:"comment" metadata:"comment" description:"Object comment/description"`
	Encoding         string `json:"encoding" metadata:"encoding" description:"Database encoding"`
	Collate          string `json:"collate" metadata:"collate" description:"Database collation"`
	CType            string `json:"ctype" metadata:"ctype" description:"Database character classification"`
	IsTemplate       bool   `json:"is_template" metadata:"is_template" description:"Whether database is a template"`
	AllowConnections bool   `json:"allow_connections" metadata:"allow_connections" description:"Whether connections to this database are allowed"`
	ConnectionLimit  int    `json:"connection_limit" metadata:"connection_limit" description:"Maximum allowed connections"`
}

// PostgresColumnFields represents PostgreSQL column-specific metadata fields
// +marmot:metadata
type PostgresColumnFields struct {
	ColumnName    string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType      string `json:"data_type" metadata:"data_type" description:"Data type"`
	IsNullable    bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	ColumnDefault string `json:"column_default" metadata:"column_default" description:"Default value expression"`
	IsPrimaryKey  bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether column is part of primary key"`
	Comment       string `json:"comment" metadata:"comment" description:"Column comment/description"`
}

// PostgresForeignKeyFields represents PostgreSQL foreign key relationship fields
// +marmot:metadata
type PostgresForeignKeyFields struct {
	ConstraintName string `json:"constraint_name" metadata:"constraint_name" description:"Foreign key constraint name"`
	SourceSchema   string `json:"source_schema" metadata:"source_schema" description:"Schema of the referencing table"`
	SourceTable    string `json:"source_table" metadata:"source_table" description:"Name of the referencing table"`
	SourceColumn   string `json:"source_column" metadata:"source_column" description:"Column in the referencing table"`
	TargetSchema   string `json:"target_schema" metadata:"target_schema" description:"Schema of the referenced table"`
	TargetTable    string `json:"target_table" metadata:"target_table" description:"Name of the referenced table"`
	TargetColumn   string `json:"target_column" metadata:"target_column" description:"Column in the referenced table"`
}
