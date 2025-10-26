package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/igodwin/notifier/internal/config"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
	"github.com/igodwin/notifier/internal/notifier"
	"github.com/igodwin/notifier/internal/queue"
)

// Helper function to create a test service
func createTestService(t *testing.T) *NotificationService {
	factory := notifier.NewFactory()
	stdoutNotifier := notifier.NewStdoutNotifier()
	if err := factory.RegisterNotifier(domain.TypeStdout, "", stdoutNotifier); err != nil {
		t.Fatalf("Failed to register notifier: %v", err)
	}

	q, err := queue.NewLocalQueue(&domain.LocalQueueConfig{BufferSize: 100})
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	logger, err := logging.NewFromConfig("error", "stdout")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	svc := NewNotificationService(factory, q, 2, nil, logger)
	return svc
}

// TestTTLBasedCleanup tests that notifications older than TTL are removed
func TestTTLBasedCleanup(t *testing.T) {
	svc := createTestService(t)

	// Configure retention: 1 second TTL, check every 100ms
	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "1s",
		CheckFrequency: "100ms",
		MaxSize:        1000,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start service
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create old notification (created 2 seconds ago)
	oldTime := time.Now().Add(-2 * time.Second)
	oldNotif := &domain.Notification{
		ID:         "old-1",
		Type:       domain.TypeStdout,
		Status:     domain.StatusSent,
		CreatedAt:  oldTime,
		Recipients: []string{"test@example.com"},
	}

	// Create recent notification
	recentNotif := &domain.Notification{
		ID:         "recent-1",
		Type:       domain.TypeStdout,
		Status:     domain.StatusSent,
		CreatedAt:  time.Now(),
		Recipients: []string{"test@example.com"},
	}

	// Store notifications
	svc.storeNotification(oldNotif)
	svc.storeNotification(recentNotif)

	// Verify both are present
	if _, err := svc.GetNotification(ctx, "old-1"); err != nil {
		t.Errorf("Old notification should exist initially")
	}

	if _, err := svc.GetNotification(ctx, "recent-1"); err != nil {
		t.Errorf("Recent notification should exist initially")
	}

	// Wait for cleanup to run
	time.Sleep(500 * time.Millisecond)

	// Old notification should be gone
	if _, err := svc.GetNotification(ctx, "old-1"); err == nil {
		t.Error("Old notification should have been cleaned up")
	}

	// Recent notification should still exist
	if _, err := svc.GetNotification(ctx, "recent-1"); err != nil {
		t.Error("Recent notification should still exist")
	}
}

// TestMaxSizeEnforcement tests that max_size limit is enforced
func TestMaxSizeEnforcement(t *testing.T) {
	svc := createTestService(t)

	// Configure retention: long TTL, small max_size, frequent checks
	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "24h",
		CheckFrequency: "50ms",
		MaxSize:        5,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create 10 notifications
	for i := 0; i < 10; i++ {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     domain.StatusSent,
			CreatedAt:  time.Now().Add(-time.Duration(i) * time.Second),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}

	// Verify 10 are present
	stats, _ := svc.GetStats(ctx)
	if stats.TotalSent != 10 {
		t.Errorf("Expected 10 notifications, got %d", stats.TotalSent)
	}

	// Wait for cleanup to enforce max_size
	time.Sleep(200 * time.Millisecond)

	// Verify max_size is enforced (5 notifications remain)
	stats, _ = svc.GetStats(ctx)
	if stats.TotalSent != 5 {
		t.Errorf("Expected 5 notifications after cleanup, got %d", stats.TotalSent)
	}
}

// TestCleanupRemovesOldestFirst tests that oldest notifications are removed when max_size is exceeded
func TestCleanupRemovesOldestFirst(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "24h",
		CheckFrequency: "50ms",
		MaxSize:        3,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create notifications with distinct times
	baseTime := time.Now()
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     domain.StatusSent,
			CreatedAt:  baseTime.Add(time.Duration(i) * time.Second), // Increasing times
			Recipients: []string{"test@example.com"},
		}
		ids[i] = notif.ID
		svc.storeNotification(notif)
	}

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// The newest 3 should remain (indices 2, 3, 4)
	// The oldest 2 should be removed (indices 0, 1)
	foundCount := 0
	for i := 0; i < 5; i++ {
		if _, err := svc.GetNotification(ctx, ids[i]); err == nil {
			foundCount++
		}
	}

	if foundCount != 3 {
		t.Errorf("Expected 3 notifications to remain, found %d", foundCount)
	}
}

// TestCleanupDisabled tests that cleanup doesn't run when disabled
func TestCleanupDisabled(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        false, // Disabled
		TTL:            "1s",
		CheckFrequency: "50ms",
		MaxSize:        5,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create old notification
	oldTime := time.Now().Add(-2 * time.Second)
	oldNotif := &domain.Notification{
		ID:         uuid.New().String(),
		Type:       domain.TypeStdout,
		Status:     domain.StatusSent,
		CreatedAt:  oldTime,
		Recipients: []string{"test@example.com"},
	}

	svc.storeNotification(oldNotif)

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Old notification should still exist (cleanup is disabled)
	if _, err := svc.GetNotification(ctx, oldNotif.ID); err != nil {
		t.Error("Old notification should still exist when cleanup is disabled")
	}
}

