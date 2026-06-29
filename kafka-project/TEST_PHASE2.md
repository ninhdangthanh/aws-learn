# Phase 2: Consumer Groups — Test Report

## Prerequisites

```bash
# 1. Start infrastructure
docker-compose up -d

# 2. Start backend with 3 consumer instances per service and 3 partitions
cd backend && CONSUMER_INSTANCES=3 KAFKA_PARTITIONS=3 go run main.go

# 3. Start frontend (separate terminal)
cd frontend && npm run dev
```

**Expected Startup Logs:**
```
Starting 3 consumer instances per service (Phase 2 - Consumer Groups)
Using 3 partitions for 'orders' topic
[PAYMENT SERVICE - payment-1] started listening to 'orders' topic
[PAYMENT SERVICE - payment-2] started listening to 'orders' topic
[PAYMENT SERVICE - payment-3] started listening to 'orders' topic
[INVENTORY SERVICE - inventory-1] started listening to 'orders' topic
...
(12 total consumer instances: 3 per service × 4 services)
```

---

## Test 1: Consumer Registration

**Objective:** Verify all consumer instances register with the registry on startup.

```bash
curl http://localhost:8000/api/consumer-groups | python3 -m json.tool
```

**Expected Response:**
```json
[
  {
    "group_id": "payment-service",
    "members": [
      { "group_id": "payment-service", "instance_id": "payment-1", "status": "active", ... },
      { "group_id": "payment-service", "instance_id": "payment-2", "status": "active", ... },
      { "group_id": "payment-service", "instance_id": "payment-3", "status": "active", ... }
    ],
    "state": "Stable"
  },
  {
    "group_id": "inventory-service",
    "members": [ ... 3 members ... ],
    "state": "Stable"
  },
  {
    "group_id": "analytics-service",
    "members": [ ... 3 members ... ],
    "state": "Stable"
  },
  {
    "group_id": "notification-service",
    "members": [ ... 3 members ... ],
    "state": "Stable"
  }
]
```

**Verification Checklist:**
- [ ] 4 consumer groups returned
- [ ] Each group has exactly 3 members
- [ ] All members have status "active"
- [ ] All groups have state "Stable"

---

## Test 2: Rebalance Events on Startup

**Objective:** Verify "joined" events are recorded for all instances.

```bash
curl "http://localhost:8000/api/rebalance-events?limit=20" | python3 -m json.tool
```

**Expected:**
- 12 "joined" events (3 instances × 4 services)
- Each event has `event_type: "joined"`
- Events ordered newest-first

**Verification Checklist:**
- [ ] At least 12 rebalance events returned
- [ ] All events have `event_type: "joined"`
- [ ] Each instance appears exactly once
- [ ] Timestamps are valid and recent

---

## Test 3: Partition Distribution — Load Balancing

**Objective:** Verify that messages are distributed across partitions and processed by different instances within the same consumer group.

```bash
# Create 10 orders to trigger partition distribution
for i in {1..10}; do
  curl -s -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"user-$(printf '%03d' $i)\",
      \"product_id\": \"product-$i\",
      \"quantity\": $i,
      \"price\": $((i * 50)).00
    }"
  echo ""
done
```

**Expected Backend Logs (look for partition distribution):**
```
[PAYMENT SERVICE - payment-1] Processing order on partition 0 offset X: ...
[PAYMENT SERVICE - payment-2] Processing order on partition 1 offset X: ...
[PAYMENT SERVICE - payment-3] Processing order on partition 2 offset X: ...
```

**Verification Checklist:**
- [ ] Orders are distributed across multiple partitions (0, 1, 2)
- [ ] Within each consumer group, different instances process different partitions
- [ ] No single instance processes ALL messages (load is shared)
- [ ] All 10 orders are processed by all 4 service groups

---

## Test 4: Consumer Groups API — After Processing

**Objective:** Verify the API reflects real message processing stats.

```bash
curl http://localhost:8000/api/consumer-groups | python3 -m json.tool
```

**Verification Checklist:**
- [ ] `messages_read` > 0 for instances that received messages
- [ ] `partitions` array is populated (shows which partitions each instance consumed from)
- [ ] `last_message` has a recent timestamp (not `0001-01-01T00:00:00Z`)
- [ ] Within each group, the sum of `messages_read` across all members = 10

---

## Test 5: Frontend — Consumer Groups Dashboard

**Objective:** Verify the Consumer Groups page displays real-time data.

1. Open http://localhost:3000
2. Click the **"👥 Consumer Groups"** tab

**Verification Checklist:**

### Summary Cards
- [ ] "Consumer Groups" shows 4
- [ ] "Total Instances" shows 12
- [ ] "Active Partitions" shows the number of unique partitions seen
- [ ] "Messages Processed" shows the total message count

### Partition Distribution
- [ ] Color-coded bars appear for each consumer group
- [ ] Each bar shows P0, P1, P2 with different colors
- [ ] Below each partition shows the instance that consumed from it

### Consumer Group Cards
- [ ] 4 expandable cards are visible
- [ ] Each shows "Stable" state badge in green
- [ ] Each shows "3 instances", partition count, and message count
- [ ] Click to expand shows a detail table with:
  - Instance IDs (e.g., payment-1, payment-2, payment-3)
  - Status badges (green "active")
  - Color-coded partition badges
  - Message count per instance
  - Last activity timestamp

### Rebalance Events
- [ ] Timeline shows "joined" events for all 12 instances
- [ ] Each event shows group/instance and timestamp
- [ ] Events have correct icons (→ for joined)

---

## Test 6: Auto-Refresh and Live Updates

**Objective:** Verify the dashboard updates in real-time.

