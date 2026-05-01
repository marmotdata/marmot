package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	coreauth "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	fositeLib "github.com/ory/fosite"
	"github.com/rs/zerolog/log"
)

// RFC 8693 grant type and token type identifiers.
const (
	grantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"  //nolint:gosec // OAuth URI, not a credential
	tokenTypeIDToken       = "urn:ietf:params:oauth:token-type:id_token"        //nolint:gosec // OAuth URI, not a credential
	tokenTypeAccessToken   = "urn:ietf:params:oauth:token-type:access_token"    //nolint:gosec // OAuth URI, not a credential
)

// tokenExchangeResponse is the RFC 8693 Section 2.2.1 token response.
type tokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
}

// oauthErrorResponse is the RFC 6749 Section 5.2 error response.
type oauthErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func respondOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(oauthErrorResponse{
		Error:       code,
		Description: description,
	})
}

// handleToken dispatches to the appropriate grant handler.
//
//	@Summary		OAuth token endpoint
//	@Description	Handles authorization_code grants (with PKCE) and token exchange (RFC 8693).
//	@Description	For token-exchange, supported subject_token_type values are
//	@Description	urn:ietf:params:oauth:token-type:id_token and urn:ietf:params:oauth:token-type:access_token.
//	@Tags			auth
//	@Accept			application/x-www-form-urlencoded
//	@Produce		json
//	@Param			grant_type			formData	string	true	"authorization_code or urn:ietf:params:oauth:grant-type:token-exchange"
//	@Param			subject_token		formData	string	false	"Token to exchange (token-exchange grant only)"
//	@Param			subject_token_type	formData	string	false	"id_token or access_token URI (token-exchange grant only)"
//	@Success		200					{object}	tokenExchangeResponse
//	@Failure		400					{object}	oauthErrorResponse
//	@Failure		401					{object}	oauthErrorResponse
//	@Failure		500					{object}	oauthErrorResponse
//	@Router			/oauth/token [post]
func (h *Handler) handleToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := r.ParseForm(); err != nil {
		respondOAuthError(w, http.StatusBadRequest, "invalid_request", "Could not parse form body")
		return
	}

	grantType := r.FormValue("grant_type")
	switch grantType {
	case "authorization_code":
		h.handleAuthorizationCodeGrant(w, r)
	case grantTypeTokenExchange:
		h.handleTokenExchangeGrant(w, r, r.FormValue("subject_token"), r.FormValue("subject_token_type"))
	default:
		respondOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("unsupported grant_type: %s", grantType))
	}
}

// handleAuthorizationCodeGrant validates the code and PKCE verifier, then issues a Marmot JWT.
func (h *Handler) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := &marmotOAuth2.MarmotSession{
		ExpiresAt: make(map[fositeLib.TokenType]time.Time),
	}

	accessReq, err := h.oauthProvider.NewAccessRequest(ctx, r, session)
	if err != nil {
		log.Debug().Err(err).Msg("Invalid access request")
		h.oauthProvider.WriteAccessError(ctx, w, accessReq, err)
		return
	}

	mSession, ok := accessReq.GetSession().(*marmotOAuth2.MarmotSession)
	if !ok || mSession.UserID == "" {
		respondOAuthError(w, http.StatusInternalServerError, "server_error", "Session missing user identity")
		return
	}

	usr, err := h.userService.Get(ctx, mSession.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", mSession.UserID).Msg("Failed to look up user for auth code grant")
		respondOAuthError(w, http.StatusInternalServerError, "server_error", "Failed to look up user")
		return
	}

	token, err := h.authService.GenerateToken(ctx, usr, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		respondOAuthError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   86400,
	})
}

// handleTokenExchangeGrant handles the RFC 8693 token exchange grant type.
func (h *Handler) handleTokenExchangeGrant(w http.ResponseWriter, r *http.Request, subjectToken, subjectTokenType string) {
	if subjectToken == "" {
		respondOAuthError(w, http.StatusBadRequest, "invalid_request", "subject_token is required")
		return
	}

	switch subjectTokenType {
	case tokenTypeIDToken:
		h.exchangeViaIDToken(w, r, subjectToken)
	case tokenTypeAccessToken:
		h.exchangeViaAccessToken(w, r, subjectToken)
	default:
		respondOAuthError(w, http.StatusBadRequest, "invalid_request",
			fmt.Sprintf("subject_token_type must be %s or %s", tokenTypeIDToken, tokenTypeAccessToken))
	}
}

// exchangeViaIDToken validates an ID token against each provider's JWKS until one succeeds.
func (h *Handler) exchangeViaIDToken(w http.ResponseWriter, r *http.Request, subjectToken string) {
	for _, provider := range h.oauthManager.GetProviders() {
		te, ok := provider.(coreauth.TokenExchanger)
		if !ok {
			continue
		}
		usr, err := te.ExchangeToken(r.Context(), subjectToken)
		if err != nil {
			log.Debug().Err(err).
				Str("provider", provider.Type()).
				Msg("ID token exchange attempt failed, trying next provider")
			continue
		}
		h.respondWithMarmotToken(w, r, usr)
		return
	}

	respondOAuthError(w, http.StatusUnauthorized, "invalid_grant",
		"No configured OIDC provider could verify the token")
}

// exchangeViaAccessToken validates an access token via each provider's UserInfo endpoint.
func (h *Handler) exchangeViaAccessToken(w http.ResponseWriter, r *http.Request, subjectToken string) {
	for _, provider := range h.oauthManager.GetProviders() {
		ate, ok := provider.(coreauth.AccessTokenExchanger)
		if !ok {
			continue
		}
		usr, err := ate.ExchangeAccessToken(r.Context(), subjectToken)
		if err != nil {
			log.Debug().Err(err).
				Str("provider", provider.Type()).
				Msg("Access token exchange attempt failed, trying next provider")
			continue
		}
		h.respondWithMarmotToken(w, r, usr)
		return
	}

	respondOAuthError(w, http.StatusUnauthorized, "invalid_grant",
		"No configured OIDC provider could verify the token")
}

// respondWithMarmotToken issues a Marmot JWT for the resolved user and writes the RFC 8693 response.
func (h *Handler) respondWithMarmotToken(w http.ResponseWriter, r *http.Request, usr *user.User) {
	token, err := h.authService.GenerateToken(r.Context(), usr, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token")
		respondOAuthError(w, http.StatusInternalServerError, "server_error", "Failed to generate token")
		return
	}

	common.RespondJSON(w, http.StatusOK, tokenExchangeResponse{
		AccessToken:     token,
		IssuedTokenType: tokenTypeAccessToken,
		TokenType:       "Bearer",
	})
}
