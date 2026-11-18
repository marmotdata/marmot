package lineage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Each row is one namespace form from the OpenLineage naming spec (https://openlineage.io/docs/spec/naming) and the MRN the system's own plugin gives the same object, so a dataset an event mentions and the asset the plugin catalogues are one asset.
func TestIdentifyDatasetFollowsTheNamingSpec(t *testing.T) {
	cases := []struct {
		name      string
		namespace string
		dataset   string
		wantMRN   string
		wantType  string
		wantName  string
	}{
		{"postgres", "postgres://pg.example.com:5432", "shop.public.orders", "mrn://asset/table/postgresql@host=pg.example.com:5432/shop.public.orders", "Table", "orders"},
		{"postgres without a port gets the default", "postgres://pg.example.com", "shop.public.orders", "mrn://asset/table/postgresql@host=pg.example.com:5432/shop.public.orders", "Table", "orders"},
		{"postgresql scheme", "postgresql://pg:5432", "shop.public.orders", "mrn://asset/table/postgresql@host=pg:5432/shop.public.orders", "Table", "orders"},
		{"mysql", "mysql://db:3306", "shop.orders", "mrn://asset/table/mysql@host=db:3306/shop.orders", "Table", "orders"},
		{"mssql", "mssql://sql:1433", "shop.dbo.orders", "mrn://asset/table/sql-server@host=sql:1433/shop.dbo.orders", "Table", "orders"},
		{"sqlserver", "sqlserver://sql", "shop.dbo.orders", "mrn://asset/table/sql-server@host=sql:1433/shop.dbo.orders", "Table", "orders"},
		{"oracle", "oracle://ora:1521", "ORCL.SALES.ORDERS", "mrn://asset/table/oracle@host=ora:1521/orcl.sales.orders", "Table", "ORDERS"},
		{"snowflake", "snowflake://acme-prod", "ANALYTICS.MARTS.ORDERS", "mrn://asset/table/snowflake@host=acme-prod/analytics.marts.orders", "Table", "ORDERS"},
		{"redshift", "redshift://cluster.eu-west-1:5439", "dev.public.orders", "mrn://asset/table/redshift@host=cluster.eu-west-1:5439/dev.public.orders", "Table", "orders"},
		{"trino", "trino://trino:8080", "hive.sales.orders", "mrn://asset/table/trino@host=trino:8080/hive.sales.orders", "Table", "orders"},
		{"hive", "hive://metastore:9083", "lake.orders", "mrn://asset/table/hive@host=metastore:9083/lake.orders", "Table", "orders"},
		{"cassandra", "cassandra://cass:9042", "shop.orders", "mrn://asset/table/cassandra@host=cass:9042/shop.orders", "Table", "orders"},
		{"clickhouse", "clickhouse://ch", "shop.orders", "mrn://asset/table/clickhouse@host=ch:8123/shop.orders", "Table", "orders"},
		{"unity catalog", "unitycatalog://uc.example.com", "main.sales.orders", "mrn://asset/table/unity-catalog@host=uc.example.com/main.sales.orders", "Table", "orders"},
		{"bigquery", "bigquery", "kelp-prod.sales.orders", "mrn://asset/table/bigquery/kelp-prod.sales.orders", "Table", "orders"},
		{"athena drops the default catalogue", "awsathena://athena.eu-west-1.amazonaws.com", "AwsDataCatalog.lake.orders", "mrn://asset/table/glue/lake.orders", "Table", "orders"},
		{"glue", "arn:aws:glue:eu-west-1:123456789012", "table/lake/orders", "mrn://asset/table/glue/lake.orders", "Table", "orders"},
		{"kafka", "kafka://broker:9092", "orders.v1", "mrn://asset/topic/kafka@host=broker:9092/orders.v1", "Topic", "orders.v1"},
		{"kafka without a port", "kafka://broker", "orders", "mrn://asset/topic/kafka@host=broker:9092/orders", "Topic", "orders"},
		{"pubsub topic", "pubsub", "topic:acme:orders", "mrn://asset/topic/googlepubsub/orders", "Topic", "orders"},
		{"pubsub subscription", "pubsub", "subscription:acme:orders-sub", "mrn://asset/subscription/googlepubsub/orders-sub", "Subscription", "orders-sub"},
		{"s3 file", "s3://lake", "raw/orders/2026.parquet", `mrn://asset/file/s3/lake."raw-orders-2026.parquet"`, "File", "2026.parquet"},
		{"s3 prefix", "s3://lake", "raw/orders", "mrn://asset/bucket/s3/lake.raw-orders", "Bucket", "orders"},
		{"s3 bucket", "s3://lake", "/", "mrn://asset/bucket/s3/lake", "Bucket", "lake"},
		{"s3a", "s3a://lake", "raw/orders.csv", `mrn://asset/file/s3/lake."raw-orders.csv"`, "File", "orders.csv"},
		{"gcs", "gs://lake", "raw/orders.json", `mrn://asset/file/gcs/lake."raw-orders.json"`, "File", "orders.json"},
		{"abfss names the container", "abfss://raw@acct.dfs.core.windows.net", "orders/2026.parquet", `mrn://asset/file/azureblob/raw."orders-2026.parquet"`, "File", "2026.parquet"},
		{"wasbs", "wasbs://raw@acct.blob.core.windows.net", "orders", "mrn://asset/bucket/azureblob/raw.orders", "Bucket", "orders"},
		{"hdfs", "hdfs://namenode:8020", "/warehouse/orders", "mrn://asset/dataset/hdfs@host=namenode:8020/-warehouse-orders", "Dataset", "orders"},
		{"dbfs", "dbfs://workspace", "/mnt/orders.parquet", "mrn://asset/file/dbfs/-mnt-orders.parquet", "File", "orders.parquet"},
		{"local file", "file", "/data/out.parquet", "mrn://asset/file/file/-data-out.parquet", "File", "out.parquet"},
		{"local directory", "file", "/data/marts/orders", "mrn://asset/dataset/file/-data-marts-orders", "Dataset", "orders"},
		{"remote file", "file://nfs01", "/exports/out.csv", "mrn://asset/file/file@host=nfs01/-exports-out.csv", "File", "out.csv"},
		{"dbt-duckdb database file", "duckdb://warehouse/shop.duckdb", "shop.main.orders", "mrn://asset/table/duckdb/shop.main.orders", "Table", "orders"},
		{"unknown scheme keeps the 0.13 naming", "crate://crate:4200", "doc.orders", "mrn://asset/dataset/openlineage@host=crate:4200/doc.orders", "Dataset", "doc.orders"},
		{"namespace that is not an address keeps the 0.13 naming", "warehouse-prod", "orders", "mrn://asset/dataset/openlineage/warehouse-prod.orders", "Dataset", "orders"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := identifyDataset(&Dataset{Namespace: c.namespace, Name: c.dataset}, testProducer)
			assert.Equal(t, c.wantMRN, id.MRN)
			assert.Equal(t, c.wantType, id.Type)
			assert.Equal(t, c.wantName, id.Name)
		})
	}
}

