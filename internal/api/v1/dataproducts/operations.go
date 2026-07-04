package dataproducts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

type OwnerRequest struct {
	ID   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required,oneof=user team"`
} // @name DataProductOwnerRequest

type RuleRequest struct {
	Name            string  `json:"name" validate:"required"`
	Description     *string `json:"description,omitempty"`
	RuleType        string  `json:"rule_type" validate:"required"`
	QueryExpression *string `json:"query_expression,omitempty"`
	MetadataField   *string `json:"metadata_field,omitempty"`
	PatternType     *string `json:"pattern_type,omitempty"`
	PatternValue    *string `json:"pattern_value,omitempty"`
	Priority        int     `json:"priority"`
	IsEnabled       bool    `json:"is_enabled"`
} // @name DataProductRuleRequest

type CreateRequest struct {
	Name        string                 `json:"name" validate:"required"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Owners      []OwnerRequest         `json:"owners,omitempty"`
	Rules       []RuleRequest          `json:"rules,omitempty"`
} // @name CreateDataProductRequest

type UpdateRequest struct {
	Name        *string                `json:"name,omitempty"`
	Description *string                `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Owners      []OwnerRequest         `json:"owners,omitempty"`
} // @name UpdateDataProductRequest

type AddAssetsRequest struct {
	AssetIDs []string `json:"asset_ids" validate:"required,min=1"`
} // @name AddDataProductAssetsRequest

type RulesResponse struct {
	Rules []dataproduct.Rule `json:"rules"`
	Total int                `json:"total"`
} // @name DataProductRulesResponse

// @Summary Create data product
// @Description Create a new data product with owners and optional membership rules
// @Tags products
// @Accept json
// @Produce json
// @Param product body CreateRequest true "Data product to create"
// @Success 201 {object} dataproduct.DataProduct
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/ [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	owners := make([]dataproduct.OwnerInput, len(req.Owners))
	for i, owner := range req.Owners {
		owners[i] = dataproduct.OwnerInput{
			ID:   owner.ID,
			Type: owner.Type,
		}
	}

	if len(owners) == 0 {
		owners = []dataproduct.OwnerInput{{ID: usr.ID, Type: "user"}}
	}

	rules := make([]dataproduct.RuleInput, len(req.Rules))
	for i, rule := range req.Rules {
		rules[i] = dataproduct.RuleInput{
			Name:            rule.Name,
			Description:     rule.Description,
			RuleType:        dataproduct.RuleType(rule.RuleType),
			QueryExpression: rule.QueryExpression,
			MetadataField:   rule.MetadataField,
			PatternType:     rule.PatternType,
			PatternValue:    rule.PatternValue,
			Priority:        rule.Priority,
			IsEnabled:       rule.IsEnabled,
		}
	}

	input := dataproduct.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
		Tags:        req.Tags,
		Owners:      owners,
		Rules:       rules,
	}

	dp, err := h.dataProductService.Create(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, dataproduct.ErrConflict):
			log.Error().Err(err).Str("name", req.Name).Msg("Data product already exists")
			common.RespondError(w, http.StatusConflict, "Data product with this name already exists")
		default:
			log.Error().Err(err).Interface("input", input).Msg("Failed to create data product")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, dp)
}

// @Summary Get data product
// @Description Get a data product by ID
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Success 200 {object} dataproduct.DataProduct
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/{id} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	dp, err := h.dataProductService.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get data product")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, dp)
}

// @Summary Update data product
// @Description Update an existing data product
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Data Product ID"
// @Param product body UpdateRequest true "Fields to update"
// @Success 200 {object} dataproduct.DataProduct
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/{id} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var owners []dataproduct.OwnerInput
	if req.Owners != nil {
		owners = make([]dataproduct.OwnerInput, len(req.Owners))
		for i, owner := range req.Owners {
			owners[i] = dataproduct.OwnerInput{
				ID:   owner.ID,
				Type: owner.Type,
			}
		}
	}

	input := dataproduct.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
		Tags:        req.Tags,
		Owners:      owners,
	}

	dp, err := h.dataProductService.Update(r.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrConflict):
			common.RespondError(w, http.StatusConflict, "Data product with this name already exists")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update data product")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, dp)
}

// @Summary Delete data product
// @Description Delete a data product by ID
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/{id} [delete]
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	err := h.dataProductService.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to delete data product")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Data product deleted successfully"})
}

// @Summary List data products
// @Description Retrieve a paginated list of data products
// @Tags products
// @Produce json
// @Param limit query int false "Maximum number of data products to return" default(20)
// @Param offset query int false "Number of data products to skip" default(0)
// @Success 200 {object} dataproduct.ListResult
// @Failure 500 {object} common.ErrorResponse
// @Router /products/list [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.dataProductService.List(r.Context(), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list data products")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Search data products
// @Description Search data products by name, description, and tags
// @Tags products
// @Produce json
// @Param q query string false "Search query"
// @Param tags query string false "Comma-separated list of tags to filter by"
// @Param limit query int false "Maximum number of data products to return" default(20)
// @Param offset query int false "Number of data products to skip" default(0)
// @Success 200 {object} dataproduct.ListResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/search [get]
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var tags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
	}

	filter := dataproduct.SearchFilter{
		Query:  query,
		Tags:   tags,
		Limit:  limit,
		Offset: offset,
	}

	result, err := h.dataProductService.Search(r.Context(), filter)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Interface("filter", filter).Msg("Failed to search data products")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Get data product assets
