package dbt

import (
	"testing"
)

func TestGetAdapter(t *testing.T) {
	tests := []struct {
		adapterType  string
		expectedName string
	}{
		// Warehouse adapters
		{"postgres", "Postgres"},
		{"postgresql", "Postgres"},
		{"alloydb", "AlloyDB"},
		{"mysql", "MySQL"},
		{"sqlserver", "SQLServer"},
		{"mssql", "SQLServer"},
		{"snowflake", "Snowflake"},
		{"bigquery", "BigQuery"},
		{"redshift", "Redshift"},
		{"synapse", "Azure Synapse"},
		{"singlestore", "SingleStore"},
		{"duckdb", "DuckDB"},

		// Cloud/lakehouse adapters
		{"databricks", "Databricks"},
		{"lakebase", "Databricks"},
		{"spark", "Spark"},
		{"athena", "Athena"},
		{"glue", "AWS Glue"},
		{"fabric", "Microsoft Fabric"},
		{"fabricspark", "Microsoft Fabric"},
		{"dremio", "Dremio"},

		// Specialized adapters
		{"clickhouse", "ClickHouse"},
		{"materialize", "Materialize"},
		{"trino", "Trino"},
		{"starburst", "Trino"},
		{"teradata", "Teradata"},
		{"oracle", "Oracle"},
		{"ibm_netezza", "Netezza"},
		{"salesforce", "Salesforce"},

		// Unknown adapter should return generic
		{"unknown_adapter", "DBT"},
		{"", "DBT"},
	}

	for _, tt := range tests {
		t.Run(tt.adapterType, func(t *testing.T) {
			adapter := GetAdapter(tt.adapterType)
			if adapter.Name() != tt.expectedName {
				t.Errorf("GetAdapter(%q).Name() = %q, want %q", tt.adapterType, adapter.Name(), tt.expectedName)
			}
		})
	}
}

func TestAdapterMaterializations(t *testing.T) {
	tests := []struct {
		adapterType     string
		materialization string
		expectedType    string
	}{
		// Standard materializations
		{"postgres", "table", "Table"},
		{"postgres", "view", "View"},
		{"postgres", "incremental", "Table"},
		{"postgres", "ephemeral", "Ephemeral"},

		// Snowflake special types
		{"snowflake", "dynamic_table", "Dynamic Table"},
		{"snowflake", "materialized_view", "Materialized View"},

		// ClickHouse special types
		{"clickhouse", "dictionary", "Dictionary"},
		{"clickhouse", "distributed", "Distributed Table"},
		{"clickhouse", "materialized_view", "Materialized View"},

		// Materialize special types
		{"materialize", "source", "Source"},
		{"materialize", "sink", "Sink"},
		{"materialize", "materializedview", "Materialized View"},

		// BigQuery
		{"bigquery", "materialized_view", "Materialized View"},

		// Databricks
		{"databricks", "streaming_table", "Streaming Table"},

		// Oracle
		{"oracle", "materialized_view", "Materialized View"},
	}

	for _, tt := range tests {
		t.Run(tt.adapterType+"_"+tt.materialization, func(t *testing.T) {
			adapter := GetAdapter(tt.adapterType)
			assetType := adapter.AssetTypeForMaterialization(tt.materialization)
			if assetType != tt.expectedType {
				t.Errorf("GetAdapter(%q).AssetTypeForMaterialization(%q) = %q, want %q",
					tt.adapterType, tt.materialization, assetType, tt.expectedType)
			}
		})
	}
}

func TestMaterializeDefaultMaterialization(t *testing.T) {
	adapter := GetAdapter("materialize")
	if adapter.DefaultMaterialization() != "materializedview" {
		t.Errorf("Materialize adapter default materialization = %q, want %q",
			adapter.DefaultMaterialization(), "materializedview")
	}
}

func TestBigQuerySupportsSchemas(t *testing.T) {
	adapter := GetAdapter("bigquery")
	if adapter.SupportsSchemas() {
		t.Error("BigQuery adapter should return false for SupportsSchemas()")
	}

	postgresAdapter := GetAdapter("postgres")
	if !postgresAdapter.SupportsSchemas() {
		t.Error("Postgres adapter should return true for SupportsSchemas()")
	}
}
