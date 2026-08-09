# Outbox: Postgres (`FOR UPDATE SKIP LOCKED`) vs MongoDB (lease trong document)

So sánh 2 implement outbox thật:

| | File |
|---|---|
| Postgres / Go | `backend/internal/store/outbox.go` |
| MongoDB / Node | `src/services/outbox/{publisher,service,model}.ts` |

Câu hỏi gốc: *"SQL có lock sẵn, còn MongoDB thì phải tự implement?"* — Đúng về bản chất, nhưng
lý do sâu hơn "Mongo thiếu tính năng", và điểm mạnh đó hẹp hơn tưởng tượng.

---

## 0. Nhắc lại outbox giải bài toán gì

Bài toán **dual-write**: ghi DB rồi gọi ES/broker là 2 hệ thống, không có transaction chung.
Ghi DB xong mà crash trước khi publish → lệch vĩnh viễn. (Chính là bài học phản diện
`WRITE_MODE=dual` trong project này.)

Outbox: ghi `products` + ghi `outbox` **trong cùng 1 transaction**. Sau đó một worker đọc `outbox`
và publish. Mọi thứ dưới đây chỉ bàn về **nửa sau** — worker lấy row ra như thế nào cho an toàn
khi chạy nhiều instance.

---

## 1. Cốt lõi: khóa sống trong transaction vs. khóa sống trong dữ liệu

### Postgres — khóa do lock manager giữ

```go
FOR UPDATE SKIP LOCKED   // outbox.go:33
```

- Worker khác gặp row đang khóa thì **bỏ qua** (skip) thay vì chờ → nhiều worker song song
  không giẫm chân.
- Worker chết → connection đứt → tx rollback → **khóa tự nhả tức thì**. Không timeout,
  không dọn dẹp.
- `UPDATE ... processed_at` nằm **cùng tx** với lock (`outbox.go:56`) → "đánh dấu xong" và
  "nhả khóa" là một hành động nguyên tử. Không có khe hở giữa hai bước.

### MongoDB — khóa phải vật chất hóa thành dữ liệu

Mongo **không có** `SKIP LOCKED`. Mongo *có* transaction (replica set), nhưng WiredTiger xử lý
tranh chấp bằng `WriteConflict` → **abort**, không có ngữ nghĩa "bỏ qua doc đang bị khóa".
Và kể cả có, cũng **không nên** giữ tx mở suốt lúc publish sang RabbitMQ —
`transactionLifetimeLimitSeconds` mặc định 60s, giữ snapshot lâu là tự bắn vào chân.

Nên khóa được lưu thành field: `claim_id` + `lease_until` + `status: PROCESSING`
(`outbox.model.ts:100-101`). Đây chính là **visibility timeout** của SQS — không phải hack riêng
của Mongo, mà là pattern chuẩn khi khóa phải sống lâu hơn transaction.

Primitive thay thế mà Mongo cấp: **mỗi update trên 1 document là atomic**. Code khai thác đúng
chỗ này — `find` lấy candidate rồi `updateMany` **lặp lại y nguyên guard filter**
(`outbox.service.ts:141`). `find` + `updateMany` không atomic theo batch, nhưng mỗi doc là một CAS:
pod khác cướp mất ở giữa thì doc đó fail guard và rơi khỏi batch. An toàn.

---

## 2. Bảng so sánh

| | Postgres (`outbox.go`) | Mongo (`outbox.publisher.ts`) |
|---|---|---|
| Cơ chế khóa | Lock manager của DB | Lease trong document (`claim_id` + `lease_until` 60s) |
| Loại trừ lẫn nhau | `SKIP LOCKED` | Guard filter lặp lại trong `updateMany` |
| Nhả khóa khi crash | Tức thì (rollback) | Sau khi lease hết hạn (≤ 60s) |
| Nhả khóa khi shutdown sạch | Tức thì | `releaseOutboxClaim()` chủ động (`publisher.ts:205`) |
| Claim + settle | **1 transaction** | **2 write tách rời** |
| Số round-trip để lấy batch | 1 (`RETURNING`) | 3 (`find` → `updateMany` → `find`) |
| Retry | Vô hạn, không backoff | `retry_count`, backoff mũ, `MAX_RETRY=10` → `FAILED` |
| Dọn row cũ | Không có | TTL index 30 ngày (`outbox.model.ts:119`) |
| Backpressure khi đích chết | Không có → hot loop | `isRabbitReady()` + poll interval (`publisher.ts:132`) |
| Chống trùng ở consumer | ES external `version` | `event_id` = uuid, consumer tự dedup |

