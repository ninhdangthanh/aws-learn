# Kế Hoạch Ôn Tập Middle Backend Go - 3 Tuần

## Mục tiêu và giới hạn

Kế hoạch này dành cho backend developer 2.5 YOE đang đi làm:

- **Thời gian:** 21 ngày, mỗi ngày 1.5-2 giờ (khoảng 10-14 giờ/tuần).
- **Tuần 1:** đóng gói Redis clone và RAG thành câu chuyện phỏng vấn: pitch, architecture, trade-off, failure mode.
- **Tuần 2-3:** ôn lý thuyết theo interview question; chỉ làm mini demo 1-2 giờ khi cần làm rõ một concept.
- Không có mục tiêu học hết. Mục tiêu là trả lời chắc, có trade-off và liên hệ được với kinh nghiệm đang làm.

## Điểm xuất phát

- Redis clone đã hoàn thành và là project tốt để nói về RESP, TTL, AOF, Pub/Sub, dispatcher, test và concurrency.
- RAG chatbot/backend-AI đã hoàn thành ở mức có thể demo; dùng nó để nói về upload/ingestion worker, vector search, citation, Redis worker/cache, Postgres/Qdrant và API design.
- Notes hiện có đã phủ database và system design. Việc cần làm là rút thành câu hỏi/trả lời và tình huống production, không đọc lại từ đầu theo kiểu giáo trình.
- Việc còn thiếu không phải build thêm project, mà là chuẩn bị cách kể: vì sao thiết kế như vậy, trade-off là gì, nếu scale/fail thì xử lý ra sao.

## Nguyên tắc

- Mỗi ngày: 60-80 phút học/review + 30-40 phút tự trả lời thành tiếng và viết note ngắn.
- Mỗi topic chỉ cần trả lời được 4 ý: **là gì, dùng khi nào, rủi ro/trade-off, từng gặp/implement ở đâu**.
- Không mở thêm scope lớn cho RAG/Redis clone trước phỏng vấn. Chỉ sửa bug nhỏ, bổ sung README/diagram nếu giúp demo rõ hơn.
- MongoDB, RabbitMQ, gRPC học ở mức interview và làm micro-demo khi cần; Postgres, Redis, Go concurrency là ưu tiên cao nhất.

---

## Tuần 1 - Đóng gói project và core language (7 ngày)

Mục tiêu tuần 1 là biến Redis clone và RAG thành "interview assets": nói được trong 60-90 giây, deep dive được 10 phút, và liên hệ được với Go/TypeScript/backend production.

| Ngày | Chủ đề ôn | Câu hỏi cần trả lời được | Output |
|---|---|---|---|
| 1 | Redis clone pitch | Project giải quyết gì, vì sao build, scope gồm RESP/TTL/AOF/PubSub/dispatcher/test | Pitch 90 giây |
| 2 | Redis clone deep dive | TTL hoạt động ra sao, command dispatch thế nào, persistence thiếu gì so với Redis thật, concurrency/race ở đâu | 8-10 Q&A ngắn |
| 3 | RAG pitch | Upload/ingestion/search/chat flow, vì sao dùng Postgres/Redis/Qdrant, citation xử lý thế nào | Pitch 90 giây |
| 4 | RAG deep dive | Worker ingestion, chunking, retry, idempotency, timeout, lỗi Qdrant/LLM, observability | 8-10 Q&A ngắn |
| 5 | Go core | interface, pointer/value receiver, `defer`, panic/recover, error wrapping, context | Tự trả lời thành tiếng 30 phút |
| 6 | Go concurrency | goroutine leak, channel close, mutex vs channel, race, `WaitGroup`, worker pool | Chạy/test lại phần liên quan nếu có |
| 7 | TypeScript/Bun core | event loop, callback/promise/async-await, Bun vs Node.js, runtime validation, type narrowing | Tự trả lời thành tiếng 30 phút |

---

## Tuần 2 - API, PostgreSQL, Redis và project deep dive (7 ngày)

Mỗi ngày đọc/review notes 60-80 phút, sau đó 30-40 phút tự trả lời. Chỉ làm mini demo nếu không tự tin.

| Ngày | Chủ đề ôn | Câu hỏi cần trả lời được | Mini demo optional (<= 2 giờ) |
|---|---|---|---|
| 8 | Gin/API | middleware, validation, error shape, pagination, idempotency, timeout, graceful shutdown | Gin middleware request ID |
| 9 | HTTP/gRPC boundary | REST vs gRPC, protobuf compatibility, deadline, status code, interceptor/middleware | Không cần |
| 10 | Project API deep dive | API shape trong RAG/Redis clone, error handling, context timeout, graceful shutdown | Bổ sung README nếu thiếu |
| 11 | Postgres index/query | B-tree, composite index, leftmost prefix, `EXPLAIN ANALYZE`, cursor vs offset | 1 query seed + compare plan |
| 12 | Transaction/concurrency | ACID, MVCC, isolation, lock wait, deadlock, optimistic vs pessimistic lock | inventory update tránh oversell |
| 13 | Redis | cache-aside, TTL, invalidation, stampede, hot key, Pub/Sub, AOF/RDB | cache-aside có TTL |
| 14 | Mock project interview | Redis clone + RAG + Go concurrency + DB/Redis trade-off | Ghi âm 30-45 phút |

