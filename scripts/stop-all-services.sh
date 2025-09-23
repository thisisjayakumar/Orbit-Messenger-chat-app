#!/bin/bash

# Orbit Messenger - Stop All Services Script
# This script stops all services gracefully

set -e

echo "🛑 Stopping Orbit Messenger Services..."
echo "====================================="

# Navigate to project root
cd "$(dirname "$0")/.."

# Stop all services
echo "📦 Stopping all containers..."
docker-compose -f docker-compose.dev.yml down

echo ""
echo "✅ All services have been stopped!"
echo ""
echo "💡 To remove all data (volumes), run:"
echo "   docker-compose -f docker-compose.dev.yml down -v"
echo ""
echo "💡 To remove all images as well, run:"
echo "   docker-compose -f docker-compose.dev.yml down -v --rmi all"
