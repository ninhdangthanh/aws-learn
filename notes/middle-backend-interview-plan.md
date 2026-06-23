# Kế Hoạch Ôn Tập Middle Backend Go - 3 Tuần

## Mục tiêu và giới hạn

Kế hoạch này dành cho backend developer 2.5 YOE đang đi làm:

- **Thời gian:** 21 ngày, mỗi ngày 1.5-2 giờ (khoảng 10-14 giờ/tuần).
- **Tuần 1:** làm RAG MVP để có project mới để demo.
- **Tuần 2-3:** ôn lý thuyết theo interview question; chỉ làm demo 1-2 giờ khi cần làm rõ một concept.
- Không có mục tiêu học hết. Mục tiêu là trả lời chắc, có trade-off và liên hệ được với kinh nghiệm đang làm.

## Điểm xuất phát

- Redis clone đã là một project tốt để nói về RESP, TTL, AOF, Pub/Sub, dispatcher, test và concurrency.
- Notes hiện có đã phủ database và system design. Việc cần làm là rút thành câu hỏi/trả lời và tình huống production, không đọc lại từ đầu theo kiểu giáo trình.
- `backend-AI/RAG-chatbot-AI-BE` hiện chưa có source Go, chỉ có roadmap. Tuần 1 cần giới hạn rõ để không nổ scope.

## Nguyên tắc

- Mỗi ngày: 60-80 phút học/review + 30-40 phút tự trả lời thành tiếng và viết note ngắn.
- Mỗi topic chỉ cần trả lời được 4 ý: **là gì, dùng khi nào, rủi ro/trade-off, từng gặp/implement ở đâu**.
- Không tách microservice trong RAG. Làm **modular monolith + background worker** là đủ để thể hiện kỹ năng backend.
- MongoDB, RabbitMQ, gRPC học ở mức interview và làm micro-demo khi cần; Postgres, Redis, Go concurrency là ưu tiên cao nhất.

---

## Tuần 1 - Ship RAG MVP (7 ngày)

Checklist chi tiết nằm tại [Kế Hoạch RAG MVP](../backend-AI/RAG-chatbot-AI-BE/docs/mvp-7-day-plan.md). Scope dừng ở upload PDF, worker ingestion, Qdrant search và chat có citation.

---

## Tuần 2 - Go, API, PostgreSQL, Redis (7 ngày)

Mỗi ngày đọc/review notes 60-80 phút, sau đó 30-40 phút tự trả lời. Chỉ làm mini demo nếu không tự tin.

| Ngày | Chủ đề ôn | Câu hỏi cần trả lời được | Mini demo optional (<= 2 giờ) |
|---|---|---|---|
| 8 | Go core | interface, pointer/value receiver, `defer`, panic/recover, error wrapping | Không cần |
| 9 | Go concurrency | goroutine leak, channel close, mutex vs channel, race, `context` cancellation, `WaitGroup` | worker pool có timeout/cancel |
| 10 | Gin/API | middleware, validation, error shape, pagination, idempotency, timeout, graceful shutdown | Gin middleware request ID |
| 11 | Postgres index/query | B-tree, composite index, leftmost prefix, `EXPLAIN ANALYZE`, cursor vs offset | 1 query seed + compare plan |
| 12 | Transaction/concurrency | ACID, MVCC, isolation, lock wait, deadlock, optimistic vs pessimistic lock | inventory update tránh oversell |
| 13 | Redis | cache-aside, TTL, invalidation, stampede, hot key, Pub/Sub, AOF/RDB | cache-aside có TTL |
| 14 | Redis clone deep dive | RESP, command dispatch, TTL, persistence, Pub/Sub; Redis clone thiếu gì so với Redis thật? | chạy `go test -race ./...` và đọc 2 test |

### Kết thúc tuần 2

- Viết `notes/middle-interview-qa.md`: 35-40 câu hỏi, mỗi câu 3-6 dòng, theo 4 ý “what/when/trade-off/example”.
- Tự pitch Redis clone trong 90 giây.
- Chọn 2 tình huống ở công việc hiện tại để kể theo STAR: production bug, performance/DB issue, hoặc feature có trade-off.

---

## Tuần 3 - Database khác, queue, gRPC và System Design (7 ngày)

| Ngày | Chủ đề ôn | Câu hỏi cần trả lời được | Mini demo optional (<= 2 giờ) |
|---|---|---|---|
| 15 | MongoDB | document model vs relational, embed/reference, index, replica set, transaction, khi nào không dùng Mongo | schema cho catalog/menu |
| 16 | RabbitMQ | exchange, routing key, ack/nack, prefetch, retry, DLQ, at-least-once, idempotent consumer | producer/consumer + DLQ |
| 17 | gRPC | protobuf, unary vs streaming, deadline, status code, backward compatibility, gRPC vs REST | unary hello + deadline |
| 18 | Microservice | modular monolith vs microservice, service boundary, sync/async, outbox, Saga concept, observability | ADR 1 trang cho RAG |
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

- [ ] Demo được RAG end-to-end và giải thích tại sao dùng Gin, GORM, Postgres, Redis worker, Qdrant.
- [ ] Pitch Redis clone và RAG trong 60-90 giây; có thể deep dive 10 phút.
- [ ] Trả lời chắc Go concurrency, `context`, error handling, race detector.
- [ ] Đọc được `EXPLAIN ANALYZE`; giải thích index, transaction, MVCC, deadlock, cursor pagination.
- [ ] Giải thích cache-aside, cache invalidation/stampede, Redis TTL/AOF/PubSub.
- [ ] Giải thích RabbitMQ retry/DLQ/idempotency và gRPC deadline/protobuf compatibility.
- [ ] Làm được ít nhất 2 bài system design trong 45 phút, có failure mode và trade-off.

## Thứ tự dùng notes hiện có

1. `database-middle-roadmap.md`: ngày 11-12.
2. `readme.md`: lấy tình huống rate limit, pagination, idempotency, queue, cache cho ngày 10, 13, 19-20.
3. `scale_system_question.md`: ngày 19-21.
4. `articles.txt`: chỉ mở bài liên quan đến chủ đề của ngày đó; không research lan man.

## Sau 3 tuần

Bắt đầu apply và phỏng vấn song song với việc ôn. Nếu chưa tự tin ở một chủ đề, lặp lại **1 ngày** của chủ đề đó bằng cách làm 10 câu hỏi và 1 mini demo; không cần quay lại một roadmap dài. Vị trí Middle thường cần thấy bạn giao tiếp rõ ràng, biết debug và biết đánh đổi, không phải biết mọi công nghệ ở mức chuyên sâu.
