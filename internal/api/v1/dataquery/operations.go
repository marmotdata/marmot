package dataquery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/plugin"
)

type StatusResponse struct {
	Instances []plugin.InstanceStatus `json:"instances"`
} // @name QueryStatusResponse

type ListQueriesResponse struct {
	Queries []*dataquery.Query `json:"queries"`
	Total   int                `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
} // @name QueryHistoryResponse

type ListSourcesResponse struct {
	Sources []*dataquery.Source `json:"sources"`
	Total   int                 `json:"total"`
} // @name QueryListSourcesResponse

// respondServiceError maps data query service errors onto HTTP statuses.
func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dataquery.ErrInvalidInput), errors.Is(err, dataquery.ErrQueryFailed):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, dataquery.ErrSourceNotFound), errors.Is(err, dataquery.ErrQueryNotFound):
		common.RespondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, dataquery.ErrDenied):
		common.RespondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, dataquery.ErrQueryUnsupported):
		common.RespondError(w, http.StatusConflict, err.Error())
	default:
		log.Error().Err(err).Msg("Query request failed")
		common.RespondError(w, http.StatusInternalServerError, "Query request failed")
	}
}

// @Summary  List queryable sources
// @Description Pages through the sources that answer data queries, in name order. Use a source's name as the "source" of a query.
// @Tags     query
// @Produce  json
// @Param    q query string false "Search by source name or plugin"
// @Param    limit query int false "Page size, at most 200" default(50)
// @Param    offset query int false "Page offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} ListSourcesResponse
// @Router   /api/v1/query/sources [get]
func (h *Handler) listSources(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	sources, total, err := h.queryService.ListSources(r.Context(), dataquery.SourceFilter{Search: q.Get("q"), Limit: limit, Offset: offset})
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, ListSourcesResponse{Sources: sources, Total: total})
}

// @Summary  List a source's tables
// @Description Lists the catalogued tables a queryable source can reach, busiest first, each with a statement that previews it in the source's own dialect.
// @Tags     query
// @Produce  json
// @Param    name path string true "Source name"
// @Param    q query string false "Search by table name"
// @Param    limit query int false "At most 100" default(25)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {array} dataquery.SourceTable
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/sources/{name}/tables [get]
func (h *Handler) listSourceTables(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tables, err := h.queryService.ListSourceTables(r.Context(), r.PathValue("name"), r.URL.Query().Get("q"), limit)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, tables)
}

// @Summary  Query plugin instance status
// @Description Reports the long-running plugin processes that serve data queries.
// @Tags     query
// @Produce  json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} StatusResponse
// @Router   /api/v1/query/status [get]
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, StatusResponse{Instances: h.queryService.InstanceStatus()})
}

// @Summary  Query statistics
// @Description Summarises data query activity over a window from hourly rollups: totals, a series and the busiest callers, sources and tables. Callers without access:view only see their own activity. The window defaults to the last 24 hours.
// @Tags     query
// @Produce  json
// @Param    from query string false "Start of the window, RFC 3339"
// @Param    to query string false "End of the window, RFC 3339"
// @Param    interval query string false "hour or day; defaults by window length"
// @Param    limit query int false "Entries in each top list" default(6)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.Stats
// @Failure  400 {object} common.ErrorResponse
// @Router   /api/v1/query/stats [get]
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	q := r.URL.Query()
	filter := dataquery.StatsFilter{Interval: q.Get("interval")}
	var err error
	if filter.From, err = parseTime(q.Get("from")); err != nil {
		common.RespondError(w, http.StatusBadRequest, "from must be an RFC 3339 timestamp")
		return
	}
	if filter.To, err = parseTime(q.Get("to")); err != nil {
		common.RespondError(w, http.StatusBadRequest, "to must be an RFC 3339 timestamp")
		return
	}
	filter.Limit, _ = strconv.Atoi(q.Get("limit"))
	if !principal.HasPermission("access", "view") && !principal.IsAdmin() {
		filter.PrincipalType = string(principal.Type())
		filter.PrincipalID = principal.ID()
	}

	stats, err := h.queryService.Stats(r.Context(), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, stats)
}

// parseTime reads an optional RFC 3339 timestamp, returning the zero time
// when it is absent.
func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

// @Summary  Run a query
// @Description Runs a read-only statement against a queryable source as the caller. The statement is planned first and refused unless the caller may query every table it references. Every attempt is recorded in the query log.
// @Tags     query
// @Accept   json
// @Produce  json
// @Param    request body dataquery.QueryInput true "Query"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.QueryResult
// @Failure  400 {object} common.ErrorResponse
// @Failure  403 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/run [post]
func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var input dataquery.QueryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.queryService.Query(r.Context(), principal, input, clientOf(r))
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary  List queries
// @Description Lists the query log, newest first. Callers without access:view only see their own queries.
// @Tags     query
// @Produce  json
// @Param    source query string false "Source name"
// @Param    decision query string false "allowed or denied"
// @Param    status query string false "running, completed, failed or denied"
// @Param    mrn query string false "Only queries that referenced this MRN"
// @Param    principal_type query string false "Principal type"
// @Param    principal_id query string false "Principal ID"
// @Param    mine query bool false "Only the caller's own queries"
// @Param    client query string false "Comma-separated clients to include: api, mcp or cli"
// @Param    limit query int false "Page size" default(50)
// @Param    offset query int false "Page offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} ListQueriesResponse
// @Router   /api/v1/query/history [get]
func (h *Handler) listQueries(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	filter := dataquery.QueryFilter{
		PrincipalType: q.Get("principal_type"),
		PrincipalID:   q.Get("principal_id"),
		SourceName:    q.Get("source"),
		Decision:      q.Get("decision"),
		Status:        q.Get("status"),
		MRN:           q.Get("mrn"),
		Clients:       splitList(q.Get("client")),
		Limit:         limit,
		Offset:        offset,
	}
	if q.Get("mine") == "true" || !principal.HasPermission("access", "view") {
		filter.PrincipalType = string(principal.Type())
		filter.PrincipalID = principal.ID()
	}

	queries, total, err := h.queryService.ListQueries(r.Context(), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	common.RespondJSON(w, http.StatusOK, ListQueriesResponse{Queries: queries, Total: total, Limit: limit, Offset: max(offset, 0)})
}

// @Summary  Get a query
// @Description Returns one query from the log, including the catalog context recorded when it ran. Callers without access:view can only read their own queries.
// @Tags     query
// @Produce  json
// @Param    id path string true "Query ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.Query
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/history/{id} [get]
func (h *Handler) getQuery(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id := r.PathValue("id")
	if uuid.Validate(id) != nil {
		respondServiceError(w, dataquery.ErrQueryNotFound)
		return
	}
	query, err := h.queryService.GetQuery(r.Context(), id)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	// Someone else's query reads as missing, so ids cannot be probed.
	ownQuery := query.PrincipalType == string(principal.Type()) && query.PrincipalID == principal.ID()
	if !ownQuery && !principal.HasPermission("access", "view") {
		respondServiceError(w, dataquery.ErrQueryNotFound)
		return
	}
	common.RespondJSON(w, http.StatusOK, query)
}

// clientOf names the client behind a REST query; the marmot CLI identifies itself in its User-Agent, so its queries are told apart from other API calls
func clientOf(r *http.Request) string {
	if strings.HasPrefix(r.UserAgent(), "marmot-cli") {
		return dataquery.ClientCLI
	}
	return dataquery.ClientAPI
}

// splitList turns a comma-separated query parameter into its non-empty parts
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