func TestIdentifyDatasetUsesTheDatasetTypeFacet(t *testing.T) {
	view := &Dataset{Namespace: "postgres://pg:5432", Name: "shop.public.eu_orders", Facets: map[string]interface{}{
		"datasetType": facet("https://openlineage.io/spec/facets/1-0-1/DatasetTypeDatasetFacet.json", map[string]any{"datasetType": "VIEW"}),
	}}
	assert.Equal(t, "mrn://asset/view/postgresql@host=pg:5432/shop.public.eu_orders", identifyDataset(view, testProducer).MRN)

	// An object store keeps its own types: a FILE facet on a bucket prefix does not make it a table.
	prefix := &Dataset{Namespace: "s3://lake", Name: "raw/orders", Facets: map[string]interface{}{
		"datasetType": facet("https://openlineage.io/spec/facets/1-0-1/DatasetTypeDatasetFacet.json", map[string]any{"datasetType": "TABLE"}),
	}}
	assert.Equal(t, "Bucket", identifyDataset(prefix, testProducer).Type)
}

// duck_lineage files every table under its own namespace, duckdb by default, and says where the table lives in the catalog and dataSource facets. These come from the captured events in testdata/openlineage/events/duck_lineage.
func TestIdentifyDatasetResolvesDuckLineageTables(t *testing.T) {
	const producer = "https://github.com/ilum-cloud/duck_lineage"
	attachedPostgres := &Dataset{Namespace: "duckdb", Name: "pg.public.orders", Facets: map[string]interface{}{
		"catalog":    map[string]interface{}{"framework": "duckdb", "metadataUri": "postgres://127.0.0.1:55433/shop", "name": "pg", "source": "duckdb", "type": "postgres"},
		"dataSource": map[string]interface{}{"name": "postgres://127.0.0.1:55433/shop", "uri": "postgres://127.0.0.1:55433/shop"},
	}}
	id := identifyDataset(attachedPostgres, producer)
	assert.Equal(t, "mrn://asset/table/postgresql@host=127.0.0.1:55433/shop.public.orders", id.MRN, "the Postgres plugin's MRN for the same table")
	assert.Contains(t, id.Legacy, "mrn://asset/dataset/openlineage/duckdb.pg.public.orders", "the stub 0.13 made is adopted")

	attachedMySQL := &Dataset{Namespace: "duckdb", Name: "my.shop.orders", Facets: map[string]interface{}{
		"catalog":    map[string]interface{}{"framework": "duckdb", "name": "my", "type": "mysql"},
		"dataSource": map[string]interface{}{"uri": "mysql://db:3306/shop"},
	}}
	assert.Equal(t, "mrn://asset/table/mysql@host=db:3306/shop.orders", identifyDataset(attachedMySQL, producer).MRN)

	native := &Dataset{Namespace: "duckdb", Name: "shopdb.main.orders", Facets: map[string]interface{}{
		"catalog":    map[string]interface{}{"framework": "duckdb", "metadataUri": "file:///data/shopdb.duckdb", "name": "shopdb", "type": "native"},
		"dataSource": map[string]interface{}{"uri": "file:///data/shopdb.duckdb"},
	}}
	assert.Equal(t, "mrn://asset/table/duckdb/shopdb.main.orders", identifyDataset(native, producer).MRN, "the DuckDB plugin's MRN for the same table")

	noFacets := &Dataset{Namespace: "analytics", Name: "shopdb.main.orders"}
	assert.Equal(t, "mrn://asset/table/duckdb/shopdb.main.orders", identifyDataset(noFacets, producer).MRN, "a custom duck_lineage_namespace still names a DuckDB table")

	assert.Equal(t, "mrn://asset/file/file/out.parquet", identifyDataset(&Dataset{Namespace: "file", Name: "out.parquet"}, producer).MRN)
}

