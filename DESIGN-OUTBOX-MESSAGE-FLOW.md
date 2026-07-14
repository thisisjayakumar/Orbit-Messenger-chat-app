# Transactional Outbox + Single-Writer — Design Document

> **Status**: Draft for review
> **Author**: Senior Software Architect
> **Product Owner Decisions Applied**:
> - ✅ Single-writer: Chat API publishes only, Message Service persists
> - ✅ Exactly-once delivery via Transactional Outbox
> - ✅ Web-first (REST primary, MQTT WebSocket for real-time)
> - ✅ Small scale (< 1K concurrent users)

---

## 1. Current Problem

### Dual-Write Hazard

```
                       ┌──────────────────────┐
User → POST /messages →│    Chat API    │──────│──→ PostgreSQL (messages table)
                       │  (Current)    │──────│──→ EMQX MQTT (chat/{id}/messages)
                       └──────────────────────┘
                                          │
                                          ▼
                                  ┌──────────────┐
                                  │  Message Svc  │──→ PostgreSQL (DUPLICATE write)
                                  │  (subscriber) │
                                  └──────────────┘
```

**Problems**:
1. **Two services write to `messages` table**: Chat API via REST handler, Message Service via MQTT subscriber. Risk of inconsistency.
2. **No atomicity**: Message persisted to PG, but MQTT publish can fail. Message is saved but never delivered in real-time. No recovery mechanism.
3. **No ordering guarantee**: MQTT publish happens after PG write, but if broker is slow/unavailable, the message is "lost" to real-time clients.
4. **Current error handling**: `SendMessage` logs the MQTT error but returns success to the client. The client thinks the message was delivered to everyone, but it wasn't.

---

## 2. Proposed Architecture

### Single-Writer + Transactional Outbox

```
User → POST /messages
         │
         ▼
┌──────────────────────────────────────────┐
│              Chat API                     │
│                                           │
│  1. Validate (auth, participant check)    │
│  2. BEGIN TX                              │
│  3. INSERT INTO messages                  │
│  4. INSERT INTO message_outbox            │
│  5. COMMIT TX (atomic!)                   │
│  6. Return 201 { message }                │
└──────────────────────────────────────────┘
         │
         │ (Outbox relay reads from DB)
         ▼
┌──────────────────────────────────────────┐
│          Outbox Relay Service             │
│                                           │
│  Background loop:                         │
│    SELECT * FROM message_outbox           │
│    WHERE status = 'pending'               │
│    ORDER BY created_at ASC LIMIT 100      │
│    FOR UPDATE SKIP LOCKED                 │
│                                           │
│    For each row:                          │
│      → Publish to EMQX                    │
│      → UPDATE status = 'published'        │
│      (Retry on fail, max 3 attempts)      │
└──────────────────────────────────────────┘
         │
         │ (MQTT topic: chat/{convId}/messages)
         ▼
┌──────────────────────────────────────────┐
│         Message Service (Consumer)        │
│                                           │
│  Subscribes to chat/+/messages            │
│  For each message received via MQTT:      │
│    - Already in DB (written by Chat API)  │
│    - No-op for persistence                │
│    - Could handle: receipts,              │
│      push notifications, indexing         │
└──────────────────────────────────────────┘
```

### Key Change: Message Service becomes a **consumer only**

**Before**: Chat API writes to PG + publishes MQTT. Message Service subscribes MQTT + writes to PG again (dual-write).

**After**: Chat API writes to PG + outbox in one transaction. Outbox Relay reads outbox → publishes MQTT. Message Service subscribes MQTT but does NOT write to PG. It handles downstream tasks (receipts, push notifications, search indexing).

---

## 3. Database Schema Changes

### New Table: `message_outbox`

```sql
CREATE TYPE outbox_status AS ENUM ('pending', 'published', 'failed');

CREATE TABLE message_outbox (
    id              BIGSERIAL PRIMARY KEY,
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    conversation_id UUID NOT NULL,
    topic           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          outbox_status NOT NULL DEFAULT 'pending',
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    next_retry_at   TIMESTAMPTZ
);

CREATE INDEX outbox_pending_idx ON message_outbox(status, created_at) 
    WHERE status = 'pending';

CREATE INDEX outbox_retry_idx ON message_outbox(status, next_retry_at)
    WHERE status = 'failed' AND retry_count < max_retries;
```

