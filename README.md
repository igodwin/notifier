# Notifier Microservice

A production-ready notification delivery microservice that supports multiple notification channels (Email, Slack, Ntfy, Stdout) with both REST and gRPC APIs running simultaneously.

[![Go Version](https://img.shields.io/badge/go-1.23.2-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Overview

Notifier is a cloud-native microservice designed to handle all your application's notification needs through a single, unified API. Send emails, Slack messages, push notifications, and more with a consistent interface and robust delivery guarantees.

Perfect for:
- Microservices architectures needing centralized notifications
- Applications requiring multiple notification channels
- Systems needing reliable async notification delivery
- Teams wanting to standardize notification handling

## Features

### Core Capabilities
- 🔔 **Multi-Channel Support**: Email (SMTP), Slack, Ntfy.sh, and Stdout
- 🚀 **Dual API**: REST and gRPC running simultaneously in one process
- 📦 **Queue-Based**: Async processing with configurable worker pools
- 🔄 **Retry Logic**: Exponential backoff with configurable attempts
- ⚡ **Priority Levels**: Low, Normal, High, and Critical
- 📊 **Batch Operations**: Send multiple notifications efficiently
- 🎯 **Status Tracking**: Monitor notification lifecycle and delivery

### Production Features
- ⚙️ **Configuration**: Viper-based with environment variable support
- 🐳 **Containerized**: Multi-stage Docker builds with non-root user
- ☸️ **Kubernetes Ready**: Complete manifests with HPA, health checks, and RBAC
- 🔒 **Secure**: Token-based auth for ntfy, TLS support, secret management
- 📈 **Observable**: Health endpoints, metrics support, structured logging
- 🔌 **Extensible**: Clean interfaces for adding new notifiers

## Quick Start

### 1. Install and Run

```bash
# Clone and install dependencies
git clone https://github.com/igodwin/notifier
cd notifier
go mod tidy

# Build and run
make build
./bin/server
```

Server starts with:
- **REST API** on `http://localhost:8080`
- **gRPC API** on `localhost:50051`
- **Stdout notifier** enabled by default

**Run in different modes:**
```bash
./bin/server                    # Both REST and gRPC (default)
SERVER_MODE=rest ./bin/server   # REST only
SERVER_MODE=grpc ./bin/server   # gRPC only
```

### 2. Send Your First Notification

```bash
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "stdout",
    "subject": "Hello from Notifier!",
    "body": "Your first notification",
    "recipients": ["console"]
  }'
```

The notification appears immediately in the server output. ✅

### 3. Check Status

```bash
# Health check
curl http://localhost:8080/health

# Statistics
curl http://localhost:8080/api/v1/stats
```

## Configuration

### Basic Setup

Create `config.yaml` in the project root:

```yaml
server:
  mode: "both"        # Options: both, rest, grpc
  rest_port: 8080
  grpc_port: 50051
  host: "0.0.0.0"

queue:
  type: "local"       # Local in-memory queue
  worker_count: 10    # Concurrent workers
  retry_attempts: 3
  retry_backoff: "exponential"

notifiers:
  stdout: true        # Always enabled for testing
```

### Email Notifications (SMTP)

Supports multiple email accounts with named instances:

```yaml
notifiers:
  smtp:
    # Personal account (default)
    personal:
      host: "smtp.gmail.com"
      port: 587
      username: "your-email@gmail.com"
      password: "your-app-password"
      from: "personal@gmail.com"
      use_tls: true
      default: true

    # Work account
    work:
      host: "smtp.company.com"
      port: 587
      username: "you@company.com"
      password: "your-work-password"
      from: "notifications@company.com"
      use_tls: true
```

**Usage:**
```bash
# Uses default account (personal)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "subject": "Welcome!",
    "body": "Thanks for signing up",
    "recipients": ["user@example.com"]
  }'

# Specify account explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "account": "work",
    "subject": "Welcome!",
    "body": "Thanks for signing up",
    "recipients": ["user@example.com"]
  }'
```

### Slack Notifications

Supports multiple workspaces with named instances:

```yaml
notifiers:
  slack:
    # Main workspace (default)
    main:
      webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
      username: "Notifier Bot"
      icon_emoji: ":bell:"
      default: true

    # Team workspace
    team-a:
      webhook_url: "https://hooks.slack.com/services/TEAM-A/WEBHOOK/URL"
      username: "Team A Bot"
      icon_emoji: ":rocket:"
      # Channel-specific webhooks
      webhooks:
        "#alerts": "https://hooks.slack.com/services/ALERTS/WEBHOOK"
        "#monitoring": "https://hooks.slack.com/services/MONITORING/WEBHOOK"
```

**Usage:**
```bash
# Uses default workspace (main)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "slack",
    "subject": "Deployment Complete",
    "body": "v2.0 deployed to production",
    "recipients": ["#alerts"]
  }'

# Specify workspace explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "slack",
    "account": "team-a",
    "subject": "Deployment Complete",
    "body": "v2.0 deployed to production",
    "recipients": ["#alerts"]
  }'
```

### Ntfy Push Notifications

Supports multiple ntfy servers with named instances:

```yaml
notifiers:
  ntfy:
    # Public ntfy.sh server (default)
    public:
      server_url: "https://ntfy.sh"
      token: "tk_your_access_token"    # Optional, for private topics
      default_topic: "myapp-alerts"
      default: true

    # Private self-hosted server
    private:
      server_url: "https://ntfy.mycompany.com"
      username: "your-username"
      password: "your-password"
      default_topic: "company-alerts"
      insecure_skip_verify: false  # Set true for self-signed certs
```

**Usage:**
```bash
# Uses default server (public)
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "priority": 3,
    "subject": "Critical Alert",
    "body": "Server CPU at 95%",
    "recipients": ["alerts"],
    "metadata": {
      "tags": ["warning", "rotating_light"],
      "click": "https://dashboard.example.com"
    }
  }'

# Specify server explicitly
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "type": "ntfy",
    "account": "private",
    "subject": "Critical Alert",
    "body": "Server CPU at 95%",
    "recipients": ["alerts"]
  }'
```

See [docs/NTFY_GUIDE.md](docs/NTFY_GUIDE.md) for advanced ntfy features (action buttons, attachments, delays, etc.).

### Environment Variables

Override any config with environment variables:

```bash
export NOTIFIER_SERVER_MODE=both
export NOTIFIER_SERVER_REST_PORT=8080
export NOTIFIER_QUEUE_WORKER_COUNT=20
export NOTIFIER_NOTIFIERS_SMTP_HOST=smtp.gmail.com
export NOTIFIER_NOTIFIERS_SMTP_PASSWORD=secret
export NOTIFIER_NOTIFIERS_NTFY_TOKEN=tk_your_token
```

Format: `NOTIFIER_<SECTION>_<KEY>` (use `_` for nested keys)

## API Reference

### REST Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/notifications` | Send single notification |
| `POST` | `/api/v1/notifications/batch` | Send multiple notifications |
| `GET` | `/api/v1/notifications` | List notifications (with filters) |
| `GET` | `/api/v1/notifications/{id}` | Get notification by ID |
| `DELETE` | `/api/v1/notifications/{id}` | Cancel pending notification |
| `POST` | `/api/v1/notifications/{id}/retry` | Retry failed notification |
| `GET` | `/api/v1/stats` | Get service statistics |

### Request Format

```json
{
  "type": "email|slack|ntfy|stdout",
  "priority": 0-3,
  "subject": "Notification title",
  "body": "Notification body",
  "recipients": ["email@example.com", "#channel", "topic"],
  "metadata": {
    "key": "value"
  },
  "max_retries": 3
}
```

### Response Format

```json
{
  "result": {
    "notification_id": "uuid",
    "success": true,
    "message": "notification queued successfully",
    "sent_at": "2025-10-16T21:05:27Z"
  }
}
```

### Priority Levels

- `0` - Low (background notifications)
- `1` - Normal (default)
- `2` - High (important updates)
- `3` - Critical (urgent alerts)

### Batch Operations

```bash
curl -X POST http://localhost:8080/api/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"type": "email", "subject": "Welcome", ...},
      {"type": "slack", "subject": "Alert", ...}
    ]
  }'
```

### Filtering Notifications

```bash
# Get all sent email notifications
curl "http://localhost:8080/api/v1/notifications?type=email&status=sent&limit=10"

# Get recent failures
curl "http://localhost:8080/api/v1/notifications?status=failed&limit=20"
```

## gRPC API

The gRPC service mirrors the REST API with full feature parity. See [api/grpc/notifier.proto](api/grpc/notifier.proto) for definitions.

**Generate Go code:**
```bash
make proto-gen
# or
protoc --go_out=. --go-grpc_out=. api/grpc/notifier.proto
```

**Note:** gRPC server is running but handler implementation is pending. Protobuf definitions are complete.

## Deployment

### Docker

**Build (single-arch, local):**
```bash
make docker-build
```

**Build and push a multi-arch image (linux/amd64 + linux/arm64):**
```bash
# Pushes $REGISTRY/$IMAGE:$VERSION and :latest.
# Requires Docker buildx (bundled with Docker Desktop; on Linux also run
# `docker run --rm --privileged tonistiigi/binfmt --install all` once to
# register the QEMU emulators).
REGISTRY=registry.example.com/org VERSION=v0.1.3 make docker-build
```

Override `IMAGE` (default `notifier`) or `PLATFORMS` (default
`linux/amd64,linux/arm64`) as needed.

**Run:**
```bash
docker run -d \
  --name notifier \
  -p 8080:8080 \
  -p 50051:50051 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  notifier:latest
```

**Run in different modes:**
```bash
# Both REST and gRPC (default)
docker run -d -p 8080:8080 -p 50051:50051 notifier:latest

# REST only
docker run -d -p 8080:8080 -e SERVER_MODE=rest notifier:latest

# gRPC only
docker run -d -p 50051:50051 -e SERVER_MODE=grpc notifier:latest
```

**Docker Compose:**
```bash
docker-compose up -d
```

Includes optional services: Kafka, Prometheus, Grafana (commented out by default)

### Kubernetes

**Deploy:**
```bash
kubectl apply -f k8s/
```

**Included resources:**
- Deployment (3 replicas with rolling updates)
- Services (separate for REST and gRPC)
- ConfigMap (configuration management)
- HPA (auto-scaling 3-10 pods based on CPU/memory)
- Ingress (external access with TLS)
- Secret template (for credentials)

**Access locally:**
```bash
kubectl port-forward svc/notifier-rest 8080:8080
kubectl port-forward svc/notifier-grpc 50051:50051
```

**Using Kustomize:**
```bash
kubectl apply -k k8s/
```

## Architecture

### High-Level Design

```
┌──────────────┐     ┌──────────────┐
│ REST Client  │────▶│  REST API    │
└──────────────┘     │  (port 8080) │
                     └──────┬───────┘
┌──────────────┐            │
│ gRPC Client  │────▶│  gRPC API    │
└──────────────┘     │ (port 50051) │
                     └──────┬───────┘
                            ▼
                  ┌──────────────────┐
                  │ Notification     │
                  │ Service          │
                  └────────┬─────────┘
                           ▼
                  ┌──────────────────┐
                  │ Queue (Local)    │◀──┐
                  └────────┬─────────┘   │
                           ▼              │
                  ┌──────────────────┐   │
                  │ Worker Pool      │───┘
                  │ (10 workers)     │
                  └────────┬─────────┘
                           ▼
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
   ┌────────┐       ┌──────────┐     ┌──────────┐
   │ SMTP   │       │  Slack   │     │   Ntfy   │
   │Notifier│       │ Notifier │     │ Notifier │
   └────────┘       └──────────┘     └──────────┘
```

### Key Components

- **API Layer**: REST (Gorilla mux) and gRPC (Protocol Buffers)
- **Service Layer**: Business logic, validation, orchestration
- **Queue**: In-memory with optional disk persistence (Kafka planned)
- **Workers**: Concurrent processors with retry logic
- **Notifiers**: Pluggable providers implementing `domain.Notifier`
- **Config**: Viper-based with file + env var support

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## Project Structure

```
notifier/
├── api/
│   ├── grpc/
│   │   └── notifier.proto          # gRPC service definition
│   └── rest/
│       ├── handlers.go              # HTTP handlers
│       ├── router.go                # Route configuration
│       └── types.go                 # Request/response types
├── cmd/
│   └── server/main.go              # Unified server (configurable mode)
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration management
│   ├── domain/
│   │   ├── notification.go         # Core types
│   │   ├── notifier.go            # Notifier interface
│   │   └── queue.go               # Queue interface
│   ├── notifier/
│   │   ├── notifier.go            # Factory & base
│   │   ├── smtp.go                # Email notifier
│   │   ├── slack.go               # Slack notifier
│   │   ├── ntfy.go                # Ntfy notifier
│   │   └── stdout.go              # Stdout notifier
│   ├── queue/
│   │   └── local.go               # In-memory queue
│   └── service/
│       └── service.go             # Business logic
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   └── kustomization.yaml
├── docs/
│   └── NTFY_GUIDE.md              # Ntfy integration guide
├── config.yaml                 # Default configuration
├── docker-compose.yaml
├── Dockerfile
├── Makefile
├── QUICKSTART.md                   # API examples
├── ARCHITECTURE.md                 # Design documentation
└── README.md                       # This file
```

## Development

### Build Commands

```bash
make build          # Build server binary
make run            # Run server (both REST and gRPC)
make run-rest       # Run server in REST-only mode
make run-grpc       # Run server in gRPC-only mode
make test           # Run tests with race detector
make test-coverage  # Generate HTML coverage report
make fmt            # Format code with gofmt
make vet            # Run go vet
make lint           # Run golangci-lint
make check          # Run fmt-check + vet + mod verify
make qa             # Run all quality checks
make proto-gen      # Generate protobuf code
make docker-build   # Build Docker image (multi-arch + push when DOCKER_REGISTRY is set)
make clean          # Clean build artifacts
make help           # Show all available targets
```

### Adding a New Notifier

1. Create `internal/notifier/mynotifier.go`
2. Implement `domain.Notifier` interface
3. Add config struct to `internal/config/config.go`
4. Register in `cmd/server/main.go`
5. Update `config.yaml` with example config
6. Add tests

Example:
```go
type MyNotifier struct {
    BaseNotifier
    config *MyNotifierConfig
}

func (m *MyNotifier) Send(ctx context.Context, n *domain.Notification) (*domain.NotificationResult, error) {
    // Implementation
}
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed guide.

## Documentation

- [QUICKSTART.md](QUICKSTART.md) - Quick start guide with examples
- [ARCHITECTURE.md](ARCHITECTURE.md) - Architecture and design details
- [docs/NTFY_GUIDE.md](docs/NTFY_GUIDE.md) - Ntfy integration guide
- [api/grpc/notifier.proto](api/grpc/notifier.proto) - gRPC API definition

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/notifier/...
```

**Current Status:** Core implementation complete, tests pending.

## Monitoring & Operations

### Health Checks

```bash
curl http://localhost:8080/health
```

Returns:
```json
{
  "status": "healthy",
  "service": "notifier",
  "time": "2025-10-16T21:05:27Z"
}
```

### Statistics

```bash
curl http://localhost:8080/api/v1/stats
```

Returns:
```json
{
  "total_sent": 1234,
  "total_failed": 5,
  "total_pending": 0,
  "total_queued": 2,
  "by_type": {
    "email": 800,
    "slack": 400,
    "ntfy": 34
  },
  "by_status": {
    "sent": 1234,
    "failed": 5
  }
}
```

### Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` gracefully:
- Stops accepting new requests
- Completes in-flight notifications
- Drains the queue
- Closes all connections
- 30-second timeout

## Roadmap

### Completed ✅
- [x] Core notification system
- [x] REST API (fully functional)
- [x] gRPC API (protobuf defined)
- [x] Local queue with workers
- [x] Multiple notifiers (SMTP, Slack, Ntfy, Stdout)
- [x] Priority and retry logic
- [x] Batch operations
- [x] Docker support
- [x] Kubernetes manifests with HPA
- [x] Health checks and stats
- [x] Configuration management

### Planned 🚧
- [ ] gRPC handler implementation
- [ ] Kafka queue adapter
- [ ] Database persistence (PostgreSQL)
- [ ] Notification templates
- [ ] Webhook callbacks
- [ ] Authentication/Authorization (API keys, OAuth)
- [ ] Rate limiting (per client, per notifier)
- [ ] Prometheus metrics
- [ ] OpenTelemetry tracing
- [ ] Comprehensive test suite
- [ ] Circuit breakers for notifiers
- [ ] Dead letter queue
- [ ] Admin dashboard

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

Please follow Go best practices and maintain test coverage.

## Security

### Reporting Vulnerabilities

Please report security vulnerabilities privately via GitHub Security Advisories.

### Best Practices

- Store credentials in environment variables or secrets managers
- Use TLS for production deployments
- Enable authentication for ntfy private topics
- Rotate tokens regularly
- Follow principle of least privilege for K8s RBAC
- Keep dependencies updated

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.

## Support & Community

- **Issues**: [GitHub Issues](https://github.com/igodwin/notifier/issues)
- **Discussions**: [GitHub Discussions](https://github.com/igodwin/notifier/discussions)
- **Documentation**: See `docs/` directory
- **Examples**: See [QUICKSTART.md](QUICKSTART.md)

## Acknowledgments

Built with:
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP routing
- [Viper](https://github.com/spf13/viper) - Configuration
- [gRPC-Go](https://github.com/grpc/grpc-go) - RPC framework
- [Ntfy](https://ntfy.sh) - Push notifications

---

**Made with ❤️ for reliable notifications**
