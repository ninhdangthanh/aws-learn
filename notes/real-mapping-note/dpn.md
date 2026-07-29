# DPN (Decentralized Proxy Network) — Note kiến thức backend để phỏng vấn

> Mục tiêu: note lại những phần **không phải CRUD** trong hệ thống này — Postgres/index, Redis, RabbitMQ, luồng accounting, kiến trúc realtime — để kể được trong phỏng vấn **Middle Backend (Golang)**.
>
> Mỗi mục đều có 3 phần: **(A) Trong repo này thực tế là gì** → **(B) Lý thuyết cần thuộc** → **(C) Cách trả lời phỏng vấn / mapping sang Go**.
>
> ⚠️ **Quy ước trung thực**: những gì có thật trong code được đánh dấu ✅ kèm đường dẫn file. Những gì **không tồn tại trong repo** được đánh dấu ❌. Những gì tôi đề xuất thêm được đánh dấu 💡 **(đề xuất, chưa có trong code)** — đừng nói với người phỏng vấn là đã làm.

---

## 0. Bản đồ hệ thống (phải thuộc để mở đầu)

```
                  ┌──────────────────────────────────────────────┐
   End user  ───► │ masternode (clientmode)  :9091 HTTP/SOCKS5   │
   (proxy client) │  - proxy_auth: parse user_COUNTRY_REGION...  │
                  │  - matcher/PeerPool: chọn peer               │
                  └───────────────┬──────────────────────────────┘
                                  │ chọn peer, mở stream
                  ┌───────────────▼──────────────────────────────┐
                  │ masternode (peermode)    :9090 QUIC/TLS      │
                  └───────────────┬──────────────────────────────┘
                                  │ QUIC bidirectional stream
                  ┌───────────────▼──────────────────────────────┐
                  │ peernode (máy người dùng chia sẻ băng thông) │
                  │  - proxy_server http.rs / socks5.rs          │
                  └───────────────┬──────────────────────────────┘
                                  ▼  Internet

   ── Control plane ─────────────────────────────────────────────
   admin :8090 (REST + Swagger) + :8099 (WebSocket)
     ├── PostgreSQL  (sqlx, 3 pool × 10 conn)
     ├── Redis       (cache + pub/sub)
     ├── RabbitMQ    (5-6 exchange)
     └── HTTP ──► accounting service :8092  ⚠️ SERVICE RIÊNG, KHÔNG CÓ TRONG REPO
```

**4 crate trong folder:**

| Crate | Binary | Vai trò | LOC |
|---|---|---|---|
| `subnet-dpn-core` | lib `dpn_core` | Types dùng chung, RedisService, GeoService, proto | ~2.4k |
| `subnet-dpn-admin` | `admin` | Control plane: REST API, DB, RabbitMQ, Redis | ~12.7k |
| `subnet-dpn-masternode` | `masternode` | Data plane: 2 mode (`NODE_TYPE=0` peermode / `=1` clientmode) | ~4.5k |
| `subnet-dpn-peernode` | `peernode` | Client chạy trên máy peer + bridge Flutter/WASM | ~2.2k |

**4 loại hạ tầng lưu trữ / truyền tin:**

| Hạ tầng | Dùng để làm gì | Đặc tính chọn |
|---|---|---|
| **PostgreSQL** | Source of truth: user, session, transaction, referral, tier | Cần ACID, cần query tổng hợp |
| **Redis (hash)** | Cache hot-path: proxy account, giá bandwidth, danh sách peer online | Đọc ở mọi request proxy → không thể đụng DB |
| **Redis (pub/sub)** | Đẩy thay đổi config xuống masternode trong <1ms | Cần realtime, chấp nhận mất message |
| **RabbitMQ** | Event bus giữa các microservice: connection, session, tx, stats | Cần bền vững + fan-out nhiều consumer |

> 🎤 **Câu mở đầu phỏng vấn:** "Hệ thống là một mạng proxy phân tán. Điểm không-CRUD nằm ở chỗ: mỗi request proxy phải chọn được một peer trong <1ms, nên toàn bộ dữ liệu xác thực và giá nằm trong RAM của masternode, được đồng bộ qua Redis pub/sub; còn số liệu băng thông thì đi bất đồng bộ qua RabbitMQ về service accounting để tính tiền."

---

# PHẦN 1 — POSTGRESQL

## 1.1 Sơ đồ bảng (migrations: `subnet-dpn-admin/lib/db/migrations/`, 48 file)

Dùng **sqlx** + migration up/down thủ công (`YYYYMMDDHHMMSS_name.up.sql` / `.down.sql`).

| Bảng | Primary key | Ghi chú |
|---|---|---|
| `users` | `deposit_addr` VARCHAR(42) | Địa chỉ ví làm PK (không phải id!) |
| `proxy_accounts` | `id` (hash của proto) | Tài khoản proxy user tạo ra |
| `sessions` | `session_hash` VARCHAR(66) | 1 phiên proxy = client ↔ peer |
| `internal_transactions` | `tx_hash` | Giao dịch nội bộ (chưa lên chain) |
| `transactions` | `tx_hash` | Giao dịch on-chain (deposit/withdrawal) |
| `referrals` | `user_addr` | `referral_code` UNIQUE |
| `user_connection_history` | **`(login_session_id, time_start)`** ⭐ | Lịch sử peer online |
| `user_online_sessions` | **`(user_addr, start_time)`** ⭐ | Phiên online tính điểm |
| `region_info_history` | **`(geoname_id, is_country)`** ⭐ | |
| `user_tiers`, `user_tier_points`, `user_xps` | `user_addr` | |
| `locations` | `geoname_id` | Dữ liệu GeoLite2 |
| `user_bandwidth_price` | `user_addr` | Giá peer đặt |

⭐ = composite primary key (xem 1.3).

**Chi tiết đáng chú ý:** migration `20240103032556_use_user_addr_instead_user_id.up.sql` đổi toàn bộ PK từ `user_id INT8` sang `user_addr VARCHAR(42)` — một cuộc refactor khóa chính trên toàn schema. `20240508123632_drop_fks.up.sql` thì **gỡ bỏ foreign key** trên `sessions` (dấu hiệu bảng ghi nhiều, FK gây chậm insert).

> 🎤 Kể được chuyện này rất tốt: "Bọn tôi drop FK trên bảng ghi nóng vì mỗi INSERT phải check tồn tại ở bảng cha, tốn thêm 1 index lookup + giữ share lock — với write throughput cao thì đó là bottleneck. Đổi lại phải đảm bảo referential integrity ở tầng application."

---

## 1.2 TOÀN BỘ index hiện có trong repo ✅

Grep `CREATE INDEX` ra đúng **11 dòng**:

```sql
-- 20231118164625_init.up.sql
CREATE UNIQUE INDEX idx_users_deposit_addr ON users (deposit_addr);

-- 20240129030137_add_index_column_on_transactions_table.up.sql
CREATE INDEX transactions_from_addr_idx           ON transactions(from_addr);
CREATE INDEX transactions_to_addr_idx             ON transactions(to_addr);
CREATE INDEX transactions_created_at_idx          ON transactions(created_at);
CREATE INDEX internal_transactions_from_addr_idx  ON internal_transactions(from_addr);
CREATE INDEX internal_transactions_to_addr_idx    ON internal_transactions(to_addr);
CREATE INDEX internal_transactions_created_at_idx ON internal_transactions(created_at);

-- 20240129131143_remove_points_type.up.sql
CREATE INDEX user_tier_points_user_addr_idx ON public.user_tier_points (user_addr);

-- 20240531084153_update-user-conn-history-indexes.up.sql
CREATE INDEX IF NOT EXISTS idx_user_connection_history_user_addr
  ON user_connection_history(user_addr);
CREATE INDEX IF NOT EXISTS idx_user_connection_history_user_addr_time_start
  ON user_connection_history(user_addr, time_start);   -- ⭐ COMPOUND

-- 20240610045833_add_session_client_addr_idx.up.sql
CREATE INDEX IF NOT EXISTS idx_sessions_client_addr ON sessions(client_addr);
```

Cộng thêm các unique index **ngầm** do PK và UNIQUE constraint sinh ra.

**Nhận xét: chỉ có DUY NHẤT 1 compound index được tạo tường minh** (`user_addr, time_start`). Còn lại composite là do **composite primary key**.

---

## 1.3 Compound index — phân tích từng cái ✅

### (1) `idx_user_connection_history_user_addr_time_start (user_addr, time_start)` — **cái hay nhất**

Nó phục vụ query này (`lib/db/src/connection_history_dal.rs:60`):

```sql
UPDATE user_connection_history
SET time_end = $2
WHERE user_addr = $1
  AND time_start = (SELECT MAX(time_start)
                    FROM user_connection_history
                    WHERE user_addr = $1)
RETURNING user_addr, time_start, time_end;
```

**Tại sao compound index này là chuẩn sách giáo khoa:**

| Thành phần | Index giúp gì |
|---|---|
| Subquery `MAX(time_start) WHERE user_addr = $1` | B-tree seek tới `user_addr = $1`, rồi **scan ngược 1 bước** để lấy giá trị `time_start` lớn nhất → **O(log n)**, đọc đúng 1 entry. Postgres biến `MAX()` thành `Index Scan Backward … Limit 1`. |
| Outer `WHERE user_addr = $1 AND time_start = X` | Cả 2 cột đều nằm trong index, cùng thứ tự → equality trên cả 2 → seek chính xác 1 row |

Nếu chỉ có index đơn `(user_addr)`: Postgres seek được user_addr nhưng phải đọc **toàn bộ** các row của user đó rồi `Sort`/`Aggregate` để tìm MAX. User online 10.000 lần thì đọc 10.000 row thay vì 1.

> 🎤 **Câu trả lời mẫu**: "Nguyên tắc thiết kế compound index là **cột equality đứng trước, cột range/sort đứng sau** (quy tắc E-R hoặc ESR: Equality → Sort → Range). Ở đây `user_addr` là equality, `time_start` là cột cần lấy MAX, nên `(user_addr, time_start)` cho phép Postgres seek + backward scan lấy đúng một entry."

### (2) PK `user_online_sessions (user_addr, start_time)` — composite PK dùng đúng cách

Phục vụ **cả 3 query** trong `user_online_sessions_dal.rs`:

```sql
-- (a) list phiên của user, mới nhất trước
SELECT ... FROM user_online_sessions WHERE user_addr = $1 ORDER BY start_time DESC;
-- (b) lấy đúng 1 phiên
SELECT ... WHERE user_addr = $1 AND start_time = $2;
-- (c) đóng phiên
UPDATE user_online_sessions SET end_time=$1, updated_at=$2, earned_lp=$3
 WHERE user_addr = $4 AND start_time = $5;
```

- (a): index cho **cả filter lẫn sort** → `Index Scan Backward`, **không có node `Sort`** trong EXPLAIN. Đây là điểm ăn tiền: loại bỏ được sort là loại bỏ được cả `work_mem` spill.
- (b), (c): equality trên full PK → 1 lần seek.

Bonus: PK composite này còn đóng vai trò **ràng buộc nghiệp vụ** — 1 user không thể có 2 phiên online cùng `start_time` (chống double-insert khi WebSocket reconnect).

### (3) PK `user_connection_history (login_session_id, time_start)`

Migration `20240518032937` đổi PK từ `time_start` (một mình!) sang `(login_session_id, time_start)`.

> 🎤 Chuyện hay để kể: "PK ban đầu là `time_start INT` — tức là **toàn hệ thống chỉ được có 1 kết nối mỗi giây**. Khi scale lên nhiều masternode thì collide ngay. Bọn tôi đổi sang composite `(login_session_id, time_start)`, trong đó `login_session_id` là ID phiên đăng nhập, đảm bảo unique theo từng phiên."

⚠️ Lưu ý thứ tự: `login_session_id` đứng trước nên index PK này **không** phục vụ được query `WHERE user_addr = ...` — đó là lý do phải tạo thêm index riêng ở (1). Đây chính là **leftmost prefix rule**.

### (4) PK `region_info_history (geoname_id, is_country)`

Composite để phân biệt "geoname_id này là mã quốc gia hay mã thành phố" — cùng một `geoname_id` có thể xuất hiện ở 2 ngữ cảnh.

### (5) PK `sessions_users (session_id, user_id)`

Bảng join classic many-to-many. Composite PK vừa là unique constraint vừa là index cho chiều `session_id → user_id`.
⚠️ Thiếu index chiều ngược (`user_id`) — muốn query "user này tham gia session nào" thì seq scan.

---

## 1.4 Index THỪA (nói ra được là ghi điểm) ⚠️

### (a) `idx_user_connection_history_user_addr` là **redundant**

```sql
CREATE INDEX idx_user_connection_history_user_addr             ON user_connection_history(user_addr);
CREATE INDEX idx_user_connection_history_user_addr_time_start  ON user_connection_history(user_addr, time_start);
```

Index thứ 2 đã bao trùm index thứ 1 theo **leftmost prefix rule**: mọi query dùng được `(user_addr)` đều dùng được `(user_addr, time_start)`.

**Cái giá phải trả cho index thừa:**
- Mỗi `INSERT`/`UPDATE`/`DELETE` phải maintain thêm 1 B-tree → chậm write
- Tốn disk + tốn shared_buffers (đẩy dữ liệu nóng ra khỏi cache)
- `VACUUM` / `ANALYZE` lâu hơn
- Planner có thêm lựa chọn → đôi khi chọn sai

💡 **Đề xuất:** `DROP INDEX idx_user_connection_history_user_addr;`

### (b) `users.deposit_addr` có tới 3 unique index chồng nhau

```sql
-- init.up.sql
deposit_addr VARCHAR(42) NOT NULL UNIQUE            -- (1) sinh unique index ngầm
CREATE UNIQUE INDEX idx_users_deposit_addr ON users (deposit_addr);  -- (2) tạo tay, TRÙNG (1)
-- 20240103032556
ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (deposit_addr);  -- (3) lại 1 unique index nữa
```

> 🎤 "Một điểm tôi nhận ra khi đọc lại schema: `UNIQUE` constraint trong Postgres **tự động tạo unique index**, nên viết thêm `CREATE UNIQUE INDEX` trên cùng cột là tạo index trùng lặp hoàn toàn. Và khi cột đó thành PK thì có tới ba B-tree y hệt nhau cho một cột."

---

## 1.5 Index còn THIẾU — map query → index đề xuất 💡

**(Toàn bộ phần này là đề xuất của tôi, chưa có trong repo. Nhưng mỗi dòng đều bám vào một query có thật.)**

### Bảng `sessions` — thiếu nhiều nhất

| Query thật trong code | File | Index đề xuất |
|---|---|---|
| `WHERE provider_addr=$1` + `COUNT(*)`, `SUM(bandwidth_usage)` | `session_dal.rs:239` | `(provider_addr) INCLUDE (bandwidth_usage)` |
| `WHERE provider_addr=$1 AND status=$2 ORDER BY handshake_at DESC` | `session_dal.rs:311` | `(provider_addr, status, handshake_at DESC)` |
| `WHERE provider_addr=$1 AND status<>$2 ORDER BY handshake_at DESC LIMIT 20` | `session_dal.rs:~330` | `(provider_addr, handshake_at DESC)` |
| `WHERE status=$1 ORDER BY handshake_at DESC` (toàn bảng) | `session_dal.rs:272` | **partial**: `(handshake_at DESC) WHERE status = 0` |

```sql
-- Session đang active thường chỉ chiếm <1% bảng → partial index nhỏ hơn hàng trăm lần
CREATE INDEX idx_sessions_active_recent
  ON sessions (handshake_at DESC)
  WHERE status = 0;  -- 0 = SessionStatus::Active

-- Cho query có status <> Active: đừng nhét status vào index vì <> không seek được,
-- để handshake_at DESC làm việc rồi filter + LIMIT 20 sẽ dừng sớm
CREATE INDEX idx_sessions_provider_time
  ON sessions (provider_addr, handshake_at DESC);
```

