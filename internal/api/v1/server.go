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
	_ "github.com/marmotdata/marmot/internal/plugin/providers/azureblob"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/bigquery"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/clickhouse"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/confluent"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/dbt"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/gcs"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/kafka"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mongodb"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/mysql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/nats"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/openapi"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/postgresql"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/redpanda"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/redis"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/s3"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sns"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/sqs"
	_ "github.com/marmotdata/marmot/internal/plugin/providers/trino"

	"github.com/marmotdata/marmot/internal/api/auth"
	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/api/v1/dataproducts"
	docsAPI "github.com/marmotdata/marmot/internal/api/v1/docs"
	"github.com/marmotdata/marmot/internal/api/v1/glossary"
	"github.com/marmotdata/marmot/internal/api/v1/lineage"
	assetrulesAPI "github.com/marmotdata/marmot/internal/api/v1/assetrules"
	mcpAPI "github.com/marmotdata/marmot/internal/api/v1/mcp"
	metricsAPI "github.com/marmotdata/marmot/internal/api/v1/metrics"
	notificationsAPI "github.com/marmotdata/marmot/internal/api/v1/notifications"
	"github.com/marmotdata/marmot/internal/api/v1/plugins"
	"github.com/marmotdata/marmot/internal/api/v1/runs"
	schedulesAPI "github.com/marmotdata/marmot/internal/api/v1/schedules"
	searchAPI "github.com/marmotdata/marmot/internal/api/v1/search"
	subscriptionsAPI "github.com/marmotdata/marmot/internal/api/v1/subscriptions"
	"github.com/marmotdata/marmot/internal/api/v1/teams"
	"github.com/marmotdata/marmot/internal/api/v1/ui"
	"github.com/marmotdata/marmot/internal/api/v1/users"
	webhooksAPI "github.com/marmotdata/marmot/internal/api/v1/webhooks"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/assetdocs"
	authService "github.com/marmotdata/marmot/internal/core/auth"
	dataproductService "github.com/marmotdata/marmot/internal/core/dataproduct"
	docsService "github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/enrichment"
	glossaryService "github.com/marmotdata/marmot/internal/core/glossary"
	lineageService "github.com/marmotdata/marmot/internal/core/lineage"
	assetruleService "github.com/marmotdata/marmot/internal/core/assetrule"
	notificationService "github.com/marmotdata/marmot/internal/core/notification"
	runService "github.com/marmotdata/marmot/internal/core/runs"
	searchService "github.com/marmotdata/marmot/internal/core/search"
	"github.com/marmotdata/marmot/internal/core/subscription"
	teamService "github.com/marmotdata/marmot/internal/core/team"
	userService "github.com/marmotdata/marmot/internal/core/user"
	webhookService "github.com/marmotdata/marmot/internal/core/webhook"
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

	// Asset rule membership evaluation
	assetRuleMembershipService *assetruleService.MembershipService
	assetRuleReconciler        *assetruleService.Reconciler

	// Notification service
	notificationService *notificationService.Service

	// Webhook dispatcher
	webhookDispatcher *webhookService.Dispatcher

	handlers []interface{ Routes() []common.Route }
}

