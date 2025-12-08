package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
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
			Path:    "/auth-providers",
			Method:  http.MethodGet,
			Handler: h.getAuthConfig,
		},
		{
			Path:    "/auth/{provider}/login",
			Method:  http.MethodGet,
			Handler: h.handleOAuthLogin,
		},
		{
			Path:    "/auth/{provider}/callback",
			Method:  http.MethodGet,
			Handler: h.handleOAuthCallback,
		},
	}
}
