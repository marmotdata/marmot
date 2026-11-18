package lineage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The replay tests send a producer's captured events (testdata/openlineage/events/<producer>) in the order they arrived and compare every asset and edge in the catalog with the lineage written out by hand from what the events say.

func dl(job string) string     { return "mrn://asset/job/duckdb/duckdb." + job }
func duck(table string) string { return "mrn://asset/table/duckdb/shopdb.main." + table }

func directCols(cols ...string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c + " => " + c + " (DIRECT)"
	}
	return out
}

func TestReplayDuckLineage(t *testing.T) {
	h := newHarness(t)
	h.replay("duck_lineage")

	// duck_lineage names the attached Postgres database pg; the dataSource facet says it is shop on 127.0.0.1:55433, the Postgres plugin's naming.
	po, pc := "mrn://asset/table/postgresql/shop.public.orders", "mrn://asset/table/postgresql/shop.public.customers"
	out, csv := "mrn://asset/file/file/out.parquet", "mrn://asset/file/file/regions.csv"
	view := "mrn://asset/view/duckdb/shopdb.main.eu_orders"
	copyJob, ctOrders, ctCustomers := dl("copy_orders_8aff966c"), dl("create_table_orders_orders_80b43dc7"), dl("create_table_customers_customers_4a7e8f3f")
	ctRegions, ctRevenue, insOrders := dl("create_table_regions_bcdfbfd7"), dl("create_table_revenue_by_region_orders_customers_dd011b2b"), dl("insert_orders_7bdd2724")
	ctTop, alter, ctScratch, drop := dl("create_table_top_regions_revenue_by_region_07987a7e"), dl("alter_70dd2edf"), dl("create_table_scratch_c9fd686f"), dl("drop_20e736a5")
	ctView, sel, ctUniq, insUniq := dl("create_view_bf84bdbb"), dl("select_orders_fe7db00f"), dl("create_table_uniq_e002db6f"), dl("insert_uniq_orders_orders_1da94a62")

	wantAssets := []string{
		po, pc, out, csv, view,
		duck("orders"), duck("customers"), duck("regions"), duck("revenue_by_region"), duck("best_regions"), duck("uniq"),
		copyJob, ctOrders, ctCustomers, ctRegions, ctRevenue, insOrders, ctTop, alter, ctScratch, drop, ctView, sel, ctUniq, insUniq,
	}
	assert.ElementsMatch(t, wantAssets, h.allMRNs(), "top_regions was renamed and scratch dropped")

	ordersCols := directCols("created_at", "customer_id", "id", "total")
	wantEdges := []string{
		// COPY (SELECT * FROM pg.public.orders) TO 'out.parquet'
		e(po, "CONSUMES", copyJob), e(copyJob, "PRODUCES", out), colEdge(po, out, copyJob, ordersCols...),
		// CREATE TABLE main.orders AS SELECT * FROM pg.public.orders
		e(po, "CONSUMES", ctOrders), e(ctOrders, "PRODUCES", duck("orders")), colEdge(po, duck("orders"), ctOrders, ordersCols...),
		// CREATE TABLE main.customers AS SELECT * FROM pg.public.customers
		e(pc, "CONSUMES", ctCustomers), e(ctCustomers, "PRODUCES", duck("customers")), colEdge(pc, duck("customers"), ctCustomers, directCols("id", "name", "region")...),
		// CREATE TABLE main.regions AS SELECT * FROM read_csv('regions.csv')
		e(csv, "CONSUMES", ctRegions), e(ctRegions, "PRODUCES", duck("regions")), colEdge(csv, duck("regions"), ctRegions, directCols("id", "label", "region")...),
		// The revenue join. duck_lineage names two of its output columns #[4.0] and #[4.1], which the output's schema does not have, so only revenue maps.
		e(duck("orders"), "CONSUMES", ctRevenue), e(duck("customers"), "CONSUMES", ctRevenue), e(duck("regions"), "CONSUMES", ctRevenue),
		e(ctRevenue, "PRODUCES", duck("revenue_by_region")),
		colEdge(duck("orders"), duck("revenue_by_region"), ctRevenue, "total => revenue (INDIRECT)"),
		// INSERT INTO main.orders SELECT * FROM read_parquet('out.parquet')
		e(out, "CONSUMES", insOrders), e(insOrders, "PRODUCES", duck("orders")), colEdge(out, duck("orders"), insOrders, ordersCols...),
		// CREATE TABLE main.top_regions ..., later renamed best_regions: the asset moved, so its lineage is best_regions'.
		e(duck("revenue_by_region"), "CONSUMES", ctTop), e(ctTop, "PRODUCES", duck("best_regions")),
		colEdge(duck("revenue_by_region"), duck("best_regions"), ctTop, directCols("region", "revenue")...),
		// ALTER TABLE ... RENAME TO best_regions
		e(alter, "PRODUCES", duck("best_regions")),
		// CREATE VIEW main.eu_orders; duck_lineage lists only orders as an input, although the view's subquery reads customers too.
		e(duck("orders"), "CONSUMES", ctView), e(ctView, "PRODUCES", view),
		// A SELECT reads orders and writes nothing.
		e(duck("orders"), "CONSUMES", sel),
		// CREATE TABLE main.uniq, then an INSERT that failed on a duplicate key, which duck_lineage reports as COMPLETE.
		e(ctUniq, "PRODUCES", duck("uniq")),
		e(duck("orders"), "CONSUMES", insUniq), e(insUniq, "PRODUCES", duck("uniq")), colEdge(duck("orders"), duck("uniq"), insUniq, directCols("id")...),
	}
	requireGraph(t, graph{Nodes: wantAssets, Edges: wantEdges}, graph{Nodes: h.allMRNs(), Edges: h.allEdges()})

	// The asset page view of the middle of the chain, two hops each way. The direct column edge makes orders one hop upstream of revenue_by_region as well as two through the job, so orders' own sources, Postgres orders and out.parquet, and the jobs that loaded it are within two hops too; customers and regions have no direct edge and are only reached through the job.
	requireGraph(t, graph{
		Nodes: []string{duck("revenue_by_region"), ctRevenue, duck("orders"), duck("customers"), duck("regions"), ctTop, duck("best_regions"), po, out, ctOrders, insOrders},
		Edges: []string{
			e(po, "CONSUMES", ctOrders), e(ctOrders, "PRODUCES", duck("orders")), colEdge(po, duck("orders"), ctOrders, ordersCols...),
			e(out, "CONSUMES", insOrders), e(insOrders, "PRODUCES", duck("orders")), colEdge(out, duck("orders"), insOrders, ordersCols...),
			colEdge(po, out, copyJob, ordersCols...),
			e(duck("orders"), "CONSUMES", ctRevenue), e(duck("customers"), "CONSUMES", ctRevenue), e(duck("regions"), "CONSUMES", ctRevenue),
			e(ctRevenue, "PRODUCES", duck("revenue_by_region")), colEdge(duck("orders"), duck("revenue_by_region"), ctRevenue, "total => revenue (INDIRECT)"),
			e(duck("revenue_by_region"), "CONSUMES", ctTop), e(ctTop, "PRODUCES", duck("best_regions")),
			colEdge(duck("revenue_by_region"), duck("best_regions"), ctTop, directCols("region", "revenue")...),
		},
	}, h.lineageOf(duck("revenue_by_region"), 2, "both"))

	// Issue 377: the parquet file's schema is one columns entry the asset page can read.
	file := h.asset(out)
	assert.JSONEq(t, `[{"column_name":"id","data_type":"INTEGER"},{"column_name":"customer_id","data_type":"INTEGER"},{"column_name":"total","data_type":"DECIMAL(10,2)"},{"column_name":"created_at","data_type":"TIMESTAMP WITH TIME ZONE"}]`, file.Schema["columns"])
	assert.Len(t, file.Schema, 1)
	assert.EqualValues(t, 1, file.Metadata["output_row_count"])

	// The rename left the old name answering.
	resolved, err := h.assets.ResolveMRN(h.ctx, duck("top_regions"))
	require.NoError(t, err)
	assert.Equal(t, duck("best_regions"), resolved)
	assert.Equal(t, "RENAME", h.asset(duck("best_regions")).Metadata["lifecycle_state"])

	// Every run finished: duck_lineage sends START and COMPLETE for each statement.
	for _, j := range []string{copyJob, ctRevenue, insUniq} {
		history := h.runsOf(j)
		require.Len(t, history, 1, j)
		assert.Equal(t, "COMPLETE", history[0].Status, j)
	}
}

