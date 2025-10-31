package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/igodwin/notifier/internal/auth"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
)

// CORSConfig contains CORS middleware configuration
type CORSConfig struct {
	// AllowedOrigins is a whitelist of allowed origins (e.g., ["https://example.com", "https://app.example.com"])
	// Wildcards are NOT supported for security reasons
	AllowedOrigins []string

	// AllowedMethods is a list of allowed HTTP methods (e.g., ["GET", "POST", "OPTIONS", "DELETE"])
	AllowedMethods []string

	// AllowedHeaders is a list of allowed HTTP headers (e.g., ["Content-Type", "Authorization"])
	AllowedHeaders []string

	// AllowCredentials indicates whether credentials (cookies, authorization headers) are allowed
	// Note: When true, AllowedOrigins must NOT contain wildcards
	AllowCredentials bool

	// MaxAge is the duration in seconds that browsers can cache preflight responses
	MaxAge int
}

// DefaultCORSConfig returns a secure default CORS configuration
// By default, no origins are allowed - you must explicitly configure allowed origins
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins:   []string{}, // Empty by default - must be explicitly configured
		AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           3600, // 1 hour
	}
}

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(service domain.NotificationService, logger *logging.Logger) *mux.Router {
	return NewRouterWithAuth(service, logger, nil)
}

// NewRouterWithAuth creates a new HTTP router with optional authentication and CORS configuration
func NewRouterWithAuth(service domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore) *mux.Router {
	return NewRouterWithAuthAndKeyStore(service, logger, authStore, nil)
}

// NewRouterWithAuthAndKeyStore creates a new HTTP router with authentication and key management
func NewRouterWithAuthAndKeyStore(service domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore, keyStore *auth.HybridKeyStore) *mux.Router {
	handler := NewHandler(service, logger)
	router := mux.NewRouter()

	// API v1 routes
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// Apply authentication middleware if auth store is provided
	if authStore != nil {
		authMiddleware := auth.NewRESTAuthMiddleware(authStore, logger)
		v1.Use(authMiddleware.Middleware)
	}

	// Notification routes
	v1.HandleFunc("/notifications", handler.SendNotification).Methods(http.MethodPost)
	v1.HandleFunc("/notifications/batch", handler.SendBatchNotifications).Methods(http.MethodPost)
	v1.HandleFunc("/notifications", handler.ListNotifications).Methods(http.MethodGet)
	v1.HandleFunc("/notifications/{id}", handler.GetNotification).Methods(http.MethodGet)
	v1.HandleFunc("/notifications/{id}", handler.CancelNotification).Methods(http.MethodDelete)
	v1.HandleFunc("/notifications/{id}/retry", handler.RetryNotification).Methods(http.MethodPost)

	// Stats route
	v1.HandleFunc("/stats", handler.GetStats).Methods(http.MethodGet)

	// Notifiers route
	v1.HandleFunc("/notifiers", handler.GetNotifiers).Methods(http.MethodGet)

	// Key management routes (requires auth and keystore)
	if authStore != nil && keyStore != nil {
		keyHandler := NewKeyManagementHandler(keyStore, logger)
		v1.HandleFunc("/admin/keys", keyHandler.CreateKey).Methods(http.MethodPost)
		v1.HandleFunc("/admin/keys", keyHandler.ListKeys).Methods(http.MethodGet)
		v1.HandleFunc("/admin/keys/{key}", keyHandler.RevokeKey).Methods(http.MethodDelete)
		v1.HandleFunc("/admin/keys/{key}/rotate", keyHandler.RotateKey).Methods(http.MethodPost)
		v1.HandleFunc("/admin/keys/{key}/audit", keyHandler.GetAuditLog).Methods(http.MethodGet)
	}

	// Health check route (no auth required)
	router.HandleFunc("/health", handler.HealthCheck).Methods(http.MethodGet)

	// Middleware - logging and CORS
	router.Use(loggingMiddleware)

	return router
}

// loggingMiddleware logs incoming requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// You can add structured logging here
		next.ServeHTTP(w, r)
	})
}
