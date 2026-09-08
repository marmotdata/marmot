package hive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The testdata fixtures are DESCRIBE FORMATTED output captured from Apache
// Hive 4.0.1 over HiveServer2: one tab-separated (col_name, data_type,
// comment) row per line, NULL cells empty, exactly as the Thrift client
// returns them.
func loadDescribe(t *testing.T, name string) [][]string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "describe_"+name+".tsv"))
	require.NoError(t, err)

	var rows [][]string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

func TestParseDescribeFormatted_ReadsColumnsWithTypesVerbatim(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	require.Len(t, info.Columns, 6)
	assert.Equal(t, columnInfo{Name: "id", DataType: "int", Comment: "Customer id"}, info.Columns[0])
	assert.Equal(t, columnInfo{Name: "balance", DataType: "decimal(10,2)", Comment: "Account balance"}, info.Columns[3])
	assert.Equal(t, "map<string,string>", info.Columns[4].DataType)
	assert.Equal(t, "array<struct<sku:string,qty:int>>", info.Columns[5].DataType)
	assert.Equal(t, "", info.Columns[2].Comment, "a column without a comment")
}

func TestParseDescribeFormatted_StopsColumnsAtTheBlankRow(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	for _, col := range info.Columns {
		assert.False(t, strings.HasPrefix(col.Name, "#"), "section header leaked into columns: %s", col.Name)
		assert.False(t, strings.HasSuffix(col.Name, ":"), "detail row leaked into columns: %s", col.Name)
	}
}

func TestParseDescribeFormatted_ReadsPartitionColumnsSeparately(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))

	require.Len(t, info.Columns, 3)
	require.Len(t, info.PartitionColumns, 1)
	assert.Equal(t, columnInfo{Name: "dt", DataType: "string", Comment: "Event date"}, info.PartitionColumns[0])
}

func TestParseDescribeFormatted_ReadsTheDetailBlock(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "events"))

	assert.Equal(t, "sales", info.Details["Database"])
	assert.Equal(t, "hive", info.Details["Owner"])
	assert.Equal(t, "USER", info.Details["OwnerType"])
	assert.Equal(t, "Tue Sep 08 02:21:24 UTC 2026", info.Details["CreateTime"])
	assert.Equal(t, "UNKNOWN", info.Details["LastAccessTime"])
	assert.Equal(t, "file:/tmp/marmot-hive-events", info.Details["Location"])
	assert.Equal(t, "EXTERNAL_TABLE", info.Details["Table Type"])
}

func TestParseDescribeFormatted_ReadsTableParameters(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	assert.Equal(t, "Customer master data", info.Parameters["comment"])
	assert.Equal(t, "2", info.Parameters["numRows"])
	assert.Equal(t, "103", info.Parameters["totalSize"])
	assert.Equal(t, "TRUE", info.Parameters["EXTERNAL"])
	assert.Equal(t, `{"BASIC_STATS":"true"}`, info.Parameters["COLUMN_STATS_ACCURATE"])
}

func TestParseDescribeFormatted_KeepsTableParametersOutOfTheDetails(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	_, leaked := info.Details["numRows"]
	assert.False(t, leaked)
	_, leaked = info.Details["Table Parameters"]
	assert.False(t, leaked)
}

func TestParseDescribeFormatted_ReadsStorageInformation(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcSerde", info.Storage["SerDe Library"])
	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcInputFormat", info.Storage["InputFormat"])
	assert.Equal(t, "org.apache.hadoop.hive.ql.io.orc.OrcOutputFormat", info.Storage["OutputFormat"])
	assert.Equal(t, "No", info.Storage["Compressed"])
	assert.Equal(t, "1", info.StorageParams["serialization.format"])
}

func TestParseDescribeFormatted_ReadsBucketing(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	assert.Equal(t, 4, info.numBuckets())
	assert.Equal(t, []string{"customer_id"}, info.bucketColumns())
	assert.Nil(t, info.sortColumns())
}

func TestParseDescribeFormatted_UnbucketedTableHasNoBuckets(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	assert.Equal(t, -1, info.numBuckets(), "Hive reports -1 for unbucketed tables")
	assert.Nil(t, info.bucketColumns())
}

func TestParseDescribeFormatted_ReadsThePrimaryKey(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "customers"))

	assert.Equal(t, []string{"id"}, info.PrimaryKey)
}

func TestParseDescribeFormatted_ReadsForeignKeys(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "orders"))

	require.Len(t, info.ForeignKeys, 1)
	assert.Equal(t, foreignKey{
		Name:         "fk_orders_customer",
		Column:       "customer_id",
		ParentTable:  "sales.customers",
		ParentColumn: "id",
	}, info.ForeignKeys[0])
}

func TestParseDescribeFormatted_ReadsNotNullAndDefaultConstraints(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock"))

	assert.Equal(t, []string{"sku"}, info.PrimaryKey)
	assert.Equal(t, []string{"sku"}, info.NotNull)
	assert.Equal(t, map[string]string{"qty": "0"}, info.Defaults)
}

func TestParseDescribeFormatted_ReadsACompositePrimaryKey(t *testing.T) {
	rows := [][]string{
		{"sku", "string", ""},
		{"warehouse", "string", ""},
		{"", "", ""},
		{"# Constraints", "", ""},
		{"", "", ""},
		{"# Primary Key", "", ""},
		{"Table:              ", "scratch.inventory   ", ""},
		{"Constraint Name:    ", "pk_98943542_1788834275800_0", ""},
		{"Column Name:        ", "sku                 ", ""},
		{"Column Name:        ", "warehouse           ", ""},
	}

	info := parseDescribeFormatted(rows)

	assert.Equal(t, []string{"sku", "warehouse"}, info.PrimaryKey)
}

