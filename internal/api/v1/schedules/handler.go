package schedules

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/pkg/config"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
)

// servedScheduleMessage explains why a served source cannot be edited or run from Marmot: its plugin and credentials live with the process that serves it.
const servedScheduleMessage = "This source is served by a plugin running outside Marmot; change or run it where that plugin runs"

// RunCRDTrigger allows the handler to trigger operator-managed runs via the K8s API.
type RunCRDTrigger interface {
	TriggerRun(ctx context.Context, name string) error
}

type Handler struct {
	service              *runs.ScheduleService
	runService           runs.Service
	userSvc              user.Service
	authSvc              auth.Service
	encryptor            *crypto.Encryptor
	config               *config.Config
	encryptionConfigured bool
	runCRDTrigger        RunCRDTrigger
}

func NewHandler(service *runs.ScheduleService, runService runs.Service, userSvc user.Service, authSvc auth.Service, encryptor *crypto.Encryptor, config *config.Config, encryptionConfigured bool) *Handler {
	return &Handler{
		service:              service,
		runService:           runService,
		userSvc:              userSvc,
		authSvc:              authSvc,
		encryptor:            encryptor,
		config:               config,
		encryptionConfigured: encryptionConfigured,
	}
}

// SetRunCRDTrigger sets the K8s CRD trigger for operator-managed schedules.
func (h *Handler) SetRunCRDTrigger(trigger RunCRDTrigger) {
	h.runCRDTrigger = trigger
}

func (h *Handler) Routes() []common.Route {
	return []common.Route{
		{
			Path:    "/api/v1/ingestion/validate",
			Method:  http.MethodPost,
			Handler: h.validateConfig,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
		{
			// manage rather than view: a test launches the plugin and connects out with whatever settings it is given.
			Path:    "/api/v1/ingestion/test-connection",
			Method:  http.MethodPost,
			Handler: h.testConnection,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
				common.WithRateLimit(h.config, 30, 60),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules",
			Method:  http.MethodPost,
			Handler: h.createSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
				common.RequireEncryption(h.encryptionConfigured),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules",
			Method:  http.MethodGet,
			Handler: h.listSchedules,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules/{id}",
			Method:  http.MethodGet,
			Handler: h.getSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules/{id}",
			Method:  http.MethodPut,
			Handler: h.updateSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
				common.RequireEncryption(h.encryptionConfigured),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules/{id}",
			Method:  http.MethodDelete,
			Handler: h.deleteSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/ingestion/schedules/{id}/trigger",
			Method:  http.MethodPost,
			Handler: h.triggerSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
				common.RequireEncryption(h.encryptionConfigured),
			},
		},
		{
			Path:    "/api/v1/ingestion/runs",
			Method:  http.MethodGet,
			Handler: h.listJobRuns,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
		{
			Path:    "/api/v1/ingestion/runs/{id}",
			Method:  http.MethodGet,
			Handler: h.getJobRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
		{
			Path:    "/api/v1/ingestion/runs/{id}/cancel",
			Method:  http.MethodPost,
			Handler: h.cancelJobRun,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
			},
		},
		{
			Path:    "/api/v1/ingestion/runs/{id}/entities",
			Method:  http.MethodGet,
			Handler: h.getJobRunEntities,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "view"),
			},
		},
	}
}

type ValidateConfigRequest struct {
	PluginID string                 `json:"plugin_id"`
	Config   map[string]interface{} `json:"config"`
} // @name ValidateConfigRequest

type ValidateConfigResponse struct {
	Valid  bool                     `json:"valid"`
	Errors []common.ValidationError `json:"errors,omitempty"`
} // @name ValidateConfigResponse

// @Summary Validate plugin configuration
// @Tags ingestion
// @Accept json
// @Produce json
// @Param config body ValidateConfigRequest true "Config to validate"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ValidateConfigResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @ID postIngestionValidate
// @Router /api/v1/ingestion/validate [post]
func (h *Handler) validateConfig(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePluginsReady(w) {
		return
	}

	var req ValidateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PluginID == "" {
		common.RespondError(w, http.StatusBadRequest, "Plugin ID is required")
		return
	}

	registry := plugin.GetRegistry()
	entry, err := registry.Get(req.PluginID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Unknown plugin ID")
		return
	}

	_, err = entry.Source.Validate(req.Config)
	if err != nil {
		if validationErrs, ok := err.(pluginsdk.ValidationErrors); ok {
			apiErrors := make([]common.ValidationError, len(validationErrs.Errors))
			for i, e := range validationErrs.Errors {
				apiErrors[i] = common.ValidationError{
					Field:   e.Field,
					Message: e.Message,
				}
			}
			common.RespondJSON(w, http.StatusOK, ValidateConfigResponse{
				Valid:  false,
				Errors: apiErrors,
			})
			return
		}

		common.RespondJSON(w, http.StatusOK, ValidateConfigResponse{
			Valid: false,
			Errors: []common.ValidationError{
				{
					Field:   "config",
					Message: err.Error(),
				},
			},
		})
		return
	}

	common.RespondJSON(w, http.StatusOK, ValidateConfigResponse{
		Valid: true,
	})
}