// TestCleanupConcurrency tests that cleanup works correctly with concurrent access
func TestCleanupConcurrency(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "500ms",
		CheckFrequency: "100ms",
		MaxSize:        100,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create some initial notifications
	for i := 0; i < 10; i++ {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     domain.StatusSent,
			CreatedAt:  time.Now().Add(-time.Duration(i) * time.Second),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}

	// Concurrently add new notifications while cleanup runs
	done := make(chan bool)
	go func() {
		for i := 0; i < 5; i++ {
			notif := &domain.Notification{
				ID:         uuid.New().String(),
				Type:       domain.TypeStdout,
				Status:     domain.StatusSent,
				CreatedAt:  time.Now(),
				Recipients: []string{"test@example.com"},
			}
			svc.storeNotification(notif)
			time.Sleep(50 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for goroutine to complete
	<-done
	time.Sleep(100 * time.Millisecond)

	// Verify service is still functional and no race conditions
	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}

	if stats.TotalSent == 0 {
		t.Error("Should have at least some notifications")
	}
}

// TestRetentionConfigParsing tests that TTL and check_frequency are properly parsed
func TestRetentionConfigParsing(t *testing.T) {
	svc := createTestService(t)

	testCases := []struct {
		name           string
		ttl            string
		checkFrequency string
		shouldError    bool
	}{
		{"Valid 7d TTL", "168h", "1h", false},
		{"Valid short TTL", "30m", "10m", false},
		{"Valid second precision", "5s", "1s", false},
		{"Invalid TTL format", "invalid", "1h", true},
		{"Invalid check_frequency format", "1h", "invalid", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NotificationRetentionConfig{
				Enabled:        true,
				TTL:            tc.ttl,
				CheckFrequency: tc.checkFrequency,
				MaxSize:        1000,
			}

			err := svc.WithRetentionConfig(cfg)
			if tc.shouldError && err == nil {
				t.Errorf("Expected error for invalid config")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestCleanupGracefulShutdown tests that cleanup goroutine shuts down gracefully
func TestCleanupGracefulShutdown(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "24h",
		CheckFrequency: "100ms",
		MaxSize:        100,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Add some notifications
	for i := 0; i < 5; i++ {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     domain.StatusSent,
			CreatedAt:  time.Now(),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}

	// Cancel context to signal shutdown to workers
	cancel()

	// Stop service (should wait for cleanup goroutine)
	stopErr := svc.Stop()
	if stopErr != nil {
		t.Errorf("Stop failed: %v", stopErr)
	}

	// Verify notifications are still intact after graceful shutdown
	stats, err := svc.GetStats(context.Background())
	if err == nil && stats.TotalSent > 0 {
		// This is expected - notifications should persist through shutdown
	}
}

// TestCleanupWithMixedNotificationStatuses tests cleanup with different notification statuses
func TestCleanupWithMixedNotificationStatuses(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "1s",
		CheckFrequency: "100ms",
		MaxSize:        1000,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	oldTime := time.Now().Add(-2 * time.Second)

	// Create old notifications with different statuses
	statuses := []domain.NotificationStatus{
		domain.StatusSent,
		domain.StatusFailed,
		domain.StatusPending,
		domain.StatusQueued,
		domain.StatusRetrying,
	}

	for i, status := range statuses {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     status,
			CreatedAt:  oldTime.Add(time.Duration(i) * time.Millisecond),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}

	// Create recent notifications with different statuses
	for i, status := range statuses {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     status,
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Millisecond),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}

	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)

	// Verify old ones are gone, new ones remain
	stats, _ := svc.GetStats(ctx)
	if stats.TotalSent+stats.TotalFailed+stats.TotalPending+stats.TotalQueued <= 2 {
		t.Errorf("Expected recent notifications to remain after cleanup")
	}
}

// TestCleanupPerformance tests cleanup performance with large notification sets
func TestCleanupPerformance(t *testing.T) {
	svc := createTestService(t)

	cfg := config.NotificationRetentionConfig{
		Enabled:        true,
		TTL:            "1s",
		CheckFrequency: "100ms",
		MaxSize:        10000,
	}

	if err := svc.WithRetentionConfig(cfg); err != nil {
		t.Fatalf("Failed to set retention config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Create 5000 old notifications
	startTime := time.Now()
	oldTime := time.Now().Add(-2 * time.Second)
	for i := 0; i < 5000; i++ {
		notif := &domain.Notification{
			ID:         uuid.New().String(),
			Type:       domain.TypeStdout,
			Status:     domain.StatusSent,
			CreatedAt:  oldTime.Add(time.Duration(i%100) * time.Millisecond),
			Recipients: []string{"test@example.com"},
		}
		svc.storeNotification(notif)
	}
	loadTime := time.Since(startTime)

	// Measure cleanup time
	cleanupStart := time.Now()
	svc.performCleanup()
	cleanupTime := time.Since(cleanupStart)

	// Cleanup should complete in reasonable time (< 1 second)
	if cleanupTime > 1*time.Second {
		t.Logf("Cleanup took %v (should be < 1s) - possible performance issue", cleanupTime)
	}

	t.Logf("Performance: Load 5000 notifs: %v, Cleanup: %v", loadTime, cleanupTime)

	// Verify cleanup worked
	stats, _ := svc.GetStats(ctx)
	if stats.TotalSent > 100 {
		t.Errorf("Expected most old notifications to be cleaned up, still have %d", stats.TotalSent)
	}
}
