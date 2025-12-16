package dataproducts

import (
	"encoding/json"
	"errors"
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
}

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
}

type CreateRequest struct {
	Name          string                 `json:"name" validate:"required"`
	Description   *string                `json:"description,omitempty"`
	Documentation *string                `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Owners        []OwnerRequest         `json:"owners,omitempty"`
	Rules         []RuleRequest          `json:"rules,omitempty"`
}

type UpdateRequest struct {
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Documentation *string                `json:"documentation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Owners        []OwnerRequest         `json:"owners,omitempty"`
}

type AddAssetsRequest struct {
	AssetIDs []string `json:"asset_ids" validate:"required,min=1"`
}

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
		Name:          req.Name,
		Description:   req.Description,
		Documentation: req.Documentation,
		Metadata:      req.Metadata,
		Tags:          req.Tags,
		Owners:        owners,
		Rules:         rules,
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
		Name:          req.Name,
		Description:   req.Description,
		Documentation: req.Documentation,
		Metadata:      req.Metadata,
		Tags:          req.Tags,
		Owners:        owners,
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

func (h *Handler) removeAsset(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/products/"), "/")
	if len(parts) < 3 {
		common.RespondError(w, http.StatusBadRequest, "Data product ID and asset ID required")
		return
	}

	dataProductID := parts[0]
	assetID := parts[2]

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

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

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
