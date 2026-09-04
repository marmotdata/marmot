package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/rs/zerolog/log"
)

const loginHandoffCookieName = "marmot_login_ticket"

// issueLoginHandoff parks the user an SSO callback just authenticated under a
// one-time ticket, so the page it redirects to can trade the ticket for a
// token. The ticket goes out in a cookie; nothing but the user id is stored.
func (h *Handler) issueLoginHandoff(w http.ResponseWriter, r *http.Request, userID string) error {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating handoff ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)

	if err := h.oauthProvider.Store.PutLoginHandoff(r.Context(), ticket, userID); err != nil {
		return fmt.Errorf("storing handoff ticket: %w", err)
	}

	isSecure := h.cookiesAreSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     loginHandoffCookieName,
		Value:    ticket,
		Path:     "/",
		MaxAge:   int(marmotOAuth2.LoginHandoffTTL.Seconds()),
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *Handler) handleLoginExchange(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(loginHandoffCookieName)
	if err != nil || cookie.Value == "" {
		common.RespondError(w, http.StatusUnauthorized, "No login handoff in progress")
		return
	}

	// Redeem first, so a failure later cannot leave the ticket usable.
	userID, takeErr := h.oauthProvider.Store.TakeLoginHandoff(r.Context(), cookie.Value)

	isSecure := h.cookiesAreSecure(r)
	http.SetCookie(w, &http.Cookie{
		Name:     loginHandoffCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if takeErr != nil {
		if !errors.Is(takeErr, marmotOAuth2.ErrNoRecord) {
			log.Error().Err(takeErr).Msg("Failed to redeem login handoff ticket")
		}
		common.RespondError(w, http.StatusUnauthorized, "Login handoff expired or already used")
		return
	}

	usr, err := h.userService.Get(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to look up user for login handoff")
		common.RespondError(w, http.StatusUnauthorized, "Login handoff expired or already used")
		return
	}

	// Minted here rather than at the callback, so no token is ever written to
	// the database.
	token, err := h.authService.GenerateToken(r.Context(), usr, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate token for login handoff")
		common.RespondError(w, http.StatusInternalServerError, "Failed to complete login")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"access_token": token})
}
