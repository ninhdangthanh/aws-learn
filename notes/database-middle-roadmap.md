# DATABASE ROADMAP CHO BACKEND 3 YOE
## Từ nền tảng Indexing/Transaction lên mức Middle Backend

Bạn đang có nền tảng tốt nếu đã nắm **Indexing** và **Transaction**. Để lên mức Middle, trọng tâm không phải học thật nhiều keyword rời rạc, mà là biết thiết kế schema hợp lý, đọc được query plan, hiểu lock/concurrency, biết khi nào dùng partition/replication/cache, và có khả năng chẩn đoán bottleneck trong production.

---

## 1. Thứ tự học đề xuất

```text
Indexing
-> Transaction
-> Query Execution Plan
-> Schema Design & Constraints
-> Locking, MVCC, Isolation Level
-> Pagination Strategy
-> Partitioning
-> Replication & Read Replica
-> Connection Pooling
-> Caching Strategy
-> Migration, Backup, Restore
-> Observability & Performance Tuning
-> Sharding
```

**Ưu tiên cho Middle Backend:** Query plan, schema design, locking/MVCC, partitioning, replication, connection pooling, caching và migration an toàn.

**Có thể học ở mức concept trước:** Sharding, distributed transaction, multi-region database. Đây là phần nghiêng về Senior/System Design hơn.

---

## 2. Indexing - Nền tảng bạn đã có, nhưng nên đào sâu thêm

Index không chỉ là "tạo index cho cột hay query". Middle level cần hiểu index ảnh hưởng đến cả read và write.

*   **B-tree index:** Phù hợp cho so sánh bằng (`=`), range query (`>`, `<`, `BETWEEN`), sort theo index, prefix matching.
*   **Composite index:** Index nhiều cột, ví dụ `(tenant_id, created_at, status)`. Cần nhớ quy tắc tiền tố trái (`leftmost prefix`): query dùng tốt nhất khi filter bắt đầu từ các cột bên trái của index.
*   **Covering index:** Query có thể lấy đủ dữ liệu từ index mà không cần quay lại bảng chính, giảm I/O đáng kể.
*   **Over-indexing:** Mỗi index làm tăng chi phí ghi vì `INSERT`, `UPDATE`, `DELETE` phải cập nhật thêm cây index. Với bảng write-heavy, index quá nhiều có thể làm DB chậm hơn.
*   **Index selectivity:** Index hiệu quả khi giá trị có độ phân biệt cao. Cột boolean như `is_active` thường không đáng index riêng nếu phân bố dữ liệu lệch mạnh.

**Câu hỏi tự kiểm tra:**
*   Query này có dùng index không?
*   Index đang phục vụ filter, sort, join hay covering?
*   Index này có làm chậm write path không?
*   Có index nào trùng hoặc không còn được dùng không?

---

## 2.1. Compound index hoạt động thế nào (phần hay bị hỏi sâu)

### Cấu trúc: index là một danh sách đã sort theo bộ (tuple)

Với index `(A, B, C)`, database **không** tạo 3 index riêng. Nó tạo **một** cây B-tree, trong đó mỗi entry là bộ giá trị `(A, B, C)` được sort theo thứ tự: sort theo `A` trước, `A` bằng nhau thì sort theo `B`, `B` bằng nhau thì sort theo `C`.

Ví dụ index `(tenant_id, status, created_at)`:

```
(1, 'paid',    2024-01-05) -> row#12
(1, 'paid',    2024-03-20) -> row#88
(1, 'pending', 2024-01-02) -> row#40
(1, 'pending', 2024-02-11) -> row#7
(2, 'paid',    2024-01-09) -> row#51
(2, 'pending', 2024-01-01) -> row#33
```

Hình dung theo dạng cây phân cấp cho dễ nhớ. Với `CREATE INDEX idx_user ON users(country, city, age)`:

```
VN
 ├── HCM
 │    ├── 20
 │    └── 22
 └── HN
      └── 18
US
 └── NY
      └── 30
```

Muốn xuống tầng `city` thì bắt buộc phải đi qua tầng `country` trước. Muốn xuống `age` thì phải qua cả `country` và `city`. Đây chính là lý do vật lý của leftmost prefix rule — **không có đường tắt nhảy thẳng vào tầng giữa**.

Giống hệt danh bạ sort theo `(họ, tên, ngày sinh)`. Biết họ → nhảy tới đúng vùng. Biết họ + tên → thu hẹp tiếp. Nhưng **chỉ biết tên mà không biết họ thì phải đọc cả cuốn danh bạ**.

Đó chính là **leftmost prefix rule**: index chỉ seek được khi bạn cung cấp giá trị **liên tục từ cột trái nhất**.

### Điểm quan trọng nhất: thứ tự viết trong `WHERE` KHÔNG quan trọng

Đây là chỗ nhiều người hiểu sai. Với index `(A, B, C)`:

```sql
-- Hai query này HOÀN TOÀN GIỐNG NHAU với optimizer
WHERE A = 1 AND B = 2 AND C = 3
WHERE C = 3 AND B = 2 AND A = 1
```

Query planner tự sắp xếp lại điều kiện, nó không đọc `WHERE` theo thứ tự bạn gõ. Cái quan trọng là **TẬP cột nào xuất hiện trong `WHERE`**, không phải thứ tự chữ.

Nên khi được hỏi "index `ABC` mà query `ACB` thì sao?" → nếu ý là **thứ tự viết** `A, C, B` thì câu trả lời là: **giống hệt `ABC`, index dùng đầy đủ cả 3 cột**. Nhưng nếu ý là query chỉ có `A` và `C` (thiếu `B`) thì mới là vấn đề thật, xem bảng dưới.

### Bảng tra: index `(A, B, C)` phục vụ query nào

