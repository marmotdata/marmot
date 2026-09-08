package spline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spline records only the URI Spark read or wrote. These tests pin the
// identity each URI resolves to, because an edge built from the wrong name
// points at an asset that does not exist and the server drops it.

func TestParseURI_PostgresTableAppendedWithAColon(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:postgresql://pg.internal:5432/warehouse:public.orders")
	require.True(t, ok)

	assert.Equal(t, "Table", ref.Type)
	assert.Equal(t, "PostgreSQL", ref.Provider)
	assert.Equal(t, "orders", ref.Name)
}

func TestParseURI_PostgresTableAsAQueryParameter(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:postgresql://pg.internal:5432/warehouse?table=orders")
	require.True(t, ok)

	assert.Equal(t, "PostgreSQL", ref.Provider)
	assert.Equal(t, "orders", ref.Name)
}

func TestParseURI_PostgresDropsTheSchemaFromTheName(t *testing.T) {
	// The PostgreSQL plugin names a table by its bare name, so the schema in
	// the URI has to be dropped or the edge would dangle.
	ref, ok := parseDataSourceURI("jdbc:postgresql://pg:5432/warehouse:reporting.orders")
	require.True(t, ok)

	assert.Equal(t, "orders", ref.Name)
}

func TestParseURI_MySQLBareTableName(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:mysql://mysql.internal/shop:customers")
	require.True(t, ok)

	assert.Equal(t, "MySQL", ref.Provider)
	assert.Equal(t, "customers", ref.Name)
}

func TestParseURI_MariaDBBareTableName(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:mariadb://maria.internal:3306/shop:customers")
	require.True(t, ok)

	assert.Equal(t, "MariaDB", ref.Provider)
	assert.Equal(t, "customers", ref.Name)
}

func TestParseURI_SQLServerUsesDatabaseSchemaTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:sqlserver://sql.internal:1433;databaseName=shop:sales.orders")
	require.True(t, ok)

	assert.Equal(t, "SQL Server", ref.Provider)
	assert.Equal(t, "shop.sales.orders", ref.Name)
}

func TestParseURI_SQLServerDefaultsTheSchemaToDbo(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:sqlserver://sql.internal:1433;databaseName=shop;encrypt=true:orders")
	require.True(t, ok)

	assert.Equal(t, "shop.dbo.orders", ref.Name)
}

func TestParseURI_OracleThinDriverUsesSchemaTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:oracle:thin:@//oracle.internal:1521/ORCL:HR.EMPLOYEES")
	require.True(t, ok)

	assert.Equal(t, "Oracle", ref.Provider)
	assert.Equal(t, "HR.EMPLOYEES", ref.Name)
}

func TestParseURI_OracleWithoutASchemaIsSkipped(t *testing.T) {
	// The Oracle plugin always qualifies a table with its schema, so a URI
	// that names only the table cannot be resolved to a real asset.
	_, ok := parseDataSourceURI("jdbc:oracle:thin:@//oracle.internal:1521/ORCL:EMPLOYEES")
	assert.False(t, ok)
}

func TestParseURI_RedshiftUsesDatabaseSchemaTable(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:redshift://cluster.eu-west-1.redshift.amazonaws.com:5439/dev:analytics.orders")
	require.True(t, ok)

	assert.Equal(t, "Redshift", ref.Provider)
	assert.Equal(t, "dev.analytics.orders", ref.Name)
}

func TestParseURI_RedshiftDefaultsTheSchemaToPublic(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:redshift://cluster.eu-west-1.redshift.amazonaws.com:5439/dev:orders")
	require.True(t, ok)

	assert.Equal(t, "dev.public.orders", ref.Name)
}

func TestParseURI_SnowflakeReadsTheDatabaseFromQueryParameters(t *testing.T) {
	// The Snowflake JDBC URL carries the database and schema as parameters
	// rather than in the path.
	ref, ok := parseDataSourceURI("jdbc:snowflake://acct.snowflakecomputing.com/?db=ANALYTICS&schema=PUBLIC&table=ORDERS")
	require.True(t, ok)

	assert.Equal(t, "Snowflake", ref.Provider)
	assert.Equal(t, "ANALYTICS.PUBLIC.ORDERS", ref.Name)
}

