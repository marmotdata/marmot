package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/services/auth"
	"github.com/marmotdata/marmot/internal/services/user"
)

type Handler struct {
	authService  auth.Service
	userService  user.Service
	oauthManager *auth.OAuthManager
	config       *config.Config
}

func NewHandler(authService auth.Service, oauthManager *auth.OAuthManager, userService user.Service, config *config.Config) *Handler {
	return &Handler{
		authService:  authService,
		userService:  userService,
		oauthManager: oauthManager,
		config:       config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/auth/config",
			Method:  http.MethodGet,
			Handler: h.getAuthConfig,
		},
		{
			Path:    "/api/v1/auth/okta/login",
			Method:  http.MethodGet,
			Handler: h.handleOktaLogin,
			// Middleware: []func(http.HandlerFunc) http.HandlerFunc{
			// 	common.WithAuth(h.userService, h.authService),
			// },
		},
		{
			Path:    "/api/v1/auth/okta/callback",
			Method:  http.MethodGet,
			Handler: h.handleOktaCallback,
			// Middleware: []func(http.HandlerFunc) http.HandlerFunc{
			// 	common.WithAuth(h.userService, h.authService),
			// },
		},
	}
}