// TestConnectionRequest is a plugin and the settings to try. ScheduleID names the pipeline being edited, so secrets it returned masked are tested with their stored values.
type TestConnectionRequest struct {
	PluginID   string                 `json:"plugin_id"`
	Config     map[string]interface{} `json:"config"`
	ScheduleID string                 `json:"schedule_id,omitempty"`
} // @name TestConnectionRequest

// TestConnectionResponse reports a connection test. Errors lists settings that failed validation, in which case the plugin was never started; otherwise Result says what the plugin could see.
type TestConnectionResponse struct {
	Valid  bool                       `json:"valid"`
	Errors []common.ValidationError   `json:"errors,omitempty"`
	Result *runs.ConnectionTestResult `json:"result,omitempty"`
} // @name TestConnectionResponse

// connectionTests bounds how many connection tests run at once, since each one starts a plugin process that connects out.
var connectionTests = make(chan struct{}, 4)

// @Summary Test a plugin connection
// @Description Validates the settings, then starts the plugin and runs a discovery that is thrown away, stopping after 30 seconds. Nothing is written to the catalog. Plugins have no separate connect step, so this is the closest check that the settings work.
// @Tags ingestion
// @Accept json
// @Produce json
// @Param request body TestConnectionRequest true "Plugin and settings to test"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} TestConnectionResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 403 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 429 {object} common.ErrorResponse
// @Failure 503 {object} common.ErrorResponse
// @ID postIngestionTestConnection
// @Router /api/v1/ingestion/test-connection [post]
func (h *Handler) testConnection(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePluginsReady(w) {
		return
	}

	var req TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.PluginID == "" {
		common.RespondError(w, http.StatusBadRequest, "Plugin ID is required")
		return
	}
	entry, err := plugin.GetRegistry().Get(req.PluginID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "Unknown plugin ID")
		return
	}
	if req.Config == nil {
		req.Config = map[string]interface{}{}
	}

	if req.ScheduleID != "" {
		existing, err := h.service.GetSchedule(r.Context(), req.ScheduleID)
		if err != nil {
			if errors.Is(err, runs.ErrScheduleNotFound) {
				common.RespondError(w, http.StatusNotFound, "Schedule not found")
				return
			}
			log.Error().Err(err).Msg("Failed to get schedule")
			common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
			return
		}
		var previous map[string]interface{}
		if existing.PluginID == req.PluginID {
			if err := runs.DecryptScheduleConfig(existing, h.encryptor); err != nil {
				log.Error().Err(err).Msg("Failed to decrypt config")
				common.RespondError(w, http.StatusInternalServerError, "Failed to decrypt config")
				return
			}
			previous = existing.Config
		}
		if err := runs.RestoreMaskedScheduleConfig(req.PluginID, req.Config, previous); err != nil {
			common.RespondError(w, http.StatusBadRequest, "Unknown plugin")
			return
		}
	}

	validated, err := entry.Source.Validate(req.Config)
	if err != nil {
		var validationErrs pluginsdk.ValidationErrors
		if errors.As(err, &validationErrs) {
			apiErrors := make([]common.ValidationError, len(validationErrs.Errors))
			for i, e := range validationErrs.Errors {
				apiErrors[i] = common.ValidationError{Field: e.Field, Message: e.Message}
			}
			common.RespondJSON(w, http.StatusOK, TestConnectionResponse{Errors: apiErrors})
			return
		}
		common.RespondJSON(w, http.StatusOK, TestConnectionResponse{
			Errors: []common.ValidationError{{Field: "config", Message: err.Error()}},
		})
		return
	}

	select {
	case connectionTests <- struct{}{}:
		defer func() { <-connectionTests }()
	default:
		w.Header().Set("Retry-After", "10")
		common.RespondError(w, http.StatusTooManyRequests, "Other connection tests are running, try again shortly")
		return
	}

	result := runs.TestConnection(r.Context(), entry.Source, validated, runs.DefaultConnectionTestTimeout)
	common.RespondJSON(w, http.StatusOK, TestConnectionResponse{Valid: true, Result: result})
}

