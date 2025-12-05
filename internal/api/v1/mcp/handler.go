package mcp

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/mcp"
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
	lineageService lineage.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	teamAdapter := &teamServiceAdapter{teamService: teamService}
	return &Handler{
		mcpServer:   mcp.NewServer(assetService, glossaryService, userService, teamAdapter, lineageService, config),
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
			},
		},
	}
}
