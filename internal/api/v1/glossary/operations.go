package glossary

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/tag"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type OwnerRequest struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
} // @name OwnerRequest

type CreateTermRequest struct {
	Name         string                 `json:"name" validate:"required"`
	Definition   string                 `json:"definition" validate:"required"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerRequest         `json:"owners,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
} // @name CreateTermRequest

type UpdateTermRequest struct {
	Name         *string                `json:"name,omitempty"`
	Definition   *string                `json:"definition,omitempty"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerRequest         `json:"owners,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
} // @name UpdateTermRequest

// CreateTerm creates a new glossary term
// @Summary Create glossary term
// @Description Create a new glossary term with name, definition, and optional metadata
// @Tags glossary
// @Accept json
// @Produce json
// @Param term body CreateTermRequest true "Glossary term to create"
// @Success 201 {object} glossary.GlossaryTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/ [post]
func (h *Handler) createTerm(w http.ResponseWriter, r *http.Request) {
	var req CreateTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	owners := make([]glossary.OwnerInput, len(req.Owners))
	for i, owner := range req.Owners {
		owners[i] = glossary.OwnerInput{
			ID:   owner.ID,
			Type: owner.Type,
		}
	}

	if len(owners) == 0 {
		owners = []glossary.OwnerInput{{ID: usr.ID, Type: "user"}}
	}

	input := glossary.CreateTermInput{
		Name:         req.Name,
		Definition:   req.Definition,
		Description:  req.Description,
		ParentTermID: req.ParentTermID,
		Owners:       owners,
		Metadata:     req.Metadata,
	}

	term, err := h.glossaryService.Create(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, glossary.ErrTermExists):
			log.Error().Err(err).Str("name", req.Name).Msg("Term already exists")
			common.RespondError(w, http.StatusConflict, "Term already exists")
		default:
			log.Error().Err(err).Interface("input", input).Msg("Failed to create glossary term")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, term)
}

// GetTerm retrieves a glossary term by ID
// @Summary Get glossary term
// @Description Retrieve a glossary term by its ID
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Success 200 {object} glossary.GlossaryTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/{id} [get]
func (h *Handler) getTerm(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/glossary/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	term, err := h.glossaryService.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get glossary term")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, term)
}

// UpdateTerm updates an existing glossary term
// @Summary Update glossary term
// @Description Update an existing glossary term by its ID
// @Tags glossary
// @Accept json
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param term body UpdateTermRequest true "Glossary term update data"
// @Success 200 {object} glossary.GlossaryTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/{id} [put]
func (h *Handler) updateTerm(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/glossary/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	var req UpdateTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var owners []glossary.OwnerInput
	if req.Owners != nil {
		owners = make([]glossary.OwnerInput, len(req.Owners))
		for i, owner := range req.Owners {
			owners[i] = glossary.OwnerInput{
				ID:   owner.ID,
				Type: owner.Type,
			}
		}
	}

	input := glossary.UpdateTermInput{
		Name:         req.Name,
		Definition:   req.Definition,
		Description:  req.Description,
		ParentTermID: req.ParentTermID,
		Owners:       owners,
		Metadata:     req.Metadata,
	}

	term, err := h.glossaryService.Update(r.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		case errors.Is(err, glossary.ErrCircularRef):
			log.Error().Err(err).Str("id", id).Msg("Circular reference detected")
			common.RespondError(w, http.StatusBadRequest, "Circular reference detected in term hierarchy")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update glossary term")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, term)
}

// DeleteTerm deletes a glossary term
// @Summary Delete glossary term
// @Description Delete a glossary term by its ID
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/{id} [delete]
func (h *Handler) deleteTerm(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/glossary/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	err := h.glossaryService.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to delete glossary term")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Term deleted successfully"})
}

