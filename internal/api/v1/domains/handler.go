// Package domains exposes the fork-only domain tree over HTTP. It is only
// registered when domains.enabled is set; see DOMAINS.md.
package domains

import (
	"context"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

// Guard is the part of domain.Guard the handlers use: moves between domains
// go through it so they are scoped and audited.
type Guard interface {
	Transfer(ctx context.Context, kind domain.Kind, ids []string, domainID string) error
	AssignPipeline(ctx context.Context, scheduleID, domainID string, moveAssets bool) (*domain.PipelineMoveResult, error)
	AuditMove(ctx context.Context, domainID string, from, to *string) error
	AuditLog(ctx context.Context, entityKind, entityID string) ([]domain.AuditEntry, error)
}

type Handler struct {
	service     domain.Service
	guard       Guard
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(service domain.Service, guard Guard, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{service: service, guard: guard, userService: userService, authService: authService, config: config}
}

func (h *Handler) Routes() []common.Route {
	view := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "domains", "view"),
	}
	manage := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "domains", "manage"),
	}
	return []common.Route{
		{Path: "/api/v1/domains", Method: http.MethodGet, Handler: h.list, Middleware: view},
		{Path: "/api/v1/domains", Method: http.MethodPost, Handler: h.create, Middleware: view},
		{Path: "/api/v1/domains/{id}", Method: http.MethodGet, Handler: h.get, Middleware: view},
		{Path: "/api/v1/domains/{id}", Method: http.MethodPut, Handler: h.update, Middleware: view},
		{Path: "/api/v1/domains/{id}", Method: http.MethodDelete, Handler: h.remove, Middleware: view},
		{Path: "/api/v1/domains/{id}/tree", Method: http.MethodGet, Handler: h.tree, Middleware: view},
		{Path: "/api/v1/domains/{id}/move", Method: http.MethodPost, Handler: h.move, Middleware: view},
		{Path: "/api/v1/domains/{id}/members", Method: http.MethodPut, Handler: h.assign, Middleware: view},
		{Path: "/api/v1/domains/of/{kind}/{id}", Method: http.MethodGet, Handler: h.domainOf, Middleware: view},
		{Path: "/api/v1/domains/import", Method: http.MethodPost, Handler: h.importMemberships, Middleware: manage},
		{Path: "/api/v1/domains/capabilities", Method: http.MethodGet, Handler: h.capabilities, Middleware: view},
		{Path: "/api/v1/domains/{id}/roles", Method: http.MethodGet, Handler: h.listRoles, Middleware: view},
		{Path: "/api/v1/domains/{id}/roles", Method: http.MethodPost, Handler: h.grantRole, Middleware: view},
		{Path: "/api/v1/domains/{id}/roles", Method: http.MethodDelete, Handler: h.revokeRole, Middleware: view},
		{Path: "/api/v1/domains/pipelines/{scheduleId}/assignment", Method: http.MethodGet, Handler: h.pipelineAssignment, Middleware: view},
		{Path: "/api/v1/domains/pipelines/{scheduleId}/assignment", Method: http.MethodPut, Handler: h.assignPipeline, Middleware: view},
		{Path: "/api/v1/domains/audit/{kind}/{id}", Method: http.MethodGet, Handler: h.auditLog, Middleware: manage},
	}
}
