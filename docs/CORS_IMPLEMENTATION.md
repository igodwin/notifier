# CORS Implementation - Security Enhancement

## Overview

This document describes the implementation of a secure CORS (Cross-Origin Resource Sharing) configuration system that replaces the previous wildcard (`*`) configuration with an explicit origin whitelist to prevent CSRF attacks.

## Implementation Summary

### 1. CORSConfig Structure (api/rest/router.go)

Created a comprehensive `CORSConfig` struct with the following fields:

```go
type CORSConfig struct {
    AllowedOrigins   []string  // Whitelist of allowed origins
    AllowedMethods   []string  // Allowed HTTP methods
    AllowedHeaders   []string  // Allowed HTTP headers
    AllowCredentials bool      // Whether to allow credentials
    MaxAge           int       // Cache duration for preflight responses
}
```

**Key Security Features:**
- Wildcards (`*`) are explicitly NOT supported
- Origins must be explicitly whitelisted
- Credentials can only be enabled with specific origins

### 2. CORS Middleware Implementation

The `newCORSMiddleware()` function implements:

- **Origin Validation**: Checks incoming `Origin` header against whitelist
- **Exact Match Required**: Only sets CORS headers if origin is in whitelist
- **Never Uses Wildcard**: Always returns the exact origin, never `*`
- **Preflight Handling**: Properly handles OPTIONS requests
- **Configurable Headers**: All CORS headers are configurable

**Code Location:** `api/rest/router.go:98-149`

### 3. Configuration System

#### Config Structure (internal/config/config.go)

Added `CORSConfig` to the main application configuration:

```go
type CORSConfig struct {
    AllowedOrigins   []string `mapstructure:"allowed_origins"`
    AllowedMethods   []string `mapstructure:"allowed_methods"`
    AllowedHeaders   []string `mapstructure:"allowed_headers"`
    AllowCredentials bool     `mapstructure:"allow_credentials"`
    MaxAge           int      `mapstructure:"max_age"`
}
```

#### Default Values (internal/config/config.go:205-210)

```go
AllowedOrigins:   []string{}                                   // Empty by default
AllowedMethods:   []string{"GET", "POST", "OPTIONS", "DELETE"} // Standard REST methods
AllowedHeaders:   []string{"Content-Type", "Authorization"}    // Common headers
AllowCredentials: false                                         // Disabled by default
MaxAge:           3600                                          // 1 hour
```

### 4. Configuration Validation (internal/config/config.go:263-282)

Implemented `validateCORS()` that enforces:

1. **Wildcard Rejection**: `*` is not allowed in `allowed_origins`
2. **Origin Format Validation**: Origins must start with `http://` or `https://`
3. **Credentials Validation**: `allow_credentials` requires at least one origin

**Example Error Messages:**
- `"wildcard (*) is not allowed in CORS allowed_origins for security reasons - specify exact origins instead"`
- `"invalid origin format: example.com - origins must start with http:// or https://"`
- `"allow_credentials is enabled but no origins are allowed - this configuration is ineffective"`

### 5. Updated Server Initialization (cmd/server/main.go:288-326)

The `startRESTServer()` function now:

1. Converts config CORS to `rest.CORSConfig`
2. Logs CORS configuration on startup
3. Warns if no origins are configured
4. Passes CORS config to router

**Example Log Output:**
```
CORS enabled for origins: [http://localhost:3000 http://localhost:8080]
```

Or:
```
CORS has no allowed origins configured - all cross-origin requests will be blocked
```

### 6. Configuration Example (config.yaml:112-146)

#### Development Configuration
```yaml
cors:
  allowed_origins:
    - "http://localhost:3000"    # React/Next.js dev port
    - "http://localhost:8080"    # Vue/Angular dev port
    - "http://localhost:5173"    # Vite dev server
  allowed_methods:
    - "GET"
    - "POST"
    - "OPTIONS"
    - "DELETE"
  allowed_headers:
    - "Content-Type"
    - "Authorization"
  allow_credentials: false
  max_age: 3600
```

#### Production Configuration (Example)
```yaml
cors:
  allowed_origins:
    - "https://app.example.com"
    - "https://dashboard.example.com"
    - "https://api-docs.example.com"
  allowed_methods:
    - "GET"
    - "POST"
    - "OPTIONS"
    - "DELETE"
  allowed_headers:
    - "Content-Type"
    - "Authorization"
  allow_credentials: true  # Enable for auth tokens
  max_age: 3600
```

## Test Coverage

