package tags

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/tag"
	"github.com/rs/zerolog/log"
)

// Request structs

type TagRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// ListTags returns all tags
// @Summary List all tags
// @Description Retrieve a list of all tags in the catalog
// @Tags tags
// @Produce json
// @Success 200 {array} tag.Tag
// @Failure 500 {object} common.ErrorResponse
// @Router /tags [get]
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.tagService.ListTags(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tags")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if tags == nil {
		tags = []tag.Tag{}
	}

	common.RespondJSON(w, http.StatusOK, tags)
}

// GetTag retrieves a single tag by ID
// @Summary Get a tag
// @Description Retrieve a single tag by its ID
// @Tags tags
// @Produce json
// @Param id path string true "Tag ID"
// @Success 200 {object} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /tags/{id} [get]
func (h *Handler) GetTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag ID required")
		return
	}

	t, err := h.tagService.GetTag(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, tag.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Tag not found")
		default:
			log.Error().Err(err).Str("tag_id", id).Msg("Failed to get tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, t)
}

// CreateTag creates a new tag
// @Summary Create a tag
// @Description Create a new tag in the catalog
// @Tags tags
// @Accept json
// @Produce json
// @Param body body TagRequest true "Tag to create"
// @Success 201 {object} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /tags [post]
func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req TagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag name required")
		return
	}

	t, err := h.tagService.CreateTag(r.Context(), tag.CreateTagInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, tag.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, tag.ErrConflict):
			common.RespondError(w, http.StatusConflict, "Tag with this name already exists")
		default:
			log.Error().Err(err).Msg("Failed to create tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, t)
}

// UpdateTag updates an existing tag
// @Summary Update a tag
// @Description Update an existing tag's name or description
// @Tags tags
// @Accept json
// @Produce json
// @Param id path string true "Tag ID"
// @Param body body TagRequest true "Updated tag fields"
// @Success 200 {object} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /tags/{id} [put]
func (h *Handler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag ID required")
		return
	}

	var req TagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// At least one field must be provided
	if req.Name == "" && req.Description == nil {
		common.RespondError(w, http.StatusBadRequest, "At least one field required")
		return
	}

	t, err := h.tagService.UpdateTag(r.Context(), id, tag.UpdateTagInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		switch {
		case errors.Is(err, tag.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Tag not found")
		case errors.Is(err, tag.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, tag.ErrConflict):
			common.RespondError(w, http.StatusConflict, "Tag with this name already exists")
		default:
			log.Error().Err(err).Str("tag_id", id).Msg("Failed to update tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, t)
}

// DeleteTag deletes a tag
// @Summary Delete a tag
// @Description Delete a tag from the catalog (removes all associations)
// @Tags tags
// @Param id path string true "Tag ID"
// @Success 204
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /tags/{id} [delete]
func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Tag ID required")
		return
	}

	if err := h.tagService.DeleteTag(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, tag.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Tag not found")
		default:
			log.Error().Err(err).Str("tag_id", id).Msg("Failed to delete tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
