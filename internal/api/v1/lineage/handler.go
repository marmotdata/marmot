package lineage

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/services/auth"
	"github.com/marmotdata/marmot/internal/services/lineage"
	"github.com/marmotdata/marmot/internal/services/user"
)

type Handler struct {
	lineageService lineage.Service
	userService    user.Service
	authService    auth.Service
}

func NewHandler(lineageService lineage.Service, userService user.Service, authService auth.Service) *Handler {
	return &Handler{
		lineageService: lineageService,
		userService:    userService,
		authService:    authService,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/lineage/assets/{id}",
			Method:  http.MethodGet,
			Handler: h.getAssetLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct",
			Method:  http.MethodPost,
			Handler: h.createDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:    "/api/v1/lineage/direct/{id}",
			Method:  http.MethodGet,
			Handler: h.getDirectLineage,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		},
	}
}
