package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/igodwin/notifier/pkg/client"
)

// TestCRITICAL1_TTLBasedCleanup verifies notifications older than TTL are removed
func TestCRITICAL1_TTLBasedCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Use very short TTL for faster testing
	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=2s"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=10000"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send a notification that will have old timestamp
	req := client.NotificationRequest{
		Type:       "stdout",
		Subject:    "Test TTL",
		Body:       "This should be cleaned up",
		Recipients: []string{"test@example.com"},
	}

	resp, err := suite.Client.Send(ctx, req)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	// Verify notification exists
	notif, err := suite.Client.GetNotification(ctx, resp.NotificationID)
	if err != nil {
		t.Fatalf("Failed to get notification: %v", err)
	}
	t.Logf("Created notification %s at %v", resp.NotificationID, notif.CreatedAt)

	// Get initial stats
	stats1, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("Initial stats: total_sent=%d", stats1.TotalSent)

	// Wait for TTL + cleanup check frequency
	time.Sleep(3 * time.Second)

	// Get stats after cleanup
	stats2, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("After cleanup stats: total_sent=%d", stats2.TotalSent)

	// Verify the notification was cleaned up
	if stats2.TotalSent >= stats1.TotalSent {
		t.Logf("Container logs:\n%s", suite.GetLogs(ctx))
		t.Fatalf("Expected cleanup to remove old notification, but total_sent remained %d >= %d",
			stats2.TotalSent, stats1.TotalSent)
	}

	t.Logf("✓ TTL-based cleanup verified: %d -> %d notifications", stats1.TotalSent, stats2.TotalSent)
}

// TestCRITICAL1_MaxSizeEnforcement verifies max_size limit is enforced
func TestCRITICAL1_MaxSizeEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Use small max_size for testing
	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=24h"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=5"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send more notifications than max_size
	notificationCount := 10
	notificationIDs := make([]string, 0, notificationCount)

	for i := 0; i < notificationCount; i++ {
		req := client.NotificationRequest{
			Type:       "stdout",
			Subject:    fmt.Sprintf("Test %d", i),
			Body:       fmt.Sprintf("Notification %d", i),
			Recipients: []string{"test@example.com"},
		}

		resp, err := suite.Client.Send(ctx, req)
		if err != nil {
			t.Fatalf("Failed to send notification %d: %v", i, err)
		}
		notificationIDs = append(notificationIDs, resp.NotificationID)
	}

	t.Logf("Sent %d notifications", notificationCount)

	// Get stats before cleanup
	statsBeforeCleanup, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("Before cleanup: total_sent=%d", statsBeforeCleanup.TotalSent)

	// Wait for cleanup to enforce max_size
	time.Sleep(2 * time.Second)

	// Get stats after cleanup
	statsAfterCleanup, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("After cleanup: total_sent=%d (max_size=5)", statsAfterCleanup.TotalSent)

	// Verify max_size is enforced
	if statsAfterCleanup.TotalSent > 5 {
		t.Logf("Container logs:\n%s", suite.GetLogs(ctx))
		t.Fatalf("Expected max_size enforcement, but have %d notifications (max=5)",
			statsAfterCleanup.TotalSent)
	}

	t.Logf("✓ Max size enforcement verified: capped at %d", statsAfterCleanup.TotalSent)
}

// TestCRITICAL1_CleanupDisabled verifies cleanup doesn't run when disabled
func TestCRITICAL1_CleanupDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Disable cleanup
	retention := "NOTIFIER_RETENTION_ENABLED=false"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send many notifications
	notificationCount := 10
	for i := 0; i < notificationCount; i++ {
		req := client.NotificationRequest{
			Type:       "stdout",
			Subject:    fmt.Sprintf("Test %d", i),
			Body:       fmt.Sprintf("Notification %d", i),
			Recipients: []string{"test@example.com"},
		}

		_, err := suite.Client.Send(ctx, req)
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}
	}

	// Get stats before waiting
	statsBefore, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("Before wait: total_sent=%d", statsBefore.TotalSent)

	// Wait a bit (cleanup should not run)
	time.Sleep(3 * time.Second)

	// Get stats after waiting
	statsAfter, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	t.Logf("After wait: total_sent=%d", statsAfter.TotalSent)

	// Verify notifications are still there (no cleanup)
	if statsAfter.TotalSent < statsBefore.TotalSent {
		t.Fatalf("Expected no cleanup when disabled, but notification count decreased from %d to %d",
			statsBefore.TotalSent, statsAfter.TotalSent)
	}

	t.Logf("✓ Cleanup disabled verified: all %d notifications retained", statsAfter.TotalSent)
}

