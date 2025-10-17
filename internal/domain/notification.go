package domain

import (
	"time"
)

// Priority defines the urgency level of a notification
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// NotificationType defines the channel through which to send the notification
type NotificationType string

const (
	TypeEmail  NotificationType = "email"
	TypeSlack  NotificationType = "slack"
	TypeNtfy   NotificationType = "ntfy"
	TypeStdout NotificationType = "stdout"
)

// NotificationStatus represents the current state of a notification
type NotificationStatus string

const (
	StatusPending    NotificationStatus = "pending"
	StatusQueued     NotificationStatus = "queued"
	StatusProcessing NotificationStatus = "processing"
	StatusSent       NotificationStatus = "sent"
	StatusFailed     NotificationStatus = "failed"
	StatusRetrying   NotificationStatus = "retrying"
)

// Notification represents a notification message with metadata
type Notification struct {
	// ID is a unique identifier for the notification
	ID string `json:"id"`

	// Type specifies which notifier should handle this notification
	Type NotificationType `json:"type"`

	// Account specifies which named account/instance to use for this notifier type (optional)
	// If not specified, the default account for the notifier type will be used
	Account string `json:"account,omitempty"`

	// Priority determines urgency and retry behavior
	Priority Priority `json:"priority"`

	// Status tracks the current state of the notification
	Status NotificationStatus `json:"status"`

	// Subject is the notification subject/title (used for email, slack, ntfy)
	Subject string `json:"subject"`

	// Body is the main content of the notification
	Body string `json:"body"`

	// Recipients contains the target addresses (email, slack channel, ntfy topic, etc.)
	Recipients []string `json:"recipients"`

	// Metadata contains additional provider-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when the notification was created
	CreatedAt time.Time `json:"created_at"`

	// ScheduledFor allows delayed sending (optional)
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`

	// SentAt is when the notification was successfully sent
	SentAt *time.Time `json:"sent_at,omitempty"`

	// RetryCount tracks how many times sending has been attempted
	RetryCount int `json:"retry_count"`

	// MaxRetries defines the maximum retry attempts
	MaxRetries int `json:"max_retries"`

	// LastError stores the most recent error message if failed
	LastError string `json:"last_error,omitempty"`
}

// NotificationResult represents the outcome of sending a notification
type NotificationResult struct {
	// NotificationID references the original notification
	NotificationID string `json:"notification_id"`

	// Success indicates if the notification was sent successfully
	Success bool `json:"success"`

	// Message provides additional context about the result
	Message string `json:"message,omitempty"`

	// Error contains error details if the notification failed
	Error string `json:"error,omitempty"`

	// SentAt is when the notification was sent
	SentAt time.Time `json:"sent_at"`

	// ProviderResponse contains raw response data from the notification provider
	ProviderResponse map[string]interface{} `json:"provider_response,omitempty"`
}

// NotificationFilter is used for querying notifications
type NotificationFilter struct {
	IDs           []string             `json:"ids,omitempty"`
	Types         []NotificationType   `json:"types,omitempty"`
	Statuses      []NotificationStatus `json:"statuses,omitempty"`
	Recipients    []string             `json:"recipients,omitempty"`
	CreatedAfter  *time.Time           `json:"created_after,omitempty"`
	CreatedBefore *time.Time           `json:"created_before,omitempty"`
	Limit         int                  `json:"limit,omitempty"`
	Offset        int                  `json:"offset,omitempty"`
}
