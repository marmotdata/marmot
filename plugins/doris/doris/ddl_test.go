package doris

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixtures below are verbatim SHOW CREATE output from Doris 2.1.0
// (apache/doris:doris-all-in-one-2.1.0), trimmed of the properties the
// parser never reads.

var eventsDDL = strings.Join([]string{
	"CREATE TABLE `events` (",
	"  `event_date` DATE NOT NULL COMMENT 'Day the event happened',",
	"  `event_id` BIGINT NOT NULL COMMENT 'Event identifier',",
	"  `customer_id` INT NOT NULL COMMENT 'Customer who triggered the event',",
	"  `event_type` VARCHAR(32) NOT NULL DEFAULT \"click\" COMMENT 'click, view or purchase',",
	"  `amount` DECIMAL(9, 2) NULL COMMENT 'Purchase amount',",
	"  `tags` ARRAY<INT> NULL COMMENT 'Tag ids attached to the event',",
	"  `props` STRUCT<source:TEXT,campaign:TEXT> NULL COMMENT 'Attribution',",
	"  `created_at` DATETIME NULL",
	") ENGINE=OLAP",
	"DUPLICATE KEY(`event_date`, `event_id`)",
	"COMMENT 'Raw click stream events'",
	"PARTITION BY RANGE(`event_date`)",
	"(PARTITION p202601 VALUES [('2026-01-01'), ('2026-02-01')),",
	"PARTITION p202602 VALUES [('2026-02-01'), ('2026-03-01')))",
	"DISTRIBUTED BY HASH(`event_id`) BUCKETS 4",
	"PROPERTIES (",
	"\"replication_allocation\" = \"tag.location.default: 1\",",
	"\"min_load_replica_num\" = \"-1\",",
	"\"storage_medium\" = \"hdd\",",
	"\"storage_format\" = \"V2\"",
	");",
}, "\n")

var customersDDL = strings.Join([]string{
	"CREATE TABLE `customers` (",
	"  `customer_id` INT NOT NULL COMMENT 'Customer identifier',",
	"  `email` VARCHAR(255) NOT NULL,",
	"  `country` CHAR(2) NULL DEFAULT \"NL\",",
	"  `lifetime_value` DECIMAL(12, 2) NULL DEFAULT \"0\"",
	") ENGINE=OLAP",
	"UNIQUE KEY(`customer_id`)",
	"COMMENT 'OLAP'",
	"DISTRIBUTED BY HASH(`customer_id`) BUCKETS 2",
	"PROPERTIES (",
	"\"replication_allocation\" = \"tag.location.default: 1\",",
	"\"enable_unique_key_merge_on_write\" = \"true\"",
	");",
}, "\n")

var dailyStatsDDL = strings.Join([]string{
	"CREATE TABLE `daily_stats` (",
	"  `stat_date` DATE NOT NULL,",
	"  `country` CHAR(2) NOT NULL,",
	"  `purchases` BIGINT SUM NULL DEFAULT \"0\" COMMENT 'Purchases that day',",
	"  `visitors` HLL HLL_UNION NOT NULL COMMENT 'Approximate distinct visitors'",
	") ENGINE=OLAP",
	"AGGREGATE KEY(`stat_date`, `country`)",
	"COMMENT 'OLAP'",
	"DISTRIBUTED BY HASH(`stat_date`) BUCKETS 1",
	"PROPERTIES (",
	"\"replication_allocation\" = \"tag.location.default: 1\"",
	");",
}, "\n")

var ordersDDL = strings.Join([]string{
	"CREATE TABLE `orders` (",
	"  `order_id` BIGINT NOT NULL,",
	"  `customer_id` INT NOT NULL",
	") ENGINE=OLAP",
	"DUPLICATE KEY(`order_id`)",
	"COMMENT 'OLAP'",
	"DISTRIBUTED BY RANDOM BUCKETS AUTO",
	"PROPERTIES (",
	"\"replication_allocation\" = \"tag.location.default: 1\"",
	");",
}, "\n")

