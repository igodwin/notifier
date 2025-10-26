# Remediation Action Plan

## Quick Reference

| Priority | Count | Effort | Timeline | Status |
|----------|-------|--------|----------|--------|
| Critical | 3 | High | Week 1 | 🔴 Not Started |
| High | 7 | High | Week 2-3 | 🔴 Not Started |
| Medium | 30 | Medium | Sprint 2-3 | 🔴 Not Started |
| Low | 10 | Low | Ongoing | 🔴 Not Started |

---

## Phase 1: Critical Issues (Week 1)

### CRITICAL-1: Unbounded Memory Growth
**Status**: 🔴 Not Started
**Effort**: 4-6 hours
**File**: `internal/service/service.go`

#### Implementation Steps:

1. Create retention policy configuration:
```go
// In config.go
type NotificationRetentionConfig struct {
    Enabled   bool          `mapstructure:"enabled"`
    TTL       time.Duration `mapstructure:"ttl"`        // Default: 7 days
    CheckFrequency time.Duration `mapstructure:"check_frequency"` // Default: 1 hour
    MaxSize   int           `mapstructure:"max_size"`   // Default: 100,000
}
```

2. Add to service initialization:
```go
// In service.go
type NotificationService struct {
    // ... existing fields ...
    retentionConfig *config.NotificationRetentionConfig
    cleanupDone      chan struct{}
}

func NewNotificationService(
    factory domain.NotifierFactory,
    q domain.Queue,
    workerCount int,
    cfg *config.Config,
    retentionCfg *config.NotificationRetentionConfig,
    logger *logging.Logger,
) *NotificationService {
    // ... existing code ...
    svc := &NotificationService{
        // ... initialization ...
        retentionConfig: retentionCfg,
        cleanupDone:     make(chan struct{}),
    }

    // Start cleanup goroutine if enabled
    if retentionCfg.Enabled {
        go svc.cleanupLoop()
    }

    return svc
}

func (s *NotificationService) cleanupLoop() {
    ticker := time.NewTicker(s.retentionConfig.CheckFrequency)
    defer ticker.Stop()

    for {
        select {
        case <-s.stopChan:
            return
        case <-ticker.C:
            s.cleanupExpiredNotifications()
        }
    }
}

func (s *NotificationService) cleanupExpiredNotifications() {
    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-s.retentionConfig.TTL)
    count := 0

    for id, notif := range s.notifications {
        if notif.CreatedAt.Before(cutoff) {
            delete(s.notifications, id)
            count++
        }
    }

    // Also check max size
    if len(s.notifications) > s.retentionConfig.MaxSize {
        // Sort by creation time and remove oldest
        var notifs []*domain.Notification
        for _, n := range s.notifications {
            notifs = append(notifs, n)
        }
        sort.Slice(notifs, func(i, j int) bool {
            return notifs[i].CreatedAt.Before(notifs[j].CreatedAt)
        })

        toRemove := len(notifs) - s.retentionConfig.MaxSize
        for i := 0; i < toRemove; i++ {
            delete(s.notifications, notifs[i].ID)
            count++
        }
    }

    if count > 0 {
        s.logger.Infof("Cleaned up %d expired notifications", count)
    }
}

func (s *NotificationService) Stop() error {
    // ... existing stop code ...
    <-s.cleanupDone  // Wait for cleanup to finish
    return nil
}
```

3. Update configuration defaults:
```yaml
# config.yaml
notification:
  retention:
    enabled: true
    ttl: 168h  # 7 days
    check_frequency: 1h
    max_size: 100000
```

4. Add tests:
```go
func TestNotificationCleanup(t *testing.T) {
    // Test TTL-based removal
    // Test max size enforcement
    // Test cleanup frequency
}
```

**Acceptance Criteria**:
- [ ] Notifications older than TTL are removed
- [ ] Maximum size limit is enforced
- [ ] Cleanup runs at specified frequency
- [ ] Memory doesn't grow indefinitely
- [ ] Tests pass with 100% coverage

---

### CRITICAL-2: Remove TLS Verification Bypass
**Status**: 🔴 Not Started
**Effort**: 2-3 hours
**File**: `internal/notifier/ntfy.go`

#### Implementation Steps:

1. Update NtfyConfig:
```go
// Remove InsecureSkipVerify, add custom CA support
type NtfyConfig struct {
    ServerURL   string `mapstructure:"server_url"`
    Token       string `mapstructure:"token"`
    Username    string `mapstructure:"username"`
    Password    string `mapstructure:"password"`
    DefaultTopic string `mapstructure:"default_topic"`
    // REMOVED: InsecureSkipVerify bool

    // ADD: Custom CA certificate support
    CACertPath  string `mapstructure:"ca_cert_path"`  // Path to CA cert file
    Default     bool   `mapstructure:"default"`
    AllowedRoles []string `mapstructure:"allowed_roles"`
}

func (nc *NtfyConfig) Validate() error {
    if nc.ServerURL == "" {
        return fmt.Errorf("server_url is required")
    }
    if nc.Token == "" && (nc.Username == "" || nc.Password == "") {
        return fmt.Errorf("either token or username/password required")
    }
    // CACertPath is optional but if provided, must exist
    if nc.CACertPath != "" {
        if _, err := os.Stat(nc.CACertPath); err != nil {
            return fmt.Errorf("ca_cert_path file not found: %w", err)
        }
    }
    return nil
}
```

2. Update HTTP client creation:
```go
func NewNtfyNotifier(config *NtfyConfig) (*NtfyNotifier, error) {
    if config == nil {
        return nil, fmt.Errorf("ntfy config is required")
    }

    if err := config.Validate(); err != nil {
        return nil, err
    }

    httpClient, err := createNtfyHTTPClient(config)
    if err != nil {
        return nil, err
    }

    notifier := &NtfyNotifier{
        config:     config,
        httpClient: httpClient,
    }
    notifier.BaseNotifier.notificationType = domain.TypeNtfy
    return notifier, nil
}

func createNtfyHTTPClient(config *NtfyConfig) (*http.Client, error) {
    var tlsConfig *tls.Config

    if config.CACertPath != "" {
        caCert, err := ioutil.ReadFile(config.CACertPath)
        if err != nil {
            return nil, fmt.Errorf("failed to read CA cert: %w", err)
        }

        caCertPool := x509.NewCertPool()
        if !caCertPool.AppendCertsFromPEM(caCert) {
            return nil, fmt.Errorf("failed to parse CA cert")
        }

        tlsConfig = &tls.Config{
            RootCAs: caCertPool,
        }
    } else {
        // Use system default CA pool
        tlsConfig = &tls.Config{}
    }

    return &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            TLSClientConfig: tlsConfig,
            MaxIdleConns:    100,
            IdleConnTimeout: 90 * time.Second,
        },
    }, nil
}
```

3. Update documentation:
```markdown
## TLS Configuration

### System Default (Recommended)
```yaml
notifiers:
  ntfy:
    default:
      server_url: "https://ntfy.sh"
      token: "your-token"
      # Uses system CA certificates automatically
```

### Custom CA Certificate (Self-Signed)
```yaml
notifiers:
  ntfy:
    default:
      server_url: "https://internal-ntfy.example.com"
      token: "your-token"
      ca_cert_path: "/etc/certs/ca.pem"
```

**IMPORTANT**: TLS verification is ALWAYS enabled. Insecure self-signed certificates cannot be accepted without providing a valid CA certificate.
```

4. Add tests:
```go
func TestNtfyTLSValidation(t *testing.T) {
    // Test that invalid certs are rejected
    // Test custom CA cert acceptance
    // Test system CA pool usage
}
```

**Acceptance Criteria**:
- [ ] InsecureSkipVerify option removed
- [ ] Custom CA certificate support works
- [ ] TLS validation always enabled
- [ ] Error messages clear when certs invalid
- [ ] Documentation updated
- [ ] Tests pass

---

### CRITICAL-3: Fix CORS Configuration
**Status**: 🔴 Not Started
**Effort**: 2-3 hours
**File**: `api/rest/router.go`

#### Implementation Steps:

1. Update CORS middleware:
```go
type CORSConfig struct {
    AllowedOrigins   []string
    AllowedMethods   []string
    AllowedHeaders   []string
    AllowCredentials bool
    MaxAge           int
}

func newCORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
    // Build origin map for O(1) lookup
    originMap := make(map[string]bool)
    for _, origin := range config.AllowedOrigins {
        originMap[origin] = true
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            // Check if origin is allowed
            if origin != "" && originMap[origin] {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                if config.AllowCredentials {
                    w.Header().Set("Access-Control-Allow-Credentials", "true")
                }
            }

            w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
            w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
            w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusOK)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func NewRouterWithAuth(service domain.NotificationService, logger *logging.Logger, authStore *auth.APIKeyStore, corsConfig *CORSConfig) *mux.Router {
    handler := NewHandler(service, logger)
    router := mux.NewRouter()

    // API v1 routes with CORS
    v1 := router.PathPrefix("/api/v1").Subrouter()
    v1.Use(newCORSMiddleware(corsConfig))

    // ... rest of router setup ...
}
```

