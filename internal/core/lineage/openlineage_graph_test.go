package lineage_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// A, B, C and D are Postgres tables; j1, j2 and j3 are jobs in namespace shop. Every expected graph below is written out by hand from what the events say.

func run(h *olHarness, runID, job string, inputs, outputs []ds) {
	h.sendJSON(runEvent("START", runID, job, inputs, outputs))
	h.sendJSON(runEvent("COMPLETE", runID, job, inputs, outputs))
}

func TestLineageSingleHop(t *testing.T) {
	h := newHarness(t)
	run(h, "r1", "j1", []ds{pgTable("a")}, []ds{pgTable("b")})

	a, b, j1 := pgMRN("a"), pgMRN("b"), jobMRN("j1")
	want := graph{
		Nodes: []string{a, b, j1},
		Edges: []string{e(a, "CONSUMES", j1), e(j1, "PRODUCES", b)},
	}
	requireGraph(t, want, h.lineageOf(b, 10, "both"))
	requireGraph(t, want, h.lineageOf(a, 10, "both"))
	requireGraph(t, graph{Nodes: []string{a, b, j1}, Edges: want.Edges}, h.lineageOf(j1, 10, "both"))
	assert.Equal(t, want.Edges, h.allEdges())
}

func TestLineageMultiHopAtEachDepth(t *testing.T) {
	h := newHarness(t)
	run(h, "r1", "j1", []ds{pgTable("a")}, []ds{pgTable("b")})
	run(h, "r2", "j2", []ds{pgTable("b")}, []ds{pgTable("c")})

	a, b, c, j1, j2 := pgMRN("a"), pgMRN("b"), pgMRN("c"), jobMRN("j1"), jobMRN("j2")
	aj1, j1b, bj2, j2c := e(a, "CONSUMES", j1), e(j1, "PRODUCES", b), e(b, "CONSUMES", j2), e(j2, "PRODUCES", c)

	upstream := []graph{
		{Nodes: []string{c, j2}, Edges: []string{j2c}},
		{Nodes: []string{c, j2, b}, Edges: []string{j2c, bj2}},
		{Nodes: []string{c, j2, b, j1}, Edges: []string{j2c, bj2, j1b}},
		{Nodes: []string{c, j2, b, j1, a}, Edges: []string{j2c, bj2, j1b, aj1}},
		{Nodes: []string{c, j2, b, j1, a}, Edges: []string{j2c, bj2, j1b, aj1}},
	}
	for depth, want := range upstream {
		t.Run(fmt.Sprintf("upstream of C at depth %d", depth+1), func(t *testing.T) {
			requireGraph(t, want, h.lineageOf(c, depth+1, "upstream"))
		})
	}
	downstream := []graph{
		{Nodes: []string{a, j1}, Edges: []string{aj1}},
		{Nodes: []string{a, j1, b}, Edges: []string{aj1, j1b}},
		{Nodes: []string{a, j1, b, j2}, Edges: []string{aj1, j1b, bj2}},
		{Nodes: []string{a, j1, b, j2, c}, Edges: []string{aj1, j1b, bj2, j2c}},
	}
	for depth, want := range downstream {
		t.Run(fmt.Sprintf("downstream of A at depth %d", depth+1), func(t *testing.T) {
			requireGraph(t, want, h.lineageOf(a, depth+1, "downstream"))
		})
	}
	t.Run("both ways from the middle", func(t *testing.T) {
		requireGraph(t, graph{Nodes: []string{a, b, c, j1, j2}, Edges: []string{aj1, j1b, bj2, j2c}}, h.lineageOf(b, 10, "both"))
	})
}

