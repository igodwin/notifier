# Client Application Development Recommendations

This guide provides best practices for building applications that integrate with the Notifier service.

## Architecture & Design

### 1. Credential Injection Pattern

Use dependency injection to pass the API key to your notification client:

```go
type NotificationService struct {
    client    *NotifierClient
    apiKey    string  // Injected at initialization
    logger    Logger
}

func NewNotificationService(addr, apiKey string, logger Logger) (*NotificationService, error) {
    client, err := NewNotifierClient(addr, apiKey)
    if err != nil {
        return nil, err
    }
    return &NotificationService{
        client: client,
        apiKey: apiKey,
        logger: logger,
    }, nil
}
```

### 2. Configuration Management

**Structure your config to externalize credentials:**

```go
type Config struct {
    Notifier NotifierConfig `yaml:"notifier"`
    // ...
}

type NotifierConfig struct {
    Address string        `yaml:"address"`  // e.g., "localhost:50051"
    APIKey  string        `yaml:"api_key"`  // Load from env var
}

func (c *Config) LoadFromEnv() {
    if key := os.Getenv("NOTIFIER_API_KEY"); key != "" {
        c.Notifier.APIKey = key
    }
}
```

### 3. Rate Limiting & Retry Logic

Implement exponential backoff for rate limit errors:

```go
func (s *NotificationService) SendWithRetry(ctx context.Context, req *SendRequest) error {
    var lastErr error
    maxRetries := 3
    baseDelay := 100 * time.Millisecond

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := s.Send(ctx, req)

        // Check if it's a rate limit error
        if err != nil && isRateLimitError(err) {
            // Exponential backoff: 100ms, 200ms, 400ms
            delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
            time.Sleep(delay)
            lastErr = err
            continue
        }

        if err != nil {
            return err  // Don't retry non-rate-limit errors
        }

        return nil  // Success
    }

    return fmt.Errorf("rate limit exceeded after %d retries: %w", maxRetries, lastErr)
}
```

### 4. Error Handling Strategy

Define clear error handling for each scenario:

```go
type NotificationError struct {
    Code    string  // "auth_failed", "rate_limited", "invalid_request", "server_error"
    Message string
    Retryable bool
}

func isRetryable(err error) bool {
    // Retryable: rate limits, temporary network errors, 503
    // Non-retryable: auth errors, validation errors, 404
    // ...
}
```

## Security Best Practices

### 1. Secret Management Hierarchy

```
Priority 1: Environment Variables
Priority 2: Configuration Files (restricted permissions)
Priority 3: Secrets Manager (Vault, AWS Secrets Manager)
Priority 4: Kubernetes Secrets (if using K8s)
```

**Example:**

```bash
# Load from highest priority available
if [ -n "$NOTIFIER_API_KEY" ]; then
    # Use env var
    API_KEY="$NOTIFIER_API_KEY"
elif [ -f /etc/notifier-secret ]; then
    # Use secret file (only readable by app user)
    API_KEY=$(cat /etc/notifier-secret)
else
    # Fail - no credential found
    exit 1
fi
```

### 2. Key Rotation Strategy

Implement zero-downtime key rotation:

```go
type NotifierClient struct {
    primaryKey   string
    secondaryKey string  // For rotation period
}

func (c *NotifierClient) Authenticate(ctx context.Context) error {
    // Try primary key first
    if err := c.tryAuthenticate(ctx, c.primaryKey); err == nil {
        return nil
    }

    // Fall back to secondary key
    if err := c.tryAuthenticate(ctx, c.secondaryKey); err == nil {
        return nil
    }

    return errors.New("authentication failed with all keys")
}

// During rotation:
// 1. Create new key
// 2. Deploy code with new key as primary
// 3. After deploy completes, disable old key in Notifier service
// 4. Remove old key from config
```

### 3. Preventing Credential Leaks

```go
// DON'T: Log credentials
logger.Infof("Using API key: %s", apiKey)  // WRONG!

// DO: Log masked credentials
maskedKey := apiKey[:10] + "..." + apiKey[len(apiKey)-4:]
logger.Infof("Using API key: %s", maskedKey)  // CORRECT

// DO: Implement SafeString for sensitive values
type SafeString string

func (s SafeString) String() string {
    str := string(s)
    if len(str) < 10 {
        return "***"
    }
    return str[:4] + "***" + str[len(str)-4:]
}

// DO: Clear sensitive data from memory after use
func (c *NotifierClient) Close() error {
    if c.apiKey != "" {
        // Clear from memory (best-effort)
        for i := 0; i < len(c.apiKey); i++ {
            c.apiKey[i] = 0
        }
    }
    return c.conn.Close()
}
```

