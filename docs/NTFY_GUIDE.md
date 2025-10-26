# Ntfy Integration Guide

This guide explains how to use ntfy.sh notifications with the Notifier service.

## What is Ntfy?

[ntfy](https://ntfy.sh) is a simple HTTP-based pub-sub notification service. You can send notifications to your phone, desktop, or any device that subscribes to your topics. It's perfect for:

- Push notifications to mobile devices
- Desktop notifications
- Server alerts and monitoring
- CI/CD pipeline notifications
- IoT device notifications

## Authentication Methods

### 1. Token-Based Authentication (Recommended)

Token authentication is the preferred method for ntfy.sh.

#### Access Tokens (for authenticated topics)
```yaml
notifiers:
  ntfy:
    server_url: "https://ntfy.sh"
    token: "tk_your_access_token"
```

Get an access token:
1. Go to https://ntfy.sh/account
2. Create an account or log in
3. Go to "Access Tokens"
4. Create a new token with appropriate permissions
5. Copy the token (starts with `tk_`)

#### Publish Tokens (for specific topics)
```yaml
notifiers:
  ntfy:
    server_url: "https://ntfy.sh"
    token: "your_publish_token"
```

Create a publish token:
1. Create a topic with reserved access
2. Generate a publish token for that topic
3. Use the token in your configuration

### 2. Basic Authentication

Alternative to token auth:

```yaml
notifiers:
  ntfy:
    server_url: "https://ntfy.sh"
    username: "your-username"
    password: "your-password"
```

### 3. No Authentication (Public Topics)

For public topics on ntfy.sh:

```yaml
notifiers:
  ntfy:
    server_url: "https://ntfy.sh"
    # No token, username, or password needed
```

## Configuration Options

### Full Configuration Example

```yaml
notifiers:
  ntfy:
    # Single instance configuration
    public:
      # Server URL (default: https://ntfy.sh)
      server_url: "https://ntfy.sh"

      # Authentication (choose one method)
      token: "tk_your_token"              # Token auth (recommended)
      # username: "user"                  # Or basic auth
      # password: "pass"

      # Optional: default topic if not specified in notification
      default_topic: "my-default-topic"

      # Mark this instance as default (used when no account specified)
      default: true

      # Optional: roles allowed to use this notifier (empty = all authenticated users)
      # allowed_roles:
      #   - "admin"
      #   - "devops"
```

### Multiple Named Instances

```yaml
notifiers:
  ntfy:
    # Public ntfy.sh instance
    public:
      server_url: "https://ntfy.sh"
      token: "tk_your_access_token"
      default_topic: "my-public-topic"
      default: true

    # Private self-hosted instance
    private:
      server_url: "https://ntfy.mycompany.com"
      username: "your-username"
      password: "your-password"
      default_topic: "internal-notifications"
      ca_cert_path: "/etc/notifier/certs/ca.pem"
      allowed_roles:
        - "admin"
        - "devops"
```

### Self-Hosted Ntfy Server with Custom CA

For self-hosted ntfy servers with self-signed certificates:

```yaml
notifiers:
  ntfy:
    private:
      server_url: "https://ntfy.yourcompany.com"
      token: "your_custom_token"
      # Path to custom CA certificate (PEM format)
      ca_cert_path: "/etc/notifier/certs/ca.pem"
```

**Important**: TLS verification is always enforced. Use `ca_cert_path` to trust custom CA certificates. The `ca_cert_path` must:
- Point to a valid PEM-formatted certificate file
- Be readable by the notifier process
- Be the root or intermediate CA certificate (not end-entity certificate)

## Configuration Fields Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `server_url` | string | No | `https://ntfy.sh` | The ntfy server URL (public or self-hosted) |
| `token` | string | No | (none) | Bearer token for authentication. Preferred over username/password. |
| `username` | string | No | (none) | Username for basic authentication (alternative to token) |
| `password` | string | No | (none) | Password for basic authentication (used with username) |
| `default_topic` | string | No | (none) | Default topic to use if not specified in the notification |
| `ca_cert_path` | string | No | (none) | Path to custom CA certificate file (PEM format) for self-hosted servers |
| `default` | boolean | No | `false` | If true, this instance is used when no account is specified |
| `allowed_roles` | string array | No | (none) | Roles allowed to use this notifier. Empty means all authenticated users. |

**Authentication Priority**:
1. Token (if provided)
2. Username + Password (if provided)
3. No authentication (for public topics)

## Sending Notifications

### Basic Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Hello from Notifier!",
    "body": "This is a test notification",
    "recipients": ["my-topic"]
  }'
```

### With Priority

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "priority": 4,
    "subject": "CRITICAL Alert",
    "body": "Something important happened!",
    "recipients": ["alerts"]
  }'
```

Priority mapping:
- `0` (Low) → ntfy priority 2
- `1` (Normal) → ntfy priority 3 (default)
- `2` (High) → ntfy priority 4
- `3` (Critical) → ntfy priority 5 (max)

