package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/igodwin/notifier/internal/domain"
)

// SlackConfig contains Slack webhook configuration
type SlackConfig struct {
	WebhookURL string            `mapstructure:"webhook_url"`
	Token      string            `mapstructure:"token"`
	Channel    string            `mapstructure:"channel"`
	Username   string            `mapstructure:"username"`
	IconEmoji  string            `mapstructure:"icon_emoji"`
	Webhooks   map[string]string `mapstructure:"webhooks"` // Channel-specific webhooks
	Default    bool              `mapstructure:"default"`  // Mark this instance as default
}

// SlackNotifier sends notifications to Slack
type SlackNotifier struct {
	BaseNotifier
	config     *SlackConfig
	httpClient *http.Client
}

// slackMessage represents the Slack API request format
type slackMessage struct {
	Channel   string       `json:"channel,omitempty"`
	Username  string       `json:"username,omitempty"`
	IconEmoji string       `json:"icon_emoji,omitempty"`
	Text      string       `json:"text,omitempty"`
	Blocks    []slackBlock `json:"blocks,omitempty"`
	Markdown  bool         `json:"mrkdwn,omitempty"`
}

// slackBlock represents a Slack block element
type slackBlock struct {
	Type string          `json:"type"`
	Text *slackTextBlock `json:"text,omitempty"`
}

// slackTextBlock represents a text element in a Slack block
type slackTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(config *SlackConfig) (*SlackNotifier, error) {
	if config == nil {
		return nil, fmt.Errorf("Slack config is required")
	}

	// Either webhook URL or token is required
	if config.WebhookURL == "" && config.Token == "" && len(config.Webhooks) == 0 {
		return nil, fmt.Errorf("Slack webhook URL, token, or channel webhooks are required")
	}

	return &SlackNotifier{
		BaseNotifier: BaseNotifier{
			notificationType: domain.TypeSlack,
		},
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Send sends a notification to Slack
func (s *SlackNotifier) Send(ctx context.Context, notification *domain.Notification) (*domain.NotificationResult, error) {
	if err := ValidateContext(ctx); err != nil {
		return nil, err
	}

	if err := s.Validate(notification); err != nil {
		return nil, err
	}

	// For Slack, recipients are channel names or webhook URLs
	for _, recipient := range notification.Recipients {
		msg := s.buildMessage(notification, recipient)
		webhookURL := s.getWebhookURL(recipient)

		if err := s.sendToSlack(ctx, webhookURL, msg); err != nil {
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
		Message:        fmt.Sprintf("Slack notification sent to %d channels", len(notification.Recipients)),
		SentAt:         time.Now(),
		ProviderResponse: map[string]interface{}{
			"channels": notification.Recipients,
		},
	}, nil
}

// buildMessage constructs a Slack message with rich formatting
func (s *SlackNotifier) buildMessage(notification *domain.Notification, channel string) *slackMessage {
	msg := &slackMessage{
		Channel:   channel,
		Username:  s.config.Username,
		IconEmoji: s.config.IconEmoji,
		Markdown:  true,
	}

	// Use blocks for rich formatting if both subject and body exist
	if notification.Subject != "" && notification.Body != "" {
		msg.Blocks = []slackBlock{
			{
				Type: "header",
				Text: &slackTextBlock{
					Type: "plain_text",
					Text: notification.Subject,
				},
			},
			{
				Type: "section",
				Text: &slackTextBlock{
					Type: "mrkdwn",
					Text: notification.Body,
				},
			},
		}
	} else {
		// Fallback to simple text
		if notification.Subject != "" {
			msg.Text = fmt.Sprintf("*%s*\n%s", notification.Subject, notification.Body)
		} else {
			msg.Text = notification.Body
		}
	}

	// Add priority indicator for high priority notifications
	if notification.Priority >= domain.PriorityHigh {
		priorityEmoji := ":warning:"
		if notification.Priority == domain.PriorityCritical {
			priorityEmoji = ":rotating_light:"
		}

		msg.Blocks = append([]slackBlock{
			{
				Type: "context",
				Text: &slackTextBlock{
					Type: "mrkdwn",
					Text: fmt.Sprintf("%s *Priority: %d*", priorityEmoji, notification.Priority),
				},
			},
		}, msg.Blocks...)
	}

	return msg
}

// getWebhookURL returns the webhook URL for a specific channel
func (s *SlackNotifier) getWebhookURL(channel string) string {
	// Check for channel-specific webhook
	if webhook, ok := s.config.Webhooks[channel]; ok {
		return webhook
	}

	// Fall back to default webhook URL
	return s.config.WebhookURL
}

// sendToSlack sends the message to Slack via webhook
func (s *SlackNotifier) sendToSlack(ctx context.Context, webhookURL string, msg *slackMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add token authentication if configured
	if s.config.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.Token))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Slack API returned status: %d", resp.StatusCode)
	}

	return nil
}

// Close closes the HTTP client
func (s *SlackNotifier) Close() error {
	s.httpClient.CloseIdleConnections()
	return nil
}
