# RBAC Quick Start

## 60-Second Overview

Role-Based Access Control (RBAC) restricts which notifiers authenticated users can see and use.

### Configuration

```yaml
notifiers:
  smtp:
    primary:
      host: smtp.example.com
      allowed_roles: [admin, ops]    # Only these roles can use
    support:
      host: smtp.example.com
      allowed_roles: [support]        # Only support can use
```

### Create Keys with Roles

```bash
# Admin key - access to all
curl -X POST /api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"client_id": "admin", "roles": ["admin"]}'

# Support key - limited access
curl -X POST /api/v1/admin/keys \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"client_id": "support", "roles": ["support"]}'
```

### Get Authorized Notifiers

Admin sees all:
```bash
curl -X GET /api/v1/notifiers \
  -H "Authorization: Bearer $ADMIN_KEY"
```
Returns: `["primary", "support"]`

Support sees only theirs:
```bash
curl -X GET /api/v1/notifiers \
  -H "Authorization: Bearer $SUPPORT_KEY"
```
Returns: `["support"]`

### Send Notifications

Use authorized accounts:
```bash
# ✅ Allowed
curl -X POST /api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -d '{"type":"email", "account":"support", ...}'

# ❌ Forbidden
curl -X POST /api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -d '{"type":"email", "account":"primary", ...}'
# 403 Authorization denied
```

## Key Concepts

| Term | Meaning |
|------|---------|
| **Role** | A permission label (e.g., "admin", "support", "engineering") |
| **allowed_roles** | Config field listing which roles can use an account |
| **API Key Roles** | Roles assigned to a key when created |
| **Authorization** | System checks if user's roles match account's allowed_roles |

## Common Patterns

### Pattern 1: By Team
```yaml
notifiers:
  slack:
    engineering:
      allowed_roles: [engineering]
    marketing:
      allowed_roles: [marketing]
    admin:
      allowed_roles: [admin]
```

### Pattern 2: By Service (Least Privilege)
```yaml
notifiers:
  smtp:
    alerts:
      allowed_roles: [alerts-service]
    billing:
      allowed_roles: [billing-service]
```

### Pattern 3: Public + Private
```yaml
notifiers:
  smtp:
    public:
      # No allowed_roles = all authenticated users
    private:
      allowed_roles: [admin]
```

## Authorization Logic (Simple)

```
User has role "support"

For account "primary":
  allowed_roles: [admin, ops]
  Does "support" match? NO → NOT VISIBLE

For account "support":
  allowed_roles: [support]
  Does "support" match? YES → VISIBLE
```

## Troubleshooting

### User Sees No Notifiers
- Check user's key roles: `curl /api/v1/admin/keys -H "Authorization: Bearer $KEY"`
- Check notifier config: `grep allowed_roles config.yaml`
- Ensure at least one role matches

### 403 When Sending Notification
- Check account name in request
- Verify that account's allowed_roles include your key's roles
- Or try without specifying account (uses default)

### Default Account Not Working
- If user not authorized for default, first authorized account is used
- Or specify the account explicitly

## Real-World Example

**Config** (`config.yaml`):
```yaml
notifiers:
  smtp:
    prod-alerts:
      host: smtp.example.com
      from: alerts@example.com
      allowed_roles: [ops, admin]  # Only ops and admin

    support-email:
      host: smtp.example.com
      from: support@example.com
      allowed_roles: [support]     # Only support
```

**Create Keys**:
```bash
# Ops team
curl -X POST /api/v1/admin/keys -d '{
  "client_id": "ops-monitor",
  "roles": ["ops"]
}'

# Support team
curl -X POST /api/v1/admin/keys -d '{
  "client_id": "support-alerts",
  "roles": ["support"]
}'
```

**Usage**:
```bash
# Ops can send prod alerts
curl -X POST /api/v1/notifications \
  -H "Authorization: Bearer $OPS_KEY" \
  -d '{
    "type": "email",
    "account": "prod-alerts",
    "recipients": ["ops@example.com"]
  }'
# ✅ Works

# Support can send support emails
curl -X POST /api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -d '{
    "type": "email",
    "account": "support-email",
    "recipients": ["customer@example.com"]
  }'
# ✅ Works

# Support cannot send prod alerts
curl -X POST /api/v1/notifications \
  -H "Authorization: Bearer $SUPPORT_KEY" \
  -d '{
    "type": "email",
    "account": "prod-alerts",
    "recipients": ["ops@example.com"]
  }'
# ❌ 403 Forbidden
```

## Security Tips

✅ **DO**:
- Create specific roles for each team/service
- Grant minimal required roles to each key
- Use "admin" only when necessary
- Rotate keys regularly

❌ **DON'T**:
- Give everyone "admin" role
- Share keys between services
- Leave restrictions empty if you want to restrict
- Grant roles you don't need

## Full Docs

See **`docs/RBAC.md`** for complete documentation.
