# Notifier Service Client Integration Prompt

**Use this prompt with Claude or another AI assistant when building a client application that integrates with the Notifier service.**

---

## Prompt for AI Assistant

I need to build a client application that integrates with a notification service using gRPC. Please help me implement a production-ready client that follows best practices for performance, security, and reliability.

### Service Details

**Protocol:** gRPC (HTTP/2)
**Service Address:** `notifier-grpc:50051` (Kubernetes internal DNS)
**Authentication:** API Key via gRPC metadata
**API Key Format:** `nk_` prefix followed by 64 hex characters (e.g., `nk_abc123...`)

### Authentication Requirements

1. **API Key Delivery:**
   - Send API key in gRPC metadata with EVERY request
   - Two accepted header formats:
     - `authorization: Bearer nk_your_api_key_here` (preferred)
     - `x-api-key: nk_your_api_key_here`

2. **Per-Request Authentication:**
   - The service validates the API key on every RPC call
   - Authentication is NOT done at connection time
   - This means you can (and should) use long-lived connections

3. **Rate Limiting:**
   - Rate limits are enforced per API key
   - Default: 100 requests per minute (configurable per key)
   - Rate limit exceeded returns: `codes.ResourceExhausted`
   - Implement exponential backoff retry for rate limit errors

4. **Credential Management:**
   - Load API key from environment variable: `NOTIFIER_API_KEY`
   - NEVER hardcode API keys in source code
   - NEVER log the full API key (mask it: `nk_abc...xyz`)
   - Store in Kubernetes Secret if deploying to k8s

### Connection Management Requirements

**CRITICAL:** Use long-lived gRPC connections for optimal performance.

1. **Connection Lifecycle:**
   - Create ONE connection at application startup
   - Reuse the connection for ALL requests
   - Close the connection only when application exits
   - Do NOT create a new connection for each request

2. **Keepalive Configuration:**
   - Configure client keepalive to prevent connection timeouts
   - Recommended settings:
     - Time: 10 seconds (ping every 10s of inactivity)
     - Timeout: 3 seconds (wait for ping response)
     - PermitWithoutStream: true (allow pings when idle)

3. **Why This Matters:**
   - Connection reuse: 10x faster (1.5ms vs 15ms per request)
   - Eliminates TCP + TLS handshake overhead (10-15ms per request)
   - HTTP/2 multiplexing: handle multiple concurrent requests on one connection
   - Lower resource usage: one connection vs thousands

### Error Handling Requirements

1. **Handle These Error Codes:**
   - `codes.Unauthenticated` - Invalid or missing API key → Log error, don't retry
   - `codes.ResourceExhausted` - Rate limit exceeded → Retry with exponential backoff
   - `codes.InvalidArgument` - Bad request payload → Log error, don't retry
   - `codes.Unavailable` - Service temporarily unavailable → Retry with backoff
   - `codes.DeadlineExceeded` - Request timeout → Retry (may be transient)

2. **Retry Strategy:**
   - Implement exponential backoff: 100ms, 200ms, 400ms, 800ms
   - Maximum 3-5 retry attempts for retryable errors
   - Only retry: rate limits, unavailable, deadline exceeded
   - Do NOT retry: authentication errors, invalid arguments

3. **Context Timeouts:**
   - Set reasonable timeout for each request (5-30 seconds)
   - Use `context.WithTimeout()` for every RPC call
   - Allow timeout to be configurable

### Code Structure Requirements

1. **Client Structure:**
   ```
   NotifierClient struct:
   - conn: *grpc.ClientConn (long-lived)
   - client: pb.NotifierServiceClient
   - apiKey: string (loaded from env)
   - logger: logging interface
   ```

2. **Required Methods:**
   - `NewNotifierClient(address, apiKey string) (*NotifierClient, error)` - Initialize
   - `SendNotification(ctx, request) (response, error)` - Send single notification
   - `SendBatchNotifications(ctx, requests) (responses, error)` - Send batch
   - `HealthCheck(ctx) error` - Verify connection health
   - `Close() error` - Clean up connection

3. **Initialization Pattern:**
   - Create client as singleton at application startup
   - Use `sync.Once` to ensure only one instance
   - Defer `Close()` in main() to ensure cleanup

### Observability Requirements

1. **Logging:**
   - Log connection establishment (info level)
   - Log authentication failures (warn level)
   - Log rate limit exceeded (warn level)
   - Log successful sends (debug level)
   - NEVER log the full API key (mask it)

2. **Metrics (if using Prometheus):**
   - Counter: `notifier_requests_total` (labels: status, method)
   - Counter: `notifier_requests_failed_total` (labels: error_code)
   - Histogram: `notifier_request_duration_seconds`
   - Counter: `notifier_rate_limit_errors_total`

3. **Health Checks:**
   - Implement background health check goroutine
   - Check every 30-60 seconds
   - Call the service's HealthCheck RPC
   - Log warnings if health checks fail

### Security Requirements

1. **Credential Security:**
   - Load API key ONLY from environment variables
   - Never commit API keys to version control
   - Use `.gitignore` to exclude any files with credentials
   - Implement a `String()` method that masks the API key for logging

2. **TLS Configuration:**
   - For local development: `grpc.WithInsecure()` is acceptable
   - For production: Use TLS with proper certificate validation
   - For Kubernetes: Internal communication may use insecure (cluster network is trusted)

3. **Graceful Degradation:**
   - If notifier service is unavailable, application should continue
   - Log notification failures but don't crash the application
   - Consider implementing a circuit breaker pattern for resilience

### Testing Requirements

1. **Unit Tests:**
   - Mock the gRPC client interface
   - Test retry logic with rate limit errors
   - Test context timeout handling
   - Test credential masking in logs