### Additional: Remove Chat API's direct MQTT publish

```sql
-- Keep existing `messages` table as-is. Chat API still writes to it.
-- No schema changes to existing tables. Only ADD the outbox table.
```

### Dead letter table (for messages that exhaust retries)

```sql
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
```

---

## 4. Service Changes

### 4.1 Chat API Changes

**`chat-api/internal/biz/chat.go` — `SendMessage`:**

```go
type ChatRepo interface {
    // Same as before but:
    CreateMessage(ctx, message) error     // Keep
    InsertOutbox(ctx, outbox) error       // NEW — inserts into outbox table
    // Remove: Message has NO outbox record — use InsertOutbox
}

type MQTTPublisher interface {
    // Keep for typing indicators (non-critical, best-effort)
    PublishTypingIndicator(ctx, convID, userID, isTyping) error
    // REMOVE: PublishMessage — no longer direct MQTT publish
}

func (uc *ChatUsecase) SendMessage(ctx, req, senderID) (*Message, error) {
    // 1. Validate participant
    participant, err := uc.repo.GetParticipant(ctx, req.ConversationID, senderID)

    // 2. Build message
    message := &Message{...}

    // 3. Persist message + outbox entry in ONE transaction
    //    The repo layer handles BEGIN/COMMIT around both writes
    if err := uc.repo.CreateMessageWithOutbox(ctx, message, buildOutbox(message)); err != nil {
        return nil, err
    }

    // 4. Return immediately — no MQTT publish here
    return message, nil
}

// buildOutbox creates the outbox entry from a message
func buildOutbox(msg *Message) *OutboxEntry {
    payload, _ := json.Marshal(msg)
    return &OutboxEntry{
        MessageID:      msg.ID,
        ConversationID: msg.ConversationID,
        Topic:          fmt.Sprintf("chat/%s/messages", msg.ConversationID),
        Payload:        payload,
    }
}
```

**`chat-api/internal/data/chat.go` — Transactional method:**

```go
func (r *chatRepo) CreateMessageWithOutbox(ctx context.Context, message *biz.Message, outbox *biz.OutboxEntry) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // no-op if committed

    // 1. Insert message
    _, err = tx.ExecContext(ctx, `INSERT INTO messages (...) VALUES (...);`, message.Fields...)
    if err != nil {
        return err
    }

    // 2. Insert outbox entry
    _, err = tx.ExecContext(ctx, `
        INSERT INTO message_outbox (message_id, conversation_id, topic, payload, created_at)
        VALUES ($1, $2, $3, $4, NOW());`,
        outbox.MessageID, outbox.ConversationID, outbox.Topic, outbox.Payload)
    if err != nil {
        return err
    }

    // 3. Commit — both writes succeed or neither does
    return tx.Commit()
}
```

**`chat-api/internal/biz/chat.go` — New outbox types:**

```go
type OutboxEntry struct {
    MessageID      uuid.UUID
    ConversationID uuid.UUID
    Topic          string
    Payload        json.RawMessage
}

// Add to ChatRepo interface:
type ChatRepo interface {
    // ... existing methods ...
    CreateMessageWithOutbox(ctx context.Context, message *Message, outbox *OutboxEntry) error  // replaces CreateMessage
}
```

### 4.2 New Service: Outbox Relay

**`outbox-relay/`** — new directory at project root:

```
outbox-relay/
  cmd/
    outbox-relay/
      main.go
  internal/
    biz/
      relay.go        → Business logic: poll, publish, retry
      relay_test.go   → Unit tests
    data/
      relay.go        → DB queries for outbox
      config.go       → Config structs
```

**`outbox-relay/internal/biz/relay.go`:**

