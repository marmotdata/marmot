package users

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/services/auth"
	"github.com/marmotdata/marmot/internal/services/user"
)

type Handler struct {
	userService user.Service
	authService auth.Service
}

func NewHandler(userService user.Service, authService auth.Service) *Handler {
	return &Handler{
		userService: userService,
		authService: authService,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/users",
			Method:  http.MethodGet,
			Handler: h.listUsers,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "view"),
			},
		},
		{
			Path:    "/api/v1/users",
			Method:  http.MethodPost,
			Handler: h.createUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodGet,
			Handler: h.getUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "view"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodPut,
			Handler: h.updateUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/login",
			Method:  http.MethodPost,
			Handler: h.login,
		},
		{
			Path:    "/api/v1/users/oauth/link",
			Method:  http.MethodPost,
			Handler: h.linkOAuthAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
			},
		},
		{
			Path:    "/api/v1/users/oauth/unlink/{id}/{provider}",
			Method:  http.MethodDelete,
			Handler: h.unlinkOAuthAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
			},
		},
		{
			Path:    "/api/v1/users/apikeys",
			Method:  http.MethodGet,
			Handler: h.listAPIKeys,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/apikeys",
			Method:  http.MethodPost,
			Handler: h.createAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/apikeys/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/me",
			Method:  http.MethodGet,
			Handler: h.getCurrentUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
			},
		},
		{
			Path:    "/api/v1/users/preferences",
			Method:  http.MethodPut,
			Handler: h.updatePreferences,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService),
			},
		},
	}
}
