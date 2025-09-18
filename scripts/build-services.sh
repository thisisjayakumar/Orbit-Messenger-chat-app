#!/bin/bash

# Build all Orbit Messenger services
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔨 Building Orbit Messenger Services...${NC}"

# Create bin directory if it doesn't exist
mkdir -p bin

# Services to build
services=("auth-service" "message-service" "chat-api" "presence-service" "media-service")

for service in "${services[@]}"; do
    echo -e "${YELLOW}Building $service...${NC}"
    
    cd "$service"
    
    # Tidy up dependencies
    go mod tidy
    
    # Build the service
    if go build -o "../bin/$service" "./cmd/$service"; then
        echo -e "${GREEN}✅ $service built successfully${NC}"
    else
        echo -e "${RED}❌ Failed to build $service${NC}"
        exit 1
    fi
    
    cd ..
done

echo -e "${GREEN}🎉 All services built successfully!${NC}"

echo -e "${BLUE}Built binaries:${NC}"
ls -la bin/

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Start Docker Desktop"
echo "2. Run: docker-compose -f docker-compose.dev.yml up -d"
echo "3. Start each service in separate terminals:"
echo "   ./bin/auth-service -conf auth-service/configs/config.yaml"
echo "   ./bin/message-service -conf message-service/configs/config.yaml"
echo "   ./bin/chat-api -conf chat-api/configs/config.yaml"
echo "   ./bin/presence-service -conf presence-service/configs/config.yaml"
echo "   ./bin/media-service -conf media-service/configs/config.yaml"
echo "4. Run tests: ./scripts/quick-test.sh"
