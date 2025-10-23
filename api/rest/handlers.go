package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
)

// Handler handles REST API requests
type Handler struct {
	service domain.NotificationService
	logger  *logging.Logger
}

// NewHandler creates a new REST handler
func NewHandler(service domain.NotificationService, logger *logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// SendNotification handles POST /api/v1/notifications
func (h *Handler) SendNotification(w http.ResponseWriter, r *http.Request) {
	var req SendNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Errorf("REST: Failed to decode request body - error=%v", err)
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.logger.Errorf("REST: Request validation failed - error=%v", err)
		respondError(w, http.StatusBadRequest, "validation failed", err)
		return
	}

	// Convert to domain notification
	notification := req.ToNotification()

	// Log incoming request
	h.logger.Infof("REST: Received notification request - type=%s, account=%s, recipients=%d, subject=%s",
		notification.Type, notification.Account, len(notification.Recipients), notification.Subject)

	// Send notification
	result, err := h.service.Send(r.Context(), notification)
	if err != nil {
		h.logger.Errorf("REST: Failed to send notification - type=%s, account=%s, error=%v",
			notification.Type, notification.Account, err)
		respondError(w, http.StatusInternalServerError, "failed to send notification", err)
		return
	}

	// Log success
	h.logger.Infof("REST: Notification queued successfully - id=%s, type=%s, recipients=%d",
		result.NotificationID, notification.Type, len(notification.Recipients))

	respondJSON(w, http.StatusAccepted, SendNotificationResponse{
		Result: NotificationResultFromDomain(result),
	})
}

// SendBatchNotifications handles POST /api/v1/notifications/batch
func (h *Handler) SendBatchNotifications(w http.ResponseWriter, r *http.Request) {
	var req SendBatchNotificationsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Errorf("REST: Failed to decode batch request body - error=%v", err)
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	h.logger.Infof("REST: Received batch notification request - count=%d", len(req.Notifications))

	// Validate and convert to domain notifications
	notifications := make([]*domain.Notification, 0, len(req.Notifications))
	for _, notifReq := range req.Notifications {
		if err := notifReq.Validate(); err != nil {
			h.logger.Errorf("REST: Batch request validation failed - error=%v", err)
			respondError(w, http.StatusBadRequest, "validation failed", err)
			return
		}
		notifications = append(notifications, notifReq.ToNotification())
	}

	// Send batch
	results, err := h.service.SendBatch(r.Context(), notifications)
	if err != nil {
		h.logger.Errorf("REST: Failed to send batch notifications - error=%v", err)
		respondError(w, http.StatusInternalServerError, "failed to send batch notifications", err)
		return
	}

	// Count successes
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	h.logger.Infof("REST: Batch notification completed - total=%d, successful=%d, failed=%d",
		len(notifications), successCount, len(notifications)-successCount)

	// Convert results
	apiResults := make([]NotificationResult, 0, len(results))
	for _, result := range results {
		apiResults = append(apiResults, NotificationResultFromDomain(result))
	}

	respondJSON(w, http.StatusAccepted, SendBatchNotificationsResponse{
		Results: apiResults,
	})
}

// GetNotification handles GET /api/v1/notifications/{id}
func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	notification, err := h.service.GetNotification(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "notification not found", err)
		return
	}

	respondJSON(w, http.StatusOK, NotificationFromDomain(notification))
}

// ListNotifications handles GET /api/v1/notifications
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	filter := parseNotificationFilter(r)

	notifications, err := h.service.ListNotifications(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list notifications", err)
		return
	}

	// Convert to API format
	apiNotifications := make([]Notification, 0, len(notifications))
	for _, notif := range notifications {
		apiNotifications = append(apiNotifications, NotificationFromDomain(notif))
	}

	respondJSON(w, http.StatusOK, ListNotificationsResponse{
		Notifications: apiNotifications,
		Total:         int64(len(apiNotifications)),
	})
}

// CancelNotification handles DELETE /api/v1/notifications/{id}
func (h *Handler) CancelNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.service.CancelNotification(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to cancel notification", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "notification canceled successfully",
	})
}

// RetryNotification handles POST /api/v1/notifications/{id}/retry
func (h *Handler) RetryNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	result, err := h.service.RetryNotification(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retry notification", err)
		return
	}

	respondJSON(w, http.StatusOK, RetryNotificationResponse{
		Result: NotificationResultFromDomain(result),
	})
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stats", err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetNotifiers handles GET /api/v1/notifiers
func (h *Handler) GetNotifiers(w http.ResponseWriter, r *http.Request) {
	h.logger.Infof("REST: Received request for available notifiers")

	notifiers, err := h.service.GetNotifiers(r.Context())
	if err != nil {
		h.logger.Errorf("REST: Failed to get notifiers - error=%v", err)
		respondError(w, http.StatusInternalServerError, "failed to get notifiers", err)
		return
	}

	respondJSON(w, http.StatusOK, notifiers)
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "notifier",
		"time":    time.Now().UTC(),
	})
}

// parseNotificationFilter parses query parameters into a NotificationFilter
func parseNotificationFilter(r *http.Request) *domain.NotificationFilter {
	query := r.URL.Query()
	filter := &domain.NotificationFilter{}

	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	// Parse types
	if types := query["type"]; len(types) > 0 {
		for _, t := range types {
			filter.Types = append(filter.Types, domain.NotificationType(t))
		}
	}

	// Parse statuses
	if statuses := query["status"]; len(statuses) > 0 {
		for _, s := range statuses {
			filter.Statuses = append(filter.Statuses, domain.NotificationStatus(s))
		}
	}

	// Parse recipients
	if recipients := query["recipient"]; len(recipients) > 0 {
		filter.Recipients = recipients
	}

	return filter
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string, err error) {
	errMsg := message
	if err != nil {
		errMsg = message + ": " + err.Error()
	}

	respondJSON(w, status, map[string]interface{}{
		"error":   message,
		"details": errMsg,
	})
}
