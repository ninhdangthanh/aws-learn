# Production Backend Concepts
## Những tình huống dễ bị hỏi khi phỏng vấn Middle/Senior Backend

File này gom lại các production concepts trước đây nằm trong `readme.md`, nhưng tổ chức theo topic để dễ ôn và dễ liên kết với system design.

Mindset chính:

> Backend không khó chỉ vì business logic. Nó khó khi có scale, concurrency, consistency, operation, migration và failure mode.

---

## 1. API, traffic và client behavior

### Rate limiting không chỉ là API gateway

Rate limit production thường có nhiều dimension:

* user, IP, API key, role, tenant.
* endpoint cost, ví dụ search rẻ hơn export report.
* concurrency, ví dụ chỉ cho 2 export jobs/user.
* internal service vs public client.

Concept cần nhớ:

* token bucket vs leaky bucket.
* sliding window.
* distributed counter bằng Redis.
* burst traffic và fairness.
* retry storm khi client đồng loạt retry sau `429`.

### Idempotency

Idempotency cần cho payment, order, webhook, sync/offline POS, queue consumer.

Idempotency nghĩa là cùng một operation được gọi nhiều lần nhưng chỉ tạo ra một kết quả/side effect như một lần gọi duy nhất. Nó không có nghĩa là request không bị retry; ngược lại, idempotency tồn tại vì retry, timeout và duplicate message luôn có thể xảy ra.

Pattern:

* client gửi `Idempotency-Key`.
* backend lưu key + request hash + response/result.
* retry cùng key trả lại kết quả cũ hoặc trạng thái đang xử lý.
* side effect phải có unique constraint/dedup table bảo vệ.

Không nên tin "client không retry". Timeout/network lỗi luôn có thể tạo duplicate.

#### Idempotency khác gì retry và dedup?

| Khái niệm | Ý nghĩa |
|---|---|
| Retry | Gọi lại khi request/job lỗi hoặc timeout |
| Deduplication | Phát hiện và bỏ qua bản ghi/message trùng |
| Idempotency | Thiết kế operation để retry/duplicate không tạo thêm side effect |

Retry mà không có idempotency có thể làm lỗi nặng hơn, ví dụ charge tiền hai lần hoặc tạo hai order.

#### Khi nào cần idempotency?

Nên có idempotency cho operation có side effect:

* tạo order.
* charge/refund payment.
* apply coupon.
* update inventory.
* webhook từ payment provider.
* POS offline sync gửi lại batch sau khi mất mạng.
* queue consumer xử lý message at-least-once.
* gửi email/SMS/push notification quan trọng.

Không nhất thiết cần idempotency key cho read-only request như `GET /products`, vì read không tạo side effect. Nhưng read vẫn cần cache consistency/rate limit nếu traffic lớn.

#### Idempotency key lifecycle

Luồng phổ biến:

```text
Client gửi POST /orders với Idempotency-Key
-> API hash request body quan trọng
-> DB insert idempotency record với status=processing
-> Nếu insert thành công: xử lý business logic
-> Commit side effect như tạo order/payment
-> Lưu response/result vào idempotency record, status=succeeded
-> Retry cùng key: trả lại response cũ
```

Nếu request cùng key đến khi request đầu đang xử lý:

```text
Idempotency-Key tồn tại với status=processing
-> trả 409/425 hoặc response "request is still processing"
-> client retry sau
```

Nếu request cùng key nhưng payload khác:

```text
Idempotency-Key giống nhau
request_hash khác nhau
-> reject 409 Conflict
```

Điểm này rất quan trọng. Nếu không lưu `request_hash`, client có thể vô tình reuse key cho operation khác và nhận response sai.

#### Schema mẫu

Ví dụ bảng idempotency:

```sql
CREATE TABLE idempotency_keys (
    key TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL, -- processing, succeeded, failed
    response_status INT,
    response_body JSONB,
    resource_type TEXT,
    resource_id TEXT,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
```

Lưu ý:

* `key` nên unique.
* Có thể scope key theo `user_id/tenant_id` để tránh conflict giữa tenants.
* `request_hash` chống reuse key với payload khác.
* `status` giúp xử lý request đang chạy hoặc đã xong.
* `resource_id` giúp trace tới order/payment thật.
* `expires_at` giúp cleanup key cũ.

#### Race condition với cùng key

Hai request cùng `Idempotency-Key` có thể đến cùng lúc.

Pattern an toàn:

1. Insert idempotency key với unique constraint.
2. Chỉ request insert thành công được xử lý side effect.
3. Request còn lại đọc record hiện có.
4. Nếu status `processing`, trả trạng thái đang xử lý hoặc đợi ngắn có timeout.
5. Nếu status `succeeded`, trả lại response cũ.

Không nên check trước rồi insert sau theo kiểu:

```text
SELECT key
-> không thấy
-> cả hai request cùng xử lý
```

Vì đây là race condition. Unique constraint hoặc transaction/lock phải là lớp bảo vệ cuối.

#### Idempotency cho payment

Payment là ví dụ kinh điển:

```text
Client gọi charge
-> server gọi payment provider
-> client timeout trước khi nhận response
-> client retry
```

Nếu không idempotent, user có thể bị charge hai lần.

Pattern:

* Client gửi `Idempotency-Key`.
* Backend lưu operation key.
* Khi gọi payment provider, truyền idempotency key nếu provider hỗ trợ.
* Lưu `payment_intent_id`/`provider_transaction_id`.
* Retry trả lại trạng thái payment hiện tại thay vì charge lại.

Nên trả lời:

> Với payment, idempotency phải có ở cả phía hệ thống mình và phía payment provider nếu provider hỗ trợ. Backend không được tự động charge lại một operation không có idempotency key.

#### Idempotency cho order/inventory

Tạo order có thể retry vì client timeout.

Pattern:

* `Idempotency-Key` đại diện cho "tạo order này".
* Nếu retry cùng key, trả lại order cũ.
* Inventory update vẫn cần transaction/locking riêng để tránh oversell.
* Idempotency không thay thế isolation/lock.

Điểm phỏng vấn hay:

> Idempotency giúp tránh tạo duplicate order khi retry, nhưng không tự giải quyết oversell. Oversell cần DB transaction, optimistic/pessimistic locking hoặc atomic update.

#### Idempotency cho webhook

Webhook provider thường retry nếu endpoint của mình timeout hoặc trả 5xx.

Pattern:

* Verify signature trước.
* Dùng `event_id` từ provider làm dedup key.
* Insert event id vào bảng processed events với unique constraint.
* Nếu event đã xử lý, trả 200 nhanh.
* Handler phải chịu được out-of-order event bằng state machine/version.

Ví dụ:

```text
payment.succeeded event đến 2 lần
-> lần đầu update order paid
-> lần hai thấy event_id đã xử lý
-> trả 200, không update duplicate
```

#### Idempotency cho queue consumer

RabbitMQ/Kafka thường là at-least-once: message có thể được giao lại.

Pattern:

* Message có `event_id`.
* Consumer insert `event_id` vào dedup table trước hoặc trong cùng transaction với side effect.
* Nếu duplicate, ack và bỏ qua.
* Side effect external cần idempotency key riêng.

Không ack message trước khi side effect thành công. Nếu ack trước rồi crash, message mất nhưng business chưa xử lý xong.

#### Idempotency cho offline POS sync

Offline POS dễ gặp duplicate vì thiết bị mất mạng rồi gửi lại batch.

Pattern:

* Mỗi local operation có `client_operation_id`.
* Server lưu mapping `device_id + client_operation_id`.
* Retry cùng operation trả lại kết quả cũ.
* Conflict cần xử lý theo version/timestamp/business rule.
* Batch sync nên xử lý từng item idempotent, tránh fail cả batch vì một item trùng.

Đây là ví dụ rất hợp với CV F&B/POS/offline-first.

#### TTL và cleanup

Idempotency key không nên giữ mãi nếu không cần.

TTL phụ thuộc business:

* payment/order: thường giữ lâu hơn, vài ngày đến vài tuần tùy policy.
* webhook event: giữ theo retry window của provider.
* POS sync: giữ đủ lâu để thiết bị offline có thể retry.
* notification: giữ theo campaign/job window.

Cleanup cần cẩn thận để không xóa key quá sớm khi client/provider còn retry.

#### Trạng thái failed xử lý thế nào?

Không phải lỗi nào cũng nên cache mãi.

Gợi ý:

* Validation error: có thể trả lỗi ngay; thường không cần tạo side effect.
* Business failure deterministic, ví dụ hết hàng: có thể lưu response failed để retry cùng key trả lại cùng lỗi.
* Transient failure trước side effect, ví dụ DB timeout chưa tạo order: có thể cho retry xử lý lại.
* Unknown state, ví dụ gọi payment provider timeout không biết charge thành công chưa: phải reconcile/check provider trước khi retry charge.

Unknown state là phần nguy hiểm nhất. Không nên đơn giản retry charge.

#### Lỗi thiết kế thường gặp

* Chỉ lưu idempotency key nhưng không lưu request hash.
* Check key bằng SELECT rồi xử lý, không có unique constraint.
* Không lưu response/result nên retry trả kết quả khác.
* Xóa key quá sớm.
* Dùng một key cho nhiều loại operation.
* Idempotency key không scope theo user/tenant.
* Retry operation non-idempotent như payment charge.
* Nghĩ idempotency thay thế transaction/locking.
* Consumer ack message trước khi side effect thành công.

