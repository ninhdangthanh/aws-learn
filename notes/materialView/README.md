# Materialized View Demo (PostgreSQL)

Demo đầy đủ: dummy data → pre-aggregate bằng materialized view → refresh tự động mỗi 30s.

## Mục đích chính của materialized view — bạn nghĩ đúng

Materialized view sinh ra để **đổi độ tươi của dữ liệu lấy tốc độ đọc**.

Cụ thể là: một query aggregate nặng (join nhiều bảng, `GROUP BY`, window function) mà
**bị gọi đi gọi lại nhiều lần hơn số lần dữ liệu gốc đổi** → tính sẵn một lần, ghi kết quả
xuống đĩa như một cái bảng, mọi lần đọc sau chỉ là `SELECT` từ bảng đó.

Ba điều kiện phải đúng cùng lúc thì matview mới hợp lý:

1. Query **đắt** — index không cứu được, vì index giúp *lọc* chứ không giúp *cộng dồn*.
2. Read/write **lệch mạnh** — đọc nhiều, dữ liệu gốc đổi ít hoặc đổi liên tục nhưng không ai cần thấy ngay.
3. Nghiệp vụ **chấp nhận trễ** — dashboard trễ 30s thì không sao, số dư ví trễ 30s là mất tiền.

Refresh 30s như bạn nói chính là đang chọn: "chấp nhận sai lệch tối đa 30 giây, đổi lấy
query nhanh gấp mấy nghìn lần". Đó là đúng bài.

## Postgres KHÔNG tự refresh — đây là chỗ hay nhầm nhất

Materialized view **không hề tự cập nhật**. Postgres không có auto-refresh, không có option,
không có setting nào bật lên được. Matview nằm im như một cái bảng chết cho tới khi **ai đó
bên ngoài** chạy:

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_daily_revenue;
```

Chu kỳ 30s trong demo này đến từ `refresher.sh` — một vòng lặp shell gọi `psql`, hoàn toàn
nằm ngoài Postgres. Chứng minh bằng `./demo.sh frozen`, output thật:

```
== tắt refresher, thêm 1 đơn 777.000.000đ ==
16:23:54 | matview=616904407.00 | realtime=1393904407.00
16:24:24 | matview=616904407.00 | realtime=1393904407.00
16:25:05 | matview=616904407.00 | realtime=1393904407.00   <- 70s, vẫn đứng im
== bật lại refresher ==
16:25:32 | matview=1393904407.00 | realtime=1393904407.00  <- khớp
```

Chờ 70 giây hay 70 ngày cũng vậy. Không ai gọi `REFRESH` thì con số đó nằm nguyên.

Nói cách khác: **matview là cache thủ công nằm trong DB**. Postgres lo phần lưu trữ và
tính toán, còn *khi nào tính lại* là việc của bạn. Khác hẳn index — index thì Postgres tự
cập nhật theo mỗi lần ghi, còn matview thì không.

## Số đo thật từ demo này

Data: 150.000 đơn, 450.000 dòng item, 90 ngày.

| Việc | Thời gian |
| --- | --- |
| `SELECT` từ view thường (join + group by 450k dòng mỗi lần) | **242 ms** |
| `SELECT` từ materialized view (index scan trên 91 dòng) | **0.023 ms** |
| Refresh `mv_daily_revenue` | ~235 ms |
| Dung lượng bảng gốc `order_items` | 52 MB |
| Dung lượng `mv_daily_revenue` | 48 kB |

Nhanh hơn ~10.000 lần, tốn thêm 48 kB đĩa, trả giá bằng 235ms CPU mỗi 30s.

Đây chính là lý do tồn tại của matview: **90 ngày dữ liệu nén xuống còn 91 dòng đã cộng sẵn.**

## Chạy

```bash
docker compose up -d          # postgres (cổng 5434) + refresher 30s
./demo.sh all                 # chạy demo 1,2,3,4
./demo.sh watch               # xem refresher 30s tự cập nhật (mất ~40s)
./demo.sh frozen              # tắt refresher -> matview đứng im (mất ~100s)

