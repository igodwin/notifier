# Issue-Specific Implementation Prompts

This document contains focused prompts for each discovered issue. Use these when implementing fixes to ensure clear, targeted work.

---

## 🔴 CRITICAL ISSUES

### CRITICAL-1: Unbounded Memory Growth in Notification Storage

**Prompt:**
```
Implement a notification retention policy with automatic cleanup to prevent
unbounded memory growth. The system should:

1. Add a NotificationRetentionConfig with:
   - enabled (bool): Toggle retention on/off
   - ttl (duration): How long to keep notifications (default: 7 days)
   - check_frequency (duration): How often to check for expired (default: 1 hour)
   - max_size (int): Maximum notifications in memory (default: 100,000)

2. Create a cleanupLoop() goroutine in NotificationService that:
   - Runs at check_frequency intervals
   - Removes notifications older than TTL
   - Removes oldest notifications when max_size is exceeded
   - Logs cleanup statistics

3. Integrate with service lifecycle:
   - Start cleanup on Start()
   - Stop cleanup gracefully on Stop()
   - Ensure cleanup completes before shutdown

4. Add configuration to config.yaml with sensible defaults

5. Add tests verifying:
   - Notifications older than TTL are removed
   - Memory usage stays bounded
   - Cleanup frequency is respected
   - Concurrent access is safe

Acceptance criteria:
- Memory grows predictably and doesn't exceed max_size
- Old notifications are automatically cleaned up
- Cleanup is configurable per environment
- No performance regression during cleanup
- Graceful shutdown waits for cleanup to complete
```

**Location:** `internal/service/service.go`
**Effort:** 4-6 hours
**Risk:** Medium (touches core service logic)

---

### CRITICAL-2: Remove TLS Verification Bypass in ntfy Notifier

**Prompt:**
```
Remove the InsecureSkipVerify security vulnerability and implement proper
TLS certificate handling. The system should:

1. Remove InsecureSkipVerify from NtfyConfig struct entirely

2. Add CACertPath field to NtfyConfig:
   - Optional path to custom CA certificate
   - Only used if self-signed certificates are needed
   - Validated at config load time

3. Implement createNtfyHTTPClient() that:
   - Uses system default CA certificates by default
   - Loads custom CA cert if CACertPath is provided
   - Returns error if CA cert file is invalid/missing
   - Configures TLS with proper settings

4. Add validation to NtfyConfig:
   - Check that ca_cert_path exists and is readable
   - Validate it's a valid PEM certificate
   - Provide clear error messages for misconfiguration

5. Update documentation to:
   - Explain why InsecureSkipVerify was removed
   - Show how to use system default CA certs
   - Show how to provide custom CA certificate
   - Provide warnings about self-signed certificates

6. Add tests verifying:
   - System default CA pool is used by default
   - Custom CA cert is loaded correctly
   - Invalid CA cert paths are rejected
   - TLS validation always occurs

Acceptance criteria:
- InsecureSkipVerify option completely removed
- TLS verification always enforced
- Custom CA support works for self-signed certs
- Error messages clearly explain TLS issues
- No ability to bypass certificate validation
```

**Location:** `internal/notifier/ntfy.go`
**Effort:** 2-3 hours
**Risk:** Low (configuration-only change)

---

### CRITICAL-3: Fix CORS Configuration - Replace Wildcard with Whitelist

**Prompt:**
```
Replace wildcard CORS configuration with explicit origin whitelist to prevent
CSRF attacks. The system should:

1. Create CORSConfig struct in api/rest/router.go containing:
   - AllowedOrigins: []string
   - AllowedMethods: []string (default: GET, POST, OPTIONS, DELETE)
   - AllowedHeaders: []string (default: Content-Type, Authorization)
   - AllowCredentials: bool
   - MaxAge: int (cache duration in seconds)

2. Implement corsMiddleware() that:
   - Checks incoming Origin header against whitelist
   - Only sets Access-Control-Allow-Origin if in whitelist
   - Never uses wildcard (*)
   - Sets other CORS headers appropriately
   - Handles preflight OPTIONS requests

3. Load CORS config from config.yaml:
   - Make all CORS settings configurable
   - Provide sensible defaults
   - Support environment-specific overrides
   - Validate at startup

4. Modify NewRouterWithAuth() to:
   - Accept CORSConfig parameter
   - Apply CORS middleware to all routes
   - Ensure auth middleware runs after CORS

5. Update configuration examples:
   - Show dev environment config (localhost:3000)
   - Show production config (specific domains)
   - Explain each setting

6. Add tests verifying:
   - Allowed origins are accepted
   - Non-whitelisted origins are rejected
   - Credentials header handling works
   - Preflight requests return 200 OK
   - Wildcard is never returned

Acceptance criteria:
- CORS whitelist fully configurable
- Wildcard configuration is impossible
- Security headers properly set
- Environment-specific configs work
- No CSRF vulnerability
```

