// Package domains exposes the fork-only domain tree over HTTP. It is only
// registered when domains.enabled is set; see DOMAINS.md.
package domains

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	service     domain.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(service domain.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{service: service, userService: userService, authService: authService, config: config}
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
		{Path: "/api/v1/domains", Method: http.MethodPost, Handler: h.create, Middleware: manage},
		{Path: "/api/v1/domains/{id}", Method: http.MethodGet, Handler: h.get, Middleware: view},
		{Path: "/api/v1/domains/{id}", Method: http.MethodPut, Handler: h.update, Middleware: manage},
		{Path: "/api/v1/domains/{id}", Method: http.MethodDelete, Handler: h.remove, Middleware: manage},
		{Path: "/api/v1/domains/{id}/tree", Method: http.MethodGet, Handler: h.tree, Middleware: view},
		{Path: "/api/v1/domains/{id}/move", Method: http.MethodPost, Handler: h.move, Middleware: manage},
		{Path: "/api/v1/domains/{id}/members", Method: http.MethodPut, Handler: h.assign, Middleware: manage},
		{Path: "/api/v1/domains/of/{kind}/{id}", Method: http.MethodGet, Handler: h.domainOf, Middleware: view},
		{Path: "/api/v1/domains/import", Method: http.MethodPost, Handler: h.importMemberships, Middleware: manage},
	}
}
