package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// search, fetch and get_lineage are the read path the audit recommended: one search across every kind, one read that takes any identifier, and lineage with a cap that says what it left out. Each answers in Markdown for the model and as structured content for clients that read it, such as ChatGPT's company knowledge.

// Kinds search can return. Knowledge entries are assets underneath, told apart by their type.
const (
	kindAsset       = "asset"
	kindKnowledge   = "knowledge"
	kindTerm        = "term"
	kindDataProduct = "data_product"
	kindTeam        = "team"
)

type SearchInput struct {
	Query     string   `json:"query,omitempty" jsonschema:"Words to look for, such as 'mrr', 'refund policy' or 'orders table'"`
	Kinds     []string `json:"kinds,omitempty" jsonschema:"Only these kinds: asset, knowledge, term, data_product or team. Default all"`
	Types     []string `json:"types,omitempty" jsonschema:"Only assets of these types, such as Table, Topic, Dashboard, Policy or Runbook"`
	Providers []string `json:"providers,omitempty" jsonschema:"Only assets from these providers, such as PostgreSQL, Kafka or BigQuery"`
	Tags      []string `json:"tags,omitempty" jsonschema:"Only assets carrying these tags, such as pii"`
	Owner     string   `json:"owner,omitempty" jsonschema:"Only what this team or person owns: a team name, username, email, ID, or me"`
	Limit     *int     `json:"limit,omitempty" jsonschema:"Results to return, 1 to 50. Default 10. Send 0 for counts only"`
	Cursor    string   `json:"cursor,omitempty" jsonschema:"The next_cursor from a previous search, to get the next page with the same filters"`
}

// SearchHit is one result: enough to choose it and to pass its id to fetch.
type SearchHit struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Type      string   `json:"type,omitempty"`
	Title     string   `json:"title"`
	URL       string   `json:"url,omitempty"`
	Snippet   string   `json:"snippet,omitempty"`
	Owners    []string `json:"owners"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	Stale     bool     `json:"stale,omitempty"`
	Flags     []string `json:"flags,omitempty"`
}

type SearchOutput struct {
	Results []SearchHit `json:"results"`
	// Total counts every match when Marmot can count them cheaply; word searches aren't counted, so there it is how many were seen and HasMore says whether there are more.
	Total      int            `json:"total"`
	HasMore    bool           `json:"has_more"`
	Counts     map[string]int `json:"counts,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
	// Note and Next carry the guidance the Markdown answer gives, for clients that show the model only this structured result.
	Note string   `json:"note,omitempty"`
	Next []string `json:"next,omitempty"`
}

type FetchInput struct {
	ID      string   `json:"id,omitempty" jsonschema:"What to read: an id from search, an MRN, a Marmot URL or an exact name"`
	IDs     []string `json:"ids,omitempty" jsonschema:"Up to 10 ids to read at once, instead of id"`
	Section string   `json:"section,omitempty" jsonschema:"For a knowledge entry: read only the write-up section with this heading"`
	Offset  int      `json:"offset,omitempty" jsonschema:"For a knowledge entry: character offset into the write-up, from a previous answer"`
}

// FetchedItem follows the shape ChatGPT's fetch contract names: id, title, text, url and metadata.
type FetchedItem struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Title    string            `json:"title"`
	URL      string            `json:"url,omitempty"`
	Text     string            `json:"text"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type FetchOutput struct {
	Items    []FetchedItem `json:"items"`
	NotFound []string      `json:"not_found,omitempty"`
}

type GetLineageInput struct {
	ID        string `json:"id" jsonschema:"The asset's MRN, ID or exact name"`
	Direction string `json:"direction,omitempty" jsonschema:"upstream (what feeds it), downstream (what depends on it) or both. Default both"`
	Depth     int    `json:"depth,omitempty" jsonschema:"Hops to follow, 1 to 10. Default 3"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Most assets to return, 1 to 200. Default 60, nearest first"`
}

type LineageNodeOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Direction     string `json:"direction"`
	Hops          int    `json:"hops"`
	LastRunStatus string `json:"last_run_status,omitempty"`
	Placeholder   bool   `json:"placeholder,omitempty"`
}

type LineageEdgeOut struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type,omitempty"`
}

type GetLineageOutput struct {
	Root    string           `json:"root"`
	Nodes   []LineageNodeOut `json:"nodes"`
	Edges   []LineageEdgeOut `json:"edges"`
	Omitted int              `json:"omitted"`
	Note    string           `json:"note,omitempty"`
	Next    []string         `json:"next,omitempty"`
}

