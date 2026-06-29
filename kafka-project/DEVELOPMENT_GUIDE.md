# Development Guide

This guide helps you develop and understand each phase of the Kafka Learning Project.

## Development Workflow

### Phase 1: Basic Event Flow (✓ COMPLETE)

**Objective:** Understand producer/consumer basics and event decoupling.

**What's implemented:**
- Order API (`POST /api/orders`)
- Kafka producer that publishes `order.created` events
- Multiple consumer services (Payment, Inventory, Analytics, Notification)
- Each consumer processes the same event independently

**How to experience Phase 1:**

1. **Start infrastructure:**
   ```bash
   docker-compose up -d
   ```

2. **Start backend (Terminal 1):**
   ```bash
   cd backend
   make dev
   ```

3. **Start frontend (Terminal 2):**
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

4. **Open Kafka UI (Terminal 3):**
   ```bash
   open http://localhost:8080
   ```

5. **Create some orders (Terminal 4):**
   ```bash
   # Create order 1
   curl -X POST http://localhost:8000/api/orders \
     -H "Content-Type: application/json" \
     -d '{"user_id": "user1", "product_id": "prod1", "quantity": 1, "price": 99.99}'
   
   # Create order 2
   curl -X POST http://localhost:8000/api/orders \
     -H "Content-Type: application/json" \
     -d '{"user_id": "user2", "product_id": "prod2", "quantity": 2, "price": 49.99}'
   ```

6. **Observe:**
   - In backend logs, you see all 4 consumers processing each order
   - In Kafka UI, you see messages in the `orders` topic
   - In frontend, orders appear in the table
   - Each service logs independently

**Key Insight:** One event can be consumed by MANY services independently. This is the core power of Kafka!

---

### Phase 2: Consumer Groups & Scaling

**Objective:** Understand horizontal scaling through multiple consumer instances.

**Prerequisites:** Phase 1 complete

**Implementation Plan:**

1. **Modify consumer services** to accept instance ID:
   ```go
   // In service/payment_service.go
   func NewPaymentService(brokers []string, instanceID string) *PaymentService {
       consumer := kafka.NewConsumer(brokers, "orders", "payment-service", 0)
       return &PaymentService{
           consumer: consumer,
           instanceID: instanceID,
       }
   }
   ```

2. **Update main.go** to launch multiple instances:
   ```go
   // Launch 3 payment service instances
   for i := 1; i <= 3; i++ {
       go func(id int) {
           svc := service.NewPaymentService(cfg.KafkaBrokers, fmt.Sprintf("payment-%d", id))
           svc.Start(ctx)
       }(i)
   }
   ```

3. **Add Partitions** to Kafka topic:
   ```bash
   # Update docker-compose to create topic with 3 partitions
   KAFKA_NUM_PARTITIONS: 3
   ```

4. **Frontend update:**
   - Add page showing consumer instances
   - Display partition assignment
   - Show rebalancing events

5. **Experiment:**
   - Create many orders, observe how they're balanced across partitions
   - Kill a consumer instance
   - Watch Kafka automatically rebalance
   - Restart the instance

**Key Insight:** Kafka automatically distributes partitions among consumers in a group. Each partition goes to exactly one consumer in the group.

---

### Phase 3: Offsets & Lag Tracking

**Objective:** Understand Kafka's core concept: the offset.

**Prerequisites:** Phase 1 complete

**Implementation Plan:**

1. **Add offset tracking API:**
   ```go
   // In api/offsets.go
   func (api *OffsetAPI) GetOffsets(w http.ResponseWriter, r *http.Request) {
       // Get current offsets for all consumer groups
       // Calculate lag = latest_offset - committed_offset
   }
   ```

2. **Update Kafka consumer** to expose offset information:
   ```go
   // In kafka/kafka.go
   func (c *Consumer) GetOffset(ctx context.Context) (int64, error) {
       // Return current consumer offset
   }
   ```

3. **Frontend page:**
   - Show current offset per partition
   - Show high water mark
   - Show lag (messages behind)
   - Real-time lag monitoring

4. **Experiment:**
   - Create many orders (push messages to Kafka)
   - Stop backend (consumer stops committing)
   - Watch lag increase in frontend
   - Restart backend and see lag decrease

**Key Insight:** Offset is the consumer's "bookmark". Kafka remembers where each consumer group is in each partition.

---

### Phase 4: Retention & Replay

**Objective:** Experience event sourcing: Kafka as immutable log.

