package starrocks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixtures below are the exact output of SHOW CREATE on
// starrocks/allin1-ubuntu (StarRocks 4.1.4), copied verbatim. The
// layout of these statements is the only place StarRocks exposes the
// key model, partitioning and distribution, so the parser is pinned to
// the real text rather than to a tidied-up version of it.

const eventsDDL = "CREATE TABLE `events` (\n" +
	"  `event_date` date NOT NULL COMMENT \"Day the event happened\",\n" +
	"  `event_id` bigint(20) NOT NULL COMMENT \"Event identifier\",\n" +
	"  `user_id` bigint(20) NULL COMMENT \"\",\n" +
	"  `event_type` varchar(64) NULL COMMENT \"\",\n" +
	"  `payload` json NULL COMMENT \"Raw event payload\",\n" +
	"  `tag_ids` array<int(11)> NULL COMMENT \"Tag identifiers\"\n" +
	") ENGINE=OLAP \n" +
	"DUPLICATE KEY(`event_date`, `event_id`)\n" +
	"COMMENT \"Raw clickstream events\"\n" +
	"PARTITION BY RANGE(`event_date`)\n" +
	"(PARTITION p202601 VALUES [(\"2026-01-01\"), (\"2026-02-01\")),\n" +
	"PARTITION p202602 VALUES [(\"2026-02-01\"), (\"2026-03-01\")))\n" +
	"DISTRIBUTED BY HASH(`event_id`) BUCKETS 4 \n" +
	"PROPERTIES (\n" +
	"\"compression\" = \"LZ4\",\n" +
	"\"fast_schema_evolution\" = \"true\",\n" +
	"\"replicated_storage\" = \"true\",\n" +
	"\"replication_num\" = \"1\"\n" +
	");"

const ordersDDL = "CREATE TABLE `orders` (\n" +
	"  `order_id` bigint(20) NOT NULL COMMENT \"Order identifier\",\n" +
	"  `order_date` date NOT NULL COMMENT \"\",\n" +
	"  `customer_id` bigint(20) NULL COMMENT \"\",\n" +
	"  `amount` decimal(10, 2) NULL COMMENT \"\"\n" +
	") ENGINE=OLAP \n" +
	"UNIQUE KEY(`order_id`, `order_date`)\n" +
	"COMMENT \"Orders, latest row wins\"\n" +
	"DISTRIBUTED BY HASH(`order_id`) BUCKETS 3 \n" +
	"ORDER BY(`order_date`, `order_id`)\n" +
	"PROPERTIES (\n" +
	"\"compression\" = \"LZ4\",\n" +
	"\"replication_num\" = \"1\"\n" +
	");"

const dailySalesViewDDL = "CREATE VIEW `daily_sales` (`order_date`, `country`, `total`)\n" +
	"COMMENT \"Daily sales by country\" SECURITY NONE AS SELECT o.order_date, c.country, SUM(o.amount) AS total\n" +
	"FROM orders o JOIN customers c ON o.customer_id = c.customer_id\n" +
	"GROUP BY o.order_date, c.country;"

const mvSalesDDL = "CREATE MATERIALIZED VIEW `mv_sales` (`order_date`, `total`)\n" +
	"COMMENT \"Materialized daily sales\"\n" +
	"DISTRIBUTED BY HASH(`order_date`) BUCKETS 2 \n" +
	"REFRESH ASYNC\n" +
	"PROPERTIES (\n" +
	"\"replicated_storage\" = \"true\",\n" +
	"\"replication_num\" = \"1\",\n" +
	"\"storage_medium\" = \"HDD\"\n" +
	")\n" +
	"AS SELECT order_date, SUM(amount) AS total FROM orders GROUP BY order_date;"

func TestParseTableDDL_ReadsDuplicateKeyModel(t *testing.T) {
	d := parseTableDDL(eventsDDL)

	assert.Equal(t, "DUPLICATE", d.KeyModel)
	assert.Equal(t, []string{"event_date", "event_id"}, d.KeyColumns)
}

func TestParseTableDDL_ReadsUniqueKeyModel(t *testing.T) {
	d := parseTableDDL(ordersDDL)

	assert.Equal(t, "UNIQUE", d.KeyModel)
	assert.Equal(t, []string{"order_id", "order_date"}, d.KeyColumns)
}

