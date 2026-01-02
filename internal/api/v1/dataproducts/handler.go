package dataproducts

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	dataProductService dataproduct.Service
	userService        user.Service
	authService        auth.Service
	config             *config.Config
}

func NewHandler(
	dataProductService dataproduct.Service,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		dataProductService: dataProductService,
		userService:        userService,
		authService:        authService,
		config:             config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/products/list",
			Method:  http.MethodGet,
			Handler: h.list,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 100, 60),
			},
		},
		{
			Path:    "/api/v1/products/search",
			Method:  http.MethodGet,
			Handler: h.search,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
				common.WithRateLimit(h.config, 50, 60),
			},
		},
		{
			Path:    "/api/v1/products/",
			Method:  http.MethodPost,
			Handler: h.create,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/rule-preview",
			Method:  http.MethodPost,
			Handler: h.previewRule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/assets/{id}",
			Method:  http.MethodPost,
			Handler: h.addAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/assets/{id}/{assetId}",
			Method:  http.MethodDelete,
			Handler: h.removeAsset,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/rules/{id}",
			Method:  http.MethodGet,
			Handler: h.getRules,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/rules/{id}",
			Method:  http.MethodPost,
			Handler: h.createRule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/rules/{id}/{ruleId}",
			Method:  http.MethodPut,
			Handler: h.updateRule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/rules/{id}/{ruleId}",
			Method:  http.MethodDelete,
			Handler: h.deleteRule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/resolved-assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getResolvedAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/{id}",
			Method:  http.MethodGet,
			Handler: h.get,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/{id}",
			Method:  http.MethodPut,
			Handler: h.update,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/{id}",
			Method:  http.MethodDelete,
			Handler: h.delete,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/images/{id}/{purpose}",
			Method:  http.MethodPost,
			Handler: h.uploadImage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/images/{id}/{purpose}",
			Method:  http.MethodGet,
			Handler: h.getImage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/products/images/{id}/{purpose}",
			Method:  http.MethodDelete,
			Handler: h.deleteImage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/products/images/{id}",
			Method:  http.MethodGet,
			Handler: h.listImages,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
	}
}
