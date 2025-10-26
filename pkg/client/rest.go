package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RESTClient is a client for the Notifier REST API
type RESTClient struct {
	baseURL      string
	apiKey       string
	timeout      time.Duration
	maxRetries   int
	retryBackoff time.Duration
	client       *http.Client
}

// NewRESTClient creates a new REST client with the given config
func NewRESTClient(cfg ClientConfig) *RESTClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecure,
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &RESTClient{
		baseURL:      cfg.BaseURL,
		apiKey:       cfg.APIKey,
		timeout:      cfg.Timeout,
		maxRetries:   cfg.MaxRetries,
		retryBackoff: cfg.RetryBackoff,
		client:       httpClient,
	}
}

// Send sends a single notification
func (c *RESTClient) Send(ctx context.Context, req NotificationRequest) (*NotificationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	respBody, statusCode, err := c.doRequest(ctx, "POST", "/api/v1/notifications", body)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated && statusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	// The API wraps the response in a "result" field
	var wrapper struct {
		Result NotificationResponse `json:"result"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &wrapper.Result, nil
}

// SendBatch sends multiple notifications
func (c *RESTClient) SendBatch(ctx context.Context, reqs []NotificationRequest) ([]*NotificationResponse, error) {
	// Wrap requests in the proper format expected by the API
	payload := struct {
		Notifications []NotificationRequest `json:"notifications"`
	}{
		Notifications: reqs,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	respBody, statusCode, err := c.doRequest(ctx, "POST", "/api/v1/notifications/batch", body)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated && statusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var wrapper struct {
		Results []*NotificationResponse `json:"results"`
	}
	if err := json.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return wrapper.Results, nil
}

// GetNotification retrieves a notification by ID
func (c *RESTClient) GetNotification(ctx context.Context, id string) (*Notification, error) {
	url := fmt.Sprintf("/api/v1/notifications/%s", id)
	respBody, statusCode, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var notif Notification
	if err := json.Unmarshal(respBody, &notif); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &notif, nil
}

// ListNotifications lists notifications with filters
func (c *RESTClient) ListNotifications(ctx context.Context, filter ListNotificationsRequest) (*ListNotificationsResponse, error) {
	respBody, statusCode, err := c.doRequest(ctx, "GET", "/api/v1/notifications", nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var resp ListNotificationsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// CancelNotification cancels a pending notification
func (c *RESTClient) CancelNotification(ctx context.Context, id string) error {
	url := fmt.Sprintf("/api/v1/notifications/%s", id)
	respBody, statusCode, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	return nil
}

// RetryNotification retries a failed notification
func (c *RESTClient) RetryNotification(ctx context.Context, id string) (*NotificationResponse, error) {
	url := fmt.Sprintf("/api/v1/notifications/%s/retry", id)
	respBody, statusCode, err := c.doRequest(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var resp NotificationResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// GetStats retrieves notification statistics
func (c *RESTClient) GetStats(ctx context.Context) (*NotificationStats, error) {
	respBody, statusCode, err := c.doRequest(ctx, "GET", "/api/v1/stats", nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var stats NotificationStats
	if err := json.Unmarshal(respBody, &stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &stats, nil
}

// GetNotifiers retrieves available notifiers
func (c *RESTClient) GetNotifiers(ctx context.Context) (*NotifiersResponse, error) {
	respBody, statusCode, err := c.doRequest(ctx, "GET", "/api/v1/notifiers", nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", statusCode, string(respBody))
	}

	var resp NotifiersResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// HealthCheck checks service health
func (c *RESTClient) HealthCheck(ctx context.Context) (bool, error) {
	url := c.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// doRequest performs an HTTP request with retry logic
func (c *RESTClient) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var lastErr error
	var lastStatusCode int

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(c.retryBackoff):
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			}
		}

		url := c.baseURL + path
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		lastStatusCode = resp.StatusCode

		// Only retry on specific status codes
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			continue
		}

		// Success or client error (don't retry)
		return respBody, resp.StatusCode, nil
	}

	return nil, lastStatusCode, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
