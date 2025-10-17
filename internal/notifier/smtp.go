package notifier

import (
	"context"
	"fmt"
	"net/smtp"
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

	// Validate email recipients
	for _, recipient := range notification.Recipients {
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

	err := smtp.SendMail(addr, auth, s.config.From, notification.Recipients, []byte(message))
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

	builder.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(notification.Recipients, ", ")))
	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", notification.Subject))
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(notification.Body)

	return builder.String()
}

// Validate checks if the notification is valid for SMTP
func (s *SMTPNotifier) Validate(notification *domain.Notification) error {
	if err := s.BaseNotifier.Validate(notification); err != nil {
		return err
	}

	if notification.Subject == "" {
		return fmt.Errorf("email subject is required")
	}

	if notification.Body == "" {
		return fmt.Errorf("email body is required")
	}

	return nil
}
