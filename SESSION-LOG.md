# Orbit Messenger — Session Log (Brain)

> **Purpose**: Persistent memory across Codebuff sessions. Read this at the start of every session to reconstruct full context.
> **Last Updated**: July 6, 2026

---

## 🎯 Current Phase: Phase C — Single-Writer Complete

All 3 phases of the Transactional Outbox migration are **complete**. The system now uses a single-writer architecture:
- **Chat API** writes messages + outbox entries atomically via PG transaction
- **Outbox Relay** polls the outbox table and publishes to MQTT
- **Message Service** is consumer-only (no message persistence)
- The legacy `OUTBOX_ENABLED` flag and `CreateMessage` code paths have been removed

### Migration Status

| Phase | Description | Status |
|-------|-------------|--------|
| **Phase A** | Add outbox table + dual-write (outbox + fallback MQTT publish) under feature flag | ✅ Complete |
| **Phase B** | Outbox Relay service goes live, remove fallback MQTT publish | ✅ Complete |
| **Phase C** | Remove flag, message service becomes consumer-only, remove legacy code | ✅ Complete |

---

## ✅ Completed Work

### ✅ All Tests Passing (July 6, 2026)
- **`go test ./...`** — All 31 tests pass across the project:
  - `auth-service/internal/biz` — 25 biz layer tests (register, login, token validation, username validation, update/delete user, search, MQTT creds, IsAdmin)
  - `auth-service/internal/server` — 11 HTTP handler tests (handleGetMe, handleGetUser scenarios)
  - `chat-api/internal/biz` — 17 biz layer tests (outbox path, legacy path, participant validation)
- **No compilation errors** — All imports, types, and references verified
- **Key fixes applied during testing**:
  - Fixed `makeUser()` in auth_test.go to generate a real bcrypt hash (login tests were failing with invalid hash)
  - Fixed `setClaimsContext()` in http_test.go to use typed `auth.SetClaims` key (all handler tests were failing with 401)

### 🏗️ Architecture Design Documents

| File | What It Contains |
|------|-----------------|
| `ARCHITECTURE-REVIEW.md` | 49-item architecture review: critical issues, pattern improvements, SOLID violations, testing strategy, prioritized 4-phase roadmap |
| `DESIGN-OUTBOX-MESSAGE-FLOW.md` | Full design: single-writer architecture, outbox pattern, relay service, 3-phase migration, failure scenarios, monitoring |
| `AI-ARCHITECTURE.md` | Condensed quick-reference: service ports, Clean Architecture layers, DB schema, MQTT topics, auth flow, business rules, Makefile commands |

### ✏️ Code Changes

#### 🔒 Cross-Service Auth Fix (Security Hole Closed)
- **`shared/auth/jwt.go`** (NEW): Shared JWT validation package with:
  - `JWTClaims` struct and `ValidateToken(tokenString, jwtSecret)` function
  - Typed context key (not magic string) — fixes the unsafe `context.WithValue` issue
  - Safe getters (`GetUserID`, `GetOrgID`, `GetClaims`, `GetEmail`, `GetRole`) that return errors instead of panicking
  - Reusable across all services that need auth
- **`auth-service/internal/server/http.go`**: Migrated to `shared/auth` — auth middleware now stores claims via `auth.SetClaims` (typed key); all 8 handlers use `auth.GetUserID`/`auth.GetOrgID` safe getters instead of `context.Value("claims").(*biz.JWTClaims)` type assertions; removed `context` import; cleaned CORS headers
- **`chat-api/internal/server/chat_http.go`**: Replaced header-based `X-User-ID`/`X-Organization-ID` impersonation hole with proper JWT validation via `auth.ValidateToken`; updated context getters to use safe `auth.GetUserID`/`auth.GetOrgID`; removed `X-User-ID`/`X-Organization-ID` from CORS headers
- **`chat-api/cmd/chat-api/main.go`**: Reads `JWT_SECRET` env var and passes to server constructor
- **`media-service/internal/server/media_http.go`**: Same JWT validation fix; removed old `strconv`/`contextKey` local types; converts int user ID to deterministic UUID for media service compatibility
- **`media-service/cmd/media-service/main.go`**: Reads `JWT_SECRET` env var and passes to server constructor
- **`docker-compose.yml`**: Added `JWT_SECRET=your-super-secret-jwt-key-change-in-production` env var for both `chat-api` and `media-service` services