func TestParseTableDDL_ReadsAggregateKeyModel(t *testing.T) {
	d := parseTableDDL("AGGREGATE KEY(`stat_date`, `country`)\nDISTRIBUTED BY HASH(`country`) BUCKETS 2")

	assert.Equal(t, "AGGREGATE", d.KeyModel)
	assert.Equal(t, []string{"stat_date", "country"}, d.KeyColumns)
}

func TestParseTableDDL_ReadsPrimaryKeyModel(t *testing.T) {
	d := parseTableDDL("PRIMARY KEY(`customer_id`)\nDISTRIBUTED BY HASH(`customer_id`) BUCKETS 3")

	assert.Equal(t, "PRIMARY", d.KeyModel)
	assert.Equal(t, []string{"customer_id"}, d.KeyColumns)
}

func TestParseTableDDL_ReadsRangePartitioning(t *testing.T) {
	// StarRocks prints the strategy on one line and the individual
	// partitions on the lines after it, so the parser must stop at the
	// end of the PARTITION BY line.
	d := parseTableDDL(eventsDDL)

	assert.Equal(t, "RANGE", d.PartitionType)
	assert.Equal(t, []string{"event_date"}, d.PartitionColumns)
	assert.Empty(t, d.PartitionExpression)
}

func TestParseTableDDL_ReadsListPartitioning(t *testing.T) {
	d := parseTableDDL("PARTITION BY LIST(`region`)\n(PARTITION p1 VALUES IN (\"eu\"))")

	assert.Equal(t, "LIST", d.PartitionType)
	assert.Equal(t, []string{"region"}, d.PartitionColumns)
}

func TestParseTableDDL_ReadsExpressionPartitioningByColumn(t *testing.T) {
	d := parseTableDDL("PARTITION BY (`dt`, `city`)")

	assert.Equal(t, "EXPRESSION", d.PartitionType)
	assert.Equal(t, []string{"dt", "city"}, d.PartitionColumns)
}

func TestParseTableDDL_ReadsExpressionPartitioningByFunction(t *testing.T) {
	// Only the bare identifier arguments are columns; the literal is not.
	d := parseTableDDL("PARTITION BY date_trunc('month', dt)")

	assert.Equal(t, "EXPRESSION", d.PartitionType)
	assert.Equal(t, []string{"dt"}, d.PartitionColumns)
	assert.Equal(t, "date_trunc('month', dt)", d.PartitionExpression)
}

func TestParseTableDDL_ReadsHashDistributionAndBuckets(t *testing.T) {
	d := parseTableDDL(eventsDDL)

	assert.Equal(t, "HASH", d.Distribution)
	assert.Equal(t, []string{"event_id"}, d.DistributionColumns)
	assert.Equal(t, 4, d.Buckets)
}

func TestParseTableDDL_ReadsRandomDistribution(t *testing.T) {
	d := parseTableDDL("DISTRIBUTED BY RANDOM")

	assert.Equal(t, "RANDOM", d.Distribution)
	assert.Empty(t, d.DistributionColumns)
	assert.Zero(t, d.Buckets)
}

func TestParseTableDDL_ReadsOrderBy(t *testing.T) {
	d := parseTableDDL(ordersDDL)

	assert.Equal(t, []string{"order_date", "order_id"}, d.OrderBy)
}

func TestParseTableDDL_LeavesOrderByEmptyWhenAbsent(t *testing.T) {
	d := parseTableDDL(eventsDDL)

	assert.Empty(t, d.OrderBy)
}

func TestParseTableDDL_ReadsProperties(t *testing.T) {
	d := parseTableDDL(eventsDDL)

	assert.Equal(t, "1", d.Properties["replication_num"])
	assert.Equal(t, "LZ4", d.Properties["compression"])
}

func TestParseTableDDL_IgnoresQuotedTextOutsideProperties(t *testing.T) {
	// Column comments are quoted too. Only the block after PROPERTIES is
	// scanned, so a comment cannot invent a property.
	d := parseTableDDL(eventsDDL)

	assert.NotContains(t, d.Properties, "Day the event happened")
	assert.NotContains(t, d.Properties, "Raw event payload")
}

func TestParseTableDDL_ReadsAutoIncrementColumns(t *testing.T) {
	// Neither SHOW FULL COLUMNS nor information_schema.columns reports
	// the flag, so the DDL is the only place it shows.
	ddl := "CREATE TABLE `t` (\n  `id` bigint(20) NOT NULL AUTO_INCREMENT,\n  `name` varchar(10) NULL\n) ENGINE=OLAP"

	d := parseTableDDL(ddl)

	assert.Equal(t, []string{"id"}, d.AutoIncrementColumns)
}

