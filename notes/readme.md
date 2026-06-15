Khi phỏng vấn hoặc chia sẻ kinh nghiệm, các kỹ sư Senior Backend thường đúc kết rằng: Sự khác biệt lớn nhất không nằm ở những thuật toán cao siêu, mà ở việc hệ thống của bạn sẽ sống sót ra sao khi mọi thứ bắt đầu fail, scale lớn, xuất hiện race condition, hay dữ liệu rác tràn vào.

Dưới đây là một cuộc hội thoại thực tế của các kỹ sư về những tình huống "dở khóc dở cười" trên production — nơi ranh giới giữa Junior và Senior được phân định rõ ràng nhất.

---

**1. Rate limit không chỉ là API gateway problem**

Junior thường nghĩ:

> “Nginx / API Gateway limit 100 req/min là xong.”

Nhưng thực tế:

* User VIP cần quota khác
* Internal service không nên bị limit như public user
* Login API cần aggressive limit
* Search API cần limit theo cost
* Một request export CSV có thể tốn gấp 100 lần request bình thường

Nên rate limit thường có nhiều dimension:

* theo user
* theo IP
* theo API key
* theo role
* theo endpoint cost
* theo concurrency
* theo tenant/company

Ví dụ:

```txt
/free-user/search = 10 req/s
/vip-user/search = 100 req/s
/export-report = max 2 concurrent jobs
```

Senior level sẽ nghĩ tới:

* token bucket vs leaky bucket
* distributed counter bằng Redis
* sliding window
* burst traffic
* fairness
* retry storm
* rate limit synchronization giữa nhiều instance

---

**2. Delete data cũ = operation cực nguy hiểm**

Câu:

```sql
DELETE FROM orders WHERE created_at < '2022-01-01';
```

ở production vài trăm triệu row có thể:

* lock table
* full scan
* replication lag
* fill WAL/binlog cực lớn
* làm DB CPU 100%
* chết read replica

Nên senior thường dùng:

### Batch delete

```sql
DELETE ...
LIMIT 1000;
```

loop nhiều lần.

Hoặc:

### Partitioning

Partition theo tháng/năm:

```txt
orders_2021
orders_2022
orders_2023
```

Muốn xoá:

```sql
DROP PARTITION
```

=> gần như instant.

Hoặc:

### Shadow table strategy

* create table mới
* copy data cần giữ
* swap table
* rename

Đây là pattern rất production-grade.

---

**3. OFFSET pagination chết ở page lớn**

Query:

```sql
LIMIT 20 OFFSET 1000000
```

DB vẫn phải:

* scan
* sort
* skip 1 triệu row

rồi mới trả 20 row.

Nên page càng lớn càng chậm.

---

Senior sẽ dùng:

## Cursor pagination

Ví dụ:

```sql
WHERE id > 12345
LIMIT 20
```

hoặc:

```sql
WHERE created_at < last_seen_created_at
```

Complexity thấp hơn rất nhiều.

---

Điểm hay là:

**cursor pagination không chỉ nhanh hơn, mà còn đúng hơn trong realtime system.**

Offset pagination bị bug:

* đang page 3
* có row mới insert
* data shift
* duplicate/missing item

Cursor tránh được chuyện này.

---

**4. “SELECT COUNT(*)” tưởng rẻ nhưng đôi khi rất đắt**

Frontend thích:

```txt
Page 1/53291
```

Nhưng:

```sql
SELECT COUNT(*)
```

trên bảng vài tỷ row cực nặng.

Nhiều system lớn:

* không trả total exact
* dùng estimated count
* hoặc chỉ show:

  * “more results”
  * “next page”

Ví dụ:

* Twitter
* Facebook
* Reddit

không ai care page số 18293.

---

**5. Idempotency nhìn đơn giản nhưng cực khó**

Ví dụ payment:

```txt
Client timeout
→ retry
→ server xử lý lần 2
→ charge tiền 2 lần
```

Senior sẽ nghĩ:

* idempotency key
* deduplication table
* exactly-once illusion
* distributed transaction
* retry safety

Đây là lý do Stripe rất nổi tiếng về idempotency design.

---

**6. Queue không phải cứ Kafka/RabbitMQ là xong**

Ví dụ consumer chậm:

* backlog tăng
* retry storm
* poison message
* duplicate processing
* ordering broken

