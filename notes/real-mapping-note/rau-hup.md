# Interview Notes — Map lý thuyết Backend vào codebase rau-hup

> Mục đích: dùng chính codebase này làm "ví dụ thật" khi trả lời phỏng vấn về Redis, Message Queue, Index, Materialized View, Replication.
>
> **3 điều phải nhớ trước khi vào phòng phỏng vấn** (rất dễ bị hỏi hớ):
> 1. DB ở đây là **MySQL/InnoDB**, KHÔNG phải PostgreSQL.
> 2. Message broker thật sự đang chạy là **Temporal**, KHÔNG phải RabbitMQ. RabbitMQ chỉ còn dấu vết qua Celery ở `bts-core` (code chết).
> 3. **Không có Materialized View và không có Read Replica** trong repo này. Nhưng biết *chỗ nào đáng lẽ nên có* còn ghi điểm hơn là bịa ra là có.

---

## 0. Bản đồ hệ thống

| Service | Ngôn ngữ | Vai trò | DB | Hạ tầng |
|---|---|---|---|---|
| `bts-core` | Python / Django | Core business (product, batch, org, access, blockchain indexer) | MySQL (Django ORM) | Celery (legacy) |
| `bts-core-service` | Go / Gin | Cầu nối Django → Temporal. Nhận HTTP, submit workflow | MySQL (GORM) | Temporal |
| `notification-service` | Go | Gửi email (~25 loại) | — | Temporal |
| `payment-service` | Go / Gin + fx DI | Stripe billing, subscription | MySQL (GORM) | Temporal + Redis (đã wire, chưa dùng) |
| `bts-data-collector` | Go / Gin | Nhận tracking event từ web (scan QR sản phẩm) | MySQL (GORM) | **Redis** |
| `bts-data-analytics` | Go | API đọc thống kê từ dữ liệu collector | MySQL (raw `database/sql`) | — |

Luồng chính:

```
Browser (quét QR)                Django Console (bts-core)
      |                                    |
      | POST /verify/entry                 | POST /v1/index-search
      | POST /log/event                    | POST /v1/sync-data-service
      v                                    | POST /v1/notification
bts-data-collector ---> Redis              | POST /v1/assets
      |                                    v
      v                            bts-core-service ---> Temporal ---> Workers
   MySQL (event_logs)                                                  |
      ^                                                                v
      |                                                    search-service / data-service
bts-data-analytics (đọc)                                    / notification-service
```

---

# PHẦN 1 — REDIS

## 1.1 Redis được dùng ở đâu?

**Chỉ 1 chỗ đang thật sự chạy**: `bts-data-collector`.

| File | Vai trò |
|---|---|
| `bts-data-collector/utils/redis_cache.go` | Client + hàm `GetOrSetCacheRedis` |
| `bts-data-collector/cmd/app/main.go:49` | Khởi tạo: `utils.InitRedisCache(host, port)` |
| `bts-data-collector/internal/api/collection/api.go:58` | Dùng trong `POST /verify/entry` |
| `bts-data-collector/internal/api/collection/api.go:137` | Dùng trong `POST /log/event` |

**Một chỗ nữa đã wire nhưng chưa dùng**: `payment-service`
- `payment-service/pkg/redis.go` — `redisProvider.Connect()` với pool config đầy đủ
- `payment-service/di/redis.go` — provide qua Uber fx
- ⚠️ Grep toàn service không thấy chỗ nào **inject** `*redis.Client` vào handler/service nào cả. Đây là dependency chết. → **Điểm hay để nói trong phỏng vấn**: "em phát hiện Redis được provide qua DI container nhưng không có consumer, tức là mỗi lần boot service nó vẫn mở connection pool + ping mà không ai dùng."

## 1.2 Redis cache CÁI GÌ? (câu trả lời quan trọng nhất)

**Bẫy:** nhìn tên hàm `GetOrSetCacheRedis` thì tưởng là caching. Nhưng đọc kỹ call-site thì **nó không phải cache để tăng tốc — nó là idempotency key / anti-spam (deduplication)**.

Bằng chứng: khi cache HIT, code log `utils.Warn("Blocked by anti-spam", ...)`. Cache hit = **chặn request**, không phải "trả nhanh hơn".

### Cấu trúc key

```
antispam_entry:<uid>:<sha_hash(raw_request_body)>
antispam_event:<uid>:<sha_hash(raw_request_body)>
```

- `uid` = user id ẩn danh (ksuid, prefix theo user class), lấy từ HTTP header
- `hash` = băm nguyên văn raw body → cùng 1 người + cùng 1 payload = cùng 1 key

### Value lưu gì (JSON thật)

Struct `CachedResponse` trong `redis_cache.go`:

```go
type CachedResponse struct {
    Value      any `json:"value"`
    StatusCode int `json:"status_code"`
}
```

Ví dụ 1 record thật trong Redis — sau khi user quét QR thành công:

```
KEY:   antispam_entry:gh_2ZxK9mQpLwR3nT8vB1cYdF4eA:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08
TTL:   600 (10 phút)
VALUE: {
         "value": {
           "status": "OK",
           "page_id": "view_01HQ8ZP3KM"
         },
         "status_code": 200
       }
```

Ví dụ 2 — event log:

```
KEY:   antispam_event:gh_2ZxK9mQpLwR3nT8vB1cYdF4eA:c3ab8ff13720e8ad9047dd39466b3c89...
TTL:   600
VALUE: {
         "value": { "status": "OK" },
         "status_code": 200
       }
```

Ví dụ 3 — trường hợp lỗi (**KHÔNG được cache**, xem 1.4):

```
Generator trả về: { "error": "invalid batch nft id" }, 400
→ status 400 không nằm trong [200,300) → KHÔNG SET vào Redis
→ request sau với cùng payload sẽ chạy lại generator
```

## 1.3 Tại sao phải cache/dedupe cái này?

Đây là hệ thống truy xuất nguồn gốc nông sản. Người dùng quét QR trên bao bì → browser bắn event. Vấn đề:

- Browser SPA re-render → bắn trùng `POST /log/event`
- User F5 liên tục → mỗi lần là 1 row `event_logs` mới
- Bot/script spam endpoint public (không có auth)

Nếu không chặn, số liệu analytics (`total_scans`, `unique_users`, `avg_duration`) bị **thổi phồng**. Redis đứng chắn trước MySQL, **bảo vệ tính đúng đắn của số liệu + giảm write load xuống DB**.

TTL mặc định: `redis_ttl: 10m` (`bts-data-collector/internal/config/config.go:32`), override bằng env `REDIS_TTL`.

## 1.4 Pattern kỹ thuật để nói tên ra

| Pattern | Ở đâu trong code |
|---|---|
| **Cache-Aside (Lazy Loading)** | `GetOrSetCacheRedis`: GET → miss → gọi `generator()` → SET |
| **Idempotency key** | key = `uid + hash(body)`, y hệt `Idempotency-Key` header của Stripe |
| **Negative caching bị chủ động tắt** | `if status >= 200 && status < 300` → chỉ cache thành công. Lỗi 4xx/5xx không cache ⇒ client retry được ngay |
| **Fail-open** | Nếu `Get` lỗi (Redis chết) → coi như miss → vẫn chạy generator. Redis down **không** làm sập API |
| **Graceful JSON degradation** | `json.Unmarshal` lỗi → coi như miss, không panic |

```go
// bts-data-collector/utils/redis_cache.go — rút gọn
func GetOrSetCacheRedis(ctx, key, ttl, generator) (any, int, bool) {
    data, err := RedisCache.Get(ctx, key).Result()
    if err == nil {                                  // ← HIT
        var cached CachedResponse
        if json.Unmarshal([]byte(data), &cached) == nil {
            return cached.Value, cached.StatusCode, true   // hit=true → BLOCK
        }
    }
    value, status := generator()                     // ← MISS: làm thật (ghi MySQL)
    if status >= 200 && status < 300 {               // ← chỉ cache success
        bytes, _ := json.Marshal(CachedResponse{value, status})
        RedisCache.Set(ctx, key, bytes, ttl)
    }
    return value, status, false
}
```

## 1.5 Điểm yếu để tự phê bình (giám khảo rất thích)

1. **Race condition**: 2 request song song cùng key đều MISS → cả 2 cùng ghi MySQL. Đây là *cache stampede*. Fix: dùng `SET key val NX EX ttl` (atomic) làm distributed lock thay vì GET-rồi-SET.
2. `DB: 0`, `Password: ""` hardcode trong `InitRedisCache` — không config được, không auth.
3. Không có pool tuning (`PoolSize`, `MinIdleConns`) — ngược lại `payment-service/pkg/redis.go` làm rất đúng nhưng lại không dùng.
4. Không set `maxmemory-policy`. Với key có TTL thì `volatile-lru` là hợp lý.
5. Không có metric hit/miss ratio — chỉ có log `Warn`.