## Performance Optimization

### 1. Connection Pooling

For gRPC:

```go
// Reuse single connection for multiple calls
conn, _ := grpc.Dial(address,
    grpc.WithDefaultCallOptions(
        grpc.MaxCallRecvMsgSize(4*1024*1024),
    ),
)
defer conn.Close()

client := pb.NewNotifierServiceClient(conn)

// Reuse for multiple calls
for _, notif := range notifications {
    client.SendNotification(ctx, notif)
}
```

For REST:

```go
// Use http.Client with connection pooling
httpClient := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        MaxConnsPerHost:     100,
    },
}

// Reuse for multiple requests
resp, _ := httpClient.Do(req)
```

### 2. Batch Operations

Group notifications to reduce API calls:

```go
type BatchNotifier struct {
    client    *NotifierClient
    batchSize int
    ticker    *time.Ticker
    queue     []*SendRequest
}

func (bn *BatchNotifier) Queue(req *SendRequest) {
    bn.queue = append(bn.queue, req)

    // Flush when batch is full
    if len(bn.queue) >= bn.batchSize {
        bn.Flush()
    }
}

func (bn *BatchNotifier) Flush() {
    if len(bn.queue) == 0 {
        return
    }

    // Send batch
    bn.client.SendBatch(context.Background(), bn.queue)
    bn.queue = nil
}
```

### 3. Caching & Memoization

Cache notifier metadata to reduce API calls:

```go
type CachedNotifierClient struct {
    client       *NotifierClient
    notifiersMu  sync.RWMutex
    notifiers    *pb.NotifiersResponse
    notifiersAge time.Time
    cacheTTL     time.Duration
}

func (cnc *CachedNotifierClient) GetNotifiers(ctx context.Context) (*pb.NotifiersResponse, error) {
    cnc.notifiersMu.RLock()
    if time.Since(cnc.notifiersAge) < cnc.cacheTTL && cnc.notifiers != nil {
        defer cnc.notifiersMu.RUnlock()
        return cnc.notifiers, nil
    }
    cnc.notifiersMu.RUnlock()

    // Fetch from server
    notifiers, err := cnc.client.GetNotifiers(ctx)
    if err != nil {
        return nil, err
    }

    // Cache result
    cnc.notifiersMu.Lock()
    cnc.notifiers = notifiers
    cnc.notifiersAge = time.Now()
    cnc.notifiersMu.Unlock()

    return notifiers, nil
}
```

## Monitoring & Observability

### 1. Instrumentation

Instrument your notification client:

```go
import "go.opentelemetry.io/otel"

type InstrumentedNotifierClient struct {
    client *NotifierClient
    tracer trace.Tracer
}

func (inc *InstrumentedNotifierClient) SendNotification(ctx context.Context, req *SendRequest) error {
    ctx, span := inc.tracer.Start(ctx, "send_notification")
    defer span.End()

    span.SetAttributes(
        attribute.String("notification.type", string(req.Type)),
        attribute.Int("notification.recipients", len(req.Recipients)),
    )

    err := inc.client.SendNotification(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return err
}
```

### 2. Metrics Collection

Track key metrics:

```go
type MetricsCollector struct {
    sendAttempts    prometheus.Counter
    sendSuccesses   prometheus.Counter
    sendFailures    prometheus.Counter
    sendDuration    prometheus.Histogram
    rateLimitErrors prometheus.Counter
}

func (mc *MetricsCollector) Record(result *SendResult) {
    mc.sendAttempts.Inc()

    if result.Error != nil {
        mc.sendFailures.Inc()
        if isRateLimitError(result.Error) {
            mc.rateLimitErrors.Inc()
        }
    } else {
        mc.sendSuccesses.Inc()
    }

    mc.sendDuration.Observe(result.Duration.Seconds())
}
```

### 3. Health Checks

Periodically verify connectivity:

```go
func (s *NotificationService) HealthCheck(ctx context.Context) error {
    deadline, _ := context.WithTimeout(ctx, 5*time.Second)
    _, err := s.client.HealthCheck(deadline)
    return err
}

// In your main loop
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        if err := s.HealthCheck(context.Background()); err != nil {
            logger.Errorf("Health check failed: %v", err)
            // Maybe trigger alerts or circuit breaker
        }
    }
}()
```

