# Câu hỏi kinh điển trong phỏng vấn Backend

File này gom các câu hỏi tình huống mở mà gần như buổi phỏng vấn Middle Backend nào cũng hỏi: "API chậm thì làm sao", "service sập lúc 3h sáng thì xử lý thế nào", "traffic tăng 10 lần thì sao".

Điểm chung của nhóm câu này: **không có đáp án đúng duy nhất**. Người phỏng vấn không chấm bạn ở kết luận mà chấm ở **quy trình suy nghĩ**. Ba thứ họ muốn thấy:

1. Bạn **đo trước khi đoán** — không nhảy ngay vào "chắc là thiếu index".
2. Bạn **thu hẹp phạm vi có hệ thống** — chia tầng, loại trừ dần.
3. Bạn **biết trade-off** của mỗi giải pháp, không chỉ liệt kê buzzword.

> Sai lầm phổ biến nhất khi trả lời nhóm câu này: liệt kê một tràng giải pháp ("thêm cache, thêm index, scale thêm pod, dùng CDN") trước khi biết nguyên nhân. Nghe thì có vẻ hiểu nhiều nhưng thực chất là đang đoán mò, và người phỏng vấn nhận ra ngay.

---

## 1. "Một API chạy chậm, bạn xử lý thế nào?"

Đây là câu kinh điển nhất. Trả lời theo 4 bước.

### Bước 1: Làm rõ câu hỏi trước khi trả lời

Đừng vội đi vào giải pháp. Hỏi ngược lại — điều này luôn được đánh giá cao:

* **Chậm là bao nhiêu?** 500ms hay 30 giây? Kỳ vọng là bao nhiêu?
* **Chậm ở p50 hay p99?** Đây là câu hỏi quan trọng nhất, xem giải thích bên dưới.
* **Chậm từ bao giờ?** Mới chậm sau deploy hôm qua, hay chậm dần trong 3 tháng?
* **Chậm với ai?** Tất cả user, hay chỉ một tenant/một vùng địa lý?
* **Chậm liên tục hay theo lúc?** Chỉ chậm giờ cao điểm? Chỉ chậm mỗi đầu giờ?
* **Chỉ API này hay tất cả API?** Nếu tất cả cùng chậm thì vấn đề ở tầng hạ tầng chung, không phải logic của endpoint.

**Vì sao p50 vs p99 quyết định hướng điều tra:**

| Triệu chứng | Ý nghĩa | Hướng nghi ngờ |
|---|---|---|
| p50 cao (mọi request đều chậm) | Vấn đề ở logic/query chính | Query thiếu index, N+1, gọi external API tuần tự, payload lớn |
| p50 thấp nhưng p99 rất cao | Đa số nhanh, một nhóm nhỏ chết | Lock contention, connection pool chờ, GC pause, hot partition, một tenant có dữ liệu khổng lồ, cache miss |
| Cả hai tăng dần theo thời gian | Cạn dần tài nguyên | Memory leak, bảng phình to mà query không đổi, connection leak, index bloat |
| Tăng theo bậc thang đúng giờ | Yếu tố bên ngoài | Cron/batch job, backup, đối thủ tranh tài nguyên trên cùng host |

> Nếu p50 = 40ms mà p99 = 8 giây, đừng đi tối ưu query — query đang chạy tốt cho 99% request. Cái cần tìm là **thứ chỉ xảy ra với 1% request**: đó gần như luôn là chờ đợi tài nguyên chứ không phải tính toán.

### Bước 2: Đo và định vị — chậm ở tầng nào?

Một request đi qua nhiều tầng. Phải xác định thời gian mất ở đâu trước khi sửa bất cứ thứ gì.

```
Client → DNS → CDN/Edge → Load Balancer → API Gateway → App
                                                          ├→ Database
                                                          ├→ Cache
                                                          ├→ Message Queue
                                                          └→ External API
```

**Cách xác định nhanh:**

