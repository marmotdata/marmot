package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/search"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSearch answers with fixed results, paging them by the filter's offset and limit, and records the filter it was given.
type fakeSearch struct {
	results []*search.Result
	got     []search.Filter
}

func (f *fakeSearch) Search(_ context.Context, filter search.Filter) (*search.Response, error) {
	f.got = append(f.got, filter)
	res := f.results
	if filter.Offset < len(res) {
		res = res[filter.Offset:]
	} else {
		res = nil
	}
	if len(res) > filter.Limit {
		res = res[:filter.Limit]
	}
	total := len(f.results)
	if filter.Query != "" {
		// Like the real service, word searches aren't counted.
		total = 0
	}
	return &search.Response{Results: res, Total: total, Facets: &search.Facets{Types: map[search.ResultType]int{search.ResultTypeAsset: len(f.results)}}}, nil
}

func assetResult(id, name, typ, provider string, owners ...string) *search.Result {
	m := "mrn://asset/" + strings.ToLower(typ) + "/" + strings.ToLower(provider) + "/" + name
	r := &search.Result{Type: search.ResultTypeAsset, ID: id, Name: name, Metadata: map[string]interface{}{"type": typ, "mrn": m}}
	for _, o := range owners {
		r.Owners = append(r.Owners, search.ResultOwner{Name: o, Kind: "team"})
	}
	return r
}

func must2[O any](t *testing.T) func(*mcpsdk.CallToolResult, O, error) (string, O) {
	return func(res *mcpsdk.CallToolResult, out O, err error) (string, O) {
		t.Helper()
		require.NoError(t, err)
		return resultText(t, res), out
	}
}

func TestSearchReturnsIDsFetchTakesAndPagesWithACursor(t *testing.T) {
	fs := &fakeSearch{results: []*search.Result{
		assetResult("a", "analytics.marts.fct_mrr", "Table", "BigQuery", "Billing"),
		assetResult("b", "No customer names on public material", "Policy", "Marmot", "Marmot"),
		assetResult("c", "analytics.legacy.customers_legacy", "Table", "BigQuery"),
	}}
	tc := readContext(&readAssets{})
	tc.searchService = fs
	two := 2

	text, out := must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Query: "customer", Limit: &two}))
	require.Len(t, out.Results, 2)
	assert.Equal(t, "mrn://asset/table/bigquery/analytics.marts.fct_mrr", out.Results[0].ID, "an asset's id is its MRN")
	assert.Equal(t, kindKnowledge, out.Results[1].Kind, "a policy is knowledge")
	assert.Equal(t, "b", out.Results[1].ID, "a knowledge entry's id is its asset ID")
	assert.True(t, out.HasMore, "one extra result tells there is another page")
	assert.Contains(t, text, "there are more")
	require.NotEmpty(t, out.NextCursor)
	assert.Contains(t, text, `fetch {"ids":["mrn://asset/table/bigquery/analytics.marts.fct_mrr","b"]}`)

	_, page2 := must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Cursor: out.NextCursor}))
	require.Len(t, page2.Results, 1)
	assert.Contains(t, page2.Results[0].Flags, "unowned")
	assert.Equal(t, "customer", fs.got[1].Query, "the cursor carries the query")
	assert.Equal(t, 2, fs.got[1].Offset)
}

func TestSearchKnowledgeKindFiltersToKnowledgeTypes(t *testing.T) {
	fs := &fakeSearch{}
	tc := readContext(&readAssets{})
	tc.searchService = fs
	must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Query: "refund", Kinds: []string{"knowledge"}}))
	require.Len(t, fs.got, 1)
	assert.Equal(t, []search.ResultType{search.ResultTypeAsset}, fs.got[0].Types)
	assert.Contains(t, fs.got[0].AssetTypes, "Policy")

	res, _, _ := tc.search(context.Background(), nil, SearchInput{Kinds: []string{"spreadsheets"}})
	assert.True(t, res.IsError)
}