#### 🐛 Auth Service — Error Handling Improvements
- **`auth-service/internal/biz/auth.go`**: Added `ErrInsufficientPermissions` and `ErrCannotDeleteSelf` sentinel errors; fixed `Login` error propagation (was masking real DB errors with `ErrUserNotFound`); replaced inline `errors.New()` calls with sentinel errors in `UpdateUser`/`DeleteUser`
- **`auth-service/internal/server/http.go`**: Changed all error comparisons from `==` to `errors.Is()` for proper error wrapping; added proper 404/403/500 distinction for `handleGetMe` and `handleGetUser`; proper error handling for requester not found and DB errors

#### 📬 Chat API — Transactional Outbox (Phase A)
- **`chat-api/internal/biz/chat.go`**: Added `OutboxEntry` struct; added `CreateMessageWithOutbox` to `ChatRepo` interface; `ChatUsecase` now accepts `outboxEnabled` flag; `SendMessage` uses outbox path when enabled (atomic PG transaction + fallback MQTT publish); added `buildOutboxEntry` helper; fixed error propagation in participant checks (returning actual errors instead of masking with `ErrNotParticipant`)
- **`chat-api/internal/data/chat.go`**: Implemented `CreateMessageWithOutbox` — opens PG transaction, inserts message + outbox entry, commits atomically
- **`chat-api/cmd/chat-api/main.go`**: Added `OUTBOX_ENABLED` env var feature flag, passed to `NewChatUsecase`

#### 🔧 Error Comparison Fixes (Cross-Service)
- **`chat-api/internal/server/chat_http.go`**: Changed `handleError` switch from `==` to `errors.Is()`
- **`media-service/internal/server/media_http.go`**: Changed `handleError` switch from `==` to `errors.Is()`

#### 🗄️ Database Migration
- **`scripts/init.sql`**: Added `outbox_status` ENUM, `message_outbox` table with indexes, `message_outbox_dead_letter` table

#### 🧪 Unit Tests
- **`auth-service/internal/server/http_test.go`**: 11 HTTP handler tests with mock repo covering `handleGetMe` (found, not found, DB error) and `handleGetUser` (same org, different org, not found, DB error, invalid ID, requester scenarios)
- **`chat-api/internal/biz/chat_test.go`**: 17 biz layer tests covering outbox-enabled path, legacy path, participant validation, dedupe key passthrough, content/meta preservation, `buildOutboxEntry` correctness, feature flag routing

#### 📦 Shared Code Consolidation (DRY)
- **`shared/config/env.go`** (NEW): `GetEnv(key, default)` — replaces 5 duplicated `getEnv` functions across all main.go files
- **`shared/server/response.go`** (NEW): `WriteJSON`, `WriteError`, `CORSMiddleware` — replaces duplicated response helpers and CORS headers across all 4 HTTP server files
- **5 main.go files** updated: removed local `getEnv` functions, use `config.GetEnv` instead; handlers wrapped with `sharedserver.CORSMiddleware`
- **4 HTTP server files** updated: removed local `writeJSON`/`writeError` methods; removed CORS header boilerplate from `ServeHTTP`; use `sharedserver.WriteJSON`/`WriteError` throughout

#### 🧹 Wire DI Dead Code Removal
- **10 Go files** across all 5 services: Removed unused `wire.NewSet`/`ProviderSet` declarations, `github.com/google/wire` imports, dead `Data` structs, and `NewData` factory functions — all were only used by Google Wire, which was never actually wired (no `wire.Build()` calls or `wire_gen.go` files existed)
- **`Makefile`**: Removed `wire` target, `generate` no longer depends on `wire`, removed `wire_gen.go` from `clean`, removed `wire/cmd/wire` from `init` install tools, updated help text
- Real code preserved: `NewDB` (message-service), `NewRedisClient` (presence-service), `NewPresenceUsecaseFromConfig`, `NewMediaUsecaseFromConfig`, all sentinel errors

