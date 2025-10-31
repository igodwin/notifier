package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCORSMiddleware_AllowedOrigin tests that allowed origins are accepted
func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"https://example.com", "https://app.example.com"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           3600,
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	tests := []struct {
		name           string
		origin         string
		expectOrigin   string
		expectMethods  string
		expectHeaders  string
		expectMaxAge   string
		expectCreds    string
	}{
		{
			name:          "allowed origin - example.com",
			origin:        "https://example.com",
			expectOrigin:  "https://example.com",
			expectMethods: "GET, POST, OPTIONS",
			expectHeaders: "Content-Type, Authorization",
			expectMaxAge:  "3600",
			expectCreds:   "",
		},
		{
			name:          "allowed origin - app.example.com",
			origin:        "https://app.example.com",
			expectOrigin:  "https://app.example.com",
			expectMethods: "GET, POST, OPTIONS",
			expectHeaders: "Content-Type, Authorization",
			expectMaxAge:  "3600",
			expectCreds:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Check CORS headers
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.expectOrigin {
				t.Errorf("Access-Control-Allow-Origin = %v, want %v", got, tt.expectOrigin)
			}
			if got := rec.Header().Get("Access-Control-Allow-Methods"); got != tt.expectMethods {
				t.Errorf("Access-Control-Allow-Methods = %v, want %v", got, tt.expectMethods)
			}
			if got := rec.Header().Get("Access-Control-Allow-Headers"); got != tt.expectHeaders {
				t.Errorf("Access-Control-Allow-Headers = %v, want %v", got, tt.expectHeaders)
			}
			if got := rec.Header().Get("Access-Control-Max-Age"); got != tt.expectMaxAge {
				t.Errorf("Access-Control-Max-Age = %v, want %v", got, tt.expectMaxAge)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != tt.expectCreds {
				t.Errorf("Access-Control-Allow-Credentials = %v, want %v", got, tt.expectCreds)
			}

			// Verify response
			if rec.Code != http.StatusOK {
				t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
			}
		})
	}
}

// TestCORSMiddleware_BlockedOrigin tests that non-whitelisted origins are rejected
func TestCORSMiddleware_BlockedOrigin(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         3600,
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	tests := []struct {
		name   string
		origin string
	}{
		{
			name:   "different domain",
			origin: "https://malicious.com",
		},
		{
			name:   "subdomain not in whitelist",
			origin: "https://subdomain.example.com",
		},
		{
			name:   "http instead of https",
			origin: "http://example.com",
		},
		{
			name:   "no origin header",
			origin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// CORS headers should NOT be set
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("Access-Control-Allow-Origin should not be set, got %v", got)
			}
			if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
				t.Errorf("Access-Control-Allow-Methods should not be set, got %v", got)
			}

			// The request should still succeed (CORS is browser-enforced)
			// But without CORS headers, browsers will block the response
			if rec.Code != http.StatusOK {
				t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
			}
		})
	}
}

// TestCORSMiddleware_PreflightRequest tests OPTIONS preflight requests
func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         7200,
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS request")
	}))

	tests := []struct {
		name          string
		origin        string
		expectStatus  int
		expectHeaders bool
	}{
		{
			name:          "preflight from allowed origin",
			origin:        "https://example.com",
			expectStatus:  http.StatusOK,
			expectHeaders: true,
		},
		{
			name:          "preflight from blocked origin",
			origin:        "https://malicious.com",
			expectStatus:  http.StatusOK,
			expectHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Preflight should always return 200 OK
			if rec.Code != tt.expectStatus {
				t.Errorf("status = %v, want %v", rec.Code, tt.expectStatus)
			}

			// Check if CORS headers are set based on origin
			hasOriginHeader := rec.Header().Get("Access-Control-Allow-Origin") != ""
			if hasOriginHeader != tt.expectHeaders {
				t.Errorf("CORS headers present = %v, want %v", hasOriginHeader, tt.expectHeaders)
			}
		})
	}
}

// TestCORSMiddleware_Credentials tests credential handling
func TestCORSMiddleware_Credentials(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify credentials header is set
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %v, want true", got)
	}
}

// TestCORSMiddleware_NoWildcard tests that wildcard is never returned
func TestCORSMiddleware_NoWildcard(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins: []string{"https://example.com", "https://app.example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         3600,
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test multiple origins to ensure wildcard is never used
	origins := []string{"https://example.com", "https://app.example.com", "https://malicious.com"}

	for _, origin := range origins {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", origin)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Verify wildcard is NEVER returned
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Errorf("Access-Control-Allow-Origin should never be wildcard, origin was %v", origin)
		}
	}
}

// TestCORSMiddleware_EmptyConfig tests behavior with empty allowed origins
func TestCORSMiddleware_EmptyConfig(t *testing.T) {
	config := &CORSConfig{
		AllowedOrigins: []string{}, // No origins allowed
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	middleware := newCORSMiddleware(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No CORS headers should be set
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin should not be set with empty config, got %v", got)
	}

	// Request should still succeed
	if rec.Code != http.StatusOK {
		t.Errorf("status = %v, want %v", rec.Code, http.StatusOK)
	}
}

// TestDefaultCORSConfig tests the default configuration
func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 0 {
		t.Errorf("Default config should have no allowed origins, got %v", config.AllowedOrigins)
	}

	if config.AllowCredentials {
		t.Error("Default config should not allow credentials")
	}

	expectedMethods := []string{"GET", "POST", "OPTIONS", "DELETE"}
	if len(config.AllowedMethods) != len(expectedMethods) {
		t.Errorf("Default methods count = %v, want %v", len(config.AllowedMethods), len(expectedMethods))
	}

	expectedHeaders := []string{"Content-Type", "Authorization"}
	if len(config.AllowedHeaders) != len(expectedHeaders) {
		t.Errorf("Default headers count = %v, want %v", len(config.AllowedHeaders), len(expectedHeaders))
	}

	if config.MaxAge != 3600 {
		t.Errorf("Default MaxAge = %v, want 3600", config.MaxAge)
	}
}

// TestCORSMiddleware_MaxAge tests custom max age values
func TestCORSMiddleware_MaxAge(t *testing.T) {
	tests := []struct {
		name           string
		maxAge         int
		expectMaxAge   string
	}{
		{
			name:         "zero max age",
			maxAge:       0,
			expectMaxAge: "",
		},
		{
			name:         "one hour",
			maxAge:       3600,
			expectMaxAge: "3600",
		},
		{
			name:         "one day",
			maxAge:       86400,
			expectMaxAge: "86400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET"},
				AllowedHeaders: []string{"Content-Type"},
				MaxAge:         tt.maxAge,
			}

			middleware := newCORSMiddleware(config)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", "https://example.com")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Max-Age"); got != tt.expectMaxAge {
				t.Errorf("Access-Control-Max-Age = %v, want %v", got, tt.expectMaxAge)
			}
		})
	}
}
