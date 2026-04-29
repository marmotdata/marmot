package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/rs/zerolog/log"
)

func (h *Handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ar, err := h.oauthProvider.NewAuthorizeRequest(ctx, r)
	if err != nil {
		log.Debug().Err(err).Msg("Invalid authorize request")
		h.oauthProvider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}

	for _, scope := range ar.GetRequestedScopes() {
		ar.GrantScope(scope)
	}

	sessionID := generateSessionID()
	h.authorizeSessionStore.Put(sessionID, ar)

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.config.Server.RootURL+"/login?oauth_pending=1", http.StatusFound)
}

func (h *Handler) HasPendingAuthorize(r *http.Request) bool {
	cookie, err := r.Cookie("oauth_session")
	if err != nil {
		return false
	}
	_, ok := h.authorizeSessionStore.Get(cookie.Value)
	return ok
}

func (h *Handler) CompleteAuthorize(w http.ResponseWriter, r *http.Request, userID, username string) (string, error) {
	cookie, err := r.Cookie("oauth_session")
	if err != nil {
		return "", err
	}

	pending, ok := h.authorizeSessionStore.Get(cookie.Value)
	if !ok {
		return "", errSessionNotFound
	}

	session := marmotOAuth2.NewMarmotSession(userID, username)

	resp, err := h.oauthProvider.NewAuthorizeResponse(r.Context(), pending.Request, session)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create authorize response")
		return "", err
	}

	redirectURI := pending.Request.GetRedirectURI()
	q := redirectURI.Query()
	for k, vs := range resp.GetParameters() {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	redirectURI.RawQuery = q.Encode()

	h.authorizeSessionStore.Delete(cookie.Value)

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return redirectURI.String(), nil
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (h *Handler) handleAuthorizeComplete(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		common.RespondError(w, http.StatusUnauthorized, "Authorization required")
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := h.authService.ValidateToken(r.Context(), tokenString)
	if err != nil {
		common.RespondError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	usr, err := h.userService.Get(r.Context(), claims.Subject)
	if err != nil {
		common.RespondError(w, http.StatusUnauthorized, "User not found")
		return
	}

	if !h.HasPendingAuthorize(r) {
		common.RespondError(w, http.StatusBadRequest, "No pending authorization")
		return
	}

	redirectURL, err := h.CompleteAuthorize(w, r, usr.ID, usr.Username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to complete OAuth authorize flow")
		common.RespondError(w, http.StatusInternalServerError, "Failed to complete authorization")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"oauth_redirect": redirectURL})
}

func (h *Handler) handleAuthorizePending(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("oauth_session")
	if err != nil {
		common.RespondError(w, http.StatusNotFound, "No pending authorization")
		return
	}
	pending, ok := h.authorizeSessionStore.Get(cookie.Value)
	if !ok {
		common.RespondError(w, http.StatusNotFound, "No pending authorization")
		return
	}

	redirectURI := pending.Request.GetRedirectURI()
	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"client_id":    pending.Request.GetClient().GetID(),
		"redirect_uri": redirectURI.String(),
		"scopes":       []string(pending.Request.GetRequestedScopes()),
	})
}

func (h *Handler) handleAuthorizeCancel(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("oauth_session"); err == nil {
		h.authorizeSessionStore.Delete(cookie.Value)
	}
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

var errSessionNotFound = fmt.Errorf("oauth session not found or expired")
