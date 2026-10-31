package dataquery

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/dataquery"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	queryService dataquery.Service
	userService  user.Service
	authService  auth.Service
	config       *config.Config
}

func NewHandler(queryService dataquery.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{
		queryService: queryService,
		userService:  userService,
		authService:  authService,
		config:       config,
	}
}

func (h *Handler) Routes() []common.Route {
	withAuth := common.WithAuth(h.userService, h.authService, h.config)
	// The service's authorizer admits a caller to data queries and then decides which tables.
	query := h.requireQuery
	view := common.RequirePermission(h.userService, "access", "view")
	// The log and its stats serve both callers reading their own queries and access:view holders reading everyone's, who need not query themselves.
	queryOrLog := h.requireQueryOrLog

	return []common.Route{
		{Path: "/api/v1/query/sources", Method: http.MethodGet, Handler: h.listSources, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, query}},
		{Path: "/api/v1/query/sources/{name}/tables", Method: http.MethodGet, Handler: h.listSourceTables, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, query}},
		{Path: "/api/v1/query/status", Method: http.MethodGet, Handler: h.status, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, view}},
		{Path: "/api/v1/query/stats", Method: http.MethodGet, Handler: h.stats, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog}},
		{Path: "/api/v1/query/sources/{name}/stats", Method: http.MethodGet, Handler: h.sourceStats, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog}},
		{Path: "/api/v1/query/asset-stats", Method: http.MethodGet, Handler: h.assetStats, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog}},

		// Running is not gated on the authorizer's admission: the service refuses a caller who may not query before it looks at the source or plans the statement, and records the refusal, so the caller learns what is missing and the audit sees the attempt.
		{Path: "/api/v1/query/run", Method: http.MethodPost, Handler: h.query, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, common.WithRateLimit(h.config, 120, 60)}},
		{Path: "/api/v1/query/history", Method: http.MethodGet, Handler: h.listQueries, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog}},
		{Path: "/api/v1/query/history/export", Method: http.MethodGet, Handler: h.exportQueries, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog, common.WithRateLimit(h.config, 10, 60)}},
		{Path: "/api/v1/query/history/{id}", Method: http.MethodGet, Handler: h.getQuery, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, queryOrLog}},
		// Cancelling checks inside that the query is the caller's own or that the caller holds access:manage.
		{Path: "/api/v1/query/history/{id}/cancel", Method: http.MethodPost, Handler: h.cancelQuery, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth}},
		// Asking for access is open to any signed-in caller, since the people who need it are exactly those who lack it.
		{Path: "/api/v1/query/access-requests", Method: http.MethodPost, Handler: h.requestAccess, Middleware: []func(http.HandlerFunc) http.HandlerFunc{withAuth, common.WithRateLimit(h.config, 20, 60)}},
	}
}

// requireQueryOrLog admits a caller who may query or may read the query log.
func (h *Handler) requireQueryOrLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if common.HasPermission(r.Context(), "access", "view") {
			next(w, r)
			return
		}
		h.requireQuery(next)(w, r)
	}
}

// requireQuery admits a caller the data query authorizer lets in at all.
func (h *Handler) requireQuery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := common.PrincipalFromContext(r.Context())
		if !ok || !h.queryService.Admits(r.Context(), principal) {
			common.RespondError(w, http.StatusForbidden, "Permission denied")
			return
		}
		next(w, r)
	}
}
