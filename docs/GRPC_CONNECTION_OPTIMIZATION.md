# gRPC Connection Optimization Guide

## TL;DR

**Yes, you can and should use long-lived gRPC connections!**

- ✅ Single connection can handle thousands of concurrent requests
- ✅ Authentication happens **per-request**, not per-connection
- ✅ Connection reuse eliminates TCP/TLS handshake overhead
- ✅ HTTP/2 multiplexing allows concurrent RPCs on one connection
- ✅ Built-in keepalive prevents connection timeouts

## How gRPC Authentication Works

### Key Point: Auth is Per-Request, Not Per-Connection

Your current implementation authenticates **each RPC call**, not the connection itself. This means:

1. Client establishes a **long-lived connection** (once)
2. Client sends **API key in metadata** with each request
3. Server validates the key for **every RPC call**
4. Connection stays open for multiple requests

```
Connection Lifecycle:
┌─────────────────────────────────────────────────────────────┐
│ TCP Connection (persistent)                                 │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ TLS Handshake (once)                                    │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                              │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Request 1: Authorization: Bearer nk_abc... → Validated  │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Request 2: Authorization: Bearer nk_abc... → Validated  │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ Request 3: Authorization: Bearer nk_abc... → Validated  │ │
│ └─────────────────────────────────────────────────────────┘ │
│                     ... (connection stays open)             │
└─────────────────────────────────────────────────────────────┘
```

### Authentication Overhead Analysis

**Per-Connection (One-Time):**
- TCP handshake: ~1-2ms (3-way handshake)
- TLS handshake: ~5-10ms (certificate exchange, key agreement)
- **Total: ~10-15ms once**

**Per-Request (Every Call):**
- API key validation: ~0.1-1ms (in-memory lookup)
- Rate limit check: ~0.1ms (in-memory counter)
- **Total: ~0.2-1ms per request**

**With Connection Reuse:**
- First request: 10-15ms (connection) + 1ms (auth) = **11-16ms**
- Subsequent requests: **1ms** (only auth, no connection setup)

**Without Connection Reuse (reconnecting each time):**
- Every request: 10-15ms (connection) + 1ms (auth) = **11-16ms**

**Savings: 10-15ms per request after the first one!**

## Recommended Client Pattern

### Basic Long-Lived Connection

```go
package main

import (
    "context"
    "log"
    "time"

    pb "github.com/igodwin/notifier/api/grpc/pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"
)

type NotifierClient struct {
    conn   *grpc.ClientConn
    client pb.NotifierServiceClient
    apiKey string
}

func NewNotifierClient(address, apiKey string) (*NotifierClient, error) {
    // Establish long-lived connection
    conn, err := grpc.Dial(address,
        grpc.WithTransportCredentials(insecure.NewCredentials()),

        // Connection pool settings
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(4*1024*1024), // 4MB
            grpc.MaxCallSendMsgSize(4*1024*1024),
        ),

        // Keepalive settings
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:                10 * time.Second, // Send keepalive ping every 10s
            Timeout:             3 * time.Second,  // Wait 3s for ping ack
            PermitWithoutStream: true,             // Allow pings when no streams
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
    // Add API key to metadata for THIS request
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+nc.apiKey)

    // Make RPC call - reuses existing connection
    return nc.client.SendNotification(ctx, req)
}

func (nc *NotifierClient) Close() error {
    return nc.conn.Close()
}

func main() {
    // Create client with long-lived connection
    client, err := NewNotifierClient("notifier-grpc:50051", "nk_your_api_key")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close() // Close when application exits

    // Reuse client for multiple requests
    for i := 0; i < 1000; i++ {
        ctx := context.Background()
        resp, err := client.SendNotification(ctx, &pb.SendNotificationRequest{
            Type:    pb.NotificationType_NOTIFICATION_TYPE_EMAIL,
            Subject: "Test notification",
            Body:    "This is a test",
            Recipients: []string{"user@example.com"},
        })

        if err != nil {
            log.Printf("Request %d failed: %v", i, err)
            continue
        }

        log.Printf("Request %d succeeded: %s", i, resp.NotificationId)
    }

    // Connection is closed when main() exits
}
```

### Advanced: Connection Pool for High Concurrency

For extremely high throughput, you can create multiple connections:

```go
package main

import (
    "context"
    "sync"

    pb "github.com/igodwin/notifier/api/grpc/pb"
    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"
)

type NotifierPool struct {
    connections []*grpc.ClientConn
    clients     []pb.NotifierServiceClient
    apiKey      string
    current     uint32
    mu          sync.Mutex
}

func NewNotifierPool(address, apiKey string, poolSize int) (*NotifierPool, error) {
    pool := &NotifierPool{
        connections: make([]*grpc.ClientConn, poolSize),
        clients:     make([]pb.NotifierServiceClient, poolSize),
        apiKey:      apiKey,
    }

    // Create multiple connections
    for i := 0; i < poolSize; i++ {
        conn, err := grpc.Dial(address,
            grpc.WithTransportCredentials(insecure.NewCredentials()),
            grpc.WithKeepaliveParams(keepalive.ClientParameters{
                Time:                10 * time.Second,
                Timeout:             3 * time.Second,
                PermitWithoutStream: true,
            }),
        )
        if err != nil {
            // Clean up any connections already created
            pool.Close()
            return nil, err
        }

        pool.connections[i] = conn
        pool.clients[i] = pb.NewNotifierServiceClient(conn)
    }

    return pool, nil
}

func (np *NotifierPool) getClient() pb.NotifierServiceClient {
    // Round-robin connection selection
    np.mu.Lock()
    defer np.mu.Unlock()

    idx := np.current % uint32(len(np.clients))
    np.current++
    return np.clients[idx]
}

func (np *NotifierPool) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
    // Add API key to metadata
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+np.apiKey)

    // Get a client from pool (round-robin)
    client := np.getClient()

    return client.SendNotification(ctx, req)
}

func (np *NotifierPool) Close() error {
    var firstErr error
    for _, conn := range np.connections {
        if conn != nil {
            if err := conn.Close(); err != nil && firstErr == nil {
                firstErr = err
            }
        }
    }
    return firstErr
}

// Usage
func main() {
    // Create pool with 4 connections
    pool, err := NewNotifierPool("notifier-grpc:50051", "nk_your_api_key", 4)
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Use pool concurrently from multiple goroutines
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            resp, err := pool.SendNotification(context.Background(), &pb.SendNotificationRequest{
                Type:    pb.NotificationType_NOTIFICATION_TYPE_EMAIL,
                Subject: fmt.Sprintf("Notification %d", id),
                Body:    "Test",
                Recipients: []string{"user@example.com"},
            })

            if err != nil {
                log.Printf("Request %d failed: %v", id, err)
                return
            }

            log.Printf("Request %d succeeded: %s", id, resp.NotificationId)
        }(i)
    }

    wg.Wait()
}
```

## Keepalive Configuration

### Why Keepalive Matters

In Kubernetes, idle connections may be terminated by:
- Load balancers (after 60-600 seconds)
- Network proxies
- Firewalls with connection tracking

**Solution:** Send periodic keepalive pings

### Client-Side Keepalive

```go
grpc.WithKeepaliveParams(keepalive.ClientParameters{
    Time:                10 * time.Second,  // Send ping every 10s of inactivity
    Timeout:             3 * time.Second,   // Wait 3s for ping response
    PermitWithoutStream: true,              // Send pings even when no active RPCs
})
```

### Server-Side Keepalive (Already Configured in Your Server)

Add to `cmd/server/main.go` in `startGRPCServer()`:

```go
serverOpts = append(serverOpts,
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute, // Close idle connections after 15m
        MaxConnectionAge:      30 * time.Minute, // Force close after 30m
        MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5s for RPCs to complete
        Time:                  5 * time.Second,  // Send ping if idle for 5s
        Timeout:               1 * time.Second,  // Wait 1s for ping response
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             5 * time.Second, // Don't allow pings more often than 5s
        PermitWithoutStream: true,            // Allow pings when no streams
    }),
)
```

## Performance Comparison

### Scenario 1: Short-Lived Connections (Creating new connection for each request)

```
Request 1: 15ms (10ms connect + 5ms TLS + 1ms auth + 0.5ms RPC)
Request 2: 15ms (10ms connect + 5ms TLS + 1ms auth + 0.5ms RPC)
Request 3: 15ms (10ms connect + 5ms TLS + 1ms auth + 0.5ms RPC)
...
1000 requests: ~15,000ms (15 seconds)
```

### Scenario 2: Long-Lived Connection (Recommended)

```
Request 1:    15ms (10ms connect + 5ms TLS + 1ms auth + 0.5ms RPC)
Request 2:    1.5ms (1ms auth + 0.5ms RPC)
Request 3:    1.5ms (1ms auth + 0.5ms RPC)
...
1000 requests: ~1,515ms (1.5 seconds)
```

**Performance Improvement: 10x faster! (15s → 1.5s)**

### Scenario 3: Connection Pool with 4 Connections

```
First 4 requests:  15ms each (connection setup)
Remaining 996:     1.5ms each (reuse connections)
1000 requests:     ~1,554ms (1.5 seconds)
Handles concurrent load better
```

## Best Practices

### 1. Connection Lifecycle Management

