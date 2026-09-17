package amundsen

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverTables_ProjectsPostgresOntoThePostgreSQLPlugin(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	assert.Equal(t, []string{"PostgreSQL"}, asset.Providers)
	assert.Equal(t, "mrn://table/postgresql/orders", *asset.MRN)
}

func TestDiscoverTables_ProjectsHiveOntoASchemaQualifiedName(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "sales.orders")
	assert.Equal(t, []string{"Hive"}, asset.Providers)
	assert.Equal(t, "mrn://table/hive/sales.orders", *asset.MRN)
}

func TestDiscoverTables_TakesTheDescriptionFromTheDescriptionNode(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	require.NotNil(t, asset.Description)
	assert.Equal(t, "One row per placed order", *asset.Description)
}

func TestDiscoverTables_LeavesTheDescriptionUnsetWhenAmundsenHasNone(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Nil(t, assetNamed(t, c, "Table", "sales.orders").Description)
}

func TestDiscoverTables_DescriptionsCanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeDescriptions = false

	c := discover(t, config, seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	assert.Nil(t, asset.Description)
	assert.NotContains(t, asset.Metadata, "schema_description")
	assert.NotContains(t, provenanceOf(t, asset), "programmatic_descriptions")
}

func TestDiscoverTables_ColumnsAreOrderedBySortOrder(t *testing.T) {
	// A COLLECT returns columns in whatever order it found them, so the
	// plugin has to put them back in the order the source system uses.
	c := discover(t, testConfig(), seededGraph())

	columns := columnsOf(t, assetNamed(t, c, "Table", "orders"))
	require.Len(t, columns, 3)
	assert.Equal(t, "id", columns[0]["column_name"])
	assert.Equal(t, "customer_id", columns[1]["column_name"])
	assert.Equal(t, "total", columns[2]["column_name"])
}

func TestDiscoverTables_ColumnsKeepTheirOwnTypes(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	columns := columnsOf(t, assetNamed(t, c, "Table", "orders"))
	assert.Equal(t, "integer", columns[0]["data_type"])
	assert.Equal(t, "numeric", columns[2]["data_type"])
}

func TestDiscoverTables_ColumnsKeepTheirOwnDescriptions(t *testing.T) {
	// Collecting name, type and description as one map per column is
	// what stops a column with no description from shifting every later
	// description onto the wrong column.
	c := discover(t, testConfig(), seededGraph())

	columns := columnsOf(t, assetNamed(t, c, "Table", "orders"))
	assert.Equal(t, "Surrogate order key", columns[0]["description"])
	assert.Equal(t, "References customers.id", columns[1]["description"])
	assert.NotContains(t, columns[2], "description", "total has no description in Amundsen")
}

func TestDiscoverTables_ColumnDescriptionsCanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeDescriptions = false

	c := discover(t, config, seededGraph())

	columns := columnsOf(t, assetNamed(t, c, "Table", "orders"))
	assert.NotContains(t, columns[0], "description")
}

func TestDiscoverTables_TableWithNoColumnsCarriesNoColumnSchema(t *testing.T) {
	// Neo4j answers a COLLECT over an OPTIONAL MATCH that found nothing
	// with one map of nulls, which must not become a nameless column.
	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{emptyTableRow()}

	c := discover(t, testConfig(), f)

	assert.NotContains(t, assetNamed(t, c, "Table", "empty_tbl").Schema, "columns")
}

func TestDiscoverTables_IsViewMakesItAView(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, "View", assetNamed(t, c, "View", "sales.orders_view").Type)
}

func TestDiscoverTables_AViewBadgeAlsoMakesItAView(t *testing.T) {
	row := hiveOrdersRow()
	row["badges"] = []any{"view"}
	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{row}

	c := discover(t, testConfig(), f)

	assert.Equal(t, "mrn://view/hive/sales.orders", *assetNamed(t, c, "View", "sales.orders").MRN)
}

func TestDiscoverTables_MissingIsViewMeansATable(t *testing.T) {
	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{emptyTableRow()}

	c := discover(t, testConfig(), f)

	assert.Equal(t, "Table", assetNamed(t, c, "Table", "empty_tbl").Type)
}

