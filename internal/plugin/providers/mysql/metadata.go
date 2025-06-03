package mysql

// MySQLFields represents MySQL-specific metadata fields
// +marmot:metadata
type MySQLFields struct {
	Host        string `json:"host" metadata:"host" description:"MySQL server hostname"`
	Port        int    `json:"port" metadata:"port" description:"MySQL server port"`
	Database    string `json:"database" metadata:"database" description:"Database name"`
	Schema      string `json:"schema" metadata:"schema" description:"Schema name"`
	TableName   string `json:"table_name" metadata:"table_name" description:"Object name"`
	ObjectType  string `json:"object_type" metadata:"object_type" description:"Object type (table, view)"`
	Engine      string `json:"engine" metadata:"engine" description:"Storage engine"`
	Collation   string `json:"collation" metadata:"collation" description:"Table collation"`
	RowCount    int64  `json:"row_count" metadata:"row_count" description:"Approximate row count"`
	DataLength  int64  `json:"data_length" metadata:"data_length" description:"Data size in bytes"`
	IndexLength int64  `json:"index_length" metadata:"index_length" description:"Index size in bytes"`
	Created     string `json:"created" metadata:"created" description:"Creation timestamp"`
	Updated     string `json:"updated" metadata:"updated" description:"Last update timestamp"`
	Comment     string `json:"comment" metadata:"comment" description:"Object comment/description"`
	Charset     string `json:"charset" metadata:"charset" description:"Character set"`
	Version     string `json:"version" metadata:"version" description:"MySQL version"`
}

// MySQLColumnFields represents MySQL column-specific metadata fields
// +marmot:metadata
type MySQLColumnFields struct {
	ColumnName      string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType        string `json:"data_type" metadata:"data_type" description:"Data type"`
	ColumnType      string `json:"column_type" metadata:"column_type" description:"Full column type definition"`
	IsNullable      bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	ColumnDefault   string `json:"column_default" metadata:"column_default" description:"Default value"`
	IsPrimaryKey    bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether column is part of primary key"`
	IsAutoIncrement bool   `json:"is_auto_increment" metadata:"is_auto_increment" description:"Whether column auto-increments"`
	CharacterSet    string `json:"character_set" metadata:"character_set" description:"Character set"`
	Collation       string `json:"collation" metadata:"collation" description:"Collation"`
	Comment         string `json:"comment" metadata:"comment" description:"Column comment/description"`
}

// MySQLForeignKeyFields represents MySQL foreign key relationship fields
// +marmot:metadata
type MySQLForeignKeyFields struct {
	ConstraintName string `json:"constraint_name" metadata:"constraint_name" description:"Foreign key constraint name"`
	SourceSchema   string `json:"source_schema" metadata:"source_schema" description:"Schema of the referencing table"`
	SourceTable    string `json:"source_table" metadata:"source_table" description:"Name of the referencing table"`
	SourceColumn   string `json:"source_column" metadata:"source_column" description:"Column in the referencing table"`
	TargetSchema   string `json:"target_schema" metadata:"target_schema" description:"Schema of the referenced table"`
	TargetTable    string `json:"target_table" metadata:"target_table" description:"Name of the referenced table"`
	TargetColumn   string `json:"target_column" metadata:"target_column" description:"Column in the referenced table"`
	UpdateRule     string `json:"update_rule" metadata:"update_rule" description:"Update rule (CASCADE, RESTRICT, etc.)"`
	DeleteRule     string `json:"delete_rule" metadata:"delete_rule" description:"Delete rule (CASCADE, RESTRICT, etc.)"`
}
