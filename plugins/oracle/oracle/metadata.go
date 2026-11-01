package oracle

// OracleDatabaseFields describes the metadata fields the Oracle plugin
// emits for the Database asset created per schema. Oracle users treat a
// schema as the database, so the asset is named after the schema.
type OracleDatabaseFields struct {
	Host                  string `json:"host" metadata:"host" description:"Oracle listener hostname"`
	Port                  int    `json:"port" metadata:"port" description:"Oracle listener port"`
	Schema                string `json:"schema" metadata:"schema" description:"Schema (user) that owns the objects"`
	ServiceName           string `json:"service_name" metadata:"service_name" description:"Service name the plugin connected to"`
	SID                   string `json:"sid" metadata:"sid" description:"SID the plugin connected to, when configured"`
	DBName                string `json:"db_name" metadata:"db_name" description:"Database name (USERENV DB_NAME)"`
	Container             string `json:"container" metadata:"container" description:"Container or pluggable database name (USERENV CON_NAME)"`
	OracleVersion         string `json:"oracle_version" metadata:"oracle_version" description:"Oracle version banner"`
	Created               string `json:"created" metadata:"created" description:"When the schema user was created"`
	TableCount            int    `json:"table_count" metadata:"table_count" description:"Number of tables in the schema"`
	ViewCount             int    `json:"view_count" metadata:"view_count" description:"Number of views in the schema"`
	MaterializedViewCount int    `json:"materialized_view_count" metadata:"materialized_view_count" description:"Number of materialized views in the schema"`
}

// OracleTableFields describes the metadata fields emitted for table, view
// and materialized view assets.
type OracleTableFields struct {
	Host          string `json:"host" metadata:"host" description:"Oracle listener hostname"`
	Port          int    `json:"port" metadata:"port" description:"Oracle listener port"`
	Schema        string `json:"schema" metadata:"schema" description:"Schema that owns the object"`
	TableName     string `json:"table_name" metadata:"table_name" description:"Object name as stored by Oracle"`
	ObjectType    string `json:"object_type" metadata:"object_type" description:"Object type (table, view, materialized_view)"`
	Tablespace    string `json:"tablespace" metadata:"tablespace" description:"Tablespace holding the table"`
	Partitioned   bool   `json:"partitioned" metadata:"partitioned" description:"Whether the table is partitioned"`
	Temporary     bool   `json:"temporary" metadata:"temporary" description:"Whether the table is a global temporary table"`
	IOT           bool   `json:"iot" metadata:"iot" description:"Whether the table is index-organized"`
	Compression   string `json:"compression" metadata:"compression" description:"Table compression setting"`
	NumRows       int64  `json:"num_rows" metadata:"num_rows" description:"Row count from optimizer statistics"`
	LastAnalyzed  string `json:"last_analyzed" metadata:"last_analyzed" description:"When optimizer statistics were last gathered"`
	Comment       string `json:"comment" metadata:"comment" description:"Table or view comment"`
	TextLength    int64  `json:"text_length" metadata:"text_length" description:"Length of the view definition"`
	Materialized  bool   `json:"materialized" metadata:"materialized" description:"Whether the view is materialized"`
	RefreshMode   string `json:"refresh_mode" metadata:"refresh_mode" description:"Materialized view refresh mode (DEMAND, COMMIT)"`
	RefreshMethod string `json:"refresh_method" metadata:"refresh_method" description:"Materialized view refresh method (COMPLETE, FAST, FORCE)"`
	BuildMode     string `json:"build_mode" metadata:"build_mode" description:"Materialized view build mode (IMMEDIATE, DEFERRED)"`
	LastRefresh   string `json:"last_refresh" metadata:"last_refresh" description:"When the materialized view was last refreshed"`
	Staleness     string `json:"staleness" metadata:"staleness" description:"Materialized view staleness (FRESH, STALE, NEEDS_COMPILE)"`
}

// OracleColumnFields describes the per-column fields embedded in an asset's
// schema.
type OracleColumnFields struct {
	ColumnName        string `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string `json:"data_type" metadata:"data_type" description:"Data type as DESCRIBE shows it, for example NUMBER(10,2) or VARCHAR2(50 CHAR)"`
	IsNullable        bool   `json:"is_nullable" metadata:"is_nullable" description:"Whether null values are allowed"`
	IsPrimaryKey      bool   `json:"is_primary_key" metadata:"is_primary_key" description:"Whether the column is part of the primary key"`
	Description       string `json:"description" metadata:"description" description:"Column comment"`
	DefaultExpression string `json:"default_expression" metadata:"default_expression" description:"Default value expression"`
	IsVirtual         bool   `json:"is_virtual" metadata:"is_virtual" description:"Whether the column is computed from an expression"`
	IsIdentity        bool   `json:"is_identity" metadata:"is_identity" description:"Whether the column is an identity column"`
}

// OracleFunctionFields describes the metadata fields emitted for procedure,
// function and package assets.
type OracleFunctionFields struct {
	Host        string `json:"host" metadata:"host" description:"Oracle listener hostname"`
	Port        int    `json:"port" metadata:"port" description:"Oracle listener port"`
	Schema      string `json:"schema" metadata:"schema" description:"Schema that owns the object"`
	ObjectName  string `json:"object_name" metadata:"object_name" description:"Object name as stored by Oracle"`
	ObjectType  string `json:"object_type" metadata:"object_type" description:"Object type (procedure, function, package)"`
	Status      string `json:"status" metadata:"status" description:"Compilation status (VALID, INVALID)"`
	Created     string `json:"created" metadata:"created" description:"When the object was created"`
	LastDDLTime string `json:"last_ddl_time" metadata:"last_ddl_time" description:"When the object was last changed"`
}
