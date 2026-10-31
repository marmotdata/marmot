package dataproducts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/dataproduct"
	"github.com/rs/zerolog/log"
)

type ReorderRulesRequest struct {
	RuleIDs []string `json:"rule_ids" validate:"required"`
} // @name ReorderDataProductRulesRequest

// @Summary Data product filter values
// @Description List the owners and tags in use on data products, for filtering the list
// @Tags products
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} dataproduct.Facets
// @Failure 500 {object} common.ErrorResponse
// @ID getProductsFacets
// @Router /api/v1/products/facets [get]
func (h *Handler) facets(w http.ResponseWriter, r *http.Request) {
	facets, err := h.dataProductService.GetFacets(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list data product facets")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	common.RespondJSON(w, http.StatusOK, facets)
}

// @Summary Get data product members
// @Description Page through a data product's assets with each asset's name, type, owners and popularity and how it joined: manually or through which rule
// @Tags products
// @Produce json
// @Param id path string true "Data Product ID"
// @Param source query string false "manual or rule"
// @Param rule_id query string false "Only assets this rule brought in"
// @Param q query string false "Match the asset name or MRN"
// @Param type query string false "Only assets of this type"
// @Param sort query string false "name (default) or popular"
// @Param limit query int false "Maximum number of assets to return" default(50)
// @Param offset query int false "Number of assets to skip" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} dataproduct.MembersResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getProductsMembersID
// @Router /api/v1/products/members/{id} [get]
func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	params := r.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))

	result, err := h.dataProductService.GetMembers(r.Context(), id, dataproduct.MemberFilter{
		Source: params.Get("source"),
		RuleID: params.Get("rule_id"),
		Query:  params.Get("q"),
		Type:   params.Get("type"),
		Sort:   params.Get("sort"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get data product members")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Reorder data product rules
// @Description Set the order of a data product's rules. rule_ids must name every rule of the product once.
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Data Product ID"
// @Param order body ReorderRulesRequest true "Rule ids in their new order"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} RulesResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putProductsRuleOrderID
// @Router /api/v1/products/rule-order/{id} [put]
func (h *Handler) reorderRules(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ReorderRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	rules, err := h.dataProductService.ReorderRules(r.Context(), id, req.RuleIDs)
	if err != nil {
		switch {
		case errors.Is(err, dataproduct.ErrNotFound):
			common.RespondError(w, http.StatusNotFound, "Data product not found")
		case errors.Is(err, dataproduct.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to reorder data product rules")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}
	common.RespondJSON(w, http.StatusOK, RulesResponse{Rules: rules, Total: len(rules)})
}