func TestLineageFanInFanOutAndDiamond(t *testing.T) {
	t.Run("fan-in", func(t *testing.T) {
		h := newHarness(t)
		run(h, "r1", "j1", []ds{pgTable("a"), pgTable("b")}, []ds{pgTable("c")})
		a, b, c, j1 := pgMRN("a"), pgMRN("b"), pgMRN("c"), jobMRN("j1")
		requireGraph(t, graph{
			Nodes: []string{a, b, c, j1},
			Edges: []string{e(a, "CONSUMES", j1), e(b, "CONSUMES", j1), e(j1, "PRODUCES", c)},
		}, h.lineageOf(c, 10, "upstream"))
	})
	t.Run("fan-out", func(t *testing.T) {
		h := newHarness(t)
		run(h, "r1", "j1", []ds{pgTable("a")}, []ds{pgTable("b"), pgTable("c")})
		a, b, c, j1 := pgMRN("a"), pgMRN("b"), pgMRN("c"), jobMRN("j1")
		requireGraph(t, graph{
			Nodes: []string{a, b, c, j1},
			Edges: []string{e(a, "CONSUMES", j1), e(j1, "PRODUCES", b), e(j1, "PRODUCES", c)},
		}, h.lineageOf(a, 10, "downstream"))
		// Upstream of one branch does not include the other.
		requireGraph(t, graph{
			Nodes: []string{a, b, j1},
			Edges: []string{e(a, "CONSUMES", j1), e(j1, "PRODUCES", b)},
		}, h.lineageOf(b, 10, "upstream"))
	})
	t.Run("diamond", func(t *testing.T) {
		h := newHarness(t)
		run(h, "r1", "j1", []ds{pgTable("a")}, []ds{pgTable("b")})
		run(h, "r2", "j2", []ds{pgTable("a")}, []ds{pgTable("c")})
		run(h, "r3", "j3", []ds{pgTable("b"), pgTable("c")}, []ds{pgTable("d")})
		a, b, c, d := pgMRN("a"), pgMRN("b"), pgMRN("c"), pgMRN("d")
		j1, j2, j3 := jobMRN("j1"), jobMRN("j2"), jobMRN("j3")
		all := []string{
			e(a, "CONSUMES", j1), e(j1, "PRODUCES", b),
			e(a, "CONSUMES", j2), e(j2, "PRODUCES", c),
			e(b, "CONSUMES", j3), e(c, "CONSUMES", j3), e(j3, "PRODUCES", d),
		}
		requireGraph(t, graph{Nodes: []string{a, b, c, d, j1, j2, j3}, Edges: all}, h.lineageOf(d, 10, "upstream"))
		requireGraph(t, graph{Nodes: []string{a, b, c, d, j1, j2, j3}, Edges: all}, h.lineageOf(a, 10, "downstream"))
		assert.Equal(t, sorted(all), h.allEdges())
	})
}

func TestLineageJobThatReadsAndWritesOneTable(t *testing.T) {
	h := newHarness(t)
	cols := facet("https://openlineage.io/spec/facets/1-2-0/ColumnLineageDatasetFacet.json", map[string]any{
		"fields": map[string]any{"total": map[string]any{"inputFields": []any{map[string]any{"namespace": pgNS, "name": "shop.public.a", "field": "total"}}}},
	})
	run(h, "r1", "dedupe", []ds{pgTable("a")}, []ds{withFacets(pgTable("a"), map[string]any{"columnLineage": cols})})

	a, j := pgMRN("a"), jobMRN("dedupe")
	want := graph{Nodes: []string{a, j}, Edges: []string{e(a, "CONSUMES", j), e(j, "PRODUCES", a)}}
	requireGraph(t, want, h.lineageOf(a, 10, "both"))
	assert.Equal(t, want.Edges, h.allEdges(), "no self edge for the column mappings")
}

func TestLineageRerunAddsNothing(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		run(h, fmt.Sprintf("r%d", i), "j1", []ds{pgTable("a")}, []ds{pgTable("b")})
	}
	a, b, j1 := pgMRN("a"), pgMRN("b"), jobMRN("j1")
	assert.Equal(t, sorted([]string{e(a, "CONSUMES", j1), e(j1, "PRODUCES", b)}), h.allEdges())
	assert.Len(t, h.allMRNs(), 3)
	runsOf := h.query(`SELECT count(DISTINCT run_id) AS runs, count(*) AS events FROM run_history`)
	assert.EqualValues(t, 3, runsOf[0]["runs"])
	assert.EqualValues(t, 6, runsOf[0]["events"])
}