* So sánh **thời gian client đo** với **thời gian server tự log**. Client thấy 3s mà server log 80ms → vấn đề ở network, payload size, TLS handshake, DNS, hoặc client đang chờ request khác. Đây là chỗ rất hay bị bỏ sót.
* Nhìn **distributed tracing** (OpenTelemetry, Jaeger, Datadog APM). Trace cho thấy ngay span nào ăn hết thời gian. Nếu chưa có tracing, đây chính là lý do nên có — nói điều này khi phỏng vấn cũng là một điểm cộng.
* Nếu chưa có tracing, thêm log timing thủ công quanh từng đoạn: thời gian query DB, thời gian gọi service ngoài, thời gian serialize response.
* Xem **metric hạ tầng** cùng khung giờ: CPU, memory, disk I/O, network của app và DB; connection pool active/idle/waiting; replication lag; cache hit rate.

**Phân loại kết quả:**

| Thời gian mất ở đâu | Nghi ngờ chính |
|---|---|
| Trong app, CPU cao | Vòng lặp nặng, serialize JSON lớn, mã hoá/nén, regex tệ, sort trong memory |
| Trong app, CPU thấp nhưng chờ lâu | Đang chờ I/O: DB, external API, lock, connection pool |
| Trong DB | Query chậm, thiếu index, lock wait, replica lag |
| Ở external call | Third-party chậm, không có timeout, retry storm |
| Giữa client và server | Payload lớn, không gzip, không CDN, kết nối xa về địa lý |

### Bước 3: Các nguyên nhân thường gặp và cách xử lý

Sắp theo tần suất thực tế gặp trong production.

**a) N+1 query — thủ phạm số một**

```go
orders := db.Find(&orders)           // 1 query
for _, o := range orders {
    o.User = db.FindUser(o.UserID)   // N query nữa
}
```

100 order thành 101 query. Mỗi query 2ms round-trip thì mất 200ms cho thứ đáng lẽ 5ms.

Xử lý: `JOIN`, eager loading (`Preload`/`Include`), hoặc gom lại thành một truy vấn `WHERE id IN (...)` rồi map trong memory. Với GraphQL thì dùng DataLoader để batch.

Dấu hiệu nhận biết: log DB thấy cùng một pattern query lặp lại hàng chục lần trong một request.

**b) Query thiếu index hoặc index sai**

Chạy `EXPLAIN ANALYZE`. Cần nhìn: `Seq Scan` trên bảng lớn, `Rows Removed by Filter` cao, node `Sort` với `external merge Disk`, chênh lệch lớn giữa `rows` ước lượng và `actual rows`.

Xử lý: thêm/sửa index theo đúng query pattern (chi tiết thứ tự cột xem [Database Middle Roadmap - Compound index](database-middle-roadmap.md)), viết lại query, hoặc `ANALYZE` lại bảng nếu statistics đã cũ khiến planner chọn sai.

**c) Offset pagination trên bảng lớn**

`LIMIT 20 OFFSET 500000` buộc DB đọc và bỏ đi 500k row. Trang càng sâu càng chậm tuyến tính.

Xử lý: chuyển sang cursor/keyset pagination — `WHERE (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT 20`.

**d) Gọi external API tuần tự, hoặc không có timeout**

3 API bên ngoài, mỗi cái 300ms, gọi tuần tự = 900ms. Gọi song song = 300ms.

Nguy hiểm hơn: **không đặt timeout**. Một service ngoài treo 60 giây sẽ giữ luôn connection pool và goroutine/thread của bạn, kéo sập cả service — đây là cascading failure.

Xử lý: gọi song song khi các call độc lập; luôn đặt timeout ngắn hơn timeout của chính API mình; thêm circuit breaker; cân nhắc đẩy phần không cần trả lời ngay sang async qua queue.

**e) Connection pool cạn**

Triệu chứng rất đặc trưng: p50 bình thường, p99 nhảy vọt, DB CPU lại thấp. Request không chậm vì query chậm mà vì **xếp hàng chờ connection**.

Nguyên nhân thường là: pool size quá nhỏ so với concurrency, connection leak (quên đóng), transaction mở quá lâu, hoặc có query chậm chiếm giữ connection.

Xử lý: sửa nguyên nhân giữ connection lâu trước, rồi mới chỉnh pool size. Tăng pool bừa bãi chỉ dời điểm nghẽn xuống DB.

**f) Lock contention**

Nhiều transaction cùng update một dòng nóng (số dư ví, tồn kho sản phẩm hot, counter). Chúng phải xếp hàng tuần tự.

