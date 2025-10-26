package notifier

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestNewNtfyNotifierWithDefaultCA tests that system default CA is used by default
func TestNewNtfyNotifierWithDefaultCA(t *testing.T) {
	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
		// CACertPath is empty, should use system default CA
	}

	notifier, err := NewNtfyNotifier(config)
	if err != nil {
		t.Fatalf("Failed to create notifier with default CA: %v", err)
	}

	if notifier == nil {
		t.Fatal("Expected notifier to be created")
	}

	// Verify HTTP client was created with TLS config
	if notifier.httpClient == nil {
		t.Fatal("Expected HTTP client to be configured")
	}

	// Verify TLS transport was configured
	if notifier.httpClient.Transport == nil {
		t.Fatal("Expected HTTP transport to be configured")
	}

	t.Logf("✓ System default CA is used when CACertPath is empty")
}

// TestNewNtfyNotifierWithCustomCA tests loading custom CA certificate
func TestNewNtfyNotifierWithCustomCA(t *testing.T) {
	// Create a temporary CA certificate file
	certPath := createTempCACert(t)
	defer os.Remove(certPath)

	config := &NtfyConfig{
		ServerURL:    "https://self-signed.example.com",
		DefaultTopic: "test",
		CACertPath:   certPath,
	}

	notifier, err := NewNtfyNotifier(config)
	if err != nil {
		t.Fatalf("Failed to create notifier with custom CA: %v", err)
	}

	if notifier == nil {
		t.Fatal("Expected notifier to be created")
	}

	t.Logf("✓ Custom CA certificate loaded successfully")
}

// TestValidateCACertPathNotFound tests error when CA cert doesn't exist
func TestValidateCACertPathNotFound(t *testing.T) {
	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
		CACertPath:   "/nonexistent/path/to/cert.pem",
	}

	_, err := NewNtfyNotifier(config)
	if err == nil {
		t.Fatal("Expected error when CA cert file doesn't exist")
	}

	if !contains(err.Error(), "not found") && !contains(err.Error(), "no such file") {
		t.Fatalf("Expected 'not found' error, got: %v", err)
	}

	t.Logf("✓ Correctly rejects non-existent CA certificate file")
}

// TestValidateCACertPathInvalidFormat tests error when file is not valid PEM
func TestValidateCACertPathInvalidFormat(t *testing.T) {
	// Create a temporary file with invalid certificate format
	tmpFile, err := os.CreateTemp("", "invalid-cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid content (not PEM format)
	if _, err := tmpFile.WriteString("This is not a valid certificate"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
		CACertPath:   tmpFile.Name(),
	}

	_, err = NewNtfyNotifier(config)
	if err == nil {
		t.Fatal("Expected error when CA cert is not valid PEM format")
	}

	if !contains(err.Error(), "PEM") && !contains(err.Error(), "parse") {
		t.Fatalf("Expected PEM format error, got: %v", err)
	}

	t.Logf("✓ Correctly rejects invalid PEM format")
}

// TestValidateCACertPathIsDirectory tests error when path is a directory
func TestValidateCACertPathIsDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "cert-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
		CACertPath:   tmpDir,
	}

	_, err = NewNtfyNotifier(config)
	if err == nil {
		t.Fatal("Expected error when CA cert path is a directory")
	}

	if !contains(err.Error(), "not a regular file") {
		t.Fatalf("Expected 'not a regular file' error, got: %v", err)
	}

	t.Logf("✓ Correctly rejects directory paths")
}

// TestValidateCACertPathEmpty tests that empty CA cert path is valid (uses system defaults)
func TestValidateCACertPathEmpty(t *testing.T) {
	err := validateCACertPath("")
	if err != nil {
		t.Fatalf("Empty CA cert path should be valid (uses system defaults), got error: %v", err)
	}

	t.Logf("✓ Empty CA cert path is valid (system defaults)")
}

// TestTLSConfigHasMinimumVersion tests TLS minimum version is set
func TestTLSConfigHasMinimumVersion(t *testing.T) {
	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
	}

	httpClient, err := createNtfyHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	transport := httpClient.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLS config to be set")
	}

	if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("Expected minimum TLS version to be 1.2 or higher, got %v", transport.TLSClientConfig.MinVersion)
	}

	t.Logf("✓ TLS minimum version is TLS 1.2 or higher")
}

// TestTLSConfigNeverSkipsVerification tests that InsecureSkipVerify is never set
func TestTLSConfigNeverSkipsVerification(t *testing.T) {
	config := &NtfyConfig{
		ServerURL:    "https://ntfy.sh",
		DefaultTopic: "test",
	}

	httpClient, err := createNtfyHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	transport := httpClient.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLS config to be set")
	}

	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify should NEVER be true - TLS verification must always be enforced")
	}

	t.Logf("✓ TLS verification is always enforced (InsecureSkipVerify is false)")
}

// TestCustomCACertLoading tests that custom CA cert is properly loaded into cert pool
func TestCustomCACertLoading(t *testing.T) {
	// Create a temporary CA certificate
	certPath := createTempCACert(t)
	defer os.Remove(certPath)

	config := &NtfyConfig{
		ServerURL:    "https://self-signed.example.com",
		DefaultTopic: "test",
		CACertPath:   certPath,
	}

	httpClient, err := createNtfyHTTPClient(config)
	if err != nil {
		t.Fatalf("Failed to create HTTP client with custom CA: %v", err)
	}

	transport := httpClient.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("Expected TLS config to be set")
	}

	// Verify custom CA cert pool is set
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("Expected custom CA certificate pool to be loaded")
	}

	t.Logf("✓ Custom CA certificate is properly loaded into cert pool")
}

// TestMissingCAFileError tests proper error message for missing CA file
func TestMissingCAFileError(t *testing.T) {
	err := validateCACertPath("/path/that/does/not/exist.pem")
	if err == nil {
		t.Fatal("Expected error for missing CA file")
	}

	errorMsg := err.Error()
	if !contains(errorMsg, "not found") && !contains(errorMsg, "no such file") {
		t.Fatalf("Expected error message about missing file, got: %s", errorMsg)
	}

	t.Logf("✓ Clear error message for missing CA file: %s", errorMsg)
}

// TestEmptyCertFileError tests error when cert file is empty
func TestEmptyCertFileError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	err = validateCACertPath(tmpFile.Name())
	if err == nil {
		t.Fatal("Expected error for empty cert file")
	}

	t.Logf("✓ Empty cert file is rejected: %v", err)
}

// Helper function to create a temporary self-signed certificate
func createTempCACert(t *testing.T) string {
	tmpFile, err := os.CreateTemp("", "ca-cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Generate a self-signed certificate for testing
	certPEM := generateSelfSignedCert(t)
	if _, err := tmpFile.WriteString(certPEM); err != nil {
		t.Fatalf("Failed to write certificate: %v", err)
	}

	return tmpFile.Name()
}

// Helper function to generate a self-signed certificate in PEM format
func generateSelfSignedCert(t *testing.T) string {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Generate certificate
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("Failed to generate serial number: %v", err)
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Test"},
			CommonName:   "test.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	if certPEM == nil {
		t.Fatal("Failed to encode certificate to PEM")
	}

	return string(certPEM)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)))
}

// Helper to find substring
func findSubstring(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