### 1. CORS Middleware Tests (api/rest/cors_test.go)

**Test Cases:**
- ✅ `TestCORSMiddleware_AllowedOrigin`: Verifies allowed origins are accepted
- ✅ `TestCORSMiddleware_BlockedOrigin`: Verifies non-whitelisted origins are rejected
- ✅ `TestCORSMiddleware_PreflightRequest`: Tests OPTIONS preflight handling
- ✅ `TestCORSMiddleware_Credentials`: Verifies credential header handling
- ✅ `TestCORSMiddleware_NoWildcard`: Ensures wildcard is never returned
- ✅ `TestCORSMiddleware_EmptyConfig`: Tests secure default (no origins)
- ✅ `TestCORSMiddleware_MaxAge`: Tests cache duration configuration
- ✅ `TestDefaultCORSConfig`: Verifies default configuration values

**Total: 8 test functions, 21 test cases**

### 2. CORS Validation Tests (internal/config/cors_test.go)

**Test Cases:**
- ✅ `TestValidateCORS_WildcardRejection`: Wildcard origins are rejected
- ✅ `TestValidateCORS_InvalidOriginFormat`: Invalid origin formats are rejected
- ✅ `TestValidateCORS_CredentialsWithoutOrigins`: Credentials require origins
- ✅ `TestValidateCORS_ValidConfigurations`: Valid configs are accepted
- ✅ `TestValidateCORS_MultipleOrigins`: Multiple origins with wildcard rejected
- ✅ `TestValidateCORS_EdgeCases`: Edge cases handled correctly

**Total: 6 test functions, 16 test cases**

## Security Improvements

### Before
```go
w.Header().Set("Access-Control-Allow-Origin", "*")  // ❌ CSRF Vulnerability
```

### After
```go
// Only set CORS headers if origin is in whitelist
if allowed {
    w.Header().Set("Access-Control-Allow-Origin", origin)  // ✅ Exact origin
}
// Otherwise, no CORS headers are set (browser blocks the response)
```

## Security Guarantees

1. **No Wildcard**: The system makes it impossible to configure wildcard CORS
2. **Explicit Whitelist**: All allowed origins must be explicitly configured
3. **Validation at Startup**: Invalid configurations are rejected before the server starts
4. **Default Secure**: Empty origin list by default (most restrictive)
5. **Environment-Specific**: Different configs for dev/staging/production

## Migration Guide

### For Development

Update your `config.yaml` to include localhost origins:

```yaml
cors:
  allowed_origins:
    - "http://localhost:3000"
    - "http://localhost:8080"
```

### For Production

Configure your production origins:

```yaml
cors:
  allowed_origins:
    - "https://yourdomain.com"
    - "https://app.yourdomain.com"
  allow_credentials: true
```

### Environment Variables

You can also configure CORS via environment variables:

```bash
export NOTIFIER_CORS_ALLOWED_ORIGINS="https://app.example.com,https://dashboard.example.com"
export NOTIFIER_CORS_ALLOW_CREDENTIALS=true
```

## Acceptance Criteria

✅ **CORS whitelist fully configurable** - via config.yaml or environment variables
✅ **Wildcard configuration is impossible** - validation rejects `*` at startup
✅ **Security headers properly set** - only for whitelisted origins
✅ **Environment-specific configs work** - examples provided for dev/prod
✅ **No CSRF vulnerability** - wildcard eliminated, exact origins only
✅ **Comprehensive tests** - 37 test cases covering all scenarios

## Files Modified

1. `api/rest/router.go` - CORS config struct and middleware
2. `api/rest/cors_test.go` - CORS middleware tests (NEW)
3. `internal/config/config.go` - CORS configuration structure
4. `internal/config/cors_test.go` - CORS validation tests (NEW)
5. `cmd/server/main.go` - Server initialization with CORS config
6. `config.yaml` - Example CORS configuration
7. `internal/auth/bootstrap.go` - Fixed unrelated string formatting bug
8. `internal/auth/keystore_hybrid.go` - Fixed unrelated rate limiter signature

## Additional Fixes

While implementing CORS, also fixed pre-existing build errors:
- String multiplication in `bootstrap.go` (changed to `strings.Repeat`)
- Rate limiter signature mismatch in `keystore_hybrid.go`
- Missing parameter in `service_retention_test.go`

## References

- **OWASP CORS**: https://owasp.org/www-community/attacks/csrf
- **MDN CORS**: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
- **RFC 6454**: The Web Origin Concept
