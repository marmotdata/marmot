package assets

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdetail"
	"github.com/marmotdata/marmot/internal/core/assetrule"
	"github.com/marmotdata/marmot/internal/core/limits"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/telemetry/lookups"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// AssetResponse wraps an asset with enriched external links from rules.
type AssetResponse struct {
	*asset.Asset
	EnrichedExternalLinks []assetrule.EnrichedExternalLink `json:"enriched_external_links,omitempty"`
	// RowCount and ColumnCount give the asset's size for its header, read from what its plugin recorded; absent when no plugin said.
	RowCount    *int64 `json:"row_count,omitempty"`
	ColumnCount *int   `json:"column_count,omitempty"`
	// FieldAnnotations are the markers people set on schema fields, merged by the UI onto the fields the schema describes.
	FieldAnnotations []assetdetail.FieldAnnotation `json:"field_annotations,omitempty"`
	// Stale is true when the pipeline that refreshes the asset has missed more than two runs' worth of time.
	Stale bool `json:"stale"`
	// Popularity is a level from 0 to 4 from the asset's views over the last 30 days.
	Popularity *int `json:"popularity,omitempty"`
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
	// UserDescription is the people-written description, kept apart from the source's own.
	UserDescription *string `json:"user_description"`
	// Owners are the users and teams to own the asset.
	Owners []CreateOwner `json:"owners"`
	// Terms are the IDs of glossary terms to link the asset to.
	Terms []string `json:"terms"`
} // @name CreateAssetRequest

// CreateOwner names a user or team to own a new asset.
type CreateOwner struct {
	Type string `json:"type" enums:"user,team"`
	ID   string `json:"id"`
} // @name CreateAssetOwner

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
// @Description Create a new asset in the system. Owners, a user description and glossary terms can be set in the same request; if any of them cannot be applied the asset is not kept.
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

	if msg := validateCreateExtras(req); msg != "" {
		common.RespondError(w, http.StatusBadRequest, msg)
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

	if status, msg := h.applyCreateExtras(r, newAsset, req); status != 0 {
		// The extras were part of the request, so a half-made asset would leave the caller to tidy up; remove it and report what went wrong instead.
		if err := h.assetService.Delete(r.Context(), newAsset.ID); err != nil {
			log.Error().Err(err).Str("id", newAsset.ID).Msg("Failed to remove asset after its owners, description or terms could not be set")
		}
		common.RespondError(w, status, msg)
		return
	}

	common.RespondJSON(w, http.StatusCreated, newAsset)
}

// validateCreateExtras checks the owners and terms of a create request before anything is written, so a mistyped ID is reported without creating an asset.
func validateCreateExtras(req CreateRequest) string {
	if len(req.Providers) == 0 {
		return "At least one provider is required"
	}
	for _, o := range req.Owners {
		if o.Type != team.OwnerTypeUser && o.Type != team.OwnerTypeTeam {
			return "Owner type must be user or team"
		}
		if _, err := uuid.Parse(o.ID); err != nil {
			return "Owner ID must be a valid ID"
		}
	}
	for _, id := range req.Terms {
		if _, err := uuid.Parse(id); err != nil {
			return "Glossary term ID must be a valid ID"
		}
	}
	return ""
}

// applyCreateExtras sets the owners, user description and glossary terms a create request carried. It returns a status and message when one could not be set, or zero when all were.
func (h *Handler) applyCreateExtras(r *http.Request, created *asset.Asset, req CreateRequest) (int, string) {
	ctx := r.Context()
	if req.UserDescription != nil && strings.TrimSpace(*req.UserDescription) != "" {
		updated, err := h.assetService.Update(ctx, created.ID, asset.UpdateInput{UserDescription: req.UserDescription, SkipNotification: true})
		if err != nil {
			log.Error().Err(err).Str("id", created.ID).Msg("Failed to set user description on new asset")
			return http.StatusInternalServerError, "Failed to set the description"
		}
		*created = *updated
	}
	for _, o := range req.Owners {
		if err := h.teamService.AddAssetOwner(ctx, created.ID, o.Type, o.ID); err != nil {
			if errors.Is(err, team.ErrTeamNotFound) {
				return http.StatusBadRequest, "Owner team not found"
			}
			log.Error().Err(err).Str("id", created.ID).Str("owner_id", o.ID).Msg("Failed to add owner to new asset")
			return http.StatusBadRequest, "Owner could not be added; check the user or team exists"
		}
	}
	if len(req.Terms) > 0 {
		principal, _ := common.PrincipalFromContext(ctx)
		// created_by is a foreign key into users, so only a person goes there; machine identities are named in source, as when adding terms on their own.
		source, author := "user", ""
		if usr := principal.AsUser(); usr != nil {
			author = usr.ID
		} else {
			source = string(principal.Type())
		}
		if err := h.assetService.AddTerms(ctx, created.ID, req.Terms, source, author); err != nil {
			log.Error().Err(err).Str("id", created.ID).Msg("Failed to link glossary terms to new asset")
			return http.StatusBadRequest, "Glossary terms could not be linked; check they exist"
		}
	}
	return 0, ""
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

	if n, ok := assetdetail.RowCount(result.Metadata); ok {
		resp.RowCount = &n
	}
	if n, ok := assetdetail.ColumnCount(result.Schema); ok {
		resp.ColumnCount = &n
	}
	if h.signals != nil {
		if stale, err := h.signals.IsStale(r.Context(), result.ID); err != nil {
			log.Warn().Err(err).Str("asset_id", result.ID).Msg("Failed to check whether the asset is stale")
		} else {
			resp.Stale = stale
		}
		if levels, err := h.signals.Popularity(r.Context(), []string{result.ID}); err != nil {
			log.Warn().Err(err).Str("asset_id", result.ID).Msg("Failed to get asset popularity")
		} else {
			level := levels[result.ID]
			resp.Popularity = &level
		}
	}
	if h.details != nil {
		annotations, err := h.details.ListFieldAnnotations(r.Context(), result.ID)
		if err != nil {
			log.Warn().Err(err).Str("asset_id", result.ID).Msg("Failed to get field annotations")
		} else if len(annotations) > 0 {
			resp.FieldAnnotations = annotations
		}
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

	common.RespondJSON(w, http.StatusOK, h.enrichAssetResponse(r, result))
}
