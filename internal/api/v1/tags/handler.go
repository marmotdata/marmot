package tags

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/tag"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/pkg/config"
)

type Handler struct {
	tagService  tag.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(
	tagService tag.Service,
	userService user.Service,
	authService auth.Service,
	config *config.Config,
) *Handler {
	return &Handler{
		tagService:  tagService,
		userService: userService,
		authService: authService,
		config:      config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:   "/api/v1/tags",
			Method: http.MethodGet,
			Handler: h.ListTags,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:   "/api/v1/tags/{id}",
			Method: http.MethodGet,
			Handler: h.GetTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:   "/api/v1/tags",
			Method: http.MethodPost,
			Handler: h.CreateTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:   "/api/v1/tags/{id}",
			Method: http.MethodPut,
			Handler: h.UpdateTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
		{
			Path:   "/api/v1/tags/{id}",
			Method: http.MethodDelete,
			Handler: h.DeleteTag,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "assets", "manage"),
			},
		},
	}
}
