package notifier

import (
	"context"
	"fmt"
	"sync"

	"github.com/igodwin/notifier/internal/domain"
)

// Factory creates and manages notifier instances
type Factory struct {
	notifiers map[domain.NotificationType]domain.Notifier
	mu        sync.RWMutex
}

// NewFactory creates a new notifier factory
func NewFactory() *Factory {
	return &Factory{
		notifiers: make(map[domain.NotificationType]domain.Notifier),
	}
}

// Create creates a notifier for the given type
func (f *Factory) Create(notificationType domain.NotificationType) (domain.Notifier, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	notifier, exists := f.notifiers[notificationType]
	if !exists {
		return nil, fmt.Errorf("unsupported notification type: %s", notificationType)
	}

	return notifier, nil
}

// RegisterNotifier registers a custom notifier implementation
func (f *Factory) RegisterNotifier(notificationType domain.NotificationType, notifier domain.Notifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.notifiers[notificationType]; exists {
		return fmt.Errorf("notifier already registered for type: %s", notificationType)
	}

	f.notifiers[notificationType] = notifier
	return nil
}

// SupportedTypes returns all supported notification types
func (f *Factory) SupportedTypes() []domain.NotificationType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]domain.NotificationType, 0, len(f.notifiers))
	for t := range f.notifiers {
		types = append(types, t)
	}

	return types
}

// BaseNotifier provides common functionality for all notifiers
type BaseNotifier struct {
	notificationType domain.NotificationType
}

// Type returns the notification type
func (b *BaseNotifier) Type() domain.NotificationType {
	return b.notificationType
}

// Validate performs basic validation common to all notifiers
func (b *BaseNotifier) Validate(notification *domain.Notification) error {
	if notification == nil {
		return fmt.Errorf("notification is nil")
	}

	if len(notification.Recipients) == 0 {
		return fmt.Errorf("notification has no recipients")
	}

	if notification.Type != b.notificationType {
		return fmt.Errorf("notification type mismatch: expected %s, got %s", b.notificationType, notification.Type)
	}

	return nil
}

// Close performs cleanup (default implementation does nothing)
func (b *BaseNotifier) Close() error {
	return nil
}

// ValidateContext checks if the context is valid
func ValidateContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
