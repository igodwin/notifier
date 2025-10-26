# Role-Based Access Control (RBAC) Guide

This guide explains how to use role-based access control (RBAC) to restrict which notifiers and accounts authenticated users can access.

## Overview

The Notifier service provides role-based access control that allows you to:
- Restrict access to specific notifier types (Email, Slack, Ntfy)
- Restrict access to specific accounts within a notifier type
- Assign roles to API keys
- Control visibility of notifiers in the `GetNotifiers` endpoint

When authentication is enabled, users will only see and be able to use the notifiers and accounts they are authorized to access based on their API key's roles.

## How It Works

### Configuration

Authorization rules are defined in your notifier configuration:

```yaml
notifiers:
  smtp:
    # Personal email account - restricted to admin and ops roles
    primary:
      host: smtp.example.com
      from: noreply@example.com
      allowed_roles:
        - admin
        - ops

    # Support team email - restricted to support role
    support:
      host: smtp.example.com
      from: support@example.com
      allowed_roles:
        - support

  slack:
    # Engineering workspace - restricted to engineering and admin
    engineering:
      webhook_url: https://hooks.slack.com/...
      allowed_roles:
        - engineering
        - admin

    # Marketing workspace - open to all authenticated users
    marketing:
      webhook_url: https://hooks.slack.com/...
      # No allowed_roles specified = all authenticated users

  ntfy:
    # Internal monitoring - admin only
    monitoring:
      server_url: https://ntfy.mycompany.com
      allowed_roles:
        - admin
```

### API Key Assignment

When you create an API key, you assign it roles that determine what it can access:

```bash
# Create admin key (access to all notifiers)
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{
    "client_id": "admin-service",
    "roles": ["admin"]
  }'

# Create support team key (email only)
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{
    "client_id": "support-service",
    "roles": ["support"]
  }'

# Create engineering key (engineering workspace)
curl -X POST http://localhost:8080/api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{
    "client_id": "engineering-service",
    "roles": ["engineering"]
  }'
```

### Authorization Logic

When a user makes a request with their API key:

1. **Authentication**: The API key is validated (exists, not expired, active)
2. **RBAC Check**: For each notifier account, the system checks:
   - Does the user's API key have any of the `allowed_roles`?
   - If yes, the account is accessible
   - If no, the account is hidden
3. **Response**: Only accessible accounts are returned to the user

### GetNotifiers Endpoint

The `GET /api/v1/notifiers` endpoint now returns only the accounts the authenticated user can access:

**Without Authentication** (auth disabled):
```json
{
  "notifiers": [
    {
      "type": "email",
      "accounts": ["primary", "support"],
      "default_account": "primary"
    },
    {
      "type": "slack",
      "accounts": ["engineering", "marketing"],
      "default_account": "engineering"
    },
    {
      "type": "ntfy",
      "accounts": ["monitoring"],
      "default_account": "monitoring"
    }
  ]
}
```

**With Authentication - Support Role**:
```bash
curl -X GET http://localhost:8080/api/v1/notifiers \
  -H "Authorization: Bearer $SUPPORT_KEY"
```

```json
{
  "notifiers": [
    {
      "type": "email",
      "accounts": ["support"],
      "default_account": "support"
    },
    {
      "type": "slack",
      "accounts": ["marketing"],
      "default_account": "marketing"
    }
  ]
}
```

Note: The `ntfy` notifier type is completely hidden because the support role has no access to any ntfy accounts.

**With Authentication - Admin Role**:
```bash
curl -X GET http://localhost:8080/api/v1/notifiers \
  -H "Authorization: Bearer $ADMIN_KEY"
```

```json
{
  "notifiers": [
    {
      "type": "email",
      "accounts": ["primary", "support"],
      "default_account": "primary"
    },
    {
      "type": "slack",
      "accounts": ["engineering", "marketing"],
      "default_account": "engineering"
    },
    {
      "type": "ntfy",
      "accounts": ["monitoring"],
      "default_account": "monitoring"
    }
  ]
}
```

### Sending Notifications with RBAC

When a user sends a notification, they can only use notifiers they're authorized for:

**Valid Request** (support role, using support email):
```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "support",
    "subject": "Help needed",
    "body": "Please call...",
    "recipients": ["customer@example.com"]
  }'
# Returns: 200 OK
```