// Replaying a producer's events a second time, as a client that retries does, changes nothing.
func TestReplayingEventsTwiceChangesNothing(t *testing.T) {
	h := newHarness(t)
	h.replay("duck_lineage")
	assets, edges := h.allMRNs(), h.allEdges()
	events := h.query(`SELECT count(*) AS n FROM run_history`)[0]["n"]
	h.replay("duck_lineage")
	assert.Equal(t, assets, h.allMRNs())
	assert.Equal(t, edges, h.allEdges())
	assert.Equal(t, events, h.query(`SELECT count(*) AS n FROM run_history`)[0]["n"])
}

func dbtTable(schema, name string) string {
	return "mrn://asset/table/duckdb/shop." + schema + "." + name
}
func dbtModel(name string) string { return "mrn://asset/model/dbt/dbt-shop.shop.main.shop." + name }
func dbtTest(name string) string  { return "mrn://asset/test/dbt/dbt-shop." + name }

func cols(pairs ...string) []string {
	out := make([]string, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, pairs[i]+" => "+pairs[i+1])
	}
	return out
}

func TestReplayDbt(t *testing.T) {
	h := newHarness(t)
	h.replay("dbt")

	project := "mrn://asset/project/dbt/dbt-shop.dbt-run-shop"
	rawOrders, rawCustomers := dbtTable("raw", "orders"), dbtTable("main", "raw_customers")
	stgCustomers, stgOrders, customerOrders, broken := dbtTable("main", "stg_customers"), dbtTable("main", "stg_orders"), dbtTable("main", "customer_orders"), dbtTable("main", "broken_orders")
	mStgCustomers, mStgOrders, mCustomerOrders, mBroken := dbtModel("stg_customers"), dbtModel("stg_orders"), dbtModel("customer_orders"), dbtModel("broken_orders")
	tCustomerOrders, tStgOrders, tStgCustomers, tSource := dbtTest("shop.main.shop.customer_orders.test"), dbtTest("shop.main.shop.stg_orders.test"), dbtTest("shop.main.shop.stg_customers.test"), dbtTest("shop.raw.source.shop.raw.orders.test")

	wantAssets := []string{
		project,
		rawOrders, rawCustomers, stgCustomers, stgOrders, customerOrders, broken,
		mStgCustomers, mStgOrders, mCustomerOrders, mBroken,
		tCustomerOrders, tStgOrders, tStgCustomers, tSource,
	}
	wantEdges := []string{
		// The dbt project run contains each model it ran.
		e(project, "CONTAINS", mStgCustomers), e(project, "CONTAINS", mStgOrders), e(project, "CONTAINS", mCustomerOrders), e(project, "CONTAINS", mBroken),
		// stg_customers selects from the raw_customers seed. dbt-ol leaves the seed out of the model's inputs but names it in the column lineage.
		e(mStgCustomers, "PRODUCES", stgCustomers),
		colEdge(rawCustomers, stgCustomers, mStgCustomers, cols("country", "country", "id", "customer_id", "first_name,last_name", "customer_name")...),
		// stg_orders selects from the raw.orders source.
		e(rawOrders, "CONSUMES", mStgOrders), e(mStgOrders, "PRODUCES", stgOrders),
		colEdge(rawOrders, stgOrders, mStgOrders, cols("amount", "amount", "customer_id", "customer_id", "id", "order_id", "ordered_at", "ordered_at", "status", "status")...),
		// customer_orders joins the two staging models. Its schema facet lists only the documented columns, and the column lineage covers all five.
		e(stgCustomers, "CONSUMES", mCustomerOrders), e(stgOrders, "CONSUMES", mCustomerOrders), e(mCustomerOrders, "PRODUCES", customerOrders),
		colEdge(stgCustomers, customerOrders, mCustomerOrders, cols("country", "country", "customer_id", "customer_id", "customer_name", "customer_name")...),
		colEdge(stgOrders, customerOrders, mCustomerOrders, cols("amount", "lifetime_value", "order_id", "order_count")...),
		// broken_orders failed to build; its START still named its input, output and columns.
		e(stgOrders, "CONSUMES", mBroken), e(mBroken, "PRODUCES", broken),
		colEdge(stgOrders, broken, mBroken, cols("no_such_column", "no_such_column", "order_id", "order_id")...),
		// Each data test reads the dataset it checks.
		e(customerOrders, "CONSUMES", tCustomerOrders), e(stgOrders, "CONSUMES", tStgOrders), e(stgCustomers, "CONSUMES", tStgCustomers), e(rawOrders, "CONSUMES", tSource),
	}
	requireGraph(t, graph{Nodes: wantAssets, Edges: wantEdges}, graph{Nodes: h.allMRNs(), Edges: h.allEdges()})

	// The project ran four times: seed, run, test, and the failing run.
	statuses := map[string]int{}
	for _, r := range h.runsOf(project) {
		statuses[r.Status]++
	}
	assert.Equal(t, map[string]int{"COMPLETE": 3, "FAIL": 1}, statuses)
	brokenRuns := h.runsOf(mBroken)
	require.Len(t, brokenRuns, 1)
	assert.Equal(t, "FAIL", brokenRuns[0].Status)
	assert.Equal(t, "Test", h.asset(tSource).Type)
	assert.Equal(t, "Model", h.asset(mStgOrders).Type)
	assert.Equal(t, "stg_orders", *h.asset(mStgOrders).Name)
}

