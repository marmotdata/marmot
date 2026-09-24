package domains

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/rs/zerolog/log"
)

const maxBodyBytes = 1 << 20

type CreateRequest struct {
	ParentID    *string        `json:"parent_id"`
	Name        string         `json:"name"`
	Description *string        `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	Tags        []string       `json:"tags"`
	Restricted  *bool          `json:"restricted"`
}

type UpdateRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Metadata    map[string]any `json:"metadata"`
	Tags        []string       `json:"tags"`
	Restricted  *bool          `json:"restricted"`
}

type MoveRequest struct {
	// ParentID is the new parent; null moves the domain to the root.
	ParentID *string `json:"parent_id"`
}

type MembersRequest struct {
	Kind domain.Kind `json:"kind" enums:"asset,data_product,glossary_term,ingestion_schedule"`
	IDs  []string    `json:"ids"`
}

// Assigning an entity to a domain edits that entity, so it also needs the
// entity's own manage permission.
var memberPermission = map[domain.Kind]struct{ resource, action string }{
	domain.KindAsset:             {"assets", "manage"},
	domain.KindDataProduct:       {"assets", "manage"},
	domain.KindGlossaryTerm:      {"glossary", "manage"},
	domain.KindIngestionSchedule: {"ingestion", "manage"},
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(v); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

// Restricted domains need read filtering on every channel first (delivery 3);
// accepting the flag earlier would promise protection that does not exist.
func rejectRestricted(w http.ResponseWriter, restricted *bool) bool {
	if restricted != nil && *restricted {
		common.RespondError(w, http.StatusBadRequest, "Restricted domains are not supported yet")
		return true
	}
	return false
}

func respondError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		common.RespondError(w, http.StatusNotFound, "Domain not found")
	case errors.Is(err, domain.ErrEntityNotFound):
		common.RespondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrCycle), errors.Is(err, domain.ErrTooDeep):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNameConflict), errors.Is(err, domain.ErrHasChildren),
		errors.Is(err, domain.ErrNotEmpty), errors.Is(err, domain.ErrProtected):
		common.RespondError(w, http.StatusConflict, err.Error())
	default:
		log.Error().Err(err).Msg("Failed to " + action)
		common.RespondError(w, http.StatusInternalServerError, "Internal server error")
	}
}

// @Summary List domains
// @Description Lists the root domains, or the direct children of parent_id.
// @Tags domains
// @Produce json
// @Param parent_id query string false "Parent domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} domain.Domain
// @Failure 404 {object} common.ErrorResponse
// @ID listDomains
// @Router /api/v1/domains [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var parent *string
	if p := r.URL.Query().Get("parent_id"); p != "" {
		parent = &p
		if _, err := h.service.Get(r.Context(), p); err != nil {
			respondError(w, err, "list domains")
			return
		}
	}
	domains, err := h.service.Children(r.Context(), parent)
	if err != nil {
		respondError(w, err, "list domains")
		return
	}
	common.RespondJSON(w, http.StatusOK, domains)
}

// @Summary Create a domain
// @Tags domains
// @Accept json
// @Produce json
// @Param domain body CreateRequest true "Domain; parent_id null creates a root"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} domain.Domain
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID createDomain
// @Router /api/v1/domains [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !decode(w, r, &req) || rejectRestricted(w, req.Restricted) {
		return
	}
	in := domain.CreateInput{
		ParentID:    req.ParentID,
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
		Tags:        req.Tags,
	}
	if principal, ok := common.PrincipalFromContext(r.Context()); ok {
		in.CreatedBy = principal.ID()
	}
	d, err := h.service.Create(r.Context(), in)
	if err != nil {
		respondError(w, err, "create domain")
		return
	}
	common.RespondJSON(w, http.StatusCreated, d)
}

// @Summary Get a domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.Domain
// @Failure 404 {object} common.ErrorResponse
// @ID getDomain
// @Router /api/v1/domains/{id} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.service.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, err, "get domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, d)
}

// @Summary Update a domain
// @Description Changes only the fields present in the body.
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param domain body UpdateRequest true "Fields to change"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.Domain
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID updateDomain
// @Router /api/v1/domains/{id} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if !decode(w, r, &req) || rejectRestricted(w, req.Restricted) {
		return
	}
	d, err := h.service.Update(r.Context(), r.PathValue("id"), domain.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
		Tags:        req.Tags,
	})
	if err != nil {
		respondError(w, err, "update domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, d)
}

// @Summary Delete a domain
// @Description Only an empty domain without subdomains can be deleted.
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID deleteDomain
// @Router /api/v1/domains/{id} [delete]
func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("id")); err != nil {
		respondError(w, err, "delete domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Domain deleted successfully"})
}

// @Summary Get a domain subtree
// @Description Returns the domain and all its descendants, shallowest first.
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} domain.Domain
// @Failure 404 {object} common.ErrorResponse
// @ID getDomainTree
// @Router /api/v1/domains/{id}/tree [get]
func (h *Handler) tree(w http.ResponseWriter, r *http.Request) {
	domains, err := h.service.Subtree(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, err, "get domain tree")
		return
	}
	common.RespondJSON(w, http.StatusOK, domains)
}

// @Summary Move a domain
// @Description Moves the domain and its whole subtree under a new parent.
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param move body MoveRequest true "New parent"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.Domain
// @Failure 400 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 409 {object} common.ErrorResponse
// @ID moveDomain
// @Router /api/v1/domains/{id}/move [post]
func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	var req MoveRequest
	if !decode(w, r, &req) {
		return
	}
	d, err := h.service.Move(r.Context(), r.PathValue("id"), req.ParentID)
	if err != nil {
		respondError(w, err, "move domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, d)
}

// @Summary Assign entities to a domain
// @Description Sets the owning domain of every listed entity, all or none. Requires the entity kind's manage permission as well.
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param members body MembersRequest true "Entities to assign"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 400 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @ID assignDomainMembers
// @Router /api/v1/domains/{id}/members [put]
func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	var req MembersRequest
	if !decode(w, r, &req) {
		return
	}
	perm, ok := memberPermission[req.Kind]
	if !ok {
		common.RespondError(w, http.StatusBadRequest, "Unknown entity kind")
		return
	}
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok || !principal.HasPermission(perm.resource, perm.action) {
		common.RespondError(w, http.StatusForbidden, "Insufficient permissions to change these entities")
		return
	}
	if err := h.service.Assign(r.Context(), req.Kind, req.IDs, r.PathValue("id")); err != nil {
		respondError(w, err, "assign domain members")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Entities assigned"})
}

// @Summary Get an entity's domain
// @Description Entities without an explicit domain belong to the Unassigned domain.
// @Tags domains
// @Produce json
// @Param kind path string true "Entity kind" Enums(asset, data_product, glossary_term, ingestion_schedule)
// @Param id path string true "Entity ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.Domain
// @Failure 400 {object} common.ErrorResponse
// @ID getEntityDomain
// @Router /api/v1/domains/of/{kind}/{id} [get]
func (h *Handler) domainOf(w http.ResponseWriter, r *http.Request) {
	id, err := h.service.DomainOf(r.Context(), domain.Kind(r.PathValue("kind")), r.PathValue("id"))
	if err != nil {
		respondError(w, err, "get entity domain")
		return
	}
	d, err := h.service.Get(r.Context(), id)
	if err != nil {
		respondError(w, err, "get entity domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, d)
}
