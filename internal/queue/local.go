package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/igodwin/notifier/internal/domain"
)

// LocalQueue is an in-memory queue implementation
type LocalQueue struct {
	queue         chan *domain.QueueMessage
	messages      map[string]*domain.QueueMessage
	mu            sync.RWMutex
	config        *domain.LocalQueueConfig
	persistToDisk bool
	persistPath   string
	closed        bool
	closeChan     chan struct{}
}

// NewLocalQueue creates a new local queue instance
func NewLocalQueue(config *domain.LocalQueueConfig) (*LocalQueue, error) {
	if config == nil {
		config = &domain.LocalQueueConfig{
			BufferSize:    1000,
			PersistToDisk: false,
		}
	}

	lq := &LocalQueue{
		queue:         make(chan *domain.QueueMessage, config.BufferSize),
		messages:      make(map[string]*domain.QueueMessage),
		config:        config,
		persistToDisk: config.PersistToDisk,
		persistPath:   config.PersistPath,
		closeChan:     make(chan struct{}),
	}

	// Load persisted messages if enabled
	if lq.persistToDisk && lq.persistPath != "" {
		if err := lq.loadFromDisk(); err != nil {
			return nil, fmt.Errorf("failed to load persisted queue: %w", err)
		}
	}

	return lq, nil
}

// Enqueue adds a notification to the queue
func (lq *LocalQueue) Enqueue(ctx context.Context, notification *domain.Notification) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return fmt.Errorf("queue is closed")
	}

	msg := &domain.QueueMessage{
		ID:           uuid.New().String(),
		Notification: notification,
		Attempt:      0,
		EnqueuedAt:   time.Now().Unix(),
	}

	select {
	case lq.queue <- msg:
		lq.messages[msg.ID] = msg
		notification.Status = domain.StatusQueued

		if lq.persistToDisk {
			return lq.persistToDiskSync()
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-lq.closeChan:
		return fmt.Errorf("queue is closed")
	}
}

// EnqueueBatch adds multiple notifications to the queue
func (lq *LocalQueue) EnqueueBatch(ctx context.Context, notifications []*domain.Notification) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return fmt.Errorf("queue is closed")
	}

	for _, notification := range notifications {
		msg := &domain.QueueMessage{
			ID:           uuid.New().String(),
			Notification: notification,
			Attempt:      0,
			EnqueuedAt:   time.Now().Unix(),
		}

		select {
		case lq.queue <- msg:
			lq.messages[msg.ID] = msg
			notification.Status = domain.StatusQueued
		case <-ctx.Done():
			return ctx.Err()
		case <-lq.closeChan:
			return fmt.Errorf("queue is closed")
		}
	}

	if lq.persistToDisk {
		return lq.persistToDiskSync()
	}
	return nil
}

// Dequeue retrieves the next notification from the queue
func (lq *LocalQueue) Dequeue(ctx context.Context) (*domain.QueueMessage, error) {
	if lq.closed {
		return nil, fmt.Errorf("queue is closed")
	}

	select {
	case msg := <-lq.queue:
		lq.mu.Lock()
		msg.Attempt++
		msg.Notification.Status = domain.StatusProcessing
		lq.mu.Unlock()
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lq.closeChan:
		return nil, fmt.Errorf("queue is closed")
	}
}

// Ack acknowledges successful processing of a message
func (lq *LocalQueue) Ack(ctx context.Context, messageID string) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if msg, exists := lq.messages[messageID]; exists {
		msg.Notification.Status = domain.StatusSent
		delete(lq.messages, messageID)

		if lq.persistToDisk {
			return lq.persistToDiskSync()
		}
	}

	return nil
}

// Nack indicates processing failure and may requeue the message
func (lq *LocalQueue) Nack(ctx context.Context, messageID string, requeue bool) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	msg, exists := lq.messages[messageID]
	if !exists {
		return fmt.Errorf("message not found: %s", messageID)
	}

	if requeue {
		msg.Notification.Status = domain.StatusRetrying
		select {
		case lq.queue <- msg:
			if lq.persistToDisk {
				return lq.persistToDiskSync()
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-lq.closeChan:
			return fmt.Errorf("queue is closed")
		}
	} else {
		msg.Notification.Status = domain.StatusFailed
		delete(lq.messages, messageID)

		if lq.persistToDisk {
			return lq.persistToDiskSync()
		}
	}

	return nil
}

// Size returns the current number of messages in the queue
func (lq *LocalQueue) Size(ctx context.Context) (int64, error) {
	lq.mu.RLock()
	defer lq.mu.RUnlock()
	return int64(len(lq.queue)), nil
}

// Purge removes all messages from the queue
func (lq *LocalQueue) Purge(ctx context.Context) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	// Drain the channel
	for len(lq.queue) > 0 {
		<-lq.queue
	}

	lq.messages = make(map[string]*domain.QueueMessage)

	if lq.persistToDisk {
		return lq.persistToDiskSync()
	}

	return nil
}

// Close cleanly shuts down the queue
func (lq *LocalQueue) Close() error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return nil
	}

	lq.closed = true
	close(lq.closeChan)

	if lq.persistToDisk {
		if err := lq.persistToDiskSync(); err != nil {
			return err
		}
	}

	close(lq.queue)
	return nil
}

// HealthCheck verifies the queue is operational
func (lq *LocalQueue) HealthCheck(ctx context.Context) error {
	lq.mu.RLock()
	defer lq.mu.RUnlock()

	if lq.closed {
		return fmt.Errorf("queue is closed")
	}

	return nil
}

// persistToDiskSync persists the queue state to disk (must be called with lock held)
func (lq *LocalQueue) persistToDiskSync() error {
	if !lq.persistToDisk || lq.persistPath == "" {
		return nil
	}

	data, err := json.Marshal(lq.messages)
	if err != nil {
		return fmt.Errorf("failed to marshal queue state: %w", err)
	}

	if err := os.WriteFile(lq.persistPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue state: %w", err)
	}

	return nil
}

// loadFromDisk loads the queue state from disk
func (lq *LocalQueue) loadFromDisk() error {
	if lq.persistPath == "" {
		return nil
	}

	data, err := os.ReadFile(lq.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No persisted state yet
		}
		return fmt.Errorf("failed to read queue state: %w", err)
	}

	var messages map[string]*domain.QueueMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("failed to unmarshal queue state: %w", err)
	}

	// Re-enqueue persisted messages
	for _, msg := range messages {
		lq.queue <- msg
		lq.messages[msg.ID] = msg
	}

	return nil
}