> 🎤 **Điểm ăn tiền**: "Toán tử `<>` không dùng được B-tree để seek — chỉ dùng được để filter sau khi đã đọc. Nên với `WHERE provider_addr=$1 AND status<>$2 ORDER BY handshake_at DESC LIMIT 20`, tôi để `(provider_addr, handshake_at DESC)`: index cho phép đọc theo đúng thứ tự cần, filter `status` loại vài row, và query dừng ngay khi đủ 20 row — không cần đọc hết."

### Bảng `transactions` / `internal_transactions` — đang có 3 index đơn, nên gộp

Hiện tại: `(from_addr)`, `(to_addr)`, `(created_at)` — 3 index rời.

| Query thật | File | Index đề xuất |
|---|---|---|
| `WHERE from_addr=$1 AND tx_type=$2 ORDER BY created_at DESC LIMIT 20` | `tx_dal.rs:122` | `(from_addr, tx_type, created_at DESC)` |
| `WHERE from_addr=$1 AND tx_type=$2 AND tx_status=$3` COUNT | `tx_dal.rs:154` | `(from_addr, tx_type, tx_status)` |
| `SELECT MAX(created_at) WHERE from_addr AND tx_type AND tx_status` (CTE) | `user_dal.rs:249` | `(from_addr, tx_type, tx_status, created_at DESC)` → index-only scan |
| `WHERE to_addr=$1 AND tx_status=$2 AND tx_type=$3` → `SUM(amount)` | `session_dal.rs:244` | `(to_addr, tx_type, tx_status) INCLUDE (amount)` → **covering index** |
| `WHERE tx_status=$1 ORDER BY created_at ASC LIMIT 1` | `tx_dal.rs:93` | `(tx_status, created_at)` |

```sql
-- Covering index: mọi cột query cần đều nằm trong index → Index Only Scan,
-- không cần đụng heap. Với SUM() trên hàng triệu row đây là khác biệt 10-50x.
CREATE INDEX idx_itx_reward_sum
  ON internal_transactions (to_addr, tx_type, tx_status)
  INCLUDE (amount);
```

**Query `WHERE tx_status=$1 ORDER BY created_at ASC LIMIT 1`** ở `tx_dal.rs:93` thực chất là một **hàng đợi job nằm trong DB** (lấy transaction pending kế tiếp để xử lý).

> 🎤 Chỗ này nói được rất nhiều: "Đây là pattern *queue in database*. Nếu chạy nhiều worker song song thì hai worker sẽ cùng lấy một row. Cách xử lý chuẩn của Postgres là `SELECT ... FOR UPDATE SKIP LOCKED` — worker nào lấy được row thì lock, worker khác **bỏ qua** row đang bị lock thay vì chờ. Nếu không thì phải chuyển hẳn sang RabbitMQ."

```sql
-- 💡 Cách đúng cho queue-in-DB
SELECT * FROM transactions
WHERE tx_status = $1
ORDER BY created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

### Bảng `referrals` — thiếu index cho query có LIMIT

```sql
-- referral_dal.rs:293 — không có index nào trên referred_by → SEQ SCAN
SELECT * FROM referrals WHERE referred_by = $1 ORDER BY referred_at DESC LIMIT 20;
-- referral_dal.rs:328
SELECT count(*) FROM referrals WHERE referred_by = $1;
```

💡 `CREATE INDEX idx_referrals_referred_by ON referrals (referred_by, referred_at DESC);`

### Bảng `proxy_accounts`

```sql
-- proxy_acc_dal.rs:111, :351
WHERE pa.user_addr = $1
-- proxy_acc_dal.rs:227 — đây là hot path xác thực proxy theo IP whitelist!
WHERE whitelisted_ip = $1
```

```sql
💡 CREATE INDEX idx_proxy_acc_user ON proxy_accounts (user_addr);
💡 CREATE UNIQUE INDEX idx_proxy_acc_wl_ip ON proxy_accounts (whitelisted_ip)
     WHERE whitelisted_ip IS NOT NULL;   -- partial unique: NULL không bị ràng buộc
```

### Bảng `user_connection_history`

```sql
-- connection_history_dal.rs:95 — chạy khi masternode shutdown
UPDATE user_connection_history SET time_end = $1 WHERE time_end IS NULL;
```
💡 `CREATE INDEX idx_uch_open ON user_connection_history (time_start) WHERE time_end IS NULL;`
Số connection đang mở luôn nhỏ → partial index cực nhỏ, cực nhanh.

---

## 1.6 Lý thuyết index phải thuộc lòng

### Leftmost prefix rule
Index `(A, B, C)` phục vụ được: `WHERE A`, `WHERE A,B`, `WHERE A,B,C`, `WHERE A ORDER BY B`.
**Không** phục vụ được: `WHERE B`, `WHERE C`, `WHERE B,C`.
→ Vì B-tree sắp xếp theo A trước; không biết A thì không seek được.

### Quy tắc ESR (Equality → Sort → Range)
Thứ tự cột trong compound index:
1. Cột so sánh **bằng** (`=`, `IN`)
2. Cột dùng để **sắp xếp** (`ORDER BY`)
3. Cột so sánh **khoảng** (`>`, `<`, `BETWEEN`, `<>`)

Sai thứ tự → sau cột range đầu tiên, các cột phía sau chỉ dùng để filter chứ không seek được.

### Các loại index Postgres nên biết tên

| Loại | Cú pháp | Khi nào dùng |
|---|---|---|
| **Compound / composite** | `(a, b)` | Query lọc/sort nhiều cột |
| **Covering** | `(a, b) INCLUDE (c)` | Đưa cột chỉ-đọc vào index → Index Only Scan |
| **Partial** | `... WHERE cond` | Chỉ index một phần nhỏ dữ liệu (status active, deleted_at IS NULL) |
| **Expression** | `(lower(email))` | Query dùng hàm: `WHERE lower(email)=...` |
| **GIN** | `USING gin(col)` | JSONB, full-text, mảng |
| **BRIN** | `USING brin(created_at)` | Bảng cực lớn, dữ liệu append theo thời gian |

### Đọc EXPLAIN — 4 thứ cần nhìn

```sql
EXPLAIN (ANALYZE, BUFFERS) SELECT ...;
```
1. `Seq Scan` trên bảng lớn → thiếu index
2. Có node `Sort` → index chưa phục vụ `ORDER BY`
3. `rows=` ước tính lệch xa `actual rows=` → thống kê cũ, chạy `ANALYZE`
4. `Index Scan` vs `Index Only Scan` → cái sau không đụng heap, nhanh hơn nhiều

---

## 1.7 MATERIALIZED VIEW ❌ — **KHÔNG có trong repo này**

Tôi đã grep toàn bộ `*.sql`, `*.rs`, `*.yaml`, `*.md`: **0 kết quả** cho `MATERIALIZED`. Cũng **0** `CREATE VIEW` thường.

### Cái dễ nhầm là materialized view

Trong `lib/db/src/model/` có các file tên rất giống matview:

```
storage_rewards_overview.rs
storage_referrals_overview.rs
storage_user_overview.rs
```

Nhưng đọc code thì đây chỉ là **struct Rust map kết quả của một query CTE tính realtime**, không phải view trong DB:

```rust
// model/storage_rewards_overview.rs — chỉ là struct hứng kết quả
#[derive(Debug, sqlx::FromRow)]
pub struct StorageRewardsOverview {
    pub total_rewards: Option<BigDecimal>,
    pub unclaimed_rewards: Option<BigDecimal>,
    pub total_network_rewards: Option<BigDecimal>,
    ...
}
```

Query sinh ra nó (`session_dal.rs:228`) tính **mỗi lần user mở app**:

```sql
WITH ProviderSessionStats AS (
    SELECT COUNT(*) as total_sessions,
           SUM(bandwidth_usage) as total_bandwidth_usages
    FROM sessions WHERE provider_addr = $1
)
SELECT (SELECT total_sessions FROM ProviderSessionStats)          AS total_sessions,
       (SELECT total_bandwidth_usages FROM ProviderSessionStats)  AS total_bandwidth_usages,
       (SELECT sum(amount) FROM internal_transactions
        WHERE to_addr=$1 AND tx_status=$2 AND tx_type=$3)         AS total_network_rewards
```

> 🎤 **Cách nói trung thực và vẫn ghi điểm**: "Hệ thống hiện **không dùng materialized view** — các bảng overview đang được tính realtime bằng CTE với `COUNT`/`SUM` trên bảng `sessions` và `internal_transactions`. Đó chính là chỗ tôi thấy nên chuyển sang materialized view, và tôi đã suy nghĩ về cách làm..."

### 💡 Nếu làm matview thì làm ở đâu, làm thế nào

**Ứng viên số 1: `sessions` → thống kê theo provider.** Vì đây là bảng ghi nhiều nhất (mỗi phiên proxy 1 row), query lại toàn `COUNT`/`SUM` toàn bộ lịch sử của một provider.

```sql
CREATE MATERIALIZED VIEW mv_provider_session_stats AS
SELECT
    provider_addr,
    COUNT(*)                    AS total_sessions,
    SUM(bandwidth_usage)        AS total_bandwidth_usage,
    SUM(total_fee)              AS total_fee,
    MAX(handshake_at)           AS last_session_at
FROM sessions
GROUP BY provider_addr
WITH NO DATA;   -- tạo rỗng, refresh sau, tránh khóa lâu lúc migrate

-- ⚠️ BẮT BUỘC: phải có UNIQUE index thì mới REFRESH CONCURRENTLY được
CREATE UNIQUE INDEX ux_mv_provider_stats ON mv_provider_session_stats (provider_addr);

REFRESH MATERIALIZED VIEW mv_provider_session_stats;             -- lần đầu
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_provider_session_stats; -- các lần sau
```

**Ứng viên số 2: `internal_transactions` → tổng reward theo user.**

```sql
CREATE MATERIALIZED VIEW mv_user_rewards AS
SELECT to_addr AS user_addr,
       tx_type,
       SUM(amount) FILTER (WHERE tx_status = 1) AS success_amount,
       COUNT(*)                                 AS tx_count
FROM internal_transactions
GROUP BY to_addr, tx_type;

CREATE UNIQUE INDEX ux_mv_user_rewards ON mv_user_rewards (user_addr, tx_type);
```

**Bốn điều bắt buộc phải biết về matview (hay bị hỏi):**

| Điều | Chi tiết |
|---|---|
| Không tự cập nhật | Matview là **snapshot đóng băng**. Phải `REFRESH` thủ công (cron / pg_cron / job trong app). |
| `REFRESH` thường **khóa đọc** | `REFRESH MATERIALIZED VIEW` lấy `ACCESS EXCLUSIVE LOCK` → mọi SELECT bị chặn tới khi xong. |
| `CONCURRENTLY` cứu điều trên | Không khóa đọc, nhưng **yêu cầu có UNIQUE index**, và chậm hơn (build bảng tạm rồi diff). |
| Đánh index được | Matview là bảng thật trên đĩa → index bình thường, thậm chí bắt buộc phải có unique index. |

**Trade-off phải nói ra:** matview đánh đổi **độ tươi của dữ liệu** lấy **tốc độ đọc**. Với "tổng reward đã kiếm" thì trễ 5 phút chấp nhận được. Với "số dư khả dụng để rút tiền" thì **tuyệt đối không** — phải đọc bảng gốc.

**Các phương án thay thế matview** (nên biết để so sánh):
1. **Summary table + trigger**: bảng tổng hợp thật, `AFTER INSERT` trigger `UPDATE ... SET total = total + NEW.amount`. Luôn tươi nhưng làm chậm write và dễ tạo hot row contention.
2. **Event sourcing / incremental**: consumer RabbitMQ nghe `SessionTerminated` rồi cộng dồn vào bảng tổng hợp — **hệ thống này đã có sẵn hạ tầng để làm cách này** (queue `events_accounting`).
3. **Cache ở Redis với TTL**: đơn giản nhất, đã dùng cho `uptime_xp_total` (mục 2.4).

---

## 1.8 READ REPLICA / MASTER-SLAVE ❌ — sự thật trong repo này

**Tôi đã tìm kỹ và phải nói thẳng: trong 4 crate này KHÔNG có read replica, KHÔNG có master-slave Postgres.**

Bằng chứng:
- Grep `replica|read_only|standby|slave|pgbouncer|primary_db|write_db`: **0 kết quả** trên toàn repo.
- `config.yaml` chỉ có **một** DB URL duy nhất:
  ```yaml
  db_url: postgresql://postgres:postgres@localhost:5432/admin
  ```
- `docker-compose.yml` chỉ có **một** container `postgres:16`, không có replica.

### Cái thật sự tồn tại — và có thể là cái đang nhớ nhầm

**(a) Hai instance REDIS tách biệt** ✅ — đây gần với "1 read 1 write instance" nhất:

```yaml
# config-masternode.yaml
dpn_redis_uri:        redis://:dpn@localhost:6379   # Redis TOÀN MẠNG (dùng chung mọi masternode)
masternode_redis_uri: redis://:dpn@localhost:6379   # Redis RIÊNG từng masternode
```

Trong dev cả hai trỏ cùng URL, nhưng **kiến trúc code đã tách hoàn toàn hai đường**:

| Instance | Chứa gì | Ai ghi | Ai đọc |
|---|---|---|---|
| `dpn_redis` (global) | `proxy_acc`, `peer_price`, `peer_geo` | admin | mọi masternode |
| `masternode_redis` (local) | `peers_ms#<id>`, channel `peers_updated_ms#<id>` | masternode peermode | masternode clientmode |

> 🎤 "Chúng tôi tách hai Redis: một global cho dữ liệu toàn mạng do admin ghi, một local cho state của riêng masternode. Mục đích là **cô lập blast radius** — masternode ghi/xóa peer liên tục nên không được làm ảnh hưởng Redis global, và khi scale ra nhiều region thì Redis local đặt cùng region với masternode để giảm latency."

**(b) BA connection pool riêng biệt trên cùng một DB** ✅ (`admin/src/main.rs`):

```rust
let user_store       = UserStorage::new(&APP_CONFIG.db_url).await?;        // pool 10
let connection_store = ConnectionStorage::new(&APP_CONFIG.db_url).await?;  // pool 10
let location_store   = LocationStorage::new(&APP_CONFIG.db_url).await?;    // pool 10
```
→ 30 connection tổng, chia theo domain (bulkhead pattern). Cùng một DB, không phải replica.

**(c) Service `accounting` là service RIÊNG** ✅ — `http://localhost:8092`, **không có trong repo này**.

Đây **rất có thể** là chỗ có read replica thật, vì:
- Nó chỉ chạy các query **tổng hợp nặng** (`connection_overview`, `rewards_overview`, `usage_history`) — đúng loại workload người ta tách sang replica.
- Nó nhận event ghi qua RabbitMQ (`events_accounting`) và trả kết quả đọc qua HTTP → **tách write path và read path ở tầng service** rồi.

Nhưng tôi **không thể xác nhận** vì source của nó không nằm ở đây.

> 🎤 **Cách trả lời trung thực và mạnh**: "Trong repo admin thì chỉ có một Postgres instance. Việc tách read/write ở hệ thống này được làm ở **tầng service** chứ không phải tầng database: đường ghi đi qua RabbitMQ vào service accounting, còn đường đọc là HTTP call sang chính service đó. Service accounting nằm ở repo riêng nên tôi không xác nhận được nó có dùng replica không, nhưng workload của nó là aggregate query nặng nên đó chính là ứng viên điển hình cho read replica."

### 💡 Lý thuyết read replica phải thuộc (kiểu gì cũng bị hỏi tiếp)

**Cơ chế:** Postgres **streaming replication** — primary ghi WAL (Write-Ahead Log), stream sang standby, standby replay WAL.

