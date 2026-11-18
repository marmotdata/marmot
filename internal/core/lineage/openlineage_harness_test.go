package lineage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

// These tests run OpenLineage events through the real services against a migrated Postgres (MARMOT_TEST_POSTGRES_DSN) and compare the lineage Marmot builds with a graph written out by hand.

type recorderStub struct{}

func (recorderStub) RecordSearchQuery(context.Context, string, string)               {}
func (recorderStub) RecordAssetView(context.Context, string, string, string, string) {}
func (recorderStub) RecordDBQuery(context.Context, string, time.Duration, bool)      {}
func (recorderStub) WrapDBQuery(_ context.Context, _ string, fn func() error) error {
	return fn()
}
func (recorderStub) RecordCustomMetrics(context.Context, []metrics.Metric) error { return nil }

type olHarness struct {
	t       *testing.T
	ctx     context.Context
	assets  asset.Service
	lineage lineage.Service
	runs    runs.Service
	exec    func(sql string, args ...any) int64
	query   func(sql string, args ...any) []map[string]any
}

func newHarness(t *testing.T) *olHarness {
	t.Helper()
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	assetSvc := asset.NewService(asset.NewPostgresRepository(pool, recorderStub{}))
	lineageSvc := lineage.NewService(lineage.NewPostgresRepository(pool), assetSvc)
	h := &olHarness{
		t:       t,
		ctx:     ctx,
		assets:  assetSvc,
		lineage: lineageSvc,
		runs:    runs.NewService(runs.NewPostgresRepository(pool), assetSvc, lineageSvc, nil, recorderStub{}),
	}
	h.exec = func(sql string, args ...any) int64 {
		tag, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
		return tag.RowsAffected()
	}
	h.query = func(sql string, args ...any) []map[string]any {
		rows, err := pool.Query(ctx, sql, args...)
		require.NoError(t, err)
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			vals, err := rows.Values()
			require.NoError(t, err)
			row := map[string]any{}
			for i, f := range rows.FieldDescriptions() {
				row[f.Name] = vals[i]
			}
			out = append(out, row)
		}
		require.NoError(t, rows.Err())
		return out
	}
	return h
}

// pluginRun ingests assets and lineage the way a plugin's pipeline run does, so tests can check OpenLineage lands on the same assets.
func (h *olHarness) pluginRun(pipeline, source string, assets []runs.CreateAssetInput, edges []runs.LineageInput) *runs.ProcessAssetsResponse {
	h.t.Helper()
	run, err := h.runs.StartRunAs(h.ctx, pipeline, source, "tester", "user:tester", plugin.RawPluginConfig{})
	require.NoError(h.t, err)
	resp, err := h.runs.ProcessEntities(h.ctx, run.RunID, assets, edges, nil, nil, nil, pipeline, source)
	require.NoError(h.t, err)
	require.NoError(h.t, h.runs.CompleteRun(h.ctx, run.RunID, plugin.StatusCompleted, &plugin.RunSummary{}, ""))
	for _, a := range resp.Assets {
		require.Empty(h.t, a.Error, a.MRN)
	}
	return resp
}

// send parses and files one event the way the API does.
func (h *olHarness) send(raw []byte) {
	h.t.Helper()
	ev, err := lineage.ParseEvent(raw)
	require.NoError(h.t, err, "%s", raw)
	require.NoError(h.t, h.lineage.ProcessOpenLineageEvent(h.ctx, ev, "openlineage"))
}

func (h *olHarness) sendJSON(v any) {
	h.t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(h.t, err)
	h.send(raw)
}

// replay sends every captured event of a producer in the order it was captured.
func (h *olHarness) replay(producer string) {
	h.t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata/openlineage/events", producer, "*.json"))
	require.NoError(h.t, err)
	require.NotEmpty(h.t, files)
	sort.Strings(files)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		require.NoError(h.t, err)
		h.send(raw)
	}
}

// asset returns the asset an MRN names, stubs included, following aliases.
func (h *olHarness) asset(m string) *asset.Asset {
	h.t.Helper()
	resolved, err := h.assets.ResolveMRN(h.ctx, m)
	require.NoError(h.t, err)
	found, err := h.assets.GetByExactMRNs(h.ctx, []string{resolved})
	require.NoError(h.t, err)
	a := found[resolved]
	require.NotNil(h.t, a, "no asset at %s (resolved %s); assets are:\n%s", m, resolved, strings.Join(h.allMRNs(), "\n"))
	return a
}

func (h *olHarness) allMRNs() []string {
	rows := h.query(`SELECT mrn FROM assets ORDER BY mrn`)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r["mrn"].(string))
	}
	return out
}

// edge is one lineage edge as the tests compare it: source and target MRNs, its type and, for a direct dataset edge, the job and column mappings.
type edge struct {
	Source  string
	Target  string
	Type    string
	Job     string
	Columns []string
}

func (e edge) String() string {
	s := fmt.Sprintf("%s -[%s]-> %s", e.Source, e.Type, e.Target)
	if e.Job != "" {
		s += " via " + e.Job
	}
	if len(e.Columns) > 0 {
		s += " {" + strings.Join(e.Columns, "; ") + "}"
	}
	return s
}

// graph is the lineage the API returns around one asset, reduced to what the tests compare.
type graph struct {
	Nodes []string
	Edges []string
}

