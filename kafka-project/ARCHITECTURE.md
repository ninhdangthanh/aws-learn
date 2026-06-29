# System Architecture

## High-Level Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           USER INTERFACE                             │
│                                                                       │
│  ┌──────────────────────────┐  ┌──────────────────────────────────┐ │
│  │   Order Creation Form    │  │    Kafka Monitoring Dashboard    │ │
│  │                          │  │                                  │ │
│  │  - User ID               │  │  - Topics & Partitions           │ │
│  │  - Product ID            │  │  - Consumer Groups               │ │
│  │  - Quantity              │  │  - Offsets & Lag                 │ │
│  │  - Price                 │  │  - Live Messages                 │ │
│  └──────────────────────────┘  └──────────────────────────────────┘ │
│           React Frontend (Port 3000)                                  │
└─────────────────────────────────────────────────────────────────────┘
                              │
                HTTP Requests │ (JSON)
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    API GATEWAY / ORDER SERVICE                       │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │              Go Backend (Port 8000)                          │  │
│  │                                                               │  │
│  │  POST /api/orders     →  Create order in PostgreSQL         │  │
│  │  GET /api/orders      →  Retrieve orders from DB            │  │
│  │  GET /health          →  Health check                       │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                          │                                            │
│                          │ Publish Event                              │
│                          ▼                                            │
│                    ┌───────────────┐                                 │
│                    │ Kafka Producer│                                 │
│                    └───────────────┘                                 │
└─────────────────────────────────────────────────────────────────────┘
                              │
                   Event: order.created
                              │
                              ▼
        ┌─────────────────────────────────────────────────────┐
        │                   KAFKA (Port 9092)                 │
        │                                                     │
        │  ┌───────────────────────────────────────────────┐ │
        │  │  Topic: orders  (Partition: 1)                │ │
        │  │                                               │ │
        │  │  Offset: 0 │ order1.created                  │ │
        │  │  Offset: 1 │ order2.created                  │ │
        │  │  Offset: 2 │ order3.created                  │ │
        │  │  ...       │                                 │ │
        │  └───────────────────────────────────────────────┘ │
        │                                                     │
        │  Zookeeper (Port 2181) - Coordination              │
        │  Kafka UI (Port 8080) - Web Monitoring             │
        └─────────────────────────────────────────────────────┘
                    │           │            │            │
          ┌─────────┘           │            │            └─────────┐
          │                     │            │                      │
          ▼                     ▼            ▼                      ▼
    ┌──────────────┐    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │   PAYMENT    │    │  INVENTORY   │  │  ANALYTICS   │  │ NOTIFICATION │
    │   SERVICE    │    │   SERVICE    │  │   SERVICE    │  │   SERVICE    │
    │              │    │              │  │              │  │              │
    │ Consumer Grp │    │ Consumer Grp │  │ Consumer Grp │  │ Consumer Grp │
    │ payment-svc  │    │ inventory-sv │  │ analytics-sv │  │notification  │
    │              │    │              │  │              │  │              │
    │ Processes:   │    │ Processes:   │  │ Processes:   │  │ Processes:   │
    │ - Payment    │    │ - Reserve    │  │ - Aggregate  │  │ - Send Email │
    │   processing │    │   stock      │  │   metrics    │  │ - Log events │
    │ - Verif      │    │ - Track qty  │  │ - Count      │  │              │
    │   fraud      │    │              │  │   orders     │  │              │
    │              │    │              │  │ - Revenue    │  │              │
    └──────────────┘    └──────────────┘  └──────────────┘  └──────────────┘
          │                   │                 │                    │
          └─────────────────┬─┴─────────────────┴────────────────────┘
                            │
                     PostgreSQL (Port 5432)
                     ┌─────────────────────┐
                     │   commerce_db       │
                     │                     │
                     │  ┌───────────────┐  │
                     │  │ orders table  │  │
                     │  │ - id (PK)     │  │
                     │  │ - user_id     │  │
                     │  │ - product_id  │  │
                     │  │ - quantity    │  │
                     │  │ - price       │  │
                     │  │ - status      │  │
                     │  │ - created_at  │  │
                     │  └───────────────┘  │
                     │                     │
                     │  ┌───────────────┐  │
                     │  │ events table  │  │
                     │  │ - event_id    │  │
                     │  │ - event_type  │  │
                     │  │ - data        │  │
                     │  │ - timestamp   │  │
                     │  └───────────────┘  │
                     │                     │
                     │  ┌───────────────┐  │
                     │  │ idempotency   │  │
                     │  │ - key         │  │
                     │  │ - value       │  │
                     │  └───────────────┘  │
                     └─────────────────────┘
                             │
                      Redis (Port 6379)
                      ┌───────────────┐
                      │  Cache Layer  │
                      │  (Optional)   │
                      └───────────────┘
