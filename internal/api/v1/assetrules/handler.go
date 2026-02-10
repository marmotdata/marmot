package assetrules

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	assetRuleService assetrule.Service
	userService      user.Service
	authService      auth.Service
	config           *config.Config
}

func NewHandler(
	assetRuleService assetrule.Service,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		assetRuleService: assetRuleService,
		userService:      userService,
		authService:      authService,
		config:           config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/asset-rules/list",
			Method:  http.MethodGet,
			Handler: h.list,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 100, 60),
			},
		},
		{
			Path:    "/api/v1/asset-rules/search",
			Method:  http.MethodGet,
			Handler: h.search,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60),
			},
		},
		{
			Path:    "/api/v1/asset-rules/preview",
			Method:  http.MethodPost,
			Handler: h.previewRule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/asset-rules/",
			Method:  http.MethodPost,
			Handler: h.create,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/asset-rules/assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/asset-rules/{id}",
			Method:  http.MethodGet,
			Handler: h.get,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/asset-rules/{id}",
			Method:  http.MethodPut,
			Handler: h.update,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/asset-rules/{id}",
			Method:  http.MethodDelete,
			Handler: h.delete,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
	}
}
