package users

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type UpdatePasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type TokenResponse struct {
	AccessToken            string `json:"access_token"`
	TokenType              string `json:"token_type"`
	ExpiresIn              int64  `json:"expires_in"`
	RequiresPasswordChange bool   `json:"requires_password_change"`
}

type OAuthLinkRequest struct {
	UserID         string                 `json:"user_id" validate:"required"`
	Provider       string                 `json:"provider" validate:"required"`
	ProviderUserID string                 `json:"provider_user_id" validate:"required"`
	UserInfo       map[string]interface{} `json:"user_info" validate:"required"`
}

// @Summary Login user
// @Description Authenticate a user with username/email and password
// @Tags users
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 400 {object} common.ErrorResponse
// @Router /users/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	authenticatedUser, err := h.userService.Authenticate(r.Context(), input.Username, input.Password)
	if err != nil {
		switch err {
		case user.ErrInvalidPassword, user.ErrUnauthorized:
			common.RespondError(w, http.StatusUnauthorized, "Invalid credentials")
		default:
			log.Error().Err(err).Str("username", input.Username).Msg("Authentication failed")
			common.RespondError(w, http.StatusInternalServerError, "Authentication failed")
		}
		return
	}

	theme := ""
	if authenticatedUser.Preferences != nil {
		if themeVal, ok := authenticatedUser.Preferences["theme"].(string); ok {
			theme = themeVal
		}
	}

	extraClaims := map[string]interface{}{
		"theme": theme,
	}

	token, err := h.authService.GenerateToken(r.Context(), authenticatedUser, extraClaims)
	if err != nil {
		log.Error().Err(err).Str("user_id", authenticatedUser.ID).Msg("Failed to generate token")
		common.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := TokenResponse{
		AccessToken:            token,
		TokenType:              "Bearer",
		ExpiresIn:              24 * 60 * 60,
		RequiresPasswordChange: authenticatedUser.MustChangePassword,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Link OAuth account
// @Description Link an OAuth account to an existing user
// @Tags users
// @Accept json
// @Produce json
// @Param link body OAuthLinkRequest true "OAuth account link request"
// @Success 200 "OK"
// @Failure 400 {object} common.ErrorResponse
// @Router /users/oauth/link [post]
func (h *Handler) linkOAuthAccount(w http.ResponseWriter, r *http.Request) {
	var input OAuthLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.userService.LinkOAuthAccount(r.Context(), input.UserID, input.Provider, input.ProviderUserID, input.UserInfo)
	if err != nil {
		log.Error().Err(err).
			Str("user_id", input.UserID).
			Str("provider", input.Provider).
			Msg("Failed to link OAuth account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to link OAuth account")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// @Summary Unlink OAuth account
// @Description Unlink an OAuth account from a user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param provider path string true "OAuth provider"
// @Success 204 "No Content"
// @Failure 400 {object} common.ErrorResponse
// @Router /users/oauth/unlink/{id}/{provider} [delete]
func (h *Handler) unlinkOAuthAccount(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/users/oauth/unlink/"), "/")
	if len(parts) != 2 {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	userID := parts[0]
	provider := parts[1]

	err := h.userService.UnlinkOAuthAccount(r.Context(), userID, provider)
	if err != nil {
		log.Error().Err(err).
			Str("user_id", userID).
			Str("provider", provider).
			Msg("Failed to unlink OAuth account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to unlink OAuth account")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Update user password
// @Description Update current user's password
// @Tags users
// @Accept json
// @Produce json
// @Param request body UpdatePasswordRequest true "Password update request"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /users/update-password [post]
func (h *Handler) updatePassword(w http.ResponseWriter, r *http.Request) {
	var input UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	updatedUser, err := h.userService.UpdatePassword(r.Context(), usr.ID, input.NewPassword)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to update password")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	theme := ""
	if updatedUser.Preferences != nil {
		if themeVal, ok := updatedUser.Preferences["theme"].(string); ok {
			theme = themeVal
		}
	}

	extraClaims := map[string]interface{}{
		"theme": theme,
	}

	token, err := h.authService.GenerateToken(r.Context(), updatedUser, extraClaims)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	response := TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   24 * 60 * 60,
	}

	common.RespondJSON(w, http.StatusOK, response)
}
