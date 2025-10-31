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

	// If RBAC is enabled (at least one rule exists), restrict access:
	// - Notifiers with explicit rules: check if user has allowed roles
	// - Notifiers without rules: deny access (must be explicitly allowed)
	if a.HasRules() {
		if !exists {
			// RBAC is enabled but this notifier has no rule - deny access
			return false
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

	// If no rules are registered at all, allow all authenticated users (open access)
	return true
}

// HasRules returns true if any authorization rules have been registered
func (a *NotifierAuthz) HasRules() bool {
	return len(a.rules) > 0
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