// Edges accumulate: OpenLineage events describe one run at a time and never say that an earlier input stopped being one, so an input a job no longer reads keeps its edge.
func TestLineageJobWhoseInputsChange(t *testing.T) {
	h := newHarness(t)
	run(h, "r1", "j1", []ds{pgTable("a"), pgTable("b")}, []ds{pgTable("c")})
	run(h, "r2", "j1", []ds{pgTable("a"), pgTable("d")}, []ds{pgTable("c")})

	a, b, c, d, j1 := pgMRN("a"), pgMRN("b"), pgMRN("c"), pgMRN("d"), jobMRN("j1")
	requireGraph(t, graph{
		Nodes: []string{a, b, c, d, j1},
		Edges: []string{e(a, "CONSUMES", j1), e(b, "CONSUMES", j1), e(d, "CONSUMES", j1), e(j1, "PRODUCES", c)},
	}, h.lineageOf(c, 10, "upstream"))
}

func TestLineageMergesWithPluginLineage(t *testing.T) {
	h := newHarness(t)
	table := func(name string) runs.CreateAssetInput {
		m := mrn.Asset("Table", "PostgreSQL", "shop", "public", name).String()
		return runs.CreateAssetInput{Name: name, MRN: &m, Type: "Table", Providers: []string{"PostgreSQL"}, Schema: map[string]interface{}{"columns": `[{"column_name":"id","data_type":"integer"}]`}}
	}
	// The Postgres plugin knows orders and the view over it.
	h.pluginRun("shop-pg", "postgresql", []runs.CreateAssetInput{table("orders"), table("orders_view")},
		[]runs.LineageInput{{Source: pgMRN("orders"), Target: pgMRN("orders_view"), Type: "DIRECT"}})

	// A job outside Postgres reads the view and writes a summary.
	run(h, "r1", "summarise", []ds{pgTable("orders_view")}, []ds{pgTable("summary")})

	orders, view, summary, j := pgMRN("orders"), pgMRN("orders_view"), pgMRN("summary"), jobMRN("summarise")
	requireGraph(t, graph{
		Nodes: []string{orders, view, summary, j},
		Edges: []string{e(orders, "DIRECT", view), e(view, "CONSUMES", j), e(j, "PRODUCES", summary)},
	}, h.lineageOf(summary, 10, "upstream"))

	v := h.asset(view)
	assert.False(t, v.IsStub, "the plugin's asset stays a real asset")
	assert.Equal(t, `[{"column_name":"id","data_type":"integer"}]`, v.Schema["columns"], "the plugin's schema wins over anything OpenLineage says")
	assert.True(t, h.asset(summary).IsStub, "a table only OpenLineage knows is a stub")
	assert.Len(t, h.allMRNs(), 4, "no duplicate assets: %v", h.allMRNs())
}

// When OpenLineage mentions a table before its plugin has run, the stub it made becomes the plugin's asset, with the same id, and stops being a stub.
func TestPluginTakesOverAnOpenLineageStub(t *testing.T) {
	h := newHarness(t)
	run(h, "r1", "load", []ds{pgTable("raw")}, []ds{pgTable("orders")})
	stub := h.asset(pgMRN("orders"))
	require.True(t, stub.IsStub)

	m := mrn.Asset("Table", "PostgreSQL", "shop", "public", "orders").String()
	h.pluginRun("shop-pg", "postgresql", []runs.CreateAssetInput{{Name: "orders", MRN: &m, Type: "Table", Providers: []string{"PostgreSQL"}}}, nil)

	after := h.asset(pgMRN("orders"))
	assert.Equal(t, stub.ID, after.ID)
	assert.False(t, after.IsStub, "an asset its own plugin catalogues must show up in search")
	assert.Len(t, h.allMRNs(), 3)
}

