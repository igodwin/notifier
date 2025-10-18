# Notifier Microservice Architecture

## Overview

The Notifier microservice is designed to provide a flexible, scalable notification delivery system that supports multiple notification channels (SMTP, Slack, Ntfy, Stdout) with both REST and gRPC APIs.

## Architecture Principles

- **Clean Architecture**: Domain-driven design with clear separation of concerns
- **Interface-based Design**: All core components use interfaces for maximum flexibility
- **Pluggable Notifiers**: Easy to add new notification providers
- **Queue Abstraction**: Supports both local and distributed queues (Kafka)
- **Configuration-driven**: Viper-based configuration with environment variable support
- **Cloud-native**: Containerized with Kubernetes support

## Core Components

### 1. Domain Layer (`internal/domain/`)

The domain layer defines the core business logic and interfaces:

#### `notification.go`
- **Notification**: Core notification entity with metadata, status tracking, and retry logic
- **Priority**: Enumeration for notification urgency (Low, Normal, High, Critical)
- **NotificationType**: Supported notification channels (Email, Slack, Ntfy, Stdout)
- **NotificationStatus**: Lifecycle states (Pending, Queued, Processing, Sent, Failed, Retrying)
- **NotificationResult**: Outcome of notification delivery attempts
- **NotificationFilter**: Query interface for retrieving notifications

#### `notifier.go`
- **Notifier**: Core interface that all notification implementations must satisfy
- **NotifierFactory**: Factory pattern for creating notifier instances
- **NotificationService**: High-level service interface for notification operations
- **NotificationStats**: Statistics and metrics about notification processing

#### `queue.go`
- **Queue**: Interface for notification queue implementations
- **QueueMessage**: Wrapper around notifications with queue-specific metadata
- **QueueConfig**: Configuration for queue implementations
- **LocalQueueConfig**: In-memory queue configuration
- **KafkaQueueConfig**: Distributed Kafka queue configuration

### 2. Queue Implementation (`internal/queue/`)

#### `local.go`
- In-memory queue implementation using Go channels
- Optional disk persistence for durability
- Thread-safe with mutex protection
- Supports enqueue, dequeue, ack, and nack operations
- Configurable buffer size and retry behavior

### 3. Notifier Implementations (`internal/notifier/`)

#### `notifier.go`
- **Factory**: Manages and creates notifier instances
- **BaseNotifier**: Common functionality shared by all notifiers
- Validation and context checking utilities

#### `stdout.go`
- Simple stdout notifier for debugging and development
- Prints notifications to console with formatted output

#### `smtp.go`
- Email notifications via SMTP
- Supports TLS/SSL
- Configurable SMTP server, port, and authentication
- RFC-compliant email message formatting

#### `ntfy.go`
- Integration with ntfy.sh push notification service
- Supports custom ntfy servers
- Priority mapping and metadata support
- Rich notification features (tags, click actions, attachments)

#### `slack.go`
- Slack webhook integration
- Channel-specific webhook support
- Rich message formatting with blocks
- Priority indicators and custom branding

### 4. Configuration (`internal/config/`)

#### `config.go`
- Viper-based configuration management
- Support for YAML config files and environment variables
- Default value handling
- Comprehensive validation
- Hierarchical configuration structure:
  - Server settings (ports, host, mode)
  - Queue configuration
  - Notifier credentials
  - Logging settings
  - Metrics and observability
  - Health check configuration

### 5. API Layer

#### gRPC API (`api/grpc/`)

**`notifier.proto`**
- Protocol buffer definitions for the gRPC service
- Operations:
  - `SendNotification`: Send single notification
  - `SendBatchNotifications`: Send multiple notifications
  - `GetNotification`: Retrieve notification by ID
  - `ListNotifications`: Query notifications with filters
  - `CancelNotification`: Cancel pending notification
  - `RetryNotification`: Retry failed notification
  - `GetStats`: Retrieve service statistics
  - `HealthCheck`: Service health verification

#### REST API (`api/rest/`)

**`handlers.go`**
- HTTP handlers for REST endpoints
- Request validation and error handling
- JSON serialization/deserialization

**`router.go`**
- Gorilla mux router configuration
- Middleware for logging and CORS
- Route definitions matching gRPC operations

**`types.go`**
- REST API request/response types
- Domain model conversions
- Validation logic

### 6. Entry Points (`cmd/`)

#### `grpcserver/main.go`
- Standalone gRPC server
- Service initialization and dependency injection
- Graceful shutdown handling

#### `restserver/main.go`
- Standalone REST server
- HTTP server configuration
- Graceful shutdown handling

## Data Flow

### Sending a Notification

