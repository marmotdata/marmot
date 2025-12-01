package websocket

import (
	"net/http"
	"strings"

	"github.com/centrifugal/centrifuge"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/rs/zerolog/log"
)

// Handler handles websocket connections using Centrifuge
type Handler struct {
	hub       *Hub
	userSvc   user.Service
	authSvc   auth.Service
	config    *config.Config
	wsHandler http.Handler
}

// NewHandler creates a new websocket handler
func NewHandler(hub *Hub, userSvc user.Service, authSvc auth.Service, config *config.Config) *Handler {
	h := &Handler{
		hub:     hub,
		userSvc: userSvc,
		authSvc: authSvc,
		config:  config,
	}

	// Create websocket handler once with proper origin checking
	h.wsHandler = centrifuge.NewWebsocketHandler(hub.Node(), centrifuge.WebsocketConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// In production, only allow same origin
			// In development, allow localhost on any port
			if isProduction() {
				// Only allow connections from the same origin (server's root URL)
				rootURL := config.Server.RootURL
				if rootURL == "" {
					// If no root URL configured, allow from same host
					expectedOrigin := "http://" + r.Host
					if r.TLS != nil {
						expectedOrigin = "https://" + r.Host
					}
					allowed := origin == expectedOrigin
					log.Debug().
						Str("origin", origin).
						Str("expected", expectedOrigin).
						Bool("allowed", allowed).
						Msg("Websocket origin check (production, no root URL)")
					return allowed
				}
				allowed := strings.HasPrefix(origin, rootURL)
				log.Debug().
					Str("origin", origin).
					Str("root_url", rootURL).
					Bool("allowed", allowed).
					Msg("Websocket origin check (production)")
				return allowed
			}

			// Development: allow localhost and 127.0.0.1 on any port
			isLocalhost := strings.HasPrefix(origin, "http://localhost:") ||
						  strings.HasPrefix(origin, "https://localhost:") ||
						  strings.HasPrefix(origin, "http://127.0.0.1:") ||
						  strings.HasPrefix(origin, "https://127.0.0.1:")
			log.Debug().
				Str("origin", origin).
				Bool("is_localhost", isLocalhost).
				Msg("Websocket origin check (development)")
			return isLocalhost
		},
	})

	return h
}

// Routes returns the websocket routes
func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:   "/api/v1/ingestion/ws",
			Method: http.MethodGet,
			Handler: func(w http.ResponseWriter, r *http.Request) {
				defer func() {
					if rec := recover(); rec != nil {
						log.Error().Interface("panic", rec).Msg("Websocket handler panic")
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
				}()

				log.Debug().
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Str("origin", r.Header.Get("Origin")).
					Str("upgrade", r.Header.Get("Upgrade")).
					Msg("Websocket connection request")

				// Use the pre-created websocket handler
				h.wsHandler.ServeHTTP(w, r)
			},
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
	}
}