**Location:** `api/rest/router.go`
**Effort:** 2-3 hours
**Risk:** Low (configuration-only change)

---

## 🟠 HIGH PRIORITY ISSUES

### HIGH-1: Add Request Size Limits to Prevent DoS

**Prompt:**
```
Implement request size limits on all HTTP endpoints to prevent out-of-memory
attacks. The system should:

1. Define constants for size limits:
   - MaxRequestSize: 10MB (configurable per endpoint)
   - MaxBatchSize: 1000 notifications per batch
   - MaxRecipients: 100 recipients per notification

2. Update handlers to enforce limits:
   - SendNotification: Wrap request body with MaxBytesReader
   - SendBatchNotifications: Validate batch size
   - ListNotifications: Validate filter parameters

3. Implement size limit checks:
   - Check request body size before JSON decode
   - Check batch notification count
   - Check recipient list count
   - Validate message/body size

4. Return appropriate errors:
   - 413 Payload Too Large for oversized requests
   - 400 Bad Request for invalid counts
   - Clear error messages explaining the limit

5. Make limits configurable:
   - Add to config.yaml
   - Support environment variable overrides
   - Log when limits are enforced

6. Add tests verifying:
   - Requests under limit are accepted
   - Requests over limit are rejected
   - Error messages are clear
   - Different limits work for different endpoints
   - Large valid requests still work

Acceptance criteria:
- All request sizes are validated
- Clear error messages on rejection
- Limits are configurable
- No legitimate requests are rejected
- DoS protection is effective
```

**Location:** `api/rest/handlers.go`
**Effort:** 2 hours
**Risk:** Low

---

### HIGH-2: Implement Sharded Locking for Concurrent Access

**Prompt:**
```
Replace single mutex lock with sharded locking to reduce lock contention
and improve concurrent throughput. The system should:

1. Design sharded storage:
   - Create 16 shards (constant: ShardCount = 16)
   - Each shard has its own RWMutex
   - Use FNV-1a hash for shard selection

2. Implement shard indexing:
   - Create getShardIdx(id string) func
   - Hash notification ID to shard index
   - Return value 0-15

3. Update NotificationService struct:
   - Replace single notifications map + mu with sharded array
   - Each shard contains: mu sync.RWMutex, notifications map[string]*Notification
   - Keep mu (for overall state changes) separate if needed

4. Refactor all notification operations:
   - storeNotification(): Get shard, lock, store
   - GetNotification(): Get shard, read lock, retrieve
   - DeleteNotification(): Get shard, write lock, delete
   - ListNotifications(): Iterate all shards safely
   - GetStats(): Aggregate from all shards

5. Implement safe aggregation:
   - ListNotifications must iterate all shards
   - Hold each shard lock briefly
   - Release before processing
   - Apply filters after release

6. Add benchmarks:
   - Single-threaded access
   - Multi-threaded with 10/50/100 goroutines
   - Compare to original mutex approach
   - Measure lock contention reduction

7. Add tests verifying:
   - Concurrent reads don't block
   - Concurrent writes don't deadlock
   - No data races (go test -race)
   - All shards stay consistent
   - Performance improves with concurrency

Acceptance criteria:
- 16x throughput improvement under high concurrency
- No race conditions
- All operations remain correct
- Memory usage slightly increases (acceptable)
- Lock contention measured and documented
```

**Location:** `internal/service/service.go`
**Effort:** 6-8 hours
**Risk:** High (touches core logic, needs thorough testing)

---

### HIGH-3: Fix Lock Ordering - Release Locks Before Channel Operations

