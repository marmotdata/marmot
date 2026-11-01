package assetrules

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	"github.com/rs/zerolog/log"
)

type CreateRequest struct {
	Name            string                    `json:"name"`
	Description     *string                   `json:"description,omitempty"`
	Links           []assetrule.ExternalLink  `json:"links,omitempty"`
	TermIDs         []string                  `json:"term_ids,omitempty"`
	RuleType        string                    `json:"rule_type"`
	QueryExpression *string                   `json:"query_expression,omitempty"`
	MetadataField   *string                   `json:"metadata_field,omitempty"`
	PatternType     *string                   `json:"pattern_type,omitempty"`
	PatternValue    *string                   `json:"pattern_value,omitempty"`
	Priority        int                       `json:"priority"`
	IsEnabled       bool                      `json:"is_enabled"`
} // @name CreateAssetRuleRequest

type UpdateRequest struct {
	Name            *string                   `json:"name,omitempty"`
	Description     *string                   `json:"description,omitempty"`
	Links           []assetrule.ExternalLink  `json:"links,omitempty"`
	TermIDs         []string                  `json:"term_ids,omitempty"`
	RuleType        *string                   `json:"rule_type,omitempty"`
	QueryExpression *string                   `json:"query_expression,omitempty"`
	MetadataField   *string                   `json:"metadata_field,omitempty"`
	PatternType     *string                   `json:"pattern_type,omitempty"`
	PatternValue    *string                   `json:"pattern_value,omitempty"`
	Priority        *int                      `json:"priority,omitempty"`
	IsEnabled       *bool                     `json:"is_enabled,omitempty"`
} // @name UpdateAssetRuleRequest

type PreviewRequest struct {
	RuleType        string  `json:"rule_type"`
	QueryExpression *string `json:"query_expression,omitempty"`
	MetadataField   *string `json:"metadata_field,omitempty"`
	PatternType     *string `json:"pattern_type,omitempty"`
	PatternValue    *string `json:"pattern_value,omitempty"`
	Limit           int     `json:"limit,omitempty"`
} // @name PreviewRequest

// @Summary Create an asset rule
// @Description Create a new asset rule that applies enrichments to matching assets
// @Tags asset-rules
// @Accept json
// @Produce json
// @Param rule body CreateRequest true "Asset rule creation request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} assetrule.AssetRule
// @Failure 400 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postAssetRules
// @Router /api/v1/asset-rules/ [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	createdBy := common.CreatedBy(r.Context())

	input := assetrule.CreateInput{
		Name:            req.Name,
		Description:     req.Description,
		Links:           req.Links,
		TermIDs:         req.TermIDs,
		RuleType:        enrichment.RuleType(req.RuleType),
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
		Priority:        req.Priority,
		IsEnabled:       req.IsEnabled,
	}

	rule, err := h.assetRuleService.Create(r.Context(), input, createdBy)
	if err != nil {
		switch {
		case errors.Is(err, assetrule.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, assetrule.ErrConflict):
			common.RespondError(w, http.StatusConflict, "Asset rule with this name already exists")
		default:
			log.Error().Err(err).Msg("Failed to create asset rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, rule)
}

// @Summary Get an asset rule
// @Description Get an asset rule by ID
// @Tags asset-rules
// @Produce json
// @Param id path string true "Asset rule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetrule.AssetRule
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetRulesID
// @Router /api/v1/asset-rules/{id} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/asset-rules/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset rule ID required")
		return
	}

	rule, err := h.assetRuleService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, assetrule.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset rule not found")
		} else {
			log.Error().Err(err).Str("id", id).Msg("Failed to get asset rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, rule)
}

// @Summary Update an asset rule
// @Description Update an existing asset rule
// @Tags asset-rules
// @Accept json
// @Produce json
// @Param id path string true "Asset rule ID"
// @Param rule body UpdateRequest true "Asset rule update request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetrule.AssetRule
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putAssetRulesID
// @Router /api/v1/asset-rules/{id} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/asset-rules/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset rule ID required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := assetrule.UpdateInput{
		Name:            req.Name,
		Description:     req.Description,
		Links:           req.Links,
		TermIDs:         req.TermIDs,
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
		Priority:        req.Priority,
		IsEnabled:       req.IsEnabled,
		UpdatedBy:       common.CreatedBy(r.Context()),
	}
	if req.RuleType != nil {
		rt := enrichment.RuleType(*req.RuleType)
		input.RuleType = &rt
	}

	rule, err := h.assetRuleService.Update(r.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, assetrule.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset rule not found")
		case errors.Is(err, assetrule.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, assetrule.ErrConflict):
			common.RespondError(w, http.StatusConflict, "Asset rule with this name already exists")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update asset rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, rule)
}

