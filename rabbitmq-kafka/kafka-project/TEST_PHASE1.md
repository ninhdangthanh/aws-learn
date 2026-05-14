# Phase 1: Basic Event Flow — Test Report

## Prerequisites

```bash
# 1. Start infrastructure
docker-compose up -d

# 2. Verify all services are healthy
docker-compose ps
# Expected: zookeeper, kafka, kafka-ui, postgres, redis — all "Up (healthy)"

# 3. Start backend
cd backend && go run main.go

# 4. Start frontend (separate terminal)
cd frontend && npm run dev
```

---

## Test 1: Health Check

**Objective:** Verify backend server is running.

```bash
curl http://localhost:8000/health
```

**Expected Response:**
```json
{"status":"ok"}
```

| Status | Result |
|--------|--------|
| ✅ Pass | Backend responds with 200 OK |

---

## Test 2: Create a Single Order

**Objective:** Verify order creation, database persistence, and Kafka event publishing.

```bash
curl -X POST http://localhost:8000/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "product_id": "product-laptop",
    "quantity": 1,
    "price": 999.99
  }'
```

**Expected Response:**
```json
{
  "id": "<uuid>",
  "status": "created"
}
```

**Expected Backend Logs (4 services process the same event):**
```
[PAYMENT SERVICE - payment-1] Processing order on partition X offset Y: <order-data>
[PAYMENT SERVICE - payment-1] Payment processed successfully for order: <order-data>
[INVENTORY SERVICE - inventory-1] Processing order on partition X offset Y: <order-data>
[INVENTORY SERVICE - inventory-1] Inventory allocated for order: <order-data>
[ANALYTICS SERVICE - analytics-1] Order recorded on partition X offset Y. Total orders: 1
[NOTIFICATION SERVICE - notification-1] Sending notification for order on partition X offset Y: <order-data>
[NOTIFICATION SERVICE - notification-1] Email sent to customer for order: <order-data>
```

**Verification Checklist:**
- [ ] API returns 200 with order ID
- [ ] All 4 consumer services log the event
- [ ] Each service processes the event independently

---

## Test 3: Retrieve Orders

**Objective:** Verify orders are persisted in PostgreSQL and retrievable via API.

```bash
curl http://localhost:8000/api/orders
```

**Expected Response:**
```json
[
  {
    "id": "<uuid>",
    "user_id": "user-001",
    "product_id": "product-laptop",
    "quantity": 1,
    "price": 999.99,
    "status": "created",
    "created_at": "<timestamp>",
    "updated_at": "<timestamp>"
  }
]
```

**Verification Checklist:**
- [ ] Returns array of orders
- [ ] Order fields match what was submitted
- [ ] Status is "created"
- [ ] Timestamps are populated

---

## Test 4: Create Multiple Orders

**Objective:** Verify system handles multiple orders correctly and events are processed in sequence.

```bash
# Create 5 orders rapidly
for i in {1..5}; do
  curl -s -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"user-$(printf '%03d' $i)\",
      \"product_id\": \"product-$i\",
      \"quantity\": $i,
      \"price\": $((i * 100)).00
    }"
  echo ""
done
```

**Expected:**
- 5 successful responses with unique IDs
- 5 × 4 = 20 log lines (each order processed by 4 services)
- Analytics service count increases to 5

**Verification Checklist:**
- [ ] All 5 orders created successfully
- [ ] `GET /api/orders` returns all orders (including previous test)
- [ ] Backend logs show all 4 services processed each order
- [ ] No errors in logs

---

## Test 5: Frontend — Order Creation

**Objective:** Verify the frontend can create orders and display them.

1. Open http://localhost:3000
2. Fill in the "Create Order" form:
   - User ID: `user-web-001`
   - Product ID: `product-keyboard`
   - Quantity: `2`
   - Price: `79.99`
3. Click "Create Order"

**Verification Checklist:**
- [ ] Form submits without errors
- [ ] New order appears in the "Recent Orders" table within 2 seconds
- [ ] Order shows correct data (user_id, product_id, quantity, price)
- [ ] Status shows "created" with green badge

---

## Test 6: Frontend — Kafka Monitor

**Objective:** Verify the Kafka Monitor page shows cluster information.

1. Click "Kafka Monitor" tab

**Verification Checklist:**
- [ ] Shows Brokers: 1
- [ ] Shows Topics: 1
- [ ] Shows Consumer Groups count (4 for Phase 1 with 1 instance each)
- [ ] Topics section shows "orders" with Partitions: 3
- [ ] Consumer Groups table lists all 4 service groups

---

## Test 7: Kafka UI Integration

**Objective:** Verify events are visible in Kafka UI.

1. Open http://localhost:8080
2. Navigate to Topics → "orders"
3. Click "Messages"

**Verification Checklist:**
- [ ] "orders" topic exists
- [ ] Messages are visible with correct JSON payload
- [ ] Message keys are order UUIDs
- [ ] Messages distributed across partitions

---

## Test 8: Database Verification

**Objective:** Verify data integrity in PostgreSQL.

```bash
docker exec -it $(docker ps -q -f name=postgres) \
  psql -U kafka_user -d commerce_db -c "SELECT count(*) FROM orders;"
```

**Verification Checklist:**
- [ ] Count matches the number of orders created
- [ ] Tables (`orders`, `events`, `idempotency`) exist

```bash
# Verify tables exist
docker exec -it $(docker ps -q -f name=postgres) \
  psql -U kafka_user -d commerce_db -c "\dt"
```

---

## Test 9: Error Handling — Invalid Request

**Objective:** Verify API handles bad input gracefully.

```bash
# Missing required fields
curl -X POST http://localhost:8000/api/orders \
  -H "Content-Type: application/json" \
  -d '{"invalid": "data"}'

# Invalid JSON
curl -X POST http://localhost:8000/api/orders \
  -H "Content-Type: application/json" \
  -d 'not-json'
```

**Expected:**
- First request: Returns 200 but with empty/zero fields (no validation yet)
- Second request: Returns 400 "Invalid request"

**Verification Checklist:**
- [ ] Invalid JSON returns 400
- [ ] Server doesn't crash
- [ ] Consumers handle the empty data gracefully

---

## Test 10: Graceful Shutdown

**Objective:** Verify the backend shuts down cleanly.

1. Press `Ctrl+C` on the backend terminal

**Expected Logs:**
```
Shutting down server...
Server stopped
```

**Verification Checklist:**
- [ ] Server logs shutdown message
- [ ] No panic or error stacktraces
- [ ] Process exits with code 0

---

## Summary

| Test # | Description | Status |
|--------|------------|--------|
| 1 | Health Check | ⬜ |
| 2 | Create Single Order | ⬜ |
| 3 | Retrieve Orders | ⬜ |
| 4 | Create Multiple Orders | ⬜ |
| 5 | Frontend Order Creation | ⬜ |
| 6 | Frontend Kafka Monitor | ⬜ |
| 7 | Kafka UI Integration | ⬜ |
| 8 | Database Verification | ⬜ |
| 9 | Error Handling | ⬜ |
| 10 | Graceful Shutdown | ⬜ |

**Overall Phase 1 Status:** ⬜ Pending

---

*Last updated: Phase 1 Test Report*