**Prompt:**
```
Fix lock ordering issues where locks are held during channel operations
that can block indefinitely. The system should:

1. Analyze LocalQueue implementation:
   - Identify all places where mu is held
   - Identify all channel operations (send, receive)
   - Flag places where both occur together

2. Refactor Enqueue() function:
   - Check closed status while holding lock
   - Copy queue reference (not channel, just get the variable)
   - Release lock before sending to channel
   - Use select with timeout for safety
   - Re-check closed after timeout

3. Refactor Dequeue() function:
   - Implement similar pattern
   - Hold lock only for critical section
   - Release before channel operations
   - Return copy of message, not reference

4. Implement safe channel operations:
   - Create helper functions for thread-safe operations
   - Use select with timeout to prevent indefinite blocks
   - Return appropriate errors on timeout
   - Document timeout behavior

5. Add tests verifying:
   - No deadlocks during concurrent enqueue/dequeue
   - Timeouts are respected
   - Channels don't block with locks held
   - Queue stays consistent under stress
   - go test -race passes

6. Document lock pattern:
   - Add comments explaining lock scope
   - Show correct pattern for channel operations with locks
   - Explain why locks are released before channel ops

Acceptance criteria:
- No locks held during channel operations
- No potential deadlocks
- Clear timeout handling
- All race detector warnings fixed
- Performance is consistent
```

**Location:** `internal/queue/local.go`
**Effort:** 4 hours
**Risk:** High (affects concurrency safety)

---

### HIGH-4: Separate Service Layer Concerns

**Prompt:**
```
Extract mixed responsibilities from NotificationService into separate,
focused components. The system should:

1. Create NotificationRepository interface:
   ```go
   type NotificationRepository interface {
       Store(notification *Notification) error
       Get(id string) (*Notification, error)
       Delete(id string) error
       List(filter *NotificationFilter) ([]*Notification, error)
       GetStats() *NotificationStats
   }
   ```

2. Create NotificationFilter service:
   ```go
   type FilterService interface {
       Apply(notifications []*Notification) []*Notification
   }
   ```

3. Create StatsCollector:
   ```go
   type StatsCollector interface {
       Record(notification *Notification, result *NotificationResult)
       GetStats() *NotificationStats
   }
   ```

4. Implement InMemoryRepository:
   - Handle all storage operations
   - Manage TTL cleanup (from CRITICAL-1)
   - Handle concurrent access (with sharding from HIGH-2)
   - Return appropriate errors

5. Implement FilterService:
   - Move all filter logic from service
   - Handle recipient matching efficiently
   - Support all filter types
   - Return filtered notifications

6. Refactor NotificationService:
   - Remove storage, filtering, stats logic
   - Inject repository, filter, stats dependencies
   - Orchestrate components
   - Handle queue and worker management

7. Update service methods:
   - Send(): Use repository to store
   - GetNotification(): Use repository
   - ListNotifications(): Use filter service
   - GetStats(): Use stats collector

8. Add tests:
   - Test each component independently
   - Test service orchestration
   - Mock repository/filter/stats
   - Verify integration

Acceptance criteria:
- Service has single responsibility (orchestration)
- Repository handles all storage
- Filter service handles all filtering
- Stats collected separately
- Each component is independently testable
- No circular dependencies
```

**Location:** `internal/service/service.go` + new files
**Effort:** 16-20 hours
**Risk:** High (major refactoring, needs comprehensive testing)

---

### HIGH-5: Optimize Filtering Algorithm from O(n*m) to O(n)

**Prompt:**
```
Replace nested-loop filtering with hash-based O(n) algorithm for better
performance with large recipient lists. The system should:

1. Identify all filtering operations:
   - Recipient matching (currently O(n*m))
   - Type matching (verify efficiency)
   - Status matching (verify efficiency)
   - Date range matching (verify efficiency)

2. Implement helper functions:
   - convertFilterToMaps(): Create maps for O(1) lookup
   - matchesTypeFilter(): Use map lookup
   - matchesStatusFilter(): Use map lookup
   - matchesRecipientFilter(): Use map lookup

3. Implement efficient recipient matching:
   ```go
   func matchesRecipientFilter(notification, filter) bool {
       if len(filter.Recipients) == 0 {
           return true  // No filter = matches all
       }
       filterSet := make(map[string]bool)
       for _, r := range filter.Recipients {
           filterSet[r] = true
       }
       for _, nr := range notification.Recipients {
           if filterSet[nr] {
               return true  // Found match
           }
       }
       return false
   }
   ```

4. Update ListNotifications() to use efficient filters:
   - Build filter maps once
   - Iterate notifications once
   - Apply all filters in single pass

5. Add benchmarks:
   - Old algorithm: 10 notifications, 50 recipients each
   - New algorithm: same data
   - Larger datasets: 1000 notifications
   - Measure improvement factor

6. Add tests verifying:
   - Empty filters match all
   - Specific filters match correctly
   - Boundary conditions work
   - Performance improves
   - No filtering behavior changed

Acceptance criteria:
- O(n*m) replaced with O(n)
- 10-100x faster with typical data
- Filtering logic still correct
- Benchmarks show improvement
- No behavior changes
```

