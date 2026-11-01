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
// @Param asset_types query string false "Comma-separated asset types to filter assets by"
// @Param providers query string false "Comma-separated providers to filter assets by"
// @Param tags query string false "Comma-separated tags to filter by"
// @Param owner query string false "Comma-separated user or team IDs; matches assets, glossary terms and data products they own"
// @Param sort query string false "Result order" Enums(relevance, popular, updated, name) default(relevance)
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} search.Response
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getSearch
// @Router /api/v1/search [get]
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

	var owners []string
	for _, param := range append(queryValues["owner"], queryValues["owners"]...) {
		for _, o := range strings.Split(param, ",") {
			if o = strings.TrimSpace(o); o != "" {
				owners = append(owners, o)
			}
		}
	}

	sort, ok := search.ParseSort(strings.TrimSpace(queryValues.Get("sort")))
	if !ok {
		common.RespondError(w, http.StatusBadRequest, "sort must be one of relevance, popular, updated or name")
		return
	}

	filter := search.Filter{
		Query:      query,
		Types:      types,
		AssetTypes: assetTypes,
		Providers:  providers,
		Tags:       tags,
		Owners:     owners,
		Sort:       sort,
		Limit:      limit,
		Offset:     offset,
	}

	response, err := h.searchService.Search(r.Context(), filter)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("Failed to execute search")
		common.RespondError(w, http.StatusInternalServerError, "Failed to execute search")
		return
	}

	// Searches that found nothing are recorded too, apart from the rest, for the zero-result list in Insights
	if query != "" {
		queryType := "full_text"
		if len(filter.Types) > 0 || len(filter.AssetTypes) > 0 || len(filter.Providers) > 0 || len(filter.Tags) > 0 || len(filter.Owners) > 0 {
			queryType = "filtered"
		}
		h.metricsService.RecordSearch(queryType, query, int64(response.Total))
	}

	common.RespondJSON(w, http.StatusOK, response)
}
