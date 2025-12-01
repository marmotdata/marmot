package schedules

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/runs"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	service    *runs.ScheduleService
	runService runs.Service
	userSvc    user.Service
	authSvc    auth.Service
	encryptor  *crypto.Encryptor
	config     *config.Config
}

func NewHandler(service *runs.ScheduleService, runService runs.Service, userSvc user.Service, authSvc auth.Service, encryptor *crypto.Encryptor, config *config.Config) *Handler {
	return &Handler{
		service:    service,
		runService: runService,
		userSvc:    userSvc,
		authSvc:    authSvc,
		encryptor:  encryptor,
		config:     config,
	}
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
			Path:    "/api/v1/ingestion/schedules",
			Method:  http.MethodPost,
			Handler: h.createSchedule,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				common.WithAuth(h.userSvc, h.authSvc, h.config),
				common.RequirePermission(h.userSvc, "ingestion", "manage"),
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
}

type ValidateConfigResponse struct {
	Valid  bool                     `json:"valid"`
	Errors []common.ValidationError `json:"errors,omitempty"`
}

// @Summary Validate plugin configuration
// @Tags ingestion
// @Accept json
// @Produce json
// @Param config body ValidateConfigRequest true "Config to validate"
// @Success 200 {object} ValidateConfigResponse
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Router /api/v1/ingestion/validate [post]
func (h *Handler) validateConfig(w http.ResponseWriter, r *http.Request) {
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
		if validationErrs, ok := err.(plugin.ValidationErrors); ok {
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

type CreateScheduleRequest struct{
	Name           string                 `json:"name"`
	PluginID       string                 `json:"plugin_id"`
	Config         map[string]interface{} `json:"config"`
	CronExpression string                 `json:"cron_expression"`
	Enabled        bool                   `json:"enabled"`
}

type UpdateScheduleRequest struct {
	Name           string                 `json:"name"`
	PluginID       string                 `json:"plugin_id"`
	Config         map[string]interface{} `json:"config"`
	CronExpression string                 `json:"cron_expression"`
	Enabled        bool                   `json:"enabled"`
}

type ListSchedulesResponse struct {
	Schedules []*runs.Schedule `json:"schedules"`
	Total     int                   `json:"total"`
	Limit     int                   `json:"limit"`
	Offset    int                   `json:"offset"`
}

type ListJobRunsResponse struct {
	Runs   []*runs.JobRun `json:"runs"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// @Summary Create a new ingestion schedule
// @Tags ingestion
// @Accept json
// @Produce json
// @Param schedule body CreateScheduleRequest true "Schedule configuration"
// @Success 201 {object} runs.Schedule
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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

	schedule, err := h.service.CreateSchedule(
		r.Context(),
		req.Name,
		req.PluginID,
		req.Config,
		req.CronExpression,
		req.Enabled,
		createdBy,
	)

	if err != nil {
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

	if h.encryptor != nil {
		if err := runs.DecryptScheduleConfig(schedule, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to decrypt config")
		}
	}

	common.RespondJSON(w, http.StatusCreated, schedule)
}

// @Summary List ingestion schedules
// @Tags ingestion
// @Produce json
// @Param enabled query boolean false "Filter by enabled status"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} ListSchedulesResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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

	schedules, total, err := h.service.ListSchedules(r.Context(), enabled, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list schedules")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list schedules")
		return
	}

	if h.encryptor != nil {
		for _, schedule := range schedules {
			if err := runs.DecryptScheduleConfig(schedule, h.encryptor); err != nil {
				log.Error().Err(err).Str("schedule_id", schedule.ID).Msg("Failed to decrypt config")
			}
		}
	}

	common.RespondJSON(w, http.StatusOK, ListSchedulesResponse{
		Schedules: schedules,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	})
}

// @Summary Get an ingestion schedule by ID
// @Tags ingestion
// @Produce json
// @Param id path string true "Schedule ID"
// @Success 200 {object} runs.Schedule
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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

	if h.encryptor != nil {
		if err := runs.DecryptScheduleConfig(schedule, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to decrypt config")
		}
	}

	common.RespondJSON(w, http.StatusOK, schedule)
}

// @Summary Update an ingestion schedule
// @Tags ingestion
// @Accept json
// @Produce json
// @Param id path string true "Schedule ID"
// @Param schedule body UpdateScheduleRequest true "Updated schedule configuration"
// @Success 200 {object} runs.Schedule
// @Failure 400 {object} common.ErrorResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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

	schedule, err := h.service.UpdateSchedule(
		r.Context(),
		id,
		req.Name,
		req.PluginID,
		req.Config,
		req.CronExpression,
		req.Enabled,
	)

	if err != nil {
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

	if h.encryptor != nil {
		if err := runs.DecryptScheduleConfig(schedule, h.encryptor); err != nil {
			log.Error().Err(err).Msg("Failed to decrypt config")
		}
	}

	common.RespondJSON(w, http.StatusOK, schedule)
}

// @Summary Delete an ingestion schedule
// @Tags ingestion
// @Param id path string true "Schedule ID"
// @Success 204
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/v1/ingestion/schedules/{id} [delete]
func (h *Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
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
// @Success 201 {object} runs.JobRun
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
// @Router /api/v1/ingestion/schedules/{id}/trigger [post]
func (h *Handler) triggerSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Schedule ID is required")
		return
	}

	_, err := h.service.GetSchedule(r.Context(), id)
	if err != nil {
		if err == runs.ErrScheduleNotFound {
			common.RespondError(w, http.StatusNotFound, "Schedule not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get schedule")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get schedule")
		return
	}

	run, err := h.service.CreateJobRun(r.Context(), &id)
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
// @Success 200 {object} ListJobRunsResponse
// @Failure 401 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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
// @Success 200 {object} runs.JobRun
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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
// @Success 204
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} common.ErrorResponse
// @Failure 404 {object} common.ErrorResponse
// @Failure 500 {object} common.ErrorResponse
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

