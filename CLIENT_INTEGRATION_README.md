# Client Integration Guide - Notifier Service

**Quick links for different use cases:**

## 📚 Documentation Index

### For Developers Building New Clients

1. **Start Here:** [`docs/QUICK_START_CLIENT.md`](docs/QUICK_START_CLIENT.md)
   - 5-minute quick start guide
   - Complete minimal working example
   - Essential requirements checklist

2. **AI Assistant Prompt:** [`docs/CLIENT_INTEGRATION_PROMPT.md`](docs/CLIENT_INTEGRATION_PROMPT.md)
   - Copy-paste prompt for Claude/ChatGPT/etc.
   - Generates production-ready client code
   - Includes all requirements and best practices

3. **Performance Deep Dive:** [`docs/GRPC_CONNECTION_OPTIMIZATION.md`](docs/GRPC_CONNECTION_OPTIMIZATION.md)
   - Why long-lived connections matter (10x faster!)
   - Connection pooling strategies
   - Keepalive configuration
   - Resilience patterns

4. **Best Practices:** [`docs/CLIENT_RECOMMENDATIONS.md`](docs/CLIENT_RECOMMENDATIONS.md)
   - Advanced patterns
   - Security hardening
   - Observability and monitoring
   - Testing strategies

### For Kubernetes Deployments

5. **CORS and Kubernetes:** [`docs/CORS_KUBERNETES_GUIDE.md`](docs/CORS_KUBERNETES_GUIDE.md)
   - Do you need CORS? (Spoiler: Not for gRPC!)
   - Service-to-service communication
   - When CORS matters (web frontends)

6. **CORS Implementation:** [`docs/CORS_IMPLEMENTATION.md`](docs/CORS_IMPLEMENTATION.md)
   - Technical details of CORS security
   - Configuration examples
   - Migration guide

### For Authentication & Security

7. **Authentication Guide:** [`docs/AUTH.md`](docs/AUTH.md)
   - Complete auth system documentation
   - API key management
   - Role-based access control (RBAC)

## 🎯 Quick Decision Tree

### "I need to build a new client application"

**Option A: Using AI Assistant (Recommended)**
→ Use [`CLIENT_INTEGRATION_PROMPT.md`](docs/CLIENT_INTEGRATION_PROMPT.md)
→ Paste the prompt into Claude/ChatGPT
→ Review generated code against checklist below

**Option B: Manual Implementation**
→ Start with [`QUICK_START_CLIENT.md`](docs/QUICK_START_CLIENT.md)
→ Copy the complete example
→ Customize for your needs
→ Reference [`GRPC_CONNECTION_OPTIMIZATION.md`](docs/GRPC_CONNECTION_OPTIMIZATION.md) for performance

### "I'm deploying to Kubernetes"

→ Read [`CORS_KUBERNETES_GUIDE.md`](docs/CORS_KUBERNETES_GUIDE.md) first
→ No CORS needed for backend gRPC communication! ✅
→ Configure Kubernetes Secret for API key
→ Use service DNS: `notifier-grpc:50051`

### "My client is too slow"

→ Check [`GRPC_CONNECTION_OPTIMIZATION.md`](docs/GRPC_CONNECTION_OPTIMIZATION.md)
→ Verify you're using long-lived connections (not creating per request)
→ Add keepalive configuration
→ Consider connection pooling for high throughput

### "Authentication isn't working"

→ See [`AUTH.md`](docs/AUTH.md) for troubleshooting
→ Verify API key format: `nk_` + 64 hex characters
→ Check you're sending it in metadata with every request
→ Confirm key is active and has required roles

## ✅ Client Implementation Checklist

Use this checklist to verify your client implementation:

### Connection Management
- [ ] Using single long-lived connection (created at startup)
- [ ] Connection reused for all requests (not creating per request)
- [ ] Keepalive configured (10s ping, 3s timeout)
- [ ] Connection closed on application shutdown
- [ ] Health checks implemented (every 30-60s)

### Authentication
- [ ] API key loaded from environment variable (`NOTIFIER_API_KEY`)
- [ ] API key sent in metadata with every request
- [ ] Using correct format: `authorization: Bearer nk_...`
- [ ] No hardcoded credentials in source code
- [ ] API key masked in logs (never logged in full)

### Error Handling
- [ ] Rate limit errors retry with exponential backoff
- [ ] Authentication errors logged and not retried
- [ ] Invalid argument errors logged and not retried
- [ ] Unavailable errors retry with backoff
- [ ] Maximum retry limit enforced (3-5 attempts)

### Context & Timeouts
- [ ] Every request uses context with timeout (5-30s)
- [ ] Contexts properly cancelled with defer
- [ ] Deadline exceeded errors handled gracefully

### Security
- [ ] Credentials in Kubernetes Secret (not ConfigMap)
- [ ] No credentials in Docker image layers
- [ ] `.gitignore` excludes credential files
- [ ] No sensitive data in logs or metrics labels

### Observability
- [ ] Connection establishment logged (info level)
- [ ] Errors logged with appropriate levels
- [ ] Success responses logged (debug level)
- [ ] Metrics instrumented (if using Prometheus)
- [ ] Distributed tracing integrated (if using OpenTelemetry)

### Testing
- [ ] Unit tests with mocked client
- [ ] Retry logic tested
- [ ] Error handling tested
- [ ] Integration tests (if service available)

### Production Readiness
- [ ] Graceful shutdown implemented
- [ ] Circuit breaker pattern (optional, for resilience)
- [ ] Notification failures don't crash application
- [ ] Health check endpoint exposed (if needed)