**Invalid Request** (support role, trying to use admin email):
```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "primary",  # Not allowed!
    "subject": "Alert",
    "body": "System down",
    "recipients": ["admin@example.com"]
  }'
# Returns: 403 Forbidden - Authorization denied
```

## Role Definitions

Common roles used in Notifier:

| Role | Description | Permissions |
|------|-------------|-------------|
| `admin` | Administrator | Full access to all notifiers and accounts |
| `notify-email` | Email notifications | Access to any email notifier marked for this role |
| `notify-slack` | Slack notifications | Access to any Slack workspace marked for this role |
| `notify-ntfy` | Ntfy notifications | Access to any Ntfy server marked for this role |
| `notify-all` | All notifications | Access to any notifier marked for this role |
| `support` | Support team | Access to support-specific notifiers |
| `engineering` | Engineering team | Access to engineering-specific notifiers |
| `ops` | Operations team | Access to ops-specific notifiers |

**Custom roles** are supported - you can define any role name you want in your configuration.

## Configuration Patterns

### Pattern 1: Role-Based Notifier Access

Restrict notifiers by team:

```yaml
notifiers:
  smtp:
    team-email:
      host: smtp.example.com
      from: team@example.com
      allowed_roles: [engineering, ops]

  slack:
    team-channel:
      webhook_url: https://hooks.slack.com/...
      allowed_roles: [engineering, ops]
```

Team members get keys with the `engineering` or `ops` role:
```bash
# Engineering member
curl -X POST http://localhost:8080/api/v1/admin/keys -d '{
  "client_id": "eng-service",
  "roles": ["engineering"]
}'
```

### Pattern 2: Public and Restricted Accounts

Some accounts are public, others restricted:

```yaml
notifiers:
  smtp:
    public:
      host: smtp.example.com
      from: public@example.com
      # No allowed_roles = all authenticated users

    restricted:
      host: smtp.example.com
      from: admin@example.com
      allowed_roles: [admin]  # Admin only
```

### Pattern 3: Multiple Roles Per Key

API keys can have multiple roles:

```bash
# Create a key for someone who needs email and slack
curl -X POST http://localhost:8080/api/v1/admin/keys -d '{
  "client_id": "team-integration",
  "roles": ["notify-email", "notify-slack"]
}'
```

This key can use any notifier that allows either `notify-email` OR `notify-slack`.

### Pattern 4: Service-Specific Roles

Grant minimal permissions to each service:

```yaml
notifiers:
  smtp:
    alerts:
      host: smtp.example.com
      from: alerts@example.com
      allowed_roles: [alerts-sender]  # Only alerting service

    billing:
      host: smtp.example.com
      from: billing@example.com
      allowed_roles: [billing-sender]  # Only billing service
```

Create minimally-permissioned keys:
```bash
# Alerts service - only send alerts
curl -X POST http://localhost:8080/api/v1/admin/keys -d '{
  "client_id": "alerts-service",
  "roles": ["alerts-sender"]
}'

# Billing service - only send billing emails
curl -X POST http://localhost:8080/api/v1/admin/keys -d '{
  "client_id": "billing-service",
  "roles": ["billing-sender"]
}'
```

## Authorization Flow

### Step 1: Configuration
Define which roles can access each notifier account:
```yaml
allowed_roles: [admin, ops]
```

### Step 2: API Key Creation
Create API keys with appropriate roles:
```bash
curl -X POST /api/v1/admin/keys -d '{
  "roles": ["ops"]
}'
```

### Step 3: Requests
User makes request with their API key:
```bash
curl -X GET /api/v1/notifiers \
  -H "Authorization: Bearer nk_user_key"
```

### Step 4: Authorization Check
System checks:
```
For each notifier account:
  - Get allowed_roles from config
  - If allowed_roles is empty: ALLOW (public)
  - If user has ANY of the allowed_roles: ALLOW
  - Otherwise: DENY (don't return in list)
```

### Step 5: Response
Only authorized accounts are returned:
```json
{
  "notifiers": [
    {
      "type": "email",
      "accounts": ["account-user-can-access"],
      "default_account": "account-user-can-access"
    }
  ]
}
```