1. Keep the Consumer Groups page open
2. In another terminal, create more orders:

```bash
for i in {11..20}; do
  curl -s -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"user-$(printf '%03d' $i)\",
      \"product_id\": \"product-$i\",
      \"quantity\": 1,
      \"price\": $((i * 25)).00
    }"
  sleep 0.5
done
```

**Verification Checklist:**
- [ ] "Messages Processed" counter increases in real-time
- [ ] Individual instance message counts update
- [ ] "● Live" button shows green (auto-refresh active)
- [ ] Click "○ Paused" to stop auto-refresh, counts stop updating
- [ ] Click "↻ Refresh" to manually refresh while paused

---

## Test 7: Partition Key Distribution

**Objective:** Verify that the same user_id consistently goes to the same partition (because orderID is used as the Kafka key).

```bash
# Create 5 orders — each will have a unique UUID key, so they may land on different partitions
for i in {1..5}; do
  curl -s -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d '{
      "user_id": "same-user",
      "product_id": "product-abc",
      "quantity": 1,
      "price": 10.00
    }'
  echo ""
done
```

**Note:** In Phase 1/2, the partition key is `orderID` (UUID), so each order goes to a potentially different partition. In Phase 5, this will be changed to `user_id` for ordering guarantees.

**Verification Checklist:**
- [ ] Orders land on different partitions (because each has a unique UUID key)
- [ ] Different instances within a group process different orders
- [ ] Consumer Groups dashboard partition badges update

---

## Test 8: Configurable Instance Count

**Objective:** Verify the `CONSUMER_INSTANCES` environment variable works.

1. Stop the backend (`Ctrl+C`)
2. Restart with a different instance count:

```bash
cd backend && CONSUMER_INSTANCES=1 go run main.go
```

**Expected:**
```
Starting 1 consumer instances per service (Phase 2 - Consumer Groups)
```

```bash
curl http://localhost:8000/api/consumer-groups | python3 -m json.tool
```

**Verification Checklist:**
- [ ] Only 1 instance per group (4 total)
- [ ] Each group shows 1 member
- [ ] Frontend updates to show 4 total instances

---

## Test 9: Configurable Partition Count

**Objective:** Verify the `KAFKA_PARTITIONS` environment variable works.

> **Note:** Kafka does not reduce partition count. You need to delete and recreate the topic, or use a new topic name to test with a different partition count.

```bash
# Stop backend, delete the topic via Kafka UI (localhost:8080), then restart:
cd backend && KAFKA_PARTITIONS=5 CONSUMER_INSTANCES=3 go run main.go
```

**Expected:**
```
Using 5 partitions for 'orders' topic
```

**Verification Checklist:**
- [ ] Backend logs show new partition count
- [ ] Topic created with specified partitions (verify in Kafka UI)

---

## Test 10: Graceful Shutdown — Consumer Unregistration

**Objective:** Verify consumers unregister cleanly and "left" events are recorded.

1. Note the current rebalance event count
2. Press `Ctrl+C` on the backend

**Expected Logs:**
```
Shutting down server...
[PAYMENT SERVICE - payment-1] stopped
[PAYMENT SERVICE - payment-2] stopped
[PAYMENT SERVICE - payment-3] stopped
[INVENTORY SERVICE - inventory-1] stopped
...
Server stopped
```

**Verification Checklist:**
- [ ] All 12 consumer instances log "stopped"
- [ ] No panics or errors
- [ ] Server exits cleanly

---

## Test 11: Kafka Monitor Page — Phase 2 Updates

**Objective:** Verify the Kafka Monitor page reflects Phase 2 data.

1. Click the **"📊 Kafka Monitor"** tab

**Verification Checklist:**
- [ ] Consumer Groups count shows 4 (from live API)
- [ ] Topics shows "orders" with "Partitions: 3"
- [ ] Consumer Groups table shows real group names and states
- [ ] Members column shows actual member count per group
- [ ] Messages column shows total messages per group
- [ ] Phase 2 note at the bottom mentions multiple instances

---

## Test 12: Stress Test — Bulk Orders

**Objective:** Verify system handles a burst of orders.

```bash
# Create 50 orders rapidly
for i in {1..50}; do
  curl -s -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{
      \"user_id\": \"stress-user-$((i % 10))\",
      \"product_id\": \"product-$((i % 5))\",
      \"quantity\": $((RANDOM % 5 + 1)),
      \"price\": $((RANDOM % 500 + 10)).99
    }" &
done
wait
echo "All 50 orders submitted"
```

**Verification Checklist:**
- [ ] All 50 orders created successfully
- [ ] Backend processes all events without errors
- [ ] Consumer Groups dashboard shows correct total message count
- [ ] Load is distributed across instances (check individual message counts)
- [ ] No data loss — `GET /api/orders` returns all 50+ orders

---

## Summary

| Test # | Description | Status |
|--------|------------|--------|
| 1 | Consumer Registration | ⬜ |
| 2 | Rebalance Events on Startup | ⬜ |
| 3 | Partition Distribution | ⬜ |
| 4 | Consumer Groups API After Processing | ⬜ |
| 5 | Frontend Consumer Groups Dashboard | ⬜ |
| 6 | Auto-Refresh and Live Updates | ⬜ |
| 7 | Partition Key Distribution | ⬜ |
| 8 | Configurable Instance Count | ⬜ |
| 9 | Configurable Partition Count | ⬜ |
| 10 | Graceful Shutdown | ⬜ |
| 11 | Kafka Monitor Updates | ⬜ |
| 12 | Stress Test | ⬜ |

**Overall Phase 2 Status:** ⬜ Pending

---

*Last updated: Phase 2 Test Report*
