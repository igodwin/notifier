package notifier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/igodwin/notifier/internal/domain"
)

// SMTPConfig contains SMTP server configuration
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
	FromName string `mapstructure:"from_name"` // Optional display name for From header
	UseTLS   bool   `mapstructure:"use_tls"`
	Default  bool   `mapstructure:"default"` // Mark this instance as default
}

// SMTPNotifier sends notifications via email using SMTP
type SMTPNotifier struct {
	BaseNotifier
	config *SMTPConfig
}

// NewSMTPNotifier creates a new SMTP notifier
func NewSMTPNotifier(config *SMTPConfig) (*SMTPNotifier, error) {
	if config == nil {
		return nil, fmt.Errorf("SMTP config is required")
	}

	if config.Host == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}

	if config.Port == 0 {
		config.Port = 587 // Default SMTP submission port
	}

	if config.From == "" {
		return nil, fmt.Errorf("SMTP from address is required")
	}

	return &SMTPNotifier{
		BaseNotifier: BaseNotifier{
			notificationType: domain.TypeEmail,
		},
		config: config,
	}, nil
}

// Send sends a notification via email
func (s *SMTPNotifier) Send(ctx context.Context, notification *domain.Notification) (*domain.NotificationResult, error) {
	if err := ValidateContext(ctx); err != nil {
		return nil, err
	}

	if err := s.Validate(notification); err != nil {
		return nil, err
	}

	// Collect all recipients (To, CC, BCC) for validation
	allRecipients := make([]string, 0, len(notification.Recipients)+len(notification.CC)+len(notification.BCC))
	allRecipients = append(allRecipients, notification.Recipients...)
	allRecipients = append(allRecipients, notification.CC...)
	allRecipients = append(allRecipients, notification.BCC...)

	// Validate email recipients
	for _, recipient := range allRecipients {
		if !strings.Contains(recipient, "@") {
			return &domain.NotificationResult{
				NotificationID: notification.ID,
				Success:        false,
				Error:          fmt.Sprintf("invalid email address: %s", recipient),
				SentAt:         time.Now(),
			}, fmt.Errorf("invalid email address: %s", recipient)
		}
	}

	// Build email message
	message := s.buildMessage(notification)

	// Send email
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	// smtp.SendMail needs all recipients (To, CC, BCC) for actual delivery
	err := smtp.SendMail(addr, auth, s.config.From, allRecipients, []byte(message))
	if err != nil {
		return &domain.NotificationResult{
			NotificationID: notification.ID,
			Success:        false,
			Error:          err.Error(),
			SentAt:         time.Now(),
		}, fmt.Errorf("failed to send email: %w", err)
	}

	return &domain.NotificationResult{
		NotificationID: notification.ID,
		Success:        true,
		Message:        fmt.Sprintf("Email sent to %d recipients", len(notification.Recipients)),
		SentAt:         time.Now(),
		ProviderResponse: map[string]interface{}{
			"smtp_server": addr,
			"from":        s.config.From,
			"to":          notification.Recipients,
		},
	}, nil
}

// buildMessage constructs the email message with headers
func (s *SMTPNotifier) buildMessage(notification *domain.Notification) string {
	var builder strings.Builder

	// Format From header with optional display name
	fromHeader := s.config.From
	if s.config.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}

	builder.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))

	// Add To header (optional if only BCC is specified)
	if len(notification.Recipients) > 0 {
		builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(notification.Recipients, ", ")))
	}

	// Add CC header (optional)
	if len(notification.CC) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(notification.CC, ", ")))
	}

	// Note: BCC is intentionally NOT included in headers (that's the point of BCC!)

	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", notification.Subject))
	builder.WriteString("MIME-Version: 1.0\r\n")

	// Auto-detect HTML if content type not set
	contentType := notification.ContentType
	if contentType == "" {
		contentType = detectContentType(notification.Body)
	}

	// Build message based on content type
	if contentType == domain.ContentTypeHTML {
		// Send multipart/alternative with both text and HTML
		s.buildMultipartMessage(&builder, notification)
	} else {
		// Send plain text only
		builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		builder.WriteString("\r\n")
		builder.WriteString(notification.Body)
	}

	return builder.String()
}

// buildMultipartMessage builds a multipart/alternative email with both text and HTML versions
func (s *SMTPNotifier) buildMultipartMessage(builder *strings.Builder, notification *domain.Notification) {
	// Generate a unique boundary
	boundary := generateBoundary()

	builder.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	builder.WriteString("\r\n")

	// Plain text version (auto-generated from HTML)
	builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(htmlToPlainText(notification.Body))
	builder.WriteString("\r\n\r\n")

	// HTML version
	builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	builder.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(notification.Body)
	builder.WriteString("\r\n\r\n")

	// End boundary
	builder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
}

// detectContentType auto-detects if the body is HTML
func detectContentType(body string) domain.ContentType {
	trimmed := strings.TrimSpace(body)
	// Check for common HTML indicators
	if strings.HasPrefix(trimmed, "<") ||
	   strings.Contains(trimmed, "<html") ||
	   strings.Contains(trimmed, "<!DOCTYPE") ||
	   strings.Contains(trimmed, "<p>") ||
	   strings.Contains(trimmed, "<div>") ||
	   strings.Contains(trimmed, "<br>") {
		return domain.ContentTypeHTML
	}
	return domain.ContentTypeText
}

// generateBoundary generates a unique boundary string for multipart emails
func generateBoundary() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return "boundary_" + hex.EncodeToString(buf)
}

// htmlToPlainText converts HTML to plain text (simple implementation)
func htmlToPlainText(htmlContent string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(htmlContent, "")

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Clean up whitespace
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}

// Validate checks if the notification is valid for SMTP
func (s *SMTPNotifier) Validate(notification *domain.Notification) error {
	if notification == nil {
		return fmt.Errorf("notification is nil")
	}

	// For email, we need at least one recipient (To, CC, or BCC)
	totalRecipients := len(notification.Recipients) + len(notification.CC) + len(notification.BCC)
	if totalRecipients == 0 {
		return fmt.Errorf("email has no recipients (To, CC, or BCC required)")
	}

	if notification.Type != s.Type() {
		return fmt.Errorf("notification type mismatch: expected %s, got %s", s.Type(), notification.Type)
	}

	if notification.Subject == "" {
		return fmt.Errorf("email subject is required")
	}

	if notification.Body == "" {
		return fmt.Errorf("email body is required")
	}

	return nil
}
