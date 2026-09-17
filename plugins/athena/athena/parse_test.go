package athena

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeViewText_PrestoViewYieldsTheOriginalSQL(t *testing.T) {
	stored := prestoViewText("SELECT sum(total) FROM shop.orders")

	assert.Equal(t, "SELECT sum(total) FROM shop.orders", decodeViewText(stored))
}

func TestDecodeViewText_PlainSQLIsReturnedUnchanged(t *testing.T) {
	assert.Equal(t, "SELECT 1", decodeViewText("SELECT 1"))
}

func TestDecodeViewText_UndecodableBase64FallsBackToTheRawText(t *testing.T) {
	// A marker with payload that is not valid base64 still tells a reader
	// what Athena stored, so the raw text is better than nothing.
	raw := "/* Presto View: not=valid=base64 */"

	assert.Equal(t, raw, decodeViewText(raw))
}

func TestDecodeViewText_MarkerWithoutOriginalSQLFallsBackToTheRawText(t *testing.T) {
	// eyJjYXRhbG9nIjoiYSJ9 is {"catalog":"a"}, a view document with no SQL.
	raw := "/* Presto View: eyJjYXRhbG9nIjoiYSJ9 */"

	assert.Equal(t, raw, decodeViewText(raw))
}

func TestDecodeViewText_EmptyTextStaysEmpty(t *testing.T) {
	assert.Empty(t, decodeViewText(""))
}

func TestExtractQueryTables_ReadsTheFromClause(t *testing.T) {
	refs := extractQueryTables("SELECT * FROM orders")

	require.Len(t, refs, 1)
	assert.Equal(t, tableRef{Table: "orders"}, refs[0])
}

func TestExtractQueryTables_ReadsJoinedTables(t *testing.T) {
	refs := extractQueryTables("SELECT * FROM orders o JOIN customers c ON c.id = o.customer_id")

	assert.Equal(t, []tableRef{{Table: "orders"}, {Table: "customers"}}, refs)
}

func TestExtractQueryTables_KeepsTheDatabaseOfAQualifiedName(t *testing.T) {
	refs := extractQueryTables("SELECT * FROM shop.orders")

	require.Len(t, refs, 1)
	assert.Equal(t, tableRef{Database: "shop", Table: "orders"}, refs[0])
}

func TestExtractQueryTables_ThreePartNameDropsTheCatalog(t *testing.T) {
	refs := extractQueryTables("SELECT * FROM awsdatacatalog.shop.orders")

	require.Len(t, refs, 1)
	assert.Equal(t, tableRef{Database: "shop", Table: "orders"}, refs[0])
}

func TestExtractQueryTables_StripsQuotedIdentifiers(t *testing.T) {
	refs := extractQueryTables(`SELECT * FROM "shop"."order items"`)

	require.Len(t, refs, 1)
	assert.Equal(t, tableRef{Database: "shop", Table: "order items"}, refs[0])
}

func TestExtractQueryTables_StripsBacktickIdentifiers(t *testing.T) {
	refs := extractQueryTables("SELECT * FROM `shop`.`orders`")

	require.Len(t, refs, 1)
	assert.Equal(t, tableRef{Database: "shop", Table: "orders"}, refs[0])
}

func TestExtractQueryTables_IgnoresTablesNamedOnlyInComments(t *testing.T) {
	query := "-- FROM secrets\nSELECT * FROM orders /* FROM audit */"

	assert.Equal(t, []tableRef{{Table: "orders"}}, extractQueryTables(query))
}

func TestExtractQueryTables_ReportsEachTableOnce(t *testing.T) {
	query := "SELECT * FROM orders UNION ALL SELECT * FROM orders"

	assert.Equal(t, []tableRef{{Table: "orders"}}, extractQueryTables(query))
}

func TestExtractQueryTables_SkipsSubqueries(t *testing.T) {
	// A subquery opens with a bracket, which is not part of a table name.
	refs := extractQueryTables("SELECT * FROM (SELECT 1) t")

	assert.Empty(t, refs)
}

func TestExtractQueryTables_IsCaseInsensitiveOnKeywords(t *testing.T) {
	assert.Equal(t, []tableRef{{Table: "orders"}}, extractQueryTables("select * from orders"))
}

func TestS3Bucket_ReadsTheBucketFromAnS3Location(t *testing.T) {
	assert.Equal(t, "marmot-lake", s3Bucket("s3://marmot-lake/orders/dt=2026-01-01/"))
}

func TestS3Bucket_HandlesALocationWithNoPrefix(t *testing.T) {
	assert.Equal(t, "marmot-lake", s3Bucket("s3://marmot-lake"))
}

func TestS3Bucket_AcceptsTheHiveS3aScheme(t *testing.T) {
	assert.Equal(t, "marmot-lake", s3Bucket("s3a://marmot-lake/orders/"))
}

func TestS3Bucket_IgnoresANonS3Location(t *testing.T) {
	assert.Empty(t, s3Bucket("hdfs://namenode/orders"))
}

func TestS3Bucket_IgnoresAnEmptyLocation(t *testing.T) {
	assert.Empty(t, s3Bucket(""))
}

func TestTableAssetType_VirtualViewIsAView(t *testing.T) {
	assert.Equal(t, "View", tableAssetType("VIRTUAL_VIEW"))
}

func TestTableAssetType_ExternalTableIsATable(t *testing.T) {
	assert.Equal(t, "Table", tableAssetType("EXTERNAL_TABLE"))
}

func TestTableAssetType_UnknownTypeIsATable(t *testing.T) {
	assert.Equal(t, "Table", tableAssetType(""))
}

func TestSavedQueryName_IsQualifiedByItsWorkGroup(t *testing.T) {
	assert.Equal(t, "analytics/daily-revenue", savedQueryName("analytics", "daily-revenue"))
}

func TestSavedQueryName_WithoutAWorkGroupIsTheBareName(t *testing.T) {
	assert.Equal(t, "daily-revenue", savedQueryName("", "daily-revenue"))
}

func TestArnPartition_CommercialRegions(t *testing.T) {
	assert.Equal(t, "aws", arnPartition("us-east-1"))
}

func TestArnPartition_ChinaRegions(t *testing.T) {
	assert.Equal(t, "aws-cn", arnPartition("cn-north-1"))
}

func TestArnPartition_GovCloudRegions(t *testing.T) {
	assert.Equal(t, "aws-us-gov", arnPartition("us-gov-west-1"))
}

func TestAthenaARN_BuildsAWorkGroupARN(t *testing.T) {
	assert.Equal(t,
		"arn:aws:athena:us-east-1:123456789012:workgroup/analytics",
		athenaARN("us-east-1", "123456789012", "workgroup/analytics"))
}

func TestAthenaARN_IsEmptyWithoutAnAccountID(t *testing.T) {
	assert.Empty(t, athenaARN("us-east-1", "", "workgroup/analytics"))
}

func TestWorkGroupConsoleURL_LinksToTheWorkGroupDetails(t *testing.T) {
	assert.Equal(t,
		"https://eu-west-1.console.aws.amazon.com/athena/home?region=eu-west-1#/workgroups/details/analytics",
		workGroupConsoleURL("eu-west-1", "analytics"))
}

func TestWorkGroupConsoleURL_IsEmptyWithoutARegion(t *testing.T) {
	assert.Empty(t, workGroupConsoleURL("", "analytics"))
}

func TestFormatTime_RendersRFC3339(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)

	assert.Equal(t, "2026-03-01T09:00:00Z", formatTime(&at))
}

func TestFormatTime_IsEmptyForNoTime(t *testing.T) {
	assert.Empty(t, formatTime(nil))
}