Xử lý: rút ngắn transaction (không gọi API ngoài bên trong transaction), giảm isolation level nếu nghiệp vụ cho phép, tách row nóng thành nhiều bucket rồi cộng dồn, hoặc dùng optimistic locking.

**g) Thiếu cache**

Query đắt hoặc dữ liệu ít đổi mà lần nào cũng vào DB.

Xử lý: cache-aside với TTL hợp lý. Nhưng **cache là bước cuối, không phải bước đầu** — cache một query tệ chỉ là giấu vấn đề, và sẽ lộ ra ngay khi cache miss hoặc Redis down.

**h) Response payload quá lớn**

Trả 5MB JSON vì `SELECT *` và không phân trang. Tốn thời gian query, serialize, truyền và cả parse ở client.

Xử lý: chỉ chọn cột cần dùng, phân trang, cho phép client chọn field, bật gzip/brotli, cân nhắc stream nếu là export.

**i) Vấn đề runtime**

Go: GC pause do allocate quá nhiều, goroutine leak. Node.js: chặn event loop bằng tính toán đồng bộ hoặc `JSON.parse` object khổng lồ. JVM: full GC.

Xử lý: profile bằng `pprof` (Go) hoặc `--prof`/clinic.js (Node), giảm allocation, đẩy tính toán nặng sang worker.

### Bước 4: Xác minh và phòng ngừa

Sửa xong phải chứng minh là đã sửa, và ngăn nó tái diễn:

* Đo lại p50/p95/p99 trước và sau, trên production chứ không chỉ local.
* Thêm alert theo latency và error rate để lần sau biết trước khi user báo.
* Thêm slow query log threshold.
* Nếu là bug về dữ liệu lớn dần, viết load test với dữ liệu ở quy mô thật.
* Ghi lại postmortem ngắn.

### Câu trả lời gọn 60 giây

> Đầu tiên tôi làm rõ "chậm" là gì: chậm bao nhiêu, ở p50 hay p99, từ khi nào, với tất cả user hay một nhóm. p50 cao nghĩa là logic hoặc query chính có vấn đề; p50 thấp mà p99 cao thường là chờ tài nguyên như connection pool, lock hay GC. Tiếp theo tôi định vị bằng tracing hoặc log timing để biết thời gian mất ở app, DB, external call hay network — so cả thời gian client đo với thời gian server log. Khi đã biết tầng nào thì mới đi vào nguyên nhân: hay gặp nhất là N+1 query, thiếu index, offset pagination sâu, gọi API ngoài tuần tự không timeout, hoặc connection pool cạn. Sửa xong tôi đo lại p99 trên production và thêm alert cùng slow query log để lần sau phát hiện sớm. Nguyên tắc của tôi là đo trước rồi mới sửa — thêm cache khi chưa biết nguyên nhân chỉ là giấu vấn đề.

---

## 2. "Traffic tăng đột ngột 10 lần, xử lý thế nào?"

> Trước hết tôi xác định traffic đó là thật hay bất thường — có thể là bot, scraper hoặc retry storm từ chính client của mình. Nếu là bất thường thì rate limit và WAF giải quyết nhanh hơn scale.
>
> Nếu là traffic thật, thứ tự ứng phó ngay lúc đó: bật/tăng autoscaling cho tầng app vì app stateless scale dễ nhất; kiểm tra ngay cache hit rate vì đây là thứ giữ cho DB sống; bật rate limit để bảo vệ phần lõi; degrade các tính năng không thiết yếu như recommendation, analytics realtime để dành tài nguyên cho luồng chính; đẩy các việc chịu được trễ sang queue để xử lý async.
>
> Điểm quan trọng là scale app gần như luôn không đủ, vì nghẽn thật sẽ dịch xuống database — connection pool và write throughput. Nên sau khi ổn định, hướng xử lý dài hạn là tăng cache hit rate, thêm read replica cho read path, chuyển write không cần đồng bộ sang queue, và chỉ tính tới sharding khi đã cạn các cách trên.

---

## 3. "Service đang chạy tốt bỗng lỗi 500 hàng loạt, làm gì đầu tiên?"