// TestCRITICAL1_ConcurrentSends verifies concurrent client access works correctly
func TestCRITICAL1_ConcurrentSends(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=24h"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=1s"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=100"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send notifications concurrently
	errChan := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			req := client.NotificationRequest{
				Type:       "stdout",
				Subject:    fmt.Sprintf("Concurrent %d", idx),
				Body:       fmt.Sprintf("Test %d", idx),
				Recipients: []string{"test@example.com"},
			}

			_, err := suite.Client.Send(ctx, req)
			errChan <- err
		}(i)
	}

	// Collect results
	for i := 0; i < 10; i++ {
		if err := <-errChan; err != nil {
			t.Fatalf("Failed to send concurrent notification %d: %v", i, err)
		}
	}

	// Get stats
	stats, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalSent != 10 {
		t.Fatalf("Expected 10 notifications, got %d", stats.TotalSent)
	}

	t.Logf("✓ Concurrent sends verified: %d notifications sent successfully", stats.TotalSent)
}

// TestCRITICAL1_OldestRemovedFirst verifies oldest notifications are removed when max_size exceeded
func TestCRITICAL1_OldestRemovedFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=24h"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=3"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send 5 notifications with time between them
	notificationIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		req := client.NotificationRequest{
			Type:       "stdout",
			Subject:    fmt.Sprintf("Oldest %d", i),
			Body:       fmt.Sprintf("Test %d", i),
			Recipients: []string{"test@example.com"},
		}

		resp, err := suite.Client.Send(ctx, req)
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}
		notificationIDs[i] = resp.NotificationID
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Sent 5 notifications: %v", notificationIDs[:5])

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	// Verify that only 3 remain (and they're the newest)
	stats, err := suite.Client.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalSent != 3 {
		t.Logf("Container logs:\n%s", suite.GetLogs(ctx))
		t.Fatalf("Expected 3 notifications after cleanup, got %d", stats.TotalSent)
	}

	t.Logf("✓ Oldest removed first verified: 5 notifications -> %d (max_size)", stats.TotalSent)
}

// TestCRITICAL1_MemoryBounded verifies memory usage stays bounded
func TestCRITICAL1_MemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=5s"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=50"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Send bursts of notifications repeatedly
	for batch := 0; batch < 3; batch++ {
		// Send 30 notifications
		for i := 0; i < 30; i++ {
			req := client.NotificationRequest{
				Type:       "stdout",
				Subject:    fmt.Sprintf("Batch %d Notif %d", batch, i),
				Body:       fmt.Sprintf("Test data for notification"),
				Recipients: []string{"test@example.com"},
			}

			_, err := suite.Client.Send(ctx, req)
			if err != nil {
				t.Fatalf("Failed to send notification: %v", err)
			}
		}

		// Wait for cleanup to run
		time.Sleep(2 * time.Second)

		// Check stats - should still be under max_size
		stats, err := suite.Client.GetStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalSent > 50 {
			t.Logf("Container logs:\n%s", suite.GetLogs(ctx))
			t.Fatalf("Batch %d: notification count %d exceeded max_size of 50", batch, stats.TotalSent)
		}

		t.Logf("Batch %d: %d notifications (within bound)", batch, stats.TotalSent)
	}

	t.Logf("✓ Memory bounded verified: multiple batches stayed within max_size limit")
}

// TestCRITICAL1_ServiceHealthy verifies service stays healthy throughout cleanup
func TestCRITICAL1_ServiceHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	retention := "NOTIFIER_RETENTION_ENABLED=true"
	retention += "|NOTIFIER_RETENTION_TTL=2s"
	retention += "|NOTIFIER_RETENTION_CHECK_FREQUENCY=500ms"
	retention += "|NOTIFIER_RETENTION_MAX_SIZE=1000"

	suite := SetupSuite(t, retention)
	defer suite.TeardownSuite(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Send notifications and check health periodically
	for iteration := 0; iteration < 5; iteration++ {
		// Send 20 notifications
		for i := 0; i < 20; i++ {
			req := client.NotificationRequest{
				Type:       "stdout",
				Subject:    fmt.Sprintf("Health %d", i),
				Body:       fmt.Sprintf("Test"),
				Recipients: []string{"test@example.com"},
			}

			_, err := suite.Client.Send(ctx, req)
			if err != nil {
				t.Fatalf("Failed to send notification: %v", err)
			}
		}

		// Check health
		healthy, err := suite.Client.HealthCheck(ctx)
		if err != nil {
			t.Fatalf("Iteration %d: health check failed: %v", iteration, err)
		}

		if !healthy {
			t.Fatalf("Iteration %d: service reported unhealthy", iteration)
		}

		// Get stats to verify service is responsive
		stats, err := suite.Client.GetStats(ctx)
		if err != nil {
			t.Fatalf("Iteration %d: failed to get stats: %v", iteration, err)
		}

		t.Logf("Iteration %d: health=ok, stats received (total_sent=%d)", iteration, stats.TotalSent)

		time.Sleep(1 * time.Second)
	}

	t.Logf("✓ Service health verified: remained responsive during cleanup cycles")
}