// searchCursor is what a next_cursor carries: the offset and every filter, so a page can't drift from the query that made it.
type searchCursor struct {
	Offset int         `json:"o"`
	Input  SearchInput `json:"i"`
}

func encodeCursor(c searchCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (searchCursor, error) {
	var c searchCursor
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// knowledgeTypes lists the asset types of every knowledge kind, as search filters take them.
func knowledgeTypes() []string {
	var out []string
	for _, k := range knowledge.Kinds() {
		out = append(out, k.Type)
	}
	return out
}

// resolveOwnerFilter turns a team name, username, email, ID or "me" into the owner ID search filters take.
func (tc *ToolContext) resolveOwnerFilter(ctx context.Context, ref string) (string, string, bool) {
	ref = strings.TrimSpace(ref)
	if strings.EqualFold(ref, "me") && tc.principal != nil {
		return tc.principal.ID(), tc.principal.DisplayName(), true
	}
	if tc.teamService != nil {
		if _, err := uuid.Parse(ref); err == nil {
			if t, err := tc.teamService.GetTeam(ctx, ref); err == nil && t != nil {
				return t.ID, t.Name, true
			}
		}
		if t, err := tc.teamService.GetTeamByName(ctx, ref); err == nil && t != nil {
			return t.ID, t.Name, true
		}
	}
	if tc.userService != nil {
		if _, err := uuid.Parse(ref); err == nil {
			if u, err := tc.userService.Get(ctx, ref); err == nil && u != nil {
				return u.ID, u.Name, true
			}
		}
		if u, err := tc.userService.GetUserByUsername(ctx, strings.TrimPrefix(ref, "@")); err == nil && u != nil {
			return u.ID, u.Name, true
		}
		// Users have no separate email field; many sign in with their email as the username, so a listing by the address finds them.
		if strings.Contains(ref, "@") {
			if users, _, err := tc.userService.List(ctx, user.Filter{Query: ref, Limit: 5}); err == nil {
				for _, u := range users {
					if strings.EqualFold(u.Username, ref) {
						return u.ID, u.Name, true
					}
				}
			}
		}
	}
	return "", "", false
}

func (tc *ToolContext) search(ctx context.Context, _ *mcpsdk.CallToolRequest, in SearchInput) (*mcpsdk.CallToolResult, SearchOutput, error) {
	offset := 0
	if in.Cursor != "" {
		c, err := decodeCursor(in.Cursor)
		if err != nil {
			return toolError(codeInvalidArgument, "That cursor isn't one search returned. Send the search again without it."), SearchOutput{}, nil
		}
		offset, in = c.Offset, c.Input
	}
	countsOnly := in.Limit != nil && *in.Limit == 0
	limitIn := 0
	if in.Limit != nil {
		limitIn = *in.Limit
	}
	limit, _ := pageBounds(limitIn, 0, 10, 50)

	kinds, bad := normaliseSearchKinds(in.Kinds)
	if len(bad) > 0 {
		return toolError(codeInvalidArgument, fmt.Sprintf("Unknown kinds %s. Use asset, knowledge, term, data_product or team.", strings.Join(bad, ", "))), SearchOutput{}, nil
	}
	filter := search.Filter{Query: strings.TrimSpace(in.Query), AssetTypes: in.Types, Providers: in.Providers, Tags: in.Tags, Limit: limit, Offset: offset}
	textSearch := filter.Query != ""
	switch {
	case countsOnly && textSearch:
		// Word searches aren't counted by the search service, so read a page to count what can be seen.
		filter.Limit, filter.Offset = 50, 0
	case countsOnly:
		filter.Limit, filter.Offset = 1, 0
	case textSearch:
		// One more than asked for tells whether another page exists.
		filter.Limit = limit + 1
	}
	// Asset filters only make sense for assets, so they narrow the search to them.
	assetOnly := len(in.Types)+len(in.Providers)+len(in.Tags) > 0
	for _, k := range kinds {
		switch k {
		case kindAsset, kindKnowledge:
			filter.Types = appendType(filter.Types, search.ResultTypeAsset)
		case kindTerm:
			if !assetOnly {
				filter.Types = appendType(filter.Types, search.ResultTypeGlossary)
			}
		case kindDataProduct:
			if !assetOnly {
				filter.Types = appendType(filter.Types, search.ResultTypeDataProduct)
			}
		case kindTeam:
			if !assetOnly {
				filter.Types = appendType(filter.Types, search.ResultTypeTeam)
			}
		}
	}
	if len(kinds) == 1 && kinds[0] == kindKnowledge && len(in.Types) == 0 {
		filter.AssetTypes = knowledgeTypes()
	}
	if assetOnly && len(filter.Types) == 0 {
		filter.Types = []search.ResultType{search.ResultTypeAsset}
	}
	ownerName := ""
	if in.Owner != "" {
		id, name, ok := tc.resolveOwnerFilter(ctx, in.Owner)
		if !ok {
			return toolError(codeNotFound, fmt.Sprintf("No team or person %q. Give a team name, a username or an email.", in.Owner), step{"list teams", "search", map[string]any{"kinds": []string{kindTeam}}}), SearchOutput{}, nil
		}
		filter.Owners, ownerName = []string{id}, name
	}

	if tc.searchService == nil {
		return unavailable("search"), SearchOutput{}, nil
	}
	resp, err := tc.searchService.Search(ctx, filter)
	if err != nil {
		log.Warn().Err(err).Msg("MCP search failed")
		return unavailable("search"), SearchOutput{}, nil
	}

	results := resp.Results
	out := SearchOutput{Total: resp.Total, Results: []SearchHit{}}
	if textSearch {
		if countsOnly {
			out.Total, out.HasMore = len(results), len(results) == filter.Limit
			out.Counts = countKinds(tc, results)
		} else {
			out.HasMore = len(results) > limit
			if out.HasMore {
				results = results[:limit]
			}
			out.Total = offset + len(results)
		}
	} else {
		out.HasMore = resp.Total > offset+len(results)
	}
	if resp.Facets != nil && !textSearch {
		out.Counts = map[string]int{}
		for t, n := range resp.Facets.Types {
			out.Counts["kind:"+string(t)] = n
		}
		for _, f := range resp.Facets.AssetTypes {
			out.Counts["type:"+f.Value] = f.Count
		}
		for _, f := range resp.Facets.Providers {
			out.Counts["provider:"+f.Value] = f.Count
		}
	}
	for _, r := range results {
		out.Results = append(out.Results, tc.hit(r))
	}
	if !countsOnly && out.HasMore && len(results) > 0 {
		next := in
		out.NextCursor = encodeCursor(searchCursor{Offset: offset + len(results), Input: next})
	}
	if countsOnly {
		out.Results = []SearchHit{}
	}
	var hints []step
	if out.Total == 0 && in.Query != "" {
		hints = tc.tagHints(ctx, in)
	}
	text := renderSearch(in, &out, offset, countsOnly, ownerName, tc.knowledgeService != nil, hints...)
	return textResult(text), out, nil
}

// tagHints suggests a tag filter when a word search found nothing but a word is one of the catalog's tags, since tags aren't part of the searched text.
func (tc *ToolContext) tagHints(ctx context.Context, in SearchInput) []step {
	if tc.assetService == nil {
		return nil
	}
	summary, err := tc.assetService.Summary(ctx)
	if err != nil || summary == nil {
		return nil
	}
	var hints []step
	for _, w := range strings.Fields(in.Query) {
		for tag := range summary.Tags {
			if strings.EqualFold(tag, w) {
				hints = append(hints, step{"assets tagged " + tag, "search", map[string]any{"tags": []string{tag}}})
			}
		}
	}
	return hints
}

// countKinds counts a page of word-search results by kind and type, since the search service counts only filter-only searches.
func countKinds(tc *ToolContext, results []*search.Result) map[string]int {
	counts := map[string]int{}
	for _, r := range results {
		h := tc.hit(r)
		counts["kind:"+h.Kind]++
		if h.Type != "" && (h.Kind == kindAsset || h.Kind == kindKnowledge) {
			counts["type:"+h.Type]++
		}
	}
	return counts
}

func appendType(ts []search.ResultType, t search.ResultType) []search.ResultType {
	for _, x := range ts {
		if x == t {
			return ts
		}
	}
	return append(ts, t)
}

func normaliseSearchKinds(in []string) ([]string, []string) {
	var out, bad []string
	for _, k := range in {
		k = strings.ToLower(strings.TrimSpace(k))
		k = strings.TrimSuffix(k, "s")
		switch k {
		case "asset", "data", "table":
			out = append(out, kindAsset)
		case "knowledge", "entry", "entrie", "policie", "policy", "decision", "runbook", "doc":
			out = append(out, kindKnowledge)
		case "term", "glossary", "glossary_term", "definition":
			out = append(out, kindTerm)
		case "data_product", "product", "dataproduct", "data product":
			out = append(out, kindDataProduct)
		case "team":
			out = append(out, kindTeam)
		case "":
		default:
			bad = append(bad, k)
		}
	}
	return out, bad
}

// hit turns a search result into what search returns, with the id fetch takes: the MRN for an asset, the ID for anything else.
func (tc *ToolContext) hit(r *search.Result) SearchHit {
	h := SearchHit{Title: r.Name, Owners: []string{}}
	switch r.Type {
	case search.ResultTypeAsset:
		h.Kind = kindAsset
		if t, ok := r.Metadata["type"].(string); ok {
			h.Type = t
			if _, isEntry := knowledge.KindOfType(t); isEntry {
				h.Kind = kindKnowledge
			}
		}
		h.ID = r.ID
		if m, ok := r.Metadata["mrn"].(string); ok && m != "" && h.Kind == kindAsset {
			h.ID = m
		}
		a := &asset.Asset{ID: r.ID, Type: h.Type, Name: &r.Name}
		if m, ok := r.Metadata["mrn"].(string); ok {
			a.MRN = &m
		}
		h.URL = assetURL(tc.rootURL, a)
		if tags, ok := r.Metadata["tags"].([]interface{}); ok {
			for _, t := range tags {
				if s, ok := t.(string); ok && strings.EqualFold(s, "deprecated") {
					h.Flags = append(h.Flags, "deprecated")
				}
			}
		}
		if stub, ok := r.Metadata["is_stub"].(bool); ok && stub {
			h.Flags = append(h.Flags, "placeholder")
		}
	case search.ResultTypeGlossary:
		h.Kind, h.Type, h.ID, h.URL = kindTerm, "Glossary term", r.ID, termURL(tc.rootURL, r.ID)
	case search.ResultTypeDataProduct:
		h.Kind, h.Type, h.ID, h.URL = kindDataProduct, "Data product", r.ID, productURL(tc.rootURL, r.ID)
	case search.ResultTypeTeam:
		h.Kind, h.Type, h.ID, h.URL = kindTeam, "Team", r.ID, teamURL(tc.rootURL, r.ID)
	}
	if r.Description != nil {
		h.Snippet = truncate(oneLine(*r.Description), maxListSummary)
	}
	for _, o := range r.Owners {
		h.Owners = append(h.Owners, o.Name)
	}
	sort.Strings(h.Owners)
	if r.UpdatedAt != nil {
		h.UpdatedAt = r.UpdatedAt.Format("2006-01-02")
	}
	if r.Stale != nil && *r.Stale {
		h.Stale = true
	}
	if (h.Kind == kindAsset || h.Kind == kindKnowledge || h.Kind == kindTerm || h.Kind == kindDataProduct) && len(h.Owners) == 0 {
		h.Flags = append(h.Flags, "unowned")
	}
	return h
}

func renderSearch(in SearchInput, out *SearchOutput, offset int, countsOnly bool, ownerName string, haveKnowledge bool, hints ...step) string {
	var b strings.Builder
	var parts []string
	if in.Query != "" {
		parts = append(parts, fmt.Sprintf("%q", in.Query))
	}
	if len(in.Kinds) > 0 {
		parts = append(parts, "kind "+strings.Join(in.Kinds, " or "))
	}
	if len(in.Types) > 0 {
		parts = append(parts, "type "+strings.Join(in.Types, " or "))
	}
	if len(in.Providers) > 0 {
		parts = append(parts, "provider "+strings.Join(in.Providers, " or "))
	}
	if len(in.Tags) > 0 {
		parts = append(parts, "tagged "+strings.Join(in.Tags, " and "))
	}
	what := "everything"
	if len(parts) > 0 {
		what = strings.Join(parts, ", ")
	}
	if ownerName != "" {
		what += " owned by " + ownerName
	}
	atLeast := ""
	if out.HasMore && in.Query != "" {
		atLeast = "at least "
	}
	switch {
	case out.Total == 0:
		fmt.Fprintf(&b, "Nothing in Marmot matches %s.", what)
	case countsOnly:
		fmt.Fprintf(&b, "%s%d results match %s.", atLeast, out.Total, what)
	case in.Query != "":
		fmt.Fprintf(&b, "Results %d to %d for %s", offset+1, offset+len(out.Results), what)
		if out.HasMore {
			b.WriteString("; there are more.")
		} else {
			b.WriteString("; that's all of them.")
		}
	default:
		fmt.Fprintf(&b, "%d results match %s; showing %d to %d.", out.Total, what, offset+1, offset+len(out.Results))
	}
	for _, h := range out.Results {
		fmt.Fprintf(&b, "\n- %s (%s", h.Title, h.Kind)
		if h.Type != "" && h.Kind != kindTerm && h.Kind != kindDataProduct && h.Kind != kindTeam {
			fmt.Fprintf(&b, ", %s", h.Type)
		}
		b.WriteString(")")
		if len(h.Flags) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(h.Flags, ", "))
		}
		if h.Stale {
			b.WriteString(" [stale: its pipeline has stopped refreshing it]")
		}
		fmt.Fprintf(&b, "\n  id: %s", h.ID)
		if len(h.Owners) > 0 {
			fmt.Fprintf(&b, "\n  owners: %s", strings.Join(h.Owners, ", "))
		}
		if h.UpdatedAt != "" {
			fmt.Fprintf(&b, "; updated %s", h.UpdatedAt)
		}
		if h.Snippet != "" {
			fmt.Fprintf(&b, "\n  %s", h.Snippet)
		}
		if h.URL != "" {
			fmt.Fprintf(&b, "\n  url: %s", h.URL)
		}
	}
	if len(out.Counts) > 0 && (countsOnly || out.HasMore) {
		b.WriteString("\n\nCounts:")
		for _, k := range byCount(out.Counts) {
			fmt.Fprintf(&b, " %s %d,", k, out.Counts[k])
		}
		s := strings.TrimSuffix(b.String(), ",")
		b.Reset()
		b.WriteString(s)
	}
	next := append([]step(nil), hints...)
	if len(out.Results) > 0 {
		var ids []string
		for i, h := range out.Results {
			if i == 3 {
				break
			}
			ids = append(ids, h.ID)
		}
		if len(ids) == 1 {
			next = append(next, step{"read it in full", "fetch", map[string]any{"id": ids[0]}})
		} else {
			next = append(next, step{"read the top results in full", "fetch", map[string]any{"ids": ids}})
		}
	}
	if out.NextCursor != "" {
		next = append(next, step{"next page", "search", map[string]any{"cursor": out.NextCursor}})
	}
	if out.Total == 0 && in.Query != "" && len(in.Tags)+len(in.Types)+len(in.Providers) > 0 {
		next = append(next, step{"the filters without the words", "search", map[string]any{"tags": in.Tags, "types": in.Types, "providers": in.Providers}})
	}
	if out.Total == 0 && in.Query != "" {
		words := strings.Fields(in.Query)
		if len(words) > 1 {
			next = append(next, step{"fewer words", "search", map[string]any{"query": words[0]}})
		}
		if haveKnowledge {
			next = append(next, step{"written knowledge, matching any word", "search_knowledge", map[string]any{"query": in.Query}})
		}
	}
	if out.Total == 0 {
		out.Note = "Nothing matched. Words are matched against names and descriptions, not tags; try fewer words, or a tag or type filter on its own."
	} else if out.HasMore && in.Query != "" {
		out.Note = "Word searches aren't counted; there are more results than shown."
	}
	out.Next = nextLines(next...)
	writeNext(&b, next...)
	return b.String()
}

