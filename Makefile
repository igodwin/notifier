.PHONY: proto proto-gen proto-clean deps build run run-grpc run-rest test lint fmt vet check docker-build docker-run clean help

# Variables
PROTO_DIR=api/grpc
PROTO_FILE=$(PROTO_DIR)/notifier.proto
PROTO_OUT=$(PROTO_DIR)/pb
GO_MODULE=$(shell head -n 1 go.mod | awk '{print $$2}')
GO_FILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./api/grpc/pb/*")

# Build information
VERSION ?= dev
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S_UTC')
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)

# Generate protobuf code
proto-gen:
	@echo "Generating protobuf code..."
	@mkdir -p $(PROTO_OUT)
	protoc -I. --go_out=. --go_opt=module=$(GO_MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(GO_MODULE) \
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

# Build binary
build:
	@echo "Building binary..."
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server
	@echo "Binary built successfully"

# Run server (default: both REST and gRPC)
run:
	@echo "Running server (both REST and gRPC)..."
	go run ./cmd/server/main.go

# Run in REST-only mode
run-rest:
	@echo "Running server in REST-only mode..."
	SERVER_MODE=rest go run ./cmd/server/main.go

# Run in gRPC-only mode
run-grpc:
	@echo "Running server in gRPC-only mode..."
	SERVER_MODE=grpc go run ./cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w $(GO_FILES)
	@echo "Code formatted"

# Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l $(GO_FILES))" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l $(GO_FILES); \
		exit 1; \
	fi
	@echo "All files are properly formatted"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "go vet passed"

# Run static analysis
check: fmt-check vet
	@echo "Running static checks..."
	go mod verify
	@echo "All checks passed"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: brew install golangci-lint" && exit 1)
	golangci-lint run ./...
	@echo "Linting passed"

# Run all quality checks
qa: fmt vet lint test
	@echo "All quality checks passed!"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t notifier:latest .
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
	@echo ""
	@echo "Build:"
	@echo "  build            - Build server binary"
	@echo "  clean            - Clean build artifacts"
	@echo ""
	@echo "Run:"
	@echo "  run              - Run server (both REST and gRPC)"
	@echo "  run-rest         - Run server in REST-only mode"
	@echo "  run-grpc         - Run server in gRPC-only mode"
	@echo ""
	@echo "Test:"
	@echo "  test             - Run tests with race detector"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt              - Format code with gofmt"
	@echo "  fmt-check        - Check if code is formatted"
	@echo "  vet              - Run go vet"
	@echo "  lint             - Run golangci-lint (requires installation)"
	@echo "  check            - Run fmt-check + vet + mod verify"
	@echo "  qa               - Run all quality checks (fmt + vet + lint + test)"
	@echo ""
	@echo "Protobuf:"
	@echo "  proto-gen        - Generate protobuf code"
	@echo "  proto-clean      - Clean generated protobuf code"
	@echo "  proto-deps       - Install protoc plugins"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps             - Install Go dependencies"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker container"
