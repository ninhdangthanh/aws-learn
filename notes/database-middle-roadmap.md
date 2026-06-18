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

## 15. MongoDB Schema Validation

MongoDB là schema-flexible, nhưng production vẫn nên có ràng buộc để tránh data pollution.

*   **Schema Validation:** Dùng `$jsonSchema` để bắt buộc kiểu dữ liệu, required field, format.
*   **schema_version:** Lưu version trong document để hỗ trợ rolling migration.
*   **Index trong MongoDB:** Vẫn cần thiết kế index theo query pattern, tránh tạo index tràn lan.
*   **Document design:** Embed khi dữ liệu con nhỏ, đọc cùng parent và không tăng vô hạn. Reference khi dữ liệu lớn, dùng độc lập hoặc quan hệ nhiều-nhiều.

---

## 16. Bảng ghi nhớ Latency/Throughput liên quan DB

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

## 17. Checklist năng lực Middle Database

Bạn có thể tự đánh dấu theo thứ tự:

*   [ ] Đọc được `EXPLAIN` / `EXPLAIN ANALYZE`.
*   [ ] Thiết kế composite index theo query pattern.
*   [ ] Biết nhận ra over-indexing.
*   [ ] Thiết kế schema có constraints hợp lý.
*   [ ] Hiểu lock wait, deadlock và cách giảm transaction dài.
*   [ ] Biết optimistic/pessimistic locking dùng khi nào.
*   [ ] Biết offset pagination chậm vì sao và chuyển sang cursor pagination.
*   [ ] Biết partitioning giúp gì, cần partition key nào.
*   [ ] Biết replication lag và read-after-write consistency.
*   [ ] Biết cấu hình connection pool theo tổng số app instances.
*   [ ] Biết cache-aside và các lỗi cache stampede/penetration/avalanche.
*   [ ] Biết migration production theo hướng backward-compatible.
*   [ ] Biết đọc slow query log và metric DB cơ bản.
*   [ ] Giải thích được sharding và trade-off mà không over-engineer.