---

## 3. Cái giá thật sự của mỗi bên

### Postgres trả giá bằng transaction dài

Toàn bộ `handle()` (gọi ES) chạy **bên trong** tx đang giữ lock (`outbox.go:52-60`):

- Connection pool bị giữ theo tốc độ của **ES**, không theo tốc độ của DB.
- Long-running tx chặn `VACUUM` dọn dead tuple → bloat bảng `outbox` (bảng ghi/update liên tục,
  rất nhạy với chuyện này).
- ES treo 30s → tx treo 30s. Một service ngoài tầm kiểm soát đang giữ tài nguyên DB.

### Mongo trả giá bằng khe hở giữa publish và settle

Publish xong, chưa kịp `markOutboxEventsPublished` mà pod chết → lease hết hạn → pod khác publish
lại. Duplicate thật, và đó là lý do bắt buộc phải có `event_id` cho consumer dedup
(comment ở `publisher.ts:56-59` đã ghi nhận đúng). Postgres không có khe hở này vì mark nằm cùng
tx với lock.

---

## 4. Vậy đây có phải điểm mạnh của SQL không?

**Có — nhưng phạm vi hẹp, và hẹp đúng vào tình huống outbox này.**

### Đúng là điểm mạnh thật

Cái Postgres cho miễn phí không phải "có lock", mà là **lock có vòng đời do DB quản lý**:

- **Không cần chọn `LEASE_MS`.** Chọn 60s là đoán mò: quá ngắn → publish chậm bị cướp row →
  duplicate; quá dài → pod chết làm row đứng im 5 phút. Postgres không có tham số này để chọn sai.
- **Không cần dọn khóa mồ côi.** Connection đứt là xong, kể cả `kill -9`.
- **Claim và settle nguyên tử với nhau** → không có khe hở "đã publish, chưa mark".
- Không cần field `claim_id`/`lease_until`, không cần guard filter, không cần logic release lúc
  shutdown. Bên Mongo, phần lớn code trong `outbox.service.ts` tồn tại chỉ để mô phỏng thứ
  Postgres làm sẵn.
- **`RETURNING`**: Postgres claim + lấy dữ liệu trong 1 câu lệnh. Mongo `updateMany` không trả về
  doc đã sửa → buộc phải query lại theo `claim_id` (`outbox.service.ts:152`). Tiện ích nhỏ nhưng
  đúng loại "SQL cho không".

Ít code tự viết = ít chỗ sai. Và thực tế **đã có 1 chỗ sai** (mục 6) — đúng loại bug mà
`FOR UPDATE` không cho phép tồn tại.

### Nhưng nó tan biến khi công việc là I/O ra ngoài

Lock của Postgres chỉ sống trong transaction. Muốn hưởng lợi, phải giữ tx mở suốt lúc gọi
ES/RabbitMQ — đúng như `outbox.go` đang làm, với cái giá ở mục 3.

Nên nhiều hệ thống Postgres production **vẫn tự viết lease**, y hệt bản Mongo:

```sql
UPDATE outbox
SET locked_until = now() + interval '1 minute', claim_id = $1
WHERE id IN (
  SELECT id FROM outbox
  WHERE processed_at IS NULL
    AND (locked_until IS NULL OR locked_until < now())
  ORDER BY id
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
RETURNING *;
```

Commit ngay → publish ngoài tx → mark. Lúc này Postgres và Mongo **cùng một pattern, cùng một tập
bug tiềm ẩn** — `SKIP LOCKED` tụt xuống thành tiện ích nhỏ trong đúng một câu lệnh.

### Kết luận

| Tình huống | Kết quả |
|---|---|
| Công việc nằm gọn trong DB (update bảng khác, state machine, counter) | **Postgres thắng rõ** |
| Công việc là I/O ra ngoài (ES, RabbitMQ, HTTP) | **Hòa** — cả hai đều phải lease |