**Location:** `internal/service/service.go`
**Effort:** 3 hours
**Risk:** Low (isolated change, easy to test)

---

### HIGH-6: Fix RWMutex Lock Held During Iteration

**Prompt:**
```
Fix factory SupportedTypes() function that holds RWMutex during iteration
and unnecessary operations. The system should:

1. Analyze current implementation:
   - Lock is held while building typeMap
   - Lock is held while building output slice
   - All operations are read-only after lock acquisition

2. Implement optimized pattern:
   - Hold read lock only while copying notifier keys
   - Release lock before processing
   - Build slice/map outside lock
   - No performance impact

3. Update SupportedTypes():
   ```go
   // Copy keys while holding lock
   f.mu.RLock()
   keys := make([]string, 0, len(f.notifiers))
   for k := range f.notifiers {
       keys = append(keys, k)
   }
   f.mu.RUnlock()

   // Process outside lock
   typeMap := make(map[NotificationType]bool)
   for _, key := range keys {
       // Extract type from key...
   }
   ```

4. Apply same pattern to other methods:
   - Create(): Copy reference, release lock, use
   - GetAccounts(): Copy data, release, process
   - Any other lock-heavy operations

5. Add tests:
   - Concurrent reads don't block each other
   - go test -race shows no races
   - Behavior unchanged
   - Performance improves

Acceptance criteria:
- Lock held for minimal time
- No blocking during iteration
- No race conditions
- Performance improves slightly
```

**Location:** `internal/notifier/notifier.go`
**Effort:** 2 hours
**Risk:** Low

---

### HIGH-7: Fix Goroutine Lifecycle and Graceful Shutdown

**Prompt:**
```
Implement proper goroutine lifecycle management with graceful shutdown to
prevent goroutine leaks. The system should:

1. Add lifecycle tracking to NotificationService:
   - workerDoneChan: chan struct{} for worker completion
   - Track worker count for verification
   - Ensure all workers exit before returning

2. Implement proper worker startup:
   - Spawn N workers with recovery
   - Each worker defers recovery logging
   - Track in workerDoneChan when exits
   - Log worker startup/shutdown

3. Update worker loop:
   ```go
   func (s *NotificationService) worker(id int, ctx context.Context) {
       defer func() {
           s.logger.Infof("Worker %d exiting", id)
           s.workerDoneChan <- struct{}{}
           if r := recover(); r != nil {
               s.logger.Errorf("Worker panic: %v", r)
           }
       }()

       for {
           select {
           case <-ctx.Done():
               return
           case <-s.stopChan:
               return
           case msg := <-s.queue.Dequeue():
               s.processNotification(ctx, msg)
           }
       }
   }
   ```

4. Implement graceful Stop():
   - Close stopChan to signal all workers
   - Wait for all workers via workerDoneChan
   - Use timeout to prevent infinite wait
   - Log any workers that don't stop
   - Return appropriate error

5. Add lifecycle guarantees:
   - All workers started during Start()
   - All workers stopped during Stop()
   - Timeout prevents hanging shutdown
   - No goroutine leaks on restart

6. Add tests:
   - Workers start correctly
   - Workers stop on Stop() call
   - Goroutine count matches expectations
   - Panic in worker doesn't crash service
   - Graceful shutdown completes
   - Force stop after timeout works

Acceptance criteria:
- No goroutine leaks
- Graceful shutdown completes
- Forced stop after timeout
- Worker crashes logged but don't crash service
- All goroutines accounted for
```

**Location:** `internal/service/service.go`
**Effort:** 4 hours
**Risk:** High (affects reliability)

---

## 🟡 MEDIUM PRIORITY ISSUES

### MEDIUM-1: Add Custom Error Types for Better Error Handling

