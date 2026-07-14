//go:build integration

package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/outbox-relay/internal/biz"
)

// ──────────────────────────────────────────────
// Test setup
// ──────────────────────────────────────────────

// outboxTableDDL contains the minimal DDL to create the outbox tables
// needed for integration testing. FK constraints are omitted to avoid
// deep dependency chains (messages → conversations → organizations/users).
const outboxTableDDL = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

DROP TABLE IF EXISTS message_outbox_dead_letter CASCADE;
DROP TABLE IF EXISTS message_outbox CASCADE;
DROP TYPE IF EXISTS outbox_status CASCADE;

CREATE TYPE outbox_status AS ENUM ('pending', 'published', 'failed');

CREATE TABLE message_outbox (
    id              BIGSERIAL PRIMARY KEY,
    message_id      UUID NOT NULL,
    conversation_id UUID NOT NULL,
    topic           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          outbox_status NOT NULL DEFAULT 'pending',
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 5,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ
);

CREATE TABLE message_outbox_dead_letter (
    id              BIGSERIAL PRIMARY KEY,
    message_id      UUID NOT NULL,
    conversation_id UUID NOT NULL,
    topic           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    retry_count     INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    failed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved        BOOLEAN NOT NULL DEFAULT FALSE
);
`

// testOutboxRepo connects to the test database and returns a fresh OutboxRepo
// with the outbox tables created. It returns the repo, a cleanup function,
// and the database handle for direct SQL seeding.
func testOutboxRepo(t *testing.T) (biz.OutboxRepo, func(), *sql.DB) {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("PostgreSQL not available at %s: %v — skipping integration test", dsn, err)
	}

	// Create outbox tables
	if _, err := db.Exec(outboxTableDDL); err != nil {
		db.Close()
		t.Fatalf("Failed to create test tables: %v", err)
	}

	repo := NewOutboxRepo(db)
	cleanup := func() {
		db.Exec("DROP TABLE IF EXISTS message_outbox_dead_letter CASCADE")
		db.Exec("DROP TABLE IF EXISTS message_outbox CASCADE")
		db.Exec("DROP TYPE IF EXISTS outbox_status CASCADE")
		db.Close()
	}

	return repo, cleanup, db
}

// testOutboxRecord creates a pending outbox record with the given overrides.
// Returns the raw row data for assertions.
func insertPendingRecord(t *testing.T, db *sql.DB, overrides map[string]interface{}) int64 {
	t.Helper()

	convID := uuid.New()
	msgID := uuid.New()
	payload, _ := json.Marshal(map[string]string{"content": "hello"})
	createdAt := time.Now().Add(-5 * time.Second) // Older than 1s pending age

	query := `
		INSERT INTO message_outbox (message_id, conversation_id, topic, payload, status, retry_count, max_retries, created_at)
		VALUES ($1, $2, $3, $4, 'pending', 0, 5, $5)
		RETURNING id`

	if v, ok := overrides["message_id"]; ok {
		msgID = v.(uuid.UUID)
	}
	if v, ok := overrides["conversation_id"]; ok {
		convID = v.(uuid.UUID)
	}
	if v, ok := overrides["created_at"]; ok {
		createdAt = v.(time.Time)
	}
	if v, ok := overrides["payload"]; ok {
		payload, _ = json.Marshal(v)
	}

	var id int64
	err := db.QueryRow(query, msgID, convID, "chat/"+convID.String()+"/messages", payload, createdAt).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test record: %v", err)
	}
	return id
}

// ──────────────────────────────────────────────
// Tests: FetchPending
// ──────────────────────────────────────────────

func TestFetchPending_ReturnsPendingRows(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}
	if len(records) == 0 {
		t.Fatal("FetchPending() returned 0 records, expected at least 1")
	}

	var found bool
	for _, r := range records {
		if r.ID == id {
			found = true
			if r.Status != biz.OutboxStatusPending {
				t.Errorf("record status = %q, want %q", r.Status, biz.OutboxStatusPending)
			}
			if r.Topic == "" {
				t.Error("record topic is empty")
			}
			if len(r.Payload) == 0 {
				t.Error("record payload is empty")
			}
			if r.MessageID == uuid.Nil {
				t.Error("record message_id is nil")
			}
			if r.ConversationID == uuid.Nil {
				t.Error("record conversation_id is nil")
			}
			break
		}
	}
	if !found {
		t.Errorf("FetchPending() did not return record with id=%d", id)
	}
}

func TestFetchPending_IgnoresRecentRows(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Insert a record created just now (should be excluded by the 1s pending age)
	recentID := insertPendingRecord(t, db, map[string]interface{}{
		"created_at": time.Now(),
	})

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}

	for _, r := range records {
		if r.ID == recentID {
			t.Errorf("FetchPending() returned recently created record %d — should be excluded by pending age", recentID)
		}
	}
}

func TestFetchPending_IgnoresPublishedRows(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)

	// Mark it as published
	_, err := db.Exec(`UPDATE message_outbox SET status = 'published', published_at = NOW() WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("Failed to mark record as published: %v", err)
	}

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}

	for _, r := range records {
		if r.ID == id {
			t.Errorf("FetchPending() returned published record %d", id)
		}
	}
}

