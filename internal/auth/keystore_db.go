package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// KeyStoreDB provides persistent storage for API keys using PostgreSQL
// It acts as the backend for the in-memory cache
type KeyStoreDB struct {
	db *sql.DB
}

// NewKeyStoreDB creates a new database-backed key store
func NewKeyStoreDB(dbURL string) (*KeyStoreDB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ks := &KeyStoreDB{db: db}

	// Initialize schema
	if err := ks.initializeSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return ks, nil
}

// initializeSchema creates the necessary tables and indexes if they don't exist
func (ks *KeyStoreDB) initializeSchema() error {
	// Create tables
	tableSchema := `
	-- API Keys table
	CREATE TABLE IF NOT EXISTS api_keys (
		id SERIAL PRIMARY KEY,
		key VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		client_id VARCHAR(255) NOT NULL,
		roles TEXT[] DEFAULT '{}',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at TIMESTAMP,
		expires_at TIMESTAMP,
		is_active BOOLEAN NOT NULL DEFAULT true,
		rate_limit INTEGER NOT NULL DEFAULT 0,
		created_by VARCHAR(255),
		metadata JSONB DEFAULT '{}'::jsonb
	);

	-- Audit log for key operations
	CREATE TABLE IF NOT EXISTS api_key_audit_log (
		id SERIAL PRIMARY KEY,
		key_id INTEGER NOT NULL REFERENCES api_keys(id),
		action VARCHAR(50) NOT NULL,
		performed_by VARCHAR(255) NOT NULL,
		performed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		details JSONB DEFAULT '{}'::jsonb
	);
	`

	if _, err := ks.db.Exec(tableSchema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Create indexes separately (PostgreSQL syntax)
	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_api_keys_key ON api_keys(key);
	CREATE INDEX IF NOT EXISTS idx_api_keys_client_id ON api_keys(client_id);
	CREATE INDEX IF NOT EXISTS idx_api_keys_active ON api_keys(is_active);
	CREATE INDEX IF NOT EXISTS idx_api_keys_expires ON api_keys(expires_at);
	CREATE INDEX IF NOT EXISTS idx_api_key_audit_log_key_id ON api_key_audit_log(key_id);
	CREATE INDEX IF NOT EXISTS idx_api_key_audit_log_performed_at ON api_key_audit_log(performed_at);
	`

	if _, err := ks.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// SaveKey persists an API key to the database
func (ks *KeyStoreDB) SaveKey(ctx context.Context, key *APIKey, createdBy string) error {
	query := `
	INSERT INTO api_keys (key, name, client_id, roles, created_at, last_used_at, expires_at, is_active, rate_limit, created_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (key) DO UPDATE SET
		name = EXCLUDED.name,
		roles = EXCLUDED.roles,
		is_active = EXCLUDED.is_active,
		rate_limit = EXCLUDED.rate_limit,
		last_used_at = COALESCE(EXCLUDED.last_used_at, api_keys.last_used_at)
	`

	_, err := ks.db.ExecContext(ctx, query,
		key.Key,
		key.Name,
		key.ClientID,
		pq.Array(key.Roles), // Convert Go slice to PostgreSQL array
		key.CreatedAt,
		key.LastUsedAt,
		key.ExpiresAt,
		key.IsActive,
		key.RateLimit,
		createdBy,
	)

	if err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}

	// Log to audit trail
	ks.logAudit(ctx, key.Key, "created", createdBy, map[string]interface{}{
		"client_id": key.ClientID,
		"roles":     key.Roles,
	})

	return nil
}

// GetKey retrieves an API key from the database
func (ks *KeyStoreDB) GetKey(ctx context.Context, keyStr string) (*APIKey, error) {
	query := `
	SELECT key, name, client_id, roles, created_at, last_used_at, expires_at, is_active, rate_limit
	FROM api_keys
	WHERE key = $1
	`

	var key APIKey
	var roles []string

	err := ks.db.QueryRowContext(ctx, query, keyStr).Scan(
		&key.Key,
		&key.Name,
		&key.ClientID,
		&roles,
		&key.CreatedAt,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.IsActive,
		&key.RateLimit,
	)

	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	key.Roles = roles
	return &key, nil
}

// ListKeys retrieves all API keys for a client
func (ks *KeyStoreDB) ListKeys(ctx context.Context, clientID string) ([]*APIKey, error) {
	query := `
	SELECT key, name, client_id, roles, created_at, last_used_at, expires_at, is_active, rate_limit
	FROM api_keys
	WHERE client_id = $1 AND is_active = true
	ORDER BY created_at DESC
	`

	rows, err := ks.db.QueryContext(ctx, query, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var roles []string

		err := rows.Scan(
			&key.Key,
			&key.Name,
			&key.ClientID,
			&roles,
			&key.CreatedAt,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.IsActive,
			&key.RateLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}

		key.Roles = roles
		keys = append(keys, &key)
	}

	return keys, rows.Err()
}

// DeactivateKey disables an API key
func (ks *KeyStoreDB) DeactivateKey(ctx context.Context, keyStr string, deactivatedBy string) error {
	query := `UPDATE api_keys SET is_active = false WHERE key = $1`

	result, err := ks.db.ExecContext(ctx, query, keyStr)
	if err != nil {
		return fmt.Errorf("failed to deactivate key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrKeyNotFound
	}

	ks.logAudit(ctx, keyStr, "deactivated", deactivatedBy, nil)
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp
func (ks *KeyStoreDB) UpdateLastUsed(ctx context.Context, keyStr string) error {
	query := `UPDATE api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE key = $1`

	_, err := ks.db.ExecContext(ctx, query, keyStr)
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	return nil
}

// LoadAllKeys loads all active keys into memory for caching
func (ks *KeyStoreDB) LoadAllKeys(ctx context.Context) ([]*APIKey, error) {
	query := `
	SELECT key, name, client_id, roles, created_at, last_used_at, expires_at, is_active, rate_limit
	FROM api_keys
	WHERE is_active = true AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`

	rows, err := ks.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var roles []string

		err := rows.Scan(
			&key.Key,
			&key.Name,
			&key.ClientID,
			&roles,
			&key.CreatedAt,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.IsActive,
			&key.RateLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}

		key.Roles = roles
		keys = append(keys, &key)
	}

	return keys, rows.Err()
}

// logAudit logs a key operation to the audit trail
func (ks *KeyStoreDB) logAudit(ctx context.Context, keyStr string, action string, performedBy string, details map[string]interface{}) {
	// Get key ID
	var keyID int
	err := ks.db.QueryRowContext(ctx, "SELECT id FROM api_keys WHERE key = $1", keyStr).Scan(&keyID)
	if err != nil {
		return // Silently fail audit logging
	}

	// Log the action
	detailsJSON := "{}"
	if len(details) > 0 {
		// Simple JSON encoding (could use jsonb package for robustness)
		detailsJSON = fmt.Sprintf(`{"event": "%s"}`, action)
	}

	query := `
	INSERT INTO api_key_audit_log (key_id, action, performed_by, details)
	VALUES ($1, $2, $3, $4)
	`

	ks.db.ExecContext(ctx, query, keyID, action, performedBy, detailsJSON)
}

// Close closes the database connection
func (ks *KeyStoreDB) Close() error {
	return ks.db.Close()
}

// GetAuditLog retrieves audit log entries for a key
func (ks *KeyStoreDB) GetAuditLog(ctx context.Context, keyStr string, limit int) ([]map[string]interface{}, error) {
	query := `
	SELECT al.action, al.performed_by, al.performed_at, al.details
	FROM api_key_audit_log al
	JOIN api_keys ak ON al.key_id = ak.id
	WHERE ak.key = $1
	ORDER BY al.performed_at DESC
	LIMIT $2
	`

	rows, err := ks.db.QueryContext(ctx, query, keyStr, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var action, performedBy, details string
		var performedAt time.Time

		err := rows.Scan(&action, &performedBy, &performedAt, &details)
		if err != nil {
			return nil, err
		}

		logs = append(logs, map[string]interface{}{
			"action":       action,
			"performed_by": performedBy,
			"performed_at": performedAt,
			"details":      details,
		})
	}

	return logs, rows.Err()
}

// Custom errors
var (
	ErrKeyNotFound = fmt.Errorf("API key not found")
)
