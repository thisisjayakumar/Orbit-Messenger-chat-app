// Package data implements the outbox-relay data access layer.
package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/thisisjayakumar/Orbit-Messenger-chat-app/outbox-relay/internal/biz"
)

type outboxRepo struct {
	db *sql.DB
}

// NewOutboxRepo creates a new outbox repository backed by PostgreSQL.
func NewOutboxRepo(db *sql.DB) biz.OutboxRepo {
	return &outboxRepo{db: db}
}

// FetchPending selects pending and retryable outbox rows using
// FOR UPDATE SKIP LOCKED to prevent multiple relay instances
// from processing the same rows concurrently.
//
// It fetches:
//   - status = 'pending' AND created_at < NOW() - pending_age
//   - status = 'failed'  AND retry_count < max_retries AND next_retry_at <= NOW()
//
// Ordered by created_at ASC (oldest first) with the given limit.
func (r *outboxRepo) FetchPending(ctx context.Context, batchSize int) ([]*biz.OutboxRecord, error) {
	query := `
		SELECT id, message_id, conversation_id, topic, payload, status,
		       retry_count, max_retries, COALESCE(last_error, ''), created_at,
		       next_retry_at
		FROM message_outbox
		WHERE (status = 'pending' AND created_at < NOW() - $2::interval)
		   OR (status = 'failed' AND retry_count < max_retries AND next_retry_at <= NOW())
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	// Use a 1-second pending age to avoid racing with the Chat API writer
	rows, err := r.db.QueryContext(ctx, query, batchSize, "1 second")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*biz.OutboxRecord
	for rows.Next() {
		rec := &biz.OutboxRecord{}
		var payloadRaw []byte
		var statusRaw string
		var lastError string

		err := rows.Scan(
			&rec.ID, &rec.MessageID, &rec.ConversationID, &rec.Topic, &payloadRaw,
			&statusRaw, &rec.RetryCount, &rec.MaxRetries, &lastError,
			&rec.CreatedAt, &rec.NextRetryAt,
		)
		if err != nil {
			return nil, err
		}

		rec.Payload = json.RawMessage(payloadRaw)
		rec.Status = biz.OutboxStatus(statusRaw)
		if lastError != "" {
			rec.LastError = lastError
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// MarkPublished sets the outbox entry as published with the current timestamp.
func (r *outboxRepo) MarkPublished(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE message_outbox SET status = 'published', published_at = NOW() WHERE id = $1`, id)
	return err
}

// MarkFailed records a failed publish attempt with the error message and
// the next scheduled retry time. The relay service calculates the backoff.
func (r *outboxRepo) MarkFailed(ctx context.Context, id int64, lastError string, nextRetryAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE message_outbox
		 SET status = 'failed',
		     retry_count = retry_count + 1,
		     last_error = $2,
		     next_retry_at = $3
		 WHERE id = $1`, id, lastError, nextRetryAt)
	return err
}

// MoveToDeadLetter copies the record to the dead-letter table and marks
// the original outbox entry as failed. This is called when max retries
// have been exhausted.
//
// The operation is performed in a single transaction to ensure atomicity.
func (r *outboxRepo) MoveToDeadLetter(ctx context.Context, record *biz.OutboxRecord, lastError string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op if committed

	// Insert into dead-letter table
	_, err = tx.ExecContext(ctx, `
		INSERT INTO message_outbox_dead_letter
			(message_id, conversation_id, topic, payload, retry_count, last_error, created_at, failed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
		record.MessageID, record.ConversationID, record.Topic,
		record.Payload, record.RetryCount+1, lastError, record.CreatedAt)
	if err != nil {
		return err
	}

	// Mark the original outbox as failed with final error
	_, err = tx.ExecContext(ctx,
		`UPDATE message_outbox
		 SET status = 'failed',
		     retry_count = retry_count + 1,
		     last_error = $2,
		     next_retry_at = NULL
		 WHERE id = $1`, record.ID, lastError)
	if err != nil {
		return err
	}

	return tx.Commit()
}
