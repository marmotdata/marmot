package dataquery

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataquery"
)

// @Summary  Query stats for one source
// @Description Counts a source's queries over a window, the last seven days by default: how many were asked, refused by policy and failed at the source. Callers without access:view see only their own queries.
// @Tags     query
// @Produce  json
// @Param    name path string true "Source name"
// @Param    from query string false "Window start, RFC 3339; defaults to seven days before to"
// @Param    to query string false "Window end, RFC 3339; defaults to now"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.SourceStats
// @Failure  400 {object} common.ErrorResponse
// @Failure  401 {object} common.ErrorResponse
// @Failure  403 {object} common.ErrorResponse
// @Router   /api/v1/query/sources/{name}/stats [get]
func (h *Handler) sourceStats(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	q := r.URL.Query()
	var filter dataquery.StatsFilter
	var err error
	if filter.From, err = parseTime(q.Get("from")); err != nil {
		common.RespondError(w, http.StatusBadRequest, "from must be an RFC 3339 timestamp")
		return
	}
	if filter.To, err = parseTime(q.Get("to")); err != nil {
		common.RespondError(w, http.StatusBadRequest, "to must be an RFC 3339 timestamp")
		return
	}
	// The same scoping as the stats dashboard: without access:view a caller counts only their own queries.
	if !common.HasPermission(r.Context(), "access", "view") {
		filter.PrincipalType = string(principal.Type())
		filter.PrincipalID = principal.ID()
	}

	stats, err := h.queryService.SourceStats(r.Context(), r.PathValue("name"), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, stats)
}
