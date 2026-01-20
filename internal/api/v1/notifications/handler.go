package notifications

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/notification"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	notificationService *notification.Service
	userService         user.Service
	authService         auth.Service
	config              *config.Config
}

func NewHandler(
	notificationService *notification.Service,
	userService user.Service,
	authService auth.Service,
	cfg *config.Config,
) *Handler {
	return &Handler{
		notificationService: notificationService,
		userService:         userService,
		authService:         authService,
		config:              cfg,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/notifications",
			Method:  http.MethodGet,
			Handler: h.listNotifications,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/summary",
			Method:  http.MethodGet,
			Handler: h.getSummary,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/mark-all-read",
			Method:  http.MethodPost,
			Handler: h.markAllAsRead,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/clear-read",
			Method:  http.MethodPost,
			Handler: h.clearReadNotifications,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/item/{id}",
			Method:  http.MethodGet,
			Handler: h.getNotification,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/item/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteNotification,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
		{
			Path:    "/api/v1/notifications/item/{id}/mark-read",
			Method:  http.MethodPost,
			Handler: h.markAsRead,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
			},
		},
	}
}
