package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/pkg/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// ToolContext holds services needed for MCP tools
type ToolContext struct {
	assetService       asset.Service
	glossaryService    GlossaryService
	userService        user.Service
	teamService        TeamService
	dataProductService DataProductService
	lineageService     lineage.Service
	searchService      search.Service
	user               *user.User
	config             *config.Config
}

type DiscoverDataInput struct {
	Query           string                `json:"query,omitempty"`
	ID              string                `json:"id,omitempty"`
	MRN             string                `json:"mrn,omitempty"`
	Types           []string              `json:"types,omitempty"`
	Providers       []string              `json:"providers,omitempty"`
	Tags            []string              `json:"tags,omitempty"`
	MetadataFilters []MetadataFilterInput `json:"metadata_filters,omitempty"`
	Limit           int                   `json:"limit,omitempty"`
	Offset          int                   `json:"offset,omitempty"`
}

type MetadataFilterInput struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type FindOwnershipInput struct {
	AssetID              string `json:"asset_id,omitempty"`
	UserID               string `json:"user_id,omitempty"`
	Username             string `json:"username,omitempty"`
	TeamID               string `json:"team_id,omitempty"`
	TeamName             string `json:"team_name,omitempty"`
	IncludeAssets        bool   `json:"include_assets,omitempty"`
	IncludeGlossaryTerms bool   `json:"include_glossary_terms,omitempty"`
	Limit                int    `json:"limit,omitempty"`
	Offset               int    `json:"offset,omitempty"`
}

type LookupTermInput struct {
	Query        string `json:"query,omitempty"`
	TermID       string `json:"term_id,omitempty"`
	ParentTermID string `json:"parent_term_id,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
}

type ExploreDataProductsInput struct {
	Query  string   `json:"query,omitempty"`
	ID     string   `json:"id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Limit  int      `json:"limit,omitempty"`
	Offset int      `json:"offset,omitempty"`
}