```

## Data Flow: Creating an Order

```
Step 1: User Creates Order
┌─────────────────────────────────────────┐
│ Frontend Form                           │
│ user123 | prod456 | qty:2 | $99.99     │
└────────────────────┬────────────────────┘
                     │
                     │ HTTP POST
                     ▼
Step 2: API Receives Request
┌─────────────────────────────────────────┐
│ Backend: POST /api/orders               │
│ Generate Order ID: uuid-1234            │
└────────────────────┬────────────────────┘
                     │
                     │ Store
                     ▼
Step 3: Save to Database
┌─────────────────────────────────────────┐
│ PostgreSQL: INSERT into orders          │
│ - id: uuid-1234                         │
│ - user_id: user123                      │
│ - product_id: prod456                   │
│ - quantity: 2                           │
│ - price: 99.99                          │
│ - status: created                       │
└────────────────────┬────────────────────┘
                     │
                     │ Publish Event
                     ▼
Step 4: Publish to Kafka
┌─────────────────────────────────────────┐
│ Kafka Producer                          │
│ Topic: orders                           │
│ Key: uuid-1234 (partition key)          │
│ Value: {event_id, type, data, ...}     │
│ Offset: N+1                             │
└────────────────────┬────────────────────┘
                     │
        ┌────────────┼────────────┬────────────┐
        │            │            │            │
        ▼            ▼            ▼            ▼
Step 5: Consumers Read Event (Parallel & Independent)
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│ Payment Service  │ │Inventory Service │ │Analytics Service │ │Notification Svc  │
│                  │ │                  │ │                  │ │                  │
│ Consumer Group:  │ │ Consumer Group:  │ │ Consumer Group:  │ │ Consumer Group:  │
│ payment-service  │ │inventory-service │ │analytics-service │ │notification-svc  │
│                  │ │                  │ │                  │ │                  │
│ Reads offset N+1 │ │ Reads offset N+1 │ │ Reads offset N+1 │ │ Reads offset N+1 │
│ Process event    │ │ Process event    │ │ Process event    │ │ Process event    │
│ Commit offset    │ │ Commit offset    │ │ Commit offset    │ │ Commit offset    │
│ (Msg consumed)   │ │ (Msg consumed)   │ │ (Msg consumed)   │ │ (Msg consumed)   │
└──────────────────┘ └──────────────────┘ └──────────────────┘ └──────────────────┘
        │                    │                    │                    │
        │ Process            │ Process            │ Process            │ Process
        ▼                    ▼                    ▼                    ▼
    Payment OK          Inventory OK        Aggregate           Email Sent
    Fraud Check         Reserve Stock       Count: 1             Log Event
    
Step 6: Event Fully Processed
┌─────────────────────────────────────────┐
│ All services consumed the event         │
│ Message remains in Kafka (retention)    │
│ Can be replayed anytime                 │
│ Audit trail complete                    │
└─────────────────────────────────────────┘
```

## Consumer Group Coordination (Phase 2+)

```
Phase 1: Single Consumer Per Service
┌──────────────────────────────────────────────────┐
│ Topic: orders  (1 partition)                     │
│ ┌──────────────────────────────────────────────┐ │
│ │ Partition 0                                  │ │
│ │ ├─ Message 1 (offset 0)                     │ │
│ │ ├─ Message 2 (offset 1)  ◄─── Committed by  │ │
│ │ ├─ Message 3 (offset 2)       Consumer Grp  │ │
│ │ └─ Message 4 (offset 3)                     │ │
│ └──────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────┘