```
Client Request (REST/gRPC)
    ↓
API Handler
    ↓
NotificationService.Send()
    ↓
Queue.Enqueue()
    ↓
[Notification queued]
    ↓
Worker dequeues (Queue.Dequeue())
    ↓
NotifierFactory.Create()
    ↓
Notifier.Send()
    ↓
Provider API (SMTP/Slack/Ntfy/Stdout)
    ↓
Queue.Ack() or Queue.Nack()
    ↓
Update notification status
    ↓
[Notification sent or failed]
```

## Queue Strategies

### Local Queue
- In-memory implementation using Go channels
- Fast and simple for single-instance deployments
- Optional disk persistence for durability
- Suitable for development and small-scale production

### Kafka Queue (Future)
- Distributed queue for multi-instance deployments
- Guarantees delivery across service restarts
- Horizontal scalability
- Exactly-once semantics with idempotence
- Suitable for high-throughput production environments

## Notification Lifecycle

1. **Pending**: Notification created but not yet queued
2. **Queued**: Added to queue, waiting for processing
3. **Processing**: Worker has dequeued and is sending
4. **Sent**: Successfully delivered to provider
5. **Failed**: Delivery failed after max retries
6. **Retrying**: Temporarily failed, will retry

## Retry Strategy

- Configurable max retries (default: 3)
- Pluggable backoff strategies:
  - **Exponential**: 2^n delay between retries
  - **Linear**: Fixed increment between retries
  - **Fixed**: Constant delay between retries

## Configuration Management

### Hierarchy (highest to lowest priority)
1. Environment variables (prefixed with `NOTIFIER_`)
2. Configuration file (notifier.config)
3. Default values

### Example Environment Variables
```bash
NOTIFIER_SERVER_GRPC_PORT=50051
NOTIFIER_SERVER_REST_PORT=8080
NOTIFIER_QUEUE_TYPE=local
NOTIFIER_NOTIFIERS_SMTP_HOST=smtp.gmail.com
NOTIFIER_NOTIFIERS_SLACK_WEBHOOK_URL=https://hooks.slack.com/...
```

## Deployment Strategies

### Docker Compose
- Single-host deployment
- All components in one compose file
- Optional Kafka, Prometheus, and Grafana services
- Volume mounts for configuration and persistence

### Kubernetes
- Multi-instance deployment with HPA
- Separate services for REST and gRPC
- ConfigMaps for configuration
- Secrets for credentials
- Ingress for external access
- Health checks and readiness probes
- Resource limits and requests

### Scaling Considerations

#### Horizontal Scaling
- Multiple instances can run concurrently
- Use Kafka queue for distributed message processing
- Stateless design allows easy scaling

#### Vertical Scaling
- Increase worker count per instance
- Tune queue buffer sizes
- Adjust resource limits

## Observability

### Metrics (Prometheus)
- Total notifications sent/failed
- Notifications by type and status
- Average latency
- Queue size
- Worker utilization

### Health Checks
- Dedicated health endpoint
- Component-level health reporting
- Kubernetes liveness/readiness probes

### Logging
- Structured JSON logging
- Configurable log levels
- Request/response logging
- Error tracking

## Security

### Authentication
- API keys for REST endpoints (to be implemented)
- mTLS for gRPC (to be implemented)
- Kubernetes RBAC for service account

### Authorization
- Per-notifier credential management
- Secrets stored in Kubernetes Secrets or external secret manager
- No credentials in configuration files

### Network Security
- Non-root container user
- Read-only root filesystem where possible
- Minimal container image (Alpine-based)
- Security context restrictions

## Extension Points

### Adding a New Notifier

1. Create new file in `internal/notifier/`
2. Implement the `domain.Notifier` interface:
   - `Send(ctx, notification) -> result`
   - `Type() -> NotificationType`
   - `Validate(notification) -> error`
   - `Close() -> error`
3. Add configuration struct to `internal/config/`
4. Register in factory during initialization
5. Update protobuf and REST API types
6. Add configuration example to `notifier.config`

### Adding a New Queue Implementation

1. Create new file in `internal/queue/`
2. Implement the `domain.Queue` interface
3. Add configuration to `domain.QueueConfig`
4. Update queue factory/initialization logic
5. Add configuration example

## Next Steps for Implementation

1. **Generate Protocol Buffers**: Run `make proto-gen` to generate gRPC code
2. **Implement Service Layer**: Create the main service that ties everything together
3. **Implement Server Main Functions**: Wire up dependencies in `cmd/`
4. **Add Tests**: Unit tests for notifiers, integration tests for queue
5. **Add Kafka Queue**: Implement distributed queue using Kafka
6. **Add Metrics**: Prometheus instrumentation
7. **Add Authentication**: API key or OAuth support
8. **Add Rate Limiting**: Prevent abuse and manage provider quotas
9. **Add Notification Templates**: Support for templated messages
10. **Add Webhooks**: Allow callbacks on notification status changes
