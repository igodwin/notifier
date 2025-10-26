# E2E Tests for Notifier Service

This directory contains end-to-end (E2E) tests for the Notifier service using testcontainers for isolated Docker-based testing.

## Overview

The E2E tests validate real-world scenarios using:
- **testcontainers-go**: Isolated containerized service instances
- **REST client library**: Type-safe API client (pkg/client)
- **Real notifications**: Actual in-memory notification processing

## CRITICAL-1 Test Scenarios

The following tests validate the notification retention policy implementation:

### 1. TTL-Based Cleanup (`TestCRITICAL1_TTLBasedCleanup`)
- **Purpose**: Verify old notifications are removed after TTL expires
- **Configuration**: TTL=2s, check frequency=500ms
- **Steps**:
  1. Send notification
  2. Wait for TTL + cleanup interval
  3. Verify notification count decreased
- **Expected Result**: ✅ Old notifications cleaned up

### 2. Max Size Enforcement (`TestCRITICAL1_MaxSizeEnforcement`)
- **Purpose**: Verify max_size limit is enforced
- **Configuration**: max_size=5, TTL=24h
- **Steps**:
  1. Send 10 notifications
  2. Wait for cleanup
  3. Verify only 5 remain
- **Expected Result**: ✅ Excess notifications removed

### 3. Cleanup Disabled (`TestCRITICAL1_CleanupDisabled`)
- **Purpose**: Verify cleanup doesn't run when disabled
- **Configuration**: enabled=false
- **Steps**:
  1. Send 10 notifications
  2. Wait for time cleanup would run
  3. Verify all 10 still exist
- **Expected Result**: ✅ No notifications removed

### 4. Concurrent Sends (`TestCRITICAL1_ConcurrentSends`)
- **Purpose**: Verify concurrent client access is safe
- **Configuration**: Default retention
- **Steps**:
  1. Send 10 notifications concurrently
  2. Verify all succeeded
  3. Check stats
- **Expected Result**: ✅ All 10 notifications processed

### 5. Oldest Removed First (`TestCRITICAL1_OldestRemovedFirst`)
- **Purpose**: Verify oldest are removed when exceeding max_size
- **Configuration**: max_size=3
- **Steps**:
  1. Send 5 notifications with delays
  2. Wait for cleanup
  3. Verify oldest 2 removed
- **Expected Result**: ✅ Newest 3 remain

### 6. Memory Bounded (`TestCRITICAL1_MemoryBounded`)
- **Purpose**: Verify memory stays bounded over time
- **Configuration**: max_size=50, TTL=5s
- **Steps**:
  1. Send 30 notifications in batches
  2. Wait for cleanup between batches
  3. Verify count stays under max_size
- **Expected Result**: ✅ Memory usage stays bounded

### 7. Service Health (`TestCRITICAL1_ServiceHealthy`)
- **Purpose**: Verify service stays responsive during cleanup
- **Configuration**: Default retention
- **Steps**:
  1. Send 20 notifications, check health
  2. Repeat 5 times
  3. Verify health check always succeeds
- **Expected Result**: ✅ Service remains responsive

## Prerequisites

- **Docker**: For running containerized tests
- **Go 1.21+**: For building and running tests
- **Docker daemon**: Must be running

## Running Tests

### All E2E Tests
```bash
go test -v ./tests/e2e -timeout 600s
```

### Single Test
```bash
go test -v ./tests/e2e -timeout 120s -run TestCRITICAL1_TTLBasedCleanup
```

### Skip Long-Running Tests
```bash
go test -v ./tests/e2e -short
```

### With Debug Output
```bash
go test -v ./tests/e2e -timeout 600s -count=1 -race
```

## Test Execution Flow

1. **Build Docker Image**
   - Uses existing Dockerfile to build `notifier:test` image
   - Compiles service with all retention features

2. **Create Container**
   - testcontainers spins up isolated container
   - Exposes port 8080 internally
   - Sets environment variables for retention config

3. **Wait for Readiness**
   - Polls `/health` endpoint until ready (max 30s)
   - Waits for service to fully initialize

