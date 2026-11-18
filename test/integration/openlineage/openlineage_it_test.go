//go:build openlineage_it

// Package openlineage runs real OpenLineage producers, DuckDB's duck_lineage extension, dbt-ol and openlineage-spark, against a running Marmot and checks the lineage Marmot builds through its API against lineage written out by hand. Run it with make openlineage-it.
package openlineage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type env struct {
	url, key, marmotBin, sourcePG, pgHost, pgPort string
}

// loadEnv reads the stack's env file, letting the process environment override it.
func loadEnv(t *testing.T) env {
	t.Helper()
	vals := map[string]string{}
	if f, err := os.Open(".stack/env"); err == nil {
		s := bufio.NewScanner(f)
		for s.Scan() {
			if k, v, ok := strings.Cut(s.Text(), "="); ok {
				vals[k] = v
			}
		}
		_ = f.Close()
	}
	get := func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return vals[k]
	}
	e := env{url: get("MARMOT_URL"), key: get("MARMOT_API_KEY"), marmotBin: get("MARMOT_BIN"), sourcePG: get("SOURCE_PG"), pgHost: get("SOURCE_PG_HOST"), pgPort: get("SOURCE_PG_PORT")}
	if e.url == "" || e.key == "" {
		t.Skip("run stack.sh up first, or set MARMOT_URL and MARMOT_API_KEY")
	}
	return e
}

func (e env) get(t *testing.T, path string, out any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.url+path, nil)
	require.NoError(t, err)
	req.Header.Set("X-API-Key", e.key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && out != nil {
		require.NoError(t, json.Unmarshal(body, out), "%s", body)
	}
	return resp.StatusCode
}

// run executes a producer's run.sh with the Marmot endpoint in its environment.
func (e env) run(t *testing.T, script string, extra ...string) {
	t.Helper()
	cmd := exec.Command(script)
	cmd.Env = append(os.Environ(), "OPENLINEAGE_URL="+e.url, "OPENLINEAGE_API_KEY="+e.key, "SOURCE_PG="+e.sourcePG)
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s failed:\n%s", script, out)
}

// ingest runs a plugin through the CLI, as a scheduled pipeline would.
func (e env) ingest(t *testing.T, name, config string) {
	t.Helper()
	require.NotEmpty(t, e.marmotBin, "MARMOT_BIN is needed to run plugins")
	dir := t.TempDir()
	path := filepath.Join(dir, "pipeline.yaml")
	require.NoError(t, os.WriteFile(path, []byte(config), 0o600))
	cmd := exec.Command(e.marmotBin, "--host", e.url, "ingest", "-c", path)
	// An empty home keeps the CLI from reading a saved context that points somewhere else.
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CONFIG_HOME="+dir, "MARMOT_HOST="+e.url, "MARMOT_API_KEY="+e.key, "MARMOT_PLUGINS_DIR="+filepath.Join(filepath.Dir(e.marmotBin), "plugins"), "MARMOT_PLUGINS_AUTOINSTALL=false")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "ingest %s failed:\n%s", name, out)
}

type assetRef struct {
	ID     string `json:"id"`
	MRN    string `json:"mrn"`
	IsStub bool   `json:"is_stub"`
}

type lineageResp struct {
	Nodes []struct {
		Asset assetRef `json:"asset"`
	} `json:"nodes"`
	Edges []struct {
		Source        string `json:"source"`
		Target        string `json:"target"`
		Type          string `json:"type"`
		JobMRN        string `json:"job_mrn"`
		ColumnLineage []struct {
			From      []string `json:"from_columns"`
			To        string   `json:"to_column"`
			Transform string   `json:"transform"`
		} `json:"column_lineage"`
	} `json:"edges"`
}

func (e env) resolve(t *testing.T, m string) (assetRef, bool) {
	t.Helper()
	var r assetRef
	code := e.get(t, "/api/v1/mrn/resolve?mrn="+url.QueryEscape(m), &r)
	return r, code == http.StatusOK
}