func New(config *config.Config, db *pgxpool.Pool) *Server {
	metricsStore := metrics.NewPostgresStore(db)
	metricsService := metrics.NewService(metricsStore, db)
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
	notificationRepo := notificationService.NewPostgresRepository(db)
	notificationSvc := notificationService.NewService(
		notificationRepo,
		&teamMembershipAdapter{teamSvc: teamSvc},
		notificationService.WithDB(db),
		notificationService.WithUserPreferencesProvider(&userPreferencesAdapter{userSvc: userSvc}),
	)
	notificationSvc.Start(context.Background())
	subscriptionRepo := subscription.NewPostgresRepository(db)
	subscriptionSvc := subscription.NewService(subscriptionRepo)
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
		DB:       db,
	})

	// Asset rule services
	enrichmentEvaluator := enrichment.NewEvaluator(db)
	assetRuleRepo := assetruleService.NewPostgresRepository(db, recorder)
	assetRuleMemberRepo := assetruleService.NewPostgresMembershipRepository(db, recorder)
	assetRuleMemberSvc := assetruleService.NewMembershipService(
		assetRuleRepo,
		assetRuleMemberRepo,
		enrichmentEvaluator,
		&assetruleService.MembershipConfig{
			MaxWorkers:    5,
			BatchSize:     50,
			FlushInterval: 500 * time.Millisecond,
		},
	)
	assetRuleReconciler := assetruleService.NewReconciler(assetRuleMemberSvc, &assetruleService.ReconcilerConfig{
		Interval: 30 * time.Minute,
		DB:       db,
	})
	assetRuleSvc := assetruleService.NewService(assetRuleRepo, assetRuleMemberRepo, enrichmentEvaluator, assetRuleMemberSvc)

	// Start membership evaluation services
	membershipSvc.Start(context.Background())
	membershipReconciler.Start(context.Background())
	assetRuleMemberSvc.Start(context.Background())
	assetRuleReconciler.Start(context.Background())

	// Register membership service with asset service for event hooks
	assetSvc.SetMembershipObserver(membershipSvc)
	assetSvc.AddMembershipObserver(assetRuleMemberSvc)

	// Register membership service with data product service for rule event hooks
	dataProductSvc.SetRuleObserver(membershipSvc)

	// Register notification observers
	runsSvc.SetCompletionObserver(&runCompletionNotifier{
		notificationSvc: notificationSvc,
		userSvc:         userSvc,
	})
	assetSvc.SetNotificationObserver(&assetChangeNotifier{
		notificationSvc: notificationSvc,
		teamSvc:         teamSvc,
		lineageSvc:      lineageSvc,
		assetSvc:        assetSvc,
		subscriptionSvc: subscriptionSvc,
	})
	lineageSvc.SetLineageChangeObserver(&lineageChangeNotifier{
		notificationSvc: notificationSvc,
		teamSvc:         teamSvc,
		assetSvc:        assetSvc,
		subscriptionSvc: subscriptionSvc,
	})
	teamSvc.SetMembershipNotifier(&teamMembershipNotifier{
		notificationSvc: notificationSvc,
	})
	docsSvc.SetMentionNotifier(&docsMentionNotifier{
		notificationSvc: notificationSvc,
		userSvc:         userSvc,
		teamSvc:         teamSvc,
	})

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
		DB:                db,
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

	// Webhook service for external notifications
	webhookRepo := webhookService.NewPostgresRepository(db)
	webhookRegistry := webhookService.DefaultRegistry()
	webhookDispatcher := webhookService.NewDispatcher(webhookRepo, webhookRegistry, webhookService.DispatcherConfig{
		MaxWorkers: 5,
		QueueSize:  100,
	})
	webhookDispatcher.Start(context.Background())
	webhookSvc := webhookService.NewService(webhookRepo, scheduleEncryptor, webhookDispatcher)
	notificationSvc.SetExternalNotifier(webhookSvc)

	server := &Server{
		config:                    config,
		metricsService:            metricsService,
		wsHub:                     wsHub,
		scheduler:                 scheduler,
		membershipService:         membershipSvc,
		membershipReconciler:      membershipReconciler,
		assetRuleMembershipService: assetRuleMemberSvc,
		assetRuleReconciler:        assetRuleReconciler,
		notificationService:       notificationSvc,
		webhookDispatcher:         webhookDispatcher,
	}

	server.handlers = []interface{ Routes() []common.Route }{
		health.NewHandler(),
		assets.NewHandler(assetSvc, assetDocsSvc, userSvc, authSvc, metricsService, runsSvc, teamSvc, assetRuleSvc, config),
		users.NewHandler(userSvc, authSvc, config),
		auth.NewHandler(authSvc, oauthManager, userSvc, config),
		lineage.NewHandler(lineageSvc, userSvc, authSvc, config),
		mcpAPI.NewHandler(assetSvc, glossarySvc, userSvc, teamSvc, lineageSvc, authSvc, config),
		metricsAPI.NewHandler(metricsService, userSvc, authSvc, config),
		runs.NewHandler(runsSvc, userSvc, authSvc, config),
		glossary.NewHandler(glossarySvc, userSvc, authSvc, config),
		dataproducts.NewHandler(dataProductSvc, userSvc, authSvc, config),
		assetrulesAPI.NewHandler(assetRuleSvc, userSvc, authSvc, config),
		docsAPI.NewHandler(docsSvc, userSvc, authSvc, config),
		notificationsAPI.NewHandler(notificationSvc, userSvc, authSvc, config),
		subscriptionsAPI.NewHandler(subscriptionSvc, userSvc, authSvc, config),
		teams.NewHandler(teamSvc, userSvc, authSvc, config),
		webhooksAPI.NewHandler(webhookSvc, teamSvc, userSvc, authSvc, config),
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
	if s.assetRuleReconciler != nil {
		s.assetRuleReconciler.Stop()
	}
	if s.assetRuleMembershipService != nil {
		s.assetRuleMembershipService.Stop()
	}
	if s.webhookDispatcher != nil {
		s.webhookDispatcher.Stop()
	}
	if s.notificationService != nil {
		s.notificationService.Stop()
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
					s.metricsService.Collector().RecordHTTPRequest(r.Method, metricPath, strconv.Itoa(wrapped.statusCode))
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

// teamMembershipAdapter adapts teamService to notification.TeamMembershipProvider
type teamMembershipAdapter struct {
	teamSvc *teamService.Service
}

func (a *teamMembershipAdapter) GetTeamMemberUserIDs(ctx context.Context, teamID string) ([]string, error) {
	members, err := a.teamSvc.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, len(members))
	for i, m := range members {
		userIDs[i] = m.UserID
	}
	return userIDs, nil
}

type userPreferencesAdapter struct {
	userSvc userService.Service
}

func (a *userPreferencesAdapter) GetNotificationPreferences(ctx context.Context, userID string) (map[string]bool, error) {
	user, err := a.userSvc.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return a.extractNotificationPrefs(user), nil
}

func (a *userPreferencesAdapter) GetNotificationPreferencesBatch(ctx context.Context, userIDs []string) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool, len(userIDs))

	users, _, err := a.userSvc.List(ctx, userService.Filter{Limit: len(userIDs)})
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]*userService.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	for _, id := range userIDs {
		if user, exists := userMap[id]; exists {
			result[id] = a.extractNotificationPrefs(user)
		}
	}

	return result, nil
}

