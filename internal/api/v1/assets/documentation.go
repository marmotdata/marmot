package assets

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/services/assetdocs"
	"github.com/rs/zerolog/log"
)

type DocumentationCreateRequest struct {
	MRN     string `json:"mrn" validate:"required"`
	Content string `json:"content" validate:"required"`
	Source  string `json:"source" validate:"required"`
}

type BatchDocumentationRequest struct {
	Documentation []assetdocs.Documentation `json:"documentation" validate:"required,min=1"`
}

type BatchDocumentationResponse struct {
	Results []BatchDocumentationResult `json:"results"`
}

type BatchDocumentationResult struct {
	Documentation assetdocs.Documentation `json:"documentation"`
	Status        string                  `json:"status"`
	Error         string                  `json:"error,omitempty"`
}

// @Summary Create asset documentation
// @Description Create or update documentation for an asset
// @Tags assets
// @Accept json
// @Produce json
// @Param request body DocumentationCreateRequest true "Documentation creation request"
// @Success 200 {object} assetdocs.Documentation
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/documentation [post]
func (h *Handler) createAssetDocumentation(w http.ResponseWriter, r *http.Request) {
	var req DocumentationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	doc := assetdocs.Documentation{
		MRN:       req.MRN,
		Content:   req.Content,
		Source:    req.Source,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.assetDocsService.Create(r.Context(), doc); err != nil {
		log.Error().Err(err).Str("mrn", req.MRN).Msg("Failed to create documentation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create documentation")
		return
	}

	common.RespondJSON(w, http.StatusOK, doc)
}

// @Summary Get asset documentation
// @Description Get documentation for a specific asset
// @Tags assets
// @Produce json
// @Param mrn path string true "Asset MRN" format(url)
// @Success 200 {array} assetdocs.Documentation
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/documentation/{mrn} [get]
func (h *Handler) getAssetDocumentation(w http.ResponseWriter, r *http.Request) {
	encodedMRN := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/documentation/")
	if encodedMRN == "" {
		common.RespondError(w, http.StatusBadRequest, "MRN required")
		return
	}

	mrn, err := url.QueryUnescape(encodedMRN)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid MRN format")
		return
	}

	docs, err := h.assetDocsService.Get(r.Context(), mrn)
	if err != nil {
		log.Error().Err(err).Str("mrn", mrn).Msg("Failed to get documentation")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get documentation")
		return
	}

	if len(docs) == 0 {
		common.RespondError(w, http.StatusNotFound, "Documentation not found")
		return
	}

	common.RespondJSON(w, http.StatusOK, docs)
}

// @Summary Batch create documentation
// @Description Create or update documentation for multiple assets
// @Tags assets
// @Accept json
// @Produce json
// @Param request body BatchDocumentationRequest true "Batch documentation request"
// @Success 200 {object} BatchDocumentationResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/documentation/batch [post]
func (h *Handler) batchCreateDocumentation(w http.ResponseWriter, r *http.Request) {
	var req BatchDocumentationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Documentation) == 0 {
		common.RespondError(w, http.StatusBadRequest, "At least one documentation entry is required")
		return
	}

	results := make([]BatchDocumentationResult, 0, len(req.Documentation))
	for _, doc := range req.Documentation {
		result := BatchDocumentationResult{Documentation: doc}

		existing, _ := h.assetDocsService.Get(r.Context(), doc.MRN)
		doc.CreatedAt = time.Now()
		doc.UpdatedAt = time.Now()

		if err := h.assetDocsService.Create(r.Context(), doc); err != nil {
			result.Error = err.Error()
		} else {
			if len(existing) > 0 {
				result.Status = "updated"
			} else {
				result.Status = "created"
			}
		}

		results = append(results, result)
	}

	common.RespondJSON(w, http.StatusOK, BatchDocumentationResponse{Results: results})
}
