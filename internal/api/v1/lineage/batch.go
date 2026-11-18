package lineage

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/lineage"
	"github.com/rs/zerolog/log"
)

type BatchLineageResult struct {
	Edge   lineage.LineageEdge `json:"edge"`
	Status string              `json:"status"` // "created", "duplicate", or "existing"
} // @name BatchLineageResult

// @Summary Batch create lineage edges or ingest OpenLineage events
// @Description Takes an array of lineage edges, or an array of OpenLineage events as the OpenLineage batch endpoint does. An array whose first element has an eventTime is read as OpenLineage events, processed in order and answered with an OpenLineageBatchResponse; anything else is read as edges. The body may be gzip-compressed. Unlike POST /api/v1/lineage, this endpoint always requires an API key.
// @Tags lineage
// @Accept json
// @Produce json
// @Param edges body []lineage.LineageEdge true "Array of lineage edges to create, or of OpenLineage events"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} BatchLineageResult
// @Failure 400 {object} common.ErrorResponse
// @Failure 413 {object} common.ErrorResponse
// @ID postLineageBatch
// @Router /api/v1/lineage/batch [post]
func (h *Handler) batchCreateLineage(w http.ResponseWriter, r *http.Request) {
	body, err := readRequestBody(w, r)
	if err != nil {
		respondBodyError(w, err)
		return
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if isOpenLineageBatch(items) {
		h.ingestOpenLineageBatch(w, r, items)
		return
	}

	var edges []lineage.LineageEdge
	if err := json.Unmarshal(body, &edges); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make([]BatchLineageResult, 0, len(edges))
	seenEdges := make(map[string]struct{})

	for _, edge := range edges {
		edgeKey := edge.Source + "->" + edge.Target
		if _, exists := seenEdges[edgeKey]; exists {
			results = append(results, BatchLineageResult{
				Edge:   edge,
				Status: "duplicate",
			})
			continue
		}
		seenEdges[edgeKey] = struct{}{}

		exists, err := h.lineageService.EdgeExists(r.Context(), edge.Source, edge.Target)
		if err != nil {
			log.Error().Err(err).
				Str("source", edge.Source).
				Str("target", edge.Target).
				Msg("Failed to check lineage edge")
			continue
		}

		// Existing edges are written too, so a re-sent edge refreshes its column lineage and declarer.
		if _, err := h.lineageService.CreateDeclaredLineage(r.Context(), edge.Source, edge.Target, edge.Type, edge.JobMRN, declarerOf(r), edge.ColumnLineage); err != nil {
			log.Error().Err(err).
				Str("source", edge.Source).
				Str("target", edge.Target).
				Msg("Failed to create lineage edge")
			continue
		}

		status := "created"
		if exists {
			status = "existing"
		}
		results = append(results, BatchLineageResult{
			Edge:   edge,
			Status: status,
		})
	}

	common.RespondJSON(w, http.StatusOK, results)
}
