package dataquery

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataquery"
)

// @Summary  Query stats for one asset
// @Description Summarises the queries through Marmot that read an asset over a window, the last 14 days by default: how many, how many distinct people and agents asked, the median run time, how many policy denied and a count per day. Callers without access:view see only their own queries.
// @Tags     query
// @Produce  json
// @Param    mrn query string true "Asset MRN"
// @Param    from query string false "Window start, RFC 3339; defaults to 14 days before to"
// @Param    to query string false "Window end, RFC 3339; defaults to now"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} dataquery.AssetStats
// @Failure  400 {object} common.ErrorResponse
// @Failure  401 {object} common.ErrorResponse
// @Failure  403 {object} common.ErrorResponse
// @Router   /api/v1/query/asset-stats [get]
func (h *Handler) assetStats(w http.ResponseWriter, r *http.Request) {
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
	if !common.HasPermission(r.Context(), "access", "view") {
		filter.PrincipalType = string(principal.Type())
		filter.PrincipalID = principal.ID()
	}

	stats, err := h.queryService.AssetStats(r.Context(), q.Get("mrn"), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, stats)
}
