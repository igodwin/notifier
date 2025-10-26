# Code Duplication Refactoring - Quick Reference Guide

## Top 3 Quick Wins (Do These First!)

### 1. Remove Duplicate Type Conversion Function (15 minutes)
**File:** `/Users/igodwin/Workspace/notifier/api/grpc/handler.go`

**Problem:** Function `convertDomainToProtoType` (lines 331-344) is an exact duplicate of `convertDomainTypeToProto` (lines 294-307).

**Action:**
1. Delete lines 331-344
2. Replace call at line 368:
   ```diff
   - Type:       convertDomainToProtoType(notif.Type),
   + Type:       convertDomainTypeToProto(notif.Type),
   ```

**Result:** Removes 14 lines of duplicate code

---

### 2. Extract Notifier Validation Pattern (30 minutes)
**Files:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/smtp.go` (lines 63-70)
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go` (lines 78-85)
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go` (lines 185-192)
- `/Users/igodwin/Workspace/notifier/internal/notifier/stdout.go` (lines 26-33)

**Problem:** All 4 notifiers have identical validation at start of Send():
```go
if err := ValidateContext(ctx); err != nil {
    return nil, err
}
if err := s.Validate(notification); err != nil {
    return nil, err
}
```

**Action:**
1. Add to `/Users/igodwin/Workspace/notifier/internal/notifier/notifier.go`:
```go
// ValidateNotification validates context and notification for sending
func ValidateNotification(ctx context.Context, n *domain.Notification, validator domain.Notifier) error {
    if err := ValidateContext(ctx); err != nil {
        return err
    }
    return validator.Validate(n)
}
```

2. Replace validation blocks in all 4 files with:
```go
if err := ValidateNotification(ctx, notification, s); err != nil {
    return nil, err
}
```

**Result:** Reduces duplication from 4 places to 1, easier maintenance

---

### 3. Add NotificationResult Error Helper (30 minutes)
**Files:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/smtp.go` (multiple locations)
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go` (multiple locations)
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go` (multiple locations)

**Problem:** Repeated error result creation pattern in all notifiers

**Action:**
1. Add to `BaseNotifier` struct:
```go
func (b *BaseNotifier) ErrorResult(notificationID, msg string) *domain.NotificationResult {
    return &domain.NotificationResult{
        NotificationID: notificationID,
        Success:        false,
        Error:          msg,
        SentAt:         time.Now(),
    }
}
```

2. Replace error creation calls with:
```go
return b.ErrorResult(notification.ID, err.Error()), err
```

**Result:** Consistent error handling across all notifiers

---

## Medium-Effort Improvements

### 4. Refactor Filter Matching (40 minutes)
**File:** `/Users/igodwin/Workspace/notifier/internal/service/service.go` (lines 481-540)

**Problem:** 60+ lines of repetitive filter matching logic

**Solution:** Extract helper functions:
```go
func contains[T comparable](items []T, target T) bool {
    for _, item := range items {
        if item == target {
            return true
        }
    }
    return false
}

func notificationHasRecipient(n *domain.Notification, recipients []string) bool {
    for _, fr := range recipients {
        if contains(n.Recipients, fr) {
            return true
        }
    }
    return false
}
```

Then simplify `matchesFilter` to use these helpers.

**Result:** Reduces 60+ lines to ~30, easier to extend

---

### 5. Extract Auth Validation (50 minutes)
**Files:**
- `/Users/igodwin/Workspace/notifier/internal/auth/rest_middleware.go` (lines 35-50)
- `/Users/igodwin/Workspace/notifier/internal/auth/grpc_middleware.go` (lines 38-50, 82-94)

**Problem:** Same validation logic in 3 places

**Solution:** Add to `APIKeyStore`:
```go
type ValidatedKey struct {
    APIKey   *APIKey
    ClientID string
    Roles    []string
}

func (s *APIKeyStore) ValidateAndAuthorize(keyStr string) (*ValidatedKey, error) {
    key, err := s.ValidateKey(keyStr)
    if err != nil {
        return nil, err
    }
    
    allowed, err := s.CheckRateLimit(keyStr)
    if err != nil || !allowed {
        return nil, fmt.Errorf("rate limit exceeded")
    }
    
    if err := s.UpdateLastUsed(keyStr); err != nil {
        // Log warning but continue
    }
    
    return &ValidatedKey{
        APIKey:   key,
        ClientID: key.ClientID,
        Roles:    key.Roles,
    }, nil
}
```

Use in both middleware implementations.

**Result:** Consistent auth logic, easier to update validation rules

---

### 6. Create HTTP Request Helper (45 minutes)
**Files:**
- `/Users/igodwin/Workspace/notifier/internal/notifier/slack.go` (sendToSlack)
- `/Users/igodwin/Workspace/notifier/internal/notifier/ntfy.go` (sendToTopic)

**Problem:** 25+ lines of identical HTTP handling

**Solution:** Add to notifier package:
```go
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

**Result:** Single source of truth for HTTP logic

---

## More Involved Refactorings

### 7. Generic Notifier Registration (60 minutes)
**File:** `/Users/igodwin/Workspace/notifier/cmd/server/main.go` (lines 193-241)

Creates generic registration helper to reduce 50+ lines of repetitive code.

### 8. Generic Default Account Resolution (45 minutes)
**File:** `/Users/igodwin/Workspace/notifier/internal/config/config.go` (lines 346-380)

Extract generic helper to eliminate 35+ lines of repetitive default account lookup.

---

## Implementation Checklist

### Phase 1 (Recommended: Next Hour)
- [ ] Remove duplicate `convertDomainToProtoType` (15 min)
- [ ] Extract validation helper (30 min)  
- [ ] Add error result helper (30 min)
- **Total: 75 minutes**

### Phase 2 (Recommended: Next Sprint)
- [ ] Refactor filter matching (40 min)
- [ ] Extract auth validation (50 min)
- [ ] Create HTTP helper (45 min)
- **Total: 135 minutes**

### Phase 3 (Nice to Have: Later)
- [ ] Generic registration function (60 min)
- [ ] Generic default resolution (45 min)
- **Total: 105 minutes**

---

## Testing Strategy

1. **Phase 1 refactorings:** Run existing unit tests - no behavior changes
2. **Phase 2 refactorings:** Add unit tests for new helper functions
3. **Phase 3 refactorings:** Comprehensive integration tests for registration flow

All changes maintain backward compatibility.

---

## Expected Benefits

| Metric | Phase 1 | Phase 1+2 | All Phases |
|--------|---------|-----------|-----------|
| Lines of duplication removed | ~60 | ~250 | ~380 |
| Number of duplicate patterns | 3 | 6 | 8 |
| Estimated maintenance effort reduction | 15% | 40% | 60% |

---

## Files to Review After Refactoring

1. All notifier implementations (`internal/notifier/`)
2. Service filtering logic (`internal/service/service.go`)
3. Auth middleware files (`internal/auth/`)
4. Main server initialization (`cmd/server/main.go`)
5. Config file (`internal/config/config.go`)
