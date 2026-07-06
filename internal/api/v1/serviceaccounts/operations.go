package serviceaccounts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/serviceaccount"
	"github.com/rs/zerolog/log"
)

type createServiceAccountRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	RoleIDs     []string `json:"role_ids,omitempty"`
} // @name CreateServiceAccountRequest

type updateServiceAccountRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	RoleIDs     []string `json:"role_ids,omitempty"`
} // @name UpdateServiceAccountRequest

type createAPIKeyRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
} // @name CreateServiceAccountAPIKeyRequest

// @Summary List service accounts
// @Description Get all service accounts
// @Tags service_accounts
// @Produce json
// @Success 200 {array} serviceaccount.ServiceAccount
// @Failure 500 {object} common.ErrorResponse
// @Router /service-accounts [get]
func (h *Handler) listServiceAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.svcService.List(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list service accounts")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list service accounts")
		return
	}
	if accounts == nil {
		accounts = []*serviceaccount.ServiceAccount{}
	}
	common.RespondJSON(w, http.StatusOK, accounts)
}

// @Summary Create service account
// @Description Create a new service account
// @Tags service_accounts
// @Accept json
// @Produce json
// @Param account body createServiceAccountRequest true "Service account"
// @Success 201 {object} serviceaccount.ServiceAccount
// @Failure 400 {object} common.ErrorResponse
// @Router /service-accounts [post]
func (h *Handler) createServiceAccount(w http.ResponseWriter, r *http.Request) {
	var req createServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}

	var createdBy *string
	if u, ok := common.GetAuthenticatedUser(r.Context()); ok {
		createdBy = &u.ID
	}

	sa, err := h.svcService.Create(r.Context(), serviceaccount.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		RoleIDs:     req.RoleIDs,
	}, createdBy)
	if err != nil {
		if errors.Is(err, serviceaccount.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "Service account already exists")
			return
		}
		log.Error().Err(err).Msg("Failed to create service account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create service account")
		return
	}

	common.RespondJSON(w, http.StatusCreated, sa)
}

// @Summary Get service account
// @Description Get a service account by ID
// @Tags service_accounts
// @Produce json
// @Param id path string true "Service account ID"
// @Success 200 {object} serviceaccount.ServiceAccount
// @Failure 404 {object} common.ErrorResponse
// @Router /service-accounts/{id} [get]
func (h *Handler) getServiceAccount(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/service-accounts/", "")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID required")
		return
	}

	sa, err := h.svcService.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, serviceaccount.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Service account not found")
			return
		}
		log.Error().Err(err).Str("id", id).Msg("Failed to get service account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to get service account")
		return
	}

	common.RespondJSON(w, http.StatusOK, sa)
}

// @Summary Update service account
// @Description Update a service account
// @Tags service_accounts
// @Accept json
// @Produce json
// @Param id path string true "Service account ID"
// @Param account body updateServiceAccountRequest true "Update fields"
// @Success 200 {object} serviceaccount.ServiceAccount
// @Failure 404 {object} common.ErrorResponse
// @Router /service-accounts/{id} [patch]
func (h *Handler) updateServiceAccount(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/service-accounts/", "")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID required")
		return
	}

	var req updateServiceAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sa, err := h.svcService.Update(r.Context(), id, serviceaccount.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Active:      req.Active,
		RoleIDs:     req.RoleIDs,
	})
	if err != nil {
		if errors.Is(err, serviceaccount.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Service account not found")
			return
		}
		if errors.Is(err, serviceaccount.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "Service account name already exists")
			return
		}
		log.Error().Err(err).Str("id", id).Msg("Failed to update service account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to update service account")
		return
	}

	common.RespondJSON(w, http.StatusOK, sa)
}

// @Summary Delete service account
// @Description Soft-delete a service account
// @Tags service_accounts
// @Param id path string true "Service account ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Router /service-accounts/{id} [delete]
func (h *Handler) deleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/v1/service-accounts/", "")
	if id == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID required")
		return
	}

	if err := h.svcService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, serviceaccount.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Service account not found")
			return
		}
		log.Error().Err(err).Str("id", id).Msg("Failed to delete service account")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete service account")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary List API keys for a service account
// @Description Get all API keys for a service account
// @Tags service_accounts
// @Produce json
// @Param id path string true "Service account ID"
// @Success 200 {array} serviceaccount.APIKey
// @Router /service-accounts/{id}/api-keys [get]
func (h *Handler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	saID := extractID(r.URL.Path, "/api/v1/service-accounts/", "/api-keys")
	if saID == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID required")
		return
	}

	keys, err := h.svcService.ListAPIKeys(r.Context(), saID)
	if err != nil {
		log.Error().Err(err).Str("sa_id", saID).Msg("Failed to list API keys")
		common.RespondError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}
	if keys == nil {
		keys = []*serviceaccount.APIKey{}
	}
	common.RespondJSON(w, http.StatusOK, keys)
}

// @Summary Create API key for a service account
// @Description Create a new API key. The plaintext key is only returned once.
// @Tags service_accounts
// @Accept json
// @Produce json
// @Param id path string true "Service account ID"
// @Param key body createAPIKeyRequest true "API key details"
// @Success 201 {object} serviceaccount.APIKey
// @Failure 400 {object} common.ErrorResponse
// @Router /service-accounts/{id}/api-keys [post]
func (h *Handler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	saID := extractID(r.URL.Path, "/api/v1/service-accounts/", "/api-keys")
	if saID == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID required")
		return
	}

	var req createAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		common.RespondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ExpiresInDays < 0 {
		common.RespondError(w, http.StatusBadRequest, "expires_in_days must not be negative")
		return
	}

	var expiresIn *time.Duration
	if req.ExpiresInDays > 0 {
		d := time.Duration(req.ExpiresInDays) * 24 * time.Hour
		expiresIn = &d
	}

	key, err := h.svcService.CreateAPIKey(r.Context(), saID, req.Name, expiresIn)
	if err != nil {
		if errors.Is(err, serviceaccount.ErrAPIKeyLimitReached) {
			common.RespondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, serviceaccount.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "API key name already exists")
			return
		}
		log.Error().Err(err).Str("sa_id", saID).Msg("Failed to create API key")
		common.RespondError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	common.RespondJSON(w, http.StatusCreated, key)
}

// @Summary Delete an API key
// @Description Delete an API key for a service account
// @Tags service_accounts
// @Param id path string true "Service account ID"
// @Param keyId path string true "API key ID"
// @Success 204 "No Content"
// @Failure 404 {object} common.ErrorResponse
// @Router /service-accounts/{id}/api-keys/{keyId} [delete]
func (h *Handler) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/service-accounts/"), "/api-keys/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		common.RespondError(w, http.StatusBadRequest, "Service account ID and key ID required")
		return
	}
	saID, keyID := parts[0], parts[1]

	if err := h.svcService.DeleteAPIKey(r.Context(), saID, keyID); err != nil {
		if errors.Is(err, serviceaccount.ErrKeyNotFound) {
			common.RespondError(w, http.StatusNotFound, "API key not found")
			return
		}
		log.Error().Err(err).Str("sa_id", saID).Str("key_id", keyID).Msg("Failed to delete API key")
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete API key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func extractID(path, prefix, suffix string) string {
	id := strings.TrimPrefix(path, prefix)
	if suffix != "" {
		id = strings.TrimSuffix(id, suffix)
		// also handle sub-paths like /{id}/api-keys
		if idx := strings.Index(id, "/"); idx != -1 {
			id = id[:idx]
		}
	}
	return id
}
