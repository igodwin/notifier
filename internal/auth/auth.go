package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// APIKeyStore manages API keys with rate limiting
type APIKeyStore struct {
	mu        sync.RWMutex
	keys      map[string]*APIKey
	rateLimits map[string]*RateLimiter
}

// APIKey represents an API key with metadata
type APIKey struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	ClientID     string   `json:"client_id"`
	Roles        []string `json:"roles"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	IsActive     bool     `json:"is_active"`
	RateLimit    int      `json:"rate_limit"` // requests per minute, 0 = unlimited
}

// RateLimiter tracks rate limiting for a key
type RateLimiter struct {
	maxRequests int
	window      time.Duration
	resetTime   time.Time
	count       int
	mu          sync.Mutex
}

// AuthContext holds auth information attached to request context
type AuthContext struct {
	APIKey  *APIKey
	ClientID string
	Roles   []string
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys:       make(map[string]*APIKey),
		rateLimits: make(map[string]*RateLimiter),
	}
}

// CreateKey generates a new API key
func (s *APIKeyStore) CreateKey(clientID string, roles []string, rateLimit int, expiresIn *time.Duration) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	key := "nk_" + hex.EncodeToString(keyBytes)

	now := time.Now().UTC()
	apiKey := &APIKey{
		Key:       key,
		ClientID:  clientID,
		Roles:     roles,
		CreatedAt: now,
		IsActive:  true,
		RateLimit: rateLimit,
		Name:      fmt.Sprintf("%s-%d", clientID, now.Unix()),
	}

	if expiresIn != nil {
		expiresAt := now.Add(*expiresIn)
		apiKey.ExpiresAt = &expiresAt
	}

	s.keys[key] = apiKey
	s.rateLimits[key] = &RateLimiter{
		maxRequests: rateLimit,
		window:      time.Minute,
		resetTime:   time.Now().Add(time.Minute),
		count:       0,
	}

	return apiKey, nil
}

// ValidateKey checks if an API key is valid and returns the key metadata
func (s *APIKeyStore) ValidateKey(keyStr string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, exists := s.keys[keyStr]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	if !key.IsActive {
		return nil, fmt.Errorf("API key is inactive")
	}

	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	return key, nil
}

// CheckRateLimit checks if a key has exceeded its rate limit
func (s *APIKeyStore) CheckRateLimit(keyStr string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, exists := s.keys[keyStr]
	if !exists {
		return false, fmt.Errorf("invalid API key")
	}

	// Unlimited rate limit
	if key.RateLimit <= 0 {
		return true, nil
	}

	limiter, exists := s.rateLimits[keyStr]
	if !exists {
		return false, fmt.Errorf("rate limiter not found")
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	if now.After(limiter.resetTime) {
		limiter.count = 0
		limiter.resetTime = now.Add(limiter.window)
	}

	if limiter.count >= limiter.maxRequests {
		return false, nil
	}

	limiter.count++
	return true, nil
}

// UpdateLastUsed updates the last used timestamp for a key
func (s *APIKeyStore) UpdateLastUsed(keyStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, exists := s.keys[keyStr]
	if !exists {
		return fmt.Errorf("invalid API key")
	}

	now := time.Now().UTC()
	key.LastUsedAt = &now
	return nil
}

// DeactivateKey deactivates an API key
func (s *APIKeyStore) DeactivateKey(keyStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, exists := s.keys[keyStr]
	if !exists {
		return fmt.Errorf("invalid API key")
	}

	key.IsActive = false
	return nil
}

// GetKey retrieves key metadata (for management purposes)
func (s *APIKeyStore) GetKey(keyStr string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, exists := s.keys[keyStr]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	return key, nil
}

// ListKeys lists all API keys for a client
func (s *APIKeyStore) ListKeys(clientID string) []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []*APIKey
	for _, key := range s.keys {
		if key.ClientID == clientID {
			keys = append(keys, key)
		}
	}
	return keys
}

// ContextWithAuth adds auth context to a request context
func ContextWithAuth(ctx context.Context, auth *AuthContext) context.Context {
	return context.WithValue(ctx, "auth", auth)
}

// GetAuthContext retrieves auth context from a request context
func GetAuthContext(ctx context.Context) (*AuthContext, bool) {
	auth, ok := ctx.Value("auth").(*AuthContext)
	return auth, ok
}
