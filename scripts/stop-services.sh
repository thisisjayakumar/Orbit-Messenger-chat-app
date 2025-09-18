#!/bin/bash

# Orbit Messenger - Service Stop Script
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🛑 Stopping Orbit Messenger Services...${NC}"

# Stop Go services
if [ -f .service_pids ]; then
    echo -e "${YELLOW}Stopping Go services...${NC}"
    PIDS=$(cat .service_pids)
    for pid in $PIDS; do
        if kill -0 $pid 2>/dev/null; then
            echo "Stopping process $pid..."
            kill $pid
        fi
    done
    rm -f .service_pids
    echo -e "${GREEN}✅ Go services stopped${NC}"
else
    echo -e "${YELLOW}No service PIDs found, attempting to kill by name...${NC}"
    pkill -f "auth-service" || true
    pkill -f "message-service" || true
    pkill -f "presence-service" || true
    pkill -f "chat-api" || true
    pkill -f "media-service" || true
fi

# Stop infrastructure services
echo -e "${YELLOW}Stopping infrastructure services...${NC}"
docker-compose -f docker-compose.dev.yml down

echo -e "${GREEN}✅ All services stopped successfully!${NC}"