func TestFetchPending_PicksUpRetryableFailedRows(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Insert a failed record that's ready for retry (retry_count < max_retries, next_retry_at <= NOW())
	convID := uuid.New()
	msgID := uuid.New()
	payload, _ := json.Marshal(map[string]string{"content": "retry-me"})
	createdAt := time.Now().Add(-10 * time.Second)
	nextRetryAt := time.Now().Add(-5 * time.Second) // In the past — ready for retry

	var id int64
	err := db.QueryRow(`
		INSERT INTO message_outbox (message_id, conversation_id, topic, payload, status, retry_count, max_retries, created_at, next_retry_at)
		VALUES ($1, $2, $3, $4, 'failed', 1, 5, $5, $6)
		RETURNING id`,
		msgID, convID, "chat/"+convID.String()+"/messages", payload, createdAt, nextRetryAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert retryable failed record: %v", err)
	}

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}

	var found bool
	for _, r := range records {
		if r.ID == id {
			found = true
			if r.Status != biz.OutboxStatusFailed {
				t.Errorf("record status = %q, want %q", r.Status, biz.OutboxStatusFailed)
			}
			if r.RetryCount != 1 {
				t.Errorf("record retry_count = %d, want 1", r.RetryCount)
			}
			break
		}
	}
	if !found {
		t.Errorf("FetchPending() did not return retryable failed record %d", id)
	}
}

func TestFetchPending_IgnoresExhaustedFailedRows(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Insert a failed record that has exhausted retries (retry_count >= max_retries)
	convID := uuid.New()
	msgID := uuid.New()
	payload, _ := json.Marshal(map[string]string{"content": "dead"})
	createdAt := time.Now().Add(-10 * time.Second)
	nextRetryAt := time.Now().Add(-5 * time.Second)

	var id int64
	err := db.QueryRow(`
		INSERT INTO message_outbox (message_id, conversation_id, topic, payload, status, retry_count, max_retries, created_at, next_retry_at)
		VALUES ($1, $2, $3, $4, 'failed', 5, 5, $5, $6)
		RETURNING id`,
		msgID, convID, "chat/"+convID.String()+"/messages", payload, createdAt, nextRetryAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert exhausted failed record: %v", err)
	}

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}

	for _, r := range records {
		if r.ID == id {
			t.Errorf("FetchPending() returned exhausted failed record %d (retry_count >= max_retries)", id)
		}
	}
}

func TestFetchPending_LimitRespected(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Insert 3 records
	for i := 0; i < 3; i++ {
		insertPendingRecord(t, db, nil)
	}

	records, err := repo.FetchPending(context.Background(), 2)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}
	if len(records) > 2 {
		t.Errorf("FetchPending(limit=2) returned %d records, expected at most 2", len(records))
	}
}

func TestFetchPending_OrderedByCreatedAt(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Insert records with different creation times
	oldTime := time.Now().Add(-10 * time.Second)
	middleTime := time.Now().Add(-7 * time.Second)
	newTime := time.Now().Add(-3 * time.Second)

	insertPendingRecord(t, db, map[string]interface{}{"created_at": newTime})
	insertPendingRecord(t, db, map[string]interface{}{"created_at": oldTime})
	insertPendingRecord(t, db, map[string]interface{}{"created_at": middleTime})

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() error = %v", err)
	}

	if len(records) >= 3 {
		if records[0].CreatedAt.After(records[1].CreatedAt) ||
			records[1].CreatedAt.After(records[2].CreatedAt) {
			t.Error("FetchPending() records are not ordered by created_at ASC")
		}
	}
}