```go
type OutboxRepo interface {
    FetchPending(ctx context.Context, limit int) ([]*OutboxRecord, error)
    MarkPublished(ctx context.Context, id int64) error
    MarkFailed(ctx context.Context, id int64, err error) error
    InsertDeadLetter(ctx context.Context, record *OutboxRecord, err error) error
}

type MQTTPublisher interface {
    Publish(topic string, payload []byte) error
}

type RelayService struct {
    repo      OutboxRepo
    publisher MQTTPublisher
    pollInterval time.Duration
    batchSize    int
}

func (s *RelayService) Run(ctx context.Context) {
    ticker := time.NewTicker(s.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.processBatch(ctx)
        }
    }
}

func (s *RelayService) processBatch(ctx context.Context) {
    records, err := s.repo.FetchPending(ctx, s.batchSize)
    if err != nil {
        log.Printf("Error fetching pending outbox: %v", err)
        return
    }

    for _, record := range records {
        err := s.publisher.Publish(record.Topic, record.Payload)
        if err != nil {
            record.RetryCount++
            if record.RetryCount >= record.MaxRetries {
                // Move to dead letter
                s.repo.InsertDeadLetter(ctx, record, err)
                s.repo.MarkFailed(ctx, record.ID, err)
                log.Printf("Outbox %d moved to dead letter after %d retries: %v",
                    record.ID, record.RetryCount, err)
            } else {
                s.repo.MarkFailed(ctx, record.ID, err)
                log.Printf("Outbox %d failed (retry %d/%d): %v",
                    record.ID, record.RetryCount, record.MaxRetries, err)
            }
        } else {
            s.repo.MarkPublished(ctx, record.ID)
        }
    }
}
```

**`outbox-relay/internal/data/relay.go`:**

```go
func (r *outboxRepo) FetchPending(ctx context.Context, limit int) ([]*biz.OutboxRecord, error) {
    query := `
        SELECT id, message_id, conversation_id, topic, payload, status,
               retry_count, max_retries, last_error, created_at
        FROM message_outbox
        WHERE (status = 'pending' AND created_at < NOW())
           OR (status = 'failed' AND retry_count < max_retries AND next_retry_at <= NOW())
        ORDER BY created_at ASC
        LIMIT $1
        FOR UPDATE SKIP LOCKED`  // Important: prevents multiple relay instances from picking same rows

    rows, err := r.db.QueryContext(ctx, query, limit)
    // ... scan rows ...
}

func (r *outboxRepo) MarkPublished(ctx context.Context, id int64) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE message_outbox SET status = 'published', published_at = NOW() WHERE id = $1`, id)
    return err
}
```

### 4.3 Message Service Changes

```go
// message-service/internal/server/mqtt.go

func (s *MQTTServer) messageHandler(client mqtt.Client, msg mqtt.Message) {
    ctx := context.Background()

    // Message Service NO LONGER persists messages.
    // Chat API already wrote them atomically via outbox.
    // Message Service handles DOWNSTREAM concerns:

    if strings.Contains(topic, "/messages") {
        // 1. Process receipts (mark as delivered for online users)
        // 2. Push notifications (for offline users)
        // 3. Search indexing
        // 4. Any other consumer-side logic
    }
}
```

---

## 5. Message Flow End-to-End

### Happy Path

```
Step 1: Client → POST /api/v1/conversations/{id}/messages
Step 2: Chat API validates JWT, checks participant
Step 3: Chat API opens PG transaction
Step 4: INSERT INTO messages (id, conv_id, sender, content, ...)
Step 5: INSERT INTO message_outbox (message_id, topic, payload, status='pending')
Step 6: COMMIT — both succeed atomically
Step 7: Chat API returns 201 { message } to client
Step 8: (separate goroutine) Outbox Relay polls message_outbox
Step 9: Outbox Relay SELECTs pending rows FOR UPDATE SKIP LOCKED
Step 10: Outbox Relay publishes to EMQX topic: chat/{convId}/messages
Step 11: Outbox Relay UPDATEs status = 'published'
Step 12: EMQX delivers to all subscribed clients (Message Service, web clients, mobile)
Step 13: Message Service receives via MQTT — handles receipts/push notifications
```

### Failure Scenarios

| Scenario | What Happens | Outcome |
|----------|-------------|---------|
| **PG write fails** | Transaction rolls back. Message NOT saved, no outbox entry. | Client gets 500. Retry safe. |
| **PG write succeeds, outbox insert fails** | Transaction rolls back. Both undone. | Same as above — no partial state. |
| **Outbox relay crashes** | Rows stay `status='pending'`. Relay picks them up on restart. | Delayed but no loss. |
| **EMQX is down** | Relay publish fails. Increments `retry_count`. Retries on next poll. | Eventually consistent. |
| **EMQX down for 3+ relay cycles** | `retry_count >= max_retries`. Row moves to dead letter. | Needs manual recovery or alert. |
| **Duplicate outbox publish (idempotency)** | Message Service or client sees duplicate message ID → dedup via `dedupe_key`. | Safe. |
| **Two relay instances pick same row** | `FOR UPDATE SKIP LOCKED` prevents this. Only one instance processes each row. | Safe. |

---

## 6. Idempotency & Deduplication

### Existing: `dedupe_key` on messages table

The `messages` table already has a partial unique index:
```sql
CREATE UNIQUE INDEX msg_dedupe_uidx ON messages(conversation_id, dedupe_key) 
WHERE dedupe_key IS NOT NULL;
```

The outbox relay uses `FOR UPDATE SKIP LOCKED` — inherently idempotent at the DB level.

For clients: the `dedupe_key` should be generated by the client (e.g., `${userId}:${timestamp}:${random}`). The server uses `ON CONFLICT DO NOTHING` if the same key is sent twice.

---

## 7. Config & Tuning

```yaml
# outbox-relay/configs/config.yaml
outbox:
  poll_interval: 100ms         # How often to poll for pending messages
  batch_size: 100              # Max rows per poll
  max_retries: 3               # Max publish attempts before dead letter
  retry_backoff: 5s            # Initial backoff before retry (doubles each attempt)