func TestLineageFromTwoProducersIsOneAsset(t *testing.T) {
	h := newHarness(t)
	// duck_lineage names the table through the attached database; another producer names it directly.
	duck := map[string]any{
		"eventType": "START", "eventTime": "2026-11-18T10:00:00Z", "producer": "https://github.com/ilum-cloud/duck_lineage", "schemaURL": runURL,
		"run": map[string]any{"runId": "01a1f0c2-57c0-a827-8b82-cf4a604fa5f7"},
		"job": map[string]any{"namespace": "duckdb", "name": "COPY_orders_8aff966c"},
		"inputs": []any{map[string]any{"namespace": "duckdb", "name": "pg.public.orders", "facets": map[string]any{
			"catalog":    facet("https://openlineage.io/spec/facets/1-0-0/CatalogDatasetFacet.json", map[string]any{"framework": "duckdb", "name": "pg", "type": "postgres", "metadataUri": "postgres://pg:5432/shop"}),
			"dataSource": facet("https://openlineage.io/spec/facets/1-0-0/DatasourceDatasetFacet.json", map[string]any{"name": "postgres://pg:5432/shop", "uri": "postgres://pg:5432/shop"}),
		}}},
		"outputs": []any{map[string]any{"namespace": "file", "name": "out.parquet"}},
	}
	h.sendJSON(duck)
	run(h, "r2", "report", []ds{pgTable("orders")}, []ds{pgTable("report")})

	orders := pgMRN("orders")
	h.asset(orders)
	n := h.query(`SELECT count(*) AS n FROM assets WHERE type = 'Table' AND name = 'orders'`)
	assert.EqualValues(t, 1, n[0]["n"], "one orders asset: %v", h.allMRNs())
	requireGraph(t, graph{
		Nodes: []string{orders, "mrn://asset/file/file/out.parquet", "mrn://asset/job/duckdb/duckdb.copy_orders_8aff966c", pgMRN("report"), jobMRN("report")},
		Edges: []string{
			e(orders, "CONSUMES", "mrn://asset/job/duckdb/duckdb.copy_orders_8aff966c"),
			e("mrn://asset/job/duckdb/duckdb.copy_orders_8aff966c", "PRODUCES", "mrn://asset/file/file/out.parquet"),
			e(orders, "CONSUMES", jobMRN("report")),
			e(jobMRN("report"), "PRODUCES", pgMRN("report")),
		},
	}, h.lineageOf(orders, 10, "downstream"))
}

func TestColumnLineageLandsOnTheRightColumns(t *testing.T) {
	h := newHarness(t)
	cl := facet("https://openlineage.io/spec/facets/1-2-0/ColumnLineageDatasetFacet.json", map[string]any{
		"fields": map[string]any{
			"customer": map[string]any{"inputFields": []any{
				map[string]any{"namespace": pgNS, "name": "shop.public.customers", "field": "name", "transformations": []any{map[string]any{"type": "DIRECT", "subtype": "IDENTITY"}}},
			}},
			"revenue": map[string]any{"inputFields": []any{
				map[string]any{"namespace": pgNS, "name": "shop.public.orders", "field": "total", "transformations": []any{map[string]any{"type": "DIRECT", "subtype": "AGGREGATION", "description": "sum"}}},
				map[string]any{"namespace": pgNS, "name": "shop.public.orders", "field": "status", "transformations": []any{map[string]any{"type": "INDIRECT", "subtype": "FILTER"}}},
			}},
		},
		"dataset": []any{map[string]any{"namespace": pgNS, "name": "shop.public.orders", "field": "customer_id", "transformations": []any{map[string]any{"type": "INDIRECT", "subtype": "JOIN"}}}},
	})
	schema := facet("https://openlineage.io/spec/facets/1-2-0/SchemaDatasetFacet.json", map[string]any{"fields": []any{
		map[string]any{"name": "customer", "type": "text"}, map[string]any{"name": "revenue", "type": "numeric"},
	}})
	ev := runEvent("COMPLETE", "r1", "revenue", []ds{pgTable("orders"), pgTable("customers")}, []ds{withFacets(pgTable("revenue"), map[string]any{"columnLineage": cl, "schema": schema})})
	raw := mustJSONBytes(t, ev)
	h.send(raw)

	orders, customers, revenue, j := pgMRN("orders"), pgMRN("customers"), pgMRN("revenue"), jobMRN("revenue")
	requireGraph(t, graph{
		Nodes: []string{orders, customers, revenue, j},
		Edges: []string{
			e(orders, "CONSUMES", j), e(customers, "CONSUMES", j), e(j, "PRODUCES", revenue),
			colEdge(orders, revenue, j, "status,total => revenue (AGGREGATION sum, FILTER)"),
			colEdge(customers, revenue, j, "name => customer"),
		},
	}, h.lineageOf(revenue, 10, "upstream"))
}

