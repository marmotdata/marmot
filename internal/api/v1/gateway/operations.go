package gateway

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/gateway"
	"github.com/marmotdata/marmot/internal/plugin"
)

type OpenSessionRequest struct {
	Purpose string `json:"purpose,omitempty"`
} // @name GatewayOpenSessionRequest

type OpenSessionResponse struct {
	Session *gateway.Session      `json:"session"`
	Targets []*SessionTargetEntry `json:"targets"`
} // @name GatewayOpenSessionResponse

type SessionTargetEntry struct {
	Name     string   `json:"name"`
	PluginID string   `json:"plugin_id"`
	Modes    []string `json:"modes"`
} // @name GatewaySessionTargetEntry

type TargetStatusResponse struct {
	Instances []plugin.InstanceStatus `json:"instances"`
} // @name GatewayTargetStatusResponse

type RevokeGrantRequest struct {
	Reason string `json:"reason,omitempty"`
} // @name GatewayRevokeGrantRequest

// respondServiceError maps gateway service errors onto HTTP statuses.
func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gateway.ErrInvalidInput):
		common.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, gateway.ErrTargetNotFound), errors.Is(err, gateway.ErrGrantNotFound), errors.Is(err, gateway.ErrSessionNotFound):
		common.RespondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, gateway.ErrDenied), errors.Is(err, gateway.ErrNotSessionOwner):
		common.RespondError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, gateway.ErrSessionClosed), errors.Is(err, gateway.ErrTargetDisabled), errors.Is(err, gateway.ErrQueryUnsupported):
		common.RespondError(w, http.StatusConflict, err.Error())
	default:
		common.RespondError(w, http.StatusInternalServerError, err.Error())
	}
}

func principalID(r *http.Request) string {
	if p, ok := common.PrincipalFromContext(r.Context()); ok {
		return p.ID()
	}
	return ""
}


// listTargets lists query targets.
//
// @Summary  List query targets
// @Tags     gateway
// @Produce  json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {array} gateway.Target
// @ID getGatewayTargets
// @Router   /api/v1/gateway/targets [get]
func (h *Handler) listTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.gatewayService.ListTargets(r.Context())
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, targets)
}



// targetStatus reports the long-running plugin instances behind targets.
//
// @Summary  Query gateway instance status
// @Tags     gateway
// @Produce  json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} TargetStatusResponse
// @ID getGatewayTargetsStatus
// @Router   /api/v1/gateway/targets/status [get]
func (h *Handler) targetStatus(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, TargetStatusResponse{Instances: h.gatewayService.InstanceStatus()})
}

// createGrant grants a principal query access to matching resources.
//
// @Summary  Create grant
// @Tags     gateway
// @Accept   json
// @Produce  json
// @Param    request body gateway.CreateGrantInput true "Grant"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  201 {object} gateway.Grant
// @Failure  400 {object} common.ErrorResponse
// @ID postGatewayGrants
// @Router   /api/v1/gateway/grants [post]
func (h *Handler) createGrant(w http.ResponseWriter, r *http.Request) {
	var input gateway.CreateGrantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	grant, err := h.gatewayService.CreateGrant(r.Context(), input, principalID(r))
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusCreated, grant)
}

// listGrants lists grants, optionally for one principal.
//
// @Summary  List grants
// @Tags     gateway
// @Produce  json
// @Param    principal_type query string false "Principal type"
// @Param    principal_id query string false "Principal ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {array} gateway.Grant
// @ID getGatewayGrants
// @Router   /api/v1/gateway/grants [get]
func (h *Handler) listGrants(w http.ResponseWriter, r *http.Request) {
	grants, err := h.gatewayService.ListGrants(r.Context(), r.URL.Query().Get("principal_type"), r.URL.Query().Get("principal_id"))
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, grants)
}

