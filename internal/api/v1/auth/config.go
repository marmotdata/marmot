package auth

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

type AuthConfig struct {
	EnabledProviders []string `json:"enabled_providers"`
}

// @Summary Get auth configuration
// @Description Returns the enabled auth providers without sensitive data
// @Tags auth
// @Produce json
// @Success 200 {object} AuthConfig
// @Router /auth/config [get]
func (h *Handler) getAuthConfig(w http.ResponseWriter, r *http.Request) {
	config := AuthConfig{
		EnabledProviders: make([]string, 0),
	}

	if _, exists := h.oauthManager.GetProvider("okta"); exists {
		config.EnabledProviders = append(config.EnabledProviders, "okta")
	}

	common.RespondJSON(w, http.StatusOK, config)
}
