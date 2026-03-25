package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/search"
)

type ReindexAcceptedResponse struct {
	Status  string `json:"status" example:"accepted"`
	Message string `json:"message" example:"Reindex started"`
}

type ReindexStatusResponse struct {
	Running      bool `json:"running"`
	ESConfigured bool `json:"es_configured"`
}

// @Summary Start search reindex
// @Description Trigger a full reindex from PostgreSQL to Elasticsearch. The reindex runs asynchronously in the background. Only one reindex can run at a time.
// @Tags admin
// @Produce json
// @Success 202 {object} ReindexAcceptedResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @Router /admin/search/reindex [post]
func (h *Handler) startReindex(w http.ResponseWriter, r *http.Request) {
	if h.reindexer == nil {
		common.RespondError(w, http.StatusServiceUnavailable, "Elasticsearch is not configured")
		return
	}

	if h.reindexer.Running() {
		common.RespondError(w, http.StatusConflict, "Reindex already in progress")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		if err := h.reindexer.RunOnce(ctx); err != nil {
			if !errors.Is(err, search.ErrReindexInProgress) {
				// Logged inside RunOnce
			}
		}
	}()

	common.RespondJSON(w, http.StatusAccepted, ReindexAcceptedResponse{
		Status:  "accepted",
		Message: "Reindex started",
	})
}

// @Summary Get reindex status
// @Description Check whether a search reindex is currently running and whether Elasticsearch is configured.
// @Tags admin
// @Produce json
// @Success 200 {object} ReindexStatusResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Router /admin/search/reindex [get]
func (h *Handler) getReindexStatus(w http.ResponseWriter, r *http.Request) {
	running := false
	if h.reindexer != nil {
		running = h.reindexer.Running()
	}

	common.RespondJSON(w, http.StatusOK, ReindexStatusResponse{
		Running:      running,
		ESConfigured: h.reindexer != nil,
	})
}
