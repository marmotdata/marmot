// Package recent serves the current person's recently viewed items.
//
// Views are recorded by the UI calling POST /users/me/recent when it opens an asset, glossary term, data product or saved query, rather than inside each resource's GET handler, so API, CLI and MCP reads (which are not someone browsing) never fill the list and the resource handlers stay unaware of it.
package recent

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/recent"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
	"github.com/marmotdata/plugin-sdk/mrn"
)

type Handler struct {
	store       *recent.Store
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(store *recent.Store, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{store: store, userService: userService, authService: authService, config: cfg}
}

func (h *Handler) Routes() []common.Route {
	withAuth := common.WithAuth(h.userService, h.authService, h.config)
	return []common.Route{
		{
			Path:       "/api/v1/users/me/recent",
			Method:     http.MethodGet,
			Handler:    h.list,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth},
		},
		{
			Path:       "/api/v1/users/me/recent",
			Method:     http.MethodPost,
			Handler:    h.record,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, common.WithRateLimit(h.config, 120, 60)},
		},
	}
}

// RecordViewRequest names the item a person opened. An asset can be named by its id or by the type, service and name in its page address.
type RecordViewRequest struct {
	Kind    string `json:"kind" example:"asset"`
	ID      string `json:"id,omitempty"`
	Type    string `json:"type,omitempty"`
	Service string `json:"service,omitempty"`
	Name    string `json:"name,omitempty"`
} // @name RecordRecentViewRequest

type ListRecentResponse struct {
	Items []recent.Item `json:"items"`
} // @name ListRecentResponse

// viewer returns the id of the person behind the request. Anonymous, operator and service account callers have no users row, so they have no recent items.
func viewer(r *http.Request) (string, bool) {
	id := common.CreatedBy(r.Context())
	if id == nil {
		return "", false
	}
	return *id, true
}

// visibleKinds leaves out kinds the caller can no longer read, so a revoked permission also hides what they opened under it. Saved queries are the caller's own.
func visibleKinds(r *http.Request) []string {
	kinds := []string{recent.KindSavedQuery}
	if common.HasPermission(r.Context(), "assets", "view") {
		// Data products are read under assets:view, the same gate as their own routes.
		kinds = append(kinds, recent.KindAsset, recent.KindDataProduct)
	}
	if common.HasPermission(r.Context(), "glossary", "view") {
		kinds = append(kinds, recent.KindGlossaryTerm)
	}
	return kinds
}

// @Summary List recently viewed items
// @Description Returns what the current person opened most recently in the UI: assets, glossary terms, data products and their saved queries, newest first, described as they are now. Items since deleted, and kinds the caller can no longer view, are left out. Callers without a user account get an empty list.
// @Tags users
// @Produce json
// @Param limit query int false "Number of items, at most 50" default(10)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ListRecentResponse
// @Failure 401 {object} common.ErrorResponse
// @ID listRecentItems
// @Router /api/v1/users/me/recent [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := viewer(r)
	if !ok {
		common.RespondJSON(w, http.StatusOK, ListRecentResponse{Items: []recent.Item{}})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.store.List(r.Context(), userID, visibleKinds(r), limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list recent items")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list recent items")
		return
	}
	common.RespondJSON(w, http.StatusOK, ListRecentResponse{Items: items})
}

// @Summary Record a view
// @Description Notes that the current person opened an item, for their recently viewed list. Repeat views within a minute are ignored and only the latest 50 items are kept. Callers without a user account are accepted and nothing is stored.
// @Tags users
// @Accept json
// @Param request body RecordViewRequest true "The item viewed"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @ID recordRecentView
// @Router /api/v1/users/me/recent [post]
func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	var req RecordViewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !recent.ValidKind(req.Kind) {
		common.RespondError(w, http.StatusBadRequest, "kind must be asset, glossary_term, data_product or saved_query")
		return
	}
	userID, ok := viewer(r)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !slices.Contains(visibleKinds(r), req.Kind) {
		common.RespondError(w, http.StatusForbidden, "Permission denied")
		return
	}

	id := strings.TrimSpace(req.ID)
	if req.Kind == recent.KindAsset && id == "" {
		if req.Type == "" || req.Service == "" || req.Name == "" {
			common.RespondError(w, http.StatusBadRequest, "an asset needs an id, or a type, service and name")
			return
		}
		resolved, err := h.store.ResolveAsset(r.Context(), mrn.New(req.Type, req.Service, req.Name))
		if err != nil {
			respondError(w, err)
			return
		}
		id = resolved
	}
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.store.Record(r.Context(), userID, req.Kind, id); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, recent.ErrItemNotFound):
		common.RespondError(w, http.StatusNotFound, "Item not found")
	case errors.Is(err, recent.ErrInvalidKind):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	default:
		log.Error().Err(err).Msg("Failed to record view")
		common.RespondError(w, http.StatusInternalServerError, "Failed to record view")
	}
}
