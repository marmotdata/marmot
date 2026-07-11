package mcp

import (
	"context"

	"github.com/marmotdata/marmot/pkg/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type GlossaryService interface {
	Get(ctx context.Context, id string) (*glossary.GlossaryTerm, error)
	Search(ctx context.Context, filter glossary.SearchFilter) (*glossary.ListResult, error)
	GetChildren(ctx context.Context, parentID string) ([]*glossary.GlossaryTerm, error)
	GetAncestors(ctx context.Context, termID string) ([]*glossary.GlossaryTerm, error)
}

type TeamService interface {
	GetTeam(ctx context.Context, id string) (*Team, error)
	GetTeamByName(ctx context.Context, name string) (*Team, error)
	FindSimilarTeamNames(ctx context.Context, searchTerm string, limit int) ([]string, error)
	ListAssetOwners(ctx context.Context, assetID string) ([]Owner, error)
	ListTeams(ctx context.Context, limit, offset int) ([]*Team, int, error)
	ListMembers(ctx context.Context, teamID string) ([]*TeamMember, error)
	ListUserTeams(ctx context.Context, userID string) ([]*Team, error)
}

type DataProductService interface {
	Get(ctx context.Context, id string) (*dataproduct.DataProduct, error)
	List(ctx context.Context, offset, limit int) (*dataproduct.ListResult, error)
	Search(ctx context.Context, filter dataproduct.SearchFilter) (*dataproduct.ListResult, error)
	GetResolvedAssets(ctx context.Context, dataProductID string, limit, offset int) (*dataproduct.ResolvedAssets, error)
	GetDataProductsForAsset(ctx context.Context, assetID string) ([]*dataproduct.DataProduct, error)
}

type Owner struct {
	Type  string  `json:"type"`
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

type Team struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type TeamMember struct {
	UserID   string  `json:"user_id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    *string `json:"email,omitempty"`
	Role     string  `json:"role"`
}

type Server struct {
	assetService       asset.Service
	glossaryService    GlossaryService
	userService        user.Service
	teamService        TeamService
	dataProductService DataProductService
	lineageService     lineage.Service
	searchService      search.Service
	config             *config.Config
}

func NewServer(
	assetService asset.Service,
	glossaryService GlossaryService,
	userService user.Service,
	teamService TeamService,
	dataProductService DataProductService,
	lineageService lineage.Service,
	searchService search.Service,
	config *config.Config,
) *Server {
	return &Server{
		assetService:       assetService,
		glossaryService:    glossaryService,
		userService:        userService,
		teamService:        teamService,
		dataProductService: dataProductService,
		lineageService:     lineageService,
		searchService:      searchService,
		config:             config,
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
		assetService:       s.assetService,
		glossaryService:    s.glossaryService,
		userService:        s.userService,
		teamService:        s.teamService,
		dataProductService: s.dataProductService,
		lineageService:     s.lineageService,
		searchService:      s.searchService,
		user:               user,
		config:             s.config,
	}

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

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "lookup_term",
		Description: `<usecase>
Use this to understand business terminology and definitions. Browse the full glossary, search
for terms by name, navigate the term hierarchy, or get a specific term's complete definition
including the data assets it's linked to. Helps answer questions like "what does customer_id mean?",
"what terms exist under Finance?", or "which assets are tagged with the Revenue term?".
</usecase>

<instructions>
Choose the right approach:
- To browse ALL terms: {} (empty parameters) - returns paginated term list
- To search by name/definition: {"query": "customer"}
- To list children of a term: {"parent_term_id": "term-123"}
- To get a specific term: {"term_id": "term-123"} - returns full definition, hierarchy (ancestors
  and children), owners, and linked data assets

Use offset/limit for pagination (default 20, max 100).
</instructions>`,
	}, tc.lookupTerm)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "explore_data_products",
		Description: `<usecase>
Use this to explore data products - curated collections of related data assets that together
serve a business purpose (e.g. "Customer 360", "Payments Reporting"). Helps answer questions
like "what data products exist?", "what assets make up the Customer 360 product?", or
"which data products does the payments team own?".
</usecase>

<instructions>
Choose the right approach:
- To browse ALL data products: {} (empty parameters)
- To search by name/description: {"query": "customer"}
- To filter by tags: {"tags": ["gold", "pii"]}
- To get a specific product with its assets: {"id": "product-id"} or {"name": "Customer 360"}

Getting a specific product returns its description, owners, membership rules, and the resolved
list of member assets. Use offset/limit for pagination (default 20, max 100).
</instructions>`,
	}, tc.exploreDataProducts)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "explore_teams",
		Description: `<usecase>
Use this to explore teams and their members. Helps answer questions like "what teams exist?",
"who is in the data-engineering team?", or "which teams does john.doe belong to?".
For what a team OWNS (assets/terms), use find_ownership instead.
</usecase>

<instructions>
Choose the right approach:
- To list ALL teams: {} (empty parameters)
- To get a team and its members: {"team_id": "team-uuid"} or {"team_name": "data-engineering"}
- To find a user's teams: {"user_id": "user-uuid"} or {"username": "john.doe"}

Team details include each member's name, username, email and role. Use offset/limit for
pagination of the team list (default 20, max 100).
</instructions>`,
	}, tc.exploreTeams)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "trace_lineage",
		Description: `<usecase>
Use this to trace data lineage: where data comes from (upstream) and what consumes it
(downstream). Helps answer questions like "what feeds this table?", "what breaks if this
topic changes?", or "trace the full pipeline around this asset".
</usecase>

<instructions>
Provide ONE of:
- asset_id: Trace lineage for an asset by ID
- mrn: Trace lineage by MRN (e.g. "postgres://db/schema/table")

Optional:
- direction: "upstream", "downstream", or "both" (default "both")
- depth: How many hops to traverse, 1-10 (default 3)

Returns upstream dependencies and downstream consumers grouped by distance from the asset.
</instructions>`,
	}, tc.traceLineage)
}
