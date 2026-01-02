package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/rs/zerolog/log"
)

type GetMetricsRequest struct {
	Start           string            `json:"start"`
	End             string            `json:"end"`
	MetricNames     []string          `json:"metric_names"`
	Labels          map[string]string `json:"labels"`
	AggregationType string            `json:"aggregation"`
	BucketSize      string            `json:"bucket_size"`
}

type GetMetricsResponse struct {
	Metrics []metrics.AggregatedMetric `json:"metrics"`
	Query   GetMetricsRequest          `json:"query"`
}

// @Summary Get metrics for UI
// @Description Get aggregated metrics for dashboard display
// @Tags metrics
// @Accept json
// @Produce json
// @Param start query string true "Start time (ISO 8601)"
// @Param end query string true "End time (ISO 8601)"
// @Param metric_names query []string false "Filter by metric names"
// @Param aggregation query string false "Aggregation type" Enums(avg,sum,max,min) default(avg)
// @Param bucket_size query string false "Time bucket size" Enums(1m,5m,1h,1d)
// @Success 200 {object} GetMetricsResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /api/v1/metrics [get]
func (h *Handler) getMetrics(w http.ResponseWriter, r *http.Request) {
	req := GetMetricsRequest{
		Start:           r.URL.Query().Get("start"),
		End:             r.URL.Query().Get("end"),
		MetricNames:     r.URL.Query()["metric_names"],
		AggregationType: r.URL.Query().Get("aggregation"),
		BucketSize:      r.URL.Query().Get("bucket_size"),
		Labels:          make(map[string]string),
	}

	for key, values := range r.URL.Query() {
		if len(values) > 0 && len(key) > 6 && key[:6] == "label." {
			labelKey := key[6:]
			req.Labels[labelKey] = values[0]
		}
	}

	if req.Start == "" || req.End == "" {
		common.RespondError(w, http.StatusBadRequest, "start and end times are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid start time format, use RFC3339")
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid end time format, use RFC3339")
		return
	}

	if endTime.Before(startTime) {
		common.RespondError(w, http.StatusBadRequest, "end time must be after start time")
		return
	}

	if endTime.Sub(startTime) > 30*24*time.Hour {
		common.RespondError(w, http.StatusBadRequest, "time range cannot exceed 30 days")
		return
	}

	var bucketSize time.Duration
	if req.BucketSize != "" {
		bucketSize, err = parseBucketSize(req.BucketSize)
		if err != nil {
			common.RespondError(w, http.StatusBadRequest, "invalid bucket_size: "+err.Error())
			return
		}
	}

	if req.AggregationType == "" {
		req.AggregationType = "avg"
	}

	opts := metrics.QueryOptions{
		TimeRange: metrics.TimeRange{
			Start: startTime,
			End:   endTime,
		},
		MetricNames:     req.MetricNames,
		Labels:          req.Labels,
		AggregationType: req.AggregationType,
		BucketSize:      bucketSize,
	}

	metricsData, err := h.metricsService.GetMetrics(r.Context(), opts)
	if err != nil {
		log.Error().Err(err).Interface("options", opts).Msg("Failed to get metrics")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve metrics")
		return
	}

	response := GetMetricsResponse{
		Metrics: metricsData,
		Query:   req,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

func parseBucketSize(s string) (time.Duration, error) {
	switch s {
	case "1m":
		return time.Minute, nil
	case "5m":
		return 5 * time.Minute, nil
	case "15m":
		return 15 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return time.ParseDuration(s)
	}
}

// @Summary Get top search queries
// @Description Get the most popular search queries
// @Tags metrics
// @Produce json
// @Param start query string true "Start time (ISO 8601)"
// @Param end query string true "End time (ISO 8601)"
// @Param limit query int false "Number of results" default(10)
// @Success 200 {object} []metrics.QueryCount
// @Router /api/v1/metrics/top-queries [get]
func (h *Handler) getTopQueries(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limitStr := r.URL.Query().Get("limit")

	if start == "" || end == "" {
		common.RespondError(w, http.StatusBadRequest, "start and end times are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid start time format")
		return
	}

	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid end time format")
		return
	}

	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	results, err := h.metricsService.GetTopQueries(r.Context(), metrics.TimeRange{
		Start: startTime,
		End:   endTime,
	}, limit)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get top queries")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve top queries")
		return
	}

	common.RespondJSON(w, http.StatusOK, results)
}

