package presto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargetFor_InternalConnectorsBelongToPresto(t *testing.T) {
	// A memory or tpch table has no other home, so it is Presto's own even if someone maps its catalog.
	cfg := &Config{CatalogMap: map[string]CatalogMapping{"cat": {}}}
	for _, connector := range []string{"memory", "tpch", "tpcds", "blackhole", "localfile"} {
		target := cfg.targetFor("cat", connector, "sch", "tbl")
		assert.False(t, target.Native, connector)
		assert.Equal(t, "Presto", target.Provider, connector)
		assert.Equal(t, []string{"cat", "sch", "tbl"}, target.Path, connector)
	}
}

func TestTargetFor_UnknownAndEmptyConnectorsBelongToPresto(t *testing.T) {
	cfg := &Config{CatalogMap: map[string]CatalogMapping{"cat": {}}}
	for _, connector := range []string{"some_future_connector", ""} {
		target := cfg.targetFor("cat", connector, "sch", "tbl")
		assert.False(t, target.Native, connector)
		assert.Equal(t, "Presto", target.Provider, connector)
	}
}

func TestTargetFor_MappedCatalogsGetTheNativeIdentity(t *testing.T) {
	cfg := &Config{CatalogMap: map[string]CatalogMapping{
		"pg":   {Zone: "prod-eu", Database: "shop"},
		"my":   {},
		"hive": {Zone: "lake"},
		"dl":   {},
		"pgno": {},
	}}

	tests := []struct {
		catalog, connector, wantMRN string
	}{
		{"pg", "postgresql", "mrn://asset/table/postgresql@prod-eu/shop.sch.tbl"},
		{"my", "mysql", "mrn://asset/table/mysql/sch.tbl"},
		{"hive", "hive-hadoop2", "mrn://asset/table/hive@lake/sch.tbl"},
		{"dl", "delta", "mrn://asset/table/delta-lake/tbl"},
		{"pgno", "postgresql", "mrn://asset/table/presto/pgno.sch.tbl"},
		{"unlisted", "postgresql", "mrn://asset/table/presto/unlisted.sch.tbl"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.wantMRN, cfg.targetFor(tt.catalog, tt.connector, "sch", "tbl").mrn("Table"), tt.catalog)
	}
}

func TestTargetFor_PrestoAssetsCarryThePrestoZone(t *testing.T) {
	cfg := &Config{}
	cfg.Zone = "dp"

	assert.Equal(t, "mrn://asset/table/presto@dp/memory.shop.orders", cfg.targetFor("memory", "memory", "shop", "orders").mrn("Table"))
	assert.Equal(t, "mrn://asset/catalog/presto@dp/memory", cfg.catalogMRN("memory"))
}

func TestTargetFor_SameNamedTablesBehindDifferentCatalogsDiffer(t *testing.T) {
	cfg := &Config{}
	assert.NotEqual(t,
		cfg.targetFor("pg_a", "postgresql", "public", "orders").mrn("Table"),
		cfg.targetFor("pg_b", "postgresql", "public", "orders").mrn("Table"))
}

func TestUnmappedReason(t *testing.T) {
	cfg := &Config{CatalogMap: map[string]CatalogMapping{"pg": {Database: "shop"}, "pgno": {}}}

	assert.Empty(t, cfg.unmappedReason("pg", "postgresql"))
	assert.Contains(t, cfg.unmappedReason("x", "postgresql"), "not in catalog_map")
	assert.Contains(t, cfg.unmappedReason("pgno", "postgresql"), "no database")
}

func TestPreviousMRNs_OnlyCatalogQualifiedNativeGuessesAreClaimed(t *testing.T) {
	s := &Source{config: &Config{}}

	hive := s.createTableAsset("lake", "sales", "orders", "BASE TABLE", "hive-hadoop2")
	assert.Equal(t, []string{"mrn://table/hive/lake.sales.orders"}, hive.PreviousMRNs)

	mem := s.createTableAsset("memory", "shop", "orders", "BASE TABLE", "memory")
	assert.Empty(t, mem.PreviousMRNs, "an internal catalog's MRN did not change")
}

func TestConnectorMap_PrestoSpellingsMatchTheirTrinoRows(t *testing.T) {
	assert.Equal(t, connectorMap["hive"].Provider, connectorMap["hive-hadoop2"].Provider)
	assert.Equal(t, connectorMap["hive"].Path("d", "s", "t"), connectorMap["hive-hadoop2"].Path("d", "s", "t"))
	assert.Equal(t, connectorMap["delta_lake"].Provider, connectorMap["delta"].Provider)
	assert.Equal(t, connectorMap["delta_lake"].Path("d", "s", "t"), connectorMap["delta"].Path("d", "s", "t"))
}

// One pin per copied row, so a drift from the Trino plugin's map shows up as a failing test rather than as a duplicated asset.

func TestConnectorMap_Shapes(t *testing.T) {
	shapes := map[string]struct {
		providers map[string]string
		path      []string
	}{
		"table only":            {map[string]string{"delta_lake": "Delta Lake", "delta": "Delta Lake", "redis": "Redis", "elasticsearch": "Elasticsearch", "opensearch": "OpenSearch", "pinot": "Pinot", "kafka": "Kafka", "kinesis": "Kinesis", "prometheus": "Prometheus", "google_sheets": "Google Sheets"}, []string{"tbl"}},
		"schema table":          {map[string]string{"mysql": "MySQL", "mariadb": "MariaDB", "singlestore": "SingleStore", "clickhouse": "ClickHouse", "iceberg": "Iceberg", "hive": "Hive", "hive-hadoop2": "Hive", "hudi": "Hudi", "mongodb": "MongoDB", "cassandra": "Cassandra", "accumulo": "Accumulo", "druid": "Druid", "phoenix": "Phoenix", "ignite": "Ignite", "kudu": "Kudu"}, []string{"sch", "tbl"}},
		"database schema table": {map[string]string{"postgresql": "PostgreSQL", "sqlserver": "SQL Server", "oracle": "Oracle", "redshift": "Redshift", "snowflake": "Snowflake", "bigquery": "BigQuery"}, []string{"db", "sch", "tbl"}},
	}

	total := 0
	for name, shape := range shapes {
		for connector, provider := range shape.providers {
			info := connectorMap[connector]
			assert.Equal(t, provider, info.Provider, connector)
			assert.Equal(t, shape.path, info.Path("db", "sch", "tbl"), name+" "+connector)
			assert.Equal(t, name == "database schema table", info.NeedsDatabase, connector)
			total++
		}
	}
	assert.Len(t, connectorMap, total, "no rows beyond the pinned ones")
}
