# Quick Start Guide

Get the Kafka Learning Project running in 5 minutes!

## Prerequisites

- Docker & Docker Compose
- Go 1.21+ (optional, for local development)
- Node.js 18+ (optional, for local frontend development)

## Step 1: Start Infrastructure (1 minute)

```bash
cd /Users/dangthanhninh/Documents/NinhData/aws-learn/rabbitmq-kafka/kafka-project

# Start all services
docker-compose up -d

# Wait for services to be healthy
sleep 10
docker-compose ps
```

You should see all services with status "Up" or "healthy":
- zookeeper
- kafka
- kafka-ui
- postgres
- redis

## Step 2: Start Backend (1 minute)

```bash
# Terminal 1
cd backend
make dev
```

Backend will start on `http://localhost:8000`

You should see logs:
```
Connected to PostgreSQL successfully
Server starting on port 8000
Payment Service started listening to 'orders' topic
Inventory Service started listening to 'orders' topic
Analytics Service started listening to 'orders' topic
Notification Service started listening to 'orders' topic
```

## Step 3: Start Frontend (1 minute)

```bash
# Terminal 2
cd frontend
npm install
npm run dev
```

Frontend will start on `http://localhost:3000`

## Step 4: Open Applications

- **Frontend:** http://localhost:3000 - Create orders and monitor
- **Kafka UI:** http://localhost:8080 - Observe Kafka internals

## Step 5: Create Your First Order

### Option A: Using Frontend
1. Go to http://localhost:3000
2. Fill in the form:
   - User ID: `user123`
   - Product ID: `prod456`
   - Quantity: `2`
   - Price: `99.99`
3. Click "Create Order"

### Option B: Using cURL
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

## Step 6: Observe the Magic

Look at backend logs - you'll see:

```
[PAYMENT SERVICE] Processing order: <order_id>:user123:prod456
[PAYMENT SERVICE] Payment processed successfully
[INVENTORY SERVICE] Processing order: <order_id>:user123:prod456
[INVENTORY SERVICE] Inventory allocated for order: <order_id>:user123:prod456
[ANALYTICS SERVICE] Order recorded. Total orders: 1
[NOTIFICATION SERVICE] Sending notification for order: <order_id>:user123:prod456
[NOTIFICATION SERVICE] Email sent to customer for order: <order_id>:user123:prod456
```

Open Kafka UI at http://localhost:8080:
- Click "Topics" → "orders"
- You'll see the event message
- It shows the partition, offset, and timestamp

## Useful Commands

### View Backend Logs
```bash
docker-compose logs -f backend
```

### View All Logs
```bash
docker-compose logs -f
```

### Stop Everything
```bash
docker-compose down
```

### Start Again
```bash
docker-compose up -d
```

### Check Service Health
```bash
docker-compose ps
```

### Connect to PostgreSQL
```bash
docker-compose exec postgres psql -U kafka_user -d commerce_db
```

Inside PostgreSQL:
```sql
SELECT * FROM orders;
SELECT * FROM events;
```

### Connect to Redis
```bash
docker-compose exec redis redis-cli
```

## Common Issues

### Backend won't start
```bash
# Check Kafka is running
docker-compose logs kafka

# Restart Kafka
docker-compose restart kafka
```

### Frontend can't connect to backend
```bash
# Check backend is running
docker-compose logs backend

# Verify port 8000 is open
curl http://localhost:8000/health
```

### Port already in use
```bash
# Find process using port
lsof -i :8000        # Backend
lsof -i :3000        # Frontend
lsof -i :8080        # Kafka UI
lsof -i :9092        # Kafka
```

## Next Steps

1. **Read the comprehensive guide:**
   ```bash
   cat README.md
   ```

2. **Understand Phase 1:**
   ```bash
   cat DEVELOPMENT_GUIDE.md
   ```

3. **Create many orders** and observe:
   - Consumer logs
   - Kafka UI messages
   - Frontend order list
   - Database records

4. **Experiment:**
   - Stop backend and create orders (test offset)
   - Restart backend (see replay)
   - Check Kafka UI for partition distribution

5. **Work through phases:**
   - Phase 2: Consumer scaling
   - Phase 3: Offset tracking
   - Phase 4: Replay/retention
   - And more...

## Understanding What Just Happened

You just built and ran an **event-driven system**!

**Key insight:** When you created an order, it didn't call payment, inventory, analytics, and notification services directly. Instead:

1. **Order Service** (your API) published an `order.created` event to Kafka
2. **Payment Service** independently read the event
3. **Inventory Service** independently read the same event
4. **Analytics Service** independently read the same event
5. **Notification Service** independently read the same event

Each service didn't know about the others. They just consumed events. This is **event-driven architecture**!

---

**Need help?** See README.md and DEVELOPMENT_GUIDE.md for detailed information.

**Ready to learn more?** Move to Phase 2 by following DEVELOPMENT_GUIDE.md!
