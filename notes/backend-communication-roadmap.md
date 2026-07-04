# Backend Communication Roadmap
## REST, gRPC, queue, retry, idempotency và schema evolution

File này là roadmap tổng quan cho service communication ở mức Middle Backend. Các topic đã có note riêng thì file này chỉ giữ khung tư duy và link để tránh trùng lặp.

Đọc sâu:

* [gRPC Middle Notes](grpc-middle-notes.md)
* [RabbitMQ Middle Notes](rabbitmq-middle-notes.md)
* [Redis Middle Notes](redis-middle-notes.md)
* [Production Backend Concepts](production-backend-concepts.md)

---

## 1. REST API production

Middle backend cần vượt qua mức "CRUD chạy được" và nói được production concerns:

* Request validation ở boundary.
* Error response shape thống nhất.
* Pagination: cursor vs offset.
* Idempotency key cho operation có side effect.
* Timeout cho request handler và downstream call.
* Graceful shutdown.
* Rate limit theo user/IP/API key/tenant/endpoint cost.
* API versioning/backward compatibility.

Error shape nên có:

```json
{
  "code": "ORDER_NOT_FOUND",
  "message": "order not found",
  "request_id": "req_123"
}
```

REST hợp:

* public API;
* browser/mobile direct;
* third-party integration;
* cần debug bằng curl/Postman dễ.

---

## 2. gRPC trong roadmap communication

gRPC hợp cho internal service communication vì dùng HTTP/2 và Protocol Buffers.

Ở mức roadmap, cần biết khi nào chọn gRPC:

* internal microservices cần contract rõ;
* latency/serialization quan trọng;
* cần streaming;
* team chấp nhận code generation và protobuf compatibility rule.

Những điểm phải trả lời chắc:

* unary vs server/client/bidirectional streaming;
* deadline/cancellation;
* status code;
* interceptor;
* protobuf backward compatibility;
* gRPC vs REST trade-off.

Chi tiết nằm ở [gRPC Middle Notes](grpc-middle-notes.md).

---

## 3. RabbitMQ trong roadmap communication

RabbitMQ hợp cho job async, task queue, decouple service và workflow cần ack/retry/DLQ rõ ràng.

Ở mức roadmap, cần biết khi nào dùng RabbitMQ:

* xử lý side effect async như email, notification, file processing;
* bảo vệ downstream bằng worker limit;
* cần retry/DLQ cho job lỗi;
* không cần Kafka-level event log/replay dài hạn.

Những điểm phải trả lời chắc:

* exchange, queue, binding, routing key;
* exchange type;
* ack/nack/reject;
* prefetch/backpressure;
* retry bằng TTL/DLX;
* DLQ/poison message;
* at-least-once và idempotent consumer.

Chi tiết nằm ở [RabbitMQ Middle Notes](rabbitmq-middle-notes.md).

---

## 4. Outbox pattern

Outbox giải quyết vấn đề DB và broker không nằm trong cùng transaction.

Vấn đề:

```text
DB commit success -> publish event fail
```

hoặc:

```text
publish event success -> DB rollback
```

Pattern:

1. Trong cùng DB transaction, update business table và insert row vào `outbox_events`.
2. Worker đọc outbox chưa publish.
3. Publish message tới broker với publisher confirm nếu có.
4. Mark published hoặc retry.

Trade-off:

* Tăng độ phức tạp.
* Cần cleanup outbox.
* Eventual consistency.
* Đổi lại tránh mất event khi DB và broker không chung transaction.

---

## 5. Retry, timeout và circuit breaker

Retry chỉ an toàn khi operation idempotent hoặc có idempotency key.

Checklist:

* timeout cho mọi downstream call;
* retry có max attempts;
* exponential backoff;
* jitter;
* circuit breaker khi dependency lỗi liên tục;
* timeout budget toàn request.

Không retry:

* validation error;
* authorization error;
* non-idempotent payment/order operation nếu thiếu idempotency key.

### Circuit breaker là gì?

Circuit breaker bảo vệ service khỏi việc tiếp tục gọi một dependency đang lỗi/chậm liên tục. Thay vì để request dồn vào dependency rồi làm cạn connection pool, goroutine/thread, memory hoặc timeout budget, circuit breaker sẽ tạm thời "ngắt mạch" và fail fast.

Dependency có thể là:

* payment gateway;
* service nội bộ qua HTTP/gRPC;
* LLM/embedding provider;
* search service như Elasticsearch/Qdrant;
* Redis/DB trong một số case cần degrade có kiểm soát.

### State machine

| State | Ý nghĩa | Hành vi |
|---|---|---|
| Closed | Dependency đang bình thường | Cho request đi qua, ghi nhận success/failure |
| Open | Dependency lỗi vượt ngưỡng | Chặn request, trả fallback/error nhanh |
| Half-open | Thử phục hồi sau cooldown | Cho một lượng nhỏ request đi qua để kiểm tra |

Luồng cơ bản:

```text
Closed
-> failure rate/timeout vượt ngưỡng
-> Open
-> chờ cooldown
-> Half-open
-> nếu request thử thành công: Closed
-> nếu request thử thất bại: Open
```

### Circuit breaker vs retry vs rate limit

