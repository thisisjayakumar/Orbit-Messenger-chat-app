package biz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Mock OutboxRepo
// ──────────────────────────────────────────────

type mockOutboxRepo struct {
	fetchPendingFunc     func(ctx context.Context, batchSize int) ([]*OutboxRecord, error)
	markPublishedFunc    func(ctx context.Context, id int64) error
	markFailedFunc       func(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error
	moveToDeadLetterFunc func(ctx context.Context, record *OutboxRecord, lastError string) error
}

func (m *mockOutboxRepo) FetchPending(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
	if m.fetchPendingFunc != nil {
		return m.fetchPendingFunc(ctx, batchSize)
	}
	return nil, nil
}
func (m *mockOutboxRepo) MarkPublished(ctx context.Context, id int64) error {
	if m.markPublishedFunc != nil {
		return m.markPublishedFunc(ctx, id)
	}
	return nil
}
func (m *mockOutboxRepo) MarkFailed(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error {
	if m.markFailedFunc != nil {
		return m.markFailedFunc(ctx, id, lastError, nextRetryAt)
	}
	return nil
}
func (m *mockOutboxRepo) MoveToDeadLetter(ctx context.Context, record *OutboxRecord, lastError string) error {
	if m.moveToDeadLetterFunc != nil {
		return m.moveToDeadLetterFunc(ctx, record, lastError)
	}
	return nil
}

// ──────────────────────────────────────────────
// Mock MQTTPublisher
// ──────────────────────────────────────────────

type mockMQTT struct {
	publishFunc func(topic string, qos byte, payload []byte) error
}

func (m *mockMQTT) Publish(topic string, qos byte, payload []byte) error {
	if m.publishFunc != nil {
		return m.publishFunc(topic, qos, payload)
	}
	return nil
}

// ──────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────

func pendingRecord(id int64) *OutboxRecord {
	return &OutboxRecord{
		ID:             id,
		MessageID:      uuid.New(),
		ConversationID: uuid.New(),
		Topic:          "chat/" + uuid.New().String() + "/messages",
		Payload:        []byte(`{"content":"hello"}`),
		Status:         OutboxStatusPending,
		RetryCount:     0,
		MaxRetries:     5,
		CreatedAt:      time.Now().Add(-5 * time.Second),
	}
}

func newRelay(repo *mockOutboxRepo, mqtt *mockMQTT, config RelayConfig) *RelayService {
	return NewRelayService(repo, mqtt, config)
}

// ──────────────────────────────────────────────
// Tests: processBatch
// ──────────────────────────────────────────────

func TestProcessBatch_NoRecords_DoesNothing(t *testing.T) {
	repo := &mockOutboxRepo{}
	mqtt := &mockMQTT{}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	// Should not panic or call anything
	relay.processBatch(context.Background())
}

func TestProcessBatch_SuccessfulPublish_MarksPublished(t *testing.T) {
	var publishedID int64
	repo := &mockOutboxRepo{
		fetchPendingFunc: func(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
			return []*OutboxRecord{pendingRecord(1)}, nil
		},
		markPublishedFunc: func(ctx context.Context, id int64) error {
			publishedID = id
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return nil
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	relay.processBatch(context.Background())

	if publishedID != 1 {
		t.Errorf("markPublished was called with id=%d, want 1", publishedID)
	}
}

func TestProcessBatch_PublishFailure_Retries(t *testing.T) {
	now := time.Now()
	var failedID int64
	var capturedError string
	var capturedRetryAt time.Time

	repo := &mockOutboxRepo{
		fetchPendingFunc: func(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
			return []*OutboxRecord{pendingRecord(1)}, nil
		},
		markFailedFunc: func(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error {
			failedID = id
			capturedError = lastError
			capturedRetryAt = nextRetryAt
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return errors.New("mqtt broker unreachable")
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	relay.processBatch(context.Background())

	if failedID != 1 {
		t.Errorf("markFailed was called with id=%d, want 1", failedID)
	}
	if capturedError != "mqtt broker unreachable" {
		t.Errorf("capturedError = %q, want %q", capturedError, "mqtt broker unreachable")
	}
	if capturedRetryAt.Before(now) {
		t.Error("nextRetryAt should be in the future")
	}
}

func TestProcessBatch_PublishFailure_ExhaustedRetries_MovesToDeadLetter(t *testing.T) {
	var movedRecord *OutboxRecord
	var movedErr string

	record := pendingRecord(1)
	record.RetryCount = 4 // One more attempt = 5, which equals max_retries
	record.MaxRetries = 5

	repo := &mockOutboxRepo{
		fetchPendingFunc: func(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
			return []*OutboxRecord{record}, nil
		},
		moveToDeadLetterFunc: func(ctx context.Context, r *OutboxRecord, lastError string) error {
			movedRecord = r
			movedErr = lastError
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return errors.New("emqx connection reset")
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	relay.processBatch(context.Background())

	if movedRecord == nil {
		t.Fatal("MoveToDeadLetter was not called")
	}
	if movedRecord.ID != 1 {
		t.Errorf("movedRecord.ID = %d, want 1", movedRecord.ID)
	}
	if movedErr != "emqx connection reset" {
		t.Errorf("movedErr = %q, want %q", movedErr, "emqx connection reset")
	}
}

func TestProcessBatch_MultipleRecords_ProcessesAll(t *testing.T) {
	var publishedCount atomic.Int64

	repo := &mockOutboxRepo{
		fetchPendingFunc: func(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
			return []*OutboxRecord{pendingRecord(1), pendingRecord(2), pendingRecord(3)}, nil
		},
		markPublishedFunc: func(ctx context.Context, id int64) error {
			publishedCount.Add(1)
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return nil
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	relay.processBatch(context.Background())

	if publishedCount.Load() != 3 {
		t.Errorf("published count = %d, want 3", publishedCount.Load())
	}
}

func TestProcessBatch_FetchError_LogsAndContinues(t *testing.T) {
	repo := &mockOutboxRepo{
		fetchPendingFunc: func(ctx context.Context, batchSize int) ([]*OutboxRecord, error) {
			return nil, errors.New("connection timeout")
		},
	}
	mqtt := &mockMQTT{}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	// Should not panic
	relay.processBatch(context.Background())
}

// ──────────────────────────────────────────────
// Tests: calculateNextRetry
// ──────────────────────────────────────────────

func TestCalculateNextRetry_ExponentialBackoff(t *testing.T) {
	config := DefaultRelayConfig()
	config.RetryBackoff = 2 * time.Second
	config.MaxBackoff = 30 * time.Second
	relay := NewRelayService(&mockOutboxRepo{}, &mockMQTT{}, config)

	// Attempt 1: ~2s (2^0 * 2s) with ±25% jitter
	t1 := relay.calculateNextRetry(1)
	expected1 := 2 * time.Second
	if t1.Before(time.Now()) {
		t.Error("nextRetryAt for attempt 1 should be in the future")
	}
	if t1.After(time.Now().Add(expected1 * 2)) {
		t.Errorf("nextRetryAt for attempt 1 too far in the future: %v", t1.Sub(time.Now()))
	}

	// Attempt 2: ~4s (2^1 * 2s) with ±25% jitter
	t2 := relay.calculateNextRetry(2)
	if t2.Before(time.Now()) {
		t.Error("nextRetryAt for attempt 2 should be in the future")
	}
	if t2.After(time.Now().Add(6 * time.Second)) {
		t.Errorf("nextRetryAt for attempt 2 too far in the future: %v", t2.Sub(time.Now()))
	}
}

func TestCalculateNextRetry_CappedAtMaxBackoff(t *testing.T) {
	config := DefaultRelayConfig()
	config.RetryBackoff = 2 * time.Second
	config.MaxBackoff = 10 * time.Second
	relay := NewRelayService(&mockOutboxRepo{}, &mockMQTT{}, config)

	// Attempt 10: 2s * 2^9 = 1024s, capped at 10s with jitter
	t10 := relay.calculateNextRetry(10)
	maxExpected := 10 * time.Second
	if t10.After(time.Now().Add(maxExpected * 2)) {
		t.Errorf("attempt 10 backoff not capped: %v", t10.Sub(time.Now()))
	}
}

// ──────────────────────────────────────────────
// Tests: NewRelayService
// ──────────────────────────────────────────────

func TestNewRelayService_DefaultsApplied(t *testing.T) {
	// Zero config should get sensible defaults
	relay := NewRelayService(&mockOutboxRepo{}, &mockMQTT{}, RelayConfig{})
	if relay.config.BatchSize != 100 {
		t.Errorf("default BatchSize = %d, want 100", relay.config.BatchSize)
	}
	if relay.config.MaxRetries != 5 {
		t.Errorf("default MaxRetries = %d, want 5", relay.config.MaxRetries)
	}
	if relay.config.PollInterval != 500*time.Millisecond {
		t.Errorf("default PollInterval = %v, want 500ms", relay.config.PollInterval)
	}
	if relay.config.RetryBackoff != 2*time.Second {
		t.Errorf("default RetryBackoff = %v, want 2s", relay.config.RetryBackoff)
	}
}

// ──────────────────────────────────────────────
// Tests: Run with context cancellation
// ──────────────────────────────────────────────

func TestRun_ContextCancellation_Stops(t *testing.T) {
	repo := &mockOutboxRepo{}
	mqtt := &mockMQTT{}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- relay.Run(ctx)
	}()

	// Short sleep to let the relay start ticking
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

// ──────────────────────────────────────────────
// Tests: processRecord (direct)
// ──────────────────────────────────────────────

func TestProcessRecord_Success(t *testing.T) {
	var published bool
	repo := &mockOutboxRepo{
		markPublishedFunc: func(ctx context.Context, id int64) error {
			published = true
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return nil
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	relay.processRecord(context.Background(), pendingRecord(42))

	if !published {
		t.Error("expected markPublished to be called on success")
	}
}

func TestProcessRecord_FirstFailure_MarksFailed(t *testing.T) {
	var failed bool
	repo := &mockOutboxRepo{
		markFailedFunc: func(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error {
			failed = true
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return errors.New("timeout")
		},
	}
	relay := newRelay(repo, mqtt, DefaultRelayConfig())

	record := pendingRecord(1)
	record.RetryCount = 0
	relay.processRecord(context.Background(), record)

	if !failed {
		t.Error("expected markFailed to be called")
	}
}

func TestProcessRecord_FinalFailure_MovesToDeadLetter(t *testing.T) {
	var deadLettered bool
	repo := &mockOutboxRepo{
		moveToDeadLetterFunc: func(ctx context.Context, record *OutboxRecord, lastError string) error {
			deadLettered = true
			return nil
		},
	}
	mqtt := &mockMQTT{
		publishFunc: func(topic string, qos byte, payload []byte) error {
			return errors.New("max retries exceeded")
		},
	}
	config := DefaultRelayConfig()
	config.MaxRetries = 1 // Only 1 allowed attempt
	relay := newRelay(repo, mqtt, config)

	record := pendingRecord(1)
	record.RetryCount = 1 // This is the 2nd attempt (1 + 1 = 2 >= max_retries=1)
	relay.processRecord(context.Background(), record)

	if !deadLettered {
		t.Error("expected MoveToDeadLetter to be called after max retries")
	}
}
