# Orbit Messenger - Complete Setup Guide

## Prerequisites

1. **Docker Desktop** - Make sure it's installed and running
2. **Go 1.21+** - For building the services
3. **jq** - For JSON parsing in test scripts
   ```bash
   brew install jq  # On macOS
   ```

## Step-by-Step Setup

### 1. Start Docker Desktop
- Open Docker Desktop application
- Wait for it to fully start (Docker icon in menu bar should be stable)

### 2. Start Infrastructure Services

```bash
# Start all infrastructure services
docker-compose -f docker-compose.dev.yml up -d

# Check if services are running
docker-compose -f docker-compose.dev.yml ps
```

Expected services:
- `chat-postgres` (PostgreSQL) - Port 5432
- `chat-redis` (Redis) - Port 6379  
- `chat-emqx` (MQTT Broker) - Ports 1883, 8083, 18083
- `chat-minio` (Object Storage) - Ports 9000, 9001
- `chat-keycloak` (Identity Management) - Port 8080

### 3. Wait for Services to Initialize

```bash
# Check logs to ensure services are ready
docker-compose -f docker-compose.dev.yml logs -f

# Or check individual service logs
docker-compose -f docker-compose.dev.yml logs postgres
docker-compose -f docker-compose.dev.yml logs keycloak
```

**Wait for these indicators:**
- PostgreSQL: "database system is ready to accept connections"
- Keycloak: "Started 590 of 885 services"
- EMQX: "EMQX 5.3 is running now!"
- MinIO: "API: http://172.x.x.x:9000"

### 4. Build Go Services

```bash
# Build all services
cd auth-service && go mod tidy && go build -o ../bin/auth-service ./cmd/auth-service && cd ..
cd message-service && go mod tidy && go build -o ../bin/message-service ./cmd/message-service && cd ..
cd chat-api && go mod tidy && go build -o ../bin/chat-api ./cmd/chat-api && cd ..
cd presence-service && go mod tidy && go build -o ../bin/presence-service ./cmd/presence-service && cd ..
cd media-service && go mod tidy && go build -o ../bin/media-service ./cmd/media-service && cd ..
```

### 5. Start Application Services

Open 5 separate terminal windows/tabs and run each service:

**Terminal 1 - Auth Service:**
```bash
cd /Users/jayakumarn/Documents/personal/Orbit-Messenger-chat-app
./bin/auth-service -conf auth-service/configs/config.yaml
```

**Terminal 2 - Message Service:**
```bash
cd /Users/jayakumarn/Documents/personal/Orbit-Messenger-chat-app
./bin/message-service -conf message-service/configs/config.yaml
```

**Terminal 3 - Chat API:**
```bash
cd /Users/jayakumarn/Documents/personal/Orbit-Messenger-chat-app
./bin/chat-api -conf chat-api/configs/config.yaml
```

**Terminal 4 - Presence Service:**
```bash
cd /Users/jayakumarn/Documents/personal/Orbit-Messenger-chat-app
./bin/presence-service -conf presence-service/configs/config.yaml
```

**Terminal 5 - Media Service:**
```bash
cd /Users/jayakumarn/Documents/personal/Orbit-Messenger-chat-app
./bin/media-service -conf media-service/configs/config.yaml
```

### 6. Verify Services are Running

Check that all services respond:

```bash
# Check service health
curl http://localhost:8000/health || echo "Auth Service: Check logs"
curl http://localhost:8001/health || echo "Message Service: Check logs"  
curl http://localhost:8002/health || echo "Presence Service: Check logs"
curl http://localhost:8003/health || echo "Chat API: Check logs"
curl http://localhost:8004/health || echo "Media Service: Check logs"
```

## Testing the APIs

### 1. Test Auth Service

```bash
# Register a new user
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123",
    "display_name": "Test User",
    "organization_name": "Test Organization"
  }'
```

**Expected Response:**
```json
{
  "user": {
    "id": "uuid-here",
    "organization_id": "org-uuid-here",
    "email": "test@example.com",
    "display_name": "Test User",
    "created_at": "2023-..."
  },
  "token": "jwt-token-here"
}
```

**Save the token and user ID for next steps!**

### 2. Test Chat API

```bash
# Set these variables from the registration response
export TOKEN="your-jwt-token-here"
export USER_ID="your-user-id-here" 
export ORG_ID="your-org-id-here"

# Create a conversation
curl -X POST http://localhost:8003/api/v1/conversations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d '{
    "type": "GROUP",
    "title": "Test Conversation",
    "participant_ids": []
  }'
```

### 3. Test Presence Service

```bash
# Set user status
curl -X PUT http://localhost:8002/api/v1/presence/$USER_ID/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "status": "online",
    "custom_status": "Testing the system"
  }'

# Get user presence
curl -X GET http://localhost:8002/api/v1/presence/$USER_ID \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Test Media Service

```bash
# Initiate file upload
curl -X POST http://localhost:8004/api/v1/upload/initiate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -d '{
    "file_name": "test.txt",
    "content_type": "text/plain",
    "size": 100
  }'
```

## Automated Testing

Run the comprehensive test suite:

```bash
# Make sure all services are running first, then:
./scripts/test-all-services.sh
```

## Infrastructure Access

- **EMQX Dashboard**: http://localhost:18083 (admin/public)
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin123)  
- **Keycloak Admin**: http://localhost:8080 (admin/admin123)
- **PostgreSQL**: localhost:5432 (chat_user/chat_password/chat_db)
- **Redis**: localhost:6379

## Troubleshooting

### Service Won't Start
1. Check if the port is already in use: `lsof -i :8000`
2. Check service logs for error messages
3. Verify infrastructure services are running: `docker-compose ps`

### Database Connection Issues
```bash
# Test PostgreSQL connection
docker exec chat-postgres psql -U chat_user -d chat_db -c "SELECT 1;"
```

### MQTT Connection Issues
```bash
# Test EMQX connection
curl http://localhost:18083
```

### Build Issues
```bash
# Clean and rebuild
go clean -cache
go mod tidy
go build -v ./cmd/service-name
```

## Stopping Services

```bash
# Stop Go services (Ctrl+C in each terminal)
# Stop infrastructure
docker-compose -f docker-compose.dev.yml down
```

## Next Steps

Once everything is running:
1. Use the API endpoints to build your frontend
2. Connect MQTT clients for real-time messaging
3. Implement your business logic
4. Add monitoring and alerting
5. Deploy to production environment