Phase 2: Multiple Consumers (Same Group)
┌──────────────────────────────────────────────────────────────┐
│ Topic: orders  (3 partitions)                                │
│ Consumer Group: payment-service                              │
│                                                              │
│ ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│ │ Partition 0  │  │ Partition 1  │  │ Partition 2  │       │
│ ├──────────────┤  ├──────────────┤  ├──────────────┤       │
│ │ Msg 1 ◄─────┐│  │ Msg 2 ◄─────┐│  │ Msg 3 ◄─────┐│       │
│ │ Msg 4        ││  │ Msg 5        ││  │ Msg 6        ││       │
│ │ Offset: 2    ││  │ Offset: 1    ││  │ Offset: 1    ││       │
│ └──────────────┘│  └──────────────┘│  └──────────────┘│       │
│        │            │                    │                    │
│        ▼            ▼                    ▼                    │
│    ┌─────────┐ ┌─────────┐         ┌─────────┐             │
│    │ Inst-1  │ │ Inst-2  │         │ Inst-3  │             │
│    │         │ │         │         │         │             │
│    │Consumes │ │Consumes │         │Consumes │             │
│    │ Part-0  │ │ Part-1  │         │ Part-2  │             │
│    └─────────┘ └─────────┘         └─────────┘             │
│                                                              │
│ Rebalancing: If Inst-1 dies, kafka redistributes:           │
│ → Inst-2 takes Part-0                                       │
│ → Inst-3 takes Part-1                                       │
│ → Inst-3 keeps Part-2                                       │
└──────────────────────────────────────────────────────────────┘
```

## Offset Tracking (Phase 3+)

```
Consumer Offset Management:

┌─────────────────────────────────────────────────────────────┐
│ Kafka Topic: orders (Partition 0)                           │
├─────────────────────────────────────────────────────────────┤
│ Offset: 0│Message 1 (created_at: 10:00:00)                 │
│ Offset: 1│Message 2 (created_at: 10:00:05)                 │
│ Offset: 2│Message 3 (created_at: 10:00:10)                 │
│ Offset: 3│Message 4 (created_at: 10:00:15)                 │
│ Offset: 4│Message 5 (created_at: 10:00:20)  ◄─ Latest      │
└─────────────────────────────────────────────────────────────┘

Consumer Group: payment-service

Instance 1 Status:
┌──────────────────────────────────────┐
│ Current Offset: 2                    │ ← Consumer position
│ Committed Offset: 2                  │ ← Last committed
│ High Water Mark: 4                   │ ← Latest message
│ Lag: 4 - 2 = 2                       │ ← Messages behind
└──────────────────────────────────────┘

Lag Timeline:
Time     │ Committed │ HWM │ Lag
─────────┼───────────┼─────┼────
10:00:00 │ 0         │ 0   │ 0
10:00:05 │ 1         │ 1   │ 0
10:00:10 │ 1         │ 2   │ 1
10:00:15 │ 1         │ 3   │ 2  ◄─ High lag!
10:00:20 │ 2         │ 4   │ 2
```

## Event Flow Through System

```
Order Created Event: {
  "event_id": "evt-abc123",
  "event_type": "order.created",
  "data": {
    "order_id": "ord-xyz789",
    "user_id": "user123",
    "product_id": "prod456",
    "quantity": 2,
    "price": 99.99,
    "timestamp": "2024-01-15T10:30:45Z"
  },
  "version": 1
}

Event Path Through System:
1. Produced to Kafka: orders topic, partition 0, offset N
2. Payment Service reads, processes, commits offset
3. Inventory Service reads, processes, commits offset
4. Analytics Service reads, aggregates, commits offset
5. Notification Service reads, processes, commits offset
6. Event stays in Kafka (retention window)
7. Can be replayed from any offset
8. Audit trail maintained in PostgreSQL