func (a *userPreferencesAdapter) extractNotificationPrefs(user *userService.User) map[string]bool {
	result := make(map[string]bool)
	if user == nil || user.Preferences == nil {
		return result
	}

	notifPrefs, ok := user.Preferences["notifications"].(map[string]interface{})
	if !ok {
		return result
	}

	for key, val := range notifPrefs {
		if boolVal, ok := val.(bool); ok {
			result[key] = boolVal
		}
	}
	return result
}

// runCompletionNotifier sends notifications when manual runs complete
type runCompletionNotifier struct {
	notificationSvc *notificationService.Service
	userSvc         userService.Service
}

func (n *runCompletionNotifier) OnRunCompleted(ctx context.Context, run *plugin.Run) {
	// Only send notifications for manual runs (not scheduled)
	if run.CreatedBy == "scheduler" || run.CreatedBy == "system" {
		return
	}

	// Look up the user who triggered the run
	user, err := n.userSvc.GetUserByUsername(ctx, run.CreatedBy)
	if err != nil {
		log.Warn().Err(err).Str("username", run.CreatedBy).Msg("Failed to find user for run completion notification")
		return
	}

	// Build notification title and message based on status
	var title, message string
	switch run.Status {
	case plugin.StatusCompleted:
		title = "Pipeline Completed"
		message = fmt.Sprintf("Pipeline \"%s\" completed successfully.", run.PipelineName)
		if run.Summary != nil && run.Summary.TotalEntities > 0 {
			message = fmt.Sprintf("Pipeline \"%s\" completed successfully. %d entities processed.", run.PipelineName, run.Summary.TotalEntities)
		}
	case plugin.StatusFailed:
		title = "Pipeline Failed"
		message = fmt.Sprintf("Pipeline \"%s\" failed.", run.PipelineName)
		if run.ErrorMessage != "" {
			message = fmt.Sprintf("Pipeline \"%s\" failed: %s", run.PipelineName, run.ErrorMessage)
		}
	case plugin.StatusCancelled:
		title = "Pipeline Cancelled"
		message = fmt.Sprintf("Pipeline \"%s\" was cancelled.", run.PipelineName)
	default:
		return
	}

	input := notificationService.CreateNotificationInput{
		Recipients: []notificationService.Recipient{{Type: notificationService.RecipientTypeUser, ID: user.ID}},
		Type:       notificationService.TypeJobComplete,
		Title:      title,
		Message:    message,
		Data: map[string]interface{}{
			"run_id":        run.ID,
			"pipeline_name": run.PipelineName,
			"status":        string(run.Status),
			"link":          fmt.Sprintf("/runs?tab=history&run=%s", run.ID),
		},
	}

	if err := n.notificationSvc.Create(ctx, input); err != nil {
		log.Warn().Err(err).Str("user_id", user.ID).Msg("Failed to send run completion notification")
	}
}