type ExploreTeamsInput struct {
	TeamID   string `json:"team_id,omitempty"`
	TeamName string `json:"team_name,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

type TraceLineageInput struct {
	AssetID   string `json:"asset_id,omitempty"`
	MRN       string `json:"mrn,omitempty"`
	Direction string `json:"direction,omitempty"`
	Depth     int    `json:"depth,omitempty"`
}

func (tc *ToolContext) discoverData(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args DiscoverDataInput,
) (*mcpsdk.CallToolResult, any, error) {
	if args.ID != "" {
		return tc.getAssetByID(ctx, args.ID)
	}

	if args.MRN != "" {
		return tc.getAssetByMRN(ctx, args.MRN)
	}

	if args.Limit == 0 {
		args.Limit = 20
	}

	return tc.searchAssets(ctx, args)
}

func (tc *ToolContext) getAssetByID(ctx context.Context, id string) (*mcpsdk.CallToolResult, any, error) {
	asset, err := tc.assetService.Get(ctx, id)
	if err != nil {
		return tc.errorWithGuidance(
			fmt.Sprintf("Asset '%s' not found", id),
			"The asset ID may be incorrect or you may not have permission to view it.",
			map[string]string{
				"Try searching instead": `{"query": "asset name"}`,
			},
		), nil, nil
	}

	return tc.renderAssetDetails(ctx, asset)
}

func (tc *ToolContext) getAssetByMRN(ctx context.Context, mrn string) (*mcpsdk.CallToolResult, any, error) {
	asset, err := tc.assetService.GetByMRN(ctx, mrn)
	if err != nil {
		return tc.errorWithGuidance(
			fmt.Sprintf("Asset with MRN '%s' not found", mrn),
			"The MRN may be incorrect. MRNs are qualified identifiers like 'postgres://database/schema/table'.",
			map[string]string{
				"Try searching instead": `{"query": "table name"}`,
				"List all of this type": `{"providers": ["postgres"]}`,
			},
		), nil, nil
	}

	return tc.renderAssetDetails(ctx, asset)
}

func (tc *ToolContext) renderAssetDetails(ctx context.Context, a *asset.Asset) (*mcpsdk.CallToolResult, any, error) {
	formatted := FormatAssetCard(a, tc.config.Server.RootURL)

	lineageResp, err := tc.lineageService.GetAssetLineage(ctx, a.ID, 5, "both")
	if err == nil && lineageResp != nil {
		formatted += "\n\n" + tc.formatLineage(lineageResp)
	}

	nextActions := map[string]string{
		"Find who owns this": fmt.Sprintf(`Use find_ownership: {"asset_id": "%s"}`, a.ID),
		"Trace full lineage": fmt.Sprintf(`Use trace_lineage: {"asset_id": "%s", "depth": 5}`, a.ID),
		"Search for similar": fmt.Sprintf(`{"query": "%s", "types": ["%s"]}`, *a.Name, a.Type),
	}

	if tc.dataProductService != nil {
		products, err := tc.dataProductService.GetDataProductsForAsset(ctx, a.ID)
		if err == nil && len(products) > 0 {
			formatted += "\n\n" + FormatAssetDataProducts(products, tc.config.Server.RootURL)
			nextActions["Explore data product"] = fmt.Sprintf(`Use explore_data_products: {"id": "%s"}`, products[0].ID)
		}
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) searchAssets(ctx context.Context, args DiscoverDataInput) (*mcpsdk.CallToolResult, any, error) {
	if args.Limit == 0 {
		args.Limit = 20
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	query := args.Query
	if query == "*" {
		query = ""
	}

	log.Debug().
		Str("query", args.Query).
		Str("normalized_query", query).
		Interface("types", args.Types).
		Interface("providers", args.Providers).
		Interface("tags", args.Tags).
		Int("limit", args.Limit).
		Bool("es_enabled", tc.searchService != nil).
		Msg("MCP discover_data search request")

	// Use ES-backed search for text queries when available and no metadata_filters
	// (metadata_filters require full asset objects which ES results don't have)
	if tc.searchService != nil && query != "" && len(args.MetadataFilters) == 0 {
		return tc.searchAssetsES(ctx, args, query)
	}

	return tc.searchAssetsPG(ctx, args, query)
}

func (tc *ToolContext) searchAssetsES(ctx context.Context, args DiscoverDataInput, query string) (*mcpsdk.CallToolResult, any, error) {
	filter := search.Filter{
		Query:      query,
		Types:      []search.ResultType{search.ResultTypeAsset},
		AssetTypes: args.Types,
		Providers:  args.Providers,
		Tags:       args.Tags,
		Limit:      args.Limit,
		Offset:     args.Offset,
	}

	resp, err := tc.searchService.Search(ctx, filter)
	if err != nil {
		return tc.errorWithGuidance(
			"Search failed",
			fmt.Sprintf("Error: %v", err),
			map[string]string{
				"Try simpler query": `{"query": "orders"}`,
			},
		), nil, nil
	}

	assets := searchResultsToAssets(resp.Results)
	total := resp.Total

	formatted := FormatAssetList(assets, total, tc.config.Server.RootURL)
	formatted += "\n\n" + FormatSearchSummary(total, len(assets), nil)

	var nextActions map[string]string
	switch {
	case len(assets) == 0:
		nextActions = map[string]string{
			"Broaden search":  "Remove filters or try a different query",
			"List all assets": `{"limit": 50}`,
		}
	case total > args.Offset+len(assets):
		nextPage := map[string]any{"offset": args.Offset + args.Limit, "limit": args.Limit}
		if query != "" {
			nextPage["query"] = query
		}
		if len(args.Types) > 0 {
			nextPage["types"] = args.Types
		}
		nextActions = map[string]string{
			"Get next page":     formatJSON(nextPage),
			"Get asset details": `{"id": "asset-id"}`,
		}
	default:
		nextActions = map[string]string{
			"Get full details": `{"id": "asset-id"}`,
			"Find who owns":    `Use find_ownership tool`,
		}
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) searchAssetsPG(ctx context.Context, args DiscoverDataInput, query string) (*mcpsdk.CallToolResult, any, error) {
	filter := asset.SearchFilter{
		Query:        query,
		Types:        args.Types,
		Providers:    args.Providers,
		Tags:         args.Tags,
		Limit:        args.Limit,
		Offset:       args.Offset,
		IncludeStubs: true,
	}

	assets, total, availableFilters, err := tc.assetService.Search(ctx, filter, true)
	if err != nil {
		return tc.errorWithGuidance(
			"Search failed",
			fmt.Sprintf("Error: %v", err),
			map[string]string{
				"Try simpler query": `{"query": "orders"}`,
			},
		), nil, nil
	}

	shouldShowSummary := query == "" && args.Offset == 0 && (total > 20 || (len(args.Types) == 0 && len(args.Providers) == 0 && len(args.Tags) == 0))

	if shouldShowSummary && total > 0 {
		return tc.formatCatalogSummary(total, availableFilters, args)
	}

	if len(args.MetadataFilters) > 0 {
		filteredAssets := []*asset.Asset{}
		for _, a := range assets {
			if matchesMetadataFilters(*a, args.MetadataFilters) {
				filteredAssets = append(filteredAssets, a)
			}
		}
		assets = filteredAssets
		total = len(assets)
	}

	formatted := FormatAssetList(assets, total, tc.config.Server.RootURL)
	formatted += "\n\n" + FormatSearchSummary(total, len(assets), availableFilters)

	var nextActions map[string]string
	switch {
	case len(assets) == 0:
		nextActions = map[string]string{
			"Broaden search":  "Remove filters or try a different query",
			"List all assets": `{"limit": 50}`,
		}
	case total > args.Offset+len(assets):
		nextPage := map[string]any{"offset": args.Offset + args.Limit, "limit": args.Limit}
		if args.Query != "" {
			nextPage["query"] = query
		}
		if len(args.Types) > 0 {
			nextPage["types"] = args.Types
		}
		nextActions = map[string]string{
			"Get next page":     formatJSON(nextPage),
			"Get asset details": `{"id": "asset-id"}`,
		}
	default:
		nextActions = map[string]string{
			"Get full details": `{"id": "asset-id"}`,
			"Find who owns":    `Use find_ownership tool`,
		}
	}

	if len(args.MetadataFilters) > 0 {
		nextActions["Remove metadata filter"] = "Call without metadata_filters"
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

// searchResultsToAssets converts search.Result objects into lightweight asset.Asset
// structs suitable for FormatAssetList rendering.
func searchResultsToAssets(results []*search.Result) []*asset.Asset {
	assets := make([]*asset.Asset, 0, len(results))
	for _, r := range results {
		a := &asset.Asset{
			ID:          r.ID,
			Name:        &r.Name,
			Description: r.Description,
		}

		if t, ok := r.Metadata["type"].(string); ok {
			a.Type = t
		}
		if mrn, ok := r.Metadata["mrn"].(string); ok {
			a.MRN = &mrn
		}
		if providers, ok := r.Metadata["providers"].([]string); ok {
			a.Providers = providers
		}
		// Handle []interface{} from JSON unmarshalling
		if providers, ok := r.Metadata["providers"].([]interface{}); ok {
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

func (tc *ToolContext) findOwnership(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args FindOwnershipInput,
) (*mcpsdk.CallToolResult, any, error) {
	if !args.IncludeAssets && !args.IncludeGlossaryTerms {
		args.IncludeAssets = true
		args.IncludeGlossaryTerms = true
	}

	hasAssetID := args.AssetID != ""
	hasOwner := args.UserID != "" || args.Username != "" || args.TeamID != "" || args.TeamName != ""

	if !hasAssetID && !hasOwner {
		return tc.errorWithGuidance(
			"No search criteria provided",
			"Provide either asset_id to find owners, OR user/team identifiers to find what they own.",
			map[string]string{
				"Find asset owners":  `{"asset_id": "asset-123"}`,
				"Find user's assets": `{"username": "john.doe"}`,
				"Find team's data":   `{"team_name": "data-engineering"}`,
			},
		), nil, nil
	}

	if hasAssetID && hasOwner {
		return tc.errorWithGuidance(
			"Ambiguous query",
			"Provide either asset_id OR owner identifiers, not both.",
			map[string]string{
				"Find asset owners":  `{"asset_id": "asset-123"}`,
				"Find user's assets": `{"username": "john.doe"}`,
			},
		), nil, nil
	}

	if hasAssetID {
		return tc.findAssetOwners(ctx, args.AssetID)
	}

	return tc.findOwnedByEntity(ctx, args)
}

func (tc *ToolContext) findAssetOwners(ctx context.Context, assetID string) (*mcpsdk.CallToolResult, any, error) {
	asset, err := tc.assetService.Get(ctx, assetID)
	if err != nil {
		return tc.errorWithGuidance(
			fmt.Sprintf("Asset '%s' not found", assetID),
			"The asset ID may be incorrect.",
			map[string]string{
				"Search for asset first": `Use discover_data with {"query": "asset name"}`,
			},
		), nil, nil
	}

	owners, err := tc.teamService.ListAssetOwners(ctx, assetID)
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch owners",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("# %s", *asset.Name))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**Type:** %s", asset.Type))

	if tc.config.Server.RootURL != "" {
		parts = append(parts, fmt.Sprintf("[View in Marmot](%s/discover/%s/%s)", tc.config.Server.RootURL, asset.Type, *asset.Name))
	}
	parts = append(parts, "")
	parts = append(parts, "## Owners")
	parts = append(parts, "")

	if len(owners) > 0 {
		for _, owner := range owners {
			ownerLine := fmt.Sprintf("- **%s** (%s)", owner.Name, owner.Type)
			if owner.Email != nil {
				ownerLine += fmt.Sprintf(" — %s", *owner.Email)
			}
			parts = append(parts, ownerLine)
		}
	} else {
		parts = append(parts, "_No owners assigned to this asset_")
	}

	formatted := strings.Join(parts, "\n")

	nextActions := map[string]string{
		"Get full asset details": fmt.Sprintf(`{"id": "%s"}`, asset.ID),
	}

	if len(owners) > 0 {
		for _, owner := range owners {
			if owner.Type == "user" {
				nextActions[fmt.Sprintf("Find all of %s's data", owner.Name)] = fmt.Sprintf(`{"user_id": "%s"}`, owner.ID)
				break
			} else {
				nextActions[fmt.Sprintf("Find all of %s's data", owner.Name)] = fmt.Sprintf(`{"team_id": "%s"}`, owner.ID)
				break
			}
		}
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) findOwnedByEntity(ctx context.Context, args FindOwnershipInput) (*mcpsdk.CallToolResult, any, error) {
	if args.Limit == 0 {
		args.Limit = 50
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	isUser := args.UserID != "" || args.Username != ""

	var entityID, entityName, entityType string
	if isUser {
		userID := args.UserID
		if userID == "" {
			user, err := tc.userService.GetUserByUsername(ctx, args.Username)
			if err != nil {
				suggestions, _ := tc.userService.FindSimilarUsernames(ctx, args.Username, 5)

				nextActions := map[string]string{
					"Try with user ID": `{"user_id": "user-uuid"}`,
				}

				var guidanceMsg string
				if len(suggestions) > 0 {
					guidanceMsg = fmt.Sprintf("User '%s' not found. Did you mean one of these users? %v", args.Username, suggestions)
					for _, suggestion := range suggestions {
						nextActions[fmt.Sprintf("Try user: %s", suggestion)] = fmt.Sprintf(`{"username": "%s"}`, suggestion)
					}
				} else {
					guidanceMsg = "The username may be incorrect."
				}

				return tc.errorWithGuidance(
					fmt.Sprintf("User '%s' not found", args.Username),
					guidanceMsg,
					nextActions,
				), nil, nil
			}
			userID = user.ID
			entityName = user.Name
		}
		entityID = userID
		entityType = "user"
	} else {
		teamID := args.TeamID
		if teamID == "" {
			team, err := tc.teamService.GetTeamByName(ctx, args.TeamName)
			if err != nil {
				suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, args.TeamName, 5)

				nextActions := map[string]string{
					"Try with team ID": `{"team_id": "team-uuid"}`,
				}

				var guidanceMsg string
				if len(suggestions) > 0 {
					guidanceMsg = fmt.Sprintf("Team '%s' not found. Did you mean one of these teams? %v", args.TeamName, suggestions)
					for _, suggestion := range suggestions {
						nextActions[fmt.Sprintf("Try team: %s", suggestion)] = fmt.Sprintf(`{"team_name": "%s"}`, suggestion)
					}
				} else {
					guidanceMsg = "The team name may be incorrect."
				}

				return tc.errorWithGuidance(
					fmt.Sprintf("Team '%s' not found", args.TeamName),
					guidanceMsg,
					nextActions,
				), nil, nil
			}
			teamID = team.ID
			entityName = team.Name
		}
		entityID = teamID
		entityType = "team"
	}

	var assets []*asset.Asset
	var terms []*glossary.GlossaryTerm

	if args.IncludeAssets {
		ownerType := entityType
		filter := asset.SearchFilter{
			OwnerType: &ownerType,
			OwnerID:   &entityID,
			Limit:     args.Limit,
			Offset:    args.Offset,
		}

		assetResults, _, _, err := tc.assetService.Search(ctx, filter, false)
		if err != nil {
			return tc.errorWithGuidance(
				"Failed to fetch assets",
				fmt.Sprintf("Error: %v", err),
				nil,
			), nil, nil
		}
		assets = assetResults
	}

	if args.IncludeGlossaryTerms {
		filter := glossary.SearchFilter{
			OwnerIDs: []string{entityID},
			Limit:    args.Limit,
			Offset:   args.Offset,
		}

		result, err := tc.glossaryService.Search(ctx, filter)
		if err != nil {
			return tc.errorWithGuidance(
				"Failed to fetch glossary terms",
				fmt.Sprintf("Error: %v", err),
				nil,
			), nil, nil
		}
		terms = result.Terms
	}

	formatted := FormatOwnershipResult(entityName, entityType, assets, terms, tc.config.Server.RootURL)

	nextActions := map[string]string{
		"Get asset details": `{"id": "asset-id"}`,
		"Get term details":  `Use lookup_term with {"term_id": "term-id"}`,
	}

	if len(assets)+len(terms) > args.Limit {
		nextActions["Get more results"] = fmt.Sprintf(`{"user_id": "%s", "offset": %d, "limit": %d}`, entityID, args.Offset+args.Limit, args.Limit)
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) lookupTerm(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args LookupTermInput,
) (*mcpsdk.CallToolResult, any, error) {
	if args.TermID != "" {
		return tc.getTermByID(ctx, args.TermID)
	}

	return tc.searchTerms(ctx, args)
}

func (tc *ToolContext) getTermByID(ctx context.Context, termID string) (*mcpsdk.CallToolResult, any, error) {
	term, err := tc.glossaryService.Get(ctx, termID)
	if err != nil {
		return tc.errorWithGuidance(
			fmt.Sprintf("Glossary term '%s' not found", termID),
			"The term ID may be incorrect.",
			map[string]string{
				"Search instead": `{"query": "term name"}`,
			},
		), nil, nil
	}

	formatted := FormatTermCard(term, tc.config.Server.RootURL)

	if ancestors, err := tc.glossaryService.GetAncestors(ctx, termID); err == nil && len(ancestors) > 0 {
		formatted += "\n\n" + FormatTermHierarchy("Ancestors", ancestors)
	}

	children, childrenErr := tc.glossaryService.GetChildren(ctx, termID)
	if childrenErr == nil && len(children) > 0 {
		formatted += "\n\n" + FormatTermHierarchy("Child Terms", children)
	}

	linkedAssets, linkedTotal, assetsErr := tc.assetService.GetAssetsByTerm(ctx, termID, 10, 0)
	if assetsErr == nil && len(linkedAssets) > 0 {
		formatted += "\n\n" + FormatTermAssets(linkedAssets, linkedTotal, tc.config.Server.RootURL)
	}

	nextActions := map[string]string{}

	if term.ParentTermID != nil && *term.ParentTermID != "" {
		nextActions["Get parent term"] = fmt.Sprintf(`{"term_id": "%s"}`, *term.ParentTermID)
	}

	if childrenErr == nil && len(children) > 0 {
		nextActions["List child terms"] = fmt.Sprintf(`{"parent_term_id": "%s"}`, termID)
	}

	if assetsErr == nil && len(linkedAssets) > 0 {
		nextActions["Get linked asset details"] = `Use discover_data with {"id": "asset-id"}`
	}

	if len(term.Owners) > 0 {
		owner := term.Owners[0]
		if owner.Type == "user" {
			nextActions[fmt.Sprintf("Find %s's other terms", owner.Name)] = fmt.Sprintf(`Use find_ownership: {"user_id": "%s", "include_assets": false}`, owner.ID)
		} else {
			nextActions[fmt.Sprintf("Find %s's other terms", owner.Name)] = fmt.Sprintf(`Use find_ownership: {"team_id": "%s", "include_assets": false}`, owner.ID)
		}
	}

	if len(term.Tags) > 0 {
		nextActions["Find similar terms"] = fmt.Sprintf(`{"query": "%s"}`, term.Tags[0])
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) searchTerms(ctx context.Context, args LookupTermInput) (*mcpsdk.CallToolResult, any, error) {
	if args.Limit == 0 {
		args.Limit = 20
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	filter := glossary.SearchFilter{
		Query:  args.Query,
		Limit:  args.Limit,
		Offset: args.Offset,
	}
	if args.ParentTermID != "" {
		filter.ParentTermID = &args.ParentTermID
	}

	result, err := tc.glossaryService.Search(ctx, filter)
	if err != nil {
		return tc.errorWithGuidance(
			"Search failed",
			fmt.Sprintf("Error: %v", err),
			map[string]string{
				"Try simpler query": `{"query": "customer"}`,
			},
		), nil, nil
	}

	formatted := FormatTermList(result.Terms, result.Total, tc.config.Server.RootURL)

	nextActions := map[string]string{}
	switch {
	case len(result.Terms) == 0:
		nextActions["Browse all terms"] = `{}`
	case result.Total > args.Offset+len(result.Terms):
		nextPage := map[string]any{"offset": args.Offset + args.Limit, "limit": args.Limit}
		if args.Query != "" {
			nextPage["query"] = args.Query
		}
		if args.ParentTermID != "" {
			nextPage["parent_term_id"] = args.ParentTermID
		}
		nextActions["Get next page"] = formatJSON(nextPage)
		nextActions["Get full details"] = `{"term_id": "term-id"}`
	default:
		nextActions["Get full details"] = `{"term_id": "term-id"}`
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) exploreDataProducts(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args ExploreDataProductsInput,
) (*mcpsdk.CallToolResult, any, error) {
	if args.Limit == 0 {
		args.Limit = 20
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	if args.ID != "" {
		return tc.getDataProductByID(ctx, args.ID)
	}

	if args.Name != "" {
		return tc.getDataProductByName(ctx, args.Name)
	}

	return tc.searchDataProducts(ctx, args)
}

func (tc *ToolContext) getDataProductByID(ctx context.Context, id string) (*mcpsdk.CallToolResult, any, error) {
	product, err := tc.dataProductService.Get(ctx, id)
	if err != nil {
		return tc.errorWithGuidance(
			fmt.Sprintf("Data product '%s' not found", id),
			"The data product ID may be incorrect.",
			map[string]string{
				"Search instead":      `{"query": "product name"}`,
				"Browse all products": `{}`,
			},
		), nil, nil
	}

	return tc.renderDataProductDetails(ctx, product)
}

func (tc *ToolContext) getDataProductByName(ctx context.Context, name string) (*mcpsdk.CallToolResult, any, error) {
	result, err := tc.dataProductService.Search(ctx, dataproduct.SearchFilter{
		Query: name,
		Limit: 20,
	})
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to look up data product",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	for _, product := range result.DataProducts {
		if strings.EqualFold(product.Name, name) {
			return tc.renderDataProductDetails(ctx, product)
		}
	}

	if len(result.DataProducts) == 1 {
		return tc.renderDataProductDetails(ctx, result.DataProducts[0])
	}

	nextActions := map[string]string{
		"Search by keyword":   fmt.Sprintf(`{"query": "%s"}`, name),
		"Browse all products": `{}`,
	}
	if len(result.DataProducts) > 0 {
		product := result.DataProducts[0]
		nextActions[fmt.Sprintf("Try: %s", product.Name)] = fmt.Sprintf(`{"id": "%s"}`, product.ID)
	}

	return tc.errorWithGuidance(
		fmt.Sprintf("No data product named '%s' found", name),
		"The name may be incorrect, or use query for a fuzzy search.",
		nextActions,
	), nil, nil
}

func (tc *ToolContext) renderDataProductDetails(ctx context.Context, product *dataproduct.DataProduct) (*mcpsdk.CallToolResult, any, error) {
	const assetSampleSize = 15

	var memberAssets []*asset.Asset
	totalAssets := 0
	resolved, err := tc.dataProductService.GetResolvedAssets(ctx, product.ID, assetSampleSize, 0)
	if err == nil && resolved != nil {
		totalAssets = resolved.Total
		for _, assetID := range resolved.AllAssets {
			if len(memberAssets) >= assetSampleSize {
				break
			}
			a, err := tc.assetService.Get(ctx, assetID)
			if err != nil {
				continue
			}
			memberAssets = append(memberAssets, a)
		}
	}

	formatted := FormatDataProductCard(product, memberAssets, totalAssets, tc.config.Server.RootURL)

	nextActions := map[string]string{
		"Get member asset details": `Use discover_data with {"id": "asset-id"}`,
	}
	if len(product.Owners) > 0 {
		owner := product.Owners[0]
		if owner.Type == "team" {
			nextActions[fmt.Sprintf("Explore team %s", owner.Name)] = fmt.Sprintf(`Use explore_teams: {"team_id": "%s"}`, owner.ID)
		} else {
			nextActions[fmt.Sprintf("Find %s's other data", owner.Name)] = fmt.Sprintf(`Use find_ownership: {"user_id": "%s"}`, owner.ID)
		}
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) searchDataProducts(ctx context.Context, args ExploreDataProductsInput) (*mcpsdk.CallToolResult, any, error) {
	var result *dataproduct.ListResult
	var err error

	if args.Query == "" && len(args.Tags) == 0 {
		result, err = tc.dataProductService.List(ctx, args.Offset, args.Limit)
	} else {
		result, err = tc.dataProductService.Search(ctx, dataproduct.SearchFilter{
			Query:  args.Query,
			Tags:   args.Tags,
			Limit:  args.Limit,
			Offset: args.Offset,
		})
	}
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch data products",
			fmt.Sprintf("Error: %v", err),
			map[string]string{
				"Browse all products": `{}`,
			},
		), nil, nil
	}

	formatted := FormatDataProductList(result.DataProducts, result.Total, tc.config.Server.RootURL)

	nextActions := map[string]string{}
	switch {
	case len(result.DataProducts) == 0:
		nextActions["Browse all products"] = `{}`
	case result.Total > args.Offset+len(result.DataProducts):
		nextPage := map[string]any{"offset": args.Offset + args.Limit, "limit": args.Limit}
		if args.Query != "" {
			nextPage["query"] = args.Query
		}
		if len(args.Tags) > 0 {
			nextPage["tags"] = args.Tags
		}
		nextActions["Get next page"] = formatJSON(nextPage)
		nextActions["Get product details"] = `{"id": "product-id"}`
	default:
		nextActions["Get product details"] = `{"id": "product-id"}`
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) exploreTeams(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args ExploreTeamsInput,
) (*mcpsdk.CallToolResult, any, error) {
	if args.Limit == 0 {
		args.Limit = 20
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	if args.TeamID != "" || args.TeamName != "" {
		return tc.getTeamDetails(ctx, args)
	}

	if args.UserID != "" || args.Username != "" {
		return tc.getUserTeams(ctx, args)
	}

	return tc.listTeams(ctx, args)
}

func (tc *ToolContext) getTeamDetails(ctx context.Context, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	var team *Team
	var err error

	if args.TeamID != "" {
		team, err = tc.teamService.GetTeam(ctx, args.TeamID)
		if err != nil {
			return tc.errorWithGuidance(
				fmt.Sprintf("Team '%s' not found", args.TeamID),
				"The team ID may be incorrect.",
				map[string]string{
					"Try by name":    `{"team_name": "data-engineering"}`,
					"List all teams": `{}`,
				},
			), nil, nil
		}
	} else {
		team, err = tc.teamService.GetTeamByName(ctx, args.TeamName)
		if err != nil {
			suggestions, _ := tc.teamService.FindSimilarTeamNames(ctx, args.TeamName, 5)

			nextActions := map[string]string{
				"List all teams": `{}`,
			}

			guidanceMsg := "The team name may be incorrect."
			if len(suggestions) > 0 {
				guidanceMsg = fmt.Sprintf("Team '%s' not found. Did you mean one of these teams? %v", args.TeamName, suggestions)
				for _, suggestion := range suggestions {
					nextActions[fmt.Sprintf("Try team: %s", suggestion)] = fmt.Sprintf(`{"team_name": "%s"}`, suggestion)
				}
			}

			return tc.errorWithGuidance(
				fmt.Sprintf("Team '%s' not found", args.TeamName),
				guidanceMsg,
				nextActions,
			), nil, nil
		}
	}

	members, err := tc.teamService.ListMembers(ctx, team.ID)
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch team members",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	formatted := FormatTeamCard(team, members, tc.config.Server.RootURL)

	nextActions := map[string]string{
		"Find what this team owns": fmt.Sprintf(`Use find_ownership: {"team_id": "%s"}`, team.ID),
	}
	if len(members) > 0 {
		nextActions[fmt.Sprintf("Find %s's teams", members[0].Name)] = fmt.Sprintf(`{"user_id": "%s"}`, members[0].UserID)
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) getUserTeams(ctx context.Context, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	userID := args.UserID
	userName := args.Username
	if userID == "" {
		u, err := tc.userService.GetUserByUsername(ctx, args.Username)
		if err != nil {
			suggestions, _ := tc.userService.FindSimilarUsernames(ctx, args.Username, 5)

			nextActions := map[string]string{
				"Try with user ID": `{"user_id": "user-uuid"}`,
			}

			guidanceMsg := "The username may be incorrect."
			if len(suggestions) > 0 {
				guidanceMsg = fmt.Sprintf("User '%s' not found. Did you mean one of these users? %v", args.Username, suggestions)
				for _, suggestion := range suggestions {
					nextActions[fmt.Sprintf("Try user: %s", suggestion)] = fmt.Sprintf(`{"username": "%s"}`, suggestion)
				}
			}

			return tc.errorWithGuidance(
				fmt.Sprintf("User '%s' not found", args.Username),
				guidanceMsg,
				nextActions,
			), nil, nil
		}
		userID = u.ID
		userName = u.Name
	}

	teams, err := tc.teamService.ListUserTeams(ctx, userID)
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch user's teams",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	if userName == "" {
		userName = userID
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("# Teams for %s (%d)", escapeMarkdown(userName), len(teams)))
	parts = append(parts, "")
	if len(teams) == 0 {
		parts = append(parts, "_Not a member of any team_")
	}
	for _, team := range teams {
		parts = append(parts, formatTeamListEntry(team, tc.config.Server.RootURL))
	}
	formatted := strings.Join(parts, "\n")

	nextActions := map[string]string{
		"Find what this user owns": fmt.Sprintf(`Use find_ownership: {"user_id": "%s"}`, userID),
	}
	if len(teams) > 0 {
		nextActions[fmt.Sprintf("Explore team %s", teams[0].Name)] = fmt.Sprintf(`{"team_id": "%s"}`, teams[0].ID)
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) listTeams(ctx context.Context, args ExploreTeamsInput) (*mcpsdk.CallToolResult, any, error) {
	teams, total, err := tc.teamService.ListTeams(ctx, args.Limit, args.Offset)
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch teams",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	formatted := FormatTeamList(teams, total, tc.config.Server.RootURL)

	nextActions := map[string]string{}
	switch {
	case len(teams) == 0:
		nextActions["No teams found"] = "There are no teams in this Marmot instance yet"
	case total > args.Offset+len(teams):
		nextActions["Get next page"] = formatJSON(map[string]any{"offset": args.Offset + args.Limit, "limit": args.Limit})
		nextActions["Get team details"] = `{"team_id": "team-uuid"}`
	default:
		nextActions["Get team details"] = `{"team_id": "team-uuid"}`
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) traceLineage(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	args TraceLineageInput,
) (*mcpsdk.CallToolResult, any, error) {
	if args.AssetID == "" && args.MRN == "" {
		return tc.errorWithGuidance(
			"No asset provided",
			"Provide either asset_id or mrn to trace lineage from.",
			map[string]string{
				"Trace by ID":  `{"asset_id": "asset-123"}`,
				"Trace by MRN": `{"mrn": "postgres://db/schema/table"}`,
			},
		), nil, nil
	}

	direction := args.Direction
	if direction == "" {
		direction = "both"
	}
	if direction != "upstream" && direction != "downstream" && direction != "both" {
		return tc.errorWithGuidance(
			fmt.Sprintf("Invalid direction '%s'", args.Direction),
			`Direction must be "upstream", "downstream", or "both".`,
			map[string]string{
				"Trace upstream": fmt.Sprintf(`{"asset_id": "%s", "direction": "upstream"}`, args.AssetID),
			},
		), nil, nil
	}

	depth := args.Depth
	if depth == 0 {
		depth = 3
	}
	if depth > 10 {
		depth = 10
	}

	var a *asset.Asset
	var err error
	if args.AssetID != "" {
		a, err = tc.assetService.Get(ctx, args.AssetID)
	} else {
		a, err = tc.assetService.GetByMRN(ctx, args.MRN)
	}
	if err != nil {
		return tc.errorWithGuidance(
			"Asset not found",
			"The asset_id or mrn may be incorrect.",
			map[string]string{
				"Search for the asset": `Use discover_data with {"query": "asset name"}`,
			},
		), nil, nil
	}

	lineageResp, err := tc.lineageService.GetAssetLineage(ctx, a.ID, depth, direction)
	if err != nil {
		return tc.errorWithGuidance(
			"Failed to fetch lineage",
			fmt.Sprintf("Error: %v", err),
			nil,
		), nil, nil
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("# %s", escapeMarkdown(*a.Name)))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**Type:** %s — **Direction:** %s — **Depth:** %d", a.Type, direction, depth))

	formatted := strings.Join(parts, "\n")

	lineageSection := tc.formatLineage(lineageResp)
	if lineageSection == "" {
		lineageSection = "## Lineage\n\n_No lineage relationships found_"
	}
	formatted += "\n\n" + lineageSection

	nextActions := map[string]string{
		"Get asset details": fmt.Sprintf(`Use discover_data: {"id": "%s"}`, a.ID),
	}
	if depth < 10 {
		nextActions["Trace deeper"] = fmt.Sprintf(`{"asset_id": "%s", "direction": "%s", "depth": %d}`, a.ID, direction, depth+2)
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}

func (tc *ToolContext) errorWithGuidance(
	what string,
	why string,
	examples map[string]string,
) *mcpsdk.CallToolResult {
	errorMsg := fmt.Sprintf("Error: %s\n\n%s", what, why)

	if len(examples) > 0 {
		errorMsg += "\n\nExamples:\n"
		for label, example := range examples {
			errorMsg += fmt.Sprintf("- %s: %s\n", label, example)
		}
	}

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: errorMsg,
			},
		},
		IsError: true,
	}
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
		actualStr := fmt.Sprintf("%v", actual)
		expectedStr := fmt.Sprintf("%v", expected)
		return strings.Contains(strings.ToLower(actualStr), strings.ToLower(expectedStr))
	default:
		return false
	}
}

func compareNumeric(actual, expected any) int {
	actualFloat := toFloat64(actual)
	expectedFloat := toFloat64(expected)

	if actualFloat > expectedFloat {
		return 1
	} else if actualFloat < expectedFloat {
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

func formatJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// formatLineage formats lineage information
func (tc *ToolContext) formatLineage(lineageResp *lineage.LineageResponse) string {
	if lineageResp == nil || len(lineageResp.Nodes) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "## Lineage")
	parts = append(parts, "")

	upstream := []lineage.LineageNode{}
	downstream := []lineage.LineageNode{}

	for _, node := range lineageResp.Nodes {
		if node.Depth < 0 {
			upstream = append(upstream, node)
		} else if node.Depth > 0 {
			downstream = append(downstream, node)
		}
	}

	if len(upstream) > 0 {
		parts = append(parts, "### Upstream Dependencies")
		parts = append(parts, "")
		for _, node := range upstream {
			if node.Asset != nil && node.Asset.Name != nil {
				parts = append(parts, fmt.Sprintf("- **%s** (%s)", *node.Asset.Name, node.Asset.Type))
			}
		}
		parts = append(parts, "")
	}

	if len(downstream) > 0 {
		parts = append(parts, "### Downstream Consumers")
		parts = append(parts, "")
		for _, node := range downstream {
			if node.Asset != nil && node.Asset.Name != nil {
				parts = append(parts, fmt.Sprintf("- **%s** (%s)", *node.Asset.Name, node.Asset.Type))
			}
		}
		parts = append(parts, "")
	}

	if len(upstream) == 0 && len(downstream) == 0 {
		parts = append(parts, "_No lineage relationships found_")
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// formatCatalogSummary creates a summary overview of the catalog
func (tc *ToolContext) formatCatalogSummary(total int, filters asset.AvailableFilters, args DiscoverDataInput) (*mcpsdk.CallToolResult, any, error) {
	var parts []string

	if len(args.Types) > 0 || len(args.Providers) > 0 || len(args.Tags) > 0 {
		parts = append(parts, "# Catalog Summary (Filtered)")
	} else {
		parts = append(parts, "# Catalog Overview")
	}
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**Total Assets:** %d", total))
	parts = append(parts, "")

	if len(args.Types) > 0 || len(args.Providers) > 0 || len(args.Tags) > 0 {
		parts = append(parts, "**Applied Filters:**")
		if len(args.Types) > 0 {
			parts = append(parts, fmt.Sprintf("- Types: %s", strings.Join(args.Types, ", ")))
		}
		if len(args.Providers) > 0 {
			parts = append(parts, fmt.Sprintf("- Providers: %s", strings.Join(args.Providers, ", ")))
		}
		if len(args.Tags) > 0 {
			parts = append(parts, fmt.Sprintf("- Tags: %s", strings.Join(args.Tags, ", ")))
		}
		parts = append(parts, "")
	}

	if len(filters.Types) > 0 {
		parts = append(parts, "## By Asset Type")
		parts = append(parts, "")
		for assetType, count := range filters.Types {
			parts = append(parts, fmt.Sprintf("- **%s**: %d", assetType, count))
		}
		parts = append(parts, "")
	}

	if len(filters.Providers) > 0 {
		parts = append(parts, "## By Provider")
		parts = append(parts, "")
		for provider, count := range filters.Providers {
			parts = append(parts, fmt.Sprintf("- **%s**: %d", provider, count))
		}
		parts = append(parts, "")
	}

	if len(filters.Tags) > 0 {
		parts = append(parts, "## Top Tags")
		parts = append(parts, "")
		count := 0
		for tag, tagCount := range filters.Tags {
			if count >= 10 {
				parts = append(parts, fmt.Sprintf("_...and %d more tags_", len(filters.Tags)-10))
				break
			}
			parts = append(parts, fmt.Sprintf("- **%s**: %d", tag, tagCount))
			count++
		}
		parts = append(parts, "")
	}

	formatted := strings.Join(parts, "\n")

	nextActions := map[string]string{}

	if len(args.Types) == 0 && len(filters.Types) > 0 {
		for assetType := range filters.Types {
			nextActions[fmt.Sprintf("View all %s", assetType)] = fmt.Sprintf(`{"types": ["%s"]}`, assetType)
			break
		}
	}

	if len(args.Providers) == 0 && len(filters.Providers) > 0 {
		for provider := range filters.Providers {
			nextActions[fmt.Sprintf("View all %s assets", provider)] = fmt.Sprintf(`{"providers": ["%s"]}`, provider)
			break
		}
	}

	if len(nextActions) == 0 {
		nextActions["View asset list"] = `{"limit": 50}`
	}

	formatted += "\n\n" + FormatNextActions(nextActions)

	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: formatted,
			},
		},
	}, nil, nil
}