**Prompt:**
```
Create custom error types to enable proper error handling with errors.Is()
and errors.As(). The system should:

1. Create internal/errors/errors.go with:
   ```go
   var (
       ErrNotFound          = errors.New("notification not found")
       ErrQueueClosed       = errors.New("queue is closed")
       ErrNotifierNotFound  = errors.New("notifier not found")
       ErrRateLimited       = errors.New("rate limit exceeded")
       ErrInvalidConfig     = errors.New("invalid configuration")
   )
   ```

2. Create error interfaces for specific cases:
   ```go
   type NotFoundError interface {
       error
       NotificationID() string
   }

   type ValidationError interface {
       error
       Field() string
   }
   ```

3. Implement error types:
   ```go
   type notificationNotFoundError struct {
       id string
   }

   func (e *notificationNotFoundError) Error() string {
       return fmt.Sprintf("notification %q not found", e.id)
   }

   func (e *notificationNotFoundError) NotificationID() string {
       return e.id
   }

   func NewNotFoundError(id string) NotFoundError {
       return &notificationNotFoundError{id}
   }
   ```

4. Update all error returns:
   - Replace fmt.Errorf(...) with custom errors where applicable
   - Use NewNotFoundError(id) where appropriate
   - Use fmt.Errorf for dynamic messages

5. Enable error handling in consumers:
   - Use errors.Is(err, ErrNotFound)
   - Use errors.As(err, &notFoundErr)
   - Type-switch on error interfaces

6. Add tests:
   - errors.Is() works correctly
   - errors.As() works correctly
   - Error messages are informative
   - Stack traces preserved

Acceptance criteria:
- All errors use custom types or error interfaces
- errors.Is() and errors.As() work throughout
- Clear error messages
- Type information preserved
- Backward compatible (error messages same)
```

**Location:** New file: `internal/errors/errors.go`
**Effort:** 4 hours
**Risk:** Medium (affects error handling)

---

### MEDIUM-2: Migrate to Structured Logging

**Prompt:**
```
Replace custom logger with structured logging using Go 1.21+ slog or zap
for better log parsing and analysis. The system should:

1. Choose logging library:
   - Option A: Use log/slog (Go 1.21+, built-in)
   - Option B: Use zap (more features, external dep)
   - Recommendation: slog for built-in, zap for advanced

2. Create logger interface:
   ```go
   type Logger interface {
       Debug(msg string, keysAndValues ...interface{})
       Info(msg string, keysAndValues ...interface{})
       Warn(msg string, keysAndValues ...interface{})
       Error(msg string, keysAndValues ...interface{})
       With(keysAndValues ...interface{}) Logger
   }
   ```

3. Implement structured logging:
   - Replace Infof/Errorf with Info/Error + fields
   - Use With() for context fields
   - Add request IDs, user IDs, etc. as fields
   - Structure data for JSON parsing

4. Update all log calls:
   - Change from: logger.Infof("User %s logged in", name)
   - Change to: logger.Info("User logged in", "user", name)
   - Add context where relevant
   - Remove PII from debug logs

5. Configure output:
   - Support JSON output (for ELK/Datadog)
   - Support text output (for development)
   - Configurable log level
   - Support different outputs per level

6. Add tests:
   - Log output contains expected fields
   - JSON is valid
   - Different log levels work
   - Sensitive data is excluded
   - Performance impact measured

Acceptance criteria:
- All logs are structured
- JSON output is valid
- Log parsing is 100x faster
- Sensitive data not logged
- Development logs still readable
- Backward compatible output available
```

**Location:** `internal/logging/logger.go`
**Effort:** 8-10 hours
**Risk:** Medium (many changes, but mostly mechanical)

---

### MEDIUM-3: Extract Logger as Interface for Testability

**Prompt:**
```
Extract Logger into an interface to enable mocking in tests and reduce
coupling to concrete logger implementation. The system should:

1. Create logger interface:
   ```go
   type Logger interface {
       Debug(msg string, keysAndValues ...interface{})
       Debugf(format string, args ...interface{})
       Info(msg string, keysAndValues ...interface{})
       Infof(format string, args ...interface{})
       Warn(msg string, keysAndValues ...interface{})
       Warnf(format string, args ...interface{})
       Error(msg string, keysAndValues ...interface{})
       Errorf(format string, args ...interface{})
   }
   ```

2. Implement in concrete logger:
   - Current Logger type implements interface
   - No changes to implementation
   - Just expose interface publicly

3. Update all signatures:
   - Functions accept Logger interface
   - Not *Logger concrete type
   - Handlers, services, all components

4. Create mock logger for tests:
   ```go
   type MockLogger struct {
       logs []LogEntry
   }

   func (m *MockLogger) Info(msg string, kv ...interface{}) {
       m.logs = append(m.logs, LogEntry{msg, kv})
   }

   func (m *MockLogger) AssertLogged(msg string) error {
       // Check if msg was logged
   }
   ```

5. Update tests:
   - Use MockLogger instead of real logger
   - Verify log calls in tests
   - No external log files in tests
   - Faster test execution

6. Add tests:
   - Mock logger works
   - Services accept Logger interface
   - Handlers accept Logger interface
   - All components properly decoupled

Acceptance criteria:
- Logger is interface, not concrete type
- Mock logger works for testing
- All code uses interface
- Tests don't depend on real logger
- No behavioral changes
```