type assetChangeNotifier struct {
	notificationSvc *notificationService.Service
	teamSvc         *teamService.Service
	lineageSvc      lineageService.Service
	assetSvc        asset.Service
	subscriptionSvc *subscription.Service
}

func (n *assetChangeNotifier) OnAssetUpdated(ctx context.Context, a *asset.Asset, changeType string) {
	owners, err := n.teamSvc.ListAssetOwners(ctx, a.ID)
	if err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to get asset owners for notification")
		return
	}

	assetName := ""
	if a.Name != nil {
		assetName = *a.Name
	}

	assetMRN := ""
	if a.MRN != nil {
		assetMRN = *a.MRN
	}

	// Notify owners and subscribers of the changed asset
	recipients := make([]notificationService.Recipient, 0, len(owners))
	seen := make(map[string]bool)
	for _, owner := range owners {
		key := owner.Type + ":" + owner.ID
		if !seen[key] {
			recipients = append(recipients, notificationService.Recipient{
				Type: owner.Type,
				ID:   owner.ID,
			})
			seen[key] = true
		}
	}

	// Also include subscribers who want this notification type
	subscriberIDs, err := n.subscriptionSvc.GetSubscribersForAsset(ctx, a.ID, changeType)
	if err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to get asset subscribers")
	} else {
		for _, userID := range subscriberIDs {
			key := notificationService.RecipientTypeUser + ":" + userID
			if !seen[key] {
				recipients = append(recipients, notificationService.Recipient{
					Type: notificationService.RecipientTypeUser,
					ID:   userID,
				})
				seen[key] = true
			}
		}
	}

	if len(recipients) > 0 {
		n.notificationSvc.QueueAssetChange(a.ID, assetMRN, assetName, changeType, recipients)
	}

	// If this is a schema change, also notify lineage neighbors' owners.
	// Dispatched to a goroutine to avoid blocking the request path.
	if changeType == notificationService.TypeSchemaChange && assetMRN != "" {
		go n.notifyLineageNeighborsOfSchemaChange(context.Background(), assetMRN, assetName)
	}
}

func (n *assetChangeNotifier) notifyLineageNeighborsOfSchemaChange(ctx context.Context, assetMRN, assetName string) {
	// Notify downstream asset owners (they have an upstream schema change)
	downstreamMRNs, err := n.lineageSvc.GetImmediateNeighbors(ctx, assetMRN, "downstream")
	if err != nil {
		log.Warn().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to get downstream neighbors for upstream schema notification")
	} else {
		for _, downMRN := range downstreamMRNs {
			n.notifyNeighborOwners(ctx, downMRN, assetMRN, assetName, notificationService.TypeUpstreamSchemaChange)
		}
	}

	// Notify upstream asset owners (they have a downstream schema change)
	upstreamMRNs, err := n.lineageSvc.GetImmediateNeighbors(ctx, assetMRN, "upstream")
	if err != nil {
		log.Warn().Err(err).Str("asset_mrn", assetMRN).Msg("Failed to get upstream neighbors for downstream schema notification")
	} else {
		for _, upMRN := range upstreamMRNs {
			n.notifyNeighborOwners(ctx, upMRN, assetMRN, assetName, notificationService.TypeDownstreamSchemaChange)
		}
	}
}