func sparkJob(name string) string { return "mrn://asset/job/spark/spark-shop.shop_orders." + name }

func TestReplaySpark(t *testing.T) {
	h := newHarness(t)
	h.replay("spark")

	rawOrders, customersCSV := "mrn://asset/dataset/file/-data-raw-orders", "mrn://asset/file/file/-data-input-customers.csv"
	customerOrders, revenue, negative := "mrn://asset/dataset/file/-data-marts-customer_orders", "mrn://asset/dataset/file/-data-warehouse-shop.db-revenue_by_country", "mrn://asset/dataset/file/-data-marts-negative_orders"
	app := "mrn://asset/job/spark/spark-shop.shop_orders"
	writeRaw, schemaScan, join := sparkJob("execute_insert_into_hadoop_fs_relation_command.raw_orders"), sparkJob("map_partitions_parallel_collection"), sparkJob("adaptive_spark_plan.marts_customer_orders")
	drop, ctas, ctasPlan, writeNegative := sparkJob("drop_table"), sparkJob("execute_create_data_source_table_as_select_command.shop_revenue_by_country"), sparkJob("adaptive_spark_plan.shop_db_revenue_by_country"), sparkJob("execute_insert_into_hadoop_fs_relation_command.marts_negative_orders")

	wantAssets := []string{rawOrders, customersCSV, customerOrders, revenue, negative, app, writeRaw, schemaScan, join, drop, ctas, ctasPlan, writeNegative}
	wantEdges := []string{
		// df.write.parquet('/data/raw/orders')
		e(writeRaw, "PRODUCES", rawOrders),
		// Spark's schema-inference jobs read a dataset and are never finished.
		e(rawOrders, "CONSUMES", schemaScan), e(customerOrders, "CONSUMES", schemaScan),
		// The join of orders and customers.csv into marts/customer_orders.
		e(rawOrders, "CONSUMES", join), e(customersCSV, "CONSUMES", join), e(join, "PRODUCES", customerOrders),
		colEdge(customersCSV, customerOrders, join, "country => country (IDENTITY)", "customer_id => customer_id (IDENTITY)", "customer_name => customer_name (IDENTITY)"),
		colEdge(rawOrders, customerOrders, join, "amount => lifetime_value (AGGREGATION, CONDITIONAL)", "order_id => order_count (AGGREGATION (masked))"), // Spark marks a count as masking its input.
		// DROP TABLE IF EXISTS of a table that did not exist yet files nothing. The CTAS is reported twice, under the command and under its plan; the direct edge keeps the job that declared it first.
		e(customerOrders, "CONSUMES", ctas), e(ctas, "PRODUCES", revenue),
		e(customerOrders, "CONSUMES", ctasPlan), e(ctasPlan, "PRODUCES", revenue),
		colEdge(customerOrders, revenue, ctas, "country => country (IDENTITY)", "lifetime_value => revenue (AGGREGATION)", "order_count => orders (AGGREGATION)"),
		// The write that failed its assertion.
		e(rawOrders, "CONSUMES", writeNegative), e(writeNegative, "PRODUCES", negative),
		colEdge(rawOrders, negative, writeNegative, "amount => amount (IDENTITY)", "amount => check (CONDITIONAL)", "customer_id => customer_id (IDENTITY)", "order_id => order_id (IDENTITY)", "ordered_at => ordered_at (IDENTITY)", "status => status (IDENTITY)"),
	}
	requireGraph(t, graph{Nodes: wantAssets, Edges: wantEdges}, graph{Nodes: h.allMRNs(), Edges: h.allEdges()})

	// The CTAS symlink names the Spark catalogue table, which leads to the same asset.
	resolved, err := h.assets.ResolveMRN(h.ctx, "mrn://asset/dataset/openlineage/shop.revenue_by_country")
	require.NoError(t, err)
	assert.Equal(t, revenue, resolved)

	// Spark sends FAIL and then COMPLETE for the failed write; it stays failed.
	failed := h.runsOf(writeNegative)
	require.Len(t, failed, 1)
	assert.Equal(t, "FAIL", failed[0].Status)
	scans := h.runsOf(schemaScan)
	require.Len(t, scans, 2)
	for _, r := range scans {
		assert.Equal(t, "RUNNING", r.Status)
	}
	assert.Equal(t, "COMPLETE", h.runsOf(app)[0].Status)
}
