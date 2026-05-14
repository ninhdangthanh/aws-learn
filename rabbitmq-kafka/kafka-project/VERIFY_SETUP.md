# Setup Verification Checklist

Use this checklist to verify the Kafka Learning Project is properly set up.

## Prerequisites

- [ ] Docker installed: `docker --version`
- [ ] Docker Compose installed: `docker-compose --version`
- [ ] Go 1.21+ installed: `go version` (optional for local dev)
- [ ] Node.js 18+ installed: `node --version` (optional for local frontend)
- [ ] Make installed: `make --version` (optional)

## Files & Folders

- [ ] `docker-compose.yml` exists and contains all services
- [ ] `backend/` folder has Go project structure
- [ ] `frontend/` folder has React project structure
- [ ] `backend/go.mod` exists with dependencies
- [ ] `backend/main.go` is the entry point
- [ ] `frontend/package.json` exists
- [ ] `frontend/src/main.tsx` is the entry point
- [ ] `README.md` is comprehensive
- [ ] `QUICKSTART.md` exists
- [ ] `DEVELOPMENT_GUIDE.md` exists
- [ ] `.env.example` exists

## Backend Verification

```bash
cd backend

# Check Go files exist
[ -f main.go ] && echo "✓ main.go found"
[ -f go.mod ] && echo "✓ go.mod found"
[ -f Dockerfile ] && echo "✓ Dockerfile found"
[ -f Makefile ] && echo "✓ Makefile found"

# Check packages
[ -d config ] && echo "✓ config package"
[ -d models ] && echo "✓ models package"
[ -d database ] && echo "✓ database package"
[ -d kafka ] && echo "✓ kafka package"
[ -d service ] && echo "✓ service package"
[ -d api ] && echo "✓ api package"

# Check dependencies
grep -q "segmentio/kafka-go" go.mod && echo "✓ kafka-go in go.mod"
grep -q "lib/pq" go.mod && echo "✓ pq (PostgreSQL) in go.mod"
grep -q "redis/go-redis" go.mod && echo "✓ redis client in go.mod"
```

## Frontend Verification

```bash
cd frontend

# Check files exist
[ -f package.json ] && echo "✓ package.json found"
[ -f index.html ] && echo "✓ index.html found"
[ -f tsconfig.json ] && echo "✓ tsconfig.json found"
[ -f vite.config.ts ] && echo "✓ vite.config.ts found"
[ -f tailwind.config.js ] && echo "✓ tailwind.config.js found"
[ -f Dockerfile ] && echo "✓ Dockerfile found"

# Check React files
[ -d src ] && echo "✓ src directory"
[ -f src/main.tsx ] && echo "✓ main.tsx"
[ -f src/App.tsx ] && echo "✓ App.tsx"
[ -d src/pages ] && echo "✓ pages directory"
```

## Infrastructure Verification

```bash
# Check Docker images available
docker images | grep -E "confluentinc/cp-kafka|confluentinc/cp-zookeeper" && echo "✓ Kafka images"
docker images | grep postgres && echo "✓ PostgreSQL image"
docker images | grep redis && echo "✓ Redis image"
docker images | grep kafka-ui && echo "✓ Kafka UI image"

# Start services
docker-compose up -d

# Wait for services
sleep 15

# Check all services running
docker-compose ps | grep -q "zookeeper.*Up" && echo "✓ Zookeeper running"
docker-compose ps | grep -q "kafka.*Up" && echo "✓ Kafka running"
docker-compose ps | grep -q "kafka-ui.*Up" && echo "✓ Kafka UI running"
docker-compose ps | grep -q "postgres.*Up" && echo "✓ PostgreSQL running"
docker-compose ps | grep -q "redis.*Up" && echo "✓ Redis running"

# Verify health
docker-compose exec kafka kafka-broker-api-versions.sh --bootstrap-server localhost:9092 && echo "✓ Kafka healthy"
docker-compose exec postgres pg_isready -U kafka_user && echo "✓ PostgreSQL healthy"
docker-compose exec redis redis-cli ping && echo "✓ Redis healthy"
```

## Backend Build Verification

```bash
cd backend

# Download dependencies
go mod download
go mod tidy

# Build
go build -o backend .

# Check binary
[ -f backend ] && echo "✓ Backend binary created"
./backend --help 2>&1 | head -1 && echo "✓ Binary executable"

# Clean up
rm backend
```

## Frontend Build Verification

```bash
cd frontend

# Install dependencies
npm install

# Check node_modules
[ -d node_modules ] && echo "✓ Dependencies installed"

# Build for production
npm run build

# Check dist folder
[ -d dist ] && echo "✓ Build successful"
[ -f dist/index.html ] && echo "✓ HTML bundle created"
```

## Runtime Verification

