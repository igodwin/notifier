
# Notifier Module

The **Notifier** module provides a flexible notification system that supports multiple notification modes (e.g., SMTP, SMS, Slack, Ntfy, and Stdout). It can operate as a standalone gRPC or REST microservice, or be used as an imported module in other projects.

## Features

- **Multi-Mode Notification**: Supports different notification methods including SMTP email, SMS, Slack, Ntfy, and standard output.
- **Extensible Design**: Add new notification modes by implementing the `Notifier` interface.
- **gRPC and REST APIs**: Accessible through both gRPC and RESTful APIs, allowing easy integration into various systems.
- **Configuration Management**: Easily configurable for different environments and API keys.

## Project Structure

```plaintext
notifier/
├── api/
│   ├── grpc/               # gRPC service definitions and generated code
│   └── rest/               # REST API handlers and router
├── cmd/
│   ├── grpcserver/         # Entrypoint for running the gRPC server
│   └── restserver/         # Entrypoint for running the REST server
├── internal/
│   ├── config/             # Configuration loading
│   └── notifier/           # Core notification logic with multiple implementations
├── pkg/
│   ├── grpcclient/         # gRPC client for interacting with the notifier service
│   └── restclient/         # REST client for interacting with the notifier service
├── Dockerfile              # Docker configuration for deploying as a microservice
├── LICENSE                 # License file
├── README.md               # Documentation
└── go.mod                  # Defines the module's dependencies, module path, and Go version
```

## Getting Started

### Prerequisites

- **Go**: Install Go 1.18 or higher.
- **Protocol Buffers**: Required if you want to regenerate gRPC code.
- **Docker** (optional): For containerized deployment.

### Installation

To install the notifier module as a dependency in another Go project, run:

```bash
go get github.com/igodwin/notifier
```

### Running the Service

#### gRPC Server

To start the gRPC server:

```bash
cd cmd/grpcserver
go run main.go
```

#### REST Server

To start the REST server:

```bash
cd cmd/restserver
go run main.go
```

### Configuration

Configure notification modes via environment variables or a configuration file (e.g., `config.yaml`). Configuration settings may include:

- **SMTP**: SMTP server details, port, credentials.
- **SMS**: SMS provider API keys.
- **Slack**: Slack webhook URLs.
- **Ntfy**: Ntfy service URL and options.

For example, using a `config.yaml`:

```yaml
smtp:
  server: "smtp.example.com"
  port: 587
  username: "user@example.com"
  password: "password"

slack:
  webhook_url: "https://hooks.slack.com/services/..."
```

### Usage

#### Using gRPC API

1. Define a gRPC client using the `pkg/grpcclient` package.
2. Call `SendNotification` to send a message through a chosen notification method.

Example:

```go
client, err := grpcclient.NewClient("localhost:50051")
if err != nil {
    log.Fatalf("Failed to create gRPC client: %v", err)
}

resp, err := client.SendNotification("recipient@example.com", "Hello via gRPC!")
if err != nil {
    log.Fatalf("Failed to send notification: %v", err)
}
```

#### Using REST API

1. Define a REST client using the `pkg/restclient` package.
2. Call `SendNotification` via REST.

Example:

```go
client := restclient.NewClient("http://localhost:8080")
err := client.SendNotification("recipient@example.com", "Hello via REST!")
if err != nil {
    log.Fatalf("Failed to send notification: %v", err)
}
```

### Implementing New Notification Modes

To add a new notification method, implement the `Notifier` interface in `internal/notifier/`:

```go
type Notifier interface {
    Send(notification Notification) error
}
```

Add the new mode (e.g., `sms.go`) with the `Send` method to implement custom logic.

## Contributing

Contributions are welcome! To contribute, fork the repository, make your changes, and submit a pull request.

1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/NewMode`).
3. Commit your changes (`git commit -am 'Add new notification mode'`).
4. Push to the branch (`git push origin feature/NewMode`).
5. Create a new Pull Request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
