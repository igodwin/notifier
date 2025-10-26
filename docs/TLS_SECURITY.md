# TLS Security and Certificate Handling

## Overview

The notifier service enforces TLS verification for all HTTPS connections. This document explains how TLS is configured and how to properly handle certificates.

## Security Model

### Default Behavior
- **TLS verification is ALWAYS enforced**
- System default CA certificates are used by default
- **`InsecureSkipVerify` option has been completely removed** for security reasons
- Minimum TLS version is set to TLS 1.2

### Why InsecureSkipVerify Was Removed

`InsecureSkipVerify` was a critical security vulnerability that allowed:
- Man-in-the-middle (MITM) attacks
- Attackers to intercept and modify notifications
- Exposure of sensitive authentication credentials
- Compromise of downstream systems relying on notifications

**Removing this option ensures your notifier cannot be configured insecurely, even by mistake.**

### Security Properties

The implementation enforces several key security properties:

- ✅ **TLS Verification is Mandatory** - No way to disable certificate validation
- ✅ **Minimum TLS Version** - TLS 1.2 minimum enforced (protects against known vulnerabilities)
- ✅ **Certificate Validation at Startup** - Invalid certificates detected immediately with clear error messages
- ✅ **Support for Custom CAs** - Self-signed certificates properly supported for internal services
- ✅ **No Bypass Possible** - Code prevents any way to skip verification

## Configuration

### Using System Default CA Certificates (Recommended)

For most deployments (including public ntfy.sh), simply omit the `ca_cert_path` setting:

```yaml
notifiers:
  ntfy:
    default:
      server_url: "https://ntfy.sh"
      default_topic: "my-topic"
      # No ca_cert_path specified = use system default CA certs
```

This is the default and most secure configuration.

### Using Custom CA Certificate (Self-Signed)

For self-hosted ntfy servers with self-signed certificates:

1. **Export the server's CA certificate in PEM format**
   ```bash
   # From the server
   openssl s_client -connect your-server.com:443 -showcerts < /dev/null | openssl x509 -outform PEM > ca.pem
   ```

2. **Configure the path in your notifier config**
   ```yaml
   notifiers:
     ntfy:
       default:
         server_url: "https://your-server.com"
         default_topic: "my-topic"
         ca_cert_path: "/etc/notifier/certs/ca.pem"  # Path to CA certificate file
   ```

3. **Ensure proper file permissions**
   ```bash
   chmod 644 /etc/notifier/certs/ca.pem
   chown notifier:notifier /etc/notifier/certs/ca.pem
   ```

## Certificate Requirements

### PEM Format
Certificates must be in PEM format (Base64-encoded X.509):

```
-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3...
... more base64 content ...
-----END CERTIFICATE-----
```

### CA Certificates
- The certificate file should contain the CA certificate (not the server certificate)
- Self-signed certificates must have `BasicConstraints: critical, CA:TRUE`
- Certificate chain is not necessary; use the root CA certificate

### Validation
The notifier validates certificate files at startup:
- File must exist and be readable
- File must be in valid PEM format
- Invalid certificates will prevent the service from starting with clear error messages

## Error Messages and Troubleshooting

### "CA certificate file not found: /path/to/cert.pem"
**Cause**: The specified certificate file doesn't exist
**Solution**: Verify the file path and ensure the file exists

### "CA certificate file is not in valid PEM format"
**Cause**: The file exists but is not valid PEM-formatted X.509 certificate
**Solution**: Export the certificate in PEM format using openssl

### "Failed to read custom CA certificate"
**Cause**: File permission issue
**Solution**: Check that the notifier process can read the file (usually needs world-readable or group-readable)

## Best Practices

### 1. Use Signed Certificates in Production
```yaml
# GOOD: Public CA-signed certificate
server_url: "https://ntfy.production.com"
# No ca_cert_path needed - system CAs will validate it
```

### 2. Use Custom CA for Self-Hosted Internal Services
```yaml
# GOOD: Self-signed internal server with explicit CA config
server_url: "https://ntfy.internal.company.com"
ca_cert_path: "/etc/notifier/certs/company-ca.pem"
```

### 3. Store Certificates Securely
```bash
# Recommended: Dedicated cert directory with restricted permissions
sudo mkdir -p /etc/notifier/certs
sudo chmod 700 /etc/notifier/certs
sudo cp ca.pem /etc/notifier/certs/
sudo chmod 644 /etc/notifier/certs/ca.pem
sudo chown notifier:notifier /etc/notifier/certs -R
```

### 4. Rotate Certificates Before Expiration
- Set calendar reminders for certificate expiration dates
- Update certificates at least 30 days before expiration
- Test certificate changes in staging before production deployment

### 5. Monitor Certificate Validity
```bash
# Check certificate expiration
openssl x509 -enddate -noout -in /etc/notifier/certs/ca.pem

# Example output:
# notAfter=Dec 25 10:00:00 2025 GMT
```

