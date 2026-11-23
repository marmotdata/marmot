package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/pkg/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateSnapshot = flag.Bool("update", false, "rewrite testdata/tools.json from the current tool list")

// TestToolListSnapshot pins every tool's name, title, description, annotations and input schema, for a reader and for a writer, so a change to what agents see shows up as a reviewed diff.
func TestToolListSnapshot(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, stubKnowledge{}, stubDocs{})
	got := map[string]any{}
	for name, p := range map[string]stubPrincipal{
		"reader": {permissions: map[string]bool{"assets:view": true}},
		"writer": {permissions: map[string]bool{"assets:manage": true}},
	} {
		session := connect(t, s, p)
		res, err := session.ListTools(context.Background(), nil)
		require.NoError(t, err)
		got[name] = map[string]any{"instructions": session.InitializeResult().Instructions, "tools": res.Tools}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	path := filepath.Join("testdata", "tools.json")
	if *updateSnapshot {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run go test ./internal/mcp -run TestToolListSnapshot -update to create it")
	assert.JSONEq(t, string(want), string(data), "the tool list changed; review it and rerun with -update")
}

func TestReadersAreNotOfferedWriteTools(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, stubKnowledge{}, stubDocs{})
	reader, readerInstructions := listTools(t, s, stubPrincipal{permissions: map[string]bool{"assets:view": true}})
	for _, w := range []string{"update_documentation", "manage_tags", "manage_owners", "create_knowledge"} {
		assert.NotContains(t, reader, w)
	}
	assert.NotContains(t, readerInstructions, "preview")
	assert.Less(t, len(readerInstructions), 2000, "instructions stay short")
}

