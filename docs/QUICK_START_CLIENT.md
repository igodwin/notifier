# Quick Start: Notifier Client Integration

**5-minute guide to integrating with the Notifier service**

## Essential Requirements

### 1. Connection: Use Long-Lived gRPC Connection ⚡

```go
// ✅ DO THIS: Create once, reuse forever
client, _ := NewNotifierClient("notifier-grpc:50051", apiKey)
defer client.Close()

for i := 0; i < 10000; i++ {
    client.SendNotification(ctx, req)  // Reuse connection
}

// ❌ DON'T DO THIS: Creating connection per request
for i := 0; i < 10000; i++ {
    client, _ := NewNotifierClient("notifier-grpc:50051", apiKey)
    client.SendNotification(ctx, req)
    client.Close()  // SLOW: 10x slower!
}
```

**Why?** Connection reuse is 10x faster (1.5ms vs 15ms per request)

### 2. Authentication: Send API Key with Every Request 🔐

```go
import "google.golang.org/grpc/metadata"

// Add API key to metadata for each request
ctx = metadata.AppendToOutgoingContext(ctx,
    "authorization", "Bearer "+apiKey)

resp, err := client.SendNotification(ctx, req)
```

**Two accepted formats:**
- `authorization: Bearer nk_your_api_key` (preferred)
- `x-api-key: nk_your_api_key`

**API Key Format:** `nk_` + 64 hex characters (e.g., `nk_a1b2c3d4...`)

### 3. Credentials: Load from Environment 🔒

```go
// ✅ DO THIS
apiKey := os.Getenv("NOTIFIER_API_KEY")

// ❌ DON'T DO THIS
apiKey := "nk_abc123..."  // NEVER hardcode!
```

**Kubernetes Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notifier-credentials
stringData:
  api-key: nk_your_actual_key_here
---
# In your deployment:
env:
- name: NOTIFIER_API_KEY
  valueFrom:
    secretKeyRef:
      name: notifier-credentials
      key: api-key
```

### 4. Keepalive: Prevent Connection Timeouts ⏰

```go
import "google.golang.org/grpc/keepalive"

conn, err := grpc.Dial(address,
    grpc.WithKeepaliveParams(keepalive.ClientParameters{
        Time:                10 * time.Second,  // Ping every 10s
        Timeout:             3 * time.Second,   // Wait 3s for response
        PermitWithoutStream: true,              // Ping when idle
    }),
)
```

### 5. Error Handling: Retry Only Specific Errors 🔄

```go
err := client.SendNotification(ctx, req)

switch status.Code(err) {
case codes.ResourceExhausted:
    // Rate limit exceeded - RETRY with backoff
    time.Sleep(100 * time.Millisecond)
    // Try again...

case codes.Unauthenticated:
    // Invalid API key - DON'T RETRY, log error
    log.Errorf("Authentication failed: %v", err)

case codes.InvalidArgument:
    // Bad request - DON'T RETRY, fix request
    log.Errorf("Invalid request: %v", err)

case codes.Unavailable:
    // Service down - RETRY with backoff
    time.Sleep(100 * time.Millisecond)
    // Try again...
}
```

**Retry Strategy:**
- Exponential backoff: 100ms → 200ms → 400ms → 800ms
- Max 3-5 attempts
- Only retry: `ResourceExhausted`, `Unavailable`, `DeadlineExceeded`

### 6. Context Timeout: Always Set Timeout ⏱️

```go
// ✅ DO THIS: Set timeout for each request
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

resp, err := client.SendNotification(ctx, req)