| Query filter | Index dùng được không | Giải thích |
|---|---|---|
| `A = ?` | ✅ Tốt | Prefix `(A)`, seek đúng vùng |
| `A = ? AND B = ?` | ✅ Rất tốt | Prefix `(A, B)` |
| `A = ? AND B = ? AND C = ?` | ✅ Tốt nhất | Dùng cả 3 cột để seek |
| `A = ? AND C = ?` | ⚠️ Một phần | Seek bằng `A`, còn `C` chỉ **lọc lại** trên các entry đã quét. Vẫn nhanh hơn seq scan nhưng quét nhiều entry thừa |
| `B = ?` | ❌ Thường không | Thiếu leftmost `A` → seq scan hoặc full index scan |
| `C = ?` | ❌ Thường không | Như trên |
| `B = ? AND C = ?` | ❌ Thường không | Vẫn thiếu `A` |
| `A = ? AND B > ? AND C = ?` | ⚠️ Dừng ở `B` | `B` là range → `C` không seek được nữa, chỉ filter |
| `A > ? AND B = ?` | ⚠️ Dừng ở `A` | Range ở cột đầu → `B` không seek được |

**Quy tắc gốc:** index seek theo các cột equality liên tiếp từ trái sang, **gặp cột range đầu tiên thì dừng lại** — các cột phía sau range chỉ còn dùng để filter, không giảm được phạm vi quét.

### Trường hợp `A = ? AND C = ?` chi tiết hơn

Đây là câu hỏi hay nhất trong nhóm này. Điều gì thực sự xảy ra:

1. Index seek tới vùng `A = ?` (thu hẹp tốt).
2. Trong vùng đó, các entry được sort theo `B` rồi mới tới `C` → giá trị `C` nằm rải rác, không liên tục.
3. Database quét **toàn bộ** entry có `A = ?` và test `C = ?` trên từng entry.

Chi phí phụ thuộc `A` lọc còn bao nhiêu row:

* `A = 'tenant_9'` còn 200 row → quét 200 entry, chấp nhận được.
* `A = 'tenant_9'` còn 2 triệu row → thảm họa, cần index `(A, C)` riêng.

PostgreSQL gọi phần này là `Index Cond` (dùng để seek) vs `Filter` (lọc sau). Đọc `EXPLAIN ANALYZE` thấy `Rows Removed by Filter` lớn là dấu hiệu index sai thứ tự cột:

```
Index Scan using idx_a_b_c on orders
  Index Cond: (a = 1)
  Filter: (c = 3)
  Rows Removed by Filter: 1998400   <-- đỏ, index không phù hợp query
```

**Ngoại lệ cần biết:** MySQL 8.0+ có **index skip scan** và PostgreSQL 18 cũng đã thêm skip scan cho B-tree. Khi cột bị bỏ qua có rất ít giá trị phân biệt (ví dụ `status` chỉ có 3 giá trị), optimizer có thể "nhảy" qua từng giá trị của cột đó rồi seek tiếp. Nhưng đây là tối ưu cơ hội, không nên thiết kế index dựa vào nó.

### Quy tắc sắp thứ tự cột: ESR (Equality → Sort → Range)

Đây là rule của MongoDB nhưng áp dụng đúng cho cả PostgreSQL/MySQL:

1. **E**quality: các cột filter bằng `=` đặt trước.
2. **S**ort: cột dùng cho `ORDER BY` đặt giữa.
3. **R**ange: các cột `>`, `<`, `BETWEEN`, `IN` range đặt cuối.

Ví dụ query:

```sql
SELECT * FROM orders
WHERE tenant_id = ? AND status = ? AND created_at > ?
ORDER BY created_at DESC
LIMIT 20;
```

Index đúng: `(tenant_id, status, created_at)`

* `tenant_id`, `status` equality → seek chính xác.
* `created_at` vừa phục vụ range vừa phục vụ `ORDER BY` → database đọc index theo chiều ngược và **dừng sau 20 row**, không cần sort.

Index sai: `(created_at, tenant_id, status)` → seek theo range trước, phải quét rộng rồi filter lại.

### Compound index và `ORDER BY`

Index `(A, B, C)` phục vụ được `ORDER BY` khi thứ tự sort khớp prefix:

| `ORDER BY` | Dùng index để sort? |
|---|---|
| `ORDER BY A` | ✅ |
| `ORDER BY A, B` | ✅ |
| `ORDER BY B` (không có `WHERE A = ?`) | ❌ |
| `WHERE A = 1 ORDER BY B` | ✅ — `A` cố định nên trong vùng đó dữ liệu đã sort sẵn theo `B` |
| `ORDER BY A ASC, B DESC` | ❌ trừ khi tạo index `(A ASC, B DESC)` |
| `ORDER BY A DESC, B DESC` | ✅ — đọc index ngược chiều |

Nếu `EXPLAIN` thấy node `Sort` kèm `external merge Disk`, tức là DB đang phải sort thủ công vì index không phục vụ được `ORDER BY`.

### Index thừa: `(A, B, C)` đã bao gồm `(A)` và `(A, B)`

Nếu đã có `(A, B, C)` thì **không cần** tạo thêm `(A)` hoặc `(A, B)` — chúng là prefix và được phục vụ sẵn. Tạo thêm chỉ tốn disk và làm chậm write.

Ngược lại `(A, B, C)` **không** thay thế được `(B, C)` hay `(C)`.

Query tìm index trùng lặp trong PostgreSQL:

```sql
SELECT indexrelid::regclass AS index_name,
       idx_scan AS times_used
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;
```

`idx_scan = 0` sau vài tuần chạy production → index chưa từng được dùng, cân nhắc drop.

### Cách thiết kế trong thực tế