type CreateScheduleRequest struct {
	Name           string                 `json:"name"`
	PluginID       string                 `json:"plugin_id"`
	Config         map[string]interface{} `json:"config"`
	CronExpression string                 `json:"cron_expression"`
	Enabled        bool                   `json:"enabled"`
	// Queryable lets agents and users query this source through Marmot; the
	// plugin must support queries.
	Queryable bool `json:"queryable"`
	// IngestOnQuery refreshes the source's catalog in the background after
	// data queries, rate-limited per source.
	IngestOnQuery bool `json:"ingest_on_query"`
	// Description says what the pipeline catalogs. Omit it to leave it unchanged; an empty string clears it.
	Description *string `json:"description,omitempty"`
	// OwnerType is "user" or "team" and, with OwnerID, names who is told when a run fails. Omit both to leave the owner unchanged; send both empty to clear it.
	OwnerType *string `json:"owner_type,omitempty"`
	OwnerID   *string `json:"owner_id,omitempty"`
} // @name CreateScheduleRequest

type UpdateScheduleRequest struct {
	Name           string                 `json:"name"`
	PluginID       string                 `json:"plugin_id"`
	Config         map[string]interface{} `json:"config"`
	CronExpression string                 `json:"cron_expression"`
	Enabled        bool                   `json:"enabled"`
	// Queryable lets agents and users query this source through Marmot; the
	// plugin must support queries.
	Queryable bool `json:"queryable"`
	// IngestOnQuery refreshes the source's catalog in the background after
	// data queries, rate-limited per source.
	IngestOnQuery bool `json:"ingest_on_query"`
	// Description says what the pipeline catalogs. Omit it to leave it unchanged; an empty string clears it.
	Description *string `json:"description,omitempty"`
	// OwnerType is "user" or "team" and, with OwnerID, names who is told when a run fails. Omit both to leave the owner unchanged; send both empty to clear it.
	OwnerType *string `json:"owner_type,omitempty"`
	OwnerID   *string `json:"owner_id,omitempty"`
} // @name UpdateScheduleRequest

type ListSchedulesResponse struct {
	Schedules []*runs.Schedule `json:"schedules"`
	Total     int              `json:"total"`
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
	// PluginCounts counts every pipeline by plugin, whatever the filters, so the plugin gallery can say how many of each are connected.
	PluginCounts map[string]int `json:"plugin_counts"`
} // @name ListSchedulesResponse

