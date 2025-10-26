package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/igodwin/notifier/internal/auth"
	"github.com/igodwin/notifier/internal/config"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
)

// AccountResolver is an interface for resolving default accounts
type AccountResolver interface {
	GetDefaultAccount(notifierType domain.NotificationType) string
}

// NotificationService implements the domain.NotificationService interface
type NotificationService struct {
	factory                domain.NotifierFactory
	queue                  domain.Queue
	accountResolver        AccountResolver
	authz                  *auth.NotifierAuthz
	notifications          map[string]*domain.Notification
	mu                     sync.RWMutex
	workerCount            int
	stopChan               chan struct{}
	wg                     sync.WaitGroup
	logger                 *logging.Logger
	retentionConfig        config.NotificationRetentionConfig
	cleanupStopChan        chan struct{}
	ttlDuration            time.Duration
	checkFrequencyDuration time.Duration
}

// NewNotificationService creates a new notification service
func NewNotificationService(factory domain.NotifierFactory, queue domain.Queue, workerCount int, accountResolver AccountResolver, authz *auth.NotifierAuthz, logger *logging.Logger) *NotificationService {
	if workerCount <= 0 {
		workerCount = 10
	}

	return &NotificationService{
		factory:         factory,
		queue:           queue,
		accountResolver: accountResolver,
		authz:           authz,
		notifications:   make(map[string]*domain.Notification),
		workerCount:     workerCount,
		stopChan:        make(chan struct{}),
		logger:          logger,
		cleanupStopChan: make(chan struct{}),
	}
}

// WithRetentionConfig sets the notification retention configuration
func (s *NotificationService) WithRetentionConfig(cfg config.NotificationRetentionConfig) error {
	s.retentionConfig = cfg

	// Parse TTL duration
	ttl, err := time.ParseDuration(cfg.TTL)
	if err != nil {
		return fmt.Errorf("invalid TTL duration: %w", err)
	}
	s.ttlDuration = ttl

	// Parse check frequency duration
	checkFreq, err := time.ParseDuration(cfg.CheckFrequency)
	if err != nil {
		return fmt.Errorf("invalid check frequency duration: %w", err)
	}
	s.checkFrequencyDuration = checkFreq

	return nil
}

// Start starts the worker pool and cleanup goroutine
func (s *NotificationService) Start(ctx context.Context) error {
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Start cleanup goroutine if retention is enabled
	if s.retentionConfig.Enabled && s.checkFrequencyDuration > 0 {
		s.wg.Add(1)
		go s.cleanupLoop(ctx)
	}

	return nil
}

// Stop stops the service gracefully
func (s *NotificationService) Stop() error {
	close(s.stopChan)
	close(s.cleanupStopChan)
	s.wg.Wait()
	return s.queue.Close()
}

// cleanupLoop runs at regular intervals to clean up old or excessive notifications
func (s *NotificationService) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkFrequencyDuration)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupStopChan:
			s.logger.Debugf("Cleanup loop stopped")
			return
		case <-ctx.Done():
			s.logger.Debugf("Cleanup loop context cancelled")
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes expired notifications and enforces maximum size limit
func (s *NotificationService) performCleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredBefore := now.Add(-s.ttlDuration)

	// Track which notifications to delete
	var toDelete []string
	var allNotifications []*domain.Notification

	// First pass: identify expired notifications and collect all for sorting
	for id, notification := range s.notifications {
		if notification.CreatedAt.Before(expiredBefore) {
			toDelete = append(toDelete, id)
		}
		allNotifications = append(allNotifications, notification)
	}

	// Delete expired notifications
	for _, id := range toDelete {
		delete(s.notifications, id)
	}

	expiredCount := len(toDelete)

	// Second pass: enforce max size limit by removing oldest notifications
	if s.retentionConfig.MaxSize > 0 && len(s.notifications) > s.retentionConfig.MaxSize {
		excessCount := len(s.notifications) - s.retentionConfig.MaxSize

		// Sort remaining notifications by creation time (oldest first)
		remaining := make([]*domain.Notification, 0, len(s.notifications))
		for _, notification := range s.notifications {
			remaining = append(remaining, notification)
		}

		// Simple bubble sort to find oldest notifications (more efficient alternatives available)
		for i := 0; i < len(remaining)-1; i++ {
			for j := 0; j < len(remaining)-i-1; j++ {
				if remaining[j].CreatedAt.After(remaining[j+1].CreatedAt) {
					remaining[j], remaining[j+1] = remaining[j+1], remaining[j]
				}
			}
		}

		// Delete the oldest excessCount notifications
		for i := 0; i < excessCount && i < len(remaining); i++ {
			delete(s.notifications, remaining[i].ID)
		}
	}

	currentSize := len(s.notifications)

	// Log cleanup statistics
	if expiredCount > 0 || currentSize > s.retentionConfig.MaxSize {
		s.logger.Infof("Cleanup completed - expired=%d, current_size=%d, max_size=%d",
			expiredCount, currentSize, s.retentionConfig.MaxSize)
	}
}

