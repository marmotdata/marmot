package runs

import (
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/user"
)

type Handler struct {
	runService  runs.Service
	userService user.Service
	authService auth.Service
	config      *config.Config
}

func NewHandler(runService runs.Service, userService user.Service, authService auth.Service, config *config.Config) *Handler {
	return &Handler{
		runService:  runService,
		userService: userService,
		authService: authService,
		config:      config,
	}
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/runs/start",
			Method:  http.MethodPost,
			Handler: h.startRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/runs/complete",
			Method:  http.MethodPost,
			Handler: h.completeRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/runs/assets/batch",
			Method:  http.MethodPost,
			Handler: h.batchCreateAssets,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/pipelines/{pipelineName}",
			Method:  http.MethodDelete,
			Handler: h.destroyPipeline,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/runs/cleanup",
			Method:  http.MethodPost,
			Handler: h.cleanupStaleRuns,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/runs",
			Method:  http.MethodGet,
			Handler: h.listRuns,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "view"),
				common.WithRateLimit(h.config, 100, 60), // 100 requests per 60 seconds
			},
		},
		{
			Path:    "/api/v1/runs/{id}",
			Method:  http.MethodGet,
			Handler: h.getRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "view"),
			},
		},
		{
			Path:    "/api/v1/runs/entities/{id}",
			Method:  http.MethodGet,
			Handler: h.getRunEntities,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userService, h.authService, h.config),
				common.RequirePermission(h.userService, "ingestion", "view"),
				common.WithRateLimit(h.config, 30, 60), // 30 requests per 60 seconds
			},
		},
	}
}

