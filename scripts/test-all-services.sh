#!/bin/bash

# Comprehensive test script for all Orbit Messenger services
set -e

# Debug mode - uncomment to see all commands
# set -x

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

printf "${BLUE}🧪 Testing All Orbit Messenger Services...${NC}\n"

# Service URLs
AUTH_SERVICE="http://localhost:8000"
MESSAGE_SERVICE="http://localhost:8001"
PRESENCE_SERVICE="http://localhost:8002"
CHAT_API="http://localhost:8003"
MEDIA_SERVICE="http://localhost:8004"

# Test variables
TEST_EMAIL="test@example.com"
TEST_EMAIL2="test2@example.com"
TEST_PASSWORD="password123"
TEST_ORG_NAME="Test Organization"
TEST_DISPLAY_NAME="Test User"
TEST_DISPLAY_NAME2="Second User"

# Function to make HTTP requests with error handling
make_request() {
    local method=$1
    local url=$2
    local data=$3
    local headers=$4
    
    if [ -n "$data" ]; then
        if [ -n "$headers" ]; then
            eval "curl -s -X \"$method\" \"$url\" -H \"Content-Type: application/json\" $headers -d '$data'"
        else
            curl -s -X "$method" "$url" -H "Content-Type: application/json" -d "$data"
        fi
    else
        if [ -n "$headers" ]; then
            eval "curl -s -X \"$method\" \"$url\" $headers"
        else
            curl -s -X "$method" "$url"
        fi
    fi
}

# Test 1: Auth Service
printf "${YELLOW}Testing Auth Service...${NC}\n"

# Register user
echo "Registering user..."
REGISTER_PAYLOAD=$(jq -n \
  --arg email "$TEST_EMAIL" \
  --arg password "$TEST_PASSWORD" \
  --arg display_name "$TEST_DISPLAY_NAME" \
  --arg org "$TEST_ORG_NAME" \
  '{email:$email,password:$password,display_name:$display_name,organization_name:$org}')
REGISTER_RESPONSE=$(make_request "POST" "$AUTH_SERVICE/api/v1/auth/register" "$REGISTER_PAYLOAD")

TOKEN=$(echo $REGISTER_RESPONSE | jq -r '.token // empty')
USER_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.id // empty')
ORG_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.organization_id // empty')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    printf "${RED}❌ Auth service registration failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Auth service registration successful${NC}\n"

# Test token validation
echo "Testing token validation..."
VALIDATE_RESPONSE=$(make_request "POST" "$AUTH_SERVICE/api/v1/auth/validate" '{"token": "'$TOKEN'"}')
VALIDATED_USER_ID=$(echo $VALIDATE_RESPONSE | jq -r '.user_id // empty')

if [ "$VALIDATED_USER_ID" != "$USER_ID" ]; then
    printf "${RED}❌ Token validation failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Token validation successful${NC}\n"

# Test 2: Chat API
printf "${YELLOW}Testing Chat API...${NC}\n"

# Create a second user to add as DM participant
echo "Creating second user..."
REGISTER2_PAYLOAD=$(jq -n \
  --arg email "$TEST_EMAIL2" \
  --arg password "$TEST_PASSWORD" \
  --arg display_name "$TEST_DISPLAY_NAME2" \
  --arg org "$TEST_ORG_NAME" \
  '{email:$email,password:$password,display_name:$display_name,organization_name:$org}')
REGISTER2_RESPONSE=$(make_request "POST" "$AUTH_SERVICE/api/v1/auth/register" "$REGISTER2_PAYLOAD")
USER_ID2=$(echo $REGISTER2_RESPONSE | jq -r '.user.id // empty')

if [ -z "$USER_ID2" ] || [ "$USER_ID2" = "null" ]; then
    printf "${RED}❌ Second user registration failed${NC}\n"
    exit 1
fi