#### 🧪 Makefile — Enhanced Test Targets
- **`make test`**: Now runs with `-count=1` to disable caching
- **`make test-short`**: Runs with `-short` flag for quick CI checks
- **`make test-race`**: Runs with race detector
- **`make test-cover`**: Runs with coverage profiling, shows top-30 by coverage %, total summary
- **`make test-cover-html`**: Generates `build/coverage.html` for browser viewing
- **`make test-cover-<service>`**: Service-specific coverage (e.g., `make test-cover-auth-service`)
- **`make test-verbose`**: Per-package loop showing full output for each package
- **`make test-all`**: Runs `test`, `test-race`, and `test-cover` sequentially

#### 📬 Phase B — Outbox Relay Service
- **`outbox-relay/`** (NEW): Standalone service for consuming the `message_outbox` table
  - **`internal/biz/relay.go`**: `OutboxRecord` types, `OutboxRepo`/`MQTTPublisher` interfaces, `RelayService` with `Run()` polling loop, `processBatch()`/`processRecord()`, exponential backoff with jitter (±25%), dead-letter escalation after max retries
  - **`internal/biz/relay_test.go`**: 13 unit tests (happy path, retry, dead letter, context cancellation, backoff calculation, default config)
  - **`internal/data/relay.go`**: PostgreSQL implementation with `FOR UPDATE SKIP LOCKED`, `FetchPending` (pending + retryable rows), `MarkPublished`, `MarkFailed` (increments retry_count + sets next_retry_at), `MoveToDeadLetter` (atomic transaction: insert dead letter + update outbox)
  - **`cmd/outbox-relay/main.go`**: Entry point with DB/MQTT setup, graceful shutdown via context cancellation, health check on port 8005
  - **`Dockerfile`**: Multi-stage build
  - **`configs/config.yaml`** and **`config-local.yaml`**: Default relay config per PO decisions (500ms poll, 5 retries, 2s backoff)
- **`chat-api/internal/biz/chat.go`**: Removed fallback `PublishMessage` call from outbox-enabled path — Outbox Relay now handles MQTT delivery
- **`chat-api/cmd/chat-api/main.go`**: Flipped `OUTBOX_ENABLED` default from `"false"` to `"true"`
- **`chat-api/internal/biz/chat_test.go`**: Updated tests to reflect Phase B (assert `PublishMessage` is NOT called on outbox path)
- **`shared/config/env.go`**: Added `GetInt` and `GetDuration` helpers
- **`docker-compose.yml`**: Added `outbox-relay` service with all env vars, depends on postgres + emqx
- **`Makefile`**: Added `outbox-relay` to `SERVICES` list

#### 🧹 Phase C — Final Cleanup (Single-Writer Complete)
- **`chat-api/internal/biz/chat.go`**: Removed `outboxEnabled` flag field entirely; removed `CreateMessage` from `ChatRepo` interface; removed `PublishMessage` from `MQTTPublisher` interface; `SendMessage` always uses `CreateMessageWithOutbox` (single code path)
- **`chat-api/internal/data/chat.go`**: Removed legacy `CreateMessage` implementation (non-outbox)
- **`chat-api/internal/data/mqtt_publisher.go`**: Removed `PublishMessage` implementation — only `PublishTypingIndicator` remains
- **`chat-api/cmd/chat-api/main.go`**: Removed `OUTBOX_ENABLED` env var and flag logic; simplified `NewChatUsecase(repo, publisher)` to 2 params
- **`chat-api/internal/biz/chat_test.go`**: Removed all legacy path tests (6 tests → 8 simplified Phase C tests); removed `createMessageFunc` and `publishMessageFunc` from mocks
- **`message-service/internal/biz/message.go`**: `ProcessIncomingMessage` is now consumer-only (logs + TODO stubs for receipts/notifications/indexing); removed `CreateMessage` from `MessageRepo` interface
- **`message-service/internal/data/message.go`**: Removed `CreateMessage` implementation