func TestParseTableDDL_ReadsForeignKeyConstraintsProperty(t *testing.T) {
	ddl := "PROPERTIES (\n\"foreign_key_constraints\" = \"(customer_id) REFERENCES default_catalog.shop.customers(customer_id)\"\n);"

	d := parseTableDDL(ddl)

	require.Len(t, d.ForeignKeys, 1)
	assert.Equal(t, []string{"customer_id"}, d.ForeignKeys[0].Columns)
	assert.Equal(t, "default_catalog", d.ForeignKeys[0].ReferencedCatalog)
	assert.Equal(t, "shop", d.ForeignKeys[0].ReferencedDB)
	assert.Equal(t, "customers", d.ForeignKeys[0].ReferencedTable)
}

func TestParseTableDDL_ReadsUnqualifiedForeignKeyReference(t *testing.T) {
	ddl := "PROPERTIES (\n\"foreign_key_constraints\" = \"(sku) REFERENCES products(sku)\"\n);"

	d := parseTableDDL(ddl)

	require.Len(t, d.ForeignKeys, 1)
	assert.Empty(t, d.ForeignKeys[0].ReferencedDB)
	assert.Equal(t, "products", d.ForeignKeys[0].ReferencedTable)
}

func TestParseTableDDL_EmptyInputYieldsNothing(t *testing.T) {
	d := parseTableDDL("")

	assert.Empty(t, d.KeyModel)
	assert.Empty(t, d.PartitionType)
	assert.Empty(t, d.Distribution)
	assert.Empty(t, d.Properties)
}

func TestRedactDDL_HidesPasswordProperty(t *testing.T) {
	ddl := "PROPERTIES (\"password\" = \"hunter2\", \"replication_num\" = \"1\")"

	out := redactDDL(ddl)

	assert.NotContains(t, out, "hunter2")
	assert.Contains(t, out, `"password" = "***"`)
	assert.Contains(t, out, `"replication_num" = "1"`)
}

func TestRedactDDL_HidesAccessKeyProperty(t *testing.T) {
	ddl := `PROPERTIES ("aws.s3.access_key" = "AKIAEXAMPLE", "aws.s3.secret_key" = "s3cr3t")`

	out := redactDDL(ddl)

	assert.NotContains(t, out, "s3cr3t")
}

func TestRedactDDL_KeepsForeignKeyConstraints(t *testing.T) {
	// The property name ends in "constraints", not in a key, and the
	// lineage it declares is worth showing.
	ddl := `PROPERTIES ("foreign_key_constraints" = "(customer_id) REFERENCES shop.customers(customer_id)")`

	out := redactDDL(ddl)

	assert.Contains(t, out, "REFERENCES shop.customers(customer_id)")
}

func TestRedactDDL_KeepsOrdinaryProperties(t *testing.T) {
	ddl := `PROPERTIES ("replication_num" = "1", "compression" = "LZ4", "storage_volume" = "builtin_storage_volume")`

	out := redactDDL(ddl)

	assert.Equal(t, ddl, out)
}

func TestViewQuery_ReturnsSelectOfAView(t *testing.T) {
	// The statement carries a column list and a SECURITY clause before
	// the AS, and the SELECT itself contains "AS total"; the first AS
	// that starts a query is the one that matters.
	query := viewQuery(dailySalesViewDDL)

	assert.True(t, len(query) > 0)
	assert.Equal(t, "SELECT", query[:6])
	assert.Contains(t, query, "FROM orders o JOIN customers c")
	assert.NotContains(t, query, "CREATE VIEW")
	assert.NotContains(t, query, ";")
}

func TestViewQuery_ReturnsSelectOfAMaterializedView(t *testing.T) {
	query := viewQuery(mvSalesDDL)

	assert.Equal(t, "SELECT order_date, SUM(amount) AS total FROM orders GROUP BY order_date", query)
}

func TestViewQuery_EmptyWhenThereIsNoQuery(t *testing.T) {
	assert.Empty(t, viewQuery(eventsDDL))
	assert.Empty(t, viewQuery(""))
}

func TestSplitIdentifiers_DropsBackticksAndSpacing(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, splitIdentifiers(" `a` ,  `b` "))
}

func TestSplitQualifiedName_SplitsCatalogDatabaseTable(t *testing.T) {
	assert.Equal(t, []string{"default_catalog", "shop", "orders"},
		splitQualifiedName("`default_catalog`.`shop`.`orders`"))
}