> Ưu tiên số một là khôi phục dịch vụ, không phải tìm nguyên nhân gốc. Câu hỏi đầu tiên tôi hỏi là "có gì vừa thay đổi không" — deploy, config, feature flag, migration, thay đổi ở dependency. Phần rất lớn sự cố production đến từ một thay đổi vừa diễn ra, nên nếu vừa deploy thì rollback trước, điều tra sau.
>
> Nếu không có thay đổi nào từ phía mình, tôi kiểm tra dependency theo thứ tự: DB còn kết nối được không, Redis, queue, các service nội bộ, third-party. Đọc log lỗi để phân biệt lỗi tự sinh hay lỗi lan từ downstream. Xem metric hạ tầng xem có chạm giới hạn nào không — disk đầy, memory, connection pool, quota API bên ngoài.
>
> Trong lúc đó vẫn thông báo trạng thái cho các bên liên quan. Sau khi hệ thống ổn định mới làm postmortem: nguyên nhân gốc, vì sao monitoring không phát hiện sớm, và thêm gì để lần sau không lặp lại.

---

## 4. "Làm sao đảm bảo một API không bị xử lý trùng?"

> Dùng idempotency key. Client sinh một key duy nhất cho mỗi thao tác nghiệp vụ và gửi kèm header. Server lưu key đó với unique constraint ở database — không phải chỉ ở Redis, vì Redis có thể mất dữ liệu. Nếu key đã tồn tại thì trả về kết quả đã lưu của lần trước thay vì thực hiện lại.
>
> Điều cần lưu ý là phải xử lý cả trường hợp request trùng đến **đồng thời**, không chỉ tuần tự. Cách chắc chắn là insert bản ghi idempotency trong cùng transaction với thao tác nghiệp vụ — request thứ hai sẽ vi phạm unique constraint và rollback. Ngoài ra cần lưu cả trạng thái đang xử lý để request trùng biết là "đang chạy" chứ không phải "chưa từng chạy".
>
> Chi tiết hơn tôi có ghi trong [notes/idempotency](idempotency/README.md).

---

## 5. "Bug chỉ xảy ra trên production, không reproduce được ở local, làm sao debug?"

> Tôi không cố đoán mà đi tìm điểm khác biệt giữa hai môi trường, vì bug kiểu này gần như luôn nằm ở đó: quy mô dữ liệu, tính đồng thời, cấu hình, phiên bản dependency, múi giờ và locale, hoặc dữ liệu bẩn có thật trên production mà local không có.
>
> Cách làm cụ thể: thu hẹp bằng dữ liệu đã có trước — log, trace, metric quanh thời điểm lỗi, tìm correlation id của request lỗi để dựng lại toàn bộ luồng. Xác định lỗi xảy ra với mọi request hay chỉ một tập dữ liệu nhất định. Nếu nghi ngờ concurrency thì thử tái hiện bằng load test thay vì gọi tay từng request. Nếu vẫn không đủ thông tin thì thêm log có cấu trúc ở đúng nhánh nghi ngờ rồi deploy, chấp nhận một vòng lặp nữa.
>
> Nếu lỗi ảnh hưởng user thì mitigate trước bằng feature flag hoặc rollback, rồi mới điều tra tiếp — không để user chịu trận trong lúc mình debug.

---

## 6. "Bảng có 500 triệu dòng, query chậm, làm sao?"

> Trước tiên tôi kiểm tra query có dùng đúng index không bằng `EXPLAIN ANALYZE`, và xem có đang offset pagination sâu không — hai thứ này giải quyết được phần lớn trường hợp mà không cần đụng tới kiến trúc.
>
> Nếu index đã đúng mà vẫn chậm thì xét tiếp: bảng có cần giữ toàn bộ dữ liệu nóng không, hay có thể archive dữ liệu cũ sang bảng lạnh hoặc S3. Sau đó tới partitioning — thường theo thời gian với dữ liệu dạng log/đơn hàng — để mỗi query chỉ chạm một vài partition thay vì cả bảng, và để xoá dữ liệu cũ bằng `DROP PARTITION` thay vì `DELETE` hàng loạt.
>
> Nếu nghẽn ở write throughput chứ không phải read, và đã cạn các cách trên, mới tính tới sharding. Tôi không đề xuất sharding sớm vì nó làm join, transaction và vận hành phức tạp hơn hẳn.

---

## 7. "Database CPU 100%, làm gì?"

