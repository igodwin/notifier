# CORS Configuration for Kubernetes Deployments

## Important: CORS Only Affects REST API, Not gRPC

**Key Point:** CORS (Cross-Origin Resource Sharing) is a **browser security mechanism** that only applies to HTTP/REST APIs accessed from web browsers. It does **NOT** affect gRPC communication.

## Service-to-Service Communication (gRPC)

### ✅ No CORS Configuration Needed

For services running in the same Kubernetes cluster communicating via gRPC:

**You don't need to configure CORS at all!**

Here's why:

1. **gRPC uses HTTP/2** - CORS is not enforced
2. **Server-to-Server** - CORS only applies to browser-based requests
3. **No Origin header** - Backend services don't send Origin headers

### gRPC Client Connection Example

```go
// Service A connecting to notifier service via gRPC
conn, err := grpc.Dial("notifier-grpc.default.svc.cluster.local:50051",
    grpc.WithInsecure())
if err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer conn.Close()

client := pb.NewNotifierServiceClient(conn)
```

**No CORS configuration required** ✅

The gRPC service runs on port `50051` (as defined in your k8s/service.yaml) and is accessible at:
- From same namespace: `notifier-grpc:50051`
- From other namespace: `notifier-grpc.default.svc.cluster.local:50051`
- Full DNS: `notifier-grpc.default.svc.cluster.local:50051`

## When You DO Need CORS Configuration

CORS configuration is **only** needed when:

1. **Web browsers** access the REST API (port 8080)
2. The web app is served from a **different origin** than the API
3. The request goes through the **REST API**, not gRPC

### Common Scenarios

| Scenario | Needs CORS? | Reason |
|----------|-------------|---------|
| Backend service → gRPC API | ❌ No | Not a browser, uses gRPC |
| Backend service → REST API | ❌ No | Not a browser |
| Web app → gRPC (grpc-web) | ✅ Yes | Browser-based, needs CORS |
| Web app → REST API | ✅ Yes | Browser-based, needs CORS |
| CLI tool → REST API | ❌ No | Not a browser |
| Postman/curl → REST API | ❌ No | Not a browser |

## CORS Configuration for Web Applications

If you have a web frontend that calls the REST API, you need to configure CORS.

### Scenario 1: Frontend in Same Cluster

**Example:** Frontend at `https://app.example.com`, API at `https://notifier.example.com`

Update your ConfigMap (k8s/configmap.yaml):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: notifier-config
data:
  config.yaml: |
    # ... existing config ...

    cors:
      allowed_origins:
        - "https://app.example.com"  # Your frontend domain
      allowed_methods:
        - "GET"
        - "POST"
        - "OPTIONS"
        - "DELETE"
      allowed_headers:
        - "Content-Type"
        - "Authorization"
      allow_credentials: true  # If your frontend sends auth tokens
      max_age: 3600
```

### Scenario 2: Multiple Frontends

```yaml
cors:
  allowed_origins:
    - "https://app.example.com"          # Main app
    - "https://dashboard.example.com"    # Admin dashboard
    - "https://mobile.example.com"       # Mobile web app
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

### Scenario 3: Development Environment