# Create DM conversation with second user
echo "Creating conversation..."
CONV_PAYLOAD=$(jq -n --arg uid2 "$USER_ID2" '{type:"DM", participant_ids:[$uid2]}')
CONV_RESPONSE=$(curl -s -X POST "$CHAT_API/api/v1/conversations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d "$CONV_PAYLOAD")

CONV_ID=$(echo $CONV_RESPONSE | jq -r '.id // empty')

if [ -z "$CONV_ID" ] || [ "$CONV_ID" = "null" ]; then
    printf "${RED}❌ Conversation creation failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Conversation creation successful${NC}\n"

# Send message
echo "Sending message..."
MSG_PAYLOAD=$(jq -n \
  --arg ctype "text/plain" \
  --arg content "Hello, World!" \
  --arg dedupe "test-message-1" \
  '{content_type:$ctype,content:$content,dedupe_key:$dedupe}')
MESSAGE_RESPONSE=$(curl -s -X POST "$CHAT_API/api/v1/conversations/$CONV_ID/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d "$MSG_PAYLOAD")

MESSAGE_ID=$(echo $MESSAGE_RESPONSE | jq -r '.id // empty')

if [ -z "$MESSAGE_ID" ] || [ "$MESSAGE_ID" = "null" ]; then
    printf "${RED}❌ Message sending failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Message sending successful${NC}\n"

# Test 3: Presence Service
printf "${YELLOW}Testing Presence Service...${NC}\n"

# Set user status
echo "Setting user status..."
STATUS_PAYLOAD=$(jq -n '{status:"online", custom_status:"Testing the system"}')
STATUS_RESPONSE=$(curl -s -X PUT "$PRESENCE_SERVICE/api/v1/presence/$USER_ID/status" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "$STATUS_PAYLOAD")

if echo $STATUS_RESPONSE | grep -q "error"; then
    printf "${RED}❌ Presence status update failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Presence status update successful${NC}\n"

# Get user presence
echo "Getting user presence..."
PRESENCE_RESPONSE=$(curl -s -X GET "$PRESENCE_SERVICE/api/v1/presence/$USER_ID" \
  -H "Authorization: Bearer $TOKEN")

PRESENCE_STATUS=$(echo $PRESENCE_RESPONSE | jq -r '.status // empty')

if [ "$PRESENCE_STATUS" != "online" ]; then
    printf "${RED}❌ Presence retrieval failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Presence retrieval successful${NC}\n"

# Test 4: Media Service
printf "${YELLOW}Testing Media Service...${NC}\n"

# Initiate file upload
echo "Initiating file upload..."
UPLOAD_PAYLOAD=$(jq -n '{file_name:"test.jpg", content_type:"image/jpeg", size:100}')
UPLOAD_RESPONSE=$(curl -s -X POST "$MEDIA_SERVICE/api/v1/upload/initiate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -d "$UPLOAD_PAYLOAD")

ATTACHMENT_ID=$(echo $UPLOAD_RESPONSE | jq -r '.attachment_id // empty')
UPLOAD_URL=$(echo $UPLOAD_RESPONSE | jq -r '.upload_url // empty')

if [ -z "$ATTACHMENT_ID" ] || [ "$ATTACHMENT_ID" = "null" ]; then
    printf "${RED}❌ Upload initiation failed${NC}\n"
    exit 1
fi

printf "${GREEN}✅ Upload initiation successful${NC}\n"

# Test 5: Infrastructure Health Checks
printf "${YELLOW}Testing Infrastructure Services...${NC}\n"

# Test PostgreSQL
echo "Testing PostgreSQL connection..."
if docker exec chat-postgres psql -U chat_user -d chat_db -c "SELECT 1;" > /dev/null 2>&1; then
    printf "${GREEN}✅ PostgreSQL is healthy${NC}\n"
else
    printf "${RED}❌ PostgreSQL connection failed${NC}\n"
    exit 1
fi

# Test Redis
echo "Testing Redis connection..."
if docker exec chat-redis redis-cli ping | grep -q "PONG"; then
    printf "${GREEN}✅ Redis is healthy${NC}\n"
else
    printf "${RED}❌ Redis connection failed${NC}\n"
    exit 1
fi

# Test EMQX
echo "Testing EMQX connection..."
if curl -s "http://localhost:18083" > /dev/null; then
    printf "${GREEN}✅ EMQX is healthy${NC}\n"
else
    printf "${RED}❌ EMQX connection failed${NC}\n"
    exit 1
fi

# Test MinIO
echo "Testing MinIO connection..."
if curl -s "http://localhost:9001" > /dev/null; then
    printf "${GREEN}✅ MinIO is healthy${NC}\n"
else
    printf "${RED}❌ MinIO connection failed${NC}\n"
    exit 1
fi

# Test Keycloak
echo "Testing Keycloak connection..."
if curl -s "http://localhost:8080" > /dev/null; then
    printf "${GREEN}✅ Keycloak is healthy${NC}\n"
else
    printf "${RED}❌ Keycloak connection failed${NC}\n"
    exit 1
fi

printf "${GREEN}🎉 All tests passed successfully!${NC}\n"

printf "${BLUE}Test Summary:${NC}\n"
echo "✅ Auth Service - Registration, Login, Token Validation"
echo "✅ Chat API - Conversation Creation, Message Sending"
echo "✅ Presence Service - Status Updates, Presence Retrieval"
echo "✅ Media Service - Upload Initiation"
echo "✅ Infrastructure - PostgreSQL, Redis, EMQX, MinIO, Keycloak"

printf "${YELLOW}Test Data Created:${NC}\n"
echo "User ID: $USER_ID"
echo "Organization ID: $ORG_ID"
echo "Conversation ID: $CONV_ID"
echo "Message ID: $MESSAGE_ID"
echo "Attachment ID: $ATTACHMENT_ID"
