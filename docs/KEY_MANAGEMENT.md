# API Key Management Guide

This guide explains how to generate, manage, and use API keys with the Notifier service. The system uses a hybrid approach combining an in-memory cache for performance with PostgreSQL for persistence.

## Overview

The API key management system provides:

- **Secure Key Generation**: Cryptographically random 32-byte keys with `nk_` prefix
- **Persistent Storage**: PostgreSQL backend with full audit trail
- **Fast Lookups**: In-memory cache with write-through consistency
- **Rate Limiting**: Per-key request rate limits (configurable per minute)
- **Expiration**: Optional key expiration dates
- **Key Rotation**: Ability to revoke and create new keys
- **Audit Logging**: Track who created/revoked keys and when
- **Role-Based Access**: Control which notifiers each key can access

## Architecture

### Hybrid Cache Strategy

The system uses a write-through hybrid approach:

```
Request Flow:
  1. Check in-memory cache (O(1) lookup) ← Fast path
  2. If hit and valid, use immediately
  3. If miss, create from database (fallback)
  4. Update cache and return

Write Flow:
  1. Write to PostgreSQL database first
  2. If successful, update in-memory cache
  3. If DB fails, cache is not updated (consistency)
```

**Benefits**:
- Fast authentication checks (cache lookup in microseconds)
- Persistent storage for durability
- Consistent state across restarts
- Multi-instance support (all instances read from same DB)

## Setup

### Prerequisites

- PostgreSQL 12+ database
- Network access from notifier to PostgreSQL

### Configuration

Add database configuration to `config.yaml`:

```yaml
auth:
  enabled: true
  default_rate_limit: 100  # requests per minute
  database:
    url: "postgresql://user:password@localhost:5432/notifier"
    # Or use environment variable: NOTIFIER_AUTH_DATABASE_URL
```

**Environment Variable**:
```bash
export NOTIFIER_AUTH_DATABASE_URL="postgresql://user:password@localhost:5432/notifier"
export NOTIFIER_AUTH_ENABLED=true
export NOTIFIER_AUTH_DEFAULT_RATE_LIMIT=100
```

### Database Setup

The schema is automatically created on first startup:

```sql
-- Tables created automatically:
-- api_keys: Stores API key metadata
-- api_key_audit_log: Tracks all key operations
```

To manually initialize the database:

```bash
psql postgresql://user:password@localhost:5432/notifier < schema.sql
```

## Bootstrap: Creating Initial Admin Key

On first deployment, you need to create an initial admin key to bootstrap the system.

### Option 1: Environment Variable (Recommended for CI/CD)

```bash
export NOTIFIER_BOOTSTRAP_ADMIN_KEY=true
./notifier serve
```

The service will:
1. Check if bootstrap has already been done
2. Create a random admin key with all permissions
3. Save it to `./notifier-admin-key.txt`
4. Print to stdout (make sure to capture and secure this!)

**Output**:
```
============================================================
NOTIFIER BOOTSTRAP: ADMIN KEY CREATED
============================================================
Key: nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
...
============================================================
```

### Option 2: Programmatic (Docker/Kubernetes)

In your startup script:

```go
import "github.com/igodwin/notifier/internal/auth"

bootstrapCfg := &auth.BootstrapConfig{
    Enabled:          true,
    AdminKeyFileName: "/var/run/notifier/admin-key.txt",
    PrintToStdout:    false,
}

adminKey, err := auth.BootstrapAdminKey(ctx, keyStore, bootstrapCfg, logger)
if err != nil && err.Error() != "bootstrap already completed" {
    logger.Fatalf("Bootstrap failed: %v", err)
}
```

### Option 3: Docker Environment

```dockerfile
FROM notifier:latest

ENV NOTIFIER_AUTH_ENABLED=true
ENV NOTIFIER_BOOTSTRAP_ADMIN_KEY=true
ENV NOTIFIER_AUTH_DATABASE_URL=postgresql://user:pass@db:5432/notifier

ENTRYPOINT ["/app/notifier", "serve"]
```

**Capture the key**:
```bash
docker logs <container-id> | grep "Key:"
```