#### 🧪 Media & Presence Service Unit Tests
- **`media-service/internal/biz/media_test.go`**: 12 tests covering InitiateUpload (success, file too large, invalid type, with messageID), CompleteUpload (ready direct, not found, wrong status, storage fails), GetDownloadURL, DeleteAttachment (success, storage fails still deletes DB), AssociateWithMessage, GetMessageAttachments, isAllowedContentType, NewMediaUsecaseFromConfig
- **`presence-service/internal/biz/presence_test.go`**: 12 tests covering HandleClientConnected, HandleClientDisconnected (last session sets offline, other active stays online), HandlePresenceUpdate (valid + invalid JSON), HandleHeartbeat (success + session not found), GetUserPresence, GetMultipleUserPresence, SetUserStatus, NewPresenceUsecaseFromConfig, CleanupStalePresence, GetUserDeviceSessions

#### 🖥️ Media & Presence HTTP Server Tests
- **`media-service/internal/server/media_http_test.go`**: 24 tests covering auth middleware (missing header, invalid format, invalid token, valid token through router) and all 8 handlers (InitiateUpload, CompleteUpload, GetAttachment, GetDownloadURL, DeleteAttachment, AssociateWithMessage, GetMessageAttachments, GenerateThumbnail) with success + error scenarios
- **`presence-service/internal/server/presence_http_test.go`**: 16 tests covering all 5 handlers (GetUserPresence, SetUserStatus, GetMultipleUserPresence, GetUserSessions, ClientConnect) with success + validation edge cases

---

## 🧭 Project Structure

```
Orbit-Messenger-chat-app/
├── auth-service/       # JWT auth, users, orgs, Keycloak OIDC (port 8080)
├── chat-api/           # REST API for conversations, messages, participants (port 8003)
├── media-service/      # File upload/download via MinIO, ClamAV scan (port 8004)
├── message-service/    # MQTT subscriber, persists messages (port 8001) — to become consumer-only
├── presence-service/   # Redis-backed online/offline, device sessions (port 8002)
├── shared/
│   ├── auth/jwt.go     # Shared JWT validation + typed context helpers
│   ├── config/         # Shared config structs (Database, Redis, MQTT, Minio, etc.)
│   └── proto/          # Protobuf definitions
├── scripts/            # init.sql, proto generation
├── docker-compose.yml  # All services + infra
├── go.mod / go.sum     # Module: github.com/thisisjayakumar/Orbit-Messenger-chat-app
├── AI-ARCHITECTURE.md  # Quick reference
├── ARCHITECTURE-REVIEW.md  # Full architecture review
├── DESIGN-OUTBOX-MESSAGE-FLOW.md  # Outbox design doc
└── SESSION-LOG.md      # THIS FILE — persistent brain
```

### Key Architectural Facts
- **Language**: Go
- **Architecture**: Clean Architecture (biz → interfaces, data → implementations, server → HTTP/MQTT handlers)
- **DB**: PostgreSQL (all UUID PKs)
- **Real-time**: EMQX MQTT broker (QoS 1 for messages, QoS 0 for typing indicators)
- **Cache**: Redis (presence service only)
- **Storage**: MinIO (S3-compatible, file attachments)
- **Auth**: JWT (HS256, 24h TTL, shared secret across all services) + optional Keycloak OIDC
- **Multi-tenancy**: Organizations scope all data (users, conversations)
- **DI**: Google Wire (not fully implemented — most services still use manual wiring)

### Infra (docker-compose ports)
| Service | Port |
|---------|------|
| PostgreSQL | 5432 |
| Redis | 6379 |
| EMQX | 1883 |
| MinIO | 9000 |
| Keycloak | 8090 |

---

## 🗺️ Roadmap Status

| Phase | Items | Status |
|-------|-------|--------|
| **Phase 1 — Foundation** | Unit tests, Transactional Outbox, error handling, **cross-service auth**, **context type safety**, shared helpers | 🟡 In progress (outbox + tests + error fixes + auth fix done) |
| **Phase 2 — Resilience** | Circuit breaker, connection pools, cursor pagination, decorator middleware, Redis cache | 🔜 Not started |
| **Phase 3 — Features** | Push notifications, command pattern, state machine, rate limiting, idempotency keys | 🔮 Not started |
| **Phase 4 — Polish** | Builder pattern, message pipeline, event bus, interface DI, N+1 fixes | 🔮 Not started |

### Known Remaining Issues
1. 🟡 **Message Service dual-write** — Persists same messages that Chat API already wrote (to be resolved in Phase B/C of outbox migration)