func connect(t *testing.T, s *Server, p stubPrincipal) *mcpsdk.ClientSession {
	t.Helper()
	server := s.CreateMCPServer(context.Background(), p, "")
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	session, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// readAssets is an asset service holding a few assets, searched by exact filters.
type readAssets struct {
	asset.Service
	assets  []*asset.Asset
	filters []asset.SearchFilter
}

func (f *readAssets) Get(_ context.Context, id string) (*asset.Asset, error) {
	for _, a := range f.assets {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, asset.ErrAssetNotFound
}

func (f *readAssets) GetByMRN(_ context.Context, m string) (*asset.Asset, error) {
	for _, a := range f.assets {
		if a.MRN != nil && *a.MRN == m {
			return a, nil
		}
	}
	return nil, asset.ErrAssetNotFound
}

func (f *readAssets) Search(_ context.Context, filter asset.SearchFilter, _ bool) ([]*asset.Asset, int, asset.AvailableFilters, error) {
	f.filters = append(f.filters, filter)
	var hits []*asset.Asset
	facets := asset.AvailableFilters{Types: map[string]int{}, Providers: map[string]int{}, Tags: map[string]int{}}
	for _, a := range f.assets {
		if filter.Query != "" && !strings.Contains(strings.ToLower(*a.Name), strings.ToLower(filter.Query)) {
			continue
		}
		if len(filter.Types) > 0 && filter.Types[0] != a.Type {
			continue
		}
		if filter.OwnerID != nil && *filter.OwnerID != "team-1" {
			continue
		}
		hits = append(hits, a)
		facets.Types[a.Type]++
		facets.Providers[a.Providers[0]]++
		for _, tag := range a.Tags {
			facets.Tags[tag]++
		}
	}
	total := len(hits)
	if filter.Offset < len(hits) {
		hits = hits[filter.Offset:]
	} else {
		hits = nil
	}
	if filter.Limit > 0 && len(hits) > filter.Limit {
		hits = hits[:filter.Limit]
	}
	return hits, total, facets, nil
}

func (f *readAssets) Summary(context.Context) (*asset.AssetSummary, error) {
	return &asset.AssetSummary{Types: map[string]asset.AssetTypeSummary{"Table": {}, "Topic": {}}, Providers: map[string]int{"PostgreSQL": 1, "Kafka": 1}, Tags: map[string]int{"pii": 3}}, nil
}

func (f *readAssets) GetTerms(context.Context, string) ([]asset.AssetTerm, error) {
	return []asset.AssetTerm{{TermName: "MRR"}}, nil
}

type readTeams struct {
	TeamService
	owners map[string][]Owner
}

func (f readTeams) ListAssetOwners(_ context.Context, id string) ([]Owner, error) {
	return f.owners[id], nil
}

func (f readTeams) GetTeam(_ context.Context, id string) (*Team, error) {
	return &Team{ID: id, Name: "Billing"}, nil
}

type readLineage struct {
	lineage.Service
	nodes []lineage.LineageNode
}

func (f readLineage) GetAssetLineage(context.Context, string, int, string) (*lineage.LineageResponse, error) {
	return &lineage.LineageResponse{Nodes: f.nodes}, nil
}

type readGlossary struct{ GlossaryService }

func (readGlossary) Search(context.Context, glossary.SearchFilter) (*glossary.ListResult, error) {
	return &glossary.ListResult{}, nil
}

func tableAsset(id, name, typ, provider string, tags ...string) *asset.Asset {
	m := "mrn://asset/" + strings.ToLower(typ) + "/" + strings.ToLower(provider) + "/" + name
	return &asset.Asset{ID: id, Name: strPtr(name), Type: typ, Providers: []string{provider}, MRN: &m, Tags: tags, UpdatedAt: time.Now().Add(-72 * time.Hour)}
}

func readContext(assets *readAssets) *ToolContext {
	return &ToolContext{
		assetService:    assets,
		teamService:     readTeams{owners: map[string][]Owner{a1: {{Type: "team", ID: "team-1", Name: "Billing"}}}},
		lineageService:  readLineage{},
		glossaryService: readGlossary{},
		rootURL:         "https://marmot.example.com",
		principal:       stubPrincipal{},
	}
}

// must turns a tool handler's three results into its text, failing the test on a Go error.
func must(t *testing.T) func(*mcpsdk.CallToolResult, any, error) string {
	return func(res *mcpsdk.CallToolResult, _ any, err error) string {
		t.Helper()
		require.NoError(t, err)
		return resultText(t, res)
	}
}

func TestAssetCardLeadsWithIdentityOwnersAndFreshness(t *testing.T) {
	mrr := tableAsset(a1, "analytics.marts.fct_mrr", "Table", "BigQuery", "finance", "certified")
	mrr.UserDescription = strPtr("MRR per organisation per month.")
	mrr.Schema = map[string]string{"month": `{"type":"DATE","description":"First day of the month"}`, "mrr_gbp": "NUMERIC"}
	tc := readContext(&readAssets{assets: []*asset.Asset{mrr}})
	tc.lineageService = readLineage{nodes: []lineage.LineageNode{
		{Asset: tableAsset("d2", "MRR overview", "Dashboard", "Metabase"), Depth: 2},
		{Asset: tableAsset("u1", "console.public.invoices", "Table", "PostgreSQL"), Depth: -1, LastRunStatus: "failed"},
		{Asset: tableAsset("d1", "analytics.marts.mrr_by_plan", "Table", "BigQuery"), Depth: 1},
	}}

	out := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{ID: *mrr.MRN}))
	assert.Contains(t, out, "mrn: mrn://asset/table/bigquery/analytics.marts.fct_mrr")
	assert.Contains(t, out, "url: https://marmot.example.com/discover/table/bigquery/analytics.marts.fct_mrr", "links use the UI's three-segment route")
	assert.Contains(t, out, "Owners: Billing (team)")
	assert.Contains(t, out, "Updated: 3 days ago")
	assert.Contains(t, out, "Glossary terms: MRR")
	assert.Contains(t, out, "- month: DATE, First day of the month")
	assert.Less(t, strings.Index(out, "1 hop: analytics.marts.mrr_by_plan"), strings.Index(out, "2 hops: MRR overview"), "lineage is nearest first")
	assert.Contains(t, out, "console.public.invoices (Table) mrn://asset/table/postgresql/console.public.invoices [last run failed]")
	assert.Contains(t, out, `get_lineage {"depth":5,"direction":"downstream","id":"mrn://asset/table/bigquery/analytics.marts.fct_mrr"}`)
	assert.Equal(t, out, must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{ID: a1})), "the same asset reads the same every time")
}

func TestAssetCardFlagsUnownedAndDeprecated(t *testing.T) {
	legacy := tableAsset("0b6f3c1e-8d0a-4c55-9a59-2b7c0c7a0009", "analytics.legacy.customers_legacy", "Table", "BigQuery", "deprecated")
	tc := readContext(&readAssets{assets: []*asset.Asset{legacy}})
	out := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{ID: legacy.ID}))
	assert.Contains(t, out, "Owners: none")
	assert.Contains(t, out, "tagged deprecated")
}

func TestUnknownAssetIsNotFoundWithARealNextCall(t *testing.T) {
	tc := readContext(&readAssets{})
	res, _, _ := tc.discoverData(context.Background(), nil, DiscoverDataInput{ID: "mrn://asset/table/postgresql/nope"})
	assert.True(t, res.IsError)
	text := resultText(t, res)
	assert.Contains(t, text, "Error (not_found)")
	assert.Contains(t, text, `discover_data {"query":"mrn://asset/table/postgresql/nope"}`)
	assert.NotContains(t, text, "asset-id", "no placeholder ids")
}

