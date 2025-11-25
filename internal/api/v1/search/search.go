package search

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/rs/zerolog/log"
)

// @Summary Unified search
// @Description Search across assets, glossary terms, teams, and users
// @Tags search
// @Produce json
// @Param q query string true "Search query"
// @Param types query []string false "Filter by result types (asset, glossary, team, user)"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} search.Response
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /search [get]
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	query := strings.TrimSpace(queryValues.Get("q"))

	if len(query) > 256 {
		common.RespondError(w, http.StatusBadRequest, "Search query must be 256 characters or less")
		return
	}

	// Parse type filters
	var types []search.ResultType
	if typeParams := queryValues["types[]"]; len(typeParams) > 0 {
		for _, t := range typeParams {
			types = append(types, search.ResultType(t))
		}
	} else if typeParam := queryValues.Get("types"); typeParam != "" {
		for _, t := range strings.Split(typeParam, ",") {
			types = append(types, search.ResultType(strings.TrimSpace(t)))
		}
	}

	// Parse limit and offset
	limit := 20
	offset := 0

	if l := queryValues.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	if o := queryValues.Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	// Parse asset-specific filters
	var assetTypes []string
	if assetTypesParam := queryValues.Get("asset_types"); assetTypesParam != "" {
		for _, t := range strings.Split(assetTypesParam, ",") {
			assetTypes = append(assetTypes, strings.TrimSpace(t))
		}
	}

	var providers []string
	if providersParam := queryValues.Get("providers"); providersParam != "" {
		for _, p := range strings.Split(providersParam, ",") {
			providers = append(providers, strings.TrimSpace(p))
		}
	}

	var tags []string
	if tagsParam := queryValues.Get("tags"); tagsParam != "" {
		for _, tag := range strings.Split(tagsParam, ",") {
			tags = append(tags, strings.TrimSpace(tag))
		}
	}

	filter := search.Filter{
		Query:      query,
		Types:      types,
		AssetTypes: assetTypes,
		Providers:  providers,
		Tags:       tags,
		Limit:      limit,
		Offset:     offset,
	}

	response, err := h.searchService.Search(r.Context(), filter)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("Failed to execute search")
		common.RespondError(w, http.StatusInternalServerError, "Failed to execute search")
		return
	}

	common.RespondJSON(w, http.StatusOK, response)
}
