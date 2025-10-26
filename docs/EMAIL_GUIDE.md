# Email (SMTP) Integration Guide

This guide explains how to use email notifications with the Notifier service. Send notifications via SMTP to any email address with support for HTML content, CC/BCC recipients, and multiple configured email accounts.

## What is SMTP?

[SMTP](https://tools.ietf.org/html/rfc5321) (Simple Mail Transfer Protocol) is the standard protocol for sending emails. The Notifier service connects to SMTP servers to deliver email notifications reliably.

### Why Use Email Notifications?

- Direct delivery to email inboxes (guaranteed delivery method)
- Support for HTML content and rich formatting
- CC and BCC recipients for notifications
- Multiple email accounts for different use cases
- Reliable, industry-standard protocol
- Wide compatibility with email providers

## Authentication Methods

### Username and Password Authentication

The only supported authentication method:

```yaml
notifiers:
  smtp:
    personal:
      host: "smtp.gmail.com"
      port: 587
      username: "your-email@gmail.com"
      password: "your-app-password"
      from: "your-email@gmail.com"
      use_tls: true
```

**Security Note**: Always use TLS encryption with username/password authentication. Never send passwords over unencrypted connections.

## Configuration Options

### Full Configuration Example

```yaml
notifiers:
  smtp:
    # Single account configuration
    personal:
      # SMTP server hostname (required)
      host: "smtp.gmail.com"

      # SMTP server port (default: 587 for TLS submission)
      port: 587

      # Authentication credentials (both required)
      username: "your-email@gmail.com"
      password: "your-app-password"

      # Email address for the "From" header (required)
      from: "your-email@gmail.com"

      # Display name for sender (optional)
      # Will appear as "Your Display Name <your-email@gmail.com>"
      from_name: "My Application"

      # Enable TLS encryption (recommended: true)
      use_tls: true

      # Mark this account as default
      default: true

      # Restrict usage to specific roles (optional)
      # Empty list means all authenticated users can use this account
      # allowed_roles:
      #   - "admin"
      #   - "devops"
```

### Multiple Named Accounts

Configure multiple email accounts for different purposes:

```yaml
notifiers:
  smtp:
    # Personal Gmail account
    personal:
      host: "smtp.gmail.com"
      port: 587
      username: "personal@gmail.com"
      password: "personal-app-password"
      from: "personal@gmail.com"
      from_name: "Personal Alerts"
      use_tls: true
      default: true

    # Work account
    work:
      host: "smtp.office365.com"
      port: 587
      username: "you@company.com"
      password: "your-password"
      from: "notifications@company.com"
      from_name: "Company Notifications"
      use_tls: true
      default: false
      allowed_roles:
        - "admin"
        - "ops"

    # Alerts account
    alerts:
      host: "smtp.company.com"
      port: 587
      username: "alerts-user@company.com"
      password: "alerts-password"
      from: "alerts@company.com"
      use_tls: true
      default: false
```

## Configuration Fields Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `host` | string | Yes | N/A | SMTP server hostname (e.g., smtp.gmail.com) |
| `port` | integer | No | 587 | SMTP server port (587 for TLS submission, 25 for plain, 465 for implicit TLS) |
| `username` | string | Yes | N/A | SMTP authentication username |
| `password` | string | Yes | N/A | SMTP authentication password or app-specific password |
| `from` | string | Yes | N/A | Email address to use in the "From" header |
| `from_name` | string | No | (empty) | Display name for the sender (formatted as "name <email>") |
| `use_tls` | boolean | No | false | Enable TLS encryption for SMTP connection |
| `default` | boolean | No | false | If true, this account is used when no account is specified |
| `allowed_roles` | string array | No | (empty) | Roles allowed to use this account. Empty means all authenticated users. |

## SMTP Server Configuration

### Popular Email Providers

#### Gmail

```yaml
notifiers:
  smtp:
    gmail:
      host: "smtp.gmail.com"
      port: 587
      username: "your-email@gmail.com"
      password: "your-app-password"  # NOT your Gmail password!
      from: "your-email@gmail.com"
      use_tls: true
```

**Setup Instructions**:
1. Enable 2-Step Verification on your Google Account
2. Go to https://myaccount.google.com/apppasswords
3. Create an app password for "Mail" and "Windows Computer" (or generic device)
4. Use the 16-character generated password in your config
5. Keep your actual Gmail password secret

#### Office 365 / Microsoft Exchange

```yaml
notifiers:
  smtp:
    office365:
      host: "smtp.office365.com"
      port: 587
      username: "you@company.com"
      password: "your-office365-password"
      from: "you@company.com"
      from_name: "Company Notifications"
      use_tls: true
```

#### AWS SES (Simple Email Service)

```yaml
notifiers:
  smtp:
    aws_ses:
      host: "email-smtp.us-east-1.amazonaws.com"  # Use your region
      port: 587
      username: "AKIA..."  # SES SMTP username from AWS console
      password: "your-ses-password"  # SES SMTP password from AWS console
      from: "noreply@yourdomain.com"  # Must be verified in SES
      from_name: "Your Application"
      use_tls: true
```

#### SendGrid

```yaml
notifiers:
  smtp:
    sendgrid:
      host: "smtp.sendgrid.net"
      port: 587
      username: "apikey"  # Always "apikey"
      password: "SG.your-api-key"  # SendGrid API key
      from: "noreply@yourdomain.com"
      from_name: "Your Application"
      use_tls: true
```

#### Self-Hosted (e.g., Postfix, Exim)

```yaml
notifiers:
  smtp:
    internal:
      host: "mail.company.com"
      port: 587
      username: "notifier-user"
      password: "internal-password"
      from: "notifications@company.com"
      from_name: "Internal Alerts"
      use_tls: true  # Recommended even for internal servers
```

## Sending Email Notifications

### Basic Email

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Test Notification",
    "body": "This is a test email from Notifier",
    "recipients": ["recipient@example.com"]
  }'