// openLineageGraph gathers what OpenLineage events put in the catalog around a set of jobs: each job's neighbours, each of those datasets' neighbours, and every edge among them, which takes in column edges from datasets no job lists as an input. Plugin edges, which name no job and are neither CONSUMES nor PRODUCES, are left out so the comparison is about OpenLineage alone.
func (e env) openLineageGraph(t *testing.T, jobs []string) (nodes map[string]assetRef, edges []string) {
	t.Helper()
	nodes = map[string]assetRef{}
	seen := map[string]bool{}
	var queue []string
	jobSet := map[string]bool{}
	for _, j := range jobs {
		ref, ok := e.resolve(t, j)
		require.True(t, ok, "job %s is not in the catalog", j)
		queue = append(queue, ref.ID)
		jobSet[ref.ID] = true
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		var resp lineageResp
		require.Equal(t, http.StatusOK, e.get(t, "/api/v1/lineage/assets/"+id+"?limit=1&direction=both", &resp))
		for _, n := range resp.Nodes {
			nodes[n.Asset.MRN] = n.Asset
			if jobSet[id] && !visited[n.Asset.ID] {
				queue = append(queue, n.Asset.ID)
			}
		}
		for _, ed := range resp.Edges {
			if ed.JobMRN == "" && ed.Type != "CONSUMES" && ed.Type != "PRODUCES" {
				continue
			}
			s := fmt.Sprintf("%s -[%s]-> %s", ed.Source, ed.Type, ed.Target)
			if ed.JobMRN != "" {
				s += " via " + ed.JobMRN
			}
			var cols []string
			for _, c := range ed.ColumnLineage {
				from := append([]string(nil), c.From...)
				sort.Strings(from)
				c := strings.Join(from, ",") + " => " + c.To
				cols = append(cols, c)
			}
			sort.Strings(cols)
			if len(cols) > 0 {
				s += " {" + strings.Join(cols, "; ") + "}"
			}
			if !seen[s] {
				seen[s] = true
				edges = append(edges, s)
			}
		}
	}
	sort.Strings(edges)
	return nodes, edges
}

func edge(source, typ, target string) string {
	return fmt.Sprintf("%s -[%s]-> %s", source, typ, target)
}

func colEdge(source, target, job string, columns ...string) string {
	sort.Strings(columns)
	return fmt.Sprintf("%s -[DIRECT]-> %s via %s {%s}", source, target, job, strings.Join(columns, "; "))
}

func same(cols ...string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c + " => " + c
	}
	return out
}

func requireEdges(t *testing.T, want, got []string) {
	t.Helper()
	sort.Strings(want)
	missing, extra := diff(want, got), diff(got, want)
	require.Empty(t, missing, "edges missing from Marmot")
	require.Empty(t, extra, "edges Marmot has that the events do not describe")
}

func diff(a, b []string) []string {
	in := map[string]bool{}
	for _, s := range b {
		in[s] = true
	}
	var out []string
	for _, s := range a {
		if !in[s] {
			out = append(out, s)
		}
	}
	return out
}

func duckJob(name string) string { return "mrn://asset/job/duckdb/duckdb." + name }

