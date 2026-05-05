package notifier

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/igodwin/notifier/internal/domain"
)

// NtfyConfig contains ntfy.sh configuration
type NtfyConfig struct {
	// ServerURL is the ntfy server URL (default: https://ntfy.sh)
	ServerURL string `mapstructure:"server_url"`

	// Token is the access token for authentication (preferred method)
	// Supports both regular tokens (tk_...) and publish tokens
	Token string `mapstructure:"token"`

	// Username for basic authentication (alternative to token)
	Username string `mapstructure:"username"`

	// Password for basic authentication (alternative to token)
	Password string `mapstructure:"password"`

	// DefaultTopic is the default topic if not specified in notification
	DefaultTopic string `mapstructure:"default_topic"`

	// CACertPath is the path to a custom CA certificate file (optional, PEM format)
	// Use this only for self-hosted ntfy servers with self-signed certificates.
	// If not specified, system default CA certificates are used.
	CACertPath string `mapstructure:"ca_cert_path"`

	// Default marks this instance as default
	Default bool `mapstructure:"default"`

	// AllowedRoles are roles allowed to use this notifier (empty = all authenticated)
	AllowedRoles []string `mapstructure:"allowed_roles"`
}

// NtfyNotifier sends notifications via ntfy.sh
type NtfyNotifier struct {
	BaseNotifier
	config     *NtfyConfig
	httpClient *http.Client
}