mqtt:
  broker_url: "tcp://emqx:1883"
  username: "outbox_relay"
  password: "outbox_relay_password"
  qos: 1                       # At-least-once delivery from MQTT perspective

monitoring:
  metrics_port: 9100           # Prometheus metrics
  health_check: "/health"
```

**Poll interval tuning for < 1K concurrent users**:
- 100ms poll interval → 10 polls/second
- Each poll fetches up to 100 messages
- At 50 msg/sec peak, each poll processes ~5 messages
- Average end-to-end latency: < 200ms (poll + publish)

For comparison: direct MQTT publish (current) takes ~50ms end-to-end. The outbox adds ~150ms — acceptable for a chat app.

---

## 8. Backward Compatibility

### Migration Plan

**Phase A — Add outbox table (no behavior change)**:
1. Run migration to create `message_outbox` table
2. Deploy Chat API with BOTH old and new paths (feature flag)
3. Initial `CreateMessage` still does old dual-write
4. Verifies outbox is being populated correctly

**Phase B — Switch to single-writer**:
1. Flip feature flag: Chat API stops direct MQTT publish
2. Chat API inserts message + outbox in single transaction
3. Outbox Relay goes live: polls outbox, publishes to EMQX
4. Monitor for issues, rollback flag if needed

**Phase C — Message Service becomes consumer**:
1. Deploy Message Service that no longer writes to PG
2. It handles receipts, push notifications, indexing via MQTT
3. Remove old `CreateMessage` from Message Service's `MessageRepo`

### Rollback

If Outbox Relay has issues:
1. Re-enable direct MQTT publish in Chat API (old code path)
2. Outbox Relay can be stopped — messages remain in outbox table
3. No data loss — messages are already in the `messages` table

---

## 9. Implementation Order

```
┌─────┬──────────────────────────────────────────────┬──────────┬──────────────────┐
│ Day │ Task                                         │ Files    │ Depends On       │
├─────┼──────────────────────────────────────────────┼──────────┼──────────────────┤
│  1  │ Add message_outbox table (init.sql migration)│ init.sql │ —                │
│  1  │ Add OutboxEntry types to chat-api/biz/       │ chat.go  │ —                │
│  2  │ Add CreateMessageWithOutbox to chatRepo      │ chat.go  │ Day 1            │
│  2  │ Update SendMessage use case to use outbox     │ chat.go  │ Day 2            │
│  3  │ Remove PublishMessage from MQTTPublisher     │ chat.go  │ Day 2            │
│  3  │ Create outbox-relay service (biz + data)     │ 4 files  │ Day 2            │
│  4  │ Wire outbox-relay main.go + Dockerfile       │ main.go  │ Day 3            │
│  4  │ Add to docker-compose.yml                    │ .yml     │ Day 4            │
│  5  │ Remove Message Service's persistence         │ mqtt.go  │ Day 2            │
│  5  │ Unit tests for all new outbox biz logic      │ _test.go │ Days 1-4         │
│  6  │ Integration test: full flow E2E              │ _test.go │ Day 5            │
│  6  │ Add outbox monitoring (Prometheus metrics)   │ relay.go │ Day 4            │
└─────┴──────────────────────────────────────────────┴──────────┴──────────────────┘
```

**Total: ~6 days for a senior Go engineer.**

---

## 10. Monitoring & Alerting

### Metrics to expose (Prometheus)

| Metric | Type | Description |
|--------|------|-------------|
| `outbox_pending_total` | Gauge | Current count of pending outbox rows |
| `outbox_published_total` | Counter | Total successfully published |
| `outbox_failed_total` | Counter | Total failed publishes |
| `outbox_dead_letter_total` | Counter | Rows moved to dead letter |
| `outbox_latency_seconds` | Histogram | Time from row creation to successful publish |
| `outbox_batch_size` | Gauge | Rows processed in each poll cycle |

### Alerts

- **`outbox_pending_total > 1000`** for 5 minutes → Outbox Relay is falling behind
- **`outbox_dead_letter_total > 0`** → Messages are stuck, manual intervention needed
- **`outbox_latency_seconds > 5`** for 5 minutes → EMQX or Relay is degraded

---

## 11. PO Decisions — Applied ✅

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Outbox Relay: standalone or embedded? | **Standalone service** | Decoupled lifecycle, scales independently, zero latency impact on Chat API |
| 2 | Dead letter recovery? | **Automatic retry** (exponential backoff, capped at 24h) | No human ops needed. Messages eventually delivered. |
| 3 | Max delivery latency? | **< 1 second** | Relaxed poll interval (500ms) saves DB CPU. Chat-like UX preserved. |
| 4 | Typing indicators via outbox? | **No — keep direct QoS 0** | Ephemeral, high-frequency, low-importance. Outbox would add unnecessary overhead. |

### Updated Config (reflecting decisions)

```yaml
# outbox-relay/configs/config.yaml
outbox:
  poll_interval: 500ms         # ✅ Updated: 500ms for < 1s latency target
  batch_size: 100
  max_retries: 5               # ✅ Updated: more retries before dead letter
  retry_backoff: 2s            # Initial backoff, doubles each attempt: 2s, 4s, 8s, 16s, 32s
  max_backoff: 24h             # ✅ New: cap at 24h (PO decision)

