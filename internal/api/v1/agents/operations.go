package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/agent"
)

const (
	defaultPeriod = 24 * time.Hour
	maxPeriod     = 30 * 24 * time.Hour
	defaultLimit  = 25
	maxLimit      = 200
)

type RecordRunRequest struct {
	AgentMRN       string            `json:"agent_mrn"`
	RunID          string            `json:"run_id"`
	StartedAt      time.Time         `json:"started_at"`
	EndedAt        *time.Time        `json:"ended_at,omitempty"`
	Status         string            `json:"status"`
	Model          string            `json:"model,omitempty"`
	TokensIn       int               `json:"tokens_in"`
	TokensOut      int               `json:"tokens_out"`
	Error          string            `json:"error,omitempty"`
	ToolCalls      []ToolCallPayload `json:"tool_calls,omitempty"`
	ObservedAssets []string          `json:"observed_assets,omitempty"`
}

type ToolCallPayload struct {
	ToolName   string    `json:"tool_name"`
	TargetMRN  string    `json:"target_mrn,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs *int      `json:"duration_ms,omitempty"`
	Status     string    `json:"status"`
}

type RunsResponse struct {
	Runs  []*agent.Run `json:"runs"`
	Total int          `json:"total"`
}

type ActivityResponse struct {
	Buckets []agent.Bucket `json:"buckets"`
}

// recordRun records a completed agent run.
//
// @Summary  Record agent run
// @Tags     agents
// @Accept   json
// @Produce  json
// @Param    request body RecordRunRequest true "Agent run record"
// @Success  201 {object} agent.Run
// @Router   /agents/runs [post]
func (h *Handler) recordRun(w http.ResponseWriter, r *http.Request) {
	var req RecordRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tcs := make([]agent.ToolCallInput, 0, len(req.ToolCalls))
	for _, tc := range req.ToolCalls {
		tcs = append(tcs, agent.ToolCallInput{
			ToolName:   tc.ToolName,
			TargetMRN:  tc.TargetMRN,
			StartedAt:  tc.StartedAt,
			DurationMs: tc.DurationMs,
			Status:     tc.Status,
		})
	}

	run, err := h.agentService.RecordRun(r.Context(), agent.RunInput{
		AgentMRN:       req.AgentMRN,
		RunID:          req.RunID,
		StartedAt:      req.StartedAt,
		EndedAt:        req.EndedAt,
		Status:         req.Status,
		Model:          req.Model,
		TokensIn:       req.TokensIn,
		TokensOut:      req.TokensOut,
		Error:          req.Error,
		ToolCalls:      tcs,
		ObservedAssets: req.ObservedAssets,
	})
	if errors.Is(err, agent.ErrAgentNotFound) {
		common.RespondError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("recording run: %v", err))
		return
	}
	common.RespondJSON(w, http.StatusCreated, run)
}

// listRuns returns recent runs for an agent.
//
// @Summary  List agent runs
// @Tags     agents
// @Produce  json
// @Param    asset_id path  string true "Agent asset id"
// @Param    period   query string false "Lookback window (e.g. 24h, 7d). Default 24h."
// @Param    limit    query int    false "Max number of runs to return"
// @Success  200 {object} RunsResponse
// @Router   /agents/{asset_id}/runs [get]
func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	period, err := parsePeriod(r.URL.Query().Get("period"))
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	runs, err := h.agentService.ListRuns(r.Context(), assetID, period, limit)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("listing runs: %v", err))
		return
	}
	common.RespondJSON(w, http.StatusOK, RunsResponse{Runs: runs, Total: len(runs)})
}

// getStats returns headline stats over a window.
//
// @Summary  Agent stats
// @Tags     agents
// @Produce  json
// @Param    asset_id path  string true  "Agent asset id"
// @Param    period   query string false "Lookback window (e.g. 24h, 7d). Default 24h."
// @Success  200 {object} agent.Stats
// @Router   /agents/{asset_id}/stats [get]
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	period, err := parsePeriod(r.URL.Query().Get("period"))
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	stats, err := h.agentService.Stats(r.Context(), assetID, period)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("computing stats: %v", err))
		return
	}
	common.RespondJSON(w, http.StatusOK, stats)
}

// getActivity returns hour-aligned run buckets for the activity chart.
//
// @Summary  Agent activity
// @Tags     agents
// @Produce  json
// @Param    asset_id path  string true  "Agent asset id"
// @Param    period   query string false "Lookback window (e.g. 24h, 7d). Default 24h."
// @Success  200 {object} ActivityResponse
// @Router   /agents/{asset_id}/activity [get]
func (h *Handler) getActivity(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	period, err := parsePeriod(r.URL.Query().Get("period"))
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	buckets, err := h.agentService.BucketRuns(r.Context(), assetID, period)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("bucketing runs: %v", err))
		return
	}
	if buckets == nil {
		buckets = []agent.Bucket{}
	}
	common.RespondJSON(w, http.StatusOK, ActivityResponse{Buckets: buckets})
}

func parsePeriod(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultPeriod, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		// Allow shorthand like "7d"
		if len(raw) > 1 && raw[len(raw)-1] == 'd' {
			n, perr := strconv.Atoi(raw[:len(raw)-1])
			if perr == nil && n > 0 {
				d = time.Duration(n) * 24 * time.Hour
				err = nil
			}
		}
	}
	if err != nil {
		return 0, fmt.Errorf("invalid period %q", raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("period must be positive")
	}
	if d > maxPeriod {
		d = maxPeriod
	}
	return d, nil
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid limit %q", raw)
	}
	if n > maxLimit {
		n = maxLimit
	}
	return n, nil
}
