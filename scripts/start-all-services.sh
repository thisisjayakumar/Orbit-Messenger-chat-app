#!/bin/bash

# Orbit Messenger - Start All Services Script
# This script starts all services with one command using Docker Compose

set -e

echo "🚀 Starting Orbit Messenger Services..."
echo "=================================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Error: Docker is not running. Please start Docker first."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Error: docker-compose is not installed. Please install docker-compose first."
    exit 1
fi

# Navigate to project root
cd "$(dirname "$0")/.."

echo "📦 Building and starting all services..."
echo "This may take a few minutes on first run..."

# Start all services
docker-compose -f docker-compose.dev.yml up --build -d

echo ""
echo "✅ All services are starting up!"
echo ""
echo "🔗 Service URLs:"
echo "=================================="
echo "🔐 Auth Service:      http://localhost:8080"
echo "💬 Chat API:          http://localhost:8003"
echo "👥 Presence Service:  http://localhost:8002"
echo "📁 Media Service:     http://localhost:8004"
echo "📨 Message Service:   http://localhost:8001"
echo ""
echo "🛠️  Infrastructure:"
echo "=================================="
echo "🔑 Keycloak Admin:    http://localhost:8090 (admin/admin123)"
echo "📊 EMQX Dashboard:    http://localhost:18083 (admin/public)"
echo "🗄️  MinIO Console:     http://localhost:9001 (minioadmin/minioadmin123)"
echo "🔍 OpenSearch:        http://localhost:9200"
echo "🐘 PostgreSQL:        localhost:5432 (chat_user/chat_password)"
echo "🔴 Redis:             localhost:6379"
echo ""
echo "📡 MQTT WebSocket:    ws://localhost:8083/mqtt"
echo ""
echo "⏳ Services are starting... Waiting for health checks..."
echo ""

# Wait for key services to be ready
echo "🔍 Checking service health..."

# Check PostgreSQL
echo -n "  📊 PostgreSQL... "
for i in {1..30}; do
    if docker exec chat-postgres pg_isready -U chat_user -d chat_db >/dev/null 2>&1; then
        echo "✅"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ (timeout)"
    else
        sleep 2
    fi
done

# Check Redis
echo -n "  🔴 Redis... "
for i in {1..15}; do
    if docker exec chat-redis redis-cli ping >/dev/null 2>&1; then
        echo "✅"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "❌ (timeout)"
    else
        sleep 2
    fi
done

# Check EMQX
echo -n "  📡 EMQX... "
for i in {1..20}; do
    if curl -f http://localhost:18083 >/dev/null 2>&1; then
        echo "✅"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "❌ (timeout)"
    else
        sleep 3
    fi
done

# Check Auth Service
echo -n "  🔐 Auth Service... "
for i in {1..30}; do
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        echo "✅"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ (may still be starting)"
    else
        sleep 2
    fi
done

echo ""
echo "📋 Useful Commands:"
echo "  • Check status: docker-compose -f docker-compose.dev.yml ps"
echo "  • View logs: docker-compose -f docker-compose.dev.yml logs -f [service-name]"
echo "  • Stop all: ./scripts/stop-all-services.sh"
echo "  • Test services: curl http://localhost:8080/health"
echo ""
echo "🎉 Setup complete! Your Orbit Messenger backend is ready!"
