package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/config"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/team"
	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/marmotdata/marmot/internal/core/webhook"
)

// Handler handles webhook API requests.
type Handler struct {
	webhookService *webhook.Service
	teamService    *team.Service
	userService    user.Service
	authService    auth.Service
	config         *config.Config
}

// NewHandler creates a new webhook handler.
func NewHandler(webhookService *webhook.Service, teamService *team.Service, userService user.Service, authService auth.Service, cfg *config.Config) *Handler {
	return &Handler{
		webhookService: webhookService,
		teamService:    teamService,
		userService:    userService,
		authService:    authService,
		config:         cfg,
	}
}

// Routes returns the webhook routes.
func (h *Handler) Routes() []common.Route {
	authMiddleware := common.WithAuth(h.userService, h.authService, h.config)

	return []common.Route{
		{
			Path:    "/api/v1/teams/{id}/webhooks",
			Method:  http.MethodGet,
			Handler: h.listWebhooks,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/webhooks",
			Method:  http.MethodPost,
			Handler: h.createWebhook,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/webhooks/{webhookId}",
			Method:  http.MethodGet,
			Handler: h.getWebhook,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/webhooks/{webhookId}",
			Method:  http.MethodPut,
			Handler: h.updateWebhook,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/webhooks/{webhookId}",
			Method:  http.MethodDelete,
			Handler: h.deleteWebhook,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
		{
			Path:    "/api/v1/teams/{id}/webhooks/{webhookId}/test",
			Method:  http.MethodPost,
			Handler: h.testWebhook,
			Middleware: []func(http.HandlerFunc) http.HandlerFunc{
				authMiddleware,
				h.requireTeamManage(),
			},
		},
	}
}

// requireTeamManage checks if user has teams:manage permission OR is a team owner.
func (h *Handler) requireTeamManage() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u, ok := common.GetAuthenticatedUser(r.Context())
			if !ok {
				common.RespondError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			teamID := r.PathValue("id")
			if teamID == "" {
				common.RespondError(w, http.StatusBadRequest, "Team ID is required")
				return
			}

			// Check system permission first
			hasPerm, err := h.userService.HasPermission(r.Context(), u.ID, "teams", "manage")
			if err == nil && hasPerm {
				next(w, r)
				return
			}

			// Check team ownership
			member, err := h.teamService.GetMember(r.Context(), teamID, u.ID)
			if err == nil && member.Role == team.RoleOwner {
				next(w, r)
				return
			}

			common.RespondError(w, http.StatusForbidden, "Permission denied: requires team owner or teams:manage permission")
		}
	}
}

func (h *Handler) listWebhooks(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")

	webhooks, err := h.webhookService.ListByTeam(r.Context(), teamID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list webhooks")
		return
	}

	if webhooks == nil {
		webhooks = []*webhook.Webhook{}
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"webhooks": webhooks,
	})
}

func (h *Handler) createWebhook(w http.ResponseWriter, r *http.Request) {
	teamID := r.PathValue("id")

	var input webhook.CreateWebhookInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	input.TeamID = teamID

	result, err := h.webhookService.Create(r.Context(), input)
	if err != nil {
		if webhook.IsValidationError(err) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to create webhook")
		return
	}

	common.RespondJSON(w, http.StatusCreated, result)
}

func (h *Handler) getWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookId")

	result, err := h.webhookService.Get(r.Context(), webhookID)
	if err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Webhook not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to get webhook")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

func (h *Handler) updateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookId")

	var input webhook.UpdateWebhookInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.webhookService.Update(r.Context(), webhookID, input)
	if err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Webhook not found")
			return
		}
		if webhook.IsValidationError(err) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to update webhook")
		return
	}

	common.RespondJSON(w, http.StatusOK, result)
}

func (h *Handler) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookId")

	if err := h.webhookService.Delete(r.Context(), webhookID); err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Webhook not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete webhook")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) testWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookId")

	if err := h.webhookService.TestWebhook(r.Context(), webhookID); err != nil {
		if errors.Is(err, webhook.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Webhook not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to send test notification")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Test notification sent",
	})
}
