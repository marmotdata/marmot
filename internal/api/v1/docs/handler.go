package docs

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/user"
)

// Handler handles documentation API requests
type Handler struct {
	docsService *docs.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

// NewHandler creates a new documentation handler
func NewHandler(
	docsService *docs.Service,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		docsService: docsService,
		userService: userService,
		authService: authService,
		config:      config,
	}
}

// Routes returns the routes for the documentation API
func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/docs/entity/{entityType}/{entityId}/pages",
			Method:  http.MethodGet,
			Handler: h.getPageTree,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/docs/entity/{entityType}/{entityId}/pages",
			Method:  http.MethodPost,
			Handler: h.createPage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/docs/entity/{entityType}/{entityId}/stats",
			Method:  http.MethodGet,
			Handler: h.getStorageStats,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/docs/entity/{entityType}/{entityId}/search",
			Method:  http.MethodGet,
			Handler: h.searchPages,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},

		{
			Path:    "/api/v1/docs/pages/{pageId}",
			Method:  http.MethodGet,
			Handler: h.getPage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/docs/pages/{pageId}",
			Method:  http.MethodPut,
			Handler: h.updatePage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/docs/pages/{pageId}",
			Method:  http.MethodDelete,
			Handler: h.deletePage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/docs/pages/{pageId}/move",
			Method:  http.MethodPut,
			Handler: h.movePage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},

		{
			Path:    "/api/v1/docs/pages/{pageId}/images",
			Method:  http.MethodGet,
			Handler: h.listPageImages,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/docs/pages/{pageId}/images",
			Method:  http.MethodPost,
			Handler: h.uploadImage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
				common.WithRateLimit(h.config, 30, 60), // Rate limit uploads
			},
		},
		{
			Path:    "/api/v1/docs/images/{imageId}",
			Method:  http.MethodGet,
			Handler: h.getImage,
		},
		{
			Path:    "/api/v1/docs/images/{imageId}",
			Method:  http.MethodDelete,
			Handler: h.deleteImage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
	}
}
