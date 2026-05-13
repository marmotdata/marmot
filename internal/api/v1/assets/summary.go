package assets

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

// AssetSummaryResponse mirrors the shape served by /assets/summary. Each
// entry under Types includes a count and the provider that contributed it.
type AssetSummaryResponse struct {
	Types     map[string]asset.AssetTypeSummary `json:"types"`
	Providers map[string]int                    `json:"providers"`
	Tags      map[string]int                    `json:"tags"`
} // @name AssetSummaryResponse

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
