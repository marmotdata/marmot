package memory

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/memory"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	memoryService memory.Service
	userService   user.Service
	authService   auth.Service
	config        *config.Config
}

func NewHandler(memoryService memory.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{
		memoryService: memoryService,
		userService:   userService,
		authService:   authService,
		config:        config,
	}
}

func (h *Handler) Routes() []common.Route {
	read := func(handler http.HandlerFunc, method, path string) common.Route {
		return common.Route{
			Path:    path,
			Method:  method,
			Handler: handler,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "view"),
			},
		}
	}
	write := func(handler http.HandlerFunc, method, path string) common.Route {
		return common.Route{
			Path:    path,
			Method:  method,
			Handler: handler,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "memory", "write"),
				common.WithRateLimit(h.config, 120, 60),
			},
		}
	}

	return []common.Route{
		read(h.list, http.MethodGet, "/api/v1/memory/{entityType}/{entityId}"),
		write(h.remember, http.MethodPost, "/api/v1/memory/{entityType}/{entityId}"),
		read(h.search, http.MethodGet, "/api/v1/memory/{entityType}/{entityId}/search"),
		read(h.get, http.MethodGet, "/api/v1/memory/{entityType}/{entityId}/{memoryId}"),
		write(h.update, http.MethodPut, "/api/v1/memory/{entityType}/{entityId}/{memoryId}"),
		write(h.forget, http.MethodDelete, "/api/v1/memory/{entityType}/{entityId}/{memoryId}"),
	}
}
