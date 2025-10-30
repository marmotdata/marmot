package runs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/plugin"
)

type StartRunRequest struct {
	PipelineName string                 `json:"pipeline_name" validate:"required"`
	SourceName   string                 `json:"source_name" validate:"required"`
	Config       plugin.RawPluginConfig `json:"config"`
}

type CompleteRunRequest struct {
	RunID   string             `json:"run_id" validate:"required"`
	Status  plugin.RunStatus   `json:"status" validate:"required"`
	Summary *plugin.RunSummary `json:"summary"`
	Error   string             `json:"error,omitempty"`
}

type BatchCreateRequest struct {
	Assets        []CreateAssetRequest   `json:"assets" validate:"required,min=1"`
	Lineage       []CreateLineageRequest `json:"lineage"`
	Documentation []CreateDocRequest     `json:"documentation"`
	Statistics    []CreateStatRequest    `json:"statistics"`
	Config        plugin.RawPluginConfig `json:"config"`
	PipelineName  string                 `json:"pipeline_name" validate:"required"`
	SourceName    string                 `json:"source_name" validate:"required"`
	RunID         string                 `json:"run_id" validate:"required"`
}

type DestroyRunResponse struct {
	AssetsDeleted        int      `json:"assets_deleted"`
	LineageDeleted       int      `json:"lineage_deleted"`
	DocumentationDeleted int      `json:"documentation_deleted"`
	DeletedEntityMRNs    []string `json:"deleted_entity_mrns"`
}

type CreateStatRequest struct {
	AssetMRN   string  `json:"asset_mrn" validate:"required"`
	MetricName string  `json:"metric_name" validate:"required"`
	Value      float64 `json:"value" validate:"required"`
}

type CreateLineageRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type CreateDocRequest struct {
	AssetMRN string `json:"asset_mrn"`
	Content  string `json:"content"`
	Type     string `json:"type"`
}

type LineageResult struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DocumentationResult struct {
	AssetMRN string `json:"asset_mrn"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

type CreateAssetRequest struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Providers     []string               `json:"providers"`
	Description   *string                `json:"description"`
	Metadata      map[string]interface{} `json:"metadata"`
	Schema        map[string]interface{} `json:"schema"`
	Tags          []string               `json:"tags"`
	Sources       []string               `json:"sources"`
	ExternalLinks []map[string]string    `json:"external_links"`
}

type BatchCreateResponse struct {
	Assets               []BatchAssetResult    `json:"assets"`
	StaleEntitiesRemoved []string              `json:"stale_entities_removed,omitempty"`
	Lineage              []LineageResult       `json:"lineage,omitempty"`
	Documentation        []DocumentationResult `json:"documentation,omitempty"`
}

type BatchAssetResult struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Provider string      `json:"provider"`
	MRN      string      `json:"mrn"`
	Asset    interface{} `json:"asset"`
	Status   string      `json:"status"`
	Error    string      `json:"error,omitempty"`
}

type RunEntitiesResponse struct {
	Entities []*runs.RunEntity `json:"entities"`
	Total    int               `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// @Summary Start run
// @Description Start a new run for tracking
// @Tags runs
// @Accept json
// @Produce json
// @Param request body StartRunRequest true "Start run request"
// @Success 200 {object} plugin.Run
// @Router /runs/start [post]
func (h *Handler) startRun(w http.ResponseWriter, r *http.Request) {
	var req StartRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	run, err := h.runService.StartRun(r.Context(), req.PipelineName, req.SourceName, usr.Name, req.Config)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start run: %v", err))
		return
	}

	common.RespondJSON(w, http.StatusOK, run)
}