| Chế độ | Cơ chế | Hệ quả |
|---|---|---|
| **Asynchronous** (mặc định) | Primary commit ngay, không chờ standby | Nhanh; có **replication lag**; mất data nếu primary chết |
| **Synchronous** | Primary chờ standby xác nhận nhận WAL | Không mất data; latency write tăng; standby chết → primary treo |

**Vấn đề kinh điển: replication lag → read-your-own-write**

> User bấm "rút tiền" (ghi vào primary) → app redirect sang trang lịch sử (đọc từ replica) → **chưa thấy giao dịch** vì replica chậm 200ms.

Cách xử lý (phải kể được ít nhất 2):
1. **Sticky/critical read**: sau khi user ghi, trong N giây route toàn bộ read của user đó về primary.
2. **Đọc theo LSN**: lưu `pg_current_wal_lsn()` lúc ghi, chờ replica `pg_last_wal_replay_lsn() >= lsn` mới đọc.
3. **Phân loại theo nghiệp vụ**: số dư/giao dịch → primary; dashboard/analytics/report → replica.
4. **Đọc từ cache thay vì replica** cho dữ liệu vừa ghi.

**Trong Go làm thế nào:**
```go
type DB struct {
    write *sql.DB   // primary
    read  *sql.DB   // replica (hoặc []*sql.DB round-robin)
}

func (d *DB) Reader(ctx context.Context) *sql.DB {
    // trong transaction hoặc cần read-after-write → ép về primary
    if mustReadPrimary(ctx) {
        return d.write
    }
    return d.read
}
```
Thư viện thật hay dùng: `pgxpool` × 2, hoặc **pgbouncer/pgpool** làm proxy tự route, hoặc `go-sql-driver` với DSN multi-host + `target_session_attrs=read-only` (Postgres 10+ hỗ trợ ở libpq).

**⚠️ Bẫy quan trọng**: bên trong một **transaction** thì tuyệt đối không được nhảy sang replica — mọi read trong transaction phải ở cùng connection với write.

---

## 1.9 Connection pool & Transaction ✅

File `lib/db/src/connection/mod.rs` — tự viết một wrapper trên `sqlx::PgPool`, có mấy thứ đáng học:

```rust
// 1. Retry có backoff khi acquire connection
async fn acquire_connection_retried(&self) -> anyhow::Result<PoolConnection<Postgres>> {
    const DB_CONNECTION_RETRIES: u32 = 3;
    const BACKOFF_INTERVAL: Duration = Duration::from_secs(1);
    // thử 3 lần, mỗi lần sleep 1s, lần cuối thử thêm rồi bail
}

// 2. statement_timeout đặt ở mức connection
connect_options = connect_options.options([("statement_timeout", timeout_string)]);
```

```rust
// 3. ConnectionHolder — trừu tượng hoá "đang chạy trên pool hay trong transaction"
pub(crate) enum ConnectionHolder<'a> {
    Pooled(PoolConnection<Postgres>),
    Transaction(Transaction<'a, Postgres>),
}
```

> 🎤 `ConnectionHolder` là pattern rất đáng kể: **cùng một hàm DAL chạy được cả trong lẫn ngoài transaction**. Trong Go tương đương với việc định nghĩa interface:
> ```go
> type Querier interface {
>     QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
>     ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
> }
> // cả *sql.DB lẫn *sql.Tx đều thoả interface này
> func (r *Repo) GetUser(ctx context.Context, q Querier, id string) (*User, error)
> ```
> Đây là câu hỏi phỏng vấn Go **rất hay gặp**: "làm sao để repository method dùng được cả với DB và với Tx?"

**🐛 Bug thật trong repo** (`connection_history_dal.rs:80`) — đáng để kể:

```rust
Err(e) => {
    transaction.commit().await.context("rollback()")?;  // ⚠️ COMMIT trong nhánh lỗi!
    Err(...)
}
```
Nhánh lỗi lại gọi `commit()` (comment ghi "rollback()"). Với transaction này thì UPDATE đã fail nên không đổi gì, nhưng đây là bug chờ nổ.

---

# PHẦN 2 — REDIS

## 2.1 Hai instance, hai vai trò ✅

Đã nói ở 1.8(a). Nhắc lại vì hay bị hỏi: **`dpn_redis` (global, admin ghi)** vs **`masternode_redis` (local, masternode ghi)**.

## 2.2 TOÀN BỘ keyspace — key, kiểu, data, ai ghi, ai đọc

Định nghĩa tập trung ở `subnet-dpn-core/src/services/redis.rs`, struct `DPNRedisKey` — **pattern rất đáng học: không rải string key khắp code, gom vào một chỗ**.

```rust
pub struct DPNRedisKey {}
impl DPNRedisKey {
    pub fn get_proxy_acc_kf(id: String) -> (String, String) { ("proxy_acc".to_owned(), id) }
    pub fn get_proxy_acc_chan() -> String { "proxy_acc_updated".to_string() }
    pub fn get_peers_kf(masternode_id: String, ip_u32: u32) -> (String, String) {
        (format!("peers_ms#{}", masternode_id), format!("{}", ip_u32))
    }
    pub fn get_peers_chan(masternode_id: String) -> String { format!("peers_updated_ms#{}", masternode_id) }
    pub fn get_price_kf(peer_addr: String) -> (String, String) { ("peer_price".to_owned(), peer_addr) }
    pub fn get_price_chan() -> String { "price_updated".to_string() }
    pub fn get_geo_kf(mn_id: String, login_session_id: String) -> (String, String) { ... }
    pub fn get_balance_kf(user_addr: String) -> (String, String) { ("client_user_balance".to_owned(), user_addr) }
    pub fn get_peer_queue_k(masternode_id: String) -> String { format!("peer_queue_ms#{}_", masternode_id) }
}
```

### Bảng đầy đủ

| Key | Kiểu | Field | Value (data) | Ai GHI | Ai ĐỌC | Phục vụ API/luồng nào |
|---|---|---|---|---|---|---|
| `proxy_acc` | HASH | `proxy_acc_id` (hash) | `ProxyAccData` JSON: `{id, password, ip_rotation_period, whitelisted_ip, user_addr, country_geoname_id, city_geoname_id, rate_per_kb, rate_per_second, prioritized_ip, prioritized_ip_level, created_at}` | admin: `POST/PUT/DELETE /clients/proxies` | masternode clientmode `client_listener.rs` | **Xác thực mọi request proxy** — hot path |
| `peer_price` | HASH | `user_addr` của peer | `UserBandwidthPrice {user_addr, rate_per_kb, rate_per_second}` | admin: `PUT /connections/bandwidth-price` + `on_peer_connected` | masternode clientmode `matcher.rs`, peermode `peer_listener.rs` | **Ghép giá client ↔ peer** khi chọn peer |
| `peers_ms#<masternode_id>` | HASH | `ip_u32` (IPv4 dạng u32) | `PeerChangedInfo {uuid, login_session_id, ip_u32}` | masternode **peermode** khi peer QUIC connect/disconnect | masternode **clientmode** `matcher.rs` | **Danh sách peer online** — service registry |
| `peer_geo` | HASH | `"{masternode_id}_{login_session_id}"` | `Geo {continent, country, city}` | admin `on_peer_connected` | ⚠️ không consumer nào trong repo này (accounting service?) | Lưu vị trí địa lý phiên |
| `uptime_xp` | HASH | `user_addr` | `start_time` (i64 unix) | admin WebSocket `StartOnlineSession` | admin `stop_online_session`, `get_online_sessions` | **Phiên online đang mở** (chưa ghi DB) |
| `uptime_xp_total` | HASH | `user_addr` | `f64` — tổng số phút online | admin `stop_online_session` | admin `get_uptime_xp` | Cache-aside cho bảng `user_xps` |
| `client_user_balance` | HASH | `user_addr` | balance | ⚠️ **không dùng** trong repo này | — | Khai báo sẵn cho service khác |
| `peer_queue_ms#<id>_` | ZSET | — | score = timestamp | ⚠️ **không dùng** — code `zadd/zrem/zgetall/zsetall` có nhưng không ai gọi | — | Đã thay bằng `PriorityQueue` in-memory |

### 3 kênh Pub/Sub

| Channel | Payload | Publisher | Subscriber |
|---|---|---|---|
| `proxy_acc_updated` | `ProxyAccChanged::{Created(ProxyAccData), Updated(ProxyAccData), Deleted(id), RefreshAll()}` | admin | masternode clientmode `client_listener.rs:174` |
| `price_updated` | `UserBandwidthPrice` | admin | masternode clientmode `matcher.rs:217` |
| `peers_updated_ms#<masternode_id>` | `PeerChanged::{Connected(PeerChangedInfo), Disconnected(PeerChangedInfo)}` | masternode **peermode** | masternode **clientmode** `matcher.rs:292` |

---

## 2.3 Các pattern Redis đã implement — đây là phần đáng kể nhất

### Pattern 1: **Snapshot + Delta (load-on-boot rồi subscribe)** ⭐ điểm nhấn

Đây là pattern hay nhất trong repo. Cả 3 subscriber đều dùng cùng một khuôn:

```rust
// masternode/src/clientmode/client_listener.rs:153  (và matcher.rs:198, :259)
async fn load_and_watch_proxy_accs(self: Arc<Self>) -> anyhow::Result<()> {
    // BƯỚC 1 — SNAPSHOT: nạp toàn bộ state hiện tại từ Redis HASH vào RAM
    let (k, _) = DPNRedisKey::get_proxy_acc_kf("".to_owned());
    let proxy_accs = redis_svc.clone().hgetall::<ProxyAccData>(k)?;
    {
        let mut cached_proxy_acc_wlk = self.cached_proxy_accs.write().await;
        for (_, proxy_acc) in proxy_accs.clone() {
            cached_proxy_acc_wlk.insert(proxy_acc.id.clone(), proxy_acc.clone());
        }
    }

    // BƯỚC 2 — DELTA: subscribe channel, áp thay đổi lên cache in-memory
    let mut subs = pubsub_con.subscribe(&DPNRedisKey::get_proxy_acc_chan()).await?;
    while let Some(msg) = subs.next().await {
        match serde_json::from_str::<ProxyAccChanged>(&resp) {
            Ok(ProxyAccChanged::Created(pad)) | Ok(ProxyAccChanged::Updated(pad)) => {
                cached_proxy_accs_wlk.insert(pad.id.clone(), pad.clone());
                if let Some(ip) = pad.whitelisted_ip.clone() {
                    cached_proxy_accs_wlk.insert(ip, pad.clone());  // index phụ theo IP
                }
            }
            Ok(ProxyAccChanged::Deleted(id)) => { cached_proxy_accs_wlk.remove(&id); }
            ...
        }
    }
}
```

**Vấn đề nó giải quyết:** mỗi request proxy phải xác thực username/password. Nếu query Postgres thì thêm 1-5ms/request và DB chết ngay ở vài nghìn RPS. Nếu query Redis thì vẫn thêm 1 network round-trip. Giải pháp: **giữ toàn bộ dataset trong RAM của masternode, Redis chỉ là kênh đồng bộ**.

Latency xác thực: **~O(1) hash lookup trong RAM, không có network I/O nào cả.**

**Chi tiết tinh tế**: cache in-memory lưu **2 key trỏ cùng 1 object** — `id` và `whitelisted_ip` — để hỗ trợ 2 kiểu xác thực (basic auth và IP whitelist) mà không cần 2 map.

> 🎤 Đây là câu chuyện mạnh nhất để kể: "Bài toán là xác thực proxy ở hot path, không được phép đụng DB. Bọn tôi làm snapshot-then-stream: lúc boot masternode `HGETALL` toàn bộ proxy account từ Redis vào một HashMap trong RAM, sau đó subscribe một channel để nhận delta. Admin khi CRUD proxy account thì vừa `HSET` vào Redis vừa `PUBLISH` sự kiện, masternode nhận và patch cache. Kết quả là xác thực chỉ còn một lookup RAM."

**⚠️ Race condition trong implementation này** (nên chủ động nói ra — chứng tỏ hiểu sâu):
> Giữa `HGETALL` (bước 1) và `SUBSCRIBE` (bước 2) có một khe thời gian. Message publish trong khe đó bị mất vĩnh viễn vì Redis pub/sub là fire-and-forget. **Cách đúng là subscribe TRƯỚC, buffer message lại, rồi mới HGETALL, rồi replay buffer.** Hoặc dùng Redis **Stream** (`XADD`/`XREAD`) thay pub/sub vì stream có lưu trữ và có offset.

### Pattern 2: **Write-through + Broadcast invalidation**

Mọi thay đổi đều làm 2 việc trong 1 hàm (`core/src/services/redis.rs`):

```rust
pub async fn publish_proxy_acc(self: Arc<Self>, proxy_acc_changed: ProxyAccChanged) -> Result<()> {
    match proxy_acc_changed.clone() {
        ProxyAccChanged::Created(pad) | ProxyAccChanged::Updated(pad) => {
            let (k, f) = DPNRedisKey::get_proxy_acc_kf(pad.id.clone());
            self.clone().hset(k, f, pad.clone())?;      // (1) cập nhật STATE (cho node boot sau)
        }
        ProxyAccChanged::Deleted(id) => { self.clone().hdel(k, f)?; }
        ProxyAccChanged::RefreshAll() => {}
    };
    self.clone().publish(                                // (2) BROADCAST cho node đang chạy
        DPNRedisKey::get_proxy_acc_chan(),
        serde_json::to_string(&proxy_acc_changed).unwrap(),
    ).await?;
    Ok(())
}
```

**Tại sao cần cả hai:** HASH phục vụ node **boot sau** (đọc snapshot); PUBLISH phục vụ node **đang chạy** (nhận delta). Thiếu HASH thì node restart mất hết state; thiếu PUBLISH thì cache stale tới lần restart sau.

**⚠️ Vấn đề dual-write phải nói ra:** thứ tự là DB → Redis HSET → Redis PUBLISH, **cả ba đều không atomic**. Nếu crash giữa chừng thì DB và Redis lệch nhau.
Mitigation trong code: khi admin khởi động thì **xoá sạch và nạp lại** (xem Pattern 4).
💡 Cách đúng hơn: **transactional outbox** — ghi DB và ghi bảng `outbox` trong cùng 1 transaction, một worker riêng đọc outbox rồi đẩy sang Redis/RabbitMQ.

### Pattern 3: **Cache-aside (lazy loading)**

Hai chỗ dùng:

```rust
// admin/src/user/mod.rs:565 — cache-aside cho tổng uptime
async fn get_uptime_xp(self: Arc<Self>, user_addr: String) -> anyhow::Result<f64> {
    let (k, f) = ("uptime_xp_total".to_string(), user_addr.clone());
    if let Ok(uptime_redis) = self.redis_svc.hget::<f64>(k.clone(), f.clone()) {
        return_uptime = uptime_redis;                              // CACHE HIT
    } else {
        let uptime_xp = self.store.find_or_create_xp(user_addr).await?;  // CACHE MISS → DB
        return_uptime = uptime_xp;
        self.redis_svc.hset(k, f, uptime_xp)?;                     // ghi ngược vào cache
    }
    Ok(return_uptime)
}
```

```rust
// masternode/src/clientmode/matcher.rs:540 — 2 tầng cache: RAM → Redis
async fn get_peer_price(self: Arc<Self>, peer_id: String) -> Result<UserBandwidthPrice> {
    let rlk = self.cached_peer_price.read().await;
    match rlk.get(&peer_id) {
        Some(price) => Ok(price.clone()),               // L1: RAM
        None => {
            drop(rlk);
            let (k, f) = DPNRedisKey::get_price_kf(peer_id.clone());
            match self.dpn_redis_svc.hget::<UserBandwidthPrice>(k, f) {   // L2: Redis
                Ok(price) => { self.cached_peer_price.write().await.insert(peer_id, price.clone()); Ok(price) }
                Err(e) => Err(e),
            }
        }
    }
}
```
→ **Cache 2 tầng**: L1 in-process HashMap, L2 Redis, L3 Postgres. Kinh điển.