func TestSearchOwnerByTeamName(t *testing.T) {
	fs := &fakeSearch{}
	tc := readContext(&readAssets{})
	tc.searchService = fs
	tc.teamService = &fakeTeamService{teams: []*Team{{ID: teamID, Name: "Data platform"}}}
	text, _ := must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Owner: "data platform"}))
	assert.Equal(t, []string{teamID}, fs.got[0].Owners)
	assert.Contains(t, text, "owned by Data platform")
}

func TestFetchReadsAssetsTermsAndURLs(t *testing.T) {
	mrr := tableAsset(a1, "analytics.marts.fct_mrr", "Table", "BigQuery")
	tc := readContext(&readAssets{assets: []*asset.Asset{mrr}})
	termID := "0b6f3c1e-8d0a-4c55-9a59-2b7c0c7a0003"
	tc.glossaryService = termGlossary{term: &glossary.GlossaryTerm{ID: termID, Name: "MRR", Definition: "Monthly recurring revenue."}}

	text, out := must2[FetchOutput](t)(tc.fetch(context.Background(), nil, FetchInput{IDs: []string{
		"https://marmot.example.com/discover/table/bigquery/analytics.marts.fct_mrr",
		termID,
		"nothing-like-this",
	}}))
	require.Len(t, out.Items, 2)
	assert.Equal(t, kindAsset, out.Items[0].Kind)
	assert.Equal(t, "Billing (team)", out.Items[0].Metadata["owners"])
	assert.Equal(t, kindTerm, out.Items[1].Kind)
	assert.Contains(t, out.Items[1].Text, "Monthly recurring revenue.")
	assert.Equal(t, []string{"nothing-like-this"}, out.NotFound)
	assert.Contains(t, text, `Not found: "nothing-like-this"`)
}

func TestFetchSaysWhenStale(t *testing.T) {
	mrr := tableAsset(a1, "analytics.marts.fct_mrr", "Table", "BigQuery")
	tc := readContext(&readAssets{assets: []*asset.Asset{mrr}})
	tc.signals = staleAll{}
	text, out := must2[FetchOutput](t)(tc.fetch(context.Background(), nil, FetchInput{ID: a1}))
	assert.Contains(t, text, "Caution: stale")
	assert.Equal(t, "true", out.Items[0].Metadata["stale"])
}

func TestGetLineageReturnsNodesEdgesAndOmitted(t *testing.T) {
	root := tableAsset(a1, "console.public.instances", "Table", "PostgreSQL")
	stg := tableAsset("s", "analytics.staging.stg_instances", "Table", "BigQuery")
	fct := tableAsset("f", "analytics.marts.fct_daily_active_instances", "Table", "BigQuery")
	dash := tableAsset("d", "Instance health", "Dashboard", "Metabase")
	tc := readContext(&readAssets{assets: []*asset.Asset{root}})
	tc.lineageService = readLineage{nodes: []lineage.LineageNode{
		{Asset: dash, Depth: 3}, {Asset: stg, Depth: 1}, {Asset: fct, Depth: 2},
	}}
	tc.lineageService = edgeLineage{readLineage: tc.lineageService.(readLineage), edges: []lineage.LineageEdge{
		{Source: *root.MRN, Target: *stg.MRN, Type: "DIRECT"}, {Source: *stg.MRN, Target: *fct.MRN, Type: "DIRECT"}, {Source: *fct.MRN, Target: *dash.MRN, Type: "DIRECT"},
	}}

	_, out := must2[GetLineageOutput](t)(tc.getLineage(context.Background(), nil, GetLineageInput{ID: a1, Direction: "downstream", Limit: 2}))
	require.Len(t, out.Nodes, 2)
	assert.Equal(t, "analytics.staging.stg_instances", out.Nodes[0].Name, "nearest first")
	assert.Equal(t, 1, out.Omitted)
	assert.Len(t, out.Edges, 2, "only edges between nodes shown")
}

