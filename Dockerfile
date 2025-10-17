# Build stage
FROM golang:1.23.2-alpine AS builder

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

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o grpcserver ./cmd/grpcserver
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o restserver ./cmd/restserver

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 notifier && \
    adduser -D -u 1000 -G notifier notifier

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/grpcserver /app/
COPY --from=builder /build/restserver /app/

# Copy default config (can be overridden with volume mount)
COPY config.yaml /app/config.yaml

# Create directory for queue persistence
RUN mkdir -p /var/lib/notifier && \
    chown -R notifier:notifier /var/lib/notifier

# Change to non-root user
USER notifier

# Expose ports
EXPOSE 8080 50051 9090 8081

# Default to running both servers
# Can be overridden with docker run command
CMD ["sh", "-c", "/app/grpcserver & /app/restserver"]