func TestFetchPending_EmptyTable_ReturnsNoError(t *testing.T) {
	repo, cleanup, _ := testOutboxRepo(t)
	defer cleanup()

	records, err := repo.FetchPending(context.Background(), 100)
	if err != nil {
		t.Fatalf("FetchPending() on empty table error = %v", err)
	}
	if len(records) != 0 {
		t.Errorf("FetchPending() on empty table returned %d records, expected 0", len(records))
	}
}

// ──────────────────────────────────────────────
// Tests: MarkPublished
// ──────────────────────────────────────────────

func TestMarkPublished_SetsStatusAndTimestamp(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)

	if err := repo.MarkPublished(context.Background(), id); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	// Verify the row was updated
	var status string
	var publishedAt *time.Time
	err := db.QueryRow(`SELECT status, published_at FROM message_outbox WHERE id = $1`, id).Scan(&status, &publishedAt)
	if err != nil {
		t.Fatalf("Failed to query updated record: %v", err)
	}
	if status != "published" {
		t.Errorf("status = %q, want %q", status, "published")
	}
	if publishedAt == nil {
		t.Error("published_at is nil, should be set")
	}
}

func TestMarkPublished_NonExistentID_ReturnsNoError(t *testing.T) {
	repo, cleanup, _ := testOutboxRepo(t)
	defer cleanup()

	// Should not error on non-existent ID — UPDATE with no matching rows is not an error
	err := repo.MarkPublished(context.Background(), 99999)
	if err != nil {
		t.Errorf("MarkPublished() on non-existent id error = %v, want nil", err)
	}
}

// ──────────────────────────────────────────────
// Tests: MarkFailed
// ──────────────────────────────────────────────

func TestMarkFailed_SetsStatusAndRetryFields(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)
	nextRetryAt := time.Now().Add(5 * time.Second)
	errMsg := "mqtt connection timeout"

	if err := repo.MarkFailed(context.Background(), id, errMsg, nextRetryAt); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	// Verify the row was updated
	var status string
	var retryCount int
	var lastError string
	var actualNextRetryAt *time.Time
	err := db.QueryRow(
		`SELECT status, retry_count, COALESCE(last_error, ''), next_retry_at FROM message_outbox WHERE id = $1`, id,
	).Scan(&status, &retryCount, &lastError, &actualNextRetryAt)
	if err != nil {
		t.Fatalf("Failed to query updated record: %v", err)
	}
	if status != "failed" {
		t.Errorf("status = %q, want %q", status, "failed")
	}
	if retryCount != 1 {
		t.Errorf("retry_count = %d, want 1", retryCount)
	}
	if lastError != errMsg {
		t.Errorf("last_error = %q, want %q", lastError, errMsg)
	}
	if actualNextRetryAt == nil {
		t.Fatal("next_retry_at is nil, should be set")
	}
	if actualNextRetryAt.Sub(nextRetryAt) > time.Second {
		t.Errorf("next_retry_at diff too large: %v", actualNextRetryAt.Sub(nextRetryAt))
	}
}

func TestMarkFailed_IncrementsRetryCount(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	// Start with retry_count = 2
	convID := uuid.New()
	msgID := uuid.New()
	payload, _ := json.Marshal(map[string]string{"content": "test"})
	var id int64
	err := db.QueryRow(`
		INSERT INTO message_outbox (message_id, conversation_id, topic, payload, status, retry_count, max_retries, created_at)
		VALUES ($1, $2, $3, $4, 'pending', 2, 5, NOW() - interval '10 seconds')
		RETURNING id`,
		msgID, convID, "chat/"+convID.String()+"/messages", payload,
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test record: %v", err)
	}

	if err := repo.MarkFailed(context.Background(), id, "error", time.Now().Add(time.Second)); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	var retryCount int
	db.QueryRow(`SELECT retry_count FROM message_outbox WHERE id = $1`, id).Scan(&retryCount)
	if retryCount != 3 {
		t.Errorf("retry_count = %d, want 3 (was 2, should increment by 1)", retryCount)
	}
}

// ──────────────────────────────────────────────
// Tests: MoveToDeadLetter
// ──────────────────────────────────────────────