var mvSalesDDL = strings.Join([]string{
	"CREATE MATERIALIZED VIEW `mv_sales` (",
	"  `event_date` DATEV2 NOT NULL,",
	"  `country` CHAR(2) NULL,",
	"  `revenue` DECIMALV3(38, 2) NULL",
	") ENGINE=MATERIALIZED_VIEW",
	"COMMENT 'MATERIALIZED_VIEW'",
	"DISTRIBUTED BY HASH(`event_date`) BUCKETS 2",
	"PROPERTIES (",
	"\"replication_allocation\" = \"tag.location.default: 1\",",
	"\"enable_duplicate_without_keys_by_default\" = \"true\"",
	");",
}, "\n")

const dailySalesViewDDL = "CREATE VIEW `daily_sales` COMMENT 'Purchases per day and country' AS " +
	"SELECT `e`.`event_date` AS `event_date`, `c`.`country` AS `country`, sum(`e`.`amount`) AS `revenue`, count(*) AS `purchases` " +
	"FROM `shop`.`events` e  INNER JOIN `shop`.`customers` c ON (`e`.`customer_id` = `c`.`customer_id`) " +
	"WHERE (`e`.`event_type` = 'purchase') GROUP BY `e`.`event_date`, `c`.`country`;"

// mv_infos hands back the query as the user typed it, unquoted.
const mvSalesQuery = "SELECT e.event_date, c.country, SUM(e.amount) AS revenue\n" +
	"FROM shop.events e JOIN shop.customers c ON e.customer_id = c.customer_id\n" +
	"GROUP BY e.event_date, c.country"

func TestParseTableDDL_ReadsTheEngine(t *testing.T) {
	assert.Equal(t, "OLAP", parseTableDDL(eventsDDL).Engine)
}

func TestParseTableDDL_ReadsADuplicateKeyModel(t *testing.T) {
	parsed := parseTableDDL(eventsDDL)

	assert.Equal(t, "DUPLICATE", parsed.KeyModel)
	assert.Equal(t, []string{"event_date", "event_id"}, parsed.KeyColumns)
}

func TestParseTableDDL_ReadsAUniqueKeyModel(t *testing.T) {
	parsed := parseTableDDL(customersDDL)

	assert.Equal(t, "UNIQUE", parsed.KeyModel)
	assert.Equal(t, []string{"customer_id"}, parsed.KeyColumns)
}

func TestParseTableDDL_ReadsAnAggregateKeyModel(t *testing.T) {
	parsed := parseTableDDL(dailyStatsDDL)

	assert.Equal(t, "AGGREGATE", parsed.KeyModel)
	assert.Equal(t, []string{"stat_date", "country"}, parsed.KeyColumns)
}

func TestParseTableDDL_ReadsRangePartitioning(t *testing.T) {
	parsed := parseTableDDL(eventsDDL)

	assert.Equal(t, "RANGE", parsed.PartitionType)
	assert.Equal(t, []string{"event_date"}, parsed.PartitionColumns)
}

func TestParseTableDDL_ReadsListPartitioning(t *testing.T) {
	ddl := strings.Replace(eventsDDL, "PARTITION BY RANGE(`event_date`)", "PARTITION BY LIST(`event_type`)", 1)

	parsed := parseTableDDL(ddl)

	assert.Equal(t, "LIST", parsed.PartitionType)
	assert.Equal(t, []string{"event_type"}, parsed.PartitionColumns)
}

func TestParseTableDDL_ReadsAutoPartitioningWithAnExpression(t *testing.T) {
	// Auto partitioning wraps the column in a function; the whole
	// expression is kept so the reader sees what the partition is on.
	ddl := strings.Replace(eventsDDL, "PARTITION BY RANGE(`event_date`)", "AUTO PARTITION BY RANGE (date_trunc(`event_date`, 'month'))", 1)

	parsed := parseTableDDL(ddl)

	assert.Equal(t, "RANGE", parsed.PartitionType)
	assert.Equal(t, []string{"date_trunc(event_date, 'month')"}, parsed.PartitionColumns)
}

func TestParseTableDDL_UnpartitionedTableHasNoPartitionType(t *testing.T) {
	parsed := parseTableDDL(customersDDL)

	assert.Empty(t, parsed.PartitionType)
	assert.Empty(t, parsed.PartitionColumns)
}