**⚠️ Không có TTL ở bất kỳ đâu** — không một lệnh `EXPIRE`/`SETEX` nào trong toàn repo. Key sẽ phình mãi. Cũng không có invalidation cho `uptime_xp_total` khi DB đổi ngoài luồng.

### Pattern 4: **Cleanup on boot / on shutdown** (self-healing)

```rust
// admin/src/connection/mod.rs:419 — khi admin KHỞI ĐỘNG
self.redis_svc.remove_all_proxy_accs().await?;          // 1. xoá sạch cache cũ
let total = self.upload_proxy_accs().await?;            // 2. nạp lại toàn bộ từ DB
self.redis_svc.publish_proxy_acc(ProxyAccChanged::RefreshAll()).await?;  // 3. báo mọi node
```

```rust
// core/src/services/redis.rs — khi masternode TẮT
pub async fn remove_all_peers(self: Arc<Self>, masternode_id: String) -> anyhow::Result<()> {
    let peers = self.hgetall::<PeerChangedInfo>(k.clone())?;
    for (_, change) in peers {
        // publish Disconnected cho từng peer để clientmode gỡ khỏi pool
        let change = PeerChanged::Disconnected(PeerChangedInfo { ... });
        self.publish(DPNRedisKey::get_peers_chan(masternode_id.clone()), ...).await?;
    }
    self.del(k)   // rồi xoá hash
}
```

> 🎤 "Vì Redis pub/sub không đảm bảo giao hàng, chúng tôi coi Redis là **soft state**: mỗi lần service khởi động thì xoá sạch và dựng lại từ Postgres — source of truth luôn là DB. Đó là cách self-heal khi cache bị lệch."

### Pattern 5: **Redis làm Service Registry / Discovery**

`peers_ms#<masternode_id>` chính là service registry: peer connect → đăng ký (HSET + PUBLISH Connected); peer disconnect → hủy đăng ký (HDEL + PUBLISH Disconnected). Thay cho Consul/etcd.

### Pattern 6: **Redis giữ ephemeral state cho WebSocket**

`uptime_xp` HASH lưu `start_time` của phiên online **đang mở**. Chỉ khi WebSocket đóng mới ghi 1 row vào Postgres:

```rust
// admin/src/user/mod.rs:670
async fn start_online_session(self: Arc<Self>, user_addr: String) -> Result<(), Error> {
    let time_now = Utc::now().timestamp();
    let (k, f) = DPNRedisKey::get_uptime_xp_kf(user_addr);
    self.redis_svc.hset(k, f, time_now)?;   // chỉ ghi Redis, KHÔNG ghi DB
    Ok(())
}

async fn stop_online_session(self: Arc<Self>, user_addr: String, start_time: i64) -> Result<(), Error> {
    if let Ok(redis_start_time) = self.redis_svc.hget::<i64>(k.clone(), f.clone()) {
        if start_time == redis_start_time {          // ⭐ optimistic check chống race
            let addition = (time_now - redis_start_time) as f64 / 60.0;
            total_uptime_redis += addition;
            self.redis_svc.hset(k_total, f_total, total_uptime_redis)?;   // cộng dồn ở Redis
            self.redis_svc.hdel(k, f);
            self.store.insert_online_session(user_addr, start_time, time_now).await;  // MỚI ghi DB
        }
    }
}
```

> 🎤 "Session online thay đổi liên tục nhưng chỉ cần persist một lần khi kết thúc. Nên chúng tôi giữ `start_time` ở Redis, tới lúc đóng WebSocket mới tính duration và ghi đúng **một row** vào Postgres — thay vì UPDATE liên tục. Đây là **write coalescing**: gộp N lần ghi thành 1."

Đoạn `if start_time == redis_start_time` là **optimistic concurrency check** — nếu user mở tab thứ 2, `start_time` trong Redis đã bị ghi đè, tab cũ đóng sẽ không ghi nhầm.

⚠️ Nhược điểm: nếu **Redis mất data** (restart, không persist) thì mọi phiên đang mở biến mất. Docker-compose có `--save 20 1` (RDB snapshot mỗi 20s nếu có ≥1 thay đổi) — vẫn có thể mất tới 20s dữ liệu. 💡 Nên bật AOF (`appendonly yes`).

---

## 2.4 Redis Pub/Sub — nó xử lý vấn đề gì ⭐

**Bài toán:** Admin đổi giá bandwidth của một peer. Có 20 masternode đang giữ giá cũ trong RAM. Làm sao để cả 20 node biết trong <100ms?

**Các lựa chọn và lý do loại:**

| Cách | Vấn đề |
|---|---|
| Masternode poll Redis mỗi giây | Trễ tới 1s + 20 node × N key = tải vô ích |
| Admin gọi HTTP tới từng masternode | Admin phải biết địa chỉ mọi node; node scale thì phải update; node chết thì retry |
| Đọc Redis mỗi request | Thêm 1 RTT mạng vào **mọi** request proxy |
| **Redis Pub/Sub** ✅ | Push, fan-out tự động, publisher không cần biết subscriber là ai |

**Vì sao ở đây pub/sub là lựa chọn ĐÚNG dù nó không đảm bảo giao hàng:**
1. Payload là **state cuối cùng**, không phải delta cộng dồn → mất 1 message rồi nhận message sau vẫn hội tụ đúng.
2. Có **snapshot ở Redis HASH** làm lưới an toàn → boot lại là đúng.
3. Có `RefreshAll()` làm nút reset thủ công.
4. Hậu quả của mất message là **giá sai tạm thời**, không phải mất tiền — chấp nhận được.

> 🎤 **Câu hỏi chắc chắn bị hỏi: "Sao không dùng RabbitMQ cho việc này?"**
> Trả lời: "Hai thứ giải quyết hai bài toán khác nhau. RabbitMQ đảm bảo *mỗi message được xử lý đúng một lần bởi một consumer* — hợp cho event tài chính. Redis pub/sub là *broadcast state tới mọi node đang sống*, không cần bền vững vì payload là trạng thái cuối. Nếu dùng RabbitMQ ở đây thì mỗi masternode phải tạo một exclusive queue riêng, và khi node chết thì message dồn lại vô ích. Ngoài ra latency Redis pub/sub thấp hơn đáng kể."

**So sánh 3 lựa chọn broadcast (nên thuộc):**

| | Redis Pub/Sub | Redis Streams | RabbitMQ |
|---|---|---|---|
| Bền vững | ❌ fire-and-forget | ✅ lưu trong stream | ✅ durable queue |
| Consumer offline | Mất message | Đọc lại từ offset | Message chờ trong queue |
| Replay | ❌ | ✅ (`XREAD` từ ID bất kỳ) | ❌ (trừ khi dùng stream plugin) |
| Ack | ❌ | ✅ (consumer group) | ✅ |
| Latency | Thấp nhất | Thấp | Cao hơn |
| Dùng khi | Broadcast state tới N node | Event log cần replay | Task queue, event tài chính |

💡 **Nếu được cải thiện**: chuyển 3 channel này sang **Redis Streams** để đóng khe race snapshot↔subscribe, và có replay khi node mất kết nối tạm thời.

---

## 2.5 Vấn đề nghiêm trọng nhất trong code Redis 🐛

**`RedisService` mở connection MỚI cho MỖI thao tác.**

```rust
// core/src/services/redis.rs — mọi hàm đều bắt đầu bằng dòng này
pub fn hset<T>(self: Arc<Self>, key: String, field: String, obj: T) -> Result<(), Error> {
    let mut conn = self.client.get_connection()?;   // ⚠️ TCP handshake + AUTH mỗi lần gọi!
    conn.hset(...)
}
pub fn hget<T>(...)     { let mut conn = self.client.get_connection()?; ... }
pub fn hgetall<T>(...)  { let mut conn = self.client.get_connection()?; ... }
pub fn del(...)         { let mut conn = self.client.get_connection()?; ... }
pub async fn publish(...) { let mut conn = self.client.get_connection()?; ... }
```

`redis::Client::get_connection()` **mở TCP connection mới** mỗi lần gọi, không phải lấy từ pool.

**Hậu quả:** mỗi lệnh Redis tốn TCP 3-way handshake + AUTH + lệnh + đóng. Với `get_peers()` lặp qua N peer gọi `hget` N lần → N connection. Ở tải cao sẽ **cạn file descriptor** và **cạn port ephemeral**.

💡 Sửa: dùng `redis::aio::ConnectionManager` hoặc `deadpool-redis` / `bb8-redis`.

> 🎤 Đây là câu chuyện **rất tốt** để kể phần "bạn tìm ra vấn đề gì": "Khi đọc lại tầng Redis tôi phát hiện mọi thao tác đều gọi `get_connection()` — trong crate `redis` của Rust thì đó là mở TCP connection mới chứ không phải lấy từ pool. Ở Go thì `go-redis` mặc định đã có pool nên ít ai vấp, nhưng nguyên tắc chung là: connection tới datastore phải được pool, vì chi phí handshake lớn hơn nhiều so với chi phí lệnh."

**Mapping Go:**
```go
rdb := redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    Password:     "dpn",
    PoolSize:     10 * runtime.NumCPU(),  // go-redis pool sẵn
    MinIdleConns: 10,
})
// Pub/Sub trong Go
pubsub := rdb.Subscribe(ctx, "proxy_acc_updated")
for msg := range pubsub.Channel() {
    var changed ProxyAccChanged
    json.Unmarshal([]byte(msg.Payload), &changed)
}
```

## 2.6 Checklist "đã implement gì với Redis" (trả lời nhanh)

1. ✅ Hash làm **distributed cache** (proxy_acc, peer_price, peers)
2. ✅ **Pub/Sub** 3 channel để invalidate/đồng bộ cache
3. ✅ **Snapshot + delta** (HGETALL rồi SUBSCRIBE)
4. ✅ **Cache-aside** 2 tầng (RAM → Redis → Postgres)
5. ✅ **Write-through + broadcast**
6. ✅ **Service registry** cho peer online
7. ✅ **Ephemeral state store** cho WebSocket session (write coalescing)
8. ✅ **Cleanup on boot/shutdown** để self-heal
9. ✅ **Key namespace tập trung** (`DPNRedisKey`)
10. ✅ **Tách 2 instance** theo blast radius (global vs per-masternode)
11. ✅ Hỗ trợ **TLS** (`rediss://`) qua `parse_redis_uri`
12. ⚠️ Có code ZSET (`zadd`/`zrem`/`zgetall`/`zsetall`) cho peer queue nhưng **không dùng** — đã thay bằng PriorityQueue in-memory
13. ❌ Không TTL, ❌ không pool connection, ❌ không Lua script, ❌ không distributed lock, ❌ không rate limiter

---

# PHẦN 3 — RABBITMQ

## 3.1 Toàn bộ Exchange ✅

Khai báo tại `admin/src/events_queue/mod.rs::declare_and_bind_queues()` — **admin là service chịu trách nhiệm khai báo topology**, chạy lúc boot (`main.rs` gọi `setup_rabbitmq()` đầu tiên, fail thì thoát).

| Exchange | Kiểu | Durable | Mục đích |
|---|---|---|---|
| `dpn-events` | **topic** | ✅ | Event nghiệp vụ chính: connection, session, deposit, withdrawal, referral, tappoint |
| `dpn-stats` | **fanout** | ✅ | Số liệu băng thông realtime |
| `dpn-txs` | **topic** | ✅ | Giao dịch |
| `dpn-withdrawals` | **fanout** | ✅ | Yêu cầu rút tiền on-chain |
| `dpn-balances` | **fanout** | ✅ | Cập nhật số dư |
| `dpn-notifications` | **topic** | ✅ | Thông báo (đăng ký...) |

## 3.2 Toàn bộ Binding — routing key nào đi vào queue nào ⭐

### `dpn-events` (topic) — quan trọng nhất

| Routing key | Queue nhận | Ai consume |
|---|---|---|
| `connection` | `connection-events_admin` | ✅ **admin** (`ConsumerServiceImpl::process_peer_connection_events`) |
| `deposit` | `connection-events_explorer` ⚠️ | service explorer (repo khác) |
| `deposit` | `events_accounting` | **accounting** (repo khác) |
| `referral` | `events_accounting` | accounting |
| `session` | `events_accounting` | accounting |
| `withdrawal` | `events_accounting` | accounting |
| `session` | `session-events_admin` | ✅ **admin** (`process_session_events`) |
| `session` | `session-events_explorer` | explorer |
| `session` | `session-events_websocket` | websocket gateway |
| `session` | `session-events_notification` | notification service |
| `tappoint` | `tappoint-events_admin` | ✅ **admin** (`process_tappoint_events`) |

### Các exchange fanout

| Exchange | Queue | Ai consume |
|---|---|---|
| `dpn-stats` (fanout) | `stats_websocket` | websocket gateway |
| `dpn-txs` (topic, bind rk="") | `txs_explorer` | explorer |
| `dpn-withdrawals` (fanout) | `txs_onchain` | worker ký & gửi giao dịch on-chain |
| `dpn-balances` (fanout) | `balances` | service cập nhật số dư |
| `dpn-notifications` (topic) | `notification_register` | notification service |

---

## 3.3 ⭐ "Có chỗ nào bắn vào exchange mà nhiều queue nhận không?" — CÓ, ĐÂY LÀ ĐIỂM NHẤN

**Routing key `session` được bind vào NĂM queue khác nhau trên cùng exchange `dpn-events`:**

```
masternode publish 1 message: DPNEvent::SessionTerminated
        │  exchange=dpn-events  routing_key="session"
        ▼
   ┌────────────────── dpn-events (topic) ──────────────────┐
   │                                                        │
   ├──► events_accounting            → tính tiền phiên      │
   ├──► session-events_admin         → ✅ admin ghi DB      │
   ├──► session-events_explorer      → hiển thị explorer    │
   ├──► session-events_websocket     → push realtime web UI │
   └──► session-events_notification  → gửi thông báo        │
```

**Một message → RabbitMQ nhân bản thành 5 bản, mỗi queue một bản độc lập.** Mỗi consumer ack riêng, consumer này chết không ảnh hưởng consumer kia.

Code sinh ra binding (`events_queue/mod.rs`):

```rust
declare_and_bind_queue(channel, EVENTS_ACCOUNTNG_QUEUE,          EVENTS_EXCHANGE, SESSION_ROUTING_KEY).await?;
declare_and_bind_queue(channel, SESSION_EVENTS_ADMIN_QUEUE,      EVENTS_EXCHANGE, SESSION_ROUTING_KEY).await?;
declare_and_bind_queue(channel, SESSION_EVENTS_EXPLORER_QUEUE,   EVENTS_EXCHANGE, SESSION_ROUTING_KEY).await?;
declare_and_bind_queue(channel, SESSION_EVENTS_WEBSOCKET_QUEUE,  EVENTS_EXCHANGE, SESSION_ROUTING_KEY).await?;
declare_and_bind_queue(channel, SESSION_EVENTS_NOTIFICATION_QUEUE, EVENTS_EXCHANGE, SESSION_ROUTING_KEY).await?;
```

Tương tự, queue `events_accounting` được bind **4 routing key** (`session`, `deposit`, `withdrawal`, `referral`) → một queue nhận 4 loại event. Đây là chiều ngược lại: **N routing key → 1 queue**.