func TestDiscoverTables_TagsBecomeAssetTags(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.ElementsMatch(t, []string{"finance", "pii"}, assetNamed(t, c, "Table", "orders").Tags)
}

func TestDiscoverTables_TagsCanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeTags = false

	c := discover(t, config, seededGraph())

	assert.Empty(t, assetNamed(t, c, "Table", "orders").Tags)
}

func TestDiscoverTables_ConfiguredTagsAreAddedToTheAmundsenOnes(t *testing.T) {
	config := testConfig()
	config.Tags = []string{"imported"}

	c := discover(t, config, seededGraph())

	assert.ElementsMatch(t, []string{"finance", "pii", "imported"}, assetNamed(t, c, "Table", "orders").Tags)
}

func TestDiscoverTables_TagsAreRecordedInMetadataEvenWhenNotCopied(t *testing.T) {
	config := testConfig()
	config.IncludeTags = false

	c := discover(t, config, seededGraph())

	assert.Equal(t, []string{"finance", "pii"}, provenanceOf(t, assetNamed(t, c, "Table", "orders"))["tags"])
}

func TestDiscoverTables_ProvenanceRecordsTheAmundsenLevels(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	provenance := provenanceOf(t, assetNamed(t, c, "Table", "orders"))
	assert.Equal(t, "postgres://prod.public/orders", provenance["key"])
	assert.Equal(t, "postgres", provenance["database"])
	assert.Equal(t, "prod", provenance["cluster"])
	assert.Equal(t, "public", provenance["schema"])
	assert.Equal(t, "orders", provenance["table"])
}

func TestDiscoverTables_ClusterIsRecordedEvenWhenItIsNotInTheName(t *testing.T) {
	// Hive's cluster is an environment label rather than a catalog, so
	// it stays out of the identity but a reader still wants to see it.
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, "gold", provenanceOf(t, assetNamed(t, c, "Table", "sales.orders"))["cluster"])
}

func TestDiscoverTables_BadgesAndProgrammaticDescriptionsAreRecorded(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	provenance := provenanceOf(t, assetNamed(t, c, "Table", "orders"))
	assert.Equal(t, []string{"certified"}, provenance["badges"])
	assert.Equal(t, []string{"Built nightly by dbt model orders"}, provenance["programmatic_descriptions"])
}

func TestDiscoverTables_LastUpdatedIsRecordedAsATimestamp(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, "2025-09-04T15:33:20Z", provenanceOf(t, assetNamed(t, c, "Table", "orders"))["last_updated_at"])
}

func TestDiscoverTables_SchemaDescriptionIsCarried(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Equal(t, "Customer facing tables", assetNamed(t, c, "Table", "orders").Metadata["schema_description"])
}

func TestDiscoverTables_OwnersLandInMetadata(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	assert.Equal(t, []string{"Ann Ops", "Bo Analyst"}, asset.Metadata["owners"])
	assert.Equal(t, []string{"ann@marmot.test", "bo@marmot.test"}, asset.Metadata["owner_emails"])
	assert.Equal(t, []string{"Data Platform", "Finance"}, asset.Metadata["owner_teams"])
}

func TestDiscoverTables_OwnersCanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeUsers = false
	f := seededGraph()

	c := discover(t, config, f)

	assert.NotContains(t, assetNamed(t, c, "Table", "orders").Metadata, "owners")
	assert.Equal(t, 0, f.callsFor(tableOwnerQuery), "a run that wants no owners should not ask for them")
}

func TestDiscoverTables_OwnerWithNoFullNameFallsBackToTheirEmail(t *testing.T) {
	f := seededGraph()
	f.rows[tableOwnerQuery] = []map[string]any{
		{"table_key": "postgres://prod.public/orders", "email": "cy@marmot.test", "full_name": nil, "team": nil},
	}

	c := discover(t, testConfig(), f)

	assert.Equal(t, []string{"cy@marmot.test"}, assetNamed(t, c, "Table", "orders").Metadata["owners"])
}

func TestDiscoverTables_UsageBecomesStatistics(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	reads, ok := statisticFor(c, *asset.MRN, metricReadCount)
	require.True(t, ok)
	assert.Equal(t, float64(42), reads)

	readers, ok := statisticFor(c, *asset.MRN, metricUniqueReaders)
	require.True(t, ok)
	assert.Equal(t, float64(2), readers)
}

