# Orbit Messenger - Private Chat Application

A comprehensive, enterprise-grade private chat application built with Go microservices, featuring real-time messaging, secure authentication, and scalable architecture.

## 🏗️ Architecture Overview

Orbit Messenger follows a distributed microservices architecture with the following components:

### Backend Services

1. **Auth Service** (Port 8000)
   - User authentication and authorization
   - Keycloak OIDC integration
   - JWT token management
   - MQTT credentials generation

2. **Message Service** (Port 8001)
   - MQTT message subscription and processing
   - Message persistence to PostgreSQL
   - Idempotent message handling

3. **Chat API** (Port 8003)
   - REST API for chat operations
   - Conversation management
   - Message history retrieval
   - Participant management

4. **Presence Service** (Port 8002)
   - Real-time user status tracking
   - MQTT Last Will and Testament (LWT)
   - Redis-based presence caching
   - Device session management

5. **Media Service** (Port 8004)
   - File upload and download management
   - MinIO S3-compatible storage
   - Antivirus scanning integration
   - Thumbnail generation

### Infrastructure Components

- **PostgreSQL**: Primary database for persistent data
- **Redis**: Caching and presence data
- **EMQX**: MQTT broker for real-time messaging
- **MinIO**: S3-compatible object storage
- **Keycloak**: Identity and access management
- **OpenSearch**: Full-text search (planned)

## 🚀 Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.21+
- jq (for testing scripts)

### 1. Clone and Setup

```bash
git clone <repository-url>
cd Orbit-Messenger-chat-app
```

### 2. Start Infrastructure Services

```bash
# Start all infrastructure services
docker-compose -f docker-compose.dev.yml up -d

# Wait for services to be ready (check logs)
docker-compose -f docker-compose.dev.yml logs -f
```

### 3. Build and Start Application Services

```bash
# Use the provided startup script
./scripts/start-services.sh
```

### 4. Run Tests

```bash
# Test all services
./scripts/test-all-services.sh
```

### 5. Stop All Services

```bash
# Stop everything
./scripts/stop-services.sh
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

### Auth Service (Port 8000)

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

## 🛠️ Development

### Project Structure

```
├── auth-service/          # Authentication service
├── message-service/       # Message processing service
├── chat-api/             # Chat REST API
├── presence-service/     # Presence tracking service
├── media-service/        # File handling service
├── shared/               # Shared utilities and proto files
├── scripts/              # Deployment and testing scripts
├── deployments/          # Kubernetes and Docker configs
└── docs/                 # Documentation
```

### Building Services

Each service can be built independently:

```bash
cd auth-service
go mod tidy
go build -o ../bin/auth-service ./cmd/auth-service

# Repeat for other services
```

### Configuration

Each service has its own `configs/config.yaml` file with service-specific settings.

### Testing

- Unit tests: `go test ./...` in each service directory
- Integration tests: `./scripts/test-all-services.sh`
- Load testing: Use the provided test scripts with tools like `ab` or `wrk`

## 📈 Monitoring and Observability

### Health Checks

Each service exposes health check endpoints:
- `/health` - Basic health check
- `/metrics` - Prometheus metrics (if enabled)

### Logging

All services use structured logging with configurable levels.

### Infrastructure Monitoring

- **EMQX Dashboard**: http://localhost:18083 (admin/public)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin123)
- **Keycloak Admin**: http://localhost:8080 (admin/admin123)

## 🚢 Deployment

### Docker Deployment

```bash
# Build all services
docker-compose -f docker-compose.prod.yml build

# Deploy
docker-compose -f docker-compose.prod.yml up -d
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/k8s/
```

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

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🆘 Support

For support and questions:
- Create an issue in the repository
- Check the documentation in the `docs/` directory
- Review the API documentation

## 🔮 Roadmap

- [ ] GraphQL API support
- [ ] End-to-end encryption
- [ ] Mobile push notifications
- [ ] Advanced search with OpenSearch
- [ ] Message reactions and threads
- [ ] Voice and video calling
- [ ] Advanced admin dashboard
- [ ] Multi-language support
- [ ] Advanced analytics and reporting