Outbox → ES/Rabbit thuộc hàng dưới. Bản `outbox.go` hiện tại chọn được cách "giữ tx" là nhờ nó là
project học: 1 worker, ES local, batch nhỏ. Lên production nhiều pod + ES chậm thì sẽ phải viết
lại thành lease — tức là **hội tụ về đúng bản Mongo**.

> **Điểm mạnh thật sự của SQL ở đây không phải `SKIP LOCKED`**, mà là ghi `products` + `outbox`
> trong cùng một transaction ACID. Đó mới là nền móng của outbox pattern, và là thứ Mongo phải mua
> bằng multi-document transaction trên replica set (chậm hơn, ràng buộc hạ tầng).
> `SKIP LOCKED` chỉ là tiện nghi ở tầng consumer.

---

## 5. Những ý còn thiếu (bổ sung)

### 5.1 Thứ tự (ordering) — **cả hai đều không đảm bảo**

- **Postgres**: `handle()` fail thì `continue` (`outbox.go:54`), row sau vẫn xử lý → row lỗi được
  publish *sau* row mới hơn.
- **Mongo**: `Promise.allSettled` + backoff → row fail quay lại hàng đợi muộn hơn hẳn.

Nên đừng bao giờ dựa vào thứ tự outbox. Cách xử lý đúng là **idempotency ở đích**:

- ES: `external version` — bản ghi version cũ đến sau bị **409 và bỏ qua**. Bản Go làm đúng.
- RabbitMQ consumer: phải tự so `updated_at`/version, hoặc upsert theo khóa. `event_id` chỉ chống
  *trùng*, không chống *ngược thứ tự*.

Muốn thứ tự nghiêm ngặt theo từng aggregate → phải serialize theo key (partition theo
`aggregate_id`, mỗi key 1 worker), không phải thứ tự toàn cục.

### 5.2 Không có exactly-once

Cả hai đều là **at-least-once**. "Exactly-once" chỉ đạt được ở mức *effectively-once* nhờ đích
idempotent. Đây là tính chất của pattern, không phải khiếm khuyết của implement.

### 5.3 Index — chỗ Postgres đang thiếu

Mongo có sẵn compound index khớp đúng claim filter (`outbox.model.ts:116`).
Postgres query `WHERE processed_at IS NULL ORDER BY id` — khi bảng phình to mà không có
**partial index** thì đây là seq scan:

```sql
CREATE INDEX outbox_pending_idx ON outbox (id) WHERE processed_at IS NULL;
```

Partial index còn có lợi thế: row đã xử lý **rơi ra khỏi index**, nên index luôn nhỏ bằng đúng
backlog.

### 5.4 Dọn dữ liệu — chỗ Postgres cũng thiếu

Bản Go set `processed_at` rồi **để đó mãi mãi**. Bảng chỉ có tăng.
Mongo có TTL index 30 ngày. Bên Postgres cần một trong hai:

- cron `DELETE FROM outbox WHERE processed_at < now() - interval '7 days'` (nhớ `LIMIT` theo batch,
  đừng xóa 10 triệu row trong 1 tx), hoặc
- partition theo ngày rồi `DROP PARTITION` — rẻ hơn `DELETE` rất nhiều.

### 5.5 Gotcha của `SKIP LOCKED`

- **Batch trả về ít hơn `LIMIT` không có nghĩa là hết việc.** Row bị worker khác giữ đã bị skip.
  Logic kiểu "batch ngắn → ngủ" (`publisher.ts:158` bên Mongo) sẽ ngủ nhầm khi nhiều worker chạy
  song song. Với nhiều worker, nên dựa vào `pending count` chứ không dựa vào độ dài batch.
- `FOR UPDATE` không dùng được với `GROUP BY`, `DISTINCT`, window function.
- Nếu có JOIN, `FOR UPDATE` khóa row của **mọi** bảng trong query → phải viết
  `FOR UPDATE OF outbox`.

### 5.6 Tương quan `BATCH_SIZE` ↔ `LEASE_MS` (bên Mongo)

`BATCH_SIZE=100`, `LEASE_MS=60s` → phải publish xong 100 message trong 60s, nếu không lease hết
**giữa lúc đang publish** và pod khác cướp row → duplicate ngay cả khi không ai chết.