// lineageOf asks the lineage service for the graph around an asset, as the UI and API do.
func (h *olHarness) lineageOf(m string, depth int, direction string) graph {
	h.t.Helper()
	a := h.asset(m)
	resp, err := h.lineage.GetAssetLineage(h.ctx, a.ID, depth, direction)
	require.NoError(h.t, err)
	var g graph
	seen := map[string]int{}
	for _, n := range resp.Nodes {
		require.NotNil(h.t, n.Asset)
		seen[*n.Asset.MRN]++
		g.Nodes = append(g.Nodes, *n.Asset.MRN)
	}
	for m, c := range seen {
		require.Equal(h.t, 1, c, "node %s appears %d times", m, c)
	}
	for _, e := range resp.Edges {
		g.Edges = append(g.Edges, toEdge(e).String())
	}
	sort.Strings(g.Nodes)
	sort.Strings(g.Edges)
	return g
}

func toEdge(e lineage.LineageEdge) edge {
	out := edge{Source: e.Source, Target: e.Target, Type: e.Type, Job: e.JobMRN}
	for _, c := range e.ColumnLineage {
		cols := append([]string(nil), c.FromColumns...)
		sort.Strings(cols)
		s := strings.Join(cols, ",") + " => " + c.ToColumn
		if c.Transform != "" {
			s += " (" + c.Transform + ")"
		}
		out.Columns = append(out.Columns, s)
	}
	sort.Strings(out.Columns)
	return out
}

// allEdges lists every edge in the catalog, so a test can assert there are no extra or duplicate ones anywhere.
func (h *olHarness) allEdges() []string {
	rows := h.query(`SELECT source_mrn, target_mrn, type, COALESCE(job_mrn, '') AS job, column_lineage FROM lineage_edges ORDER BY 1, 2`)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		e := lineage.LineageEdge{Source: r["source_mrn"].(string), Target: r["target_mrn"].(string), Type: fmt.Sprint(r["type"]), JobMRN: r["job"].(string)}
		if raw := r["column_lineage"]; raw != nil {
			b, _ := json.Marshal(raw)
			_ = json.Unmarshal(b, &e.ColumnLineage)
		}
		out = append(out, toEdge(e).String())
	}
	sort.Strings(out)
	return out
}

// requireGraph compares a graph with the one written out by hand, listing what is missing and what is extra.
func requireGraph(t *testing.T, want, got graph) {
	t.Helper()
	sort.Strings(want.Nodes)
	sort.Strings(want.Edges)
	if want.Nodes == nil {
		want.Nodes = []string{}
	}
	if want.Edges == nil {
		want.Edges = []string{}
	}
	if got.Nodes == nil {
		got.Nodes = []string{}
	}
	if got.Edges == nil {
		got.Edges = []string{}
	}
	require.Equal(t, want.Nodes, got.Nodes, "nodes\nmissing: %v\nextra: %v", missing(want.Nodes, got.Nodes), missing(got.Nodes, want.Nodes))
	require.Equal(t, want.Edges, got.Edges, "edges\nmissing:\n  %s\nextra:\n  %s", strings.Join(missing(want.Edges, got.Edges), "\n  "), strings.Join(missing(got.Edges, want.Edges), "\n  "))
}

func missing(want, got []string) []string {
	have := map[string]int{}
	for _, g := range got {
		have[g]++
	}
	var out []string
	for _, w := range want {
		if have[w] > 0 {
			have[w]--
			continue
		}
		out = append(out, w)
	}
	return out
}

// e builds the string form of an edge for an expected graph.
func e(source, typ, target string) string {
	return edge{Source: source, Target: target, Type: typ}.String()
}

// colEdge builds the string form of a direct dataset edge carrying column mappings.
func colEdge(source, target, job string, columns ...string) string {
	sort.Strings(columns)
	return edge{Source: source, Target: target, Type: "DIRECT", Job: job, Columns: columns}.String()
}

// Event builders for hand-written scenarios.

const (
	producer  = "https://github.com/marmotdata/marmot/tests"
	runURL    = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/RunEvent"
	jobURL    = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/JobEvent"
	dsURL     = "https://openlineage.io/spec/2-0-2/OpenLineage.json#/$defs/DatasetEvent"
	pgNS      = "postgres://pg:5432"
	pgZoneful = "@host=pg:5432"
)

type ds = map[string]any

func pgTable(name string) ds { return ds{"namespace": pgNS, "name": "shop.public." + name} }

// pgMRN is the MRN the Postgres plugin and OpenLineage both give shop.public.<name> on pg:5432 with no zone bound to the host.
func pgMRN(name string) string { return "mrn://asset/table/postgresql/shop.public." + name }

func jobMRN(name string) string { return "mrn://asset/job/openlineage/shop." + name }

var clock = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func runEvent(eventType, runID, job string, inputs, outputs []ds) map[string]any {
	clock = clock.Add(time.Second)
	ev := map[string]any{
		"eventType": eventType,
		"eventTime": clock.Format(time.RFC3339Nano),
		"producer":  producer,
		"schemaURL": runURL,
		"run":       map[string]any{"runId": runID},
		"job":       map[string]any{"namespace": "shop", "name": job},
	}
	if inputs != nil {
		ev["inputs"] = inputs
	}
	if outputs != nil {
		ev["outputs"] = outputs
	}
	return ev
}

func facet(schema string, fields map[string]any) map[string]any {
	out := map[string]any{"_producer": producer, "_schemaURL": schema}
	for k, v := range fields {
		out[k] = v
	}
	return out
}

func withFacets(d ds, facets map[string]any) ds {
	c := ds{}
	for k, v := range d {
		c[k] = v
	}
	c["facets"] = facets
	return c
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func mustJSONBytes(t testing.TB, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

func parse(raw []byte) (*lineage.RunEvent, error) { return lineage.ParseEvent(raw) }