// @Description Get the manually added assets of a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param limit query int false "Maximum number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dataproduct.AssetsResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/assets/{id} [get]
func (h *Handler) getAssets(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.dataProductService.GetManualAssets(r.Context(), id, limit, offset)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get manual assets")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Add data product assets
// @Description Manually add assets to a data product
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Data Product ID"
// @Param assets body AddAssetsRequest true "Asset IDs to add"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/assets/{id} [post]
func (h *Handler) addAssets(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	var req AddAssetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	err := h.dataProductService.AddAssets(r.Context(), id, req.AssetIDs, usr.ID)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to add assets")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Assets added successfully"})
}

// @Summary Remove data product asset
// @Description Remove a manually added asset from a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param assetId path string true "Asset ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/assets/{id}/{assetId} [delete]
func (h *Handler) removeAsset(w http.ResponseWriter, r *http.Request) {
	// URL format: /api/v1/products/assets/{productId}/{assetId}
	// After trimming prefix: assets/{productId}/{assetId}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/assets/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and asset ID required")
		return
	}

	dataProductID := parts[0]
	assetID := parts[1]

	err := h.dataProductService.RemoveAsset(r.Context(), dataProductID, assetID)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found in data product")
		default:
			log.Error().Err(err).Str("dataProductId", dataProductID).Str("assetId", assetID).Msg("Failed to remove asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Asset removed successfully"})
}

// @Summary Get data product rules
// @Description Get the membership rules of a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Success 200 {object} RulesResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/rules/{id} [get]
func (h *Handler) getRules(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/rules/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	rules, err := h.dataProductService.GetRules(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get rules")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, RulesResponse{
		Rules: rules,
		Total: len(rules),
	})
}

// @Summary Create data product rule
// @Description Create a membership rule for a data product
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Data Product ID"
// @Param rule body RuleRequest true "Rule to create"
// @Success 201 {object} dataproduct.Rule
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/rules/{id} [post]
func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/rules/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := dataproduct.RuleInput{
		Name:            req.Name,
		Description:     req.Description,
		RuleType:        dataproduct.RuleType(req.RuleType),
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
		Priority:        req.Priority,
		IsEnabled:       req.IsEnabled,
	}

	rule, err := h.dataProductService.CreateRule(r.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to create rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, rule)
}

// @Summary Update data product rule
// @Description Update a membership rule of a data product
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Data Product ID"
// @Param ruleId path string true "Rule ID"
// @Param rule body RuleRequest true "Rule fields to update"
// @Success 200 {object} dataproduct.Rule
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/rules/{id}/{ruleId} [put]
func (h *Handler) updateRule(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/"), "/")
	if len(parts) < 3 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and rule ID required")
		return
	}

	ruleID := parts[2]

	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := dataproduct.RuleInput{
		Name:            req.Name,
		Description:     req.Description,
		RuleType:        dataproduct.RuleType(req.RuleType),
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
		Priority:        req.Priority,
		IsEnabled:       req.IsEnabled,
	}

	rule, err := h.dataProductService.UpdateRule(r.Context(), ruleID, input)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrRuleNotFound):
			common.RespondError(w, http.StatusNotFound, "Rule not found")
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("ruleId", ruleID).Msg("Failed to update rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, rule)
}

// @Summary Delete data product rule
// @Description Delete a membership rule from a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param ruleId path string true "Rule ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/rules/{id}/{ruleId} [delete]
func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/"), "/")
	if len(parts) < 3 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and rule ID required")
		return
	}

	ruleID := parts[2]

	err := h.dataProductService.DeleteRule(r.Context(), ruleID)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrRuleNotFound):
			common.RespondError(w, http.StatusNotFound, "Rule not found")
		default:
			log.Error().Err(err).Str("ruleId", ruleID).Msg("Failed to delete rule")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Rule deleted successfully"})
}

