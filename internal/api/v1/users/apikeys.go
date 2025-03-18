package users

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/rs/zerolog/log"
)

type CreateAPIKeyRequest struct {
	Name          string `json:"name" validate:"required"`
	ExpiresInDays int    `json:"expires_in_days"`
}

// @Summary List API keys
// @Description Get all API keys for a user
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {array} user.APIKey
// @Failure 500 {object} common.ErrorResponse
// @Router /users/apikeys [get]
func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	keys, err := h.userService.ListAPIKeys(r.Context(), usr.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to list API keys")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}

	common.RespondJSON(w, http.StatusOK, keys)
}

// @Summary Create API key
// @Description Create a new API key for a user
// @Tags users
// @Accept json
// @Produce json
// @Param key body CreateAPIKeyRequest true "API key creation request"
// @Success 200 {object} user.APIKey
// @Failure 400 {object} common.ErrorResponse
// @Router /users/apikeys [post]
func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var expiresIn *time.Duration
	if input.ExpiresInDays > 0 {
		duration := time.Duration(input.ExpiresInDays) * 24 * time.Hour
		expiresIn = &duration
	}

	key, err := h.userService.CreateAPIKey(r.Context(), usr.ID, input.Name, expiresIn)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to create API key")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	common.RespondJSON(w, http.StatusOK, key)
}

// @Summary Delete API key
// @Description Delete an API key
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "API key ID"
// @Success 204 "No Content"
// @Failure 400 {object} common.ErrorResponse
// @Router /users/apikeys/{id} [delete]
func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := strings.TrimPrefix(r.URL.Path, "/api/v1/users/apikeys/")
	if keyID == "" {
		common.RespondError(w, http.StatusBadRequest, "API key ID required")
		return
	}

	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	err := h.userService.DeleteAPIKey(r.Context(), usr.ID, keyID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Str("key_id", keyID).Msg("Failed to delete API key")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete API key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