#### Câu trả lời phỏng vấn mẫu

> Idempotency là thiết kế để cùng một operation dù bị retry nhiều lần vẫn chỉ tạo một side effect. Ví dụ tạo order hoặc charge payment, client gửi `Idempotency-Key`, backend lưu key, request hash, trạng thái xử lý và response/result. Nếu retry cùng key và payload giống nhau, tôi trả lại kết quả cũ; nếu cùng key nhưng payload khác, trả conflict. Để chống race condition, bảng idempotency cần unique constraint và request đầu tiên insert thành công mới được xử lý side effect. Với queue/webhook, tôi dùng event id/dedup table vì delivery thường là at-least-once.

### API versioning

Thay đổi response JSON có thể làm mobile app cũ hoặc third-party integration chết.

Cần nghĩ:

* backward compatibility.
* deprecation window.
* version endpoint/header khi cần.
* schema evolution.
* contract test với client quan trọng.

---

## 2. PostgreSQL/database production

### Delete dữ liệu cũ rất nguy hiểm

`DELETE FROM orders WHERE created_at < ...` trên bảng lớn có thể gây lock, full scan, replication lag, WAL/binlog lớn và CPU spike.

Pattern an toàn:

* batch delete nhỏ.
* partition theo thời gian rồi drop partition.
* shadow table/copy-swap cho case đặc biệt.
* archive trước khi delete nếu cần audit.

### Offset pagination chết ở page lớn

`LIMIT 20 OFFSET 1000000` vẫn phải scan/skip rất nhiều rows. Cursor pagination ổn hơn cho feed/order history/notification.

Best practice:

* cursor theo indexed column.
* nếu `created_at` trùng, dùng cursor kép `(created_at, id)`.
* offset chỉ hợp khi dữ liệu nhỏ hoặc cần nhảy trang ngẫu nhiên.

### `SELECT COUNT(*)` không phải lúc nào cũng rẻ

Trên bảng rất lớn, count exact có thể đắt. Có thể dùng:

* estimated count.
* cached count.
* "has more" thay vì total page exact.
* async/reporting query nếu business thật sự cần total.

### Online schema migration

"Chỉ thêm một cột" có thể lock bảng hoặc rewrite table tùy DB/version/câu lệnh.

Pattern:

* expand/contract migration.
* add nullable column trước, backfill batch, rồi add constraint sau.
* deploy code backward-compatible.
* dùng gh-ost/pt-online-schema-change với MySQL khi phù hợp.

### Unique constraint và race condition

Check ở application không đủ:

```text
check username exists -> insert
```

Hai request song song có thể cùng pass check. DB unique constraint mới là lớp bảo vệ cuối, app cần handle duplicate key và retry/return conflict.

### Read replica stale read

Flow `write primary -> read replica` có thể đọc data cũ vì replication lag.

Cách xử lý:

* read-after-write đọc primary trong thời gian ngắn.
* sticky session.
* primary fallback cho action vừa ghi.
* hiển thị trạng thái pending nếu eventual consistency chấp nhận được.

### N+1 query

Code đẹp như `orders.map(loadUser)` có thể thành 1001 queries.

Fix:

* join.
* batch loading.
* dataloader.
* prefetch.
* đo bằng query count trong test/log.

### Connection pool exhaustion

DB có thể chưa chậm, nhưng app hết connection vì pool size sai hoặc leak connection.

Cần monitor:

* active/idle/wait connection.
* query latency.
* connection lifetime.
* tổng connection = pool size x số app instances.

### Soft delete

`deleted_at IS NULL` dễ quên filter, gây lộ dữ liệu đã xóa.

Rủi ro:

* unique constraint conflict.
* storage growth.
* foreign key phức tạp.
* query/report sai nếu quên condition.

Pattern:

* partial unique index.
* repository/query scope bắt buộc.
* hard delete/archive policy rõ ràng.

---

## 3. Cache, Redis và distributed coordination

Chi tiết sâu hơn nằm ở [Redis Middle Notes](redis-middle-notes.md): use case, data type, sorted-set scheduler/reminder, JWT blacklist, edge cases và cache strategies.

### Cache không chỉ là "add Redis"

Bug phổ biến:

* stale data.
* invalidation sai.
* hot key.
* cache penetration.
* cache stampede.
* cache avalanche.

Giảm rủi ro:

* TTL jitter.
* singleflight/request coalescing.
* background refresh.
* cache null ngắn hạn cho miss hợp lệ.
* rate limit khi cache miss.

### Distributed lock

`SETNX lock` chưa đủ cho production.

Cần nghĩ:

* lock expiry giữa chừng.
* process chết.
* network partition.
* lock ownership.
* fencing token để downstream biết request nào mới hơn.

