package roles

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/rs/zerolog/log"
)

// @Summary List roles
// @Description List all active roles with user counts and permissions
// @Tags roles
// @Produce json
// @Success 200 {array} role.Role
// @Failure 500 {object} common.ErrorResponse
// @Router /roles [get]
func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roleService.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list roles")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list roles")
		return
	}
	common.RespondJSON(w, http.StatusOK, roles)
}

// @Summary Get a role
// @Description Get a role by ID with its permissions
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Success 200 {object} role.Role
// @Failure 404 {object} common.ErrorResponse
// @Router /roles/{id} [get]
func (h *Handler) getRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	result, err := h.roleService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, role.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		log.Error().Err(err).Str("id", id).Msg("Failed to get role")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get role")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

type createRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PermIDs     []string `json:"permission_ids,omitempty"`
}

// @Summary Create a role
// @Description Create a new role with optional initial permissions
// @Tags roles
// @Accept json
// @Produce json
// @Param role body createRoleRequest true "Role creation request"
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Router /roles [post]
func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}

	result, err := h.roleService.Create(r.Context(), role.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		PermIDs:     req.PermIDs,
	})
	if err != nil {
		if errors.Is(err, role.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "Role already exists")
			return
		}
		log.Error().Err(err).Msg("Failed to create role")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create role")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

type updateRoleRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// @Summary Update a role
// @Description Update a role's name or description. System roles cannot be renamed.
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param role body updateRoleRequest true "Role update request"
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @Router /roles/{id} [patch]
func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.roleService.Update(r.Context(), id, role.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, role.ErrNotFound):
			http.NotFound(w, r)
		case errors.Is(err, role.ErrSystemRoleProtected):
			common.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update role")
			common.RespondError(w, http.StatusInternalServerError, "Failed to update role")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Delete a role
// @Description Soft-delete a role. Fails if the role is a system role or has active user assignments.
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @Router /roles/{id} [delete]
func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	err := h.roleService.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, role.ErrNotFound):
			http.NotFound(w, r)
		case errors.Is(err, role.ErrSystemRoleProtected):
			common.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, role.ErrRoleInUse):
			common.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to delete role")
			common.RespondError(w, http.StatusInternalServerError, "Failed to delete role")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type replacePermissionsRequest struct {
	PermIDs []string `json:"permission_ids"`
}

// @Summary Replace role permissions
// @Description Atomically replace all permissions on a role. System roles enforce a minimum permission floor.
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param body body replacePermissionsRequest true "Permission IDs to assign"
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @Router /roles/{id}/permissions [post]
func (h *Handler) replacePermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req replacePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.roleService.ReplacePermissions(r.Context(), id, req.PermIDs); err != nil {
		switch {
		case errors.Is(err, role.ErrNotFound):
			http.NotFound(w, r)
		case errors.Is(err, role.ErrSystemRoleProtected):
			common.RespondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to replace permissions")
			common.RespondError(w, http.StatusInternalServerError, "Failed to replace permissions")
		}
		return
	}

	result, err := h.roleService.Get(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get role after permission update")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get updated role")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary List all permissions
// @Description List all defined permissions grouped by resource type
// @Tags roles
// @Produce json
// @Success 200 {array} role.Permission
// @Failure 500 {object} common.ErrorResponse
// @Router /permissions [get]
func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.roleService.ListPermissions(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list permissions")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list permissions")
		return
	}
	common.RespondJSON(w, http.StatusOK, perms)
}
