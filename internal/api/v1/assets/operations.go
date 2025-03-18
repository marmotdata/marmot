package assets

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/mrn"
	"github.com/marmotdata/marmot/internal/services/asset"
	"github.com/marmotdata/marmot/internal/services/user"
	"github.com/rs/zerolog/log"
)

type CreateRequest struct {
	Name          string                       `json:"name" validate:"required"`
	Type          string                       `json:"type" validate:"required"`
	Providers     []string                     `json:"providers" validate:"required"`
	Description   *string                      `json:"description"`
	Metadata      map[string]interface{}       `json:"metadata"`
	Schema        map[string]interface{}       `json:"schema"`
	Tags          []string                     `json:"tags"`
	Sources       []asset.AssetSource          `json:"sources"`
	Environments  map[string]asset.Environment `json:"environments"`
	ExternalLinks []asset.ExternalLink         `json:"external_links"`
}

type UpdateRequest struct {
	Name          *string                      `json:"name"`
	Description   *string                      `json:"description"`
	Metadata      map[string]interface{}       `json:"metadata"`
	Type          string                       `json:"type"`
	Providers     []string                     `json:"providers"`
	Schema        map[string]interface{}       `json:"schema"`
	Tags          []string                     `json:"tags"`
	Sources       []asset.AssetSource          `json:"sources"`
	Environments  map[string]asset.Environment `json:"environments"`
	ExternalLinks []asset.ExternalLink         `json:"external_links"`
}

// @Summary Create a new asset
// @Description Create a new asset in the system
// @Tags assets
// @Accept json
// @Produce json
// @Param asset body CreateRequest true "Asset creation request"
// @Success 201 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Router /assets [post]
func (h *Handler) createAsset(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	usr, ok := r.Context().Value(common.UserContextKey).(*user.User)
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "User context required")
		return
	}

	mrn := mrn.New(req.Type, req.Providers[0], req.Name)
	input := asset.CreateInput{
		Name:          &req.Name,
		Type:          req.Type,
		Providers:     req.Providers,
		Description:   req.Description,
		Metadata:      req.Metadata,
		Schema:        req.Schema,
		Tags:          req.Tags,
		Sources:       req.Sources,
		Environments:  req.Environments,
		ExternalLinks: req.ExternalLinks,
		MRN:           &mrn,
		CreatedBy:     usr.Name,
	}

	log.Info().Interface("input", input).Msg("createAsset: Input to assetService.Create")

	newAsset, err := h.assetService.Create(r.Context(), input)

	log.Info().Interface("asset", newAsset).Msg("createAsset: Output to assetService.Create")
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrInvalidInput):
			log.Error().Err(err).Interface("request", req).Msg("Invalid input")
			common.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, asset.ErrAlreadyExists):
			log.Error().Err(err).Str("mrn", mrn).Msg("Asset already exists")
			common.RespondError(w, http.StatusConflict, "Asset already exists")
		default:
			log.Error().Err(err).Interface("input", input).Msg("Failed to create asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusCreated, newAsset)
}

// @Summary Get an asset by ID
// @Description Get detailed information about a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/{id} [get]
func (h *Handler) getAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	result, err := h.assetService.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to get asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

// @Summary Update an asset
// @Description Update an existing asset's information
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param asset body UpdateRequest true "Asset update request"
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/{id} [put]
func (h *Handler) updateAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	input := asset.UpdateInput{
		Name:          req.Name,
		Description:   req.Description,
		Type:          req.Type,
		Providers:     req.Providers,
		Metadata:      req.Metadata,
		Schema:        req.Schema,
		Tags:          req.Tags,
		Sources:       req.Sources,
		Environments:  req.Environments,
		ExternalLinks: req.ExternalLinks,
	}

	updated, err := h.assetService.Update(r.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		case errors.Is(err, asset.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to update asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, updated)
}

// @Summary Delete an asset
// @Description Delete an asset from the system
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/{id} [delete]
func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	err := h.assetService.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		case errors.Is(err, asset.ErrInvalidInput):
			common.RespondError(w, http.StatusConflict, "Asset has dependencies")
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to delete asset")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get an asset by qualified name
// @Description Get detailed information about a specific asset using its qualified name
// @Tags assets
// @Accept json
// @Produce json
// @Param qualifiedName path string true "Asset qualified name"
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /assets/qualified-name/{qualifiedName} [get]
func (h *Handler) getAssetByMRN(w http.ResponseWriter, r *http.Request) {
	qualifiedName := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/qualified-name/")
	if qualifiedName == "" {
		http.NotFound(w, r)
		return
	}

	result, err := h.assetService.GetByMRN(r.Context(), qualifiedName)
	if err != nil {
		switch err {
		case asset.ErrAssetNotFound:
			http.NotFound(w, r)
		default:
			log.Error().
				Err(err).
				Str("endpoint", r.URL.Path).
				Str("method", r.Method).
				Msg("Failed to get asset")

			common.RespondError(w, http.StatusInternalServerError, "Failed to get asset")
		}
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}