func TestDiscoverTables_UsageCanBeTurnedOff(t *testing.T) {
	config := testConfig()
	config.IncludeUsage = false
	f := seededGraph()

	c := discover(t, config, f)

	assert.Empty(t, c.statistics)
	assert.Equal(t, 0, f.callsFor(tableUsageQuery), "a run that wants no usage should not ask for it")
}

func TestDiscoverTables_ATableNobodyReadEmitsNoStatistic(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	asset := assetNamed(t, c, "Table", "sales.orders")
	_, ok := statisticFor(c, *asset.MRN, metricReadCount)
	assert.False(t, ok, "a zero read count is noise, not information")
}

func TestDiscoverTables_LinksBackToTheAmundsenPage(t *testing.T) {
	config := testConfig()
	config.AmundsenURL = "https://amundsen.marmot.test"

	c := discover(t, config, seededGraph())

	asset := assetNamed(t, c, "Table", "orders")
	require.NotEmpty(t, asset.ExternalLinks)
	assert.Equal(t, "Open in Amundsen", asset.ExternalLinks[0].Name)
	assert.Equal(t, "https://amundsen.marmot.test/table_detail/prod/postgres/public/orders", asset.ExternalLinks[0].URL)
}

func TestDiscoverTables_NoAmundsenURLMeansNoLink(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.Empty(t, assetNamed(t, c, "Table", "orders").ExternalLinks)
}

func TestDiscoverTables_RecordsAmundsenAsTheSource(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	sources := assetNamed(t, c, "Table", "orders").Sources
	require.Len(t, sources, 1)
	assert.Equal(t, "Amundsen", sources[0].Name)
}

func TestDiscoverTables_TwoTablesProjectingOntoOneMRNBecomeOneAsset(t *testing.T) {
	// A Postgres table's Marmot name is its bare name, so public.orders
	// and staging.orders collide. Matching the native plugin matters
	// more than telling them apart, so the first one wins.
	staging := ordersRow()
	staging["schema"] = "staging"
	staging["key"] = "postgres://prod.staging/orders"

	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{ordersRow(), staging}

	c := discover(t, testConfig(), f)

	assert.Len(t, c.assets, 1)
	assert.Equal(t, "public", provenanceOf(t, c.assets[0])["schema"])
}

func TestDiscoverTables_ATableWithNoNameIsSkipped(t *testing.T) {
	row := ordersRow()
	row["name"] = nil

	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{row}

	c := discover(t, testConfig(), f)

	assert.Empty(t, c.assets)
}