**Location:** `internal/logging/logger.go` + all files using Logger
**Effort:** 3 hours
**Risk:** Low (interfaces are non-invasive)

---

### MEDIUM-4: Add Input Validation for URLs and Email Addresses

**Prompt:**
```
Implement comprehensive input validation for email addresses, URLs, and
domains to prevent invalid configuration and injection attacks. The system should:

1. Create validation package:
   ```go
   // internal/validation/validation.go
   func ValidateEmail(email string) error
   func ValidateURL(urlStr string) error
   func ValidateDomain(domain string) error
   func ValidateRecipients(recipients []string) error
   ```

2. Implement email validation:
   - Use net/mail.ParseAddress()
   - Not just check for @
   - Validate format
   - Return clear errors

3. Implement URL validation:
   - Use url.Parse()
   - Check scheme (https, http only)
   - Check domain for specific notifiers
   - Prevent localhost in production

4. Implement domain validation:
   - Check against whitelist if needed
   - Validate format
   - Prevent invalid characters

5. Update notifier configurations:
   - Validate SMTP host/port
   - Validate Slack webhook URL
   - Validate ntfy server URL
   - At startup, not at request time

6. Add to request validation:
   - Validate all recipients
   - Validate URLs in metadata
   - Validate before queuing

7. Add tests:
   - Valid inputs accepted
   - Invalid inputs rejected
   - Clear error messages
   - Edge cases handled
   - Performance acceptable

Acceptance criteria:
- All email addresses validated
- All URLs validated
- Invalid configurations caught at startup
- Invalid requests rejected early
- Clear error messages
- No bypasses possible
```

**Location:** New file: `internal/validation/validation.go`
**Effort:** 6 hours
**Risk:** Low (additive, non-breaking)

---

### MEDIUM-5: Add Configuration Validation at Startup

**Prompt:**
```
Implement comprehensive configuration validation at startup to catch invalid
settings before runtime. The system should:

1. Enhance Config.Validate():
   - Validate all numeric ranges
   - Validate all required fields
   - Validate all file paths
   - Validate all URLs

2. Add specific validators:
   ```go
   func (c *Config) validateServer() error
   func (c *Config) validateQueue() error
   func (c *Config) validateNotifiers() error
   func (c *Config) validateAuth() error
   func (c *Config) validateLogging() error
   ```

3. Validate numeric ranges:
   - Worker count: 1-1000
   - Queue buffer: 1-1000000
   - Timeouts: 1s-5m
   - Ports: 1-65535
   - Rate limits: 0-10000

4. Validate required fields:
   - Notifier credentials
   - Server host/port
   - File paths exist

5. Validate consistency:
   - Different ports for different servers
   - Sensible ratios (workers vs buffer)
   - Compatible timeouts

6. Provide helpful errors:
   - Say what value is invalid
   - Say what range is valid
   - Suggest fixes where possible

7. Add tests:
   - Valid config passes
   - Invalid values rejected
   - Error messages are helpful
   - All validators work
   - Performance acceptable

Acceptance criteria:
- All invalid configs caught at startup
- Clear error messages
- No runtime panics for config issues
- Configuration documented with ranges
- Examples for valid config
```

**Location:** `internal/config/config.go`
**Effort:** 4 hours
**Risk:** Low

---

### MEDIUM-6: Close File Handles Properly

**Prompt:**
```
Implement proper file handle lifecycle management to prevent resource leaks
when logging to files. The system should:

1. Update logger initialization:
   - Return interface with Close() method
   - Track opened file handle
   - Ensure cleanup on process exit

2. Implement Closer interface:
   ```go
   type Logger interface {
       // ... existing methods ...
       Close() error
   }
   ```

3. Update NewFromConfig():
   - Open file for logging
   - Return logger with file handle
   - Return error if can't open

4. Implement Close():
   - Close file handle
   - Flush pending logs
   - Return any errors
   - Safe to call multiple times

5. Update main.go:
   - Call logger.Close() on exit
   - Use defer to ensure cleanup
   - Capture any close errors
   - Log them before exit

6. Handle cleanup on exit:
   - Defer close in main
   - Close before other cleanup
   - Handle panic in Close()

7. Add tests:
   - File is created
   - Logs are written
   - File is closed
   - Can reopen after close
   - Multiple close calls safe

Acceptance criteria:
- File handles properly closed
- No resource leaks
- Graceful degradation if close fails
- Test coverage for close
- No data loss during close
```