```

### With Display Name in From Header

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Hello!",
    "body": "A message from your application",
    "recipients": ["user@example.com"]
  }'
# Sends from: "My Application <my-email@gmail.com>"
```

### HTML Email

Send rich HTML formatted emails:

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Deployment Complete",
    "body": "<h1>Deployment Successful</h1><p>Version 2.0 is now live!</p><a href=\"https://example.com\">View deployment</a>",
    "content_type": "html",
    "recipients": ["team@example.com"]
  }'
```

**HTML Auto-Detection**:
The system automatically detects HTML content if your body contains:
- `<html`, `<!DOCTYPE`, `<body`, `<div`, `<p`, `<h1-h6`, `<br>`, etc.

Even without specifying `"content_type": "html"`, the email will be sent as HTML.

### With CC Recipients

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Project Status Update",
    "body": "Here is the status of our project...",
    "recipients": ["manager@example.com"],
    "cc": ["team@example.com", "stakeholder@example.com"]
  }'
```

### With BCC Recipients

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Public Announcement",
    "body": "We are proud to announce...",
    "recipients": ["public@example.com"],
    "bcc": ["admin@example.com"]  # Hidden from other recipients
  }'
```

**BCC Security Note**: BCC recipients are not included in email headers, maintaining privacy.

### Multiple Recipients

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Team Alert",
    "body": "All team members need to review this alert immediately",
    "recipients": [
      "member1@example.com",
      "member2@example.com",
      "member3@example.com"
    ]
  }'
```

### Using a Specific Email Account

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "work",
    "subject": "Internal Notification",
    "body": "This comes from our work email account",
    "recipients": ["colleague@company.com"]
  }'
```

Replace `"work"` with the name of the SMTP account you configured.

### Complex Example: All Features

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "alerts",
    "subject": "Critical Alert: Database Performance Degradation",
    "body": "<h2>Alert Summary</h2><p>Database query response times have increased significantly.</p><h3>Details</h3><ul><li>Average response time: 2.5s (normal: 100ms)</li><li>Affected queries: SELECT from users table</li><li>Impact: High</li></ul><p><a href=\"https://monitoring.example.com/alerts/123\">View in Monitoring Dashboard</a></p>",
    "content_type": "html",
    "recipients": [
      "ops-lead@company.com"
    ],
    "cc": [
      "engineering@company.com"
    ],
    "bcc": [
      "cto@company.com"
    ]
  }'
```

## Environment Variables

Override configuration with environment variables using the account name in the path:

```bash
# Personal Gmail account
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_HOST=smtp.gmail.com
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_PORT=587
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_USERNAME=your-email@gmail.com
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_PASSWORD=your-app-password
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_FROM=your-email@gmail.com
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_FROM_NAME="My Application"
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_USE_TLS=true
export NOTIFIER_NOTIFIERS_SMTP_PERSONAL_DEFAULT=true

# Work account
export NOTIFIER_NOTIFIERS_SMTP_WORK_HOST=smtp.office365.com
export NOTIFIER_NOTIFIERS_SMTP_WORK_PORT=587
export NOTIFIER_NOTIFIERS_SMTP_WORK_USERNAME=you@company.com
export NOTIFIER_NOTIFIERS_SMTP_WORK_PASSWORD=your-password
export NOTIFIER_NOTIFIERS_SMTP_WORK_FROM=notifications@company.com
export NOTIFIER_NOTIFIERS_SMTP_WORK_FROM_NAME="Company Notifications"
export NOTIFIER_NOTIFIERS_SMTP_WORK_USE_TLS=true
export NOTIFIER_NOTIFIERS_SMTP_WORK_DEFAULT=false
```

