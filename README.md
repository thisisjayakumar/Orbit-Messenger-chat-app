# Orbit Messenger Backend

A comprehensive microservices-based chat application backend built with Go, featuring real-time messaging, secure authentication, and scalable architecture.

## 🏗️ Architecture Overview

The backend consists of 5 microservices working together to provide a complete chat platform:

### Core Services

| Service | Port | Description |
|---------|------|-------------|
| **Auth Service** | 8080 | User authentication, JWT tokens, MQTT credentials |
| **Message Service** | 8001 | MQTT message processing and database persistence |
| **Presence Service** | 8002 | Real-time user status tracking and device sessions |
| **Chat API** | 8003 | REST API for conversations, messages, and participants |
| **Media Service** | 8004 | File upload/download with MinIO storage |

### Infrastructure Services

| Service | Port | Description |
|---------|------|-------------|
| **PostgreSQL** | 5432 | Primary database for all persistent data |
| **Redis** | 6379 | Caching and presence data storage |
| **EMQX** | 1883/8083 | MQTT broker for real-time messaging |
| **MinIO** | 9000/9001 | S3-compatible object storage for files |
| **Keycloak** | 8090 | Identity and access management |

## 🚀 Quick Start

### Prerequisites

- **Docker and Docker Compose** (for containerized deployment)
- **Go 1.23+** (for development)
- **Protocol Buffer tools** (for code generation)
- **Make** (for build automation)

### Option 1: Docker Compose (Recommended)

#### 1. Start All Services

```bash
# Clone and navigate to the project
git clone <repository-url>
cd Orbit-Messenger-chat-app

# Start all services with Docker Compose
docker-compose up -d

# Check service status
docker-compose ps
```

#### 2. Verify Services

```bash
# Test all services health endpoints
curl http://localhost:8080/health  # Auth Service
curl http://localhost:8001/health  # Message Service
curl http://localhost:8002/health  # Presence Service
curl http://localhost:8003/health  # Chat API
curl http://localhost:8004/health  # Media Service
```

#### 3. Access Management Interfaces

- **Keycloak Admin**: http://localhost:8090 (admin/admin123)
- **EMQX Dashboard**: http://localhost:18083 (admin/public)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin123)

### Option 2: Local Development

#### 1. Install Development Tools

```bash
# Install required tools
make init

# Download dependencies
make deps
```

#### 2. Start Infrastructure Services

```bash
# Start only infrastructure services
docker-compose up -d postgres redis emqx minio keycloak

# Wait for services to be ready (check with docker-compose ps)
```

#### 3. Build and Run Services Locally

```bash
# Generate code (protobuf, wire)
make generate

# Build all services
make build

# Run individual services (in separate terminals)
make run-auth-service
make run-message-service
make run-presence-service
make run-chat-api
make run-media-service
```

#### 4. Alternative: Run All Services

```bash
# Run all services with go run
make run-auth-service & \
make run-message-service & \
make run-presence-service & \
make run-chat-api & \
make run-media-service
```

## 📊 Database Schema

The application uses PostgreSQL with the following main entities:

- **Organizations**: Multi-tenant organization support
- **Users**: User accounts with Keycloak integration
- **Conversations**: Chat rooms (DM or Group)
- **Participants**: User membership in conversations
- **Messages**: Chat messages with metadata
- **Receipts**: Message delivery and read receipts
- **Attachments**: File attachments with metadata
- **Device Sessions**: User device/client sessions
- **Audit Events**: System audit trail

## 🔐 Security Features

- **Authentication**: Keycloak OIDC integration
- **Authorization**: JWT-based API access
- **MQTT Security**: JWT-based MQTT authentication
- **File Security**: Antivirus scanning, type validation
- **Data Encryption**: TLS in transit, encryption at rest support
- **Audit Logging**: Comprehensive audit trail

## 🌐 API Endpoints

### Auth Service (Port 8080)

```
POST /api/v1/auth/register       - User registration
POST /api/v1/auth/login          - User login
POST /api/v1/auth/oidc/login     - OIDC login
POST /api/v1/auth/validate       - Token validation
GET  /api/v1/auth/me             - Get current user
GET  /api/v1/auth/mqtt-credentials - Get MQTT credentials
```

### Chat API (Port 8003)

```
POST /api/v1/conversations                           - Create conversation
GET  /api/v1/conversations                           - Get user conversations
GET  /api/v1/conversations/{id}                      - Get conversation details
PUT  /api/v1/conversations/{id}                      - Update conversation
GET  /api/v1/conversations/{id}/messages             - Get messages
POST /api/v1/conversations/{id}/messages             - Send message
GET  /api/v1/conversations/{id}/participants         - Get participants
POST /api/v1/conversations/{id}/participants         - Add participant
POST /api/v1/conversations/{id}/read                 - Mark as read
POST /api/v1/conversations/{id}/typing               - Send typing indicator
```

