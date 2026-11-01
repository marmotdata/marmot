package plugins

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/plugin/updates"
)

// WithVersions enables the plugin version routes. A nil manager leaves them answering that updates are unavailable.
func (h *Handler) WithVersions(m *updates.Manager) *Handler {
	h.versions = m
	return h
}

func (h *Handler) versionRoutes() []common.Route {
	authenticated := common.WithAuth(h.userService, h.authService, h.config)
	view := []func(http.HandlerFunc) http.HandlerFunc{authenticated, common.RequirePermission(h.userService, "ingestion", "view")}
	manage := []func(http.HandlerFunc) http.HandlerFunc{authenticated, common.RequirePermission(h.userService, "ingestion", "manage")}
	return []common.Route{
		{Path: "/api/v1/plugins/versions", Method: http.MethodGet, Middleware: view, Handler: h.getVersions},
		{Path: "/api/v1/plugins/versions/check", Method: http.MethodPost, Middleware: manage, Handler: h.checkVersions},
		{Path: "/api/v1/plugins/versions/{id}", Method: http.MethodPut, Middleware: manage, Handler: h.setVersion},
		{Path: "/api/v1/plugins/versions/{id}", Method: http.MethodDelete, Middleware: manage, Handler: h.resetVersion},
		{Path: "/api/v1/plugins/versions/{id}/pin", Method: http.MethodPut, Middleware: manage, Handler: h.pinVersion},
		{Path: "/api/v1/plugins/versions/{id}/rollback", Method: http.MethodPost, Middleware: manage, Handler: h.rollbackVersion},
	}
}

// SetPluginVersionRequest moves a plugin to a version from the plugin index.
type SetPluginVersionRequest struct {
	Version string `json:"version"`
} // @name SetPluginVersionRequest

// PinPluginVersionRequest pins or unpins a plugin at the version it runs.
type PinPluginVersionRequest struct {
	Pinned bool `json:"pinned"`
} // @name PinPluginVersionRequest

// CheckPluginVersionsResponse reports whether checking found a newer plugin index.
type CheckPluginVersionsResponse struct {
	Updated bool `json:"updated"`
} // @name CheckPluginVersionsResponse

// @Summary List core plugin versions, available updates and advisories
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} updates.Status
// @Failure 503 {object} common.ErrorResponse
// @ID getPluginVersions
// @Router /api/v1/plugins/versions [get]
func (h *Handler) getVersions(w http.ResponseWriter, r *http.Request) {
	if h.versions == nil {
		common.RespondError(w, http.StatusServiceUnavailable, "Plugin updates are unavailable on this server")
		return
	}
	status, err := h.versions.Status(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to read plugin versions")
		common.RespondError(w, http.StatusInternalServerError, "Failed to read plugin versions")
		return
	}
	common.RespondJSON(w, http.StatusOK, status)
}

// @Summary Check the plugin index for new versions and advisories now
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} CheckPluginVersionsResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 502 {object} common.ErrorResponse
// @ID checkPluginVersions
// @Router /api/v1/plugins/versions/check [post]
func (h *Handler) checkVersions(w http.ResponseWriter, r *http.Request) {
	if h.versions == nil {
		common.RespondError(w, http.StatusServiceUnavailable, "Plugin updates are unavailable on this server")
		return
	}
	updated, err := h.versions.Check(r.Context(), true)
	if errors.Is(err, updates.ErrDisabled) {
		common.RespondError(w, http.StatusConflict, "Plugin update checks are disabled in this server's config")
		return
	}
	if err != nil {
		common.RespondError(w, http.StatusBadGateway, "Checking the plugin index failed: "+err.Error())
		return
	}
	common.RespondJSON(w, http.StatusOK, CheckPluginVersionsResponse{Updated: updated})
}

// @Summary Move a core plugin to a version from the plugin index
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Plugin ID"
// @Param request body SetPluginVersionRequest true "Version"
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @ID setPluginVersion
// @Router /api/v1/plugins/versions/{id} [put]
func (h *Handler) setVersion(w http.ResponseWriter, r *http.Request) {
	var req SetPluginVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Version == "" {
		common.RespondError(w, http.StatusBadRequest, "A version is required")
		return
	}
	h.change(w, r, func(id, by string) error { return h.versions.SetVersion(r.Context(), id, req.Version, by) })
}

// @Summary Return a core plugin to the version this Marmot release pins
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @ID resetPluginVersion
// @Router /api/v1/plugins/versions/{id} [delete]
func (h *Handler) resetVersion(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, func(id, by string) error { return h.versions.Reset(r.Context(), id, by) })
}

// @Summary Pin or unpin a core plugin at the version it runs
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Accept json
// @Param id path string true "Plugin ID"
// @Param request body PinPluginVersionRequest true "Pinned"
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @ID pinPluginVersion
// @Router /api/v1/plugins/versions/{id}/pin [put]
func (h *Handler) pinVersion(w http.ResponseWriter, r *http.Request) {
	var req PinPluginVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	h.change(w, r, func(id, by string) error { return h.versions.SetPinned(r.Context(), id, req.Pinned, by) })
}

// @Summary Roll a core plugin back to the version it ran before its last change, and pin it
// @Tags plugins
// @Security ApiKeyAuth
// @Security BearerAuth
// @Param id path string true "Plugin ID"
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID rollbackPluginVersion
// @Router /api/v1/plugins/versions/{id}/rollback [post]
func (h *Handler) rollbackVersion(w http.ResponseWriter, r *http.Request) {
	h.change(w, r, func(id, by string) error { return h.versions.Rollback(r.Context(), id, by) })
}

// change runs a version change as the signed-in user and maps the manager's errors to statuses.
func (h *Handler) change(w http.ResponseWriter, r *http.Request, fn func(id, by string) error) {
	if h.versions == nil {
		common.RespondError(w, http.StatusServiceUnavailable, "Plugin updates are unavailable on this server")
		return
	}
	id := r.PathValue("id")
	by := "unknown"
	if u, ok := common.GetAuthenticatedUser(r.Context()); ok {
		by = u.Username
	}

	err := fn(id, by)
	switch {
	case err == nil:
		log.Info().Str("plugin", id).Str("by", by).Str("action", r.Method+" "+r.URL.Path).Msg("Plugin version changed")
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, updates.ErrUnknownPlugin):
		common.RespondError(w, http.StatusNotFound, "Not a core plugin")
	case errors.Is(err, updates.ErrUnknownVersion), errors.Is(err, updates.ErrYanked), errors.Is(err, updates.ErrIncompatible):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, updates.ErrNoIndex), errors.Is(err, updates.ErrNoPrevious), errors.Is(err, updates.ErrLocalOverride):
		common.RespondError(w, http.StatusConflict, err.Error())
	default:
		// The choice is stored even when this replica could not install it yet; it retries on its next reconcile and the error shows on the settings page.
		log.Error().Err(err).Str("plugin", id).Msg("Plugin version change failed")
		common.RespondError(w, http.StatusBadGateway, "Saved, but installing the plugin failed: "+err.Error())
	}
}
