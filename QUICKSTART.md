# Quick Start Guide

## Prerequisites

1. Go 1.23.2 or higher installed
2. Dependencies installed: `go mod tidy`

## Running the REST Server

### Option 1: Using go run
```bash
go run ./cmd/restserver/main.go
```

### Option 2: Build and run
```bash
go build -o bin/restserver ./cmd/restserver
./bin/restserver
```

### Option 3: Using Make
```bash
make build
make run-rest
```

The server will start on `http://localhost:8080` by default.

## Testing with cURL

### 1. Health Check
```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "service": "notifier",
  "time": "2025-10-16T21:05:27Z"
}
```

### 2. Send a Simple Notification
```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "stdout",
    "subject": "Hello World",
    "body": "This is a test notification!",
    "recipients": ["console"]
  }'
```

Expected response:
```json
{
  "result": {
    "notification_id": "abc123...",
    "success": true,
    "message": "notification queued successfully",
    "sent_at": "2025-10-16T21:05:27Z"
  }
}
```

The notification will be printed to the server's stdout.

### 3. Send with Priority
```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "stdout",
    "priority": 4,
    "subject": "CRITICAL Alert",
    "body": "This is a critical notification!",
    "recipients": ["console"],
    "max_retries": 5
  }'
```

Priority levels:
- `0` - Low
- `1` - Normal (default)
- `2` - High
- `3` - Critical

### 4. Send Batch Notifications
```bash
curl -X POST http://localhost:8080/api/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "type": "stdout",
        "subject": "Notification 1",
        "body": "First notification",
        "recipients": ["console"]
      },
      {
        "type": "stdout",
        "subject": "Notification 2",
        "body": "Second notification",
        "recipients": ["console"]
      }
    ]
  }'
```

### 5. Get Notification Statistics
```bash
curl http://localhost:8080/api/v1/stats
```

Expected response:
```json
{
  "total_sent": 5,
  "total_failed": 0,
  "total_pending": 0,
  "total_queued": 0,
  "by_type": {
    "stdout": 5
  },
  "by_status": {
    "sent": 5
  },
  "average_latency_ms": 0
}
```

### 6. List Notifications
```bash
# List all notifications
curl http://localhost:8080/api/v1/notifications

# List with filters
curl "http://localhost:8080/api/v1/notifications?type=stdout&status=sent&limit=10"
```

### 7. Get Specific Notification
```bash
curl http://localhost:8080/api/v1/notifications/{notification-id}
```

## Testing with Other Notifiers

### SMTP (Email)
Update `notifier.config` with named accounts:
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
      default: true
    work:
      host: "smtp.company.com"
      port: 587
      username: "you@company.com"
      password: "your-work-password"
      from: "notifications@company.com"
      use_tls: true
```

Then send:
```bash
# Uses default account (personal)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Test Email",
    "body": "This is a test email!",
    "recipients": ["recipient@example.com"]
  }'

# Specify account explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "work",
    "subject": "Test Email",
    "body": "This is a test email!",
    "recipients": ["recipient@example.com"]
  }'
```

### Slack
Update `notifier.config` with named workspaces:
```yaml
notifiers:
  slack:
    main:
      webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
      username: "Notifier Bot"
      icon_emoji: ":bell:"
      default: true
    team-a:
      webhook_url: "https://hooks.slack.com/services/TEAM-A/WEBHOOK/URL"
      username: "Team A Bot"
      icon_emoji: ":rocket:"
```

Then send:
```bash
# Uses default workspace (main)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "slack",
    "subject": "Deployment Alert",
    "body": "Application deployed successfully to production!",
    "recipients": ["#alerts"]
  }'

# Specify workspace explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "slack",
    "account": "team-a",
    "subject": "Deployment Alert",
    "body": "Application deployed successfully!",
    "recipients": ["#alerts"]
  }'
```

### Ntfy
Update `notifier.config` with named servers:
```yaml
notifiers:
  ntfy:
    public:
      server_url: "https://ntfy.sh"
      default: true
    private:
      server_url: "https://ntfy.mycompany.com"
      username: "your-username"
      password: "your-password"
```

Then send:
```bash
# Uses default server (public)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "subject": "Mobile Alert",
    "body": "This will appear on your phone!",
    "recipients": ["mytopic"],
    "metadata": {
      "tags": ["warning", "skull"]
    }
  }'

# Specify server explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "account": "private",
    "subject": "Mobile Alert",
    "body": "This will appear on your phone!",
    "recipients": ["mytopic"]
  }'
```

## Configuration

### Using Environment Variables
```bash
export NOTIFIER_SERVER_REST_PORT=9000
export NOTIFIER_QUEUE_WORKER_COUNT=20
export NOTIFIER_NOTIFIERS_SMTP_HOST=smtp.gmail.com
export NOTIFIER_NOTIFIERS_SMTP_PASSWORD=secret

./bin/restserver
```

### Using notifier.config
Create or modify `notifier.config` in the project root:
```yaml
server:
  rest_port: 8080
  host: "0.0.0.0"

queue:
  type: "local"
  worker_count: 10

notifiers:
  stdout: true
```

## Docker Deployment

### Build Docker Image
```bash
docker build -t notifier:latest .
```

### Run with Docker
```bash
docker run -p 8080:8080 \
  -v $(pwd)/notifier.config:/app/notifier.config \
  notifier:latest
```

### Using Docker Compose
```bash
docker-compose up
```

## Kubernetes Deployment

### Deploy to Kubernetes
```bash
# Apply all resources
kubectl apply -f k8s/

# Check status
kubectl get pods -l app=notifier
kubectl get svc -l app=notifier

# View logs
kubectl logs -f -l app=notifier

# Port forward to access locally
kubectl port-forward svc/notifier-rest 8080:8080
```

## Next Steps

1. **Add Authentication**: Implement API key or OAuth authentication
2. **Add Email Templates**: Support for templated notifications
3. **Add Webhooks**: Get callbacks when notifications are sent/failed
4. **Add Persistence**: Store notifications in a database
5. **Add Kafka Queue**: For distributed processing
6. **Add Metrics**: Instrument with Prometheus metrics
7. **Add Tests**: Write unit and integration tests

## Troubleshooting

### Server won't start
- Check if port 8080 is already in use: `lsof -i :8080`
- Check notifier.config syntax
- Verify all dependencies are installed: `go mod tidy`

### Notifications not sending
- Check server logs for errors
- Verify the notifier is enabled in notifier.config
- For SMTP: Verify credentials and allow less secure apps
- For Slack: Verify webhook URL is correct
- For Ntfy: Ensure topic name is valid

### Queue filling up
- Increase worker count in notifier.config
- Check if notifiers are failing
- Review retry configuration

## API Reference

Full API documentation available in `ARCHITECTURE.md`.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /health | Health check |
| POST | /api/v1/notifications | Send notification |
| POST | /api/v1/notifications/batch | Send batch |
| GET | /api/v1/notifications | List notifications |
| GET | /api/v1/notifications/{id} | Get notification |
| DELETE | /api/v1/notifications/{id} | Cancel notification |
| POST | /api/v1/notifications/{id}/retry | Retry notification |
| GET | /api/v1/stats | Get statistics |
