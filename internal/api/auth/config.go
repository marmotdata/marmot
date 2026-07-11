package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	coreauth "github.com/marmotdata/marmot/internal/core/auth"
)

type AuthConfig struct {
	EnabledProviders []string `json:"enabled_providers"`
} // @name AuthConfig

// @Summary Get auth configuration
// @Description Returns the enabled auth providers without sensitive data
// @Tags auth
// @Produce json
// @Success 200 {object} AuthConfig
// @Router /auth-providers [get]
func (h *Handler) getAuthConfig(w http.ResponseWriter, r *http.Request) {
	config := AuthConfig{
		EnabledProviders: h.oauthManager.GetProviderNames(),
	}

	common.RespondJSON(w, http.StatusOK, config)
}

// SSOProvider is an admin-facing view of a configured SSO provider.
type SSOProvider struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	IssuerURL string `json:"issuer_url,omitempty"`
} // @name SSOProvider

type ssoProvidersResponse struct {
	Providers []SSOProvider `json:"providers"`
} // @name SSOProvidersResponse

// @Summary List configured SSO providers (admin)
// @Description Read-only view of SSO providers wired via server config. Editing is done in config.yaml.
// @Tags auth
// @Produce json
// @Success 200 {object} ssoProvidersResponse
// @Router /sso-providers [get]
func (h *Handler) getSSOProviders(w http.ResponseWriter, r *http.Request) {
	out := ssoProvidersResponse{Providers: []SSOProvider{}}
	for _, p := range h.oauthManager.GetProviders() {
		item := SSOProvider{Type: p.Type(), Name: p.Name()}
		if ip, ok := p.(coreauth.IssuerProvider); ok {
			item.IssuerURL = ip.IssuerURL()
		}
		out.Providers = append(out.Providers, item)
	}
	common.RespondJSON(w, http.StatusOK, out)
}
