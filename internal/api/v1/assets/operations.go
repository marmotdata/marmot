package assets

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/limits"
	"github.com/marmotdata/marmot/internal/core/metamodel"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// AssetResponse wraps an asset with enriched external links from rules.
type AssetResponse struct {
	*asset.Asset
	EnrichedExternalLinks []assetrule.EnrichedExternalLink `json:"enriched_external_links,omitempty"`
}

type CreateRequest struct {
	Name          string                       `json:"name" validate:"required"`
	Type          string                       `json:"type" validate:"required"`
	Providers     []string                     `json:"providers" validate:"required"`
	Description   *string                      `json:"description"`
	Metadata      map[string]interface{}       `json:"metadata"`
	Schema        map[string]string            `json:"schema"`
	Tags          []string                     `json:"tags"`
	Sources       []asset.AssetSource          `json:"sources"`
	Environments  map[string]asset.Environment `json:"environments"`
	ExternalLinks []asset.ExternalLink         `json:"external_links"`
} // @name CreateAssetRequest

type UpdateRequest struct {
	Name            *string                      `json:"name"`
	Description     *string                      `json:"description"`
	UserDescription *string                      `json:"user_description"`
	Metadata        map[string]interface{}       `json:"metadata"`
	Type            string                       `json:"type"`
	Providers       []string                     `json:"providers"`
	Schema          map[string]string            `json:"schema"`
	Tags            []string                     `json:"tags"`
	Sources         []asset.AssetSource          `json:"sources"`
	Environments    map[string]asset.Environment `json:"environments"`
	ExternalLinks   []asset.ExternalLink         `json:"external_links"`
} // @name UpdateAssetRequest

// @Summary Create a new asset
// @Description Create a new asset in the system
// @Tags assets
// @Accept json
// @Produce json
// @Param asset body CreateRequest true "Asset creation request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID postAssets
// @Router /api/v1/assets/ [post]
func (h *Handler) createAsset(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
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
		CreatedBy:     principal.DisplayName(),
	}

	log.Info().Interface("input", input).Msg("createAsset: Input to assetService.Create")

	newAsset, err := h.assetService.Create(r.Context(), input)

	log.Info().Interface("asset", newAsset).Msg("createAsset: Output to assetService.Create")
	if err != nil {
		if limitErr, ok := limits.AsLimitExceeded(err); ok {
			common.RespondLimitExceeded(w, limitErr)
			return
		}
		if respondAssetWriteError(w, err) {
			return
		}
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

func (h *Handler) enrichAssetResponse(r *http.Request, result *asset.Asset) *AssetResponse {
	resp := &AssetResponse{Asset: result}

	enrichedLinks, err := h.assetRuleService.GetEnrichedLinks(r.Context(), result.ID)
	if err != nil {
		log.Warn().Err(err).Str("asset_id", result.ID).Msg("Failed to get enriched links")
	} else if len(enrichedLinks) > 0 {
		// Merge direct links (source: "asset") with rule-managed links
		allLinks := make([]assetrule.EnrichedExternalLink, 0, len(result.ExternalLinks)+len(enrichedLinks))
		for _, l := range result.ExternalLinks {
			allLinks = append(allLinks, assetrule.EnrichedExternalLink{
				ExternalLink: l,
				Source:       "asset",
			})
		}
		allLinks = append(allLinks, enrichedLinks...)
		resp.EnrichedExternalLinks = allLinks
	}

	return resp
}

// @Summary Get an asset by ID
// @Description Get detailed information about a specific asset
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetsID
// @Router /api/v1/assets/{id} [get]
func (h *Handler) getAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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

	h.metricsService.GetRecorder().RecordAssetView(r.Context(), result.ID, result.Type, *result.Name, result.Providers[0])
	h.lookups.Record(r.Context(), lookups.CategoryAssetDetail)

	w.Header().Set("ETag", assetETag(result.Version))
	common.RespondJSON(w, http.StatusOK, h.enrichAssetResponse(r, result))
}