## Default Account Selection

When no account is specified in a notification request, the default account is used. The default account selection respects RBAC:

1. System tries to use the configured default account
2. If user is not authorized for that account, the first authorized account is used
3. If user has no authorized accounts, request fails with 403

Example:
```yaml
notifiers:
  smtp:
    primary:    # This is the default_account
      host: smtp.example.com
      allowed_roles: [admin]

    backup:
      host: smtp.backup.com
      allowed_roles: [ops]
```

**Request from ops key** (no account specified):
- System wants to use `primary` (default) but ops isn't allowed
- Falls back to `backup` (first authorized account)
- Uses `backup`

## Testing RBAC

### Test 1: Verify Admin Can See All Accounts

```bash
curl -X GET http://localhost:8080/api/v1/notifiers \
  -H "Authorization: Bearer $ADMIN_KEY" | jq '.notifiers[].accounts'

# Should show: ["primary", "support"] for email, etc.
```

### Test 2: Verify Restricted User Sees Only Allowed

```bash
curl -X GET http://localhost:8080/api/v1/notifiers \
  -H "Authorization: Bearer $SUPPORT_KEY" | jq '.notifiers[] | select(.type=="email").accounts'

# Should show: ["support"] only
```

### Test 3: Verify Unauthorized Notifier Requests Fail

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "primary",
    "subject": "Test",
    "body": "Test",
    "recipients": ["test@example.com"]
  }'

# Should return: 403 Forbidden
```

### Test 4: Verify Allowed Requests Succeed

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "support",
    "subject": "Test",
    "body": "Test",
    "recipients": ["test@example.com"]
  }'

# Should return: 200 OK or 202 Accepted
```

## Security Best Practices

### DO

✅ **Use restrictive roles** - Grant only necessary permissions
```yaml
allowed_roles: [specific-team]  # Good
```

✅ **Create service-specific keys** - One key per service/app
```bash
curl -X POST /api/v1/admin/keys -d '{
  "client_id": "alerts-service",
  "roles": ["alerts-sender"]
}'
```

✅ **Rotate keys regularly** - Update roles periodically
```bash
# Create new key, migrate to it, revoke old key
```

✅ **Audit access** - Check API key usage and audit logs
```bash
curl -X GET /api/v1/admin/keys/nk_xxx/audit
```

✅ **Test authorization** - Verify RBAC works as expected
```bash
# Test with different keys/roles
```

### DON'T

❌ **Use permissive roles** - Don't give "admin" to non-admins
```yaml
allowed_roles: [admin]  # Bad - too permissive
```

❌ **Share keys between services** - Create separate keys
```bash
# Bad: Using same key for alerts and billing
# Good: One key per service
```

❌ **Leave roles empty if you want restrictions** - Empty means public
```yaml
allowed_roles: []  # Public! Only use if intentional
```

❌ **Grant unused roles** - Minimize attack surface
```bash
# Bad: roles: [admin, ops, support, everything]
# Good: roles: [ops]  # Only what's needed
```

## Troubleshooting

### User Sees No Notifiers

**Problem**: `GetNotifiers` returns empty list

**Cause**: User's roles don't match any notifier's `allowed_roles`

**Solution**:
1. Check user's API key roles: `curl /api/v1/admin/keys`
2. Check notifier config: `cat config.yaml | grep allowed_roles`
3. Verify at least one role matches
4. Add user's role to `allowed_roles` or give admin role

### Authorization Denied on Valid Request

**Problem**: 403 Forbidden when sending notification

**Cause**: User not authorized for the specific account

**Solution**:
1. Check account name in request
2. Check that account's `allowed_roles`
3. Verify user's key has one of those roles
4. Update `allowed_roles` in config or user's roles

### Default Account Not Used

**Problem**: Specified default account not being used

**Cause**: User not authorized for default account

**Solution**:
1. Check that user is authorized for default account
2. Or don't specify account and system picks first authorized
3. Or change default account to one user has access to

## Related Documentation

- [Authentication Guide](./AUTH.md) - How authentication works
- [API Key Management](./KEY_MANAGEMENT.md) - Creating and managing keys
- [Configuration Guide](./CONFIG.md) - Configuration options
