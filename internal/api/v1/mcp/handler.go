package mcp

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	memoryAPI "github.com/marmotdata/marmot/internal/api/v1/memory"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/memory"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/mcp"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
)

type Handler struct {
	mcpServer   *mcp.Server
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(
	assetService asset.Service,
	glossaryService mcp.GlossaryService,
	userService user.Service,
	teamService *team.Service,
	dataProductService dataproduct.Service,
	lineageService lineage.Service,
	searchService search.Service,
	authService auth.Service,
	config *config.Config,
	lookupsRecorder lookups.Recorder,
) *Handler {
	teamAdapter := &teamServiceAdapter{teamService: teamService}
	return &Handler{
		mcpServer:   mcp.NewServer(assetService, glossaryService, userService, teamAdapter, dataProductService, lineageService, searchService, config, lookupsRecorder),
		userService: userService,
		authService: authService,
		config:      config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/mcp",
			Method:  http.MethodOptions,
			Handler: h.handleMCP,
		},
		{
			Path:    "/api/v1/mcp",
			Method:  http.MethodGet,
			Handler: h.handleMCP,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.RequirePermission(h.userService, "glossary", "view"),
				common.RequirePermission(h.userService, "teams", "view"),
			},
		},
		{
			Path:    "/api/v1/mcp",
			Method:  http.MethodPost,
			Handler: h.handleMCP,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.RequirePermission(h.userService, "glossary", "view"),
				common.RequirePermission(h.userService, "teams", "view"),
			},
		},
	}
}

// SetMemory enables the memory tools, with access checked like the REST API.
func (h *Handler) SetMemory(svc memory.Service) {
	h.mcpServer.SetMemory(svc, memoryAPI.Access{Users: h.userService})
}
