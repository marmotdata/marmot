package users

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type ListUsersResponse struct {
	Users  []*user.User `json:"users"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

// @Summary List users
// @Description Get a list of users with optional filtering
// @Tags users
// @Accept json
// @Produce json
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Param query query string false "Search query for username or email"
// @Param role_ids query []string false "Filter by role IDs"
// @Param active query bool false "Filter by active status"
// @Success 200 {object} ListUsersResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /users [get]
func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	filter := user.Filter{
		Limit:  50,
		Offset: 0,
		Query:  r.URL.Query().Get("query"),
	}

	if roleIDs := r.URL.Query()["role_ids"]; len(roleIDs) > 0 {
		filter.RoleIDs = roleIDs
	}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		filter.Active = &active
	}

	users, total, err := h.userService.List(r.Context(), filter)
	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", r.URL.Path).
			Str("method", r.Method).
			Msg("Failed to list users")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	response := ListUsersResponse{
		Users:  users,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Create a new user
// @Description Create a new user in the system
// @Tags users
// @Accept json
// @Produce json
// @Param user body user.CreateUserInput true "User creation request"
// @Success 200 {object} user.User
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Router /users [post]
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var input user.CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	newUser, err := h.userService.Create(r.Context(), input)
	if err != nil {
		switch err {
		case user.ErrInvalidInput:
			common.RespondError(w, http.StatusBadRequest, "Invalid input")
		case user.ErrAlreadyExists:
			common.RespondError(w, http.StatusConflict, "User already exists")
		default:
			log.Error().Err(err).Msg("Failed to create user")
			common.RespondError(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, newUser)
}

// @Summary Get a user by ID
// @Description Get detailed information about a specific user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} user.User
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /users/{id} [get]
func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	result, err := h.userService.Get(r.Context(), id)
	if err != nil {
		switch err {
		case user.ErrUserNotFound:
			http.NotFound(w, r)
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get user")
			common.RespondError(w, http.StatusInternalServerError, "Failed to get user")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Update a user
// @Description Update user information
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body user.UpdateUserInput true "User update request"
// @Success 200 {object} user.User
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /users/{id} [put]
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var input user.UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updatedUser, err := h.userService.Update(r.Context(), id, input)
	if err != nil {
		switch err {
		case user.ErrUserNotFound:
			http.NotFound(w, r)
		case user.ErrInvalidInput:
			common.RespondError(w, http.StatusBadRequest, "Invalid input")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update user")
			common.RespondError(w, http.StatusInternalServerError, "Failed to update user")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, updatedUser)
}

// @Summary Delete a user
// @Description Delete a user from the system
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /users/{id} [delete]
func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	err := h.userService.Delete(r.Context(), usr.ID, id)
	if err != nil {
		switch err {
		case user.ErrUserNotFound:
			http.NotFound(w, r)
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to delete user")
			common.RespondError(w, http.StatusInternalServerError, "Failed to delete user")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