## 1.6 Nếu được thiết kế lại — Redis còn nên cache gì?

Chỗ **đáng cache nhất mà hiện tại không cache**: `bts-data-analytics`. Toàn bộ API analytics chạy `COUNT`/`GROUP BY`/`AVG` trên `event_logs` **mỗi request**, không cache gì cả.

```
KEY:   analytics:total_scan:0xAbC123...:batches=[101,102]:2026-01-01_2026-06-30:countries=VN,US
TTL:   300
VALUE: {"value":{"total_scan":18432},"status_code":200}
```

Chỗ thứ hai: `ipgeo.ResolveGeolocation(ip)` — gọi mỗi event. IP → geo là quan hệ gần như bất biến, cache 24h là chuẩn (`geo:ip:1.2.3.4`).

---

# PHẦN 2 — INDEX (COMPOUND INDEX)

## 2.1 Compound index CÓ tồn tại — nhưng dưới dạng `unique_together` của Django

Đây là câu trả lời chính. Django `unique_together` **sinh ra một UNIQUE composite index** ở tầng MySQL. Nhiều dev không biết điều này và trả lời "không có compound index" — sai.

| Model | File | Compound index (thứ tự cột quan trọng!) |
|---|---|---|
| `dexer.Event` | `bts-core/dexer/models/events.py:39` | `(block_number, log_index, transaction_index)` |
| `dexer.Balance` | `bts-core/dexer/models/inventory.py:108` | `(account, contract_address, token_id)` |
| `core.Membership` | `bts-core/core/models/memberships.py:65` | `(user, organisation)` |
| `core.Invitation` | `bts-core/core/models/organisations.py:376` | `(organisation, email)` |
| `AccessGroupPolicy` | `bts-core/core/models/access.py:343` | `(group, policy)` |
| `AccessGroupKey` | `bts-core/core/models/access.py:508` | `(group, key)` |
| `AccessPolicyKey` | `bts-core/core/models/access.py:531` | `(policy, key)` |
| `AccessGroupMembership` | `bts-core/core/models/access.py:554` | `(group, membership)` |
| `AccessPolicyMembership` | `bts-core/core/models/access.py:588` | `(policy, membership)` |
| `ProductBatchAsset` | `bts-core/core/models/products.py:1300` | `(batch, asset)` |
| `AssetAttribute` | `bts-core/core/models/assets.py:248` | `(asset, key)` |
| `Thumbnail` | `bts-core/core/models/assets.py:344` | `(asset, max_size)` |

Và các **single-column index**:

| Index | File |
|---|---|
| `Index(fields=['contract_address'])` | `dexer/models/events.py:37` |
| `Index(fields=['token_id'])` | `dexer/models/inventory.py:106` |
| `Index(fields=['deployed_chain'])`, `Index(fields=['blockchain_address'])` | `core/models/entities.py:99-100` |
| `Index(fields=['name'])` | `core/models/assets.py:279` |
| `Index(fields=['max_size'])` | `core/models/assets.py:342` |
| `idx_product_address (product_addr)` | `bts-data-analytics/migrations/000003_*.up.sql:8` |
| `idx_batch_nft_id (batch_nft_id)` | `bts-data-analytics/migrations/000003_*.up.sql:11` |
| `idx_pb_mapping_id`, `idx_event_id` | `bts-data-analytics/migrations/000004_*.up.sql:12,14` |

## 2.2 Compound index nào tối ưu cho query nào? — Ví dụ đắt giá nhất

### `dexer.Event`: `UNIQUE (block_number, log_index, transaction_index)`

Đây là bảng lưu event đọc từ blockchain. Index này phục vụ **2 mục đích cùng lúc**:

**(a) Đảm bảo tính đúng đắn — chống double-ingest.**
Blockchain indexer có thể re-scan cùng 1 block (restart, chain reorg, crash-and-resume). Bộ ba `(block_number, log_index, transaction_index)` **định danh duy nhất một log trên chain**. UNIQUE constraint biến việc chống trùng thành trách nhiệm của DB thay vì của application code — nếu ghi trùng, MySQL ném `IntegrityError`. Đây là **idempotency ở tầng DB**, cùng ý tưởng với Redis key ở Phần 1.

**(b) Tăng tốc query resume.**
```sql
-- Indexer khởi động lại, hỏi "đã index tới block nào rồi?"
SELECT MAX(block_number) FROM dexer_event;
-- → leftmost prefix (block_number) → MySQL đọc ngược index, O(1). Không index thì full table scan.

-- Đọc tất cả event trong 1 block
SELECT * FROM dexer_event WHERE block_number = 45231000;
-- → leftmost prefix hit.
```

**Leftmost prefix rule** — điểm chốt phải nói:
```
Index (block_number, log_index, transaction_index) phục vụ được:
  WHERE block_number = ?                                       ✅
  WHERE block_number = ? AND log_index = ?                     ✅
  WHERE block_number = ? AND log_index = ? AND transaction_index = ?  ✅
KHÔNG phục vụ được:
  WHERE log_index = ?                                          ❌ (bỏ qua cột đầu)
  WHERE transaction_index = ?                                  ❌
```
Vì vậy mới cần thêm `Index(fields=['contract_address'])` riêng ở dòng 37 — query "lấy hết event của contract X" không thể tận dụng compound index kia.

### `dexer.Balance`: `UNIQUE (account, contract_address, token_id)`

Query trong `inventory.py:89-95`:
```python
sender_.balances.select_for_update().filter(
    contract_address=event.contract_address, token_id=token_id
).update(value=F('value') - quantity)
```
Django dịch ra:
```sql
SELECT ... FROM dexer_balance
WHERE account_id = ? AND contract_address = ? AND token_id = ?
FOR UPDATE;
```
→ Khớp **chính xác cả 3 cột** theo đúng thứ tự index ⇒ index lookup trả về đúng 1 row.

⚠️ Điểm cực quan trọng cho phỏng vấn: `SELECT ... FOR UPDATE` trong InnoDB **khoá theo index range mà nó quét**. Nếu không có index này, InnoDB phải scan toàn bảng và sẽ **khoá gần như mọi row** → deadlock hàng loạt khi nhiều transfer NFT chạy song song. Ở đây, index đảm bảo chỉ khoá đúng 1 row. Đây là ví dụ **index ảnh hưởng tới concurrency, không chỉ tới speed** — trả lời được ý này là ăn điểm cao.

Thêm: `value=F('value') - quantity` là **atomic update ngay trong SQL** (`SET value = value - 5`), không phải đọc-về-Python-rồi-ghi-lại. Tránh lost update.

## 2.3 Compound index CÒN THIẾU — nơi query đang chậm

Đây là phần "thể hiện tư duy tối ưu". Toàn bộ `bts-data-analytics/internal/database/analytics_query.go` là **8 query dashboard**, tất cả đều theo đúng một khuôn:

```sql
-- buildTotalScanQuery / buildDailyScanQuery / buildTopCountriesQuery /
-- buildAvgDurationQuery / buildTotalUniqueUsersQuery / buildDailyUniqueUsersQuery / buildTopBatchesQuery
SELECT COUNT(e.view_id)
FROM event_logs AS e
JOIN product_batch_mapping AS d ON e.id = d.event_id
WHERE d.product_addr = ?
  AND d.batch_nft_id IN (?, ?, ?)
  AND e.event_date >= FROM_UNIXTIME(?) AND e.event_date <= FROM_UNIXTIME(?)
  AND e.country_code IN ('VN','US')
```

### Vấn đề 1 — `product_batch_mapping` chỉ có 2 index rời

Hiện tại: `idx_product_address(product_addr)` và `idx_batch_nft_id(batch_nft_id)` — **2 index đơn, riêng biệt**.

MySQL chỉ chọn được **1 index cho 1 bảng** trong đa số trường hợp (index merge có nhưng optimizer hay bỏ qua với `IN` list). Nên nó dùng `idx_product_address`, lấy về **toàn bộ** row của product đó (có thể hàng trăm nghìn), rồi lọc `batch_nft_id` và JOIN từng row bằng cách lookup ngược lại clustered index (`id`) — tốn rất nhiều random I/O.

