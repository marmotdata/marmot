package dbt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAdapter(t *testing.T) {
	tests := []struct {
		adapterType  string
		expectedName string
	}{
		// Warehouse adapters
		{"postgres", "PostgreSQL"},
		{"postgresql", "PostgreSQL"},
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
		{"athena", "Glue"},
		{"glue", "Glue"},
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

		// ClickHouse special types
		{"clickhouse", "dictionary", "Dictionary"},
		{"clickhouse", "distributed", "Distributed Table"},

		// Materialize special types
		{"materialize", "source", "Source"},
		{"materialize", "sink", "Sink"},

		// Databricks
		{"databricks", "streaming_table", "Streaming Table"},

		// A materialized view is catalogued as a plain View, the way
		// plugins/postgresql and plugins/clickhouse already do it.
		{"postgres", "materialized_view", "View"},
		{"snowflake", "materialized_view", "View"},
		{"clickhouse", "materialized_view", "View"},
		{"materialize", "materializedview", "View"},
		{"bigquery", "materialized_view", "View"},
		{"oracle", "materialized_view", "View"},
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

// Each adapter must address a table the way the Marmot plugin that owns
// that technology addresses it, or dbt files the same physical table a
// second time instead of merging with the plugin's asset.
func TestAdapterMRNNameMatchesTheNativePlugin(t *testing.T) {
	tests := []struct {
		adapterType string
		want        string
	}{
		// plugins/postgresql: schema.table
		{"postgres", "public.orders"},
		// plugins/mysql: database.table
		{"mysql", "public.orders"},
		// plugins/bigquery: dataset.table
		{"bigquery", "public.orders"},
		// plugins/clickhouse: database.table
		{"clickhouse", "public.orders"},
		// plugins/glue: database.table
		{"glue", "public.orders"},
		// Athena reads the Glue catalog, so it addresses tables as Glue does
		{"athena", "public.orders"},
		// plugins/duckdb: schema.table
		{"duckdb", "public.orders"},
		// No Marmot plugin owns these, so all three levels stay
		{"snowflake", "analytics.public.orders"},
		{"redshift", "analytics.public.orders"},
		{"databricks", "analytics.public.orders"},
	}

	for _, tt := range tests {
		t.Run(tt.adapterType, func(t *testing.T) {
			got := GetAdapter(tt.adapterType).MRNName("analytics", "public", "orders")
			assert.Equal(t, tt.want, got)
		})
	}
}

// The provider word is the service component of the MRN, so it has to be
// the word the native plugin passes to mrn.New rather than a synonym.
func TestAdapterProviderMatchesTheNativePlugin(t *testing.T) {
	assert.Equal(t, "PostgreSQL", GetAdapter("postgres").Name(), "plugins/postgresql uses PostgreSQL, not Postgres")
	assert.Equal(t, "Glue", GetAdapter("glue").Name(), "plugins/glue uses Glue, not AWS Glue")
	assert.Equal(t, "Glue", GetAdapter("athena").Name(), "Athena tables are Glue catalog tables")
}

// mrn.New sanitizes the name it is given but not the type or the service,
// so a spaced provider or asset type would put a raw space in the MRN and
// in every URL built from it.
func TestMRNComponentsNeverCarryASpace(t *testing.T) {
	assert.Equal(t, "MicrosoftFabric", mrnService("Microsoft Fabric"))
	assert.Equal(t, "AzureSynapse", mrnService("Azure Synapse"))
	assert.Equal(t, "DynamicTable", mrnType("Dynamic Table"))
	assert.Equal(t, "StreamingTable", mrnType("Streaming Table"))

	for adapterType := range adapterRegistry {
		a := adapterRegistry[adapterType]
		assert.NotContains(t, mrnService(a.Name()), " ", "adapter %q leaks a space into the MRN service", adapterType)
	}
}