> 🎤 **Trả lời mẫu**: "Có. Sự kiện `SessionTerminated` được publish một lần vào topic exchange `dpn-events` với routing key `session`, và có năm queue cùng bind key đó: accounting để tính tiền, admin để ghi DB, explorer để hiển thị, websocket để push realtime, notification để gửi thông báo. RabbitMQ nhân bản message ra năm queue, mỗi consumer xử lý và ack độc lập.
> Điểm quan trọng của **publish/subscribe qua exchange** là **producer không biết gì về consumer**. Khi cần thêm một service mới, chỉ cần bind thêm một queue vào exchange — **không sửa một dòng nào ở masternode**. Đó chính là điểm khác biệt với việc gọi HTTP trực tiếp giữa các service."

**So sánh topic vs fanout — biết lúc nào dùng cái nào:**

| | Topic (`dpn-events`) | Fanout (`dpn-stats`) |
|---|---|---|
| Routing | Theo routing key + pattern (`*`, `#`) | Bỏ qua routing key, gửi mọi queue đã bind |
| Dùng khi | Nhiều loại event, consumer chọn lọc | Mọi consumer cần mọi message |
| Ở đây | 6 loại event khác nhau | Stats — ai cũng cần hết |

Trong code có comment thẳng chỗ này:
```rust
// masternode/src/integration/msg_queue.rs
.basic_publish(
    STATS_EXCHANGE,
    "",  // fanout exchange type ignores routing key
    ...
)
```

---

## 3.4 Payload — vận chuyển data gì

Định nghĩa ở `core/src/types/msg_queue.rs`. Serialize bằng **JSON** (`serde_json::to_string`).

```rust
pub enum DPNEvent {
    PeerConnected(PeerConnectedExtra),        // → rk "connection"
    PeerDisconnected(PeerDisconnectedExtra),  // → rk "connection"
    SessionCreated(SessionCreatedExtra),      // → rk "session"
    SessionTerminated(SessionTerminatedExtra),// → rk "session"
    Deposit(DepositExtra),                    // → rk "deposit"
    Withdrawal(WithdrawalExtra),              // → rk "withdrawal"
    Referral(ReferralExtra),                  // → rk "referral"
}
```

| Event | Trường dữ liệu |
|---|---|
| `PeerConnected` | `{masternode_id, peer_addr, login_session_id, info: PeernodeInfo{peer_id, ip_addr, throughput, rate_per_kb, rate_per_second, city_geoname_id, country_geoname_id}}` |
| `PeerDisconnected` | `{masternode_id, peer_addr, login_session_id}` |
| `SessionCreated` / `SessionTerminated` | `{masternode_id, session: EphemeralSession{hash, client_identifier, client_addr, peer_addr, rate_per_kb, rate_per_second, bandwidth_usage, handshaked_at, end_at, login_session_id}, reason: SessionTerminationReason}` |
| `Deposit` | `{from, to, amount, tx_hash}` |
| `Withdrawal` | `{user_addr, withdrawal_addr}` |
| `Referral` | `{referrer_addr, referee_addr}` |
| `PeerStats` (dpn-stats) | `{masternode_id, session_hash, download, upload, c_download, c_upload, login_session_id}` — **đơn vị KB** |
| tappoint | `Vec<Vec<u8>>` — mảng byte protobuf `UserOnlinePoint{user_addr, poll_at, last_poll_at}` |

**Chi tiết hay:** stats được **đổi đơn vị byte → KB và làm tròn lên** ngay tại masternode trước khi publish:

```rust
// masternode/src/integration/msg_queue.rs::publish_stats
let admin_stats = PeerStats {
    download:   (stats.download   as f64 / 1024.0).ceil() as u64,
    upload:     (stats.upload     as f64 / 1024.0).ceil() as u64,
    c_download: (stats.c_download as f64 / 1024.0).ceil() as u64,
    c_upload:   (stats.c_upload   as f64 / 1024.0).ceil() as u64,
    ...
};
```
`.ceil()` = làm tròn **lên**, nghĩa là **luôn có lợi cho peer** khi tính tiền. Đây là quyết định nghiệp vụ nằm trong code.

**`SessionTerminationReason`** — enum lý do kết thúc phiên, đi kèm event để accounting biết cách xử lý:
```rust
pub enum SessionTerminationReason {
    ClientInactive, PeerDisconnected, SystemShutdown, ClientLowBalance, RotatedIP,
}
```

---

## 3.5 Producer / Consumer — ai làm gì

### Producer

| Service | File | Publish gì |
|---|---|---|
| **masternode** (cả 2 mode) | `integration/msg_queue.rs` | `dpn-events` (connection/session/tappoint), `dpn-stats` (fanout) |
| **admin** | `events_queue/publisher.rs` | `dpn-events` (đủ 7 loại), `dpn-notifications` |

Kiến trúc producer trong masternode đáng học — **channel nội bộ tách producer khỏi network**:

```rust
pub struct MsgQueueService {
    dpn_events_tx: Sender<DPNEvent>,         // các module khác gửi vào đây (non-blocking)
    dpn_events_rx: RwLock<Receiver<DPNEvent>>,
    stats_tx: Sender<PeerStats>,
    stats_rx: RwLock<Receiver<PeerStats>>,
    peers_online_point_tx: Sender<Vec<Vec<u8>>>,
    ...
}
// 3 tokio task riêng, mỗi task giữ 1 AMQP channel, đọc từ mpsc rồi publish
```
Buffer 1024 message. Business logic gọi `send_dpn_event()` không bao giờ bị chặn bởi network AMQP.

> 🎤 Trong Go y hệt: `chan Event` với buffer + goroutine publisher. Điểm cần nói thêm: **buffer đầy thì làm gì?** Ở đây `.send().await` sẽ chờ (backpressure). Trong Go phải chọn: block, drop, hay `select` với `default` để drop có kiểm soát.

Có **auto-reconnect** khi AMQP channel chết:
```rust
let channel_state = channel.status().state();
if channel_state == lapin::ChannelState::Closed || channel_state == lapin::ChannelState::Error {
    info!("reconnecting to rabbitmq");
    loop {
        if let Ok(conn) = Connection::connect(&self.rmq_uri, ConnectionProperties::default()).await {
            if let Ok(c) = conn.create_channel().await { channel = c; break; }
        }
    }
}
```
⚠️ Reconnect loop này **không có backoff** — RabbitMQ down thì spin liên tục 100% CPU. 💡 Cần exponential backoff + jitter.

### Consumer (chỉ trong admin) — `events_queue/consumer.rs`

3 consumer chạy song song qua `tokio::select!`:

```rust
tokio::select! {
    _ = shutdown_rx.recv() => Ok(()),
    Err(e) = _self.clone().process_peer_connection_events() => Err(e),  // CONNECTION_EVENTS_ADMIN_QUEUE
    Err(e) = _self.clone().process_session_events()         => Err(e),  // SESSION_EVENTS_ADMIN_QUEUE
    Err(e) = _self.clone().process_tappoint_events()        => Err(e),  // TAPPOINT_EVENT_QUEUE
}
```

**Xử lý ack/nack đúng chuẩn:**
```rust
match _self.conn_svc.on_peer_connected(extra.info, extra.masternode_id, extra.login_session_id).await {
    Ok(_)  => { delivery.ack(BasicAckOptions::default()).await; }
    Err(e) => { delivery.nack(BasicNackOptions::default()).await; error!("handle event failed err={}", e); }
}
```

---

## 3.6 Lý thuyết RabbitMQ + các vấn đề trong code này

### Manual ack — cơ chế bảo đảm

`BasicConsumeOptions::default()` có `no_ack = false` → **manual acknowledgement**. Message chỉ bị xoá khỏi queue khi consumer gọi `ack`. Consumer crash giữa chừng → RabbitMQ redeliver cho consumer khác.

→ Đây là **at-least-once delivery**. Hệ quả bắt buộc: **consumer phải idempotent**.

**Code đã xử lý idempotency đúng ở vài chỗ:**
```sql
-- tx_dal.rs:35
INSERT INTO transactions (...) VALUES (...) ON CONFLICT (tx_hash) DO NOTHING;
-- user_xp_dal.rs:24
INSERT INTO user_xps (...) VALUES (...) ON CONFLICT (user_addr) DO NOTHING;
```
`ON CONFLICT DO NOTHING` = nhận lại message trùng thì không sinh bản ghi trùng. **Đây chính là cách xử lý duplicate delivery.**

> 🎤 "Vì RabbitMQ đảm bảo at-least-once chứ không phải exactly-once, consumer bắt buộc phải idempotent. Chúng tôi làm bằng cách dùng natural key có tính xác định — `tx_hash` là hash của nội dung giao dịch — rồi `INSERT ... ON CONFLICT DO NOTHING`. Nhận lại cùng một message thì kết quả không đổi."

### 🐛 Vấn đề 1: `nack` mặc định KHÔNG requeue và KHÔNG có DLQ

`BasicNackOptions::default()` có `requeue = false`. Không có exchange nào được khai báo làm **dead-letter exchange**, cũng không có argument `x-dead-letter-exchange` trong `QueueDeclareOptions`.

→ **Message xử lý lỗi bị vứt đi vĩnh viễn.** Nếu DB tạm thời down, các event `SessionTerminated` trong thời gian đó **mất luôn** → mất tiền của peer.

💡 Cách đúng:
```
1. Khai báo DLX + DLQ:
   x-dead-letter-exchange: "dpn-events-dlx"
   x-message-ttl / x-max-length
2. nack(requeue=false) → message rơi vào DLQ thay vì bị xoá
3. Có retry queue với TTL tăng dần (delayed retry pattern)
4. Alert khi DLQ có message
```

> 🎤 Đây là câu chuyện **rất giá trị**: "Một điểm tôi thấy cần sửa là consumer `nack` với `requeue=false` mà không có dead-letter queue — message lỗi biến mất hẳn. Với event tài chính thì đó là mất tiền. Pattern chuẩn là DLX + retry queue với TTL tăng dần, và alert khi DLQ có message."

### 🐛 Vấn đề 2: KHÔNG có `basic_qos` / prefetch

Không có lệnh `basic_qos` nào trong toàn repo. Mặc định AMQP là **prefetch không giới hạn** → RabbitMQ đẩy toàn bộ queue vào consumer. Cộng thêm mỗi delivery lại `tokio::spawn` một task:

```rust
while let Some(delivery) = consumer.next().await {
    tokio::spawn(async move { ... });   // không giới hạn số task
}
```
→ Queue tồn 100.000 message thì spawn 100.000 task đồng thời, cạn connection pool DB (chỉ có 10), OOM.

💡 `channel.basic_qos(50, BasicQosOptions::default()).await?` → mỗi lúc tối đa 50 message chưa ack.

> 🎤 "Prefetch count là cơ chế backpressure của RabbitMQ. Không đặt thì broker đẩy hết queue vào consumer. Rule of thumb: prefetch ≈ số worker song song, và phải nhỏ hơn kích thước connection pool DB."

Mapping Go: `ch.Qos(50, 0, false)` trong `amqp091-go`.

### 🐛 Vấn đề 3: `tokio::spawn` mỗi message → mất thứ tự

`SessionCreated` và `SessionTerminated` của cùng một session có thể được xử lý **song song, sai thứ tự** → terminate xử lý trước create → mất dữ liệu.

💡 Cách xử lý: partition theo key (mọi event cùng `session_hash` vào cùng worker) hoặc dùng consistent hashing exchange plugin.

### 🐛 Vấn đề 4: Publisher tạo connection AMQP MỚI cho MỖI message

```rust
// admin/src/events_queue/publisher.rs — trong publish_dpn_event()
async fn publish_dpn_event(self: Arc<Self>, event: DPNEvent) -> Result<(), Error> {
    let mut channel = Connection::connect(&APP_CONFIG.rmq_uri, ConnectionProperties::default())
        .await?
        .create_channel()
        .await?;   // ⚠️ TCP + AMQP handshake mỗi lần publish!
    ...
}
```
Giống hệt lỗi ở Redis. Masternode thì làm đúng (giữ channel dài hạn), admin thì làm sai.

**Nguyên tắc AMQP cần thuộc:** *Connection* là TCP, tốn kém, dùng lâu dài, chia sẻ toàn app. *Channel* là kênh logic trong connection, nhẹ, **mỗi goroutine/thread một channel**, không share channel giữa các thread.

### 🐛 Vấn đề 5: Không có Publisher Confirms

`basic_publish` không bật `confirm_select`. Publish xong là coi như xong — **không biết broker đã nhận chưa**. Nếu broker chết đúng lúc đó, message mất mà producer không hay.

💡 `channel.confirm_select()` rồi `channel.wait_for_confirms()`. Trade-off: chậm hơn, nhưng bắt buộc với event tài chính.

### 🐛 Vấn đề 6: Binding sai tên (bug thật)

```rust
declare_and_bind_queue(channel, CONNECTION_EVENTS_EXPLORER_QUEUE, EVENTS_EXCHANGE, DEPOSIT_ROUTING_KEY).await?;
//                              ^^^ tên là "connection-events_explorer"      ^^^ nhưng bind key "deposit"
```
Queue tên `connection-events_explorer` lại bind routing key `deposit` → **explorer không bao giờ nhận được connection event**.

Ngoài ra:
- `TXS_ADMIN_QUEUE` được khai báo hằng số nhưng **không bao giờ được bind** → dead code.
- `dpn-txs` là **topic** exchange nhưng bind với routing key rỗng `""` → chỉ khớp message publish với routing key rỗng. Nếu producer dùng `TXS_ROUTING_KEY="txs"` thì message **rơi vào hư vô**. (Với fanout thì `""` mới đúng.)
- Hằng số `EVENTS_ACCOUNTNG_QUEUE` — typo, thiếu chữ `I` trong "ACCOUNTING".

> 🎤 Kể mấy cái này chứng tỏ đọc code kỹ: "Có một class lỗi rất khó phát hiện với RabbitMQ: bind sai routing key thì **không có lỗi nào cả** — message chỉ đơn giản là không tới. Producer publish thành công, exchange nhận, nhưng không queue nào khớp thì message bị **drop im lặng**. Cách phòng: bật `mandatory` flag để nhận `basic.return` khi không route được, và monitor metric `unroutable messages` của broker."

---

## 3.7 Checklist RabbitMQ (trả lời nhanh)

1. ✅ 6 exchange durable: 3 topic + 3 fanout
2. ✅ Topic routing với 7 routing key
3. ✅ **Fan-out 1 exchange → 5 queue** cho routing key `session`
4. ✅ **N routing key → 1 queue** (`events_accounting` nhận 4 key)
5. ✅ Manual ack + nack
6. ✅ Idempotent consumer bằng `ON CONFLICT DO NOTHING`
7. ✅ Auto-reconnect channel
8. ✅ Producer tách qua mpsc channel nội bộ (backpressure)
9. ✅ Topology khai báo tập trung ở admin lúc boot
10. ❌ Không DLQ/DLX, ❌ không prefetch/QoS, ❌ không publisher confirms, ❌ không backoff khi reconnect, ❌ không đảm bảo thứ tự

---

# PHẦN 4 — LUỒNG ACCOUNTING (tính tiền)

## 4.1 Accounting là SERVICE RIÊNG ✅

**Không nằm trong repo này.** Chạy ở `accounting_service_url: http://localhost:8092`.

Admin giao tiếp với nó theo **hai chiều, hai giao thức khác nhau** — đây là điểm kiến trúc đáng nói nhất:

```
┌──────────┐  RabbitMQ (async, ghi)  ┌────────────┐
│ admin +  │ ──────────────────────► │            │
│masternode│  queue events_accounting│ accounting │
└──────────┘                         │  :8092     │
     ▲                               │            │
     └───────────────────────────────┤ (+ DB riêng?)
         HTTP GET (sync, đọc)        └────────────┘
```

**Chiều GHI (bất đồng bộ, qua RabbitMQ):** queue `events_accounting` bind 4 routing key trên `dpn-events`:
`session` (SessionCreated/Terminated), `deposit`, `withdrawal`, `referral`.

**Chiều ĐỌC (đồng bộ, qua HTTP):** `admin/src/integration/accounting.rs`

