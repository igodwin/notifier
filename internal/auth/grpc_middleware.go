package auth

import (
	"context"
	"strings"

	"github.com/igodwin/notifier/internal/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCAuthMiddleware provides authentication for gRPC APIs
type GRPCAuthMiddleware struct {
	store  *APIKeyStore
	logger *logging.Logger
}

// NewGRPCAuthMiddleware creates a new gRPC auth middleware
func NewGRPCAuthMiddleware(store *APIKeyStore, logger *logging.Logger) *GRPCAuthMiddleware {
	return &GRPCAuthMiddleware{
		store:  store,
		logger: logger,
	}
}

// UnaryInterceptor returns a unary server interceptor for gRPC authentication
func (m *GRPCAuthMiddleware) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract API key from metadata
		apiKey := m.extractAPIKey(ctx)
		if apiKey == "" {
			m.logger.Warnf("gRPC: Missing API key in request for method=%s", info.FullMethod)
			return nil, status.Error(codes.Unauthenticated, "Missing or invalid Authorization header")
		}

		// Validate API key
		key, err := m.store.ValidateKey(apiKey)
		if err != nil {
			m.logger.Warnf("gRPC: Invalid API key for method=%s - error=%v", info.FullMethod, err)
			return nil, status.Error(codes.Unauthenticated, "Invalid API key")
		}

		// Check rate limit
		allowed, err := m.store.CheckRateLimit(apiKey)
		if err != nil || !allowed {
			m.logger.Warnf("gRPC: Rate limit exceeded for client=%s method=%s", key.ClientID, info.FullMethod)
			return nil, status.Error(codes.ResourceExhausted, "Rate limit exceeded")
		}

		// Update last used timestamp
		if err := m.store.UpdateLastUsed(apiKey); err != nil {
			m.logger.Errorf("gRPC: Failed to update last used time for client=%s - error=%v", key.ClientID, err)
		}

		// Create auth context and attach to request
		authCtx := &AuthContext{
			APIKey:   key,
			ClientID: key.ClientID,
			Roles:    key.Roles,
		}

		// Add auth context to request context
		newCtx := ContextWithAuth(ctx, authCtx)
		m.logger.Debugf("gRPC: Authenticated request from client=%s method=%s with roles=%v", key.ClientID, info.FullMethod, key.Roles)

		return handler(newCtx, req)
	}
}

// StreamInterceptor returns a stream server interceptor for gRPC authentication
func (m *GRPCAuthMiddleware) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Extract API key from metadata
		apiKey := m.extractAPIKey(ss.Context())
		if apiKey == "" {
			m.logger.Warnf("gRPC: Missing API key in stream for method=%s", info.FullMethod)
			return status.Error(codes.Unauthenticated, "Missing or invalid Authorization header")
		}

		// Validate API key
		key, err := m.store.ValidateKey(apiKey)
		if err != nil {
			m.logger.Warnf("gRPC: Invalid API key for stream method=%s - error=%v", info.FullMethod, err)
			return status.Error(codes.Unauthenticated, "Invalid API key")
		}

		// Check rate limit
		allowed, err := m.store.CheckRateLimit(apiKey)
		if err != nil || !allowed {
			m.logger.Warnf("gRPC: Rate limit exceeded for client=%s stream method=%s", key.ClientID, info.FullMethod)
			return status.Error(codes.ResourceExhausted, "Rate limit exceeded")
		}

		// Update last used timestamp
		if err := m.store.UpdateLastUsed(apiKey); err != nil {
			m.logger.Errorf("gRPC: Failed to update last used time for client=%s - error=%v", key.ClientID, err)
		}

		// Create auth context and attach to request
		authCtx := &AuthContext{
			APIKey:   key,
			ClientID: key.ClientID,
			Roles:    key.Roles,
		}

		// Add auth context to request context
		newCtx := ContextWithAuth(ss.Context(), authCtx)
		m.logger.Debugf("gRPC: Authenticated stream from client=%s method=%s with roles=%v", key.ClientID, info.FullMethod, key.Roles)

		// Create wrapped server stream with new context
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: newCtx}
		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream wraps grpc.ServerStream to override context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// extractAPIKey extracts API key from gRPC metadata
func (m *GRPCAuthMiddleware) extractAPIKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Try authorization header first
	if authHeaders := md.Get("authorization"); len(authHeaders) > 0 {
		authHeader := authHeaders[0]
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Try x-api-key header
	if keyHeaders := md.Get("x-api-key"); len(keyHeaders) > 0 {
		return keyHeaders[0]
	}

	return ""
}