func TestParseURI_SnowflakeFullyQualifiedTableWins(t *testing.T) {
	ref, ok := parseDataSourceURI("jdbc:snowflake://acct.snowflakecomputing.com/?db=IGNORED&table=ANALYTICS.SALES.ORDERS")
	require.True(t, ok)

	assert.Equal(t, "ANALYTICS.SALES.ORDERS", ref.Name)
}

func TestParseURI_UnknownJDBCEngineIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("jdbc:db2://db2.internal:50000/shop:orders")
	assert.False(t, ok)
}

func TestParseURI_JDBCWithoutATableIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("jdbc:postgresql://pg.internal:5432/warehouse")
	assert.False(t, ok)
}

func TestParseURI_S3BucketIsTheAssetName(t *testing.T) {
	ref, ok := parseDataSourceURI("s3://marmot-lake/raw/events/date=2026-09-01")
	require.True(t, ok)

	assert.Equal(t, "Bucket", ref.Type)
	assert.Equal(t, "S3", ref.Provider)
	assert.Equal(t, "marmot-lake", ref.Name)
}

func TestParseURI_S3AAndS3NResolveToTheSameBucket(t *testing.T) {
	a, ok := parseDataSourceURI("s3a://marmot-lake/reports/nightly")
	require.True(t, ok)
	n, ok := parseDataSourceURI("s3n://marmot-lake/reports/nightly")
	require.True(t, ok)

	assert.Equal(t, "marmot-lake", a.Name)
	assert.Equal(t, a, n)
}

func TestParseURI_GCSBucket(t *testing.T) {
	ref, ok := parseDataSourceURI("gs://marmot-lake/raw/events")
	require.True(t, ok)

	assert.Equal(t, "Bucket", ref.Type)
	assert.Equal(t, "GCS", ref.Provider)
	assert.Equal(t, "marmot-lake", ref.Name)
}

func TestParseURI_AzureContainerDropsTheStorageAccount(t *testing.T) {
	ref, ok := parseDataSourceURI("abfss://lake@marmot.dfs.core.windows.net/raw/events")
	require.True(t, ok)

	assert.Equal(t, "Container", ref.Type)
	assert.Equal(t, "AzureBlob", ref.Provider)
	assert.Equal(t, "lake", ref.Name)
}

func TestParseURI_HiveURIWithASlash(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales/orders")
	require.True(t, ok)

	assert.Equal(t, "Table", ref.Type)
	assert.Equal(t, "Hive", ref.Provider)
	assert.Equal(t, "sales.orders", ref.Name)
}

func TestParseURI_HiveURIWithADot(t *testing.T) {
	ref, ok := parseDataSourceURI("hive://sales.orders")
	require.True(t, ok)

	assert.Equal(t, "sales.orders", ref.Name)
}

func TestParseURI_BareDatabaseTableIsHive(t *testing.T) {
	// Spark's built-in catalog reports a managed table with no scheme at all.
	ref, ok := parseDataSourceURI("sales.orders")
	require.True(t, ok)

	assert.Equal(t, "Hive", ref.Provider)
	assert.Equal(t, "sales.orders", ref.Name)
}

func TestParseURI_DeltaTableIsTheLastPathSegment(t *testing.T) {
	ref, ok := parseDataSourceURI("delta://warehouse/gold/orders")
	require.True(t, ok)

	assert.Equal(t, "Table", ref.Type)
	assert.Equal(t, "Delta Lake", ref.Provider)
	assert.Equal(t, "orders", ref.Name)
}

func TestParseURI_HDFSIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("hdfs://namenode:8020/user/hive/warehouse/orders")
	assert.False(t, ok)
}

func TestParseURI_LocalFileIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("file:/tmp/spark-output")
	assert.False(t, ok)
}

func TestParseURI_EmptyStringIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("")
	assert.False(t, ok)
}

func TestParseURI_UnknownSchemeIsSkipped(t *testing.T) {
	_, ok := parseDataSourceURI("kudu://master:7051/impala::default.orders")
	assert.False(t, ok)
}
