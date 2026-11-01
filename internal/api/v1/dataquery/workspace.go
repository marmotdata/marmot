package dataquery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

// WorkspaceHandler serves what sits around running queries: each person's saved queries and the per-agent summaries Home lists. It is apart from Handler so the query service interface does not grow for features that only read and write their own tables.
type WorkspaceHandler struct {
	*Handler
	saved *dataquery.SavedQueryStore
	repo  *dataquery.PostgresRepository
}

func NewWorkspaceHandler(queryService dataquery.Service, saved *dataquery.SavedQueryStore, repo *dataquery.PostgresRepository, userService user.Service, authService auth.Service, cfg *config.Config) *WorkspaceHandler {
	return &WorkspaceHandler{
		Handler: NewHandler(queryService, userService, authService, cfg),
		saved:   saved,
		repo:    repo,
	}
}

func (h *WorkspaceHandler) Routes() []common.Route {
	withAuth := common.WithAuth(h.userService, h.authService, h.config)
	query := h.requireQuery
	queryOrLog := h.requireQueryOrLog
	mw := func(m ...func(http.HandlerFunc) http.HandlerFunc) []func(http.HandlerFunc) http.HandlerFunc { return m }

	return []common.Route{
		{Path: "/api/v1/query/saved", Method: http.MethodGet, Handler: h.listSaved, Middleware: mw(withAuth, query)},
		{Path: "/api/v1/query/saved", Method: http.MethodPost, Handler: h.createSaved, Middleware: mw(withAuth, query)},
		{Path: "/api/v1/query/saved/{id}", Method: http.MethodGet, Handler: h.getSaved, Middleware: mw(withAuth, query)},
		{Path: "/api/v1/query/saved/{id}", Method: http.MethodPut, Handler: h.updateSaved, Middleware: mw(withAuth, query)},
		{Path: "/api/v1/query/saved/{id}", Method: http.MethodDelete, Handler: h.deleteSaved, Middleware: mw(withAuth, query)},
		{Path: "/api/v1/query/saved/{id}/run", Method: http.MethodPost, Handler: h.markSavedRun, Middleware: mw(withAuth, query)},

		{Path: "/api/v1/query/agents", Method: http.MethodGet, Handler: h.agentSummaries, Middleware: mw(withAuth, queryOrLog)},
	}
}

type ListSavedQueriesResponse struct {
	Queries []*dataquery.SavedQuery `json:"queries"`
	Total   int                     `json:"total"`
} // @name ListSavedQueriesResponse

type AgentSummariesResponse struct {
	Agents []dataquery.AgentSummary `json:"agents"`
} // @name QueryAgentSummariesResponse

// owner returns the user a saved query belongs to. Saved queries are personal, so callers without a users row (service accounts, anonymous and operator access) cannot keep them.
func owner(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := common.CreatedBy(r.Context())
	if id == nil {
		common.RespondError(w, http.StatusForbidden, "Saved queries need a user account")
		return "", false
	}
	return *id, true
}

func respondSavedError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dataquery.ErrInvalidInput):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, dataquery.ErrSavedQueryNotFound):
		common.RespondError(w, http.StatusNotFound, err.Error())
	default:
		log.Error().Err(err).Msg("Saved query request failed")
		common.RespondError(w, http.StatusInternalServerError, "Saved query request failed")
	}
}

// savedID reads the path id, treating anything that is not a UUID as missing so it never reaches the uuid column.
func savedID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil {
		respondSavedError(w, dataquery.ErrSavedQueryNotFound)
		return "", false
	}
	return id, true
}

func decodeSaved(w http.ResponseWriter, r *http.Request) (dataquery.SavedQueryInput, bool) {
	var in dataquery.SavedQueryInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return in, false
	}
	return in, true
}

// @Summary  List saved queries
// @Description Lists the current person's saved queries, the most recently run or edited first.
// @Tags     query
// @Produce  json
// @Param    limit query int false "Page size, at most 500" default(50)
// @Param    offset query int false "Page offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} ListSavedQueriesResponse
// @Failure  403 {object} common.ErrorResponse
// @Router   /api/v1/query/saved [get]
func (h *WorkspaceHandler) listSaved(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	queries, total, err := h.saved.List(r.Context(), userID, limit, offset)
	if err != nil {
		respondSavedError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, ListSavedQueriesResponse{Queries: queries, Total: total})
}