**Đề xuất:**
```sql
CREATE INDEX idx_pbm_product_batch_event
  ON product_batch_mapping (product_addr, batch_nft_id, event_id);
```
Vì sao thứ tự này:
1. `product_addr` — luôn là **equality** (`= ?`), nằm đầu. Selectivity cao nhất.
2. `batch_nft_id` — điều kiện **range/IN**, đặt sau equality. (Quy tắc: equality trước, range sau.)
3. `event_id` — không dùng để lọc, nhưng là **cột JOIN**. Đưa vào đuôi index biến nó thành **covering index**: MySQL đọc xong 3 cột này là có đủ mọi thứ nó cần từ bảng `product_batch_mapping`, **không cần đọc bảng gốc lần nào** (`EXPLAIN` sẽ hiện `Using index`).

Bảng này chỉ có 5 cột nên index này gần như là bản sao thu nhỏ của bảng — cực rẻ và cực nhanh.

### Vấn đề 2 — `event_logs` không có index nào cả

Sau khi JOIN, mọi query đều lọc thêm trên `event_logs`:
- `e.event_date BETWEEN ? AND ?` — mọi query
- `e.country_code IN (...)` — mọi query
- `e.action = 'duration'` — `buildAvgDurationQuery`, `buildDailyAvgDurationQuery`
- `COUNT(DISTINCT e.user_uid)` — `buildTotalUniqueUsersQuery`

Bảng này là bảng **write-heavy nhất hệ thống** (mọi lượt quét QR đều ghi 1 row) và **không có một index nào ngoài PK**.

**Đề xuất:**
```sql
-- Cho các query lọc theo khoảng ngày + quốc gia (đa số dashboard)
CREATE INDEX idx_ev_date_country ON event_logs (event_date, country_code);

-- Cho 2 query tính duration
CREATE INDEX idx_ev_action_date ON event_logs (action, event_date);

-- Nếu muốn COUNT(DISTINCT user_uid) theo ngày nhanh
CREATE INDEX idx_ev_date_user ON event_logs (event_date, user_uid);
```

Lý do `(event_date, country_code)` chứ không phải ngược lại: `country_code` chỉ có ~200 giá trị (cardinality thấp), còn `event_date` là range filter luôn có mặt và cắt bớt dữ liệu mạnh nhất. Đặt cột lọc mạnh nhất lên trước.

### Vấn đề 3 — `DATE(e.event_date)` giết index

```sql
SELECT DATE(e.event_date) AS date, COUNT(e.view_id)
FROM event_logs AS e ...
GROUP BY DATE(e.event_date) ORDER BY DATE(e.event_date)
```
Bọc cột trong hàm ⇒ **non-sargable** ⇒ index trên `event_date` vô dụng cho phần `GROUP BY`, MySQL phải làm filesort/temp table.

Cách sửa (theo thứ tự ưu tiên):
1. Thêm cột generated + index (MySQL 5.7+):
   ```sql
   ALTER TABLE event_logs
     ADD COLUMN event_day DATE AS (DATE(event_date)) STORED,
     ADD INDEX idx_ev_day (event_day);
   ```
2. Hoặc dùng functional index (MySQL 8.0.13+):
   ```sql
   CREATE INDEX idx_ev_date_fn ON event_logs ((CAST(event_date AS DATE)));
   ```

Lưu ý phần `WHERE` thì đã viết đúng rồi — `e.event_date >= FROM_UNIXTIME(?)` đặt hàm ở **vế phải**, cột để trần ⇒ vẫn sargable. Người viết code biết điều này ở `WHERE` nhưng bỏ quên ở `GROUP BY`.

### Tổng kết cải thiện dự kiến

| Query | Trước | Sau |
|---|---|---|
| `buildTotalScanQuery` | scan toàn bộ mapping của product + N lần lookup `event_logs` | covering index trên mapping + range scan `event_logs` |
| `buildDailyScanQuery` | thêm filesort do `DATE()` | index trên `event_day` → `Using index for group-by` |
| `buildTopBatchesQuery` | `GROUP BY batch_nft_id` sau full scan | index `(product_addr, batch_nft_id, ...)` đã sort sẵn theo `batch_nft_id` ⇒ bỏ được sort |

---

# PHẦN 3 — MATERIALIZED VIEW

## 3.1 Trả lời thẳng: KHÔNG có materialized view nào

⚠️ **Bẫy tên gọi**: có một bảng tên `top_info_view`, dễ tưởng là view.

```sql
-- bts-data-analytics/migrations/000004_create_top_info_views_table.up.sql
CREATE TABLE IF NOT EXISTS top_info_view (
    id            INT AUTO_INCREMENT PRIMARY KEY,
    section_name  VARCHAR(255) NOT NULL,
    duration      INT NOT NULL,
    pb_mapping_id INT NOT NULL,
    event_id      INT NOT NULL,
    FOREIGN KEY (pb_mapping_id) REFERENCES product_batch_mapping (id),
    FOREIGN KEY (event_id)      REFERENCES event_logs (id)
);
CREATE INDEX idx_pb_mapping_id ON top_info_view (pb_mapping_id);
CREATE INDEX idx_event_id      ON top_info_view (event_id);
```

Đây là **base table bình thường**, ghi trực tiếp từ application code (`AddInfoView` trong `bts-data-collector/internal/database/collector.go`), không phải view, không refresh từ query nào.

## 3.2 MySQL không có Materialized View — đây mới là câu trả lời "ăn tiền"

Nếu giám khảo hỏi "sao không dùng materialized view", trả lời:

> "MySQL không hỗ trợ materialized view native — chỉ có regular VIEW (chỉ là macro SQL, không lưu dữ liệu, không tăng tốc gì). PostgreSQL mới có `CREATE MATERIALIZED VIEW` + `REFRESH MATERIALIZED VIEW CONCURRENTLY`. Codebase này chạy MySQL/InnoDB (`ENGINE = InnoDB` trong migration `000001`, driver `gorm.io/driver/mysql` + `go-sql-driver/mysql` ở cả 4 service Go). Nên muốn có matview trên MySQL thì phải **tự emulate**: bảng summary + job refresh."

### Nếu là PostgreSQL thì sẽ viết thế nào

```sql
CREATE MATERIALIZED VIEW mv_daily_product_stats AS
SELECT
    d.product_addr,
    d.batch_nft_id,
    DATE(e.event_date)          AS day,
    e.country_code,
    COUNT(e.view_id)            AS total_scans,
    COUNT(DISTINCT e.user_uid)  AS unique_users
FROM event_logs e
JOIN product_batch_mapping d ON e.id = d.event_id
GROUP BY d.product_addr, d.batch_nft_id, DATE(e.event_date), e.country_code;

-- Bắt buộc có UNIQUE index thì mới REFRESH CONCURRENTLY được
CREATE UNIQUE INDEX ux_mv_daily
  ON mv_daily_product_stats (product_addr, batch_nft_id, day, country_code);

-- Refresh không khoá reader
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_daily_product_stats;
```

Sau đó `buildDailyScanQuery` từ scan hàng triệu row → đọc vài trăm row đã aggregate sẵn.

### Emulate trên MySQL (giải pháp thực tế cho repo này)

```sql
CREATE TABLE daily_product_stats (
    product_addr  VARCHAR(50)  NOT NULL,
    batch_nft_id  INT          NOT NULL,
    day           DATE         NOT NULL,
    country_code  CHAR(2)      NOT NULL,
    total_scans   INT          NOT NULL DEFAULT 0,
    unique_users  INT          NOT NULL DEFAULT 0,
    total_duration BIGINT      NOT NULL DEFAULT 0,
    refreshed_at  TIMESTAMP    DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (product_addr, batch_nft_id, day, country_code)
) ENGINE = InnoDB;
```

Refresh **incremental** (chỉ tính lại hôm qua + hôm nay, không tính lại toàn bộ lịch sử):
```sql
INSERT INTO daily_product_stats
    (product_addr, batch_nft_id, day, country_code, total_scans, unique_users)
SELECT d.product_addr, d.batch_nft_id, DATE(e.event_date), e.country_code,
       COUNT(e.view_id), COUNT(DISTINCT e.user_uid)
FROM event_logs e
JOIN product_batch_mapping d ON e.id = d.event_id
WHERE e.event_date >= CURDATE() - INTERVAL 1 DAY
GROUP BY 1,2,3,4
ON DUPLICATE KEY UPDATE
    total_scans  = VALUES(total_scans),
    unique_users = VALUES(unique_users);
```

Chạy bằng gì? Repo này **đã có sẵn Temporal** → dùng **Temporal Schedule / Cron Workflow**, không cần thêm hạ tầng mới. Đây là câu trả lời ghi điểm: tận dụng cái đang có.