| Method | Endpoint | Trả về |
|---|---|---|
| `get_connection_overview` | `GET /api/connection_overview/{user_addr}` | `ConnectionOverview` |
| `get_connection_history` | `GET /api/connection_history/{user_addr}` | `Vec<Session>` |
| `get_usage_history` | `GET /api/usage_history/{user_addr}` | `Vec<Session>` |
| `get_reward_overview` | `GET /api/rewards_overview/{user_addr}` | `RewardsOverview` |
| `get_referral_overview` | `GET /api/referrals_overview/{user_addr}` | `ReferralsOverview` |
| `get_withdrawal_history` | `GET /api/withdrawal_history/{user_addr}` | `Vec<WithdrawalHistoryDto>` |

> 🎤 **Đây chính là CQRS ở tầng service**: đường ghi đi qua message queue (bất đồng bộ, chịu tải cao, chấp nhận eventual consistency), đường đọc đi qua HTTP (đồng bộ, cần kết quả ngay). Nếu accounting có read replica thì nó nằm ở chiều đọc này. Cách nói này vừa trung thực (tôi không xác nhận được có replica) vừa cho thấy hiểu pattern.

**Chi tiết implementation đáng học** — HTTP client **tái sử dụng, khởi tạo một lần**:
```rust
#[dynamic]
pub static HTTP_CLIENT: reqwest::Client = {
    let mut headers = reqwest::header::HeaderMap::new();
    headers.insert("Content-Type", HeaderValue::from_static("application/json"));
    reqwest::Client::builder().default_headers(headers).build().unwrap()
};
```
(Ngược hẳn với lỗi ở Redis/RabbitMQ publisher — chỗ này làm đúng.)
Trong Go: `http.Client` với `Transport` cấu hình `MaxIdleConnsPerHost`, dùng chung toàn app, **không** tạo mới mỗi request.

⚠️ **Thiếu**: không có timeout trên `HTTP_CLIENT`, không có circuit breaker. Accounting treo → admin treo theo. 💡 `.timeout(Duration::from_secs(5))` + circuit breaker.

## 4.2 Luồng tiền end-to-end (kể được là ăn điểm lớn)

```
1. Client kết nối proxy tới masternode clientmode
       │  proxy_auth: parse username → tra cache proxy_acc trong RAM
       ▼
2. matcher.match_peer() chọn peer thoả: giá, quốc gia, thành phố, prioritized IP
       │  điều kiện giá:  client.rate_per_kb >= peer.rate_per_kb
       ▼
3. Tạo EphemeralSession → hash = sha3(protobuf{provider, client, identifier, handshaked_at})
       │  publish DPNEvent::SessionCreated  → dpn-events rk=session
       ▼
4. Trong suốt phiên: đếm byte upload/download ở tầng copy (util/copy.rs, field `amt`)
       │  đổi byte → KB, .ceil()  → publish PeerStats → dpn-stats (fanout)
       ▼
5. Phiên kết thúc (5 lý do: ClientInactive / PeerDisconnected / SystemShutdown /
   ClientLowBalance / RotatedIP)
       │  publish DPNEvent::SessionTerminated → dpn-events rk=session
       ▼
6. FAN-OUT 5 queue: accounting / admin / explorer / websocket / notification
       │
       ├─► admin: ghi bảng sessions (session_dal)
       └─► accounting: tính duration_fee + bandwidth_fee → total_fee
                       ghi internal_transactions (tx_hash làm PK, idempotent)
       ▼
7. User bấm "rút tiền" → admin publish DPNEvent::Withdrawal (rk=withdrawal)
       │  → events_accounting; accounting tạo bản ghi rút
       │  → dpn-withdrawals (fanout) → txs_onchain
       ▼
8. Worker on-chain ký & gửi giao dịch lên blockchain (ethers, lib/contracts/dpn.rs)
       │  → dpn-balances (fanout) → cập nhật số dư
       ▼
9. User mở app → admin gọi HTTP GET /api/rewards_overview/{addr} sang accounting
```

**Công thức tính phí** (suy ra từ schema `sessions`): `total_fee = duration_fee + bandwidth_fee`, với `duration_fee = duration × rate_per_second`, `bandwidth_fee = bandwidth_usage(KB) × rate_per_kb`.

**Hai loại transaction tách bạch:**
- `internal_transactions` — ghi sổ nội bộ, chưa lên chain, PK `tx_hash`, có `tx_type` (Network/Referral/Commission/Task) và `tx_status`
- `transactions` — giao dịch on-chain thật, có `attempts`, `no_retry` (retry logic)

**Số tiền "có thể rút" tính bằng CTE** (`user_dal.rs:245`) — logic hay:
```sql
WITH LastSuccessfulWithdrawalTransaction AS (
    SELECT COALESCE(MAX(created_at), 0) AS last_successful_withdrawal_timestamp
    FROM transactions
    WHERE from_addr = $1 AND tx_type = $2 AND tx_status = $3
)
SELECT SUM(amount) as rewards_amount
FROM internal_transactions
WHERE to_addr = $1
  AND created_at >= (SELECT last_successful_withdrawal_timestamp FROM ...);
```
→ "Tổng reward kiếm được **kể từ lần rút thành công gần nhất**". Tránh phải maintain một cột `claimed` mutable — chỉ cộng dồn từ một mốc thời gian. **Immutable ledger pattern**.

> 🎤 "Chỗ này tôi thích: thay vì có cột `is_claimed` phải UPDATE (dễ race, dễ sai), họ tính bằng cách lấy mốc thời gian rút gần nhất rồi cộng mọi khoản thu sau mốc đó. Sổ cái chỉ append, không sửa — giống double-entry bookkeeping. Đúng nguyên tắc: **với dữ liệu tài chính, đừng UPDATE, hãy INSERT**."

💡 Query này chính là ứng viên số 1 cho covering index `(to_addr, created_at) INCLUDE (amount)`.

---

# PHẦN 5 — CÁC PHẦN HAY KHÁC TRONG SOURCE (không phải CRUD)

## 5.1 Thuật toán ghép peer — `matcher.rs::match_peer()` ⭐

Đây là phần "thuật toán" đáng kể nhất, 743 dòng.

**Cấu trúc dữ liệu:**
```rust
pub struct PeerPool {
    peer_queue: Arc<RwLock<PriorityQueue<u32, Reverse<u32>>>>,  // key=ip_u32, priority=Reverse(timestamp)
    peers:      Arc<DashMap<u32, Arc<SnapPeer>>>,               // lookup O(1) theo IP
    clients:    Arc<DashMap<Vec<u8>, Arc<Client>>>,
    cached_peer_price: RwLock<HashMap<String, UserBandwidthPrice>>,
    epoch_info: Arc<RwLock<EpochInfo>>,
}
```

**`Reverse(timestamp)` = least-recently-used đứng đầu.** Peer nào lâu chưa được dùng nhất sẽ được ưu tiên → **cân bằng tải tự nhiên**, không cần round-robin counter.

**Thuật toán chọn peer (duyệt queue theo thứ tự LRU, lấy peer đầu tiên thoả hết):**
```rust
for (ip_u32, _) in peer_queue.into_sorted_iter() {
    if current_peer_ip_u32 == Some(peer.ip_u32) { continue; }   // 1. không chọn lại peer cũ
    if peer.max_client_reached() { continue; }                  // 2. giới hạn max_clients_per_peer
    if !(client_price.rate_per_kb >= peer_rate_per_kb) { continue; }  // 3. client trả đủ giá peer
    if client_country != 0 && peer_country != client_country { continue; }  // 4. khớp quốc gia
    if client_city != 0 && peer_city != client_city { continue; }           // 5. khớp thành phố
    if prioritized_ip mismatch && level == Strict { continue; }             // 6. IP ưu tiên
    // đạt hết → đổi priority thành now (đẩy xuống cuối LRU) rồi break
    peer_queue_wlk.change_priority(&ip_u32, Reverse(Utc::now().timestamp() as u32));
    matched_peer = Some((peer_price, peer)); break;
}
```

**Epoch** — cứ `epoch_duration` giây (config: 90s) thì **reset toàn bộ priority về `now`**:
```rust
async fn run_epoch(self: Arc<Self>) -> anyhow::Result<()> {
    loop {
        let mut peer_queue_wlk = self.peer_queue.write().await;
        let peer_queue = peer_queue_wlk.clone();
        peer_queue_wlk.clear();
        let now = Utc::now().timestamp() as u32;
        for (peer_elm, _) in peer_queue.into_iter() {
            peer_queue_wlk.push(peer_elm, Reverse(now));   // reset về cùng vạch xuất phát
        }
        ...
        sleep(Duration::from_secs(self.config.epoch_duration)).await;
    }
}
```
→ Chống **starvation**: peer mới join không bị peer cũ chiếm chỗ mãi mãi.

**IP rotation** — cứ `ip_rotation_period` giây (mặc định 300s) thì đổi peer, để IP thoát của client thay đổi:
```rust
async fn new_peer_needed(...) -> Option<SessionTerminationReason> {
    if client_session.start_time.elapsed() > Duration::from_secs(ip_rotation_period) {
        Some(SessionTerminationReason::RotatedIP)
    } else if client_session.last_failed_streams.load(Ordering::SeqCst) > MAX_FAILED_STREAMS {
        Some(SessionTerminationReason::PeerDisconnected)   // circuit breaker thô sơ
    } else { None }
}
```

**Có đo và log hiệu năng ngay trong hot path:**
```rust
debug!("peer match stats identifier={} queue_len={} looked_up_index={}, elapsed_nanos={}",
       proxy_acc_data.id, peer_queue_len, i, instant.elapsed().as_nanos());
```

> 🎤 "Điểm hay của thiết kế này là dùng priority queue với priority là `Reverse(timestamp lần dùng cuối)` — tức là một **LRU queue**. Peer nào lâu chưa dùng nhất tự động lên đầu. Chọn xong thì đặt lại priority = now, đẩy xuống cuối. Cộng thêm cơ chế epoch reset toàn bộ priority mỗi 90 giây để peer mới không bị đói.
> Trong Go tôi sẽ dùng `container/heap` với `sync.RWMutex`, hoặc nếu cần lock-free thì shard theo hash IP. `DashMap` ở đây tương đương `sync.Map` hoặc sharded map."

**⚠️ Vấn đề complexity:** vòng lặp là **O(n) trên số peer** cho **mỗi lần match**, và mỗi peer lại có thể `await` để lấy giá. Với 10.000 peer và tỷ lệ khớp thấp thì rất chậm. 💡 Nên index sẵn peer theo `(country_geoname_id, city_geoname_id)` để chỉ duyệt tập ứng viên.

## 5.2 Parse username proxy — `proxy_auth.rs`

Format (từ README): `[username]_[COUNTRY_CODE]_REGION-[region]_PLATFORM-[platform]`

→ **Nhét tham số routing vào username** vì giao thức SOCKS5/HTTP proxy không có chỗ nào khác để truyền metadata. Đây là kỹ thuật chuẩn của ngành proxy.

**SOCKS5 handshake tự viết bằng tay** (đọc byte thô, đúng RFC 1928/1929):
```rust
pub async fn socks5_auth(stream: &mut TcpStream) -> Result<(String, String)> {
    let mut data: [u8; 2] = [0; 2];
    stream.read_exact(&mut data).await?;          // VER, NMETHODS
    let mut methods = vec![0; data[1] as usize];
    stream.read_exact(&mut methods).await?;
    if !methods.contains(&2) {                    // 2 = USERNAME/PASSWORD
        stream.write_all(&[5, 0xff]).await?;      // NO ACCEPTABLE METHODS
        return Err(anyhow!("client doesn't support password auth method"));
    }
    stream.write_all(&[5, 2]).await?;             // chọn method 2
    // đọc ULEN, UNAME, PLEN, PASSWD
    ...
    stream.write_all(&[1, 0]).await?;             // status = success
}
```

**HTTP proxy auth parse bằng `memchr`/`memmem` (SIMD)** thay vì parse HTTP đầy đủ:
```rust
pub fn http_auth(req: &[u8]) -> Result<((String, String), usize, usize)> {
    let needle = b"Proxy-Authorization: Basic ";
    let finder = Finder::new(needle);
    if let Some(pos) = finder.find(req) {
        let auth = &req[pos + 27..];
        let end_pos = memchr(b'\r', auth).ok_or(anyhow!("no proxy auth header"))?;
        let decoded_auth = String::from_utf8(GP_BASE64.decode(auth)?)?;
        let colon_index = decoded_auth.find(':')...;
        // trả về cả (pos, end_pos) để CẮT header này ra khỏi request trước khi forward
    }
}
```
Trả về vị trí byte để **strip header `Proxy-Authorization` khỏi request** trước khi chuyển tiếp — không rò rỉ credential ra server đích. Chi tiết bảo mật nhỏ nhưng quan trọng.

> 🎤 "Ở hot path này họ không parse HTTP đầy đủ mà chỉ tìm đúng chuỗi `Proxy-Authorization: Basic ` bằng `memmem` (SIMD-accelerated substring search), lấy vị trí, decode base64. Tiết kiệm được toàn bộ chi phí allocate của một HTTP parser. Trong Go tương đương với `bytes.Index` thay vì `http.ReadRequest`."

## 5.3 QUIC transport — `masternode/src/integration/quic.rs` + `peernode/lib/src/core/quic.rs`

Dùng **quinn** (QUIC over UDP), mTLS với rootCA tự ký (`create-cert.sh`).

```rust
pub const ALPN_QUIC_HTTP: &[&[u8]] = &[b"hq-29"];
pub trait AcceptBidirectionalStream { async fn accept_bidirectional_stream(&self) -> io::Result<BidirectionalStream>; }
pub trait OpenBidirectionalStream  { async fn open_bidirectional_stream(&self)  -> io::Result<BidirectionalStream>; }
pub struct BidirectionalStream { pub write: SendStream, pub read: RecvStream }
// impl AsyncRead + AsyncWrite cho BidirectionalStream → dùng như TcpStream
```

**Tại sao QUIC chứ không TCP:** một peer phục vụ nhiều client cùng lúc. Với TCP thì phải mở N connection hoặc tự làm multiplexing. QUIC có **stream multiplexing sẵn trong protocol** và **không bị head-of-line blocking** giữa các stream (khác HTTP/2 trên TCP). Peer ở nhà thường sau NAT, mạng chập chờn — QUIC có connection migration.

> 🎤 Rất đáng nói: "Peer chạy trên máy người dùng, sau NAT, mạng không ổn định. Chúng tôi dùng QUIC vì ba lý do: multiplexing nhiều stream trên một connection UDP (mỗi client một stream), không bị head-of-line blocking giữa các stream, và 0-RTT reconnect. Trong Go tương đương là `quic-go`."

## 5.4 Zero-copy proxy loop — `peernode/lib/src/util/copy.rs`

Tự implement `Future` để copy hai chiều, thay vì `tokio::io::copy_bidirectional`:

```rust
impl<S, D> Future for Copy<S, D> where S: AsyncBufRead, D: AsyncWrite {
    type Output = Result<usize, Error>;
    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output> {
        loop {
            let buffer = match this.reader.as_mut().poll_fill_buf(cx) { ... };  // mượn buffer, KHÔNG copy
            match ready!(this.writer.as_mut().poll_write(cx, buffer)) {
                Ok(n) => {
                    this.reader.as_mut().consume(n);
                    *this.amt += n;          // ⭐ ĐÂY LÀ CHỖ ĐẾM BYTE ĐỂ TÍNH TIỀN
                }
                ...
            }
        }
    }
}
```

Ba điểm hay:
1. `poll_fill_buf` + `consume` = **ghi thẳng từ buffer của reader**, không allocate buffer trung gian.
2. `flush_on_pending` — chỉ flush khi reader hết dữ liệu (`Poll::Pending`), tức là **gom nhiều lần đọc thành một lần ghi** → giảm syscall.
3. `amt` chính là **nguồn dữ liệu tính tiền** — con số này đi thẳng vào `PeerStats.download/upload`.

