package v1

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/marmotdata/marmot/docs"
	"github.com/marmotdata/marmot/internal/api/v1/assets"
	"github.com/marmotdata/marmot/internal/api/v1/auth"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/api/v1/lineage"
	"github.com/marmotdata/marmot/internal/api/v1/users"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	authService "github.com/marmotdata/marmot/internal/core/auth"
	lineageService "github.com/marmotdata/marmot/internal/core/lineage"
	userService "github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/sync"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Marmot API
// @version 0.1
// @description API for interacting with Marmot
// @BasePath /api/v1
// @license.name MIT
// @license.url https://opensource.org/license/MIT
type Server struct {
	config *config.Config

	handlers []interface{ Routes() []common.Route }
}

func New(config *config.Config, db *pgxpool.Pool) *Server {
	// Initialize repositories
	assetRepo := asset.NewPostgresRepository(db)
	userRepo := userService.NewPostgresRepository(db)
	lineageRepo := lineageService.NewPostgresRepository(db)
	assetDocsRepo := assetdocs.NewPostgresRepository(db)
	authRepo := authService.NewPostgresRepository(db)

	// Initialize services
	assetSvc := asset.NewService(assetRepo)
	userSvc := userService.NewService(userRepo)
	lineageSvc := lineageService.NewService(lineageRepo, assetSvc)
	assetDocsSvc := assetdocs.NewService(assetDocsRepo)
	authSvc := authService.NewService(authRepo, userSvc)
	oauthManager := authService.NewOAuthManager()

	// Configure OAuth providers
	if config.Auth.Providers != nil {
		if oktaConfig, exists := config.Auth.Providers["okta"]; exists && oktaConfig != nil {
			if oktaConfig.ClientID != "" && oktaConfig.ClientSecret != "" && oktaConfig.URL != "" {
				oktaProvider := authService.NewOktaProvider(config, userSvc, authSvc)
				oauthManager.RegisterProvider(oktaProvider)
			} else {
				log.Warn().Msg("Incomplete Okta configuration found - provider will not be initialized")
			}
		}
	}

	server := &Server{
		config: config,
	}

	// Initialize handlers
	server.handlers = []interface{ Routes() []common.Route }{
		assets.NewHandler(assetSvc, assetDocsSvc, sync.NewAssetSyncer(), userSvc, authSvc),
		users.NewHandler(userSvc, authSvc),
		auth.NewHandler(authSvc, oauthManager, userSvc, config),
		lineage.NewHandler(lineageSvc, userSvc, authSvc),
	}

	return server
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	var routes []common.Route
	for _, handler := range s.handlers {
		routes = append(routes, handler.Routes()...)
	}

	routesByPath := make(map[string][]common.Route)
	for _, route := range routes {
		path := route.Path
		pathWithoutSlash := strings.TrimSuffix(path, "/")
		pathWithSlash := pathWithoutSlash + "/"

		routesByPath[pathWithoutSlash] = append(routesByPath[pathWithoutSlash], route)
		routesByPath[pathWithSlash] = append(routesByPath[pathWithSlash], route)
	}

	for path, pathRoutes := range routesByPath {
		handlers := make(map[string]http.HandlerFunc)
		for _, route := range pathRoutes {
			handler := route.Handler
			for i := len(route.Middleware) - 1; i >= 0; i-- {
				handler = route.Middleware[i](handler)
			}
			handlers[route.Method] = handler
		}

		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			for key, value := range s.config.Server.CustomResponseHeaders {
				w.Header().Set(key, value)
			}

			if r.Method == http.MethodOptions {
				return
			}

			if handler, ok := handlers[r.Method]; ok {
				handler(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})
	}

	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}
