package assets

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type TagRequest struct {
	Tag string `json:"tag" validate:"required"`
}

// @Summary Add tag to asset
// @Description Add a new tag to an existing asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param tag body TagRequest true "Tag to add"
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /assets/{id}/tags [post]
func (h *Handler) addTag(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/tags/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := parts[0]

	var input TagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Tag == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag is required")
		return
	}

	updated, err := h.assetService.AddTag(r.Context(), id, input.Tag)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		case errors.Is(err, asset.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Str("tag", input.Tag).Msg("Failed to add tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, updated)
}

// @Summary Remove tag from asset
// @Description Remove a tag from an existing asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param tag body TagRequest true "Tag to remove"
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /assets/{id}/tags [delete]
func (h *Handler) removeTag(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/tags/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := parts[0]

	var input TagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.Tag == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag is required")
		return
	}

	updated, err := h.assetService.RemoveTag(r.Context(), id, input.Tag)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		default:
			log.Error().Err(err).Str("id", id).Str("tag", input.Tag).Msg("Failed to remove tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, updated)
}

// @Summary Get tag suggestions
// @Description Get suggestions for asset tags
// @Tags assets
// @Produce json
// @Param prefix query string false "Tag prefix to filter by"
// @Param limit query int false "Maximum number of suggestions" default(10)
// @Success 200 {array} string
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/suggestions/tags [get]
func (h *Handler) getTagSuggestions(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	limit := common.ParseLimit(r.URL.Query().Get("limit"), 10, 100)

	suggestions, err := h.assetService.GetTagSuggestions(r.Context(), prefix, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get tag suggestions")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get suggestions")
		return
	}

	common.RespondJSON(w, http.StatusOK, suggestions)
}