// TestDuckLineageWithPlugins is the issue 377 flow: the Postgres plugin catalogues the source, DuckDB copies and transforms it with duck_lineage sending events, then the DuckDB plugin catalogues the DuckDB file. Every table must be one asset, the plugins' own, with OpenLineage's lineage on it and no merging by hand.
func TestDuckLineageWithPlugins(t *testing.T) {
	e := loadEnv(t)
	e.ingest(t, "postgres", fmt.Sprintf("name: it-postgres\nruns:\n  - postgresql:\n      host: %s\n      port: %s\n      user: shop\n      password: shop\n", e.pgHost, e.pgPort))
	e.run(t, "./duck_lineage/run.sh")
	dbFile, err := filepath.Abs("duck_lineage/work/shopdb.duckdb")
	require.NoError(t, err)
	// DuckDB lets one process at a time open a database file; give duck_lineage's session time to close.
	time.Sleep(time.Second)
	e.ingest(t, "duckdb", fmt.Sprintf("name: it-duckdb\nruns:\n  - duckdb:\n      path: %s\n", dbFile))

	// The DuckDB plugin files a local database file under a zone named after the machine it runs on, and adopts the stubs OpenLineage made under the unzoned name, so each DuckDB table is compared under the MRN its name now leads to.
	duckTable := func(name string) string {
		m := "mrn://asset/table/duckdb/shopdb.main." + name
		if ref, ok := e.resolve(t, m); ok {
			return ref.MRN
		}
		return m
	}
	view := "mrn://asset/view/duckdb/shopdb.main.eu_orders"
	if ref, ok := e.resolve(t, view); ok {
		view = ref.MRN
	}
	po, pc := "mrn://asset/table/postgresql/shop.public.orders", "mrn://asset/table/postgresql/shop.public.customers"
	out, csv := "mrn://asset/file/file/out.parquet", "mrn://asset/file/file/regions.csv"
	copyJob, ctOrders, ctCustomers := duckJob("copy_orders_8aff966c"), duckJob("create_table_orders_orders_80b43dc7"), duckJob("create_table_customers_customers_4a7e8f3f")
	ctRegions, ctRevenue, insOrders := duckJob("create_table_regions_bcdfbfd7"), duckJob("create_table_revenue_by_region_orders_customers_dd011b2b"), duckJob("insert_orders_7bdd2724")
	ctTop, alter, ctView := duckJob("create_table_top_regions_revenue_by_region_07987a7e"), duckJob("alter_70dd2edf"), duckJob("create_view_bf84bdbb")
	ctUniq, insUniq := duckJob("create_table_uniq_e002db6f"), duckJob("insert_uniq_orders_orders_1da94a62")
	jobs := []string{copyJob, ctOrders, ctCustomers, ctRegions, ctRevenue, insOrders, ctTop, alter, ctView, ctUniq, insUniq}

	nodes, edges := e.openLineageGraph(t, jobs)
	ordersCols := same("created_at", "customer_id", "id", "total")
	requireEdges(t, []string{
		edge(po, "CONSUMES", copyJob), edge(copyJob, "PRODUCES", out), colEdge(po, out, copyJob, ordersCols...),
		edge(po, "CONSUMES", ctOrders), edge(ctOrders, "PRODUCES", duckTable("orders")), colEdge(po, duckTable("orders"), ctOrders, ordersCols...),
		edge(pc, "CONSUMES", ctCustomers), edge(ctCustomers, "PRODUCES", duckTable("customers")), colEdge(pc, duckTable("customers"), ctCustomers, same("id", "name", "region")...),
		edge(csv, "CONSUMES", ctRegions), edge(ctRegions, "PRODUCES", duckTable("regions")), colEdge(csv, duckTable("regions"), ctRegions, same("id", "label", "region")...),
		edge(duckTable("orders"), "CONSUMES", ctRevenue), edge(duckTable("customers"), "CONSUMES", ctRevenue), edge(duckTable("regions"), "CONSUMES", ctRevenue),
		edge(ctRevenue, "PRODUCES", duckTable("revenue_by_region")), colEdge(duckTable("orders"), duckTable("revenue_by_region"), ctRevenue, "total => revenue"),
		edge(out, "CONSUMES", insOrders), edge(insOrders, "PRODUCES", duckTable("orders")), colEdge(out, duckTable("orders"), insOrders, ordersCols...),
		edge(duckTable("revenue_by_region"), "CONSUMES", ctTop), edge(ctTop, "PRODUCES", duckTable("best_regions")),
		colEdge(duckTable("revenue_by_region"), duckTable("best_regions"), ctTop, same("region", "revenue")...),
		edge(alter, "PRODUCES", duckTable("best_regions")),
		edge(duckTable("orders"), "CONSUMES", ctView), edge(ctView, "PRODUCES", view),
		edge(ctUniq, "PRODUCES", duckTable("uniq")),
		edge(duckTable("orders"), "CONSUMES", insUniq), edge(insUniq, "PRODUCES", duckTable("uniq")), colEdge(duckTable("orders"), duckTable("uniq"), insUniq, same("id")...),
	}, edges)

	// The plugins' assets are the ones lineage lands on, so nothing is a stub except the two files no plugin catalogues.
	for m, a := range nodes {
		wantStub := m == out || m == csv
		require.Equal(t, wantStub, a.IsStub, "%s stub", m)
	}
	_, ok := e.resolve(t, "mrn://asset/table/duckdb/shopdb.main.scratch")
	require.False(t, ok, "scratch was dropped")
	moved, ok := e.resolve(t, "mrn://asset/table/duckdb/shopdb.main.top_regions")
	require.True(t, ok)
	require.Equal(t, duckTable("best_regions"), moved.MRN, "the renamed table answers to its old name")
}

func dbtTable(schema, name string) string {
	return "mrn://asset/table/duckdb/shop." + schema + "." + name
}
func dbtModel(name string) string { return "mrn://asset/model/dbt/dbt-shop.shop.main.shop." + name }