### Presence Service (Port 8002)

```
GET /api/v1/presence/{userID}                        - Get user presence
PUT /api/v1/presence/{userID}/status                 - Set user status
POST /api/v1/presence/bulk                           - Get multiple user presence
GET /api/v1/presence/{userID}/sessions               - Get user sessions
```

### Media Service (Port 8004)

```
POST /api/v1/upload/initiate                         - Initiate file upload
POST /api/v1/upload/{id}/complete                    - Complete upload
GET  /api/v1/attachments/{id}                        - Get attachment info
GET  /api/v1/attachments/{id}/download               - Get download URL
DELETE /api/v1/attachments/{id}                      - Delete attachment
GET  /api/v1/messages/{id}/attachments               - Get message attachments
```

## 🔄 MQTT Topics

The system uses MQTT for real-time communication:

- `chat/{conversationId}/messages` - Real-time messages
- `chat/{conversationId}/typing` - Typing indicators
- `presence/{userId}/status` - Presence updates
- `$SYS/brokers/+/clients/+/connected` - Client connections
- `$SYS/brokers/+/clients/+/disconnected` - Client disconnections

## 🧪 API Testing

### Authentication Flow

#### 1. Register a User

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123",
    "display_name": "Test User"
  }'
```

#### 2. Login and Get Token

```bash
# Login and get JWT token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'

# Save the token from response for subsequent requests
export TOKEN="your-jwt-token-here"
```

#### 3. Get MQTT Credentials

```bash
# Get MQTT credentials for real-time messaging
curl -X GET http://localhost:8080/api/v1/auth/mqtt-credentials \
  -H "Authorization: Bearer $TOKEN"
```

### Chat API Testing

#### 1. Create a Conversation

```bash
# Create a direct message conversation
curl -X POST http://localhost:8003/api/v1/conversations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "DM",
    "participants": ["user-id-here"]
  }'
```

#### 2. Send a Message

```bash
# Send a text message
curl -X POST http://localhost:8003/api/v1/conversations/{conversation-id}/messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Hello, World!",
    "content_type": "text/plain"
  }'
```

#### 3. Get Messages

```bash
# Get conversation messages
curl -X GET http://localhost:8003/api/v1/conversations/{conversation-id}/messages \
  -H "Authorization: Bearer $TOKEN"
```

### Presence Service Testing

```bash
# Get user presence
curl -X GET http://localhost:8002/api/v1/presence/{user-id} \
  -H "Authorization: Bearer $TOKEN"

# Set user status
curl -X PUT http://localhost:8002/api/v1/presence/{user-id}/status \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "online",
    "status_text": "Available"
  }'
```

### Media Service Testing

#### 1. Initiate File Upload

```bash
# Initiate file upload
curl -X POST http://localhost:8004/api/v1/upload/initiate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "test.txt",
    "mime_type": "text/plain",
    "size": 1024
  }'
```

#### 2. Complete Upload

```bash
# Complete file upload
curl -X POST http://localhost:8004/api/v1/upload/{upload-id}/complete \
  -H "Authorization: Bearer $TOKEN"
```

### MQTT Testing

#### Connect to MQTT Broker

```bash
# Install MQTT client tools
# On macOS: brew install mosquitto
# On Ubuntu: sudo apt-get install mosquitto-clients

# Subscribe to messages
mosquitto_sub -h localhost -p 1883 \
  -u "your-mqtt-username" \
  -P "your-mqtt-password" \
  -t "chat/{conversation-id}/messages"

# Publish a test message
mosquitto_pub -h localhost -p 1883 \
  -u "your-mqtt-username" \
  -P "your-mqtt-password" \
  -t "chat/{conversation-id}/messages" \
  -m '{"content": "Test MQTT message"}'
```

## 🛠️ Development

### Project Structure

```
├── auth-service/          # Authentication service
│   ├── cmd/auth-service/  # Main application entry
│   ├── internal/         # Internal packages
│   │   ├── biz/          # Business logic
│   │   ├── data/         # Data access layer
│   │   ├── server/       # HTTP server
│   │   └── conf/         # Configuration & protobuf
│   ├── configs/          # Configuration files
│   └── Dockerfile        # Container definition
├── message-service/       # Message processing service
├── chat-api/             # Chat REST API
├── presence-service/     # Presence tracking service
├── media-service/        # File handling service
├── shared/               # Shared utilities and proto files
├── scripts/              # Database initialization scripts
├── docker-compose.yml    # Infrastructure orchestration
├── go.mod                # Go module dependencies
└── Makefile              # Build automation
```

### Development Commands

#### Code Generation

```bash
# Install development tools
make init

