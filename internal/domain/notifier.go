package domain

import (
	"context"
)

// Notifier is the core interface that all notification implementations must satisfy
type Notifier interface {
	// Send sends a notification and returns the result
	Send(ctx context.Context, notification *Notification) (*NotificationResult, error)

	// Type returns the notification type this notifier handles
	Type() NotificationType

	// Validate checks if a notification can be sent with this notifier
	Validate(notification *Notification) error

	// Close performs cleanup when the notifier is no longer needed
	Close() error
}

// NotifierFactory creates notifier instances based on configuration
type NotifierFactory interface {
	// Create creates a notifier for the given type
	Create(notificationType NotificationType) (Notifier, error)

	// RegisterNotifier registers a custom notifier implementation
	RegisterNotifier(notificationType NotificationType, notifier Notifier) error

	// SupportedTypes returns all supported notification types
	SupportedTypes() []NotificationType
}

// NotificationService is the high-level service interface for managing notifications
type NotificationService interface {
	// Send queues a notification for delivery
	Send(ctx context.Context, notification *Notification) (*NotificationResult, error)

	// SendBatch queues multiple notifications for delivery
	SendBatch(ctx context.Context, notifications []*Notification) ([]*NotificationResult, error)

	// GetNotification retrieves a notification by ID
	GetNotification(ctx context.Context, id string) (*Notification, error)

	// ListNotifications retrieves notifications matching the filter
	ListNotifications(ctx context.Context, filter *NotificationFilter) ([]*Notification, error)

	// CancelNotification cancels a pending notification
	CancelNotification(ctx context.Context, id string) error

	// RetryNotification retries a failed notification
	RetryNotification(ctx context.Context, id string) (*NotificationResult, error)

	// GetStats returns notification statistics
	GetStats(ctx context.Context) (*NotificationStats, error)
}

// NotificationStats contains statistics about notification processing
type NotificationStats struct {
	TotalSent      int64              `json:"total_sent"`
	TotalFailed    int64              `json:"total_failed"`
	TotalPending   int64              `json:"total_pending"`
	TotalQueued    int64              `json:"total_queued"`
	ByType         map[string]int64   `json:"by_type"`
	ByStatus       map[string]int64   `json:"by_status"`
	AverageLatency float64            `json:"average_latency_ms"`
}
