package lineage

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	lineageService lineage.Service
	userService    user.Service
	authService    auth.Service
	config         *config.Config
}

func NewHandler(lineageService lineage.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{
		lineageService: lineageService,
		userService:    userService,
		authService:    authService,
		config:         config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/lineage/assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getAssetLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct",
			Method:  http.MethodPost,
			Handler: h.createDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct/{id}",
			Method:  http.MethodGet,
			Handler: h.getDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/lineage/batch",
			Method:  http.MethodPost,
			Handler: h.batchCreateLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		// OpenLineage endpoint
		{
			Path:    "/api/v1/lineage",
			Method:  http.MethodPost,
			Handler: h.ingestOpenLineageEvent,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
	}
}
