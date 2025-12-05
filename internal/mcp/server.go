package mcp

import (
	"context"

	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/user"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// GlossaryService interface for glossary operations (avoiding circular import)
type GlossaryService interface {
	Get(ctx context.Context, id string) (*glossary.GlossaryTerm, error)
	Search(ctx context.Context, filter glossary.SearchFilter) (*glossary.ListResult, error)
}

// TeamService interface for team operations (avoiding circular import)
type TeamService interface {
	GetTeam(ctx context.Context, id string) (*Team, error)
	GetTeamByName(ctx context.Context, name string) (*Team, error)
	FindSimilarTeamNames(ctx context.Context, searchTerm string, limit int) ([]string, error)
	ListAssetOwners(ctx context.Context, assetID string) ([]Owner, error)
}

type Owner struct {
	Type  string  `json:"type"`
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Server struct {
	assetService    asset.Service
	glossaryService GlossaryService
	userService     user.Service
	teamService     TeamService
	lineageService  lineage.Service
	config          *config.Config
}

func NewServer(
	assetService asset.Service,
	glossaryService GlossaryService,
	userService user.Service,
	teamService TeamService,
	lineageService lineage.Service,
	config *config.Config,
) *Server {
	return &Server{
		assetService:    assetService,
		glossaryService: glossaryService,
		userService:     userService,
		teamService:     teamService,
		lineageService:  lineageService,
		config:          config,
	}
}

func (s *Server) CreateMCPServer(ctx context.Context, user *user.User) *mcpsdk.Server {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "marmot-catalog",
			Version: "1.0.0",
		},
		nil,
	)

	s.registerTools(server, user)

	return server
}

func (s *Server) registerTools(server *mcpsdk.Server, user *user.User) {
	tc := &ToolContext{
		assetService:    s.assetService,
		glossaryService: s.glossaryService,
		userService:     s.userService,
		teamService:     s.teamService,
		lineageService:  s.lineageService,
		user:            user,
		config:          s.config,
	}

	// Tool 1: discover_data - Unified data discovery
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "discover_data",
		Description: `<usecase>
Use this to find any data asset in the Marmot catalog. Supports browsing all assets, filtering by
type/provider/tags, searching by name and metadata-based queries.
</usecase>

<instructions>
Choose the right approach:
- To browse ALL assets: {} (empty parameters) - returns summary breakdown
- To count assets: Use filters without query - returns summary with counts
- To filter by type: {"types": ["topic", "table", "bucket"]} - returns summary if >20 results
- To filter by provider: {"providers": ["kafka", "postgres", "s3"]} - returns summary if >20 results
- To search by name: {"query": "customer"} (searches asset names/descriptions)
- To get specific asset: {"id": "asset-id"} or {"mrn": "postgres://db/schema/table"}
- To filter by metadata: {"metadata_filters": [{"key": "partitions", "operator": ">", "value": 5}]}

IMPORTANT:
- For "how many X" or "what's in my catalog", use filters (not query) to get summary breakdowns
- For "show me all X" requests, use types/providers filters
- For "find assets named X", use query parameter
- Default limit is 20 results, max is 100
- Use offset for pagination: {"offset": 20, "limit": 20} for next page

Returns either a summary breakdown (for browse/count queries) or asset list with details.
</instructions>`,
	}, tc.discoverData)

	// Tool 2: find_ownership - Bidirectional ownership queries
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "find_ownership",
		Description: `<usecase>
Use this to answer ownership questions: "who owns this asset?", "what does this user own?",
"show me all data owned by the data-eng team". Works bidirectionally - find owners of an
asset OR find everything a user/team owns (both data assets and glossary terms).
</usecase>

<instructions>
Provide ONE of:
- asset_id: To find who owns a specific asset
- user_id or username: To find everything a user owns
- team_id or team_name: To find everything a team owns

Optional: include_assets (default true), include_glossary_terms (default true) to control what's returned.
Returns ownership details and suggests related queries.
</instructions>`,
	}, tc.findOwnership)

	// Tool 3: lookup_term - Business glossary
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "lookup_term",
		Description: `<usecase>
Use this to understand business terminology and definitions. Search for glossary terms by name
or get specific term definitions. Helps answer questions like "what does customer_id mean?"
or "find all terms related to customer".
</usecase>

<instructions>
Provide ONE of:
- query: Search terms by name or definition (e.g., "customer", "revenue")
- term_id: Get a specific term's complete definition

Returns term definitions, ownership, related terms, and parent/child relationships in the glossary hierarchy.
</instructions>`,
	}, tc.lookupTerm)
}
