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

type PipelineAssignmentRequest struct {
	DomainID string `json:"domain_id"`
	// MoveAssets also moves the assets this pipeline ingested that are still
	// in its previous domain. Never implied: the client must ask for it.
	MoveAssets bool `json:"move_assets"`
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
		respondCode(w, http.StatusBadRequest, "invalid_input", "Invalid request body")
		return false
	}
	return true
}

// Restricted domains need read filtering on every channel first (delivery 3);
// accepting the flag earlier would promise protection that does not exist.
func rejectRestricted(w http.ResponseWriter, restricted *bool) bool {
	if restricted != nil && *restricted {
		respondCode(w, http.StatusBadRequest, "restricted_unsupported", "Restricted domains are not supported yet")
		return true
	}
	return false
}

// ErrorResponse carries a stable code next to the English message, so clients
// can translate it.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
} // @name DomainErrorResponse

func respondCode(w http.ResponseWriter, status int, code, message string) {
	common.RespondJSON(w, status, ErrorResponse{Error: message, Code: code})
}

var errorCodes = []struct {
	err    error
	status int
	code   string
}{
	{domain.ErrNotFound, http.StatusNotFound, "not_found"},
	{domain.ErrEntityNotFound, http.StatusNotFound, "entity_not_found"},
	{domain.ErrInvalidInput, http.StatusBadRequest, "invalid_input"},
	{domain.ErrCycle, http.StatusBadRequest, "cycle"},
	{domain.ErrTooDeep, http.StatusBadRequest, "too_deep"},
	{domain.ErrNameConflict, http.StatusConflict, "name_conflict"},
	{domain.ErrHasChildren, http.StatusConflict, "has_children"},
	{domain.ErrNotEmpty, http.StatusConflict, "not_empty"},
	{domain.ErrProtected, http.StatusConflict, "protected"},
	{domain.ErrDuplicate, http.StatusConflict, "duplicate"},
	{domain.ErrForbidden, http.StatusForbidden, "forbidden"},
}

func respondError(w http.ResponseWriter, err error, action string) {
	for _, e := range errorCodes {
		if errors.Is(err, e.err) {
			respondCode(w, e.status, e.code, err.Error())
			return
		}
	}
	log.Error().Err(err).Msg("Failed to " + action)
	common.RespondError(w, http.StatusInternalServerError, "Internal server error")
}