mqtt:
  broker_url: "tcp://emqx:1883"
  username: "outbox_relay"
  password: "outbox_relay_password"
  qos: 1

monitoring:
  metrics_port: 9100
  health_check: "/health"
```

Cross-service typing indicator config (no change from current):
```yaml
# Typing indicators remain direct MQTT QoS 0 — no outbox, no persistence
mqtt:
  topics:
    - "chat/+/typing"    # QoS 0, best-effort
```

---

## 12. Architectural Principles Satisfied

| Principle | How This Design Satisfies It |
|-----------|------------------------------|
| **Single Responsibility** | Chat API = validate + persist. Outbox Relay = publish. Message Service = consume. |
| **Open/Closed** | Add new downstream consumers by subscribing to MQTT. No changes to Chat API or Relay. |
| **Liskov Substitution** | MQTT publisher could be replaced with Kafka, RabbitMQ, or SQS via same outbox pattern. |
| **Interface Segregation** | OutboxRepo has only 4 methods. ChatRepo is split (CreateMessage → CreateMessageWithOutbox). |
| **Dependency Inversion** | Relay depends on `OutboxRepo` interface, not on PG directly. |
| **DRY** | MQTT publish code lives in ONE place (Relay), not 3 services. |
| **Exactly-once delivery** | Outbox + idempotent consumers = no duplicates, no loss. |
| **Transactional integrity** | PG transaction guarantees message + outbox are atomic. |
