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
	"github.com/marmotdata/marmot/internal/crypto"

	// Plugin imports - blank imports trigger init() functions to register plugins
	_ "github.com/marmotdata/marmot/internal/plugin/providers/asyncapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/bigquery"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/dbt"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mongodb"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mysql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/openapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/postgresql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/s3"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sns"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sqs"
	"github.com/marmotdata/marmot/internal/api/v1/auth"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/api/v1/glossary"
	schedulesAPI "github.com/marmotdata/marmot/internal/api/v1/schedules"
	"github.com/marmotdata/marmot/internal/api/v1/lineage"
	metricsAPI "github.com/marmotdata/marmot/internal/api/v1/metrics"
	"github.com/marmotdata/marmot/internal/api/v1/plugins"
	"github.com/marmotdata/marmot/internal/api/v1/runs"
	searchAPI "github.com/marmotdata/marmot/internal/api/v1/search"
	"github.com/marmotdata/marmot/internal/api/v1/teams"
	"github.com/marmotdata/marmot/internal/api/v1/ui"
	"github.com/marmotdata/marmot/internal/api/v1/users"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	authService "github.com/marmotdata/marmot/internal/core/auth"
	glossaryService "github.com/marmotdata/marmot/internal/core/glossary"
	lineageService "github.com/marmotdata/marmot/internal/core/lineage"
	runService "github.com/marmotdata/marmot/internal/core/runs"
	searchService "github.com/marmotdata/marmot/internal/core/search"
	teamService "github.com/marmotdata/marmot/internal/core/team"
	userService "github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/metrics"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/websocket"
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
	wsHub          *websocket.Hub
	scheduler      *runService.Scheduler

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
	glossaryRepo := glossaryService.NewPostgresRepository(db, recorder)
	searchRepo := searchService.NewPostgresRepository(db, recorder)

	assetSvc := asset.NewService(assetRepo)
	userSvc := userService.NewService(userRepo)
	lineageSvc := lineageService.NewService(lineageRepo, assetSvc)
	assetDocsSvc := assetdocs.NewService(assetDocsRepo)
	authSvc := authService.NewService(authRepo, userSvc)
	runsSvc := runService.NewService(runRepo, assetSvc, lineageSvc, recorder)
	glossarySvc := glossaryService.NewService(glossaryRepo)
	teamRepo := teamService.NewPostgresRepository(db)
	teamSvc := teamService.NewService(teamRepo)
	searchSvc := searchService.NewService(searchRepo)
	scheduleRepo := runService.NewSchedulePostgresRepository(db)
	scheduleSvc := runService.NewScheduleService(scheduleRepo)

	wsHub := websocket.NewHub()
	wsHub.Start(context.Background())

	jobRunBroadcaster := websocket.NewJobRunBroadcaster(wsHub)
	scheduleSvc.SetBroadcaster(jobRunBroadcaster)

	var scheduleEncryptor *crypto.Encryptor
	if config.Server.EncryptionKey != "" {
		var err error
		scheduleEncryptor, err = runService.GetEncryptor(config)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize encryption - invalid encryption key")
		}
		log.Info().Msg("Encryption enabled for pipeline credentials")
	} else {
		if !config.Server.AllowUnencrypted {
			log.Fatal().Msg(
				"═══════════════════════════════════════════════════════════════\n" +
					"⚠️  ENCRYPTION KEY REQUIRED\n" +
					"═══════════════════════════════════════════════════════════════\n" +
					"Marmot requires an encryption key to protect sensitive pipeline credentials.\n" +
					"\n" +
					"To generate a key, run:\n" +
					"  marmot generate-encryption-key\n" +
					"\n" +
					"Then set it via:\n" +
					"  export MARMOT_SERVER_ENCRYPTION_KEY=\"your-generated-key\"\n" +
					"\n" +
					"Or to run WITHOUT encryption (NOT RECOMMENDED):\n" +
					"  export MARMOT_SERVER_ALLOW_UNENCRYPTED=true\n" +
					"\n" +
					"⚠️  Running unencrypted means pipeline credentials will be stored\n" +
					"    in PLAINTEXT in the database. This is a SECURITY RISK.\n" +
					"═══════════════════════════════════════════════════════════════",
			)
		}
		log.Warn().Msg(
			"═══════════════════════════════════════════════════════════════\n" +
				"⚠️  WARNING: ENCRYPTION DISABLED\n" +
				"═══════════════════════════════════════════════════════════════\n" +
				"Pipeline credentials will be stored in PLAINTEXT in the database.\n" +
				"This is a SECURITY RISK and should only be used for development.\n" +
				"\n" +
				"To enable encryption, run:\n" +
				"  marmot generate-encryption-key\n" +
				"═══════════════════════════════════════════════════════════════",
		)
	}

	pluginRegistry := plugin.GetRegistry()

	schedulerConfig := &runService.SchedulerConfig{
		MaxWorkers:        config.Pipelines.MaxWorkers,
		SchedulerInterval: time.Duration(config.Pipelines.SchedulerInterval) * time.Second,
		LeaseExpiry:       time.Duration(config.Pipelines.LeaseExpiry) * time.Second,
		ClaimExpiry:       time.Duration(config.Pipelines.ClaimExpiry) * time.Second,
	}
	scheduler := runService.NewScheduler(scheduleSvc, runsSvc, scheduleEncryptor, pluginRegistry, schedulerConfig)

	if err := scheduler.Start(context.Background()); err != nil {
		log.Error().Err(err).Msg("Failed to start scheduler")
	}

	oauthManager := authService.NewOAuthManager()

	if config.Auth.Providers != nil {
		if oktaConfig, exists := config.Auth.Providers["okta"]; exists && oktaConfig != nil && oktaConfig.Enabled {
			if oktaConfig.ClientID != "" && oktaConfig.ClientSecret != "" && oktaConfig.URL != "" {
				oktaProvider := authService.NewOktaProvider(config, userSvc, authSvc, teamSvc)
				oauthManager.RegisterProvider(oktaProvider)
			} else {
				log.Warn().Msg("Incomplete Okta configuration found - provider will not be initialized")
			}
		}
	}

	server := &Server{
		config:         config,
		metricsService: metricsService,
		wsHub:          wsHub,
		scheduler:      scheduler,
	}

	server.handlers = []interface{ Routes() []common.Route }{
		assets.NewHandler(assetSvc, assetDocsSvc, userSvc, authSvc, metricsService, runsSvc, teamSvc, config),
		users.NewHandler(userSvc, authSvc, config),
		auth.NewHandler(authSvc, oauthManager, userSvc, config),
		lineage.NewHandler(lineageSvc, userSvc, authSvc, config),
		metricsAPI.NewHandler(metricsService, userSvc, authSvc, config),
		runs.NewHandler(runsSvc, userSvc, authSvc, config),
		glossary.NewHandler(glossarySvc, userSvc, authSvc, config),
		teams.NewHandler(teamSvc, userSvc, authSvc, config),
		searchAPI.NewHandler(searchSvc, userSvc, authSvc, metricsService, config),
		schedulesAPI.NewHandler(scheduleSvc, runsSvc, userSvc, authSvc, scheduleEncryptor, config),
		websocket.NewHandler(wsHub, userSvc, authSvc, config),
		plugins.NewHandler(),
		ui.NewHandler(config),
	}

	return server
}

func (s *Server) Stop() {
	if s.scheduler != nil {
		s.scheduler.Stop()
	}
	if s.wsHub != nil {
		s.wsHub.Stop()
	}
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

			for key, value := range s.config.Server.CustomResponseHeaders {
				w.Header().Set(key, value)
			}

			if r.Method == http.MethodOptions {
				return
			}

			if handler, ok := handlers[r.Method]; ok {
				// Check if this is a websocket upgrade request
				isWebSocket := r.Header.Get("Upgrade") == "websocket"

				if isWebSocket {
					// For websocket connections, use the raw ResponseWriter
					// Wrapping breaks the upgrade process
					handler(w, r)
				} else {
					// For regular HTTP requests, use the wrapped ResponseWriter for metrics
					wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
					handler(wrapped, r)

					duration := time.Since(start)
					metricPath := s.getMetricPath(path, r.URL.Path)
					err := s.metricsService.Collector().RecordHTTPRequest(r.Method, metricPath, strconv.Itoa(wrapped.statusCode))
					if err != nil {
						log.Error().Err(err)
					}
					s.metricsService.Collector().RecordHTTPDuration(r.Method, metricPath, duration)
				}
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
