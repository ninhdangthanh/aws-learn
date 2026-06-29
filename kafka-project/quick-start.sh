#!/bin/bash

set -e

echo "🚀 Kafka Learning Project - Quick Start"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check prerequisites
echo -e "${BLUE}Checking prerequisites...${NC}"

if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "⚠️  Go is not installed. Backend won't build locally."
fi

if ! command -v npm &> /dev/null; then
    echo "⚠️  npm is not installed. Frontend won't build locally."
fi

echo -e "${GREEN}✓ Prerequisites OK${NC}"
echo ""

# Start infrastructure
echo -e "${BLUE}Starting Kafka infrastructure...${NC}"
docker-compose up -d

echo -e "${GREEN}✓ Infrastructure started${NC}"
echo ""

# Wait for services
echo -e "${BLUE}Waiting for services to be healthy...${NC}"
sleep 10

# Check if services are up
echo -e "${BLUE}Checking service health...${NC}"

services=("zookeeper" "kafka" "postgres" "redis" "kafka-ui")
for service in "${services[@]}"; do
    status=$(docker-compose ps $service | grep -q "healthy\|Up" && echo "✓" || echo "✗")
    echo -e "${status} ${service}"
done

echo ""
echo -e "${GREEN}Setup complete!${NC}"
echo ""

echo -e "${YELLOW}Next steps:${NC}"
echo ""
echo "1. Backend (in separate terminal):"
echo "   cd backend && make dev"
echo ""
echo "2. Frontend (in another terminal):"
echo "   cd frontend && npm install && npm run dev"
echo ""
echo "3. Open in browser:"
echo "   - Frontend: http://localhost:3000"
echo "   - Kafka UI: http://localhost:8080"
echo ""
echo -e "${YELLOW}Services:${NC}"
echo "   - Backend API: http://localhost:8000"
echo "   - Frontend: http://localhost:3000"
echo "   - Kafka UI: http://localhost:8080"
echo "   - Kafka: localhost:9092"
echo "   - PostgreSQL: localhost:5432"
echo "   - Redis: localhost:6379"
echo ""
echo "📖 For detailed guide, see DEVELOPMENT_GUIDE.md"
echo ""