4. **Run Test Scenarios**
   - Send notifications via REST client
   - Wait for cleanup to run
   - Verify expected behavior

5. **Cleanup**
   - Stops and removes container
   - Cleans up Docker resources

## Configuration

Each test can customize retention settings via environment variables:

```go
retention := "NOTIFIER_RETENTION_ENABLED=true"
retention += "|NOTIFIER_RETENTION_TTL=2s"
retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
retention += "|NOTIFIER_RETENTION_MAX_SIZE=50"

suite := SetupSuite(t, retention)
```

### Available Settings
- `NOTIFIER_RETENTION_ENABLED`: Turn cleanup on/off
- `NOTIFIER_RETENTION_TTL`: Time-to-live (e.g., "2s", "24h")
- `NOTIFIER_RETENTION_CHECK_FREQUENCY`: Cleanup interval (e.g., "500ms", "1h")
- `NOTIFIER_RETENTION_MAX_SIZE`: Max notifications (e.g., 50, 100000)

## Client Library

Tests use the REST client library in `pkg/client/`:

```go
// Create client
cfg := client.ClientConfig{
    BaseURL: "http://localhost:8080",
    Timeout: 30 * time.Second,
}
c := client.NewRESTClient(cfg)

// Send notification
resp, err := c.Send(ctx, client.NotificationRequest{
    Type: "stdout",
    Subject: "Test",
    Body: "Message",
    Recipients: []string{"test@example.com"},
})

// Get stats
stats, err := c.GetStats(ctx)
```

## Debugging

### View Container Logs
Tests automatically capture logs on failure. Access via:

```go
logs := suite.GetLogs(context.Background())
fmt.Println(logs)
```

### Inspect Running Container
```bash
# Find container ID
docker ps | grep notifier:test

# View logs
docker logs <container-id>

# Connect to container
docker exec -it <container-id> /bin/sh
```

### Common Issues

**Issue**: Container fails to build
```
Error: Failed to build docker image
```
**Solution**: Ensure Dockerfile exists and Docker daemon is running

**Issue**: Port already in use
```
Error: Failed to listen on
```
**Solution**: Stop other services on port 8080 or retry

**Issue**: Timeout waiting for service
```
Error: Service failed to become ready
```
**Solution**: Check Docker logs, may need more CPU/memory

## Performance Expectations

### Cleanup Performance
- Cleanup 5000 notifications: ~1.4ms
- Per-item overhead: <1μs
- No impact on normal operations

### Test Duration
- Single test: 5-10 seconds
- Full suite: 45-60 seconds (running sequentially)
- CI/CD recommendation: Run with `-timeout 600s`

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Run E2E Tests
  run: |
    go test -v ./tests/e2e -timeout 600s
```

### Docker-in-Docker
```yaml
services:
  docker:
    image: docker:dind
    options: --privileged
```

## Test Matrix

| Scenario | TTL | Frequency | Max Size | Expected |
|----------|-----|-----------|----------|----------|
| TTL Cleanup | 2s | 500ms | 10k | Old removed |
| Max Size | 24h | 500ms | 5 | Size capped |
| Disabled | - | - | - | No cleanup |
| Concurrent | 24h | 1s | 1k | All succeed |
| Oldest First | 24h | 500ms | 3 | Newest kept |
| Bounded | 5s | 500ms | 50 | Size bounded |
| Health | 5s | 500ms | 1k | Always healthy |

## Future Enhancements

- [ ] gRPC client library + tests
- [ ] Load testing scenarios (10k+ notifications)
- [ ] Memory profiling validation
- [ ] Performance benchmarking
- [ ] Stress testing (high throughput)
- [ ] Multi-container orchestration tests
- [ ] Kubernetes integration tests

## References

- [testcontainers-go](https://github.com/testcontainers/testcontainers-go)
- [Client Library](../../pkg/client/)
- [CRITICAL-1 Implementation](../../docs/CRITICAL_1_IMPLEMENTATION.md)
- [Retention Configuration](../../docs/CRITICAL_1_QUICK_START.md)