**Location:** `internal/logging/logger.go`, `cmd/server/main.go`
**Effort:** 2 hours
**Risk:** Low

---

### MEDIUM-7: Fix SMTP Email Validation

**Prompt:**
```
Replace simple string checking with proper email validation and improve
SMTP configuration validation. The system should:

1. Use net/mail for validation:
   ```go
   import "net/mail"

   func validateEmail(email string) error {
       _, err := mail.ParseAddress(email)
       return err
   }
   ```

2. Update SendNotification() to validate:
   - Check all recipients are valid emails
   - Check From address is valid
   - Check CC/BCC addresses are valid
   - Return error before queuing

3. Add SMTP config validation:
   - Validate host not empty
   - Validate port in 1-65535
   - Validate from address format
   - Check credentials if needed

4. Update error messages:
   - Say which email is invalid
   - Suggest correct format
   - Don't leak system information

5. Add tests:
   - Valid emails accepted
   - Invalid emails rejected
   - Edge cases: quoted strings, special chars
   - Clear error messages
   - Performance acceptable

Acceptance criteria:
- All email addresses validated
- Invalid emails caught early
- Proper format checking
- Clear error messages
- RFC 5322 compliant
```

**Location:** `internal/notifier/smtp.go`
**Effort:** 2 hours
**Risk:** Low

---

### MEDIUM-8: Increase SMTP Boundary Random Size

**Prompt:**
```
Increase MIME boundary random size from 16 to 32 bytes to reduce collision
risk in large email messages. The system should:

1. Update boundary generation:
   ```go
   // From: 16 bytes
   buf := make([]byte, 16)

   // To: 32 bytes
   buf := make([]byte, 32)
   ```

2. Verify boundary format:
   - Still uses hex encoding
   - Still has "boundary_" prefix
   - Just longer random part
   - Still valid in MIME spec

3. Add tests:
   - Boundary is unique
   - Boundary is valid MIME
   - Large emails work
   - Boundary doesn't appear in body
   - Multiple messages don't collide

Acceptance criteria:
- Boundary is 64 hex chars (32 bytes)
- No collision risk
- All emails valid
- Performance unchanged
```

**Location:** `internal/notifier/smtp.go`
**Effort:** 1 hour
**Risk:** Very Low

---

### MEDIUM-9: Implement No Unused Configuration Fields Cleanup

**Prompt:**
```
Remove or implement unused configuration fields to reduce confusion and
maintain consistency. The system should:

1. Identify unused fields:
   - MetricsConfig (defined but unused)
   - HealthCheckConfig (defined but unused)
   - Any others not referenced in code

2. Options for each:
   - Remove entirely (if not needed)
   - Implement fully (if needed)
   - Document why present

3. For fields to keep:
   - Implement the feature
   - Update configuration documentation
   - Add to code that uses it

4. For fields to remove:
   - Remove from Config struct
   - Remove from config.yaml examples
   - Remove from defaults
   - Update documentation

5. For future features:
   - Create feature branch
   - Don't add config until implemented
   - Keep configs minimal

6. Document decision:
   - Add comments explaining which are used
   - Link to implementation
   - Explain why removed

Acceptance criteria:
- No unused configuration fields
- All fields documented
- Clear what's implemented
- Clear what's not
- Examples accurate
```

**Location:** `internal/config/config.go`
**Effort:** 1 hour
**Risk:** Low

---

### MEDIUM-10: Implement Duplicate Key Generation Logic Consolidation

**Prompt:**
```
Consolidate duplicate key generation logic between notifier factory and
authorization service into shared utility. The system should:

1. Create common utility:
   ```go
   // internal/common/keys/keys.go
   func MakeKey(keyType string, name string) string {
       if name == "" {
           return keyType
       }
       return fmt.Sprintf("%s:%s", keyType, name)
   }
   ```

2. Update notifier factory:
   - Use MakeKey instead of duplicate logic
   - Update documentation

3. Update authorization service:
   - Use MakeKey instead of duplicate logic
   - Update documentation

4. Update tests:
   - Test MakeKey directly
   - Verify both components use it

5. Document pattern:
   - Explain key format
   - Explain when to use
   - Link to implementation

Acceptance criteria:
- No duplicate code
- Single source of truth
- Both components use shared function
- Tests verify consistency
```

