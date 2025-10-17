package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/igodwin/notifier/internal/domain"
)

// NotificationService implements the domain.NotificationService interface
type NotificationService struct {
	factory       domain.NotifierFactory
	queue         domain.Queue
	notifications map[string]*domain.Notification
	mu            sync.RWMutex
	workerCount   int
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewNotificationService creates a new notification service
func NewNotificationService(factory domain.NotifierFactory, queue domain.Queue, workerCount int) *NotificationService {
	if workerCount <= 0 {
		workerCount = 10
	}

	return &NotificationService{
		factory:       factory,
		queue:         queue,
		notifications: make(map[string]*domain.Notification),
		workerCount:   workerCount,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the worker pool
func (s *NotificationService) Start(ctx context.Context) error {
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}
	return nil
}

// Stop stops the service gracefully
func (s *NotificationService) Stop() error {
	close(s.stopChan)
	s.wg.Wait()
	return s.queue.Close()
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

	// Get the appropriate notifier
	notifier, err := s.factory.Create(notification.Type)
	if err != nil {
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
			s.queue.Nack(ctx, msg.ID, true) // Requeue
		} else {
			notification.Status = domain.StatusFailed
			s.queue.Nack(ctx, msg.ID, false) // Don't requeue
		}
	} else {
		notification.Status = domain.StatusSent
		now := time.Now()
		notification.SentAt = &now
		s.queue.Ack(ctx, msg.ID)
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
