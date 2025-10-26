# Authentication & Authorization Guide

This guide explains how to use the authentication and authorization features in the Notifier service.

## Overview

The Notifier service includes:
- **API Key Authentication**: Simple token-based authentication using Bearer tokens or API keys
- **Role-Based Access Control (RBAC)**: Fine-grained authorization for specific notifiers
- **Rate Limiting**: Per-key request rate limiting to prevent abuse
- **Audit Logging**: All auth failures and API key usage are logged

## Enabling Authentication

Authentication is **disabled by default**. To enable it, set in your configuration file:

```yaml
auth:
  enabled: true
  default_rate_limit: 100  # requests per minute, 0 = unlimited
```

Or via environment variable:

```bash
NOTIFIER_AUTH_ENABLED=true
NOTIFIER_AUTH_DEFAULT_RATE_LIMIT=100
```

## Creating API Keys

API keys can be created programmatically. Here's an example:

```go
package main

import (
	"fmt"
	"time"
	"github.com/igodwin/notifier/internal/auth"
)

func main() {
	// Create a new key store
	store := auth.NewAPIKeyStore()

	// Create an API key for a client
	// Parameters: clientID, roles, rateLimit (req/min), expiresIn (optional)
	expiresIn := 30 * 24 * time.Hour  // 30 days
	key, err := store.CreateKey(
		"billing-service",                    // Client ID
		[]string{"notify-email", "notify-slack"}, // Roles
		100,                                  // Rate limit: 100 requests/minute
		&expiresIn,                          // Expires in 30 days
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("API Key: %s\n", key.Key)
	fmt.Printf("Client ID: %s\n", key.ClientID)
	fmt.Printf("Roles: %v\n", key.Roles)
	fmt.Printf("Rate Limit: %d req/min\n", key.RateLimit)
	fmt.Printf("Expires At: %v\n", key.ExpiresAt)

	// Example output:
	// API Key: nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0
	// Client ID: billing-service
	// Roles: [notify-email notify-slack]
	// Rate Limit: 100 req/min
	// Expires At: 2025-11-24 10:30:00 +0000 UTC
}
```

### Key Naming Convention

Generated API keys follow the format: `nk_<32-hex-characters>`

- `nk_` prefix identifies it as a Notifier API key
- The hex string is cryptographically secure random

### Key Properties

| Property | Description |
|----------|-------------|
| `Key` | The actual API key to use in requests |
| `ClientID` | Identifier for the client/service using the key |
| `Roles` | List of roles granted to this key (e.g., "notify-email", "notify-slack") |
| `RateLimit` | Requests per minute allowed (0 = unlimited) |
| `ExpiresAt` | Optional expiration date (if set, key becomes invalid after this time) |
| `CreatedAt` | Timestamp when the key was created |
| `LastUsedAt` | Timestamp of the last successful authentication |
| `IsActive` | Whether the key is currently active (can be deactivated) |

## API Key Roles

Roles control which notifiers a client can use. Common role patterns:

| Role | Purpose |
|------|---------|
| `notify-email` | Can use email (SMTP) notifiers |
| `notify-slack` | Can use Slack notifiers |
| `notify-ntfy` | Can use ntfy.sh notifiers |
| `notify-all` | Can use all notification types |
| `admin` | Full access (optional, for admin operations) |

You define your own roles based on your needs.

## Configuring Role-Based Access

Control which roles can use specific notifiers in your config:

```yaml
notifiers:
  smtp:
    default:
      host: "smtp.example.com"
      port: 587
      username: "user@example.com"
      password: "${SMTP_PASSWORD}"
      from: "noreply@example.com"
      use_tls: true
      allowed_roles:  # Empty list = all authenticated users can use
        - "notify-email"
        - "admin"

    internal:
      host: "smtp-internal.example.com"
      port: 587
      username: "internal@example.com"
      password: "${SMTP_INTERNAL_PASSWORD}"
      from: "internal@example.com"
      use_tls: true
      allowed_roles:
        - "admin"  # Only admins can use internal SMTP

  slack:
    default:
      webhook_url: "${SLACK_WEBHOOK}"
      username: "Notifier"
      allowed_roles:
        - "notify-slack"
        - "notify-all"

  ntfy:
    default:
      server_url: "https://ntfy.sh"
      token: "${NTFY_TOKEN}"
      allowed_roles:
        - "notify-all"
```

## Using API Keys

### REST API

Include the API key in the `Authorization` header as a Bearer token:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Hello",
    "body": "World",
    "recipients": ["user@example.com"]
  }'
```

Alternatively, use the `X-API-Key` header:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "X-API-Key: nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

### gRPC

Include the API key in gRPC metadata:

```go
package main

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	pb "github.com/igodwin/notifier/api/grpc/pb"
)