func (tc *ToolContext) fetch(ctx context.Context, req *mcpsdk.CallToolRequest, in FetchInput) (*mcpsdk.CallToolResult, FetchOutput, error) {
	ids := in.IDs
	if strings.TrimSpace(in.ID) != "" {
		ids = append([]string{in.ID}, ids...)
	}
	if len(ids) == 0 {
		return toolError(codeInvalidArgument, "Give id: an id from search, an MRN, a Marmot URL or an exact name."), FetchOutput{}, nil
	}
	if len(ids) > 10 {
		return toolError(codeInvalidArgument, "Fetch reads at most 10 ids at once."), FetchOutput{}, nil
	}
	out := FetchOutput{Items: []FetchedItem{}}
	var texts []string
	var candidates []string
	failed := false
	budget := cardBudget
	if len(ids) > 1 {
		// Several items share the budget, so a batch of ten stays as readable as one.
		budget = cardBudget * 3 / 2 / len(ids)
	}
	for _, ref := range ids {
		item, hint, err := tc.fetchOne(ctx, req, strings.TrimSpace(ref), in)
		switch {
		case err != nil:
			log.Warn().Err(err).Str("ref", ref).Msg("MCP fetch failed")
			failed = true
			out.NotFound = append(out.NotFound, ref)
		case item == nil:
			out.NotFound = append(out.NotFound, ref)
			candidates = append(candidates, hint...)
		default:
			if len([]rune(item.Text)) > budget {
				item.Text = truncate(item.Text, budget) + fmt.Sprintf("\n[Cut to fit a batch. Fetch %s on its own for all of it.]", item.ID)
			}
			out.Items = append(out.Items, *item)
			texts = append(texts, item.Text)
		}
	}
	if len(out.Items) == 0 {
		if failed {
			return unavailable("read that"), out, nil
		}
		var next []step
		for _, c := range candidates {
			next = append(next, step{"did you mean", "fetch", map[string]any{"id": c}})
		}
		if len(next) == 0 {
			next = append(next, step{"search for it", "search", map[string]any{"query": ids[0]}})
		}
		return toolError(codeNotFound, fmt.Sprintf("Nothing in Marmot matches %s.", strings.Join(quoteAll(out.NotFound), ", ")), next...), out, nil
	}
	text := strings.Join(texts, "\n\n---\n\n")
	if len(out.NotFound) > 0 {
		text += fmt.Sprintf("\n\nNot found: %s.", strings.Join(quoteAll(out.NotFound), ", "))
	}
	return textResult(text), out, nil
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strconv.Quote(s)
	}
	return out
}

