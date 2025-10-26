# Code Duplication Refactoring - Quick Reference Guide

**Status**: Ready to Implement
**Total Time**: 2.5 hours
**Complexity**: LOW
**Risk**: MINIMAL

---

## Quick Overview

| Priority | Issue | Files | Lines | Time | Status |
|----------|-------|-------|-------|------|--------|
| 🔴 HIGH | Auth validation dedup | 3 | 32 | 30m | Ready |
| 🟡 MEDIUM | API key extraction | 2 | 15 | 20m | Ready |
| 🟡 MEDIUM | Notifier registration | 1 | 30 | 45m | Ready |
| 🟢 LOW | Error result helpers | 3 | 15 | 25m | Ready |

---

## Phase 1: Quick Wins (1 hour) ⚡

### Issue #2: Extract Bearer Token Parsing

**Time**: 20 minutes
**Files Changed**: 2
**Lines Removed**: 15

**Step 1**: Add helper to `internal/auth/auth.go`
```go
// At package level, add after APIKeyStore definition:

// extractBearerToken extracts bearer token from Authorization header
func extractBearerToken(authHeader string) string {
    if authHeader == "" {
        return ""
    }
    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
        return parts[1]
    }
    return ""
}
```

**Step 2**: Update `internal/auth/rest_middleware.go`

Replace lines 73-89:
```go
// OLD (17 lines):
func (m *RESTAuthMiddleware) extractAPIKey(r *http.Request) string {
    authHeader := r.Header.Get("Authorization")
    if authHeader != "" {
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
            return parts[1]
        }
    }

    if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
        return apiKey
    }

    return ""
}

// NEW (6 lines):
func (m *RESTAuthMiddleware) extractAPIKey(r *http.Request) string {
    if token := extractBearerToken(r.Header.Get("Authorization")); token != "" {
        return token
    }
    return r.Header.Get("X-API-Key")
}
```

**Step 3**: Update `internal/auth/grpc_middleware.go`

Replace lines 129-150:
```go
// OLD (22 lines):
func (m *GRPCAuthMiddleware) extractAPIKey(ctx context.Context) string {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return ""
    }

    if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
        authHeader := authHeaders[0]
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
            return parts[1]
        }
    }

    if keyHeaders := md.Get("x-api-key"); len(keyHeaders) > 0 {
        return keyHeaders[0]
    }

    return ""
}

// NEW (11 lines):
func (m *GRPCAuthMiddleware) extractAPIKey(ctx context.Context) string {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return ""
    }

    if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
        if token := extractBearerToken(authHeaders[0]); token != "" {
            return token
        }
    }

    return getFirstIfPresent(md.Get("x-api-key"))
}

// Add helper for getting first element
func getFirstIfPresent(vals []string) string {
    if len(vals) > 0 {
        return vals[0]
    }
    return ""
}
```

**Verification**:
```bash
go test ./internal/auth -v
# Should pass all auth tests
```

---

### Issue #4: Add Result Helpers to BaseNotifier

**Time**: 25 minutes
**Files Changed**: 4 (notifier.go + 3 notifiers)
**Lines Removed**: 15

**Step 1**: Update `internal/notifier/notifier.go`

Add to `BaseNotifier` struct (after `Type()` method):
```go
// ErrorResult returns a failed notification result
func (b *BaseNotifier) ErrorResult(notification *domain.Notification, err error) *domain.NotificationResult {
    return &domain.NotificationResult{
        NotificationID: notification.ID,
        Success:        false,
        Error:          err.Error(),
        SentAt:         time.Now(),
    }
}

// SuccessResult returns a successful notification result
func (b *BaseNotifier) SuccessResult(
    notification *domain.Notification,
    message string,
    recipientCount int,
    providerResponse map[string]interface{},
) *domain.NotificationResult {
    if message == "" {
        message = fmt.Sprintf("Notification sent to %d recipient(s)", recipientCount)
    }
    return &domain.NotificationResult{
        NotificationID: notification.ID,
        Success:        true,
        Message:        message,
        SentAt:         time.Now(),
        ProviderResponse: providerResponse,
    }
}
```

**Step 2**: Update each notifier's Send() method

In `internal/notifier/slack.go` (line ~93):
```go
// OLD:
return &domain.NotificationResult{
    NotificationID: notification.ID,
    Success:        false,
    Error:          err.Error(),
    SentAt:         time.Now(),
}, err

// NEW:
return s.ErrorResult(notification, err), err
```

