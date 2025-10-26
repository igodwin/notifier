package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/igodwin/notifier/internal/logging"
)

// BootstrapConfig holds configuration for bootstrap operations
type BootstrapConfig struct {
	// Enabled triggers automatic bootstrap key creation on first startup
	Enabled bool
	// AdminKeyFileName is where to store the generated admin key
	AdminKeyFileName string
	// PrintToStdout prints the admin key to stdout (DANGEROUS - only for setup)
	PrintToStdout bool
}

// BootstrapAdminKey creates an initial admin API key on first startup
// This should be called once per deployment
func BootstrapAdminKey(ctx context.Context, keyStore *HybridKeyStore, cfg *BootstrapConfig, logger *logging.Logger) (*APIKey, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("bootstrap is disabled")
	}

	// Check if bootstrap has already been done
	if cfg.AdminKeyFileName != "" {
		if _, err := os.Stat(cfg.AdminKeyFileName); err == nil {
			// File exists, bootstrap already done
			logger.Infof("Bootstrap key file exists at %s, skipping bootstrap", cfg.AdminKeyFileName)
			return nil, fmt.Errorf("bootstrap already completed")
		}
	}

	// Create admin key with all roles
	adminRoles := []string{"admin", "notify-email", "notify-slack", "notify-ntfy"}
	apiKey, err := keyStore.CreateKey(
		ctx,
		"admin-bootstrap",
		adminRoles,
		0, // Unlimited rate limit
		nil, // No expiration
		"system",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap admin key: %w", err)
	}

	// Save key to file if configured
	if cfg.AdminKeyFileName != "" {
		keyContent := fmt.Sprintf(`# Notifier Admin Key
# Created: %s
# This key has full admin permissions
# KEEP THIS SECRET!

%s
`, time.Now().Format(time.RFC3339), apiKey.Key)

		if err := os.WriteFile(cfg.AdminKeyFileName, []byte(keyContent), 0600); err != nil {
			logger.Warnf("Failed to save admin key to file: %v", err)
		} else {
			logger.Infof("Admin key saved to %s", cfg.AdminKeyFileName)
		}
	}

	// Print to stdout if configured (DANGEROUS - only for interactive setup)
	if cfg.PrintToStdout {
		fmt.Println("\n" + "="*60)
		fmt.Println("NOTIFIER BOOTSTRAP: ADMIN KEY CREATED")
		fmt.Println("="*60)
		fmt.Printf("Key: %s\n", apiKey.Key)
		fmt.Println("\nSave this key in a secure location. You will not be able to see it again.")
		fmt.Println("Use this key to create additional API keys via the key management API.")
		fmt.Println("="*60 + "\n")
	}

	logger.Infof("Bootstrap admin key created successfully")
	return apiKey, nil
}

// LoadBootstrapKeyFromEnv checks if a bootstrap key was provided via environment variable
// This allows injecting a pre-generated key via CI/CD
func LoadBootstrapKeyFromEnv(ctx context.Context, keyStore *HybridKeyStore, logger *logging.Logger) error {
	bootstrapKey := os.Getenv("NOTIFIER_BOOTSTRAP_ADMIN_KEY")
	if bootstrapKey == "" {
		return nil // Not set, skip
	}

	// Check if key already exists in database
	// For now, we skip if environment variable is set
	// In production, you'd want to verify the key is already in the database

	logger.Infof("Bootstrap key detected from environment variable")
	return nil
}
