# Kafka Learning Project: Realtime Commerce Event Platform

A comprehensive Kafka learning platform that demonstrates event-driven architecture through a realtime commerce system. This project is designed to teach you Kafka concepts through hands-on experience with observable, interactive features.

## Project Goals

- Understand Kafka deeply by building a real event-driven system
- Learn partitions, offsets, replay, consumer groups, retention, scaling, retries, DLQ, ordering
- Compare Kafka vs RabbitMQ mindsets
- Build observable frontend + backend system to SEE how Kafka behaves

## Architecture

```
Frontend (React / Next.js)
        |
    API Gateway / Order Service
        |
       Kafka
        |
    ----|----|----|----
    |    |    |    |
Payment  Inventory  Analytics  Notification
Service  Service    Service    Service
    |    |    |    |
    ----------- Persistence Layer
         |
    PostgreSQL / Redis
```

## Tech Stack

**Backend:**
- Go 1.21
- kafka-go (Segmentio)
- PostgreSQL
- Redis
- Docker

**Frontend:**
- React 18
- TypeScript
- Tailwind CSS
- Vite
- axios

**Infrastructure:**
- Kafka
- Zookeeper
- Kafka UI
- Docker Compose

## Project Phases

### Phase 1: Basic Event Flow ✓
Understand producer/consumer basics.

- Create Order API
- Publish OrderCreated event
- Multiple services consume the same event independently

**Concepts:**
- Kafka topics
- Producers & Consumers
- Consumer groups
- Event schemas
- Event-driven decoupling

### Phase 2: Consumer Groups
Understand horizontal scaling through partitions.

- Deploy multiple consumer instances
- Observe partition reassignment
- See rebalancing in action

**Concepts:**
- Partition assignment
- Rebalancing
- Horizontal scaling
- Consumer group coordination

### Phase 3: Offsets
Understand Kafka's powerful offset concept.

- Display current offsets
- Show consumer lag
- Reset offset and replay events
- Observe offset commits

**Concepts:**
- Committed offsets
- Offset lag
- Pull model
- Consumer position tracking

### Phase 4: Retention + Replay
Experience event sourcing through Kafka.

- Set retention policy
- Replay all historical events
- Rebuild state from event log

**Concepts:**
- Message retention
- Event sourcing
- Kafka as source of truth
- Historical replay

### Phase 5: Partition + Ordering
Understand ordering guarantees and partition keys.

- Visualize partition distribution
- Set partition key based on user_id
- See ordering guarantees within partitions

**Concepts:**
- Partition keys
- Ordering guarantees
- Partition strategy
- Message routing

### Phase 6: Retry + DLQ
Learn production-grade error handling.

- Create retry topics (5s, 30s delays)
- Create dead letter queue (DLQ)
- Simulate failures
- Visualize retry flow

**Concepts:**
- Retry patterns
- Dead letter queues
- Exponential backoff
- Poison message handling

### Phase 7: Idempotency
Handle duplicate message processing gracefully.

- Simulate consumer crashes
- Implement idempotency checks
- Prevent double-processing

**Concepts:**
- At-least-once delivery
- Duplicate detection
- Idempotent consumers
- Exactly-once semantics (myth!)

### Phase 8: Stream Processing
Learn real-time data processing.

- Calculate orders per minute
- Calculate revenue per minute
- Track top selling products
- Build live analytics dashboard

**Concepts:**
- Stream processing
- Continuous aggregation
- Realtime pipelines
- Time windows

### Phase 9: CDC (Advanced)
Understand Change Data Capture.

- PostgreSQL + Debezium setup
- Auto-emit events from DB changes
- Event-driven database sync

**Concepts:**
- Change Data Capture
- Event-driven DB sync
- Data pipeline architecture

### Phase 10: Observability
Learn production debugging and monitoring.

- Consumer lag tracking
- Message throughput metrics
- Processing time analysis
- Partition metrics
- Prometheus + Grafana integration

**Concepts:**
- Observability
- Metrics collection
- Lag monitoring
- System health

## Getting Started

### Prerequisites

- Docker & Docker Compose
- Go 1.21+
- Node.js 18+
- Make (optional, for convenience)

### Quick Start

1. **Clone and navigate to project:**
```bash
cd /Users/dangthanhninh/Documents/NinhData/aws-learn/rabbitmq-kafka/kafka-project
```

2. **Start infrastructure (Kafka, PostgreSQL, Redis, etc.):**
```bash
docker-compose up -d
```

Wait for all services to be healthy:
```bash
docker-compose ps
```

3. **Install backend dependencies:**
```bash
cd backend
go mod download
go mod tidy
```

4. **Run backend server:**
```bash
# Using Makefile
make dev

# Or directly
DB_HOST=localhost DB_PORT=5432 DB_USER=kafka_user DB_PASSWORD=kafka_pass \
DB_NAME=commerce_db KAFKA_BROKER=localhost:9092 SERVER_PORT=8000 \
go run main.go
```

