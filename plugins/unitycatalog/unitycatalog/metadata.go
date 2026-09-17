package unitycatalog

// UnityCatalogCatalogFields describes the metadata fields emitted for
// catalog assets. It is kept as a documentation-only struct so downstream
// tooling can introspect the shape of the metadata map.
type UnityCatalogCatalogFields struct {
	CatalogName string            `json:"catalog_name" metadata:"catalog_name" description:"Catalog name"`
	CatalogID   string            `json:"catalog_id" metadata:"catalog_id" description:"Catalog id"`
	Comment     string            `json:"comment" metadata:"comment" description:"Catalog comment"`
	Owner       string            `json:"owner" metadata:"owner" description:"Catalog owner"`
	Properties  map[string]string `json:"properties" metadata:"properties" description:"User-defined properties"`
	SchemaCount int               `json:"schema_count" metadata:"schema_count" description:"Number of schemas in the catalog"`
	TableCount  int               `json:"table_count" metadata:"table_count" description:"Number of tables and views in the catalog"`
	CreatedAt   string            `json:"created_at" metadata:"created_at" description:"Creation time (RFC 3339)"`
	UpdatedAt   string            `json:"updated_at" metadata:"updated_at" description:"Last update time (RFC 3339)"`
}

// UnityCatalogTableFields describes the metadata fields emitted for table
// and view assets.
type UnityCatalogTableFields struct {
	Catalog          string            `json:"catalog" metadata:"catalog" description:"Catalog the table belongs to"`
	Schema           string            `json:"schema" metadata:"schema" description:"Schema the table belongs to"`
	TableName        string            `json:"table_name" metadata:"table_name" description:"Table or view name"`
	TableType        string            `json:"table_type" metadata:"table_type" description:"Table type (MANAGED, EXTERNAL, VIEW, MATERIALIZED_VIEW, STREAMING_TABLE, FOREIGN)"`
	DataSourceFormat string            `json:"data_source_format" metadata:"data_source_format" description:"Data source format (DELTA, PARQUET, CSV, ...)"`
	StorageLocation  string            `json:"storage_location" metadata:"storage_location" description:"Storage location of the table data"`
	Owner            string            `json:"owner" metadata:"owner" description:"Table owner"`
	Comment          string            `json:"comment" metadata:"comment" description:"Table comment"`
	Properties       map[string]string `json:"properties" metadata:"properties" description:"User-defined properties"`
	TableID          string            `json:"table_id" metadata:"table_id" description:"Table id"`
	PartitionColumns []string          `json:"partition_columns" metadata:"partition_columns" description:"Partition columns in partition order"`
	PrimaryKey       []string          `json:"primary_key" metadata:"primary_key" description:"Primary key columns"`
	ForeignKeys      []map[string]any  `json:"foreign_keys" metadata:"foreign_keys" description:"Foreign key constraints (columns, parent_table, parent_columns)"`
	CreatedAt        string            `json:"created_at" metadata:"created_at" description:"Creation time (RFC 3339)"`
	UpdatedAt        string            `json:"updated_at" metadata:"updated_at" description:"Last update time (RFC 3339)"`
}

// UnityCatalogColumnFields describes the per-column fields embedded in an
// asset's schema.
type UnityCatalogColumnFields struct {
	ColumnName     string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType       string `json:"data_type" metadata:"data_type" description:"Column type as written (type_text)"`
	TypeName       string `json:"type_name" metadata:"type_name" description:"Column type name (INT, STRING, DECIMAL, ...)"`
	IsNullable     bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey   bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	PartitionIndex int    `json:"partition_index" metadata:"partition_index" description:"Position in the partition key, for partition columns"`
	Description    string `json:"description" metadata:"description" description:"Column comment"`
}

