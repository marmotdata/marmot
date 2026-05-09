package agents

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/agent"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	agentService agent.Service
	userService  user.Service
	authService  auth.Service
	config       *config.Config
}

func NewHandler(agentService agent.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{
		agentService: agentService,
		userService:  userService,
		authService:  authService,
		config:       config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/agents/runs",
			Method:  http.MethodPost,
			Handler: h.recordRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "agents", "emit"),
			},
		},
		{
			Path:    "/api/v1/agents/{asset_id}/runs",
			Method:  http.MethodGet,
			Handler: h.listRuns,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 60, 60),
			},
		},
		{
			Path:    "/api/v1/agents/{asset_id}/stats",
			Method:  http.MethodGet,
			Handler: h.getStats,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 60, 60),
			},
		},
		{
			Path:    "/api/v1/agents/{asset_id}/activity",
			Method:  http.MethodGet,
			Handler: h.getActivity,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 60, 60),
			},
		},
	}
}
