package dbt

// PostgresAdapter handles PostgreSQL
type PostgresAdapter struct {
	BaseAdapter
}

// Name is "PostgreSQL", not "Postgres": plugins/postgresql passes
// "PostgreSQL" to mrn.New, and a different word is a different asset.
func (a *PostgresAdapter) Name() string {
	return "PostgreSQL"
}

// MRNName matches plugins/postgresql/postgresql/source.go, which
// identifies a table as schema.table.
func (a *PostgresAdapter) MRNName(_, schema, table string) string {
	return joinNonEmpty(schema, table)
}

// AlloyDBAdapter handles Google AlloyDB
type AlloyDBAdapter struct {
	BaseAdapter
}

func (a *AlloyDBAdapter) Name() string {
	return "AlloyDB"
}

// MySQLAdapter handles MySQL
type MySQLAdapter struct {
	BaseAdapter
}

func (a *MySQLAdapter) Name() string {
	return "MySQL"
}

// MRNName matches plugins/mysql/mysql/source.go, which identifies a table
// as database.table. dbt fills MySQL's schema field with the database.
func (a *MySQLAdapter) MRNName(_, schema, table string) string {
	return joinNonEmpty(schema, table)
}

// SQLServerAdapter handles Microsoft SQL Server
type SQLServerAdapter struct {
	BaseAdapter
}

func (a *SQLServerAdapter) Name() string {
	return "SQLServer"
}

// SnowflakeAdapter handles Snowflake Data Cloud
type SnowflakeAdapter struct {
	BaseAdapter
}

func (a *SnowflakeAdapter) Name() string {
	return "Snowflake"
}

func (a *SnowflakeAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "View"
	case "dynamic_table":
		return "Dynamic Table"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// BigQueryAdapter handles Google BigQuery
type BigQueryAdapter struct {
	BaseAdapter
}

func (a *BigQueryAdapter) Name() string {
	return "BigQuery"
}

// MRNName matches plugins/bigquery/bigquery/source.go, which identifies a
// table as dataset.table. dbt's database field is the GCP project, which
// the BigQuery plugin deliberately leaves out of the identity because it
// takes one required project_id.
func (a *BigQueryAdapter) MRNName(_, dataset, table string) string {
	return joinNonEmpty(dataset, table)
}

// RedshiftAdapter handles Amazon Redshift
type RedshiftAdapter struct {
	BaseAdapter
}

func (a *RedshiftAdapter) Name() string {
	return "Redshift"
}

func (a *RedshiftAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "View"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
}

// SynapseAdapter handles Azure Synapse Analytics
type SynapseAdapter struct {
	BaseAdapter
}

func (a *SynapseAdapter) Name() string {
	return "Azure Synapse"
}

// SingleStoreAdapter handles SingleStore
type SingleStoreAdapter struct {
	BaseAdapter
}

func (a *SingleStoreAdapter) Name() string {
	return "SingleStore"
}

// DuckDBAdapter handles DuckDB
type DuckDBAdapter struct {
	BaseAdapter
}

func (a *DuckDBAdapter) Name() string {
	return "DuckDB"
}

// MRNName matches plugins/duckdb/duckdb/source.go, which identifies a
// table as schema.table.
func (a *DuckDBAdapter) MRNName(_, schema, table string) string {
	return joinNonEmpty(schema, table)
}

func registerWarehouseAdapters() {
	RegisterAdapter("postgres", &PostgresAdapter{})
	RegisterAdapter("postgresql", &PostgresAdapter{})
	RegisterAdapter("alloydb", &AlloyDBAdapter{})
	RegisterAdapter("mysql", &MySQLAdapter{})
	RegisterAdapter("sqlserver", &SQLServerAdapter{})
	RegisterAdapter("mssql", &SQLServerAdapter{})
	RegisterAdapter("snowflake", &SnowflakeAdapter{})
	RegisterAdapter("bigquery", &BigQueryAdapter{})
	RegisterAdapter("redshift", &RedshiftAdapter{})
	RegisterAdapter("synapse", &SynapseAdapter{})
	RegisterAdapter("singlestore", &SingleStoreAdapter{})
	RegisterAdapter("duckdb", &DuckDBAdapter{})
}