**Environment Variable Format**:
```
NOTIFIER_NOTIFIERS_SMTP_<ACCOUNT_NAME>_<FIELD_NAME>
```

**Examples**:
- `NOTIFIER_NOTIFIERS_SMTP_PERSONAL_HOST` - Host for "personal" account
- `NOTIFIER_NOTIFIERS_SMTP_WORK_PASSWORD` - Password for "work" account
- `NOTIFIER_NOTIFIERS_SMTP_ALERTS_USE_TLS` - TLS for "alerts" account

Environment variables override YAML configuration file settings.

## Kubernetes Deployment

### Creating Secrets

Store sensitive credentials in Kubernetes Secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: notifier-smtp-secrets
type: Opaque
stringData:
  personal-password: "your-app-password"
  work-password: "your-work-password"
  alerts-password: "alerts-password"
```

### Deployment Configuration with Multiple Accounts

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notifier
spec:
  template:
    spec:
      containers:
      - name: notifier
        image: notifier:latest
        env:
        # Personal Gmail account
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_HOST
          value: "smtp.gmail.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_PORT
          value: "587"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_USERNAME
          value: "your-email@gmail.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: notifier-smtp-secrets
              key: personal-password
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_FROM
          value: "your-email@gmail.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_FROM_NAME
          value: "My Application"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_USE_TLS
          value: "true"
        - name: NOTIFIER_NOTIFIERS_SMTP_PERSONAL_DEFAULT
          value: "true"

        # Work Office 365 account
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_HOST
          value: "smtp.office365.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_PORT
          value: "587"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_USERNAME
          value: "you@company.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_PASSWORD
          valueFrom:
            secretKeyRef:
              name: notifier-smtp-secrets
              key: work-password
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_FROM
          value: "notifications@company.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_FROM_NAME
          value: "Company Notifications"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_USE_TLS
          value: "true"
        - name: NOTIFIER_NOTIFIERS_SMTP_WORK_DEFAULT
          value: "false"

        # Alerts account
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_HOST
          value: "smtp.company.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_PORT
          value: "587"
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_USERNAME
          value: "alerts-user@company.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: notifier-smtp-secrets
              key: alerts-password
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_FROM
          value: "alerts@company.com"
        - name: NOTIFIER_NOTIFIERS_SMTP_ALERTS_USE_TLS
          value: "true"
```

### ConfigMap for Non-Sensitive Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: notifier-config
data:
  config.yaml: |
    notifiers:
      smtp:
        personal:
          host: "smtp.gmail.com"
          port: 587
          from: "your-email@gmail.com"
          from_name: "My Application"
          use_tls: true
          default: true

        work:
          host: "smtp.office365.com"
          port: 587
          from: "notifications@company.com"
          from_name: "Company Notifications"
          use_tls: true
          allowed_roles:
            - "admin"
            - "ops"

        alerts:
          host: "smtp.company.com"
          port: 587
          from: "alerts@company.com"
          use_tls: true
```

## Security Considerations

### Credential Security

- **Never commit passwords to version control** - Use environment variables or secrets management
- **Use app-specific passwords** - Many providers (Gmail, Office 365) support app passwords separate from account passwords
- **Rotate credentials regularly** - Change passwords and regenerate API keys periodically
- **Store in secure vaults** - Use Kubernetes Secrets, AWS Secrets Manager, HashiCorp Vault, etc.

### TLS/STARTTLS

- **Always use `use_tls: true`** - Encrypts credentials and email content in transit
- **Use port 587** (submission port with STARTTLS) or 465 (implicit TLS)
- **Avoid port 25** for authentication - Typically for relay without auth

### Email Content

- **Don't include secrets in email bodies** - Avoid API keys, passwords, tokens
- **Sanitize user input** - If building HTML emails from user data, properly escape content
- **Use HTML escaping** - Prevent email injection attacks

### Access Control

```yaml
notifiers:
  smtp:
    production:
      # ... config ...
      allowed_roles:
        - "admin"
        - "ops"  # Only these roles can send from this account