Senior sẽ phải nghĩ:

* DLQ
* retry strategy
* exponential backoff
* partition key
* consumer lag
* at least once vs exactly once
* rebalancing

---

**7. Cache là nơi sinh ra bug kinh dị**

Không phải “add Redis là nhanh”.

Mà là:

* cache invalidation
* stale data
* thundering herd
* hot key
* cache penetration
* cache stampede

Ví dụ:

1 key product hot bị expire:

```txt
100k requests
→ cùng miss cache
→ đập DB cùng lúc
→ DB chết
```

Nên cần:

* singleflight
* request coalescing
* staggered TTL
* background refresh

---

**8. Transaction không phải lúc nào cũng dùng được**

Junior:

```txt
BEGIN
update order
call payment API
send email
COMMIT
```

Problem:

* external API không participate trong DB transaction
* transaction giữ lock quá lâu

Senior sẽ nghĩ:

* saga pattern
* outbox pattern
* eventual consistency
* compensating transaction

---

**9. Search keyword tưởng LIKE là đủ**

```sql
WHERE name LIKE '%iphone%'
```

Scale lên:

* chậm
* typo search tệ
* ranking tệ

Senior sẽ dùng:

* Elasticsearch
* Meilisearch
* trigram index
* full-text index
* inverted index

---

**10. “Chỉ thêm một cột” đôi khi thành incident**

```sql
ALTER TABLE orders ADD COLUMN ...
```

Ở bảng vài TB:

* rewrite table
* lock write
* replication lag
* downtime

Senior phải biết:

* online schema migration
* gh-ost
* pt-online-schema-change
* expand/contract migration

---

Đặc điểm chung của mấy bài này:

> complexity không nằm ở business logic,
> mà nằm ở scale, concurrency, consistency, operation, migration, failure mode.

Đó là chỗ khác biệt lớn giữa:

* “code chạy được”
  vs
* “system sống được ở production vài năm”.


Có rất nhiều. Senior backend thường không “khó” ở thuật toán, mà khó ở chỗ:

> system hoạt động thế nào khi mọi thứ bắt đầu fail, scale, race condition, hoặc dirty data xuất hiện.

Một loạt situation khác rất điển hình:

---

**11. Distributed lock tưởng dễ nhưng cực dễ sai**

Ví dụ:

```txt id="w6m3dx"
cron job generate monthly invoice
```

Deploy 5 instances.

Nếu không lock:

```txt id="6qdrmy"
5 instance chạy cùng lúc
→ generate duplicate invoice
```

Junior thường:

```txt id="jw1jyb"
SETNX redis_lock
```

Nhưng production thật phải nghĩ:

* lock expire giữa chừng
* process chết
* clock drift
* network partition
* lock ownership
* fencing token

Đây là lý do distributed locking là topic rất sâu.

---

**12. Cron job “đơn giản” nhưng đầy edge case**

Ví dụ:

```txt id="h9ekb4"
0 0 * * *
```

Nhưng:

* timezone khác nhau
* DST (daylight saving)
* job chạy 2 lần
* server restart
* missed schedule
* long-running job overlap

Senior sẽ nghĩ:

* job idempotency
* dedup
* job recovery
* scheduler persistence

---

**13. Unique constraint không solve hết race condition**

Ví dụ register username:

```txt id="04c7v6"
check username exists
→ insert
```

2 request cùng lúc:

```txt id="6gf4mv"
both pass check
→ duplicate
```

Senior sẽ rely vào:

* DB unique constraint
* transaction isolation
* retry strategy

Không trust application check.

---

**14. Read replica gây stale read**

Flow:

```txt id="pr8d3r"
write → primary
read → replica
```

Có replication lag:

```txt id="u4p9fu"
user vừa update profile
→ refresh page
→ thấy data cũ
```

Senior sẽ nghĩ:

* read-after-write consistency
* sticky session
* primary read fallback
* causal consistency

---

**15. “Retry” có thể làm sập hệ thống**

Ví dụ downstream timeout.

1000 request retry cùng lúc:

```txt id="h1f9hk"
retry storm
```

=> service chết hoàn toàn.

Nên cần:

* exponential backoff
* jitter
* circuit breaker
* timeout budget

---

**16. N+1 query problem**

Code nhìn đẹp:

```js id="f3pgqt"
orders.map(order => loadUser(order.userId))
```