// fetchOne reads one identifier of any kind. It returns nil with candidate ids when nothing matches exactly.
func (tc *ToolContext) fetchOne(ctx context.Context, req *mcpsdk.CallToolRequest, ref string, in FetchInput) (*FetchedItem, []string, error) {
	ref = fromMarmotURL(ref)
	kindHint := ""
	if i := strings.Index(ref, ":"); i > 0 && !strings.Contains(ref, "://") {
		switch ref[:i] {
		case kindTerm, kindDataProduct, kindTeam, kindAsset, kindKnowledge:
			kindHint, ref = ref[:i], ref[i+1:]
		}
	}
	_, idErr := uuid.Parse(ref)
	isUUID := idErr == nil

	if kindHint == "" || kindHint == kindAsset || kindHint == kindKnowledge {
		if isUUID || strings.Contains(ref, "://") || kindHint != "" {
			a, _, err := tc.findAsset(ctx, ref)
			if err != nil {
				return nil, nil, err
			}
			if a != nil {
				return tc.assetItem(ctx, req, a, in), nil, nil
			}
		}
	}
	if isUUID && (kindHint == "" || kindHint == kindTerm) && tc.glossaryService != nil {
		if t, err := tc.glossaryService.Get(ctx, ref); err == nil && t != nil {
			return tc.termItem(ctx, t), nil, nil
		}
	}
	if isUUID && (kindHint == "" || kindHint == kindDataProduct) && tc.dataProductService != nil {
		if p, err := tc.dataProductService.Get(ctx, ref); err == nil && p != nil {
			return tc.productItem(ctx, p), nil, nil
		}
	}
	if isUUID && (kindHint == "" || kindHint == kindTeam) && tc.teamService != nil {
		if t, err := tc.teamService.GetTeam(ctx, ref); err == nil && t != nil {
			return tc.teamItem(ctx, t), nil, nil
		}
	}
	if isUUID || strings.Contains(ref, "://") {
		return nil, nil, nil
	}

	// A name: an exact asset, term, product or team name, in that order, since assets are what people ask about most.
	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	if a != nil {
		return tc.assetItem(ctx, req, a, in), nil, nil
	}
	if tc.glossaryService != nil {
		if res, err := tc.glossaryService.Search(ctx, glossary.SearchFilter{Query: ref, Limit: 10}); err == nil {
			for _, t := range res.Terms {
				if strings.EqualFold(t.Name, ref) {
					return tc.termItem(ctx, t), nil, nil
				}
			}
		}
	}
	if tc.dataProductService != nil {
		if res, err := tc.dataProductService.Search(ctx, dataproduct.SearchFilter{Query: ref, Limit: 10}); err == nil {
			for _, p := range res.DataProducts {
				if strings.EqualFold(p.Name, ref) {
					return tc.productItem(ctx, p), nil, nil
				}
			}
		}
	}
	if tc.teamService != nil {
		if t, err := tc.teamService.GetTeamByName(ctx, ref); err == nil && t != nil {
			return tc.teamItem(ctx, t), nil, nil
		}
	}
	var hints []string
	for _, c := range candidates {
		hints = append(hints, idFor(c))
	}
	return nil, hints, nil
}