## Testing

### 1. Mock the Notifier Client

```go
type MockNotifierClient struct {
    SendNotificationFunc func(context.Context, *SendRequest) error
}

func (m *MockNotifierClient) SendNotification(ctx context.Context, req *SendRequest) error {
    if m.SendNotificationFunc != nil {
        return m.SendNotificationFunc(ctx, req)
    }
    return nil
}

// In tests
func TestNotificationService(t *testing.T) {
    mock := &MockNotifierClient{
        SendNotificationFunc: func(ctx context.Context, req *SendRequest) error {
            assert.Equal(t, "email", string(req.Type))
            return nil
        },
    }

    svc := NewNotificationService(mock)
    err := svc.Notify("test@example.com", "Hello")
    assert.NoError(t, err)
}
```

### 2. Test Rate Limiting

```go
func TestRateLimitHandling(t *testing.T) {
    responses := []error{
        status.Error(codes.ResourceExhausted, "rate limit"),
        status.Error(codes.ResourceExhausted, "rate limit"),
        nil,  // Success on third try
    }

    callCount := 0
    mock := &MockNotifierClient{
        SendNotificationFunc: func(ctx context.Context, req *SendRequest) error {
            err := responses[callCount]
            callCount++
            return err
        },
    }

    svc := NewNotificationService(mock)
    err := svc.SendWithRetry(context.Background(), &SendRequest{...})
    assert.NoError(t, err)
    assert.Equal(t, 3, callCount)
}
```

## Deployment Considerations

### 1. Environment Variables Checklist

```bash
# Production checklist
NOTIFIER_API_KEY=nk_...           # From secure secrets manager
NOTIFIER_ADDRESS=notifier:50051    # Use internal DNS
NOTIFIER_TIMEOUT=30s               # Reasonable timeout
APP_LOG_LEVEL=info                 # Not debug (sensitive logs)
```

### 2. Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notifier-credentials
type: Opaque
stringData:
  api-key: nk_...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: my-app
        env:
        - name: NOTIFIER_API_KEY
          valueFrom:
            secretKeyRef:
              name: notifier-credentials
              key: api-key
        - name: NOTIFIER_ADDRESS
          value: notifier:50051
```

### 3. Docker Best Practices

```dockerfile
# DON'T embed credentials
ARG API_KEY=default
ENV NOTIFIER_API_KEY=$API_KEY

# DO mount secrets
# docker run -v /run/secrets/notifier_api_key:/etc/notifier-secret ...

# DO use multi-stage builds to exclude dev dependencies
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o app .

FROM alpine:latest
COPY --from=builder /build/app .
# Credentials provided at runtime only
CMD ["./app"]
```

## Versioning & Compatibility

### 1. API Versioning

Your client should handle API changes gracefully:

```go
type APIVersion struct {
    Major int
    Minor int
    Patch int
}

func (s *NotificationService) CheckCompatibility(version APIVersion) error {
    if version.Major != 1 {
        return fmt.Errorf("incompatible API version: %d", version.Major)
    }
    return nil
}
```

### 2. Feature Detection

Detect available features instead of hardcoding versions:

```go
func (s *NotificationService) SupportsHTMLEmail() bool {
    notifiers, _ := s.GetNotifiers(context.Background())
    for _, n := range notifiers.Notifiers {
        if n.Type == TypeEmail {
            return true  // Assume HTML support in email notifiers
        }
    }
    return false
}
```

## Troubleshooting Checklist

- [ ] API key format is correct: `nk_<32-hex>`
- [ ] API key hasn't expired
- [ ] Client has required roles for the notifier
- [ ] Rate limit hasn't been exceeded
- [ ] Notifier service is accessible (network, firewall)
- [ ] Request payload is valid JSON/protobuf
- [ ] Notifier account exists in service config
- [ ] Credentials are being loaded from environment (not hardcoded)
- [ ] Connection is using correct protocol (HTTP/2 for gRPC)
- [ ] Logs are not leaking sensitive data

## Summary

1. **Externalize credentials** - Use env vars or secrets managers
2. **Implement retries** - Handle rate limits gracefully
3. **Cache when possible** - Reduce API calls
4. **Monitor health** - Regular health checks
5. **Instrument code** - Add tracing and metrics
6. **Test thoroughly** - Mock clients and test error cases
7. **Secure deployment** - Mount secrets at runtime, not build time
8. **Log carefully** - Never log API keys or sensitive data