# Generate protobuf files and wire code
make generate

# Generate only shared proto files
make generate-shared-proto

# Generate only service proto files
make generate-service-protos

# Generate wire dependency injection
make wire
```

#### Building Services

```bash
# Build all services
make build

# Build specific service
make build-auth-service
make build-message-service
make build-chat-api
make build-presence-service
make build-media-service
```

#### Running Services

```bash
# Run specific service locally
make run-auth-service
make run-message-service
make run-chat-api
make run-presence-service
make run-media-service
```

#### Testing

```bash
# Run all tests
make test

# Run tests for specific service
make test-auth-service
make test-message-service
make test-chat-api
make test-presence-service
make test-media-service
```

#### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Download and tidy dependencies
make deps

# Clean build artifacts
make clean
```

### Configuration

Each service has its own configuration files:
- `configs/config.yaml` - Production configuration
- `configs/config-local.yaml` - Local development configuration

### Environment Variables

Services can be configured using environment variables:

```bash
# Database
export DATABASE_URL="postgres://chat_user:chat_password@localhost:5432/chat_db?sslmode=disable"

# Redis
export REDIS_ADDR="localhost:6379"

# MQTT
export MQTT_BROKER_URL="tcp://localhost:1883"
export MQTT_USERNAME="service_username"
export MQTT_PASSWORD="service_password"

# MinIO
export MINIO_ENDPOINT="localhost:9000"
export MINIO_ACCESS_KEY="minioadmin"
export MINIO_SECRET_KEY="minioadmin123"

# Keycloak
export KEYCLOAK_URL="http://localhost:8090"
export KEYCLOAK_REALM="orbit-chat"
export KEYCLOAK_CLIENT_ID="orbit-chat-client"
export KEYCLOAK_CLIENT_SECRET="your-client-secret"

# Security
export JWT_SECRET="your-super-secret-jwt-key"
```

## 📈 Monitoring and Observability

### Health Checks

Each service exposes health check endpoints:
- `/health` - Basic health check
- `/metrics` - Prometheus metrics (if enabled)

### Logging

All services use structured logging with configurable levels. Logs are stored in the `logs/` directory.

### Infrastructure Monitoring

- **EMQX Dashboard**: http://localhost:18083 (admin/public)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin123)
- **Keycloak Admin**: http://localhost:8090 (admin/admin123)

## 🔧 Configuration

### Environment Variables

Key environment variables for production:

```bash
# Database
DATABASE_URL=postgres://user:pass@host:5432/dbname

# Redis
REDIS_URL=redis://host:6379

# MQTT
MQTT_BROKER_URL=tcp://host:1883

# MinIO
MINIO_ENDPOINT=host:9000
MINIO_ACCESS_KEY=access_key
MINIO_SECRET_KEY=secret_key

# Keycloak
KEYCLOAK_URL=https://keycloak.example.com
KEYCLOAK_REALM=orbit-chat
KEYCLOAK_CLIENT_ID=orbit-chat-client
KEYCLOAK_CLIENT_SECRET=client_secret

# Security
JWT_SECRET=your-super-secret-jwt-key
```

## 🚢 Deployment

### Docker Deployment

```bash
# Build and start all services (run from this directory)
docker-compose up --build -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f [service-name]

# Stop all services
docker-compose down
```

### Production Considerations

- Use environment-specific configuration files
- Set up proper SSL/TLS certificates
- Configure firewall rules for exposed ports
- Set up monitoring and alerting
- Implement backup strategies for databases

## 🆘 Troubleshooting

### Common Issues

#### 1. Services Not Starting

```bash
# Check Docker Compose logs
docker-compose logs [service-name]

# Check specific service logs
docker-compose logs auth-service
docker-compose logs message-service
docker-compose logs chat-api
docker-compose logs presence-service
docker-compose logs media-service

# Check infrastructure services
docker-compose logs postgres
docker-compose logs redis
docker-compose logs emqx
docker-compose logs minio
docker-compose logs keycloak
```

#### 2. Database Connection Issues

```bash
# Test PostgreSQL connection
docker exec orbit-postgres psql -U chat_user -d chat_db -c "SELECT 1;"

# Check database logs
docker-compose logs postgres

# Verify database initialization
docker exec orbit-postgres psql -U chat_user -d chat_db -c "\dt"
```

#### 3. MQTT Connection Problems

