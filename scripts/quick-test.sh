#!/bin/bash

# Quick API Test Script for Orbit Messenger
# Run this after all services are started

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧪 Quick API Test for Orbit Messenger${NC}"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo -e "${RED}❌ jq is required but not installed. Install with: brew install jq${NC}"
    exit 1
fi

# Service URLs
AUTH_SERVICE="http://localhost:8000"
CHAT_API="http://localhost:8003"
PRESENCE_SERVICE="http://localhost:8002"
MEDIA_SERVICE="http://localhost:8004"

echo -e "${YELLOW}Testing service availability...${NC}"

# Check if services are running
for service in "Auth:8000" "Chat:8003" "Presence:8002" "Media:8004"; do
    name=$(echo $service | cut -d: -f1)
    port=$(echo $service | cut -d: -f2)
    
    if curl -s "http://localhost:$port" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ $name Service (port $port) is running${NC}"
    else
        echo -e "${RED}❌ $name Service (port $port) is not responding${NC}"
        echo "Please make sure the service is started. Check the setup guide."
        exit 1
    fi
done

echo -e "\n${YELLOW}Step 1: Testing Auth Service - User Registration${NC}"

# Register a new user
REGISTER_RESPONSE=$(curl -s -X POST "$AUTH_SERVICE/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123",
    "display_name": "Test User",
    "organization_name": "Test Organization"
  }')

echo "Registration Response:"
echo "$REGISTER_RESPONSE" | jq '.'

# Extract values
TOKEN=$(echo $REGISTER_RESPONSE | jq -r '.token // empty')
USER_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.id // empty')
ORG_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.organization_id // empty')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo -e "${RED}❌ Registration failed - no token received${NC}"
    echo "Response: $REGISTER_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ User registered successfully!${NC}"
echo "Token: $TOKEN"
echo "User ID: $USER_ID"
echo "Organization ID: $ORG_ID"

echo -e "\n${YELLOW}Step 2: Testing Auth Service - Token Validation${NC}"

VALIDATE_RESPONSE=$(curl -s -X POST "$AUTH_SERVICE/api/v1/auth/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$TOKEN\"}")

echo "Validation Response:"
echo "$VALIDATE_RESPONSE" | jq '.'

VALIDATED_USER_ID=$(echo $VALIDATE_RESPONSE | jq -r '.user_id // empty')
if [ "$VALIDATED_USER_ID" != "$USER_ID" ]; then
    echo -e "${RED}❌ Token validation failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Token validation successful!${NC}"

echo -e "\n${YELLOW}Step 3: Testing Chat API - Create Conversation${NC}"

CONV_RESPONSE=$(curl -s -X POST "$CHAT_API/api/v1/conversations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d '{
    "type": "GROUP",
    "title": "Test Conversation",
    "participant_ids": []
  }')

echo "Conversation Response:"
echo "$CONV_RESPONSE" | jq '.'

CONV_ID=$(echo $CONV_RESPONSE | jq -r '.id // empty')
if [ -z "$CONV_ID" ] || [ "$CONV_ID" = "null" ]; then
    echo -e "${RED}❌ Conversation creation failed${NC}"
    echo "Response: $CONV_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ Conversation created successfully!${NC}"
echo "Conversation ID: $CONV_ID"

echo -e "\n${YELLOW}Step 4: Testing Chat API - Send Message${NC}"

MESSAGE_RESPONSE=$(curl -s -X POST "$CHAT_API/api/v1/conversations/$CONV_ID/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -H "X-Organization-ID: $ORG_ID" \
  -d '{
    "content_type": "text/plain",
    "content": "Hello, World! This is a test message.",
    "dedupe_key": "test-message-1"
  }')

echo "Message Response:"
echo "$MESSAGE_RESPONSE" | jq '.'

MESSAGE_ID=$(echo $MESSAGE_RESPONSE | jq -r '.id // empty')
if [ -z "$MESSAGE_ID" ] || [ "$MESSAGE_ID" = "null" ]; then
    echo -e "${RED}❌ Message sending failed${NC}"
    echo "Response: $MESSAGE_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ Message sent successfully!${NC}"
echo "Message ID: $MESSAGE_ID"

echo -e "\n${YELLOW}Step 5: Testing Presence Service - Set Status${NC}"

STATUS_RESPONSE=$(curl -s -X PUT "$PRESENCE_SERVICE/api/v1/presence/$USER_ID/status" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "status": "online",
    "custom_status": "Testing the Orbit Messenger system"
  }')

echo "Status Response:"
echo "$STATUS_RESPONSE" | jq '.'

if echo $STATUS_RESPONSE | grep -q "error"; then
    echo -e "${RED}❌ Presence status update failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Presence status updated successfully!${NC}"

echo -e "\n${YELLOW}Step 6: Testing Presence Service - Get Status${NC}"

PRESENCE_RESPONSE=$(curl -s -X GET "$PRESENCE_SERVICE/api/v1/presence/$USER_ID" \
  -H "Authorization: Bearer $TOKEN")

echo "Presence Response:"
echo "$PRESENCE_RESPONSE" | jq '.'

PRESENCE_STATUS=$(echo $PRESENCE_RESPONSE | jq -r '.status // empty')
if [ "$PRESENCE_STATUS" != "online" ]; then
    echo -e "${RED}❌ Presence retrieval failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Presence retrieved successfully!${NC}"

echo -e "\n${YELLOW}Step 7: Testing Media Service - Upload Initiation${NC}"

UPLOAD_RESPONSE=$(curl -s -X POST "$MEDIA_SERVICE/api/v1/upload/initiate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-User-ID: $USER_ID" \
  -d '{
    "file_name": "test-document.txt",
    "content_type": "text/plain",
    "size": 1024
  }')

echo "Upload Response:"
echo "$UPLOAD_RESPONSE" | jq '.'

ATTACHMENT_ID=$(echo $UPLOAD_RESPONSE | jq -r '.attachment_id // empty')
UPLOAD_URL=$(echo $UPLOAD_RESPONSE | jq -r '.upload_url // empty')

if [ -z "$ATTACHMENT_ID" ] || [ "$ATTACHMENT_ID" = "null" ]; then
    echo -e "${RED}❌ Upload initiation failed${NC}"
    echo "Response: $UPLOAD_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✅ Upload initiated successfully!${NC}"
echo "Attachment ID: $ATTACHMENT_ID"

echo -e "\n${GREEN}🎉 All API tests passed successfully!${NC}"

echo -e "\n${BLUE}📊 Test Summary:${NC}"
echo "✅ Auth Service - User Registration & Token Validation"
echo "✅ Chat API - Conversation Creation & Message Sending"
echo "✅ Presence Service - Status Management"
echo "✅ Media Service - File Upload Initiation"

echo -e "\n${BLUE}📋 Test Data Created:${NC}"
echo "User ID: $USER_ID"
echo "Organization ID: $ORG_ID"
echo "Conversation ID: $CONV_ID"
echo "Message ID: $MESSAGE_ID"
echo "Attachment ID: $ATTACHMENT_ID"
echo "JWT Token: $TOKEN"

echo -e "\n${YELLOW}💡 Next Steps:${NC}"
echo "1. Use these IDs to test more API endpoints"
echo "2. Connect an MQTT client to test real-time messaging"
echo "3. Access the web dashboards:"
echo "   - EMQX: http://localhost:18083 (admin/public)"
echo "   - MinIO: http://localhost:9001 (minioadmin/minioadmin123)"
echo "   - Keycloak: http://localhost:8080 (admin/admin123)"
