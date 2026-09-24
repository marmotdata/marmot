package memory

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/memory"
	"github.com/rs/zerolog/log"
)

type RememberRequest struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id,omitempty"`
} // @name RememberRequest

type UpdateRequest struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id,omitempty"`
} // @name UpdateMemoryRequest

func respondServiceError(w http.ResponseWriter, err error, msg string) {
	switch {
	case errors.Is(err, memory.ErrNotFound):
		common.RespondError(w, http.StatusNotFound, "Memory not found")
	case errors.Is(err, memory.ErrInvalid):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	default:
		log.Error().Err(err).Msg(msg)
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
	}
}

// filterFrom reads the query parameters shared by list and search. The limit
// is 0 when unset, and the service applies its default.
func filterFrom(q url.Values) (memory.Filter, int) {
	return memory.Filter{
		SessionID: q.Get("session_id"),
	}, common.ParseLimit(q.Get("limit"), 0, memory.MaxLimit)
}

// @Summary List memory
// @Description List an entity's memory. By default the most recently changed comes first; sort=created lists the newest first.
// @Tags memory
// @Produce json
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param session_id query string false "Session ID"
// @Param sort query string false "Sort order" Enums(changed, created) default(changed)
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} memory.ListResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId} [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, limit := filterFrom(q)
	result, err := h.memoryService.List(r.Context(), e, memory.ListFilter{
		Filter: filter,
		Sort:   memory.Sort(q.Get("sort")),
		Limit:  limit,
		Offset: common.ParseOffset(q.Get("offset")),
	})
	if err != nil {
		respondServiceError(w, err, "Failed to list memory")
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Search memory
// @Description Search an entity's memory.
// @Tags memory
// @Produce json
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param q query string true "Query"
// @Param session_id query string false "Session ID"
// @Param limit query int false "Limit" default(20)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} memory.SearchResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId}/search [get]
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	filter, limit := filterFrom(q)
	result, err := h.memoryService.Search(r.Context(), e, memory.SearchQuery{
		Filter: filter,
		Query:  q.Get("q"),
		Limit:  limit,
	})
	if err != nil {
		respondServiceError(w, err, "Failed to search memory")
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Get a memory entry
// @Tags memory
// @Produce json
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param memoryId path string true "Memory ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} memory.Memory
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId}/{memoryId} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	m, err := h.memoryService.Get(r.Context(), e, r.PathValue("memoryId"))
	if err != nil {
		respondServiceError(w, err, "Failed to get memory")
		return
	}
	common.RespondJSON(w, http.StatusOK, m)
}

// @Summary Remember
// @Description Add a memory entry to an entity.
// @Tags memory
// @Accept json
// @Produce json
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param memory body RememberRequest true "Entry"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} memory.Memory
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId} [post]
func (h *Handler) remember(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	author, authed := AuthorFromContext(r.Context())
	if !authed {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req RememberRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	m, err := h.memoryService.Remember(r.Context(), e, memory.RememberInput{
		Content:   req.Content,
		SessionID: req.SessionID,
		Author:    author,
	})
	if err != nil {
		respondServiceError(w, err, "Failed to write memory")
		return
	}
	common.RespondJSON(w, http.StatusCreated, m)
}

// @Summary Update a memory entry
// @Description Replace an entry's content in place.
// @Tags memory
// @Accept json
// @Produce json
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param memoryId path string true "Memory ID"
// @Param memory body UpdateRequest true "Content"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} memory.Memory
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId}/{memoryId} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	author, authed := AuthorFromContext(r.Context())
	if !authed {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	m, err := h.memoryService.Update(r.Context(), e, r.PathValue("memoryId"), memory.UpdateInput{
		Content:   req.Content,
		SessionID: req.SessionID,
		Author:    author,
	})
	if err != nil {
		respondServiceError(w, err, "Failed to update memory")
		return
	}
	common.RespondJSON(w, http.StatusOK, m)
}

// @Summary Forget a memory entry
// @Tags memory
// @Param entityType path string true "Entity type" Enums(asset, data_product)
// @Param entityId path string true "Entity ID"
// @Param memoryId path string true "Memory ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /api/v1/memory/{entityType}/{entityId}/{memoryId} [delete]
func (h *Handler) forget(w http.ResponseWriter, r *http.Request) {
	e, ok := entityFrom(w, r)
	if !ok {
		return
	}
	if err := h.memoryService.Forget(r.Context(), e, r.PathValue("memoryId")); err != nil {
		respondServiceError(w, err, "Failed to delete memory")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
