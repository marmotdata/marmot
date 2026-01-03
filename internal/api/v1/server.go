package v1

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/marmotdata/marmot/docs"
	"github.com/marmotdata/marmot/internal/api/v1/assets"
	"github.com/marmotdata/marmot/internal/api/v1/health"
	"github.com/marmotdata/marmot/internal/crypto"

	// Plugin imports
	_ "github.com/marmotdata/marmot/internal/plugin/providers/airflow"
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

	"github.com/marmotdata/marmot/internal/api/auth"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/api/v1/dataproducts"
	docsAPI "github.com/marmotdata/marmot/internal/api/v1/docs"
	"github.com/marmotdata/marmot/internal/api/v1/glossary"
	"github.com/marmotdata/marmot/internal/api/v1/lineage"
	mcpAPI "github.com/marmotdata/marmot/internal/api/v1/mcp"
	metricsAPI "github.com/marmotdata/marmot/internal/api/v1/metrics"
	"github.com/marmotdata/marmot/internal/api/v1/plugins"
	"github.com/marmotdata/marmot/internal/api/v1/runs"
	schedulesAPI "github.com/marmotdata/marmot/internal/api/v1/schedules"
	searchAPI "github.com/marmotdata/marmot/internal/api/v1/search"
	"github.com/marmotdata/marmot/internal/api/v1/teams"
	"github.com/marmotdata/marmot/internal/api/v1/ui"
	"github.com/marmotdata/marmot/internal/api/v1/users"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	authService "github.com/marmotdata/marmot/internal/core/auth"
	dataproductService "github.com/marmotdata/marmot/internal/core/dataproduct"
	docsService "github.com/marmotdata/marmot/internal/core/docs"
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

	// Data product membership evaluation
	membershipService    *dataproductService.MembershipService
	membershipReconciler *dataproductService.Reconciler

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
	dataProductRepo := dataproductService.NewPostgresRepository(db, recorder)

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
	dataProductSvc := dataproductService.NewService(dataProductRepo)
	docsRepo := docsService.NewPostgresRepository(db)
	docsSvc := docsService.NewService(docsRepo)
	membershipRepo := dataproductService.NewPostgresMembershipRepository(db, recorder)
	membershipSvc := dataproductService.NewMembershipService(
		dataProductRepo,
		membershipRepo,
		assetSvc,
		&dataproductService.MembershipConfig{
			MaxWorkers:    5,
			BatchSize:     50,
			FlushInterval: 500 * time.Millisecond,
		},
	)
	membershipReconciler := dataproductService.NewReconciler(membershipSvc, &dataproductService.ReconcilerConfig{
		Interval: 30 * time.Minute,
	})

	// Start membership evaluation services
	membershipSvc.Start(context.Background())
	membershipReconciler.Start(context.Background())

	// Register membership service with asset service for event hooks
	assetSvc.SetMembershipObserver(membershipSvc)

	// Register membership service with data product service for rule event hooks
	dataProductSvc.SetRuleObserver(membershipSvc)

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
			fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
			fmt.Fprintln(os.Stderr, "⚠️  ENCRYPTION KEY REQUIRED")
			fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
			fmt.Fprintln(os.Stderr, "Marmot requires an encryption key to protect sensitive pipeline credentials.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "To generate a key, run:")
			fmt.Fprintln(os.Stderr, "  marmot generate-encryption-key")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Then set it via:")
			fmt.Fprintln(os.Stderr, "  export MARMOT_SERVER_ENCRYPTION_KEY=\"your-generated-key\"")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Or to run WITHOUT encryption (NOT RECOMMENDED):")
			fmt.Fprintln(os.Stderr, "  export MARMOT_SERVER_ALLOW_UNENCRYPTED=true")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "⚠️  Running unencrypted means pipeline credentials will be stored")
			fmt.Fprintln(os.Stderr, "    in PLAINTEXT in the database. This is a SECURITY RISK.")
			fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
			log.Fatal().Msg("Encryption key required")
		}
		fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
		fmt.Fprintln(os.Stderr, "⚠️  WARNING: ENCRYPTION DISABLED")
		fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
		fmt.Fprintln(os.Stderr, "Pipeline credentials will be stored in PLAINTEXT in the database.")
		fmt.Fprintln(os.Stderr, "This is a SECURITY RISK and should only be used for development.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To enable encryption, run:")
		fmt.Fprintln(os.Stderr, "  marmot generate-encryption-key")
		fmt.Fprintln(os.Stderr, "═══════════════════════════════════════════════════════════════")
		log.Warn().Msg("Encryption disabled - credentials stored in plaintext")
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

	if oktaConfig := config.Auth.Okta; oktaConfig != nil && oktaConfig.Enabled {
		if oktaConfig.ClientID != "" && oktaConfig.ClientSecret != "" && oktaConfig.URL != "" {
			oktaProvider := authService.NewOktaProvider(config, userSvc, authSvc, teamSvc)
			oauthManager.RegisterProvider(oktaProvider)
		} else {
			log.Warn().Msg("Incomplete Okta configuration found - provider will not be initialized")
		}
	}

	if googleConfig := config.Auth.Google; googleConfig != nil && googleConfig.Enabled {
		if googleConfig.ClientID != "" && googleConfig.ClientSecret != "" {
			googleProvider := authService.NewGoogleProvider(config, userSvc)
			oauthManager.RegisterProvider(googleProvider)
		} else {
			log.Warn().Msg("Incomplete Google configuration found - provider will not be initialized")
		}
	}

	if githubConfig := config.Auth.GitHub; githubConfig != nil && githubConfig.Enabled {
		if githubConfig.ClientID != "" && githubConfig.ClientSecret != "" {
			githubProvider := authService.NewGitHubProvider(config, userSvc)
			oauthManager.RegisterProvider(githubProvider)
		} else {
			log.Warn().Msg("Incomplete GitHub configuration found - provider will not be initialized")
		}
	}

	if gitlabConfig := config.Auth.GitLab; gitlabConfig != nil && gitlabConfig.Enabled {
		if gitlabConfig.ClientID != "" && gitlabConfig.ClientSecret != "" {
			gitlabProvider := authService.NewGitLabProvider(config, userSvc)
			oauthManager.RegisterProvider(gitlabProvider)
		} else {
			log.Warn().Msg("Incomplete GitLab configuration found - provider will not be initialized")
		}
	}

	if slackConfig := config.Auth.Slack; slackConfig != nil && slackConfig.Enabled {
		if slackConfig.ClientID != "" && slackConfig.ClientSecret != "" {
			slackProvider := authService.NewSlackProvider(config, userSvc)
			oauthManager.RegisterProvider(slackProvider)
		} else {
			log.Warn().Msg("Incomplete Slack configuration found - provider will not be initialized")
		}
	}

	if auth0Config := config.Auth.Auth0; auth0Config != nil && auth0Config.Enabled {
		if auth0Config.ClientID != "" && auth0Config.ClientSecret != "" && auth0Config.URL != "" {
			auth0Provider := authService.NewAuth0Provider(config, userSvc, authSvc, teamSvc)
			oauthManager.RegisterProvider(auth0Provider)
		} else {
			log.Warn().Msg("Incomplete Auth0 configuration found - provider will not be initialized")
		}
	}

	server := &Server{
		config:               config,
		metricsService:       metricsService,
		wsHub:                wsHub,
		scheduler:            scheduler,
		membershipService:    membershipSvc,
		membershipReconciler: membershipReconciler,
	}

	server.handlers = []interface{ Routes() []common.Route }{
		health.NewHandler(),
		assets.NewHandler(assetSvc, assetDocsSvc, userSvc, authSvc, metricsService, runsSvc, teamSvc, config),
		users.NewHandler(userSvc, authSvc, config),
		auth.NewHandler(authSvc, oauthManager, userSvc, config),
		lineage.NewHandler(lineageSvc, userSvc, authSvc, config),
		mcpAPI.NewHandler(assetSvc, glossarySvc, userSvc, teamSvc, lineageSvc, authSvc, config),
		metricsAPI.NewHandler(metricsService, userSvc, authSvc, config),
		runs.NewHandler(runsSvc, userSvc, authSvc, config),
		glossary.NewHandler(glossarySvc, userSvc, authSvc, config),
		dataproducts.NewHandler(dataProductSvc, userSvc, authSvc, config),
		docsAPI.NewHandler(docsSvc, userSvc, authSvc, config),
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
	if s.membershipReconciler != nil {
		s.membershipReconciler.Stop()
	}
	if s.membershipService != nil {
		s.membershipService.Stop()
	}
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