**Prerequisites:** Phase 1 & 3 complete

**Implementation Plan:**

1. **Add replay API:**
   ```go
   // In api/orders.go
   func (api *OrderAPI) ReplayOrders(w http.ResponseWriter, r *http.Request) {
       // Reset consumer offset to beginning
       // Consumer re-processes all historical events
   }
   ```

2. **Update Analytics service** to rebuild statistics:
   ```go
   // When offset is reset, recalculate all metrics from scratch
   ```

3. **Frontend button:**
   - "Replay All Orders" button
   - Show analytics rebuild in real-time

4. **Set retention policy:**
   ```go
   // In Kafka config
   retention.ms: 86400000 // 24 hours
   ```

5. **Experiment:**
   - Create 10 orders
   - Reset analytics offset
   - Watch it recalculate from the beginning
   - Old orders are replayed

**Key Insight:** Messages stay in Kafka (based on retention). You can replay anytime. This enables event sourcing!

---

### Phase 5: Partitions & Ordering

**Objective:** Understand ordering guarantees and partition keys.

**Prerequisites:** Phase 1 & 2 complete

**Implementation Plan:**

1. **Add partition key to producer:**
   ```go
   // In kafka/kafka.go - use user_id as partition key
   err := p.writer.WriteMessages(ctx, kafka.Message{
       Topic: topic,
       Key:   []byte(userID),  // Partition by user
       Value: eventJSON,
   })
   ```

2. **Add partition visualization:**
   - Frontend page showing event distribution across partitions
   - Color-code by user_id
   - Show which events go to which partition

3. **Frontend tracking:**
   - Show order timeline per partition
   - Highlight ordering

4. **Experiment:**
   - Create orders for same user (should go same partition)
   - Create orders for different users (distributed)
   - Verify ordering: same user's orders are sequential

**Key Insight:** Ordering is ONLY guaranteed within a partition. Choose partition key carefully!

---

### Phase 6: Retry & DLQ

**Objective:** Learn production error handling patterns.

**Prerequisites:** Phase 1 complete

**Implementation Plan:**

1. **Create retry and DLQ topics:**
   ```go
   // In database initialization
   topics := []string{"orders", "orders.retry", "orders.dlq"}
   ```

2. **Add failure simulation:**
   ```go
   // In payment_service.go
   if rand.Float64() < 0.2 {  // 20% failure rate
       // Publish to retry topic instead
   }
   ```

3. **Implement retry consumer:**
   ```go
   // New service that reads retry topic
   // Waits for delay, retries
   // If fails again, publishes to retry.30s or DLQ
   ```

4. **DLQ dashboard:**
   - Frontend page showing failed messages
   - Ability to replay from DLQ

5. **Experiment:**
   - Create orders
   - Watch failures and retries
   - See messages eventually reach DLQ
   - Replay from DLQ

**Key Insight:** Kafka doesn't have built-in retry. You build it yourself with topics!

---

### Phase 7: Idempotency

**Objective:** Handle at-least-once delivery and prevent duplicates.

**Prerequisites:** Phase 1 & 6 complete

**Implementation Plan:**

1. **Add idempotency key tracking:**
   ```go
   // In database - idempotency table
   // key = event_id, value = processing result
   ```

2. **Check before processing:**
   ```go
   // Before processing event:
   // 1. Check if event_id exists in idempotency table
   // 2. If yes, return cached result
   // 3. If no, process and store result
   ```

3. **Simulate crashes:**
   ```go
   // Consumer crashes before committing offset
   // Same message re-delivered
   // But idempotency check prevents double-processing
   ```

4. **Frontend visualization:**
   - Track processing results
   - Show deduplicated vs duplicated events

5. **Experiment:**
   - Create order
   - Simulate consumer crash (kill terminal)
   - Restart consumer
   - Verify idempotent processing

**Key Insight:** Kafka guarantees at-least-once delivery. YOU must ensure exactly-once semantics through idempotency!

---

### Phase 8: Stream Processing

**Objective:** Real-time data processing and aggregation.

**Prerequisites:** Phase 1 complete

**Implementation Plan:**

1. **Create analytics service:**
   ```go
   type MetricsAggregator struct {
       ordersPerMinute    map[time.Time]int64
       revenuePerMinute   map[time.Time]float64
       topProducts        map[string]int64
   }
   ```

2. **Update analytics service:**
   ```go
   // Instead of just counting:
   // - Aggregate orders by minute
   // - Calculate revenue per minute
   // - Track top products
   // - Maintain sliding window
   ```

