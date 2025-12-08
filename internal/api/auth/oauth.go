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

// @Summary Initiate OAuth login
// @Description Redirects the user to the OAuth provider for authentication
// @Tags auth
// @Produce json
// @Param provider path string true "OAuth provider (okta, google, github, etc.)"
// @Param redirect_uri query string false "Custom redirect URI for CLI login"
// @Success 307 {string} string "Temporary Redirect"
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /auth/{provider}/login [get]
func (h *Handler) handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	if providerName == "" {
		common.RespondError(w, http.StatusBadRequest, "Provider name is required")
		return
	}

	provider, exists := h.oauthManager.GetProvider(providerName)
	if !exists {
		common.RespondError(w, http.StatusNotFound, fmt.Sprintf("OAuth provider '%s' not configured", providerName))
		return
	}

	state := generateRandomState()

	// Use Secure flag only for HTTPS requests
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_redirect_uri",
			Value:    redirectURI,
			MaxAge:   int(time.Hour.Seconds()),
			Secure:   isSecure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	redirectURL := provider.GetAuthURL(state)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// @Summary Handle OAuth callback
// @Description Processes the OAuth callback from any provider
// @Tags auth
// @Produce json
// @Param provider path string true "OAuth provider (okta, google, github, etc.)"
// @Param code query string true "Authorization code"
// @Param state query string true "State parameter for CSRF protection"
// @Success 307 {string} string "Temporary Redirect"
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /auth/{provider}/callback [get]
func (h *Handler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("provider")
	if providerName == "" {
		common.RespondError(w, http.StatusBadRequest, "Provider name is required")
		return
	}

	provider, exists := h.oauthManager.GetProvider(providerName)
	if !exists {
		common.RespondError(w, http.StatusNotFound, fmt.Sprintf("OAuth provider '%s' not configured", providerName))
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
		log.Error().Err(err).Str("provider", providerName).Msg("Failed to handle OAuth callback")
		common.RespondError(w, http.StatusInternalServerError, "Authentication failed")
		return
	}

	token, err := h.authService.GenerateToken(r.Context(), user, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		common.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	redirectURICookie, err := r.Cookie("oauth_redirect_uri")
	var frontendURL string
	if err == nil && redirectURICookie.Value != "" {
		isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_redirect_uri",
			Value:    "",
			MaxAge:   -1,
			Secure:   isSecure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		frontendURL = redirectURICookie.Value + "?token=" + token
	} else {
		frontendURL = h.config.Server.RootURL + "/auth/callback?token=" + token
	}

	http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
}

func generateRandomState() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Errorf("error generating random bytes: %w", err))
	}
	return base64.URLEncoding.EncodeToString(b)
}
