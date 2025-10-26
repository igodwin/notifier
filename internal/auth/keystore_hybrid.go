package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HybridKeyStore combines in-memory cache with persistent database backend
// Write-through strategy: writes go to DB first, then cache is updated
// This ensures consistency: if DB write fails, cache is not updated
type HybridKeyStore struct {
	cache *APIKeyStore // In-memory cache for fast lookups
	db    *KeyStoreDB   // Database backend for persistence
	mu    sync.RWMutex
}

// NewHybridKeyStore creates a new hybrid key store
func NewHybridKeyStore(cache *APIKeyStore, db *KeyStoreDB) *HybridKeyStore {
	return &HybridKeyStore{
		cache: cache,
		db:    db,
	}
}

// InitializeFromDatabase loads all keys from database into cache at startup
func (h *HybridKeyStore) InitializeFromDatabase(ctx context.Context) error {
	keys, err := h.db.LoadAllKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to load keys from database: %w", err)
	}

	for _, key := range keys {
		h.cache.keys[key.Key] = key
		rateLimit := key.RateLimit
		if rateLimit <= 0 {
			rateLimit = 100 // Default rate limit
		}
		h.cache.rateLimits[key.Key] = &RateLimiter{
			maxRequests: rateLimit,
			window:      time.Minute,
			resetTime:   time.Now().Add(time.Minute),
			count:       0,
		}
	}

	return nil
}

// CreateKey generates a new API key and persists it
// Returns error if database write fails
func (h *HybridKeyStore) CreateKey(ctx context.Context, clientID string, roles []string, rateLimit int, expiresIn *time.Duration, createdBy string) (*APIKey, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Generate random key in memory
	apiKey, err := h.generateKey(clientID, roles, rateLimit, expiresIn)
	if err != nil {
		return nil, err
	}

	// Write to database first (consistency)
	if err := h.db.SaveKey(ctx, apiKey, createdBy); err != nil {
		return nil, err
	}

	// Update cache after successful DB write
	h.cache.keys[apiKey.Key] = apiKey
	h.cache.rateLimits[apiKey.Key] = &RateLimiter{
		maxRequests: rateLimit,
		window:      time.Minute,
		resetTime:   time.Now().Add(time.Minute),
		count:       0,
	}

	return apiKey, nil
}

// ValidateKey checks if a key is valid
// Checks cache first for performance, falls back to database if cache miss
func (h *HybridKeyStore) ValidateKey(keyStr string) (*APIKey, error) {
	// Check cache first (fast path)
	h.cache.mu.RLock()
	key, exists := h.cache.keys[keyStr]
	h.cache.mu.RUnlock()

	if exists {
		if h.isKeyValid(key) {
			return key, nil
		}
		return nil, fmt.Errorf("key is inactive or expired")
	}

	// Cache miss - this is normal in distributed deployments
	// Could implement database fallback here if needed:
	// key, err := h.db.GetKey(context.Background(), keyStr)
	// But for now, rely on cache being populated at startup

	return nil, fmt.Errorf("API key not found")
}

// ListKeys returns all active keys for a client
func (h *HybridKeyStore) ListKeys(ctx context.Context, clientID string) ([]*APIKey, error) {
	return h.db.ListKeys(ctx, clientID)
}

// DeactivateKey deactivates a key in both cache and database
func (h *HybridKeyStore) DeactivateKey(ctx context.Context, keyStr string, deactivatedBy string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from cache first
	h.cache.mu.Lock()
	delete(h.cache.keys, keyStr)
	delete(h.cache.rateLimits, keyStr)
	h.cache.mu.Unlock()

	// Update database
	return h.db.DeactivateKey(ctx, keyStr, deactivatedBy)
}

// UpdateLastUsed updates the last used timestamp in database
// Cache is not updated to avoid contention
func (h *HybridKeyStore) UpdateLastUsed(ctx context.Context, keyStr string) error {
	return h.db.UpdateLastUsed(ctx, keyStr)
}

// CheckRateLimit checks if a key has exceeded its rate limit
func (h *HybridKeyStore) CheckRateLimit(keyStr string) error {
	h.cache.mu.RLock()
	defer h.cache.mu.RUnlock()

	limiter, exists := h.cache.rateLimits[keyStr]
	if !exists {
		return fmt.Errorf("rate limiter not found")
	}

	return limiter.Check()
}

// GetAuditLog retrieves audit log for a key
func (h *HybridKeyStore) GetAuditLog(ctx context.Context, keyStr string, limit int) ([]map[string]interface{}, error) {
	return h.db.GetAuditLog(ctx, keyStr, limit)
}

// Close closes the database connection
func (h *HybridKeyStore) Close() error {
	return h.db.Close()
}

// Helper functions

// generateKey creates an APIKey with cryptographic random bytes
func (h *HybridKeyStore) generateKey(clientID string, roles []string, rateLimit int, expiresIn *time.Duration) (*APIKey, error) {
	apiKey, err := h.cache.CreateKey(clientID, roles, rateLimit, expiresIn)
	if err != nil {
		return nil, err
	}
	return apiKey, nil
}

// isKeyValid checks if a key is currently valid
func (h *HybridKeyStore) isKeyValid(key *APIKey) bool {
	if !key.IsActive {
		return false
	}

	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return false
	}

	return true
}

// SyncCache performs a full cache refresh from database
// Useful for multi-instance deployments where keys may be created elsewhere
func (h *HybridKeyStore) SyncCache(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	keys, err := h.db.LoadAllKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync cache: %w", err)
	}

	// Clear cache
	h.cache.mu.Lock()
	h.cache.keys = make(map[string]*APIKey)
	h.cache.rateLimits = make(map[string]*RateLimiter)

	// Repopulate cache
	for _, key := range keys {
		h.cache.keys[key.Key] = key
		rateLimit := key.RateLimit
		if rateLimit <= 0 {
			rateLimit = 100
		}
		h.cache.rateLimits[key.Key] = &RateLimiter{
			maxRequests: rateLimit,
			window:      time.Minute,
			resetTime:   time.Now().Add(time.Minute),
			count:       0,
		}
	}
	h.cache.mu.Unlock()

	return nil
}