// worker processes notifications from the queue
func (s *NotificationService) worker(ctx context.Context, id int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Try to dequeue with timeout
			workerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			msg, err := s.queue.Dequeue(workerCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if msg == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process the notification
			s.processNotification(ctx, msg)
		}
	}
}

// processNotification sends a notification and handles the result
func (s *NotificationService) processNotification(ctx context.Context, msg *domain.QueueMessage) {
	notification := msg.Notification

	s.logger.Debugf("Processing notification - id=%s, type=%s, recipients=%d",
		notification.ID, notification.Type, len(notification.Recipients))

	// Resolve account if not specified
	account := notification.Account
	if account == "" && s.accountResolver != nil {
		account = s.accountResolver.GetDefaultAccount(notification.Type)
	}

	// Get the appropriate notifier
	notifier, err := s.factory.Create(notification.Type, account)
	if err != nil {
		s.logger.Errorf("Failed to create notifier - id=%s, type=%s, account=%s, error=%v",
			notification.ID, notification.Type, account, err)
		notification.Status = domain.StatusFailed
		notification.LastError = fmt.Sprintf("failed to create notifier: %v", err)
		s.queue.Nack(ctx, msg.ID, false)
		s.updateNotification(notification)
		return
	}

	// Send the notification
	result, err := notifier.Send(ctx, notification)
	if err != nil || !result.Success {
		notification.RetryCount++
		notification.LastError = result.Error
		if err != nil {
			notification.LastError = err.Error()
		}

		// Check if we should retry
		if notification.RetryCount < notification.MaxRetries {
			notification.Status = domain.StatusRetrying
			s.logger.Warnf("Notification send failed, will retry - id=%s, type=%s, account=%s, attempt=%d/%d, error=%s",
				notification.ID, notification.Type, account, notification.RetryCount, notification.MaxRetries, notification.LastError)
			s.queue.Nack(ctx, msg.ID, true) // Requeue
		} else {
			notification.Status = domain.StatusFailed
			s.logger.Errorf("Notification send failed permanently - id=%s, type=%s, account=%s, recipients=%v, attempts=%d, error=%s",
				notification.ID, notification.Type, account, notification.Recipients, notification.RetryCount, notification.LastError)
			s.queue.Nack(ctx, msg.ID, false) // Don't requeue
		}
	} else {
		notification.Status = domain.StatusSent
		now := time.Now()
		notification.SentAt = &now
		s.queue.Ack(ctx, msg.ID)
		s.logger.Infof("Notification sent successfully - id=%s, type=%s, account=%s, recipients=%v",
			notification.ID, notification.Type, account, notification.Recipients)
	}

	s.updateNotification(notification)
}

// Send queues a notification for delivery
func (s *NotificationService) Send(ctx context.Context, notification *domain.Notification) (*domain.NotificationResult, error) {
	// Store the notification
	s.storeNotification(notification)

	// Enqueue for processing
	if err := s.queue.Enqueue(ctx, notification); err != nil {
		return &domain.NotificationResult{
			NotificationID: notification.ID,
			Success:        false,
			Error:          fmt.Sprintf("failed to enqueue: %v", err),
			SentAt:         time.Now(),
		}, err
	}

	return &domain.NotificationResult{
		NotificationID: notification.ID,
		Success:        true,
		Message:        "notification queued successfully",
		SentAt:         time.Now(),
	}, nil
}

// SendBatch queues multiple notifications for delivery
func (s *NotificationService) SendBatch(ctx context.Context, notifications []*domain.Notification) ([]*domain.NotificationResult, error) {
	results := make([]*domain.NotificationResult, 0, len(notifications))

	// Store all notifications
	for _, notification := range notifications {
		s.storeNotification(notification)
	}

	// Enqueue batch
	if err := s.queue.EnqueueBatch(ctx, notifications); err != nil {
		return nil, fmt.Errorf("failed to enqueue batch: %w", err)
	}

	// Create results
	for _, notification := range notifications {
		results = append(results, &domain.NotificationResult{
			NotificationID: notification.ID,
			Success:        true,
			Message:        "notification queued successfully",
			SentAt:         time.Now(),
		})
	}

	return results, nil
}

// GetNotification retrieves a notification by ID
func (s *NotificationService) GetNotification(ctx context.Context, id string) (*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notification, exists := s.notifications[id]
	if !exists {
		return nil, fmt.Errorf("notification not found: %s", id)
	}

	return notification, nil
}