// fromMarmotURL reads the identifier out of a link to a Marmot page, so an agent can fetch a URL a person pasted.
func fromMarmotURL(ref string) string {
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := range parts {
		if p, err := url.PathUnescape(parts[i]); err == nil {
			parts[i] = p
		}
	}
	switch {
	case len(parts) == 4 && parts[0] == "discover":
		return fmt.Sprintf("mrn://asset/%s/%s/%s", parts[1], parts[2], parts[3])
	case len(parts) == 2 && parts[0] == "knowledge":
		return kindKnowledge + ":" + parts[1]
	case len(parts) == 2 && parts[0] == "glossary":
		return kindTerm + ":" + parts[1]
	case len(parts) == 2 && parts[0] == "products":
		return kindDataProduct + ":" + parts[1]
	case len(parts) == 2 && parts[0] == "teams":
		return kindTeam + ":" + parts[1]
	}
	return ref
}

func (tc *ToolContext) assetItem(ctx context.Context, req *mcpsdk.CallToolRequest, a *asset.Asset, in FetchInput) *FetchedItem {
	item := &FetchedItem{ID: idFor(a), Kind: kindAsset, Title: deref(a.Name), URL: assetURL(tc.rootURL, a), Metadata: map[string]string{"type": a.Type}}
	if len(a.Providers) > 0 {
		item.Metadata["provider"] = a.Providers[0]
	}
	if owners := tc.ownerList(ctx, a.ID); owners != "" {
		item.Metadata["owners"] = owners
	}
	if !a.UpdatedAt.IsZero() {
		item.Metadata["updated_at"] = a.UpdatedAt.Format(time.RFC3339)
	}
	if _, isEntry := knowledge.KindOfType(a.Type); isEntry && tc.knowledgeService != nil {
		item.Kind, item.ID = kindKnowledge, a.ID
		res, _, _ := tc.readKnowledge(ctx, req, ReadKnowledgeInput{ID: a.ID, Section: in.Section, Offset: in.Offset})
		if res != nil && len(res.Content) > 0 {
			if t, ok := res.Content[0].(*mcpsdk.TextContent); ok {
				item.Text = t.Text
				return item
			}
		}
	}
	tc.recordLookup(ctx, lookups.CategoryAssetDetail)
	item.Text = tc.assetCard(ctx, a)
	if tc.stale(ctx, a.ID) {
		item.Metadata["stale"] = "true"
	}
	return item
}

