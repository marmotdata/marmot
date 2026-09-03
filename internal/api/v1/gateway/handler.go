package gateway

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/gateway"
	"github.com/marmotdata/marmot/internal/core/user"

	authcore "github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	gatewayService gateway.Service
	userService    user.Service
	authService    authcore.Service
	config         *config.Config
}

func NewHandler(gatewayService gateway.Service, userService user.Service, authService authcore.Service, config *config.Config) *Handler {
	return &Handler{
		gatewayService: gatewayService,
		userService:    userService,
		authService:    authService,
		config:         config,
	}
}

func (h *Handler) Routes() []common.Route {
	auth := common.WithAuth(h.userService, h.authService, h.config)
	view := common.RequirePermission(h.userService, "gateway", "view")
	manage := common.RequirePermission(h.userService, "gateway", "manage")
	query := common.RequirePermission(h.userService, "gateway", "query")
	chain := func(mws ...func(http.HandlerFunc) http.HandlerFunc) []func(http.HandlerFunc) http.HandlerFunc {
		return mws
	}

	return []common.Route{
		{Path: "/api/v1/gateway/targets", Method: http.MethodGet, Handler: h.listTargets, Middleware: chain(auth, view)},
		{Path: "/api/v1/gateway/targets/status", Method: http.MethodGet, Handler: h.targetStatus, Middleware: chain(auth, view)},

		{Path: "/api/v1/gateway/grants", Method: http.MethodPost, Handler: h.createGrant, Middleware: chain(auth, manage)},
		{Path: "/api/v1/gateway/grants", Method: http.MethodGet, Handler: h.listGrants, Middleware: chain(auth, view)},
		{Path: "/api/v1/gateway/grants/{id}", Method: http.MethodDelete, Handler: h.revokeGrant, Middleware: chain(auth, manage)},

		{Path: "/api/v1/gateway/sessions", Method: http.MethodPost, Handler: h.openSession, Middleware: chain(auth, query)},
		{Path: "/api/v1/gateway/sessions", Method: http.MethodGet, Handler: h.listSessions, Middleware: chain(auth, view)},
		{Path: "/api/v1/gateway/sessions/{id}", Method: http.MethodDelete, Handler: h.revokeSession, Middleware: chain(auth, query)},

		{Path: "/api/v1/gateway/query", Method: http.MethodPost, Handler: h.query, Middleware: chain(auth, query)},

		{Path: "/api/v1/gateway/audit", Method: http.MethodGet, Handler: h.listAudit, Middleware: chain(auth, view)},
	}
}