> 🎤 "Đây là chỗ nối giữa tầng network và tầng tính tiền: biến `amt` trong vòng copy chính là số byte được chuyển, sau đó đổi sang KB, làm tròn lên, publish qua RabbitMQ. Toàn bộ doanh thu của peer bắt nguồn từ con số này."

## 5.5 Unix Domain Socket — IPC giữa 2 mode

`masternode/src/peermode/unix_sock.rs` — peermode và clientmode chạy như **2 process riêng trên cùng máy**, giao tiếp qua Unix socket (`dpn.sock`) thay vì TCP localhost.

```rust
let _ = std::fs::remove_file(sock_path.clone());   // dọn socket file cũ (từ lần crash trước)
let listener = UnixListener::bind(sock_path.clone())?;
```
→ Unix socket nhanh hơn TCP loopback (không qua network stack, không checksum, không TCP state machine).

## 5.6 WebSocket theo dõi phiên online — `admin/src/webapi/online_session_ws.rs`

Server WebSocket riêng ở port `:8099` (tách khỏi REST `:8090`).

**Protocol tự định nghĩa bằng enum, serialize JSON:**
```rust
pub enum WsMessage {
    // Incoming
    Ping(()),
    StartOnlineSession(String),   // JWT token
    // Outgoing
    UnknownMessage(()), Authorized(()), Unauthorized(()), Pong(()),
    OnlineSessionStarted(()), StartOnlineSessionFailed(String),
}
```

**Auth qua message chứ không qua header** (browser WebSocket API không cho set header):
```rust
WsMessage::StartOnlineSession(token) => {
    if let Ok(user_claims) = get_user_claims(token).await {   // decode JWT HS256
        self.user_svc.start_online_session(user_claims.user_addr).await
        ...
    } else { /* Unauthorized */ }
}
```

**Cleanup khi disconnect — điểm quan trọng nhất:**
```rust
// khi vòng lặp rx.next() kết thúc = connection đóng
let mut online_users_wlk = self.online_users.write().await;
if let Some((user_addr, start_time)) = online_users_wlk.get(&addr.to_string()) {
    self.user_svc.stop_online_session(user_addr.to_string(), *start_time).await;  // → ghi DB
}
online_users_wlk.remove(&addr.to_string());
```
State giữ trong `Arc<RwLock<HashMap<String, (String, i64)>>>` — key là `SocketAddr`.

⚠️ **Vấn đề**: state ở trong RAM của **một** instance admin. Chạy 2 replica admin sau load balancer thì mỗi instance chỉ biết session của mình. 💡 Đưa state xuống Redis (đã có sẵn key `uptime_xp`, chỉ cần bỏ HashMap in-memory đi).
⚠️ Không có **heartbeat/timeout server-side**: client mất mạng đột ngột (không gửi FIN) thì connection treo mãi, phiên online không bao giờ đóng.

## 5.7 Auth — `admin/src/webapi/auth.rs`

| Endpoint | Cơ chế |
|---|---|
| `POST /auth/sign-up` | **Argon2** hash password với `SaltString::generate(&mut OsRng)` |
| `POST /auth/sign-in` | `Argon2::verify_password` → phát access + refresh token |
| `POST /auth/sso_sign_in` | Google/Apple SSO, verify qua **JWKS** (`jwks_client::keyset::KeyStore`) |
| `POST /auth/refresh_token` | Đổi refresh token lấy access token mới |
| `POST /auth/reset-password` | Hash lại |

```rust
pub struct UserClaims {
    pub user_addr: String,
    pub login_session_id: String,   // ⭐ dùng để đối chiếu với user_connection_history
    pub exp: u64,
}
```
JWT ký **HS256** (symmetric, secret trong config). SSO thì verify RS256 qua JWKS công khai của Google/Apple.

> 🎤 Điểm nên nói: "Password dùng **Argon2** chứ không phải bcrypt — Argon2id là khuyến nghị hiện tại của OWASP vì chống được cả GPU cracking lẫn side-channel. Điểm thứ hai: `login_session_id` được nhúng vào JWT claims và dùng làm một phần **primary key của bảng `user_connection_history`** — nhờ vậy truy vết được mọi kết nối thuộc về phiên đăng nhập nào."

⚠️ HS256 với secret nằm trong `config.yaml` commit vào repo (`jwt_secret_key: 5kjsIQ...`) — trong production phải lấy từ secret manager.

## 5.8 Bootnode — service discovery cho masternode

`admin/src/bootnode/mod.rs`

```rust
pub trait BootnodeService {
    async fn register_masternode(...);
    async fn deregister_masternode(...);
    async fn assign_masternode(self: Arc<Self>, continent_code: String) -> Option<MasternodeInfo>;
    async fn online_masternodes(self: Arc<Self>) -> Vec<(String, MasternodeInfo)>;
    async fn online_peers(self: Arc<Self>) -> Result<Vec<PeernodeInfo>>;
    async fn masternode_metrics(self: Arc<Self>) -> Result<Vec<MasternodeMetric>>;
}
pub struct BootnodeServiceImpl {
    pub registered_masternodes: Arc<RwLock<HashMap<String, (String, MasternodeInfo)>>>,
    ...
}
```

- `POST /masternode/register_masternode` — masternode tự đăng ký khi boot (bảo vệ bằng header `x-api-key`)
- `assign_masternode(continent_code)` — **geo-routing**: client ở châu Á thì trả về masternode châu Á → giảm latency
- Registry nằm **in-memory** trong admin

⚠️ In-memory → admin restart là mất hết registry (phải chờ masternode re-register), và không chạy được nhiều replica admin. 💡 Nên đưa xuống Redis với TTL + heartbeat (chính là cái Consul/etcd làm).

Admin còn có **health check ngược**: `admin_health_check` trong `peermode/mod.rs` — masternode chủ động ping admin định kỳ.

## 5.9 Hash session bằng protobuf + SHA3

`core/src/types/bandwidth.rs`:

```rust
impl EphemeralSession {
    pub fn new(client_identifier, client_addr, peer_addr, rate_per_kb, rate_per_second, login_session_id) -> Self {
        let handshaked_at_micros = Utc::now().timestamp_micros();   // MICRO giây
        let mut _self = Self { hash: "".to_string(), ..., handshaked_at: handshaked_at_micros, ... };
        let proto: ProtoSession = _self.clone().into();
        let bz = ::prost::Message::encode_to_vec(&proto);
        _self.hash = bytes_to_hex_string(hash(bz).as_bytes());       // SHA3

        // TODO(rameight): we use microsecs to avoid hash collision
        // now we convert microsecs to secs back
        _self.handshaked_at /= 1_000_000;
        _self
    }
}
```

**Deterministic ID**: `session_hash = SHA3(protobuf{provider_addr, client_addr, client_identifier, handshaked_at})`.

Ba lý do dùng cách này thay vì UUID/auto-increment:
1. **Idempotency key tự nhiên** — nhận lại cùng event thì hash y hệt → `ON CONFLICT DO NOTHING` chặn duplicate.
2. **Sinh được ở client, không cần round-trip DB** để lấy ID.
3. Dùng protobuf (không phải JSON) để serialize vì protobuf **có thứ tự field xác định** → hash ổn định. JSON không đảm bảo thứ tự key.

Comment `// TODO(rameight): we use microsecs to avoid hash collision` cho thấy họ đã gặp **hash collision thật** khi dùng giây, phải nâng lên micro giây rồi chia ngược lại.

> 🎤 Rất đáng kể: "ID của session là hash SHA3 của nội dung, serialize bằng protobuf chứ không phải JSON — vì protobuf có thứ tự field xác định nên hash ổn định giữa các lần chạy và giữa các ngôn ngữ. Điều này biến session_hash thành **idempotency key tự nhiên**: dù RabbitMQ redeliver bao nhiêu lần thì INSERT ... ON CONFLICT DO NOTHING vẫn chỉ tạo một row. Và họ phải dùng micro giây thay vì giây vì đã gặp collision thật."

## 5.10 Danh sách API endpoint đầy đủ

### Admin `:8090` (actix-web + Swagger UI qua `utoipa`)

| Scope | Endpoint | Ghi chú |
|---|---|---|
| `/auth` | `POST /sign-up`, `/sign-in`, `/reset-password`, `/sso_sign_in`, `/refresh_token` | Argon2 + JWT + JWKS |
| `/users` | `GET /detail`, `/tier_points`, `POST /referral/create`, `GET /referral/link`, `/referral/history`, `/referral/overview`, `POST /rewards/claim`, `GET /rewards/claim_history` | referral/rewards proxy sang accounting |
| `/clients` | `GET/POST/PUT/DELETE /proxies`, `GET /overview`, `/usage-history` | CRUD proxy account → **ghi Redis + publish** |
| `/connections` | `GET /overview`, `/connection_history`, `/detail/{session_id}`, `/suggested_bandwidth_price`, `GET/PUT /bandwidth-price`, `POST /assign_masternode` | `PUT /bandwidth-price` → **publish `price_updated`** |
| `/locations` | `GET /continents`, `/countries`, `/continents/{geoname_id}/countries` | GeoLite2 |
| `/masternode` | `POST /register_masternode`, `/deregister_masternode` | bảo vệ bằng `x-api-key` |
| `/metrics` | `GET /health`, `/online_peers`, `/online_masternodes`, `/masternode-metrics` | |
| `/monitoring` | Prometheus scrape | |
| WS `:8099` | `Ping`, `StartOnlineSession(token)` | |

### Masternode clientmode `:9093` / peermode `:9092`
`/health`, `/metrics`, `/admin` (xem peer queue status, danh sách client/peer đang kết nối), `/swagger`.

## 5.11 Monitoring
- Prometheus metrics: `incoming_requests` (Counter), `connected_clients` (Gauge), `response_code` (CounterVec theo statuscode+env), `response_time` (HistogramVec)
- Middleware actix-web tự động ghi metrics mỗi request
- Stack: Prometheus + Grafana + Alertmanager + cAdvisor (docker-compose)

⚠️ `monitoring/collector.rs` đang sinh **số ngẫu nhiên** (`rng.gen_range(0.001, 10.0)`) mỗi 10ms — code demo còn sót lại, không phải metrics thật. Đang bơm rác vào Prometheus.

---

# PHẦN 6 — DANH SÁCH BUG & ĐIỂM CẢI THIỆN (để kể "tôi tìm ra gì")

| # | Vấn đề | File | Mức độ |
|---|---|---|---|
| 1 | `RedisService` mở TCP connection mới **mỗi operation** | `core/src/services/redis.rs` (mọi hàm) | 🔴 Cao |
| 2 | RabbitMQ publisher tạo AMQP connection mới **mỗi message** | `admin/src/events_queue/publisher.rs` | 🔴 Cao |
| 3 | `nack(requeue=false)` + không có DLQ → **mất message tài chính** | `admin/src/events_queue/consumer.rs` | 🔴 Cao |
| 4 | Không `basic_qos`/prefetch + `tokio::spawn` mỗi message → cạn pool DB, OOM | consumer.rs | 🔴 Cao |
| 5 | `connection_svc` được tạo **2 lần**, consumer giữ instance cũ | `admin/src/main.rs:38` và `:54` | 🟠 Trung bình |
| 6 | `transaction.commit()` trong nhánh **lỗi** (comment ghi "rollback") | `connection_history_dal.rs:80` | 🟠 Trung bình |
| 7 | Queue `connection-events_explorer` bind nhầm routing key `deposit` | `events_queue/mod.rs` | 🟠 Trung bình |
| 8 | `dpn-txs` là topic nhưng bind rk `""`; `TXS_ADMIN_QUEUE` không bao giờ bind | `events_queue/mod.rs` | 🟠 Trung bình |
| 9 | Reconnect loop RabbitMQ **không có backoff** → spin 100% CPU | msg_queue.rs, publisher.rs | 🟠 Trung bình |
| 10 | Race: `HGETALL` rồi mới `SUBSCRIBE` → mất message trong khe giữa | matcher.rs, client_listener.rs | 🟠 Trung bình |
| 11 | Redis **không có TTL** ở bất kỳ key nào | toàn bộ | 🟡 Thấp |
| 12 | `update_uptime_xp` bị comment hết ruột, chỉ `Ok(())` | `admin/src/user/mod.rs` | 🟡 Thấp |
| 13 | Index `idx_user_connection_history_user_addr` thừa | migration | 🟡 Thấp |
| 14 | `users.deposit_addr` có 3 unique index chồng nhau | migration | 🟡 Thấp |
| 15 | `monitoring/collector.rs` bơm số random vào Prometheus | collector.rs | 🟡 Thấp |
| 16 | WebSocket state in-memory → không scale ngang được | online_session_ws.rs | 🟠 Trung bình |
| 17 | Không timeout / circuit breaker khi gọi accounting service | `integration/accounting.rs` | 🟠 Trung bình |
| 18 | Bootnode registry in-memory → mất khi restart | `bootnode/mod.rs` | 🟠 Trung bình |
| 19 | `match_peer` O(n) trên số peer mỗi lần match | matcher.rs | 🟡 Thấp |
| 20 | Không có publisher confirms | mọi producer | 🟠 Trung bình |

---

# PHẦN 7 — BỘ CÂU HỎI PHỎNG VẤN + TRẢ LỜI MẪU

### Database

**Q: Compound index là gì, thứ tự cột quan trọng thế nào?**
> Index nhiều cột, B-tree sắp xếp theo cột đầu trước rồi mới tới cột sau. Thứ tự quyết định index dùng được cho query nào — **leftmost prefix rule**: index `(A,B,C)` dùng được cho `WHERE A`, `WHERE A,B`, `WHERE A,B,C`, nhưng **không** dùng được cho `WHERE B` hay `WHERE C`. Quy tắc chọn thứ tự là **ESR: Equality trước, Sort giữa, Range sau**.
> Ví dụ trong hệ thống tôi làm: bảng `user_connection_history` có index `(user_addr, time_start)` phục vụ query tìm bản ghi mới nhất của user — `WHERE user_addr=$1 AND time_start=(SELECT MAX(time_start) WHERE user_addr=$1)`. Nhờ compound index, Postgres seek tới user rồi backward scan lấy đúng 1 entry thay vì đọc hết rồi sort.

**Q: Làm sao biết index có được dùng không?**
> `EXPLAIN (ANALYZE, BUFFERS)`. Nhìn 4 thứ: có `Seq Scan` trên bảng lớn không; có node `Sort` không (nghĩa là index chưa phục vụ ORDER BY); `rows` ước tính có lệch nhiều so với `actual rows` không (thống kê cũ, cần ANALYZE); và là `Index Scan` hay `Index Only Scan` (cái sau không đụng heap).

**Q: Covering index / partial index?**
> Covering index là index chứa đủ mọi cột query cần, kể cả cột chỉ để đọc — dùng `INCLUDE`. Kết quả là **Index Only Scan**, không phải đọc heap. Ví dụ `(to_addr, tx_type, tx_status) INCLUDE (amount)` cho query `SELECT SUM(amount) WHERE to_addr=... AND tx_type=... AND tx_status=...`.
> Partial index chỉ index một phần dữ liệu: `CREATE INDEX ... WHERE status = 'active'`. Rất hiệu quả khi tỷ lệ thoả điều kiện nhỏ — session active chỉ chiếm <1% bảng thì index nhỏ hơn 100 lần.