⚠️ Cảnh báo phải nói kèm: `COUNT(DISTINCT user_uid)` **không cộng dồn được** — tổng của distinct theo ngày ≠ distinct theo tháng. Muốn rollup chính xác phải lưu HyperLogLog (Redis `PFADD`/`PFCOUNT`) hoặc lưu raw set. Nêu được cái này là hiểu sâu về pre-aggregation.

---

# PHẦN 4 — RABBITMQ / MESSAGE QUEUE

## 4.1 Sự thật: RabbitMQ gần như không được dùng

Kết quả grep toàn repo:
- `bts-core/requirements.txt:3` → `amqp==5.1.1` (transitive dep của `kombu`, của Celery)
- `bts-core/beetee/settings.py:479` → `CELERY_BROKER_URL = _celery_configurations.try_get('broker_url', ...)` — đọc từ file config bên ngoài repo, nhưng `amqp` trong deps ⇒ broker là RabbitMQ
- `notification-service/go.mod:25` → `github.com/StackExchange/wmi` — **false positive**, chỉ trùng chữ "Exchange"

**Quan trọng hơn**: 7 Celery task được **định nghĩa** trong `bts-core/core/tasks/` nhưng grep `.delay(` / `.apply_async(` toàn repo → **không có chỗ nào gọi**. Celery ở đây là **code chết**, di sản của kiến trúc cũ. Toàn bộ đã được migrate sang Temporal.

Bằng chứng migration 1-1 rất rõ:

| Celery task (cũ, Python) | Temporal workflow (mới, Go) |
|---|---|
| `core/tasks/search.py :: index_user / index_access_group / index_entity` | `bts-core-service/worker/on_index_search.go :: OnIndexSearchWorkflow` |
| `core/tasks/data_service.py :: sync_checkpoint / sync_product_batch` | `worker/on_sync_data_service.go :: OnSyncCheckpointWorkflow / OnSyncProductBatchWorkflow` |
| `core/tasks/notification.py :: send_share_batch_notification` | `worker/on_send_notification.go :: OnSendNotificationReceiveBatchWorkflow` |
| `core/tasks/assets.py :: generate_thumbnail` | `worker/on_handle_assets.go :: OnGenerateThumbnailWorkflow` |

## 4.2 Bảng ánh xạ khái niệm RabbitMQ ↔ Temporal (học thuộc bảng này)

| RabbitMQ | Temporal (trong repo) | Ghi chú |
|---|---|---|
| Producer | `temporalClient.ExecuteWorkflow(...)` | `bts-core-service/server/main.go:124` |
| Queue | **Task Queue** — `config.AppConfig.Temporal.TaskQueue`, vd `"payment-service"` | |
| Consumer | **Worker** — `worker.New(client, taskQueue, ...)` | `bts-core-service/worker/main.go:22` |
| Message | Workflow input args (JSON-serialized) | |
| Routing key | `req.Type` / `event.Type` trong `switch` chọn workflow name | |
| Exchange + binding | Chính cái `switch` đó (routing ở tầng application) | |
| `basic.ack` | Activity trả `nil` | |
| `basic.nack` + requeue | Activity trả `error` → Temporal tự retry | |
| Dead Letter Queue | `temporal.NewNonRetryableApplicationError(...)` | `payment-service/worker/handler/subscription.go:42` |
| Message TTL | `ScheduleToStartTimeout: time.Hour` | |
| Consumer prefetch | `worker.Options{MaxConcurrentActivityExecutionSize}` (đang để default) | |
| Message deduplication | **Workflow ID** — Temporal đảm bảo 1 workflow ID chỉ chạy 1 instance | |

**Khác biệt cốt lõi phải nói được**: RabbitMQ là *dumb broker* — mất message là mất luôn, retry phải tự code. Temporal là *durable execution* — nó ghi **event history** của từng workflow vào DB, worker chết giữa chừng thì worker khác pick up và **chạy tiếp từ đúng chỗ dừng**, không phải chạy lại từ đầu. Đổi lại: chậm hơn, nặng hơn, không hợp cho throughput cực cao.

## 4.3 Data được vận chuyển — JSON đầy đủ

### (1) Index search — Django → Core-Service → Temporal → Search Service

Producer: `bts-core/console/helpers/bts_core_service_client.py:27`
```http
POST http://bts-core-service:5005/v1/index-search
Content-Type: application/json

{
  "type": "product",
  "id": 4821,
  "is_deleted": false
}
```
`type` ∈ `{user, access_group, entity, product}` — đây chính là **routing key**.

Core-service tạo workflow (`server/main.go:118`):
```go
options := client.StartWorkflowOptions{
    ID:        fmt.Sprintf("%s-%d-%v", req.Type, req.ID, uuid.ClockSequence()),
    TaskQueue: s.taskQueue,
}
// → workflow ID = "product-4821-3821"
ExecuteWorkflow(ctx, options, "OnIndexSearchWorkflow", req.ID, req.Type, req.IsDeleted)
```

Workflow dispatch tới activity tương ứng (`on_index_search.go:26-37`), activity **tự đọc DB lấy full document** rồi POST sang search-service:
```http
POST {SEARCH_SERVICE}/v1/index/product
{
  "id": "4821",
  "name": "Rau muống hữu cơ Đà Lạt",
  "organisation_id": "77",
  "is_deleted": false
}
```

📌 **Pattern quan trọng — "Claim Check"**: message chỉ mang `{type, id}`, KHÔNG mang payload. Consumer tự query DB lấy dữ liệu đầy đủ. Ưu điểm: message nhỏ, và **luôn lấy được state mới nhất** dù message bị delay/retry (tránh ghi đè bằng dữ liệu cũ). Nhược: consumer phải truy cập được DB, và có race nếu record bị xoá trước khi consume — code xử lý bằng nhánh `isDeleted` (`on_index_search.go:47-62`).

### (2) Sync data service

```http
POST /v1/sync-data-service
{
  "type": "product_batch",
  "id": 9013,
  "operation": "update"
}
```
- `type` ∈ `{checkpoint, product_batch, product_batch_transfer, product}` → chọn workflow
- `operation` ∈ `{create, update, delete}` → chọn activity **bên trong** workflow

Đây là **routing 2 tầng**: tầng 1 (`type`) chọn workflow — giống binding key của exchange; tầng 2 (`operation`) chọn activity — giống consumer tự phân nhánh.

### (3) Notification

```http
POST /v1/notification
{
  "type": "batch_transfer_feedback",
  "id": 331,
  "params": {
    "to_email": "farm@example.com",
    "to_org_name": "HTX Rau Sạch Đà Lạt",
    "from_org_name": "Siêu thị ABC",
    "status": "accepted"
  }
}
```
Ở đây `params` là **map tự do** — message mang cả payload thật, khác với claim-check ở (1). Lý do: nội dung email là **snapshot tại thời điểm gửi**, không được đổi theo state mới của DB.

`notification-service/server/main.go:88-116` route `type` sang **~25 workflow khác nhau** (`WELCOME_NEW_USER`, `PASSWORD_RESET_REQUEST`, `INVOICE_PAID`, …) — đây là **topic exchange lớn nhất repo**.

### (4) Stripe webhook — ví dụ topic exchange đẹp nhất

`payment-service/routes/webhook.go:78-93`:
```go
switch constant.StripeEventType(event.Type) {
case StripeEventCustomerCreated, StripeEventCustomerUpdated:
    workflowName = "UpdateCustomerInfoWorkflow"
case StripeEventInvoiceCreated, StripeEventInvoiceUpdated,
     StripeEventInvoicePaymentSucceeded, StripeEventInvoicePaymentFailed,
     StripeEventInvoicePaymentActionRequired:
    workflowName = "UpdateInvoiceWorkflow"          // ← 5 routing key → 1 queue
case StripeEventSubscriptionCreated, StripeEventSubscriptionUpdated,
     StripeEventSubscriptionDeleted:
    workflowName = "UpdateSubscriptionWorkflow"
case StripeEventChargeRefunded:
    workflowName = "ChargeRefundedWorkflow"
default:
    return &utils.HTTPResponse{StatusCode: http.StatusOK}  // ← 2xx để Stripe khỏi retry
}
```

Đây **chính xác là semantic của topic exchange**: nhiều routing key (`invoice.*`) bind vào cùng 1 queue.