// ❌ DON'T DO THIS: No timeout
ctx := context.Background()
resp, err := client.SendNotification(ctx, req)  // Could hang forever!
```

## Complete Minimal Example

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    pb "github.com/igodwin/notifier/api/grpc/pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"
    "google.golang.org/grpc/metadata"
)

type NotifierClient struct {
    conn   *grpc.ClientConn
    client pb.NotifierServiceClient
    apiKey string
}

func NewNotifierClient(address, apiKey string) (*NotifierClient, error) {
    // Create long-lived connection
    conn, err := grpc.Dial(address,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                10 * time.Second,
            Timeout:             3 * time.Second,
            PermitWithoutStream: true,
        }),
    )
    if err != nil {
        return nil, err
    }

    return &NotifierClient{
        conn:   conn,
        client: pb.NewNotifierServiceClient(conn),
        apiKey: apiKey,
    }, nil
}

func (nc *NotifierClient) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
    // Add timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Add API key to metadata
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+nc.apiKey)

    // Make RPC call
    return nc.client.SendNotification(ctx, req)
}

func (nc *NotifierClient) Close() error {
    return nc.conn.Close()
}

func main() {
    // Load credentials
    address := os.Getenv("NOTIFIER_ADDRESS")
    apiKey := os.Getenv("NOTIFIER_API_KEY")

    if address == "" {
        address = "notifier-grpc:50051"
    }
    if apiKey == "" {
        log.Fatal("NOTIFIER_API_KEY environment variable required")
    }

    // Create client (once)
    client, err := NewNotifierClient(address, apiKey)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Send notification
    resp, err := client.SendNotification(context.Background(), &pb.SendNotificationRequest{
        Type:       pb.NotificationType_NOTIFICATION_TYPE_EMAIL,
        Subject:    "Test Notification",
        Body:       "This is a test from the notifier client",
        Recipients: []string{"user@example.com"},
        Priority:   pb.Priority_PRIORITY_NORMAL,
    })

    if err != nil {
        log.Fatalf("Failed to send notification: %v", err)
    }

    log.Printf("Notification sent successfully: %s", resp.NotificationId)
}
```

## Service Connection Details

**Address:**
- Same namespace: `notifier-grpc:50051`
- Different namespace: `notifier-grpc.default.svc.cluster.local:50051`

**Rate Limit:** 100 requests/minute (default, configurable per key)

**Available Methods:**
- `SendNotification` - Send single notification
- `SendBatchNotifications` - Send multiple notifications
- `GetNotification` - Get notification status
- `ListNotifications` - List recent notifications
- `CancelNotification` - Cancel pending notification
- `RetryNotification` - Retry failed notification
- `GetStats` - Get service statistics
- `GetNotifiers` - List available notifiers
- `HealthCheck` - Check service health

**Notification Types:**
- `NOTIFICATION_TYPE_EMAIL` - Email via SMTP
- `NOTIFICATION_TYPE_SLACK` - Slack webhook
- `NOTIFICATION_TYPE_NTFY` - Ntfy.sh push notifications
- `NOTIFICATION_TYPE_STDOUT` - Console output (dev only)

## Security Checklist

- [ ] API key loaded from environment variable
- [ ] No hardcoded credentials in source code
- [ ] API key never logged in full (mask it: `nk_abc...xyz`)
- [ ] Kubernetes Secret created for API key
- [ ] Deployment configured to load secret as env var
- [ ] `.gitignore` excludes any credential files

## Testing Your Integration

```bash
# Set environment variables
export NOTIFIER_ADDRESS="notifier-grpc:50051"
export NOTIFIER_API_KEY="nk_your_api_key_here"

# Run your application
go run main.go

# Check logs for:
# ✅ "Notification sent successfully: <id>"
# ❌ "Authentication failed" - check API key
# ❌ "Rate limit exceeded" - slow down requests
```

## Common Issues

**"Unauthenticated" error:**
- Check API key format starts with `nk_`
- Verify key is 66 characters total (nk_ + 64 hex)
- Confirm key is active in notifier service
- Check you're adding it to metadata correctly

**"Rate limit exceeded" error:**
- Implement exponential backoff retry
- Consider batching notifications
- Request higher rate limit for your key

**Connection timeouts:**
- Add keepalive configuration
- Check network connectivity to `notifier-grpc:50051`
- Verify notifier service is running: `kubectl get pods -l app=notifier`

**Slow performance:**
- Confirm you're reusing connection (not creating per request)
- Check keepalive is configured
- Verify you're using connection pooling for high concurrency

## Next Steps

For more details, see:
- `CLIENT_INTEGRATION_PROMPT.md` - Complete prompt for AI assistants
- `GRPC_CONNECTION_OPTIMIZATION.md` - Deep dive on performance
- `CLIENT_RECOMMENDATIONS.md` - Advanced patterns and best practices
- `AUTH.md` - Authentication system details

## Getting Your API Key

Contact your Notifier service administrator to:
1. Create an API key for your application
2. Set appropriate rate limits
3. Assign required roles for notifier access
4. Get the key in format: `nk_<64-hex-characters>`

Store it securely and never commit it to version control!
