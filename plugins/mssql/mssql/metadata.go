package mssql

// MSSQLDatabaseFields describes the metadata fields the SQL Server plugin
// emits for database assets. It is kept as a documentation-only struct so
// downstream tooling can introspect the shape of the metadata map.
type MSSQLDatabaseFields struct {
	Host          string `json:"host" metadata:"host" description:"SQL Server hostname or IP address"`
	Port          int    `json:"port" metadata:"port" description:"SQL Server port"`
	Database      string `json:"database" metadata:"database" description:"Database name"`
	DatabaseID    int    `json:"database_id" metadata:"database_id" description:"Database id within the instance"`
	Collation     string `json:"collation" metadata:"collation" description:"Database collation"`
	RecoveryModel string `json:"recovery_model" metadata:"recovery_model" description:"Recovery model (SIMPLE, FULL, BULK_LOGGED)"`
	State         string `json:"state" metadata:"state" description:"Database state, always ONLINE for a discovered database"`
	Created       string `json:"created" metadata:"created" description:"When the database was created"`
	Owner         string `json:"owner" metadata:"owner" description:"Login that owns the database"`
	ServerVersion string `json:"server_version" metadata:"server_version" description:"Instance product version"`
	Edition       string `json:"edition" metadata:"edition" description:"Instance edition"`
	SchemaCount   int    `json:"schema_count" metadata:"schema_count" description:"Number of discovered schemas"`
	TableCount    int    `json:"table_count" metadata:"table_count" description:"Number of discovered tables"`
	ViewCount     int    `json:"view_count" metadata:"view_count" description:"Number of discovered views"`
}

// MSSQLTableFields describes the metadata fields emitted for table and view
// assets.
type MSSQLTableFields struct {
	Host          string `json:"host" metadata:"host" description:"SQL Server hostname or IP address"`
	Port          int    `json:"port" metadata:"port" description:"SQL Server port"`
	Database      string `json:"database" metadata:"database" description:"Database holding the object"`
	Schema        string `json:"schema" metadata:"schema" description:"Schema holding the object"`
	TableName     string `json:"table_name" metadata:"table_name" description:"Table or view name, without the database and schema"`
	ObjectType    string `json:"object_type" metadata:"object_type" description:"Object type (user_table, view)"`
	Comment       string `json:"comment" metadata:"comment" description:"MS_Description extended property on the object"`
	SchemaComment string `json:"schema_comment" metadata:"schema_comment" description:"MS_Description extended property on the schema"`
	Created       string `json:"created" metadata:"created" description:"When the object was created"`
	Modified      string `json:"modified" metadata:"modified" description:"When the object was last altered"`
}

// MSSQLFunctionFields describes the metadata fields emitted for stored
// procedure and function assets.
type MSSQLFunctionFields struct {
	Host       string `json:"host" metadata:"host" description:"SQL Server hostname or IP address"`
	Port       int    `json:"port" metadata:"port" description:"SQL Server port"`
	Database   string `json:"database" metadata:"database" description:"Database holding the routine"`
	Schema     string `json:"schema" metadata:"schema" description:"Schema holding the routine"`
	ObjectType string `json:"object_type" metadata:"object_type" description:"Routine type (stored_procedure, scalar_function, inline_table_function, table_function)"`
	Created    string `json:"created" metadata:"created" description:"When the routine was created"`
	Modified   string `json:"modified" metadata:"modified" description:"When the routine was last altered"`
	Encrypted  bool   `json:"encrypted" metadata:"encrypted" description:"Whether the routine body is encrypted and so unreadable"`
}

// MSSQLColumnFields describes the per-column fields embedded in an asset's
// schema.
type MSSQLColumnFields struct {
	ColumnName         string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType           string `json:"data_type" metadata:"data_type" description:"Column type as SQL Server declares it, for example nvarchar(100) or decimal(10,2)"`
	IsNullable         bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey       bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	Description        string `json:"description" metadata:"description" description:"MS_Description extended property on the column"`
	DefaultExpression  string `json:"default_expression" metadata:"default_expression" description:"Default constraint expression"`
	IsIdentity         bool   `json:"is_identity" metadata:"is_identity" description:"Whether the column is an IDENTITY column"`
	IdentitySeed       int64  `json:"identity_seed" metadata:"identity_seed" description:"First value an IDENTITY column produces"`
	IdentityIncrement  int64  `json:"identity_increment" metadata:"identity_increment" description:"Step between IDENTITY values"`
	IsComputed         bool   `json:"is_computed" metadata:"is_computed" description:"Whether the column is computed from other columns"`
	IsPersisted        bool   `json:"is_persisted" metadata:"is_persisted" description:"Whether a computed column is stored rather than evaluated on read"`
	ComputedDefinition string `json:"computed_definition" metadata:"computed_definition" description:"Expression a computed column is derived from"`
	Collation          string `json:"collation" metadata:"collation" description:"Column collation for text types"`
}
