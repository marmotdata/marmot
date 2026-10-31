package assets

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type BatchGetRequest struct {
	IDs []string `json:"ids" validate:"required,max=200"`
} // @name BatchGetAssetsRequest

type BatchGetResponse struct {
	Assets []*asset.Asset `json:"assets"`
} // @name BatchGetAssetsResponse

// @Summary Get several assets
// @Description Read up to 200 assets by id in one request, in the order asked for. Ids that match no asset are left out. Unlike reading one asset, this does not count as a view.
// @Tags assets
// @Accept json
// @Produce json
// @Param request body BatchGetRequest true "Asset ids"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} BatchGetResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postAssetsBatch
// @Router /api/v1/assets/batch [post]
func (h *Handler) batchGetAssets(w http.ResponseWriter, r *http.Request) {
	var req BatchGetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	assets, err := h.assetService.GetByIDs(r.Context(), req.IDs)
	if err != nil {
		if errors.Is(err, asset.ErrInvalidInput) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Error().Err(err).Int("count", len(req.IDs)).Msg("Failed to get assets by id")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, BatchGetResponse{Assets: assets})
}
