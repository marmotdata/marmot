package search

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	searchService     search.Service
	userService       user.Service
	authService       auth.Service
	metricsService    *metrics.Service
	config            *config.Config
	metamodelRegistry *metamodel.Registry
}

func NewHandler(
	searchService search.Service,
	userService user.Service,
	authService auth.Service,
	metricsService *metrics.Service,
	config *config.Config,
	metamodelRegistry *metamodel.Registry,
) *Handler {
	return &Handler{
		searchService:     searchService,
		userService:       userService,
		authService:       authService,
		metricsService:    metricsService,
		config:            config,
		metamodelRegistry: metamodelRegistry,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/search",
			Method:  http.MethodGet,
			Handler: h.search,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
	}
}
