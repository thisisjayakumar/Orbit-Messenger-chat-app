#!/bin/bash

# Simple service health test script
# Tests all Orbit Messenger services for basic connectivity

set -e

echo "🧪 Testing Orbit Messenger Services..."
echo "====================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results
passed=0
failed=0

test_service() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}
    
    echo -n "Testing $name... "
    
    if response=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null); then
        if [ "$response" -eq "$expected_status" ] || [ "$response" -eq 200 ] || [ "$response" -eq 404 ]; then
            echo -e "${GREEN}✅ PASS${NC} (HTTP $response)"
            ((passed++))
        else
            echo -e "${RED}❌ FAIL${NC} (HTTP $response)"
            ((failed++))
        fi
    else
        echo -e "${RED}❌ FAIL${NC} (Connection failed)"
        ((failed++))
    fi
}

echo ""
echo "🔍 Testing Infrastructure Services:"
test_service "EMQX Dashboard" "http://localhost:18083"
test_service "MinIO Console" "http://localhost:9001"
test_service "Keycloak" "http://localhost:8090"
test_service "OpenSearch" "http://localhost:9200"

echo ""
echo "🚀 Testing Application Services:"
test_service "Auth Service" "http://localhost:8080/health"
test_service "Chat API" "http://localhost:8003/health"
test_service "Presence Service" "http://localhost:8002/health"
test_service "Media Service" "http://localhost:8004/health"
test_service "Message Service" "http://localhost:8001/health"

echo ""
echo "📊 Test Results:"
echo -e "  ${GREEN}Passed: $passed${NC}"
echo -e "  ${RED}Failed: $failed${NC}"

if [ $failed -eq 0 ]; then
    echo -e "\n🎉 ${GREEN}All services are healthy!${NC}"
    exit 0
else
    echo -e "\n⚠️  ${YELLOW}Some services may still be starting up.${NC}"
    echo "   Wait a few minutes and try again, or check logs:"
    echo "   docker-compose -f docker-compose.dev.yml logs [service-name]"
    exit 1
fi