// @Summary  Save a query
// @Description Saves a statement and the source it runs against for the current person to run again.
// @Tags     query
// @Accept   json
// @Produce  json
// @Param    request body dataquery.SavedQueryInput true "Saved query"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  201 {object} dataquery.SavedQuery
// @Failure  400 {object} common.ErrorResponse
// @Failure  403 {object} common.ErrorResponse
// @Router   /api/v1/query/saved [post]
func (h *WorkspaceHandler) createSaved(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	in, ok := decodeSaved(w, r)
	if !ok {
		return
	}
	q, err := h.saved.Create(r.Context(), userID, in)
	if err != nil {
		respondSavedError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusCreated, q)
}

// @Summary  Get a saved query
// @Description Returns one of the current person's saved queries.
// @Tags     query
// @Produce  json
// @Param    id path string true "Saved query ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.SavedQuery
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/saved/{id} [get]
func (h *WorkspaceHandler) getSaved(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	id, ok := savedID(w, r)
	if !ok {
		return
	}
	q, err := h.saved.Get(r.Context(), userID, id)
	if err != nil {
		respondSavedError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, q)
}

// @Summary  Update a saved query
// @Description Replaces the name, statement and source of one of the current person's saved queries.
// @Tags     query
// @Accept   json
// @Produce  json
// @Param    id path string true "Saved query ID"
// @Param    request body dataquery.SavedQueryInput true "Saved query"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.SavedQuery
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/saved/{id} [put]
func (h *WorkspaceHandler) updateSaved(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	id, ok := savedID(w, r)
	if !ok {
		return
	}
	in, ok := decodeSaved(w, r)
	if !ok {
		return
	}
	q, err := h.saved.Update(r.Context(), userID, id, in)
	if err != nil {
		respondSavedError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, q)
}

// @Summary  Delete a saved query
// @Description Deletes one of the current person's saved queries. The query log keeps every run.
// @Tags     query
// @Param    id path string true "Saved query ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  204
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/saved/{id} [delete]
func (h *WorkspaceHandler) deleteSaved(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	id, ok := savedID(w, r)
	if !ok {
		return
	}
	if err := h.saved.Delete(r.Context(), userID, id); err != nil {
		respondSavedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Mark a saved query as run
// @Description Records that the current person ran a saved query, setting its last_run_at. The statement itself runs through POST /query/run, which checks access and logs it like any other.
// @Tags     query
// @Produce  json
// @Param    id path string true "Saved query ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.SavedQuery
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/saved/{id}/run [post]
func (h *WorkspaceHandler) markSavedRun(w http.ResponseWriter, r *http.Request) {
	userID, ok := owner(w, r)
	if !ok {
		return
	}
	id, ok := savedID(w, r)
	if !ok {
		return
	}
	q, err := h.saved.MarkRun(r.Context(), userID, id)
	if err != nil {
		respondSavedError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, q)
}

// @Summary  Summarise agents' queries
// @Description Lists the service accounts agents query as, busiest first over the last 7 days, with queries and refusals in the last 24 hours and 7 days, the tables read and when each last queried. Pass principal_id for one account. Callers without access:view only see their own activity. Each row's principal_type and principal_id filter GET /query/history to that agent's queries.
// @Tags     query
// @Produce  json
// @Param    principal_id query string false "One service account"
// @Param    limit query int false "At most 100" default(10)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} AgentSummariesResponse
// @Router   /api/v1/query/agents [get]
func (h *WorkspaceHandler) agentSummaries(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := dataquery.AgentSummaryFilter{
		PrincipalType: dataquery.PrincipalTypeServiceAccount,
		PrincipalID:   r.URL.Query().Get("principal_id"),
		Limit:         limit,
	}
	if !common.HasPermission(r.Context(), "access", "view") {
		// Only a service account has an agent summary of its own; anyone else without access:view sees none.
		if string(principal.Type()) != dataquery.PrincipalTypeServiceAccount {
			common.RespondJSON(w, http.StatusOK, AgentSummariesResponse{Agents: []dataquery.AgentSummary{}})
			return
		}
		filter.PrincipalID = principal.ID()
	}
	agents, err := h.repo.AgentSummaries(r.Context(), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, AgentSummariesResponse{Agents: agents})
}