func TestMoveToDeadLetter_AtomicallyMovesToDeadLetterAndUpdatesOutbox(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)

	// Fetch the record first
	record := &biz.OutboxRecord{
		ID:             id,
		MessageID:      uuid.New(),
		ConversationID: uuid.New(),
		Topic:          "chat/test/messages",
		Payload:        []byte(`{"content":"dead-letter-test"}`),
		RetryCount:     4,
		CreatedAt:      time.Now().Add(-10 * time.Second),
	}

	errMsg := "max retries exceeded"
	if err := repo.MoveToDeadLetter(context.Background(), record, errMsg); err != nil {
		t.Fatalf("MoveToDeadLetter() error = %v", err)
	}

	// Verify the outbox entry is now marked as failed with updated retry_count
	var status string
	var retryCount int
	var lastError string
	err := db.QueryRow(
		`SELECT status, retry_count, COALESCE(last_error, '') FROM message_outbox WHERE id = $1`, id,
	).Scan(&status, &retryCount, &lastError)
	if err != nil {
		t.Fatalf("Failed to query updated outbox record: %v", err)
	}
	if status != "failed" {
		t.Errorf("outbox status = %q, want %q", status, "failed")
	}
	if retryCount != 5 {
		t.Errorf("outbox retry_count = %d, want 5 (was 4, incremented by 1)", retryCount)
	}
	if lastError != errMsg {
		t.Errorf("outbox last_error = %q, want %q", lastError, errMsg)
	}

	// Verify the dead letter entry was created
	var dlID int64
	var dlMessageID uuid.UUID
	var dlRetryCount int
	var dlLastError string
	var dlResolved bool
	err = db.QueryRow(`
		SELECT id, message_id, retry_count, COALESCE(last_error, ''), resolved
		FROM message_outbox_dead_letter
		ORDER BY id DESC LIMIT 1`,
	).Scan(&dlID, &dlMessageID, &dlRetryCount, &dlLastError, &dlResolved)
	if err != nil {
		t.Fatalf("Failed to query dead letter entry: %v", err)
	}
	if dlID <= 0 {
		t.Error("dead letter entry has invalid id")
	}
	if dlMessageID != record.MessageID {
		t.Errorf("dead letter message_id = %v, want %v", dlMessageID, record.MessageID)
	}
	if dlRetryCount != 5 {
		t.Errorf("dead letter retry_count = %d, want 5", dlRetryCount)
	}
	if dlLastError != errMsg {
		t.Errorf("dead letter last_error = %q, want %q", dlLastError, errMsg)
	}
	if dlResolved {
		t.Error("dead letter resolved should be false initially")
	}
}

func TestMoveToDeadLetter_Atomicity_NoPartialState(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	id := insertPendingRecord(t, db, nil)

	// Use a nil record — this should cause MoveToDeadLetter to fail at the
	// dead letter insert step (NULL message_id in UUID NOT NULL column),
	// and the transaction should roll back, leaving the outbox unchanged.
	// But actually a nil record will panic at record.MessageID before any SQL runs.
	// Let's test with a record that has nil UUID values instead.
	record := &biz.OutboxRecord{
		ID: id,
		// MessageID is uuid.Nil — this will fail the NOT NULL constraint
	}

	err := repo.MoveToDeadLetter(context.Background(), record, "test error")
	if err == nil {
		t.Fatal("MoveToDeadLetter() expected error for nil UUID, got nil")
	}

	// Verify the original outbox record was NOT touched (transaction rolled back)
	var status string
	err = db.QueryRow(`SELECT status FROM message_outbox WHERE id = $1`, id).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query outbox record: %v", err)
	}
	if status != "pending" {
		t.Errorf("outbox status after failed MoveToDeadLetter = %q, want %q (should be unchanged)", status, "pending")
	}

	// Verify no dead letter entries were created
	var dlCount int
	db.QueryRow(`SELECT COUNT(*) FROM message_outbox_dead_letter`).Scan(&dlCount)
	if dlCount > 0 {
		t.Errorf("dead letter entries = %d, want 0 (should be empty after rollback)", dlCount)
	}
}

// ──────────────────────────────────────────────
// Tests: NewOutboxRepo
// ──────────────────────────────────────────────

func TestNewOutboxRepo_ReturnsNonNil(t *testing.T) {
	repo, cleanup, db := testOutboxRepo(t)
	defer cleanup()

	if repo == nil {
		t.Fatal("NewOutboxRepo() returned nil")
	}
	_ = db
}