// TestDbt runs dbt-ol and checks Marmot's lineage against dbt's own DAG in manifest.json.
func TestDbt(t *testing.T) {
	e := loadEnv(t)
	e.run(t, "./dbt/run.sh", "DBT_OL_FAIL=1")

	raw, err := os.ReadFile("dbt/target/manifest.json")
	require.NoError(t, err)
	var manifest struct {
		Nodes map[string]struct {
			ResourceType string `json:"resource_type"`
			Database     string `json:"database"`
			Schema       string `json:"schema"`
			Alias        string `json:"alias"`
			Name         string `json:"name"`
			DependsOn    struct {
				Nodes []string `json:"nodes"`
			} `json:"depends_on"`
		} `json:"nodes"`
		Sources map[string]struct {
			Database   string `json:"database"`
			Schema     string `json:"schema"`
			Identifier string `json:"identifier"`
		} `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(raw, &manifest))
	relation := func(id string) string {
		if n, ok := manifest.Nodes[id]; ok {
			return dbtTable(n.Schema, n.Alias)
		}
		s := manifest.Sources[id]
		return dbtTable(s.Schema, s.Identifier)
	}

	var models []string
	for id, n := range manifest.Nodes {
		if n.ResourceType == "model" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	var jobs []string
	for _, id := range models {
		jobs = append(jobs, dbtModel(manifest.Nodes[id].Name))
	}
	_, edges := e.openLineageGraph(t, jobs)

	// Every dependency dbt records for a model is upstream of the model's table in Marmot, through the model job or a column-level edge, and nothing else is.
	for _, id := range models {
		n := manifest.Nodes[id]
		table, job := dbtTable(n.Schema, n.Alias), dbtModel(n.Name)
		var want []string
		for _, dep := range n.DependsOn.Nodes {
			want = append(want, relation(dep))
		}
		got := map[string]bool{}
		for _, s := range edges {
			if strings.HasSuffix(s, " -[CONSUMES]-> "+job) {
				got[strings.SplitN(s, " ", 2)[0]] = true
			}
			if strings.Contains(s, " -[DIRECT]-> "+table+" via "+job) {
				got[strings.SplitN(s, " ", 2)[0]] = true
			}
		}
		var have []string
		for k := range got {
			have = append(have, k)
		}
		sort.Strings(want)
		sort.Strings(have)
		require.Equal(t, want, have, "upstream of %s", n.Name)
		require.Contains(t, edges, edge(job, "PRODUCES", table))
	}
	project, ok := e.resolve(t, "mrn://asset/project/dbt/dbt-shop.dbt-run-shop")
	require.True(t, ok)
	var resp lineageResp
	require.Equal(t, http.StatusOK, e.get(t, "/api/v1/lineage/assets/"+project.ID+"?limit=1&direction=downstream", &resp))
	contained := map[string]bool{}
	for _, ed := range resp.Edges {
		if ed.Type == "CONTAINS" && ed.Source == "mrn://asset/project/dbt/dbt-shop.dbt-run-shop" {
			contained[ed.Target] = true
		}
	}
	for _, j := range jobs {
		require.True(t, contained[j], "the project contains %s", j)
	}
}

// TestSpark runs a PySpark job with openlineage-spark and checks the files it read and wrote and their column lineage.
func TestSpark(t *testing.T) {
	e := loadEnv(t)
	e.run(t, "./spark/run.sh")
	job := func(name string) string { return "mrn://asset/job/spark/spark-shop.shop_orders." + name }
	raw, customers := "mrn://asset/dataset/file/-data-raw-orders", "mrn://asset/file/file/-data-input-customers.csv"
	customerOrders, revenue, negative := "mrn://asset/dataset/file/-data-marts-customer_orders", "mrn://asset/dataset/file/-data-warehouse-shop.db-revenue_by_country", "mrn://asset/dataset/file/-data-marts-negative_orders"
	writeRaw, join, ctas, ctasPlan, writeNegative := job("execute_insert_into_hadoop_fs_relation_command.raw_orders"), job("adaptive_spark_plan.marts_customer_orders"), job("execute_create_data_source_table_as_select_command.shop_revenue_by_country"), job("adaptive_spark_plan.shop_db_revenue_by_country"), job("execute_insert_into_hadoop_fs_relation_command.marts_negative_orders")

	_, edges := e.openLineageGraph(t, []string{writeRaw, join, ctas, ctasPlan, writeNegative})
	requireEdges(t, []string{
		edge(writeRaw, "PRODUCES", raw),
		edge(raw, "CONSUMES", join), edge(customers, "CONSUMES", join), edge(join, "PRODUCES", customerOrders),
		colEdge(customers, customerOrders, join, same("country", "customer_id", "customer_name")...),
		colEdge(raw, customerOrders, join, "amount => lifetime_value", "order_id => order_count"),
		edge(customerOrders, "CONSUMES", ctas), edge(ctas, "PRODUCES", revenue), edge(customerOrders, "CONSUMES", ctasPlan), edge(ctasPlan, "PRODUCES", revenue),
		colEdge(customerOrders, revenue, ctas, "country => country", "lifetime_value => revenue", "order_count => orders"),
		edge(raw, "CONSUMES", writeNegative), edge(writeNegative, "PRODUCES", negative),
		colEdge(raw, negative, writeNegative, "amount => amount", "amount => check", "customer_id => customer_id", "order_id => order_id", "ordered_at => ordered_at", "status => status"),
		// Spark's schema-inference job reads both datasets and never finishes.
		edge(raw, "CONSUMES", job("map_partitions_parallel_collection")), edge(customerOrders, "CONSUMES", job("map_partitions_parallel_collection")),
	}, edges)

	ref, ok := e.resolve(t, writeNegative)
	require.True(t, ok)
	var page struct {
		RunHistory []struct {
			Status string `json:"status"`
		} `json:"run_history"`
	}
	require.Equal(t, http.StatusOK, e.get(t, "/api/v1/assets/run-history/"+ref.ID, &page))
	history := page.RunHistory
	require.NotEmpty(t, history)
	require.Equal(t, "FAIL", history[0].Status, "a run Spark reports as FAIL and then COMPLETE stays failed")
}
