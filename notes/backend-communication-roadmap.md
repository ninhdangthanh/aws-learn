# Backend Communication Roadmap
## REST, gRPC, RabbitMQ, retry, idempotency và async workflow

File này gom các kiến thức giao tiếp backend dễ bị hỏi trong CV có Golang, TypeScript, microservices, gRPC và RabbitMQ.

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

---

## 2. gRPC

gRPC hợp cho internal service communication vì dùng HTTP/2 và Protocol Buffers.

### Cần nắm

* Unary: request/response thường gặp nhất.
* Server streaming: server trả nhiều response.
* Client streaming: client gửi nhiều request.
* Bidirectional streaming: hai chiều đồng thời.
* Deadline/cancellation truyền qua context.
* Interceptor tương tự middleware.
* Status code rõ hơn HTTP status cho internal RPC.

### Status code hay dùng

| Code | Khi dùng |
|---|---|
| `InvalidArgument` | Input sai format/rule |
| `NotFound` | Resource không tồn tại |
| `AlreadyExists` | Duplicate unique resource |
| `FailedPrecondition` | State hiện tại không cho phép action |
| `PermissionDenied` | Không có quyền |
| `Unauthenticated` | Chưa xác thực |
| `Unavailable` | Dependency/service tạm thời không sẵn sàng |
| `DeadlineExceeded` | Quá timeout/deadline |
| `Internal` | Lỗi không mong muốn |

### Protobuf compatibility

Rules quan trọng:

* Không đổi field number.
* Không đổi ý nghĩa field cũ.
* Không reuse field number đã xóa.
* Dùng `reserved` cho field/name đã xóa.
* Thêm field mới theo hướng optional/backward-compatible.
* Enum cần xử lý unknown value.

### REST vs gRPC

REST hợp:

* public API.
* browser/mobile direct.
* debug bằng curl dễ.
* integration với third-party.

gRPC hợp:

* internal microservices.
* schema contract rõ.
* latency/serialization tốt.
* streaming.
* code generation.

Trade-off:

* gRPC tooling/debug cần setup hơn REST.
* Browser support cần gRPC-Web/proxy.
* Schema versioning phải kỷ luật.

---

## 3. RabbitMQ

RabbitMQ là message broker, phù hợp job async, task queue, decouple service và workflow không cần Kafka-level event log.

### Thành phần

* Producer: publish message.
* Exchange: nhận message và route.
* Queue: lưu message chờ consumer.
* Binding: nối exchange với queue.
* Routing key: key dùng để route.
* Consumer: xử lý message.

### Exchange types

| Type | Cách route | Use case |
|---|---|---|
| Direct | match routing key chính xác | task theo loại cụ thể |
| Topic | match pattern `order.*` | domain event nhiều loại |
| Fanout | broadcast tới mọi queue bound | notification/cache invalidation |
| Headers | route theo headers | ít dùng hơn |

### Ack, nack và prefetch

* Ack sau khi xử lý thành công.
* Nack/reject khi xử lý thất bại.
* Requeue cẩn thận vì có thể tạo infinite retry loop.
* Prefetch giới hạn số message unacked mỗi consumer để tạo backpressure.

Interview answer:

> Tôi không ack ngay khi nhận message. Tôi xử lý xong và persist side effect thành công rồi mới ack. Nếu lỗi transient thì retry có backoff, nếu lỗi permanent hoặc quá số lần thì đưa vào DLQ.

### Retry và DLQ

Retry strategy nên có:

* max retry count.
* exponential backoff.
* jitter nếu nhiều consumer.
* DLQ cho poison message.
* metric retry count/DLQ count.

Không nên retry liên tục ngay lập tức vì sẽ tạo retry storm và làm nghẽn queue.

### Idempotent consumer

RabbitMQ thường dùng at-least-once delivery, nghĩa là duplicate message có thể xảy ra.

Pattern:

* message id/event id unique.
* dedup table.
* unique constraint trên business key.
* check current state trước khi update.
* side effect external có idempotency key.

---

## 4. Outbox pattern

Problem:

```text
DB commit success -> publish event fail
```

hoặc:

```text
publish event success -> DB rollback
```

Outbox pattern:

1. Trong cùng DB transaction, update business table và insert row vào `outbox_events`.
2. Worker đọc outbox chưa publish.
3. Publish message tới broker.
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

* timeout cho mọi downstream call.
* retry có max attempts.
* exponential backoff.
* jitter.
* circuit breaker khi dependency lỗi liên tục.
* timeout budget toàn request.

Không retry:

* validation error.
* authorization error.
* non-idempotent payment/order operation nếu thiếu idempotency key.

---

## 6. Webhook

Webhook giống async API từ external system gọi vào mình.

Cần:

* verify signature.
* timestamp/nonce chống replay.
* idempotency theo event id.
* return nhanh, xử lý nặng bằng queue.
* retry policy rõ với provider.
* lưu raw payload để audit/debug nếu không chứa secret quá nhạy cảm.

---

## 7. WebSocket/realtime

WebSocket scale cần nghĩ:

* heartbeat/ping-pong.
* reconnect storm.
* sticky session hoặc shared connection registry.
* pub/sub fanout qua Redis/Kafka/NATS.
* backpressure khi client chậm.
* auth refresh trên long-lived connection.
* memory per connection.

Khi chỉ cần server push đơn giản, cân nhắc SSE trước WebSocket.

---

## 8. API versioning và schema evolution

### REST

* Không xóa/đổi nghĩa field đang public.
* Thêm field thường backward-compatible.
* Mobile app cũ có thể tồn tại lâu.
* Có deprecation plan trước khi bỏ field/version.

### gRPC/protobuf

* Không đổi field number.
* Reserve field đã xóa.
* Thêm field optional.
* Consumer phải chịu được unknown field/enum value.

### Event schema

* Event nên có `event_id`, `event_type`, `version`, `occurred_at`.
* Consumer phải idempotent.
* Khi đổi schema, giữ backward compatibility hoặc version event type.

---

## 9. Checklist phỏng vấn

Bạn nên trả lời chắc:

* REST vs gRPC chọn khi nào?
* gRPC deadline khác timeout thường ở đâu?
* Protobuf backward compatibility gồm rule nào?
* RabbitMQ exchange/queue/binding/routing key là gì?
* Ack/nack/prefetch ảnh hưởng reliability và throughput ra sao?
* Retry + DLQ thiết kế thế nào để tránh retry storm?
* Tại sao consumer phải idempotent?
* Outbox pattern giải quyết vấn đề gì?
* Webhook cần bảo mật và idempotency ra sao?
* API versioning tránh breaking change thế nào?

