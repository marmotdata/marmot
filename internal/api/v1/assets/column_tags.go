package assets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type AddColumnTagRequest struct {
	ColumnPath string `json:"column_path"`
	TagID      string `json:"tag_id"`
}

type ReplaceColumnTagsRequest struct {
	ColumnPath string   `json:"column_path"`
	TagIDs     []string `json:"tag_ids"`
}

type RemoveColumnTagRequest struct {
	ColumnPath string `json:"column_path"`
	TagID      string `json:"tag_id"`
}

func (h *Handler) getColumnTags(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	tags, err := h.assetService.GetColumnTags(r.Context(), assetID)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to get column tags")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if tags == nil {
		tags = make(asset.ColumnTagMap)
	}

	common.RespondJSON(w, http.StatusOK, tags)
}

func (h *Handler) addColumnTag(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var req AddColumnTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ColumnPath == "" {
		common.RespondError(w, http.StatusBadRequest, "column_path is required")
		return
	}
	if req.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	ct, err := h.assetService.AddColumnTag(r.Context(), assetID, req.ColumnPath, req.TagID)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrColumnTagInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, asset.ErrColumnTagNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset or tag not found")
		default:
			log.Error().Err(err).Str("asset_id", assetID).Str("column_path", req.ColumnPath).Msg("Failed to add column tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, ct)
}

// @Summary Replace tags for a single column on an asset
// @Description Atomically replace the tag set assigned to one column. Tags already attached are preserved.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param body body ReplaceColumnTagsRequest true "Column path and tag IDs to assign"
// @Success 204 "No Content"
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/column-tags/{id} [put]
func (h *Handler) replaceColumnTags(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var req ReplaceColumnTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ColumnPath == "" {
		common.RespondError(w, http.StatusBadRequest, "column_path is required")
		return
	}
	if req.TagIDs == nil {
		req.TagIDs = []string{}
	}
	if err := h.assetService.SetColumnTags(r.Context(), assetID, req.ColumnPath, req.TagIDs); err != nil {
		switch {
		case errors.Is(err, asset.ErrColumnTagInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, asset.ErrColumnTagNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		default:
			log.Error().Err(err).Str("asset_id", assetID).Str("column_path", req.ColumnPath).Msg("Failed to set column tags")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Remove a single tag from a column
// @Description Delete one (column_path, tag_id) assignment for an asset.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param body body RemoveColumnTagRequest true "Column path and tag ID to remove"
// @Success 204 "No Content"
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/column-tags/{id} [delete]
func (h *Handler) deleteColumnTag(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset id is required")
		return
	}

	var input RemoveColumnTagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.ColumnPath == "" {
		common.RespondError(w, http.StatusBadRequest, "column_path is required")
		return
	}
	if input.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	if err := h.assetService.RemoveColumnTag(r.Context(), assetID, input.ColumnPath, input.TagID); err != nil {
		switch {
		case errors.Is(err, asset.ErrColumnTagInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, asset.ErrColumnTagNotFound):
			common.RespondError(w, http.StatusNotFound, "Column tag assignment not found")
		default:
			log.Error().Err(err).Str("asset_id", assetID).Str("column_path", input.ColumnPath).Str("tag_id", input.TagID).Msg("Failed to delete column tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