func TestConcurrentEventsMakeOneAssetAndOneEdge(t *testing.T) {
	h := newHarness(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ev := runEvent("START", fmt.Sprintf("r%d", i), "j1", []ds{pgTable("a")}, []ds{pgTable("b")})
			raw := mustJSONBytes(t, ev)
			evt, err := parse(raw)
			if assert.NoError(t, err) {
				assert.NoError(t, h.lineage.ProcessOpenLineageEvent(h.ctx, evt, "openlineage"))
			}
		}(i)
	}
	wg.Wait()
	a, b, j1 := pgMRN("a"), pgMRN("b"), jobMRN("j1")
	assert.ElementsMatch(t, []string{a, b, j1}, h.allMRNs())
	assert.Equal(t, sorted([]string{e(a, "CONSUMES", j1), e(j1, "PRODUCES", b)}), h.allEdges())
}

// A run's parent facet names the run that started it. The job that recorded that run contains the child job, whichever arrives first and whatever produced them.
func TestParentRunsContainTheirChildren(t *testing.T) {
	child := func(runID, job, parentRun string) map[string]any {
		ev := runEvent("COMPLETE", runID, job, []ds{pgTable("a")}, []ds{pgTable("b")})
		ev["run"].(map[string]any)["facets"] = map[string]any{
			"parent": facet("https://openlineage.io/spec/facets/1-2-0/ParentRunFacet.json", map[string]any{
				"run": map[string]any{"runId": parentRun},
				"job": map[string]any{"namespace": "shop", "name": "app"},
			}),
		}
		return ev
	}
	t.Run("parent first", func(t *testing.T) {
		h := newHarness(t)
		h.sendJSON(runEvent("START", "app-run", "app", nil, nil))
		h.sendJSON(child("c1", "app.step_one", "app-run"))
		h.sendJSON(child("c2", "app.step_two", "app-run"))
		assert.Contains(t, h.allEdges(), e(jobMRN("app"), "CONTAINS", jobMRN("app.step_one")))
		assert.Contains(t, h.allEdges(), e(jobMRN("app"), "CONTAINS", jobMRN("app.step_two")))
	})
	t.Run("children first", func(t *testing.T) {
		h := newHarness(t)
		h.sendJSON(child("c1", "app.step_one", "app-run"))
		assert.NotContains(t, h.allMRNs(), jobMRN("app"), "an unseen parent is not invented")
		h.sendJSON(runEvent("COMPLETE", "app-run", "app", nil, nil))
		assert.Contains(t, h.allEdges(), e(jobMRN("app"), "CONTAINS", jobMRN("app.step_one")))
	})
	t.Run("a parent run that never arrives links nothing", func(t *testing.T) {
		h := newHarness(t)
		h.sendJSON(child("c1", "app.step_one", "unknown-run"))
		for _, edge := range h.allEdges() {
			assert.NotContains(t, edge, "CONTAINS")
		}
	})
}
