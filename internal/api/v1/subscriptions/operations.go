package subscriptions

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/subscription"
)

func (h *Handler) getSubscription(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id query parameter is required")
		return
	}

	sub, err := h.svc.GetByAssetAndUser(r.Context(), assetID, usr.ID)
	if err != nil {
		if errors.Is(err, subscription.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to get subscription")
		return
	}

	common.RespondJSON(w, http.StatusOK, sub)
}

func (h *Handler) listSubscriptions(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	subs, err := h.svc.ListByUser(r.Context(), usr.ID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list subscriptions")
		return
	}

	if subs == nil {
		subs = []*subscription.SubscriptionWithAsset{}
	}

	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"subscriptions": subs,
	})
}

func (h *Handler) createSubscription(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var input struct {
		AssetID           string   `json:"asset_id"`
		NotificationTypes []string `json:"notification_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if input.AssetID == "" {
		common.RespondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}

	if len(input.NotificationTypes) == 0 {
		input.NotificationTypes = []string{"asset_change", "schema_change"}
	}

	sub, err := h.svc.Create(r.Context(), input.AssetID, usr.ID, input.NotificationTypes)
	if err != nil {
		if errors.Is(err, subscription.ErrAlreadyExists) {
			common.RespondError(w, http.StatusConflict, "Already subscribed to this asset")
			return
		}
		if subscription.IsValidationError(err) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to create subscription")
		return
	}

	common.RespondJSON(w, http.StatusCreated, sub)
}

func (h *Handler) updateSubscription(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id := r.PathValue("id")

	var input struct {
		NotificationTypes []string `json:"notification_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		common.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sub, err := h.svc.Update(r.Context(), id, usr.ID, input.NotificationTypes)
	if err != nil {
		if errors.Is(err, subscription.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Subscription not found")
			return
		}
		if errors.Is(err, subscription.ErrUnauthorized) {
			common.RespondError(w, http.StatusForbidden, "Not authorized to modify this subscription")
			return
		}
		if subscription.IsValidationError(err) {
			common.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to update subscription")
		return
	}

	common.RespondJSON(w, http.StatusOK, sub)
}

func (h *Handler) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id := r.PathValue("id")

	if err := h.svc.Delete(r.Context(), id, usr.ID); err != nil {
		if errors.Is(err, subscription.ErrNotFound) {
			common.RespondError(w, http.StatusNotFound, "Subscription not found")
			return
		}
		if errors.Is(err, subscription.ErrUnauthorized) {
			common.RespondError(w, http.StatusForbidden, "Not authorized to delete this subscription")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete subscription")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
