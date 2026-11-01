package roles

import (
	"errors"
	"net/http"
	"slices"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/role"
	"github.com/rs/zerolog/log"
)

// @Summary List roles
// @Description List all active roles with user counts and permissions
// @Tags roles
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} role.Role
// @Failure 500 {object} common.ErrorResponse
// @ID getRoles
// @Router /api/v1/roles [get]
func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roleService.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list roles")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list roles")
		return
	}
	common.RespondDocument(w, r, http.StatusOK, roles)
}

// @Summary Get a role
// @Description Get a role by ID with its permissions
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} role.Role
// @Failure 404 {object} common.ErrorResponse
// @ID getRolesID
// @Router /api/v1/roles/{id} [get]
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

	common.RespondDocument(w, r, http.StatusOK, result)
}

type createRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	PermIDs     []string `json:"permission_ids,omitempty"`
	// PermissionKeys names permissions as resource_type:action keys, the form a hand-written role document uses.
	PermissionKeys []string `json:"permission_keys,omitempty"`
}

// @Summary Create a role
// @Description Create a new role with optional initial permissions, named by id or by resource_type:action key. The body may be JSON or YAML.
// @Tags roles
// @Accept json
// @Accept application/yaml
// @Produce json
// @Produce application/yaml
// @Param role body createRoleRequest true "Role creation request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID postRoles
// @Router /api/v1/roles [post]
func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := common.DecodeDocument(r, &req); err != nil {
		common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}

	result, err := h.roleService.Create(r.Context(), role.CreateInput{
		Name:           req.Name,
		Description:    req.Description,
		PermIDs:        req.PermIDs,
		PermissionKeys: req.PermissionKeys,
	})
	if err != nil {
		if errors.Is(err, role.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "Role already exists")
			return
		}
		if errors.Is(err, role.ErrUnknownPermission) {
			common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		log.Error().Err(err).Msg("Failed to create role")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create role")
		return
	}

	common.RespondDocument(w, r, http.StatusOK, result)
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
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @ID patchRolesID
// @Router /api/v1/roles/{id} [patch]
func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req updateRoleRequest
	if err := common.DecodeDocument(r, &req); err != nil {
		common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
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

	common.RespondDocument(w, r, http.StatusOK, result)
}

// @Summary Delete a role
// @Description Soft-delete a role. Fails if the role is a system role or has active user assignments.
// @Tags roles
// @Produce json
// @Param id path string true "Role ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @ID deleteRolesID
// @Router /api/v1/roles/{id} [delete]
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
	PermIDs []string `json:"permission_ids,omitempty"`
	// PermissionKeys names permissions as resource_type:action keys. They are added to PermIDs.
	PermissionKeys []string `json:"permission_keys,omitempty"`
}

// @Summary Replace role permissions
// @Description Atomically replace all permissions on a role, named by id or by resource_type:action key. System roles enforce a minimum permission floor.
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID"
// @Param body body replacePermissionsRequest true "Permission IDs to assign"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} role.Role
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 422 {object} common.ErrorResponse
// @ID postRolesIDPermissions
// @Router /api/v1/roles/{id}/permissions [post]
func (h *Handler) replacePermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var req replacePermissionsRequest
	if err := common.DecodeDocument(r, &req); err != nil {
		common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}
	permIDs := req.PermIDs
	if len(req.PermissionKeys) > 0 {
		ids, err := h.roleService.ResolvePermissionKeys(r.Context(), req.PermissionKeys)
		if err != nil {
			if errors.Is(err, role.ErrUnknownPermission) {
				common.RespondDocumentError(w, http.StatusBadRequest, err.Error(), nil)
				return
			}
			log.Error().Err(err).Str("id", id).Msg("Failed to resolve permission keys")
			common.RespondError(w, http.StatusInternalServerError, "Failed to replace permissions")
			return
		}
		for _, pid := range ids {
			if !slices.Contains(permIDs, pid) {
				permIDs = append(permIDs, pid)
			}
		}
	}

	if err := h.roleService.ReplacePermissions(r.Context(), id, permIDs); err != nil {
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

	common.RespondDocument(w, r, http.StatusOK, result)
}

// @Summary List all permissions
// @Description List all defined permissions grouped by resource type
// @Tags roles
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} role.Permission
// @Failure 500 {object} common.ErrorResponse
// @ID getPermissions
// @Router /api/v1/permissions [get]
func (h *Handler) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.roleService.ListPermissions(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list permissions")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list permissions")
		return
	}
	common.RespondJSON(w, http.StatusOK, perms)
}
