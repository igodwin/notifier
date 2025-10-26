package auth

import (
	"net/http"
	"strings"

	"github.com/igodwin/notifier/internal/logging"
)

// RESTAuthMiddleware provides authentication for REST APIs
type RESTAuthMiddleware struct {
	store  *APIKeyStore
	logger *logging.Logger
}

// NewRESTAuthMiddleware creates a new REST auth middleware
func NewRESTAuthMiddleware(store *APIKeyStore, logger *logging.Logger) *RESTAuthMiddleware {
	return &RESTAuthMiddleware{
		store:  store,
		logger: logger,
	}
}

// Middleware returns an HTTP middleware function
func (m *RESTAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header or X-API-Key header
		apiKey := m.extractAPIKey(r)
		if apiKey == "" {
			m.logger.Warnf("REST: Missing API key in request from %s", r.RemoteAddr)
			http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		// Validate API key
		key, err := m.store.ValidateKey(apiKey)
		if err != nil {
			m.logger.Warnf("REST: Invalid API key from %s - error=%v", r.RemoteAddr, err)
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// Check rate limit
		allowed, err := m.store.CheckRateLimit(apiKey)
		if err != nil || !allowed {
			m.logger.Warnf("REST: Rate limit exceeded for key=%s from %s", key.ClientID, r.RemoteAddr)
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Update last used timestamp
		if err := m.store.UpdateLastUsed(apiKey); err != nil {
			m.logger.Errorf("REST: Failed to update last used time for key=%s - error=%v", key.ClientID, err)
		}

		// Create auth context and attach to request
		authCtx := &AuthContext{
			APIKey:   key,
			ClientID: key.ClientID,
			Roles:    key.Roles,
		}

		// Add auth context to request context
		ctx := ContextWithAuth(r.Context(), authCtx)
		m.logger.Debugf("REST: Authenticated request from client=%s with roles=%v", key.ClientID, key.Roles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractAPIKey extracts API key from Authorization header or X-API-Key header
func (m *RESTAuthMiddleware) extractAPIKey(r *http.Request) string {
	// Try Authorization header first (Bearer token)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Try X-API-Key header
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}

	return ""
}