### Terminal 1: Start Backend
```bash
cd backend
make dev

# You should see:
# - "Connected to PostgreSQL successfully"
# - "Server starting on port 8000"
# - "Payment Service started listening..."
# - "Inventory Service started listening..."
# - "Analytics Service started listening..."
# - "Notification Service started listening..."
```

### Terminal 2: Start Frontend
```bash
cd frontend
npm run dev

# You should see:
# - "VITE v5.x.x  ready in xxx ms"
# - "➜  Local:   http://localhost:3000"
```

### Terminal 3: Test API
```bash
# Create order
curl -X POST http://localhost:8000/api/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test1","product_id":"prod1","quantity":1,"price":99.99}'

# Expected response: {"id":"<uuid>","status":"created"}

# Get orders
curl http://localhost:8000/api/orders

# Expected: JSON array of orders
```

### Terminal 4: Check Kafka
```bash
# Open browser
open http://localhost:8080

# You should see:
# - Kafka cluster info
# - orders topic with partitions
# - Consumer groups (payment-service, inventory-service, etc.)
# - Messages in topic
```

### Terminal 5: Check Frontend
```bash
# Open browser
open http://localhost:3000

# You should see:
# - Navigation tabs (Create Order, Kafka Monitor)
# - Order form
# - Recent orders table
```

## Data Verification

### PostgreSQL
```bash
docker-compose exec postgres psql -U kafka_user -d commerce_db

# Inside psql:
\dt  # List tables
SELECT * FROM orders;  # View orders
SELECT * FROM events;  # View events
SELECT COUNT(*) FROM orders;  # Count orders
```

### Kafka
```bash
docker-compose exec kafka kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic orders \
  --from-beginning \
  --max-messages 5
```

## Common Issues & Fixes

### Issue: Port already in use
```bash
# Find and kill process
lsof -i :8000  # Backend
lsof -i :3000  # Frontend
lsof -i :8080  # Kafka UI
lsof -i :9092  # Kafka
lsof -i :5432  # PostgreSQL

# Kill process (macOS)
kill -9 <PID>
```

### Issue: Kafka not connecting
```bash
# Restart Kafka
docker-compose restart kafka

# Wait for health check
sleep 10

# Verify
docker-compose logs kafka | tail -20
```

### Issue: PostgreSQL connection failed
```bash
# Verify container running
docker-compose ps postgres

# Check logs
docker-compose logs postgres

# Restart if needed
docker-compose restart postgres
```

### Issue: Frontend can't reach backend
```bash
# Verify backend running
curl http://localhost:8000/health

# Check vite proxy config
grep -A 5 "proxy:" frontend/vite.config.ts
```

## Full System Test

```bash
# 1. Start everything
docker-compose up -d
sleep 15

# 2. Start backend (terminal 1)
cd backend && make dev

# 3. Start frontend (terminal 2)  
cd frontend && npm run dev

# 4. Create 5 orders (terminal 3)
for i in {1..5}; do
  curl -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{\"user_id\":\"user$i\",\"product_id\":\"prod$i\",\"quantity\":$((i+1)),\"price\":$((i*10.00))}"
  sleep 1
done

# 5. Verify in backend logs: See all 4 consumers process each order

# 6. Check Kafka UI: http://localhost:8080
# - Should see 5 messages in orders topic

# 7. Check frontend: http://localhost:3000
# - Should see 5 orders in table

# 8. Check database
docker-compose exec postgres psql -U kafka_user -d commerce_db -c "SELECT COUNT(*) FROM orders;"
# Should return 5
```

## Documentation Verification

- [ ] README.md: Comprehensive overview ✓
- [ ] QUICKSTART.md: 5-minute setup guide ✓
- [ ] DEVELOPMENT_GUIDE.md: All phases detailed ✓
- [ ] PROJECT_STATUS.md: Detailed status tracking ✓
- [ ] QUICKSTART.md: Instructions present ✓

## Final Checklist

- [ ] All files exist as specified
- [ ] Docker Compose starts all services
- [ ] Backend compiles without errors
- [ ] Frontend dependencies install
- [ ] Backend starts and connects to services
- [ ] Frontend starts on port 3000
- [ ] API endpoints respond correctly
- [ ] Kafka UI shows topics and messages
- [ ] Orders are created and persisted
- [ ] All 4 consumers process events
- [ ] Logs show proper event flow
- [ ] Documentation is complete

---

## Next Steps After Verification

1. **Read QUICKSTART.md** for quick overview
2. **Test event flow** by creating multiple orders
3. **Observe Kafka UI** at localhost:8080
4. **Study backend logs** to understand flow
5. **Follow DEVELOPMENT_GUIDE.md** for Phase 2

---

**If all checks pass:** ✅ System is ready for learning!

**If any check fails:** Review the error logs and troubleshooting section above.
