package assets

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/rs/zerolog/log"
)

type AssetSummaryResponse struct {
	Types    map[string]int `json:"types"`
	Services map[string]int `json:"services"`
	Tags     map[string]int `json:"tags"`
}

// @Summary Get asset summary
// @Description Get the total count of assets by type
// @Tags assets
// @Accept json
// @Produce json
// @Success 200 {object} AssetSummaryResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/summary [get]
func (h *Handler) summaryAssets(w http.ResponseWriter, r *http.Request) {
	summary, err := h.assetService.Summary(r.Context())
	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", r.URL.Path).
			Str("method", r.Method).
			Msg("Failed to get asset summary")

		common.RespondError(w, http.StatusInternalServerError, "Failed to get asset summary")
		return
	}

	common.RespondJSON(w, http.StatusOK, summary)
}
