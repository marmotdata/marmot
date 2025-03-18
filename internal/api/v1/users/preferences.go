package users

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/rs/zerolog/log"
)

// @Summary Get current user profile
// @Description Get detailed information about the currently authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} user.User
// @Failure 401 {object} common.ErrorResponse
// @Router /users/me [get]
func (h *Handler) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	fullUser, err := h.userService.Get(r.Context(), usr.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to get user details")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get user details")
		return
	}

	common.RespondJSON(w, http.StatusOK, fullUser)
}

// @Summary Update user preferences
// @Description Update preferences for the current user
// @Tags users
// @Accept json
// @Produce json
// @Param preferences body map[string]interface{} true "User preferences"
// @Success 200 "OK"
// @Failure 400 {object} common.ErrorResponse
// @Router /users/preferences [put]
func (h *Handler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input struct {
		Preferences map[string]interface{} `json:"preferences"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if theme, ok := input.Preferences["theme"].(string); ok {
		if theme != "light" && theme != "dark" && theme != "auto" {
			common.RespondError(w, http.StatusBadRequest, "Invalid theme value")
			return
		}
	}

	if err := h.userService.UpdatePreferences(r.Context(), usr.ID, input.Preferences); err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to update preferences")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	w.WriteHeader(http.StatusOK)
}