// ListNotifications retrieves notifications matching the filter
func (s *NotificationService) ListNotifications(ctx context.Context, filter *domain.NotificationFilter) ([]*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simple in-memory filtering
	var results []*domain.Notification

	for _, notification := range s.notifications {
		if s.matchesFilter(notification, filter) {
			results = append(results, notification)
		}
	}

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// CancelNotification cancels a pending notification
func (s *NotificationService) CancelNotification(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification, exists := s.notifications[id]
	if !exists {
		return fmt.Errorf("notification not found: %s", id)
	}

	if notification.Status == domain.StatusSent {
		return fmt.Errorf("notification already sent")
	}

	notification.Status = domain.StatusFailed
	notification.LastError = "cancelled by user"

	return nil
}

// RetryNotification retries a failed notification
func (s *NotificationService) RetryNotification(ctx context.Context, id string) (*domain.NotificationResult, error) {
	notification, err := s.GetNotification(ctx, id)
	if err != nil {
		return nil, err
	}

	if notification.Status == domain.StatusSent {
		return &domain.NotificationResult{
			NotificationID: id,
			Success:        false,
			Error:          "notification already sent",
			SentAt:         time.Now(),
		}, fmt.Errorf("notification already sent")
	}

	// Reset retry count and status
	notification.RetryCount = 0
	notification.Status = domain.StatusPending

	// Re-enqueue
	return s.Send(ctx, notification)
}

// GetStats returns notification statistics
func (s *NotificationService) GetStats(ctx context.Context) (*domain.NotificationStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &domain.NotificationStats{
		ByType:   make(map[string]int64),
		ByStatus: make(map[string]int64),
	}

	for _, notification := range s.notifications {
		switch notification.Status {
		case domain.StatusSent:
			stats.TotalSent++
		case domain.StatusFailed:
			stats.TotalFailed++
		case domain.StatusPending:
			stats.TotalPending++
		case domain.StatusQueued:
			stats.TotalQueued++
		}

		stats.ByType[string(notification.Type)]++
		stats.ByStatus[string(notification.Status)]++
	}

	return stats, nil
}

// GetNotifiers returns information about available notifiers, filtered by authorization if auth context is provided
func (s *NotificationService) GetNotifiers(ctx context.Context) (*domain.NotifiersResponse, error) {
	// Extract auth context from request context if available
	var authCtx *auth.AuthContext
	if authVal := ctx.Value("auth"); authVal != nil {
		if ac, ok := authVal.(*auth.AuthContext); ok {
			authCtx = ac
		}
	}

	supportedTypes := s.factory.SupportedTypes()
	notifiers := make([]domain.NotifierInfo, 0, len(supportedTypes))

	for _, notifType := range supportedTypes {
		accounts := s.factory.GetAccounts(notifType)

		// Filter accounts by authorization if auth context is available and authz is configured
		if authCtx != nil && s.authz != nil {
			authorizedAccounts := make([]string, 0, len(accounts))
			for _, account := range accounts {
				if s.authz.IsAuthorized(authCtx, notifType, account) {
					authorizedAccounts = append(authorizedAccounts, account)
				}
			}
			accounts = authorizedAccounts
		}

		// Skip notifier type if no authorized accounts
		if len(accounts) == 0 && authCtx != nil {
			continue
		}

		defaultAccount := ""
		if s.accountResolver != nil {
			defaultAccount = s.accountResolver.GetDefaultAccount(notifType)
		}

		// If default account was filtered out, clear it
		if authCtx != nil && s.authz != nil && defaultAccount != "" {
			if !s.authz.IsAuthorized(authCtx, notifType, defaultAccount) {
				defaultAccount = ""
				// If available, use first authorized account as default
				if len(accounts) > 0 {
					defaultAccount = accounts[0]
				}
			}
		}

		notifiers = append(notifiers, domain.NotifierInfo{
			Type:           notifType,
			Accounts:       accounts,
			DefaultAccount: defaultAccount,
		})
	}

	return &domain.NotifiersResponse{
		Notifiers: notifiers,
	}, nil
}

// storeNotification stores a notification in memory
func (s *NotificationService) storeNotification(notification *domain.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications[notification.ID] = notification
}

// updateNotification updates a notification in memory
func (s *NotificationService) updateNotification(notification *domain.Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications[notification.ID] = notification
}

// matchesFilter checks if a notification matches the filter
func (s *NotificationService) matchesFilter(notification *domain.Notification, filter *domain.NotificationFilter) bool {
	if filter == nil {
		return true
	}

	// Check IDs
	if len(filter.IDs) > 0 {
		found := false
		for _, id := range filter.IDs {
			if notification.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if notification.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check statuses
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if notification.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check recipients
	if len(filter.Recipients) > 0 {
		found := false
		for _, fr := range filter.Recipients {
			for _, nr := range notification.Recipients {
				if fr == nr {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check time ranges
	if filter.CreatedAfter != nil && notification.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}

	if filter.CreatedBefore != nil && notification.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	return true
}