psql "postgresql://app:app@localhost:5434/matview_lab"   # vào tay
```

Schema + seed + matview được nạp tự động lúc container khởi tạo lần đầu.
Muốn seed lại từ đầu: `docker compose down -v && docker compose up -d`.

## 6 kịch bản demo

| Lệnh | Chứng minh điều gì |
| --- | --- |
| `./demo.sh speed` | Pre-aggregate: 242ms → 0.023ms. Kèm `EXPLAIN ANALYZE` hai bên. |
| `./demo.sh stale` | Ghi vào bảng gốc **không** chảy vào matview. Phải refresh mới thấy. |
| `./demo.sh lock` | `REFRESH` thường khoá reader **5,1 giây**; `REFRESH CONCURRENTLY` reader chỉ mất **1 ms**. |
| `./demo.sh inspect` | `pg_matviews`, dung lượng, lịch sử refresh + thời gian trung bình. |
| `./demo.sh watch` | Cắm 1 đơn mới rồi poll 5s/lần — thấy matview đuổi kịp trong vòng 30s. |
| `./demo.sh frozen` | Tắt refresher → matview đứng im vĩnh viễn. Postgres không tự refresh. |

Output thật của `demo.sh watch`:

```
16:08:48 | matview=1614614524.00 | realtime=2114614524.00   <- lệch
16:08:53 | matview=1614614524.00 | realtime=2114614524.00   <- vẫn lệch
16:08:58 | matview=2114614524.00 | realtime=2114614524.00   <- refresher chạy, khớp
```

## File

| File | Nội dung |
| --- | --- |
| `docker-compose.yml` | Postgres 16 (cổng **5434**) + container `refresher` |
| `refresher.sh` | Vòng lặp `psql` gọi `refresh_all_matviews()` mỗi 30s |
| `sql/001-schema.sql` | Bảng gốc + bảng `matview_refresh_log` |
| `sql/002-seed.sql` | Dummy data, `setseed(0.42)` nên ai chạy cũng ra cùng số |
| `sql/003-matviews.sql` | View thường, 3 matview, hàm `refresh_matview()` |
| `sql/004-demo-speed.sql` | Demo 1 |
| `sql/005-demo-staleness.sql` | Demo 2 |
| `sql/006-inspect.sql` | Demo 4 |
| `demo.sh` | Chạy các kịch bản |

## Ba matview trong demo, mỗi cái minh hoạ một dạng

```sql
-- 1. Rollup theo thời gian — dạng phổ biến nhất. 450k dòng -> 91 dòng.
mv_daily_revenue   (day, orders, units, revenue)

-- 2. Aggregate + window function. rank() là thứ index không cứu được,
--    phải tính lại toàn bộ partition mỗi lần chạy.
mv_product_sales   (product_id, category, units_sold, revenue, rank_in_category)