// @Summary Preview data product rule
// @Description Preview which assets would match a rule configuration
// @Tags products
// @Accept json
// @Produce json
// @Param rule body RuleRequest true "Rule to preview"
// @Param limit query int false "Maximum number of matching assets to return" default(20)
// @Success 200 {object} dataproduct.RulePreview
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/rule-preview [post]
func (h *Handler) previewRule(w http.ResponseWriter, r *http.Request) {
	var req RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	input := dataproduct.RuleInput{
		Name:            req.Name,
		Description:     req.Description,
		RuleType:        dataproduct.RuleType(req.RuleType),
		QueryExpression: req.QueryExpression,
		MetadataField:   req.MetadataField,
		PatternType:     req.PatternType,
		PatternValue:    req.PatternValue,
		Priority:        req.Priority,
		IsEnabled:       true,
	}

	preview, err := h.dataProductService.PreviewRule(r.Context(), input, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to preview rule")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	common.RespondJSON(w, http.StatusOK, preview)
}

// @Summary Get resolved data product assets
// @Description Get all assets of a data product, both manually added and matched by rules
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param limit query int false "Maximum number of assets to return" default(20)
// @Param offset query int false "Number of assets to skip" default(0)
// @Success 200 {object} dataproduct.ResolvedAssets
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/resolved-assets/{id} [get]
func (h *Handler) getResolvedAssets(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/v1/products/resolved-assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.dataProductService.GetResolvedAssets(r.Context(), id, limit, offset)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get resolved assets")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

func extractIDFromPath(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	return id
}

// Image handlers

// @Summary Upload product image
// @Description Upload an icon or header image for a data product
// @Tags products
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Data Product ID"
// @Param purpose path string true "Image purpose (icon or header)"
// @Param file formData file true "Image file"
// @Success 200 {object} dataproduct.ProductImageMeta
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/images/{id}/{purpose} [post]
func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/v1/products/images/{id}/{purpose}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/images/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and purpose required")
		return
	}

	productID := parts[0]
	purpose := dataproduct.ImagePurpose(parts[1])

	// Validate purpose
	if purpose != dataproduct.ImagePurposeIcon && purpose != dataproduct.ImagePurposeHeader {
		common.RespondError(w, http.StatusBadRequest, "Invalid purpose: must be 'icon' or 'header'")
		return
	}

	// Parse multipart form (max 10MB)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil { //nolint:gosec // G120: body size limited by MaxBytesReader above
		common.RespondError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "No file provided")
		return
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read uploaded file")
		common.RespondError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	// Get content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Get user for created_by
	var createdBy *string
	if usr, ok := r.Context().Value(common.UserContextKey).(*user.User); ok {
		createdBy = &usr.ID
	}

	input := dataproduct.UploadImageInput{
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        data,
	}

	meta, err := h.dataProductService.UploadImage(r.Context(), productID, purpose, input, createdBy)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrInvalidImageType):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, dataproduct.ErrImageTooLarge):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("productId", productID).Str("purpose", string(purpose)).Msg("Failed to upload image")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, meta)
}

// @Summary Get product image
// @Description Get an icon or header image for a data product
// @Tags products
// @Produce image/jpeg,image/png,image/gif,image/webp
// @Param id path string true "Data Product ID"
// @Param purpose path string true "Image purpose (icon or header)"
// @Success 200 {file} binary
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/images/{id}/{purpose} [get]
func (h *Handler) getImage(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/v1/products/images/{id}/{purpose}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/images/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and purpose required")
		return
	}

	productID := parts[0]
	purpose := dataproduct.ImagePurpose(parts[1])

	image, err := h.dataProductService.GetImageByPurpose(r.Context(), productID, purpose)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrImageNotFound):
			common.RespondError(w, http.StatusNotFound, "Image not found")
		default:
			log.Error().Err(err).Str("productId", productID).Str("purpose", string(purpose)).Msg("Failed to get image")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Generate ETag based on image ID (which changes on each upload due to upsert)
	etag := fmt.Sprintf(`"%s"`, image.ID)

	// Check If-None-Match header for cache validation
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Header().Set("ETag", etag)
	_, _ = w.Write(image.Data) //nolint:gosec // G705: image is re-encoded on upload, served with CSP default-src 'none' and nosniff
}

// @Summary Delete product image
// @Description Delete an icon or header image for a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param purpose path string true "Image purpose (icon or header)"
// @Success 200 {object} map[string]string
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/images/{id}/{purpose} [delete]
func (h *Handler) deleteImage(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/v1/products/images/{id}/{purpose}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/images/"), "/")
	if len(parts) < 2 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and purpose required")
		return
	}

	productID := parts[0]
	purpose := dataproduct.ImagePurpose(parts[1])

	err := h.dataProductService.DeleteImage(r.Context(), productID, purpose)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrImageNotFound):
			common.RespondError(w, http.StatusNotFound, "Image not found")
		default:
			log.Error().Err(err).Str("productId", productID).Str("purpose", string(purpose)).Msg("Failed to delete image")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Image deleted successfully"})
}

// @Summary List product images
// @Description List all images for a data product
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /products/images/{id} [get]
func (h *Handler) listImages(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/v1/products/images/{id}
	productID := strings.TrimPrefix(r.URL.Path, "/api/v1/products/images/")
	productID = strings.TrimSuffix(productID, "/")
	if productID == "" {
		common.RespondError(w, http.StatusBadRequest, "Data product ID required")
		return
	}

	images, err := h.dataProductService.ListImages(r.Context(), productID)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		default:
			log.Error().Err(err).Str("productId", productID).Msg("Failed to list images")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"images": images,
		"total":  len(images),
	})
}