1. Gom các query thật hay chạy nhất (không thiết kế index theo tưởng tượng).
2. Cột luôn xuất hiện trong mọi query đặt trái nhất — thường là `tenant_id`, `user_id`, `shop_id`.
3. Áp ESR cho phần còn lại.
4. Nếu query trả về ít cột, cân nhắc thêm covering: `INDEX (tenant_id, status) INCLUDE (total, currency)` để đạt `Index Only Scan`.
5. Chạy `EXPLAIN ANALYZE` xác minh, đừng tin cảm giác.
6. Tạo index trên bảng lớn ở production luôn dùng `CREATE INDEX CONCURRENTLY` (PostgreSQL) để không khóa write.

### Câu trả lời gọn khi phỏng vấn

> Compound index là một B-tree sort theo bộ giá trị các cột, sort theo cột trái trước rồi mới tới cột phải. Vì vậy nó chỉ seek hiệu quả khi query cung cấp prefix liên tục từ trái. Thứ tự viết điều kiện trong `WHERE` không quan trọng vì optimizer tự sắp lại — cái quan trọng là tập cột nào có mặt. Với index `(A, B, C)`: query `A`, `A+B`, `A+B+C` dùng tốt; query `A+C` chỉ seek được bằng `A` rồi filter `C` nên vẫn quét thừa; query chỉ có `B` hoặc `C` thì không dùng được. Ngoài ra index dừng seek ở cột range đầu tiên, nên tôi sắp cột theo ESR: equality trước, sort, rồi range. Tôi luôn xác minh bằng `EXPLAIN ANALYZE`, nhìn `Index Cond` vs `Filter` và `Rows Removed by Filter`.

---

## 3. Transaction - Nền tảng bạn đã có, nối tiếp sang consistency

Transaction bảo vệ dữ liệu bằng ACID:

*   **Atomicity:** Hoặc tất cả thành công, hoặc rollback toàn bộ.
*   **Consistency:** Dữ liệu sau transaction vẫn thỏa constraint và business rule.
*   **Isolation:** Các transaction đồng thời không nhìn thấy trạng thái trung gian sai lệch.
*   **Durability:** Commit xong thì dữ liệu không mất dù hệ thống lỗi.

Middle level cần đi xa hơn câu lệnh `BEGIN/COMMIT`:

*   Transaction càng dài thì giữ lock càng lâu, dễ gây nghẽn.
*   Không nên gọi external API khi đang mở transaction nếu có thể tránh.
*   Với thao tác có retry, cần thiết kế idempotency để tránh tạo dữ liệu trùng.
*   Trong hệ thống phân tán, transaction ACID xuyên nhiều service rất đắt. Thường dùng Saga, outbox pattern hoặc eventual consistency.

---

## 4. Query Execution Plan - Kỹ năng bắt buộc để lên Middle

Đây là phần nên học sớm sau Indexing. Nếu không đọc được execution plan, mình chỉ đang "đoán" query chậm vì lý do gì.

**Cần biết đọc `EXPLAIN` / `EXPLAIN ANALYZE`:**

*   Query dùng `Index Scan`, `Bitmap Index Scan`, hay `Seq Scan`.
*   DB ước lượng bao nhiêu rows và thực tế scan bao nhiêu rows.
*   Bottleneck nằm ở filter, join, sort, aggregate hay network transfer.
*   Query có sort trong memory hay spill ra disk.
*   Join đang dùng nested loop, hash join hay merge join.

**Dấu hiệu cần chú ý:**

*   `Seq Scan` trên bảng lớn trong request latency-sensitive.
*   Estimated rows lệch rất xa actual rows, có thể statistics cũ hoặc data distribution lệch.
*   Sort/aggregate trên tập dữ liệu lớn mà không có index hỗ trợ.
*   Query trả ít row nhưng scan rất nhiều row.

**Mục tiêu Middle:** Khi một API chậm, bạn có thể lấy query, chạy explain, chỉ ra vì sao chậm và đề xuất index/schema/query rewrite hợp lý.

---

## 5. Schema Design & Constraints

Schema design tốt giúp code đơn giản hơn và dữ liệu khó sai hơn.

*   **Normalization:** Tách dữ liệu theo quan hệ để tránh duplicate và update anomaly. Phù hợp cho core business data như user, order, payment.
*   **Denormalization:** Lưu dư một phần dữ liệu để tối ưu read, ví dụ lưu `order_total`, `customer_name_snapshot`. Phải có chiến lược cập nhật khi dữ liệu gốc thay đổi.
*   **Primary key:** Nên ổn định, không phụ thuộc dữ liệu có thể thay đổi.
*   **Foreign key:** Giữ toàn vẹn quan hệ, nhưng cần hiểu chi phí lock/cascade trên bảng lớn.
*   **Unique constraint:** Dùng DB bảo vệ uniqueness thay vì chỉ check ở application, vì check ở app có thể race condition.
*   **Check constraint:** Chặn dữ liệu sai ngay tại DB, ví dụ `amount >= 0`, `status IN (...)`.

**Góc nhìn Middle:** Đừng để application gánh toàn bộ data integrity. Những rule chắc chắn đúng ở mọi hoàn cảnh nên để database bảo vệ.

---

## 6. Locking, MVCC và Isolation Level

Đây là phần thường tách Middle khỏi Junior. Bạn không cần thuộc mọi chi tiết nội bộ database, nhưng phải hiểu vì sao request bị treo, deadlock, hoặc đọc ra dữ liệu "lạ".

### Locking

*   **Row lock:** Khóa một số dòng cụ thể khi update/delete/select for update.
*   **Table lock:** Khóa phạm vi lớn hơn, có thể xảy ra khi DDL, migration, hoặc một số thao tác đặc biệt.
*   **Deadlock:** Transaction A giữ lock X và chờ Y, transaction B giữ Y và chờ X. DB thường tự detect và kill một transaction.
*   **Lock wait:** Request không chết ngay mà chờ transaction khác nhả lock, làm latency tăng cao.

### Row lock, table lock và `SELECT ... FOR UPDATE`

