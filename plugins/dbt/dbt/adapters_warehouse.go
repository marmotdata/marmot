package dbt

// PostgresAdapter handles PostgreSQL
type PostgresAdapter struct {
	BaseAdapter
}

func (a *PostgresAdapter) Name() string {
	return "Postgres"
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
		return "Materialized View"
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

func (a *BigQueryAdapter) SupportsSchemas() bool {
	return false
}

func (a *BigQueryAdapter) AssetTypeForMaterialization(materialization string) string {
	switch materialization {
	case "view":
		return "View"
	case "table", "incremental":
		return "Table"
	case "materialized_view":
		return "Materialized View"
	case "ephemeral":
		return "Ephemeral"
	default:
		return "Table"
	}
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
		return "Materialized View"
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