func (tc *ToolContext) termItem(ctx context.Context, t *glossary.GlossaryTerm) *FetchedItem {
	res, _, _ := tc.getTerm(ctx, t.ID, LookupTermInput{})
	return &FetchedItem{ID: t.ID, Kind: kindTerm, Title: t.Name, URL: termURL(tc.rootURL, t.ID), Text: resultString(res), Metadata: map[string]string{"definition": truncate(t.Definition, 500)}}
}

func (tc *ToolContext) productItem(ctx context.Context, p *dataproduct.DataProduct) *FetchedItem {
	res, _, _ := tc.productCard(ctx, p, 50, 0)
	return &FetchedItem{ID: p.ID, Kind: kindDataProduct, Title: p.Name, URL: productURL(tc.rootURL, p.ID), Text: resultString(res)}
}

func (tc *ToolContext) teamItem(ctx context.Context, t *Team) *FetchedItem {
	res, _, _ := tc.teamCard(ctx, ExploreTeamsInput{TeamID: t.ID})
	return &FetchedItem{ID: t.ID, Kind: kindTeam, Title: t.Name, URL: teamURL(tc.rootURL, t.ID), Text: resultString(res)}
}

func resultString(res *mcpsdk.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if t, ok := res.Content[0].(*mcpsdk.TextContent); ok {
		return t.Text
	}
	return ""
}