### With Tags (Emojis)

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Deployment Complete",
    "body": "Application deployed successfully",
    "recipients": ["deployments"],
    "metadata": {
      "tags": ["rocket", "tada", "white_check_mark"]
    }
  }'
```

Common tags:
- `warning`, `rotating_light`, `skull` - Alerts
- `tada`, `rocket`, `sparkles` - Success
- `x`, `no_entry`, `stop_sign` - Errors
- `information_source`, `eyes` - Info

### With Click Action

Make the notification clickable:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "New Pull Request",
    "body": "PR #123 needs review",
    "recipients": ["github-notifications"],
    "metadata": {
      "click": "https://github.com/your-org/your-repo/pull/123",
      "tags": ["github"]
    }
  }'
```

### With Attachment

Attach an image or file:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Server Stats",
    "body": "Current server metrics",
    "recipients": ["monitoring"],
    "metadata": {
      "attach": "https://example.com/metrics.png",
      "tags": ["chart_with_upwards_trend"]
    }
  }'
```

### With Custom Icon

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Custom Notification",
    "body": "With a custom icon",
    "recipients": ["custom-alerts"],
    "metadata": {
      "icon": "https://example.com/logo.png"
    }
  }'
```

### With Delayed Delivery

Schedule notification for later:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Reminder",
    "body": "Meeting in 30 minutes",
    "recipients": ["reminders"],
    "metadata": {
      "delay": "30m",
      "tags": ["alarm_clock"]
    }
  }'
```

Delay formats:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `2h` - 2 hours
- `tomorrow 10am` - Tomorrow at 10 AM

### With Action Buttons

Add interactive buttons:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Deploy to Production?",
    "body": "Version 2.0 is ready",
    "recipients": ["deployments"],
    "metadata": {
      "actions": [
        {
          "action": "view",
          "label": "View Release",
          "url": "https://github.com/your-org/your-repo/releases/v2.0",
          "clear": true
        },
        {
          "action": "http",
          "label": "Deploy",
          "url": "https://api.yourcompany.com/deploy",
          "body": "{\"version\": \"2.0\"}",
          "clear": true
        }
      ]
    }
  }'
```

Action types:
- `view` - Open a URL
- `http` - Send HTTP request
- `broadcast` - Android broadcast intent

### With Email Forwarding

Forward to email (requires ntfy.sh tier 2+):

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Important Alert",
    "body": "This will also be sent via email",
    "recipients": ["alerts"],
    "metadata": {
      "email": "admin@example.com",
      "tags": ["email"]
    }
  }'
```

### Multiple Topics

Send to multiple topics:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "System Update",
    "body": "System will restart in 5 minutes",
    "recipients": ["admins", "monitoring", "alerts"],
    "metadata": {
      "tags": ["warning"]
    }
  }'
```

## Mobile App Setup

### iOS
1. Download ntfy from the App Store
2. Add a topic subscription
3. Use the same topic name in your notifications

### Android
1. Download ntfy from Google Play or F-Droid
2. Add a topic subscription
3. Configure notification settings
4. Use the same topic name in your notifications

### Desktop (Linux/macOS/Windows)
```bash
# Install ntfy CLI
curl -sSL https://ntfy.sh/install.sh | sh

# Subscribe to topics
ntfy subscribe mytopic
```

## Topic Naming Best Practices

### Public Topics
- Use unique, hard-to-guess names
- Consider including random strings: `myapp-alerts-x7k9p2`
- Anyone who knows the name can subscribe

### Private Topics (Recommended)
- Requires authentication
- Create reserved topics on ntfy.sh
- Use access control lists (ACLs)

### Topic Organization
```yaml
# Example topic structure
- myapp-prod-alerts      # Production alerts
- myapp-prod-info        # Production info
- myapp-staging-alerts   # Staging alerts
- myapp-ci-cd            # CI/CD notifications
- myapp-monitoring       # Monitoring metrics
```

## Environment Variables

Override configuration with environment variables using the instance name in the path:

```bash
# Public instance (ntfy.sh)
export NOTIFIER_NOTIFIERS_NTFY_PUBLIC_SERVER_URL=https://ntfy.sh
export NOTIFIER_NOTIFIERS_NTFY_PUBLIC_TOKEN=tk_your_access_token
export NOTIFIER_NOTIFIERS_NTFY_PUBLIC_DEFAULT_TOPIC=my-topic

# Private instance (self-hosted)
export NOTIFIER_NOTIFIERS_NTFY_PRIVATE_SERVER_URL=https://ntfy.mycompany.com
export NOTIFIER_NOTIFIERS_NTFY_PRIVATE_USERNAME=your-username
export NOTIFIER_NOTIFIERS_NTFY_PRIVATE_PASSWORD=your-password
export NOTIFIER_NOTIFIERS_NTFY_PRIVATE_CA_CERT_PATH=/etc/notifier/certs/ca.pem
export NOTIFIER_NOTIFIERS_NTFY_PRIVATE_DEFAULT_TOPIC=internal-topic
```

