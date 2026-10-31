package users

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

// CurrentUserResponse is the signed-in user with the profile fields only the account page needs.
type CurrentUserResponse struct {
	*user.User
	JobTitle    string                   `json:"job_title"`
	Teams       []user.ProfileTeam       `json:"teams"`
	RoleSources []user.ProfileRoleSource `json:"role_sources"`
} // @name CurrentUser

// UpdateProfileRequest carries the profile fields a user may change about themselves.
type UpdateProfileRequest struct {
	JobTitle string `json:"job_title"`
} // @name UpdateProfileRequest

// withProfile adds job title and teams to the user; a failure there is logged and left out so the account page still loads.
func (h *Handler) withProfile(r *http.Request, u *user.User) CurrentUserResponse {
	resp := CurrentUserResponse{User: u, Teams: []user.ProfileTeam{}, RoleSources: []user.ProfileRoleSource{}}
	profile, err := h.userService.GetProfile(r.Context(), u.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", u.ID).Msg("Failed to load user profile")
		return resp
	}
	resp.JobTitle = profile.JobTitle
	resp.Teams = profile.Teams
	resp.RoleSources = profile.RoleSources
	return resp
}

// @Summary Update current user profile
// @Description Update the profile fields the current user may set about themselves, such as job title
// @Tags users
// @Accept json
// @Produce json
// @Param profile body UpdateProfileRequest true "Profile fields"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} CurrentUserResponse
// @Failure 400 {object} common.ValidationErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @ID putUsersMeProfile
// @Router /api/v1/users/me/profile [put]
func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input UpdateProfileRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.userService.UpdateJobTitle(r.Context(), usr.ID, input.JobTitle); err != nil {
		if respondUserError(w, err) {
			return
		}
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to update profile")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	fullUser, err := h.userService.Get(r.Context(), usr.ID)
	if err != nil {
		log.Error().Err(err).Str("user_id", usr.ID).Msg("Failed to get user details")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get user details")
		return
	}
	common.RespondJSON(w, http.StatusOK, h.withProfile(r, fullUser))
}

// @Summary Rotate API key
// @Description Issue a new secret for one of the current user's API keys, keeping its name and expiry. The old secret stops working at once and the new key is returned only in this response.
// @Tags users
// @Produce json
// @Param id path string true "API key ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} user.APIKey
// @Failure 404 {object} common.ErrorResponse
// @ID postUsersApikeysIDRotate
// @Router /api/v1/users/apikeys/{id}/rotate [post]
func (h *Handler) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	keyID := r.PathValue("id")
	key, err := h.userService.RotateAPIKey(r.Context(), usr.ID, keyID)
	if err != nil {
		// Same answer for absent and not-yours, as on delete, so ids cannot be probed.
		if errors.Is(err, user.ErrUserNotFound) || errors.Is(err, user.ErrUnauthorized) {
			common.RespondError(w, http.StatusNotFound, "API key not found")
			return
		}
		log.Error().Err(err).Str("user_id", usr.ID).Str("key_id", keyID).Msg("Failed to rotate API key")
		common.RespondError(w, http.StatusInternalServerError, "Failed to rotate API key")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	common.RespondJSON(w, http.StatusOK, key)
}
