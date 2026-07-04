# Redis Middle Backend Notes

Redis không chỉ là cache. Trong backend production, Redis thường nằm ở các vai trò:

* Tăng tốc read path cho dữ liệu đọc nhiều.
* Giảm tải database hoặc downstream service.
* Lưu shared state ngắn hạn cho nhiều app instances.
* Làm counter, rate limiter, quota tracker.
* Làm scheduler nhẹ cho delayed job/reminder.
* Hỗ trợ session, token blacklist, distributed lock.
* Fanout tín hiệu realtime hoặc cache invalidation.
* Dedup/idempotency trong một time window ngắn.

Mindset quan trọng:

> Dùng Redis để tăng tốc hoặc điều phối state ngắn hạn. Dùng database làm source of truth khi cần durability, audit, transaction và consistency mạnh.

---

## 1. Các tính năng/use case có thể dùng Redis

### Cache-aside cho read-heavy API

Use case:

* Profile user đọc nhiều.
* Product detail, category, menu.
* Permission/config/feature flag.
* Count/list đã tính sẵn.
* Response của endpoint tốn DB query nặng.

Flow phổ biến:

```text
READ:
API -> Redis GET key
    -> hit: trả data
    -> miss: đọc DB -> SET key TTL -> trả data

WRITE:
API -> ghi DB thành công -> DEL key liên quan
```

Thường xóa cache sau khi ghi DB thay vì update cache trực tiếp, vì update cache dễ gặp race khi nhiều writer cùng ghi.

### Session store

Khi không muốn JWT thuần stateless:

```text
session:{session_id} -> user_id, device_id, ip, user_agent, expires_at
TTL = thời gian sống của session
```

Kiểu dữ liệu hay dùng:

* `String` JSON nếu object nhỏ.
* `Hash` nếu cần update từng field như `last_active_at`.

Lưu ý:

* Session trong Redis mất thì user có thể bị logout hàng loạt nếu Redis không có persistence/replica phù hợp.
* Nếu session là security-critical, cần policy rõ: Redis outage thì fail closed hay fail open?

### JWT blacklist / token revoke

Dùng khi access token là JWT nhưng vẫn cần revoke trước khi hết hạn, ví dụ logout, đổi mật khẩu, ban account.

Pattern:

```text
blacklist:jti:{jti} -> 1
TTL = exp(token) - now
```

Kiểu dữ liệu:

* `String` với TTL là phù hợp nhất.
* Không nên gom nhiều `jti` vào một `Set` nếu mỗi token cần TTL riêng.

Trade-off:

* Mỗi request phải check Redis, làm JWT không còn hoàn toàn stateless.
* Nếu token sống ngắn 5-15 phút, blacklist size sẽ tự giảm theo TTL.
* Đăng xuất tất cả thiết bị thường dùng `token_version` trong DB/cache thay vì blacklist từng token.

### Refresh token rotation / session family

Redis có thể lưu state ngắn hạn cho refresh token:

```text
refresh:{token_id} -> user_id, session_id, used=false
session_family:{family_id} -> active/revoked
```

Use case:

* Refresh token rotation.
* Reuse detection.
* Revoke một device hoặc toàn bộ session family.

Nếu cần audit/security forensic, nên lưu bản ghi chính trong DB, Redis làm cache/fast lookup.

### Rate limiting

Use case:

* Login attempts theo IP/account.
* Public API theo user/API key/tenant.
* Endpoint đắt như search/export.
* Bảo vệ worker/downstream service.

Kiểu dữ liệu:

* `String` counter + `INCR` + `EXPIRE`: fixed window.
* `Sorted Set`: sliding window log, lưu timestamp request.
* `Hash`: gom counters theo dimension.
* Lua script: atomic check + increment + expire.

Ví dụ key:

```text
rate:login:ip:{ip}
rate:api:user:{user_id}:{window}
rate:search:tenant:{tenant_id}
```

Edge case:

