package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/rs/zerolog/log"
)

// @Summary Initiate Okta OAuth login
// @Description Redirects the user to Okta for authentication
// @Tags auth
// @Produce json
// @Success 307 {string} string "Temporary Redirect"
// @Failure 500 {object} common.ErrorResponse
// @Router /auth/okta/login [get]
func (h *Handler) handleOktaLogin(w http.ResponseWriter, r *http.Request) {
	provider, exists := h.oauthManager.GetProvider("okta")
	if !exists {
		common.RespondError(w, http.StatusInternalServerError, "Okta provider not configured")
		return
	}

	state := generateRandomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURL := provider.GetAuthURL(state)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// @Summary Handle Okta OAuth callback
// @Description Processes the OAuth callback from Okta
// @Tags auth
// @Produce json
// @Param code query string true "Authorization code"
// @Param state query string true "State parameter for CSRF protection"
// @Success 307 {string} string "Temporary Redirect"
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /auth/okta/callback [get]
func (h *Handler) handleOktaCallback(w http.ResponseWriter, r *http.Request) {
	provider, exists := h.oauthManager.GetProvider("okta")
	if !exists {
		common.RespondError(w, http.StatusInternalServerError, "Okta provider not configured")
		return
	}

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "State cookie not found")
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		common.RespondError(w, http.StatusBadRequest, "State mismatch")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		common.RespondError(w, http.StatusBadRequest, "Code not found")
		return
	}

	user, err := provider.HandleCallback(r.Context(), code)
	if err != nil {
		log.Error().Err(err).Msg("Failed to handle Okta callback")
		common.RespondError(w, http.StatusInternalServerError, "Authentication failed")
		return
	}

	token, err := h.authService.GenerateToken(r.Context(), user, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		common.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	frontendURL := h.config.Server.RootURL + "/auth/callback?token=" + token
	http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
}

// generateRandomState generates a cryptographically secure random state string.
func generateRandomState() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Errorf("error generating random bytes: %w", err))
	}
	return base64.URLEncoding.EncodeToString(b)
}