func TestIdentifyDatasetIgnoresADataSourceThatIsNotAnAddress(t *testing.T) {
	// When an attach string has no host or port, duck_lineage puts the raw connection string, password included, in dataSource.
	ds := &Dataset{Namespace: "duckdb", Name: "pg.public.orders", Facets: map[string]interface{}{
		"catalog":    map[string]interface{}{"framework": "duckdb", "name": "pg", "type": "postgres"},
		"dataSource": map[string]interface{}{"uri": "file://dbname=shop user=shop password=hunter2"},
	}}
	id := identifyDataset(ds, "https://github.com/ilum-cloud/duck_lineage")
	assert.NotContains(t, id.MRN, "hunter2")
	fields := datasetFields(ds, id)
	for k, v := range fields.Metadata {
		assert.NotContains(t, v, "hunter2", "metadata %s", k)
	}
}

func TestCleanURI(t *testing.T) {
	cases := map[string]string{
		"postgres://pg:5432/shop":                 "postgres://pg:5432/shop",
		"postgres://user:secret@pg:5432/shop?x=1": "postgres://pg:5432/shop",
		"file:///data/shop.duckdb":                "file:///data/shop.duckdb",
		"file":                                    "",
		"host=pg password=secret":                 "",
		"file://dbname=shop password=secret":      "",
		"":                                        "",
	}
	for in, want := range cases {
		assert.Equal(t, want, cleanURI(in), in)
	}
}

