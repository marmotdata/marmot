package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/rs/zerolog/log"
)

// ExistsResponse is the 409 a create returns when the MRN is taken, naming the asset that holds it so the caller can import or update that asset, or send a more specific MRN or zone.
type ExistsResponse struct {
	Error       string `json:"error"`
	ExistingID  string `json:"existing_id,omitempty"`
	ExistingMRN string `json:"existing_mrn,omitempty"`
} // @name AssetExistsResponse

// IdentityResponse says who an asset is: its MRN split into parts, the MRNs it used to have, and what each writer says about it.
type IdentityResponse struct {
	ID            string               `json:"id"`
	MRN           string               `json:"mrn"`
	Parts         *asset.MRNParts      `json:"mrn_parts,omitempty"`
	Aliases       []string             `json:"aliases"`
	Contributions []asset.Contribution `json:"contributions"`
} // @name AssetIdentityResponse

// MergeRequest names the asset another is folded into.
type MergeRequest struct {
	Into string `json:"into"`
} // @name MergeAssetRequest

// UpsertRequest creates the asset an MRN names or, when it exists, updates what the caller says about it.
type UpsertRequest struct {
	CreateRequest
	MRN string `json:"mrn"`
} // @name UpsertAssetRequest

// ResolveResponse is what an MRN of any kind names.
type ResolveResponse struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	MRN  string `json:"mrn"`
	// Moved is true when the MRN asked for is an alias of this one.
	Moved bool `json:"moved"`
} // @name ResolveMRNResponse

// apiWriter is the contribution a person, Terraform or an API client makes through the REST API.
func apiWriter(p auth.Principal) string {
	return "api:" + string(p.Type()) + ":" + p.ID()
}

// requestedMRN is the MRN a create asks for: the explicit one, or one built from type, first provider, zone and name.
func requestedMRN(req CreateRequest) (string, error) {
	if req.MRN != nil && strings.TrimSpace(*req.MRN) != "" {
		c, err := asset.CanonicalMRN(*req.MRN)
		if err != nil {
			return "", err
		}
		return c, nil
	}
	return mrn.Asset(req.Type, req.Providers[0], req.Name).WithZone(req.Zone).String(), nil
}

func respondExists(w http.ResponseWriter, err error, requested string) {
	resp := ExistsResponse{Error: fmt.Sprintf("An asset with MRN %s already exists. Import or update it, or set mrn or zone to tell the new asset apart", requested)}
	var exists *asset.ExistsError
	if errors.As(err, &exists) && exists.Asset != nil {
		resp.ExistingID = exists.Asset.ID
		if exists.Asset.MRN != nil {
			resp.ExistingMRN = *exists.Asset.MRN
		}
	}
	common.RespondJSON(w, http.StatusConflict, resp)
}

// applyManualUpdate records what an API caller says about an asset as its own contribution, so it survives the next pipeline run instead of being overwritten by it, and applies the fields that are not writer-owned directly.
func (h *Handler) applyManualUpdate(r *http.Request, id string, req UpdateRequest) (*asset.Asset, error) {
	ctx := r.Context()
	principal, ok := common.PrincipalFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: authentication required", asset.ErrInvalidInput)
	}
	if _, err := h.assetService.Get(ctx, id); err != nil {
		return nil, err
	}

	writer := apiWriter(principal)
	var fields asset.ContributionFields
	contribs, err := h.assetService.Contributions(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, c := range contribs {
		if c.Writer == writer {
			fields = c.Fields
		}
	}
	touched := false
	if req.Name != nil {
		fields.Name, touched = req.Name, true
	}
	if req.Description != nil {
		fields.Description, touched = req.Description, true
	}
	if req.Type != "" {
		fields.Type, touched = req.Type, true
	}
	if req.Providers != nil {
		fields.Providers, touched = req.Providers, true
	}
	if req.Metadata != nil {
		fields.Metadata, touched = req.Metadata, true
	}
	if req.Schema != nil {
		fields.Schema, touched = req.Schema, true
	}
	if req.Tags != nil {
		fields.Tags, touched = req.Tags, true
	}
	if req.ExternalLinks != nil {
		fields.ExternalLinks, touched = req.ExternalLinks, true
	}

	var updated *asset.Asset
	if touched {
		if updated, err = h.assetService.Contribute(ctx, id, writer, asset.RoleManual, fields, false); err != nil {
			return nil, err
		}
	}
	if req.UserDescription != nil || req.Sources != nil || req.Environments != nil {
		if updated, err = h.assetService.Update(ctx, id, asset.UpdateInput{
			UserDescription: req.UserDescription,
			Sources:         req.Sources,
			Environments:    req.Environments,
		}); err != nil {
			return nil, err
		}
	}
	if updated == nil {
		return h.assetService.Get(ctx, id)
	}
	return updated, nil
}