func TestParseTableDDL_ReadsHashDistributionAndBuckets(t *testing.T) {
	parsed := parseTableDDL(eventsDDL)

	assert.Equal(t, "HASH", parsed.DistributionType)
	assert.Equal(t, []string{"event_id"}, parsed.DistributionColumns)
	assert.Equal(t, 4, parsed.Buckets)
	assert.False(t, parsed.AutoBucket)
}

func TestParseTableDDL_ReadsRandomDistributionWithAutoBuckets(t *testing.T) {
	parsed := parseTableDDL(ordersDDL)

	assert.Equal(t, "RANDOM", parsed.DistributionType)
	assert.Empty(t, parsed.DistributionColumns)
	assert.True(t, parsed.AutoBucket)
	assert.Zero(t, parsed.Buckets)
}

func TestParseTableDDL_ReadsProperties(t *testing.T) {
	parsed := parseTableDDL(eventsDDL)

	assert.Equal(t, "tag.location.default: 1", parsed.Properties["replication_allocation"])
	assert.Equal(t, "hdd", parsed.Properties["storage_medium"])
}

func TestParseTableDDL_MaterializedViewHasNoKeyModel(t *testing.T) {
	parsed := parseTableDDL(mvSalesDDL)

	assert.Equal(t, "MATERIALIZED_VIEW", parsed.Engine)
	assert.Empty(t, parsed.KeyModel)
	assert.Equal(t, "HASH", parsed.DistributionType)
	assert.Equal(t, []string{"event_date"}, parsed.DistributionColumns)
	assert.Equal(t, 2, parsed.Buckets)
}

func TestParseProperties_IgnoresColumnDefaults(t *testing.T) {
	// DEFAULT "click" higher up uses the same quoting as a property but
	// is not one.
	props := parseProperties(eventsDDL)

	assert.NotContains(t, props, "click")
	assert.Contains(t, props, "storage_medium")
}

func TestParseProperties_NoPropertiesBlockGivesNil(t *testing.T) {
	assert.Nil(t, parseProperties("CREATE TABLE `t` (`a` INT) ENGINE=OLAP"))
}

func TestExtractViewQuery_ReturnsTheSelectBody(t *testing.T) {
	query := extractViewQuery(dailySalesViewDDL)

	assert.True(t, strings.HasPrefix(query, "SELECT `e`.`event_date`"), query)
	assert.True(t, strings.HasSuffix(query, "GROUP BY `e`.`event_date`, `c`.`country`"), query)
	assert.NotContains(t, query, "CREATE VIEW")
}

func TestExtractViewQuery_KeepsAWithClause(t *testing.T) {
	ddl := "CREATE VIEW `v` AS WITH recent AS (SELECT * FROM events) SELECT count(*) FROM recent"

	assert.Equal(t, "WITH recent AS (SELECT * FROM events) SELECT count(*) FROM recent", extractViewQuery(ddl))
}

func TestExtractViewQuery_FallsBackToTheWholeStatement(t *testing.T) {
	assert.Equal(t, "CREATE VIEW `v`", extractViewQuery("  CREATE VIEW `v`  "))
}

func TestReferencedTables_FindsFromAndJoinTables(t *testing.T) {
	refs := referencedTables(extractViewQuery(dailySalesViewDDL))

	assert.Equal(t, []tableRef{
		{Database: "shop", Table: "events"},
		{Database: "shop", Table: "customers"},
	}, refs)
}

func TestReferencedTables_HandlesUnquotedNames(t *testing.T) {
	refs := referencedTables(mvSalesQuery)

	assert.Equal(t, []tableRef{
		{Database: "shop", Table: "events"},
		{Database: "shop", Table: "customers"},
	}, refs)
}

func TestReferencedTables_LeavesTheDatabaseEmptyForABareName(t *testing.T) {
	assert.Equal(t, []tableRef{{Table: "events"}}, referencedTables("SELECT * FROM events WHERE amount > 0"))
}

func TestReferencedTables_DropsTheCatalogQualifier(t *testing.T) {
	assert.Equal(t, []tableRef{{Database: "shop", Table: "events"}}, referencedTables("SELECT * FROM internal.shop.events"))
}

func TestReferencedTables_HandlesCommaSeparatedFrom(t *testing.T) {
	refs := referencedTables("SELECT * FROM events e, customers AS c WHERE e.customer_id = c.customer_id")

	assert.Equal(t, []tableRef{{Table: "events"}, {Table: "customers"}}, refs)
}

