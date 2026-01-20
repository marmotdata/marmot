package notifications

import (
	"net/http"
	"strconv"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/notification"
)

func (h *Handler) getSummary(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	summary, err := h.notificationService.GetSummary(r.Context(), usr.ID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to get notification summary")
		return
	}

	common.RespondJSON(w, http.StatusOK, summary)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	filter := notification.NotificationFilter{
		UserID: usr.ID,
		Type:   r.URL.Query().Get("type"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  limit,
		Offset: offset,
	}

	if readParam := r.URL.Query().Get("read"); readParam != "" {
		read := readParam == "true"
		filter.ReadOnly = &read
	}

	result, err := h.notificationService.List(r.Context(), filter)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to list notifications")
		return
	}

	response := map[string]interface{}{
		"notifications": result.Notifications,
		"limit":         limit,
	}

	if result.Total >= 0 {
		response["total"] = result.Total
		response["offset"] = offset
	}

	if result.NextCursor != "" {
		response["next_cursor"] = result.NextCursor
	}

	common.RespondJSON(w, http.StatusOK, response)
}

func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id := r.PathValue("id")
	n, err := h.notificationService.Get(r.Context(), id)
	if err != nil {
		if err == notification.ErrNotificationNotFound {
			common.RespondError(w, http.StatusNotFound, "Notification not found")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to get notification")
		return
	}

	if n.UserID != usr.ID {
		common.RespondError(w, http.StatusForbidden, "Access denied")
		return
	}

	common.RespondJSON(w, http.StatusOK, n)
}

func (h *Handler) markAsRead(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id := r.PathValue("id")
	if err := h.notificationService.MarkAsRead(r.Context(), id, usr.ID); err != nil {
		if err == notification.ErrNotificationNotFound {
			common.RespondError(w, http.StatusNotFound, "Notification not found")
			return
		}
		if err == notification.ErrUnauthorized {
			common.RespondError(w, http.StatusForbidden, "Access denied")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to mark notification as read")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

func (h *Handler) markAllAsRead(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.notificationService.MarkAllAsRead(r.Context(), usr.ID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to mark all notifications as read")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}

func (h *Handler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	id := r.PathValue("id")
	if err := h.notificationService.Delete(r.Context(), id, usr.ID); err != nil {
		if err == notification.ErrNotificationNotFound {
			common.RespondError(w, http.StatusNotFound, "Notification not found")
			return
		}
		if err == notification.ErrUnauthorized {
			common.RespondError(w, http.StatusForbidden, "Access denied")
			return
		}
		common.RespondError(w, http.StatusInternalServerError, "Failed to delete notification")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Notification deleted"})
}

func (h *Handler) clearReadNotifications(w http.ResponseWriter, r *http.Request) {
	usr, ok := common.GetAuthenticatedUser(r.Context())
	if !ok {
		common.RespondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.notificationService.DeleteAllRead(r.Context(), usr.ID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "Failed to clear read notifications")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "Read notifications cleared"})
}
