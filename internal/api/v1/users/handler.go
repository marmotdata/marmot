package users

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/mfa"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	mfa         *mfa.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(userService user.Service, authService auth.Service, cfg *config.Config, factors ...*mfa.Service) *Handler {
	h := &Handler{
		userService: userService,
		authService: authService,
		config:      cfg,
	}
	if len(factors) > 0 {
		h.mfa = factors[0]
	}
	return h
}

func (h *Handler) Routes() []common.Route {
	passwordCfg := *h.config
	passwordCfg.RateLimit.Enabled = true
	routes := []common.Route{
		{
			Path:    "/api/v1/users",
			Method:  http.MethodGet,
			Handler: h.listUsers,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "view"),
			},
		},
		{
			Path:    "/api/v1/users",
			Method:  http.MethodPost,
			Handler: h.createUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodGet,
			Handler: h.getUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "view"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodPut,
			Handler: h.updateUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/sign-out-all/{id}",
			Method:  http.MethodPost,
			Handler: h.signOutAllSessions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/login",
			Method:  http.MethodPost,
			Handler: h.login,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithRateLimit(h.config, 10, 60),
			},
		},
		{
			Path:    "/api/v1/users/oauth/link",
			Method:  http.MethodPost,
			Handler: h.linkOAuthAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/oauth/unlink/{id}/{provider}",
			Method:  http.MethodDelete,
			Handler: h.unlinkOAuthAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/apikeys",
			Method:  http.MethodGet,
			Handler: h.listAPIKeys,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/apikeys",
			Method:  http.MethodPost,
			Handler: h.createAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/apikeys/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "users", "manage"),
			},
		},
		{
			Path:    "/api/v1/users/me",
			Method:  http.MethodGet,
			Handler: h.getCurrentUser,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/users/search",
			Method:  http.MethodGet,
			Handler: h.searchUsers,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/users/preferences",
			Method:  http.MethodPut,
			Handler: h.updatePreferences,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    common.UpdatePasswordPath,
			Method:  http.MethodPost,
			Handler: h.updatePassword,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequireJWTSession("local"),
				common.WithRateLimit(h.config, 10, 60),
			},
		},
		{
			Path: "/api/v1/users/change-password", Method: http.MethodPost, Handler: h.changeOwnPassword,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{common.WithAuth(h.userService, h.authService, h.config), common.RequireJWTSession("local"), common.WithRateLimit(&passwordCfg, 10, 60)},
		},
		{
			Path: "/api/v1/users/password/require-change/{id}", Method: http.MethodPost, Handler: h.requirePasswordChange,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{common.WithAuth(h.userService, h.authService, h.config), common.RequirePermission(h.userService, "users", "manage")},
		},
	}
	if h.config.Auth.TOTP.Enabled {
		routes = append(routes, h.totpRoutes()...)
	}
	return routes
}