5. **In another terminal, install and run frontend:**
```bash
cd frontend
npm install
npm run dev
```

Frontend will be available at `http://localhost:3000`

6. **Monitor Kafka:**
Open Kafka UI: `http://localhost:8080`

## API Endpoints

### Orders
- `POST /api/orders` - Create a new order
- `GET /api/orders` - List all orders

### Health
- `GET /health` - Health check

## Example: Creating an Order

```bash
curl -X POST http://localhost:8000/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "product_id": "prod456",
    "quantity": 2,
    "price": 99.99
  }'
```

## File Structure

```
kafka-project/
├── docker-compose.yml        # Infrastructure setup
├── backend/
│   ├── main.go              # Entry point
│   ├── go.mod               # Dependencies
│   ├── Makefile            # Build commands
│   ├── Dockerfile          # Backend image
│   ├── config/             # Configuration
│   ├── models/             # Domain models
│   ├── database/           # DB initialization
│   ├── kafka/              # Kafka producer/consumer
│   ├── service/            # Consumer services
│   └── api/                # HTTP API handlers
└── frontend/
    ├── package.json        # Dependencies
    ├── vite.config.ts      # Build config
    ├── tsconfig.json       # TypeScript config
    ├── tailwind.config.js  # Tailwind setup
    ├── index.html          # Entry HTML
    └── src/
        ├── main.tsx        # React entry
        ├── App.tsx         # Main component
        └── pages/
            ├── OrderPage.tsx
            └── KafkaMonitor.tsx
```

## Useful Commands

### Backend
```bash
cd backend

# Development
make dev                    # Run with hot reload
make build                  # Build binary
make test                   # Run tests
make fmt                    # Format code

# Docker
make docker-build          # Build Docker image
make docker-up             # Start all services
make docker-down           # Stop all services
make docker-logs           # View logs
```

### Frontend
```bash
cd frontend

npm run dev                 # Development server
npm run build              # Build for production
npm run lint               # Run ESLint
npm run preview            # Preview production build
```

### Docker Compose
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down

# Remove volumes (WARNING: deletes data)
docker-compose down -v
```

## Monitoring

### Kafka UI
Access at `http://localhost:8080`
- View topics and partitions
- Monitor consumer groups
- Check message content
- Track offsets and lag

### PostgreSQL
```bash
docker-compose exec postgres psql -U kafka_user -d commerce_db

# List tables
\dt

# Query orders
SELECT * FROM orders;
```

### Redis
```bash
docker-compose exec redis redis-cli

# Check keys
KEYS *
```

## Key Learning Moments

1. **Create an order** and watch it flow through all consumer services in logs
2. **Open Kafka UI** and observe messages in the topic
3. **Stop the backend** and create orders, then restart to see replay
4. **Scale consumers** (Phase 2) and watch rebalancing
5. **Monitor lag** (Phase 3) as you create orders
6. **Trigger failures** to understand retry patterns (Phase 6)
7. **Build realtime dashboard** with stream processing (Phase 8)

## Troubleshooting

### Backend fails to connect to Kafka
```bash
# Check Kafka is running
docker-compose ps

# Check Kafka logs
docker-compose logs kafka

# Restart Kafka
docker-compose restart kafka
```

### Frontend can't connect to backend
```bash
# Backend must be running on port 8000
# Check CORS is properly configured
# Verify proxy in vite.config.ts
```

### Database connection errors
```bash
# Ensure PostgreSQL is running
docker-compose logs postgres

# Check credentials in backend/main.go
# Default: kafka_user / kafka_pass on commerce_db
```

## Next Steps

1. **Complete Phase 1** - Get comfortable with producer/consumer basics
2. **Experiment** - Try creating many orders, observe consumer lag
3. **Explore Kafka UI** - Get familiar with the Kafka topology
4. **Read** - Review the plan.txt for detailed learning objectives
5. **Implement** - Work through remaining phases incrementally

## Resources

- [Kafka Official Documentation](https://kafka.apache.org/documentation/)
- [kafka-go Documentation](https://pkg.go.dev/github.com/segmentio/kafka-go)
- [Kafka UI GitHub](https://github.com/provectus/kafka-ui)
- [Kafka Design Patterns](https://kafka.apache.org/documentation/#design)

## Phase Progression

To implement later phases, follow this pattern:

1. Add database schema migrations (if needed)
2. Implement new API endpoints
3. Create new consumer services
4. Add frontend pages to visualize new concepts
5. Update Docker Compose if new services needed
6. Document learnings

Each phase builds on the previous. Complete Phase 1 thoroughly before moving to Phase 2.

## Support

For issues or questions:
1. Check Kafka UI at `http://localhost:8080`
2. Review service logs with `docker-compose logs`
3. Verify all containers are running: `docker-compose ps`
4. Check the plan.txt for phase-specific learning goals

---

**Happy Learning! 🚀**

This project transforms Kafka from abstract concepts into tangible, observable systems.