// UnityCatalogVolumeFields describes the metadata fields emitted for
// volume assets.
type UnityCatalogVolumeFields struct {
	Catalog         string `json:"catalog" metadata:"catalog" description:"Catalog the volume belongs to"`
	Schema          string `json:"schema" metadata:"schema" description:"Schema the volume belongs to"`
	VolumeName      string `json:"volume_name" metadata:"volume_name" description:"Volume name"`
	VolumeType      string `json:"volume_type" metadata:"volume_type" description:"Volume type (MANAGED, EXTERNAL)"`
	StorageLocation string `json:"storage_location" metadata:"storage_location" description:"Storage location of the volume"`
	Comment         string `json:"comment" metadata:"comment" description:"Volume comment"`
	Owner           string `json:"owner" metadata:"owner" description:"Volume owner"`
	VolumeID        string `json:"volume_id" metadata:"volume_id" description:"Volume id"`
	CreatedAt       string `json:"created_at" metadata:"created_at" description:"Creation time (RFC 3339)"`
	UpdatedAt       string `json:"updated_at" metadata:"updated_at" description:"Last update time (RFC 3339)"`
}

// UnityCatalogFunctionFields describes the metadata fields emitted for
// function assets.
type UnityCatalogFunctionFields struct {
	Catalog         string   `json:"catalog" metadata:"catalog" description:"Catalog the function belongs to"`
	Schema          string   `json:"schema" metadata:"schema" description:"Schema the function belongs to"`
	FunctionName    string   `json:"function_name" metadata:"function_name" description:"Function name"`
	Parameters      []string `json:"parameters" metadata:"parameters" description:"Input parameters as \"name type\", in declaration order"`
	ReturnType      string   `json:"return_type" metadata:"return_type" description:"Return type"`
	RoutineBody     string   `json:"routine_body" metadata:"routine_body" description:"Routine body kind (SQL, EXTERNAL)"`
	Language        string   `json:"language" metadata:"language" description:"Language the body is written in (SQL, python, ...)"`
	IsDeterministic bool     `json:"is_deterministic" metadata:"is_deterministic" description:"Whether the function is deterministic"`
	SQLDataAccess   string   `json:"sql_data_access" metadata:"sql_data_access" description:"SQL data access (CONTAINS_SQL, READS_SQL_DATA, NO_SQL)"`
	Comment         string   `json:"comment" metadata:"comment" description:"Function comment"`
	Owner           string   `json:"owner" metadata:"owner" description:"Function owner"`
	FunctionID      string   `json:"function_id" metadata:"function_id" description:"Function id"`
	CreatedAt       string   `json:"created_at" metadata:"created_at" description:"Creation time (RFC 3339)"`
	UpdatedAt       string   `json:"updated_at" metadata:"updated_at" description:"Last update time (RFC 3339)"`
}

// UnityCatalogModelFields describes the metadata fields emitted for
// registered model assets.
type UnityCatalogModelFields struct {
	Catalog         string            `json:"catalog" metadata:"catalog" description:"Catalog the model belongs to"`
	Schema          string            `json:"schema" metadata:"schema" description:"Schema the model belongs to"`
	ModelName       string            `json:"model_name" metadata:"model_name" description:"Model name"`
	Comment         string            `json:"comment" metadata:"comment" description:"Model comment"`
	Owner           string            `json:"owner" metadata:"owner" description:"Model owner"`
	StorageLocation string            `json:"storage_location" metadata:"storage_location" description:"Storage location of the model artifacts"`
	ModelID         string            `json:"model_id" metadata:"model_id" description:"Model id"`
	VersionCount    int               `json:"version_count" metadata:"version_count" description:"Number of registered versions"`
	LatestVersion   int               `json:"latest_version" metadata:"latest_version" description:"Highest registered version number"`
	Versions        map[string]string `json:"versions" metadata:"versions" description:"Version number to status (READY, PENDING_REGISTRATION, ...)"`
	CreatedAt       string            `json:"created_at" metadata:"created_at" description:"Creation time (RFC 3339)"`
	UpdatedAt       string            `json:"updated_at" metadata:"updated_at" description:"Last update time (RFC 3339)"`
}