func TestUpdateEntityNeedsTheSignedPreview(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{a1: writeTestAsset()}}
	ft := &fakeTeamService{teams: []*Team{{ID: teamID, Name: "Billing"}}}
	tc := writeToolContext(fa, ft)
	tc.previewKey = []byte("k")
	in := UpdateEntityInput{ID: a1, AddTags: []string{"gold", "pii"}, AddOwners: []string{"billing"}}

	text, out := must2[UpdateEntityOutput](t)(tc.updateEntity(context.Background(), nil, in))
	assert.Contains(t, text, "nothing has changed yet")
	assert.Equal(t, []string{`add tag "gold"`, "add owner Billing (team)"}, out.Changes, "existing tags are left out")
	assert.Empty(t, fa.addedTags)

	tampered := in
	tampered.AddTags = []string{"gold", "pii", "admin"}
	tampered.PreviewID = out.PreviewID
	res, _, _ := tc.updateEntity(context.Background(), nil, tampered)
	assert.True(t, res.IsError, "a preview_id only applies the change it previewed")
	assert.Empty(t, fa.addedTags)

	in.PreviewID = out.PreviewID
	_, applied := must2[UpdateEntityOutput](t)(tc.updateEntity(context.Background(), nil, in))
	assert.True(t, applied.Applied)
	assert.Equal(t, []string{"gold"}, fa.addedTags)
	assert.Equal(t, []string{"team:" + teamID}, ft.added)
}

func TestUpdateEntityRefusesAChangedEntity(t *testing.T) {
	a := writeTestAsset()
	fa := &fakeAssetService{assets: map[string]*asset.Asset{a1: a}}
	tc := writeToolContext(fa, &fakeTeamService{})
	tc.previewKey = []byte("k")
	in := UpdateEntityInput{ID: a1, AddTags: []string{"gold"}}
	_, out := must2[UpdateEntityOutput](t)(tc.updateEntity(context.Background(), nil, in))
	a.UpdatedAt = a.UpdatedAt.Add(time.Second)
	in.PreviewID = out.PreviewID
	res, _, _ := tc.updateEntity(context.Background(), nil, in)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "changed since")
}

func TestUpdateEntityRefusesSecretsAndUnknownOwners(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{a1: writeTestAsset()}}
	tc := writeToolContext(fa, &fakeTeamService{})
	doc := "Connect with postgres://app:hunter2hunter2@db/prod"
	res, _, _ := tc.updateEntity(context.Background(), nil, UpdateEntityInput{ID: a1, Documentation: &doc})
	assert.True(t, res.IsError)

	res, _, _ = tc.updateEntity(context.Background(), nil, UpdateEntityInput{ID: a1, AddOwners: []string{"nobody"}})
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "not_found")
}

type termGlossary struct {
	GlossaryService
	term *glossary.GlossaryTerm
}

func (g termGlossary) Get(_ context.Context, id string) (*glossary.GlossaryTerm, error) {
	if g.term != nil && g.term.ID == id {
		return g.term, nil
	}
	return nil, glossary.ErrTermNotFound
}

func (termGlossary) Search(context.Context, glossary.SearchFilter) (*glossary.ListResult, error) {
	return &glossary.ListResult{}, nil
}

func (termGlossary) GetAncestors(context.Context, string) ([]*glossary.GlossaryTerm, error) {
	return nil, nil
}

func (termGlossary) GetChildren(context.Context, string) ([]*glossary.GlossaryTerm, error) {
	return nil, nil
}

type staleAll struct{}

func (staleAll) IsStale(context.Context, string) (bool, error) { return true, nil }

type edgeLineage struct {
	readLineage
	edges []lineage.LineageEdge
}

func (f edgeLineage) GetAssetLineage(context.Context, string, int, string) (*lineage.LineageResponse, error) {
	return &lineage.LineageResponse{Nodes: f.nodes, Edges: f.edges}, nil
}

func TestEmptyWordSearchSuggestsAMatchingTag(t *testing.T) {
	tc := readContext(&readAssets{})
	tc.searchService = &fakeSearch{}
	text, _ := must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Query: "PII tables"}))
	assert.Contains(t, text, `search {"tags":["pii"]}`, "tags aren't searched as words, so the tag filter is offered")

	text, _ = must2[SearchOutput](t)(tc.search(context.Background(), nil, SearchInput{Tags: []string{"pii"}}))
	assert.Contains(t, text, "tagged pii")
}
