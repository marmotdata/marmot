package lineage

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/services/lineage"
	"github.com/rs/zerolog/log"
)

// @Summary Get direct lineage by ID
// @Description Get a specific direct lineage connection by its ID
// @Tags lineage
// @Accept json
// @Produce json
// @Param id path string true "Edge ID" format(uuid)
// @Success 200 {object} lineage.LineageEdge
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /lineage/direct/{id} [get]
func (h *Handler) getDirectLineage(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		common.RespondError(w, http.StatusBadRequest, "Edge ID is required")
		return
	}
	edgeID := parts[len(parts)-1]

	log.Info().
		Str("edge_id", edgeID).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Getting direct lineage connection")

	edge, err := h.lineageService.GetDirectLineage(r.Context(), edgeID)
	if err != nil {
		log.Error().Err(err).
			Str("edge_id", edgeID).
			Msg("Failed to get direct lineage")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get lineage")
		return
	}

	if edge == nil {
		common.RespondError(w, http.StatusNotFound, "Lineage edge not found")
		return
	}

	common.RespondJSON(w, http.StatusOK, edge)
}

// @Summary Create direct lineage
// @Description Create a direct lineage connection between two assets and returns the created edge
// @Tags lineage
// @Accept json
// @Produce json
// @Param edge body lineage.LineageEdge true "Lineage edge to create"
// @Success 200 {object} lineage.LineageEdge
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /lineage/direct [post]
func (h *Handler) createDirectLineage(w http.ResponseWriter, r *http.Request) {
	var edge lineage.LineageEdge
	if err := json.NewDecoder(r.Body).Decode(&edge); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Info().
		Str("source", edge.Source).
		Str("target", edge.Target).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Creating direct lineage connection")

	edgeID, err := h.lineageService.CreateDirectLineage(r.Context(), edge.Source, edge.Target)
	if err != nil {
		log.Error().Err(err).
			Str("source", edge.Source).
			Str("target", edge.Target).
			Msg("Failed to create direct lineage")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create lineage")
		return
	}

	edge.ID = edgeID
	common.RespondJSON(w, http.StatusOK, edge)
}

// @Summary Delete direct lineage
// @Description Delete a direct lineage connection by its ID
// @Tags lineage
// @Accept json
// @Produce json
// @Param id path string true "Edge ID" format(uuid)
// @Success 200 "OK"
// @Failure 400 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /lineage/direct/{id} [delete]
func (h *Handler) deleteDirectLineage(w http.ResponseWriter, r *http.Request) {
	// Extract the ID from the path
	//TODO: Move to Chi
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		common.RespondError(w, http.StatusBadRequest, "Edge ID is required")
		return
	}
	edgeID := parts[len(parts)-1]

	log.Info().
		Str("edge_id", edgeID).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("Deleting direct lineage connection")

	if err := h.lineageService.DeleteDirectLineage(r.Context(), edgeID); err != nil {
		log.Error().Err(err).
			Str("edge_id", edgeID).
			Msg("Failed to delete direct lineage")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete lineage")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// @Summary Get asset lineage
// @Description Get upstream and downstream lineage for a specific asset
// @Tags lineage
// @Accept json
// @Produce json
// @Param id path string true "Asset ID" format(uuid)
// @Param limit query int false "Maximum depth of lineage graph" default(10)
// @Param direction query string false "Direction of lineage (upstream, downstream, or both)" Enums(upstream, downstream, both) default(both)
// @Success 200 {object} lineage.LineageResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /lineage/assets/{id} [get]
func (h *Handler) getAssetLineage(w http.ResponseWriter, r *http.Request) {
	// Extract the asset ID from the path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		common.RespondError(w, http.StatusBadRequest, "Asset ID is required")
		return
	}
	assetID := parts[len(parts)-1]

	limit := 10
	if limitStr := r.URL.Query().Get("depth"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}

	lineage, err := h.lineageService.GetAssetLineage(r.Context(), assetID, limit, direction)
	if err != nil {
		log.Error().Err(err).
			Str("asset_id", assetID).
			Int("limit", limit).
			Str("direction", direction).
			Msg("Failed to get asset lineage")

		if errors.Is(err, asset.ErrAssetNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset not found")
			return
		}

		common.RespondError(w, http.StatusInternalServerError, "Failed to get asset lineage")
		return
	}

	common.RespondJSON(w, http.StatusOK, lineage)
}
