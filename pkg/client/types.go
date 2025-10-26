package client

import "time"

// NotificationRequest represents a notification to send
type NotificationRequest struct {
	Type       string            `json:"type"`               // stdout, email, slack, ntfy
	Account    string            `json:"account"`            // Optional: account name (uses default if empty)
	Subject    string            `json:"subject"`            // Email subject or title
	Body       string            `json:"body"`               // Notification message body
	Recipients []string          `json:"recipients"`         // Email addresses, Slack channels, etc.
	Metadata   map[string]string `json:"metadata,omitempty"` // Optional metadata
}

// NotificationResponse represents the response from sending a notification
type NotificationResponse struct {
	NotificationID string    `json:"notification_id"`
	Success        bool      `json:"success"`
	Message        string    `json:"message,omitempty"`
	Error          string    `json:"error,omitempty"`
	SentAt         time.Time `json:"sent_at"`
}

// NotificationStatus represents the status of a notification
type NotificationStatus string

const (
	StatusPending  NotificationStatus = "pending"
	StatusQueued   NotificationStatus = "queued"
	StatusRetrying NotificationStatus = "retrying"
	StatusSent     NotificationStatus = "sent"
	StatusFailed   NotificationStatus = "failed"
)

// Notification represents a notification with full details
type Notification struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Account    string             `json:"account"`
	Subject    string             `json:"subject"`
	Body       string             `json:"body"`
	Recipients []string           `json:"recipients"`
	Status     NotificationStatus `json:"status"`
	RetryCount int                `json:"retry_count"`
	MaxRetries int                `json:"max_retries"`
	LastError  string             `json:"last_error,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	SentAt     *time.Time         `json:"sent_at,omitempty"`
	Metadata   map[string]string  `json:"metadata,omitempty"`
}

// NotificationStats represents statistics about notifications
type NotificationStats struct {
	TotalSent    int64            `json:"total_sent"`
	TotalFailed  int64            `json:"total_failed"`
	TotalPending int64            `json:"total_pending"`
	TotalQueued  int64            `json:"total_queued"`
	ByType       map[string]int64 `json:"by_type"`
	ByStatus     map[string]int64 `json:"by_status"`
}

// ListNotificationsRequest represents filters for listing notifications
type ListNotificationsRequest struct {
	IDs           []string             `json:"ids,omitempty"`
	Types         []string             `json:"types,omitempty"`
	Statuses      []NotificationStatus `json:"statuses,omitempty"`
	Recipients    []string             `json:"recipients,omitempty"`
	CreatedAfter  *time.Time           `json:"created_after,omitempty"`
	CreatedBefore *time.Time           `json:"created_before,omitempty"`
	Offset        int                  `json:"offset,omitempty"`
	Limit         int                  `json:"limit,omitempty"`
}

// ListNotificationsResponse represents the response from listing notifications
type ListNotificationsResponse struct {
	Notifications []*Notification `json:"notifications"`
	Total         int             `json:"total"`
}

// NotifierInfo represents information about an available notifier
type NotifierInfo struct {
	Type           string   `json:"type"`
	Accounts       []string `json:"accounts"`
	DefaultAccount string   `json:"default_account"`
}

// NotifiersResponse represents available notifiers
type NotifiersResponse struct {
	Notifiers []NotifierInfo `json:"notifiers"`
}

// ClientConfig contains configuration for the client
type ClientConfig struct {
	BaseURL      string        // Base URL for REST API (e.g., "http://localhost:8080")
	APIKey       string        // Optional API key for authentication
	Timeout      time.Duration // Request timeout (default: 30s)
	MaxRetries   int           // Max retries on failure (default: 3)
	RetryBackoff time.Duration // Backoff between retries (default: 100ms)
	TLSInsecure  bool          // Disable TLS verification (for testing only)
}
