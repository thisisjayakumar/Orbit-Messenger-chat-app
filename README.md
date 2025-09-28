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

- Docker and Docker Compose
- Go 1.21+ (for development)
- jq (for testing scripts)

### 1. Start All Services

```bash
# Start everything with one command (run from this directory)
docker-compose up -d

# Check service status
docker-compose ps
```

### 2. Verify Services

```bash
# Test all services
curl http://localhost:8080/health  # Auth Service
curl http://localhost:8001/health  # Message Service
curl http://localhost:8002/health  # Presence Service
curl http://localhost:8003/health  # Chat API
curl http://localhost:8004/health  # Media Service
```

### 3. Access Management Interfaces

- **Keycloak Admin**: http://localhost:8090 (admin/admin123)
- **EMQX Dashboard**: http://localhost:18083 (admin/public)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin123)

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

## 🛠️ Development

### Project Structure

```
├── auth-service/          # Authentication service
├── message-service/       # Message processing service
├── chat-api/             # Chat REST API
├── presence-service/     # Presence tracking service
├── media-service/        # File handling service
├── shared/               # Shared utilities and proto files
└── scripts/              # Database initialization scripts
```

### Building Services

Each service can be built independently:

```bash
cd auth-service
go mod tidy
go build -o bin/auth-service cmd/auth-service/main.go

# Repeat for other services
```

### Configuration

Each service has its own `configs/config.yaml` file with service-specific settings.

### Testing

- Unit tests: `go test ./...` in each service directory
- Integration tests: Use the provided test scripts
- Load testing: Use tools like `ab` or `wrk`

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

1. **Services not starting**: Check Docker logs with `docker-compose logs [service-name]`
2. **Database connection issues**: Verify PostgreSQL is running and accessible
3. **MQTT connection problems**: Check EMQX dashboard and network connectivity
4. **Authentication failures**: Verify Keycloak configuration and user setup

### Debug Commands

```bash
# Check service health
docker-compose ps

# View service logs
docker-compose logs -f auth-service

# Test database connection
docker exec orbit-postgres psql -U chat_user -d chat_db -c "SELECT 1;"

# Test Redis connection
docker exec orbit-redis redis-cli ping

# Test MQTT connection
docker exec orbit-emqx emqx_ctl status
```

## 📄 License

This project is licensed under the MIT License.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request