Quy tắc: `LEASE_MS` ≥ vài lần thời gian xử lý batch tệ nhất. Hoặc chủ động gia hạn lease
(heartbeat) khi batch chạy lâu. Postgres không có class lỗi này.

### 5.7 Backpressure

Bên Mongo có `isRabbitReady()` gác trước khi claim (`publisher.ts:132`) — Rabbit chết thì không
claim gì cả, không đốt row nào vào retry_count.

Bên Go **không có gì tương đương**: ES chết → mỗi vòng loop vẫn claim → `handle()` fail toàn bộ →
lặp lại ngay lập tức. Vừa hot-loop vừa spam log. Cần: ping ES trước, hoặc backoff khi cả batch fail.

### 5.8 Con đường thứ 3: CDC (không polling, không lock)

Cả hai DB đều có cơ chế đọc log thay đổi, khi đó **không cần lock, không cần lease, không cần
polling**:

- Postgres: **logical decoding / WAL** (Debezium, `wal2json`) — đọc thẳng WAL của bảng `outbox`.
- Mongo: **Change Streams** (oplog) — `watch()` trên collection `outbox`.

Đổi lại: thêm hạ tầng (connector, offset store), và vẫn là at-least-once. Đáng cân nhắc khi
throughput cao hoặc muốn bỏ hẳn độ trễ polling. Cả hai bản hiện tại đều là polling — hoàn toàn hợp
lý ở quy mô nhỏ.

---

## 6. Chỗ đáng vá

### 6.1 Mongo — settle không guard `claim_id` ⚠️

`markOutboxEventsPublished` / `markOutboxEventsFailed` chỉ filter theo `uuid`
(`outbox.service.ts:170`, `:214`).

Kịch bản hỏng:

1. Pod A claim row X, bắt đầu publish.
2. Pod A bị GC pause / publish chậm > 60s → lease hết hạn.
3. Pod B claim row X, publish, mark published.
4. Pod A tỉnh dậy, mark published → **ghi đè**, đồng thời **xóa `claim_id`/`lease_until` của B**.

Row bị publish 2 lần và claim của B bị phá.

**Sửa**: truyền `claimId` xuống và thêm vào filter —

```ts
{ uuid: { $in: uuids }, claim_id: claimId }
```

Update nào không khớp nghĩa là mình đã mất quyền sở hữu → bỏ qua. Đây đúng là thứ Postgres cho
miễn phí và Mongo bắt viết tay.

### 6.2 Go — không có retry_count / backoff / DLQ

`handle()` fail thì `continue` (`outbox.go:54`). Một row "độc" (payload sai, ES từ chối vĩnh viễn)
sẽ được thử lại **mỗi vòng loop mãi mãi**, và luôn nằm đầu batch do `ORDER BY id`. Nó không chặn
row khác, nhưng cũng không bao giờ bị park.

Bên Mongo có `MAX_RETRY` + trạng thái `FAILED` + `last_error` xử lý đúng chuyện này. Về mặt vận
hành, **bản Mongo trưởng thành hơn bản Go**.

### 6.3 Go — `done` trả về có thể sai sự thật

Trong `outbox.go:56-58`, nếu `UPDATE ... processed_at` fail giữa batch thì `return done, err` →
`defer tx.Rollback` **hủy luôn mọi mark trước đó**. Hàm báo "đã xử lý N row" nhưng thực tế 0 row
được đánh dấu. Không mất dữ liệu (at-least-once, vòng sau chạy lại), nhưng metric/log sai.
Nên trả `0` khi rollback.

---

## 7. Checklist rút ra

- [ ] Postgres: thêm partial index `WHERE processed_at IS NULL`.
- [ ] Postgres: cron dọn row đã processed (hoặc partition).
- [ ] Postgres: backoff khi cả batch fail, đừng hot-loop lúc ES chết.
- [ ] Postgres: cân nhắc đổi sang lease + commit sớm nếu chạy nhiều worker / ES chậm.
- [ ] Mongo: thêm guard `claim_id` vào cả 2 hàm settle.
- [ ] Mongo: đảm bảo `LEASE_MS` > thời gian publish 1 batch tệ nhất, hoặc heartbeat gia hạn.
- [ ] Cả hai: consumer/đích **bắt buộc** idempotent — không dựa vào thứ tự outbox.