Trong SQL database như PostgreSQL/MySQL, lock thường xuất hiện ở nhiều mức. Middle backend không cần thuộc toàn bộ lock matrix, nhưng cần hiểu lock nào làm request bị chờ và vì sao.

| Loại lock | Khóa cái gì? | Thường gặp khi nào? |
|---|---|---|
| Row lock | Một số dòng cụ thể | `UPDATE`, `DELETE`, `SELECT ... FOR UPDATE` |
| Table lock | Cả bảng hoặc metadata của bảng | DDL, migration, một số `ALTER TABLE`, operation lớn |
| Advisory lock | Lock logic do app tự định nghĩa | Cron/job chỉ cho một instance chạy |

`UPDATE` và `DELETE` thường tự lấy row lock trên các dòng bị tác động. Không có cú pháp `FOR DELETE`; khi bạn chạy `DELETE FROM ... WHERE ...`, DB tự khóa các row phù hợp để xóa.

`SELECT ... FOR UPDATE` dùng khi muốn đọc một row và khóa nó để transaction khác không sửa cùng lúc.

Ví dụ tránh oversell inventory:

```sql
BEGIN;

SELECT id, stock
FROM products
WHERE id = 10
FOR UPDATE;

UPDATE products
SET stock = stock - 1
WHERE id = 10 AND stock > 0;

COMMIT;
```

Trong lúc transaction này chưa commit, transaction khác muốn update cùng product row sẽ phải chờ hoặc fail tùy mode.

Các biến thể hay gặp:

| Cú pháp | Ý nghĩa | Use case |
|---|---|---|
| `FOR UPDATE` | Khóa row để update/delete | inventory, wallet, booking slot |
| `FOR NO KEY UPDATE` | Khóa nhẹ hơn `FOR UPDATE` trong PostgreSQL khi không đổi key | update field thường |
| `FOR SHARE` | Khóa để đọc ổn định, chặn update/delete nhất định | ít dùng hơn trong app backend |
| `FOR KEY SHARE` | Bảo vệ foreign key/reference | thường do DB dùng nội bộ |
| `NOWAIT` | Không chờ lock, fail ngay nếu row đang bị khóa | API cần phản hồi nhanh |
| `SKIP LOCKED` | Bỏ qua row đang bị khóa | job queue bằng DB |

Ví dụ worker lấy job bằng `SKIP LOCKED`:

```sql
BEGIN;

SELECT id
FROM jobs
WHERE status = 'pending'
ORDER BY created_at
LIMIT 10
FOR UPDATE SKIP LOCKED;

UPDATE jobs
SET status = 'processing'
WHERE id IN (...);

COMMIT;
```

Pattern này cho phép nhiều worker cùng lấy job mà không xử lý trùng một row.

Khi dùng lock cần chú ý:

* Transaction càng dài, lock giữ càng lâu.
* Không gọi external API trong lúc đang giữ lock nếu có thể tránh.
* Luôn có timeout để tránh request treo.
* Truy cập các bảng/row theo thứ tự nhất quán để giảm deadlock.
* Log/monitor lock wait và deadlock count.

### Optimistic vs. Pessimistic Locking

*   **Pessimistic locking:** Khóa trước khi sửa, ví dụ `SELECT ... FOR UPDATE`. Phù hợp khi xung đột cao, dữ liệu nhạy cảm như số dư, tồn kho.
*   **Optimistic locking:** Không khóa trước, dùng `version` hoặc `updated_at` để kiểm tra khi update. Phù hợp khi xung đột thấp, giúp throughput tốt hơn.

Ví dụ optimistic locking:

```sql
UPDATE products
SET stock = stock - 1, version = version + 1
WHERE id = 10 AND version = 7 AND stock > 0;
```

Nếu affected rows = 0, nghĩa là dữ liệu đã bị thay đổi hoặc hết stock, application cần retry hoặc trả lỗi.

### MVCC

MVCC cho phép reader và writer không nhất thiết block nhau bằng cách lưu nhiều phiên bản dữ liệu. Reader có thể nhìn snapshot cũ trong khi writer đang cập nhật bản mới.

Điều cần nhớ:

*   MVCC giúp read concurrency tốt hơn.
*   Transaction dài có thể giữ version cũ lâu, làm phình storage/vacuum pressure.
*   Bạn vẫn có thể gặp lock khi update cùng row hoặc chạy migration.

### Isolation Level

*   **Read Committed:** Mỗi câu query chỉ thấy dữ liệu đã commit trước thời điểm query chạy. Phổ biến, cân bằng tốt.
*   **Repeatable Read:** Trong cùng transaction, đọc lại cùng dữ liệu sẽ thấy cùng snapshot. Giảm non-repeatable read.
*   **Serializable:** Mạnh nhất, gần như các transaction chạy tuần tự. An toàn hơn nhưng dễ conflict/retry hơn.

**Mục tiêu Middle:** Biết chọn cơ chế lock phù hợp cho inventory, wallet, booking slot, coupon usage, order state transition.

---

## 7. Pagination Strategy

Phân trang ảnh hưởng trực tiếp đến latency khi bảng lớn.

### Offset-based Pagination

```sql
SELECT *
FROM orders
ORDER BY created_at DESC
LIMIT 20 OFFSET 10000;
```

*   DB phải quét/bỏ qua nhiều dòng trước khi lấy dữ liệu cần trả.
*   Khi offset lớn, query chậm dần.
*   Dễ trùng/bỏ sót dữ liệu nếu có bản ghi mới insert/delete trong lúc user đang phân trang.

### Cursor-based Pagination

```sql
SELECT *
FROM orders
WHERE created_at < :last_seen_created_at
ORDER BY created_at DESC
LIMIT 20;
```

Tốt hơn khi bảng lớn, feed, timeline, order history, notification list.

Best practice:

*   Cursor nên dựa trên cột có index.
*   Nếu `created_at` có thể trùng, dùng cursor kép `(created_at, id)`.
*   Cursor phù hợp cho next/previous, không phù hợp nếu business cần nhảy thẳng tới trang 1000.

---

## 8. Partitioning - Phần nên học kỹ ở giai đoạn Middle

Partitioning là chia một bảng lớn thành nhiều phần nhỏ hơn để query và bảo trì hiệu quả hơn. Đây không phải giải pháp mặc định cho mọi bảng lớn; nó chỉ tốt khi query pattern và data lifecycle phù hợp.

### Khi nào nên nghĩ tới Partitioning?

*   Bảng rất lớn và query thường lọc theo một key rõ ràng, ví dụ `created_at`, `tenant_id`, `region`.
*   Cần xóa/archive dữ liệu cũ nhanh, ví dụ drop partition theo tháng thay vì delete từng dòng.
*   Index của bảng quá lớn, maintenance chậm.
*   Dữ liệu có lifecycle tự nhiên, ví dụ logs, events, orders, audit records.

### Các kiểu Partitioning

*   **Range Partitioning:** Chia theo khoảng giá trị, thường là thời gian.
    *   Ví dụ: `orders_2026_01`, `orders_2026_02`.
    *   Hợp với log/order/event theo thời gian.
    *   Rủi ro: partition hiện tại bị hot vì toàn bộ write mới dồn vào đó.
*   **Hash Partitioning:** Băm key để phân phối đều.
    *   Ví dụ hash theo `user_id` hoặc `tenant_id`.
    *   Hợp khi muốn chia đều tải.
    *   Rủi ro: query theo range thời gian có thể phải quét nhiều partition.
*   **List Partitioning:** Chia theo danh sách giá trị cố định.
    *   Ví dụ region: `VN`, `SG`, `US`.
    *   Hợp khi miền giá trị ổn định.
    *   Rủi ro: phải xử lý khi xuất hiện giá trị mới.
*   **Vertical Partitioning:** Tách cột nặng ra bảng riêng.
    *   Ví dụ `users` chứa thông tin hay đọc, `user_profiles` chứa bio/avatar/settings lớn.
    *   Hợp khi một số cột rất lớn hoặc ít khi dùng.

### Partition Pruning

Partitioning chỉ thực sự giúp query nhanh khi database loại bỏ được partition không liên quan.

Ví dụ nếu partition theo `created_at`, query này có lợi:

```sql
SELECT *
FROM orders
WHERE created_at >= '2026-01-01'
  AND created_at < '2026-02-01';
```

Query này có thể không tận dụng tốt partition:

```sql
SELECT *
FROM orders
WHERE status = 'PAID';
```

Vì không có điều kiện trên partition key, database có thể phải scan nhiều partition.

### Partitioning vs. Sharding

*   **Partitioning:** Thường vẫn nằm trong cùng database/cluster logic. Mục tiêu là quản lý bảng lớn tốt hơn.
*   **Sharding:** Chia dữ liệu ra nhiều database/node độc lập. Mục tiêu là scale vượt giới hạn một database.

Middle nên nắm partitioning thực tế trước. Sharding học concept và trade-off là đủ.

---

## 9. Replication & Read Replica

Replication là sao chép dữ liệu từ primary sang replica để tăng khả năng đọc, backup hoặc high availability.

### Mô hình phổ biến

*   **Primary:** Nhận write.
*   **Replica/Secondary:** Nhận bản sao dữ liệu, thường dùng cho read query/reporting.

### Lợi ích

*   Tăng read throughput bằng cách route read sang replica.
*   Giảm tải primary.
*   Có node dự phòng nếu primary lỗi.
*   Tách workload reporting khỏi transactional workload.

### Rủi ro quan trọng: Replication Lag

Replica thường cập nhật trễ hơn primary một chút. Điều này tạo lỗi kinh điển:

1. User vừa tạo order thành công trên primary.
2. Request tiếp theo đọc từ replica.
3. Replica chưa kịp sync.
4. User thấy "không tìm thấy order".

### Cách xử lý read-after-write

*   Sau write, đọc lại từ primary trong một khoảng thời gian ngắn.
*   Dùng session consistency: cùng user/session vừa ghi thì ưu tiên primary.
*   Chấp nhận eventual consistency nếu use case không nhạy cảm.
*   Hiển thị trạng thái pending thay vì khẳng định dữ liệu mất.

**Mục tiêu Middle:** Biết khi nào đọc replica được, khi nào bắt buộc đọc primary.

---

## 10. Connection Pooling

Connection pool là phần rất thực tế trong backend. Nhiều hệ thống chết không phải vì query quá phức tạp, mà vì mở quá nhiều connection hoặc giữ connection quá lâu.

### Cần hiểu các thông số

*   **Max open connections:** Số connection tối đa app được mở tới DB.
*   **Max idle connections:** Số connection rảnh được giữ lại để tái sử dụng.
*   **Connection max lifetime:** Tuổi thọ tối đa của connection.
*   **Query timeout:** Thời gian tối đa một query được phép chạy.

### Pool quá nhỏ

*   Request phải chờ connection.
*   API latency tăng dù DB chưa hẳn quá tải.

### Pool quá lớn

*   DB phải xử lý quá nhiều connection.
*   Tăng memory, context switching, lock contention.
*   Có thể làm database nghẽn hoặc crash.

**Rule thực tế:** Tổng connection từ tất cả instance app phải nằm trong giới hạn DB chịu được, không chỉ nhìn từng instance.

---

## 11. Caching Strategy với Redis

Caching giúp giảm latency và tải DB, nhưng cache sai có thể tạo bug consistency rất khó chịu.

### Vì sao Redis nhanh?

*   Dữ liệu nằm trong RAM.
*   Non-blocking I/O multiplexing giúp xử lý nhiều connection trên một thread.
*   Ít chi phí context switching và lock contention trong core execution model.