### Option 4: Kubernetes

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: notifier-bootstrap
spec:
  template:
    spec:
      containers:
      - name: notifier
        image: notifier:latest
        env:
        - name: NOTIFIER_AUTH_ENABLED
          value: "true"
        - name: NOTIFIER_BOOTSTRAP_ADMIN_KEY
          value: "true"
        - name: NOTIFIER_AUTH_DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: notifier-db-secret
              key: url
        volumeMounts:
        - name: keys
          mountPath: /var/run/notifier
      volumes:
      - name: keys
        secret:
          secretName: notifier-keys
      restartPolicy: Never
```

After the job completes, extract the key from the secret or logs.

## Managing API Keys

### Creating New Keys

Use the admin key to create additional keys via REST API:

```bash
# Create a key for sending emails only
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer nk_admin_key_here" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app-email",
    "roles": ["notify-email"],
    "rate_limit": 1000,
    "expires_in": "8760h"
  }'
```

**Response**:
```json
{
  "key": "nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "name": "my-app-email-1698297600",
  "client_id": "my-app-email",
  "roles": ["notify-email"],
  "created_at": "2024-10-26T12:00:00Z",
  "expires_at": "2025-10-26T12:00:00Z",
  "rate_limit": 1000
}
```

### Listing Keys

List all keys for your client:

```bash
curl -X GET http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer nk_your_key"
```

List keys for specific client (admin only):

```bash
curl -X GET "http://localhost:8080/api/v1/admin/keys?client_id=other-app" \
  -H "Authorization: Bearer nk_admin_key"
```

### Revoking Keys

Disable a key (cannot be undone, but you can create a new one):

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/keys/nk_key_to_revoke \
  -H "Authorization: Bearer nk_admin_key"
```

### Rotating Keys

Create a new key with same permissions, then revoke the old one:

```bash
# 1. Create new key with same roles
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer nk_admin_key" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app",
    "roles": ["notify-email", "notify-slack"],
    "rate_limit": 1000
  }'

# 2. Update your application to use the new key

# 3. Verify everything works

# 4. Revoke the old key
curl -X DELETE http://localhost:8080/api/v1/admin/keys/nk_old_key \
  -H "Authorization: Bearer nk_admin_key"
```

### Viewing Audit Log

See who created/revoked a key and when:

```bash
curl -X GET "http://localhost:8080/api/v1/admin/keys/nk_key/audit?limit=50" \
  -H "Authorization: Bearer nk_admin_key"
```

**Response**:
```json
{
  "key_preview": "nk_o5p6",
  "audit_log": [
    {
      "action": "created",
      "performed_by": "admin-bootstrap",
      "performed_at": "2024-10-26T12:00:00Z",
      "details": {
        "client_id": "my-app",
        "roles": ["notify-email"]
      }
    },
    {
      "action": "deactivated",
      "performed_by": "admin-user",
      "performed_at": "2024-10-26T14:30:00Z",
      "details": null
    }
  ]
}
```

## Using API Keys

### With REST API

Include the key in the `Authorization: Bearer` header:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer nk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Hello",
    "body": "World",
    "recipients": ["user@example.com"]
  }'
```

Or use the `X-API-Key` header:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "X-API-Key: nk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Hello",
    "body": "World",
    "recipients": ["user@example.com"]
  }'
```

### With gRPC

Include the key in gRPC metadata:

**Go Client**:
```go
import "google.golang.org/grpc/metadata"

ctx := metadata.AppendToOutgoingContext(context.Background(),
    "authorization", "Bearer nk_your_api_key")

client := pb.NewNotificationServiceClient(conn)
resp, err := client.SendNotification(ctx, &pb.SendNotificationRequest{
    // ...
})
```

**Python Client**:
```python
import grpc

metadata = [('authorization', 'Bearer nk_your_api_key')]
channel = grpc.secure_channel('localhost:50051', grpc.ssl_channel_credentials())
client = pb.NotificationServiceStub(channel)
response = client.SendNotification(request, metadata=metadata)
```

## Key Naming and Organization

### Naming Convention

Keys are generated with format: `nk_<32-hex-chars>`

The system auto-generates a name based on client_id and timestamp:
```
my-app-email-1698297600
my-slack-integration-1698297700
```

You can customize via the `name` field when creating keys.

### Recommended Organization