```bash
# Check EMQX status
docker exec orbit-emqx emqx_ctl status

# Check EMQX dashboard
open http://localhost:18083

# Test MQTT connection
mosquitto_pub -h localhost -p 1883 -t "test/topic" -m "test message"
```

#### 4. Authentication Failures

```bash
# Check Keycloak logs
docker-compose logs keycloak

# Access Keycloak admin console
open http://localhost:8090

# Verify Keycloak configuration
curl http://localhost:8090/realms/orbit-chat/.well-known/openid_configuration
```

#### 5. Redis Connection Issues

```bash
# Test Redis connection
docker exec orbit-redis redis-cli ping

# Check Redis logs
docker-compose logs redis
```

#### 6. MinIO Storage Issues

```bash
# Check MinIO logs
docker-compose logs minio

# Access MinIO console
open http://localhost:9001

# Test MinIO connection
curl http://localhost:9000/minio/health/live
```

### Debug Commands

```bash
# Check all service status
docker-compose ps

# View real-time logs
docker-compose logs -f

# Restart specific service
docker-compose restart [service-name]

# Rebuild and restart service
docker-compose up --build -d [service-name]

# Check service health
curl http://localhost:8080/health  # Auth Service
curl http://localhost:8001/health  # Message Service
curl http://localhost:8002/health  # Presence Service
curl http://localhost:8003/health  # Chat API
curl http://localhost:8004/health  # Media Service
```

### Development Debugging

#### Local Development Issues

```bash
# Check if infrastructure services are running
docker-compose ps postgres redis emqx minio keycloak

# Verify Go modules
go mod tidy
go mod verify

# Check generated code
make generate

# Run with verbose logging
go run ./auth-service/cmd/auth-service -conf ./auth-service/configs -log.level=debug
```

#### Port Conflicts

If you encounter port conflicts:

```bash
# Check what's using the ports
lsof -i :8080  # Auth Service
lsof -i :8001  # Message Service
lsof -i :8002  # Presence Service
lsof -i :8003  # Chat API
lsof -i :8004  # Media Service

# Kill processes using ports
sudo kill -9 $(lsof -t -i:8080)
```

### Performance Issues

#### Database Performance

```bash
# Check PostgreSQL performance
docker exec orbit-postgres psql -U chat_user -d chat_db -c "
SELECT schemaname,tablename,attname,n_distinct,correlation 
FROM pg_stats 
WHERE schemaname = 'public' 
ORDER BY n_distinct DESC;"

# Check slow queries
docker exec orbit-postgres psql -U chat_user -d chat_db -c "
SELECT query, mean_time, calls 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;"
```

#### Redis Performance

```bash
# Check Redis memory usage
docker exec orbit-redis redis-cli info memory

# Check Redis performance
docker exec orbit-redis redis-cli --latency
```

### Log Analysis

#### Service Logs Location

```bash
# View service logs
tail -f logs/auth-service.log
tail -f logs/message-service.log
tail -f logs/chat-api.log
tail -f logs/presence-service.log
tail -f logs/media-service.log

# Search logs for errors
grep -i error logs/*.log
grep -i "panic\|fatal" logs/*.log
```

#### Docker Logs

```bash
# Follow all logs
docker-compose logs -f

# Follow specific service logs
docker-compose logs -f auth-service

# Show last 100 lines
docker-compose logs --tail=100 auth-service
```

## 📄 License

This project is licensed under the MIT License.

## 📋 Quick Reference

### Essential Commands

```bash
# Start everything
docker-compose up -d

# Stop everything
docker-compose down

# View logs
docker-compose logs -f

# Build and run locally
make generate && make build

# Run tests
make test

# Check service health
curl http://localhost:8080/health  # Auth
curl http://localhost:8001/health  # Message
curl http://localhost:8002/health  # Presence
curl http://localhost:8003/health  # Chat API
curl http://localhost:8004/health  # Media
```

### Service URLs

| Service | URL | Credentials |
|---------|-----|-------------|
| Auth Service | http://localhost:8080 | - |
| Message Service | http://localhost:8001 | - |
| Presence Service | http://localhost:8002 | - |
| Chat API | http://localhost:8003 | - |
| Media Service | http://localhost:8004 | - |
| Keycloak Admin | http://localhost:8090 | admin/admin123 |
| EMQX Dashboard | http://localhost:18083 | admin/public |
| MinIO Console | http://localhost:9001 | minioadmin/minioadmin123 |

### Database Credentials

```bash
# PostgreSQL
Host: localhost:5432
Database: chat_db
Username: chat_user
Password: chat_password

# Redis
Host: localhost:6379
Password: (none)

# MQTT (EMQX)
Host: localhost:1883
Username: (varies by service)
Password: (varies by service)
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request