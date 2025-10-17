package domain

import (
	"context"
)

// QueueMessage wraps a notification with queue-specific metadata
type QueueMessage struct {
	// ID is a unique identifier for this queue message
	ID string `json:"id"`

	// Notification is the actual notification to be sent
	Notification *Notification `json:"notification"`

	// Attempt is the current delivery attempt number
	Attempt int `json:"attempt"`

	// EnqueuedAt is when the message was added to the queue
	EnqueuedAt int64 `json:"enqueued_at"`
}

// Queue defines the interface for a notification queue
type Queue interface {
	// Enqueue adds a notification to the queue
	Enqueue(ctx context.Context, notification *Notification) error

	// EnqueueBatch adds multiple notifications to the queue
	EnqueueBatch(ctx context.Context, notifications []*Notification) error

	// Dequeue retrieves the next notification from the queue
	// Returns nil if the queue is empty
	Dequeue(ctx context.Context) (*QueueMessage, error)

	// Ack acknowledges successful processing of a message
	Ack(ctx context.Context, messageID string) error

	// Nack indicates processing failure and may requeue the message
	Nack(ctx context.Context, messageID string, requeue bool) error

	// Size returns the current number of messages in the queue
	Size(ctx context.Context) (int64, error)

	// Purge removes all messages from the queue
	Purge(ctx context.Context) error

	// Close cleanly shuts down the queue
	Close() error

	// HealthCheck verifies the queue is operational
	HealthCheck(ctx context.Context) error
}

// QueueConfig contains configuration for queue implementations
type QueueConfig struct {
	// Type specifies the queue implementation (local, kafka, etc.)
	Type string `mapstructure:"type"`

	// MaxSize is the maximum number of messages the queue can hold
	MaxSize int64 `mapstructure:"max_size"`

	// WorkerCount is the number of concurrent workers processing the queue
	WorkerCount int `mapstructure:"worker_count"`

	// RetryAttempts is the number of times to retry failed notifications
	RetryAttempts int `mapstructure:"retry_attempts"`

	// RetryBackoff is the backoff strategy for retries (exponential, linear, fixed)
	RetryBackoff string `mapstructure:"retry_backoff"`

	// Local queue specific config
	Local *LocalQueueConfig `mapstructure:"local,omitempty"`

	// Kafka specific config
	Kafka *KafkaQueueConfig `mapstructure:"kafka,omitempty"`
}

// LocalQueueConfig contains configuration for the in-memory queue
type LocalQueueConfig struct {
	// BufferSize is the channel buffer size
	BufferSize int `mapstructure:"buffer_size"`

	// PersistToDisk enables writing queue state to disk for recovery
	PersistToDisk bool `mapstructure:"persist_to_disk"`

	// PersistPath is where to store the queue state
	PersistPath string `mapstructure:"persist_path"`
}

// KafkaQueueConfig contains configuration for Kafka queue
type KafkaQueueConfig struct {
	// Brokers is the list of Kafka broker addresses
	Brokers []string `mapstructure:"brokers"`

	// Topic is the Kafka topic for notifications
	Topic string `mapstructure:"topic"`

	// ConsumerGroup is the Kafka consumer group ID
	ConsumerGroup string `mapstructure:"consumer_group"`

	// PartitionCount is the number of partitions for the topic
	PartitionCount int `mapstructure:"partition_count"`

	// ReplicationFactor is the replication factor for the topic
	ReplicationFactor int `mapstructure:"replication_factor"`

	// EnableIdempotence ensures exactly-once delivery semantics
	EnableIdempotence bool `mapstructure:"enable_idempotence"`

	// CompressionType defines compression (none, gzip, snappy, lz4, zstd)
	CompressionType string `mapstructure:"compression_type"`
}