// ListTerms lists all glossary terms
// @Summary List glossary terms
// @Description Retrieve a paginated list of all glossary terms
// @Tags glossary
// @Produce json
// @Param limit query int false "Maximum number of terms to return" default(20)
// @Param offset query int false "Number of terms to skip" default(0)
// @Success 200 {object} glossary.ListResult
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/list [get]
func (h *Handler) listTerms(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	result, err := h.glossaryService.List(r.Context(), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list glossary terms")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// SearchTerms searches glossary terms
// @Summary Search glossary terms
// @Description Search for glossary terms by query string and filters
// @Tags glossary
// @Produce json
// @Param q query string false "Search query"
// @Param parent_term_id query string false "Filter by parent term ID"
// @Param limit query int false "Maximum number of terms to return" default(20)
// @Param offset query int false "Number of terms to skip" default(0)
// @Success 200 {object} glossary.ListResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/search [get]
func (h *Handler) searchTerms(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	parentTermID := r.URL.Query().Get("parent_term_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := glossary.SearchFilter{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	}

	if parentTermID != "" {
		filter.ParentTermID = &parentTermID
	}

	result, err := h.glossaryService.Search(r.Context(), filter)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Interface("filter", filter).Msg("Failed to search glossary terms")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// GetChildren retrieves child terms of a glossary term
// @Summary Get child terms
// @Description Retrieve all child terms of a glossary term
// @Tags glossary
// @Produce json
// @Param id path string true "Parent Term ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/children/{id} [get]
func (h *Handler) getChildren(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/glossary/children/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	children, err := h.glossaryService.GetChildren(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get children")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"children": children,
		"total":    len(children),
	})
}

// GetAncestors retrieves ancestor terms of a glossary term
// @Summary Get ancestor terms
// @Description Retrieve all ancestor terms of a glossary term (parent chain)
// @Tags glossary
// @Produce json
// @Param id path string true "Term ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/ancestors/{id} [get]
func (h *Handler) getAncestors(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/glossary/ancestors/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	ancestors, err := h.glossaryService.GetAncestors(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get ancestors")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ancestors": ancestors,
		"total":     len(ancestors),
	})
}

type AddTermTagRequest struct {
	TagID string `json:"tag_id"`
} // @name AddGlossaryTermTagRequest

type ReplaceTermTagsRequest struct {
	TagIDs []string `json:"tag_ids"`
} // @name ReplaceGlossaryTermTagsRequest

type RemoveTermTagRequest struct {
	TagID string `json:"tag_id"`
} // @name RemoveGlossaryTermTagRequest

// @Summary Add a tag to a glossary term
// @Description Add a single tag association to a glossary term
// @Tags glossary
// @Accept json
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param body body AddTermTagRequest true "Tag ID to add"
// @Success 201 {array} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/tags/{id} [post]
func (h *Handler) addTermTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	var input AddTermTagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	if err := h.glossaryService.AddTermTag(r.Context(), id, input.TagID); err != nil {
		switch {
		case errors.Is(err, glossary.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term or tag not found")
		default:
			log.Error().Err(err).Str("id", id).Str("tag_id", input.TagID).Msg("Failed to add term tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	tags, err := h.glossaryService.ListGlossaryTermTags(r.Context(), id)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch updated tags")
		return
	}
	if tags == nil {
		tags = []tag.Tag{}
	}
	common.RespondJSON(w, http.StatusCreated, tags)
}

// ListTermTags retrieves all tags for a glossary term
// @Summary List glossary term tags
// @Description Get all tags associated with a glossary term
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Success 200 {array} tag.Tag
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/tags/{id} [get]
func (h *Handler) listTermTags(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	tags, err := h.glossaryService.ListGlossaryTermTags(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get term tags")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	if tags == nil {
		tags = []tag.Tag{}
	}

	common.RespondJSON(w, http.StatusOK, tags)
}

// ReplaceTermTags replaces all tags on a glossary term
// @Summary Replace glossary term tags
// @Description Atomically replace all tag associations for a glossary term
// @Tags glossary
// @Accept json
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param body body ReplaceTermTagsRequest true "Tag IDs to assign"
// @Success 200 {object} glossary.GlossaryTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/tags/{id} [put]
func (h *Handler) replaceTermTags(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	var input ReplaceTermTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.TagIDs == nil {
		input.TagIDs = []string{}
	}

	if err := h.glossaryService.SetTermTags(r.Context(), id, input.TagIDs); err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to replace term tags")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	updatedTerm, err := h.glossaryService.Get(r.Context(), id)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch updated term")
		return
	}
	common.RespondJSON(w, http.StatusOK, updatedTerm)
}

// RemoveTermTag removes a single tag from a glossary term
// @Summary Remove glossary term tag
// @Description Remove a single tag association from a glossary term
// @Tags glossary
// @Accept json
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param body body RemoveTermTagRequest true "Tag ID to remove"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /glossary/tags/{id} [delete]
func (h *Handler) removeTermTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID required")
		return
	}

	var input RemoveTermTagRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.TagID == "" {
		common.RespondError(w, http.StatusBadRequest, "tag_id is required")
		return
	}

	if err := h.glossaryService.RemoveTermTag(r.Context(), id, input.TagID); err != nil {
		switch {
		case errors.Is(err, glossary.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term or tag association not found")
		default:
			log.Error().Err(err).Str("id", id).Str("tag_id", input.TagID).Msg("Failed to remove term tag")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Tag removed"})
}