## 🔐 API Key Management

### Getting an API Key

Contact your Notifier service administrator to create a key:

```bash
# Using the notifier CLI (if available)
notifier-cli keys create \
  --client-id "my-application" \
  --roles "email,slack" \
  --rate-limit 1000

# Output:
# Created API key: nk_a1b2c3d4e5f6...
# Save this key securely - it won't be shown again!
```

### API Key Format

```
nk_<64 hex characters>

Example:
nk_a1b2c3d4e5f6789012345678901234567890123456789012345678901234
│  └─────────────────────────────────────────────────────────┘
│                            64 characters
└── Prefix (always "nk_")
```

### Storing API Keys

**Development:**
```bash
export NOTIFIER_API_KEY="nk_your_dev_key_here"
```

**Production (Kubernetes):**
```yaml
# 1. Create secret
kubectl create secret generic notifier-credentials \
  --from-literal=api-key=nk_your_prod_key_here

# 2. Reference in deployment
env:
- name: NOTIFIER_API_KEY
  valueFrom:
    secretKeyRef:
      name: notifier-credentials
      key: api-key
```

### Rate Limits

Default: **100 requests per minute** (configurable per key)

When rate limit is exceeded:
- Error code: `codes.ResourceExhausted`
- Retry after: 60 seconds (check `Retry-After` header)
- Strategy: Implement exponential backoff

To request higher rate limit, contact your administrator.

## 🚀 Example Implementations

### Minimal Example (50 lines)

See [`QUICK_START_CLIENT.md`](docs/QUICK_START_CLIENT.md) for complete code.

### Production-Ready Example

Use the AI assistant prompt in [`CLIENT_INTEGRATION_PROMPT.md`](docs/CLIENT_INTEGRATION_PROMPT.md) to generate a complete implementation with:
- Connection pooling
- Retry logic
- Health checks
- Metrics
- Graceful shutdown
- Unit tests

### Language-Specific Examples

**Go:** See `QUICK_START_CLIENT.md` (complete example included)

**Python/Java/Node.js:** Use the prompt in `CLIENT_INTEGRATION_PROMPT.md` with your AI assistant and specify your language:

```
[Include the prompt from CLIENT_INTEGRATION_PROMPT.md]

Please implement this in Python using the grpcio library.
```

## 🔧 Troubleshooting

### Common Issues and Solutions

| Issue | Symptom | Solution |
|-------|---------|----------|
| **Slow performance** | 10-15ms per request | Verify using long-lived connection, not creating new connection per request |
| **Authentication failed** | `codes.Unauthenticated` | Check API key format, confirm key is active, verify metadata header |
| **Rate limit exceeded** | `codes.ResourceExhausted` | Implement exponential backoff retry, consider batching requests |
| **Connection timeout** | Requests hang indefinitely | Add keepalive configuration, set context timeout |
| **Connection dropped** | Errors after idle period | Configure keepalive: 10s ping interval |
| **Health checks failing** | Service appears down | Check network connectivity, verify service is running, check DNS resolution |

### Debug Checklist

```bash
# 1. Verify service is running
kubectl get pods -l app=notifier
kubectl get svc notifier-grpc

# 2. Check service is reachable
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup notifier-grpc

# 3. Test gRPC connectivity (if grpcurl installed)
grpcurl -plaintext notifier-grpc:50051 list

# 4. Verify API key
echo $NOTIFIER_API_KEY | wc -c  # Should be 67 (nk_ + 64 chars + newline)

# 5. Check application logs
kubectl logs -l app=your-app --tail=100
```

## 📊 Performance Expectations

**With Long-Lived Connection (Recommended):**
- First request: ~15ms (connection setup + auth)
- Subsequent requests: ~1-2ms (auth only)
- Throughput: ~500-1000 req/s per connection
- Latency p99: <5ms

**Without Connection Reuse (Bad):**
- Every request: ~15ms (repeated connection setup)
- Throughput: ~60 req/s per connection
- Latency p99: ~20ms

**Connection Pool (High Throughput):**
- 4 connections: ~2000-4000 req/s
- Linear scaling with connection count
- Use for >1000 req/s sustained load

## 🎓 Learn More

- **gRPC Documentation:** https://grpc.io/docs/
- **Protocol Buffers:** https://protobuf.dev/
- **gRPC Best Practices:** https://grpc.io/docs/guides/performance/
- **Kubernetes Secrets:** https://kubernetes.io/docs/concepts/configuration/secret/

## 📞 Support

**Questions about:**
- API keys, rate limits, permissions → Contact Notifier admin
- Client implementation, best practices → See documentation above
- Service issues, downtime → Check service health dashboard
- Feature requests → Open issue in Notifier repository

## 📝 Quick Reference Card

```
Service:     notifier-grpc:50051
Protocol:    gRPC (HTTP/2)
Auth:        authorization: Bearer nk_<64-hex>
Rate Limit:  100 req/min (default)
Timeout:     10s recommended
Keepalive:   10s ping, 3s timeout
Connection:  Long-lived (reuse)
```

**Essential imports (Go):**
```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"
    "google.golang.org/grpc/metadata"
    pb "github.com/igodwin/notifier/api/grpc/pb"
)
```

**Minimal client setup:**
```go
// 1. Create connection with keepalive
conn, _ := grpc.Dial(address, grpc.WithKeepaliveParams(...))

// 2. Create client
client := pb.NewNotifierServiceClient(conn)

// 3. Add auth to context
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+apiKey)

// 4. Make request
resp, err := client.SendNotification(ctx, req)
```

That's it! You're ready to integrate with the Notifier service. 🚀
