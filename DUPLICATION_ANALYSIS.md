# Code Duplication and Refactoring Analysis - Notifier Codebase

## Executive Summary
The notifier codebase shows several areas of code duplication and repeated patterns that can be refactored to improve maintainability without increasing complexity. Most duplication issues are low-to-medium complexity to address and would benefit from extraction into utility functions or helper methods.

---

## 1. REPEATED VALIDATION AND INITIALIZATION PATTERNS IN NOTIFIER IMPLEMENTATIONS

### Issue 1.1: Identical Validation Pattern in All Notifier Send Methods
**File Locations:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/smtp.go`: Lines 63-70
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go`: Lines 78-85
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go`: Lines 185-192
- `/Users/igodwin/Workspace/notifier/internal/notifier/stdout.go`: Lines 26-33

**Description:**
Every notifier implementation repeats the same validation pattern at the start of the Send method:
```go
if err := ValidateContext(ctx); err != nil {
    return nil, err
}

if err := s.Validate(notification); err != nil {
    return nil, err
}
```

**Impact:**
- Code duplication (4 instances of identical pattern)
- Maintenance burden if validation requirements change
- Inconsistency risk if one is updated and others aren't

**Refactoring Recommendation:**
Create an exported wrapper function in the `notifier` package that encapsulates this validation pattern. Could be implemented as a helper that all Send implementations call first.

```go
// Example helper function
func ValidateNotification(ctx context.Context, notif *domain.Notification, validator domain.Notifier) error {
    if err := ValidateContext(ctx); err != nil {
        return err
    }
    return validator.Validate(notif)
}
```

**Estimated Effort:** Low (30 minutes)
- Extract into 1 utility function
- Update 4 notifier files
- No behavioral changes needed

---

## 2. REPEATED HTTP REQUEST PATTERNS IN EXTERNAL NOTIFIERS

### Issue 2.1: Identical HTTP Request/Response Handling in Slack and Ntfy Notifiers
**File Locations:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go`: Lines 182-211 (sendToSlack method)
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go`: Lines 290-323 (sendToTopic method)

**Description:**
Both Slack and Ntfy notifiers have near-identical HTTP request handling:
```go
jsonData, err := json.Marshal(msg/req)
if err != nil {
    return fmt.Errorf("failed to marshal: %w", err)
}

httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
if err != nil {
    return fmt.Errorf("failed to create request: %w", err)
}