// @Summary Update an asset
// @Description Update an existing asset's information
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param asset body UpdateRequest true "Asset update request"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putAssetsID
// @Router /api/v1/assets/{id} [put]
func (h *Handler) updateAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
		Name:            req.Name,
		Description:     req.Description,
		UserDescription: req.UserDescription,
		Type:            req.Type,
		Providers:       req.Providers,
		Metadata:        req.Metadata,
		Schema:          req.Schema,
		Tags:            req.Tags,
		Sources:         req.Sources,
		Environments:    req.Environments,
		ExternalLinks:   req.ExternalLinks,
	}
	if header := r.Header.Get("If-Match"); header != "" {
		version, ok := parseIfMatch(header)
		if !ok {
			common.RespondError(w, http.StatusBadRequest, "If-Match must contain one quoted asset version")
			return
		}
		input.ExpectedVersion = &version
	}

	updated, err := h.assetService.Update(r.Context(), id, input)
	if err != nil {
		if respondAssetWriteError(w, err) {
			return
		}
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

	w.Header().Set("ETag", assetETag(updated.Version))
	common.RespondJSON(w, http.StatusOK, updated)
}

// @Summary Delete an asset
// @Description Delete an asset from the system
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteAssetsID
// @Router /api/v1/assets/{id} [delete]
func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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
// @Security ApiKeyAuth
// @Security BearerAuth
// @Param name path string true "Asset qualified name"
// @Success 200 {object} asset.Asset
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getAssetsQualifiedNameQualifiedName
// @Router /api/v1/assets/qualified-name/{name} [get]
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

	h.lookups.Record(r.Context(), lookups.CategoryAssetDetail)

	w.Header().Set("ETag", assetETag(result.Version))
	common.RespondJSON(w, http.StatusOK, h.enrichAssetResponse(r, result))
}

// @Summary Get the effective metamodel schema
// @Description Returns the composed native and configured field schema. Clients must not reinterpret source YAML.
// @Tags metamodel
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} metamodel.Schema
// @ID getMetamodel
// @Router /api/v1/metamodel [get]
func (h *Handler) getMetamodel(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, h.assetService.Metamodel())
}

type patchFieldsRequest struct {
	Fields map[string]any `json:"fields"`
}

// @Summary Patch governed asset fields
// @Description Partial update of governed fields. Absence preserves; null deletes only when the field is nullable. Requires If-Match.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID"
// @Param If-Match header string true "Expected asset version"
// @Param request body patchFieldsRequest true "Fields to apply"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 412 {object} common.ErrorResponse
// @Failure 428 {object} common.ErrorResponse
// @ID patchAssetsID
// @Router /api/v1/assets/{id} [patch]
func (h *Handler) patchAssetFields(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Asset ID required")
		return
	}

	header := r.Header.Get("If-Match")
	if header == "" {
		common.RespondError(w, http.StatusPreconditionRequired, "If-Match required")
		return
	}

	version, ok := parseIfMatch(header)
	if !ok {
		common.RespondError(w, http.StatusBadRequest, "If-Match must contain one quoted asset version")
		return
	}
	var req patchFieldsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		common.RespondError(w, http.StatusBadRequest, "Expected one JSON object")
		return
	}
	if req.Fields == nil {
		common.RespondError(w, http.StatusBadRequest, "fields required")
		return
	}

	updated, err := h.assetService.PatchFields(r.Context(), id, version, req.Fields)
	if err != nil {
		if respondAssetWriteError(w, err) {
			return
		}
		switch {
		case errors.Is(err, asset.ErrAssetNotFound):
			common.RespondError(w, http.StatusNotFound, "Asset not found")
		case errors.Is(err, asset.ErrInvalidInput):
			common.RespondError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error().Err(err).Str("id", id).Msg("Failed to patch asset fields")
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	w.Header().Set("ETag", assetETag(updated.Version))
	common.RespondJSON(w, http.StatusOK, updated)
}

func assetETag(version int64) string { return `"` + strconv.FormatInt(version, 10) + `"` }

func parseIfMatch(header string) (int64, bool) {
	header = strings.TrimSpace(header)
	if len(header) < 3 || header[0] != '"' || header[len(header)-1] != '"' {
		return 0, false
	}
	version, err := strconv.ParseInt(header[1:len(header)-1], 10, 64)
	return version, err == nil && version > 0 && header == assetETag(version)
}

func respondAssetWriteError(w http.ResponseWriter, err error) bool {
	var validation *metamodel.ValidationError
	switch {
	case errors.As(err, &validation):
		common.RespondJSON(w, http.StatusBadRequest, validation)
		return true
	case errors.Is(err, asset.ErrVersionConflict):
		common.RespondError(w, http.StatusPreconditionFailed, "Asset version conflict")
		return true
	case errors.Is(err, asset.ErrVersionRequired):
		common.RespondError(w, http.StatusPreconditionRequired, "If-Match required to change governed fields")
		return true
	}
	return false
}
