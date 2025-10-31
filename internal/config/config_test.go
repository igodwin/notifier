package config

import (
	"testing"
)

func TestSanitizeDatabaseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL with password",
			input:    "postgresql://user:password@localhost:5432/dbname",
			expected: "postgresql://user:***REDACTED***@localhost:5432/dbname",
		},
		{
			name:     "PostgreSQL without password",
			input:    "postgresql://user@localhost:5432/dbname",
			expected: "postgresql://user@localhost:5432/dbname",
		},
		{
			name:     "MySQL with special characters in password",
			input:    "mysql://root:SuperSecret123!@db.example.com:3306/mydb",
			expected: "mysql://root:***REDACTED***@db.example.com:3306/mydb",
		},
		{
			name:     "Empty URL",
			input:    "",
			expected: "",
		},
		{
			name:     "Invalid URL without protocol",
			input:    "invalid-url",
			expected: "invalid-url",
		},
		{
			name:     "URL without credentials",
			input:    "postgresql://localhost:5432/dbname",
			expected: "postgresql://localhost:5432/dbname",
		},
		{
			name:     "PostgreSQL with password containing colons",
			input:    "postgresql://user:pass:word@localhost:5432/dbname",
			expected: "postgresql://user:***REDACTED***@localhost:5432/dbname",
		},
		{
			name:     "PostgreSQL with complex hostname and port",
			input:    "postgresql://admin:p@ssw0rd!@db-prod.example.com:5432/production",
			expected: "postgresql://admin:***REDACTED***@db-prod.example.com:5432/production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeDatabaseURL(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeDatabaseURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