Message thật gửi vào Temporal là `event.Data.Raw` (raw JSON từ Stripe) + `event.Created`:
```json
{
  "id": "sub_1QxYzABC123",
  "object": "subscription",
  "status": "active",
  "customer": { "id": "cus_PabcDEF456" },
  "latest_invoice": { "id": "in_1QxYzGHI789" },
  "items": { "data": [ { "price": { "id": "price_1QxYz", "product": { "id": "prod_Rabc" } } } ] },
  "metadata": { "organisation_id": "77" },
  "current_period_start": 1751328000,
  "current_period_end": 1753920000
}
```

📌 Chi tiết đắt: `default:` trả **HTTP 200** kèm comment `// return 2xx code to prevent Stripe from retrying`. Đây là **poison message handling** — event không hỗ trợ thì ack luôn thay vì để Stripe retry vô hạn. Trong RabbitMQ tương đương `basic.ack` một message không xử lý được thay vì `nack(requeue=true)` gây infinite loop.

⚠️ Có bug bảo mật đáng nói: `webhook.ConstructEvent` (verify chữ ký Stripe) bị **comment out** ở dòng 57-64, thay bằng `json.Unmarshal` trần. Nghĩa là **bất kỳ ai POST vào endpoint này đều kích được workflow billing**. Nói ra được lỗi này trong phỏng vấn là điểm cộng lớn.

## 4.4 "Bắn vào exchange mà 2 queue cùng nhận" — có không?

**Không có fanout exchange đúng nghĩa** (Temporal không có khái niệm exchange). Nhưng **có 3 pattern fan-out thật sự**, ở 3 tầng khác nhau:

### Fan-out kiểu A — Application-level, 1 event → 2 workflow độc lập

`bts-core/console/views/products.py:416-417`:
```python
product.sync_to_data_service(product_id=product.id, operation='create')   # → OnSyncProductWorkflow
product.update_search()                                                   # → OnIndexSearchWorkflow
```

Một sự kiện nghiệp vụ (**"product được tạo"**) → **2 HTTP call → 2 workflow riêng biệt → 2 hệ thống downstream khác nhau** (data-service và search-service).

Trong RabbitMQ đây chính là:
```
                    ┌──> queue.data_service  ──> data-service worker
publish("product.created") ──> [fanout exchange]
                    └──> queue.search_index  ──> search worker
```
Nhưng ở đây fan-out **được hardcode ở producer** thay vì broker lo. Trade-off:
- ❌ Thêm consumer mới = phải sửa code Django (coupling ngược — vi phạm ý nghĩa của pub/sub)
- ❌ **Không atomic**: nếu `sync_to_data_service` OK mà `update_search` fail thì hệ thống lệch. Cả 2 method đều bọc `try/except` chỉ `logging.error` (`products.py:290, 310`) → **lỗi bị nuốt im lặng**
- ✅ Đơn giản, dễ debug, thấy rõ ai gọi ai

Các chỗ fan-out tương tự:
- `console/views/products.py:552-553` (update)
- `console/views/products.py:583-584` (delete)
- `console/views/products.py:633-634`, `833-834`
- `console/views/blockchain_webhook.py:138-139` — entity: `update_search()` + `sync_to_data_service()`
- `console/views/blockchain_webhook.py:238-242` — batch sync + `product.update_search()`
- `console/views/blockchain_webhook.py:158-162` — `sync_to_data_service()` + `update_resource_used(-1)` (gọi sang **payment-service**!) → 1 event → 3 service

### Fan-out kiểu B — Trong workflow, 1 message → nhiều activity tuần tự

`bts-core-service/worker/on_sync_data_service.go:38-67`:
```go
func (w *Worker) OnSyncProductBatchWorkflow(ctx workflow.Context, id int, operation string) error {
    switch operation {
    case "create": err = workflow.ExecuteActivity(ctx, w.SyncCreateProductBatchActivity, id).Get(ctx, nil)
    case "update": err = workflow.ExecuteActivity(ctx, w.SyncUpdateProductBatchActivity, id).Get(ctx, nil)
    case "delete": err = workflow.ExecuteActivity(ctx, w.SyncDeleteProductBatchActivity, id).Get(ctx, nil)
    }
    if err != nil { return err }

    // ↓ LUÔN chạy thêm, bất kể operation là gì — đây là "consumer thứ 2"
    if err := workflow.ExecuteActivity(ctx, w.SyncProductLineageTreeActivity, id).Get(ctx, nil); err != nil {
        return fmt.Errorf("failed to sync product lineage tree for ID %d: %w", id, err)
    }
    return nil
}
```

**1 message `{type:"product_batch", id:9013, operation:"update"}` → 2 side-effect**: cập nhật batch + rebuild cây lineage (truy xuất nguồn gốc). Khác kiểu A ở chỗ **đây là atomic về mặt workflow**: nếu `SyncProductLineageTreeActivity` fail, Temporal retry cả bước đó cho tới khi thành công. Đây là fan-out **có đảm bảo**.

### Fan-out kiểu C — Chain qua nhiều service

`payment-service/worker/handler/subscription.go:38-59`:
```go
func (h *subscriptionHandler) UpdateSubscriptionWorkflow(ctx workflow.Context, data json.RawMessage, createdAt int64) error {
    subscription := stripe.Subscription{}
    if err := json.Unmarshal(data, &subscription); err != nil {
        return temporal.NewNonRetryableApplicationError(...)   // ← DLQ, không retry
    }
    ctx = workflow.WithActivityOptions(ctx, config.ActivityOptions())

    var subscriptionData *model.Subscription
    // Activity 1: ghi DB payment-service
    if err := workflow.ExecuteActivity(ctx, h.UpdateSubscriptionActivity, subscription, createdAt).Get(ctx, &subscriptionData); err != nil {
        return err
    }

    // Activity 2: CHỈ chạy nếu có thay đổi thật sự — conditional fan-out
    if subscriptionData != nil {
        if err := workflow.ExecuteActivity(ctx, h.TriggerCoreWebhook, subscriptionData).Get(ctx, nil); err != nil {
            return err
        }
    }
    return nil
}
```

`TriggerCoreWebhook` POST ngược về Django (`payment-service/xservices/bts_core.go:30` → `/console/webhook/payment/`):
```json
{
  "action": "create",
  "organisation_id": 77,
  "codenames": ["product.create", "batch.create", "analytics.view"],
  "subscription_name": "Growth Plan"
}
```

→ **Chuỗi hoàn chỉnh**: Stripe → payment-service webhook → Temporal → DB → HTTP → Django → cấp quyền cho org. Một event Stripe lan qua 3 hệ thống.

📌 Chi tiết cực hay: `subscriptionData != nil` là kết quả của `isUpdateNeeded` từ service layer. Xem 4.5.

## 4.5 Xử lý message đến sai thứ tự (out-of-order) — chủ đề phỏng vấn kinh điển

`payment-service/internal/services/subscription.go:129-133`:
```go
if subscription.UpdatedAt.After(utils.ParseUnixToUTC(updatedAt)) {
    utils.Info("received out of order update for subscription plan customer",
        "stripe_subscription_id", stripeSub.ID)
    return subscription, isUpdateNeeded, nil // not the latest update
}
```

Vấn đề: Stripe gửi webhook **không đảm bảo thứ tự**. Có thể `subscription.updated (t=100)` đến **sau** `subscription.deleted (t=200)`. Nếu ghi mù quáng, subscription đã huỷ sẽ bị "sống lại".

Giải pháp trong code: dùng `event.Created` (timestamp của Stripe) làm **version / logical clock**. So với `UpdatedAt` đang lưu trong DB — nếu DB mới hơn thì **bỏ qua message**. Đây là **Last-Write-Wins với version check**, tương đương optimistic concurrency control.

Và `isUpdateNeeded` (dòng 155-157):
```go
return subscription,
    oldActiveStatus != newActiveStatus || oldPlanID != planIDInt64,
    nil
```
Chỉ trả `true` khi **trạng thái active đổi HOẶC plan đổi**. Nghĩa là: Stripe bắn 20 event `subscription.updated` cho những thay đổi vặt (đổi payment method, đổi metadata…) → DB vẫn được cập nhật, nhưng `TriggerCoreWebhook` **không bị gọi 20 lần**. Đây là **change-data-capture filtering / event debouncing** — chỉ fan-out khi có thay đổi *ý nghĩa*.

## 4.6 Retry & Backoff

`notification-service/server/main.go:71-79`:
```go
workflowOptions := client.StartWorkflowOptions{
    ID:        workflowId,
    TaskQueue: utils.NotificationServiceConfig.TaskQueue,
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,          // exponential: 1s, 2s, 4s, 8s, 16s...
        MaximumInterval:    time.Second * 100,   // trần 100s
    },
}
```

