package admin

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	reindexer   *search.Reindexer
	userService user.Service
	authService auth.Service
	config      *config.Config
	extras      *Extras
}

func NewHandler(
	reindexer *search.Reindexer,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		reindexer:   reindexer,
		userService: userService,
		authService: authService,
		config:      config,
	}
}

func (h *Handler) Routes() []common.Route {
	authMiddleware := []func(http.HandlerFunc) http.HandlerFunc{
		common.WithAuth(h.userService, h.authService, h.config),
		common.RequirePermission(h.userService, "users", "manage"),
	}

	return append(h.extraRoutes(), []common.Route{
		{
			Path:       "/api/v1/admin/search/reindex",
			Method:     http.MethodPost,
			Handler:    h.startReindex,
			Middleware: authMiddleware,
		},
		{
			Path:       "/api/v1/admin/search/reindex",
			Method:     http.MethodGet,
			Handler:    h.getReindexStatus,
			Middleware: authMiddleware,
		},
	}...)
}
