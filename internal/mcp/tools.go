package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
	"github.com/marmotdata/marmot/pkg/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// ToolContext holds the services the tools use and who is calling.
type ToolContext struct {
	assetService       asset.Service
	glossaryService    GlossaryService
	userService        user.Service
	teamService        TeamService
	dataProductService DataProductService
	lineageService     lineage.Service
	searchService      search.Service
	queryService       dataquery.Service
	principal          auth.Principal
	config             *config.Config
	lookups            lookups.Recorder
	knowledgeService   KnowledgeService
	docsService        DocsService
	previewKey         []byte
	// rootURL is the address people open Marmot at, from config or else from the request, so links work without a configured root URL.
	rootURL string
}

// recordLookup counts a lookup with source=mcp; safe to call without a recorder.
func (tc *ToolContext) recordLookup(ctx context.Context, cat lookups.Category) {
	if tc.lookups == nil {
		return
	}
	tc.lookups.Record(lookups.WithSource(ctx, lookups.SourceMCP), cat)
}

type DiscoverDataInput struct {
	Query           string                `json:"query,omitempty" jsonschema:"Words to match in asset names, descriptions and summaries, such as 'mrr' or 'orders'"`
	ID              string                `json:"id,omitempty" jsonschema:"An asset's MRN, ID or exact name. Returns that one asset in full: owners, documentation pages, schema, lineage, data products and the knowledge that applies to it"`
	MRN             string                `json:"mrn,omitempty" jsonschema:"Same as id, kept for older callers"`
	Types           []string              `json:"types,omitempty" jsonschema:"Only these asset types, such as Table, Topic or Dashboard. Close spellings match"`
	Providers       []string              `json:"providers,omitempty" jsonschema:"Only these providers, such as PostgreSQL, Kafka or BigQuery. Close spellings match"`
	Tags            []string              `json:"tags,omitempty" jsonschema:"Only assets carrying all of these tags"`
	MetadataFilters []MetadataFilterInput `json:"metadata_filters,omitempty" jsonschema:"Only assets whose metadata matches, such as {key: partitions, operator: '>', value: 5}"`
	Limit           *int                  `json:"limit,omitempty" jsonschema:"Assets to list, 1 to 100. Default 20. Send 0 for counts by type, provider and tag only"`
	Offset          int                   `json:"offset,omitempty" jsonschema:"Results to skip, for the next page. Default 0"`
}

type MetadataFilterInput struct {
	Key      string `json:"key" jsonschema:"Metadata key, such as partitions or schedule"`
	Operator string `json:"operator" jsonschema:"One of =, >, <, >=, <= or contains"`
	Value    any    `json:"value" jsonschema:"The value to compare with"`
}

type FindOwnershipInput struct {
	AssetID              string `json:"asset_id,omitempty" jsonschema:"An asset's MRN, ID or exact name. Returns who owns it and how to reach them"`
	UserID               string `json:"user_id,omitempty" jsonschema:"A user's ID. Returns what they own"`
	Username             string `json:"username,omitempty" jsonschema:"A username. Returns what they own"`
	TeamID               string `json:"team_id,omitempty" jsonschema:"A team's ID. Returns what it owns"`
	TeamName             string `json:"team_name,omitempty" jsonschema:"A team's name, such as 'Data platform'. Returns what it owns"`
	IncludeAssets        *bool  `json:"include_assets,omitempty" jsonschema:"List owned assets. Default true"`
	IncludeGlossaryTerms *bool  `json:"include_glossary_terms,omitempty" jsonschema:"List owned glossary terms. Default true"`
	Limit                int    `json:"limit,omitempty" jsonschema:"Items to list of each kind, 1 to 100. Default 50"`
	Offset               int    `json:"offset,omitempty" jsonschema:"Items to skip, for the next page. Default 0"`
}

type LookupTermInput struct {
	Query        string `json:"query,omitempty" jsonschema:"Words to match in term names and definitions, such as 'active customer'"`
	TermID       string `json:"term_id,omitempty" jsonschema:"A term's ID or exact name. Returns its full definition, owners, hierarchy and linked assets"`
	ParentTermID string `json:"parent_term_id,omitempty" jsonschema:"List the terms directly under this term"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Terms or linked assets to list, 1 to 100. Default 20"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Items to skip, for the next page. Default 0"`
}

type ExploreDataProductsInput struct {
	Query  string   `json:"query,omitempty" jsonschema:"Words to match in data product names and descriptions"`
	ID     string   `json:"id,omitempty" jsonschema:"A data product's ID. Returns it with its owners and member assets"`
	Name   string   `json:"name,omitempty" jsonschema:"A data product's exact name, such as 'Revenue'"`
	Tags   []string `json:"tags,omitempty" jsonschema:"Only data products with these tags"`
	Limit  int      `json:"limit,omitempty" jsonschema:"Products or member assets to list, 1 to 100. Default 20"`
	Offset int      `json:"offset,omitempty" jsonschema:"Items to skip, for the next page. Default 0"`
}

type ExploreTeamsInput struct {
	TeamID   string `json:"team_id,omitempty" jsonschema:"A team's ID. Returns the team and its members"`
	TeamName string `json:"team_name,omitempty" jsonschema:"A team's name. Returns the team and its members"`
	UserID   string `json:"user_id,omitempty" jsonschema:"A user's ID. Returns the teams they belong to"`
	Username string `json:"username,omitempty" jsonschema:"A username. Returns the teams they belong to"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Teams to list, 1 to 100. Default 20"`
	Offset   int    `json:"offset,omitempty" jsonschema:"Teams to skip, for the next page. Default 0"`
}

type TraceLineageInput struct {
	ID        string `json:"id,omitempty" jsonschema:"The asset's MRN, ID or exact name"`
	AssetID   string `json:"asset_id,omitempty" jsonschema:"Same as id, kept for older callers"`
	MRN       string `json:"mrn,omitempty" jsonschema:"Same as id, kept for older callers"`
	Direction string `json:"direction,omitempty" jsonschema:"upstream (what feeds it), downstream (what depends on it) or both. Default both"`
	Depth     int    `json:"depth,omitempty" jsonschema:"Hops to follow, 1 to 10. Default 3"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Most assets to list, 1 to 200. Default 60, nearest first"`
}