Interviewer rất thích câu này vì nó bẫy người trả lời nhảy ngay vào "scale up DB". Đó là câu trả lời tệ — nếu nguyên nhân là một query tệ, thì tăng gấp đôi CPU chỉ mua được thêm vài tuần và tốn gấp đôi tiền.

> Việc đầu tiên là tìm cái gì đang ăn CPU, không phải scale. Tôi xem ngay các query đang chạy và query tốn nhiều thời gian tích luỹ nhất — trong PostgreSQL là `pg_stat_activity` cho query đang chạy và `pg_stat_statements` cho tổng chi phí. Thường sẽ thấy một hoặc hai query chiếm phần lớn CPU.
>
> Sau đó phân loại nguyên nhân:
>
> * **Một query tệ mới xuất hiện** — thường do deploy mới, hoặc dữ liệu đã lớn tới ngưỡng khiến planner đổi plan. Chữa bằng index hoặc viết lại query.
> * **Query cũ vẫn thế nhưng bảng đã phình to** — plan cũ không còn phù hợp, cần `ANALYZE` lại, thêm index, hoặc partition.
> * **Không phải CPU tính toán mà là chờ** — lock wait, deadlock. Lúc này CPU cao đi kèm nhiều transaction đang `idle in transaction`. Chữa bằng rút ngắn transaction.
> * **Quá nhiều connection** — mỗi connection tốn CPU cho context switch. Chữa bằng connection pooler như PgBouncer.
> * **Việc nền** — autovacuum, backup, tạo index đang chạy. Cái này chỉ cần đợi hoặc dời lịch.
>
> Xử lý khẩn cấp nếu đang sập: kill query đang chạy quá lâu, bật rate limit ở tầng app, tạm tắt tính năng nặng như report/export. Rồi mới sửa gốc. Scale up chỉ là phương án khi đã xác nhận workload thật sự vượt capacity chứ không phải do query tệ.

---

## 8. "Connection pool là gì, vì sao cần?"

> Mỗi kết nối tới PostgreSQL là một process riêng ở phía server, tốn vài MB RAM và chi phí bắt tay TCP + xác thực khoảng vài chục ms. Nếu mỗi request mở một connection mới rồi đóng, phần lớn thời gian sẽ tiêu vào việc tạo kết nối chứ không phải chạy query. Tệ hơn, khi có 5000 request đồng thời thì 5000 process sẽ giết chết database bằng RAM và context switch — Postgres không được thiết kế cho hàng nghìn connection.
>
> Connection pool giữ sẵn một số kết nối và tái sử dụng, nên request chỉ mượn rồi trả. Số connection tới DB bị giới hạn ở mức DB chịu được, phần dư xếp hàng chờ ở app thay vì làm sập DB.
>
> Điểm hay bị hỏi tiếp là **pool size bao nhiêu là đúng**. Không phải càng lớn càng tốt — pool lớn hơn khả năng xử lý của DB chỉ chuyển hàng đợi từ app xuống DB và làm mọi query cùng chậm. Công thức tham khảo phổ biến là khoảng `số core CPU × 2 + số spindle disk`, thực tế thường rơi vào 10-30 connection cho mỗi instance app. Cần tính tổng trên toàn bộ instance: 20 pod × pool 50 là 1000 connection tới DB, thường đã quá tải. Với số lượng pod lớn thì nên đặt PgBouncer ở giữa.
>
> Triệu chứng pool cạn rất đặc trưng: p99 latency tăng vọt trong khi CPU của DB lại thấp — request đang chờ connection chứ không chờ query. Nguyên nhân gốc thường là connection leak, transaction mở quá lâu, hoặc gọi API bên ngoài khi đang giữ connection.

---

## 9. "Khi nào nên thêm index?"

> Không phải cứ query chậm là thêm index. Index tăng tốc read nhưng làm chậm mọi write, vì mỗi `INSERT`, `UPDATE`, `DELETE` phải cập nhật thêm cây B-tree, và index còn tốn disk cùng RAM cho buffer cache.
>
> Tôi thêm index khi: query chạy thường xuyên, `EXPLAIN ANALYZE` xác nhận đang `Seq Scan` trên bảng lớn, và cột có độ phân biệt tốt. Tôi không thêm index cho bảng nhỏ vài nghìn dòng vì DB đọc thẳng còn nhanh hơn, không thêm cho cột có ít giá trị phân biệt như boolean trừ khi phân bố rất lệch và dùng partial index, và không thêm cho query chỉ chạy một lần mỗi tháng.
>
> Trước khi thêm tôi luôn kiểm tra index hiện có đã phục vụ được chưa — nếu đã có `(A, B, C)` thì không cần tạo thêm `(A)` hay `(A, B)` vì chúng là prefix. Định kỳ tôi cũng rà `pg_stat_user_indexes` để drop index có `idx_scan = 0`, vì index không dùng vẫn đang bắt mọi write phải trả giá. Trên bảng lớn ở production thì luôn tạo bằng `CREATE INDEX CONCURRENTLY` để không khoá write.