### Cache-aside Pattern

```text
READ:
API -> Redis
    -> cache hit: trả data
    -> cache miss: đọc DB -> ghi Redis -> trả data

WRITE:
API -> ghi DB -> xóa cache key liên quan
```

Thường xóa cache sau khi ghi DB thay vì update cache trực tiếp để giảm rủi ro race condition.

### Các vấn đề cần biết

*   **Cache invalidation:** Khi dữ liệu gốc đổi, cache phải được xóa/cập nhật đúng.
*   **Cache stampede:** Nhiều request cùng miss một key hot và cùng đánh vào DB.
*   **Cache penetration:** Request liên tục vào key không tồn tại, cache luôn miss.
*   **Cache avalanche:** Nhiều key hết hạn cùng lúc, DB bị dồn tải.

### Cách giảm rủi ro

*   TTL có jitter để tránh hết hạn đồng loạt.
*   Lock/singleflight cho key hot khi rebuild cache.
*   Cache cả negative result trong thời gian ngắn.
*   Không cache dữ liệu có consistency requirement quá chặt nếu chưa có chiến lược invalidation rõ.

---

## 12. Migration, Backup và Restore

Middle backend cần biết thay đổi schema an toàn trên production.

### Migration an toàn

Nguyên tắc chung:

1.  Thêm thay đổi backward-compatible trước.
2.  Deploy app hỗ trợ cả schema cũ và mới.
3.  Backfill dữ liệu theo batch nhỏ.
4.  Chuyển read/write sang schema mới.
5.  Sau khi ổn định mới xóa cột/bảng cũ.

Ví dụ thêm cột bắt buộc:

```text
Sai: thêm NOT NULL column không default vào bảng lớn.
Đúng: thêm nullable column -> backfill -> deploy code mới -> set NOT NULL sau.
```

### Backup/Restore

Backup chỉ có ý nghĩa nếu restore được.

*   Cần biết RPO: chấp nhận mất tối đa bao nhiêu dữ liệu.
*   Cần biết RTO: cần khôi phục trong bao lâu.
*   Phải định kỳ test restore, không chỉ tin rằng backup đang chạy.

---

## 13. Observability & Performance Tuning

Khi lên Middle, bạn cần biết nhìn database như một hệ thống đang sống, không chỉ là nơi lưu dữ liệu.

Metric nên theo dõi:

*   Slow query count và slow query log.
*   Query latency p95/p99.
*   Connection count, idle vs active connection.
*   Lock wait time, deadlock count.
*   CPU, memory, disk I/O, disk space.
*   Index hit ratio/cache hit ratio.
*   Replication lag.
*   Rows scanned vs rows returned.

Checklist khi API chậm:

1.  API chậm ở app, network hay DB?
2.  Query nào chậm nhất?
3.  Query plan có scan nhiều row không?
4.  Có lock wait/deadlock không?
5.  Connection pool có bị chờ không?
6.  Replica có lag không?
7.  Cache hit rate có tụt không?

---

## 14. Sharding - Nên biết, nhưng chưa cần quá sâu ở Middle

Sharding là chia dữ liệu sang nhiều database/node độc lập. Đây là kỹ thuật mạnh nhưng làm hệ thống phức tạp hơn rõ rệt.

### Khi nào cần Sharding?

*   Một database không còn chịu nổi dữ liệu/write/read dù đã tối ưu index, query, partitioning, replication.
*   Cần scale theo tenant/user/region rất lớn.
*   Giới hạn storage hoặc throughput của một node đã bị chạm.

### Khó ở đâu?

*   Chọn shard key sai tạo hot shard.
*   Query cross-shard rất đắt.
*   Join cross-shard khó hoặc không khả thi.
*   Transaction cross-shard phức tạp.
*   Rebalancing dữ liệu khi shard đầy không đơn giản.

### Shard key tốt

*   Phân phối dữ liệu đều.
*   Phù hợp query pattern phổ biến.
*   Ít thay đổi.
*   Tránh key tăng tuần tự nếu làm write dồn vào một shard.

**Mục tiêu Middle:** Giải thích được sharding khác partitioning thế nào, vì sao shard key quan trọng, và vì sao không nên sharding quá sớm.

---

## 15. MongoDB vs PostgreSQL

MongoDB và PostgreSQL không phải cái nào "tốt hơn tuyệt đối". Chọn cái nào phụ thuộc data model, query pattern, consistency requirement, team experience và vận hành production.

| Tiêu chí | PostgreSQL | MongoDB |
|---|---|---|
| Mô hình dữ liệu | Relational, bảng, row, foreign key | Document, collection, BSON document |
| Schema | Rõ, enforce mạnh bằng DB | Linh hoạt, cần schema governance/validation |
| Quan hệ dữ liệu | Mạnh với join, constraint, transaction | Hợp document aggregate, embed/reference có chọn lọc |
| Transaction | ACID mạnh, quen thuộc | Có transaction nhưng chi phí cao hơn, không nên lạm dụng |
| Query phức tạp | SQL mạnh cho join/reporting | Aggregation pipeline mạnh nhưng dễ tốn memory/CPU |
| Consistency | Strong consistency dễ hơn | Cần hiểu read/write concern, replica lag |
| Scale | Vertical, read replica, partitioning, sharding khi rất lớn | Replica set, sharding built-in hơn nhưng shard key khó |
| Use case hợp | order, payment, inventory, user, permission, tài chính | catalog/menu, content/config, event-like document, dữ liệu nested |

### Khi nên chọn PostgreSQL?

* Dữ liệu có quan hệ rõ: user, order, payment, inventory.
* Cần constraint mạnh: foreign key, unique, check constraint.
* Cần transaction ACID thường xuyên.
* Query/reporting cần join.
* Team cần data integrity được DB bảo vệ.

Ví dụ F&B/POS:

* `orders`, `payments`, `inventory_movements`, `users`, `stores` thường hợp PostgreSQL.
* Các operation như trừ tồn kho, thanh toán, booking slot cần transaction/locking rõ.

### Khi nên chọn MongoDB?

* Dữ liệu tự nhiên là document/nested object.
* Shape thay đổi theo tenant/client nhưng vẫn kiểm soát được schema.
* Đọc một aggregate lớn cùng lúc, ví dụ catalog/menu/config.
* Muốn embed data con nhỏ để giảm join.
* Cần phát triển nhanh nhưng vẫn có schema validation và index strategy.

Ví dụ F&B/POS:

* menu/catalog có option groups, modifiers, store-specific config có thể hợp MongoDB nếu đọc theo document.
* Nhưng order/payment core vẫn cần cân nhắc relational nếu consistency cao.

### Lỗi chọn MongoDB hay gặp

* Nghĩ schema-less là không cần schema.
* Embed array tăng vô hạn như order logs/comments/events.
* Dùng `$lookup` như join SQL ở scale lớn.
* Không thiết kế index theo query pattern.
* Lạm dụng transaction nhiều document.
* Shard key sai gây hot shard.

### Lỗi chọn PostgreSQL hay gặp

* Normalize quá mức làm read path phải join quá nhiều.
* Dùng JSONB để né schema nhưng không governance.
* Thiếu index cho query phổ biến.
* Migration bảng lớn không theo expand/contract.
* Dùng offset pagination trên bảng lớn.

### Câu trả lời phỏng vấn mẫu

> Với dữ liệu core như order, payment, inventory, tôi nghiêng về PostgreSQL vì transaction, constraint và query relational rõ hơn. Với dữ liệu document như menu/catalog/config có cấu trúc nested và thay đổi theo tenant, MongoDB có thể hợp hơn. Nhưng MongoDB không có nghĩa là bỏ schema; production vẫn cần schema validation, index strategy và tránh unbounded array/hot document.

### Ghi chú thực tế: câu phỏng vấn "chọn SQL hay MongoDB, và scale thế nào?"

Câu hỏi interviewer đưa ra: chọn MySQL/SQL hay MongoDB, sau đó hỏi tiếp về khả năng scale của lựa chọn đó.

Câu trả lời lúc phỏng vấn: đã chọn SQL vì "truyền thống hơn và tốt cho tác vụ bình thường hơn". Khi bị hỏi sâu về scale thì chỉ nói được MongoDB scale ngang tốt và dễ, còn MySQL thì scale dọc và có thể scale ngang bằng sharding. Thiếu phần lý do/trade-off phía sau, nên câu trả lời nghe cảm tính.

Bản đầy đủ hơn nên trả lời theo mạch sau:

1.  **Điểm mạnh cốt lõi của từng loại**
    *   SQL (MySQL/PostgreSQL): consistency mạnh, quan hệ giữa bảng rõ ràng, transaction chuẩn ACID → an toàn và logic cho nghiệp vụ CRUD/giao dịch/tài chính/đơn hàng.
    *   MongoDB (NoSQL, document): schema linh hoạt, hợp dữ liệu phi cấu trúc/log/nội dung/feed, dễ mở rộng theo chiều ngang.
2.  **Scale của SQL truyền thống** đi theo các hướng:
    *   Scale-up: tăng CPU/RAM.
    *   Replication: read replica để chia tải đọc.
    *   Partitioning: chia nhỏ bảng.
    *   Sharding: tự chia dữ liệu ra nhiều node (phức tạp hơn vì không built-in).
3.  **Scale của MongoDB**: hỗ trợ sharding sẵn (built-in), schema linh hoạt nên tách dữ liệu ra nhiều node để chịu tải thường triển khai dễ hơn SQL.
4.  **Đánh đổi (trade-off) cần nói ra**: ở MongoDB, đảm bảo ACID mạnh ở quy mô lớn phức tạp hơn, và join phức tạp không tự nhiên như SQL — đây chính là phần bị thiếu trong câu trả lời lúc phỏng vấn.
5.  **Kết luận không phải "cái nào tốt hơn" mà là "hợp bài toán nào"**: hệ thống kế toán/giao dịch → SQL; nền tảng có nội dung do người dùng tạo nhiều, schema đổi thường xuyên, cần scale nhanh → MongoDB hợp hơn.

Bài học: khi trả lời câu hỏi kiểu này, cần thể hiện hiểu trade-off (consistency vs scalability) thay vì chỉ nói theo cảm tính kiểu "truyền thống hơn" hay "mới hơn".

---

## 16. MongoDB Schema Validation

MongoDB là schema-flexible, nhưng production vẫn nên có ràng buộc để tránh data pollution.

*   **Schema Validation:** Dùng `$jsonSchema` để bắt buộc kiểu dữ liệu, required field, format.
*   **schema_version:** Lưu version trong document để hỗ trợ rolling migration.
*   **Index trong MongoDB:** Vẫn cần thiết kế index theo query pattern, tránh tạo index tràn lan.
*   **Document design:** Embed khi dữ liệu con nhỏ, đọc cùng parent và không tăng vô hạn. Reference khi dữ liệu lớn, dùng độc lập hoặc quan hệ nhiều-nhiều.

---

## 17. Bảng ghi nhớ Latency/Throughput liên quan DB

| Kỹ thuật | Latency | Throughput | Ghi chú |
| --- | --- | --- | --- |
| Cache | Giảm mạnh | Tăng | Cẩn thận invalidation |
| Database index | Giảm | Tăng read | Tăng chi phí write |
| Read replica | Không đổi/giảm nhẹ | Tăng read | Cẩn thận replication lag |
| Connection pool | Giảm | Tăng | Sai cấu hình có thể làm DB nghẽn |
| Batch processing | Tăng từng request | Tăng tổng thể | Hợp job nền, import/export |
| Lock nhiều | Tăng | Giảm | Dễ gây wait/deadlock |
| Partitioning | Giảm nếu pruning tốt | Tăng/ổn định hơn | Phụ thuộc query có partition key |
| Sharding | Có thể giảm | Tăng lớn | Đổi lại vận hành phức tạp |