func TestIdentifyJobKeepsTheWholeName(t *testing.T) {
	airflow := func(name string) *RunEvent {
		return &RunEvent{Producer: "https://github.com/apache/airflow/tree/providers-openlineage/2.3.0", Job: Job{Namespace: "prod", Name: name, Facets: map[string]interface{}{
			"jobType": map[string]interface{}{"processingType": "BATCH", "integration": "AIRFLOW", "jobType": "TASK"},
		}}}
	}
	a, b := identifyJob(airflow("dag_a.load")), identifyJob(airflow("dag_b.load"))
	assert.NotEqual(t, a.MRN, b.MRN, "two DAGs' tasks with the same task id are two assets")
	assert.Equal(t, "mrn://asset/task/airflow/prod.dag_a.load", a.MRN)
	assert.Equal(t, "load", a.Name)
	assert.Equal(t, []string{"mrn://asset/task/airflow/prod.load"}, a.Legacy)

	cases := []struct {
		name     string
		event    *RunEvent
		provider string
		typ      string
	}{
		{"integration facet wins", &RunEvent{Producer: "https://example.com/custom", Job: Job{Namespace: "n", Name: "j", Facets: map[string]interface{}{"jobType": map[string]interface{}{"integration": "FLINK", "processingType": "STREAMING"}}}}, "Flink", "Job"},
		{"duck_lineage producer", &RunEvent{Producer: "https://github.com/ilum-cloud/duck_lineage", Job: Job{Namespace: "duckdb", Name: "COPY_orders_8aff966c"}}, "DuckDB", "Job"},
		{"spark producer", &RunEvent{Producer: "https://github.com/OpenLineage/OpenLineage/tree/1.55.0/integration/spark", Job: Job{Namespace: "spark-shop", Name: "shop_orders.drop_table"}}, "Spark", "Job"},
		{"dbt model", &RunEvent{Producer: "https://github.com/OpenLineage/OpenLineage/tree/1.55.0/integration/dbt", Job: Job{Namespace: "dbt-shop", Name: "shop.main.shop.stg_orders", Facets: map[string]interface{}{"jobType": map[string]interface{}{"integration": "DBT", "jobType": "MODEL", "processingType": "BATCH"}}}}, "DBT", "Model"},
		{"unknown producer", &RunEvent{Producer: "https://example.com/custom", Job: Job{Namespace: "n", Name: "j"}}, "OpenLineage", "Job"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := identifyJob(c.event)
			assert.Equal(t, c.provider, id.Provider)
			assert.Equal(t, c.typ, id.Type)
		})
	}
}

func TestParentJobReadsParentAndTheOlderParentRun(t *testing.T) {
	parent := map[string]interface{}{"run": map[string]interface{}{"runId": "d1"}, "job": map[string]interface{}{"namespace": "prod", "name": "orders_etl"}}
	for _, key := range []string{"parent", "parentRun"} {
		ns, name, ok := parentJob(&RunEvent{Run: Run{RunID: "t1", Facets: map[string]interface{}{key: parent}}})
		assert.True(t, ok, key)
		assert.Equal(t, "prod", ns, key)
		assert.Equal(t, "orders_etl", name, key)
	}
	// duck_lineage sends a parent with no job when only OPENLINEAGE_PARENT_RUN_ID is set.
	_, _, ok := parentJob(&RunEvent{Run: Run{Facets: map[string]interface{}{"parent": map[string]interface{}{"run": map[string]interface{}{"runId": "d1"}}}}})
	assert.False(t, ok)
}
