package serviceaccounts

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	svcService  serviceaccount.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(svcService serviceaccount.Service, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		svcService:  svcService,
		userService: userService,
		authService: authService,
		config:      cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/service-accounts",
			Method:  http.MethodGet,
			Handler: h.listServiceAccounts,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "view"),
			},
		},
		{
			Path:    "/api/v1/service-accounts",
			Method:  http.MethodPost,
			Handler: h.createServiceAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "manage"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}",
			Method:  http.MethodGet,
			Handler: h.getServiceAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "view"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}",
			Method:  http.MethodPatch,
			Handler: h.updateServiceAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "manage"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteServiceAccount,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "manage"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}/api-keys",
			Method:  http.MethodGet,
			Handler: h.listAPIKeys,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "view"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}/api-keys",
			Method:  http.MethodPost,
			Handler: h.createAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "manage"),
			},
		},
		{
			Path:    "/api/v1/service-accounts/{id}/api-keys/{keyId}",
			Method:  http.MethodDelete,
			Handler: h.deleteAPIKey,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "service_accounts", "manage"),
			},
		},
	}
}
