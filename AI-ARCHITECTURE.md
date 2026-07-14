# ORBIT MESSENGER — AI Architecture QuickRef

## SYSTEM OVERVIEW
Go microservice chat app: 5 services + 5 infra deps. Real-time via MQTT. Multi-tenant (org-scoped).

## SERVICE LAYOUT (Ports)

```
auth-service:8080   → JWT auth, users, orgs, Keycloak OIDC
message-service:8001 → MQTT subscriber, persists msgs + receipts + attachments to PG
presence-service:8002 → Redis-backed online/offline, device sessions, heartbeat
chat-api:8003        → REST API for conversations, msgs, participants, typing
media-service:8004   → File upload/download via MinIO, optional ClamAV scan
```

INFRA: PostgreSQL:5432 | Redis:6379 | EMQX:1883 | MinIO:9000 | Keycloak:8090

## CLEAN ARCHITECTURE LAYER (per service)

```
cmd/{service}/main.go  → Entry, wires deps, starts servers
internal/
  biz/                 → UseCase interfaces + implementations (pure Go, no framework)
  data/                → Repo implementations (sql.DB, redis.Client, MQTT publisher)
  server/              → HTTP handlers (gorilla/mux), MQTT message handlers
  conf/                → Config protobuf defs + generated .pb.go
configs/config.yaml    → YAML overrides via kratos config
```

**Rule**: `biz` NEVER imports `data` or `server`. `biz` defines interfaces; `data` implements them.

## DEPENDENCY INJECTION (Google Wire)
Each service has `wire.NewSet(...)` in `biz/biz.go`, `data/data.go`, `server/...`. Wire generates `wire_gen.go` in `cmd/`. Run `make wire` to regenerate.

```
ProviderSet = wire.NewSet(NewXxxUsecase)  // in biz/
ProviderSet = wire.NewSet(NewData, NewXxxRepo)  // in data/
```

## SHARED CODE
```
shared/
  config/config.go   → Structs: Database, Redis, MQTT, Minio, OpenSearch, Server
  proto/v1/          → common.proto → common.pb.go (Organization, User, Message)
```

## DB SCHEMA (PostgreSQL, all UUID PKs)
```
organizations (id, name, settings JSONB)
users (id, org_id FK, email CITEXT, display_name, avatar_url, profile JSONB, password_hash, keycloak_id)
conversations (id, org_id FK, type ENUM(DM|GROUP), title, created_by FK, is_encrypted)
conversation_participants (id, conv_id FK, user_id FK, role ENUM(admin|member), last_read_at)
messages (id, conv_id FK, sender_id FK, content_type, content, meta JSONB, dedupe_key, sent_at, edited_at, deleted BOOL)
message_receipts (id, msg_id FK, user_id FK, status ENUM(delivered|read), at)
attachments (id, msg_id FK, object_key, file_name, mime_type, size, status, meta JSONB)
device_sessions (id, user_id FK, client_id, device_info, ip INET)
audit_events (id BIGSERIAL, org_id FK, user_id FK, action, target_type, target_id, details JSONB)
```

**Index patterns**: `conv_org_type_idx`, `msg_conv_time_idx`, `msg_dedupe_uidx` (partial), `conv_part_unique`, `msg_receipt_unique`.

## REAL-TIME (MQTT / EMQX)
```
Chat API publishes → EMQX → Message Service subscribes & persists
chat/{convId}/messages     → QoS 1, payload = Message JSON
chat/{convId}/typing       → QoS 0, broadcast only, no DB persist

Presence Service subscribes to:
  presence/+/status         → user status changes
  $SYS/brokers/+/clients/+/connected    → EMQX system topic for connect events
  $SYS/brokers/+/clients/+/disconnected → EMQX system topic for disconnect events
```

## AUTH FLOW
```
Register → bcrypt hash → INSERT user → generate JWT (HS256, 24h TTL)
Login → bcrypt compare → update last_seen → return JWT
Validate → jwt.ParseWithClaims → extract JWTClaims{UserID, OrgID, Email, Role}
```
Claims injected into `context.Value("claims")` via middleware. Chat API & Media Service use simpler header-based `X-User-ID` / `X-Organization-ID` (TODO: real JWT validation cross-service).

## BIZ RULES (Key Logic)

### Chat Uc
- DM convs: creator + exactly 1 other participant (total 2)
- Group convs: creator + any number
- Creator becomes admin; others are member
- Send msg: persist to PG → publish MQTT (best-effort)
- Only admin can add/remove participants
- Any participant can leave
- `MarkAsRead` updates `last_read_at` on participant row
- `is_read` derived via EXISTS subquery checking if any other participant has `last_read_at >= msg.sent_at`

### Presence Uc
- Redis key pattern: `presence:user:{userID}` → JSON of UserPresence
- Device session keys: `session:device:{clientID}` + `sessions:user:{userID}` (Redis Set)
- Connect → create session + set online
- Disconnect → mark session disconnected + remove from user set → if no active sessions left, set offline
- Heartbeat updates `LastHeartbeat` on session
- Cleanup routine every 5 minutes: scans keys, marks stale sessions/presence offline

### Media Uc
- Upload flow: initiate → get presigned MinIO URL → client uploads → complete → verify file → optional ClamAV async scan → mark ready
- File statuses: uploading → scanning → ready | quarantine | error
- Max 100MB, whitelist content types only
- Object key format: `attachments/{userUUID}/{timestamp}_{fileUUID}.{ext}`
- Download via presigned URL (1h expiry)

### Auth Uc
- Username rules (Instagram-like): 3-30 chars, a-z0-9._ only, no start/end with dot, no consecutive dots, reserved names blocked
- Users scoped to organizations. Register requires org_id or org_name (auto-create org)
- Admin role can CRUD any user in org. Non-admin can only update own display_name/avatar/profile
- Keycloak OIDC optional (graceful fallback if unreachable)

## NON-FUNCTIONAL

- **Graceful shutdown**: SIGINT/SIGTERM → 5s timeout context → http.Server.Shutdown
- **Config loading**: Kratos file config, env var overrides via `getEnv(key, default)` (env wins)
- **Soft deletes**: Messages only (deleted=true). Users hard-deleted. Conversations cascade-delete.
- **Deduplication**: `dedupe_key` + partial unique index `ON CONFLICT DO NOTHING` in message insert
- **CORS**: All services allow `*` origin
- **Wire DI**: Not fully implemented yet (most services still use manual wiring in main.go)
- **Auth gap**: Chat API & Media Service use header-based X-User-ID instead of JWT validation with Auth Service
- **Error types**: Standard sentinel errors in each `biz/biz.go` (ErrNotFound, ErrUnauthorized, etc.)

## MAKEFILE COMMANDS
```
make init              Install protoc, wire, kratos tools
make generate          Proto gen + wire gen + go generate
make build             Build all 5 services to bin/
make run-auth-service  go run, same for other services
make test              go test ./...
make lint/fmt          golangci-lint / go fmt
```

## PACKAGE DEPENDENCIES (go.mod)
```
go-kratos/kratos/v2    → config, log framework
gorilla/mux            → HTTP routing
google/wire            → DI codegen
eclipse/paho.mqtt      → MQTT client
lib/pq                 → PostgreSQL driver
redis/go-redis/v9      → Redis client
golang-jwt/jwt/v5      → JWT parse/sign
Nerzal/gocloak/v13     → Keycloak API client
minio/minio-go/v7      → S3-compatible storage
google/uuid            → UUID generation
golang.org/x/crypto    → bcrypt
```