// @Summary Create or update an asset by MRN
// @Description Idempotent write for automation: creates the asset the MRN names, or records the caller's fields on it when it already exists. Unlike POST /assets/ this never fails because the asset exists, so only use it when the caller means the same asset.
// @Tags assets
// @Accept json
// @Produce json
// @Param asset body UpsertRequest true "Asset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.Asset
// @Success 201 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @ID putAssetsByMRN
// @Router /api/v1/assets/by-mrn [put]
func (h *Handler) upsertByMRN(w http.ResponseWriter, r *http.Request) {
	var req UpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.MRN) == "" {
		common.RespondError(w, http.StatusBadRequest, "mrn is required")
		return
	}
	parsed, err := mrn.ParseMRN(req.MRN)
	if err != nil || parsed.Kind != mrn.KindAsset {
		common.RespondError(w, http.StatusBadRequest, "mrn must be an asset MRN")
		return
	}
	if req.Name == "" {
		req.Name = parsed.Name
	}
	if req.Type == "" {
		req.Type = parsed.Type
	}
	if len(req.Providers) == 0 {
		req.Providers = []string{parsed.Provider}
	}

	existing, err := h.assetService.GetByMRNs(r.Context(), []string{req.MRN})
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if a := existing[req.MRN]; a != nil {
		updated, err := h.applyManualUpdate(r, a.ID, UpdateRequest{
			Name: &req.Name, Description: req.Description, UserDescription: req.UserDescription, Metadata: req.Metadata,
			Schema: req.Schema, Tags: req.Tags, Sources: req.Sources, Environments: req.Environments, ExternalLinks: req.ExternalLinks,
		})
		if err != nil {
			common.RespondError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		common.RespondJSON(w, http.StatusOK, updated)
		return
	}

	m := req.MRN
	req.CreateRequest.MRN = &m
	body, _ := json.Marshal(req.CreateRequest)
	r2 := r.Clone(r.Context())
	r2.Body = ioNopCloser(body)
	h.createAsset(w, r2)
}

// @Summary Merge an asset into another
// @Description Folds the asset into the one named by into when both turn out to be the same thing, such as an OpenLineage stub and the table a plugin found, or a federated copy and the native one. Owners, terms, data products, subscriptions, lineage and documentation move across, and the merged asset's MRN keeps working as an alias.
// @Tags assets
// @Accept json
// @Produce json
// @Param id path string true "Asset ID to merge away"
// @Param request body MergeRequest true "Asset to merge into"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.Asset
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @ID postAssetsMergeID
// @Router /api/v1/assets/merge/{id} [post]
func (h *Handler) mergeAsset(w http.ResponseWriter, r *http.Request) {
	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Into == "" {
		common.RespondError(w, http.StatusBadRequest, "into is required")
		return
	}
	merged, err := h.assetService.Merge(r.Context(), r.PathValue("id"), req.Into)
	switch {
	case errors.Is(err, asset.ErrAssetNotFound):
		common.RespondError(w, http.StatusNotFound, "Asset not found")
	case errors.Is(err, asset.ErrInvalidInput):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		log.Error().Err(err).Msg("Failed to merge assets")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
	default:
		common.RespondJSON(w, http.StatusOK, merged)
	}
}