```go
// Application-scoped client (singleton)
var (
    notifierClient *NotifierClient
    once           sync.Once
)

func GetNotifierClient() *NotifierClient {
    once.Do(func() {
        client, err := NewNotifierClient(
            os.Getenv("NOTIFIER_ADDRESS"),
            os.Getenv("NOTIFIER_API_KEY"),
        )
        if err != nil {
            log.Fatalf("Failed to create notifier client: %v", err)
        }
        notifierClient = client
    })
    return notifierClient
}

// In main():
func main() {
    client := GetNotifierClient()
    defer client.Close()

    // ... run application ...
}
```

### 2. Context with Timeout

Always use context with timeout to prevent hanging requests:

```go
func (nc *NotifierClient) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
    // Set timeout for this request
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Add API key
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+nc.apiKey)

    return nc.client.SendNotification(ctx, req)
}
```

### 3. Health Checks

Periodically verify the connection is healthy:

```go
func (nc *NotifierClient) HealthCheck(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    // Add API key
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+nc.apiKey)

    _, err := nc.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
    return err
}

// In background goroutine:
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := client.HealthCheck(context.Background()); err != nil {
            log.Printf("Health check failed: %v", err)
            // Consider reconnecting or alerting
        }
    }
}()
```

### 4. Graceful Reconnection

Handle connection failures gracefully:

```go
type ResilientNotifierClient struct {
    address string
    apiKey  string
    client  *NotifierClient
    mu      sync.RWMutex
}

func (rnc *ResilientNotifierClient) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
    rnc.mu.RLock()
    client := rnc.client
    rnc.mu.RUnlock()

    resp, err := client.SendNotification(ctx, req)
    if err != nil && isConnectionError(err) {
        // Try to reconnect
        log.Printf("Connection error, attempting reconnect: %v", err)
        if err := rnc.reconnect(); err != nil {
            return nil, fmt.Errorf("reconnection failed: %w", err)
        }

        // Retry request with new connection
        rnc.mu.RLock()
        client = rnc.client
        rnc.mu.RUnlock()

        return client.SendNotification(ctx, req)
    }

    return resp, err
}

func (rnc *ResilientNotifierClient) reconnect() error {
    rnc.mu.Lock()
    defer rnc.mu.Unlock()

    // Close old connection
    if rnc.client != nil {
        rnc.client.Close()
    }

    // Create new connection
    client, err := NewNotifierClient(rnc.address, rnc.apiKey)
    if err != nil {
        return err
    }

    rnc.client = client
    return nil
}
```

## Kubernetes Deployment Considerations

### 1. Service Configuration

Your services are already correctly configured for long-lived connections:

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: notifier-grpc
spec:
  type: ClusterIP  # ✅ Good: Stable internal endpoint
  ports:
  - port: 50051
    targetPort: grpc
    protocol: TCP
```

### 2. Client Connection String

```go
// Within same namespace
client, _ := NewNotifierClient("notifier-grpc:50051", apiKey)

// From different namespace
client, _ := NewNotifierClient("notifier-grpc.default.svc.cluster.local:50051", apiKey)
```

### 3. Load Balancing

Kubernetes service provides **connection-level** load balancing. For better **request-level** load balancing with long-lived connections, consider:

**Option A: Client-Side Load Balancing**

```go
import "google.golang.org/grpc/resolver"

conn, err := grpc.Dial(
    "dns:///notifier-grpc:50051",  // DNS resolver
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

**Option B: Connection Pool** (shown earlier)

## Rate Limiting Considerations

### Your Current Implementation

The rate limiter checks on **every request** (in `grpc_middleware.go:46-50`):

```go
allowed, err := m.store.CheckRateLimit(apiKey)
if err != nil || !allowed {
    return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
}
```

### Impact with Long-Lived Connections

**No negative impact!** Rate limiting works the same:
- Each RPC call is checked independently
- Connection reuse doesn't bypass rate limits
- Rate limit is per API key, not per connection

## Summary

### ✅ DO: Use Long-Lived Connections

```go
// Create once at application startup
client, _ := NewNotifierClient("notifier-grpc:50051", apiKey)
defer client.Close()

// Reuse for all requests
for {
    client.SendNotification(ctx, req)
}
```

**Benefits:**
- 10x faster (eliminates connection setup overhead)
- Lower latency (1.5ms vs 15ms per request)
- Fewer resources (one connection vs. thousands)
- Better throughput (HTTP/2 multiplexing)
- Automatic keepalive prevents timeouts

### ❌ DON'T: Create Connection Per Request

```go
// BAD: Don't do this!
for {
    client, _ := NewNotifierClient("notifier-grpc:50051", apiKey)
    client.SendNotification(ctx, req)
    client.Close()
}
```

**Problems:**
- Slow (15ms per request)
- Wasteful (repeated TCP/TLS handshakes)
- Resource-intensive (thousands of connections)

### Authentication Still Happens Per-Request

- API key sent in metadata with every RPC
- Server validates on every call
- No security trade-off
- Just eliminates connection setup overhead

**You get both: Maximum performance AND full security!**