* `INCR` xong crash trước `EXPIRE` có thể tạo key không TTL. Nên dùng Lua hoặc set expire khi value mới là 1.
* Distributed app instances phải dùng shared limiter, không dùng in-memory local nếu cần limit global.

### Distributed counter / quota

Use case:

* Số request trong ngày.
* Quota AI token/API usage.
* View count/like count tạm thời.
* Online users/concurrent sessions.

Kiểu dữ liệu:

* `String` + `INCRBY`.
* `HyperLogLog` cho approximate unique count, ví dụ unique visitors.
* `Bitmap` cho daily active flag theo user id số.

Lưu ý:

* Redis counter nhanh, nhưng nếu là billing/money quota thì DB phải là source of truth hoặc cần reconciliation.
* Counter có thể bị lệch khi retry/duplicate nếu operation không idempotent.

### Leaderboard, ranking, priority queue

Kiểu dữ liệu:

* `Sorted Set` (`ZADD`, `ZREVRANGE`, `ZRANK`, `ZSCORE`).

Use case:

* Leaderboard game/sales.
* Top products/search terms.
* Priority queue theo score.
* Top N realtime.
* Ranking user theo điểm.

Ví dụ key:

```text
leaderboard:daily:2026-07-04
top:search:hour:2026070413
```

### Scheduler nhẹ / delayed jobs / reminders

`Sorted Set` có thể làm scheduler nhẹ:

```text
ZADD reminders {run_at_unix_ms} reminder:{id}
Worker mỗi vài giây:
  ZRANGEBYSCORE reminders -inf now LIMIT 0 100
  claim job atomically
  process
```

Use case:

* Nhắc hẹn do user đặt trước.
* Delayed notification.
* Retry job sau backoff.
* Expire business object cần xử lý logic, không chỉ delete key.

Cần cẩn thận:

* `Sorted Set` không tự chạy job; cần worker poll.
* Nhiều worker có thể pick cùng job nếu claim không atomic. Dùng Lua script hoặc `ZPOPMIN`/move sang processing set.
* Redis restart/mất data có thể mất reminders nếu không có persistence. Reminder quan trọng nên lưu DB là source of truth, Redis là index đến hạn.
* Clock skew giữa app instances có thể làm job chạy sớm/muộn. Nên dùng một chuẩn thời gian rõ, thường lấy server time/Redis `TIME`.

Pattern an toàn hơn:

```text
DB reminders(id, run_at, status)
Redis ZSET reminder_due_index score=run_at member=id
Worker claim id -> update DB status processing with condition status=pending
Nếu Redis mất data -> rebuild ZSET từ DB pending reminders
```

### Deduplication/idempotency window

Use case:

* Webhook event đã xử lý.
* Queue message at-least-once.
* Request có `Idempotency-Key`.
* Notification không gửi duplicate trong 24h.

Pattern:

```text
SET dedup:event:{event_id} 1 NX EX 86400
-> OK: xử lý
-> nil: duplicate, bỏ qua
```

Với payment/order quan trọng, Redis marker không thay thế unique constraint trong DB.

### Distributed lock

Use case:

* Chỉ một worker rebuild cache key hot.
* Chỉ một instance chạy cron.
* Serialize job theo resource id.

Command nên dùng:

```text
SET lock:{resource_id} {owner_token} NX PX 30000
```

Khi unlock, chỉ owner mới được xóa lock, thường dùng Lua script check value rồi `DEL`.

Cần nhớ:

* Lock có thể hết hạn khi process vẫn đang chạy.
* Network pause/GC pause có thể làm owner cũ tiếp tục side effect sau khi lock đã sang owner mới.
* Nếu side effect quan trọng, cần fencing token hoặc DB conditional update.
* Nếu có thể giải bằng DB transaction/unique constraint thì thường an toàn hơn distributed lock.

### Pub/Sub cho realtime signal và cache invalidation

Use case:

* Notify app instances xóa local cache.
* WebSocket fanout nhẹ.
* Config changed.
* Event transient không cần durability.

Lưu ý:

* Redis Pub/Sub không durable. Subscriber offline thì mất message.
* Không phù hợp cho workflow business cần retry/DLQ/audit.
* Với event quan trọng nên dùng RabbitMQ/Kafka/SQS hoặc Redis Streams.

### Redis Streams cho queue nhẹ

Kiểu dữ liệu:

* Stream (`XADD`, `XREADGROUP`, `XACK`).

Use case:

* Activity/event stream nhẹ.
* Async jobs nhỏ.
* Consumer group cần ack.
* Audit ngắn hạn.

So với Pub/Sub:

* Pub/Sub: fire-and-forget, subscriber offline là mất.
* Streams: có log, consumer group, pending entries, ack.

Cần cẩn thận:

* Trim stream (`MAXLEN`) để không phình memory.
* Xử lý pending messages khi consumer chết.
* Vẫn cần idempotent consumer.

### Presence / online users

Use case:

* User online/offline.
* Active WebSocket connections.
* Device heartbeat.

Kiểu dữ liệu:

* `Set`: `online_users`.
* `String` key per connection với TTL: `presence:{user_id}:{connection_id}`.
* `Sorted Set`: score là last_seen timestamp.

Pattern:

```text
SET presence:{user_id}:{conn_id} 1 EX 60
heartbeat refresh TTL
```

### Feature flag/config cache

Use case:

* Cache permission/role.
* Tenant config.
* Feature flag.
* Pricing/config ít đổi.

Cần cẩn thận invalidation: config đổi mà cache stale có thể gây lỗi logic/security. Nên TTL ngắn + event invalidation + versioning nếu quan trọng.

---

## 2. Ứng dụng thực tế theo từng kiểu dữ liệu Redis

### String

`String` là kiểu linh hoạt nhất: value có thể là text, JSON, protobuf, integer counter hoặc marker.

Ứng dụng thực tế:

* Cache response/object: `product:{id}` -> JSON.
* JWT blacklist: `blacklist:jti:{jti}` -> `1` với TTL.
* Session đơn giản: `session:{id}` -> JSON với TTL.
* Rate limit fixed window: `rate:user:{id}:2026070410` -> counter.
* Dedup marker: `dedup:event:{event_id}` -> `1` với `SET NX EX`.
* Distributed lock: `lock:{resource}` -> owner token với `SET NX PX`.
* Feature flag/config nhỏ: `config:tenant:{tenant_id}` -> JSON.

Commands hay gặp:

* `GET`, `SET`, `MGET`, `SETEX`.
* `INCR`, `INCRBY`.
* `SET key value NX EX`.

Edge case:

* `SET` lại key có thể làm mất TTL nếu không dùng option giữ TTL.
* Value quá lớn biến thành big key, làm latency spike.

### Hash

`Hash` phù hợp cho object có nhiều field nhỏ và cần update từng field.

Ứng dụng thực tế:

* User/session metadata: `HSET session:{id} user_id ... last_active_at ...`.
* Cart nhỏ: `cart:{user_id}` field là `product_id`, value là quantity.
* Feature/config theo field: `tenant_config:{tenant_id}`.
* Rate limit nhiều dimension trong cùng window.
* Cache profile nếu cần update một field mà không serialize toàn bộ JSON.

Commands hay gặp:

* `HGET`, `HSET`, `HMGET`, `HGETALL`, `HINCRBY`.

Edge case:

* `HGETALL` trên hash rất lớn có thể block.
* TTL chỉ áp dụng cho cả key, không áp dụng riêng từng field.

### List

`List` là linked list/quicklist, mạnh cho push/pop hai đầu.

Ứng dụng thực tế:

* Queue đơn giản: `LPUSH jobs`, worker `BRPOP jobs`.
* Recent activity: lưu 100 event gần nhất bằng `LPUSH` + `LTRIM`.
* Notification inbox ngắn hạn.
* Buffer tạm cho batch processing.

Commands hay gặp:

* `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `BRPOP`, `LRANGE`, `LTRIM`.

Edge case:

* Queue bằng List không có ack/pending message tốt như Streams.
* Worker pop xong crash thì job có thể mất nếu không có processing list/idempotency.
* `LRANGE 0 -1` trên list lớn dễ gây latency spike.

### Set

`Set` lưu collection unique, không có thứ tự.

Ứng dụng thực tế:

* Membership: user đã join group nào, group có những user nào.
* Role/permission set: `user_roles:{user_id}`.
* Unique tags/categories.
* Dedup trong batch nhỏ.
* Mutual friends/common items bằng intersection.
* Online users nếu không cần TTL theo từng connection.
* Feature rollout: set user id được bật feature.

Commands hay gặp:

* `SADD`, `SREM`, `SISMEMBER`, `SMEMBERS`.
* `SINTER`, `SUNION`, `SDIFF`.
* `SCARD`.

Edge case:

* TTL áp dụng cho cả set, không áp dụng từng member.
* `SMEMBERS` set lớn có thể block; dùng `SSCAN` hoặc thiết kế pagination.
* Nếu cần score/rank/time ordering thì dùng `Sorted Set`, không dùng `Set`.

### Sorted Set

`Sorted Set` là set unique member nhưng mỗi member có score. Đây là kiểu rất hay cho backend vì vừa unique vừa có thứ tự.

Ứng dụng thực tế:

* Leaderboard: score là điểm.
* Ranking/top N: score là số lượt xem/doanh thu.
* Scheduler/reminder: score là `run_at` timestamp.
* Delayed retry queue: score là thời điểm retry tiếp theo.
* Sliding window rate limit: member là request id/timestamp, score là timestamp.
* Priority queue: score là priority hoặc deadline.
* Presence last seen: score là `last_seen`.
* Time-series index nhẹ: member là event id, score là event time.

Commands hay gặp:

* `ZADD`, `ZRANGE`, `ZREVRANGE`, `ZRANGEBYSCORE`.
* `ZPOPMIN`, `ZPOPMAX`.
* `ZRANK`, `ZREVRANK`, `ZSCORE`.
* `ZREM`, `ZREMRANGEBYSCORE`.

Edge case:

* Member phải unique; nếu cùng member `ZADD` lại thì score bị update.
* Scheduler cần claim atomic để nhiều worker không xử lý cùng job.
* Sorted set quá lớn cần cleanup, shard theo ngày/tenant hoặc archive.

Ví dụ reminder:

```text
ZADD reminders 1783152000000 reminder:123
ZRANGEBYSCORE reminders -inf now LIMIT 0 100
```

Ví dụ sliding window:

```text
ZADD rate:user:42 now_ms request_id
ZREMRANGEBYSCORE rate:user:42 -inf now_ms-window
ZCARD rate:user:42
```

### Bitmap

`Bitmap` dùng string như mảng bit, rất tiết kiệm memory nếu id là số nguyên liên tục/tương đối nhỏ.

Ứng dụng thực tế:

* Daily active users: `SETBIT dau:2026-07-04 {user_id} 1`.
* User đã check-in ngày nào.
* Feature exposure flag theo user id.
* Attendance/visited flags.

Commands hay gặp:

* `SETBIT`, `GETBIT`, `BITCOUNT`, `BITOP`.

Edge case:

* Nếu user id rất lớn và sparse, bitmap có thể phình memory.
* Không phù hợp nếu id là UUID string.

### HyperLogLog

`HyperLogLog` dùng để đếm approximate unique với memory rất nhỏ.

Ứng dụng thực tế:

* Unique visitors theo ngày.
* Unique search users.
* Unique IP/user xem campaign.
* Estimate reach của notification/campaign.

Commands hay gặp:

* `PFADD`, `PFCOUNT`, `PFMERGE`.

Edge case:

* Kết quả là approximate, không dùng cho billing/finance.
* Không lấy lại được danh sách member, chỉ đếm.

### GEO

`GEO` hỗ trợ lưu tọa độ và query khoảng cách/gần nhất.

Ứng dụng thực tế:

* Store/restaurant gần user.
* Driver/shipper gần đơn hàng.
* Location-based matching ở quy mô nhỏ-vừa.

Commands hay gặp:

* `GEOADD`, `GEODIST`, `GEOSEARCH`.

Edge case:

* Không thay thế PostGIS/Elasticsearch cho bài toán geospatial phức tạp.
* Cần cân nhắc privacy nếu lưu location realtime.

### Stream

`Stream` giống append-only log có consumer group và ack.

Ứng dụng thực tế:

* Queue nhẹ cần ack.
* Activity feed/event log ngắn hạn.
* Async worker nội bộ.
* Audit stream tạm.
* Pipeline ingestion nhỏ.

Commands hay gặp:

* `XADD`, `XREAD`, `XREADGROUP`.
* `XACK`, `XPENDING`, `XCLAIM`.
* `XTRIM`.

Edge case:

* Stream cần trim để không ăn memory.
* Consumer chết tạo pending entries, cần recovery.
* Vẫn là at-least-once, consumer phải idempotent.

### Pub/Sub

Pub/Sub là messaging transient, không phải queue durable.

Ứng dụng thực tế:

* Broadcast cache invalidation.
* Notify app instances reload config.
* WebSocket fanout nhẹ.
* Signal realtime không quan trọng nếu mất.

Edge case:

* Subscriber offline là mất message.
* Không có ack, retry, DLQ.
* Không dùng cho payment/order/business workflow quan trọng.

---

## 3. Những thứ bạn có thể đã dùng Redis mà không nhận ra

Từ các notes hiện tại:

* U2U: `Redis/PostgreSQL/RabbitMQ integration` có thể gồm cache, session, distributed counter/rate limit hoặc shared state cho proxy platform.
* RAG project: Redis worker/cache có thể là cache-aside cho result, job queue nhẹ, dedup ingestion job, rate limit API model, hoặc lock khi ingest cùng document.
* Backend security notes: brute-force login counter là Redis rate limit theo IP/account.
* JWT/session notes: `jti` blacklist với TTL là Redis `String` key; token version/session lookup có thể cache bằng Redis.
* Idempotency/webhook/queue notes: processed event ID với TTL có thể dùng Redis `SET NX EX`, nhưng DB unique constraint vẫn tốt hơn cho payment/order.
* Cron/background job notes: persisted scheduler có thể dùng DB source of truth + Redis `Sorted Set` làm due index.
* Scale/system design notes: cache layer, distributed cache, stateless app cần shared Redis cho session/cache/rate limit.

---

## 4. Issues và edge cases cần nhớ

### Cache invalidation sai

Vấn đề:

* Ghi DB thành công nhưng quên xóa key.
* Xóa sai key pattern.
* Data có nhiều view/list/detail cache, chỉ invalidate detail.
* Cache update trực tiếp bị writer cũ ghi đè writer mới.

Giảm rủi ro:

* Đặt key naming convention rõ.
* Write DB trước, sau đó delete cache.
* TTL là safety net, không phải chiến lược invalidation duy nhất.
* Với data nhạy cảm, dùng version trong key: `user:{id}:v{version}`.

### Race condition read/write

Case:

```text
Request A cache miss -> đọc DB old value
Request B update DB new value -> DEL cache
Request A SET cache old value với TTL
```

Mitigation:

* TTL ngắn hơn cho data dễ race.
* Double delete: delete cache sau write, delay nhỏ rồi delete lại.
* Singleflight/lock khi rebuild cache key hot.
* Versioned cache: value kèm `updated_at/version`, không set nếu version cũ hơn.

### Cache stampede

Nhiều request cùng miss một key hot, tất cả cùng đánh DB.

Mitigation:

* Singleflight trong cùng process.
* Distributed lock ngắn cho rebuild.
* Stale-while-revalidate: trả stale data ngắn hạn và refresh background.
* TTL jitter.
* Warmup key hot.

### Cache penetration

Request vào key không tồn tại liên tục, cache luôn miss và đánh DB.

Mitigation:

* Cache null/negative result TTL ngắn.
* Validate input sớm.
* Rate limit.
* Bloom filter nếu keyspace lớn và bị scan.

### Cache avalanche

Nhiều key hết hạn cùng lúc làm DB bị dồn tải.

Mitigation:

* TTL jitter.
* Không deploy/warm cache bằng cùng TTL cố định cho hàng loạt key.
* Prewarm hot keys.
* Degrade/rate limit khi cache miss tăng đột biến.

### Hot key

Một key quá nóng làm một Redis shard/instance bị nghẽn.

Ví dụ:

* Global config.
* Top product.
* Viral post.
* Leaderboard hot.
* Permission của tenant lớn.

Mitigation:

* Local in-memory cache TTL rất ngắn.
* Read replicas.
* Split key/shard logical nếu có thể.
* Cache tại CDN/edge cho public data.
* Request coalescing.

### Big key

Key quá lớn làm command chậm và block event loop.

Ví dụ:

* `Set`/`List` có hàng triệu item.
* JSON string vài MB.
* `Hash` quá nhiều field.

Rủi ro:

* Latency spike.
* Memory fragmentation.
* Slow replication.
* `DEL` key lớn block lâu.

Mitigation:

* Paginate/chunk key.
* Dùng `SCAN` thay vì `KEYS`.
* Dùng `UNLINK` thay vì `DEL` cho key lớn nếu phù hợp.
* Monitor big keys.

### Eviction và memory pressure

Redis hết memory sẽ xử lý theo `maxmemory-policy`:

* `noeviction`: write bị lỗi.
* `allkeys-lru/lfu`: evict cả key không TTL.
* `volatile-lru/ttl`: chỉ evict key có TTL.

Edge case:

* Session/JWT blacklist bị evict có thể thành security bug.
* Cache key evict thì chỉ làm miss, nhưng lock/dedup key evict có thể tạo duplicate side effect.

Nên tách Redis theo mục đích nếu cần:

* cache volatile;
* session/security;
* queue/stream;
* lock/rate limit.

### Persistence không giống database

Redis có RDB/AOF nhưng không nên coi như DB chính cho business critical data nếu chưa thiết kế kỹ.

Cần biết:

* RDB snapshot có thể mất data giữa hai lần snapshot.
* AOF everysec có thể mất khoảng 1 giây write.
* AOF always chậm hơn.
* Replica async có thể lag.

Reminder, token/session, queue job quan trọng cần DB source of truth hoặc queue durable.

### Replica lag / failover

Rủi ro:

* Write vào primary, read từ replica chưa thấy data.
* Failover mất write vừa ack nếu replication async.
* Lock trên Redis failover có thể nguy hiểm.

Mitigation:

* Đọc critical-after-write từ primary.
* Chấp nhận eventual consistency cho cache.
* Không dùng Redis lock cho side effect tài chính nếu không có fencing/DB guard.

### Command blocking

Cần tránh:

* `KEYS *` trên production.
* `SMEMBERS` set cực lớn.
* `LRANGE 0 -1` list lớn.
* Lua script chạy quá lâu.
* Transaction/pipeline quá lớn.

Dùng:

* `SCAN`/`SSCAN`/`HSCAN`.
* Pagination.
* Key size limit.
* Slowlog/latency monitor.

### TTL semantics

Edge case:

* Update key bằng `SET` có thể làm mất TTL nếu không dùng option giữ TTL.
* `EXPIRE` không atomic với `SET` nếu gọi tách rồi crash giữa chừng.
* TTL quá ngắn làm miss nhiều; TTL quá dài làm stale data.
* Random TTL quá rộng có thể làm data quan trọng stale quá lâu.

Dùng:

* `SET key value EX seconds`.
* `SET key value NX EX seconds` cho lock/dedup.
* Kiểm tra TTL trong test cho các key cần auto expire.

### Serialization/schema evolution

Cache JSON/protobuf/msgpack có thể bị lỗi khi deploy version mới.

Mitigation:

* Version trong key: `product:v2:{id}`.
* Backward-compatible parser.
* TTL ngắn trong giai đoạn migrate.
* Invalidate namespace cũ sau deploy.

### Observability thiếu

Cần monitor:

* Cache hit/miss rate theo endpoint/key group.
* Redis latency p95/p99.
* Memory used, fragmentation, evicted keys, expired keys.
* Connected clients, blocked clients.
* Command rate.
* Slowlog.
* Keyspace size.
* Replication lag.
* Rate limit rejects.
* Lock acquire failure/time held.

### Security

Rủi ro:

* Redis expose public internet.
* Key chứa PII/token raw.
* Shared Redis không namespace/ACL.
* Command nguy hiểm bị dùng nhầm.

Cần:

* Private network/security group.
* TLS/auth/ACL nếu phù hợp.
* Prefix key theo service/env/tenant.
* Không lưu access token raw nếu không cần.
* Encrypt/hash sensitive identifiers khi cần.

---

## 5. Cache strategies

### Cache-aside / lazy loading

App tự đọc cache, miss thì đọc DB và set cache.

Phù hợp:

* Data đọc nhiều.
* Chấp nhận stale ngắn.
* Invalidation theo write path làm được.

Pros:

* Đơn giản.
* Cache chỉ chứa data thật sự được đọc.
* Redis down có thể fallback DB nếu tải chịu được.

Cons:

* Request đầu tiên miss chậm.
* Dễ stampede.
* Invalidation phải tự làm.

### Read-through cache

App hỏi cache layer, cache layer tự load DB khi miss.

Phù hợp khi có cache abstraction/library rõ. Redis thuần không tự read-through nếu không có app layer wrapper.

### Write-through cache

Write vào cache và DB cùng lúc, hoặc cache layer ghi xuống DB.

Pros:

* Cache gần như luôn mới.
* Read sau write nhanh.

Cons:

* Write latency tăng.
* Cần xử lý failure khi DB thành công/cache fail hoặc ngược lại.
* Phức tạp hơn cache-aside.

### Write-around cache

Write chỉ vào DB, không update cache. Lần read sau miss thì load lại.

Đây là cách hay dùng với cache-aside write path:

```text
WRITE DB -> DEL cache
READ miss -> load DB -> SET cache
```

Phù hợp với data write nhiều/đọc ít, tránh làm cache đầy data ít được đọc.

### Write-back / write-behind

Write vào cache trước, async flush xuống DB sau.

Phù hợp:

* Analytics/counter tạm thời.
* Buffer write high throughput.
* Chấp nhận mất/lệch data trong window nhỏ.

Không phù hợp:

* Payment/order/inventory critical.
* Data cần transaction/audit mạnh.

Rủi ro:

* Redis crash trước khi flush.
* Duplicate flush.
* Ordering conflict.
* DB fail làm backlog.

### TTL-only cache

Chỉ set TTL, không chủ động invalidate.

Phù hợp:

* Data ít đổi.
* Stale ok trong vài giây/phút.
* MVP/đơn giản.

Không phù hợp:

* Permission/security.
* Inventory/price cần gần realtime.
* User-specific sensitive state.

### Event-driven invalidation

Sau khi DB thay đổi, publish event để các service/cache xóa key.

```text
ProductUpdated(id)
-> consumers DEL product:{id}, product_list:category:{category_id}
```

Dùng khi:

* Nhiều service có cache riêng.
* Có local cache + Redis cache.
* Invalidation cần fanout.

Cần nhớ:

* Pub/Sub không durable; message mất thì cache stale đến TTL.
* Event quan trọng nên qua durable queue/stream hoặc TTL ngắn làm safety net.

### Stale-while-revalidate

Khi data hết TTL mềm:

* Trả stale data nếu còn trong hard TTL.
* Một worker/request refresh background.
* User không bị latency spike.

Phù hợp:

* Homepage/list/top products.
* Analytics count.
* Config public.
* Data đọc cực nhiều.

Không phù hợp nếu stale data gây lỗi nghiêm trọng.

### Cache warming/preloading

Preload key hot sau deploy/restart hoặc trước campaign.

Phù hợp:

* Menu/category hot.
* Landing page.
* Leaderboard/top list.
* Pricing/config public.

Cần thêm jitter để tránh tất cả key expire cùng lúc.

### Multi-level cache

Tầng cache:

```text
in-process local cache -> Redis distributed cache -> DB
```

Pros:

* Giảm Redis round-trip cho hot keys.
* Chống hot key.

Cons:

* Invalidation khó hơn.
* Mỗi instance có bản stale riêng.
* Cần TTL rất ngắn hoặc event invalidation.

### Negative caching

Cache cả kết quả không tồn tại:

```text
product:{id} -> "__NULL__" TTL 30s
```

Phù hợp:

* Chống scan random id.
* Endpoint detail bị bot hit key không tồn tại.

Cần TTL ngắn để object mới tạo không bị ẩn quá lâu.

---

## 6. Cách chọn Redis data type nhanh

| Bài toán | Data type nên nghĩ tới |
|---|---|
| Cache object/response | `String` JSON/protobuf, `Hash` |
| Session | `String`/`Hash` + TTL |
| JWT blacklist theo `jti` | `String` + TTL |
| Rate limit fixed window | `String` counter + TTL |
| Rate limit sliding log | `Sorted Set` |
| Leaderboard/ranking | `Sorted Set` |
| Reminder/delayed job | `Sorted Set` score = run_at |
| Dedup/idempotency ngắn hạn | `String` + `SET NX EX` |
| Membership/tag/user ids | `Set` |
| Queue đơn giản | `List` hoặc `Stream` |
| Durable-ish queue nhẹ | `Stream` + consumer group |
| Pub/Sub realtime signal | Pub/Sub channel |
| Unique approximate count | `HyperLogLog` |
| Boolean flags theo user id số | `Bitmap` |
| Nearby location nhỏ-vừa | `GEO` |
| Lock ngắn hạn | `String` + `SET NX PX` + owner token |

---

## 7. Redis interview answer mẫu

### "Bạn đã dùng Redis để làm gì?"

> Tôi dùng Redis không chỉ cho cache-aside mà còn cho shared state ngắn hạn như rate limit counter, session/token blacklist với TTL, dedup key cho event/job, và một số flow async nhẹ. Nếu làm reminder/delayed job, tôi có thể dùng sorted set với score là thời điểm chạy, nhưng với reminder quan trọng tôi vẫn để DB làm source of truth và Redis chỉ là due index có thể rebuild.

### "Cache invalidation xử lý thế nào?"

> Pattern cơ bản là write DB thành công rồi delete cache key liên quan. Tôi không update cache trực tiếp nếu có nhiều writer vì dễ race condition. Với list/detail cache, tôi đặt key convention rõ và invalidate cả các view liên quan. TTL là safety net. Nếu consistency cần chặt hơn, tôi dùng versioned key hoặc đọc từ DB cho critical path.

### "Redis có những lỗi production nào?"

> Các lỗi hay gặp là stale cache, stampede, penetration, avalanche, hot key, big key, eviction làm mất key quan trọng, TTL bị mất khi SET lại key, Pub/Sub mất message, replica lag/failover và dùng command blocking như KEYS. Tôi sẽ monitor hit rate, latency p95/p99, memory/eviction, slowlog, key size và replication lag.

### "Sorted set làm cron/reminder như thế nào?"

> Lưu reminder id vào sorted set, score là run_at timestamp. Worker poll các item có score <= now, claim atomically bằng Lua hoặc ZPOPMIN/move sang processing, sau đó xử lý. Nhưng Redis không nên là source of truth cho reminder quan trọng; DB lưu reminder/status, Redis index đến hạn. Nếu Redis mất data thì rebuild sorted set từ DB.

### "JWT blacklist dùng Redis kiểu gì?"

> Mỗi JWT nên có `jti`. Khi revoke token, set key `blacklist:jti:{jti} = 1` với TTL bằng thời gian còn lại của token. Mỗi request validate signature/exp xong check Redis xem `jti` có bị blacklist không. Dùng String key có TTL tốt hơn Set, vì mỗi token có TTL riêng.
