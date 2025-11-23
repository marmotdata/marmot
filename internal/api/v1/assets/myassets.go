package assets

import (
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

// @Summary Get user's assets
// @Description Get assets owned by the current user or their teams
// @Tags assets
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} SearchResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/my-assets [get]
func (h *Handler) getMyAssets(w http.ResponseWriter, r *http.Request) {
	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}
	userID := usr.ID

	queryValues := r.URL.Query()
	limit := 20
	offset := 0
	if l := queryValues.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := queryValues.Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	teams, err := h.teamService.ListUserTeams(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get user teams")
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch user teams")
		return
	}

	teamIDs := make([]string, len(teams))
	for i, team := range teams {
		teamIDs[i] = team.ID
	}

	assets, total, err := h.assetService.GetMyAssets(r.Context(), userID, teamIDs, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to fetch user assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to fetch assets")
		return
	}

	response := SearchResponse{
		Assets: assets,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	common.RespondJSON(w, http.StatusOK, response)
}
