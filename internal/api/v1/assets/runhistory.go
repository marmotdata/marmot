package assets

import (
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type RunHistoryResponse struct {
	RunHistory []*asset.RunHistory `json:"run_history"`
	Total      int                 `json:"total"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

// @Summary Get asset run history
// @Description Get paginated run history for a specific asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param limit query int false "Number of items per page" default(10)
// @Param offset query int false "Number of items to skip" default(0)
// @Success 200 {object} RunHistoryResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/{id}/run-history [get]
func (h *Handler) getRunHistory(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	runHistory, total, err := h.assetService.GetRunHistory(r.Context(), assetID, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to get run history")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get run history")
		return
	}

	response := RunHistoryResponse{
		RunHistory: runHistory,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

type HistogramResponse struct {
	Buckets []asset.HistogramBucket `json:"buckets"`
	Period  string                  `json:"period"`
}

// @Summary Get asset run history histogram
// @Description Get histogram data for asset run history over specified period
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Param period query string false "Time period (7d, 30d, 90d)" default(30d)
// @Success 200 {object} HistogramResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/{id}/run-history/histogram [get]
func (h *Handler) getRunHistoryHistogram(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	default:
		common.RespondError(w, http.StatusBadRequest, "Invalid period. Supported: 7d, 30d, 90d")
		return
	}

	histogram, err := h.assetService.GetRunHistoryHistogram(r.Context(), assetID, days)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to get run history histogram")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get run history histogram")
		return
	}

	response := HistogramResponse{
		Buckets: histogram,
		Period:  period,
	}

	common.RespondJSON(w, http.StatusOK, response)
}