**Location:** New file: `internal/common/keys/keys.go`
**Effort:** 1 hour
**Risk:** Very Low

---

## 🟢 LOW PRIORITY ISSUES

### LOW-1: Add Package-Level Documentation

**Prompt:**
```
Add package-level documentation with doc.go files to every package explaining
purpose and key types. The system should:

1. Create doc.go for each package:
   ```go
   // Package auth provides authentication and authorization.
   //
   // API Keys
   //
   // The package manages API keys for authentication...
   //
   // Authorization
   //
   // Authorization is role-based...
   package auth
   ```

2. Document all packages:
   - internal/auth
   - internal/service
   - internal/notifier
   - internal/queue
   - internal/logging
   - internal/config
   - api/rest
   - api/grpc

3. Include in doc.go:
   - Package purpose
   - Main types
   - Key functions
   - Examples where helpful
   - Related packages

4. Add function comments:
   - Every exported function documented
   - Start with function name
   - Explain purpose
   - Mention error cases

5. Add type comments:
   - Every exported type documented
   - Explain when to use
   - Mention related types

Acceptance criteria:
- All packages documented
- All exported items documented
- Examples provided
- godoc builds without warnings
```

**Location:** Each package with `doc.go`
**Effort:** 8 hours
**Risk:** Very Low

---

### LOW-2: Standardize Receiver Names

**Prompt:**
```
Standardize receiver variable names across codebase for consistency and
better readability. The system should:

1. Define standard receiver names:
   - Service receivers: svc
   - Handler receivers: h
   - Notifier receivers: n
   - Queue receivers: q
   - Logger receivers: l or logger

2. Update all receivers:
   - Service methods: change s to svc
   - Handler methods: change h to h (already good)
   - Notifier methods: change s/n to n
   - Find any others inconsistent

3. Make mechanical changes:
   - Use refactor/rename tool
   - Verify all references updated
   - Run tests to ensure correctness

4. Document standard:
   - Add to CONTRIBUTING.md (if exists)
   - Or add comment to code

5. Verify changes:
   - Tests still pass
   - No functional change
   - Code review for style

Acceptance criteria:
- All receivers follow standard
- Consistent throughout codebase
- No functional changes
- Tests pass
```

**Location:** Multiple files
**Effort:** 2 hours
**Risk:** Very Low

---

### LOW-3: Make Timeout Values Configurable

**Prompt:**
```
Extract hardcoded timeout values into configuration to allow per-environment
tuning without code changes. The system should:

1. Identify hardcoded timeouts:
   - HTTP client: 30s
   - Notifier timeouts
   - Database timeouts
   - Context timeouts

2. Create timeout config:
   ```yaml
   timeouts:
     http_client: 30s
     smtp_send: 30s
     slack_send: 30s
     ntfy_send: 30s
   ```

3. Add to Config struct:
   - TimeoutsConfig with all timeouts
   - Sensible defaults
   - Validation (minimum 1s)

4. Update all timeout usages:
   - Use config values
   - Fall back to defaults
   - Log actual value used

5. Document in examples:
   - Show timeout settings
   - Explain impact of each
   - Recommend values

Acceptance criteria:
- All hardcoded timeouts extracted
- Configurable per environment
- Clear defaults
- Documentation provided
```

**Location:** `internal/config/config.go`, notifier files
**Effort:** 3 hours
**Risk:** Low

---

## Summary Table

| Category | Count | Total Effort | Priority |
|----------|-------|--------------|----------|
| Critical | 3 | 8-12 hrs | Fix immediately |
| High | 7 | 30-40 hrs | Fix before release |
| Medium | 10+ | 40 hrs | This quarter |
| Low | 5+ | 20 hrs | Ongoing |
| **TOTAL** | **49** | **~120 hrs** | **Staged** |

---

## Using These Prompts

1. **For Implementation**: Copy the prompt when starting work on an issue
2. **For Code Review**: Use acceptance criteria to verify completion
3. **For Planning**: Group related issues by effort and dependency
4. **For Documentation**: Reference these when explaining changes to team

Each prompt includes:
- Clear objectives
- Implementation details
- Testing requirements
- Acceptance criteria
- Effort and risk estimates

---

## References

- Full audit details: See `AUDIT_REPORT.md`
- Implementation guide: See `REMEDIATION_PLAN.md`
- Architecture context: See `IMPLEMENTATION_SUMMARY.md`
