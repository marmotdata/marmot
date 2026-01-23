package subscriptions

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/subscription"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	svc         *subscription.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(
	svc *subscription.Service,
	userService user.Service,
	authService auth.Service,
	cfg *config.Config,
) *Handler {
	return &Handler{
		svc:         svc,
		userService: userService,
		authService: authService,
		config:      cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/subscriptions",
			Method:  http.MethodGet,
			Handler: h.getSubscription,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/subscriptions/list",
			Method:  http.MethodGet,
			Handler: h.listSubscriptions,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/subscriptions",
			Method:  http.MethodPost,
			Handler: h.createSubscription,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/subscriptions/{id}",
			Method:  http.MethodPut,
			Handler: h.updateSubscription,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/subscriptions/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteSubscription,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
	}
}
