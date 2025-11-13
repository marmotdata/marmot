package assets

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

type ListResponse struct {
	Assets  []*asset.Asset         `json:"assets"`
	Total   int                    `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
	Filters asset.AvailableFilters `json:"filters"`
}

type SearchFilter struct {
	Query     string   `json:"query" validate:"omitempty"`
	Types     []string `json:"types" validate:"omitempty"`
	Providers []string `json:"services" validate:"omitempty"`
	Tags      []string `json:"tags" validate:"omitempty"`
}

type SearchResponse struct {
	Assets  []*asset.Asset         `json:"assets"`
	Total   int                    `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
	Filters asset.AvailableFilters `json:"filters"`
}

// @Summary List assets with pagination
// @Description Get a paginated list of assets
// @Tags assets
// @Produce json
// @Param offset query int false "Offset for pagination"
// @Param limit query int false "Limit for pagination"
// @Success 200 {object} asset.ListResult
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/list [get]
func (h *Handler) listAssets(w http.ResponseWriter, r *http.Request) {
	offset := 0
	limit := 100

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	result, err := h.assetService.List(r.Context(), offset, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list assets")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Search assets
// @Description Search for assets using query string and filters
// @Tags assets
// @Accept json
// @Produce json
// @Param q query string false "Search query"
// @Param types query []string false "Filter by asset types"
// @Param services query []string false "Filter by services"
// @Param tags query []string false "Filter by tags"
// @Param limit query int false "Number of items to return" default(50)
// @Param offset query int false "Number of items to skip" default(0)
// @Param calculateCounts query bool false "Calculate filter counts" default(false)
// @Success 200 {object} SearchResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/search [get]
func (h *Handler) searchAssets(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()
	searchQuery := queryValues.Get("q")

	if len(searchQuery) > 256 {
		common.RespondError(w, http.StatusBadRequest, "Search query must be 256 characters or less")
		return
	}

	filter, err := parseFilter(r)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid filter parameters")
		return
	}

	searchFilter := asset.SearchFilter{
		Query:     searchQuery,
		Types:     filter.Types,
		Providers: filter.Providers,
		Tags:      filter.Tags,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}

	calculateCounts := queryValues.Get("calculateCounts") == "true"

	results, total, availableFilters, err := h.assetService.Search(r.Context(), searchFilter, calculateCounts)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, "Invalid search query")
		default:
			log.Error().Err(err).Str("query", searchQuery).Msg("Search failed")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	if searchQuery != "" && total > 0 {
		recorder := h.metricsService.GetRecorder()
		queryType := "full_text"
		if len(filter.Types) > 0 || len(filter.Providers) > 0 || len(filter.Tags) > 0 {
			queryType = "filtered"
		}
		err = recorder.RecordSearchQuery(r.Context(), queryType, searchQuery)
		if err != nil {
			log.Error().Err(err)
		}
	}

	response := SearchResponse{
		Assets:  results,
		Total:   total,
		Limit:   searchFilter.Limit,
		Offset:  searchFilter.Offset,
		Filters: availableFilters,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Match asset pattern
// @Description Find assets matching a pattern
// @Tags assets
// @Produce json
// @Param pattern query string true "Asset pattern to match"
// @Param type query string true "Asset type"
// @Success 200 {array} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/match-pattern [get]
func (h *Handler) matchAssetPattern(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		common.RespondError(w, http.StatusBadRequest, "pattern parameter is required")
		return
	}

	assetType := r.URL.Query().Get("type")
	if assetType == "" {
		common.RespondError(w, http.StatusBadRequest, "type parameter is required")
		return
	}

	result, err := h.assetService.ListByPattern(r.Context(), pattern, assetType)
	if err != nil {
		log.Error().Err(err).
			Str("pattern", pattern).
			Str("type", assetType).
			Msg("Failed to match pattern")
		common.RespondError(w, http.StatusInternalServerError, "Failed to match pattern")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

func parseFilter(r *http.Request) (asset.Filter, error) {
	query := r.URL.Query()

	limit := common.ParseLimit(query.Get("limit"), 50, 1000)
	offset := common.ParseOffset(query.Get("offset"))

	var types, providers, tags []string
	if typesStr := query.Get("types"); typesStr != "" {
		types = strings.Split(typesStr, ",")
	}
	if providersStr := query.Get("providers"); providersStr != "" {
		providers = strings.Split(providersStr, ",")
	}
	if tagsStr := query.Get("tags"); tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}

	var updatedAfter *time.Time
	if updatedAfterStr := query.Get("updatedAfter"); updatedAfterStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, updatedAfterStr)
		if err != nil {
			return asset.Filter{}, err
		}
		updatedAfter = &parsedTime
	}

	return asset.Filter{
		Limit:        limit,
		Offset:       offset,
		Types:        types,
		Providers:    providers,
		Tags:         tags,
		UpdatedAfter: updatedAfter,
	}, nil
}

// @Summary Lookup asset by type and name
// @Description Get an asset using its type and name
// @Tags assets
// @Produce json
// @Param type path string true "Asset type"
// @Param name path string true "Asset name"
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/lookup/{type}/{name} [get]
func (h *Handler) lookupAsset(w http.ResponseWriter, r *http.Request) {
	assetType := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/api/v1/assets/lookup/"))
	if assetType == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset type is required")
		return
	}

	// Split the remaining path to get name
	parts := strings.SplitN(assetType, "/", 2)
	if len(parts) != 2 {
		common.RespondError(w, http.StatusBadRequest, "Asset name is required")
		return
	}
	assetType = parts[0]
	assetName := parts[1]

	result, err := h.assetService.GetByTypeAndName(r.Context(), assetType, assetName)
	if err != nil {
		switch err {
		case asset.ErrAssetNotFound:
			http.NotFound(w, r)
		default:
			log.Error().
				Err(err).
				Str("endpoint", r.URL.Path).
				Str("method", r.Method).
				Str("assetType", assetType).
				Str("assetName", assetName).
				Msg("Failed to lookup asset")

			common.RespondError(w, http.StatusInternalServerError, "Failed to lookup asset")
		}
		return
	}

	err = h.metricsService.GetRecorder().RecordAssetView(r.Context(), result.ID, result.Type, *result.Name, result.Providers[0])
	if err != nil {
		log.Error().Err(err)
	}

	common.RespondJSON(w, http.StatusOK, result)
}
