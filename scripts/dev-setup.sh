#!/bin/bash

echo "🚀 Setting up Hybrid Chat System development environment..."

# Start infrastructure services
echo "📦 Starting infrastructure services..."
docker-compose -f docker-compose.dev.yml up -d

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 10

# Check if PostgreSQL is ready
until docker exec chat-postgres pg_isready -U chat_user -d chat_db; do
  echo "Waiting for PostgreSQL..."
  sleep 2
done

echo "✅ PostgreSQL is ready!"

# Check if Redis is ready
until docker exec chat-redis redis-cli ping; do
  echo "Waiting for Redis..."
  sleep 2
done

echo "✅ Redis is ready!"

# Check if EMQX is ready
until curl -f http://localhost:18083 > /dev/null 2>&1; do
  echo "Waiting for EMQX..."
  sleep 2
done

echo "✅ EMQX is ready!"

echo "🎉 Development environment is ready!"
echo ""
echo "📋 Service URLs:"
echo "  - PostgreSQL: localhost:5432"
echo "  - Redis: localhost:6379"
echo "  - EMQX MQTT: localhost:1883"
echo "  - EMQX WebSocket: localhost:8083"
echo "  - EMQX Dashboard: http://localhost:18083 (admin/public)"
echo "  - MinIO: http://localhost:9001 (minioadmin/minioadmin123)"
echo "  - OpenSearch: http://localhost:9200"