// revokeGrant revokes a grant immediately.
//
// @Summary  Revoke grant
// @Tags     gateway
// @Accept   json
// @Param    id path string true "Grant ID"
// @Param    request body RevokeGrantRequest false "Revocation reason"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  204
// @Failure  404 {object} common.ErrorResponse
// @ID deleteGatewayGrant
// @Router   /api/v1/gateway/grants/{id} [delete]
func (h *Handler) revokeGrant(w http.ResponseWriter, r *http.Request) {
	var req RevokeGrantRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.gatewayService.RevokeGrant(r.Context(), r.PathValue("id"), principalID(r), req.Reason); err != nil {
		respondServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// openSession opens a gateway session for the calling principal.
//
// @Summary  Open session
// @Tags     gateway
// @Accept   json
// @Produce  json
// @Param    request body OpenSessionRequest false "Session purpose"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  201 {object} OpenSessionResponse
// @ID postGatewaySessions
// @Router   /api/v1/gateway/sessions [post]
func (h *Handler) openSession(w http.ResponseWriter, r *http.Request) {
	var req OpenSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	p, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "no authenticated principal")
		return
	}

	session, err := h.gatewayService.OpenSession(r.Context(), p, req.Purpose)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	targets, err := h.gatewayService.ListTargets(r.Context())
	if err != nil {
		respondServiceError(w, err)
		return
	}
	entries := make([]*SessionTargetEntry, 0, len(targets))
	for _, t := range targets {
		if !t.Enabled {
			continue
		}
		entries = append(entries, &SessionTargetEntry{Name: t.Name, PluginID: t.PluginID, Modes: t.Modes})
	}

	common.RespondJSON(w, http.StatusCreated, OpenSessionResponse{Session: session, Targets: entries})
}

// listSessions lists gateway sessions.
//
// @Summary  List sessions
// @Tags     gateway
// @Produce  json
// @Param    limit query int false "Limit"
// @Param    offset query int false "Offset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {array} gateway.Session
// @ID getGatewaySessions
// @Router   /api/v1/gateway/sessions [get]
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	limit := common.ParseLimit(r.URL.Query().Get("limit"), 50, 500)
	offset := common.ParseOffset(r.URL.Query().Get("offset"))

	sessions, err := h.gatewayService.ListSessions(r.Context(), limit, offset)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, sessions)
}

// revokeSession revokes a session; owners revoke their own and gateway
// managers revoke any.
//
// @Summary  Revoke session
// @Tags     gateway
// @Param    id path string true "Session ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  204
// @Failure  403 {object} common.ErrorResponse
// @Failure  404 {object} common.ErrorResponse
// @ID deleteGatewaySession
// @Router   /api/v1/gateway/sessions/{id} [delete]
func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	p, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "no authenticated principal")
		return
	}

	if err := h.gatewayService.RevokeSession(r.Context(), r.PathValue("id"), p); err != nil {
		respondServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// query runs a statement through the gateway: policy-checked, audited and
// returned with fused catalogue context.
//
// @Summary  Execute query
// @Tags     gateway
// @Accept   json
// @Produce  json
// @Param    request body gateway.QueryInput true "Query"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {object} gateway.QueryResult
// @Failure  400 {object} common.ErrorResponse
// @Failure  403 {object} common.ErrorResponse
// @Failure  409 {object} common.ErrorResponse
// @ID postGatewayQuery
// @Router   /api/v1/gateway/query [post]
func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	var input gateway.QueryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, ok := common.PrincipalFromContext(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "no authenticated principal")
		return
	}

	result, err := h.gatewayService.Query(r.Context(), p, input)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, result)
}

// listAudit lists query audit entries.
//
// @Summary  List query audit log
// @Tags     gateway
// @Produce  json
// @Param    principal_id query string false "Principal ID"
// @Param    session_id query string false "Session ID"
// @Param    target query string false "Target name"
// @Param    decision query string false "Decision (allowed|denied)"
// @Param    limit query int false "Limit"
// @Param    offset query int false "Offset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success  200 {array} gateway.AuditEntry
// @ID getGatewayAudit
// @Router   /api/v1/gateway/audit [get]
func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := gateway.AuditFilter{
		PrincipalID: q.Get("principal_id"),
		SessionID:   q.Get("session_id"),
		TargetName:  q.Get("target"),
		Decision:    q.Get("decision"),
		Limit:       common.ParseLimit(q.Get("limit"), 50, 500),
		Offset:      common.ParseOffset(q.Get("offset")),
	}

	entries, err := h.gatewayService.ListAudit(r.Context(), filter)
	if err != nil {
		respondServiceError(w, err)
		return
	}
	common.RespondJSON(w, http.StatusOK, entries)
}