func TestParseDescribeFormatted_ReadsTheViewQueryAcrossLines(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	assert.Equal(t, "VIRTUAL_VIEW", info.Details["Table Type"])
	assert.Equal(t, "SELECT c.name, SUM(o.amount) AS total\nFROM sales.orders o\nJOIN sales.customers c ON o.customer_id = c.id\nGROUP BY c.name", info.OriginalQuery)
	assert.Equal(t, "SELECT `c`.`name`, SUM(`o`.`amount`) AS `total`\nFROM `sales`.`orders` `o`\nJOIN `sales`.`customers` `c` ON `o`.`customer_id` = `c`.`id`\nGROUP BY `c`.`name`", info.ExpandedQuery)
}

func TestParseDescribeFormatted_ViewHasNoSerde(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "daily_totals"))

	assert.Equal(t, "null", info.Storage["SerDe Library"], "Hive prints the literal null for a view")
	assert.Empty(t, info.Details["Location"])
}

func TestParseDescribeFormatted_ReadsMaterializedViewSources(t *testing.T) {
	info := parseDescribeFormatted(loadDescribe(t, "stock_totals"))

	assert.Equal(t, "MATERIALIZED_VIEW", info.Details["Table Type"])
	assert.Equal(t, "SELECT sku, SUM(qty) AS total FROM sales.stock GROUP BY sku", info.OriginalQuery)
	assert.Equal(t, []string{"sales.stock"}, info.SourceTables)
	assert.Equal(t, "Yes", info.Details["Rewrite Enabled"])
}

func TestParseDescribeFormatted_ObjectTypes(t *testing.T) {
	assert.Equal(t, "external", parseDescribeFormatted(loadDescribe(t, "customers")).objectType())
	assert.Equal(t, "managed", parseDescribeFormatted(loadDescribe(t, "stock")).objectType())
	assert.Equal(t, "view", parseDescribeFormatted(loadDescribe(t, "daily_totals")).objectType())
	assert.Equal(t, "materialized_view", parseDescribeFormatted(loadDescribe(t, "stock_totals")).objectType())
}

func TestParseDescribeFormatted_ViewsAreViews(t *testing.T) {
	assert.True(t, parseDescribeFormatted(loadDescribe(t, "daily_totals")).isView())
	assert.True(t, parseDescribeFormatted(loadDescribe(t, "stock_totals")).isView())
	assert.False(t, parseDescribeFormatted(loadDescribe(t, "stock")).isView())
}

func TestParseDescribeFormatted_ReadsTransactional(t *testing.T) {
	transactional, ok := parseDescribeFormatted(loadDescribe(t, "stock")).paramBool("transactional")
	assert.True(t, ok)
	assert.True(t, transactional)

	_, ok = parseDescribeFormatted(loadDescribe(t, "customers")).paramBool("transactional")
	assert.False(t, ok, "a non-ACID table has no transactional parameter")
}

func TestParseDescribeFormatted_EmptyInput(t *testing.T) {
	info := parseDescribeFormatted(nil)

	assert.Empty(t, info.Columns)
	assert.Equal(t, "", info.objectType())
	assert.False(t, info.isView())
}

func TestParseDescribeFormatted_ToleratesShortRows(t *testing.T) {
	info := parseDescribeFormatted([][]string{{"id", "int"}, {"name"}, {}})

	require.Len(t, info.Columns, 2)
	assert.Equal(t, "int", info.Columns[0].DataType)
	assert.Equal(t, "name", info.Columns[1].Name)
}

func TestParamInt_ReadsNumbersAndRejectsTheRest(t *testing.T) {
	info := &tableInfo{Parameters: map[string]string{"numRows": " 42 ", "totalSize": "big"}}

	n, ok := info.paramInt("numRows")
	assert.True(t, ok)
	assert.Equal(t, int64(42), n)

	_, ok = info.paramInt("totalSize")
	assert.False(t, ok)
	_, ok = info.paramInt("missing")
	assert.False(t, ok)
}

func TestParseList_ReadsJavaListsAndSortOrders(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, parseList("[a, b]"))
	assert.Nil(t, parseList("[]"))
	assert.Equal(t, []string{"ts", "b"}, parseList("[Order(col:ts, order:1), Order(col:b, order:0)]"))
}

func TestHiveTime_ConvertsJavaDatesToRFC3339(t *testing.T) {
	assert.Equal(t, "2026-09-08T02:21:24Z", hiveTime("Tue Sep 08 02:21:24 UTC 2026"))
}

func TestHiveTime_DropsUnknownAndKeepsUnparseableText(t *testing.T) {
	assert.Equal(t, "", hiveTime("UNKNOWN"))
	assert.Equal(t, "", hiveTime(""))
	assert.Equal(t, "yesterday", hiveTime("yesterday"))
}

func TestKeyValue_SplitsBothShapesHiveUses(t *testing.T) {
	key, value, ok := keyValue("Owner:              ", "hive                ")
	assert.True(t, ok)
	assert.Equal(t, "Owner", key)
	assert.Equal(t, "hive", value)

	key, value, ok = keyValue("Column Name:qty     ", "")
	assert.True(t, ok)
	assert.Equal(t, "Column Name", key)
	assert.Equal(t, "qty", value)

	_, _, ok = keyValue("id", "int")
	assert.False(t, ok)
}

func TestSplitParentColumn_SeparatesTheColumnFromTheTable(t *testing.T) {
	table, column := splitParentColumn("sales.customers.id")
	assert.Equal(t, "sales.customers", table)
	assert.Equal(t, "id", column)
}