func TestDiscoverTables_AFailedQueryIsFatal(t *testing.T) {
	// A graph that cannot be read is not a graph with nothing in it.
	f := newFakeReader()
	f.errs[tableQuery] = errors.New("connection reset")

	c := newCollector(testConfig())
	err := c.collect(t.Context(), f)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

func TestDiscoverTableLineage_UpstreamFeedsDownstream(t *testing.T) {
	c := discover(t, testConfig(), seededGraph())

	assert.True(t, hasEdge(c, "mrn://table/postgresql/customers", "mrn://table/postgresql/orders", "FEEDS"))
}

func TestDiscoverTableLineage_IsRecordedOnlyOnce(t *testing.T) {
	// Amundsen writes the same lineage as HAS_UPSTREAM and again as
	// HAS_DOWNSTREAM, and a graph can hold the pair twice over.
	f := seededGraph()
	f.rows[tableLineageQuery] = append(f.rows[tableLineageQuery], map[string]any{
		"downstream": "postgres://prod.public/orders",
		"upstream":   "postgres://prod.public/customers",
	})

	c := discover(t, testConfig(), f)

	edges := 0
	for _, edge := range c.lineage {
		if edge.Source == "mrn://table/postgresql/customers" {
			edges++
		}
	}
	assert.Equal(t, 1, edges)
}

func TestDiscoverTableLineage_ResolvesATableOutsideTheRunFromItsKey(t *testing.T) {
	// A table Amundsen knows only as a lineage endpoint still projects
	// onto the identity its own Marmot plugin would produce, so the edge
	// reaches the asset that plugin discovered.
	f := seededGraph()
	f.rows[tableLineageQuery] = []map[string]any{
		{"downstream": "postgres://prod.public/orders", "upstream": "snowflake://analytics.public/raw_orders"},
	}

	c := discover(t, testConfig(), f)

	assert.True(t, hasEdge(c, "mrn://table/snowflake/analytics.public.raw_orders", "mrn://table/postgresql/orders", "FEEDS"))
}

func TestDiscoverTableLineage_SkipsAnUnreadableKey(t *testing.T) {
	f := seededGraph()
	f.rows[tableLineageQuery] = []map[string]any{
		{"downstream": "postgres://prod.public/orders", "upstream": "not-an-amundsen-key"},
	}

	c := discover(t, testConfig(), f)

	assert.False(t, hasEdge(c, "", "mrn://table/postgresql/orders", "FEEDS"))
	for _, edge := range c.lineage {
		assert.NotEmpty(t, edge.Source)
	}
}

func TestParseTableKey_ReadsAllFourLevels(t *testing.T) {
	parts, ok := parseTableKey("postgres://prod.public/orders")

	require.True(t, ok)
	assert.Equal(t, "postgres", parts.Database)
	assert.Equal(t, "prod", parts.Cluster)
	assert.Equal(t, "public", parts.Schema)
	assert.Equal(t, "orders", parts.Table)
}

func TestParseTableKey_KeepsADotInsideASchema(t *testing.T) {
	// The cluster is the first dot separated part, everything after it
	// is the schema.
	parts, ok := parseTableKey("hive://gold.my.schema/orders")

	require.True(t, ok)
	assert.Equal(t, "gold", parts.Cluster)
	assert.Equal(t, "my.schema", parts.Schema)
}

func TestParseTableKey_AcceptsAKeyWithNoSchema(t *testing.T) {
	parts, ok := parseTableKey("elasticsearch://prod/orders")

	require.True(t, ok)
	assert.Equal(t, "prod", parts.Cluster)
	assert.Equal(t, "", parts.Schema)
	assert.Equal(t, "orders", parts.Table)
}

func TestParseTableKey_RejectsAKeyWithNoScheme(t *testing.T) {
	_, ok := parseTableKey("prod.public/orders")

	assert.False(t, ok)
}

func TestParseTableKey_RejectsAKeyWithNoTable(t *testing.T) {
	_, ok := parseTableKey("postgres://prod.public")

	assert.False(t, ok)
}

func TestParseTableKey_RejectsAnEmptyKey(t *testing.T) {
	_, ok := parseTableKey("")

	assert.False(t, ok)
}

func TestEachPage_FollowsPaginationToTheEnd(t *testing.T) {
	config := testConfig()
	config.PageSize = 2

	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{hiveOrdersRow(), hiveOrdersViewRow(), customersRow(), ordersRow()}

	c := discover(t, config, f)

	assert.Len(t, c.assets, 4, "every page has to be read, not just the first")
	assert.Equal(t, 3, f.callsFor(tableQuery), "two full pages then one short page")
}

func TestEachPage_StopsOnAShortPage(t *testing.T) {
	config := testConfig()
	config.PageSize = 10

	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{ordersRow()}

	discover(t, config, f)

	assert.Equal(t, 1, f.callsFor(tableQuery))
}

func TestEachPage_ReportsWhichQueryFailed(t *testing.T) {
	f := newFakeReader()
	f.errs[tableOwnerQuery] = errors.New("boom")

	c := newCollector(testConfig())
	err := c.collect(t.Context(), f)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "table owners")
}

func TestEachPage_PassesSkipAndLimitAsInts(t *testing.T) {
	// Neo4j rejects a SKIP or LIMIT that is not an integer.
	config := testConfig()
	config.PageSize = 2

	f := newFakeReader()
	f.rows[tableQuery] = []map[string]any{ordersRow(), customersRow(), hiveOrdersRow()}

	discover(t, config, f)

	var seen []string
	for _, call := range f.calls {
		if call.query == tableQuery {
			seen = append(seen, fmt.Sprintf("%d:%d", call.skip, call.limit))
		}
	}
	assert.Equal(t, []string{"0:2", "2:2"}, seen)
}
