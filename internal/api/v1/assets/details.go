package assets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdetail"
	"github.com/rs/zerolog/log"
)

// FieldAnnotationsResponse lists the annotated fields of an asset.
type FieldAnnotationsResponse struct {
	Annotations []assetdetail.FieldAnnotation `json:"annotations"`
} // @name FieldAnnotationsResponse

func (h *Handler) detailRoutes() []common.Route {
	withAuth := common.WithAuth(h.userService, h.authService, h.config)
	view := common.RequirePermission(h.userService, "assets", "view")
	manage := common.RequirePermission(h.userService, "assets", "manage")
	return []common.Route{
		{Path: "/api/v1/assets/counts/{id}", Method: http.MethodGet, Handler: h.getTabCounts, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, view}},
		{Path: "/api/v1/assets/field-annotations/{id}", Method: http.MethodGet, Handler: h.listFieldAnnotations, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, view}},
		{Path: "/api/v1/assets/field-annotations/{id}", Method: http.MethodPut, Handler: h.setFieldAnnotation, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, manage}},
		{Path: "/api/v1/assets/field-annotations/{id}", Method: http.MethodDelete, Handler: h.deleteFieldAnnotation, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, manage}},
	}
}

// assetOr404 loads the asset named in the path, answering 404 or 500 itself when it cannot.
func (h *Handler) assetOr404(w http.ResponseWriter, r *http.Request) (*asset.Asset, bool) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return nil, false
	}
	a, err := h.assetService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, asset.ErrAssetNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		} else {
			log.Error().Err(err).Str("id", id).Msg("Failed to get asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return nil, false
	}
	return a, true
}

// @Summary Get asset tab counts
// @Description Counts shown beside the asset page's tabs: lineage neighbours one edge away, pipeline runs, agent runs and queries through Marmot in the last 14 days. queries is left out for callers who may not read the query log; callers who may query but not read the whole log only count their own queries.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetdetail.TabCounts
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetsCountsID
// @Router /api/v1/assets/counts/{id} [get]
func (h *Handler) getTabCounts(w http.ResponseWriter, r *http.Request) {
	a, ok := h.assetOr404(w, r)
	if !ok {
		return
	}
	mrn := ""
	if a.MRN != nil {
		mrn = *a.MRN
	}

	countQueries := false
	var scope assetdetail.QueryScope
	switch {
	case common.HasPermission(r.Context(), "access", "view"):
		countQueries = true
	case common.HasPermission(r.Context(), "assets", "query"):
		if principal, ok := common.PrincipalFromContext(r.Context()); ok {
			countQueries = true
			scope = assetdetail.QueryScope{PrincipalType: string(principal.Type()), PrincipalID: principal.ID()}
		}
	}

	counts, err := h.details.TabCounts(r.Context(), a.ID, mrn, countQueries, scope)
	if err != nil {
		log.Error().Err(err).Str("asset_id", a.ID).Msg("Failed to count asset tabs")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	common.RespondJSON(w, http.StatusOK, counts)
}

// @Summary List field annotations
// @Description Lists the markers people set on an asset's schema fields: personal data, certified, an edited description and glossary terms. The asset response carries the same list as field_annotations.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} FieldAnnotationsResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetsFieldAnnotationsID
// @Router /api/v1/assets/field-annotations/{id} [get]
func (h *Handler) listFieldAnnotations(w http.ResponseWriter, r *http.Request) {
	a, ok := h.assetOr404(w, r)
	if !ok {
		return
	}
	annotations, err := h.details.ListFieldAnnotations(r.Context(), a.ID)
	if err != nil {
		log.Error().Err(err).Str("asset_id", a.ID).Msg("Failed to list field annotations")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	common.RespondJSON(w, http.StatusOK, FieldAnnotationsResponse{Annotations: annotations})
}

// @Summary Set a field annotation
// @Description Sets the markers on one schema field, merging into what is there: omitted markers keep their value, an empty description clears an edited one and a field left with no markers is removed. Returns every annotated field of the asset.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param annotation body assetdetail.FieldAnnotationInput true "Field markers"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} FieldAnnotationsResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putAssetsFieldAnnotationsID
// @Router /api/v1/assets/field-annotations/{id} [put]
func (h *Handler) setFieldAnnotation(w http.ResponseWriter, r *http.Request) {
	a, ok := h.assetOr404(w, r)
	if !ok {
		return
	}
	var in assetdetail.FieldAnnotationInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	updatedBy := ""
	if principal, ok := common.PrincipalFromContext(r.Context()); ok {
		updatedBy = principal.DisplayName()
	}
	if err := h.details.SetFieldAnnotation(r.Context(), a.ID, in, updatedBy); err != nil {
		if errors.Is(err, assetdetail.ErrInvalidInput) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Str("asset_id", a.ID).Msg("Failed to set field annotation")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.listFieldAnnotations(w, r)
}

// @Summary Delete a field annotation
// @Description Removes every marker from one schema field.
// @Tags assets
// @Param id path string true "Asset ID"
// @Param schema query string false "Schema key the field belongs to"
// @Param path query string true "Dotted path of the field"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteAssetsFieldAnnotationsID
// @Router /api/v1/assets/field-annotations/{id} [delete]
func (h *Handler) deleteFieldAnnotation(w http.ResponseWriter, r *http.Request) {
	a, ok := h.assetOr404(w, r)
	if !ok {
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		common.RespondError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := h.details.DeleteFieldAnnotation(r.Context(), a.ID, r.URL.Query().Get("schema"), path); err != nil {
		log.Error().Err(err).Str("asset_id", a.ID).Msg("Failed to delete field annotation")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
