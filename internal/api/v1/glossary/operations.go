package glossary

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type OwnerRequest struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
}

type CreateTermRequest struct {
	Name         string                 `json:"name" validate:"required"`
	Definition   string                 `json:"definition" validate:"required"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerRequest         `json:"owners" validate:"required,min=1"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateTermRequest struct {
	Name         *string                `json:"name,omitempty"`
	Definition   *string                `json:"definition,omitempty"`
	Description  *string                `json:"description,omitempty"`
	ParentTermID *string                `json:"parent_term_id,omitempty"`
	Owners       []OwnerRequest         `json:"owners,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

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