| Pattern | Bảo vệ khỏi gì? | Cách hoạt động |
|---|---|---|
| Retry | Lỗi tạm thời ngắn hạn | Gọi lại có giới hạn |
| Timeout | Request treo quá lâu | Cắt request sau deadline |
| Circuit breaker | Dependency lỗi/chậm liên tục | Tạm dừng gọi dependency, fail fast |
| Rate limit | Caller gửi quá nhiều request | Giới hạn request đầu vào |
| Bulkhead | Một dependency làm cạn tài nguyên chung | Tách pool/concurrency theo dependency |

Các pattern này thường dùng chung:

```text
timeout ngắn
-> retry có backoff/jitter nếu idempotent
-> circuit breaker nếu lỗi liên tục
-> fallback/degrade rõ ràng
```

Ví dụ trả lời phỏng vấn:

> Nếu service gọi payment provider bị timeout liên tục, tôi sẽ đặt timeout ngắn cho mỗi call, chỉ retry khi request có idempotency key, và dùng circuit breaker theo rolling error rate. Khi circuit open, service fail fast hoặc trả trạng thái payment tạm thời không khả dụng thay vì tiếp tục dồn request vào provider. Tôi cũng expose metrics như error rate, timeout rate, circuit state và fallback count để biết dependency đang phục hồi hay vẫn lỗi.

---

## 6. Webhook

Webhook giống async API từ external system gọi vào mình.

Cần:

* verify signature;
* timestamp/nonce chống replay;
* idempotency theo event id;
* return nhanh, xử lý nặng bằng queue;
* retry policy rõ với provider;
* lưu raw payload để audit/debug nếu không chứa secret quá nhạy cảm.

---

## 7. WebSocket/realtime

WebSocket scale cần nghĩ:

* heartbeat/ping-pong;
* reconnect storm;
* sticky session hoặc shared connection registry;
* pub/sub fanout qua Redis/Kafka/NATS;
* backpressure khi client chậm;
* auth refresh trên long-lived connection;
* memory per connection.

Khi chỉ cần server push đơn giản, cân nhắc SSE trước WebSocket.

---

## 8. API versioning và schema evolution

Version trong backend không chỉ là version của REST API. Nó có thể là version của API contract, database schema, event schema, protobuf, token, optimistic lock hoặc app release.

Mục tiêu của versioning là thay đổi hệ thống mà không làm client/service cũ chết ngay lập tức.

### REST

* Không xóa/đổi nghĩa field đang public.
* Thêm field thường backward-compatible.
* Mobile app cũ có thể tồn tại lâu.
* Có deprecation plan trước khi bỏ field/version.

Cách version REST thường gặp:

| Cách | Ví dụ | Ghi chú |
|---|---|---|
| Path version | `/api/v1/orders` | Dễ hiểu, phổ biến |
| Header version | `Accept: application/vnd.app.v2+json` | Sạch URL hơn nhưng tooling phức tạp hơn |
| Query version | `?version=2` | Dễ dùng nhưng ít được ưu tiên cho public API |

### Protobuf/event schema

* Protobuf không được đổi field number hoặc reuse field number đã xóa.
* Event nên có `event_id`, `event_type`, `version`, `occurred_at`.
* Consumer phải idempotent.
* Khi đổi schema, giữ backward compatibility hoặc version event type.

Ví dụ event envelope:

```json
{
  "event_id": "evt_123",
  "event_type": "order.created",
  "version": 2,
  "occurred_at": "2026-06-30T10:00:00Z",
  "payload": {}
}
```

### Database/document/token version

* Database migration có version và chạy theo expand/contract khi production table lớn.
* Document store có thể có `schema_version` để migrate dần.
* Optimistic lock dùng `version` để tránh lost update.
* `token_version` giúp revoke token/session cũ.
* App release version/git SHA giúp debug incident và rollback.

### Câu trả lời phỏng vấn mẫu

> Version trong backend có nhiều lớp: API version cho client contract, protobuf/event schema version cho service communication, database migration version cho schema, document `schema_version` với MongoDB, và optimistic lock `version` để tránh lost update. Khi thay đổi production system, tôi ưu tiên backward compatibility, expand/contract migration và deprecation plan thay vì đổi breaking change đột ngột.

---

## 9. Checklist phỏng vấn

Bạn nên trả lời chắc:

* REST vs gRPC chọn khi nào?
* gRPC deadline khác timeout thường ở đâu?
* Protobuf backward compatibility gồm rule nào?
* RabbitMQ exchange/queue/binding/routing key là gì?
* Ack/nack/prefetch ảnh hưởng reliability và throughput ra sao?
* Retry + DLQ thiết kế thế nào để tránh retry storm?
* Circuit breaker có những state nào và khác retry/rate limit ra sao?
* Khi downstream lỗi liên tục, fallback thế nào để không trả dữ liệu sai?
* Tại sao consumer phải idempotent?
* Outbox pattern giải quyết vấn đề gì?
* Webhook cần bảo mật và idempotency ra sao?
* API versioning tránh breaking change thế nào?
* Version trong backend gồm API, DB schema, event/protobuf, optimistic lock và token version khác nhau ra sao?