### Kết thúc tuần 2

- Viết `notes/middle-interview-qa.md`: 35-40 câu hỏi, mỗi câu 3-6 dòng, theo 4 ý “what/when/trade-off/example”.
- Tự pitch Redis clone và RAG trong 90 giây mỗi project.
- Chọn 2 tình huống ở công việc hiện tại để kể theo STAR: production bug, performance/DB issue, hoặc feature có trade-off.

---

## Tuần 3 - Database khác, queue, gRPC và System Design (7 ngày)

| Ngày | Chủ đề ôn | Câu hỏi cần trả lời được | Mini demo optional (<= 2 giờ) |
|---|---|---|---|
| 15 | MongoDB | document model vs relational, embed/reference, index, replica set, transaction, khi nào không dùng Mongo | schema cho catalog/menu |
| 16 | RabbitMQ | exchange, routing key, ack/nack, prefetch, retry, DLQ, at-least-once, idempotent consumer | producer/consumer + DLQ |
| 17 | gRPC | protobuf, unary vs streaming, deadline, status code, backward compatibility, gRPC vs REST | unary hello + deadline |
| 18 | Microservice | modular monolith vs microservice, service boundary, sync/async, outbox, Saga concept, observability | ADR 1 trang từ RAG đã làm |
| 19 | System design 1 | URL shortener hoặc notification system: requirements, API, data model, scale, cache, failure | vẽ 45 phút |
| 20 | System design 2 | flash sale/inventory: concurrency, DB lock, idempotency, queue, oversell, hot key | vẽ 45 phút |
| 21 | Mock interview | Go + DB/Redis + project deep dive + 1 system design | ghi âm 45-60 phút |

### System design template (dùng cho ngày 19-21)

1. Làm rõ functional/non-functional requirements và traffic estimate.
2. API + data model trước, sau đó mới vẽ components.
3. Nói read path, write path, cache, async flow.
4. Nêu bottleneck và failure: DB slow, duplicate message, cache miss/stampede, dependency timeout.
5. Kết bằng trade-off và scale path; không cần vẽ hệ thống 100M user ngay từ đầu.

---

## Checklist trước khi apply / phỏng vấn

- [x] RAG đã hoàn thành ở mức có thể demo end-to-end.
- [x] Redis clone đã hoàn thành ở mức có thể dùng làm project deep dive.
- [ ] Giải thích được RAG end-to-end: API, upload/ingestion, Redis worker/cache, Postgres, Qdrant, citation, failure mode.
- [ ] Pitch Redis clone và RAG trong 60-90 giây mỗi project; có thể deep dive 10 phút.
- [ ] Trả lời chắc Go concurrency, `context`, error handling, race detector.
- [ ] Đọc được `EXPLAIN ANALYZE`; giải thích index, transaction, MVCC, deadlock, cursor pagination.
- [ ] Giải thích cache-aside, cache invalidation/stampede, Redis TTL/AOF/PubSub.
- [ ] Giải thích RabbitMQ retry/DLQ/idempotency và gRPC deadline/protobuf compatibility.
- [ ] Làm được ít nhất 2 bài system design trong 45 phút, có failure mode và trade-off.

## Thứ tự dùng notes hiện có

1. `golang-core-interview.md`: ngày 5-6, ưu tiên interface, context, concurrency, memory, error handling, gRPC/RabbitMQ.
2. `typescript-core-interview.md`: ngày 7, ôn bổ sung cho Bun/Node.js/React TypeScript trong CV, ưu tiên event loop, async/callback/promise, runtime validation, type narrowing.
3. `database-middle-roadmap.md`: ngày 11-12.
4. `backend-communication-roadmap.md`: ngày 9, 16-18 cho REST/gRPC/RabbitMQ/retry/outbox/API versioning.
5. `production-backend-concepts.md`: lấy tình huống rate limit, pagination, idempotency, queue, cache cho ngày 10, 13, 19-20.
6. `scale_system_question.md`: ngày 19-21.
7. `articles.txt`: chỉ mở bài liên quan đến chủ đề của ngày đó; không research lan man.

## Sau 3 tuần

Bắt đầu apply và phỏng vấn song song với việc ôn. Nếu chưa tự tin ở một chủ đề, lặp lại **1 ngày** của chủ đề đó bằng cách làm 10 câu hỏi và 1 mini demo; không cần quay lại một roadmap dài. Vị trí Middle thường cần thấy bạn giao tiếp rõ ràng, biết debug và biết đánh đổi, không phải biết mọi công nghệ ở mức chuyên sâu.
