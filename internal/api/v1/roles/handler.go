package roles

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	roleService role.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(roleService role.Service, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		roleService: roleService,
		userService: userService,
		authService: authService,
		config:      cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	authMiddleware := common.WithAuth(h.userService, h.authService, h.config)
	requireManage := common.RequirePermission(h.userService, "roles", "manage")

	return []common.Route{
		{
			Path:    "/api/v1/roles",
			Method:  http.MethodGet,
			Handler: h.listRoles,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/roles",
			Method:  http.MethodPost,
			Handler: h.createRole,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/roles/{id}",
			Method:  http.MethodGet,
			Handler: h.getRole,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/roles/{id}",
			Method:  http.MethodPatch,
			Handler: h.updateRole,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/roles/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteRole,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/roles/{id}/permissions",
			Method:  http.MethodPost,
			Handler: h.replacePermissions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
		{
			Path:    "/api/v1/permissions",
			Method:  http.MethodGet,
			Handler: h.listPermissions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware, requireManage,
			},
		},
	}
}
