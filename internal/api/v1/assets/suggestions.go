package assets

import (
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/rs/zerolog/log"
)

// @Summary Get metadata field suggestions
// @Description Get suggestions for metadata fields and their types
// @Tags assets
// @Produce json
// @Success 200 {array} asset.MetadataFieldSuggestion
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/suggestions/metadata/fields [get]
func (h *Handler) getMetadataFieldSuggestions(w http.ResponseWriter, r *http.Request) {
	var queryContext *asset.MetadataContext
	if contextQuery := r.URL.Query().Get("context"); contextQuery != "" {
		queryContext = &asset.MetadataContext{
			Query: contextQuery,
		}
	}

	suggestions, err := h.assetService.GetMetadataFields(r.Context(), queryContext)
	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", r.URL.Path).
			Str("method", r.Method).
			Msg("Failed to get metadata field suggestions")

		common.RespondError(w, http.StatusInternalServerError, "Failed to get suggestions")
		return
	}

	common.RespondJSON(w, http.StatusOK, suggestions)
}

// @Summary Get metadata value suggestions
// @Description Get suggestions for values of a specific metadata field
// @Tags assets
// @Produce json
// @Param field query string true "Metadata field name"
// @Param prefix query string false "Value prefix to filter by"
// @Param limit query int false "Maximum number of suggestions" default(10)
// @Success 200 {array} asset.MetadataValueSuggestion
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/suggestions/metadata/values [get]
func (h *Handler) getMetadataValueSuggestions(w http.ResponseWriter, r *http.Request) {
	field := r.URL.Query().Get("field")
	if field == "" {
		common.RespondError(w, http.StatusBadRequest, "field parameter is required")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	limit := 10

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var queryContext *asset.MetadataContext
	if contextQuery := r.URL.Query().Get("context"); contextQuery != "" {
		queryContext = &asset.MetadataContext{
			Query: contextQuery,
		}
	}

	suggestions, err := h.assetService.GetMetadataValues(r.Context(), field, prefix, limit, queryContext)
	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", r.URL.Path).
			Str("method", r.Method).
			Msg("Failed to get metadata value suggestions")

		common.RespondError(w, http.StatusInternalServerError, "Failed to get suggestions")
		return
	}

	common.RespondJSON(w, http.StatusOK, suggestions)
}
