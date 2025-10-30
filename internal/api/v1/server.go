package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/marmotdata/marmot/docs"
	"github.com/marmotdata/marmot/internal/api/v1/assets"
	"github.com/marmotdata/marmot/internal/api/v1/auth"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/api/v1/lineage"
	metricsAPI "github.com/marmotdata/marmot/internal/api/v1/metrics"
	"github.com/marmotdata/marmot/internal/api/v1/runs"
	"github.com/marmotdata/marmot/internal/api/v1/users"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	authService "github.com/marmotdata/marmot/internal/core/auth"
	lineageService "github.com/marmotdata/marmot/internal/core/lineage"
	runService "github.com/marmotdata/marmot/internal/core/runs"
	userService "github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/metrics"
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
	config         *config.Config
	metricsService *metrics.Service

	handlers []interface{ Routes() []common.Route }
}

func New(config *config.Config, db *pgxpool.Pool) *Server {
	metricsStore := metrics.NewPostgresStore(db)
	metricsService := metrics.NewService(metricsStore)
	metricsService.Start(context.Background())
	recorder := metricsService.GetRecorder()

	assetRepo := asset.NewPostgresRepository(db, recorder)
	userRepo := userService.NewPostgresRepository(db)
	lineageRepo := lineageService.NewPostgresRepository(db)
	assetDocsRepo := assetdocs.NewPostgresRepository(db)
	authRepo := authService.NewPostgresRepository(db)
	runRepo := runService.NewPostgresRepository(db)

	assetSvc := asset.NewService(assetRepo)
	userSvc := userService.NewService(userRepo)
	lineageSvc := lineageService.NewService(lineageRepo, assetSvc)
	assetDocsSvc := assetdocs.NewService(assetDocsRepo)
	authSvc := authService.NewService(authRepo, userSvc)
	runsSvc := runService.NewService(runRepo, assetSvc, lineageSvc, recorder)

	oauthManager := authService.NewOAuthManager()

	if config.Auth.Providers != nil {
		if oktaConfig, exists := config.Auth.Providers["okta"]; exists && oktaConfig != nil && oktaConfig.Enabled {
			if oktaConfig.ClientID != "" && oktaConfig.ClientSecret != "" && oktaConfig.URL != "" {
				oktaProvider := authService.NewOktaProvider(config, userSvc, authSvc)
				oauthManager.RegisterProvider(oktaProvider)
			} else {
				log.Warn().Msg("Incomplete Okta configuration found - provider will not be initialized")
			}
		}
	}

	server := &Server{
		config:         config,
		metricsService: metricsService,
	}

	server.handlers = []interface{ Routes() []common.Route }{
		assets.NewHandler(assetSvc, assetDocsSvc, userSvc, authSvc, metricsService, runsSvc, config),
		users.NewHandler(userSvc, authSvc, config),
		auth.NewHandler(authSvc, oauthManager, userSvc, config),
		lineage.NewHandler(lineageSvc, userSvc, authSvc, config),
		metricsAPI.NewHandler(metricsService, userSvc, authSvc, config),
		runs.NewHandler(runsSvc, userSvc, authSvc, config),
	}

	return server
}

func (s *Server) Stop() {
	if s.metricsService != nil {
		s.metricsService.Stop()
	}
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
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			for key, value := range s.config.Server.CustomResponseHeaders {
				w.Header().Set(key, value)
			}

			if r.Method == http.MethodOptions {
				return
			}

			if handler, ok := handlers[r.Method]; ok {
				handler(wrapped, r)

				duration := time.Since(start)
				metricPath := s.getMetricPath(path, r.URL.Path)
				err := s.metricsService.Collector().RecordHTTPRequest(r.Method, metricPath, strconv.Itoa(wrapped.statusCode))
				if err != nil {
					log.Error().Err(err)
				}
				s.metricsService.Collector().RecordHTTPDuration(r.Method, metricPath, duration)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})
	}

	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

func (s *Server) getMetricPath(routePath, actualPath string) string {
	if strings.Contains(routePath, "{") {
		return routePath
	}
	return actualPath
}