2. Add configuration:
```yaml
server:
  cors:
    allowed_origins:
      - "https://example.com"
      - "https://app.example.com"
    allowed_methods:
      - "GET"
      - "POST"
      - "OPTIONS"
      - "DELETE"
    allowed_headers:
      - "Content-Type"
      - "Authorization"
    allow_credentials: true
    max_age: 3600
```

3. Update main.go:
```go
corsConfig := &rest.CORSConfig{
    AllowedOrigins:   cfg.Server.CORS.AllowedOrigins,
    AllowedMethods:   cfg.Server.CORS.AllowedMethods,
    AllowedHeaders:   cfg.Server.CORS.AllowedHeaders,
    AllowCredentials: cfg.Server.CORS.AllowCredentials,
    MaxAge:           cfg.Server.CORS.MaxAge,
}

restServer := startRESTServer(ctx, &wg, cfg, svc, logger, authStore, corsConfig)
```

4. Add tests and validation:
```go
func TestCORSOriginValidation(t *testing.T) {
    // Test allowed origins accepted
    // Test disallowed origins rejected
    // Test preflight requests
    // Test credentials header handling
}
```

**Acceptance Criteria**:
- [ ] CORS origins are configurable
- [ ] Wildcard (*) is not accepted
- [ ] Only configured origins are allowed
- [ ] Preflight requests handled correctly
- [ ] Configuration validated at startup
- [ ] Tests pass

---

## Phase 2: High Priority Issues (Week 2-3)

### HIGH-1: Add Request Size Limits
**Status**: 🔴 Not Started
**Effort**: 2 hours
**File**: `api/rest/handlers.go`

**Steps**:
1. Add constant: `const MaxRequestSize = 10 * 1024 * 1024  // 10MB`
2. Update SendNotification handler: Add `http.MaxBytesReader()`
3. Update SendBatchNotifications handler: Add `http.MaxBytesReader()`
4. Add configuration option for max size
5. Add tests for size limit enforcement

### HIGH-2: Implement Sharded Locks
**Status**: 🔴 Not Started
**Effort**: 6-8 hours
**File**: `internal/service/service.go`

**Steps**:
1. Create sharded storage structure
2. Implement shard index function (fnv hash)
3. Update all notification operations
4. Add benchmarks comparing to original
5. Add concurrency tests

### HIGH-3: Fix Lock Ordering Issues
**Status**: 🔴 Not Started
**Effort**: 4 hours
**File**: `internal/queue/local.go`

**Steps**:
1. Release locks before channel operations
2. Copy references while holding locks
3. Add timeout for channel operations
4. Add deadlock detection tests

### HIGH-4: Separate Service Concerns
**Status**: 🔴 Not Started
**Effort**: 16-20 hours
**File**: `internal/service/service.go` + new files

**Steps**:
1. Create `internal/repository/repository.go`
2. Create `internal/filter/filter.go`
3. Create `internal/stats/stats.go`
4. Update service to use these components
5. Add comprehensive tests

### HIGH-5: Fix Filtering Algorithm
**Status**: 🔴 Not Started
**Effort**: 3 hours
**File**: `internal/service/service.go`

**Steps**:
1. Replace nested loops with map-based lookup
2. Add benchmarks
3. Update tests
4. Document O(n) vs O(n*m) improvement

### HIGH-6: Fix RWMutex Usage in Factory
**Status**: 🔴 Not Started
**Effort**: 2 hours
**File**: `internal/notifier/notifier.go`

**Steps**:
1. Copy keys under lock
2. Process outside lock
3. Add tests for concurrent access

### HIGH-7: Fix Goroutine Lifecycle
**Status**: 🔴 Not Started
**Effort**: 4 hours
**File**: `internal/service/service.go`

**Steps**:
1. Add workerDoneChan
2. Implement graceful shutdown
3. Add timeout for worker stoppage
4. Add stress tests

---

## Phase 3: Medium Priority (Sprint 2-3)

### MEDIUM-1: Add Custom Error Types
**Status**: 🔴 Not Started
**Effort**: 4 hours
**File**: New `internal/errors/errors.go`

**Implementation**:
```go
// errors.go
var (
    ErrNotFound          = errors.New("notification not found")
    ErrQueueClosed       = errors.New("queue is closed")
    ErrNotifierNotFound  = errors.New("notifier not found")
    ErrRateLimited       = errors.New("rate limit exceeded")
    ErrInvalidConfig     = errors.New("invalid configuration")
)

// Use in code:
if err := doSomething(); err != nil {
    if errors.Is(err, ErrQueueClosed) {
        // Handle specific error
    }
}
```

