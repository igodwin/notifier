package config

import (
	"strings"
	"testing"

	"github.com/igodwin/notifier/internal/domain"
)

// TestValidateCORS_WildcardRejection tests that wildcard origins are rejected
func TestValidateCORS_WildcardRejection(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			GRPCPort: 50051,
			RESTPort: 8080,
			Mode:     "both",
		},
		Queue: domain.QueueConfig{
			Type: "local",
		},
		Notifiers: NotifiersConfig{
			Stdout: true,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail with wildcard origin")
	}

	if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("Expected error to mention wildcard, got: %v", err)
	}
}

// TestValidateCORS_InvalidOriginFormat tests origin format validation
func TestValidateCORS_InvalidOriginFormat(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		valid  bool
	}{
		{
			name:   "valid https",
			origin: "https://example.com",
			valid:  true,
		},
		{
			name:   "valid http",
			origin: "http://localhost:3000",
			valid:  true,
		},
		{
			name:   "missing protocol",
			origin: "example.com",
			valid:  false,
		},
		{
			name:   "invalid protocol",
			origin: "ftp://example.com",
			valid:  false,
		},
		{
			name:   "just domain without protocol",
			origin: "www.example.com",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 50051,
					RESTPort: 8080,
					Mode:     "both",
				},
				Queue: domain.QueueConfig{
					Type: "local",
				},
				Notifiers: NotifiersConfig{
					Stdout: true,
				},
				CORS: CORSConfig{
					AllowedOrigins: []string{tt.origin},
					AllowedMethods: []string{"GET"},
					AllowedHeaders: []string{"Content-Type"},
				},
			}

			err := config.Validate()
			if tt.valid && err != nil && strings.Contains(err.Error(), "invalid origin format") {
				t.Errorf("Expected origin %v to be valid, got error: %v", tt.origin, err)
			}
			if !tt.valid && (err == nil || !strings.Contains(err.Error(), "invalid origin format")) {
				t.Errorf("Expected origin %v to be invalid, got error: %v", tt.origin, err)
			}
		})
	}
}

// TestValidateCORS_CredentialsWithoutOrigins tests that credentials require origins
func TestValidateCORS_CredentialsWithoutOrigins(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			GRPCPort: 50051,
			RESTPort: 8080,
			Mode:     "both",
		},
		Queue: domain.QueueConfig{
			Type: "local",
		},
		Notifiers: NotifiersConfig{
			Stdout: true,
		},
		CORS: CORSConfig{
			AllowedOrigins:   []string{}, // Empty origins
			AllowedMethods:   []string{"GET"},
			AllowedHeaders:   []string{"Content-Type"},
			AllowCredentials: true, // But credentials enabled
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail when credentials enabled but no origins allowed")
	}

	if !strings.Contains(err.Error(), "allow_credentials") {
		t.Errorf("Expected error to mention allow_credentials, got: %v", err)
	}
}

// TestValidateCORS_ValidConfigurations tests valid CORS configurations
func TestValidateCORS_ValidConfigurations(t *testing.T) {
	tests := []struct {
		name string
		cors CORSConfig
	}{
		{
			name: "no origins (default secure config)",
			cors: CORSConfig{
				AllowedOrigins: []string{},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
		},
		{
			name: "single origin",
			cors: CORSConfig{
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
		},
		{
			name: "multiple origins",
			cors: CORSConfig{
				AllowedOrigins: []string{
					"https://example.com",
					"https://app.example.com",
					"http://localhost:3000",
				},
				AllowedMethods: []string{"GET", "POST", "DELETE"},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
			},
		},
		{
			name: "with credentials",
			cors: CORSConfig{
				AllowedOrigins:   []string{"https://example.com"},
				AllowedMethods:   []string{"GET", "POST"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				AllowCredentials: true,
			},
		},
		{
			name: "localhost development config",
			cors: CORSConfig{
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:8080",
					"http://localhost:5173",
				},
				AllowedMethods: []string{"GET", "POST", "OPTIONS", "DELETE"},
				AllowedHeaders: []string{"Content-Type", "Authorization"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 50051,
					RESTPort: 8080,
					Mode:     "both",
				},
				Queue: domain.QueueConfig{
					Type: "local",
				},
				Notifiers: NotifiersConfig{
					Stdout: true,
				},
				CORS: tt.cors,
			}

			err := config.Validate()
			if err != nil && strings.Contains(err.Error(), "CORS") {
				t.Errorf("Expected valid CORS config, got error: %v", err)
			}
		})
	}
}

// TestValidateCORS_MultipleOrigins tests validation with multiple origins including invalid ones
func TestValidateCORS_MultipleOrigins(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			GRPCPort: 50051,
			RESTPort: 8080,
			Mode:     "both",
		},
		Queue: domain.QueueConfig{
			Type: "local",
		},
		Notifiers: NotifiersConfig{
			Stdout: true,
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{
				"https://example.com",
				"*", // Wildcard in the middle
				"https://app.example.com",
			},
			AllowedMethods: []string{"GET"},
			AllowedHeaders: []string{"Content-Type"},
		},
	}

	err := config.Validate()
	if err == nil {
		t.Error("Expected validation to fail with wildcard in origins list")
	}

	if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("Expected error to mention wildcard, got: %v", err)
	}
}

// TestValidateCORS_EdgeCases tests edge cases in CORS validation
func TestValidateCORS_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		cors      CORSConfig
		shouldErr bool
		errText   string
	}{
		{
			name: "empty string in origins",
			cors: CORSConfig{
				AllowedOrigins: []string{"https://example.com", ""},
				AllowedMethods: []string{"GET"},
			},
			shouldErr: false, // Empty strings are ignored
		},
		{
			name: "origin with port",
			cors: CORSConfig{
				AllowedOrigins: []string{"https://example.com:8443"},
				AllowedMethods: []string{"GET"},
			},
			shouldErr: false,
		},
		{
			name: "origin with path (invalid)",
			cors: CORSConfig{
				AllowedOrigins: []string{"https://example.com/path"},
				AllowedMethods: []string{"GET"},
			},
			shouldErr: false, // Path is technically valid in origin
		},
		{
			name: "credentials without origins",
			cors: CORSConfig{
				AllowedOrigins:   []string{},
				AllowedMethods:   []string{"GET"},
				AllowCredentials: true,
			},
			shouldErr: true,
			errText:   "allow_credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{
					GRPCPort: 50051,
					RESTPort: 8080,
					Mode:     "both",
				},
				Queue: domain.QueueConfig{
					Type: "local",
				},
				Notifiers: NotifiersConfig{
					Stdout: true,
				},
				CORS: tt.cors,
			}

			err := config.Validate()
			if tt.shouldErr && err == nil {
				t.Errorf("Expected validation to fail for %s", tt.name)
			}
			if tt.shouldErr && err != nil && tt.errText != "" && !strings.Contains(err.Error(), tt.errText) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errText, err)
			}
			if !tt.shouldErr && err != nil && strings.Contains(err.Error(), "CORS") {
				t.Errorf("Expected validation to pass for %s, got error: %v", tt.name, err)
			}
		})
	}
}
