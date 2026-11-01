package assets

import (
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/rs/zerolog/log"
)

// @Summary List stale assets
// @Description List assets whose pipeline has not refreshed them for more than twice its schedule interval, longest unrefreshed first. Assets no enabled pipeline refreshes are never stale.
// @Tags assets
// @Produce json
// @Param limit query int false "Maximum number of assets to return (1 to 100)" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetsignal.StaleList
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetsStale
// @Router /api/v1/assets/stale [get]
func (h *Handler) listStaleAssets(w http.ResponseWriter, r *http.Request) {
	limit, offset := 20, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	list, err := h.signals.ListStale(r.Context(), limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list stale assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list stale assets")
		return
	}
	common.RespondJSON(w, http.StatusOK, list)
}
