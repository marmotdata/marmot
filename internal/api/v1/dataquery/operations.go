package dataquery

import (
	"encoding/csv"
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

// QueryRefusedResponse is returned with HTTP 403 when a query is refused. Error keeps the message clients have always read; Refusal says what refused it, what would allow it and the caller's roles.
type QueryRefusedResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	Refusal dataquery.Refusal `json:"refusal"`
} // @name QueryRefusedResponse

// respondServiceError maps data query service errors onto HTTP statuses.
func respondServiceError(w http.ResponseWriter, err error) {
	var denied *dataquery.DeniedError
	switch {
	case errors.As(err, &denied):
		common.RespondJSON(w, http.StatusForbidden, QueryRefusedResponse{Error: err.Error(), Code: "query_denied", Refusal: denied.Refusal})
	case errors.Is(err, dataquery.ErrCancelled), errors.Is(err, dataquery.ErrNotRunning):
		common.RespondError(w, http.StatusConflict, err.Error())
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
	principal, _ := common.PrincipalFromContext(r.Context())
	sources, total, err := h.queryService.ListSources(r.Context(), dataquery.SourceFilter{Search: q.Get("q"), Limit: limit, Offset: offset, Principal: principal})
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
	principal, _ := common.PrincipalFromContext(r.Context())
	tables, err := h.queryService.ListSourceTables(r.Context(), principal, r.PathValue("name"), r.URL.Query().Get("q"), limit)
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
	if !common.HasPermission(r.Context(), "access", "view") {
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
// @Param    X-Marmot-Client header string false "editor (or web) when the query comes from the web UI's query editor"
// @Failure  403 {object} QueryRefusedResponse
// @Failure  404 {object} common.ErrorResponse
// @Failure  409 {object} common.ErrorResponse "The query was cancelled"
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

	if credential, ok := common.CredentialFromContext(r.Context()); ok {
		input.Credential = credential.Type
	}
	result, err := h.queryService.Query(r.Context(), principal, input, clientOf(r))
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary  List queries
// @Description Lists the query log, newest first. Callers without access:view only see their own queries. Every filter can be passed in a link, so other pages can open the audit already filtered.
// @Tags     query
// @Produce  json
// @Param    id query string false "Only the query with this id"
// @Param    source query string false "Source name"
// @Param    caller query string false "A caller as principal_type:principal_id or as its audit subject, such as user:alice"
// @Param    outcome query string false "allowed (ran to completion), denied, failed, cancelled or running"
// @Param    reason query string false "Outcome reason: role, permission, policy, source_error, timeout, unreachable or cancelled"
// @Param    decision query string false "allowed or denied"
// @Param    status query string false "running, completed, failed, cancelled or denied"
// @Param    mrn query string false "Only queries that referenced this MRN"
// @Param    principal_type query string false "Principal type"
// @Param    principal_id query string false "Principal ID"
// @Param    mine query bool false "Only the caller's own queries"
// @Param    client query string false "Comma-separated clients to include: api, mcp, cli or editor"
// @Param    from query string false "Only queries started at or after this time (RFC 3339)"
// @Param    to query string false "Only queries started before this time (RFC 3339)"
// @Param    q query string false "Matches the caller, agent, service account, source, statement or a referenced MRN"
// @Param    limit query int false "Page size" default(50)
// @Param    offset query int false "Page offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} ListQueriesResponse
// @Failure  400 {object} common.ErrorResponse
// @Router   /api/v1/query/history [get]
func (h *Handler) listQueries(w http.ResponseWriter, r *http.Request) {
	filter, ok := h.historyFilter(w, r)
	if !ok {
		return
	}
	queries, total, err := h.queryService.ListQueries(r.Context(), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	common.RespondJSON(w, http.StatusOK, ListQueriesResponse{Queries: queries, Total: total, Limit: limit, Offset: max(filter.Offset, 0)})
}

// historyFilter reads the query log filters shared by the listing and the export, narrowing callers without access:view to their own queries. It writes the error response itself and reports false when the request is invalid.
func (h *Handler) historyFilter(w http.ResponseWriter, r *http.Request) (dataquery.QueryFilter, bool) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return dataquery.QueryFilter{}, false
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	filter := dataquery.QueryFilter{
		ID:            q.Get("id"),
		PrincipalType: q.Get("principal_type"),
		PrincipalID:   q.Get("principal_id"),
		Caller:        q.Get("caller"),
		SourceName:    q.Get("source"),
		Decision:      q.Get("decision"),
		Status:        q.Get("status"),
		Outcome:       q.Get("outcome"),
		Reason:        q.Get("reason"),
		MRN:           q.Get("mrn"),
		Clients:       splitList(q.Get("client")),
		Search:        q.Get("q"),
		Limit:         limit,
		Offset:        offset,
	}
	if filter.ID != "" && uuid.Validate(filter.ID) != nil {
		common.RespondError(w, http.StatusBadRequest, "id must be a query id")
		return filter, false
	}
	switch filter.Outcome {
	case "", dataquery.OutcomeAllowed, dataquery.OutcomeDenied, dataquery.OutcomeFailed, dataquery.OutcomeCancelled, dataquery.OutcomeRunning:
	default:
		common.RespondError(w, http.StatusBadRequest, "outcome must be allowed, denied, failed, cancelled or running")
		return filter, false
	}
	for name, dst := range map[string]*time.Time{"from": &filter.From, "to": &filter.To} {
		raw := q.Get(name)
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			common.RespondError(w, http.StatusBadRequest, name+" must be an RFC 3339 time")
			return filter, false
		}
		*dst = t
	}
	if q.Get("mine") == "true" || !common.HasPermission(r.Context(), "access", "view") {
		filter.PrincipalType = string(principal.Type())
		filter.PrincipalID = principal.ID()
	}
	return filter, true
}

// exportColumns are the CSV export's columns, in order.
var exportColumns = []string{"started_at", "completed_at", "query_id", "caller", "caller_type", "agent", "service_account", "credential", "client",
	"source", "outcome", "outcome_reason", "rows_returned", "truncated", "duration_ms", "reason", "tables", "statement"}

// exportMax bounds one export so a request cannot hold a connection open over an unbounded log; a wider range can be exported in parts.
const exportMax = 100_000

// @Summary  Export queries
// @Description Streams the query log as CSV, newest first, with the same filters as the listing (limit and offset are ignored). Callers without access:view only export their own queries. At most 100,000 rows are written.
// @Tags     query
// @Produce  text/csv
// @Param    format query string false "csv" default(csv)
// @Param    id query string false "Only the query with this id"
// @Param    source query string false "Source name"
// @Param    caller query string false "A caller as principal_type:principal_id or as its audit subject"
// @Param    outcome query string false "allowed, denied, failed, cancelled or running"
// @Param    reason query string false "Outcome reason"
// @Param    client query string false "Comma-separated clients"
// @Param    from query string false "RFC 3339"
// @Param    to query string false "RFC 3339"
// @Param    q query string false "Search"
// @Param    mine query bool false "Only the caller's own queries"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {string} string "CSV"
// @Failure  400 {object} common.ErrorResponse
// @Router   /api/v1/query/history/export [get]
func (h *Handler) exportQueries(w http.ResponseWriter, r *http.Request) {
	if format := r.URL.Query().Get("format"); format != "" && format != "csv" {
		common.RespondError(w, http.StatusBadRequest, "format must be csv")
		return
	}
	filter, ok := h.historyFilter(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="query-audit-`+time.Now().UTC().Format("20060102-150405")+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	out := csv.NewWriter(w)
	flusher, _ := w.(http.Flusher)
	_ = out.Write(exportColumns)

	written := 0
	errStop := errors.New("export limit reached")
	err := h.queryService.EachQuery(r.Context(), filter, func(q *dataquery.Query) error {
		if written >= exportMax {
			return errStop
		}
		if err := out.Write(exportRow(q)); err != nil {
			return err
		}
		written++
		// Flushing every few hundred rows keeps memory flat and shows the download progressing.
		if written%500 == 0 {
			out.Flush()
			if flusher != nil {
				flusher.Flush()
			}
		}
		return out.Error()
	})
	out.Flush()
	if err != nil && !errors.Is(err, errStop) {
		// The header is already sent, so the failure can only be logged; the truncated file ends early.
		log.Error().Err(err).Msg("Query log export failed")
	}
}

func exportRow(q *dataquery.Query) []string {
	optional := func(v *string) string {
		if v == nil {
			return ""
		}
		return csvSafe(*v)
	}
	completed, duration, rows := "", "", ""
	if q.CompletedAt != nil {
		completed = q.CompletedAt.UTC().Format(time.RFC3339)
		duration = strconv.FormatInt(q.CompletedAt.Sub(q.StartedAt).Milliseconds(), 10)
	}
	if q.RowsReturned != nil {
		rows = strconv.FormatInt(*q.RowsReturned, 10)
	}
	outcome := q.Status
	switch {
	case q.Decision == dataquery.DecisionDenied:
		outcome = dataquery.OutcomeDenied
	case q.Status == dataquery.StatusCompleted:
		outcome = dataquery.OutcomeAllowed
	}
	reason := ""
	if q.Decision == dataquery.DecisionDenied {
		reason, _ = q.DecisionDetail["reason"].(string)
	} else if q.Error != nil {
		reason = *q.Error
	}
	caller := q.AuditSubject
	if _, name, found := strings.Cut(caller, ":"); found {
		caller = name
	}
	var tables []string
	for _, m := range q.ReferencedMRNs {
		if !strings.HasPrefix(m, "mrn://source/") {
			tables = append(tables, m)
		}
	}
	return []string{
		q.StartedAt.UTC().Format(time.RFC3339), completed, q.ID, csvSafe(caller), q.PrincipalType,
		optional(q.Agent), optional(q.ServiceAccount), optional(q.Credential), q.Client,
		csvSafe(q.SourceName), outcome, q.OutcomeReason, rows, strconv.FormatBool(q.Truncated), duration,
		csvSafe(reason), csvSafe(strings.Join(tables, " ")), csvSafe(q.Statement),
	}
}

// csvSafe stops a spreadsheet reading a cell that starts with a formula character as a formula, since callers, statements and reasons are free text.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
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
	if !ownQuery && !common.HasPermission(r.Context(), "access", "view") {
		respondServiceError(w, dataquery.ErrQueryNotFound)
		return
	}
	common.RespondJSON(w, http.StatusOK, query)
}

// clientOf names the client behind a REST query: the marmot CLI identifies itself in its User-Agent and the web UI sends X-Marmot-Client on every request, so editor queries are told apart from other API calls. Both are informational and taken at the caller's word.
func clientOf(r *http.Request) string {
	if strings.HasPrefix(r.UserAgent(), "marmot-cli") {
		return dataquery.ClientCLI
	}
	switch r.Header.Get("X-Marmot-Client") {
	case "editor", "web":
		return dataquery.ClientEditor
	}
	return dataquery.ClientAPI
}

// @Summary  Cancel a query
// @Description Stops a running query. The query must be the caller's own unless the caller holds access:manage. A query runs on one replica, which is the only one that can stop it; elsewhere this answers 409, and the caller can still stop its own query by closing the request that runs it.
// @Tags     query
// @Produce  json
// @Param    id path string true "Query ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  204
// @Failure  404 {object} common.ErrorResponse
// @Failure  409 {object} common.ErrorResponse "The query is not running here"
// @Router   /api/v1/query/history/{id}/cancel [post]
func (h *Handler) cancelQuery(w http.ResponseWriter, r *http.Request) {
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
	ownQuery := query.PrincipalType == string(principal.Type()) && query.PrincipalID == principal.ID()
	if !ownQuery && !common.HasPermission(r.Context(), "access", "manage") {
		respondServiceError(w, dataquery.ErrQueryNotFound)
		return
	}
	if query.Status != dataquery.StatusRunning {
		common.RespondError(w, http.StatusConflict, "the query has already finished")
		return
	}
	if err := h.queryService.Cancel(r.Context(), id); err != nil {
		respondServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  Request query access
// @Description Asks the people who can grant access for what a refused query needed, and notifies them in Marmot. Pass the refused query's id, or a source when there is no query. Asking again for the same access within a day returns the earlier request without notifying anyone again.
// @Tags     query
// @Accept   json
// @Produce  json
// @Param    request body dataquery.AccessRequestInput true "Request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  201 {object} dataquery.AccessRequest
// @Success  200 {object} dataquery.AccessRequest "An identical recent request"
// @Failure  400 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @Router   /api/v1/query/access-requests [post]
func (h *Handler) requestAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var input dataquery.AccessRequestInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if input.QueryID != "" && uuid.Validate(input.QueryID) != nil {
		respondServiceError(w, dataquery.ErrQueryNotFound)
		return
	}
	req, err := h.queryService.RequestAccess(r.Context(), principal, input)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	status := http.StatusCreated
	if req.Repeat {
		status = http.StatusOK
	}
	common.RespondJSON(w, status, req)
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
