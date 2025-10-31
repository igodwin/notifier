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
	return NewRouterWithAuth(service, logger, nil, DefaultCORSConfig())
}

// NewRouterWithAuth creates a new HTTP router with optional authentication and CORS configuration
func NewRouterWithAuth(service domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore, corsConfig *CORSConfig) *mux.Router {
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

	// Health check route (no auth required)
	router.HandleFunc("/health", handler.HealthCheck).Methods(http.MethodGet)

	// Middleware - CORS must be applied before auth to handle preflight requests
	router.Use(loggingMiddleware)
	if corsConfig != nil {
		router.Use(newCORSMiddleware(corsConfig))
	}

	return router
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
