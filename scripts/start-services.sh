#!/bin/bash

# Orbit Messenger - Service Startup Script
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Starting Orbit Messenger Services...${NC}"

# Function to check if a service is running
check_service() {
    local service_name=$1
    local port=$2
    local max_attempts=30
    local attempt=1

    echo -e "${YELLOW}Waiting for $service_name to be ready on port $port...${NC}"
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "http://localhost:$port" > /dev/null 2>&1 || nc -z localhost $port 2>/dev/null; then
            echo -e "${GREEN}✅ $service_name is ready!${NC}"
            return 0
        fi
        
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo -e "${RED}❌ $service_name failed to start within timeout${NC}"
    return 1
}

# Start infrastructure services
echo -e "${YELLOW}Starting infrastructure services...${NC}"
docker-compose -f docker-compose.dev.yml up -d

# Wait for infrastructure services
check_service "PostgreSQL" 5432
check_service "Redis" 6379
check_service "EMQX" 1883
check_service "MinIO" 9000
check_service "Keycloak" 8080

echo -e "${GREEN}✅ Infrastructure services are ready!${NC}"

# Build Go services
echo -e "${YELLOW}Building Go services...${NC}"

# Build auth service
echo "Building auth-service..."
cd auth-service
go mod tidy
go build -o ../bin/auth-service ./cmd/auth-service
cd ..

# Build message service
echo "Building message-service..."
cd message-service
go mod tidy
go build -o ../bin/message-service ./cmd/message-service
cd ..

# Build chat-api
echo "Building chat-api..."
cd chat-api
go mod tidy
go build -o ../bin/chat-api ./cmd/chat-api
cd ..

# Build presence service
echo "Building presence-service..."
cd presence-service
go mod tidy
go build -o ../bin/presence-service ./cmd/presence-service
cd ..

# Build media service
echo "Building media-service..."
cd media-service
go mod tidy
go build -o ../bin/media-service ./cmd/media-service
cd ..

echo -e "${GREEN}✅ All services built successfully!${NC}"

# Start Go services in background
echo -e "${YELLOW}Starting Go services...${NC}"

# Start auth service
echo "Starting auth-service on port 8000..."
./bin/auth-service -conf auth-service/configs/config.yaml > logs/auth-service.log 2>&1 &
AUTH_PID=$!

# Start message service
echo "Starting message-service on port 8001..."
./bin/message-service -conf message-service/configs/config.yaml > logs/message-service.log 2>&1 &
MESSAGE_PID=$!

# Start presence service
echo "Starting presence-service on port 8002..."
./bin/presence-service -conf presence-service/configs/config.yaml > logs/presence-service.log 2>&1 &
PRESENCE_PID=$!

# Start chat API
echo "Starting chat-api on port 8003..."
./bin/chat-api -conf chat-api/configs/config.yaml > logs/chat-api.log 2>&1 &
CHAT_PID=$!

# Start media service
echo "Starting media-service on port 8004..."
./bin/media-service -conf media-service/configs/config.yaml > logs/media-service.log 2>&1 &
MEDIA_PID=$!

# Create logs directory if it doesn't exist
mkdir -p logs

# Wait for services to start
sleep 5

# Check if services are running
echo -e "${YELLOW}Checking service health...${NC}"

check_service "Auth Service" 8000
check_service "Message Service" 8001
check_service "Presence Service" 8002
check_service "Chat API" 8003
check_service "Media Service" 8004

echo -e "${GREEN}🎉 All services are running successfully!${NC}"

echo -e "${BLUE}Service URLs:${NC}"
echo "Auth Service:     http://localhost:8000"
echo "Message Service:  http://localhost:8001"
echo "Presence Service: http://localhost:8002"
echo "Chat API:         http://localhost:8003"
echo "Media Service:    http://localhost:8004"
echo ""
echo "Infrastructure:"
echo "PostgreSQL:       localhost:5432"
echo "Redis:            localhost:6379"
echo "EMQX Dashboard:   http://localhost:18083 (admin/public)"
echo "MinIO Console:    http://localhost:9001 (minioadmin/minioadmin123)"
echo "Keycloak:         http://localhost:8080 (admin/admin123)"

# Save PIDs for cleanup
echo "$AUTH_PID $MESSAGE_PID $PRESENCE_PID $CHAT_PID $MEDIA_PID" > .service_pids

echo -e "${YELLOW}To stop all services, run: ./scripts/stop-services.sh${NC}"