// @Summary Delete an asset rule
// @Description Delete an asset rule by ID
// @Tags asset-rules
// @Param id path string true "Asset rule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteAssetRulesID
// @Router /api/v1/asset-rules/{id} [delete]
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/asset-rules/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset rule ID required")
		return
	}

	err := h.assetRuleService.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, assetrule.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset rule not found")
		} else {
			log.Error().Err(err).Str("id", id).Msg("Failed to delete asset rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary List asset rules
// @Description List all asset rules with pagination
// @Tags asset-rules
// @Produce json
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetrule.ListResult
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetRulesList
// @Router /api/v1/asset-rules/list [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.assetRuleService.List(r.Context(), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list asset rules")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Search asset rules
// @Description Search asset rules by name
// @Tags asset-rules
// @Produce json
// @Param query query string false "Search query"
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetrule.ListResult
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetRulesSearch
// @Router /api/v1/asset-rules/search [get]
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := assetrule.SearchFilter{
		Query:  r.URL.Query().Get("query"),
		Limit:  limit,
		Offset: offset,
	}

	result, err := h.assetRuleService.Search(r.Context(), filter)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search asset rules")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Preview an asset rule
// @Description Preview which assets would match a rule configuration
// @Tags asset-rules
// @Accept json
// @Produce json
// @Param rule body PreviewRequest true "Rule preview request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} assetrule.RulePreview
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postAssetRulesPreview
// @Router /api/v1/asset-rules/preview [post]
func (h *Handler) previewRule(w http.ResponseWriter, r *http.Request) {
	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := assetrule.RulePreviewInput{
		RuleType:        enrichment.RuleType(req.RuleType),
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
	}

	result, err := h.assetRuleService.PreviewRule(r.Context(), input, req.Limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to preview asset rule")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// RuleAssetsResponse is a page of a rule's matched assets. asset_ids is kept for older clients; assets carries the same page with names.
type RuleAssetsResponse struct {
	AssetIDs []string              `json:"asset_ids"`
	Assets   []assetrule.RuleAsset `json:"assets"`
	Total    int                   `json:"total"`
} // @name AssetRuleAssetsResponse

// @Summary Get assets matched by a rule
// @Description Page through the assets an asset rule applies to, by name, with each asset's name, MRN, type and providers. q narrows the page to assets whose name or MRN contains it.
// @Tags asset-rules
// @Produce json
// @Param id path string true "Asset rule ID"
// @Param q query string false "Match the asset name or MRN"
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} RuleAssetsResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetRulesAssetsID
// @Router /api/v1/asset-rules/assets/{id} [get]
func (h *Handler) getAssets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset rule ID required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	assets, total, err := h.assetRuleService.SearchRuleAssets(r.Context(), id, r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		if errors.Is(err, assetrule.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset rule not found")
		} else {
			log.Error().Err(err).Str("id", id).Msg("Failed to get asset rule assets")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	assetIDs := make([]string, len(assets))
	for i, a := range assets {
		assetIDs[i] = a.ID
	}

	common.RespondJSON(w, http.StatusOK, RuleAssetsResponse{
		AssetIDs: assetIDs,
		Assets:   assets,
		Total:    total,
	})
}

func extractIDFromPath(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	if idx := strings.Index(id, "/"); idx != -1 {
		id = id[:idx]
	}
	return id
}