In `internal/notifier/slack.go` (line ~102):
```go
// OLD:
return &domain.NotificationResult{
    NotificationID: notification.ID,
    Success:        true,
    Message:        fmt.Sprintf("Slack notification sent to %d channels", len(notification.Recipients)),
    SentAt:         time.Now(),
    ProviderResponse: map[string]interface{}{
        "channels": notification.Recipients,
    },
}, nil

// NEW:
return s.SuccessResult(
    notification,
    "",  // Will use default message
    len(notification.Recipients),
    map[string]interface{}{"channels": notification.Recipients},
), nil
```

Do the same for: `ntfy.go`, `smtp.go`, `stdout.go`

**Verification**:
```bash
go test ./internal/notifier -v
# Should pass all notifier tests
```

---

## Phase 2: Core Refactoring (1.5 hours) 🔧

### Issue #1: Extract Auth Validation

**Time**: 30 minutes
**Files Changed**: 3
**Lines Removed**: 32

**Step 1**: Add validation method to auth middleware base

In `internal/auth/auth.go`, add:
```go
// validateAndAuthorize performs validation and authorization checks
// Returns error if validation fails, or logs but continues on UpdateLastUsed failure
func (m *baseAuthMiddleware) validateAndAuthorize(apiKey string) (*APIKey, error) {
    // Validate API key
    key, err := m.store.ValidateKey(apiKey)
    if err != nil {
        return nil, err
    }

    // Check rate limit
    allowed, err := m.store.CheckRateLimit(apiKey)
    if err != nil || !allowed {
        return nil, ErrRateLimited
    }

    // Update last used (log but don't fail on error)
    if err := m.store.UpdateLastUsed(apiKey); err != nil {
        m.logger.Errorf("Failed to update last used time: %v", err)
    }

    return key, nil
}

// Define base struct for shared auth logic
type baseAuthMiddleware struct {
    store  *APIKeyStore
    logger *logging.Logger
}
```

**Step 2**: Update `RESTAuthMiddleware` in `rest_middleware.go`

```go
// Change struct to embed base:
type RESTAuthMiddleware struct {
    baseAuthMiddleware
}

// Update Middleware method:
func (m *RESTAuthMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := m.extractAPIKey(r)
        if apiKey == "" {
            m.logger.Warnf("REST: Missing API key in request from %s", r.RemoteAddr)
            http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
            return
        }

        // Use extracted validation method
        key, err := m.validateAndAuthorize(apiKey)
        if err != nil {
            m.logger.Warnf("REST: Validation failed from %s: %v", r.RemoteAddr, err)
            http.Error(w, "Invalid API key or rate limited", http.StatusUnauthorized)
            return
        }

        // Create auth context
        authCtx := &AuthContext{
            APIKey:   key,
            ClientID: key.ClientID,
            Roles:    key.Roles,
        }

        ctx := ContextWithAuth(r.Context(), authCtx)
        m.logger.Debugf("REST: Authenticated client=%s with roles=%v", key.ClientID, key.Roles)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Step 3**: Update `GRPCAuthMiddleware` in `grpc_middleware.go`

```go
// Change struct:
type GRPCAuthMiddleware struct {
    baseAuthMiddleware
}

// Update UnaryInterceptor:
func (m *GRPCAuthMiddleware) UnaryInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        apiKey := m.extractAPIKey(ctx)
        if apiKey == "" {
            m.logger.Warnf("gRPC: Missing API key for method=%s", info.FullMethod)
            return nil, status.Error(codes.Unauthenticated, "Missing API key")
        }

        key, err := m.validateAndAuthorize(apiKey)
        if err != nil {
            m.logger.Warnf("gRPC: Validation failed for method=%s: %v", info.FullMethod, err)
            return nil, status.Error(codes.Unauthenticated, "Invalid API key or rate limited")
        }

        authCtx := &AuthContext{
            APIKey:   key,
            ClientID: key.ClientID,
            Roles:    key.Roles,
        }

        newCtx := ContextWithAuth(ctx, authCtx)
        m.logger.Debugf("gRPC: Authenticated client=%s method=%s", key.ClientID, info.FullMethod)

        return handler(newCtx, req)
    }
}