httpReq.Header.Set("Content-Type", "application/json")
// Add auth header
resp, err := s.httpClient.Do(httpReq)
if err != nil {
    return fmt.Errorf("failed to send: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return fmt.Errorf("API returned status: %d", resp.StatusCode)
}
```

**Impact:**
- 25+ lines of duplicated code across 2 files
- Difficult to maintain and update HTTP handling logic
- Risk of inconsistent error handling between implementations
- Makes future HTTP-related improvements (retries, timeouts, etc.) harder

**Refactoring Recommendation:**
Create a shared HTTP utility function for JSON POST requests with authentication support.

```go
// Helper function in notifier package
func sendJSONPostRequest(ctx context.Context, client *http.Client, 
    url string, payload interface{}, headers map[string]string) error {
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal: %w", err)
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Content-Type", "application/json")
    for k, v := range headers {
        req.Header.Set(k, v)
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return fmt.Errorf("API returned status: %d", resp.StatusCode)
    }
    return nil
}
```

**Estimated Effort:** Low-Medium (45 minutes)
- Create 1 utility function in notifier package
- Refactor 2 send methods to use utility
- Comprehensive testing needed

---

## 3. REPEATED FILTER MATCHING LOGIC

### Issue 3.1: Identical List Membership Check Pattern in matchesFilter
**File Location:**
- `/Users/igodwin/Workspace/notifier/internal/service/service.go`: Lines 481-540

**Description:**
The `matchesFilter` method checks if a notification matches filters using the same pattern repeated 4 times:
```go
if len(filter.IDs) > 0 {
    found := false
    for _, id := range filter.IDs {
        if notification.ID == id {
            found = true
            break
        }
    }
    if !found {
        return false
    }
}
// ... repeated for Types, Statuses, Recipients with nearly identical logic
```

This pattern is repeated for:
- IDs (lines 481-492)
- Types (lines 495-506)
- Statuses (lines 509-520)
- Recipients (with nested loop, lines 523-539)

**Impact:**
- 60+ lines with highly repetitive matching logic
- Hard to maintain and extend if matching logic needs changes
- Error-prone for adding new filter fields
- Readability suffers due to repetitive structure

**Refactoring Recommendation:**
Create a generic "contains" helper function and use it for simpler cases. For recipients, create a dedicated function.

```go
// Helper function
func containsValue[T comparable](haystack []T, needle T) bool {
    for _, h := range haystack {
        if h == needle {
            return true
        }
    }
    return false
}

// In matchesFilter:
if len(filter.IDs) > 0 && !containsValue(filter.IDs, notification.ID) {
    return false
}

if len(filter.Types) > 0 && !containsValue(filter.Types, notification.Type) {
    return false
}

if len(filter.Statuses) > 0 && !containsValue(filter.Statuses, notification.Status) {
    return false
}

// For recipients (more complex):
if len(filter.Recipients) > 0 && !notificationHasRecipient(notification, filter.Recipients) {
    return false
}

func notificationHasRecipient(notif *domain.Notification, recipients []string) bool {
    for _, fr := range recipients {
        for _, nr := range notif.Recipients {
            if fr == nr {
                return true
            }
        }
    }
    return false
}
```

**Estimated Effort:** Low (40 minutes)
- Create 2 helper functions
- Refactor matchesFilter method
- Unit tests for helpers
- Reduces 60+ lines to ~30 lines

---

## 4. REPEATED AUTHENTICATION VALIDATION LOGIC

### Issue 4.1: Duplicated API Key Validation in REST and gRPC Middleware
**File Locations:**
- `/Users/igodwin/Workspace/notifier/internal/auth/rest_middleware.go`: Lines 35-50
- `/Users/igodwin/Workspace/notifier/internal/auth/grpc_middleware.go`: Lines 38-50 (Unary), Lines 82-94 (Stream)

**Description:**
Both REST and gRPC middleware repeat the same validation sequence:
```go
// Validate API key
key, err := m.store.ValidateKey(apiKey)
if err != nil {
    // log error
    // return auth error
}

// Check rate limit
allowed, err := m.store.CheckRateLimit(apiKey)
if err != nil || !allowed {
    // log error
    // return rate limit error
}

// Update last used
if err := m.store.UpdateLastUsed(apiKey); err != nil {
    // log error
}

// Create auth context
authCtx := &AuthContext{...}
```

This pattern appears 3 times (REST middleware once, gRPC unary once, gRPC stream once).

**Impact:**
- 30+ lines of duplicated validation logic across 2 files
- Changes to validation logic must be made in 3 places
- Inconsistency risk between REST and gRPC authentication
- Testing burden increased

**Refactoring Recommendation:**
Extract validation into a reusable helper method in APIKeyStore.

```go
type ValidatedKey struct {
    APIKey   *APIKey
    ClientID string
    Roles    []string
}

// In APIKeyStore:
func (s *APIKeyStore) ValidateAndAuthorize(keyStr string) (*ValidatedKey, error) {
    // Validate key
    key, err := s.ValidateKey(keyStr)
    if err != nil {
        return nil, err
    }
    
    // Check rate limit
    allowed, err := s.CheckRateLimit(keyStr)
    if err != nil || !allowed {
        return nil, fmt.Errorf("rate limit exceeded")
    }
    
    // Update last used
    if err := s.UpdateLastUsed(keyStr); err != nil {
        // Log but don't fail
        return nil, err
    }
    
    return &ValidatedKey{
        APIKey:   key,
        ClientID: key.ClientID,
        Roles:    key.Roles,
    }, nil
}
```

Then in both middlewares:
```go
validatedKey, err := m.store.ValidateAndAuthorize(apiKey)
if err != nil {
    // return auth error
}

authCtx := &AuthContext{
    APIKey:   validatedKey.APIKey,
    ClientID: validatedKey.ClientID,
    Roles:    validatedKey.Roles,
}
```

**Estimated Effort:** Low-Medium (50 minutes)
- Add helper method to APIKeyStore
- Refactor 3 middleware methods
- Update error handling to match
- Better error semantics (different errors for different failures)

---

## 5. REPEATED NOTIFIER REGISTRATION PATTERN

### Issue 5.1: Identical Notifier Registration Logging in Main
**File Location:**
- `/Users/igodwin/Workspace/notifier/cmd/server/main.go`: Lines 193-241

**Description:**
The `registerNotifiers` function repeats the same registration pattern 3 times (for SMTP, Slack, Ntfy):
```go
for accountName, smtpConfig := range cfg.Notifiers.SMTP {
    notifier, err := notifier.NewSMTPNotifier(smtpConfig)
    if err != nil {
        logger.Warnf("Failed to create ... notifier for account '%s': %v", accountName, err)
    } else {
        if err := factory.RegisterNotifier(domain.TypeEmail, accountName, notifier); err != nil {
            logger.Fatalf("Failed to register ... notifier for account '%s': %v", accountName, err)
        }
        defaultStr := ""
        if smtpConfig.Default {
            defaultStr = " (default)"
        }
        logger.Infof("Registered ... notifier for account '%s'%s", accountName, defaultStr)
    }
}
// ... repeat for Slack ...
// ... repeat for Ntfy ...
```

**Impact:**
- 50+ lines with highly repetitive registration logic
- Maintenance burden - any changes must be made in 3 places
- Hard to add new notifier types
- Inconsistent error handling between types requires careful duplication

**Refactoring Recommendation:**
Create a generic registration helper function.

```go
type NotifierConfig interface {
    Default() bool
}

func registerNotifierType[T NotifierConfig](
    cfg map[string]T,
    notifierType domain.NotificationType,
    factory *notifier.Factory,
    logger *logging.Logger,
    creator func(T) (domain.Notifier, error),
    creatorName string) {
    
    for accountName, config := range cfg {
        notif, err := creator(config)
        if err != nil {
            logger.Warnf("Failed to create %s notifier for account '%s': %v", 
                creatorName, accountName, err)
        } else {
            if err := factory.RegisterNotifier(notifierType, accountName, notif); err != nil {
                logger.Fatalf("Failed to register %s notifier for account '%s': %v",
                    creatorName, accountName, err)
            }
            defaultStr := ""
            if config.Default() {
                defaultStr = " (default)"
            }
            logger.Infof("Registered %s notifier for account '%s'%s", 
                creatorName, accountName, defaultStr)
        }
    }
}

// In registerNotifiers:
registerNotifierType(cfg.Notifiers.SMTP, domain.TypeEmail, factory, logger,
    func(c *notifier.SMTPConfig) (domain.Notifier, error) {
        return notifier.NewSMTPNotifier(c)
    }, "SMTP")
```

**Estimated Effort:** Medium (1 hour)
- Refactor config structs to implement interface
- Create generic registration function
- Update 3 registration calls
- Add type-specific naming

---

## 6. REPEATED DEFAULT ACCOUNT RESOLUTION PATTERN

### Issue 6.1: Identical Default Account Search in Config
**File Location:**
- `/Users/igodwin/Workspace/notifier/internal/config/config.go`: Lines 346-380

**Description:**
The `GetDefaultAccount` method repeats the same search pattern 3 times:
```go
switch notifierType {
case domain.TypeEmail:
    for name, cfg := range c.Notifiers.SMTP {
        if cfg.Default {
            return name
        }
    }
    // Return first account if no default is set
    for name := range c.Notifiers.SMTP {
        return name
    }
// ... repeat for TypeSlack ...
// ... repeat for TypeNtfy ...
}
```

**Impact:**
- 35+ lines with repetitive logic
- Hard to maintain - changes needed in 3 places
- Error-prone for adding new notifier types
- The "get default or first" pattern is repeated 3 times

**Refactoring Recommendation:**
Create a generic helper function for finding default in a config map.

```go
// Helper function
func getDefaultOrFirst[T interface{ Default() bool }](configs map[string]T) string {
    // First pass: find explicitly marked default
    for name, cfg := range configs {
        if cfg.Default() {
            return name
        }
    }
    // Second pass: return first if no default
    for name := range configs {
        return name
    }
    return ""
}

// Update config structs to implement interface:
func (s *notifier.SMTPConfig) Default() bool { return s.Default }

// In GetDefaultAccount:
switch notifierType {
case domain.TypeEmail:
    return getDefaultOrFirst(c.Notifiers.SMTP)
case domain.TypeSlack:
    return getDefaultOrFirst(c.Notifiers.Slack)
case domain.TypeNtfy:
    return getDefaultOrFirst(c.Notifiers.Ntfy)
}
```

**Estimated Effort:** Low-Medium (45 minutes)
- Create generic helper function
- Refactor config structs to expose Default() method
- Update GetDefaultAccount to use helper
- Reduces 35+ lines to ~15 lines

---

## 7. REPEATED NOTIFICATIONRESULT ERROR CREATION

### Issue 7.1: Duplicated NotificationResult Creation with Error
**File Locations:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/smtp.go`: Lines 80-86, Lines 100-105
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go`: Lines 93-98
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go`: Lines 268-273

**Description:**
All notifier implementations repeat a pattern for creating error results:
```go
return &domain.NotificationResult{
    NotificationID: notification.ID,
    Success:        false,
    Error:          err.Error(),
    SentAt:         time.Now(),
}, fmt.Errorf("...")
```

This pattern (with variations) appears 4+ times across different notifier files.

**Impact:**
- Duplicated error result creation
- Inconsistent handling of SentAt timestamp
- Risk of incomplete error results

**Refactoring Recommendation:**
Create a helper in the BaseNotifier for error results.

```go
// In BaseNotifier:
func (b *BaseNotifier) ErrorResult(notificationID, errorMsg string, err error) *domain.NotificationResult {
    return &domain.NotificationResult{
        NotificationID: notificationID,
        Success:        false,
        Error:          errorMsg,
        SentAt:         time.Now(),
    }
}

// Usage in notifiers:
return b.ErrorResult(notification.ID, err.Error(), err), err
```

**Estimated Effort:** Low (30 minutes)
- Add helper method to BaseNotifier
- Update 4 error creation sites

---

## 8. REPEATED LOCK/UNLOCK PATTERNS IN API KEY STORE

### Issue 8.2: Similar RWMutex Lock Patterns
**File Location:**
- `/Users/igodwin/Workspace/notifier/internal/auth/auth.go`: Multiple methods

**Description:**
Multiple methods in APIKeyStore follow the same pattern:
```go
s.mu.RLock()
defer s.mu.RUnlock()
// read-only operations
```

And for write operations:
```go
s.mu.Lock()
defer s.mu.Unlock()
// write operations
```

While this is acceptable Go practice, it's a repeated pattern that could be standardized.

**Impact:**
- Not a critical issue - proper defer usage is correct
- Consistent approach across codebase
- Could be improved by adding higher-level thread-safe methods

**Refactoring Recommendation:**
This is acceptable as-is. The repetition is appropriate for the pattern.

**Estimated Effort:** N/A - Not recommended for change

---

## 9. REPEATED TYPE CONVERSION FUNCTIONS IN GRPC HANDLER

### Issue 9.1: Multiple Similar Type Conversion Functions
**File Location:**
- `/Users/igodwin/Workspace/notifier/api/grpc/handler.go`: Lines 279-449

**Description:**
Multiple conversion functions follow the same switch-case pattern:
- `convertProtoTypeToDomain` (lines 279-292)
- `convertDomainTypeToProto` (lines 294-307)
- `convertDomainToProtoType` (lines 331-344) - **DUPLICATE of above**
- `convertProtoContentTypeToDomain` (lines 309-318)
- `convertDomainContentTypeToProto` (lines 320-329)
- `convertDomainToProtoStatus` (lines 346-363)

Lines 331-344 are a duplicate of lines 294-307!

**Impact:**
- Exact duplication: `convertDomainTypeToProto` and `convertDomainToProtoType` do the same thing
- Code maintenance burden
- Risk of inconsistency between duplicate functions
- Takes up 30+ unnecessary lines

**Refactoring Recommendation:**
Remove the duplicate `convertDomainToProtoType` function and use `convertDomainTypeToProto` instead.

Search all files for usages:
- `convertDomainToProtoType` is used at line 368 in `convertDomainToProtoNotification`
- Replace this single usage with `convertDomainTypeToProto`
- Delete lines 331-344

```go
// REMOVE THIS (duplicate):
func convertDomainToProtoType(domainType domain.NotificationType) pb.NotificationType {
    // ... identical to convertDomainTypeToProto ...
}

// USE THIS INSTEAD:
func convertDomainTypeToProto(domainType domain.NotificationType) pb.NotificationType {
    // existing implementation
}

// Update line 368:
- Type:       convertDomainToProtoType(notif.Type),
+ Type:       convertDomainTypeToProto(notif.Type),
```

**Estimated Effort:** Low (15 minutes)
- Remove duplicate function (14 lines)
- Update 1 call site
- Simple find-and-replace

---

## 10. REPEATED REST RESPONSE HELPER PATTERN

### Issue 10.1: REST Response Helpers as Only Response Utility
**File Location:**
- `/Users/igodwin/Workspace/notifier/api/rest/handlers.go`: Lines 264-284

**Description:**
The REST handler has two response helper functions that format responses:
```go
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func respondError(w http.ResponseWriter, status int, message string, err error) {
    errMsg := message
    if err != nil {
        errMsg = message + ": " + err.Error()
    }
    respondJSON(w, status, map[string]interface{}{
        "error":   message,
        "details": errMsg,
    })
}
```

These are well-factored helpers already, but they're not shared with gRPC handler if it needs similar utilities.

**Impact:**
- REST-specific utilities (proper placement)
- gRPC has different response patterns (also proper)
- No duplication issue identified

**Refactoring Recommendation:**
Keep as-is. These are properly scoped to REST handler.

**Estimated Effort:** N/A - Not recommended for change

---

## SUMMARY TABLE OF REFACTORING OPPORTUNITIES

| Issue | Location | Type | Lines | Effort | Priority | Impact |
|-------|----------|------|-------|--------|----------|--------|
| 1.1 | Notifier Send validation | Repeated pattern | 4x 8 lines | Low | Medium | Maintainability |
| 2.1 | HTTP request handling | Code duplication | 25+ lines | Low-Med | High | Maintainability + Testing |
| 3.1 | Filter matching logic | Repeated pattern | 60+ lines | Low | High | Readability + Extensibility |
| 4.1 | Auth validation | Code duplication | 30+ lines | Low-Med | Medium | Maintainability + Consistency |
| 5.1 | Notifier registration | Repeated pattern | 50+ lines | Medium | Medium | Extensibility |
| 6.1 | Default account search | Repeated pattern | 35+ lines | Low-Med | Low | Code cleanliness |
| 7.1 | Error result creation | Repeated pattern | 4x 5 lines | Low | Low | Consistency |
| 9.1 | Type conversion (DUPLICATE) | Exact duplication | 14 lines | Low | High | Code cleanliness |

---

## IMPLEMENTATION PRIORITY

### Phase 1 (High Impact, Low Effort - Do First)
1. **Issue 9.1** - Remove duplicate `convertDomainToProtoType` (15 min)
2. **Issue 1.1** - Extract validation pattern (30 min)
3. **Issue 7.1** - Add error result helper (30 min)

### Phase 2 (Medium Impact, Low-Medium Effort)
4. **Issue 3.1** - Refactor filter matching (40 min)
5. **Issue 4.1** - Extract auth validation (50 min)
6. **Issue 2.1** - Create HTTP helper (45 min)

### Phase 3 (Good to Have, Medium Effort)
7. **Issue 5.1** - Generic registration function (60 min)
8. **Issue 6.1** - Generic default resolution (45 min)

---

## RISK ASSESSMENT

All recommended refactorings are **LOW RISK** because:
- They extract existing patterns without changing behavior
- No API changes or breaking changes
- All can be easily tested with existing test suite
- Changes are localized to internal utilities
- Incremental refactoring possible (Phase 1, 2, 3)