func (tc *ToolContext) getLineage(ctx context.Context, _ *mcpsdk.CallToolRequest, in GetLineageInput) (*mcpsdk.CallToolResult, GetLineageOutput, error) {
	ref := strings.TrimSpace(in.ID)
	if ref == "" {
		return toolError(codeInvalidArgument, "Give id: the asset's MRN, ID or exact name."), GetLineageOutput{}, nil
	}
	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	if direction == "" {
		direction = "both"
	}
	if direction != "upstream" && direction != "downstream" && direction != "both" {
		return toolError(codeInvalidArgument, fmt.Sprintf("direction %q isn't one of upstream, downstream or both.", in.Direction),
			step{"what depends on it", "get_lineage", map[string]any{"id": ref, "direction": "downstream"}}), GetLineageOutput{}, nil
	}
	depth := in.Depth
	if depth <= 0 {
		depth = 3
	}
	if depth > 10 {
		depth = 10
	}
	limit, _ := pageBounds(in.Limit, 0, maxLineageInTrace, 200)

	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Msg("MCP get_lineage lookup failed")
		return unavailable("look that asset up"), GetLineageOutput{}, nil
	}
	if a == nil {
		return assetNotFound("get_lineage", "id", ref, candidates), GetLineageOutput{}, nil
	}
	resp, err := tc.lineageService.GetAssetLineage(ctx, a.ID, depth, direction)
	if err != nil {
		log.Warn().Err(err).Msg("MCP get_lineage failed")
		return unavailable("read the lineage"), GetLineageOutput{}, nil
	}
	tc.recordLookup(ctx, lookups.CategoryLineage)
	out := lineageOutput(resp, a, limit)

	var b strings.Builder
	fmt.Fprintf(&b, "Lineage of %s (%s), %s, up to %d hops\nid: %s", deref(a.Name), a.Type, direction, depth, idFor(a))
	if section := lineageSection(resp, limit, a.ID); section == "" {
		out.Note = "No lineage is recorded in that direction. That means none has been catalogued, not that nothing depends on it."
		b.WriteString("\n\n" + out.Note)
	} else {
		b.WriteString("\n\n" + section)
	}
	var next []step
	if len(out.Nodes) > 0 && depth < 10 {
		next = append(next, step{"follow it further", "get_lineage", map[string]any{"id": idFor(a), "direction": direction, "depth": min(depth+3, 10)}})
	}
	if len(out.Nodes) > 0 {
		var ids []string
		for i, n := range out.Nodes {
			if i == 3 {
				break
			}
			ids = append(ids, n.ID)
		}
		next = append(next, step{"owners and details of the nearest", "fetch", map[string]any{"ids": ids}})
	}
	if out.Omitted > 0 {
		out.Note = fmt.Sprintf("%d more assets not shown; raise limit or narrow direction or depth.", out.Omitted)
	}
	out.Next = nextLines(next...)
	writeNext(&b, next...)
	return textResult(b.String()), out, nil
}

