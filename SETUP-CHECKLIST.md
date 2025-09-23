# 🚀 Orbit Messenger Setup Checklist

Follow this step-by-step checklist to get your Orbit Messenger system running.

## ✅ Prerequisites Check

- [ ] **Docker Desktop** is installed and running
- [ ] **Go 1.21+** is installed (`go version`)
- [ ] **jq** is installed (`which jq`) ✅ Already installed
- [ ] **Terminal/Command Line** access

## 📋 Setup Steps

### Step 1: Build Services
```bash
# Run the build script
./scripts/build-services.sh
```

**Expected Output:**
```
🔨 Building Orbit Messenger Services...
Building auth-service...
✅ auth-service built successfully
Building message-service...
✅ message-service built successfully
...
🎉 All services built successfully!
```

### Step 2: Start Infrastructure Services

```bash
# Start Docker Desktop first, then run:
docker-compose -f docker-compose.dev.yml up -d
```

**Expected Output:**
```
Creating network "orbit-messenger-chat-app_default" with the default driver
Creating chat-postgres ... done
Creating chat-redis    ... done
Creating chat-emqx     ... done
Creating chat-minio    ... done
Creating chat-keycloak ... done
```

**Verify services are running:**
```bash
docker-compose -f docker-compose.dev.yml ps
```

### Step 3: Wait for Services to Initialize

**Check service logs to ensure they're ready:**
```bash
# Check all services
docker-compose -f docker-compose.dev.yml logs

# Or check individual services
docker-compose -f docker-compose.dev.yml logs postgres
docker-compose -f docker-compose.dev.yml logs keycloak
```

**Wait for these indicators:**
- [ ] PostgreSQL: "database system is ready to accept connections"
- [ ] Keycloak: "Started 590 of 885 services" or similar
- [ ] EMQX: "EMQX 5.3 is running now!"
- [ ] MinIO: "API: http://172.x.x.x:9000"
- [ ] Redis: "Ready to accept connections"

### Step 4: Start Application Services

**Open 5 separate terminal windows/tabs and run each service:**

**Terminal 1 - Auth Service (Port 8000):**
```bash
cd /Users/jayakumarn/Documents/personal/orbit-internal-communication/Orbit-Messenger-chat-app
./bin/auth-service -conf auth-service/configs/config.yaml
```

**Terminal 2 - Message Service (Port 8001):**
```bash
cd /Users/jayakumarn/Documents/personal/orbit-internal-communication/Orbit-Messenger-chat-app
./bin/message-service -conf message-service/configs/config.yaml
```

**Terminal 3 - Chat API (Port 8003):**
```bash
cd /Users/jayakumarn/Documents/personal/orbit-internal-communication/Orbit-Messenger-chat-app
./bin/chat-api -conf chat-api/configs/config.yaml
```

**Terminal 4 - Presence Service (Port 8002):**
```bash
cd /Users/jayakumarn/Documents/personal/orbit-internal-communication/Orbit-Messenger-chat-app
./bin/presence-service -conf presence-service/configs/config.yaml
```

**Terminal 5 - Media Service (Port 8004):**
```bash
cd /Users/jayakumarn/Documents/personal/orbit-internal-communication/Orbit-Messenger-chat-app
./bin/media-service -conf media-service/configs/config.yaml
```

### Step 5: Verify Services are Running

**Quick health check:**
```bash
curl http://localhost:8000 && echo " - Auth Service OK"
curl http://localhost:8001 && echo " - Message Service OK"  
curl http://localhost:8002 && echo " - Presence Service OK"
curl http://localhost:8003 && echo " - Chat API OK"
curl http://localhost:8004 && echo " - Media Service OK"
```

### Step 6: Run API Tests

```bash
# Run the comprehensive test suite
./scripts/quick-test.sh
```

**Expected Output:**
```
🧪 Quick API Test for Orbit Messenger
✅ Auth Service (port 8000) is running
✅ Chat Service (port 8003) is running
✅ Presence Service (port 8002) is running
✅ Media Service (port 8004) is running

Step 1: Testing Auth Service - User Registration
✅ User registered successfully!

Step 2: Testing Auth Service - Token Validation  
✅ Token validation successful!

...

🎉 All API tests passed successfully!
```

## 🌐 Access Points

Once everything is running, you can access:

### Application Services
- **Auth Service**: http://localhost:8000
- **Message Service**: http://localhost:8001
- **Presence Service**: http://localhost:8002
- **Chat API**: http://localhost:8003
- **Media Service**: http://localhost:8004

### Infrastructure Dashboards
- **EMQX Dashboard**: http://localhost:18083 
  - Username: `admin`
  - Password: `public`
- **MinIO Console**: http://localhost:9001
  - Username: `minioadmin`
  - Password: `minioadmin123`
- **Keycloak Admin**: http://localhost:8080
  - Username: `admin`
  - Password: `admin123`

### Database Connections
- **PostgreSQL**: `localhost:5432`
  - Database: `chat_db`
  - Username: `chat_user`
  - Password: `chat_password`
- **Redis**: `localhost:6379`

## 🧪 API Testing Examples

### 1. Register a User
```bash
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "password123",
    "display_name": "John Doe",
    "organization_name": "My Company"
  }'
```

### 2. Create a Conversation
```bash
# Use the token from registration
export TOKEN="your-jwt-token-here"
export USER_ID="your-user-id-here"
export ORG_ID="your-org-id-here"

curl -X POST http://localhost:8003/api/v1/conversations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d '{
    "type": "GROUP",
    "title": "Team Chat",
    "participant_ids": []
  }'
```

### 3. Send a Message
```bash
# Use conversation ID from previous step
export CONV_ID="your-conversation-id-here"

curl -X POST http://localhost:8003/api/v1/conversations/$CONV_ID/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d '{
    "content_type": "text/plain",
    "content": "Hello, team!",
    "dedupe_key": "msg-1"
  }'
```

## 🔧 Troubleshooting

### Services Won't Start
- [ ] Check if ports are already in use: `lsof -i :8000`
- [ ] Verify Docker is running: `docker ps`
- [ ] Check service logs for errors

### Database Connection Issues
```bash
# Test PostgreSQL
docker exec chat-postgres psql -U chat_user -d chat_db -c "SELECT 1;"

# Test Redis
docker exec chat-redis redis-cli ping
```

### Build Issues
```bash
# Clean and rebuild
go clean -cache
./scripts/build-services.sh
```

### Docker Issues
```bash
# Restart infrastructure
docker-compose -f docker-compose.dev.yml down
docker-compose -f docker-compose.dev.yml up -d
```

## 🛑 Stopping Services

### Stop Application Services
- Press `Ctrl+C` in each terminal running the Go services

### Stop Infrastructure Services
```bash
docker-compose -f docker-compose.dev.yml down
```

## ✅ Success Criteria

You'll know everything is working when:
- [ ] All 5 Go services start without errors
- [ ] All infrastructure services are running (`docker-compose ps`)
- [ ] The test script (`./scripts/quick-test.sh`) passes all tests
- [ ] You can access the web dashboards
- [ ] API calls return expected JSON responses

## 🎯 Next Steps

Once everything is running:
1. **Explore the APIs** using the provided examples
2. **Connect MQTT clients** for real-time messaging
3. **Build a frontend** using the REST APIs
4. **Monitor the system** using the dashboards
5. **Scale services** as needed for your use case
