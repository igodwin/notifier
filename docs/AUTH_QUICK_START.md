# Authentication Quick Start Guide

## 1. Enable Authentication

Update your `config.yaml`:

```yaml
auth:
  enabled: true
  default_rate_limit: 100  # requests/minute
```

## 2. Generate an API Key (Programmatically)

```go
store := auth.NewAPIKeyStore()

// Create a key that expires in 30 days
expiresIn := 30 * 24 * time.Hour
key, _ := store.CreateKey(
    "my-app",                              // Client ID
    []string{"notify-email", "notify-slack"}, // Roles
    100,                                   // Rate limit (req/min)
    &expiresIn,                           // Expiration
)

fmt.Println(key.Key)  // nk_<hex>
```

## 3. Use the Key in REST API

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer nk_<your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Test",
    "body": "Hello!",
    "recipients": ["user@example.com"]
  }'
```

## 4. Use the Key in gRPC

```go
md := metadata.New(map[string][]string{
    "authorization": {"bearer nk_<your-api-key>"},
})
ctx := metadata.NewOutgoingContext(context.Background(), md)

client.SendNotification(ctx, &pb.SendNotificationRequest{...})
```

## 5. Configure Role-Based Access (Optional)

In `config.yaml`, restrict which roles can use each notifier:

```yaml
notifiers:
  smtp:
    default:
      host: "smtp.example.com"
      ...
      allowed_roles:
        - "notify-email"  # Only clients with this role can use
        - "admin"

  slack:
    default:
      webhook_url: "..."
      allowed_roles:
        - "notify-slack"
```

If `allowed_roles` is empty or omitted, any authenticated user can use the notifier.

## 6. Store Keys Securely

**Never commit API keys to Git.**

Use environment variables:

```bash
# .env (not in Git)
export NOTIFIER_API_KEY="nk_abc123..."

# In your app
apiKey := os.Getenv("NOTIFIER_API_KEY")
```

Or use a secrets manager (Vault, AWS Secrets Manager, etc.).

## 7. Monitor Logs

Authentication events are logged. Look for:
- `auth_success` - Successful API key validation
- `auth_failure` - Failed authentication attempts
- `rate_limit_exceeded` - Rate limit violation

## Key Concepts

| Term | Meaning |
|------|---------|
| **API Key** | Token used to authenticate requests (format: `nk_<32-hex>`) |
| **Client ID** | Identifier for the app/service using the key |
| **Role** | Permission level (e.g., "notify-email", "admin") |
| **Rate Limit** | Max requests per minute (0 = unlimited) |
| **Expiration** | Optional date when key becomes invalid |

## Default Configuration (Auth Disabled)

If you don't set `auth.enabled: true`, authentication is **not enforced** and API keys are not checked. This is the default for backward compatibility.

## Troubleshooting

| Error | Cause | Solution |
|-------|-------|----------|
| `401 Unauthorized` | Missing/invalid API key | Check header is set correctly |
| `403 Forbidden` | Role not allowed | Add role to notifier's `allowed_roles` |
| `429 Too Many Requests` | Rate limit exceeded | Wait 60 seconds or create new key with higher limit |
| `Invalid API key` | Key doesn't exist or expired | Check key format and expiration date |

## Full Documentation

See `docs/AUTH.md` for comprehensive documentation including:
- Credential management best practices
- Multi-language client examples
- Configuration examples
- Monitoring and auditing
- Advanced scenarios
