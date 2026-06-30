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

### Circuit breaker là gì?

Circuit breaker là pattern bảo vệ service khỏi việc tiếp tục gọi một dependency đang lỗi/chậm liên tục. Thay vì để request dồn vào dependency rồi làm cạn connection pool, goroutine/thread, memory hoặc timeout budget, circuit breaker sẽ tạm thời "ngắt mạch" và fail fast.

Dependency có thể là:

* payment gateway.
* service nội bộ qua HTTP/gRPC.
* LLM/embedding provider.
* search service như Elasticsearch/Qdrant.
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

### Khi nào mở circuit?

Không nên mở circuit chỉ vì một vài lỗi đơn lẻ. Thường cần tính theo rolling window:

* failure rate, ví dụ > 50% trong 30 giây.
* timeout rate.
* số lỗi tối thiểu trước khi đánh giá, ví dụ ít nhất 20 requests.
* lỗi loại nào được tính: timeout, 5xx, connection refused.
* lỗi loại nào không tính: 4xx do client input sai.

Ví dụ:

```text
Trong 60 giây gần nhất:
- tổng request tới payment provider >= 50
- timeout/error >= 60%
=> open circuit trong 30 giây
```

### Fallback

Fallback phụ thuộc business, không phải lúc nào cũng trả data giả.

Ví dụ fallback hợp lý:

* Search service lỗi: trả kết quả cache gần nhất hoặc thông báo search tạm thời không khả dụng.
* Recommendation lỗi: ẩn block recommendation.
* LLM provider lỗi: trả message "không thể tạo câu trả lời lúc này", lưu request để retry nếu phù hợp.
* Payment lỗi: không tự động charge lại nếu thiếu idempotency; trả trạng thái pending/failed rõ ràng.

Fallback xấu:

* Trả dữ liệu sai để che lỗi.
* Retry ngầm một operation non-idempotent.
* Nuốt lỗi khiến user nghĩ thao tác đã thành công.

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

### Metrics cần theo dõi

* request count theo dependency.
* success/error/timeout rate.
* latency p50/p95/p99.
* circuit state: closed/open/half-open.
* số lần circuit open.
* fallback count.
* retry count.
* saturation của connection pool/worker pool.

Log nên có dependency name, request id/correlation id, circuit state và reason circuit open.

### Ví dụ trả lời phỏng vấn

> Nếu service gọi payment provider bị timeout liên tục, tôi sẽ đặt timeout ngắn cho mỗi call, chỉ retry khi request có idempotency key, và dùng circuit breaker theo rolling error rate. Khi circuit open, service fail fast hoặc trả trạng thái payment tạm thời không khả dụng thay vì tiếp tục dồn request vào provider. Tôi cũng expose metrics như error rate, timeout rate, circuit state và fallback count để biết dependency đang phục hồi hay vẫn lỗi.

### Lỗi thiết kế thường gặp

* Không đặt timeout, khiến circuit breaker không cứu được request đang treo quá lâu.
* Retry quá nhiều trước circuit breaker, làm lỗi nặng hơn.
* Dùng chung một circuit cho nhiều dependency khác nhau.
* Không phân biệt lỗi client 4xx và lỗi dependency 5xx/timeout.
* Fallback trả dữ liệu sai hoặc che mất lỗi quan trọng.
* Không có metric nên không biết circuit đang open.

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
* Circuit breaker có những state nào và khác retry/rate limit ra sao?
* Khi downstream lỗi liên tục, fallback thế nào để không trả dữ liệu sai?
* Tại sao consumer phải idempotent?
* Outbox pattern giải quyết vấn đề gì?
* Webhook cần bảo mật và idempotency ra sao?
* API versioning tránh breaking change thế nào?
