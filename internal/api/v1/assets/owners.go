package assets

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/team"
)

func (h *Handler) listAssetOwners(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id query parameter is required")
		return
	}

	owners, err := h.teamService.ListAssetOwners(r.Context(), assetID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list asset owners")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"owners": owners,
	})
}

func (h *Handler) addAssetOwner(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id query parameter is required")
		return
	}

	var req struct {
		OwnerType string `json:"owner_type"`
		OwnerID   string `json:"owner_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.OwnerType == "" || req.OwnerID == "" {
		common.RespondError(w, http.StatusBadRequest, "Owner type and ID are required")
		return
	}

	if req.OwnerType != team.OwnerTypeUser && req.OwnerType != team.OwnerTypeTeam {
		common.RespondError(w, http.StatusBadRequest, "Invalid owner type")
		return
	}

	err := h.teamService.AddAssetOwner(r.Context(), assetID, req.OwnerType, req.OwnerID)
	if err != nil {
		if err == team.ErrTeamNotFound {
			common.RespondError(w, http.StatusNotFound, "Team not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to add asset owner")
		return
	}

	common.RespondJSON(w, http.StatusCreated, map[string]string{"message": "Owner added"})
}

func (h *Handler) removeAssetOwner(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id query parameter is required")
		return
	}

	ownerType := r.URL.Query().Get("owner_type")
	ownerID := r.URL.Query().Get("owner_id")

	if ownerType == "" || ownerID == "" {
		common.RespondError(w, http.StatusBadRequest, "Owner type and ID are required")
		return
	}

	err := h.teamService.RemoveAssetOwner(r.Context(), assetID, ownerType, ownerID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to remove asset owner")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Owner removed"})
}
