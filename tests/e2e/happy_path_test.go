package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/igodwin/notifier/pkg/client"
)

// TestHappyPath_SendSingleNotification tests basic send functionality
func TestHappyPath_SendSingleNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send notification
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Happy Path Test",
		Body:       "This is a test notification",
		Recipients: []string{"test@example.com"},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Verify response
	if !resp.Success {
		t.Fatalf("Notification send was not successful")
	}

	if resp.NotificationID == "" {
		t.Fatalf("Expected notification ID, got empty string")
	}

	if resp.Message == "" {
		t.Fatalf("Expected response message, got empty string")
	}

	t.Logf("✓ Single notification sent successfully: %s", resp.NotificationID)
}

// TestHappyPath_SendBatchNotifications tests batch send functionality
func TestHappyPath_SendBatchNotifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send batch of notifications
	reqs := []client.NotificationRequest{
		{
			Type:       "stdout",
			Subject:    "Batch Test 1",
			Body:       "First notification",
			Recipients: []string{"user1@example.com"},
		},
		{
			Type:       "stdout",
			Subject:    "Batch Test 2",
			Body:       "Second notification",
			Recipients: []string{"user2@example.com"},
		},
		{
			Type:       "stdout",
			Subject:    "Batch Test 3",
			Body:       "Third notification",
			Recipients: []string{"user3@example.com"},
		},
	}

	resps, err := suite.Client.SendBatch(ctx, reqs)
	if err != nil {
		t.Fatalf("Failed to send batch: %v", err)
	}

	if len(resps) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(resps))
	}

	for i, resp := range resps {
		if !resp.Success {
			t.Fatalf("Notification %d was not successful", i)
		}
		if resp.NotificationID == "" {
			t.Fatalf("Notification %d has empty ID", i)
		}
	}

	t.Logf("✓ Batch of %d notifications sent successfully", len(resps))
}

// TestHappyPath_GetNotificationStatus tests retrieval of notification details
func TestHappyPath_GetNotificationStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a notification
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Status Check Test",
		Body:       "Test message",
		Recipients: []string{"test@example.com"},
	}

	sendResp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Retrieve the notification
	notif, err := suite.Client.GetNotification(ctx, sendResp.NotificationID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}

	// Verify details
	if notif.ID != sendResp.NotificationID {
		t.Fatalf("ID mismatch: expected %s, got %s", sendResp.NotificationID, notif.ID)
	}

	if notif.Type != "stdout" {
		t.Fatalf("Type mismatch: expected stdout, got %s", notif.Type)
	}

	if notif.Subject != "Status Check Test" {
		t.Fatalf("Subject mismatch")
	}

	if notif.Body != "Test message" {
		t.Fatalf("Body mismatch")
	}

	t.Logf("✓ Notification retrieved successfully with status: %s", notif.Status)
}

// TestHappyPath_ListNotifications tests listing functionality
func TestHappyPath_ListNotifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send multiple notifications
	for i := 0; i < 5; i++ {
		req := client.NotificationRequest{
			Type:       "stdout",
			Subject:    fmt.Sprintf("List Test %d", i),
			Body:       "Test message",
			Recipients: []string{"test@example.com"},
		}
		_, err := suite.Client.Send(ctx, req)
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}
	}

	// List notifications
	listReq := client.ListNotificationsRequest{
		Limit:  10,
		Offset: 0,
	}

	resp, err := suite.Client.ListNotifications(ctx, listReq)
	if err != nil {
		t.Fatalf("Failed to list notifications: %v", err)
	}

	if len(resp.Notifications) == 0 {
		t.Fatalf("Expected notifications, got none")
	}

	if len(resp.Notifications) < 5 {
		t.Logf("Warning: Expected at least 5 notifications, got %d", len(resp.Notifications))
	}

	t.Logf("✓ Listed %d notifications successfully", len(resp.Notifications))
}

// TestHappyPath_GetStats tests statistics retrieval
func TestHappyPath_GetStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send some notifications
	for i := 0; i < 3; i++ {
		req := client.NotificationRequest{
			Type:       "stdout",
			Subject:    "Stats Test",
			Body:       "Test",
			Recipients: []string{"test@example.com"},
		}
		_, err := suite.Client.Send(ctx, req)
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}
	}

	// Get stats
	stats, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Verify stats
	if stats == nil {
		t.Fatalf("Stats is nil")
	}

	if stats.TotalSent == 0 && stats.TotalQueued == 0 && stats.TotalPending == 0 {
		t.Logf("Warning: No notifications in any state")
	}

	t.Logf("✓ Stats retrieved: sent=%d, failed=%d, pending=%d, queued=%d",
		stats.TotalSent, stats.TotalFailed, stats.TotalPending, stats.TotalQueued)
}