func TestReferencedTables_HandlesSubqueries(t *testing.T) {
	// The subquery alias follows a closing bracket, not FROM, so it is
	// never taken for a table.
	refs := referencedTables("SELECT * FROM (SELECT * FROM events) recent JOIN customers ON recent.customer_id = customers.customer_id")

	assert.Equal(t, []tableRef{{Table: "events"}, {Table: "customers"}}, refs)
}

func TestReferencedTables_SkipsFunctionCalls(t *testing.T) {
	assert.Empty(t, referencedTables(`SELECT Name FROM mv_infos("database"="shop")`))
}

func TestReferencedTables_DedupesRepeatedReferences(t *testing.T) {
	refs := referencedTables("SELECT * FROM events UNION ALL SELECT * FROM events")

	assert.Equal(t, []tableRef{{Table: "events"}}, refs)
}

func TestReferencedTables_IgnoresComments(t *testing.T) {
	query := "-- FROM nowhere\nSELECT * /* FROM elsewhere */ FROM events"

	assert.Equal(t, []tableRef{{Table: "events"}}, referencedTables(query))
}

func TestReferencedTables_IgnoresStringLiterals(t *testing.T) {
	query := "SELECT * FROM events WHERE note = 'FROM fake JOIN other'"

	assert.Equal(t, []tableRef{{Table: "events"}}, referencedTables(query))
}

func TestReferencedTables_StopsAtReservedWords(t *testing.T) {
	// LEFT after the table name is the next join, not an alias.
	refs := referencedTables("SELECT * FROM events LEFT JOIN customers ON 1 = 1")

	assert.Equal(t, []tableRef{{Table: "events"}, {Table: "customers"}}, refs)
}

func TestTokenize_KeepsDottedBacktickNamesTogether(t *testing.T) {
	tokens := tokenize("FROM `shop`.`events` e")

	require.Len(t, tokens, 3)
	assert.Equal(t, "FROM", tokens[0].text)
	assert.Equal(t, "shop.events", tokens[1].text)
	assert.Equal(t, "e", tokens[2].text)
}

func TestTokenize_ReadsAWholeBareWord(t *testing.T) {
	tokens := tokenize("SELECT amount FROM shop.events")

	require.Len(t, tokens, 4)
	assert.Equal(t, "SELECT", tokens[0].text)
	assert.Equal(t, "amount", tokens[1].text)
	assert.Equal(t, "FROM", tokens[2].text)
	assert.Equal(t, "shop.events", tokens[3].text)
}

func TestParseForeignKey_ReadsTheReferencedTable(t *testing.T) {
	// Verbatim SHOW CONSTRAINTS definition from Doris 2.1.0.
	fk, ok := parseForeignKey("FOREIGN KEY (customer_id) REFERENCES shop.customers (customer_id)")

	require.True(t, ok)
	assert.Equal(t, []string{"customer_id"}, fk.Columns)
	assert.Equal(t, tableRef{Database: "shop", Table: "customers"}, fk.ReferencedTable)
	assert.Equal(t, []string{"customer_id"}, fk.ReferencedColumns)
}

func TestParseForeignKey_HandlesCompositeKeys(t *testing.T) {
	fk, ok := parseForeignKey("FOREIGN KEY (`a`, `b`) REFERENCES `db`.`t` (`x`, `y`)")

	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, fk.Columns)
	assert.Equal(t, tableRef{Database: "db", Table: "t"}, fk.ReferencedTable)
	assert.Equal(t, []string{"x", "y"}, fk.ReferencedColumns)
}

func TestParseForeignKey_LeavesTheDatabaseEmptyForABareName(t *testing.T) {
	fk, ok := parseForeignKey("FOREIGN KEY (customer_id) REFERENCES customers (customer_id)")

	require.True(t, ok)
	assert.Equal(t, tableRef{Table: "customers"}, fk.ReferencedTable)
}

func TestParseForeignKey_RejectsOtherConstraints(t *testing.T) {
	_, ok := parseForeignKey("PRIMARY KEY (customer_id)")

	assert.False(t, ok)
}