// Update StreamInterceptor similarly
```

**Verification**:
```bash
go test ./internal/auth -v
# Should pass all auth tests
```

---

### Issue #3: Extract Notifier Registration Pattern

**Time**: 45 minutes
**Files Changed**: 1
**Lines Removed**: 30

**Step 1**: Add generic registration helper to `cmd/server/main.go`

```go
// notifierRegistration handles the common pattern for registering notifiers
type notifierRegistration struct {
    notifyType domain.NotificationType
    accountMap map[string]interface{}  // Generic map of configs
    creator    func(string, interface{}) (domain.Notifier, error)
    isDefault  func(interface{}) bool
}

func registerNotifierType(
    reg notifierRegistration,
    factory *notifier.Factory,
    logger *logging.Logger,
) {
    for accountName, config := range reg.accountMap {
        notif, err := reg.creator(accountName, config)
        if err != nil {
            logger.Warnf("Failed to create %s notifier for account '%s': %v",
                reg.notifyType, accountName, err)
            continue
        }

        if err := factory.RegisterNotifier(reg.notifyType, accountName, notif); err != nil {
            logger.Fatalf("Failed to register %s notifier for account '%s': %v",
                reg.notifyType, accountName, err)
        }

        defaultStr := ""
        if reg.isDefault(config) {
            defaultStr = " (default)"
        }
        logger.Infof("Registered %s notifier for account '%s'%s",
            reg.notifyType, accountName, defaultStr)
    }
}
```

**Step 2**: Refactor `registerNotifiers()` function

```go
// OLD: 58 lines with repeated pattern

func registerNotifiers(cfg *config.Config, factory *notifier.Factory, logger *logging.Logger) {
    if cfg.Notifiers.Stdout {
        stdoutNotifier := notifier.NewStdoutNotifier()
        if err := factory.RegisterNotifier(domain.TypeStdout, "", stdoutNotifier); err != nil {
            logger.Fatalf("Failed to register stdout notifier: %v", err)
        }
        logger.Info("Registered stdout notifier")
    }

    // Convert config maps to interface{} for generic handler
    smtpMap := make(map[string]interface{})
    for k, v := range cfg.Notifiers.SMTP {
        smtpMap[k] = v
    }

    registerNotifierType(notifierRegistration{
        notifyType: domain.TypeEmail,
        accountMap: smtpMap,
        creator: func(accountName string, config interface{}) (domain.Notifier, error) {
            return notifier.NewSMTPNotifier(config.(*config.SMTPConfig))
        },
        isDefault: func(config interface{}) bool {
            return config.(*config.SMTPConfig).Default
        },
    }, factory, logger)

    slackMap := make(map[string]interface{})
    for k, v := range cfg.Notifiers.Slack {
        slackMap[k] = v
    }

    registerNotifierType(notifierRegistration{
        notifyType: domain.TypeSlack,
        accountMap: slackMap,
        creator: func(accountName string, config interface{}) (domain.Notifier, error) {
            return notifier.NewSlackNotifier(config.(*config.SlackConfig))
        },
        isDefault: func(config interface{}) bool {
            return config.(*config.SlackConfig).Default
        },
    }, factory, logger)

    // Same for Ntfy...
}

// NEW: 28 lines (30 lines removed)
```

**Verification**:
```bash
go build ./cmd/server
# Should compile and run successfully
```

---

## Testing Checklist

### Before Starting
- [ ] All tests passing: `go test ./...`
- [ ] Code compiles: `go build ./cmd/server`

### After Phase 1
- [ ] `go test ./internal/auth -v` - All auth tests pass
- [ ] `go test ./internal/notifier -v` - All notifier tests pass
- [ ] `go build ./cmd/server` - Server builds successfully
- [ ] Verify no behavior changes with `go test ./...`

### After Phase 2
- [ ] All tests pass: `go test ./...`
- [ ] `go fmt ./...` - Code is formatted
- [ ] `go vet ./...` - No vet issues
- [ ] `go build ./cmd/server && ./server` - Runs successfully

---

## Rollback Instructions

Each refactoring is a separate commit, so if issues arise:

```bash
# Rollback Phase 1
git revert <phase-1-commit>

# Or rollback specific issue
git revert <issue-2-commit>
git revert <issue-4-commit>
```

---

## Summary

**Total Time Investment**: 2.5 hours
**Lines Removed**: ~92 lines
**Complexity Reduction**: ~40-50%
**Risk Level**: MINIMAL
**Benefits**:
- Easier maintenance
- Reduced duplication
- Better consistency
- Easier to test
- No behavior changes

**Recommendation**: Implement Phase 1 first (1 hour, high value), then Phase 2 in next sprint.
