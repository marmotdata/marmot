package dbt

// DBTModelFields represents DBT model-specific metadata fields
// +marmot:metadata
type DBTModelFields struct {
	DBTUniqueID          string  `json:"dbt_unique_id" metadata:"dbt_unique_id" description:"DBT's unique identifier for this node"`
	DBTPackage           string  `json:"dbt_package" metadata:"dbt_package" description:"DBT package name"`
	DBTPath              string  `json:"dbt_path" metadata:"dbt_path" description:"Path to the model file"`
	DBTOriginalPath      string  `json:"dbt_original_path" metadata:"dbt_original_path" description:"Original path in the DBT project"`
	DBTMaterialized      string  `json:"dbt_materialized" metadata:"dbt_materialized" description:"Materialization type (table, view, incremental, ephemeral)"`
	Database             string  `json:"database" metadata:"database" description:"Target database name"`
	Schema               string  `json:"schema" metadata:"schema" description:"Target schema name"`
	ModelName            string  `json:"model_name" metadata:"model_name" description:"DBT model name"`
	TableName            string  `json:"table_name" metadata:"table_name" description:"Physical table/view name in database"`
	FullyQualifiedName   string  `json:"fully_qualified_name" metadata:"fully_qualified_name" description:"Fully qualified name (database.schema.table)"`
	ProjectName          string  `json:"project_name" metadata:"project_name" description:"DBT project name"`
	Environment          string  `json:"environment" metadata:"environment" description:"Deployment environment (dev, prod, etc)"`
	Alias                string  `json:"alias" metadata:"alias" description:"Table alias if different from model name"`
	AdapterType          string  `json:"adapter_type" metadata:"adapter_type" description:"Database adapter type (postgres, snowflake, bigquery, etc)"`
	DBTVersion           string  `json:"dbt_version" metadata:"dbt_version" description:"DBT version used to generate this model"`
	LastRunStatus        string  `json:"last_run_status" metadata:"last_run_status" description:"Status of the last DBT run (success, error, skipped)"`
	LastRunExecutionTime float64 `json:"last_run_execution_time" metadata:"last_run_execution_time" description:"Execution time of last run in seconds"`
	LastRunMessage       string  `json:"last_run_message" metadata:"last_run_message" description:"Message from last DBT run"`
	LastRunFailures      int     `json:"last_run_failures" metadata:"last_run_failures" description:"Number of failures in last run"`
	RawSQL               string  `json:"raw_sql" metadata:"raw_sql" description:"Raw SQL before compilation"`
	Owner                string  `json:"owner" metadata:"owner" description:"Table/view owner from database catalog"`
	CatalogComment       string  `json:"catalog_comment" metadata:"catalog_comment" description:"Comment from database catalog"`
}

// DBTSourceFields represents DBT source-specific metadata fields
// +marmot:metadata
type DBTSourceFields struct {
	DBTUniqueID        string `json:"dbt_unique_id" metadata:"dbt_unique_id" description:"DBT's unique identifier for this source"`
	DBTPackage         string `json:"dbt_package" metadata:"dbt_package" description:"DBT package name"`
	Database           string `json:"database" metadata:"database" description:"Source database name"`
	Schema             string `json:"schema" metadata:"schema" description:"Source schema name"`
	TableName          string `json:"table_name" metadata:"table_name" description:"Source table name"`
	FullyQualifiedName string `json:"fully_qualified_name" metadata:"fully_qualified_name" description:"Fully qualified name (database.schema.table)"`
	SourceName         string `json:"source_name" metadata:"source_name" description:"DBT source name"`
	Identifier         string `json:"identifier" metadata:"identifier" description:"Physical table identifier"`
	Loaded             bool   `json:"loaded" metadata:"loaded" description:"Whether source was loaded at time of DBT execution"`
	ProjectName        string `json:"project_name" metadata:"project_name" description:"DBT project name"`
	Environment        string `json:"environment" metadata:"environment" description:"Deployment environment"`
	FreshnessChecked   bool   `json:"freshness_checked" metadata:"freshness_checked" description:"Whether freshness checks are configured"`
}

// DBTSeedFields represents DBT seed-specific metadata fields
// +marmot:metadata
type DBTSeedFields struct {
	DBTUniqueID        string `json:"dbt_unique_id" metadata:"dbt_unique_id" description:"DBT's unique identifier for this seed"`
	DBTPackage         string `json:"dbt_package" metadata:"dbt_package" description:"DBT package name"`
	Database           string `json:"database" metadata:"database" description:"Target database name"`
	Schema             string `json:"schema" metadata:"schema" description:"Target schema name"`
	TableName          string `json:"table_name" metadata:"table_name" description:"Seed table name"`
	FullyQualifiedName string `json:"fully_qualified_name" metadata:"fully_qualified_name" description:"Fully qualified name (database.schema.table)"`
	SeedPath           string `json:"seed_path" metadata:"seed_path" description:"Path to seed CSV file"`
	ProjectName        string `json:"project_name" metadata:"project_name" description:"DBT project name"`
	Environment        string `json:"environment" metadata:"environment" description:"Deployment environment"`
}

// DBTColumnFields represents DBT column-specific metadata fields
// +marmot:metadata
type DBTColumnFields struct {
	ColumnName        string   `json:"column_name" metadata:"column_name" description:"Column name"`
	DataType          string   `json:"data_type" metadata:"data_type" description:"Column data type"`
	ColumnDescription string   `json:"column_description" metadata:"column_description" description:"Column description from DBT"`
	ColumnTags        []string `json:"column_tags" metadata:"column_tags" description:"Tags applied to this column"`
	ColumnComment     string   `json:"column_comment" metadata:"column_comment" description:"Column comment from database catalog"`
}

// DBTConfigFields represents DBT config-specific metadata fields
// Fields with config_ prefix contain DBT model configuration
// +marmot:metadata
type DBTConfigFields struct {
	ConfigEnabled       bool   `json:"config_enabled" metadata:"config_enabled" description:"Whether model is enabled"`
	ConfigMaterialized  string `json:"config_materialized" metadata:"config_materialized" description:"Materialization strategy from config"`
	ConfigTags          string `json:"config_tags" metadata:"config_tags" description:"Tags from config"`
	ConfigPersistDocs   bool   `json:"config_persist_docs" metadata:"config_persist_docs" description:"Whether to persist documentation to database"`
	ConfigFullRefresh   bool   `json:"config_full_refresh" metadata:"config_full_refresh" description:"Whether to perform full refresh"`
	ConfigOnSchemaChange string `json:"config_on_schema_change" metadata:"config_on_schema_change" description:"Behavior when schema changes (append_new_columns, fail, ignore)"`
}

// DBTStatsFields represents DBT catalog statistics fields
// Fields with stat_ prefix contain statistics from database catalog
// +marmot:metadata
type DBTStatsFields struct {
	StatRowCount         int64   `json:"stat_row_count" metadata:"stat_row_count" description:"Number of rows"`
	StatBytes            int64   `json:"stat_bytes" metadata:"stat_bytes" description:"Size in bytes"`
	StatLastModified     string  `json:"stat_last_modified" metadata:"stat_last_modified" description:"Last modification timestamp"`
	StatNumRows          int64   `json:"stat_num_rows" metadata:"stat_num_rows" description:"Number of rows (alternative)"`
	StatApproximateCount int64   `json:"stat_approximate_count" metadata:"stat_approximate_count" description:"Approximate row count"`
	StatSize             float64 `json:"stat_size" metadata:"stat_size" description:"Table size"`
}