Production:

```txt id="73o6fz"
1000 orders
→ 1001 queries
```

DB chết.

Senior sẽ nghĩ:

* batch loading
* join
* dataloader
* prefetch

---

**17. File upload rất nhiều hidden complexity**

Không chỉ:

```txt id="0h1x1h"
multipart/form-data
```

Mà còn:

* virus scan
* content-type spoofing
* huge file
* partial upload
* CDN
* signed URL
* image processing queue
* storage lifecycle

---

**18. WebSocket scale không đơn giản**

10 connection dễ.

1 triệu connection:

* connection fanout
* heartbeat
* reconnect storm
* sticky session
* pub/sub scaling
* memory per connection

Đây là lý do realtime infra rất khó.

---

**19. “Search by nearest location” là bài toán khó**

```txt id="j0dhqo"
find restaurant near me
```

Naive:

```sql id="9tb4s6"
calculate distance for all rows
```

=> chết.

Senior sẽ dùng:

* geohash
* PostGIS
* spatial index
* bounding box optimization

---

**20. Money calculation cực nguy hiểm**

Junior:

```js id="onfjlwm"
0.1 + 0.2
```

=> floating point issue.

Senior:

* decimal type
* smallest currency unit
* rounding policy
* currency conversion consistency

Payment system rất ghét float.

---

**21. Time là địa ngục**

Ví dụ:

```txt id="v7v3gs"
store local datetime
```

Sau này:

* timezone bug
* DST bug
* parsing inconsistency

Senior gần như luôn:

```txt id="0nt5o0"
store UTC
convert at edge
```

---

**22. “Just send email” cũng phức tạp**

Nếu email provider timeout:

* retry?
* duplicate email?
* order success nhưng email fail?
* transactional email guarantee?

Senior sẽ tách async bằng queue.

---

**23. Logging quá nhiều cũng có thể giết system**

Ví dụ:

```txt id="4drj90"
log every request body
```

Scale lớn:

* disk full
* I/O bottleneck
* expensive ingestion
* sensitive data leak

Senior sẽ nghĩ:

* structured logging
* sampling
* redact PII
* log level strategy

---

**24. Feature flag không đơn giản là if/else**

Production cần:

* percentage rollout
* tenant rollout
* instant rollback
* flag dependency
* stale flag cleanup

Feature flag system thực ra là infrastructure.

---

**25. Multi-tenant architecture**

Nghe đơn giản:

```txt id="8c48gm"
tenant_id column
```

Nhưng sau này:

* noisy neighbor
* tenant isolation
* data leak risk
* per-tenant rate limit
* tenant-specific config

Scale lớn thường phải:

* shard per tenant
* database per tenant

---

**26. Soft delete gây bug âm thầm**

```txt id="v4e4a5"
deleted_at IS NULL
```

Quên filter 1 query:

→ lộ data đã xoá.

Ngoài ra còn:

* unique constraint conflict
* storage growth
* foreign key complexity

---

**27. Event-driven system bị out-of-order**

Ví dụ Kafka:

```txt id="91o3wy"
OrderCreated
OrderCancelled
```

Nhưng consumer nhận:

```txt id="9pjlwm"
Cancelled trước
Created sau
```

Senior phải design:

* ordering strategy
* versioning
* state machine
* idempotent consumer

---

**28. API versioning**

Junior:

```txt id="r71x7m"
change response JSON
```

Production:

* mobile app cũ chết
* third-party integration break

Senior phải nghĩ:

* backward compatibility
* schema evolution
* deprecation strategy

---

**29. Connection pool exhaustion**

DB chưa chết.

Nhưng:

```txt id="x2j8m3"
max connections reached
```

vì app leak connection hoặc traffic spike.

Senior phải monitor:

* pool size
* idle timeout
* query latency
* connection lifetime

---

**30. “Exactly once” gần như là myth**

Distributed system thực tế:

* at least once
* at most once

Exactly once thường là:

> simulate bằng idempotency + deduplication.

Đây là realization rất “senior”.

---

Sau một thời gian làm backend lớn, mindset sẽ chuyển từ:

```txt id="c9nlt5"
How to make it work?
```

sang:

```txt id="8fgn8o"
How will this fail?
```

Và đó là lúc bắt đầu bước vào engineering level cao hơn.