Đối chiếu Celery cũ:
```python
@shared_task(autoretry_for=(Exception,), bind=True, max_retries=None, default_retry_delay=300)
```
Celery: retry **cố định 300s, vô hạn lần**. Temporal: **exponential backoff có trần**. Temporal tốt hơn — retry nhanh cho lỗi thoáng qua (network blip), giãn dần cho lỗi kéo dài (service down), tránh thundering herd khi downstream hồi phục.

`temporal.NewNonRetryableApplicationError("UpdateSubscriptionWorkflow", "UnmarshalError", err)` = **phân biệt lỗi transient và lỗi permanent**. JSON hỏng thì retry 1000 lần cũng hỏng → đẩy thẳng vào DLQ. Đây là điều Celery `autoretry_for=(Exception,)` làm **sai** — nó retry vô hạn cả lỗi permanent.

---

# PHẦN 5 — READ REPLICATION

## 5.1 Trả lời thẳng: KHÔNG có read replica

Grep toàn repo `replica|slave|readonly|DATABASE_ROUTERS|dbresolver|master` → không có kết quả nào liên quan.

Bằng chứng cụ thể:
- `bts-data-analytics/internal/database/db.go` — 1 DSN duy nhất, `sql.Open("mysql", dsn)`
- `bts-data-collector/internal/database/db.go` — 1 DSN, `gorm.Open(mysql.Open(dsn))`
- `bts-core-service/database/db.go:34` — 1 DSN
- `payment-service/config.example.yaml` — 1 block `database:`, không có `replica:`
- Django: `DATABASES = _app_configurations.try_get('databases', ...)` (`settings.py:320`) — nạp từ file config ngoài repo, nhưng **không có `DATABASE_ROUTERS`** trong settings ⇒ dù có khai nhiều DB, Django vẫn chỉ dùng `default`
- Chỉ 1 chỗ dùng `.using(...)`: `bts-core/core/apps.py:56,66` — nhưng đây là param `using` của Django migration signal, **không phải chọn replica**

## 5.2 Nơi read replica sẽ có giá trị nhất

Kiến trúc hiện tại có một điểm rất đáng nói:

```
bts-data-collector  ──WRITE──┐
  (mọi lượt quét QR,          ├──> MySQL: gh_analytics
   write-heavy, latency-      │      (event_logs, page_view,
   sensitive)                 │       product_batch_mapping, top_info_view)
                              │
bts-data-analytics  ──READ───┘
  (COUNT/GROUP BY/AVG toàn bảng,
   long-running, không cần realtime)
```

**Hai service đã tách rời nhau về code, nhưng vẫn dùng chung 1 MySQL instance.** Đây là ứng viên hoàn hảo cho read replica:

- Workload đã tách sẵn: collector chỉ `INSERT`, analytics chỉ `SELECT` (đã verify: `analytics_query.go` chỉ có `SELECT`)
- Analytics chạy aggregate nặng trên `event_logs` — **không có index** (xem 2.3) ⇒ full table scan ⇒ chiếm buffer pool, đẩy hot data của collector ra khỏi cache
- Analytics **chấp nhận được replication lag**: dashboard "tổng lượt quét 30 ngày" trễ 5 giây không ai chết
- Sửa cực rẻ: chỉ cần đổi DSN trong `bts-data-analytics/internal/config` sang endpoint replica. **Không phải sửa 1 dòng logic nào.**

## 5.3 Nếu bị hỏi sâu — những cái phải cân nhắc

| Vấn đề | Xử lý |
|---|---|
| **Replication lag** | MySQL async replication. Cần monitor `Seconds_Behind_Master`. Nếu > ngưỡng thì fallback về primary hoặc hiện banner "dữ liệu có thể trễ" |
| **Read-your-own-writes** | Bug kinh điển: user vừa quét QR, dashboard đọc replica chưa kịp sync → thấy số cũ. Fix: sticky-to-primary trong X giây sau write, hoặc dùng `MASTER_POS_WAIT()` / GTID |
| **Chỗ KHÔNG được đọc replica** | `bts-core/dexer/models/inventory.py:89-95` — `select_for_update()` + `F('value') - quantity`. Đọc replica ở đây sẽ tính sai balance NFT. Row lock chỉ tồn tại trên primary. **Luôn đọc primary với read-modify-write** |
| **Failover** | Cần proxy (ProxySQL / RDS Proxy / Vitess), không hardcode host replica vào config |

## 5.4 So sánh: cái repo này ĐANG làm thay cho replication

Repo không có replica, nhưng có 2 kỹ thuật giảm tải đọc tương đương:

1. **CQRS ở tầng service** — Django (write model, đầy đủ nghiệp vụ) đẩy dữ liệu sang **search-service** (read model, đã denormalize) qua `OnIndexSearchWorkflow`. Query tìm kiếm không đụng vào MySQL của Django. Đây là **read scaling bằng cách tách read model**, mạnh hơn read replica ở chỗ read model có thể có schema/engine hoàn toàn khác (Elasticsearch chẳng hạn).
2. **CDN** — `AWS_CLOUDFRONT_DISTRIBUTION_ID` (`settings.py:443, 461`) cho media library và product metadata. Metadata sản phẩm (đọc rất nhiều, ghi rất ít) được cache ở edge; khi đổi thì gọi CloudFront invalidation (`common/helpers/storage.py:69-83`). Đây là **caching layer ngoài cùng**, chặn traffic trước cả khi tới app server.

---

# PHẦN 6 — CÁC KỸ THUẬT KHÁC TRONG REPO

## 6.1 `transaction.on_commit` — Transactional messaging (rất hay)

`bts-core/console/views/blockchain_webhook.py:136-139`:
```python
with transaction.atomic():
    entity.save()
    ...
    transaction.on_commit(lambda: entity.update_search())
    transaction.on_commit(lambda: entity.sync_to_data_service(entity_id=entity.id, operation='update'))
```

**Vấn đề nó giải quyết** (dual-write problem): nếu bắn message *bên trong* transaction, mà transaction sau đó rollback → consumer nhận message về một record **không tồn tại**. Ngược lại nếu commit rồi mới bắn mà app crash ở giữa → mất message.

`on_commit` giải quyết vế thứ nhất: callback chỉ chạy **sau khi COMMIT thành công**. Vẫn còn vế thứ hai (crash sau commit trước khi POST) — đó là lý do người ta cần **Transactional Outbox**. Nói được cả 2 vế + biết `on_commit` chỉ giải quyết một nửa ⇒ ăn điểm.

⚠️ Không nhất quán: `products.py:416-417` gọi thẳng **không** bọc `on_commit`, trong khi `blockchain_webhook.py` thì có. Bug tiềm ẩn.

## 6.2 `select_for_update` + `F()` expression — Pessimistic locking

`bts-core/dexer/models/inventory.py:88-95`:
```python
sender_.balances.get_or_create(contract_address=..., token_id=token_id)
sender_.balances.select_for_update().filter(
    contract_address=event.contract_address, token_id=token_id
).update(value=F('value') - quantity)

recipient_.balances.get_or_create(contract_address=..., token_id=token_id)
recipient_.balances.select_for_update().filter(...).update(value=F('value') + quantity)
```

Hai kỹ thuật chồng nhau:
- `select_for_update()` → `SELECT ... FOR UPDATE`, khoá row cho tới hết transaction. Chống 2 transfer song song đọc cùng balance.
- `F('value') - quantity` → SQL `SET value = value - ?`. Phép trừ do **DB** làm, không round-trip qua Python ⇒ chống lost update ngay cả khi không lock.

⚠️ Rủi ro deadlock để nói thêm: sender được khoá trước, recipient sau. Nếu A→B và B→A chạy đồng thời sẽ deadlock. Fix chuẩn: **luôn khoá theo thứ tự cố định** (ví dụ sort theo `account_id` tăng dần) — kinh nghiệm kinh điển của bài toán chuyển tiền.

Thêm: `select_for_update()` đặt **sau** `get_or_create()` là subtle bug — `get_or_create` không nằm trong lock, 2 luồng có thể cùng tạo row (may là có `unique_together (account, contract_address, token_id)` đỡ cho, xem 2.1 — đây là ví dụ đẹp: **compound unique index cứu concurrency bug**).

## 6.3 Chống N+1 query

`bts-core/console/views/products.py:226`:
```python
.select_related("organisation").prefetch_related("preset_attributes", "organisation__assets")
```
- `select_related` → SQL `JOIN`, cho quan hệ **one-to-one / FK** (1 query)
- `prefetch_related` → query riêng + join trong Python, cho quan hệ **many-to-many / reverse FK** (2 query, nhưng vẫn hơn N+1)

