package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/user"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// ToolContext holds services needed for MCP tools
type ToolContext struct {
	assetService    asset.Service
	glossaryService GlossaryService
	userService     user.Service
	teamService     TeamService
	lineageService  lineage.Service
	user            *user.User
	config          *config.Config
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
	Query   string `json:"query,omitempty"`
	TermID  string `json:"term_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
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

	formatted := FormatAssetCard(asset, tc.config.Server.RootURL)

	lineageResp, err := tc.lineageService.GetAssetLineage(ctx, id, 5, "both")
	if err == nil && lineageResp != nil {
		formatted += "\n\n" + tc.formatLineage(lineageResp)
	}

	nextActions := map[string]string{
		"Find who owns this": fmt.Sprintf(`{"asset_id": "%s"}`, asset.ID),
		"Search for similar":  fmt.Sprintf(`{"query": "%s", "types": ["%s"]}`, *asset.Name, asset.Type),
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

	formatted := FormatAssetCard(asset, tc.config.Server.RootURL)

	lineageResp, err := tc.lineageService.GetAssetLineage(ctx, asset.ID, 5, "both")
	if err == nil && lineageResp != nil {
		formatted += "\n\n" + tc.formatLineage(lineageResp)
	}

	nextActions := map[string]string{
		"Find who owns this": fmt.Sprintf(`{"asset_id": "%s"}`, asset.ID),
		"Search for similar":  fmt.Sprintf(`{"query": "%s", "types": ["%s"]}`, *asset.Name, asset.Type),
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

	filter := asset.SearchFilter{
		Query:        query,
		Types:        args.Types,
		Providers:    args.Providers,
		Tags:         args.Tags,
		Limit:        args.Limit,
		Offset:       args.Offset,
		IncludeStubs: true,
	}

	log.Debug().
		Str("query", args.Query).
		Str("normalized_query", query).
		Interface("types", args.Types).
		Interface("providers", args.Providers).
		Interface("tags", args.Tags).
		Int("limit", args.Limit).
		Msg("MCP discover_data search request")

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
		nextActions = map[string]string{
			"Get next page":     fmt.Sprintf(`{"offset": %d, "limit": %d}`, args.Offset+args.Limit, args.Limit),
			"Get asset details": `{"id": "asset-id"}`,
		}
		if args.Query != "" {
			nextActions["Get next page"] = fmt.Sprintf(`{"query": "%s", "offset": %d, "limit": %d}`, query, args.Offset+args.Limit, args.Limit)
		}
		if len(args.Types) > 0 {
			nextActions["Get next page"] = fmt.Sprintf(`{"types": %s, "offset": %d, "limit": %d}`, formatJSON(args.Types), args.Offset+args.Limit, args.Limit)
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
	icon := getAssetIcon(asset.Type)
	parts = append(parts, fmt.Sprintf("# %s %s", icon, *asset.Name))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("**Type:** %s", asset.Type))

	if tc.config.Server.RootURL != "" {
		parts = append(parts, fmt.Sprintf("🔗 [View in Marmot](%s/discover/%s/%s)", tc.config.Server.RootURL, asset.Type, *asset.Name))
	}
	parts = append(parts, "")
	parts = append(parts, "## 👥 Owners")
	parts = append(parts, "")

	if len(owners) > 0 {
		for _, owner := range owners {
			ownerIcon := "👤"
			if owner.Type == "team" {
				ownerIcon = "👥"
			}
			ownerLine := fmt.Sprintf("- %s **%s**", ownerIcon, owner.Name)
			if owner.Email != nil {
				ownerLine += fmt.Sprintf(" (%s)", *owner.Email)
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
	if args.Query == "" && args.TermID == "" {
		return tc.errorWithGuidance(
			"No search criteria provided",
			"Provide either query (to search) or term_id (for specific term).",
			map[string]string{
				"Search example": `{"query": "customer"}`,
				"Lookup example": `{"term_id": "term-123"}`,
			},
		), nil, nil
	}

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

	nextActions := map[string]string{}

	if term.ParentTermID != nil && *term.ParentTermID != "" {
		nextActions["Get parent term"] = fmt.Sprintf(`{"term_id": "%s"}`, *term.ParentTermID)
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

	response := map[string]interface{}{
		"total": result.Total,
		"count": len(result.Terms),
		"terms": result.Terms,
	}

	nextActions := map[string]string{}
	switch {
	case len(result.Terms) == 0:
		nextActions["No results"] = "Try a different query or browse all terms"
	case len(result.Terms) == args.Limit:
		nextActions["Get more results"] = fmt.Sprintf(`{"query": "%s", "offset": %d, "limit": %d}`, args.Query, args.Offset+args.Limit, args.Limit)
		nextActions["Get full details"] = `Use lookup_term with {"term_id": "term-id"} for complete information`
	default:
		nextActions["Get full details"] = `Use lookup_term with {"term_id": "term-id"} for any term`
	}

	response["next_actions"] = nextActions

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{
				Text: string(responseJSON),
			},
		},
	}, nil, nil
}

func (tc *ToolContext) errorWithGuidance(
	what string,
	why string,
	examples map[string]string,
) *mcpsdk.CallToolResult {
	errorMsg := fmt.Sprintf("❌ %s\n\n💡 %s", what, why)

	if len(examples) > 0 {
		errorMsg += "\n\n📋 Examples:\n"
		for label, example := range examples {
			errorMsg += fmt.Sprintf("  • %s: %s\n", label, example)
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
	parts = append(parts, "## 🔄 Data Lineage")
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
		parts = append(parts, "### ⬅️ Upstream Dependencies")
		parts = append(parts, "")
		for _, node := range upstream {
			if node.Asset != nil && node.Asset.Name != nil {
				icon := getAssetIcon(node.Asset.Type)
				parts = append(parts, fmt.Sprintf("- %s **%s** (%s)", icon, *node.Asset.Name, node.Asset.Type))
			}
		}
		parts = append(parts, "")
	}

	if len(downstream) > 0 {
		parts = append(parts, "### ➡️ Downstream Consumers")
		parts = append(parts, "")
		for _, node := range downstream {
			if node.Asset != nil && node.Asset.Name != nil {
				icon := getAssetIcon(node.Asset.Type)
				parts = append(parts, fmt.Sprintf("- %s **%s** (%s)", icon, *node.Asset.Name, node.Asset.Type))
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
		parts = append(parts, "# 📊 Catalog Summary (Filtered)")
	} else {
		parts = append(parts, "# 📊 Catalog Overview")
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
		parts = append(parts, "## 📦 By Asset Type")
		parts = append(parts, "")
		for assetType, count := range filters.Types {
			icon := getAssetIcon(assetType)
			parts = append(parts, fmt.Sprintf("- %s **%s**: %d", icon, assetType, count))
		}
		parts = append(parts, "")
	}

	if len(filters.Providers) > 0 {
		parts = append(parts, "## 🔌 By Provider")
		parts = append(parts, "")
		for provider, count := range filters.Providers {
			parts = append(parts, fmt.Sprintf("- **%s**: %d", provider, count))
		}
		parts = append(parts, "")
	}

	if len(filters.Tags) > 0 {
		parts = append(parts, "## 🏷️  Top Tags")
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
