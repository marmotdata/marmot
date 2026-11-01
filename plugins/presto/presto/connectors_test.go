package presto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnectorInfoForName_InternalConnectorsBelongToPresto(t *testing.T) {
	// Trino skips these catalogs; a memory or tpch table has no other
	// home, so here they are catalogued under Presto's full path.
	for _, connector := range []string{"memory", "tpch", "tpcds", "blackhole", "localfile"} {
		info := connectorInfoForName(connector)
		assert.Equal(t, "Presto", info.Provider, connector)
		assert.Equal(t, "cat.sch.tbl", info.MRNName("cat", "sch", "tbl"), connector)
	}
}

func TestConnectorInfoForName_UnknownConnectorBelongsToPresto(t *testing.T) {
	info := connectorInfoForName("some_future_connector")
	assert.Equal(t, "Presto", info.Provider)
	assert.Equal(t, "cat.sch.tbl", info.MRNName("cat", "sch", "tbl"))
}

func TestConnectorInfoForName_EmptyConnectorBelongsToPresto(t *testing.T) {
	// The connector name is unknown when system.metadata.catalogs is
	// unreadable; the tables still need an owner.
	info := connectorInfoForName("")
	assert.Equal(t, "Presto", info.Provider)
}

func TestConnectorInfoForName_MappedConnectorUsesItsProvider(t *testing.T) {
	info := connectorInfoForName("postgresql")
	assert.Equal(t, "PostgreSQL", info.Provider)
	assert.Equal(t, "orders", info.MRNName("pg", "public", "orders"))
}

func TestConnectorMap_PrestoSpellingsMatchTheirTrinoRows(t *testing.T) {
	assert.Equal(t, connectorMap["hive"].Provider, connectorMap["hive-hadoop2"].Provider)
	assert.Equal(t, connectorMap["hive"].MRNName("c", "s", "t"), connectorMap["hive-hadoop2"].MRNName("c", "s", "t"))
	assert.Equal(t, connectorMap["delta_lake"].Provider, connectorMap["delta"].Provider)
	assert.Equal(t, connectorMap["delta_lake"].MRNName("c", "s", "t"), connectorMap["delta"].MRNName("c", "s", "t"))
}

// One pin per copied row, so a drift from the Trino plugin's map shows
// up as a failing test rather than as a duplicated asset.

func TestConnectorMap_BareTableNameRows(t *testing.T) {
	for connector, provider := range map[string]string{
		"postgresql":    "PostgreSQL",
		"mysql":         "MySQL",
		"mariadb":       "MariaDB",
		"singlestore":   "SingleStore",
		"mongodb":       "MongoDB",
		"redis":         "Redis",
		"elasticsearch": "Elasticsearch",
		"opensearch":    "OpenSearch",
		"pinot":         "Pinot",
		"kafka":         "Kafka",
		"kinesis":       "Kinesis",
		"prometheus":    "Prometheus",
		"google_sheets": "Google Sheets",
	} {
		info := connectorMap[connector]
		assert.Equal(t, provider, info.Provider, connector)
		assert.Equal(t, "tbl", info.MRNName("cat", "sch", "tbl"), connector)
	}
}

func TestConnectorMap_SchemaQualifiedRows(t *testing.T) {
	for connector, provider := range map[string]string{
		"sqlserver":  "SQL Server",
		"oracle":     "Oracle",
		"clickhouse": "ClickHouse",
		"redshift":   "Redshift",
		"snowflake":  "Snowflake",
		"bigquery":   "BigQuery",
		"cassandra":  "Cassandra",
		"accumulo":   "Accumulo",
		"druid":      "Druid",
		"phoenix":    "Phoenix",
		"ignite":     "Ignite",
		"kudu":       "Kudu",
	} {
		info := connectorMap[connector]
		assert.Equal(t, provider, info.Provider, connector)
		assert.Equal(t, "sch.tbl", info.MRNName("cat", "sch", "tbl"), connector)
	}
}

func TestConnectorMap_FullyQualifiedRows(t *testing.T) {
	for connector, provider := range map[string]string{
		"iceberg":      "Iceberg",
		"delta_lake":   "Delta Lake",
		"hive":         "Hive",
		"hudi":         "Hudi",
		"hive-hadoop2": "Hive",
		"delta":        "Delta Lake",
	} {
		info := connectorMap[connector]
		assert.Equal(t, provider, info.Provider, connector)
		assert.Equal(t, "cat.sch.tbl", info.MRNName("cat", "sch", "tbl"), connector)
	}
}

func TestConnectorMap_HasNoRowsBeyondThePinnedOnes(t *testing.T) {
	assert.Len(t, connectorMap, 13+12+6)
}