### MEDIUM-2: Add Structured Logging
**Status**: 🔴 Not Started
**Effort**: 8-10 hours
**File**: `internal/logging/logger.go`

**Implementation**:
- Migrate from custom logger to `log/slog` (Go 1.21+)
- Support JSON output
- Add structured fields
- Update all log calls

### MEDIUM-3: Extract Logger Interface
**Status**: 🔴 Not Started
**Effort**: 3 hours

**Implementation**:
```go
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

### MEDIUM-4: Add Input Validation
**Status**: 🔴 Not Started
**Effort**: 6 hours

**Add validation for**:
- Email addresses (use `net/mail`)
- URLs (use `url.Parse()`)
- Domain checks
- Recipient limits
- Message size limits

### MEDIUM-5: Add Configuration Validation
**Status**: 🔴 Not Started
**Effort**: 4 hours
**File**: `internal/config/config.go`

**Validate**:
- Worker count > 0
- Queue size > 0
- Timeouts reasonable
- Port ranges valid

---

## Phase 4: Low Priority (Ongoing)

### LOW-1: Add Package Documentation
**Status**: 🔴 Not Started
**Effort**: 8 hours
**Action**: Create `doc.go` in each package

### LOW-2: Consistent Naming
**Status**: 🔴 Not Started
**Effort**: 2 hours
**Action**:
- Use `svc` for service receivers
- Use `n` for notifier receivers
- Use `h` for handler receivers

### LOW-3: Remove Unused Code
**Status**: 🔴 Not Started
**Effort**: 1 hour
**Action**:
- Remove unused config fields
- Remove TODO comments
- Remove stub implementations

---

## Testing Strategy

### Unit Tests to Add
```
- [x] Notification TTL/cleanup
- [x] CORS origin validation
- [x] Request size limits
- [x] Sharded lock functionality
- [x] Filter algorithm performance
- [x] Error type handling
- [x] Configuration validation
- [x] Custom error type usage
```

### Integration Tests to Add
```
- [x] End-to-end with authentication
- [x] Concurrent notifications
- [x] Graceful shutdown
- [x] TLS certificate validation
- [x] Rate limiting boundaries
- [x] Queue overflow handling
```

### Benchmarks to Add
```
- [x] Filtering performance (nested vs map)
- [x] Lock contention (single vs sharded)
- [x] Memory usage over time
- [x] Concurrent operations
```

---

## Deployment Checklist

Before deploying each phase:

- [ ] All tests pass
- [ ] No race condition warnings
- [ ] Code reviewed
- [ ] Documentation updated
- [ ] Backward compatibility verified
- [ ] Performance benchmarks acceptable
- [ ] Security review completed
- [ ] Monitoring alerts configured

---

## Timeline

| Phase | Effort | Timeline | Status |
|-------|--------|----------|--------|
| Phase 1 (Critical) | 20 hours | Week 1 | 🔴 Planned |
| Phase 2 (High) | 40 hours | Week 2-3 | 🔴 Planned |
| Phase 3 (Medium) | 40 hours | Sprint 2-3 | 🔴 Planned |
| Phase 4 (Low) | 20 hours | Ongoing | 🔴 Planned |
| **Total** | **120 hours** | **4-6 weeks** | |

---

## Success Metrics

After remediation:

- [ ] Zero memory leaks in long-running tests
- [ ] 95%+ latency unchanged under load
- [ ] 16x improvement in lock contention
- [ ] 100% TLS validation in place
- [ ] All critical security issues resolved
- [ ] 90%+ test coverage
- [ ] Structured logging enabled
- [ ] Custom error types in use
- [ ] Configuration validation at startup
- [ ] Documentation complete

---

## Notes for Implementation

1. **Backward Compatibility**: Each phase should maintain backward compatibility
2. **Gradual Rollout**: Test thoroughly before deploying to production
3. **Monitoring**: Add metrics to detect improvements
4. **Documentation**: Update docs with each change
5. **Code Review**: Require review for security-critical changes
6. **Testing**: Add tests before implementation where possible

---

## Questions & Clarifications

1. **Configuration retention TTL**: Should this be per-notification or global?
   - Recommendation: Global with override per environment

2. **CORS origins**: Should these be environment-specific?
   - Recommendation: Yes, different for dev/staging/prod

3. **Custom error types**: Should these be exported?
   - Recommendation: Yes, for consumers to use `errors.Is()`

4. **Logging migration**: Breaking change or gradual?
   - Recommendation: Gradual, add structured logging alongside current

5. **Sharded locks**: How many shards optimal?
   - Recommendation: Start with 16, benchmark for your use case