Các chỗ khác: `organisations.py:97, 432, 531, 1193`, `product_batches.py:218, 1420, 1440`, `interorg.py:107, 474`.

⚠️ Phần lớn view **không** dùng — điểm cần cải thiện thật.

## 6.4 Bulk insert

`bts-core/core/models/assets.py:158` — `self.thumbnails.bulk_create(...)`: tạo tất cả thumbnail size trong **1 INSERT** thay vì N lần.
`bts-core/console/views/organisations.py:971` — `OrganisationCheckpointHistory.objects.bulk_create([...])`.

## 6.5 Unique constraint = idempotency ở tầng DB

Tổng hợp lại thành một luận điểm mạnh cho phỏng vấn — **hệ thống này chống trùng ở 3 tầng khác nhau**:

| Tầng | Cơ chế | Ví dụ |
|---|---|---|
| Edge / API | Redis idempotency key | `antispam_entry:<uid>:<hash>` (Phần 1) |
| Message broker | Temporal Workflow ID | `product-4821-3821` (`server/main.go:119`) |
| Database | Compound UNIQUE index | `(block_number, log_index, transaction_index)` (Phần 2) |

Defense in depth: Redis có thể mất key (TTL/evict), Temporal ID có `uuid.ClockSequence()` nên không thật sự unique — nhưng **DB constraint thì không bao giờ sai**. Tầng cuối là tầng đáng tin nhất.

## 6.6 Connection pooling

`payment-service/pkg/redis.go:26-35` là ví dụ chuẩn:
```go
opts := &redis.Options{
    Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
    DB:           cfg.DB,
    DialTimeout:  cfg.Timeout,
    ReadTimeout:  cfg.ReadTimeout,
    WriteTimeout: cfg.WriteTimeout,
    PoolSize:     cfg.PoolSize,
    MinIdleConns: cfg.MinIdleConns,
}
rdb := redis.NewClient(opts)
if err := rdb.Ping(ctx).Err(); err != nil {    // fail-fast lúc boot
    return nil, fmt.Errorf("failed to connect to redis: %w", err)
}
```
So với `bts-data-collector/utils/redis_cache.go:17-22` — hardcode, không pool config, không ping. Đối lập rõ ràng: service dùng thật thì config kém, service không dùng thì config chuẩn.

## 6.7 Dependency Injection (Uber fx) — payment-service

`payment-service/worker/worker.go:19-40` dùng `go.uber.org/fx`: khai báo module, fx tự resolve dependency graph, `fx.Lifecycle` quản lý start/stop. Đáng nói khi được hỏi về kiến trúc service — đây là service duy nhất trong repo có DI container thật sự; các service khác wire thủ công.

## 6.8 Fail-open vs Fail-closed

Đối lập rất đáng phân tích:
- **Redis fail-open** (`redis_cache.go:30`): Redis chết → coi như miss → API vẫn chạy. Đúng, vì Redis chỉ là lớp chống spam.
- **Sync fail-open silent** (`products.py:290`): `except Exception as e: logging.error(...)` — sync sang data-service fail thì **nuốt lỗi**, user vẫn thấy "tạo thành công". Sai, vì đây là dữ liệu nghiệp vụ ⇒ nên có outbox/retry chứ không nuốt.
- **Temporal fail-closed**: activity trả error → retry cho tới khi thành công. Đúng.

---

# PHẦN 7 — CHEAT SHEET TRẢ LỜI PHỎNG VẤN

### "Bạn dùng Redis để làm gì trong dự án?"
> "Ở service thu thập dữ liệu tracking (`bts-data-collector`), em dùng Redis làm **idempotency layer chứ không phải cache tăng tốc**. Key là `antispam_entry:<user_uid>:<hash(raw_body)>`, TTL 10 phút, value là JSON `{value, status_code}` — chính là response đã trả trước đó. Endpoint này public, không auth, nhận event mỗi lượt quét QR, nên rất dễ bị bắn trùng do SPA re-render hoặc bot. Nếu không dedupe thì số liệu analytics bị thổi phồng. Pattern là cache-aside, và em cố ý **chỉ cache response 2xx** — lỗi 4xx/5xx không cache để client retry được ngay. Redis chết thì fail-open, API vẫn chạy. Điểm yếu em biết là GET-rồi-SET không atomic nên vẫn có cache stampede — sửa thì dùng `SET NX EX`."

### "Compound index là gì, dự án bạn dùng ở đâu?"
> "Compound index là index trên nhiều cột, tuân theo leftmost prefix rule. Ví dụ đắt nhất trong dự án em là bảng lưu event blockchain: `UNIQUE (block_number, log_index, transaction_index)`. Bộ ba này định danh duy nhất một log trên chain, nên nó vừa là index tăng tốc, vừa là **cơ chế chống double-ingest khi indexer restart hoặc chain reorg** — biến idempotency thành trách nhiệm của DB. Nó phục vụ query `WHERE block_number = ?` và `SELECT MAX(block_number)` để resume, nhưng **không** phục vụ được query chỉ theo `log_index` — nên bảng có thêm index đơn trên `contract_address` cho use case khác.
>
> Một ví dụ nữa hay hơn về mặt concurrency: bảng balance có `UNIQUE (account, contract_address, token_id)`, và code dùng `SELECT ... FOR UPDATE` khớp đúng cả 3 cột. InnoDB khoá theo index range, nên nhờ index này nó chỉ khoá 1 row. Nếu không có, nó phải scan cả bảng và sẽ khoá gần như mọi row → deadlock khi nhiều transfer chạy song song. Đó là ví dụ index ảnh hưởng đến concurrency chứ không chỉ tốc độ."

### "Có chỗ nào query chậm mà index sửa được không?"
> "Có. Service analytics có 8 query dashboard đều theo khuôn `event_logs JOIN product_batch_mapping WHERE product_addr = ? AND batch_nft_id IN (...) AND event_date BETWEEN ? AND ?`. Bảng mapping hiện chỉ có 2 index **đơn** rời nhau — MySQL chỉ chọn được một, nên nó lấy toàn bộ row của product rồi lọc và lookup ngược. Em sẽ thay bằng compound `(product_addr, batch_nft_id, event_id)`: equality trước, range sau, và cột JOIN ở đuôi để thành **covering index** — `EXPLAIN` sẽ ra `Using index`, không đọc bảng gốc.
>
> Còn `event_logs` là bảng write-heavy nhất mà **không có index nào ngoài PK** — em sẽ thêm `(event_date, country_code)`, đặt `event_date` trước vì nó là filter cắt dữ liệu mạnh nhất còn `country_code` cardinality chỉ ~200.
>
> Ngoài ra có một chỗ non-sargable: `GROUP BY DATE(event_date)` — bọc cột trong hàm nên index vô dụng, phải filesort. Sửa bằng generated column `event_day DATE AS (DATE(event_date)) STORED` rồi index lên đó. Thú vị là ở mệnh đề `WHERE` thì code viết đúng — `event_date >= FROM_UNIXTIME(?)`, hàm ở vế phải — nhưng quên mất ở `GROUP BY`."

### "Materialized view thì sao?"
> "Dự án không có, và có một cái bẫy: bảng tên `top_info_view` nghe như view nhưng thật ra là base table bình thường, application ghi trực tiếp vào. Quan trọng hơn là **MySQL không hỗ trợ materialized view native** — chỉ Postgres mới có `CREATE MATERIALIZED VIEW` + `REFRESH ... CONCURRENTLY`. Dự án chạy MySQL/InnoDB nên muốn có thì phải tự emulate bằng bảng summary + job refresh.
>
> Chỗ đáng làm nhất là các query aggregate của dashboard. Em sẽ tạo `daily_product_stats` với PK `(product_addr, batch_nft_id, day, country_code)`, refresh **incremental** — chỉ tính lại 2 ngày gần nhất với `INSERT ... ON DUPLICATE KEY UPDATE` — và chạy bằng **Temporal Schedule**, tận dụng hạ tầng đã có sẵn thay vì thêm cron mới. Một cái bẫy em sẽ nói kèm: `COUNT(DISTINCT user_uid)` **không cộng dồn được** — tổng distinct theo ngày khác distinct theo tháng — nên muốn rollup chính xác phải dùng HyperLogLog."