## Testing Certificate Configuration

### Test with OpenSSL
```bash
# Verify certificate is valid PEM
openssl x509 -in ca.pem -text -noout

# Test connection to ntfy server
openssl s_client -connect your-server.com:443 -CAfile ca.pem
```

### Test with notifier Client
```bash
# Test connection (will fail if cert is invalid)
notifier-client health --url https://your-server.com

# Or programmatically
curl -v --cacert ca.pem https://your-server.com/health
```

## Docker/Kubernetes Deployment

### Docker
```dockerfile
FROM notifier:latest

# Copy CA certificate
COPY ca.pem /etc/notifier/certs/ca.pem

# Reference in config
ENV NOTIFIER_NOTIFIERS_NTFY_DEFAULT_CA_CERT_PATH=/etc/notifier/certs/ca.pem
```

### Kubernetes
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ntfy-ca-cert
data:
  ca.pem: |
    -----BEGIN CERTIFICATE-----
    MIIDXTCCAkWgAwIBAgIJAJC1/iNAZwqDMA0GCSqGSIb3...
    -----END CERTIFICATE-----
---
apiVersion: v1
kind: Pod
metadata:
  name: notifier
spec:
  containers:
  - name: notifier
    image: notifier:latest
    volumeMounts:
    - name: ca-cert
      mountPath: /etc/notifier/certs
      readOnly: true
    env:
    - name: NOTIFIER_NOTIFIERS_NTFY_DEFAULT_CA_CERT_PATH
      value: /etc/notifier/certs/ca.pem
  volumes:
  - name: ca-cert
    configMap:
      name: ntfy-ca-cert
