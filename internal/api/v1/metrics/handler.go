package metrics

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/metrics"
)

type Handler struct {
	metricsService *metrics.Service
	userService    user.Service
	authService    auth.Service
	config         *config.Config
}

func NewHandler(metricsService *metrics.Service, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		metricsService: metricsService,
		userService:    userService,
		authService:    authService,
		config:         cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/metrics",
			Method:  http.MethodGet,
			Handler: h.getMetrics,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/top-queries",
			Method:  http.MethodGet,
			Handler: h.getTopQueries,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/top-assets",
			Method:  http.MethodGet,
			Handler: h.getTopAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/assets/total",
			Method:  http.MethodGet,
			Handler: h.getTotalAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/assets/by-type",
			Method:  http.MethodGet,
			Handler: h.getAssetsByType,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/assets/by-provider",
			Method:  http.MethodGet,
			Handler: h.getAssetsByProvider,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/assets/with-schemas",
			Method:  http.MethodGet,
			Handler: h.getAssetsWithSchemas,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
		{
			Path:    "/api/v1/metrics/assets/by-owner",
			Method:  http.MethodGet,
			Handler: h.getAssetsByOwner,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "metrics", "view"),
			},
		},
	}
}
