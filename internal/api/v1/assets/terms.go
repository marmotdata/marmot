package assets

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type AddTermsRequest struct {
	TermIDs []string `json:"term_ids" validate:"required,min=1"`
}

type RemoveTermRequest struct {
	TermID string `json:"term_id" validate:"required"`
}

// @Summary Add glossary terms to asset
// @Description Associate one or more glossary terms with an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param terms body AddTermsRequest true "Term IDs to add"
// @Success 200 {array} asset.AssetTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /assets/{id}/terms [post]
func (h *Handler) addTerms(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/terms/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := parts[0]

	var input AddTermsRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(input.TermIDs) == 0 {
		common.RespondError(w, http.StatusBadRequest, "At least one term ID is required")
		return
	}

	// Get user from context
	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	if err := h.assetService.AddTerms(r.Context(), id, input.TermIDs, "user", usr.ID); err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to add terms to asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Return updated list of terms
	terms, err := h.assetService.GetTerms(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get asset terms")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, terms)
}

// @Summary Remove glossary term from asset
// @Description Remove a glossary term association from an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param term body RemoveTermRequest true "Term ID to remove"
// @Success 200 {array} asset.AssetTerm
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Router /assets/{id}/terms [delete]
func (h *Handler) removeTerm(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/terms/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := parts[0]

	var input RemoveTermRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.TermID == "" {
		common.RespondError(w, http.StatusBadRequest, "Term ID is required")
		return
	}

	if err := h.assetService.RemoveTerm(r.Context(), id, input.TermID); err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset or term association not found")
		default:
			log.Error().Err(err).Str("id", id).Str("term_id", input.TermID).Msg("Failed to remove term from asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Return updated list of terms
	terms, err := h.assetService.GetTerms(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get asset terms")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, terms)
}

// @Summary Get asset's glossary terms
// @Description Retrieve all glossary terms associated with an asset
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {array} asset.AssetTerm
// @Failure 404 {object} common.ErrorResponse
// @Router /assets/{id}/terms [get]
func (h *Handler) getAssetTerms(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/terms/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		common.RespondError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	id := parts[0]

	terms, err := h.assetService.GetTerms(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get asset terms")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, terms)
}

// @Summary Get assets by glossary term
// @Description Retrieve all assets associated with a specific glossary term
// @Tags assets
// @Produce json
// @Param term_id path string true "Glossary Term ID"
// @Param limit query int false "Maximum number of assets" default(20)
// @Param offset query int false "Pagination offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/by-glossary-term/{term_id} [get]
func (h *Handler) getAssetsByTerm(w http.ResponseWriter, r *http.Request) {
	termID := r.PathValue("term_id")
	if termID == "" {
		common.RespondError(w, http.StatusBadRequest, "Missing term_id")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	assets, total, err := h.assetService.GetAssetsByTerm(r.Context(), termID, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("term_id", termID).Msg("Failed to get assets by term")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	response := map[string]interface{}{
		"assets": assets,
		"total":  total,
	}

	common.RespondJSON(w, http.StatusOK, response)
}
