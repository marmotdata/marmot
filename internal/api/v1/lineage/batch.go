package lineage

import (
	"encoding/json"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/services/lineage"
	"github.com/rs/zerolog/log"
)

type BatchLineageResult struct {
	Edge   lineage.LineageEdge `json:"edge"`
	Status string              `json:"status"` // "created", "duplicate", or "existing"
}

// @Summary Batch create lineage edges
// @Description Create lineage edges in batch
// @Tags lineage
// @Accept json
// @Produce json
// @Param edges body []lineage.LineageEdge true "Array of lineage edges to create"
// @Success 200 {array} BatchLineageResult
// @Failure 400 {object} common.ErrorResponse
// @Router /lineage/batch [post]
func (h *Handler) batchCreateLineage(w http.ResponseWriter, r *http.Request) {
	var edges []lineage.LineageEdge
	if err := json.NewDecoder(r.Body).Decode(&edges); err != nil {
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

		if exists {
			results = append(results, BatchLineageResult{
				Edge:   edge,
				Status: "existing",
			})
			continue
		}

		if _, err := h.lineageService.CreateDirectLineage(r.Context(), edge.Source, edge.Target); err != nil {
			log.Error().Err(err).
				Str("source", edge.Source).
				Str("target", edge.Target).
				Msg("Failed to create lineage edge")
			continue
		}

		results = append(results, BatchLineageResult{
			Edge:   edge,
			Status: "created",
		})
	}

	common.RespondJSON(w, http.StatusOK, results)
}
