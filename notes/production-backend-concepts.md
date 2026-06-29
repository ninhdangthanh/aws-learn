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

Pattern:

* client gửi `Idempotency-Key`.
* backend lưu key + request hash + response/result.
* retry cùng key trả lại kết quả cũ hoặc trạng thái đang xử lý.
* side effect phải có unique constraint/dedup table bảo vệ.

Không nên tin "client không retry". Timeout/network lỗi luôn có thể tạo duplicate.

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