---

## 10. "Khi nào nên dùng queue?"

> Nguyên tắc của tôi: những gì user không cần chờ để biết kết quả thì đẩy sang queue. Ví dụ đăng ký tài khoản — việc tạo user phải đồng bộ vì user cần biết thành công hay không, nhưng gửi email chào mừng thì không. Nếu gửi email đồng bộ, API phải chờ SMTP 2 giây và sẽ lỗi nếu SMTP down, dù user đã được tạo thành công.
>
> Queue giải quyết bốn việc: giảm latency của API vì trả về ngay sau khi enqueue; cách ly lỗi vì service email chết không làm hỏng luồng đăng ký; hấp thụ spike vì queue giữ lại để worker xử lý từ từ thay vì đánh sập downstream; và cho phép retry có kiểm soát với DLQ khi thất bại.
>
> Cái giá phải trả là hệ thống thành eventual consistency — user thấy "thành công" trước khi email thật sự được gửi, nên UI phải phản ánh đúng trạng thái đó. Ngoài ra phải xử lý at-least-once delivery bằng idempotent consumer, phải theo dõi queue depth và consumer lag, và debug khó hơn vì luồng bị cắt làm nhiều đoạn — cần correlation id để trace.
>
> Việc hợp với queue: gửi email/SMS/push, xử lý ảnh và video, export báo cáo, đồng bộ sang service khác, cập nhật search index, gọi webhook bên thứ ba, tính toán analytics. Việc **không** nên đưa vào queue: bất cứ thứ gì user cần kết quả ngay, và các thao tác cần đọc lại chính dữ liệu vừa ghi ở request kế tiếp.

---

## 11. "Đồng bộ hay bất đồng bộ?"

> Câu hỏi tôi tự đặt là: người gọi có cần kết quả để đi tiếp không, và nếu bước này thất bại thì có phải huỷ cả thao tác không.
>
> Đồng bộ khi cần kết quả ngay để quyết định, cần tính nhất quán mạnh, hoặc lỗi phải làm rollback toàn bộ — ví dụ trừ tiền và tạo đơn hàng phải nằm cùng một transaction. Bất đồng bộ khi thao tác chỉ là hệ quả phụ, chịu được trễ, hoặc phụ thuộc vào hệ thống bên ngoài không đáng tin.
>
> Trade-off cốt lõi là chuyển từ strong sang eventual consistency. Đổi lại độ trễ thấp hơn và khả năng chịu lỗi tốt hơn, nhưng phải chấp nhận có khoảng thời gian dữ liệu chưa đồng bộ giữa các service, phải thiết kế idempotency, và phải có cách xử lý khi bước async thất bại vĩnh viễn — thường là DLQ cộng cảnh báo cộng quy trình xử lý tay.
>
> Một cái bẫy hay gặp là dual write: ghi DB rồi publish message, nếu publish lỗi thì dữ liệu lệch. Chỗ này tôi dùng transactional outbox — ghi event vào bảng outbox trong cùng transaction với nghiệp vụ, rồi có tiến trình riêng đọc outbox và publish.

---

## 12. "Service của bạn gọi 3 service khác, một cái bị chậm thì sao?"

