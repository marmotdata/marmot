package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	authService           auth.Service
	userService           user.Service
	oauthManager          *auth.OAuthManager
	oauthProvider         *marmotOAuth2.Provider
	authorizeSessionStore *marmotOAuth2.AuthorizeSessionStore
	loginHandoffStore     *LoginHandoffStore
	config                *config.Config
}

func NewHandler(authService auth.Service, oauthManager *auth.OAuthManager, userService user.Service, config *config.Config, oauthProvider *marmotOAuth2.Provider, authorizeSessionStore *marmotOAuth2.AuthorizeSessionStore) *Handler {
	return &Handler{
		authService:           authService,
		userService:           userService,
		oauthManager:          oauthManager,
		oauthProvider:         oauthProvider,
		authorizeSessionStore: authorizeSessionStore,
		loginHandoffStore:     NewLoginHandoffStore(),
		config:                config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/.well-known/oauth-protected-resource",
			Method:  http.MethodGet,
			Handler: h.handleProtectedResourceMetadata,
		},
		{
			Path:    "/.well-known/oauth-authorization-server",
			Method:  http.MethodGet,
			Handler: h.handleASMetadata,
		},
		{
			Path:    "/auth-providers",
			Method:  http.MethodGet,
			Handler: h.getAuthConfig,
		},
		{
			Path:    "/auth/callback",
			Method:  http.MethodGet,
			Handler: h.handleAuthCallback,
		},
		{
			Path:    "/auth/exchange",
			Method:  http.MethodPost,
			Handler: h.handleLoginExchange,
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
		{
			Path:    "/oauth/authorize",
			Method:  http.MethodGet,
			Handler: h.handleAuthorize,
		},
		{
			Path:    "/oauth/register",
			Method:  http.MethodPost,
			Handler: h.handleDCR,
		},
		{
			Path:    "/oauth/authorize/complete",
			Method:  http.MethodPost,
			Handler: h.handleAuthorizeComplete,
		},
		{
			Path:    "/oauth/authorize/pending",
			Method:  http.MethodGet,
			Handler: h.handleAuthorizePending,
		},
		{
			Path:    "/oauth/authorize/cancel",
			Method:  http.MethodPost,
			Handler: h.handleAuthorizeCancel,
		},
		{
			Path:    "/oauth/token",
			Method:  http.MethodPost,
			Handler: h.handleToken,
		},
	}
}