2. **Integration Tests:**
   - Test actual connection to notifier service (if available)
   - Test authentication with valid and invalid keys
   - Test rate limiting behavior

### Kubernetes Deployment Requirements (if applicable)

1. **Environment Variables:**
   ```yaml
   env:
   - name: NOTIFIER_API_KEY
     valueFrom:
       secretKeyRef:
         name: notifier-credentials
         key: api-key
   - name: NOTIFIER_ADDRESS
     value: "notifier-grpc:50051"
   ```

2. **Secrets Configuration:**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: notifier-credentials
   type: Opaque
   stringData:
     api-key: nk_your_actual_api_key_here
   ```

### Available gRPC Methods

The NotifierService provides these RPC methods:

1. **SendNotification** - Send a single notification
   - Request: `SendNotificationRequest`
   - Response: `SendNotificationResponse` (contains notification_id)

2. **SendBatchNotifications** - Send multiple notifications at once
   - Request: `SendBatchNotificationsRequest`
   - Response: `SendBatchNotificationsResponse` (contains notification_ids)

3. **GetNotification** - Retrieve notification status by ID
   - Request: `GetNotificationRequest` (notification_id)
   - Response: `GetNotificationResponse`

4. **ListNotifications** - List recent notifications
   - Request: `ListNotificationsRequest` (pagination)
   - Response: `ListNotificationsResponse`

5. **CancelNotification** - Cancel a pending notification
   - Request: `CancelNotificationRequest` (notification_id)
   - Response: `CancelNotificationResponse`

6. **RetryNotification** - Retry a failed notification
   - Request: `RetryNotificationRequest` (notification_id)
   - Response: `RetryNotificationResponse`

7. **GetStats** - Get service statistics
   - Request: `GetStatsRequest`
   - Response: `GetStatsResponse`

8. **GetNotifiers** - List available notifier types and accounts
   - Request: `GetNotifiersRequest`
   - Response: `GetNotifiersResponse`

9. **HealthCheck** - Verify service health
   - Request: `HealthCheckRequest`
   - Response: `HealthCheckResponse`

### Notification Types Supported

- `NOTIFICATION_TYPE_EMAIL` - Email notifications
- `NOTIFICATION_TYPE_SLACK` - Slack messages
- `NOTIFICATION_TYPE_NTFY` - Ntfy.sh push notifications
- `NOTIFICATION_TYPE_STDOUT` - Console output (development only)

### Request Example Structure

```protobuf
message SendNotificationRequest {
  NotificationType type = 1;           // Required: email, slack, ntfy, stdout
  string subject = 2;                  // Required: notification subject/title
  string body = 3;                     // Required: notification body/content
  repeated string recipients = 4;      // Required: email addresses, slack channels, etc.
  Priority priority = 5;               // Optional: normal, high, critical
  string account = 6;                  // Optional: specific notifier account to use
  map<string, string> metadata = 7;    // Optional: additional metadata
}
```

### Implementation Goals

Please implement a client that:

1. ✅ Uses a single long-lived gRPC connection
2. ✅ Sends API key in metadata with every request
3. ✅ Implements exponential backoff retry for rate limits
4. ✅ Loads credentials from environment variables
5. ✅ Uses context with timeout for all requests
6. ✅ Implements keepalive to prevent connection timeouts
7. ✅ Includes proper error handling for all error codes
8. ✅ Logs important events without exposing credentials
9. ✅ Provides a health check mechanism
10. ✅ Includes graceful shutdown (connection cleanup)
11. ✅ Is production-ready with proper error handling and logging
12. ✅ Includes basic unit tests

### Example Usage Pattern

The client should be usable like this:

```go
// Initialize once at startup
client, err := notifier.NewClient(
    os.Getenv("NOTIFIER_ADDRESS"),
    os.Getenv("NOTIFIER_API_KEY"),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Use throughout application lifetime
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.SendNotification(ctx, &notifier.SendRequest{
    Type:       notifier.TypeEmail,
    Subject:    "Important Alert",
    Body:       "This is a critical notification",
    Recipients: []string{"admin@example.com"},
    Priority:   notifier.PriorityHigh,
})

if err != nil {
    log.Printf("Failed to send notification: %v", err)
    // Application continues - notification failure is not fatal
    return
}

log.Printf("Notification sent successfully: %s", resp.NotificationID)
```

### Additional Context

- This is a microservices architecture running in Kubernetes
- The notifier service handles routing to multiple notification backends (SMTP, Slack, Ntfy)
- The client may need to send notifications from multiple goroutines concurrently
- High reliability is important, but notification failures should not crash the application
- The API key has a rate limit of 100 requests per minute (may vary per key)

### Code Generation Note

If you need the protobuf definitions, they can be generated from the service's proto files located at `api/grpc/proto/notifier.proto` in the notifier service repository.

---

## Usage Instructions

**Copy the prompt above and provide it to your AI assistant when building the client. The prompt includes:**

- All authentication requirements and formats
- Connection management best practices
- Error handling strategies
- Security requirements
- Observability guidelines
- Complete API method listing
- Production-ready patterns
- Kubernetes deployment configuration

**The AI assistant should generate:**
- Complete, production-ready client code
- Proper connection pooling and keepalive
- Comprehensive error handling
- Secure credential management
- Health check implementation
- Unit tests
- Usage documentation

**Review the generated code for:**
- ✅ Single long-lived connection (not creating connections per request)
- ✅ API key sent in metadata with every RPC
- ✅ Keepalive configuration present
- ✅ Exponential backoff retry logic
- ✅ No hardcoded credentials
- ✅ Proper context timeouts
- ✅ Error handling for all gRPC error codes
- ✅ Graceful shutdown (Close() method)
