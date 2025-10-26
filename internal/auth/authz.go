package auth

import (
	"fmt"

	"github.com/igodwin/notifier/internal/domain"
)

// NotifierAuthz manages authorization rules for notifiers
type NotifierAuthz struct {
	// Map of "type:account" -> allowed roles
	rules map[string][]string
}

// NewNotifierAuthz creates a new notifier authorization manager
func NewNotifierAuthz() *NotifierAuthz {
	return &NotifierAuthz{
		rules: make(map[string][]string),
	}
}

// RegisterRule registers authorization rule for a notifier type and account
func (a *NotifierAuthz) RegisterRule(notificationType domain.NotificationType, account string, allowedRoles []string) {
	key := makeAuthzKey(notificationType, account)
	a.rules[key] = allowedRoles
}

// IsAuthorized checks if an auth context is authorized to use a specific notifier
func (a *NotifierAuthz) IsAuthorized(auth *AuthContext, notificationType domain.NotificationType, account string) bool {
	if auth == nil || len(auth.Roles) == 0 {
		return false
	}

	key := makeAuthzKey(notificationType, account)
	allowedRoles, exists := a.rules[key]

	// If no specific rule is registered, allow all authenticated users
	if !exists {
		return true
	}

	// Check if any of the user's roles is in the allowed roles
	for _, userRole := range auth.Roles {
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				return true
			}
		}
	}

	return false
}

// GetAllowedRoles returns the allowed roles for a notifier
func (a *NotifierAuthz) GetAllowedRoles(notificationType domain.NotificationType, account string) []string {
	key := makeAuthzKey(notificationType, account)
	return a.rules[key]
}

// SetAllowedRoles sets the allowed roles for a notifier
func (a *NotifierAuthz) SetAllowedRoles(notificationType domain.NotificationType, account string, allowedRoles []string) {
	key := makeAuthzKey(notificationType, account)
	a.rules[key] = allowedRoles
}

// makeAuthzKey creates a compound key from notification type and account
func makeAuthzKey(notificationType domain.NotificationType, account string) string {
	if account == "" {
		return string(notificationType)
	}
	return fmt.Sprintf("%s:%s", notificationType, account)
}
