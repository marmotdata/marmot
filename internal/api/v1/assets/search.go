package assets

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/rs/zerolog/log"
)

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
		OwnerType: filter.OwnerType,
		OwnerID:   filter.OwnerID,
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
		recorder.RecordSearchQuery(r.Context(), queryType, searchQuery)
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

	var ownerType, ownerID *string
	if ownerTypeStr := query.Get("owner_type"); ownerTypeStr != "" {
		ownerType = &ownerTypeStr
	}
	if ownerIDStr := query.Get("owner_id"); ownerIDStr != "" {
		ownerID = &ownerIDStr
	}

	return asset.Filter{
		Limit:        limit,
		Offset:       offset,
		Types:        types,
		Providers:    providers,
		Tags:         tags,
		UpdatedAfter: updatedAfter,
		OwnerType:    ownerType,
		OwnerID:      ownerID,
	}, nil
}

// @Summary Lookup asset by type, service, and name
// @Description Get an asset by its type, service (provider), and name
// @Tags assets
// @Produce json
// @Param type path string true "Asset type"
// @Param service path string true "Service/Provider name"
// @Param name path string true "Asset name"
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/lookup/{type}/{service}/{name} [get]
func (h *Handler) lookupAsset(w http.ResponseWriter, r *http.Request) {
	pathPart := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/lookup/")
	if pathPart == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset type is required")
		return
	}

	parts := strings.SplitN(pathPart, "/", 3)

	if len(parts) != 3 {
		common.RespondError(w, http.StatusBadRequest, "Invalid path format. Expected /assets/lookup/{type}/{service}/{name}")
		return
	}

	assetType := strings.ToUpper(parts[0])
	assetService := parts[1]
	assetName := parts[2]

	mrnStr := mrn.New(assetType, assetService, assetName)
	result, err := h.assetService.GetByMRN(r.Context(), mrnStr)
	if err != nil {
		switch err {
		case asset.ErrAssetNotFound:
			http.NotFound(w, r)
		default:
			log.Error().
				Err(err).
				Str("endpoint", r.URL.Path).
				Str("method", r.Method).
				Str("mrn", mrnStr).
				Msg("Failed to lookup asset")

			common.RespondError(w, http.StatusInternalServerError, "Failed to lookup asset")
		}
		return
	}

	h.metricsService.GetRecorder().RecordAssetView(r.Context(), result.ID, result.Type, *result.Name, result.Providers[0])

	common.RespondJSON(w, http.StatusOK, h.enrichAssetResponse(r, result))
}