func (n *assetChangeNotifier) notifyNeighborOwners(ctx context.Context, neighborMRN, changedAssetMRN, changedAssetName, notifType string) {
	neighborAsset, err := n.assetSvc.GetByMRN(ctx, neighborMRN)
	if err != nil {
		log.Warn().Err(err).Str("mrn", neighborMRN).Msg("Failed to get neighbor asset for lineage schema notification")
		return
	}

	owners, err := n.teamSvc.ListAssetOwners(ctx, neighborAsset.ID)
	if err != nil {
		log.Warn().Err(err).Str("asset_id", neighborAsset.ID).Msg("Failed to get neighbor asset owners")
		return
	}

	recipients := make([]notificationService.Recipient, 0, len(owners))
	seen := make(map[string]bool)
	for _, owner := range owners {
		key := owner.Type + ":" + owner.ID
		if !seen[key] {
			recipients = append(recipients, notificationService.Recipient{
				Type: owner.Type,
				ID:   owner.ID,
			})
			seen[key] = true
		}
	}

	// Also include subscribers of the neighbor asset who want this notification type
	subscriberIDs, subErr := n.subscriptionSvc.GetSubscribersForAsset(ctx, neighborAsset.ID, notifType)
	if subErr != nil {
		log.Warn().Err(subErr).Str("asset_id", neighborAsset.ID).Msg("Failed to get neighbor asset subscribers")
	} else {
		for _, userID := range subscriberIDs {
			key := notificationService.RecipientTypeUser + ":" + userID
			if !seen[key] {
				recipients = append(recipients, notificationService.Recipient{
					Type: notificationService.RecipientTypeUser,
					ID:   userID,
				})
				seen[key] = true
			}
		}
	}

	if len(recipients) == 0 {
		return
	}

	// Use the changed asset's MRN and name for the notification content
	// The aggregator will format the appropriate title/message based on notifType
	n.notificationSvc.QueueAssetChange(neighborAsset.ID, changedAssetMRN, changedAssetName, notifType, recipients)
}

type lineageChangeNotifier struct {
	notificationSvc *notificationService.Service
	teamSvc         *teamService.Service
	assetSvc        asset.Service
	subscriptionSvc *subscription.Service
}

func (n *lineageChangeNotifier) OnEdgeCreated(ctx context.Context, sourceMRN, targetMRN, edgeType string) {
	n.notifyLineageChange(ctx, sourceMRN, targetMRN)
}

func (n *lineageChangeNotifier) OnEdgeDeleted(ctx context.Context, sourceMRN, targetMRN string) {
	n.notifyLineageChange(ctx, sourceMRN, targetMRN)
}

func (n *lineageChangeNotifier) notifyLineageChange(ctx context.Context, sourceMRN, targetMRN string) {
	sourceAsset, err := n.assetSvc.GetByMRN(ctx, sourceMRN)
	if err != nil {
		log.Warn().Err(err).Str("mrn", sourceMRN).Msg("Failed to get source asset for lineage notification")
		return
	}
	targetAsset, err := n.assetSvc.GetByMRN(ctx, targetMRN)
	if err != nil {
		log.Warn().Err(err).Str("mrn", targetMRN).Msg("Failed to get target asset for lineage notification")
		return
	}

	// Queue source asset through the aggregator
	n.queueLineageChangeForAsset(ctx, sourceAsset)

	// Queue target asset through the aggregator
	n.queueLineageChangeForAsset(ctx, targetAsset)
}

