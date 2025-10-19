# Build stage
FROM golang:1.24-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary with version information
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o server ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 notifier && \
    adduser -D -u 1000 -G notifier notifier

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/server /app/

# Copy default config (can be overridden with volume mount)
COPY config.yaml /app/config.yaml

# Create directory for queue persistence
RUN mkdir -p /var/lib/notifier && \
    chown -R notifier:notifier /var/lib/notifier

# Change to non-root user
USER notifier

# Expose ports
EXPOSE 8080 50051 9090 8081

# Run server (defaults to both REST and gRPC)
# Override mode with environment variable: -e SERVER_MODE=rest or -e SERVER_MODE=grpc
CMD ["/app/server"]