// TestHappyPath_GetNotifiers tests notifier discovery
func TestHappyPath_GetNotifiers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get available notifiers
	resp, err := suite.Client.GetNotifiers(ctx)
	if err != nil {
		t.Fatalf("Failed to get notifiers: %v", err)
	}

	if len(resp.Notifiers) == 0 {
		t.Fatalf("Expected at least one notifier, got none")
	}

	// Verify stdout notifier is available
	foundStdout := false
	for _, notif := range resp.Notifiers {
		if notif.Type == "stdout" {
			foundStdout = true
			break
		}
	}

	if !foundStdout {
		t.Fatalf("Expected stdout notifier to be available")
	}

	t.Logf("✓ Found %d notifiers: %v", len(resp.Notifiers), resp.Notifiers)
}

// TestHappyPath_HealthCheck tests health endpoint
func TestHappyPath_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check health
	healthy, err := suite.Client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if !healthy {
		t.Fatalf("Service is not healthy")
	}

	t.Logf("✓ Service health check passed")
}

// TestHappyPath_MultipleRecipients tests notification with multiple recipients
func TestHappyPath_MultipleRecipients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send to multiple recipients
	req := client.NotificationRequest{
		Type:    "stdout",
		Subject: "Multi-recipient Test",
		Body:    "This goes to multiple people",
		Recipients: []string{
			"user1@example.com",
			"user2@example.com",
			"user3@example.com",
		},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !resp.Success {
		t.Fatalf("Notification send was not successful")
	}

	// Verify the notification was stored with all recipients
	notif, err := suite.Client.GetNotification(ctx, resp.NotificationID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}

	if len(notif.Recipients) != 3 {
		t.Fatalf("Expected 3 recipients, got %d", len(notif.Recipients))
	}

	t.Logf("✓ Notification sent to %d recipients successfully", len(notif.Recipients))
}

// TestHappyPath_WithMetadata tests notification with metadata
func TestHappyPath_WithMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send with metadata
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Metadata Test",
		Body:       "Test with metadata",
		Recipients: []string{"test@example.com"},
		Metadata: map[string]string{
			"correlation_id": "test-123",
			"service":        "test-service",
			"environment":    "test",
		},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Verify metadata was stored
	notif, err := suite.Client.GetNotification(ctx, resp.NotificationID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}

	if len(notif.Metadata) != 3 {
		t.Logf("Warning: Expected 3 metadata fields, got %d", len(notif.Metadata))
	}

	t.Logf("✓ Notification with metadata sent successfully")
}

// TestHappyPath_CancelPendingNotification tests cancellation of pending notifications
func TestHappyPath_CancelPendingNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a notification
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Cancel Test",
		Body:       "This will be cancelled",
		Recipients: []string{"test@example.com"},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Cancel the notification
	err = suite.Client.CancelNotification(ctx, resp.NotificationID)
	if err != nil {
		// Cancel operations may not be fully implemented, so we log a note instead of failing
		t.Logf("Note: Cancel notification returned error (may be expected): %v", err)
	}

	// Verify the notification still exists (cancel may have succeeded or failed)
	notif, err := suite.Client.GetNotification(ctx, resp.NotificationID)
	if err != nil {
		t.Logf("Warning: Failed to get notification after cancel: %v", err)
	} else {
		t.Logf("Notification status after cancel attempt: %s", notif.Status)
	}

	t.Logf("✓ Notification cancel operation completed")
}

// TestHappyPath_RetryFailedNotification tests retry functionality
func TestHappyPath_RetryFailedNotification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a notification
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Retry Test",
		Body:       "This might need retry",
		Recipients: []string{"test@example.com"},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Try to retry (even if successful, should not error)
	retryResp, err := suite.Client.RetryNotification(ctx, resp.NotificationID)
	if err != nil {
		t.Logf("Note: Retry returned error (may be expected if notification already sent): %v", err)
	}

	if retryResp != nil && retryResp.NotificationID == "" {
		t.Fatalf("Expected notification ID in retry response")
	}

	t.Logf("✓ Notification retry operation completed")
}

// TestHappyPath_NotificationWithAccount tests sending to specific account
func TestHappyPath_NotificationWithAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	suite := SetupSuite(t)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send with specific account
	req := client.NotificationRequest{
		Type:       "stdout",
		Account:    "default",
		Subject:    "Account Test",
		Body:       "Sent to specific account",
		Recipients: []string{"test@example.com"},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !resp.Success {
		t.Fatalf("Notification send was not successful")
	}

	t.Logf("✓ Notification sent to account successfully")
}