// @Summary Get an asset's identity
// @Description The asset's MRN in parts, the MRNs it used to have, and what each pipeline, import, person or Terraform resource says about it.
// @Tags assets
// @Produce json
// @Param id path string true "Asset ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} IdentityResponse
// @Failure 404 {object} common.ErrorResponse
// @ID getAssetsIdentityID
// @Router /api/v1/assets/identity/{id} [get]
func (h *Handler) getIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := h.assetService.Get(ctx, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, asset.ErrAssetNotFound) {
			common.RespondError(w, http.StatusNotFound, "Asset not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	aliases, err := h.assetService.Aliases(ctx, a.ID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	contribs, err := h.assetService.Contributions(ctx, a.ID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if aliases == nil {
		aliases = []string{}
	}
	if contribs == nil {
		contribs = []asset.Contribution{}
	}
	names := map[string]string{}
	for i := range contribs {
		info := asset.ParseWriter(contribs[i].Writer)
		if info.PrincipalType == "user" && info.PrincipalID != "" {
			if _, seen := names[info.PrincipalID]; !seen {
				names[info.PrincipalID] = ""
				if u, err := h.userService.Get(ctx, info.PrincipalID); err == nil && u != nil {
					names[info.PrincipalID] = u.Name
					if names[info.PrincipalID] == "" {
						names[info.PrincipalID] = u.Username
					}
				}
			}
			info.PrincipalName = names[info.PrincipalID]
		}
		contribs[i].WriterInfo = &info
	}
	common.RespondJSON(w, http.StatusOK, IdentityResponse{ID: a.ID, MRN: *a.MRN, Parts: a.MRNParts, Aliases: aliases, Contributions: contribs})
}

// @Summary List zone bindings
// @Description Which zone each host belongs to. Plugins report the hosts they connect to; an admin can pin a binding so a Kafka Connect sink or an OpenLineage event that only names a host lands on the right system's assets.
// @Tags assets
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} asset.ZoneBinding
// @ID getZoneBindings
// @Router /api/v1/zone-bindings [get]
func (h *Handler) listZoneBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.assetService.ListZoneBindings(r.Context())
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if bindings == nil {
		bindings = []asset.ZoneBinding{}
	}
	common.RespondJSON(w, http.StatusOK, bindings)
}

// @Summary Pin a zone binding
// @Description Binds a provider's host, host:port, to a zone. A pinned binding wins over what plugins report.
// @Tags assets
// @Accept json
// @Produce json
// @Param binding body asset.ZoneBinding true "Binding"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} asset.ZoneBinding
// @Failure 400 {object} common.ErrorResponse
// @ID putZoneBindings
// @Router /api/v1/zone-bindings [put]
func (h *Handler) putZoneBinding(w http.ResponseWriter, r *http.Request) {
	var b asset.ZoneBinding
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Provider == "" || b.Host == "" {
		common.RespondError(w, http.StatusBadRequest, "provider and host are required")
		return
	}
	if err := h.assetService.UpsertZoneBindings(r.Context(), []asset.ZoneBinding{b}, "manual"); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	b.Origin = "manual"
	common.RespondJSON(w, http.StatusOK, b)
}