Key Insight: Event is published ONCE, consumed by MANY.
No direct service-to-service coupling!
```

## Database Schema

```
Orders Table:
┌────────────────────────────────────────┐
│ id (UUID, PK)                          │
│ user_id (VARCHAR)                      │
│ product_id (VARCHAR)                   │
│ quantity (INT)                         │
│ price (DECIMAL)                        │
│ status (VARCHAR): 'created'            │
│ created_at (TIMESTAMP)                 │
│ updated_at (TIMESTAMP)                 │
└────────────────────────────────────────┘

Events Table:
┌────────────────────────────────────────┐
│ event_id (UUID, PK)                    │
│ event_type (VARCHAR): 'order.created'  │
│ data (TEXT): JSON serialized           │
│ timestamp (TIMESTAMP)                  │
│ version (INT): 1                       │
├────────────────────────────────────────┤
│ Audit trail of all events              │
│ Immutable log of changes               │
└────────────────────────────────────────┘

Idempotency Table:
┌────────────────────────────────────────┐
│ key (VARCHAR, PK): event_id            │
│ value (TEXT): processing_result        │
│ created_at (TIMESTAMP)                 │
├────────────────────────────────────────┤
│ Prevents duplicate processing          │
│ Caches results for retries             │
└────────────────────────────────────────┘
```

## Service Communication Pattern

```
Traditional Microservices (Synchronous):
┌──────────┐      HTTP      ┌──────────┐      HTTP      ┌──────────┐
│  Order   ├─────────────────►│ Payment  ├─────────────────►│Inventory│
│ Service  │  Wait for resp   │ Service  │  Wait for resp   │ Service │
└──────────┘                  └──────────┘                  └──────────┘
  Problem: Cascading failures, tight coupling, slow

Event-Driven Architecture (This Project):
┌──────────┐                                              ┌──────────┐
│  Order   │──┐                                      ┌────►│ Payment  │
│ Service  │  │  Publish Event                      │     │ Service  │
└──────────┘  │  (Fire & Forget)      ┌──────────┐  │     └──────────┘
              └─────────────────────►  │  KAFKA   │──┤
                                       └──────────┘  │     ┌──────────┐
                                                     ├────►│Inventory │
                                                     │     │ Service  │
                                                     │     └──────────┘
                                                     │
                                                     └────►┌──────────┐
                                                           │Analytics │
                                                           │ Service  │
                                                           └──────────┘

Benefits: Loosely coupled, scalable, resilient, async, observable
```

## Deployment Topology

```
Docker Host (localhost)
├── Zookeeper (Port 2181)
├── Kafka Broker (Port 9092, 29092)
├── Kafka UI (Port 8080)
├── PostgreSQL (Port 5432)
├── Redis (Port 6379)
├── Backend Service (Port 8000)
│   ├── Order API
│   ├── Payment Consumer
│   ├── Inventory Consumer
│   ├── Analytics Consumer
│   └── Notification Consumer
└── Frontend Service (Port 3000)
    └── React SPA

All services on same Docker network (kafka-network)
Direct communication via service names (e.g., kafka:29092)
```

## Key Architectural Decisions

1. **Event-Driven:** Events are the source of truth, not direct RPC calls
2. **Kafka:** Persistent message log with replay capability
3. **Consumer Groups:** Each service independently processes all events
4. **Database per Service:** Each consumer can store its own data (if needed)
5. **Immutable Audit Trail:** All events stored in PostgreSQL
6. **Observable:** Kafka UI provides full visibility
7. **Scalable:** Add consumers independently without modifying producer

## Evolution Path (10 Phases)

```
Phase 1: Basic Flow     → Producer/Consumer pattern
Phase 2: Scaling        → Consumer groups, rebalancing
Phase 3: Offsets        → Lag tracking, commit management
Phase 4: Replay         → Event sourcing, retention
Phase 5: Ordering       → Partition keys
Phase 6: Reliability    → Retry, DLQ
Phase 7: Idempotency    → Duplicate handling
Phase 8: Aggregation    → Stream processing
Phase 9: CDC            → Database-driven events
Phase 10: Operations    → Monitoring, observability
```

---

This architecture demonstrates fundamental Kafka patterns used in production systems at companies like Uber, LinkedIn, Netflix, and Airbnb.