> Nguy hiểm nhất ở đây là cascading failure: một service chậm sẽ giữ connection và thread/goroutine của mình, dần dần làm cạn tài nguyên và kéo sập luôn service của mình, rồi lan tiếp lên service gọi mình. Một service chậm nguy hiểm hơn một service chết hẳn, vì chết thì fail nhanh còn chậm thì bào mòn tài nguyên.
>
> Lớp phòng thủ tôi áp dụng theo thứ tự:
>
> * **Timeout** — bắt buộc, và phải ngắn hơn timeout của API mình đang phục vụ. Không đặt timeout là lỗi nghiêm trọng nhất. Nên dùng deadline lan truyền qua context để tổng thời gian không vượt ngân sách chung.
> * **Retry có backoff và jitter** — nhưng chỉ retry với lỗi tạm thời và thao tác idempotent. Retry vô tội vạ tạo retry storm, biến sự cố nhỏ thành sự cố lớn. Luôn giới hạn số lần và có budget.
> * **Circuit breaker** — khi tỉ lệ lỗi vượt ngưỡng thì mở mạch, fail nhanh trong một khoảng, rồi thử lại thăm dò. Điều này bảo vệ cả mình lẫn service đang gặp sự cố.
> * **Bulkhead** — tách pool tài nguyên riêng cho từng downstream, để service chậm không nuốt hết connection dùng chung.
> * **Gọi song song** nếu ba call độc lập, để tổng thời gian bằng call chậm nhất chứ không phải tổng ba call.
> * **Graceful degradation** — nếu service đó không thiết yếu, trả về dữ liệu mặc định, dữ liệu cache cũ, hoặc bỏ hẳn phần đó khỏi response thay vì fail toàn bộ. Ví dụ trang sản phẩm vẫn hiển thị được khi service gợi ý sản phẩm chết.
>
> Nếu phần chậm đó không cần trả lời ngay thì hướng tốt nhất là chuyển hẳn sang async qua queue.

---

## 13. "10.000 request/giây thì scale thế nào?"

> Tôi bắt đầu bằng ước lượng để biết con số đó có thật sự lớn không. 10k RPS chia cho ví dụ 500 RPS mỗi instance thì cần khoảng 20 instance — con số hoàn toàn xử lý được, không cần kiến trúc đặc biệt. Sau đó tôi hỏi tỉ lệ read/write, vì hệ read-heavy và write-heavy scale theo hai hướng khác nhau.
>
> Với read-heavy, thứ tự là: CDN cho tài nguyên tĩnh và response cache được; cache tầng ứng dụng bằng Redis để đưa phần lớn read khỏi DB; app stateless đứng sau load balancer để scale ngang; read replica cho phần read còn lại phải xuống DB. Nếu đạt hit rate 95% thì 10k RPS chỉ còn 500 RPS xuống DB, hoàn toàn trong tầm.
>
> Với write-heavy thì cache không giúp được. Hướng là gom write theo batch, đẩy phần chịu được trễ sang queue để làm phẳng spike, tối ưu để mỗi transaction thật ngắn, rồi mới tới partitioning và cuối cùng là sharding theo key phân phối đều.
>
> Điều tôi luôn nói thêm: scale tầng app là phần dễ nhất và thường không phải nút thắt thật. Nghẽn sẽ dịch xuống database, connection pool và các external dependency — nên tôi cần load test để biết đâu là trần thật, thay vì cứ thêm pod.

---

## 14. Cách trả lời nhóm câu hỏi tình huống

Khung chung áp dụng được cho hầu hết câu mở:

1. **Làm rõ** — hỏi lại 2-3 câu để thu hẹp đề bài. Đây là bước hay bị bỏ qua nhất và cũng dễ ghi điểm nhất.
2. **Phân loại** — chia không gian vấn đề thành các tầng/nhóm rồi loại trừ dần, thay vì bắn ngẫu nhiên từng giả thuyết.
3. **Đo** — nói rõ bạn sẽ nhìn metric/log/trace nào để xác nhận, không đoán suông.
4. **Xử lý theo thứ tự** — mitigate trước, root cause sau; sửa nguyên nhân trước, tối ưu sau.
5. **Trade-off** — mỗi giải pháp nêu kèm cái giá phải trả.
6. **Phòng ngừa** — kết bằng alert/test/monitoring để lần sau phát hiện sớm.

Vài câu nên chuẩn bị sẵn theo cùng khung này:

* Memory của service tăng dần rồi OOM, xử lý thế nào?
* Message trong queue bị xử lý hai lần thì sao?
* Hai request cùng trừ tiền một tài khoản, làm sao tránh âm số dư? → đã viết ở [Database Middle Roadmap - Race condition và isolation anomalies](database-middle-roadmap.md)
* Deploy version mới mà cần đổi schema database, làm sao không downtime?
* Third-party API mà bạn phụ thuộc bị down, hệ thống của bạn ra sao?
* Làm sao biết hệ thống đang khoẻ mà không cần user báo lỗi?

