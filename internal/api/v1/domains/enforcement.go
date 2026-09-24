package domains

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

type EnforcementRequest struct {
	Write bool `json:"write"`
	// Confirm is the hash of the plan the caller reviewed. Required to turn
	// enforcement on; ignored when turning it off.
	Confirm string `json:"confirm"`
}

// @Summary Get domain write enforcement state
// @Description Whether writes are scoped by domain. Off, native RBAC alone decides.
// @Tags domains
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.EnforcementState
// @ID getDomainEnforcement
// @Router /api/v1/domains/enforcement [get]
func (h *Handler) enforcement(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.Enforcement(r.Context())
	if err != nil {
		respondError(w, err, "get domain enforcement")
		return
	}
	common.RespondJSON(w, http.StatusOK, state)
}

// @Summary Plan domain write enforcement
// @Description Who would lose write access, and where, once writes are scoped by domain, and which pipelines have assets outside their domain. Its hash confirms the activation. Global scope only.
// @Tags domains
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.EnforcementPlan
// @Failure 403 {object} ErrorResponse
// @ID getDomainEnforcementPlan
// @Router /api/v1/domains/enforcement/plan [get]
func (h *Handler) enforcementPlan(w http.ResponseWriter, r *http.Request) {
	principal, _ := common.PrincipalFromContext(r.Context())
	plan, err := h.service.EnforcementPlan(r.Context(), principal)
	if err != nil {
		respondError(w, err, "plan domain enforcement")
		return
	}
	common.RespondJSON(w, http.StatusOK, plan)
}

// @Summary Turn domain write enforcement on or off
// @Description Turning it on needs the hash of the current plan; a plan that changed since it was reviewed is refused with plan_changed. Both directions are audited. Global scope only.
// @Tags domains
// @Accept json
// @Produce json
// @Param request body EnforcementRequest true "Target state"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} domain.EnforcementState
// @Failure 403 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @ID setDomainEnforcement
// @Router /api/v1/domains/enforcement [post]
func (h *Handler) setEnforcement(w http.ResponseWriter, r *http.Request) {
	var req EnforcementRequest
	if !decode(w, r, &req) {
		return
	}
	principal, _ := common.PrincipalFromContext(r.Context())
	state, err := h.service.SetWriteEnforcement(r.Context(), principal, req.Write, req.Confirm)
	if err != nil {
		respondError(w, err, "set domain enforcement")
		return
	}
	common.RespondJSON(w, http.StatusOK, state)
}
