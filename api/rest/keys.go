package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/igodwin/notifier/internal/auth"
	"github.com/igodwin/notifier/internal/logging"
)

// KeyManagementHandler handles API key management endpoints
type KeyManagementHandler struct {
	keyStore *auth.HybridKeyStore
	logger   *logging.Logger
}

// NewKeyManagementHandler creates a new key management handler
func NewKeyManagementHandler(keyStore *auth.HybridKeyStore, logger *logging.Logger) *KeyManagementHandler {
	return &KeyManagementHandler{
		keyStore: keyStore,
		logger:   logger,
	}
}

// CreateKeyRequest is the request body for creating a new API key
type CreateKeyRequest struct {
	ClientID  string   `json:"client_id"`
	Roles     []string `json:"roles"`
	RateLimit int      `json:"rate_limit,omitempty"`
	ExpiresIn string   `json:"expires_in,omitempty"` // Duration string like "8760h", "30d", "1h", etc.
}

// CreateKeyResponse is the response body when creating an API key
type CreateKeyResponse struct {
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	ClientID  string     `json:"client_id"`
	Roles     []string   `json:"roles"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RateLimit int        `json:"rate_limit"`
}

// ListKeysResponse is the response body for listing API keys
type ListKeysResponse struct {
	Keys []*KeyInfo `json:"keys"`
}

// KeyInfo contains metadata about an API key (without the key itself)
type KeyInfo struct {
	Key        string     `json:"key_preview"` // Only last 4 chars
	Name       string     `json:"name"`
	ClientID   string     `json:"client_id"`
	Roles      []string   `json:"roles"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	IsActive   bool       `json:"is_active"`
	RateLimit  int        `json:"rate_limit"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// CreateKey creates a new API key
// POST /api/v1/admin/keys
// Requires: admin role
func (h *KeyManagementHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check authorization - must have admin role
	authCtx, ok := auth.GetAuthContext(ctx)
	if !ok || !h.hasRole(authCtx, "admin") {
		h.respondError(w, http.StatusForbidden, "Insufficient permissions", "admin role required")
		return
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate request
	if req.ClientID == "" {
		h.respondError(w, http.StatusBadRequest, "Missing client_id", "")
		return
	}

	if len(req.Roles) == 0 {
		h.respondError(w, http.StatusBadRequest, "At least one role is required", "")
		return
	}

	// Default rate limit
	if req.RateLimit == 0 {
		req.RateLimit = 100
	}

	// Parse expires_in duration string if provided
	var expiresInDuration *time.Duration
	if req.ExpiresIn != "" {
		duration, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "Invalid expires_in format", fmt.Sprintf("expected duration format like '8760h' or '30d': %v", err))
			return
		}
		expiresInDuration = &duration
	}

	// Create the key
	apiKey, err := h.keyStore.CreateKey(ctx, req.ClientID, req.Roles, req.RateLimit, expiresInDuration, authCtx.ClientID)
	if err != nil {
		h.logger.Errorf("Failed to create API key: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to create API key", err.Error())
		return
	}

	resp := CreateKeyResponse{
		Key:       apiKey.Key,
		Name:      apiKey.Name,
		ClientID:  apiKey.ClientID,
		Roles:     apiKey.Roles,
		CreatedAt: apiKey.CreatedAt,
		ExpiresAt: apiKey.ExpiresAt,
		RateLimit: apiKey.RateLimit,
	}

	h.respondJSON(w, http.StatusCreated, resp)
	h.logger.Infof("Created API key for client %s", req.ClientID)
}

// ListKeys lists all API keys for the authenticated client
// GET /api/v1/admin/keys
// Requires: admin role (to list other users' keys)
func (h *KeyManagementHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authCtx, ok := auth.GetAuthContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	// Get client_id from query param, default to authenticated client
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = authCtx.ClientID
	}

	// If requesting other client's keys, require admin role
	if clientID != authCtx.ClientID && !h.hasRole(authCtx, "admin") {
		h.respondError(w, http.StatusForbidden, "Insufficient permissions", "admin role required to list other clients' keys")
		return
	}

	keys, err := h.keyStore.ListKeys(ctx, clientID)
	if err != nil {
		h.logger.Errorf("Failed to list API keys: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to list API keys", err.Error())
		return
	}

	// Convert to response format (hide full key)
	keyInfos := make([]*KeyInfo, len(keys))
	for i, key := range keys {
		keyInfos[i] = &KeyInfo{
			Key:        "nk_" + key.Key[len(key.Key)-4:], // Show only last 4 chars
			Name:       key.Name,
			ClientID:   key.ClientID,
			Roles:      key.Roles,
			CreatedAt:  key.CreatedAt,
			LastUsedAt: key.LastUsedAt,
			ExpiresAt:  key.ExpiresAt,
			IsActive:   key.IsActive,
			RateLimit:  key.RateLimit,
		}
	}

	h.respondJSON(w, http.StatusOK, ListKeysResponse{Keys: keyInfos})
}

// RevokeKeyRequest is the request body for revoking a key
type RevokeKeyRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RevokeKey deactivates an API key
// DELETE /api/v1/admin/keys/:name
// Requires: admin role
func (h *KeyManagementHandler) RevokeKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authCtx, ok := auth.GetAuthContext(ctx)
	if !ok || !h.hasRole(authCtx, "admin") {
		h.respondError(w, http.StatusForbidden, "Insufficient permissions", "admin role required")
		return
	}

	// Extract key name from path parameter (not the raw key, to avoid leaking secrets in URLs)
	vars := mux.Vars(r)
	keyName := vars["name"]

	var req RevokeKeyRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // Ignore decode errors, reason is optional

	err := h.keyStore.DeactivateKeyByName(ctx, keyName, authCtx.ClientID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "Key not found", "")
		} else {
			h.logger.Errorf("Failed to revoke API key: %v", err)
			h.respondError(w, http.StatusInternalServerError, "Failed to revoke API key", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Infof("Revoked API key")
}

// RotateKeyRequest is the request body for rotating a key
type RotateKeyRequest struct {
	PreserveRoles bool `json:"preserve_roles,omitempty"`
}

// RotateKey creates a new API key to replace the old one
// POST /api/v1/admin/keys/:name/rotate
// Requires: admin role
func (h *KeyManagementHandler) RotateKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authCtx, ok := auth.GetAuthContext(ctx)
	if !ok || !h.hasRole(authCtx, "admin") {
		h.respondError(w, http.StatusForbidden, "Insufficient permissions", "admin role required")
		return
	}

	vars := mux.Vars(r)
	_ = vars["name"] // Key name from URL (rotation not yet implemented)

	var req RotateKeyRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// For now, rotation means:
	// 1. Get the old key metadata
	// 2. Create a new key with same properties
	// 3. Deactivate old key
	// In a real implementation, you might want to keep both active for a grace period

	// Since we don't have direct key lookup in cache, return error
	h.respondError(w, http.StatusNotImplemented, "Key rotation not yet implemented", "Use revoke + create new key")
}

// GetAuditLogResponse is the response for audit log
type GetAuditLogResponse struct {
	Key      string                   `json:"key_preview"`
	AuditLog []map[string]interface{} `json:"audit_log"`
}

// GetAuditLog retrieves the audit log for a key
// GET /api/v1/admin/keys/:name/audit
// Requires: admin role
func (h *KeyManagementHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authCtx, ok := auth.GetAuthContext(ctx)
	if !ok || !h.hasRole(authCtx, "admin") {
		h.respondError(w, http.StatusForbidden, "Insufficient permissions", "admin role required")
		return
	}

	vars := mux.Vars(r)
	keyName := vars["name"]

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	logs, err := h.keyStore.GetAuditLogByName(ctx, keyName, limit)
	if err != nil {
		h.logger.Errorf("Failed to get audit log: %v", err)
		h.respondError(w, http.StatusInternalServerError, "Failed to get audit log", err.Error())
		return
	}

	resp := GetAuditLogResponse{
		Key:      keyName,
		AuditLog: logs,
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// Helper methods

// hasRole checks if the auth context has a specific role
func (h *KeyManagementHandler) hasRole(authCtx *auth.AuthContext, role string) bool {
	for _, r := range authCtx.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// respondJSON writes a JSON response
func (h *KeyManagementHandler) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error JSON response
func (h *KeyManagementHandler) respondError(w http.ResponseWriter, statusCode int, error string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := ErrorResponse{
		Error:   error,
		Message: message,
	}
	json.NewEncoder(w).Encode(resp)
}