~*(Issues 1 (Wire DI), 2 (cross-service auth), 3 (context type safety), 4 (auth-service migration), and 5 (duplicated code) from original review are now resolved)*~

---

## 📋 Decisions Log

| # | Decision | Date | Context |
|---|----------|------|---------|
| 1 | **Transactional Outbox** for exactly-once delivery (not direct MQTT publish) | Jul 6 | Architecture review finding #8 |
| 2 | **Single-writer**: Chat API publishes only, Message Service becomes consumer-only | Jul 6 | Eliminates dual-write hazard |
| 3 | **Phase A dual-write fallback**: Outbox + direct MQTT publish during migration | Jul 6 | Backward compatibility until Relay is live |
| 4 | **Outbox Relay as standalone service** (not embedded in Chat API) | Jul 6 | Decoupled lifecycle, scales independently, zero latency impact |
| 5 | **Typing indicators stay direct QoS 0** (no outbox) | Jul 6 | Ephemeral, high-frequency, low-importance |
| 6 | **Web-first** (REST primary, MQTT WebSocket for real-time) | Jul 6 | PO decision: small scale (< 1K concurrent users) |
| 7 | **`errors.Is()` over `==`** for error comparisons going forward | Jul 6 | Proper error wrapping support |
| 8 | **Sentinel errors in biz layer** instead of inline `errors.New()` | Jul 6 | Enables `errors.Is()` matching in server handlers |
| 9 | **Local JWT validation over remote Auth Service RPC** | Jul 6 | Shared secret approach: simpler, no extra latency, no extra RPC |
| 10 | **Typed context keys in `shared/auth`** instead of magic strings | Jul 6 | Eliminates runtime panic risk from type assertions |
| 11 | **Auth service server migrated to `shared/auth` context helpers** | Jul 6 | All 3 authenticated services now use same typed context key for claims |

---

## 🚧 Next Steps (Ordered)

1. **Implement downstream tasks in Message Service**: Add delivery receipts, push notifications, and search indexing to the consumer-only `ProcessIncomingMessage` handler
2. **Add Prometheus metrics to outbox-relay**: Track pending count, latency, published/failed/dead-letter counters
3. **More tests**: Media service tests (biz + server), presence service tests, message service tests for consumer logic
4. **Add integration test CI**: Ensure `go test -tags=integration ./...` runs as part of CI pipeline with a PostgreSQL service: Track pending count, latency, published/failed/dead-letter counters
5. **More tests**: Media service tests, presence service tests

---

## 📐 Current Git State

- **Branch**: `main` (up to date with `origin/main`)
- **Uncommitted changes**: Phase A + Phase B work (outbox, error handling, tests, docs, auth fix, outbox-relay)
- **New directories**: `outbox-relay/` (cmd, internal/biz, internal/data, Dockerfile, Makefile, configs)
- **Untracked files**: `outbox-relay/...`, `AI-ARCHITECTURE.md`, `ARCHITECTURE-REVIEW.md`, `DESIGN-OUTBOX-MESSAGE-FLOW.md`, `SESSION-LOG.md`
- **Modified files**: `auth-service/internal/server/http.go`, `chat-api/internal/biz/chat.go`, `chat-api/internal/biz/chat_test.go`, `chat-api/cmd/chat-api/main.go`, `chat-api/internal/server/chat_http.go`, `media-service/internal/server/media_http.go`, `media-service/cmd/media-service/main.go`, `docker-compose.yml`, `shared/config/env.go`, `Makefile`

---

## 🔍 How to Use This Brain

When starting a new Codebuff session, I will:
1. Read `SESSION-LOG.md` to understand where we left off
2. Read `AI-ARCHITECTURE.md` for quick architectural context
3. Read any relevant design docs (`ARCHITECTURE-REVIEW.md`, `DESIGN-OUTBOX-MESSAGE-FLOW.md`)
4. Check git status for latest uncommitted changes
5. Continue from the next steps listed above

To expand this brain, add entries to:
- **✅ Completed Work** when finishing a task
- **📋 Decisions Log** when making an architectural choice
- **🚧 Next Steps** when adding or reordering work items
