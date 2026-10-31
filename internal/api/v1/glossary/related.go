package glossary

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/glossary"
	"github.com/rs/zerolog/log"
)

type RelatedTermsResponse struct {
	RelatedTerms []glossary.RelatedTerm `json:"related_terms"`
	Total        int                    `json:"total"`
} // @name RelatedTermsResponse

type SuggestedAssetsResponse struct {
	Assets []glossary.SuggestedAsset `json:"assets"`
	Total  int                       `json:"total"`
} // @name GlossarySuggestedAssetsResponse

// @Summary Get related terms
// @Description List the terms related to a glossary term
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} RelatedTermsResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getGlossaryRelatedID
// @Router /api/v1/glossary/related/{id} [get]
func (h *Handler) getRelatedTerms(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	related, err := h.glossaryService.GetRelatedTerms(r.Context(), id)
	h.respondRelated(w, id, related, err)
}

// @Summary Add a related term
// @Description Relate two glossary terms to each other. The link shows on both terms.
// @Tags glossary
// @Accept json
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param term body AddRelatedTermRequest true "Term to relate"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} RelatedTermsResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postGlossaryRelatedID
// @Router /api/v1/glossary/related/{id} [post]
func (h *Handler) addRelatedTerm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req AddRelatedTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TermID == "" {
		common.RespondError(w, http.StatusBadRequest, "term_id is required")
		return
	}
	related, err := h.glossaryService.AddRelatedTerm(r.Context(), id, req.TermID)
	h.respondRelated(w, id, related, err)
}

// @Summary Remove a related term
// @Description Unrelate two glossary terms. The link goes from both terms.
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param related_id path string true "Related Term ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} RelatedTermsResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteGlossaryRelatedIDRelatedID
// @Router /api/v1/glossary/related/{id}/{related_id} [delete]
func (h *Handler) removeRelatedTerm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	related, err := h.glossaryService.RemoveRelatedTerm(r.Context(), id, r.PathValue("related_id"))
	h.respondRelated(w, id, related, err)
}

func (h *Handler) respondRelated(w http.ResponseWriter, id string, related []glossary.RelatedTerm, err error) {
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound), errors.Is(err, glossary.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		case errors.Is(err, glossary.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to manage related terms")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	common.RespondJSON(w, http.StatusOK, RelatedTermsResponse{RelatedTerms: related, Total: len(related)})
}

// @Summary Suggest assets for a term
// @Description Assets whose name, tags or description mention the term's name and that are not linked to it yet, best match first
// @Tags glossary
// @Produce json
// @Param id path string true "Glossary Term ID"
// @Param limit query int false "Maximum number of assets to return" default(10)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} SuggestedAssetsResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getGlossarySuggestedAssetsID
// @Router /api/v1/glossary/suggested-assets/{id} [get]
func (h *Handler) suggestAssets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	assets, err := h.glossaryService.SuggestAssets(r.Context(), id, limit)
	if err != nil {
		switch {
		case errors.Is(err, glossary.ErrTermNotFound), errors.Is(err, glossary.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Glossary term not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to suggest assets")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	common.RespondJSON(w, http.StatusOK, SuggestedAssetsResponse{Assets: assets, Total: len(assets)})
}