**Note**: Replace `PUBLIC` and `PRIVATE` with your actual instance names. Environment variable names are case-insensitive and follow the pattern: `NOTIFIER_NOTIFIERS_NTFY_<INSTANCE_NAME>_<FIELD_NAME>`

## Security Considerations

### Token Security
- **Never commit tokens to version control**
- Store tokens in Kubernetes Secrets or environment variables
- Rotate tokens regularly
- Use publish tokens with limited scope when possible

### Topic Security
- Use reserved/private topics for sensitive data
- Don't include secrets in notification bodies
- Consider encryption for highly sensitive data
- Use unique topic names to prevent enumeration

### Self-Hosted Servers
- Use TLS with valid certificates
- Enable authentication
- Configure rate limiting
- Monitor access logs
- Keep ntfy server updated

## Kubernetes Deployment

### Creating Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notifier-secrets
type: Opaque
stringData:
  ntfy-public-token: "tk_your_access_token"
  ntfy-private-username: "your-username"
  ntfy-private-password: "your-password"
  ca-cert.pem: |
    -----BEGIN CERTIFICATE-----
    MIIBkTCB+wIJAKHHCgVkEkGZMA0GCSqGSIb3DQEBBQUAMBMxETAPBgNVBAMMCENB
    ... (certificate content) ...
    -----END CERTIFICATE-----
```

### Deployment Configuration with Multiple Instances

```yaml
env:
# Public ntfy.sh instance
- name: NOTIFIER_NOTIFIERS_NTFY_PUBLIC_SERVER_URL
  value: "https://ntfy.sh"
- name: NOTIFIER_NOTIFIERS_NTFY_PUBLIC_TOKEN
  valueFrom:
    secretKeyRef:
      name: notifier-secrets
      key: ntfy-public-token
- name: NOTIFIER_NOTIFIERS_NTFY_PUBLIC_DEFAULT
  value: "true"

# Private self-hosted instance
- name: NOTIFIER_NOTIFIERS_NTFY_PRIVATE_SERVER_URL
  value: "https://ntfy.mycompany.com"
- name: NOTIFIER_NOTIFIERS_NTFY_PRIVATE_USERNAME
  valueFrom:
    secretKeyRef:
      name: notifier-secrets
      key: ntfy-private-username
- name: NOTIFIER_NOTIFIERS_NTFY_PRIVATE_PASSWORD
  valueFrom:
    secretKeyRef:
      name: notifier-secrets
      key: ntfy-private-password
- name: NOTIFIER_NOTIFIERS_NTFY_PRIVATE_CA_CERT_PATH
  value: "/etc/notifier/certs/ca.pem"

volumeMounts:
- name: ca-certs
  mountPath: /etc/notifier/certs

volumes:
- name: ca-certs
  secret:
    secretName: notifier-secrets
    items:
    - key: ca-cert.pem
      path: ca.pem
```

## Rate Limits

### ntfy.sh Free Tier
- 250 messages/day per visitor
- Unlimited topics
- Message retention: 12 hours

### ntfy.sh Tier 1 ($5/month)
- 500 messages/day
- Message retention: 1 day
- Attachment & email support

### ntfy.sh Tier 2 ($10/month)
- 1000 messages/day
- Message retention: 7 days
- Higher attachment limits

### Self-Hosted
- Configure your own limits
- Full control over retention
- No external dependencies

## Troubleshooting

### Authentication Errors
```
Error: ntfy server returned status: 401
```
**Solution**: Verify token is correct and has necessary permissions

### Topic Not Found
```
Error: ntfy server returned status: 404
```
**Solution**: Check topic name spelling, ensure topic exists if using reserved topics

### TLS Errors (Self-Hosted)
```
Error: x509: certificate signed by unknown authority
```
**Solution**: Use `ca_cert_path` to specify the path to your CA certificate:
```yaml
notifiers:
  ntfy:
    private:
      server_url: "https://ntfy.yourcompany.com"
      token: "your_token"
      ca_cert_path: "/etc/notifier/certs/ca.pem"
```
Ensure the certificate file is accessible and in PEM format. TLS verification is always enforced for security.

### Rate Limit Exceeded
```
Error: ntfy server returned status: 429
```
**Solution**: Reduce message frequency or upgrade ntfy.sh tier

### Connection Timeout
```
Error: context deadline exceeded
```
**Solution**: Check network connectivity, firewall rules, or server URL

## Examples

See [QUICKSTART.md](../QUICKSTART.md) for more examples of using ntfy with the Notifier service.

## Resources

- [Ntfy Documentation](https://docs.ntfy.sh)
- [Ntfy.sh Public Instance](https://ntfy.sh)
- [Self-Hosting Guide](https://docs.ntfy.sh/install/)
- [API Reference](https://docs.ntfy.sh/publish/)
