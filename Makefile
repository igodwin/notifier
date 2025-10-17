.PHONY: proto proto-gen proto-clean deps build run-grpc run-rest run-both test lint docker-build docker-run clean

# Variables
PROTO_DIR=api/grpc
PROTO_FILE=$(PROTO_DIR)/notifier.proto
PROTO_OUT=$(PROTO_DIR)/pb
GO_MODULE=$(shell head -n 1 go.mod | awk '{print $$2}')

# Generate protobuf code
proto-gen:
	@echo "Generating protobuf code..."
	@mkdir -p $(PROTO_OUT)
	protoc --go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)
	@echo "Protobuf code generated successfully"

# Clean generated protobuf code
proto-clean:
	@echo "Cleaning generated protobuf code..."
	@rm -rf $(PROTO_OUT)
	@echo "Cleaned"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

# Install protoc plugins
proto-deps:
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Protoc plugins installed"

# Build binaries
build:
	@echo "Building binaries..."
	@mkdir -p bin
	go build -o bin/grpcserver ./cmd/grpcserver
	go build -o bin/restserver ./cmd/restserver
	@echo "Binaries built successfully"

# Run gRPC server
run-grpc:
	@echo "Running gRPC server..."
	go run ./cmd/grpcserver/main.go

# Run REST server
run-rest:
	@echo "Running REST server..."
	go run ./cmd/restserver/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t notifier:latest .
	@echo "Docker image built successfully"

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -p 50051:50051 -v $(PWD)/config.yaml:/app/config.yaml notifier:latest

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf $(PROTO_OUT)
	@echo "Cleaned"

# Help
help:
	@echo "Available targets:"
	@echo "  proto-gen     - Generate protobuf code"
	@echo "  proto-clean   - Clean generated protobuf code"
	@echo "  proto-deps    - Install protoc plugins"
	@echo "  deps          - Install Go dependencies"
	@echo "  build         - Build binaries"
	@echo "  run-grpc      - Run gRPC server"
	@echo "  run-rest      - Run REST server"
	@echo "  test          - Run tests"
	@echo "  lint          - Run linter"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  clean         - Clean build artifacts"
