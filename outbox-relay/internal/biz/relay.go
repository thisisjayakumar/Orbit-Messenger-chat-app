// Package biz contains the business logic for the Outbox Relay service.
// It polls the message_outbox table, publishes to MQTT, and handles
// retries with exponential backoff and dead-letter escalation.
package biz

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// OutboxStatus represents the status of an outbox entry.
type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusPublished OutboxStatus = "published"
	OutboxStatusFailed    OutboxStatus = "failed"
)

// OutboxRecord represents a row from the message_outbox table.
type OutboxRecord struct {
	ID             int64           `json:"id"`
	MessageID      uuid.UUID       `json:"message_id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	Topic          string          `json:"topic"`
	Payload        json.RawMessage `json:"payload"`
	Status         OutboxStatus    `json:"status"`
	RetryCount     int             `json:"retry_count"`
	MaxRetries     int             `json:"max_retries"`
	LastError      string          `json:"last_error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty"`
}

// OutboxRepo defines the data operations the relay needs.
type OutboxRepo interface {
	// FetchPending returns pending and retryable outbox rows,
	// locked for update via SELECT ... FOR UPDATE SKIP LOCKED.
	FetchPending(ctx context.Context, batchSize int) ([]*OutboxRecord, error)

	// MarkPublished sets status to 'published' and records published_at.
	MarkPublished(ctx context.Context, id int64) error

	// MarkFailed sets status to 'failed', records the error, and sets next_retry_at.
	MarkFailed(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error

	// MoveToDeadLetter copies the record to the dead-letter table and marks it failed.
	MoveToDeadLetter(ctx context.Context, record *OutboxRecord, lastError string) error
}

// MQTTPublisher defines the interface for publishing messages to MQTT.
type MQTTPublisher interface {
	// Publish sends a payload to the given topic with the specified QoS.
	Publish(topic string, qos byte, payload []byte) error
}

// RelayConfig holds configuration for the outbox relay service.
type RelayConfig struct {
	PollInterval time.Duration // How often to poll for pending messages (default: 500ms)
	BatchSize    int           // Max rows per poll cycle (default: 100)
	MaxRetries   int           // Max publish attempts before dead letter (default: 5)
	RetryBackoff time.Duration // Initial backoff before retry (default: 2s, doubles each attempt)
	MaxBackoff   time.Duration // Cap for backoff (default: 24h)
	PendingAge   time.Duration // Minimum age of pending rows to pick up (default: 1s, avoids racing with writer)
}

// DefaultRelayConfig returns a RelayConfig with sensible defaults
// tuned for < 1K concurrent users (per PO decisions).
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		PollInterval: 500 * time.Millisecond,
		BatchSize:    100,
		MaxRetries:   5,
		RetryBackoff: 2 * time.Second,
		MaxBackoff:   24 * time.Hour,
		PendingAge:   1 * time.Second, // Avoid picking up rows still being written
	}
}

// RelayService polls the outbox table and publishes pending messages to MQTT.
// It handles retries with exponential backoff and escalates to dead-letter
// when max retries are exhausted.
type RelayService struct {
	repo       OutboxRepo
	publisher  MQTTPublisher
	config     RelayConfig
	publishQoS byte // QoS level for MQTT publish (default: 1)
	stopCh     chan struct{}
}

// NewRelayService creates a new outbox relay service.
func NewRelayService(repo OutboxRepo, publisher MQTTPublisher, config RelayConfig) *RelayService {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 5
	}
	if config.RetryBackoff <= 0 {
		config.RetryBackoff = 2 * time.Second
	}
	if config.MaxBackoff <= 0 {
		config.MaxBackoff = 24 * time.Hour
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 500 * time.Millisecond
	}
	if config.PendingAge <= 0 {
		config.PendingAge = 1 * time.Second
	}

	return &RelayService{
		repo:       repo,
		publisher:  publisher,
		config:     config,
		publishQoS: 1,
		stopCh:     make(chan struct{}),
	}
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (s *RelayService) Run(ctx context.Context) error {
	log.Printf("[outbox-relay] Starting relay — poll_interval=%v, batch_size=%d, max_retries=%d",
		s.config.PollInterval, s.config.BatchSize, s.config.MaxRetries)

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[outbox-relay] Shutting down: %v", ctx.Err())
			return ctx.Err()
		case <-s.stopCh:
			log.Println("[outbox-relay] Stop requested")
			return nil
		case <-ticker.C:
			s.processBatch(ctx)
		}
	}
}

// Stop signals the relay to stop after the current batch completes.
func (s *RelayService) Stop() {
	close(s.stopCh)
}

// processBatch fetches pending rows and processes them.
func (s *RelayService) processBatch(ctx context.Context) {
	records, err := s.repo.FetchPending(ctx, s.config.BatchSize)
	if err != nil {
		log.Printf("[outbox-relay] Error fetching pending outbox rows: %v", err)
		return
	}

	if len(records) == 0 {
		return // Nothing to do
	}

	log.Printf("[outbox-relay] Processing %d outbox row(s)", len(records))

	for _, record := range records {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.processRecord(ctx, record)
	}
}

// processRecord handles a single outbox record: publish, then mark published or failed.
func (s *RelayService) processRecord(ctx context.Context, record *OutboxRecord) {
	err := s.publisher.Publish(record.Topic, s.publishQoS, record.Payload)
	if err != nil {
		s.handlePublishFailure(ctx, record, err)
		return
	}

	// Success — mark as published
	if err := s.repo.MarkPublished(ctx, record.ID); err != nil {
		log.Printf("[outbox-relay] Failed to mark outbox %d as published: %v", record.ID, err)
	}
}

// handlePublishFailure handles a failed publish with retry/dead-letter logic.
func (s *RelayService) handlePublishFailure(ctx context.Context, record *OutboxRecord, publishErr error) {
	errMsg := publishErr.Error()
	newRetryCount := record.RetryCount + 1

	log.Printf("[outbox-relay] Publish failed for outbox %d (topic=%s, retry=%d/%d): %v",
		record.ID, record.Topic, newRetryCount, s.config.MaxRetries, publishErr)

	if newRetryCount >= s.config.MaxRetries {
		// Exhausted retries — move to dead letter
		log.Printf("[outbox-relay] Moving outbox %d to dead letter after %d retries", record.ID, newRetryCount)
		if err := s.repo.MoveToDeadLetter(ctx, record, errMsg); err != nil {
			log.Printf("[outbox-relay] Failed to move outbox %d to dead letter: %v", record.ID, err)
		}
		return
	}

	// Calculate exponential backoff with jitter
	nextRetryAt := s.calculateNextRetry(newRetryCount)

	if err := s.repo.MarkFailed(ctx, record.ID, errMsg, nextRetryAt); err != nil {
		log.Printf("[outbox-relay] Failed to mark outbox %d as failed: %v", record.ID, err)
	}
}

// calculateNextRetry computes the next retry time using exponential backoff
// with random jitter (±25%).
//
//	delay = min(backoff * 2^(attempt-1), maxBackoff)
//	jitter = delay * (0.75 + rand.Float64() * 0.5)  (±25%)
//	next_retry_at = now + jitter
func (s *RelayService) calculateNextRetry(attempt int) time.Time {
	backoff := float64(s.config.RetryBackoff) * math.Pow(2, float64(attempt-1))
	backoff = math.Min(backoff, float64(s.config.MaxBackoff))

	// Add jitter: ±25%
	jitter := backoff * (0.75 + rand.Float64()*0.5) //nolint:gosec

	return time.Now().Add(time.Duration(jitter))
}
