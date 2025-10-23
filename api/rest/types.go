package rest

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/igodwin/notifier/internal/domain"
)

// SendNotificationRequest is the REST API request for sending a notification
type SendNotificationRequest struct {
	Type         string                 `json:"type"`
	Account      string                 `json:"account,omitempty"` // Optional account name for multi-account configs
	Priority     int                    `json:"priority,omitempty"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body"`
	ContentType  string                 `json:"content_type,omitempty"` // "text" or "html" - auto-detected if not specified
	Recipients   []string               `json:"recipients"`
	CC           []string               `json:"cc,omitempty"`  // Carbon copy recipients (email only)
	BCC          []string               `json:"bcc,omitempty"` // Blind carbon copy recipients (email only)
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ScheduledFor *time.Time             `json:"scheduled_for,omitempty"`
	MaxRetries   int                    `json:"max_retries,omitempty"`
}

// Validate validates the request
func (r *SendNotificationRequest) Validate() error {
	if r.Type == "" {
		return fmt.Errorf("type is required")
	}

	// For email, allow BCC-only (at least one recipient in To, CC, or BCC)
	// For other types, require Recipients
	totalRecipients := len(r.Recipients) + len(r.CC) + len(r.BCC)
	if totalRecipients == 0 {
		return fmt.Errorf("at least one recipient is required (recipients, cc, or bcc)")
	}

	if r.Body == "" {
		return fmt.Errorf("body is required")
	}

	return nil
}

// ToNotification converts the request to a domain notification
func (r *SendNotificationRequest) ToNotification() *domain.Notification {
	maxRetries := r.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default
	}

	// Convert content type, defaulting to text
	contentType := domain.ContentType(r.ContentType)
	if contentType == "" {
		contentType = domain.ContentTypeText
	}

	return &domain.Notification{
		ID:           uuid.New().String(),
		Type:         domain.NotificationType(r.Type),
		Account:      r.Account,
		Priority:     domain.Priority(r.Priority),
		Status:       domain.StatusPending,
		Subject:      r.Subject,
		Body:         r.Body,
		ContentType:  contentType,
		Recipients:   r.Recipients,
		CC:           r.CC,
		BCC:          r.BCC,
		Metadata:     r.Metadata,
		CreatedAt:    time.Now(),
		ScheduledFor: r.ScheduledFor,
		MaxRetries:   maxRetries,
		RetryCount:   0,
	}
}

// SendNotificationResponse is the REST API response for sending a notification
type SendNotificationResponse struct {
	Result NotificationResult `json:"result"`
}

// SendBatchNotificationsRequest is the REST API request for sending multiple notifications
type SendBatchNotificationsRequest struct {
	Notifications []SendNotificationRequest `json:"notifications"`
}

// SendBatchNotificationsResponse is the REST API response for sending multiple notifications
type SendBatchNotificationsResponse struct {
	Results []NotificationResult `json:"results"`
}

// Notification represents a notification in the REST API
type Notification struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Account      string                 `json:"account,omitempty"`
	Priority     int                    `json:"priority"`
	Status       string                 `json:"status"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body"`
	ContentType  string                 `json:"content_type,omitempty"`
	Recipients   []string               `json:"recipients"`
	CC           []string               `json:"cc,omitempty"`
	BCC          []string               `json:"bcc,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	ScheduledFor *time.Time             `json:"scheduled_for,omitempty"`
	SentAt       *time.Time             `json:"sent_at,omitempty"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	LastError    string                 `json:"last_error,omitempty"`
}

// NotificationFromDomain converts a domain notification to API format
func NotificationFromDomain(n *domain.Notification) Notification {
	return Notification{
		ID:           n.ID,
		Type:         string(n.Type),
		Account:      n.Account,
		Priority:     int(n.Priority),
		Status:       string(n.Status),
		Subject:      n.Subject,
		Body:         n.Body,
		ContentType:  string(n.ContentType),
		Recipients:   n.Recipients,
		CC:           n.CC,
		BCC:          n.BCC,
		Metadata:     n.Metadata,
		CreatedAt:    n.CreatedAt,
		ScheduledFor: n.ScheduledFor,
		SentAt:       n.SentAt,
		RetryCount:   n.RetryCount,
		MaxRetries:   n.MaxRetries,
		LastError:    n.LastError,
	}
}

// NotificationResult represents the result of a notification operation
type NotificationResult struct {
	NotificationID   string                 `json:"notification_id"`
	Success          bool                   `json:"success"`
	Message          string                 `json:"message,omitempty"`
	Error            string                 `json:"error,omitempty"`
	SentAt           time.Time              `json:"sent_at"`
	ProviderResponse map[string]interface{} `json:"provider_response,omitempty"`
}

// NotificationResultFromDomain converts a domain result to API format
func NotificationResultFromDomain(r *domain.NotificationResult) NotificationResult {
	return NotificationResult{
		NotificationID:   r.NotificationID,
		Success:          r.Success,
		Message:          r.Message,
		Error:            r.Error,
		SentAt:           r.SentAt,
		ProviderResponse: r.ProviderResponse,
	}
}

// ListNotificationsResponse is the REST API response for listing notifications
type ListNotificationsResponse struct {
	Notifications []Notification `json:"notifications"`
	Total         int64          `json:"total"`
}

// RetryNotificationResponse is the REST API response for retrying a notification
type RetryNotificationResponse struct {
	Result NotificationResult `json:"result"`
}