```

- Use `allowed_roles` to restrict which users can send from specific accounts
- Separate accounts for different purposes (personal, work, alerts)

## Troubleshooting

### Authentication Failed

```
Error: SMTP server returned status 535
```

**Possible Causes**:
- Incorrect username or password
- Password needs to be app-specific password (for Gmail, Office 365)
- Username format incorrect for your provider

**Solution**:
1. Verify credentials on your email provider's website
2. Check username format (may require full email address)
3. For Gmail, ensure you're using an app password, not your main password
4. Test connection with telnet: `telnet smtp.gmail.com 587`

### Connection Refused

```
Error: connection refused
```

**Possible Causes**:
- Wrong host or port
- Firewall blocking the connection
- SMTP server is down

**Solution**:
1. Verify SMTP server hostname and port
2. Check firewall rules allow outbound connections to SMTP port
3. Test DNS resolution: `nslookup smtp.gmail.com`
4. Try alternative port (587 vs 465)

### No Recipients Error

```
Error: email has no recipients (To, CC, or BCC required)
```

**Solution**:
The `recipients` array is empty. Provide at least one email address in:
- `recipients` (To:)
- `cc` (CC:)
- `bcc` (BCC:)

```bash
# Fix: Add recipients
"recipients": ["user@example.com"]
```

### Invalid Email Address

```
Error: invalid email address: user@example
```

**Solution**:
Email addresses must contain the `@` symbol. Verify email addresses:
- Contain `@` character
- Have text before and after `@`
- Use proper format: `local@domain.com`

### Account Not Found

```
Error: notifier not found for type: email, account: unknown
```

**Solution**:
The specified account name doesn't exist. Check:
1. Account name matches your YAML config
2. Environment variables use correct naming
3. Account is properly registered

```bash
# If using account "work", ensure it exists in config:
"account": "work"
```

### Certificate Errors (Self-Hosted)

```
Error: x509: certificate signed by unknown authority
```

**Solution**:
- Use proper certificates from a trusted CA
- Many self-hosted SMTP servers already have valid certs
- Contact your mail server administrator for details
- Go's standard certificate validation is used (system CA bundle)

## Email Content Guidelines

### Plain Text Emails

Best for transactional notifications:

```json
{
  "type": "email",
  "subject": "Your confirmation code is: 123456",
  "body": "Use this code to complete your action. This code expires in 10 minutes.",
  "recipients": ["user@example.com"]
}
```

### HTML Emails

Better for formatted notifications with styling:

```json
{
  "type": "email",
  "subject": "Order Confirmation",
  "body": "<h1>Order #12345</h1><p>Thank you for your purchase!</p><p><strong>Total: $99.99</strong></p><a href='https://example.com/orders/12345' style='background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none;'>View Order</a>",
  "content_type": "html",
  "recipients": ["customer@example.com"]
}
```

### HTML Best Practices

1. **Use inline styles** - Not all email clients support `<style>` tags
2. **Test in multiple clients** - Gmail, Outlook, Apple Mail, mobile clients
3. **Provide plain text fallback** - The system automatically creates one
4. **Avoid large images** - Keep email size reasonable
5. **Use web fonts carefully** - Not all clients support custom fonts
6. **Make links obvious** - Use underlines and contrasting colors

## Examples

### CI/CD Pipeline Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "alerts",
    "subject": "Build #456 Failed",
    "body": "<h2>Build Failure Alert</h2><p><strong>Pipeline:</strong> my-app/main</p><p><strong>Status:</strong> FAILED</p><p><strong>Error:</strong> Test suite failed with 3 failures</p><p><a href=\"https://ci.example.com/builds/456\">View Build Details</a></p>",
    "content_type": "html",
    "recipients": ["dev-team@company.com"],
    "cc": ["tech-lead@company.com"]
  }'
```

### System Alert

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "HIGH: CPU Usage Alert",
    "body": "Server prod-01 CPU usage has exceeded 90% for 5 minutes.\n\nCurrent: 95%\nThreshold: 80%\n\nPlease investigate immediately.",
    "recipients": ["ops@company.com", "oncall@company.com"]
  }'
```

### User Welcome Email

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "personal",
    "subject": "Welcome to Our Service!",
    "body": "<h1>Welcome!</h1><p>Thank you for signing up. Your account is ready to use.</p><p><a href=\"https://example.com/login\">Log In Now</a></p><p>Questions? <a href=\"mailto:support@example.com\">Contact Support</a></p>",
    "content_type": "html",
    "recipients": ["newuser@example.com"]
  }'
```

## References

- [SMTP RFC 5321](https://tools.ietf.org/html/rfc5321) - Protocol specification
- [MIME Types RFC 2045](https://tools.ietf.org/html/rfc2045) - Email content types
- [Gmail App Passwords](https://support.google.com/accounts/answer/185833)
- [Office 365 SMTP](https://support.microsoft.com/en-us/office/pop-imap-and-smtp-settings-for-outlook-com-d88de319-24ca-4986-ab5b-c869cb6f4142)
- [AWS SES SMTP](https://docs.aws.amazon.com/ses/latest/dg/send-email-smtp.html)
- [SendGrid SMTP](https://sendgrid.com/docs/for-developers/sending-email/getting-started-smtp/)
