# Backend Interview Notes

Thư mục này là bộ notes ôn phỏng vấn Middle Backend theo CV hiện tại: Golang, Bun/TypeScript, PostgreSQL, MongoDB, Redis, RabbitMQ, gRPC, microservices, AWS và system design.

`readme.md` chỉ dùng để giới thiệu, định hướng plan và trỏ đến các file topic. Kiến thức chi tiết nằm trong từng file riêng để dễ ôn, dễ bổ sung và tránh bị biến thành một file quá dài.

---

## Plan chính

1. [Kế hoạch ôn Middle Backend Go - 3 tuần](middle-backend-interview-plan.md)
2. Tuần 1: đóng gói Redis clone và RAG thành project pitch/deep dive.
3. Tuần 2: Go/API/PostgreSQL/Redis và project deep dive.
4. Tuần 3: MongoDB, RabbitMQ, gRPC, microservice và system design.

---

## Core language

| File | Dùng để ôn |
|---|---|
| [Golang Core Interview Notes](golang-core-interview.md) | Interface, pointer/value receiver, goroutine, channel, context, memory, error handling, gRPC/RabbitMQ trong Go |
| [TypeScript Core Interview Notes](typescript-core-interview.md) | Bun/Node.js runtime, event loop, callback/promise/async-await, runtime validation, type narrowing, React TS practical |

---

## Database, cache và storage

| File | Dùng để ôn |
|---|---|
| [Database Middle Roadmap](database-middle-roadmap.md) | PostgreSQL/index/query plan/transaction/MVCC/partition/replication/connection pool/cache/migration |
| [MongoDB F&B Notes](mongo_fnb.md) | Document model, embed/reference, hot document, aggregation, index, sharding, transaction, TTL, change stream |
| [Production Backend Concepts](production-backend-concepts.md) | Các tình huống production dễ bị hỏi: pagination, idempotency, cache stampede, retry storm, online migration, read replica stale |

Concept nên nắm thêm trong nhóm này:

* PostgreSQL: `EXPLAIN ANALYZE`, composite index, lock wait/deadlock, MVCC, isolation level, read replica lag, online migration, connection pool exhaustion.
* Redis/cache: cache-aside, invalidation, TTL jitter, hot key, stampede, penetration, distributed lock và fencing token.
* MongoDB: schema governance, aggregation memory limit, unbounded array, document relocation, write concern, replica lag.

---

## Service communication, queue và API

| File | Dùng để ôn |
|---|---|
| [Backend Communication Roadmap](backend-communication-roadmap.md) | REST/gRPC boundary, RabbitMQ, retry/backoff, circuit breaker, DLQ, idempotent consumer, outbox, API versioning, WebSocket scale |
| [Notebook - Architecture](notebook.md) | Microservices, DDD, Saga, CQRS, EDA, circuit breaker, service discovery, API-led architecture, Kafka overview |

Concept nên nắm thêm trong nhóm này:

* RabbitMQ: exchange, routing key, queue binding, ack/nack, prefetch, retry, DLQ, poison message, at-least-once delivery, idempotent consumer.
* gRPC: protobuf field compatibility, unary/server-stream/client-stream/bidi-stream, deadline, status code, interceptor, REST vs gRPC trade-off.
* API production: rate limit nhiều dimension, idempotency key, request timeout, circuit breaker, graceful shutdown, API versioning, backward compatibility.

---

## System design và scale

| File | Dùng để ôn |
|---|---|
| [Scale System Questions](scale_system_question.md) | DNS, CDN, WAF, load balancer, API gateway, stateless app, autoscaling, cache, queue, database, sharding, observability |
| [Production Backend Concepts](production-backend-concepts.md) | Checklist failure mode theo từng topic để đưa vào system design answer |
| [Production Scale Metrics](production-scale-metrics.md) | Map số liệu production theo CV: users, CCU, RPS/TPS, latency, data volume, throughput và mức độ bạn trực tiếp chạm |

Khi luyện system design, luôn đi theo thứ tự:

1. Requirements và traffic estimate.
2. API và data model.
3. Read path, write path, async path.
4. Bottleneck và failure mode.
5. Trade-off và scale path.

---

## Project pitch/deep dive

| Project | Nên nhấn mạnh |
|---|---|
| Redis clone | RESP parser, command dispatch, TTL, AOF/RDB concept, Pub/Sub, concurrency, tests, phần còn thiếu so với Redis thật |
| RAG chatbot/backend-AI | Upload, ingestion worker, chunking, vector search, Qdrant, Postgres metadata, Redis worker/cache, citation, timeout/retry/failure mode |

Output nên có trước khi phỏng vấn:

* Pitch 60-90 giây cho mỗi project.
* 8-10 câu Q&A deep dive cho mỗi project.
* 2 câu chuyện STAR từ kinh nghiệm làm việc thật.
* 2 bài system design luyện trong 45 phút.
* 1 bảng production metrics đã điền số thật hoặc ghi rõ phần nào chưa sở hữu metric.

---

## Cách dùng notes hằng ngày

1. Mở [middle-backend-interview-plan.md](middle-backend-interview-plan.md) để biết hôm nay ôn gì.
2. Mở đúng file topic, đọc 60-80 phút.
3. Tự trả lời thành tiếng 30-40 phút.
4. Ghi lại câu hỏi chưa chắc vào `notes/middle-interview-qa.md`.
5. Chỉ làm mini demo khi một concept còn mơ hồ; không mở thêm project lớn trước phỏng vấn.
