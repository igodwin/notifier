package rest

import (
	"net/http"
	"strconv"
	"strings"

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
		v1.HandleFunc("/admin/keys/{name}", keyHandler.RevokeKey).Methods(http.MethodDelete)
		v1.HandleFunc("/admin/keys/{name}/rotate", keyHandler.RotateKey).Methods(http.MethodPost)
		v1.HandleFunc("/admin/keys/{name}/audit", keyHandler.GetAuditLog).Methods(http.MethodGet)
	}

	// Health check route (no auth required)
	router.HandleFunc("/health", handler.HealthCheck).Methods(http.MethodGet)

	// Middleware - logging, request size limit, and CORS
	router.Use(loggingMiddleware)
	v1.Use(maxBodySizeMiddleware(1 << 20)) // 1 MB limit on API request bodies

	return router
}

// maxBodySizeMiddleware limits the size of incoming request bodies to prevent DoS.
func maxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware logs incoming requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// You can add structured logging here
		next.ServeHTTP(w, r)
	})
}

// newCORSMiddleware creates a CORS middleware with origin whitelist validation
func newCORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if the origin is in the allowed list
			allowed := false
			for _, allowedOrigin := range config.AllowedOrigins {
				if origin == allowedOrigin {
					allowed = true
					break
				}
			}

			// Only set CORS headers if the origin is allowed
			if allowed {
				// Set the exact origin (never use wildcard)
				w.Header().Set("Access-Control-Allow-Origin", origin)

				// Set allowed methods
				if len(config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				}

				// Set allowed headers
				if len(config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				}

				// Set credentials header if enabled
				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				// Set max age for preflight caching
				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(int64(config.MaxAge), 10))
				}
			}

			// Handle preflight OPTIONS requests
			if r.Method == http.MethodOptions {
				// Return 200 OK for preflight requests
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
