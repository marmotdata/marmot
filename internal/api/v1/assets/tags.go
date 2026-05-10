package assets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/tag"
	"github.com/rs/zerolog/log"
)

type ReplaceTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
}

type AddTagRequest struct {
	TagID string `json:"tag_id"`
}

type RemoveTagRequest struct {
	TagID string `json:"tag_id"`
}

// @Summary Add a tag to an asset
// @Description Add a single tag association to an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param body body AddTagRequest true "Tag ID to add"
// @Success 201 {array} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/tags/{id} [post]
func (h *Handler) addAssetTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var input AddTagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	if err := h.assetService.AddTag(r.Context(), id, input.TagID); err != nil {
		switch {
		case errors.Is(err, asset.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset or tag not found")
		default:
			log.Error().Err(err).Str("id", id).Str("tag_id", input.TagID).Msg("Failed to add asset tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	tags, err := h.assetService.ListAssetTags(r.Context(), id)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch updated tags")
		return
	}
	if tags == nil {
		tags = []tag.Tag{}
	}
	common.RespondJSON(w, http.StatusCreated, tags)
}

// @Summary Replace all tags on an asset
// @Description Atomically replace all tag associations for an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param body body ReplaceTagsRequest true "Tag IDs to assign"
// @Success 200 {array} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/tags/{id} [put]
func (h *Handler) replaceAssetTags(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var input ReplaceTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.TagIDs == nil {
		input.TagIDs = []string{}
	}

	if err := h.assetService.SetTags(r.Context(), id, input.TagIDs); err != nil {
		switch {
		case errors.Is(err, asset.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to replace asset tags")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	tags, err := h.assetService.ListAssetTags(r.Context(), id)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch updated tags")
		return
	}
	if tags == nil {
		tags = []tag.Tag{}
	}
	common.RespondJSON(w, http.StatusOK, tags)
}

// @Summary List asset tags
// @Description Get all tags associated with an asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {array} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/tags/{id} [get]
func (h *Handler) listAssetTags(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	tags, err := h.assetService.ListAssetTags(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get asset tags")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if tags == nil {
		tags = []tag.Tag{}
	}

	common.RespondJSON(w, http.StatusOK, tags)
}

// @Summary Remove a tag from an asset
// @Description Remove a single tag association from an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param body body RemoveTagRequest true "Tag ID to remove"
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/tags/{id} [delete]
func (h *Handler) removeAssetTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var input RemoveTagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	if err := h.assetService.RemoveTag(r.Context(), id, input.TagID); err != nil {
		switch {
		case errors.Is(err, asset.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset or tag association not found")
		default:
			log.Error().Err(err).Str("id", id).Str("tag_id", input.TagID).Msg("Failed to remove asset tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
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