```

---

## Implementation Details

### Code Changes

The TLS security hardening involved changes to the ntfy notifier implementation:

**File**: `internal/notifier/ntfy.go`

#### 1. NtfyConfig Structure

Removed the insecure `InsecureSkipVerify` field and added proper certificate handling:

```go
type NtfyConfig struct {
    // ServerURL is the ntfy server URL (default: https://ntfy.sh)
    ServerURL string `mapstructure:"server_url"`

    // Token is the access token for authentication
    Token string `mapstructure:"token"`

    // Username for basic authentication (alternative to token)
    Username string `mapstructure:"username"`

    // Password for basic authentication (alternative to token)
    Password string `mapstructure:"password"`

    // DefaultTopic is the default topic if not specified in notification
    DefaultTopic string `mapstructure:"default_topic"`

    // CACertPath is the path to a custom CA certificate file (optional, PEM format)
    // Use this only for self-hosted ntfy servers with self-signed certificates.
    // If not specified, system default CA certificates are used.
    CACertPath string `mapstructure:"ca_cert_path"`

    // Default marks this instance as default
    Default bool `mapstructure:"default"`

    // AllowedRoles are roles allowed to use this notifier (empty = all authenticated)
    AllowedRoles []string `mapstructure:"allowed_roles"`
}
```

**Change**: Removed `InsecureSkipVerify bool` field, added `CACertPath string` field.

#### 2. Certificate Validation

Implemented validation functions that run at service startup:

**validateCACertPath(caCertPath string) error**
- Checks file exists and is readable
- Validates it's a regular file (not directory or symlink)
- Verifies PEM format with x509.NewCertPool().AppendCertsFromPEM()
- Provides clear error messages for each failure case
- Accepts empty string (uses system defaults)

**isPEMCertificate(data []byte) bool**
- Uses Go's x509 package to validate PEM format
- Returns true only for valid PEM certificates
- Returns false for invalid or corrupted formats

#### 3. HTTP Client Creation

Implemented `createNtfyHTTPClient(config *NtfyConfig) (*http.Client, error)`:

```go
func createNtfyHTTPClient(config *NtfyConfig) (*http.Client, error) {
    tlsConfig := &tls.Config{
        // TLS verification ALWAYS enforced (InsecureSkipVerify never set)
        MinVersion: tls.VersionTLS12,
    }

    // Load custom CA certificate if provided
    if config.CACertPath != "" {
        certData, err := os.ReadFile(config.CACertPath)
        if err != nil {
            return nil, fmt.Errorf("failed to read custom CA certificate: %w", err)
        }

        certPool := x509.NewCertPool()
        if !certPool.AppendCertsFromPEM(certData) {
            return nil, fmt.Errorf("failed to parse custom CA certificate as PEM")
        }

        tlsConfig.RootCAs = certPool
    }
    // If RootCAs is not set, the default system CA pool will be used

    transport := &http.Transport{
        TLSClientConfig: tlsConfig,
    }

    return &http.Client{
        Timeout:   30 * time.Second,
        Transport: transport,
    }, nil
}
```

**Key Security Properties**:
- `InsecureSkipVerify` is never set to true
- Minimum TLS 1.2 enforced
- Custom CA properly loaded via x509.NewCertPool
- System default CA used when CACertPath is empty
- Returns error if certificate is invalid

### Test Coverage

Comprehensive test suite validates all TLS security properties:

**File**: `internal/notifier/ntfy_tls_test.go`

**10 Tests Implemented**:

1. **TestNewNtfyNotifierWithDefaultCA** - Verifies system default CA used when CACertPath empty
2. **TestNewNtfyNotifierWithCustomCA** - Verifies custom CA certificate loads successfully
3. **TestValidateCACertPathNotFound** - Rejects non-existent certificate files
4. **TestValidateCACertPathInvalidFormat** - Rejects invalid PEM format
5. **TestValidateCACertPathIsDirectory** - Rejects directory paths
6. **TestValidateCACertPathEmpty** - Allows empty CA cert path (uses system defaults)
7. **TestTLSConfigHasMinimumVersion** - Verifies TLS 1.2 minimum enforced
8. **TestTLSConfigNeverSkipsVerification** - **CRITICAL**: Verifies InsecureSkipVerify never true
9. **TestCustomCACertLoading** - Verifies custom CA cert properly loaded into cert pool
10. **TestMissingCAFileError** - Verifies clear error messages for missing files

**Test Results**: ✅ All 10/10 tests PASSING

**Test Coverage Includes**:
- ✅ System default CA pool usage
- ✅ Custom CA certificate loading
- ✅ Invalid file rejection
- ✅ Invalid format rejection
- ✅ Directory path rejection
- ✅ TLS version enforcement
- ✅ TLS verification enforcement
- ✅ Error message clarity
- ✅ Edge cases (empty files, missing files, permission issues)

### Acceptance Criteria Met

✅ **Removed InsecureSkipVerify Completely**
- Field removed from NtfyConfig struct
- No way to create insecure configurations

✅ **TLS Verification Always Enforced**
- `tls.Config.InsecureSkipVerify` never set to true
- TestTLSConfigNeverSkipsVerification verifies this critical property
- Minimum TLS version 1.2 enforced

✅ **Custom CA Support Works**
- CACertPath field added and validated
- Certificates validated at service startup
- Tests: TestNewNtfyNotifierWithCustomCA and TestCustomCACertLoading pass

✅ **Error Messages Clear and Helpful**
- "CA certificate file not found: /path/to/cert.pem"
- "CA certificate file is not in valid PEM format: /path"
- "CA certificate file error: permission denied"

✅ **No Ability to Bypass Certificate Validation**
- InsecureSkipVerify field removed from production code
- TLS config always includes verification
- No conditional path that disables verification
- Code review confirms no skip verify anywhere

### Files Modified

1. `internal/notifier/ntfy.go` - Core TLS implementation (lines 6-182)
2. `internal/notifier/ntfy_tls_test.go` - Comprehensive test suite (NEW, 350 lines)
3. `pkg/client/types.go` - Updated ClientConfig documentation

### Code Quality

- ✅ All code formatted with `gofmt`
- ✅ All code passes `go vet`
- ✅ No warnings or errors
- ✅ Proper error handling with wrapped errors
- ✅ Clear code comments explaining security decisions

---

## Migration from InsecureSkipVerify

If you were previously using `insecure_skip_verify: true`, follow these steps:

1. **For public services (ntfy.sh)**:
   - Simply remove the `insecure_skip_verify: true` line
   - No other changes needed

2. **For self-hosted services**:
   - Export the CA certificate: `openssl s_client -connect server.com:443 -showcerts < /dev/null | openssl x509 -outform PEM > ca.pem`
   - Add `ca_cert_path: "/path/to/ca.pem"` to your config
   - Test to verify connectivity works
   - Remove `insecure_skip_verify: true` line

3. **Test thoroughly** before deploying to production

## Security Audit

✅ **TLS Verification is Mandatory**
- No way to disable certificate validation
- System always enforces proper TLS handshake
- Man-in-the-middle attacks are prevented

✅ **Certificate Validation at Startup**
- Invalid certificates detected at service start
- Clear error messages guide proper configuration
- Prevents running with misconfigured certificates

✅ **Minimum TLS Version**
- TLS 1.2 minimum (more secure than TLS 1.0/1.1)
- Protects against known TLS vulnerabilities
- Aligns with industry best practices and compliance standards (PCI-DSS, NIST, etc.)

✅ **Support for Custom CAs**
- Self-signed certificates properly supported
- No need to use insecure configuration methods
- Proper separation of public CA and custom CA handling

## References

- [RFC 5246: TLS 1.2](https://tools.ietf.org/html/rfc5246)
- [OWASP: Transport Layer Protection](https://owasp.org/www-community/controls/Transport_Layer_Protection)
- [OpenSSL Certificate Usage](https://www.openssl.org/docs/man1.0.2/man1/x509.html)
- [Go crypto/tls Documentation](https://golang.org/pkg/crypto/tls/)