### "RabbitMQ dùng thế nào?"
> "Thẳng thắn là dự án **không dùng RabbitMQ đang chạy**. Có dấu vết Celery ở service Django (`amqp` trong requirements, `CELERY_BROKER_URL` trong settings), nhưng 7 task đó **không có chỗ nào gọi `.delay()`** — đã migrate hết sang **Temporal**, map 1-1: `index_user` → `OnIndexSearchWorkflow`, `sync_checkpoint` → `OnSyncCheckpointWorkflow`, `generate_thumbnail` → `OnGenerateThumbnailWorkflow`.
>
> Em map được khái niệm: Task Queue ↔ queue, Worker ↔ consumer, workflow input ↔ message, cái `switch` chọn workflow name ↔ exchange + binding, activity trả error ↔ nack-requeue, `NewNonRetryableApplicationError` ↔ DLQ, Workflow ID ↔ message deduplication. Khác biệt cốt lõi: RabbitMQ là dumb broker, mất message là mất; Temporal ghi event history nên worker chết giữa chừng thì worker khác **chạy tiếp từ đúng chỗ dừng** chứ không phải làm lại từ đầu. Đổi lại nó nặng hơn và không hợp throughput cực cao."

### "Data gì được đẩy qua queue?"
> "Có hai kiểu message rõ rệt trong dự án. Kiểu thứ nhất là **claim-check**: index-search chỉ gửi `{type:'product', id:4821, is_deleted:false}` — không có payload. Consumer tự query DB lấy full document rồi POST sang search-service. Ưu điểm là message nhỏ và **luôn lấy state mới nhất**, nên message bị delay hay retry cũng không ghi đè bằng dữ liệu cũ.
>
> Kiểu thứ hai là **full payload**: notification gửi `{type:'batch_transfer_feedback', id:331, params:{to_email, to_org_name, from_org_name, status}}`. Ở đây phải mang cả nội dung vì email là **snapshot tại thời điểm gửi**, không được đổi theo state mới của DB.
>
> Còn Stripe webhook thì đẩy nguyên `event.Data.Raw` kèm `event.Created` làm version."

### "Có chỗ nào 1 message mà nhiều consumer nhận không?"
> "Không có fanout exchange đúng nghĩa vì Temporal không có khái niệm exchange, nhưng có 3 kiểu fan-out.
>
> Kiểu thứ nhất là **application-level**: khi tạo product, Django gọi liền 2 lệnh — `sync_to_data_service()` và `update_search()` — thành 2 workflow độc lập tới 2 hệ thống downstream. Đây đúng là ngữ nghĩa fanout exchange nhưng fan-out bị hardcode ở producer. Trade-off là thêm consumer mới phải sửa code Django, và **không atomic** — cả 2 đều bọc `try/except` chỉ log lỗi, nên nếu một cái fail thì hệ thống lệch âm thầm.
>
> Kiểu thứ hai **có đảm bảo hơn**: `OnSyncProductBatchWorkflow` nhận 1 message rồi chạy activity theo `operation`, xong **luôn** chạy thêm `SyncProductLineageTreeActivity` để rebuild cây truy xuất nguồn gốc. Vì nằm trong cùng workflow nên Temporal retry đến khi cả 2 xong.
>
> Kiểu thứ ba là chain xuyên service: 1 event Stripe → payment-service ghi DB → `TriggerCoreWebhook` POST ngược về Django cấp quyền. Và ở đây có chi tiết em thích: activity thứ hai **chỉ chạy khi có thay đổi thật sự** — service trả `isUpdateNeeded = (oldActiveStatus != newActiveStatus || oldPlanID != newPlanID)`. Stripe bắn cả chục event `subscription.updated` cho thay đổi vặt, nhưng webhook về Django chỉ bắn khi status hoặc plan đổi. Đó là event debouncing / CDC filtering."

### "Xử lý message trùng và sai thứ tự thế nào?"
> "Cái em thấy hay nhất trong repo là chỗ xử lý Stripe webhook. Stripe **không đảm bảo thứ tự**, nên `subscription.updated` ở t=100 có thể đến sau `subscription.deleted` ở t=200 — ghi mù quáng thì subscription đã huỷ sẽ sống lại. Code dùng `event.Created` làm **logical clock**: `if subscription.UpdatedAt.After(eventCreatedAt) { return; }` — DB đang mới hơn thì bỏ qua message. Đây là last-write-wins với version check, tức optimistic concurrency.
>
> Còn chống trùng thì hệ thống có **3 tầng**: Redis idempotency key ở edge, Temporal Workflow ID ở broker, và compound UNIQUE index ở DB. Defense in depth — Redis có thể mất key do TTL, Workflow ID lại có `uuid.ClockSequence()` nên không thật sự unique, nhưng DB constraint thì không bao giờ sai."

### "Read replica?"
> "Không có. Cả 4 service Go đều chỉ 1 DSN, Django không cấu hình `DATABASE_ROUTERS`. Nhưng kiến trúc lại **rất sẵn sàng** cho nó: `bts-data-collector` chỉ INSERT, `bts-data-analytics` chỉ SELECT, hai service đã tách code hoàn toàn nhưng vẫn chung 1 MySQL. Analytics chạy full table scan trên `event_logs` không index nên chiếm buffer pool, đẩy hot data của collector ra khỏi cache. Chuyển analytics sang replica chỉ cần đổi DSN, **không sửa một dòng logic nào**, và dashboard hoàn toàn chấp nhận được lag vài giây.
>
> Chỗ **tuyệt đối không được** đọc replica là logic cập nhật balance NFT — nó dùng `select_for_update()` với `F('value') - quantity`, row lock chỉ tồn tại trên primary, đọc replica sẽ tính sai số dư.
>
> Thay vì replica, dự án đang giảm tải đọc bằng 2 cách khác: **CQRS ở tầng service** — Django đẩy read model sang search-service riêng qua workflow, nên query tìm kiếm không đụng MySQL chính; và **CDN** — CloudFront cho media với product metadata, đọc nhiều ghi ít, đổi thì gọi invalidation."

---

# PHỤ LỤC — Tra cứu file nhanh

| Chủ đề | File:line |
|---|---|
| Redis client + cache-aside | `bts-data-collector/utils/redis_cache.go:29` |
| Redis call-site (anti-spam) | `bts-data-collector/internal/api/collection/api.go:58, 137` |
| Redis TTL config | `bts-data-collector/internal/config/config.go:32` (`redis_ttl: 10m`) |
| Redis pool config (không dùng) | `payment-service/pkg/redis.go:26` |
| Compound index — blockchain event | `bts-core/dexer/models/events.py:39` |
| Compound index — balance | `bts-core/dexer/models/inventory.py:108` |
| Single index — analytics | `bts-data-analytics/migrations/000003_*.up.sql:8,11` |
| 8 query dashboard cần index | `bts-data-analytics/internal/database/analytics_query.go` |
| Query non-sargable `DATE()` | `analytics_query.go:63` (`buildDailyScanQuery`) |
| Bảng `top_info_view` (KHÔNG phải view) | `bts-data-analytics/migrations/000004_*.up.sql` |
| Celery legacy (code chết) | `bts-core/core/tasks/*.py` |
| Celery broker config | `bts-core/beetee/settings.py:479` |
| Temporal producer (HTTP → workflow) | `bts-core-service/server/main.go:118-124` |
| Temporal worker registration | `bts-core-service/worker/main.go:29-76` |
| Fan-out kiểu A (2 workflow) | `bts-core/console/views/products.py:416-417` |
| Fan-out kiểu B (2 activity) | `bts-core-service/worker/on_sync_data_service.go:63` |
| Fan-out kiểu C (xuyên service) | `payment-service/worker/handler/subscription.go:52` |
| Topic exchange (Stripe routing) | `payment-service/routes/webhook.go:78-93` |
| Poison message → ack 2xx | `payment-service/routes/webhook.go:93-96` |
| ⚠️ Webhook signature bị comment out | `payment-service/routes/webhook.go:57-64` |
| Out-of-order guard | `payment-service/internal/services/subscription.go:129` |
| Event debouncing (`isUpdateNeeded`) | `payment-service/internal/services/subscription.go:155-157` |
| Retry + exponential backoff | `notification-service/server/main.go:74-78` |
| Non-retryable error (DLQ) | `payment-service/worker/handler/subscription.go:42` |
| `transaction.on_commit` | `bts-core/console/views/blockchain_webhook.py:138-139` |
| `select_for_update` + `F()` | `bts-core/dexer/models/inventory.py:88-95` |
| `select_related`/`prefetch_related` | `bts-core/console/views/products.py:226` |
| CloudFront CDN | `bts-core/beetee/settings.py:443, 461` |
| Uber fx DI | `payment-service/worker/worker.go:19-40` |