For local development (frontend at http://localhost:3000):

```yaml
cors:
  allowed_origins:
    - "http://localhost:3000"    # React/Next.js dev server
    - "http://localhost:8080"    # Vue/Angular dev server
    - "http://localhost:5173"    # Vite dev server
    - "https://app.example.com"  # Production frontend
  allowed_methods:
    - "GET"
    - "POST"
    - "OPTIONS"
    - "DELETE"
  allowed_headers:
    - "Content-Type"
    - "Authorization"
  allow_credentials: false
  max_age: 3600
```

### Scenario 4: No Web Frontend (Backend Services Only)

If you only have backend services using gRPC:

```yaml
cors:
  # Empty or minimal config - no origins needed
  allowed_origins: []  # No browser clients
  allowed_methods:
    - "GET"
    - "POST"
  allowed_headers:
    - "Content-Type"
```

The REST API will still work for non-browser clients (like curl, Postman, or backend HTTP clients), but browsers will be blocked unless their origin is whitelisted.

## Complete Kubernetes Configuration Example

Here's your updated k8s/configmap.yaml with CORS:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: notifier-config
  labels:
    app: notifier
data:
  config.yaml: |
    server:
      grpc_port: 50051
      rest_port: 8080
      host: "0.0.0.0"
      mode: "both"

    queue:
      type: "local"
      max_size: 10000
      worker_count: 10
      retry_attempts: 3
      retry_backoff: "exponential"
      local:
        buffer_size: 1000
        persist_to_disk: false

    notifiers:
      stdout: true

    logging:
      level: "info"
      format: "json"
      output_path: "stdout"

    metrics:
      enabled: true
      port: 9090
      path: "/metrics"
      prometheus_enabled: true

    health_check:
      enabled: true
      port: 8081
      path: "/health"
      interval: 30

    # CORS configuration for REST API
    # Only needed if web browsers will access the REST API
    cors:
      # Add your frontend domains here
      # For backend-only services, leave empty
      allowed_origins: []

      # Or if you have a web frontend:
      # allowed_origins:
      #   - "https://app.example.com"

      allowed_methods:
        - "GET"
        - "POST"
        - "OPTIONS"
        - "DELETE"
      allowed_headers:
        - "Content-Type"
        - "Authorization"
      allow_credentials: false
      max_age: 3600
```

## Environment-Specific Configuration

You can also use environment variables for different environments:

### Development ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: notifier-config-dev
  namespace: development
data:
  NOTIFIER_CORS_ALLOWED_ORIGINS: "http://localhost:3000,http://localhost:8080"
  NOTIFIER_CORS_ALLOW_CREDENTIALS: "false"
```

### Production ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: notifier-config-prod
  namespace: production
data:
  NOTIFIER_CORS_ALLOWED_ORIGINS: "https://app.example.com,https://dashboard.example.com"
  NOTIFIER_CORS_ALLOW_CREDENTIALS: "true"
```

## Troubleshooting

### "CORS error" in Browser Console

**Symptom:**
```
Access to fetch at 'https://notifier.example.com/api/v1/notifications'
from origin 'https://app.example.com' has been blocked by CORS policy
```

**Solution:**
Add `https://app.example.com` to `allowed_origins` in your ConfigMap.

### Backend Service Can't Connect

**Symptom:**
```go
// Service in cluster trying to connect
conn, err := grpc.Dial("notifier-grpc:50051", grpc.WithInsecure())
// Error: connection refused
```

**Solution:**
This is NOT a CORS issue. Check:
1. Service name is correct (`notifier-grpc`)
2. Port is correct (`50051`)
3. Service is running: `kubectl get pods -l app=notifier`
4. Service endpoints: `kubectl get endpoints notifier-grpc`

### REST API Returns 200 but No CORS Headers

**Symptom:**
Browser blocks the response even though the API returns 200 OK.

**Solution:**
The origin is not in the whitelist. Check:
1. Origin is exactly matching (including protocol and port)
2. ConfigMap has been updated
3. Pod has been restarted to pick up new config:
   ```bash
   kubectl rollout restart deployment notifier
   ```

## Quick Reference

### For Backend Services Only (Recommended)
```yaml
cors:
  allowed_origins: []  # Empty - no browser access needed
```

### For Web Frontend + Backend Services
```yaml
cors:
  allowed_origins:
    - "https://your-frontend-domain.com"
  allow_credentials: true  # If using auth tokens
```

### Applying Changes

After updating the ConfigMap:

```bash
# Update ConfigMap
kubectl apply -f k8s/configmap.yaml

# Restart pods to pick up new config
kubectl rollout restart deployment notifier

# Verify pods are running
kubectl get pods -l app=notifier

# Check logs for CORS configuration
kubectl logs -l app=notifier | grep CORS
```

You should see:
```
CORS enabled for origins: [https://app.example.com]
```

Or if no origins configured:
```
CORS has no allowed origins configured - all cross-origin requests will be blocked
```

## Summary

**For your use case (services in Kubernetes using gRPC):**

✅ **You don't need to configure CORS at all!**

CORS only applies to web browsers accessing the REST API. Your backend services communicating via gRPC are completely unaffected by CORS configuration.

**Only add CORS configuration if:**
- You have a web frontend (React, Vue, Angular, etc.)
- That web frontend calls the REST API (not gRPC)
- The frontend is served from a different domain than the API

For purely backend service-to-service communication in Kubernetes, CORS is irrelevant.
