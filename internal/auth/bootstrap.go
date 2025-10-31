package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/igodwin/notifier/internal/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

// BootstrapAdminKeyInMemory creates an initial admin API key on first startup (in-memory store)
// This is a simpler version for in-memory APIKeyStore (without database persistence)
func BootstrapAdminKeyInMemory(keyStore *APIKeyStore, cfg *BootstrapConfig, logger *logging.Logger) (*APIKey, error) {
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
		"admin-bootstrap",
		adminRoles,
		0,   // Unlimited rate limit
		nil, // No expiration
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
		separator := strings.Repeat("=", 60)
		fmt.Println("\n" + separator)
		fmt.Println("NOTIFIER BOOTSTRAP: ADMIN KEY CREATED")
		fmt.Println(separator)
		fmt.Printf("Key: %s\n", apiKey.Key)
		fmt.Println("\nSave this key in a secure location. You will not be able to see it again.")
		fmt.Println("Use this key to create additional API keys via the key management API.")
		fmt.Println(separator + "\n")
	}

	logger.Infof("Bootstrap admin key created successfully")
	return apiKey, nil
}

// RegisterAdminKeyInMemory registers a pre-existing admin API key in the keystore
// Used when loading from Kubernetes secret or environment variable
func RegisterAdminKeyInMemory(keyStore *APIKeyStore, adminKey string, logger *logging.Logger) (*APIKey, error) {
	if adminKey == "" {
		return nil, fmt.Errorf("admin key value is empty")
	}

	// Validate key format (should start with "nk_")
	if !strings.HasPrefix(adminKey, "nk_") {
		return nil, fmt.Errorf("invalid admin key format: must start with 'nk_'")
	}

	// Create APIKey object with the provided key
	adminRoles := []string{"admin", "notify-email", "notify-slack", "notify-ntfy"}
	now := time.Now().UTC()
	apiKey := &APIKey{
		Key:       adminKey,
		ClientID:  "admin-bootstrap",
		Roles:     adminRoles,
		CreatedAt: now,
		IsActive:  true,
		RateLimit: 0, // Unlimited
		Name:      fmt.Sprintf("admin-bootstrap-%d", now.Unix()),
	}

	// Add to keystore
	keyStore.mu.Lock()
	defer keyStore.mu.Unlock()

	keyStore.keys[adminKey] = apiKey
	keyStore.rateLimits[adminKey] = &RateLimiter{
		maxRequests: 0, // Unlimited
		window:      time.Minute,
		resetTime:   time.Now().Add(time.Minute),
		count:       0,
	}

	logger.Infof("Registered existing admin key from Kubernetes secret")
	return apiKey, nil
}

// BootstrapAdminKey creates an initial admin API key on first startup (with database persistence)
// This should be called once per deployment when using HybridKeyStore with database
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
		0,   // Unlimited rate limit
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
		separator := strings.Repeat("=", 60)
		fmt.Println("\n" + separator)
		fmt.Println("NOTIFIER BOOTSTRAP: ADMIN KEY CREATED")
		fmt.Println(separator)
		fmt.Printf("Key: %s\n", apiKey.Key)
		fmt.Println("\nSave this key in a secure location. You will not be able to see it again.")
		fmt.Println("Use this key to create additional API keys via the key management API.")
		fmt.Println(separator + "\n")
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

// getKubernetesNamespace reads the pod's namespace from the service account token
func getKubernetesNamespace() (string, error) {
	const namespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(namespacePath)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace from service account: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// LoadAdminKeyFromKubernetesSecret attempts to load an existing admin key from a Kubernetes secret
// Returns the key string if found, empty string if secret doesn't exist, or error on failure
func LoadAdminKeyFromKubernetesSecret(ctx context.Context, secretName, secretKey string, logger *logging.Logger) (string, error) {
	// Try to create Kubernetes client (will fail gracefully if not in cluster)
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Debugf("Not running in Kubernetes cluster or in-cluster config unavailable: %v", err)
		return "", nil // Not in Kubernetes, return empty (not an error)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Warnf("Failed to create Kubernetes client: %v", err)
		return "", nil // Failed to create client, but not a fatal error
	}

	namespace, err := getKubernetesNamespace()
	if err != nil {
		logger.Warnf("Failed to determine pod namespace: %v", err)
		return "", nil // Failed to get namespace, but not a fatal error
	}

	// Try to get the secret
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		// Secret doesn't exist or other error occurred
		logger.Debugf("Admin key secret not found in namespace %s: %v", namespace, err)
		return "", nil // Secret not found is not an error
	}

	// Extract the key value from the secret
	if secretValue, exists := secret.Data[secretKey]; exists {
		logger.Infof("Found existing admin key in Kubernetes secret %s/%s", namespace, secretName)
		return string(secretValue), nil
	}

	logger.Warnf("Kubernetes secret %s/%s exists but key %q not found", namespace, secretName, secretKey)
	return "", nil
}

// CreateKubernetesSecret creates or updates a Kubernetes secret with the admin key
func CreateKubernetesSecret(ctx context.Context, secretName, secretKey, adminKey string, logger *logging.Logger) error {
	// Try to create Kubernetes client (will fail gracefully if not in cluster)
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Debugf("Not running in Kubernetes cluster, skipping secret creation: %v", err)
		return nil // Not in Kubernetes, skip (not an error)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Warnf("Failed to create Kubernetes client, skipping secret creation: %v", err)
		return nil // Failed to create client, but not a fatal error
	}

	namespace, err := getKubernetesNamespace()
	if err != nil {
		logger.Warnf("Failed to determine pod namespace, skipping secret creation: %v", err)
		return nil // Failed to get namespace, but not a fatal error
	}

	// Create or update the secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "notifier",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretKey: []byte(adminKey),
		},
	}

	// Try to get existing secret first
	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		// Secret exists, update it
		secret.ResourceVersion = existingSecret.ResourceVersion
		_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			logger.Warnf("Failed to update Kubernetes secret %s/%s: %v", namespace, secretName, err)
			return nil // Log warning but don't fail
		}
		logger.Infof("Updated admin key in Kubernetes secret %s/%s", namespace, secretName)
	} else {
		// Secret doesn't exist, create it
		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			logger.Warnf("Failed to create Kubernetes secret %s/%s: %v", namespace, secretName, err)
			return nil // Log warning but don't fail
		}
		logger.Infof("Created Kubernetes secret %s/%s with admin key", namespace, secretName)
	}

	return nil
}
