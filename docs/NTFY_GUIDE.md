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
    # Server URL (default: https://ntfy.sh)
    server_url: "https://ntfy.sh"

    # Authentication (choose one method)
    token: "tk_your_token"              # Token auth (recommended)
    # username: "user"                  # Or basic auth
    # password: "pass"

    # Optional: default topic if not specified in notification
    default_topic: "my-default-topic"

    # Optional: skip TLS verification (for self-hosted with self-signed certs)
    insecure_skip_verify: false
```

### Self-Hosted Ntfy Server

```yaml
notifiers:
  ntfy:
    server_url: "https://ntfy.yourcompany.com"
    token: "your_custom_token"
    # For self-signed certificates
    insecure_skip_verify: true
```

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

Override configuration with environment variables:

```bash
export NOTIFIER_NOTIFIERS_NTFY_SERVER_URL=https://ntfy.yourcompany.com
export NOTIFIER_NOTIFIERS_NTFY_TOKEN=tk_your_token
export NOTIFIER_NOTIFIERS_NTFY_DEFAULT_TOPIC=default-topic
export NOTIFIER_NOTIFIERS_NTFY_INSECURE_SKIP_VERIFY=false
```

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

### Using Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notifier-secrets
type: Opaque
stringData:
  ntfy-token: "tk_your_access_token"
```

### Deployment Configuration

```yaml
env:
- name: NOTIFIER_NOTIFIERS_NTFY_TOKEN
  valueFrom:
    secretKeyRef:
      name: notifier-secrets
      key: ntfy-token
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
**Solution**: Either fix certificate or set `insecure_skip_verify: true` (not recommended for production)

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
