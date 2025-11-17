package glossary

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	glossaryService glossary.Service
	userService     user.Service
	authService     auth.Service
	config          *config.Config
}

func NewHandler(
	glossaryService glossary.Service,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		glossaryService: glossaryService,
		userService:     userService,
		authService:     authService,
		config:          config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/glossary/list",
			Method:  http.MethodGet,
			Handler: h.listTerms,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "view"),
				common.WithRateLimit(h.config, 100, 60),
			},
		},
		{
			Path:    "/api/v1/glossary/search",
			Method:  http.MethodGet,
			Handler: h.searchTerms,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "view"),
				common.WithRateLimit(h.config, 50, 60),
			},
		},
		{
			Path:    "/api/v1/glossary/",
			Method:  http.MethodPost,
			Handler: h.createTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "manage"),
			},
		},
		{
			Path:    "/api/v1/glossary/children/{id}",
			Method:  http.MethodGet,
			Handler: h.getChildren,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "view"),
			},
		},
		{
			Path:    "/api/v1/glossary/ancestors/{id}",
			Method:  http.MethodGet,
			Handler: h.getAncestors,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "view"),
			},
		},
		{
			Path:    "/api/v1/glossary/{id}",
			Method:  http.MethodGet,
			Handler: h.getTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "view"),
			},
		},
		{
			Path:    "/api/v1/glossary/{id}",
			Method:  http.MethodPut,
			Handler: h.updateTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "manage"),
			},
		},
		{
			Path:    "/api/v1/glossary/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteTerm,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "glossary", "manage"),
			},
		},
	}
}