**By Application**:
```
client_id: api-gateway
client_id: worker-service
client_id: monitoring-system
```

**By Permission**:
```
notify-email: Email notifications only
notify-slack: Slack notifications only
notify-all: All notification types
admin: Key management + all notifications
```

**By Environment**:
```
my-app-prod-email
my-app-staging-email
my-app-dev-email
```

## Security Best Practices

### Do's

✅ **Rotate keys regularly** - Create new keys every 90 days, revoke old ones

✅ **Use unique keys per service** - Don't share keys between different apps

✅ **Limit permissions** - Only grant roles needed (e.g., `notify-email` not `admin`)

✅ **Use reasonable rate limits** - Prevent accidental DoS from misconfiguration

✅ **Store in secure vaults** - Use Kubernetes Secrets, AWS Secrets Manager, HashiCorp Vault

✅ **Set expiration dates** - Keys should expire after a period

✅ **Monitor audit logs** - Regularly check who's creating/revoking keys

✅ **Use short-lived keys for CI/CD** - Rotate automatically during deployments

### Don'ts

❌ **Don't commit keys to version control** - Even private repos

❌ **Don't use admin key in production** - Create limited permission keys

❌ **Don't share keys between teams** - Each team/app gets its own

❌ **Don't set unlimited rate limits** - Prevents accidental overload

❌ **Don't use expired keys** - Revoke and create new ones

❌ **Don't store plaintext in logs** - Only last 4 characters should be visible

## Rate Limiting

Each API key has a configurable rate limit (requests per minute):

```bash
# Create key with 1000 req/min limit
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer nk_admin_key" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app",
    "roles": ["notify-email"],
    "rate_limit": 1000
  }'
```

**Rate Limit Errors**:
- Exceeding limit returns HTTP 429 Too Many Requests
- Limit resets every minute
- Set `rate_limit: 0` for unlimited (not recommended)

### Recommended Limits

- Admin/Testing: 10,000 req/min
- Production Email: 1,000-5,000 req/min
- Production Slack: 500-1,000 req/min
- Production Ntfy: 500-1,000 req/min
- Development: 100-500 req/min

## Expiration Dates

Keys can optionally expire:

```bash
# Create key that expires in 24 hours
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer nk_admin_key" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app",
    "roles": ["notify-email"],
    "expires_in": "24h"
  }'

# Expires in 30 days
"expires_in": "720h"

# Expires in 1 year
"expires_in": "8760h"
```

**Duration Format**: Go duration format
- `s` - seconds (30s)
- `m` - minutes (30m)
- `h` - hours (24h)
- `d` - days (not supported, use 24h instead)

Expired keys are automatically filtered when loading cache at startup.

## Troubleshooting

### Key Creation Fails

**Error**: `Failed to create API key`

**Causes**:
- Database connection issue
- PostgreSQL not running
- Network connectivity problem

**Solution**:
```bash
# Test PostgreSQL connection
psql postgresql://user:password@localhost:5432/notifier -c "SELECT 1"
```

### Authentication Fails

**Error**: `401 Unauthorized` or `API key not found`

**Causes**:
- Wrong key format
- Key is revoked/expired
- Key not in cache (distributed setup issue)

**Solutions**:
```bash
# Verify key format
echo $API_KEY | grep "^nk_"

# List your keys
curl -X GET http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $API_KEY"

# Check if key is active
curl -X GET "http://localhost:8080/api/v1/admin/keys?client_id=your-app" \
  -H "Authorization: Bearer $ADMIN_KEY"
```

### Rate Limit Exceeded

**Error**: `429 Too Many Requests`

**Causes**:
- Key rate limit exceeded in current minute
- Misconfiguration sending too many requests

**Solutions**:
- Wait a minute for limit window to reset
- Check your request volume
- Increase rate limit for the key:
  ```bash
  # Create new key with higher limit
  curl -X POST http://localhost:8080/api/v1/admin/keys \
    -H "Authorization: Bearer nk_admin_key" \
    -H "Content-Type: application/json" \
    -d '{
      "client_id": "my-app",
      "roles": ["notify-email"],
      "rate_limit": 5000
    }'
  ```

### Bootstrap Key Lost

**If you lose the bootstrap admin key**:

