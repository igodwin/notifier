package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/igodwin/notifier/internal/config"
)

type NtfyNotifier struct {
	config config.NtfyConfig
}

type NtfyMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

func NewNtfyNotifier(config config.NtfyConfig) (*NtfyNotifier, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &NtfyNotifier{config: config}, nil
}

// Send sends a notification via Ntfy
func (n *NtfyNotifier) Send(notification Notification) error {
	url := fmt.Sprintf("%s/%s", n.config.URL, n.config.Topic)

	// Construct the Ntfy message
	message := NtfyMessage{
		Topic:   n.config.Topic,
		Message: notification.Message,
	}

	// Serialize message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers for authentication
	req.Header.Set("Content-Type", "application/json")
	if n.config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.config.Token)
	} else if n.config.Username != "" && n.config.Password != "" {
		req.SetBasicAuth(n.config.Username, n.config.Password)
	}

	// Send the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send notification, status: %s", resp.Status)
	}

	return nil
}
