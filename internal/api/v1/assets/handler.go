package assets

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/metrics"
)

type Handler struct {
	assetService     asset.Service
	assetDocsService assetdocs.Service
	userService      user.Service
	authService      auth.Service
	metricsService   *metrics.Service
	runService       runs.Service
	teamService      *team.Service
	config           *config.Config
}

func NewHandler(
	assetService asset.Service,
	assetDocsService assetdocs.Service,
	userService user.Service,
	authService auth.Service,
	metricsService *metrics.Service,
	runService runs.Service,
	teamService *team.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		assetService:     assetService,
		assetDocsService: assetDocsService,
		userService:      userService,
		authService:      authService,
		metricsService:   metricsService,
		runService:       runService,
		teamService:      teamService,
		config:           config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/assets/",
			Method:  http.MethodPost,
			Handler: h.createAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/{id}",
			Method:  http.MethodPut,
			Handler: h.updateAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/search",
			Method:  http.MethodGet,
			Handler: h.searchAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/my-assets",
			Method:  http.MethodGet,
			Handler: h.getMyAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/summary",
			Method:  http.MethodGet,
			Handler: h.summaryAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 30, 60), // 30 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/qualified-name/",
			Method:  http.MethodGet,
			Handler: h.getAssetByMRN,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/tags/",
			Method:  http.MethodPost,
			Handler: h.addTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/tags/",
			Method:  http.MethodDelete,
			Handler: h.removeTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/documentation/",
			Method:  http.MethodGet,
			Handler: h.getAssetDocumentation,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/documentation/",
			Method:  http.MethodPost,
			Handler: h.createAssetDocumentation,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/documentation/batch",
			Method:  http.MethodPost,
			Handler: h.batchCreateDocumentation,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/match-pattern/",
			Method:  http.MethodGet,
			Handler: h.matchAssetPattern,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/suggestions/metadata/fields",
			Method:  http.MethodGet,
			Handler: h.getMetadataFieldSuggestions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/suggestions/metadata/values",
			Method:  http.MethodGet,
			Handler: h.getMetadataValueSuggestions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/suggestions/tags",
			Method:  http.MethodGet,
			Handler: h.getTagSuggestions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60), // 50 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/lookup/{type}/{name}",
			Method:  http.MethodGet,
			Handler: h.lookupAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/run-history/{id}",
			Method:  http.MethodGet,
			Handler: h.getRunHistory,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 30, 60), // 30 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/assets/run-history-histogram/{id}",
			Method:  http.MethodGet,
			Handler: h.getRunHistoryHistogram,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 30, 60), // 30 requests per 60 seconds
			},
		},
		// Term associations
		{
			Path:    "/api/v1/assets/terms/",
			Method:  http.MethodPost,
			Handler: h.addTerms,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/terms/",
			Method:  http.MethodDelete,
			Handler: h.removeTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/terms/",
			Method:  http.MethodGet,
			Handler: h.getAssetTerms,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/by-glossary-term/{term_id}",
			Method:  http.MethodGet,
			Handler: h.getAssetsByTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/owners/",
			Method:  http.MethodGet,
			Handler: h.listAssetOwners,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/assets/owners/",
			Method:  http.MethodPost,
			Handler: h.addAssetOwner,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/assets/owners/",
			Method:  http.MethodDelete,
			Handler: h.removeAssetOwner,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
	}
}
