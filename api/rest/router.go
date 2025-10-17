package rest

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/igodwin/notifier/internal/domain"
)

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(service domain.NotificationService) *mux.Router {
	handler := NewHandler(service)
	router := mux.NewRouter()

	// API v1 routes
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// Notification routes
	v1.HandleFunc("/notifications", handler.SendNotification).Methods(http.MethodPost)
	v1.HandleFunc("/notifications/batch", handler.SendBatchNotifications).Methods(http.MethodPost)
	v1.HandleFunc("/notifications", handler.ListNotifications).Methods(http.MethodGet)
	v1.HandleFunc("/notifications/{id}", handler.GetNotification).Methods(http.MethodGet)
	v1.HandleFunc("/notifications/{id}", handler.CancelNotification).Methods(http.MethodDelete)
	v1.HandleFunc("/notifications/{id}/retry", handler.RetryNotification).Methods(http.MethodPost)

	// Stats route
	v1.HandleFunc("/stats", handler.GetStats).Methods(http.MethodGet)

	// Health check route
	router.HandleFunc("/health", handler.HealthCheck).Methods(http.MethodGet)

	// Middleware
	router.Use(loggingMiddleware)
	router.Use(corsMiddleware)

	return router
}

// loggingMiddleware logs incoming requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// You can add structured logging here
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