-- 3. KPI một dòng cho màn hình dashboard. Matview 1 dòng vẫn cần unique
--    index để CONCURRENTLY chạy được -> dùng hằng số `1 AS id` làm khoá.
mv_dashboard_kpi   (id, total_paid_orders, total_customers, total_revenue, ...)
```

## Những chỗ dễ vấp

**`REFRESH` thường khoá cả reader.** Nó lấy `ACCESS EXCLUSIVE LOCK`, mọi `SELECT` lên matview
đó bị treo cho tới khi refresh xong. Trên production với matview refresh 5 giây, đó là 5 giây
dashboard đứng hình. Đo được trong `./demo.sh lock`: **5148 ms**.

**`REFRESH CONCURRENTLY` không khoá reader** — reader đọc bản cũ trong lúc bản mới đang được
dựng, xong thì tráo. Đo được: **1 ms**. Đánh đổi:

- Bắt buộc phải có **UNIQUE INDEX** phủ toàn bộ dòng, không có thì Postgres từ chối.
- Matview phải **đã populate** ít nhất một lần (`WITH NO DATA` thì lần đầu phải refresh thường).
- Chậm hơn refresh thường vì phải dựng bảng tạm rồi diff — refresh full bảng nhỏ có khi lại nhanh hơn.
- Không giảm được I/O: vẫn tính lại **toàn bộ**, không có refresh incremental.

**Postgres không lưu thời điểm refresh cuối.** Không có cột nào trong `pg_matviews` cho biết
"lần cuối refresh lúc nào". Muốn biết thì tự ghi log — demo này làm vậy qua bảng
`matview_refresh_log` và hàm `refresh_matview()`.

**Không có incremental refresh.** Mỗi lần refresh là tính lại từ đầu 100%. Matview 300 triệu
dòng refresh mỗi 30s là tự sát — lúc đó phải chuyển sang rollup table + trigger/CDC, hoặc
extension như `pg_ivm`.

**Refresh chồng nhau.** Nếu refresh mất 40s mà lịch là 30s, các lần refresh sẽ xếp hàng chờ
nhau. Production nên bọc `pg_try_advisory_lock()` để lần sau bỏ qua nếu lần trước chưa xong.

## Chạy refresh bằng gì trong production

Demo này dùng container chạy vòng lặp `psql` cho dễ nhìn — refresh chỉ là một câu SQL, ai gọi
nó cũng được:

| Cách | Khi nào hợp |
| --- | --- |
| Vòng lặp trong app / goroutine ticker | App đã có sẵn, muốn log/metrics chung một chỗ |
| **pg_cron** | Đúng nhất — lịch nằm trong DB, DB failover thì lịch đi theo |
| Kubernetes `CronJob` | Hạ tầng đã k8s, muốn tách khỏi app |
| Trigger sau khi ghi | Dữ liệu đổi rất ít, cần tươi ngay — nhưng làm write chậm hẳn |

Với **pg_cron** (cần image có extension, ví dụ `postgres:16` + build hoặc bản managed của AWS RDS):

```sql
CREATE EXTENSION pg_cron;

-- pg_cron >= 1.5 hỗ trợ cú pháp giây
SELECT cron.schedule('refresh-matviews', '30 seconds',
                     $$SELECT refresh_all_matviews()$$);

SELECT * FROM cron.job;
SELECT * FROM cron.job_run_details ORDER BY start_time DESC LIMIT 10;
```

## Khi nào ĐỪNG dùng materialized view

- Cần đọc real-time tuyệt đối: số dư ví, tồn kho, giá bán tại thời điểm checkout.
- Query đã nhanh sẵn nhờ index — thêm matview chỉ tốn chỗ và thêm chỗ để hỏng.
- Dữ liệu gốc đổi liên tục và ai cũng cần thấy ngay → rollup table cập nhật bằng trigger,
  hoặc cộng dồn trong app khi ghi.
- Matview quá lớn khiến thời gian refresh gần bằng chu kỳ refresh.
- Chỉ cần cache kết quả một query cho một user → Redis đơn giản hơn nhiều.

## Matview vs các lựa chọn khác

| | Tươi | Tốc độ đọc | Chi phí ghi | Độ phức tạp |
| --- | --- | --- | --- | --- |
| View thường | Real-time | Chậm (tính lại mỗi lần) | 0 | Thấp |
| **Materialized view** | Trễ theo chu kỳ | Rất nhanh | 0 (dồn vào lúc refresh) | Thấp |
| Rollup table + trigger | Real-time | Rất nhanh | Mỗi write nặng thêm | Trung bình |
| Cache Redis | Trễ theo TTL | Rất nhanh | 0 | Trung bình (invalidation) |
| CDC → OLAP | Trễ vài giây | Rất nhanh | 0 | Cao |

Matview thắng ở chỗ: **gần như không phải viết code**. Một câu `CREATE MATERIALIZED VIEW` +
một cái lịch refresh, không cần đụng vào đường ghi, không cần lo invalidation.

## Dọn

```bash
docker compose down -v
```