// lineageOutput is lineage as data: nodes nearest first with the edges between the nodes shown.
func lineageOutput(resp *lineage.LineageResponse, root *asset.Asset, limit int) GetLineageOutput {
	out := GetLineageOutput{Root: idFor(root), Nodes: []LineageNodeOut{}, Edges: []LineageEdgeOut{}}
	var nodes []lineage.LineageNode
	for _, n := range resp.Nodes {
		if n.Asset != nil && n.Asset.ID != root.ID && n.Depth != 0 {
			nodes = append(nodes, n)
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if abs(nodes[i].Depth) != abs(nodes[j].Depth) {
			return abs(nodes[i].Depth) < abs(nodes[j].Depth)
		}
		return deref(nodes[i].Asset.Name) < deref(nodes[j].Asset.Name)
	})
	if len(nodes) > limit {
		out.Omitted = len(nodes) - limit
		nodes = nodes[:limit]
	}
	shown := map[string]bool{idFor(root): true}
	for _, n := range nodes {
		dir := "downstream"
		if n.Depth < 0 {
			dir = "upstream"
		}
		out.Nodes = append(out.Nodes, LineageNodeOut{ID: idFor(n.Asset), Name: deref(n.Asset.Name), Type: n.Asset.Type, Direction: dir, Hops: abs(n.Depth), LastRunStatus: n.LastRunStatus, Placeholder: n.Asset.IsStub})
		shown[idFor(n.Asset)] = true
	}
	for _, e := range resp.Edges {
		if shown[e.Source] && shown[e.Target] {
			out.Edges = append(out.Edges, LineageEdgeOut{Source: e.Source, Target: e.Target, Type: e.Type})
		}
	}
	sort.SliceStable(out.Edges, func(i, j int) bool {
		if out.Edges[i].Source != out.Edges[j].Source {
			return out.Edges[i].Source < out.Edges[j].Source
		}
		return out.Edges[i].Target < out.Edges[j].Target
	})
	return out
}

// StaleChecker reports whether an asset's pipeline has stopped refreshing it.
type StaleChecker interface {
	IsStale(ctx context.Context, id string) (bool, error)
}

func (tc *ToolContext) stale(ctx context.Context, id string) bool {
	if tc.signals == nil {
		return false
	}
	s, err := tc.signals.IsStale(ctx, id)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Debug().Err(err).Msg("MCP staleness check failed")
	}
	return s
}