// ntfyRequest represents the ntfy API request format
type ntfyRequest struct {
	Topic    string       `json:"topic"`
	Message  string       `json:"message"`
	Title    string       `json:"title,omitempty"`
	Priority int          `json:"priority,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Click    string       `json:"click,omitempty"`
	Attach   string       `json:"attach,omitempty"`
	Actions  []ntfyAction `json:"actions,omitempty"`
	Icon     string       `json:"icon,omitempty"`
	Delay    string       `json:"delay,omitempty"`
	Email    string       `json:"email,omitempty"`
}

// ntfyAction represents an action button in ntfy
type ntfyAction struct {
	Action string `json:"action"`
	Label  string `json:"label"`
	URL    string `json:"url,omitempty"`
	Body   string `json:"body,omitempty"`
	Clear  bool   `json:"clear,omitempty"`
}

// NewNtfyNotifier creates a new ntfy notifier
func NewNtfyNotifier(config *NtfyConfig) (*NtfyNotifier, error) {
	if config == nil {
		return nil, fmt.Errorf("ntfy config is required")
	}

	if config.ServerURL == "" {
		config.ServerURL = "https://ntfy.sh" // Default public ntfy server
	}

	// Validate CA certificate path if provided
	if err := validateCACertPath(config.CACertPath); err != nil {
		return nil, err
	}

	// Create HTTP client with proper TLS configuration
	httpClient, err := createNtfyHTTPClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &NtfyNotifier{
		BaseNotifier: BaseNotifier{
			notificationType: domain.TypeNtfy,
		},
		config:     config,
		httpClient: httpClient,
	}, nil
}

// validateCACertPath validates that the CA certificate path exists and is readable
func validateCACertPath(caCertPath string) error {
	if caCertPath == "" {
		// CA cert path is optional
		return nil
	}

	// Check if file exists
	info, err := os.Stat(caCertPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CA certificate file not found: %s", caCertPath)
		}
		return fmt.Errorf("CA certificate file error: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("CA certificate path is not a regular file: %s", caCertPath)
	}

	// Try to read and parse the certificate
	certData, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w", err)
	}

	// Verify it's valid PEM format
	if !isPEMCertificate(certData) {
		return fmt.Errorf("CA certificate file is not in valid PEM format: %s", caCertPath)
	}

	return nil
}

// isPEMCertificate checks if the data is a valid PEM certificate
func isPEMCertificate(data []byte) bool {
	// Try to parse as PEM format
	roots := x509.NewCertPool()
	return roots.AppendCertsFromPEM(data)
}

// createNtfyHTTPClient creates an HTTP client with proper TLS configuration
func createNtfyHTTPClient(config *NtfyConfig) (*http.Client, error) {
	tlsConfig := &tls.Config{
		// Require TLS verification (default Go behavior, never skip)
		// InsecureSkipVerify is explicitly NOT set, ensuring verification is always on
		MinVersion: tls.VersionTLS12,
	}

	// Load custom CA certificate if provided
	if config.CACertPath != "" {
		certData, err := os.ReadFile(config.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read custom CA certificate: %w", err)
		}

		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(certData) {
			return nil, fmt.Errorf("failed to parse custom CA certificate as PEM")
		}

		tlsConfig.RootCAs = certPool
	}
	// If RootCAs is not set, the default system CA pool will be used

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}, nil
}

// Validate overrides BaseNotifier.Validate to allow 0 recipients when a DefaultTopic is configured.
func (n *NtfyNotifier) Validate(notification *domain.Notification) error {
	if notification == nil {
		return fmt.Errorf("notification is nil")
	}
	if notification.Type != domain.TypeNtfy {
		return fmt.Errorf("notification type mismatch: expected %s, got %s", domain.TypeNtfy, notification.Type)
	}
	if len(notification.Recipients) == 0 && n.config.DefaultTopic == "" {
		return fmt.Errorf("notification has no recipients and no default topic is configured")
	}
	return nil
}

// Send sends a notification via ntfy
func (n *NtfyNotifier) Send(ctx context.Context, notification *domain.Notification) (*domain.NotificationResult, error) {
	if err := ValidateContext(ctx); err != nil {
		return nil, err
	}

	if err := n.Validate(notification); err != nil {
		return nil, err
	}

	// For ntfy, recipients are topics
	recipients := notification.Recipients
	if len(recipients) == 0 && n.config.DefaultTopic != "" {
		recipients = []string{n.config.DefaultTopic}
	}

	for _, topic := range recipients {
		req := ntfyRequest{
			Topic:    topic,
			Message:  notification.Body,
			Title:    notification.Subject,
			Priority: n.mapPriority(notification.Priority),
		}

		// Add custom tags from metadata
		if tags, ok := notification.Metadata["tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					req.Tags = append(req.Tags, tagStr)
				}
			}
		}

		// Add click action from metadata
		if click, ok := notification.Metadata["click"].(string); ok {
			req.Click = click
		}

		// Add attachment from metadata
		if attach, ok := notification.Metadata["attach"].(string); ok {
			req.Attach = attach
		}

		// Add icon from metadata
		if icon, ok := notification.Metadata["icon"].(string); ok {
			req.Icon = icon
		}

		// Add delay from metadata (e.g., "30s", "1m", "1h")
		if delay, ok := notification.Metadata["delay"].(string); ok {
			req.Delay = delay
		}

		// Add email from metadata (for email notifications)
		if email, ok := notification.Metadata["email"].(string); ok {
			req.Email = email
		}

		// Add actions from metadata
		if actions, ok := notification.Metadata["actions"].([]interface{}); ok {
			for _, action := range actions {
				if actionMap, ok := action.(map[string]interface{}); ok {
					ntfyAct := ntfyAction{}
					if actionType, ok := actionMap["action"].(string); ok {
						ntfyAct.Action = actionType
					}
					if label, ok := actionMap["label"].(string); ok {
						ntfyAct.Label = label
					}
					if url, ok := actionMap["url"].(string); ok {
						ntfyAct.URL = url
					}
					if body, ok := actionMap["body"].(string); ok {
						ntfyAct.Body = body
					}
					if clear, ok := actionMap["clear"].(bool); ok {
						ntfyAct.Clear = clear
					}
					req.Actions = append(req.Actions, ntfyAct)
				}
			}
		}

		if err := n.sendToTopic(ctx, &req); err != nil {
			return &domain.NotificationResult{
				NotificationID: notification.ID,
				Success:        false,
				Error:          err.Error(),
				SentAt:         time.Now(),
			}, err
		}
	}

	return &domain.NotificationResult{
		NotificationID: notification.ID,
		Success:        true,
		Message:        fmt.Sprintf("Notification sent to %d topics", len(notification.Recipients)),
		SentAt:         time.Now(),
		ProviderResponse: map[string]interface{}{
			"server": n.config.ServerURL,
			"topics": notification.Recipients,
		},
	}, nil
}

// sendToTopic sends a notification to a specific ntfy topic
func (n *NtfyNotifier) sendToTopic(ctx context.Context, req *ntfyRequest) error {
	url := fmt.Sprintf("%s", n.config.ServerURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal ntfy request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add authentication if configured
	if n.config.Token != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.config.Token))
	} else if n.config.Username != "" && n.config.Password != "" {
		httpReq.SetBasicAuth(n.config.Username, n.config.Password)
	}

	resp, err := n.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy server returned status: %d", resp.StatusCode)
	}

	return nil
}

// mapPriority maps domain priority to ntfy priority (1-5)
func (n *NtfyNotifier) mapPriority(priority domain.Priority) int {
	switch priority {
	case domain.PriorityLow:
		return 2
	case domain.PriorityNormal:
		return 3
	case domain.PriorityHigh:
		return 4
	case domain.PriorityCritical:
		return 5
	default:
		return 3
	}
}

// Close closes the HTTP client
func (n *NtfyNotifier) Close() error {
	n.httpClient.CloseIdleConnections()
	return nil
}