Nếu side effect là DB update quan trọng, ưu tiên DB transaction/unique constraint/optimistic lock khi có thể.

---

## 4. Queue, event và async workflow

### Queue không phải cứ RabbitMQ/Kafka là xong

Khi consumer chậm:

* backlog tăng.
* retry storm.
* poison message.
* duplicate processing.
* ordering broken.

Cần thiết kế:

* ack/nack đúng thời điểm.
* prefetch/concurrency limit.
* retry với exponential backoff + jitter.
* DLQ.
* idempotent consumer.
* consumer lag/backlog metric.

### Exactly-once gần như là illusion

Distributed system thực tế thường là at-least-once hoặc at-most-once.

Exactly-once thường được mô phỏng bằng:

* idempotency.
* deduplication.
* transaction/outbox.
* unique constraint.

### Event out-of-order

Consumer có thể nhận `OrderCancelled` trước `OrderCreated`.

Cách xử lý:

* partition/routing key theo aggregate id.
* event version/sequence.
* state machine.
* idempotent handler.
* retry/defer event chưa đủ context.

### Transaction với external side effect

Không nên giữ DB transaction trong lúc gọi payment API/email provider.

Pattern:

* local transaction ghi state.
* outbox table ghi event.
* worker publish event.
* consumer xử lý side effect idempotent.
* Saga/compensating action cho workflow nhiều bước.

---

## 5. Background jobs, cron và retry

### Cron job có nhiều edge case

Rủi ro:

* timezone/DST.
* missed schedule khi server restart.
* job chạy overlap.
* nhiều instance chạy cùng job.

Pattern:

* job idempotency.
* persisted scheduler.
* distributed lock hoặc leader election.
* dedup key theo business period.
* retry/recovery rõ ràng.

### Retry có thể làm sập hệ thống

Retry đồng loạt sau timeout tạo retry storm.

Cần:

* timeout budget.
* exponential backoff.
* jitter.
* max retry.
* circuit breaker.
* không retry non-idempotent operation nếu không có idempotency key.

### "Just send email" cũng phức tạp

Email provider timeout không nên làm fail order nếu email không critical.

Pattern:

* queue async.
* idempotent notification key.
* retry + DLQ.
* audit trạng thái gửi.

---

## 6. Search, file, realtime và domain edge cases

### Search keyword

`LIKE '%iphone%'` không scale tốt và ranking kém.

Lựa chọn:

* PostgreSQL full-text search/trigram index.
* Elasticsearch/OpenSearch.
* Meilisearch.
* inverted index.

### Search by nearest location

Naive calculate distance toàn bộ rows sẽ chậm.

Lựa chọn:

* PostGIS.
* spatial index.
* geohash.
* bounding box prefilter.

### File upload

Hidden complexity:

* file size limit.
* content-type spoofing.
* virus scan.
* partial upload/resume.
* signed URL.
* image/video processing queue.
* storage lifecycle.

### WebSocket scale

1 triệu connection cần nghĩ:

* heartbeat.
* reconnect storm.
* sticky session hoặc connection routing.
* pub/sub fanout.
* memory per connection.
* backpressure khi client chậm.

### Money calculation

Không dùng float cho tiền.

Pattern:

* smallest currency unit.
* decimal type.
* rounding policy.
* currency conversion rate snapshot.

### Time

Time bug thường đến từ timezone, DST và parsing.

Pattern:

* store UTC.
* convert at edge.
* lưu timezone của user/store nếu business cần lịch địa phương.
* test quanh DST/month-end.

---

## 7. Observability, rollout và multi-tenant

### Logging quá nhiều cũng làm chết system

Rủi ro:

* disk full.
* I/O bottleneck.
* ingestion cost cao.
* lộ PII/secret.

Pattern:

* structured logging.
* sampling.
* redact PII.
* log level theo môi trường.
* correlation/request id.

### Feature flag

Không chỉ là `if/else`.

Cần:

* percentage rollout.
* tenant/user targeting.
* instant rollback.
* flag dependency.
* cleanup stale flag.

### Multi-tenant architecture

Rủi ro:

* noisy neighbor.
* data leak.
* per-tenant config/rate limit.
* tenant-specific migration.

Mức cô lập:

* shared DB + `tenant_id`.
* schema per tenant.
* database per tenant.
* shard per tenant group.

---

## 8. Cách dùng trong phỏng vấn

Khi gặp một câu system design hoặc project deep dive, chọn 3-5 failure mode liên quan nhất:

* API: rate limit, idempotency, versioning.
* DB: lock, query plan, migration, replica lag.
* Cache: stampede, hot key, stale data.
* Queue: duplicate, retry, DLQ, ordering.
* Ops: logging, metrics, graceful shutdown, rollback.

Không cần kể hết mọi concept. Điểm mạnh là chọn đúng rủi ro cho bài toán.