func TestListingPagesKeepEveryFilter(t *testing.T) {
	var as []*asset.Asset
	for i := 0; i < 5; i++ {
		as = append(as, tableAsset(strings.Repeat("x", i+1), "orders_"+string(rune('a'+i)), "Table", "PostgreSQL", "pii"))
	}
	fa := &readAssets{assets: as}
	tc := readContext(fa)
	two := 2
	out := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{Query: "orders", Types: []string{"table"}, Tags: []string{"pii"}, Limit: &two}))
	assert.Contains(t, out, "5 assets match")
	assert.Contains(t, out, "showing 1 to 2")
	assert.Contains(t, out, `discover_data {"limit":2,"offset":2,"query":"orders","tags":["pii"],"types":["Table"]}`, "the next page keeps query, types and tags, with types in the catalog's spelling")
}

func TestFilteredListIsAListNotCounts(t *testing.T) {
	var as []*asset.Asset
	for i := 0; i < 25; i++ {
		as = append(as, tableAsset(strings.Repeat("y", i+1), "t"+strings.Repeat("y", i+1), "Table", "PostgreSQL"))
	}
	tc := readContext(&readAssets{assets: as})
	out := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{Types: []string{"Table"}}))
	assert.Contains(t, out, "showing 1 to 20", "a filtered browse over 20 results still lists assets")

	zero := 0
	counts := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{Limit: &zero}))
	assert.Contains(t, counts, "The catalog holds 25 assets.")
	assert.Contains(t, counts, "By type: Table 25")
}

func TestMetadataFiltersCountAcrossPages(t *testing.T) {
	var as []*asset.Asset
	for i := 0; i < 30; i++ {
		a := tableAsset(strings.Repeat("z", i+1), "topic_"+strings.Repeat("z", i+1), "Topic", "Kafka")
		a.Metadata = map[string]interface{}{"partitions": i}
		as = append(as, a)
	}
	tc := readContext(&readAssets{assets: as})
	out := must(t)(tc.discoverData(context.Background(), nil, DiscoverDataInput{MetadataFilters: []MetadataFilterInput{{Key: "partitions", Operator: ">=", Value: 10}}}))
	assert.Contains(t, out, "20 assets match", "the total counts every match, not only those on the first page")
}

func TestFindOwnershipByTeamIDNamesTheTeam(t *testing.T) {
	tc := readContext(&readAssets{assets: []*asset.Asset{tableAsset(a1, "fct_mrr", "Table", "BigQuery")}})
	no := false
	out := must(t)(tc.findOwnership(context.Background(), nil, FindOwnershipInput{TeamID: "team-1", IncludeGlossaryTerms: &no}))
	assert.Contains(t, out, "What Billing (team, id team-1) owns")
	assert.Contains(t, out, "Assets: 1")
	assert.NotContains(t, out, "Glossary terms", "include_glossary_terms false on its own is honoured")
}

func TestTraceLineageSaysWhenNothingIsRecorded(t *testing.T) {
	tc := readContext(&readAssets{assets: []*asset.Asset{tableAsset(a1, "fct_mrr", "Table", "BigQuery")}})
	out := must(t)(tc.traceLineage(context.Background(), nil, TraceLineageInput{AssetID: a1, Direction: "downstream"}))
	assert.Contains(t, out, "No lineage is recorded in that direction")

	res, _, _ := tc.traceLineage(context.Background(), nil, TraceLineageInput{ID: a1, Direction: "sideways"})
	assert.True(t, res.IsError)
}

func TestTruncateKeepsRunesWhole(t *testing.T) {
	assert.Equal(t, "héllo wö…", truncate("héllo wörld", 9), "eight whole runes and an ellipsis")
	assert.Equal(t, "short", truncate("short", 9))
}

func TestAssetURLForKnowledgeAndZones(t *testing.T) {
	m := "mrn://asset/table/postgresql@prod-eu/analytics.public.orders"
	assert.Equal(t, "https://m.example/discover/table/postgresql@prod-eu/analytics.public.orders", assetURL("https://m.example/", &asset.Asset{MRN: &m, Type: "Table"}))
	assert.Equal(t, "https://m.example/knowledge/abc", assetURL("https://m.example", &asset.Asset{ID: "abc", Type: "Policy"}))
	assert.Equal(t, "", assetURL("", &asset.Asset{ID: "abc", Type: "Table"}))
}

func (f readTeams) GetTeamByName(context.Context, string) (*Team, error) {
	return nil, errors.New("no team")
}

func (f *readAssets) GetAssetsByTerm(context.Context, string, int, int) ([]*asset.Asset, int, error) {
	return nil, 0, nil
}
