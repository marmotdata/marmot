package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

// PreviewResponse represents the response structure for preview data
type PreviewResponse struct {
	ColumnNames []string        `json:"column_names"`
	Rows        [][]interface{} `json:"rows"`
	TotalRows   *int            `json:"total_rows,omitempty"`
}

// getAssetPreview handles GET /api/v1/assets/preview/{id}
// @Summary Get preview data for an asset
// @Description Fetches sample data from the asset's data source. Requires assets:preview permission.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} PreviewResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse "Missing assets:preview permission"
// @Failure 404 {object} common.ErrorResponse
// @Failure 501 {object} common.ErrorResponse "Data preview not supported for this asset"
// @Failure 500 {object} common.ErrorResponse
// @Router /api/v1/assets/preview/{id} [get]
func (h *Handler) getAssetPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assetID := r.PathValue("id")

	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset ID is required")
		return
	}

	assetObj, err := h.assetService.Get(ctx, assetID)
	if err != nil {
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to fetch asset")
		common.RespondError(w, http.StatusNotFound, "asset not found")
		return
	}

	if !isTableAsset(assetObj.Type) {
		common.RespondError(w, http.StatusNotImplemented, "preview not supported for this asset type")
		return
	}

	if len(assetObj.Providers) == 0 {
		common.RespondError(w, http.StatusBadRequest, "asset has no provider information")
		return
	}

	providerName := assetObj.Providers[0]

	// we lower provider name to match plugin id but this is an assumption.
	// we should consider storing plugin id separately.
	pluginSource, err := plugin.GetRegistry().GetSource(strings.ToLower(providerName))
	if err != nil {
		log.Error().Err(err).Str("provider", providerName).Msg("Plugin not found")
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("plugin not found for asset provider: %s", providerName))
		return
	}

	dataFetcher, ok := pluginSource.(plugin.DataFetcher)
	if !ok {
		log.Debug().Str("provider", providerName).Msg("Plugin does not support data preview")
		common.RespondError(w, http.StatusNotImplemented, fmt.Sprintf("data preview not supported for %s assets", providerName))
		return
	}

	// Get schedule from asset association
	schedule, err := h.scheduleService.GetScheduleForAsset(ctx, assetObj.ID)
	if err != nil {
		if errors.Is(err, runs.ErrScheduleNotFound) {
			common.RespondError(w, http.StatusBadRequest, "no schedule associated with this asset; run an ingestion job first")
			return
		}
		log.Error().Err(err).Str("asset_id", assetID).Msg("Failed to get schedule for asset")
		common.RespondError(w, http.StatusInternalServerError, "failed to look up schedule for asset")
		return
	}

	var pluginConfig plugin.RawPluginConfig

	if h.encryptor != nil {
		if err := runs.DecryptScheduleConfig(schedule, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to decrypt schedule config")
			common.RespondError(w, http.StatusInternalServerError, "failed to decrypt schedule configuration")
			return
		}
	}

	pluginConfig = schedule.Config

	// Set a timeout for data fetching
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Info().
		Str("asset_id", assetID).
		Str("provider", providerName).
		Msg("Fetching preview data")

	columnNames, rows, err := dataFetcher.FetchSampleData(fetchCtx, pluginConfig, assetObj)
	if err != nil {
		log.Error().Err(err).
			Str("asset_id", assetID).
			Str("provider", providerName).
			Msg("Failed to fetch sample data")

		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to fetch data: %v", err))
		return
	}

	response := PreviewResponse{
		ColumnNames: columnNames,
		Rows:        rows,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
	}
}

// isTableAsset checks if an asset type represents a table-like entity
func isTableAsset(assetType string) bool {
	if assetType == "" {
		return false
	}
	tableKeywords := []string{"table", "view"}
	lowerType := strings.ToLower(assetType)
	for _, keyword := range tableKeywords {
		if strings.Contains(lowerType, keyword) {
			return true
		}
	}
	return false
}
