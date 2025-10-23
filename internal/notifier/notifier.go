package notifier

import (
	"context"
	"fmt"
	"sync"

	"github.com/igodwin/notifier/internal/domain"
)

// Factory creates and manages notifier instances
type Factory struct {
	// Map of "type:account" -> notifier instance
	notifiers map[string]domain.Notifier
	mu        sync.RWMutex
}

// NewFactory creates a new notifier factory
func NewFactory() *Factory {
	return &Factory{
		notifiers: make(map[string]domain.Notifier),
	}
}

// makeKey creates a compound key from notification type and account
func makeKey(notificationType domain.NotificationType, account string) string {
	if account == "" {
		// For backward compatibility, if account is empty, just use the type
		return string(notificationType)
	}
	return fmt.Sprintf("%s:%s", notificationType, account)
}

// Create creates a notifier for the given type and account
func (f *Factory) Create(notificationType domain.NotificationType, account string) (domain.Notifier, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	key := makeKey(notificationType, account)
	notifier, exists := f.notifiers[key]
	if !exists {
		if account != "" {
			return nil, fmt.Errorf("unsupported notification type: %s with account: %s", notificationType, account)
		}
		return nil, fmt.Errorf("unsupported notification type: %s", notificationType)
	}

	return notifier, nil
}

// RegisterNotifier registers a custom notifier implementation
func (f *Factory) RegisterNotifier(notificationType domain.NotificationType, account string, notifier domain.Notifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := makeKey(notificationType, account)
	if _, exists := f.notifiers[key]; exists {
		if account != "" {
			return fmt.Errorf("notifier already registered for type: %s with account: %s", notificationType, account)
		}
		return fmt.Errorf("notifier already registered for type: %s", notificationType)
	}

	f.notifiers[key] = notifier
	return nil
}

// SupportedTypes returns all supported notification types (unique types only)
func (f *Factory) SupportedTypes() []domain.NotificationType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	typeMap := make(map[domain.NotificationType]bool)
	for key := range f.notifiers {
		// Extract the type from the key (type:account or just type)
		var notifType domain.NotificationType
		if colonIdx := findColon(key); colonIdx >= 0 {
			// Key format: "type:account"
			notifType = domain.NotificationType(key[:colonIdx])
		} else {
			// Key format: just "type" (backward compatibility)
			notifType = domain.NotificationType(key)
		}
		typeMap[notifType] = true
	}

	types := make([]domain.NotificationType, 0, len(typeMap))
	for t := range typeMap {
		types = append(types, t)
	}

	return types
}

// findColon finds the index of ':' in a string, returns -1 if not found
func findColon(s string) int {
	for i, c := range s {
		if c == ':' {
			return i
		}
	}
	return -1
}

// GetAccounts returns all registered accounts for a given notification type
func (f *Factory) GetAccounts(notificationType domain.NotificationType) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	accounts := []string{}
	prefix := string(notificationType) + ":"

	for key := range f.notifiers {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			account := key[len(prefix):]
			accounts = append(accounts, account)
		}
	}

	return accounts
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