---

## 18. PostgreSQL production patterns dễ bị hỏi

Phần này gom các tình huống production hay bị hỏi khi CV có PostgreSQL/MySQL và backend API thực tế.

### Delete/archive dữ liệu lớn

Không nên chạy một câu `DELETE` lớn trên bảng nhiều triệu/tỷ rows nếu chưa đánh giá lock, WAL/binlog, replication lag và I/O.

Pattern:

* Batch delete theo primary key/time range.
* Partition theo `created_at` rồi drop partition khi archive.
* Copy-swap/shadow table cho case đặc biệt.
* Chạy ngoài giờ cao điểm, có sleep giữa batch và metric theo dõi.

### Online schema migration

Migration production cần backward-compatible.

Quy trình expand/contract:

1. Add column/table mới theo cách không phá code cũ.
2. Deploy code ghi cả old/new nếu cần.
3. Backfill dữ liệu theo batch.
4. Đọc từ schema mới.
5. Sau khi ổn định mới drop old column/path.

Lưu ý:

* Add `NOT NULL`/default/constraint trên bảng lớn có thể tốn lock tùy DB/version.
* Index mới nên cân nhắc `CONCURRENTLY` trong PostgreSQL.
* Luôn có rollback plan.

### Count exact và pagination

`SELECT COUNT(*)` trên bảng lớn có thể đắt nếu request nào cũng cần total.

Lựa chọn:

* Estimated count.
* Cached count.
* Chỉ trả `has_next`.
* Async/reporting query cho màn hình admin cần tổng chính xác.

Cursor pagination nên dùng cho feed, notification, order history; offset pagination chỉ hợp dữ liệu nhỏ hoặc cần nhảy trang ngẫu nhiên.

### Unique constraint chống race condition

Application check không đủ để đảm bảo unique.

Pattern:

* Dùng unique constraint/index ở DB.
* App bắt duplicate key và trả `409 Conflict` hoặc retry tùy case.
* Với soft delete, dùng partial unique index nếu cần unique trên dữ liệu chưa xóa.

Ví dụ:

```sql
CREATE UNIQUE INDEX users_email_active_unique
ON users(email)
WHERE deleted_at IS NULL;
```

### Read replica và read-after-write

Read replica giúp tăng read throughput nhưng có replication lag.

Cách xử lý read-after-write:

* Đọc primary ngay sau write cho user/session đó.
* Sticky read trong vài giây sau write.
* Primary fallback nếu data chưa thấy trên replica.
* UI hiển thị pending nếu eventual consistency chấp nhận được.

### N+1 query

N+1 xảy ra khi load list rồi query từng item liên quan.

Fix:

* Join nếu dữ liệu quan hệ rõ.
* Batch query bằng `WHERE id IN (...)`.
* Dataloader pattern.
* Preload/prefetch có kiểm soát.
* Log query count trong request để phát hiện sớm.

### Connection pool exhaustion

Pool quá nhỏ làm request chờ connection. Pool quá lớn làm DB quá tải vì tổng connection của nhiều app instances vượt capacity DB.

Cần monitor:

* active/idle/wait connections.
* wait duration.
* query latency.
* transaction duration.
* max connection trên DB.

Rule thực tế:

```text
total_connections = app_instances * max_open_connections
```

Con số này phải nhỏ hơn capacity DB sau khi trừ connection cho migration, admin, background jobs và monitoring.

### Money, decimal và time

Money:

* Không dùng float cho tiền.
* Dùng smallest currency unit hoặc decimal.
* Có rounding policy rõ ràng.
* Lưu tỷ giá snapshot nếu có currency conversion.

Time:

* Store UTC.
* Convert at edge.
* Lưu timezone của user/store nếu business cần lịch địa phương.
* Test quanh month-end/DST nếu hệ thống có timezone quốc tế.

### Geospatial search

Search "near me" không nên tính distance toàn bộ rows.

Lựa chọn:

* PostGIS.
* spatial index.
* bounding box prefilter.
* geohash nếu cần partition/cache theo vùng.

---

## 19. Checklist năng lực Middle Database

Bạn có thể tự đánh dấu theo thứ tự:

*   [ ] Đọc được `EXPLAIN` / `EXPLAIN ANALYZE`.
*   [ ] Thiết kế composite index theo query pattern.
*   [ ] Biết nhận ra over-indexing.
*   [ ] Thiết kế schema có constraints hợp lý.
*   [ ] Hiểu lock wait, deadlock và cách giảm transaction dài.
*   [ ] Biết optimistic/pessimistic locking dùng khi nào.
*   [ ] Biết row lock, table lock, `SELECT ... FOR UPDATE`, `NOWAIT`, `SKIP LOCKED`.
*   [ ] Biết offset pagination chậm vì sao và chuyển sang cursor pagination.
*   [ ] Biết partitioning giúp gì, cần partition key nào.
*   [ ] Biết replication lag và read-after-write consistency.
*   [ ] Biết cấu hình connection pool theo tổng số app instances.
*   [ ] Biết xử lý online schema migration theo expand/contract.
*   [ ] Biết phát hiện và xử lý N+1 query.
*   [ ] Biết dùng unique constraint/partial unique index để chống race condition.
*   [ ] Biết cache-aside và các lỗi cache stampede/penetration/avalanche.
*   [ ] Biết migration production theo hướng backward-compatible.
*   [ ] Biết đọc slow query log và metric DB cơ bản.
*   [ ] Giải thích được sharding và trade-off mà không over-engineer.
*   [ ] So sánh được MongoDB vs PostgreSQL theo data model, transaction, query pattern và production trade-off.