// pageBounds clamps limit and offset to what the tools accept.
func pageBounds(limit, offset, def, max int) (int, int) {
	if limit <= 0 {
		limit = def
	}
	if limit > max {
		limit = max
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// findAsset resolves an MRN, ID or name to one asset, telling a missing asset apart from a failed lookup; with no single match it returns close candidates.
func (tc *ToolContext) findAsset(ctx context.Context, ref string) (*asset.Asset, []*asset.Asset, error) {
	ref = strings.TrimSpace(ref)
	notFound := func(err error) bool {
		return errors.Is(err, asset.ErrAssetNotFound) || errors.Is(err, asset.ErrNotFound)
	}
	if _, err := uuid.Parse(ref); err == nil {
		a, err := tc.assetService.Get(ctx, ref)
		if notFound(err) {
			return nil, nil, nil
		}
		return a, nil, err
	}
	if strings.Contains(ref, "://") {
		a, err := tc.assetService.GetByMRN(ctx, ref)
		if notFound(err) {
			return nil, nil, nil
		}
		return a, nil, err
	}
	return tc.resolveAsset(ctx, ref)
}

// assetNotFound explains a reference that matched nothing, offering the closest assets as real calls.
func assetNotFound(tool, param, ref string, candidates []*asset.Asset) *mcpsdk.CallToolResult {
	if len(candidates) == 0 {
		return toolError(codeNotFound, fmt.Sprintf("No asset matches %q.", ref),
			step{"search by words instead", "discover_data", map[string]any{"query": ref}})
	}
	var next []step
	for _, c := range candidates {
		next = append(next, step{fmt.Sprintf("%s (%s)", deref(c.Name), c.Type), tool, map[string]any{param: idFor(c)}})
	}
	return toolError(codeAmbiguous, fmt.Sprintf("%q matches more than one asset, or none exactly. Pick one:", ref), next...)
}

func (tc *ToolContext) discoverData(ctx context.Context, _ *mcpsdk.CallToolRequest, args DiscoverDataInput) (*mcpsdk.CallToolResult, any, error) {
	ref := args.ID
	if ref == "" {
		ref = args.MRN
	}
	if ref != "" {
		a, candidates, err := tc.findAsset(ctx, ref)
		if err != nil {
			log.Warn().Err(err).Str("ref", ref).Msg("MCP discover_data lookup failed")
			return unavailable("look that asset up"), nil, nil
		}
		if a == nil {
			return assetNotFound("discover_data", "id", ref, candidates), nil, nil
		}
		tc.recordLookup(ctx, lookups.CategoryAssetDetail)
		return textResult(tc.assetCard(ctx, a)), nil, nil
	}
	return tc.searchAssets(ctx, args)
}

type filterKind int

const (
	filterKindType filterKind = iota
	filterKindProvider
	filterKindTag
)

func (k filterKind) String() string {
	switch k {
	case filterKindType:
		return "type"
	case filterKindProvider:
		return "provider"
	default:
		return "tag"
	}
}

// canonicaliseFilterValues maps each supplied filter value to the matching value the catalog actually stores, so an agent's "postgres" reaches "PostgreSQL" and "kafka" reaches "Kafka" instead of matching nothing. A value with no match is passed through unchanged so a genuinely absent filter still returns an honest empty result. The second return lists every rewrite so callers can surface which filters actually ran.
func canonicaliseFilterValues(values []string, kind filterKind, summary *asset.AssetSummary) ([]string, []string) {
	if len(values) == 0 || summary == nil {
		return values, nil
	}

	var known []string
	switch kind {
	case filterKindType:
		for t := range summary.Types {
			known = append(known, t)
		}
	case filterKindProvider:
		for p := range summary.Providers {
			known = append(known, p)
		}
	case filterKindTag:
		for t := range summary.Tags {
			known = append(known, t)
		}
	}
	// A stable order makes the match deterministic when several known values would satisfy the same looser rule.
	sort.Strings(known)

	out := make([]string, len(values))
	var rewrites []string
	for i, v := range values {
		out[i] = v
		if canonical, ok := bestFilterMatch(v, known); ok {
			out[i] = canonical
			if canonical != v {
				rewrites = append(rewrites, fmt.Sprintf("%s %q matched catalog value %q", kind, v, canonical))
			}
		}
	}
	return out, rewrites
}

// filterRewriteNote tells the agent which filter values were canonicalised so results are never silently presented under a different filter than the one requested.
func filterRewriteNote(rewrites []string) string {
	if len(rewrites) == 0 {
		return ""
	}
	return "Filter values adjusted to the catalog's spelling: " + strings.Join(rewrites, "; ") + "."
}

// bestFilterMatch finds the catalog value that best corresponds to a supplied filter value, trying an exact case-insensitive match first, then a prefix relationship in either direction so both "postgres" against "PostgreSQL" and a plural "databases" against "Database" resolve, and finally a substring containment for values of three or more characters so a stray short value cannot latch onto an arbitrary catalog entry. It reports whether any rule matched.
func bestFilterMatch(value string, known []string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "", false
	}

	var prefixMatch, containsMatch string
	for _, k := range known {
		lk := strings.ToLower(k)
		if lk == v {
			return k, true
		}
		if prefixMatch == "" && (strings.HasPrefix(lk, v) || strings.HasPrefix(v, lk)) {
			prefixMatch = k
		}
		if len(v) >= 3 && containsMatch == "" && (strings.Contains(lk, v) || strings.Contains(v, lk)) {
			containsMatch = k
		}
	}
	if prefixMatch != "" {
		return prefixMatch, true
	}
	if containsMatch != "" {
		return containsMatch, true
	}
	return "", false
}

// metadataScanCap bounds how many assets are read to apply metadata filters, which the stores can't evaluate themselves.
const metadataScanCap = 1000

func (tc *ToolContext) searchAssets(ctx context.Context, args DiscoverDataInput) (*mcpsdk.CallToolResult, any, error) {
	query := strings.TrimSpace(args.Query)
	if query == "*" {
		query = ""
	}
	hasFilters := len(args.Types)+len(args.Providers)+len(args.Tags)+len(args.MetadataFilters) > 0

	// Counts only when asked for with limit 0, or for a bare call, which asks what the catalog holds.
	countsOnly := args.Limit != nil && *args.Limit == 0
	if args.Limit == nil && query == "" && !hasFilters && args.Offset == 0 {
		countsOnly = true
	}
	limitIn := 0
	if args.Limit != nil {
		limitIn = *args.Limit
	}
	limit, offset := pageBounds(limitIn, args.Offset, 20, 100)

	// Agents guess filter values from natural language ("postgres", "kafka") while assets store "PostgreSQL" and "Kafka", so match them to the catalog's own values first; a failed summary leaves them as given.
	var rewrites []string
	if len(args.Types)+len(args.Providers)+len(args.Tags) > 0 {
		if summary, err := tc.assetService.Summary(ctx); err == nil {
			var r []string
			args.Types, r = canonicaliseFilterValues(args.Types, filterKindType, summary)
			rewrites = append(rewrites, r...)
			args.Providers, r = canonicaliseFilterValues(args.Providers, filterKindProvider, summary)
			rewrites = append(rewrites, r...)
			args.Tags, r = canonicaliseFilterValues(args.Tags, filterKindTag, summary)
			rewrites = append(rewrites, r...)
		}
	}

	filter := asset.SearchFilter{Query: query, Types: args.Types, Providers: args.Providers, Tags: args.Tags, Limit: limit, Offset: offset, IncludeStubs: true}
	if countsOnly {
		filter.Limit, filter.Offset = 1, 0
	}
	if len(args.MetadataFilters) > 0 {
		filter.Limit, filter.Offset = metadataScanCap, 0
	}

	var (
		assets []*asset.Asset
		total  int
		facets asset.AvailableFilters
		err    error
	)
	// The search service ranks text queries better, but can't apply metadata filters or count facets the same way, so it is used for plain text lists only.
	if tc.searchService != nil && query != "" && len(args.MetadataFilters) == 0 && !countsOnly {
		var resp *search.Response
		resp, err = tc.searchService.Search(ctx, search.Filter{Query: query, Types: []search.ResultType{search.ResultTypeAsset}, AssetTypes: args.Types, Providers: args.Providers, Tags: args.Tags, Limit: limit, Offset: offset})
		if err == nil {
			assets, total = searchResultsToAssets(resp.Results), resp.Total
		}
	} else {
		assets, total, facets, err = tc.assetService.Search(ctx, filter, true)
	}
	if err != nil {
		if errors.Is(err, asset.ErrInvalidQuery) {
			return toolError(codeInvalidArgument, "Marmot couldn't read that query. Use plain words without quotes or brackets.", step{"try plain words", "discover_data", map[string]any{"query": strings.Trim(query, `"'()`)}}), nil, nil
		}
		log.Warn().Err(err).Msg("MCP discover_data search failed")
		return unavailable("search the catalog"), nil, nil
	}

	capped := false
	if len(args.MetadataFilters) > 0 {
		capped = total > metadataScanCap
		var kept []*asset.Asset
		for _, a := range assets {
			if matchesMetadataFilters(*a, args.MetadataFilters) {
				kept = append(kept, a)
			}
		}
		total = len(kept)
		if offset >= len(kept) {
			kept = nil
		} else {
			kept = kept[offset:]
		}
		if len(kept) > limit {
			kept = kept[:limit]
		}
		assets = kept
	}

	if countsOnly {
		return textResult(tc.catalogCounts(total, facets, args, rewrites)), nil, nil
	}

	var b strings.Builder
	desc := describeSearch(query, args)
	switch {
	case total == 0:
		fmt.Fprintf(&b, "No assets match %s.", desc)
	case len(assets) == 0:
		fmt.Fprintf(&b, "%d assets match %s, but none are left after offset %d.", total, desc, offset)
	default:
		fmt.Fprintf(&b, "%d assets match %s; showing %d to %d.", total, desc, offset+1, offset+len(assets))
		if capped {
			fmt.Fprintf(&b, " Metadata filters were applied to the first %d assets only; narrow with types, providers or a query to see the rest.", metadataScanCap)
		}
	}
	if note := filterRewriteNote(rewrites); note != "" {
		b.WriteString(" " + note)
	}
	for _, a := range assets {
		b.WriteString("\n" + assetLine(a, tc.rootURL))
	}
	if len(facets.Types)+len(facets.Providers) > 0 && total > len(assets) {
		fmt.Fprintf(&b, "\n\nAcross all %d matches: types %s; providers %s.", total, facetLine(facets.Types, 8), facetLine(facets.Providers, 8))
	}

	var next []step
	if len(assets) > 0 {
		next = append(next, step{"read one in full", "discover_data", map[string]any{"id": idFor(assets[0])}})
	}
	if total > offset+len(assets) && len(assets) > 0 {
		page := searchArgs(query, args)
		page["offset"] = offset + len(assets)
		page["limit"] = limit
		next = append(next, step{"next page", "discover_data", page})
	}
	if total == 0 {
		if hasFilters {
			next = append(next, step{"drop the filters", "discover_data", map[string]any{"query": query}})
		}
		next = append(next, step{"see what the catalog holds", "discover_data", map[string]any{"limit": 0}})
		if tc.knowledgeService != nil && query != "" {
			next = append(next, step{"look in written knowledge instead", "search_knowledge", map[string]any{"query": query}})
		}
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

// searchArgs rebuilds the arguments of a search so a next-page call keeps every filter.
func searchArgs(query string, args DiscoverDataInput) map[string]any {
	m := map[string]any{}
	if query != "" {
		m["query"] = query
	}
	if len(args.Types) > 0 {
		m["types"] = args.Types
	}
	if len(args.Providers) > 0 {
		m["providers"] = args.Providers
	}
	if len(args.Tags) > 0 {
		m["tags"] = args.Tags
	}
	if len(args.MetadataFilters) > 0 {
		m["metadata_filters"] = args.MetadataFilters
	}
	return m
}

func describeSearch(query string, args DiscoverDataInput) string {
	var parts []string
	if query != "" {
		parts = append(parts, fmt.Sprintf("%q", query))
	}
	if len(args.Types) > 0 {
		parts = append(parts, "type "+strings.Join(args.Types, " or "))
	}
	if len(args.Providers) > 0 {
		parts = append(parts, "provider "+strings.Join(args.Providers, " or "))
	}
	if len(args.Tags) > 0 {
		parts = append(parts, "tags "+strings.Join(args.Tags, " and "))
	}
	if len(args.MetadataFilters) > 0 {
		parts = append(parts, fmt.Sprintf("%d metadata filters", len(args.MetadataFilters)))
	}
	if len(parts) == 0 {
		return "everything"
	}
	return strings.Join(parts, ", ")
}

// catalogCounts answers "what is in the catalog" with totals by type, provider and tag, largest first.
func (tc *ToolContext) catalogCounts(total int, f asset.AvailableFilters, args DiscoverDataInput, rewrites []string) string {
	var b strings.Builder
	if len(args.Types)+len(args.Providers)+len(args.Tags) > 0 {
		fmt.Fprintf(&b, "%d assets match %s.", total, describeSearch("", args))
	} else {
		fmt.Fprintf(&b, "The catalog holds %d assets.", total)
	}
	if note := filterRewriteNote(rewrites); note != "" {
		b.WriteString(" " + note)
	}
	if len(f.Types) > 0 {
		b.WriteString("\nBy type: " + facetLine(f.Types, 0))
	}
	if len(f.Providers) > 0 {
		b.WriteString("\nBy provider: " + facetLine(f.Providers, 0))
	}
	if len(f.Tags) > 0 {
		b.WriteString("\nTop tags: " + facetLine(f.Tags, 15))
	}
	var next []step
	if types := byCount(f.Types); len(types) > 0 && len(args.Types) == 0 {
		next = append(next, step{"list the " + types[0] + " assets", "discover_data", map[string]any{"types": []string{types[0]}}})
	}
	if len(args.Types)+len(args.Providers)+len(args.Tags) > 0 {
		page := searchArgs("", args)
		page["limit"] = 20
		next = append(next, step{"list these assets", "discover_data", page})
	}
	if tc.knowledgeService != nil {
		next = append(next, step{"see written policies, decisions and runbooks", "search_knowledge", map[string]any{"query": "policy"}})
	}
	writeNext(&b, next...)
	return b.String()
}

// searchResultsToAssets converts search results into lightweight assets for listing.
func searchResultsToAssets(results []*search.Result) []*asset.Asset {
	assets := make([]*asset.Asset, 0, len(results))
	for _, r := range results {
		name := r.Name
		a := &asset.Asset{ID: r.ID, Name: &name, Description: r.Description}
		if t, ok := r.Metadata["type"].(string); ok {
			a.Type = t
		}
		if m, ok := r.Metadata["mrn"].(string); ok {
			a.MRN = &m
		}
		switch providers := r.Metadata["providers"].(type) {
		case []string:
			a.Providers = providers
		case []interface{}:
			for _, p := range providers {
				if s, ok := p.(string); ok {
					a.Providers = append(a.Providers, s)
				}
			}
		}
		assets = append(assets, a)
	}
	return assets
}

// assetCard is one asset in full: identity, owners and freshness first, then what people wrote, then structure, lineage and what applies to it, all within cardBudget.
func (tc *ToolContext) assetCard(ctx context.Context, a *asset.Asset) string {
	var b strings.Builder
	kind, isEntry := knowledge.KindOfType(a.Type)
	fmt.Fprintf(&b, "# %s\n%s", deref(a.Name), a.Type)
	if len(a.Providers) > 0 {
		fmt.Fprintf(&b, " in %s", strings.Join(a.Providers, ", "))
	}
	if a.MRN != nil {
		fmt.Fprintf(&b, "\nmrn: %s", *a.MRN)
	}
	fmt.Fprintf(&b, "\nid: %s", a.ID)
	if u := assetURL(tc.rootURL, a); u != "" {
		fmt.Fprintf(&b, "\nurl: %s", u)
	}
	if isEntry && tc.knowledgeService != nil {
		fmt.Fprintf(&b, "\n\nThis is a %s knowledge entry; this card has only its summary. Read the whole entry with read_knowledge {\"id\": %q}.", strings.ToLower(kind.Label), a.ID)
	}

	// Trust: who answers for it, when it last changed and anything that should make a reader careful.
	owners := tc.ownerList(ctx, a.ID)
	if owners == "" {
		b.WriteString("\nOwners: none. Nobody is accountable for this asset; say so if an answer depends on it.")
	} else {
		b.WriteString("\nOwners: " + owners)
	}
	if u := updatedText(a.UpdatedAt); u != "" {
		b.WriteString("\nUpdated: " + u)
		if !a.LastSyncAt.IsZero() {
			b.WriteString("; last synced from its source " + updatedText(a.LastSyncAt))
		}
	}
	if len(a.Tags) > 0 {
		tags := append([]string(nil), a.Tags...)
		sort.Strings(tags)
		b.WriteString("\nTags: " + strings.Join(tags, ", "))
	}
	if terms, err := tc.assetService.GetTerms(ctx, a.ID); err == nil && len(terms) > 0 {
		names := make([]string, len(terms))
		for i, t := range terms {
			names[i] = t.TermName
		}
		sort.Strings(names)
		b.WriteString("\nGlossary terms: " + strings.Join(names, ", ") + " (definitions with lookup_term)")
	}
	if a.IsStub {
		b.WriteString("\nCaution: this is a placeholder created from lineage. No plugin has catalogued it, so its details may be missing.")
	}
	if deprecated(a) {
		b.WriteString("\nCaution: tagged deprecated. Don't build anything new on it; look for what replaced it.")
	}

	if s := strings.TrimSpace(deref(a.UserDescription)); s != "" {
		b.WriteString("\n\n## Summary (written by people in the company)\n" + truncate(s, 3000))
	}
	if s := strings.TrimSpace(deref(a.Description)); s != "" {
		b.WriteString("\n\n## Description (from the source system)\n" + truncate(s, 3000))
	}

	if !isEntry {
		tc.writeDocPages(ctx, &b, a)
		tc.writeAppliedKnowledge(ctx, &b, a)
	}

	writeSchema(&b, a.Schema)
	writeMetadata(&b, a.Metadata)
	if a.Query != nil && strings.TrimSpace(*a.Query) != "" {
		lang := "sql"
		if a.QueryLanguage != nil && *a.QueryLanguage != "" {
			lang = *a.QueryLanguage
		}
		fmt.Fprintf(&b, "\n\n## Query\n```%s\n%s\n```", lang, truncate(*a.Query, maxQueryText))
	}

	lineageShown := false
	if tc.lineageService != nil {
		if resp, err := tc.lineageService.GetAssetLineage(ctx, a.ID, 2, "both"); err == nil && resp != nil {
			tc.recordLookup(ctx, lookups.CategoryLineage)
			if s := lineageSection(resp, maxLineageInCard, a.ID); s != "" {
				b.WriteString("\n\n## Lineage (up to 2 hops, nearest first)\n" + s)
				lineageShown = true
			}
		}
	}

	if tc.dataProductService != nil {
		if products, err := tc.dataProductService.GetDataProductsForAsset(ctx, a.ID); err == nil && len(products) > 0 {
			b.WriteString("\n\n## Data products it belongs to")
			for _, p := range products {
				fmt.Fprintf(&b, "\n- %s (id %s)", p.Name, p.ID)
			}
		}
	}

	out := b.String()
	if len([]rune(out)) > cardBudget {
		out = truncate(out, cardBudget) + "\n\n[This card was cut to fit. Ask for lineage with trace_lineage and owners with find_ownership.]"
	}
	var next []step
	if lineageShown {
		next = append(next, step{"full downstream impact", "trace_lineage", map[string]any{"id": idFor(a), "direction": "downstream", "depth": 5}})
	}
	if tc.knowledgeService != nil && !isEntry {
		next = append(next, step{"all knowledge that applies here", "search_knowledge", map[string]any{"about": idFor(a)}})
	}
	var nb strings.Builder
	writeNext(&nb, next...)
	return out + nb.String()
}

// ownerList names an asset's owners with their kind and email, or returns empty.
func (tc *ToolContext) ownerList(ctx context.Context, assetID string) string {
	if tc.teamService == nil {
		return ""
	}
	owners, err := tc.teamService.ListAssetOwners(ctx, assetID)
	if err != nil {
		return ""
	}
	parts := make([]string, 0, len(owners))
	for _, o := range owners {
		s := fmt.Sprintf("%s (%s)", o.Name, o.Type)
		if o.Email != nil && *o.Email != "" {
			s = fmt.Sprintf("%s (%s, %s)", o.Name, o.Type, *o.Email)
		}
		parts = append(parts, s)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// writeDocPages adds an asset's documentation pages, which often hold the only explanation of how a table is built, within docsBudget.
func (tc *ToolContext) writeDocPages(ctx context.Context, b *strings.Builder, a *asset.Asset) {
	if tc.docsService == nil || a.MRN == nil {
		return
	}
	tree, err := tc.docsService.GetPageTree(ctx, docs.EntityTypeAsset, *a.MRN)
	if err != nil || tree == nil {
		return
	}
	var pages []*docs.Page
	var walk func([]*docs.Page)
	walk = func(ps []*docs.Page) {
		for _, p := range ps {
			pages = append(pages, p)
			walk(p.Children)
		}
	}
	walk(tree.Pages)
	if len(pages) == 0 {
		return
	}
	fmt.Fprintf(b, "\n\n## Documentation pages (%d)", len(pages))
	left := docsBudget
	for _, p := range pages {
		fmt.Fprintf(b, "\n\n### %s (updated %s)", p.Title, updatedText(p.UpdatedAt))
		body := strings.TrimSpace(deref(p.Content))
		// Pages usually repeat their title as a first heading.
		body = strings.TrimSpace(strings.TrimPrefix(body, "## "+p.Title))
		body = strings.TrimSpace(strings.TrimPrefix(body, "# "+p.Title))
		if body == "" {
			continue
		}
		if left <= 0 {
			b.WriteString("\n[Not shown: over the size limit. Open the asset's url to read it.]")
			continue
		}
		n := len([]rune(body))
		if n > left {
			body = truncate(body, left) + "\n[Cut here to fit. Open the asset's url for the rest.]"
			n = left
		}
		left -= n
		b.WriteString("\n" + body)
	}
}

// writeAppliedKnowledge lists the policies, decisions and runbooks that apply to an asset, so a reader sees the rules without a second call.
func (tc *ToolContext) writeAppliedKnowledge(ctx context.Context, b *strings.Builder, a *asset.Asset) {
	if tc.knowledgeService == nil {
		return
	}
	page, err := tc.knowledgeService.About(ctx, a.ID, knowledge.AboutFilter{States: []knowledge.State{knowledge.StateCurrent, knowledge.StatePending}, Limit: 10})
	if err != nil || page == nil || len(page.Entries) == 0 {
		return
	}
	fmt.Fprintf(b, "\n\n## Knowledge that applies (%d)", page.Total)
	for _, it := range page.Entries {
		label := it.Kind
		if k, ok := knowledge.KindByID(it.Kind); ok {
			label = k.Label + ", " + statusLabel(k, it.Status)
		}
		fmt.Fprintf(b, "\n- %s (%s): %s", it.Name, label, truncate(oneLine(it.Summary.Summary), maxListSummary))
		if why := whyApplies(it); why != "" {
			fmt.Fprintf(b, " %s.", strings.TrimSuffix(why, "."))
		}
		fmt.Fprintf(b, " Read with read_knowledge {\"id\": %q}.", it.AssetID)
	}
	if page.Total > len(page.Entries) {
		fmt.Fprintf(b, "\n%d more: search_knowledge {\"about\": %q}.", page.Total-len(page.Entries), idFor(a))
	}
}

// schemaColumn reads a schema value, which plugins store either as a bare type or as JSON with a type and description.
func schemaColumn(raw string) (string, string) {
	var c struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "{") && json.Unmarshal([]byte(raw), &c) == nil && c.Type != "" {
		return c.Type, c.Description
	}
	return raw, ""
}

func writeSchema(b *strings.Builder, schema map[string]string) {
	if len(schema) == 0 {
		return
	}
	cols := sortedKeys(schema)
	fmt.Fprintf(b, "\n\n## Schema (%d columns)", len(cols))
	for i, col := range cols {
		if i == maxSchemaColumns {
			fmt.Fprintf(b, "\n[%d more columns not shown]", len(cols)-maxSchemaColumns)
			break
		}
		typ, desc := schemaColumn(schema[col])
		fmt.Fprintf(b, "\n- %s: %s", col, truncate(oneLine(typ), 80))
		if desc != "" {
			fmt.Fprintf(b, ", %s", truncate(oneLine(desc), maxListSummary))
		}
	}
}

func writeMetadata(b *strings.Builder, md map[string]interface{}) {
	if len(md) == 0 {
		return
	}
	keys := sortedKeys(md)
	b.WriteString("\n\n## Metadata")
	for i, k := range keys {
		if i == maxMetadataKeys {
			fmt.Fprintf(b, "\n[%d more keys not shown]", len(keys)-maxMetadataKeys)
			break
		}
		v := md[k]
		var s string
		switch x := v.(type) {
		case string:
			s = x
		default:
			s = compactJSON(x)
		}
		fmt.Fprintf(b, "\n- %s: %s", k, truncate(oneLine(s), maxMetadataValue))
	}
}

// lineageSection lists lineage nodes nearest first, upstream then downstream, each with its MRN and last run status, up to limit nodes.
func lineageSection(resp *lineage.LineageResponse, limit int, selfID string) string {
	var up, down []lineage.LineageNode
	for _, n := range resp.Nodes {
		if n.Asset == nil || n.Asset.ID == selfID {
			continue
		}
		if n.Depth < 0 {
			up = append(up, n)
		} else if n.Depth > 0 {
			down = append(down, n)
		}
	}
	order := func(ns []lineage.LineageNode) {
		sort.SliceStable(ns, func(i, j int) bool {
			di, dj := abs(ns[i].Depth), abs(ns[j].Depth)
			if di != dj {
				return di < dj
			}
			return deref(ns[i].Asset.Name) < deref(ns[j].Asset.Name)
		})
	}
	order(up)
	order(down)
	if len(up)+len(down) == 0 {
		return ""
	}
	var b strings.Builder
	shown := 0
	write := func(title string, ns []lineage.LineageNode) {
		if len(ns) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n", title, len(ns))
		for _, n := range ns {
			if shown == limit {
				return
			}
			hops := "hop"
			if abs(n.Depth) > 1 {
				hops = "hops"
			}
			fmt.Fprintf(&b, "- %d %s: %s (%s) %s", abs(n.Depth), hops, deref(n.Asset.Name), n.Asset.Type, idFor(n.Asset))
			if n.LastRunStatus != "" {
				fmt.Fprintf(&b, " [last run %s]", n.LastRunStatus)
			}
			if n.Asset.IsStub {
				b.WriteString(" [placeholder]")
			}
			b.WriteString("\n")
			shown++
		}
	}
	write("Upstream, what feeds it", up)
	write("Downstream, what depends on it", down)
	if omitted := len(up) + len(down) - shown; omitted > 0 {
		fmt.Fprintf(&b, "%d more not shown.\n", omitted)
	}
	return strings.TrimRight(b.String(), "\n")
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (tc *ToolContext) findOwnership(ctx context.Context, _ *mcpsdk.CallToolRequest, args FindOwnershipInput) (*mcpsdk.CallToolResult, any, error) {
	hasAsset := strings.TrimSpace(args.AssetID) != ""
	hasOwner := args.UserID != "" || args.Username != "" || args.TeamID != "" || args.TeamName != ""
	if !hasAsset && !hasOwner {
		return toolError(codeInvalidArgument, "Give asset_id to find who owns an asset, or team_name, team_id, username or user_id to find what they own.",
			step{"list teams", "explore_teams", nil}), nil, nil
	}
	if hasAsset && hasOwner {
		return toolError(codeInvalidArgument, "Give either asset_id or an owner, not both."), nil, nil
	}
	if hasAsset {
		return tc.findAssetOwners(ctx, args.AssetID)
	}
	return tc.findOwnedByEntity(ctx, args)
}

func (tc *ToolContext) findAssetOwners(ctx context.Context, ref string) (*mcpsdk.CallToolResult, any, error) {
	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Msg("MCP find_ownership lookup failed")
		return unavailable("look that asset up"), nil, nil
	}
	if a == nil {
		return assetNotFound("find_ownership", "asset_id", ref, candidates), nil, nil
	}
	owners, err := tc.teamService.ListAssetOwners(ctx, a.ID)
	if err != nil {
		log.Warn().Err(err).Msg("MCP find_ownership owners failed")
		return unavailable("read the owners"), nil, nil
	}
	sort.SliceStable(owners, func(i, j int) bool { return owners[i].Name < owners[j].Name })

	var b strings.Builder
	fmt.Fprintf(&b, "Owners of %s (%s)\nmrn: %s", deref(a.Name), a.Type, idFor(a))
	if u := assetURL(tc.rootURL, a); u != "" {
		fmt.Fprintf(&b, "\nurl: %s", u)
	}
	var next []step
	if len(owners) == 0 {
		b.WriteString("\n\nNobody owns this asset. Say so rather than guessing; the owners of what feeds it may know more.")
		next = append(next, step{"see what feeds it", "trace_lineage", map[string]any{"id": idFor(a), "direction": "upstream", "depth": 1}})
	}
	for _, o := range owners {
		fmt.Fprintf(&b, "\n- %s (%s, id %s)", o.Name, o.Type, o.ID)
		if o.Email != nil && *o.Email != "" {
			fmt.Fprintf(&b, ", %s", *o.Email)
		}
		if o.Type == "team" {
			next = append(next, step{"who is in " + o.Name, "explore_teams", map[string]any{"team_id": o.ID}})
		}
	}
	next = append(next, step{"the asset in full", "discover_data", map[string]any{"id": idFor(a)}})
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) findOwnedByEntity(ctx context.Context, args FindOwnershipInput) (*mcpsdk.CallToolResult, any, error) {
	limit, offset := pageBounds(args.Limit, args.Offset, 50, 100)
	includeAssets := args.IncludeAssets == nil || *args.IncludeAssets
	includeTerms := args.IncludeGlossaryTerms == nil || *args.IncludeGlossaryTerms
	if !includeAssets && !includeTerms {
		return toolError(codeInvalidArgument, "include_assets and include_glossary_terms are both false, so there is nothing to list."), nil, nil
	}

	var entityID, entityName, entityType string
	if args.UserID != "" || args.Username != "" {
		entityType = "user"
		var u *user.User
		var err error
		if args.UserID != "" {
			u, err = tc.userService.Get(ctx, args.UserID)
		} else {
			u, err = tc.userService.GetUserByUsername(ctx, args.Username)
		}
		if err != nil || u == nil {
			ref := args.Username
			if ref == "" {
				ref = args.UserID
			}
			var next []step
			if args.Username != "" {
				suggestions, _ := tc.userService.FindSimilarUsernames(ctx, args.Username, 5)
				for _, s := range suggestions {
					next = append(next, step{"user " + s, "find_ownership", map[string]any{"username": s}})
				}
			}
			return toolError(codeNotFound, fmt.Sprintf("No user %q.", ref), next...), nil, nil
		}
		entityID, entityName = u.ID, u.Name
	} else {
		entityType = "team"
		var t *Team
		var err error
		if args.TeamID != "" {
			t, err = tc.teamService.GetTeam(ctx, args.TeamID)
		} else {
			t, err = tc.teamService.GetTeamByName(ctx, args.TeamName)
		}
		if err != nil || t == nil {
			ref := args.TeamName
			if ref == "" {
				ref = args.TeamID
			}
			var next []step
			if args.TeamName != "" {
				suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, args.TeamName, 5)
				for _, s := range suggestions {
					next = append(next, step{"team " + s, "find_ownership", map[string]any{"team_name": s}})
				}
			}
			next = append(next, step{"list every team", "explore_teams", nil})
			return toolError(codeNotFound, fmt.Sprintf("No team %q.", ref), next...), nil, nil
		}
		entityID, entityName = t.ID, t.Name
	}

	var b strings.Builder
	fmt.Fprintf(&b, "What %s (%s, id %s) owns", entityName, entityType, entityID)
	more := false
	if includeAssets {
		ownerType := entityType
		assets, total, _, err := tc.assetService.Search(ctx, asset.SearchFilter{OwnerType: &ownerType, OwnerID: &entityID, Limit: limit, Offset: offset}, false)
		if err != nil {
			log.Warn().Err(err).Msg("MCP find_ownership assets failed")
			return unavailable("list what they own"), nil, nil
		}
		fmt.Fprintf(&b, "\n\nAssets: %d", total)
		if len(assets) > 0 {
			fmt.Fprintf(&b, ", showing %d to %d", offset+1, offset+len(assets))
		}
		for _, a := range assets {
			b.WriteString("\n" + assetLine(a, tc.rootURL))
		}
		more = more || total > offset+len(assets)
	}
	if includeTerms {
		res, err := tc.glossaryService.Search(ctx, glossary.SearchFilter{OwnerIDs: []string{entityID}, Limit: limit, Offset: offset})
		if err != nil {
			log.Warn().Err(err).Msg("MCP find_ownership terms failed")
			return unavailable("list what they own"), nil, nil
		}
		fmt.Fprintf(&b, "\n\nGlossary terms: %d", res.Total)
		for _, t := range res.Terms {
			fmt.Fprintf(&b, "\n- %s (id %s): %s", t.Name, t.ID, truncate(oneLine(t.Definition), maxListSummary))
		}
		more = more || res.Total > offset+len(res.Terms)
	}
	var next []step
	if more {
		page := map[string]any{"offset": offset + limit, "limit": limit}
		if entityType == "user" {
			page["user_id"] = entityID
		} else {
			page["team_id"] = entityID
		}
		next = append(next, step{"next page", "find_ownership", page})
	}
	if entityType == "team" {
		next = append(next, step{"who is in the team", "explore_teams", map[string]any{"team_id": entityID}})
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) lookupTerm(ctx context.Context, _ *mcpsdk.CallToolRequest, args LookupTermInput) (*mcpsdk.CallToolResult, any, error) {
	if strings.TrimSpace(args.TermID) != "" {
		return tc.getTerm(ctx, strings.TrimSpace(args.TermID), args)
	}
	return tc.searchTerms(ctx, args)
}

// getTerm accepts a term ID or an exact name, since agents usually know the name.
func (tc *ToolContext) getTerm(ctx context.Context, ref string, args LookupTermInput) (*mcpsdk.CallToolResult, any, error) {
	var term *glossary.GlossaryTerm
	if _, err := uuid.Parse(ref); err == nil {
		t, err := tc.glossaryService.Get(ctx, ref)
		if err != nil && !errors.Is(err, glossary.ErrTermNotFound) {
			log.Warn().Err(err).Msg("MCP lookup_term get failed")
			return unavailable("read that term"), nil, nil
		}
		term = t
	} else {
		res, err := tc.glossaryService.Search(ctx, glossary.SearchFilter{Query: ref, Limit: 10})
		if err != nil {
			log.Warn().Err(err).Msg("MCP lookup_term search failed")
			return unavailable("search the glossary"), nil, nil
		}
		for _, t := range res.Terms {
			if strings.EqualFold(t.Name, ref) {
				term = t
				break
			}
		}
	}
	if term == nil {
		return toolError(codeNotFound, fmt.Sprintf("No glossary term %q.", ref), step{"search terms by words", "lookup_term", map[string]any{"query": ref}}), nil, nil
	}
	tc.recordLookup(ctx, lookups.CategoryGlossaryTerm)
	limit, offset := pageBounds(args.Limit, args.Offset, 25, 100)

	var b strings.Builder
	fmt.Fprintf(&b, "# %s (glossary term)\nid: %s", term.Name, term.ID)
	if u := termURL(tc.rootURL, term.ID); u != "" {
		fmt.Fprintf(&b, "\nurl: %s", u)
	}
	if len(term.Owners) > 0 {
		names := make([]string, len(term.Owners))
		for i, o := range term.Owners {
			names[i] = fmt.Sprintf("%s (%s)", o.Name, o.Type)
		}
		b.WriteString("\nOwners: " + strings.Join(names, ", "))
	} else {
		b.WriteString("\nOwners: none")
	}
	if u := updatedText(term.UpdatedAt); u != "" {
		b.WriteString("\nUpdated: " + u)
	}
	b.WriteString("\n\nDefinition: " + term.Definition)
	if term.Description != nil && strings.TrimSpace(*term.Description) != "" {
		b.WriteString("\n\n" + truncate(*term.Description, 4000))
	}
	if ancestors, err := tc.glossaryService.GetAncestors(ctx, term.ID); err == nil && len(ancestors) > 0 {
		names := make([]string, len(ancestors))
		for i, t := range ancestors {
			names[i] = t.Name
		}
		b.WriteString("\nUnder: " + strings.Join(names, " > "))
	}
	if children, err := tc.glossaryService.GetChildren(ctx, term.ID); err == nil && len(children) > 0 {
		b.WriteString("\nNarrower terms:")
		for _, c := range children {
			fmt.Fprintf(&b, "\n- %s (id %s)", c.Name, c.ID)
		}
	}
	if len(term.Metadata) > 0 {
		writeMetadata(&b, term.Metadata)
	}
	var next []step
	linked, total, err := tc.assetService.GetAssetsByTerm(ctx, term.ID, limit, offset)
	if err == nil {
		fmt.Fprintf(&b, "\n\n## Assets linked to it (%d)", total)
		for _, a := range linked {
			b.WriteString("\n" + assetLine(a, tc.rootURL))
		}
		if total > offset+len(linked) {
			next = append(next, step{"more linked assets", "lookup_term", map[string]any{"term_id": term.ID, "offset": offset + len(linked), "limit": limit}})
		}
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) searchTerms(ctx context.Context, args LookupTermInput) (*mcpsdk.CallToolResult, any, error) {
	limit, offset := pageBounds(args.Limit, args.Offset, 20, 100)
	filter := glossary.SearchFilter{Query: args.Query, Limit: limit, Offset: offset}
	if args.ParentTermID != "" {
		filter.ParentTermID = &args.ParentTermID
	}
	res, err := tc.glossaryService.Search(ctx, filter)
	if err != nil {
		log.Warn().Err(err).Msg("MCP lookup_term search failed")
		return unavailable("search the glossary"), nil, nil
	}
	var b strings.Builder
	switch {
	case res.Total == 0 && args.Query != "":
		fmt.Fprintf(&b, "No glossary terms match %q.", args.Query)
	case res.Total == 0:
		b.WriteString("No glossary terms here.")
	default:
		fmt.Fprintf(&b, "%d glossary terms; showing %d to %d.", res.Total, offset+1, offset+len(res.Terms))
	}
	for _, t := range res.Terms {
		fmt.Fprintf(&b, "\n- %s (id %s): %s", t.Name, t.ID, truncate(oneLine(t.Definition), 240))
	}
	var next []step
	if len(res.Terms) > 0 {
		next = append(next, step{"a term in full, with its linked assets", "lookup_term", map[string]any{"term_id": res.Terms[0].ID}})
	}
	if res.Total > offset+len(res.Terms) {
		page := map[string]any{"offset": offset + len(res.Terms), "limit": limit}
		if args.Query != "" {
			page["query"] = args.Query
		}
		if args.ParentTermID != "" {
			page["parent_term_id"] = args.ParentTermID
		}
		next = append(next, step{"next page", "lookup_term", page})
	}
	if res.Total == 0 && args.Query != "" {
		next = append(next, step{"browse every term", "lookup_term", nil})
		if tc.knowledgeService != nil {
			next = append(next, step{"look in written knowledge", "search_knowledge", map[string]any{"query": args.Query}})
		}
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) exploreDataProducts(ctx context.Context, _ *mcpsdk.CallToolRequest, args ExploreDataProductsInput) (*mcpsdk.CallToolResult, any, error) {
	limit, offset := pageBounds(args.Limit, args.Offset, 20, 100)
	if args.ID != "" {
		p, err := tc.dataProductService.Get(ctx, args.ID)
		if err != nil || p == nil {
			return toolError(codeNotFound, fmt.Sprintf("No data product with id %q.", args.ID), step{"list data products", "explore_data_products", nil}), nil, nil
		}
		return tc.productCard(ctx, p, limit, offset)
	}
	if args.Name != "" {
		res, err := tc.dataProductService.Search(ctx, dataproduct.SearchFilter{Query: args.Name, Limit: 20})
		if err != nil {
			log.Warn().Err(err).Msg("MCP explore_data_products search failed")
			return unavailable("look that data product up"), nil, nil
		}
		for _, p := range res.DataProducts {
			if strings.EqualFold(p.Name, args.Name) {
				return tc.productCard(ctx, p, limit, offset)
			}
		}
		if len(res.DataProducts) == 1 {
			return tc.productCard(ctx, res.DataProducts[0], limit, offset)
		}
		var next []step
		for _, p := range res.DataProducts {
			next = append(next, step{p.Name, "explore_data_products", map[string]any{"id": p.ID}})
		}
		next = append(next, step{"list data products", "explore_data_products", nil})
		return toolError(codeNotFound, fmt.Sprintf("No data product named %q.", args.Name), next...), nil, nil
	}

	var res *dataproduct.ListResult
	var err error
	if args.Query == "" && len(args.Tags) == 0 {
		res, err = tc.dataProductService.List(ctx, offset, limit)
	} else {
		res, err = tc.dataProductService.Search(ctx, dataproduct.SearchFilter{Query: args.Query, Tags: args.Tags, Limit: limit, Offset: offset})
	}
	if err != nil {
		log.Warn().Err(err).Msg("MCP explore_data_products list failed")
		return unavailable("list data products"), nil, nil
	}
	var b strings.Builder
	if res.Total == 0 {
		b.WriteString("No data products match.")
	} else {
		fmt.Fprintf(&b, "%d data products; showing %d to %d.", res.Total, offset+1, offset+len(res.DataProducts))
	}
	for _, p := range res.DataProducts {
		fmt.Fprintf(&b, "\n- %s (id %s), %d assets", p.Name, p.ID, p.AssetCount)
		if p.Description != nil && *p.Description != "" {
			fmt.Fprintf(&b, ": %s", truncate(oneLine(*p.Description), maxListSummary))
		}
	}
	var next []step
	if len(res.DataProducts) > 0 {
		next = append(next, step{"a product with its assets", "explore_data_products", map[string]any{"id": res.DataProducts[0].ID}})
	}
	if res.Total > offset+len(res.DataProducts) {
		page := map[string]any{"offset": offset + len(res.DataProducts), "limit": limit}
		if args.Query != "" {
			page["query"] = args.Query
		}
		if len(args.Tags) > 0 {
			page["tags"] = args.Tags
		}
		next = append(next, step{"next page", "explore_data_products", page})
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) productCard(ctx context.Context, p *dataproduct.DataProduct, limit, offset int) (*mcpsdk.CallToolResult, any, error) {
	tc.recordLookup(ctx, lookups.CategoryDataProduct)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (data product)\nid: %s", p.Name, p.ID)
	if u := productURL(tc.rootURL, p.ID); u != "" {
		fmt.Fprintf(&b, "\nurl: %s", u)
	}
	if len(p.Owners) > 0 {
		names := make([]string, len(p.Owners))
		for i, o := range p.Owners {
			names[i] = fmt.Sprintf("%s (%s)", o.Name, o.Type)
		}
		b.WriteString("\nOwners: " + strings.Join(names, ", "))
	} else {
		b.WriteString("\nOwners: none")
	}
	if len(p.Tags) > 0 {
		b.WriteString("\nTags: " + strings.Join(p.Tags, ", "))
	}
	if p.Description != nil && *p.Description != "" {
		b.WriteString("\n\n" + truncate(*p.Description, 3000))
	}
	if len(p.Rules) > 0 {
		b.WriteString("\n\nMembership rules:")
		for _, r := range p.Rules {
			state := "on"
			if !r.IsEnabled {
				state = "off"
			}
			fmt.Fprintf(&b, "\n- %s (%s, %s, %d assets)", r.Name, r.RuleType, state, r.MatchedAssetCount)
		}
	}
	var next []step
	resolved, err := tc.dataProductService.GetResolvedAssets(ctx, p.ID, limit, offset)
	if err == nil && resolved != nil {
		ids := resolved.AllAssets
		if len(ids) > limit {
			ids = ids[:limit]
		}
		fmt.Fprintf(&b, "\n\n## Member assets (%d)", resolved.Total)
		var members []*asset.Asset
		for _, id := range ids {
			if a, err := tc.assetService.Get(ctx, id); err == nil {
				members = append(members, a)
			}
		}
		sort.SliceStable(members, func(i, j int) bool { return deref(members[i].Name) < deref(members[j].Name) })
		for _, a := range members {
			b.WriteString("\n" + assetLine(a, tc.rootURL))
		}
		if resolved.Total > offset+len(ids) {
			next = append(next, step{"more member assets", "explore_data_products", map[string]any{"id": p.ID, "offset": offset + len(ids), "limit": limit}})
		}
	}
	for _, o := range p.Owners {
		if o.Type == "team" {
			next = append(next, step{"who is in " + o.Name, "explore_teams", map[string]any{"team_id": o.ID}})
			break
		}
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) exploreTeams(ctx context.Context, _ *mcpsdk.CallToolRequest, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	limit, offset := pageBounds(args.Limit, args.Offset, 20, 100)
	if args.TeamID != "" || args.TeamName != "" {
		return tc.teamCard(ctx, args)
	}
	if args.UserID != "" || args.Username != "" {
		return tc.userTeams(ctx, args)
	}
	teams, total, err := tc.teamService.ListTeams(ctx, limit, offset)
	if err != nil {
		log.Warn().Err(err).Msg("MCP explore_teams list failed")
		return unavailable("list teams"), nil, nil
	}
	var b strings.Builder
	if total == 0 {
		b.WriteString("There are no teams in this Marmot yet.")
	} else {
		fmt.Fprintf(&b, "%d teams; showing %d to %d.", total, offset+1, offset+len(teams))
	}
	for _, t := range teams {
		fmt.Fprintf(&b, "\n- %s (id %s)", t.Name, t.ID)
		if t.Description != "" {
			fmt.Fprintf(&b, ": %s", truncate(oneLine(t.Description), maxListSummary))
		}
	}
	var next []step
	if len(teams) > 0 {
		next = append(next, step{"a team's members", "explore_teams", map[string]any{"team_id": teams[0].ID}},
			step{"what a team owns", "find_ownership", map[string]any{"team_id": teams[0].ID}})
	}
	if total > offset+len(teams) {
		next = append(next, step{"next page", "explore_teams", map[string]any{"offset": offset + len(teams), "limit": limit}})
	}
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) teamCard(ctx context.Context, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	var t *Team
	var err error
	if args.TeamID != "" {
		t, err = tc.teamService.GetTeam(ctx, args.TeamID)
	} else {
		t, err = tc.teamService.GetTeamByName(ctx, args.TeamName)
	}
	if err != nil || t == nil {
		ref := args.TeamName
		if ref == "" {
			ref = args.TeamID
		}
		var next []step
		if args.TeamName != "" {
			suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, args.TeamName, 5)
			for _, s := range suggestions {
				next = append(next, step{"team " + s, "explore_teams", map[string]any{"team_name": s}})
			}
		}
		next = append(next, step{"list every team", "explore_teams", nil})
		return toolError(codeNotFound, fmt.Sprintf("No team %q.", ref), next...), nil, nil
	}
	members, err := tc.teamService.ListMembers(ctx, t.ID)
	if err != nil {
		log.Warn().Err(err).Msg("MCP explore_teams members failed")
		return unavailable("list the team's members"), nil, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (team)\nid: %s", t.Name, t.ID)
	if u := teamURL(tc.rootURL, t.ID); u != "" {
		fmt.Fprintf(&b, "\nurl: %s", u)
	}
	if t.Description != "" {
		b.WriteString("\n\n" + t.Description)
	}
	fmt.Fprintf(&b, "\n\nMembers (%d):", len(members))
	if len(members) == 0 {
		b.WriteString("\nNone recorded in Marmot.")
	}
	for _, m := range members {
		fmt.Fprintf(&b, "\n- %s (@%s, %s)", m.Name, m.Username, m.Role)
		if m.Email != nil && *m.Email != "" {
			fmt.Fprintf(&b, ", %s", *m.Email)
		}
	}
	var nb strings.Builder
	writeNext(&nb, step{"what the team owns", "find_ownership", map[string]any{"team_id": t.ID}})
	return textResult(b.String() + nb.String()), nil, nil
}

func (tc *ToolContext) userTeams(ctx context.Context, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	var u *user.User
	var err error
	if args.UserID != "" {
		u, err = tc.userService.Get(ctx, args.UserID)
	} else {
		u, err = tc.userService.GetUserByUsername(ctx, args.Username)
	}
	if err != nil || u == nil {
		ref := args.Username
		if ref == "" {
			ref = args.UserID
		}
		var next []step
		if args.Username != "" {
			suggestions, _ := tc.userService.FindSimilarUsernames(ctx, args.Username, 5)
			for _, s := range suggestions {
				next = append(next, step{"user " + s, "explore_teams", map[string]any{"username": s}})
			}
		}
		return toolError(codeNotFound, fmt.Sprintf("No user %q.", ref), next...), nil, nil
	}
	teams, err := tc.teamService.ListUserTeams(ctx, u.ID)
	if err != nil {
		log.Warn().Err(err).Msg("MCP explore_teams user teams failed")
		return unavailable("list the user's teams"), nil, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (@%s, id %s) is in %d teams.", u.Name, u.Username, u.ID, len(teams))
	for _, t := range teams {
		fmt.Fprintf(&b, "\n- %s (id %s)", t.Name, t.ID)
	}
	writeNext(&b, step{"what they own", "find_ownership", map[string]any{"user_id": u.ID}})
	return textResult(b.String()), nil, nil
}

func (tc *ToolContext) traceLineage(ctx context.Context, _ *mcpsdk.CallToolRequest, args TraceLineageInput) (*mcpsdk.CallToolResult, any, error) {
	ref := strings.TrimSpace(args.ID)
	if ref == "" {
		ref = strings.TrimSpace(args.AssetID)
	}
	if ref == "" {
		ref = strings.TrimSpace(args.MRN)
	}
	if ref == "" {
		return toolError(codeInvalidArgument, "Give id: the asset's MRN, ID or exact name."), nil, nil
	}
	direction := strings.ToLower(strings.TrimSpace(args.Direction))
	if direction == "" {
		direction = "both"
	}
	if direction != "upstream" && direction != "downstream" && direction != "both" {
		return toolError(codeInvalidArgument, fmt.Sprintf("direction %q isn't one of upstream, downstream or both.", args.Direction),
			step{"what depends on it", "trace_lineage", map[string]any{"id": ref, "direction": "downstream"}}), nil, nil
	}
	depth := args.Depth
	if depth <= 0 {
		depth = 3
	}
	if depth > 10 {
		depth = 10
	}
	limit, _ := pageBounds(args.Limit, 0, maxLineageInTrace, 200)

	a, candidates, err := tc.findAsset(ctx, ref)
	if err != nil {
		log.Warn().Err(err).Msg("MCP trace_lineage lookup failed")
		return unavailable("look that asset up"), nil, nil
	}
	if a == nil {
		return assetNotFound("trace_lineage", "id", ref, candidates), nil, nil
	}
	resp, err := tc.lineageService.GetAssetLineage(ctx, a.ID, depth, direction)
	if err != nil {
		log.Warn().Err(err).Msg("MCP trace_lineage failed")
		return unavailable("read the lineage"), nil, nil
	}
	tc.recordLookup(ctx, lookups.CategoryLineage)

	var b strings.Builder
	fmt.Fprintf(&b, "Lineage of %s (%s), %s, up to %d hops\nmrn: %s", deref(a.Name), a.Type, direction, depth, idFor(a))
	section := lineageSection(resp, limit, a.ID)
	if section == "" {
		b.WriteString("\n\nNo lineage is recorded in that direction. That means none has been catalogued, not that nothing depends on it.")
	} else {
		b.WriteString("\n\n" + section)
	}
	var next []step
	if section != "" && depth < 10 {
		next = append(next, step{"follow it further", "trace_lineage", map[string]any{"id": idFor(a), "direction": direction, "depth": min(depth+3, 10)}})
	}
	next = append(next, step{"owners of the asset", "find_ownership", map[string]any{"asset_id": idFor(a)}})
	writeNext(&b, next...)
	return textResult(b.String()), nil, nil
}

// errorWithGuidance is the older error shape still used by the query tools; examples come out in a fixed order.
func (tc *ToolContext) errorWithGuidance(what, why string, examples map[string]string) *mcpsdk.CallToolResult {
	msg := fmt.Sprintf("Error: %s\n\n%s", what, why)
	if len(examples) > 0 {
		msg += "\n\nExamples:"
		for _, label := range sortedKeys(examples) {
			msg += fmt.Sprintf("\n- %s: %s", label, examples[label])
		}
	}
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: msg}}, IsError: true}
}

func matchesMetadataFilters(a asset.Asset, filters []MetadataFilterInput) bool {
	if a.Metadata == nil {
		return false
	}
	for _, filter := range filters {
		value, exists := a.Metadata[filter.Key]
		if !exists {
			return false
		}
		if !matchesOperator(value, filter.Operator, filter.Value) {
			return false
		}
	}
	return true
}

func matchesOperator(actual any, operator string, expected any) bool {
	switch operator {
	case "=", "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case ">":
		return compareNumeric(actual, expected) > 0
	case "<":
		return compareNumeric(actual, expected) < 0
	case ">=":
		return compareNumeric(actual, expected) >= 0
	case "<=":
		return compareNumeric(actual, expected) <= 0
	case "contains":
		return strings.Contains(strings.ToLower(fmt.Sprintf("%v", actual)), strings.ToLower(fmt.Sprintf("%v", expected)))
	default:
		return false
	}
}

func compareNumeric(actual, expected any) int {
	a, e := toFloat64(actual), toFloat64(expected)
	switch {
	case a > e:
		return 1
	case a < e:
		return -1
	}
	return 0
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}