**Q: Materialized view khác view thường thế nào?**
> View thường chỉ là query được lưu tên — mỗi lần SELECT là chạy lại query gốc, không tiết kiệm gì. Materialized view **lưu kết quả thật xuống đĩa** như một bảng, đọc rất nhanh, đánh index được, nhưng **không tự cập nhật** — phải `REFRESH`.
> Điểm quan trọng: `REFRESH MATERIALIZED VIEW` lấy `ACCESS EXCLUSIVE LOCK`, chặn mọi SELECT trong lúc refresh. Dùng `REFRESH ... CONCURRENTLY` để không khoá, nhưng bắt buộc phải có **UNIQUE index** trên matview.
> Trade-off là đánh đổi độ tươi lấy tốc độ — hợp cho dashboard/report, không hợp cho số dư tài khoản.

**Q: Read replica giải quyết gì, có vấn đề gì?**
> Tách read khỏi write để scale đọc. Cơ chế là streaming replication qua WAL. Vấn đề lớn nhất là **replication lag** dẫn đến **read-your-own-write**: user vừa ghi, đọc lại từ replica thì chưa thấy. Xử lý bằng cách: sau khi ghi thì ép read của user đó về primary trong N giây, hoặc chờ replica đuổi kịp LSN, hoặc phân loại theo nghiệp vụ — dữ liệu tài chính đọc primary, dashboard/analytics đọc replica.
> Một bẫy nữa: bên trong transaction thì tuyệt đối không được nhảy sang replica.

### Redis

**Q: Redis dùng làm gì trong hệ thống của bạn?**
> Ba vai trò tách bạch. Một là **distributed cache** cho hot path — toàn bộ proxy account và bảng giá nằm trong Redis hash, masternode nạp vào RAM để xác thực request mà không đụng DB. Hai là **pub/sub để invalidate cache** — admin đổi giá thì publish, mọi masternode nhận và patch cache in-memory trong vài ms. Ba là **ephemeral state store** cho phiên online qua WebSocket — chỉ persist xuống Postgres một lần khi phiên kết thúc, thay vì UPDATE liên tục.

**Q: Cache invalidation làm thế nào?**
> Chúng tôi dùng **snapshot + delta**. Lúc boot, node `HGETALL` toàn bộ dataset từ Redis vào HashMap trong RAM. Sau đó `SUBSCRIBE` một channel để nhận thay đổi. Admin khi ghi thì làm hai việc: `HSET` cập nhật state (cho node boot sau) và `PUBLISH` event (cho node đang chạy).
> Điểm yếu tôi nhận ra: giữa `HGETALL` và `SUBSCRIBE` có một khe race — message publish trong khe đó bị mất vì pub/sub là fire-and-forget. Cách sửa là subscribe trước, buffer lại, rồi mới HGETALL và replay buffer; hoặc chuyển sang **Redis Streams** vì stream có lưu trữ và có offset.

**Q: Redis pub/sub không đảm bảo giao hàng, sao vẫn dùng?**
> Vì payload là **trạng thái cuối cùng**, không phải delta cộng dồn. Mất một message rồi nhận message sau thì vẫn hội tụ về đúng giá trị. Cộng thêm có snapshot trong Redis hash làm lưới an toàn, và mỗi lần service boot thì xoá sạch cache rồi nạp lại từ Postgres. Chúng tôi coi Redis là **soft state**, source of truth luôn là Postgres.
> Nếu payload là delta kiểu "cộng thêm 100" thì tuyệt đối không được dùng pub/sub — phải dùng RabbitMQ hoặc Redis Streams.

**Q: Cache stampede xử lý sao?**
> (Hệ thống này chưa xử lý.) Ba cách: **single-flight** — chỉ cho một request đi xuống DB, các request khác chờ kết quả đó (`golang.org/x/sync/singleflight` trong Go); **probabilistic early expiration** — refresh trước khi hết hạn với xác suất tăng dần; và **stale-while-revalidate** — trả giá trị cũ ngay, refresh nền.

### RabbitMQ

**Q: Exchange có mấy loại, khác nhau thế nào?**
> Direct (khớp routing key chính xác), Topic (khớp pattern với `*` và `#`), Fanout (bỏ qua routing key, gửi mọi queue đã bind), Headers (khớp theo header).
> Hệ thống tôi dùng topic cho event nghiệp vụ vì có nhiều loại event và consumer cần chọn lọc, dùng fanout cho stream số liệu băng thông vì mọi consumer đều cần hết.

**Q: Một message vào nhiều queue được không?**
> Được, và đó là điểm mấu chốt của publish/subscribe. Trong hệ thống tôi, event `SessionTerminated` publish một lần vào topic exchange với routing key `session`, và năm queue cùng bind key đó — accounting, admin, explorer, websocket, notification. RabbitMQ nhân bản ra năm queue, mỗi consumer xử lý và ack độc lập.
> Giá trị lớn nhất là **producer không biết gì về consumer**. Thêm service mới chỉ cần bind thêm queue, không sửa dòng nào ở producer.

**Q: At-least-once, exactly-once?**
> RabbitMQ với manual ack cho **at-least-once**: message chỉ bị xoá khi consumer ack, consumer crash thì redeliver. Exactly-once về mặt lý thuyết là **không thể** trong hệ phân tán, nên cách làm thực tế là at-least-once + consumer idempotent.
> Chúng tôi làm idempotent bằng natural key có tính xác định: `session_hash = SHA3(protobuf của session)`, rồi `INSERT ... ON CONFLICT (tx_hash) DO NOTHING`. Nhận lại cùng message thì kết quả không đổi.

**Q: Message xử lý lỗi thì sao?**
> Đây chính là điểm tôi thấy hệ thống làm chưa đúng: consumer `nack` với `requeue=false` mà không khai báo dead-letter exchange, nên message lỗi **biến mất vĩnh viễn**. Với event tài chính thì đó là mất tiền.
> Cách chuẩn: khai báo DLX qua argument `x-dead-letter-exchange` khi declare queue, nack với requeue=false thì message rơi vào DLQ; thêm retry queue với TTL tăng dần để retry có backoff; và alert khi DLQ có message.

**Q: Prefetch count là gì?**
> Số message tối đa broker đẩy cho một consumer mà chưa được ack. Là cơ chế **backpressure**. Không đặt thì mặc định không giới hạn — broker đẩy hết queue vào consumer, dễ OOM.
> Rule of thumb: prefetch ≈ số worker song song, và phải nhỏ hơn kích thước connection pool DB. Hệ thống này chưa đặt `basic_qos` — đó là một trong những thứ tôi sẽ sửa đầu tiên.

**Q: Connection vs Channel?**
> Connection là TCP thật, tốn kém, dùng lâu dài, chia sẻ toàn app. Channel là kênh logic bên trong connection, nhẹ, mỗi goroutine/thread một channel riêng — **không được share channel giữa các thread**.
> Trong repo này masternode làm đúng (giữ channel dài hạn) nhưng admin publisher lại tạo connection mới mỗi lần publish — đó là bug hiệu năng nghiêm trọng.

### Kiến trúc

**Q: Khi nào dùng message queue, khi nào gọi HTTP trực tiếp?**
> Message queue khi: cần fan-out nhiều consumer, chấp nhận eventual consistency, cần buffer khi consumer chậm/chết, hoặc muốn tách coupling giữa producer và consumer. HTTP khi cần kết quả ngay và cần biết thành công/thất bại đồng bộ.
> Hệ thống tôi dùng cả hai với chính một service: đường ghi vào accounting đi qua RabbitMQ (bất đồng bộ, tải cao, không cần biết kết quả ngay), đường đọc dữ liệu tổng hợp đi qua HTTP (cần kết quả để render UI). Đó là **CQRS ở tầng service**.

**Q: Idempotency trong hệ thống phân tán?**
> Nguyên tắc: mọi thao tác ghi qua message queue phải idempotent vì at-least-once là mặc định. Cách làm là dùng **idempotency key có tính xác định** — trong hệ thống tôi là hash SHA3 của nội dung session, serialize bằng protobuf (không dùng JSON vì JSON không đảm bảo thứ tự field). Rồi dựa vào unique constraint của DB: `ON CONFLICT DO NOTHING`.
> Với dữ liệu tài chính còn một nguyên tắc nữa: **đừng UPDATE, hãy INSERT**. Sổ cái chỉ append. Chúng tôi tính "reward khả dụng" bằng cách lấy mốc thời gian rút gần nhất rồi SUM mọi khoản thu sau mốc đó — không có cột mutable nào để race.

---

# PHẦN 8 — MAPPING SANG GOLANG (vì phỏng vấn Go)

| Rust ở đây | Go tương đương |
|---|---|
| `Arc<T>` | con trỏ + GC |
| `Arc<RwLock<HashMap>>` | `sync.RWMutex` + `map`, hoặc `sync.Map` |
| `DashMap` | sharded map, hoặc `sync.Map` |
| `tokio::spawn` | `go func()` |
| `tokio::select!` | `select { case <-ch1: ... }` |
| `mpsc::channel(1024)` | `make(chan T, 1024)` |
| `tokio::sync::mpsc` shutdown | `context.WithCancel` |
| `anyhow::Result<T>` | `(T, error)` |
| `#[async_trait] trait` | `interface` |
| `mockall::automock` | `gomock` / `mockery` |
| `sqlx::query!` (compile-time check) | `sqlc` (gần nhất), hoặc `pgx` |
| `PgPool` | `*pgxpool.Pool` / `*sql.DB` |
| `lapin` | `amqp091-go` |
| `redis-rs` + `redis-async` | `go-redis/v9` |
| `quinn` (QUIC) | `quic-go` |
| `actix-web` | `gin` / `echo` / `chi` |
| `utoipa` (OpenAPI) | `swaggo/swag` |
| `prometheus` crate | `prometheus/client_golang` |
| `tokio_tungstenite` | `gorilla/websocket` / `nhooyr.io/websocket` |
| `serde` | `encoding/json` + struct tag |
| `prost` (protobuf) | `google.golang.org/protobuf` |
| `PriorityQueue` | `container/heap` |
| `ConnectionHolder` enum | `interface { QueryContext; ExecContext }` (nhận cả `*sql.DB` và `*sql.Tx`) |

**Đoạn code Go nên chuẩn bị sẵn để viết trên bảng:**

```go
// 1. Consumer RabbitMQ với prefetch + ack/nack + DLQ
func consume(ctx context.Context, ch *amqp.Channel, queue string, handle func([]byte) error) error {
    if err := ch.Qos(50, 0, false); err != nil { return err }   // ⭐ prefetch — đừng quên
    msgs, err := ch.Consume(queue, "", false /* autoAck=false */, false, false, false, nil)
    if err != nil { return err }

    sem := make(chan struct{}, 20)   // giới hạn concurrency
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case d, ok := <-msgs:
            if !ok { return errors.New("channel closed") }
            sem <- struct{}{}
            go func(d amqp.Delivery) {
                defer func() { <-sem }()
                if err := handle(d.Body); err != nil {
                    d.Nack(false, false)   // requeue=false → rơi vào DLQ (đã cấu hình x-dead-letter-exchange)
                    return
                }
                d.Ack(false)
            }(d)
        }
    }
}

// 2. Repository chạy được cả trong lẫn ngoài transaction
type Querier interface {
    QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
    ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}
func (r *Repo) GetSession(ctx context.Context, q Querier, hash string) (*Session, error) { ... }

// 3. Cache-aside với singleflight chống stampede
var g singleflight.Group
func (s *Svc) GetUptime(ctx context.Context, addr string) (float64, error) {
    if v, err := s.rdb.HGet(ctx, "uptime_xp_total", addr).Float64(); err == nil {
        return v, nil
    }
    v, err, _ := g.Do(addr, func() (any, error) {          // chỉ 1 request xuống DB
        x, err := s.db.FindOrCreateXP(ctx, addr)
        if err != nil { return nil, err }
        s.rdb.HSet(ctx, "uptime_xp_total", addr, x)
        s.rdb.Expire(ctx, "uptime_xp_total", time.Hour)     // ⭐ TTL — repo gốc thiếu
        return x, nil
    })
    if err != nil { return 0, err }
    return v.(float64), nil
}

// 4. Redis Pub/Sub — snapshot + delta (sửa được race bằng cách subscribe TRƯỚC)
func (s *Svc) loadAndWatch(ctx context.Context) error {
    pubsub := s.rdb.Subscribe(ctx, "proxy_acc_updated")   // ⭐ subscribe TRƯỚC
    defer pubsub.Close()
    ch := pubsub.Channel()

    all, err := s.rdb.HGetAll(ctx, "proxy_acc").Result()  // rồi mới snapshot
    if err != nil { return err }
    s.cache.ReplaceAll(all)

    for msg := range ch {                                 // delta (đã buffer sẵn trong channel)
        var changed ProxyAccChanged
        if err := json.Unmarshal([]byte(msg.Payload), &changed); err != nil { continue }
        s.cache.Apply(changed)
    }
    return nil
}
```

---

# PHẦN 9 — CHECKLIST ÔN NHANH TRƯỚC PHỎNG VẤN

**Postgres**
- [ ] Leftmost prefix rule + quy tắc ESR
- [ ] Compound vs covering (`INCLUDE`) vs partial index
- [ ] Đọc `EXPLAIN ANALYZE`: Seq Scan / Sort / Index Only Scan / rows lệch
- [ ] Materialized view: không tự refresh, `REFRESH` khoá đọc, `CONCURRENTLY` cần UNIQUE index
- [ ] Read replica: WAL streaming, sync vs async, replication lag, read-your-own-write
- [ ] `FOR UPDATE SKIP LOCKED` cho queue-in-DB
- [ ] Isolation level: Read Committed (mặc định PG) vs Repeatable Read vs Serializable
- [ ] Connection pool: tại sao cần, pool size ≈ `(core × 2) + disk spindle`

**Redis**
- [ ] Kiểu dữ liệu: String, Hash, List, Set, ZSet, Stream
- [ ] Cache-aside vs write-through vs write-behind
- [ ] Pub/Sub vs Stream vs List — khi nào dùng cái nào
- [ ] Cache stampede / avalanche / penetration và cách chống
- [ ] TTL, eviction policy (`allkeys-lru`, `volatile-ttl`...)
- [ ] Persistence: RDB vs AOF
- [ ] Distributed lock: `SET NX PX` + Redlock và tại sao Redlock gây tranh cãi

**RabbitMQ**
- [ ] 4 loại exchange, binding, routing key pattern (`*` = 1 word, `#` = 0+ word)
- [ ] Connection vs Channel
- [ ] Manual ack, nack, reject; prefetch (`basic_qos`)
- [ ] DLX/DLQ, retry với TTL tăng dần
- [ ] Publisher confirms, mandatory flag, `basic.return`
- [ ] At-least-once + idempotent consumer
- [ ] Quorum queue vs classic queue (HA)

**Kiến trúc**
- [ ] CQRS, event-driven, eventual consistency
- [ ] Transactional outbox (giải quyết dual-write)
- [ ] Idempotency key
- [ ] Circuit breaker, timeout, retry với backoff + jitter
- [ ] Backpressure

---

## Phụ lục — mấy chi tiết nhỏ vui vui

- `Cargo.toml` của admin và masternode ghi `authors = ["dái bò"]` 🙂
- Masternode dùng **nightly Rust** (`#![feature(ip_bits)]`, `more_qualified_paths`, `io_error_more`)
- Hằng số `EVENTS_ACCOUNTNG_QUEUE` — typo, thiếu chữ `I`
- README của `subnet-dpn-admin` bị copy nguyên từ masternode (nói về proxy/username format, không liên quan admin)
- `dpn_core` trong repo là bản **cũ hơn** bản admin/masternode thực dùng (chúng lấy từ `git = "https://github.com/unicornultralabs/subnet-dpn-core"`) — nên `NOTIFICATION_EXCHANGE`, `DPNRedisKey::get_uptime_xp_kf` có trong code admin nhưng không có trong `subnet-dpn-core/` ở đây
- `lib/contracts/src/dpn.rs` — 1720 dòng binding smart contract (ethers-rs), nhưng **đang bị comment out** trong `Cargo.toml`