func main() {
	conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
	defer conn.Close()

	// Create context with API key
	ctx := context.Background()
	md := metadata.New(map[string][]string{
		"authorization": {"bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"},
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Use the client
	client := pb.NewNotifierServiceClient(conn)
	resp, err := client.SendNotification(ctx, &pb.SendNotificationRequest{
		Type:       pb.NotificationType_NOTIFICATION_TYPE_EMAIL,
		Subject:    "Hello",
		Body:       "World",
		Recipients: []string{"user@example.com"},
	})
	// ...
}
```

Or use `grpcurl`:

```bash
grpcurl -plaintext \
  -H "authorization: bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  -d '{"type":"NOTIFICATION_TYPE_EMAIL","subject":"Hello","body":"World","recipients":["user@example.com"]}' \
  localhost:50051 notifier.v1.NotifierService/SendNotification
```

## Credential Management Best Practices

### For Self-Created Clients

**DO:**
- ✅ Store API keys in environment variables
- ✅ Store API keys in secure configuration management (Vault, AWS Secrets Manager)
- ✅ Rotate keys periodically (every 90 days recommended)
- ✅ Use separate keys per environment (dev, staging, prod)
- ✅ Use separate keys per service/application
- ✅ Monitor key usage via logs and audit trails
- ✅ Set expiration times on keys
- ✅ Use appropriate rate limits

**DON'T:**
- ❌ Store API keys in code or version control
- ❌ Include API keys in Docker images or build artifacts
- ❌ Log or display API keys in error messages
- ❌ Use wildcard roles like "admin" for non-admin services
- ❌ Share API keys between services
- ❌ Use the same key for multiple environments

### Example: Storing in Environment Variables

```bash
# .env file (not committed to git)
NOTIFIER_API_KEY="nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"
```

```go
// In your application
import "os"

apiKey := os.Getenv("NOTIFIER_API_KEY")
```

### Example: Using with Configuration Management (Vault)

```go
package main

import (
	"fmt"
	"os"
	vault "github.com/hashicorp/vault/api"
)

func getAPIKeyFromVault() (string, error) {
	client, err := vault.NewClient(&vault.Config{
		Address: os.Getenv("VAULT_ADDR"),
	})
	if err != nil {
		return "", err
	}

	secret, err := client.Logical().Read("secret/data/notifier/api-key")
	if err != nil {
		return "", err
	}

	data := secret.Data["data"].(map[string]interface{})
	return data["key"].(string), nil
}
```

## Client Implementation Examples

### Go Client

```go
package main

import (
	"context"
	"fmt"
	"os"
	"github.com/igodwin/notifier/api/grpc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type NotifierClient struct {
	client pb.NotifierServiceClient
	apiKey string
}

func NewNotifierClient(addr, apiKey string) (*NotifierClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &NotifierClient{
		client: pb.NewNotifierServiceClient(conn),
		apiKey: apiKey,
	}, nil
}

func (nc *NotifierClient) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	// Add API key to context metadata
	md := metadata.New(map[string][]string{
		"authorization": {fmt.Sprintf("bearer %s", nc.apiKey)},
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	return nc.client.SendNotification(ctx, req)
}

func main() {
	apiKey := os.Getenv("NOTIFIER_API_KEY")
	client, err := NewNotifierClient("localhost:50051", apiKey)
	if err != nil {
		panic(err)
	}

	resp, err := client.SendNotification(context.Background(), &pb.SendNotificationRequest{
		Type:       pb.NotificationType_NOTIFICATION_TYPE_EMAIL,
		Subject:    "Hello",
		Body:       "World",
		Recipients: []string{"user@example.com"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Notification sent: %s\n", resp.Result.NotificationId)
}
```

### Python Client

```python
import os
import grpc
from notifier.api.grpc import notifier_pb2, notifier_pb2_grpc

def send_notification(subject, body, recipients):
    api_key = os.getenv("NOTIFIER_API_KEY")

    # Create secure channel
    channel = grpc.secure_channel("localhost:50051", grpc.ssl_channel_credentials())
    stub = notifier_pb2_grpc.NotifierServiceStub(channel)

    # Create metadata with API key
    metadata = [("authorization", f"bearer {api_key}")]

    # Send notification
    request = notifier_pb2.SendNotificationRequest(
        type=notifier_pb2.NOTIFICATION_TYPE_EMAIL,
        subject=subject,
        body=body,
        recipients=recipients,
    )

    response = stub.SendNotification(request, metadata=metadata)
    return response.result.notification_id

if __name__ == "__main__":
    notif_id = send_notification(
        "Hello",
        "World",
        ["user@example.com"]
    )
    print(f"Notification sent: {notif_id}")
```

### Node.js/TypeScript Client

```typescript
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import * as os from "os";

const NOTIFIER_API_KEY = os.getenv("NOTIFIER_API_KEY");

const packageDef = protoLoader.loadSync("notifier.proto", {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const notifierProto = grpc.loadPackageDefinition(packageDef);

async function sendNotification(subject: string, body: string, recipients: string[]) {
  // Create metadata with API key
  const metadata = new grpc.Metadata();
  metadata.set("authorization", `bearer ${NOTIFIER_API_KEY}`);

  // Create client
  const client = new (notifierProto.notifier.v1.NotifierService as any)(
    "localhost:50051",
    grpc.credentials.createInsecure()
  );

  return new Promise((resolve, reject) => {
    client.sendNotification(
      {
        type: "NOTIFICATION_TYPE_EMAIL",
        subject,
        body,
        recipients,
      },
      metadata,
      (err: any, response: any) => {
        if (err) reject(err);
        else resolve(response.result.notification_id);
      }
    );
  });
}

// Usage
sendNotification("Hello", "World", ["user@example.com"])
  .then((notifId) => console.log(`Notification sent: ${notifId}`))
  .catch((err) => console.error(err));
```

### cURL Examples

```bash
# Send email notification
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Alert",
    "body": "Something happened",
    "recipients": ["admin@example.com"]
  }'

# Batch notifications
curl -X POST http://localhost:8080/api/v1/notifications/batch \
  -H "Authorization: Bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0" \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "type": "email",
        "subject": "Alert 1",
        "body": "First alert",
        "recipients": ["user1@example.com"]
      },
      {
        "type": "slack",
        "subject": "Alert 2",
        "body": "Second alert",
        "recipients": ["#alerts"]
      }
    ]
  }'

# Get notification status
curl -X GET http://localhost:8080/api/v1/notifications/{id} \
  -H "Authorization: Bearer nk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"
```

## Error Responses

### Authentication Failures

**REST API:**

```
401 Unauthorized
Missing or invalid Authorization header

403 Forbidden
Rate limit exceeded

401 Unauthorized
Invalid API key

401 Unauthorized
API key has expired
```

**gRPC:**

```
UNAUTHENTICATED: Missing or invalid Authorization header
UNAUTHENTICATED: Invalid API key
UNAUTHENTICATED: API key has expired
RESOURCE_EXHAUSTED: Rate limit exceeded
PERMISSION_DENIED: Insufficient permissions for this notifier
```

## Configuration Examples

### Example 1: Multi-Tenant Setup

```yaml
auth:
  enabled: true
  default_rate_limit: 100

notifiers:
  smtp:
    default:
      host: "smtp.example.com"
      port: 587
      username: "shared@example.com"
      password: "${SMTP_PASSWORD}"
      from: "notifications@example.com"
      allowed_roles:
        - "notify-all"

    tenant-a:
      host: "smtp.tenant-a.com"
      port: 587
      username: "notifications@tenant-a.com"
      password: "${TENANT_A_SMTP_PASSWORD}"
      from: "notifications@tenant-a.com"
      allowed_roles:
        - "tenant-a-notifications"

    tenant-b:
      host: "smtp.tenant-b.com"
      port: 587
      username: "notifications@tenant-b.com"
      password: "${TENANT_B_SMTP_PASSWORD}"
      from: "notifications@tenant-b.com"
      allowed_roles:
        - "tenant-b-notifications"
```

### Example 2: Restricted Access

```yaml
auth:
  enabled: true
  default_rate_limit: 50

notifiers:
  smtp:
    default:
      host: "smtp.example.com"
      port: 587
      username: "user@example.com"
      password: "${SMTP_PASSWORD}"
      from: "noreply@example.com"
      allowed_roles:
        - "admin"  # Only admins
        - "email-service"

  slack:
    default:
      webhook_url: "${SLACK_WEBHOOK}"
      allowed_roles:
        - "admin"
        - "alerts"  # Only alert systems
```

## Monitoring & Auditing

Authentication events are logged with the following information:

```json
{
  "timestamp": "2025-10-25T10:30:00Z",
  "event": "auth_success",
  "client_id": "billing-service",
  "roles": ["notify-email", "notify-slack"],
  "rate_limit_remaining": 95,
  "endpoint": "/api/v1/notifications"
}
```

Authentication failures are also logged for security auditing:

```json
{
  "timestamp": "2025-10-25T10:31:00Z",
  "event": "auth_failure",
  "reason": "invalid_api_key",
  "remote_addr": "192.168.1.100"
}
```

Monitor these logs for:
- Brute force attempts (multiple failed authentications from same IP)
- Unusual access patterns
- Rate limit violations
- Key expiration approaching
- Inactive keys being used

## Summary

1. **Enable auth** in config: `auth.enabled: true`
2. **Create API keys** with appropriate roles and rate limits
3. **Configure role-based access** for each notifier
4. **Use environment variables** or secrets manager for key storage
5. **Monitor logs** for security events
6. **Rotate keys regularly** and set expiration dates
7. **Use separate keys** for each service/application