// @Summary Get top viewed assets
// @Description Get the most viewed assets
// @Tags metrics
// @Produce json
// @Param start query string true "Start time (ISO 8601)"
// @Param end query string true "End time (ISO 8601)"
// @Param limit query int false "Number of results" default(10)
// @Success 200 {object} []metrics.AssetCount
// @Router /api/v1/metrics/top-assets [get]
func (h *Handler) getTopAssets(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	limitStr := r.URL.Query().Get("limit")

	if start == "" || end == "" {
		common.RespondError(w, http.StatusBadRequest, "start and end times are required")
		return
	}

	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid start time format")
		return
	}

	endTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid end time format")
		return
	}

	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	results, err := h.metricsService.GetTopAssets(r.Context(), metrics.TimeRange{
		Start: startTime,
		End:   endTime,
	}, limit)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get top assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve top assets")
		return
	}

	common.RespondJSON(w, http.StatusOK, results)
}

type TotalAssetsResponse struct {
	Count int64 `json:"count"`
}

// @Summary Get total assets count
// @Description Get the total number of assets
// @Tags metrics
// @Produce json
// @Success 200 {object} TotalAssetsResponse
// @Router /api/v1/metrics/assets/total [get]
func (h *Handler) getTotalAssets(w http.ResponseWriter, r *http.Request) {
	count, err := h.metricsService.GetTotalAssets(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve total assets")
		return
	}

	common.RespondJSON(w, http.StatusOK, TotalAssetsResponse{Count: count})
}

type AssetsByTypeResponse struct {
	Assets map[string]int64 `json:"assets"`
}

// @Summary Get assets by type
// @Description Get asset counts grouped by type
// @Tags metrics
// @Produce json
// @Success 200 {object} AssetsByTypeResponse
// @Router /api/v1/metrics/assets/by-type [get]
func (h *Handler) getAssetsByType(w http.ResponseWriter, r *http.Request) {
	assets, err := h.metricsService.GetAssetsByType(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get assets by type")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve assets by type")
		return
	}

	common.RespondJSON(w, http.StatusOK, AssetsByTypeResponse{Assets: assets})
}

type AssetsByProviderResponse struct {
	Assets map[string]int64 `json:"assets"`
}

// @Summary Get assets by provider
// @Description Get asset counts grouped by provider
// @Tags metrics
// @Produce json
// @Success 200 {object} AssetsByProviderResponse
// @Router /api/v1/metrics/assets/by-provider [get]
func (h *Handler) getAssetsByProvider(w http.ResponseWriter, r *http.Request) {
	assets, err := h.metricsService.GetAssetsByProvider(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get assets by provider")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve assets by provider")
		return
	}

	common.RespondJSON(w, http.StatusOK, AssetsByProviderResponse{Assets: assets})
}

type AssetsWithSchemasResponse struct {
	Count      int64   `json:"count"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
}

// @Summary Get assets with schemas count
// @Description Get the count of assets that have schemas defined
// @Tags metrics
// @Produce json
// @Success 200 {object} AssetsWithSchemasResponse
// @Router /api/v1/metrics/assets/with-schemas [get]
func (h *Handler) getAssetsWithSchemas(w http.ResponseWriter, r *http.Request) {
	schemasCount, err := h.metricsService.GetAssetsWithSchemas(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get assets with schemas")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve assets with schemas")
		return
	}

	totalCount, err := h.metricsService.GetTotalAssetsFiltered(r.Context(),
		h.config.Metrics.Schemas.ExcludedAssetTypes,
		h.config.Metrics.Schemas.ExcludedProviders)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total assets")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve total assets")
		return
	}

	var percentage float64
	if totalCount > 0 {
		percentage = (float64(schemasCount) / float64(totalCount)) * 100
	}

	common.RespondJSON(w, http.StatusOK, AssetsWithSchemasResponse{
		Count:      schemasCount,
		Total:      totalCount,
		Percentage: percentage,
	})
}

type AssetsByOwnerResponse struct {
	Assets map[string]int64 `json:"assets"`
}

// @Summary Get assets by owner
// @Description Get asset counts grouped by owner
// @Tags metrics
// @Produce json
// @Success 200 {object} AssetsByOwnerResponse
// @Router /api/v1/metrics/assets/by-owner [get]
func (h *Handler) getAssetsByOwner(w http.ResponseWriter, r *http.Request) {
	assets, err := h.metricsService.GetAssetsByOwner(r.Context(), h.config.Metrics.OwnerMetadataFields)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get assets by owner")
		common.RespondError(w, http.StatusInternalServerError, "Failed to retrieve assets by owner")
		return
	}

	common.RespondJSON(w, http.StatusOK, AssetsByOwnerResponse{Assets: assets})
}