// @Summary Complete run
// @Description Complete a run with results
// @Tags runs
// @Accept json
// @Produce json
// @Param request body CompleteRunRequest true "Complete run request"
// @Success 200 {object} map[string]string
// @Router /runs/complete [post]
func (h *Handler) completeRun(w http.ResponseWriter, r *http.Request) {
	var req CompleteRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.runService.CompleteRun(r.Context(), req.RunID, req.Status, req.Summary, req.Error)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to complete run: %v", err))
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// @Summary Batch create assets
// @Description Create/update assets within a run
// @Tags runs
// @Accept json
// @Produce json
// @Param request body BatchCreateRequest true "Batch create request"
// @Success 200 {object} BatchCreateResponse
// @Router /runs/assets/batch [post]
func (h *Handler) batchCreateAssets(w http.ResponseWriter, r *http.Request) {
	var req BatchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	assets := make([]runs.CreateAssetInput, len(req.Assets))
	for i, asset := range req.Assets {
		assets[i] = runs.CreateAssetInput{
			Name:          asset.Name,
			Type:          asset.Type,
			Providers:     asset.Providers,
			Description:   asset.Description,
			Metadata:      asset.Metadata,
			Schema:        asset.Schema,
			Tags:          asset.Tags,
			Sources:       asset.Sources,
			ExternalLinks: asset.ExternalLinks,
		}
	}
	lineageRequests := make([]runs.LineageInput, len(req.Lineage))
	for i, lineage := range req.Lineage {
		lineageRequests[i] = runs.LineageInput{
			Source: lineage.Source,
			Target: lineage.Target,
			Type:   lineage.Type,
		}
	}
	docRequests := make([]runs.DocumentationInput, len(req.Documentation))
	for i, doc := range req.Documentation {
		docRequests[i] = runs.DocumentationInput{
			AssetMRN: doc.AssetMRN,
			Content:  doc.Content,
			Type:     doc.Type,
		}
	}
	statsRequests := make([]runs.StatisticInput, len(req.Statistics))
	for i, stat := range req.Statistics {
		statsRequests[i] = runs.StatisticInput{
			AssetMRN:   stat.AssetMRN,
			MetricName: stat.MetricName,
			Value:      stat.Value,
		}
	}
	response, err := h.runService.ProcessEntities(r.Context(), req.RunID, assets, lineageRequests, docRequests, statsRequests, req.PipelineName, req.SourceName)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to process entities: %v", err))
		return
	}
	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Destroy pipeline
// @Description Delete all resources ever created by a pipeline (across all sources)
// @Tags pipelines
// @Produce json
// @Param pipelineName path string true "Pipeline Name"
// @Success 200 {object} DestroyRunResponse
// @Router /pipelines/{pipelineName} [delete]
func (h *Handler) destroyPipeline(w http.ResponseWriter, r *http.Request) {
	pipelineName := r.PathValue("pipelineName")

	if pipelineName == "" {
		common.RespondError(w, http.StatusBadRequest, "Pipeline name is required")
		return
	}

	response, err := h.runService.DestroyPipeline(r.Context(), pipelineName)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to destroy pipeline: %v", err))
		return
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Get run entities
// @Description Get paginated list of entities for a specific run
// @Tags runs
// @Produce json
// @Param id path string true "Run ID"
// @Param entity_type query string false "Filter by entity type (asset, lineage, documentation)"
// @Param status query string false "Filter by status (created, updated, deleted, failed)"
// @Param limit query int false "Number of results per page" default(100)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} RunEntitiesResponse
// @Router /runs/{id}/entities [get]
func (h *Handler) getRunEntities(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		common.RespondError(w, http.StatusBadRequest, "Run ID is required")
		return
	}

	entityType := r.URL.Query().Get("entity_type")
	status := r.URL.Query().Get("status")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	entities, total, err := h.runService.ListRunEntities(r.Context(), runID, entityType, status, limit, offset)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list run entities: %v", err))
		return
	}

	response := RunEntitiesResponse{
		Entities: entities,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Cleanup stale runs
// @Description Mark runs as failed if they've been running too long without updates
// @Tags runs
// @Success 200 {object} map[string]int
// @Router /runs/cleanup [post]
func (h *Handler) cleanupStaleRuns(w http.ResponseWriter, r *http.Request) {
	timeoutMinutes := 60
	if t := r.URL.Query().Get("timeout_minutes"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil && parsed > 0 {
			timeoutMinutes = parsed
		}
	}

	timeout := time.Duration(timeoutMinutes) * time.Minute
	count, err := h.runService.CleanupStaleRuns(r.Context(), timeout)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to cleanup stale runs: %v", err))
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]int{"cleaned_up": count})
}

// @Summary List runs
// @Description Get paginated list of runs with filtering
// @Tags runs
// @Produce json
// @Param pipelines query string false "Comma-separated list of pipeline names"
// @Param statuses query string false "Comma-separated list of statuses"
// @Param limit query int false "Number of results per page" default(50)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} object{runs=[]plugin.Run,total=int,limit=int,offset=int,pipelines=[]string}
// @Router /runs [get]
func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	var pipelines []string
	if pipelinesParam := r.URL.Query().Get("pipelines"); pipelinesParam != "" {
		pipelines = strings.Split(pipelinesParam, ",")
		for i := range pipelines {
			pipelines[i] = strings.TrimSpace(pipelines[i])
		}
	}

	var statuses []string
	if statusesParam := r.URL.Query().Get("statuses"); statusesParam != "" {
		validStatuses := map[string]bool{
			"running": true, "completed": true, "failed": true, "cancelled": true,
		}
		rawStatuses := strings.Split(statusesParam, ",")
		for _, status := range rawStatuses {
			status = strings.TrimSpace(status)
			if validStatuses[status] {
				statuses = append(statuses, status)
			}
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	runs, total, availablePipelines, err := h.runService.ListRunsWithFilters(r.Context(), pipelines, statuses, limit, offset)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list runs: %v", err))
		return
	}

	response := struct {
		Runs      []*plugin.Run `json:"runs"`
		Total     int           `json:"total"`
		Limit     int           `json:"limit"`
		Offset    int           `json:"offset"`
		Pipelines []string      `json:"pipelines"`
	}{
		Runs:      runs,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		Pipelines: availablePipelines,
	}

	common.RespondJSON(w, http.StatusOK, response)
}

// @Summary Get run
// @Description Get a specific run by ID
// @Tags runs
// @Produce json
// @Param id path string true "Run ID"
// @Success 200 {object} plugin.Run
// @Router /runs/{id} [get]
func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		common.RespondError(w, http.StatusBadRequest, "Run ID is required")
		return
	}

	run, err := h.runService.GetRun(r.Context(), runID)
	if err != nil {
		if err.Error() == "run not found" {
			common.RespondError(w, http.StatusNotFound, "Run not found")
		} else {
			common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get run: %v", err))
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, run)
}