3. **Analytics API endpoint:**
   ```go
   // GET /api/analytics/metrics
   // Returns realtime aggregations
   ```

4. **Frontend live dashboard:**
   - Chart showing orders/minute
   - Chart showing revenue/minute
   - Top products list
   - Auto-refresh every second

5. **Experiment:**
   - Create orders steadily
   - Watch metrics update in real-time
   - Stop creating orders, watch metrics change

**Key Insight:** Kafka is perfect for continuous aggregation. Process stream as it flows!

---

### Phase 9: CDC (Advanced)

**Objective:** Automatic event generation from database changes.

**Prerequisites:** Phase 1 complete

**Implementation Plan:**

1. **Setup Debezium with PostgreSQL:**
   - Debezium connector (Docker service)
   - PostgreSQL logical replication setup

2. **Auto-emit events:**
   ```sql
   -- When order status changes:
   INSERT INTO orders SET status = 'completed'
   -- Debezium automatically publishes event to Kafka
   ```

3. **Consumers react:**
   - Notification service sees order.completed
   - Sends notification
   - No code change in order service!

4. **Frontend update:**
   - Show order status changes happening via CDC

5. **Experiment:**
   - Create order (emits order.created via API)
   - Update order status directly in DB
   - See Kafka automatically gets the event
   - Notification service reacts

**Key Insight:** CDC decouples database from services. Any status change becomes an event!

---

### Phase 10: Observability

**Objective:** Production-grade monitoring and debugging.

**Prerequisites:** All phases complete

**Implementation Plan:**

1. **Add metrics collection:**
   ```go
   // Prometheus metrics:
   // - consumer_lag
   // - messages_processed_total
   // - processing_duration_seconds
   // - retry_count
   // - dlq_count
   ```

2. **Export metrics:**
   ```go
   // POST /metrics in Prometheus format
   ```

3. **Setup Prometheus + Grafana:**
   ```yaml
   # docker-compose additions:
   prometheus:
       image: prom/prometheus
   
   grafana:
       image: grafana/grafana
   ```

4. **Grafana dashboards:**
   - Consumer lag over time
   - Throughput graph
   - Error rate
   - Partition distribution

5. **Frontend health page:**
   - Show system metrics
   - Alert on high lag

**Key Insight:** Observability is critical for Kafka in production. Know your lag, throughput, errors!

---

## Testing & Experimentation

### Common Experiments

**Test 1: Consumer Failure**
```bash
# Terminal 1: Run backend
# Terminal 2: Create order
# Terminal 3: Kill backend (Ctrl+C)
# Terminal 4: Create more orders
# Terminal 1: Restart backend
# Observe: Kafka remembers offset, replays messages
```

**Test 2: Throughput Test**
```bash
# Create 1000 orders rapidly
for i in {1..1000}; do
  curl -X POST http://localhost:8000/api/orders \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"user$i\", \"product_id\": \"prod$i\", \"quantity\": 1, \"price\": $(($RANDOM % 100))}"
done

# Observe consumer lag in Kafka UI
# Watch metrics update
```

**Test 3: Scale Consumers**
```bash
# In Phase 2, launch 5 payment instances
# Create orders
# Observe load balancing across instances
# Kill one instance, watch rebalancing
```

---

## Debugging Tips

1. **Check Kafka UI:** Always check `http://localhost:8080` first
   - Verify messages in topic
   - Check consumer group status
   - Monitor offsets and lag

2. **Check backend logs:**
   ```bash
   docker-compose logs backend -f
   ```

3. **Check PostgreSQL:**
   ```bash
   docker-compose exec postgres psql -U kafka_user -d commerce_db
   SELECT * FROM orders;
   SELECT * FROM events;
   SELECT * FROM idempotency;
   ```

4. **Check Kafka directly:**
   ```bash
   docker-compose exec kafka kafka-console-consumer.sh \
     --bootstrap-server localhost:9092 \
     --topic orders \
     --from-beginning
   ```

---

## Moving Forward

After completing all phases:

1. **Extend the system:**
   - Add more microservices
   - Implement more event types
   - Build more dashboards

2. **Production considerations:**
   - Schema versioning (Avro/Protocol Buffers)
   - Security (SSL/TLS)
   - High availability
   - Multi-region setup

3. **Real use cases:**
   - User activity tracking
   - Log aggregation
   - Real-time recommendations
   - Analytics pipelines

---

Happy learning! Each phase builds understanding. Take your time!
