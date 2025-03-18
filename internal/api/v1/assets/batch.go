package assets

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/services/user"
	"github.com/rs/zerolog/log"
)

type BatchCreateRequest struct {
	Assets []CreateRequest        `json:"assets" validate:"required,min=1"`
	Config plugin.RawPluginConfig `json:"config"`
}

type BatchCreateResponse struct {
	Assets []BatchAssetResult `json:"assets"`
}

type BatchAssetResult struct {
	Asset  *asset.Asset `json:"asset"`
	Status string       `json:"status"`
	Error  string       `json:"error,omitempty"`
}

// @Summary Batch create assets
// @Description Create or update multiple assets in a single request
// @Tags assets
// @Accept json
// @Produce json
// @Param request body BatchCreateRequest true "Batch creation request"
// @Success 200 {object} BatchCreateResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/batch [post]
func (h *Handler) batchCreateAssets(w http.ResponseWriter, r *http.Request) {
	var req BatchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Assets) == 0 {
		common.RespondError(w, http.StatusBadRequest, "At least one asset is required")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	results := make([]BatchAssetResult, 0, len(req.Assets))
	for _, assetReq := range req.Assets {
		result := BatchAssetResult{}

		mrn := mrn.New(assetReq.Type, assetReq.Providers[0], assetReq.Name)
		existingAsset, _ := h.assetService.GetByMRN(r.Context(), mrn)

		if existingAsset != nil {
			input := asset.UpdateInput{
				Name:          &assetReq.Name,
				Type:          assetReq.Type,
				Providers:     assetReq.Providers,
				Description:   assetReq.Description,
				Metadata:      assetReq.Metadata,
				Schema:        assetReq.Schema,
				Tags:          assetReq.Tags,
				Sources:       assetReq.Sources,
				ExternalLinks: assetReq.ExternalLinks,
			}

			updated, err := h.assetService.Update(r.Context(), existingAsset.ID, input)
			if err != nil {
				result.Error = fmt.Sprintf("Failed to update asset: %v", err)
				log.Error().Err(err).Str("mrn", mrn).Msg("Failed to update asset in batch")
			} else {
				result.Asset = updated
				result.Status = "updated"
			}
		} else {
			input := asset.CreateInput{
				Name:          &assetReq.Name,
				Type:          assetReq.Type,
				Providers:     assetReq.Providers,
				Description:   assetReq.Description,
				Metadata:      assetReq.Metadata,
				Schema:        assetReq.Schema,
				Tags:          assetReq.Tags,
				Sources:       assetReq.Sources,
				ExternalLinks: assetReq.ExternalLinks,
				MRN:           &mrn,
				CreatedBy:     usr.Name,
			}

			created, err := h.assetService.Create(r.Context(), input)
			if err != nil {
				result.Error = fmt.Sprintf("Failed to create asset: %v", err)
				log.Error().Err(err).Str("mrn", mrn).Msg("Failed to create asset in batch")
			} else {
				result.Asset = created
				result.Status = "created"
			}
		}

		results = append(results, result)
	}

	common.RespondJSON(w, http.StatusOK, BatchCreateResponse{Assets: results})
}