// @Summary List domains
// @Description Lists the root domains, or the direct children of parent_id.
// @Tags domains
// @Produce json
// @Param parent_id query string false "Parent domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} domain.Domain
// @Failure 404 {object} ErrorResponse
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID createDomain
// @Router /api/v1/domains [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !decode(w, r, &req) || rejectRestricted(w, req.Restricted) {
		return
	}
	parent := ""
	if req.ParentID != nil {
		parent = *req.ParentID
	}
	if !h.requireAdmin(w, r, parent) {
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
// @Failure 404 {object} ErrorResponse
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID updateDomain
// @Router /api/v1/domains/{id} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if !decode(w, r, &req) || rejectRestricted(w, req.Restricted) {
		return
	}
	if !h.requireAdmin(w, r, r.PathValue("id")) {
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
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID deleteDomain
// @Router /api/v1/domains/{id} [delete]
func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOfParent(w, r, r.PathValue("id")) {
		return
	}
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
// @Failure 404 {object} ErrorResponse
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
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID moveDomain
// @Router /api/v1/domains/{id}/move [post]
func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	var req MoveRequest
	if !decode(w, r, &req) {
		return
	}
	target := ""
	if req.ParentID != nil {
		target = *req.ParentID
	}
	if !h.requireAdminOfParent(w, r, r.PathValue("id")) || !h.requireAdmin(w, r, target) {
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
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID assignDomainMembers
// @Router /api/v1/domains/{id}/members [put]
func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
	var req MembersRequest
	if !decode(w, r, &req) {
		return
	}
	perm, ok := memberPermission[req.Kind]
	if !ok {
		respondCode(w, http.StatusBadRequest, "invalid_input", "Unknown entity kind")
		return
	}
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok || !principal.HasPermission(perm.resource, perm.action) {
		respondCode(w, http.StatusForbidden, "forbidden", "Insufficient permissions to change these entities")
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
// @Failure 400 {object} ErrorResponse
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

// @Summary Import domain memberships from metadata
// @Description Maps the values at a metadata path (such as metadata.dgu.domain) to domains. Only entities still in Unassigned are assigned. A dry run unless apply is true. Global administrators only.
// @Tags domains
// @Accept json
// @Produce json
// @Param import body domain.ImportInput true "Source path, value-to-domain mapping and apply flag"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.ImportReport
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID importDomainMemberships
// @Router /api/v1/domains/import [post]
func (h *Handler) importMemberships(w http.ResponseWriter, r *http.Request) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok || !principal.IsAdmin() {
		respondCode(w, http.StatusForbidden, "forbidden", "Importing domain memberships requires a global administrator")
		return
	}
	var in domain.ImportInput
	if !decode(w, r, &in) {
		return
	}
	report, err := h.service.Import(r.Context(), in)
	if err != nil {
		respondError(w, err, "import domain memberships")
		return
	}
	common.RespondJSON(w, http.StatusOK, report)
}

func requirePermissions(w http.ResponseWriter, r *http.Request, perms ...[2]string) bool {
	principal, ok := common.PrincipalFromContext(r.Context())
	for _, p := range perms {
		if !ok || !principal.HasPermission(p[0], p[1]) {
			respondCode(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
			return false
		}
	}
	return true
}

// @Summary Get a pipeline's domain
// @Description The domain new assets from this pipeline go to, and how many assets it ingested are still in that domain.
// @Tags domains
// @Produce json
// @Param scheduleId path string true "Ingestion schedule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.PipelineAssignment
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID getPipelineDomain
// @Router /api/v1/domains/pipelines/{scheduleId}/assignment [get]
func (h *Handler) pipelineAssignment(w http.ResponseWriter, r *http.Request) {
	if !requirePermissions(w, r, [2]string{"ingestion", "view"}) {
		return
	}
	assignment, err := h.service.PipelineAssignment(r.Context(), r.PathValue("scheduleId"))
	if err != nil {
		respondError(w, err, "get pipeline domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, assignment)
}

// @Summary Change a pipeline's domain
// @Description Sets the domain for the pipeline's new assets. With move_assets, the assets it already ingested that are still in its previous domain move too, in the same transaction; assets placed in another domain stay. Moving assets also requires assets:manage.
// @Tags domains
// @Accept json
// @Produce json
// @Param scheduleId path string true "Ingestion schedule ID"
// @Param assignment body PipelineAssignmentRequest true "Target domain and whether to move the ingested assets"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.PipelineMoveResult
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID setPipelineDomain
// @Router /api/v1/domains/pipelines/{scheduleId}/assignment [put]
func (h *Handler) assignPipeline(w http.ResponseWriter, r *http.Request) {
	var req PipelineAssignmentRequest
	if !decode(w, r, &req) {
		return
	}
	perms := [][2]string{{"ingestion", "manage"}}
	if req.MoveAssets {
		perms = append(perms, [2]string{"assets", "manage"})
	}
	if !requirePermissions(w, r, perms...) {
		return
	}
	if req.DomainID == "" {
		req.DomainID = domain.UnassignedID
	}
	result, err := h.service.AssignPipeline(r.Context(), r.PathValue("scheduleId"), req.DomainID, req.MoveAssets)
	if err != nil {
		respondError(w, err, "change pipeline domain")
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// mayAdminister reports whether the caller administers domainID: through the
// global domains:manage permission, or as domain_admin of it or an ancestor.
// The root ("") is only administered globally.
func (h *Handler) mayAdminister(r *http.Request, domainID string) (bool, error) {
	principal, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		return false, nil
	}
	if principal.HasPermission("domains", "manage") {
		return true, nil
	}
	if domainID == "" {
		return false, nil
	}
	d, err := h.service.Get(r.Context(), domainID)
	if err != nil {
		return false, err
	}
	scope, err := h.service.Scope(r.Context(), principal)
	if err != nil {
		return false, err
	}
	return scope.Can(domain.ActionAdmin, d.Path), nil
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request, domainID string) bool {
	allowed, err := h.mayAdminister(r, domainID)
	if err != nil {
		respondError(w, err, "check domain permissions")
		return false
	}
	if !allowed {
		respondCode(w, http.StatusForbidden, "forbidden", "Administering this domain is not allowed")
		return false
	}
	return true
}

// Deleting or moving a domain changes its parent's contents, so it is decided
// at the parent: a domain admin cannot remove or relocate its own root.
func (h *Handler) requireAdminOfParent(w http.ResponseWriter, r *http.Request, domainID string) bool {
	d, err := h.service.Get(r.Context(), domainID)
	if err != nil {
		respondError(w, err, "check domain permissions")
		return false
	}
	parent := ""
	if d.ParentID != nil {
		parent = *d.ParentID
	}
	return h.requireAdmin(w, r, parent)
}

// @Summary List a domain's role assignments
// @Description Includes the assignments inherited from its ancestors, marked inherited.
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {array} domain.RoleAssignment
// @Failure 404 {object} ErrorResponse
// @ID listDomainRoles
// @Router /api/v1/domains/{id}/roles [get]
func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.Roles(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, err, "list domain roles")
		return
	}
	common.RespondJSON(w, http.StatusOK, roles)
}

// @Summary Grant a role on a domain
// @Description Needs domain_admin on the domain or an ancestor, or global scope. The role applies to the whole subtree.
// @Tags domains
// @Accept json
// @Produce json
// @Param id path string true "Domain ID"
// @Param grant body domain.GrantInput true "Subject and role"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} domain.RoleAssignment
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID grantDomainRole
// @Router /api/v1/domains/{id}/roles [post]
func (h *Handler) grantRole(w http.ResponseWriter, r *http.Request) {
	var in domain.GrantInput
	if !decode(w, r, &in) {
		return
	}
	principal, _ := common.PrincipalFromContext(r.Context())
	ra, err := h.service.GrantRole(r.Context(), principal, r.PathValue("id"), in)
	if err != nil {
		respondError(w, err, "grant domain role")
		return
	}
	common.RespondJSON(w, http.StatusCreated, ra)
}

// @Summary Revoke a role on a domain
// @Tags domains
// @Produce json
// @Param id path string true "Domain ID"
// @Param assignment_id query string true "Role assignment ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID revokeDomainRole
// @Router /api/v1/domains/{id}/roles [delete]
func (h *Handler) revokeRole(w http.ResponseWriter, r *http.Request) {
	principal, _ := common.PrincipalFromContext(r.Context())
	if err := h.service.RevokeRole(r.Context(), principal, r.PathValue("id"), r.URL.Query().Get("assignment_id")); err != nil {
		respondError(w, err, "revoke domain role")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Role revoked"})
}

type Capabilities struct {
	DomainID string `json:"domain_id"`
	// Write: edit entities in the domain (enforced from delivery 2 on).
	Write bool `json:"write"`
	// Admin: manage its subdomains and role assignments.
	Admin bool `json:"admin"`
}

// @Summary What the caller may do in a domain
// @Description Informative, for hiding actions in a UI; the server checks every operation again. Pass domain_id, or kind and id of an entity to use its domain.
// @Tags domains
// @Produce json
// @Param domain_id query string false "Domain ID"
// @Param kind query string false "Entity kind" Enums(asset, data_product, glossary_term, ingestion_schedule)
// @Param id query string false "Entity ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} Capabilities
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @ID getDomainCapabilities
// @Router /api/v1/domains/capabilities [get]
func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	domainID := q.Get("domain_id")
	if domainID == "" {
		if q.Get("kind") == "" || q.Get("id") == "" {
			respondCode(w, http.StatusBadRequest, "invalid_input", "Pass domain_id, or kind and id")
			return
		}
		var err error
		if domainID, err = h.service.DomainOf(r.Context(), domain.Kind(q.Get("kind")), q.Get("id")); err != nil {
			respondError(w, err, "resolve entity domain")
			return
		}
	}
	d, err := h.service.Get(r.Context(), domainID)
	if err != nil {
		respondError(w, err, "get domain capabilities")
		return
	}
	principal, _ := common.PrincipalFromContext(r.Context())
	scope, err := h.service.Scope(r.Context(), principal)
	if err != nil {
		respondError(w, err, "get domain capabilities")
		return
	}
	admin, err := h.mayAdminister(r, d.ID)
	if err != nil {
		respondError(w, err, "get domain capabilities")
		return
	}
	common.RespondJSON(w, http.StatusOK, Capabilities{DomainID: d.ID, Write: scope.Can(domain.ActionWrite, d.Path), Admin: admin})
}