func (n *lineageChangeNotifier) queueLineageChangeForAsset(ctx context.Context, a *asset.Asset) {
	owners, err := n.teamSvc.ListAssetOwners(ctx, a.ID)
	if err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to get asset owners for lineage notification")
	}

	recipients := make([]notificationService.Recipient, 0, len(owners))
	seen := make(map[string]bool)
	for _, owner := range owners {
		key := owner.Type + ":" + owner.ID
		if !seen[key] {
			recipients = append(recipients, notificationService.Recipient{Type: owner.Type, ID: owner.ID})
			seen[key] = true
		}
	}

	subscriberIDs, err := n.subscriptionSvc.GetSubscribersForAsset(ctx, a.ID, "lineage_change")
	if err != nil {
		log.Warn().Err(err).Str("asset_id", a.ID).Msg("Failed to get asset subscribers for lineage notification")
	}
	for _, userID := range subscriberIDs {
		key := notificationService.RecipientTypeUser + ":" + userID
		if !seen[key] {
			recipients = append(recipients, notificationService.Recipient{
				Type: notificationService.RecipientTypeUser,
				ID:   userID,
			})
			seen[key] = true
		}
	}

	if len(recipients) == 0 {
		return
	}

	assetName := ""
	if a.Name != nil {
		assetName = *a.Name
	}
	assetMRN := ""
	if a.MRN != nil {
		assetMRN = *a.MRN
	}

	n.notificationSvc.QueueAssetChange(a.ID, assetMRN, assetName, notificationService.TypeLineageChange, recipients)
}

type teamMembershipNotifier struct {
	notificationSvc *notificationService.Service
}

func (n *teamMembershipNotifier) OnMemberAdded(ctx context.Context, teamID, teamName, userID, role string) {
	input := notificationService.CreateNotificationInput{
		Recipients: []notificationService.Recipient{{Type: notificationService.RecipientTypeUser, ID: userID}},
		Type:       notificationService.TypeTeamInvite,
		Title:      "Added to Team",
		Message:    fmt.Sprintf("You have been added to team \"%s\" as %s.", teamName, role),
		Data: map[string]interface{}{
			"team_id":   teamID,
			"team_name": teamName,
			"role":      role,
			"link":      fmt.Sprintf("/teams/%s", teamID),
		},
	}

	if err := n.notificationSvc.Create(ctx, input); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Str("team_id", teamID).Msg("Failed to send team membership notification")
	}
}

type docsMentionNotifier struct {
	notificationSvc *notificationService.Service
	userSvc         userService.Service
	teamSvc         *teamService.Service
}

func (n *docsMentionNotifier) OnMention(ctx context.Context, mention docsService.Mention, pageID, pageTitle, entityType, entityID, mentionerID, mentionerName string) {
	log.Debug().
		Str("mention_label", mention.Label).
		Str("mention_type", mention.Type).
		Str("mention_id", mention.ID).
		Str("page_id", pageID).
		Str("mentioner_id", mentionerID).
		Msg("Processing mention notification")

	var link string
	switch entityType {
	case "asset":
		// Strip mrn:// prefix and add tab=documentation
		assetPath := strings.TrimPrefix(entityID, "mrn://")
		link = fmt.Sprintf("/discover/%s?page=%s&tab=documentation", assetPath, pageID)
	case "data_product":
		link = fmt.Sprintf("/products/%s?page=%s&tab=documentation", entityID, pageID)
	default:
		link = fmt.Sprintf("/docs/pages/%s", pageID)
	}

	// Handle based on mention type
	if mention.Type == "team" {
		n.handleTeamMention(ctx, mention, pageID, pageTitle, entityType, entityID, mentionerName, link)
	} else {
		n.handleUserMention(ctx, mention, pageID, pageTitle, entityType, entityID, mentionerID, mentionerName, link)
	}
}