// @Summary Delete a zone binding
// @Tags assets
// @Param provider query string true "Provider"
// @Param host query string true "Host"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 404 {object} common.ErrorResponse
// @ID deleteZoneBindings
// @Router /api/v1/zone-bindings [delete]
func (h *Handler) deleteZoneBinding(w http.ResponseWriter, r *http.Request) {
	provider := mrn.Asset("x", r.URL.Query().Get("provider"), "x").Provider
	if err := h.assetService.DeleteZoneBinding(r.Context(), provider, r.URL.Query().Get("host")); err != nil {
		if errors.Is(err, asset.ErrAssetNotFound) {
			common.RespondError(w, http.StatusNotFound, "Binding not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Resolve an MRN
// @Description Says what an MRN of any kind names: an asset, glossary term, data product, team, user, pipeline or source. Legacy asset MRNs and MRNs a thing used to have resolve too.
// @Tags assets
// @Produce json
// @Param mrn query string true "MRN"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ResolveResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @ID getMRNResolve
// @Router /api/v1/mrn/resolve [get]
func (h *Handler) resolveMRN(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("mrn")
	parsed, err := mrn.ParseMRN(raw)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.resolveAnyKind(r.Context(), parsed)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && resp == nil) {
		common.RespondError(w, http.StatusNotFound, "Nothing has this MRN")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("mrn", raw).Msg("Failed to resolve MRN")
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	common.RespondJSON(w, http.StatusOK, resp)
}

func (h *Handler) resolveAnyKind(ctx context.Context, m mrn.MRN) (*ResolveResponse, error) {
	canonical := m.String()
	if m.Kind == mrn.KindAsset {
		a, err := h.assetService.GetByMRNs(ctx, []string{canonical})
		if err != nil {
			return nil, err
		}
		found := a[canonical]
		if found == nil {
			return nil, nil
		}
		return &ResolveResponse{Kind: string(m.Kind), ID: found.ID, MRN: *found.MRN, Moved: *found.MRN != canonical}, nil
	}
	if h.db == nil {
		return nil, nil
	}
	var id, current string
	var err error
	switch m.Kind {
	case mrn.KindGlossaryTerm:
		err = h.db.QueryRow(ctx, `
			SELECT id::text, mrn FROM glossary_terms WHERE mrn = $1 AND deleted_at IS NULL
			UNION ALL
			SELECT g.id::text, g.mrn FROM mrn_aliases al JOIN glossary_terms g ON g.id::text = al.entity_id
			WHERE al.alias = $1 AND al.kind = 'glossary-term' AND g.deleted_at IS NULL
			LIMIT 1`, canonical).Scan(&id, &current)
	case mrn.KindDataProduct:
		err = h.db.QueryRow(ctx, `
			SELECT id::text, mrn FROM data_products WHERE mrn = $1
			UNION ALL
			SELECT p.id::text, p.mrn FROM mrn_aliases al JOIN data_products p ON p.id::text = al.entity_id
			WHERE al.alias = $1 AND al.kind = 'data-product'
			LIMIT 1`, canonical).Scan(&id, &current)
	case mrn.KindTeam:
		id, current, err = h.matchByName(ctx, `SELECT id::text, name FROM teams`, canonical, func(n string) string { return mrn.Team(n).String() })
	case mrn.KindUser:
		id, current, err = h.matchByName(ctx, `SELECT id::text, username FROM users`, canonical, func(n string) string { return mrn.User(n).String() })
	case mrn.KindPipeline:
		id, current, err = h.matchByName(ctx, `SELECT id::text, name FROM ingestion_schedules`, canonical, func(n string) string { return mrn.Pipeline(n).String() })
	case mrn.KindSource:
		id, current, err = h.matchByName(ctx, `SELECT id::text, name FROM ingestion_schedules WHERE queryable`, canonical, func(n string) string { return mrn.Source(n).String() })
	}
	if err != nil {
		return nil, err
	}
	return &ResolveResponse{Kind: string(m.Kind), ID: id, MRN: current, Moved: current != canonical}, nil
}

// matchByName finds the row whose name builds the MRN, in Go so the match is exactly what the mrn package produces whatever the database's collation.
func (h *Handler) matchByName(ctx context.Context, query, canonical string, build func(string) string) (string, string, error) {
	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return "", "", err
		}
		if build(name) == canonical {
			return id, canonical, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	return "", "", pgx.ErrNoRows
}

// WithIdentity lets the MRN resolver look up kinds other than assets directly.
func (h *Handler) WithIdentity(db *pgxpool.Pool) *Handler {
	h.db = db
	return h
}

func ioNopCloser(b []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(b)) }