type ListJobRunsResponse struct {
	Runs   []*runs.JobRun `json:"runs"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
} // @name ListJobRunsResponse

// @Summary Create a new ingestion schedule
// @Tags ingestion
// @Accept json
// @Produce json
// @Param schedule body CreateScheduleRequest true "Schedule configuration"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} runs.Schedule
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postIngestionSchedules
// @Router /api/v1/ingestion/schedules [post]
func (h *Handler) createSchedule(w http.ResponseWriter, r *http.Request) {
	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	if req.PluginID == "" {
		common.RespondError(w, http.StatusBadRequest, "Plugin ID is required")
		return
	}
	if req.Queryable && !pluginSupportsQuery(req.PluginID) {
		common.RespondError(w, http.StatusBadRequest, "This plugin does not support queries")
		return
	}
	if req.Queryable && !canManageQueries(r) {
		common.RespondError(w, http.StatusForbidden, "Making a source answer queries requires the access:manage permission")
		return
	}

	user, _ := common.GetAuthenticatedUser(r.Context())
	var createdBy *string
	if user != nil {
		createdBy = &user.ID
	}

	if h.encryptor != nil {
		if err := runs.EncryptScheduleConfig(&runs.Schedule{
			PluginID: req.PluginID,
			Config:   req.Config,
		}, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to encrypt config")
			common.RespondError(w, http.StatusInternalServerError, "Failed to encrypt config")
			return
		}
	}

	schedule, err := h.service.CreateScheduleWithDetails(
		r.Context(),
		req.Name,
		req.PluginID,
		req.Config,
		req.CronExpression,
		req.Enabled,
		req.Queryable,
		req.IngestOnQuery,
		createdBy,
		runs.ScheduleDetails{Description: req.Description, OwnerType: req.OwnerType, OwnerID: req.OwnerID},
	)

	if err != nil {
		if errors.Is(err, runs.ErrScheduleOwnerNotFound) {
			common.RespondError(w, http.StatusBadRequest, "Owner must be an existing user or team")
			return
		}
		if err == runs.ErrScheduleNameExists {
			common.RespondError(w, http.StatusConflict, "Schedule with this name already exists")
			return
		}
		if err == runs.ErrInvalidCronExpression {
			common.RespondError(w, http.StatusBadRequest, "Invalid cron expression")
			return
		}
		log.Error().Err(err).Msg("Failed to create schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create schedule")
		return
	}

	runs.MaskScheduleConfig(schedule)

	common.RespondJSON(w, http.StatusCreated, schedule)
}

// @Summary List ingestion schedules
// @Tags ingestion
// @Produce json
// @Param enabled query boolean false "Filter by enabled status"
// @Param search query string false "Match pipeline name or plugin, case-insensitively"
// @Param status query string false "Latest run status: failed, succeeded, cancelled, running (queued or under way) or never (no runs)"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ListSchedulesResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getIngestionSchedules
// @Router /api/v1/ingestion/schedules [get]
func (h *Handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	var enabled *bool
	if enabledStr := r.URL.Query().Get("enabled"); enabledStr != "" {
		enabledVal, _ := strconv.ParseBool(enabledStr)
		enabled = &enabledVal
	}

	status := r.URL.Query().Get("status")
	if !runs.ValidScheduleStatusFilter(status) {
		common.RespondError(w, http.StatusBadRequest, "status must be one of failed, succeeded, cancelled, running or never")
		return
	}

	schedules, total, err := h.service.SearchSchedules(r.Context(), runs.ScheduleFilter{
		Enabled: enabled,
		Search:  r.URL.Query().Get("search"),
		Status:  status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list schedules")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list schedules")
		return
	}

	pluginCounts, err := h.service.CountSchedulesByPlugin(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to count schedules by plugin")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list schedules")
		return
	}

	for _, schedule := range schedules {
		runs.MaskScheduleConfig(schedule)
	}

	common.RespondJSON(w, http.StatusOK, ListSchedulesResponse{
		Schedules:    schedules,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
		PluginCounts: pluginCounts,
	})
}

// @Summary Get an ingestion schedule by ID
// @Tags ingestion
// @Produce json
// @Param id path string true "Schedule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} runs.Schedule
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getIngestionSchedulesID
// @Router /api/v1/ingestion/schedules/{id} [get]
func (h *Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
	}

	schedule, err := h.service.GetSchedule(r.Context(), id)
	if err != nil {
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
		return
	}

	// Without its plugin's spec the config cannot be masked and comes back empty, which the edit form would save over the real one, so ask the caller to retry until plugins have loaded
	if _, err := plugin.GetRegistry().Get(schedule.PluginID); err != nil && !plugin.GetLoadState().Ready() {
		common.RespondError(w, http.StatusServiceUnavailable, "Plugins are still loading")
		return
	}

	runs.MaskScheduleConfig(schedule)

	common.RespondJSON(w, http.StatusOK, schedule)
}

// @Summary Update an ingestion schedule
// @Tags ingestion
// @Accept json
// @Produce json
// @Param id path string true "Schedule ID"
// @Param schedule body UpdateScheduleRequest true "Updated schedule configuration"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} runs.Schedule
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID putIngestionSchedulesID
// @Router /api/v1/ingestion/schedules/{id} [put]
func (h *Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	if req.PluginID == "" {
		common.RespondError(w, http.StatusBadRequest, "Plugin ID is required")
		return
	}
	if req.Queryable && !pluginSupportsQuery(req.PluginID) {
		common.RespondError(w, http.StatusBadRequest, "This plugin does not support queries")
		return
	}

	existing, err := h.service.GetSchedule(r.Context(), id)
	if err != nil {
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
		return
	}

	if existing.Served() {
		common.RespondError(w, http.StatusConflict, servedScheduleMessage)
		return
	}

	if (req.Queryable != existing.Queryable || req.IngestOnQuery != existing.IngestOnQuery) && !canManageQueries(r) {
		common.RespondError(w, http.StatusForbidden, "Changing whether a source answers queries requires the access:manage permission")
		return
	}

	// Secrets are only ever returned masked, so an unchanged secret comes back as the mask and has to be restored from what is stored; switching plugin carries nothing over
	var previous map[string]interface{}
	if existing.PluginID == req.PluginID {
		if err := runs.DecryptScheduleConfig(existing, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to decrypt config")
			common.RespondError(w, http.StatusInternalServerError, "Failed to decrypt config")
			return
		}
		previous = existing.Config
	}
	if err := runs.RestoreMaskedScheduleConfig(req.PluginID, req.Config, previous); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Unknown plugin")
		return
	}

	if h.encryptor != nil {
		if err := runs.EncryptScheduleConfig(&runs.Schedule{
			PluginID: req.PluginID,
			Config:   req.Config,
		}, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to encrypt config")
			common.RespondError(w, http.StatusInternalServerError, "Failed to encrypt config")
			return
		}
	}

	schedule, err := h.service.UpdateScheduleWithDetails(
		r.Context(),
		id,
		req.Name,
		req.PluginID,
		req.Config,
		req.CronExpression,
		req.Enabled,
		req.Queryable,
		req.IngestOnQuery,
		runs.ScheduleDetails{Description: req.Description, OwnerType: req.OwnerType, OwnerID: req.OwnerID},
	)

	if err != nil {
		if errors.Is(err, runs.ErrScheduleOwnerNotFound) {
			common.RespondError(w, http.StatusBadRequest, "Owner must be an existing user or team")
			return
		}
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		if err == runs.ErrScheduleNameExists {
			common.RespondError(w, http.StatusConflict, "Schedule with this name already exists")
			return
		}
		if err == runs.ErrInvalidCronExpression {
			common.RespondError(w, http.StatusBadRequest, "Invalid cron expression")
			return
		}
		log.Error().Err(err).Msg("Failed to update schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update schedule")
		return
	}

	runs.MaskScheduleConfig(schedule)

	common.RespondJSON(w, http.StatusOK, schedule)
}

// @Summary Delete an ingestion schedule
// @Tags ingestion
// @Param id path string true "Schedule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID deleteIngestionSchedulesID
// @Router /api/v1/ingestion/schedules/{id} [delete]
func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
	}

	// Check if user wants to teardown all assets/lineage created by this pipeline
	teardown := r.URL.Query().Get("teardown") == "true"

	if teardown {
		// Get the schedule to find the pipeline name
		schedule, err := h.service.GetSchedule(r.Context(), id)
		if err != nil {
			if err == runs.ErrScheduleNotFound {
				common.RespondError(w, http.StatusNotFound, "Schedule not found")
				return
			}
			log.Error().Err(err).Msg("Failed to get schedule for teardown")
			common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
			return
		}

		// Destroy all entities created by this pipeline
		log.Info().
			Str("schedule_id", id).
			Str("pipeline_name", schedule.Name).
			Msg("Tearing down pipeline entities before deletion")

		destroyResp, err := h.runService.DestroyPipeline(r.Context(), schedule.Name)
		if err != nil {
			log.Error().Err(err).Str("pipeline_name", schedule.Name).Msg("Failed to destroy pipeline entities")
			common.RespondError(w, http.StatusInternalServerError, "Failed to teardown pipeline entities")
			return
		}

		log.Info().
			Str("pipeline_name", schedule.Name).
			Int("assets_deleted", destroyResp.AssetsDeleted).
			Int("lineage_deleted", destroyResp.LineageDeleted).
			Msg("Pipeline entities torn down successfully")
	}

	err := h.service.DeleteSchedule(r.Context(), id)
	if err != nil {
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to delete schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete schedule")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Manually trigger an ingestion schedule
// @Tags ingestion
// @Param id path string true "Schedule ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 201 {object} runs.JobRun
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postIngestionSchedulesIDTrigger
// @Router /api/v1/ingestion/schedules/{id}/trigger [post]
func (h *Handler) triggerSchedule(w http.ResponseWriter, r *http.Request) {
	if !common.RequirePluginsReady(w) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
	}

	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	schedule, err := h.service.GetSchedule(r.Context(), id)
	if err != nil {
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
		return
	}

	if schedule.Served() {
		common.RespondError(w, http.StatusConflict, servedScheduleMessage)
		return
	}

	// For operator-managed schedules, patch the Run CRD annotation via K8s API
	if schedule.ManagedBy != nil && *schedule.ManagedBy != "" {
		if h.runCRDTrigger == nil {
			common.RespondError(w, http.StatusServiceUnavailable, "Operator integration not configured")
			return
		}
		if err := h.runCRDTrigger.TriggerRun(r.Context(), schedule.Name); err != nil {
			log.Error().Err(err).Str("schedule", schedule.Name).Msg("Failed to trigger operator-managed run")
			common.RespondError(w, http.StatusInternalServerError, "Failed to trigger run")
			return
		}
		runs.MaskScheduleConfig(schedule)
		common.RespondJSON(w, http.StatusAccepted, schedule)
		return
	}

	run, err := h.service.CreateJobRun(r.Context(), &id, usr.Username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create job run")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create job run")
		return
	}

	common.RespondJSON(w, http.StatusCreated, run)
}

// @Summary List ingestion job runs
// @Tags ingestion
// @Produce json
// @Param schedule_id query string false "Filter by schedule ID"
// @Param status query string false "Filter by status"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} ListJobRunsResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getIngestionRuns
// @Router /api/v1/ingestion/runs [get]
func (h *Handler) listJobRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	var scheduleID *string
	if sid := r.URL.Query().Get("schedule_id"); sid != "" {
		scheduleID = &sid
	}

	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	runs, total, err := h.service.ListJobRuns(r.Context(), scheduleID, status, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list job runs")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list job runs")
		return
	}

	common.RespondJSON(w, http.StatusOK, ListJobRunsResponse{
		Runs:   runs,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// @Summary Get a job run by ID
// @Tags ingestion
// @Produce json
// @Param id path string true "Job run ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} runs.JobRun
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getIngestionRunsID
// @Router /api/v1/ingestion/runs/{id} [get]
func (h *Handler) getJobRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Job run ID is required")
		return
	}

	run, err := h.service.GetJobRun(r.Context(), id)
	if err != nil {
		if err == runs.ErrJobRunNotFound {
			common.RespondError(w, http.StatusNotFound, "Job run not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get job run")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get job run")
		return
	}

	common.RespondJSON(w, http.StatusOK, run)
}

// @Summary Cancel a running job
// @Tags ingestion
// @Param id path string true "Job run ID"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID postIngestionRunsIDCancel
// @Router /api/v1/ingestion/runs/{id}/cancel [post]
func (h *Handler) cancelJobRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Job run ID is required")
		return
	}

	err := h.service.CancelJobRun(r.Context(), id)
	if err != nil {
		if err == runs.ErrJobRunNotFound {
			common.RespondError(w, http.StatusNotFound, "Job run not found")
			return
		}
		log.Error().Err(err).Msg("Failed to cancel job run")
		common.RespondError(w, http.StatusInternalServerError, "Failed to cancel job run")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary Get entities for a job run
// @Tags ingestion
// @Produce json
// @Param id path string true "Job run ID"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @ID getIngestionRunsIDEntities
// @Router /api/v1/ingestion/runs/{id}/entities [get]
func (h *Handler) getJobRunEntities(w http.ResponseWriter, r *http.Request) {
	jobRunID := r.PathValue("id")
	if jobRunID == "" {
		common.RespondError(w, http.StatusBadRequest, "Job run ID is required")
		return
	}

	pluginRunID, err := h.service.GetJobRunPluginRunID(r.Context(), jobRunID)
	if err != nil {
		if err == runs.ErrJobRunNotFound {
			common.RespondError(w, http.StatusNotFound, "Job run not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get plugin run ID")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get job run")
		return
	}

	if pluginRunID == nil {
		common.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"entities": []interface{}{},
			"total":    0,
			"limit":    10,
			"offset":   0,
		})
		return
	}

	entityType := r.URL.Query().Get("entity_type")
	status := r.URL.Query().Get("status")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	} else if limit > 1000 {
		limit = 1000
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	entities, total, err := h.runService.ListRunEntities(r.Context(), *pluginRunID, entityType, status, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list run entities")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list run entities")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"entities": entities,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// pluginSupportsQuery reports whether a plugin can serve data queries, so a
// source is never marked queryable when no query could ever succeed.
func pluginSupportsQuery(pluginID string) bool {
	entry, err := plugin.GetRegistry().Get(pluginID)
	if err != nil {
		return false
	}
	return entry.Meta.SupportsQuery
}

// canManageQueries reports whether the caller may decide which sources answer
// data queries, which is separate from managing ingestion.
func canManageQueries(r *http.Request) bool {
	p, ok := common.PrincipalFromContext(r.Context())
	return ok && (p.IsAdmin() || p.HasPermission("access", "manage"))
}