1. Create a new database admin user with direct SQL access
2. Insert a new admin key into the database:
   ```sql
   INSERT INTO api_keys (
     key, name, client_id, roles, is_active, rate_limit, created_by
   ) VALUES (
     'nk_your_new_key_here',
     'recovery-admin',
     'admin-recovery',
     ARRAY['admin', 'notify-email', 'notify-slack', 'notify-ntfy'],
     true,
     0,
     'system-recovery'
   );
   ```

## API Reference

### POST /api/v1/admin/keys

Create a new API key.

**Headers**:
- `Authorization: Bearer <admin-key>` (required, admin role)
- `Content-Type: application/json`

**Request Body**:
```json
{
  "client_id": "string",          // Required: client identifier
  "roles": ["string"],            // Required: array of role names
  "rate_limit": 100,              // Optional: requests per minute
  "expires_in": "24h"             // Optional: expiration duration
}
```

**Response**: 201 Created
```json
{
  "key": "string",
  "name": "string",
  "client_id": "string",
  "roles": ["string"],
  "created_at": "RFC3339",
  "expires_at": "RFC3339",
  "rate_limit": 100
}
```

### GET /api/v1/admin/keys

List API keys.

**Headers**:
- `Authorization: Bearer <key>` (required)

**Query Parameters**:
- `client_id`: Filter by client (requires admin role if different from authenticated client)

**Response**: 200 OK
```json
{
  "keys": [
    {
      "key_preview": "nk_xxxx",
      "name": "string",
      "client_id": "string",
      "roles": ["string"],
      "created_at": "RFC3339",
      "last_used_at": "RFC3339",
      "expires_at": "RFC3339",
      "is_active": true,
      "rate_limit": 100
    }
  ]
}
```

### DELETE /api/v1/admin/keys/{key}

Revoke an API key.

**Headers**:
- `Authorization: Bearer <admin-key>` (required, admin role)

**Request Body** (optional):
```json
{
  "reason": "Compromised key"
}
```

**Response**: 204 No Content

### GET /api/v1/admin/keys/{key}/audit

Get audit log for a key.

**Headers**:
- `Authorization: Bearer <admin-key>` (required, admin role)

**Query Parameters**:
- `limit`: Number of log entries to return (default: 100, max: 1000)

**Response**: 200 OK
```json
{
  "key_preview": "nk_xxxx",
  "audit_log": [
    {
      "action": "created|deactivated",
      "performed_by": "string",
      "performed_at": "RFC3339",
      "details": {}
    }
  ]
}
```

## Complete Example

### Step 1: Bootstrap

```bash
export NOTIFIER_BOOTSTRAP_ADMIN_KEY=true
export NOTIFIER_AUTH_ENABLED=true
export NOTIFIER_AUTH_DATABASE_URL="postgresql://user:pass@localhost:5432/notifier"

./notifier serve

# Output:
# NOTIFIER BOOTSTRAP: ADMIN KEY CREATED
# Key: nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

### Step 2: Save Admin Key

```bash
export ADMIN_KEY="nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
echo $ADMIN_KEY > ~/.notifier-admin-key
chmod 600 ~/.notifier-admin-key
```

### Step 3: Create Service Keys

```bash
# Email service key
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app-email",
    "roles": ["notify-email"],
    "rate_limit": 1000,
    "expires_in": "8760h"
  }' | jq -r '.key' > ~/.notifier-email-key

# Slack service key
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app-slack",
    "roles": ["notify-slack"],
    "rate_limit": 500
  }' | jq -r '.key' > ~/.notifier-slack-key
```

### Step 4: Use Keys in Application

```bash
export NOTIFIER_EMAIL_KEY=$(cat ~/.notifier-email-key)
export NOTIFIER_SLACK_KEY=$(cat ~/.notifier-slack-key)

# Send email
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $NOTIFIER_EMAIL_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Hello",
    "body": "World",
    "recipients": ["user@example.com"]
  }'

# Send Slack
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $NOTIFIER_SLACK_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "slack",
    "subject": "Hello",
    "body": "World",
    "recipients": ["#general"]
  }'
```

## References

- [API Authentication Guide](./AUTH.md)
- [REST API Documentation](./REST_API.md)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