---

## Bảng tra nhanh: câu hỏi kinh điển nằm ở đâu

Các câu hay gặp nhưng nội dung nằm ở file chuyên đề, để đây cho dễ tìm khi ôn gấp.

| Câu hỏi | Đọc ở |
|---|---|
| API chậm xử lý thế nào | Mục 1 file này |
| Traffic tăng 10 lần / 10.000 RPS | Mục 2 và 13 file này |
| DB CPU 100% | Mục 7 file này |
| Connection pool là gì, pool size bao nhiêu | Mục 8 file này |
| Khi nào thêm index | Mục 9 file này |
| Khi nào dùng queue | Mục 10 file này |
| Đồng bộ hay bất đồng bộ | Mục 11 file này |
| Service downstream chậm | Mục 12 file này |
| Tại sao dùng Redis, giảm tải DB thế nào | [Redis Notes](redis-middle-notes.md) mục 8 |
| Redis down thì hệ thống ra sao | [Redis Notes](redis-middle-notes.md) mục 8 |
| Cache và DB lệch nhau, invalidate thế nào | [Redis Notes](redis-middle-notes.md) mục 8 |
| Tại sao không cache tất cả | [Redis Notes](redis-middle-notes.md) mục 8 |
| Cache cái gì, TTL bao nhiêu | [Redis Notes](redis-middle-notes.md) mục 8, bảng ví dụ |
| Penetration vs breakdown vs avalanche | [Redis Notes](redis-middle-notes.md) mục 4 |
| Redis có thay được DB không | [Redis Notes](redis-middle-notes.md) mục 8 |
| Compound index, leftmost prefix, query thiếu cột giữa | [Database Roadmap](database-middle-roadmap.md) mục 2.1 |
| Khi nào dùng compound index, một compound hay nhiều single | [Database Roadmap](database-middle-roadmap.md) mục 2.2 |
| Race condition trừ tiền/trừ kho, lost update, write skew | [Database Roadmap](database-middle-roadmap.md) mục 6.1 |
| `SELECT FOR UPDATE` vs optimistic lock | [Database Roadmap](database-middle-roadmap.md) mục 6.1 |
| Isolation level chặn được anomaly nào | [Database Roadmap](database-middle-roadmap.md) mục 6.1 |
| OFFSET/LIMIT có vấn đề gì, keyset pagination | [Database Roadmap](database-middle-roadmap.md) mục 7 |
| Read replica, replication lag | [Database Roadmap](database-middle-roadmap.md) mục 9 |
| Partitioning vs sharding | [Database Roadmap](database-middle-roadmap.md) mục 8 và 14 |
| MongoDB vs PostgreSQL chọn cái nào | [Database Roadmap](database-middle-roadmap.md) mục 15 |
| REST hay gRPC, khi nào dùng cái nào | [gRPC Notes](grpc-middle-notes.md) mục 2 |
| Message bị xử lý hai lần | [Idempotency](idempotency/README.md), [EDA Notes](event-driven-architecture.md) |
| Dual write, transactional outbox | [EDA Notes](event-driven-architecture.md) |
| At-least-once, ordering, DLQ, poison message | [RabbitMQ Notes](rabbitmq-middle-notes.md) |
| JWT hết hạn, logout mọi thiết bị | [JWT & Session Notes](jwt-session-middle-notes.md) |
| Rate limit chọn thuật toán nào | [Redis Notes](redis-middle-notes.md) mục 1, [Production Backend Concepts](production-backend-concepts.md) |

---

## Liên kết

* [Database Middle Roadmap](database-middle-roadmap.md) — compound index, execution plan, lock, connection pool, partitioning
* [Redis Middle Notes](redis-middle-notes.md) — tại sao dùng Redis, giảm tải DB, cache strategies, edge case
* [Production Backend Concepts](production-backend-concepts.md) — failure mode theo từng topic
* [Scale System Questions](scale_system_question.md) — toàn bộ đường đi của request và các tầng scale
* [Production Scale Metrics](production-scale-metrics.md) — cách nói số liệu production đúng mức