func (n *docsMentionNotifier) handleUserMention(ctx context.Context, mention docsService.Mention, pageID, pageTitle, entityType, entityID, mentionerID, mentionerName, link string) {
	var mentionedUser *userService.User

	// If we have an ID, try to get user directly
	if mention.ID != "" {
		if user, err := n.userSvc.Get(ctx, mention.ID); err == nil {
			mentionedUser = user
			log.Debug().Str("mention_label", mention.Label).Str("user_id", user.ID).Msg("Found user by ID")
		}
	}

	// Fall back to searching by username/name
	if mentionedUser == nil {
		if user, err := n.userSvc.GetUserByUsername(ctx, mention.Label); err == nil {
			mentionedUser = user
			log.Debug().Str("mention_label", mention.Label).Str("user_id", user.ID).Msg("Found user by username")
		} else {
			// Search by name
			active := true
			users, _, err := n.userSvc.List(ctx, userService.Filter{
				Query:  mention.Label,
				Active: &active,
				Limit:  10,
			})
			if err == nil {
				for _, u := range users {
					if u.Name == mention.Label {
						mentionedUser = u
						log.Debug().Str("mention_label", mention.Label).Str("user_id", u.ID).Msg("Found user by name")
						break
					}
				}
			}
		}
	}

	if mentionedUser == nil {
		log.Debug().Str("mention_label", mention.Label).Msg("No user found for mention")
		return
	}

	if mentionedUser.ID == mentionerID {
		log.Debug().Str("mention_label", mention.Label).Msg("Skipping self-mention")
		return
	}

	input := notificationService.CreateNotificationInput{
		Recipients: []notificationService.Recipient{{Type: notificationService.RecipientTypeUser, ID: mentionedUser.ID}},
		Type:       notificationService.TypeMention,
		Title:      "Mentioned in Documentation",
		Message:    fmt.Sprintf("%s mentioned you in \"%s\".", mentionerName, pageTitle),
		Data: map[string]interface{}{
			"page_id":     pageID,
			"page_title":  pageTitle,
			"entity_type": entityType,
			"entity_id":   entityID,
			"mentioner":   mentionerName,
			"link":        link,
		},
	}

	if err := n.notificationSvc.Create(ctx, input); err != nil {
		log.Warn().Err(err).Str("user_id", mentionedUser.ID).Msg("Failed to send mention notification")
	} else {
		log.Info().Str("user_id", mentionedUser.ID).Str("page_id", pageID).Msg("Sent mention notification")
	}
}

func (n *docsMentionNotifier) handleTeamMention(ctx context.Context, mention docsService.Mention, pageID, pageTitle, entityType, entityID, mentionerName, link string) {
	var team *teamService.Team

	// If we have an ID, try to get team directly
	if mention.ID != "" {
		if t, err := n.teamSvc.GetTeam(ctx, mention.ID); err == nil {
			team = t
			log.Debug().Str("mention_label", mention.Label).Str("team_id", t.ID).Msg("Found team by ID")
		}
	}

	// Fall back to searching by name
	if team == nil {
		if t, err := n.teamSvc.GetTeamByName(ctx, mention.Label); err == nil {
			team = t
			log.Debug().Str("mention_label", mention.Label).Str("team_id", t.ID).Msg("Found team by name")
		}
	}

	if team == nil {
		log.Debug().Str("mention_label", mention.Label).Msg("No team found for mention")
		return
	}

	input := notificationService.CreateNotificationInput{
		Recipients: []notificationService.Recipient{{Type: notificationService.RecipientTypeTeam, ID: team.ID}},
		Type:       notificationService.TypeMention,
		Title:      "Team Mentioned in Documentation",
		Message:    fmt.Sprintf("%s mentioned your team in \"%s\".", mentionerName, pageTitle),
		Data: map[string]interface{}{
			"page_id":     pageID,
			"page_title":  pageTitle,
			"entity_type": entityType,
			"entity_id":   entityID,
			"mentioner":   mentionerName,
			"link":        link,
		},
	}

	if err := n.notificationSvc.Create(ctx, input); err != nil {
		log.Warn().Err(err).Str("team_id", team.ID).Msg("Failed to send team mention notification")
	} else {
		log.Info().Str("team_id", team.ID).Str("page_id", pageID).Msg("Sent team mention notification")
	}
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
