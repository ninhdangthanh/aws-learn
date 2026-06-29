# MongoDB cho hệ thống F&B - roadmap thực chiến cho Backend 3 YOE

Tài liệu này tập trung vào các vấn đề MongoDB hay gặp khi làm backend F&B/POS/order/kitchen/payment ở mức junior lên middle. Mục tiêu không phải học mọi edge case lớn như sharding, mà là hiểu đúng các điểm dễ làm sai trong production: document design, index, aggregation, transaction, read/write concern, migration và dữ liệu realtime.

Nguyên tắc học:

- Ưu tiên case xuất hiện trong CRUD, reporting, order flow, POS, kitchen display, payment, audit log.
- Luôn hỏi: query này đọc theo pattern nào, ghi có thường xuyên không, document có lớn dần không, có cần consistency ngay không.
- Không over-engineer. Sharding chỉ cần biết khái niệm; chưa cần học sâu shard key edge cases nếu chưa vận hành cluster sharded thật.

---

## 1. BSON size overhead: field name cũng tốn storage

MongoDB lưu document bằng BSON. BSON không chỉ lưu value, mà còn lưu cả tên field trong từng document.

Ví dụ:

```json
{
  "customerFirstName": "An",
  "customerPhoneNumber": "090..."
}
```

Nếu collection có hàng chục hoặc hàng trăm triệu document, field name dài và lặp lại nhiều sẽ làm tăng:

- Disk usage: tốn dung lượng lưu trữ.
- Cache pressure: cùng một lượng RAM cache được ít document hơn.
- Network payload: response lớn hơn nếu trả nhiều document.
- Backup/restore time: dữ liệu phình to thì vận hành chậm hơn.

Không có nghĩa là phải đặt field kiểu `fn`, `pn` khó đọc. Cách tốt hơn là đặt tên vừa đủ rõ:

```json
{
  "customerName": "An",
  "phone": "090...",
  "storeId": "s_001",
  "createdAt": "2026-06-29T10:00:00.000Z"
}
```

Case F&B:

- `order.items.productDisplayName` có thể ổn vì cần snapshot tên món tại thời điểm order.
- Nhưng `order.items.productDescriptionForCustomerFrontendLongText` là dấu hiệu field name quá dài.
- Các field lặp trong item như `quantity`, `price`, `note` nên đặt rõ nhưng gọn: `qty`, `unitPrice`, `note` nếu team thống nhất convention.

Điểm middle cần nhớ:

- Tối ưu field name chỉ đáng quan tâm khi collection rất lớn hoặc document rất nhiều field.
- Đừng hy sinh readability quá sớm.
- Nếu payload API lớn, dùng projection trước khi nghĩ tới đổi tên field.

---

## 2. TTL index không xóa ngay lập tức

TTL index dùng để tự động xóa document sau một thời điểm hoặc sau một khoảng thời gian.

Ví dụ:

```js
db.sessions.createIndex(
  { expireAt: 1 },
  { expireAfterSeconds: 0 }
)
```

TTL không chạy realtime. MongoDB có background job quét định kỳ, nên document hết hạn có thể vẫn tồn tại thêm một lúc.

Case F&B nên dùng TTL:

- OTP login cho nhân viên.
- Session tạm cho POS terminal.
- Temporary cart hoặc draft order.
- Idempotency key cho payment request.
- Short-lived lock hoặc reservation tạm.

Không nên dùng TTL nếu nghiệp vụ yêu cầu xóa chính xác tới từng giây.

Ví dụ sai:

```js
// Sai nếu business bắt buộc voucher hết hạn đúng tuyệt đối tại 12:00:00
db.voucherLocks.createIndex({ expireAt: 1 }, { expireAfterSeconds: 0 })
```

Cách đúng hơn:

- Khi đọc hoặc confirm nghiệp vụ, vẫn check `expireAt > now`.
- TTL chỉ là cleanup mechanism, không phải business guard.

```js
db.voucherLocks.findOne({
  _id: lockId,
  expireAt: { $gt: new Date() }
})
```

Điểm middle cần nhớ:

- TTL index phải đặt trên field kiểu Date.
- TTL index là single-field index trong use case cơ bản.
- TTL deletion tạo write load, nên tránh tạo TTL cho lượng document hết hạn cùng lúc quá lớn nếu chưa đo tải.

---

## 3. Aggregation memory limit và `allowDiskUse`

Aggregation pipeline rất tiện, nhưng các stage như `$group`, `$sort`, `$lookup`, `$facet` có thể dùng nhiều memory.

Ví dụ pipeline dễ nặng:

```js
db.orders.aggregate([
  { $match: { storeId, createdAt: { $gte: from, $lt: to } } },
  { $unwind: "$items" },
  { $group: { _id: "$items.productId", qty: { $sum: "$items.qty" } } },
  { $sort: { qty: -1 } }
])
```

Khi data ít, query chạy ổn. Khi data tăng, `$group` và `$sort` có thể vượt memory limit hoặc spill ra disk nếu bật:

```js
{ allowDiskUse: true }
```

`allowDiskUse` giúp query không fail ngay, nhưng không biến query nặng thành query nhanh. Nó thường là dấu hiệu cần tối ưu:

- `$match` càng sớm càng tốt.
- Chỉ project field cần dùng.
- Index phải support `$match` và `$sort` nếu có thể.
- Report nặng nên chạy background job hoặc precompute read model.

Case F&B:

- Top món bán chạy theo ngày.
- Doanh thu theo cashier/shift.
- Báo cáo order bị hủy theo reason.
- Thống kê thời gian chế biến trung bình của kitchen.

Với các report được xem nhiều, nên cân nhắc collection tổng hợp:

```json
{
  "storeId": "s_001",
  "date": "2026-06-29",
  "productId": "p_001",
  "soldQty": 128,
  "grossRevenue": 2560000
}
```

Điểm middle cần nhớ:

- Aggregation không thay thế data modeling.
- Report realtime tuyệt đối thường đắt. Hỏi lại business có chấp nhận delay 1-5 phút không.
- Luôn chạy `explain("executionStats")` cho query quan trọng.

---

## 4. Unbounded array: mảng tăng vô hạn là anti-pattern

Một document có giới hạn 16MB. Nếu nhúng một mảng có thể tăng mãi, document sẽ ngày càng nặng và chậm update.

Ví dụ không nên:

```json
{
  "_id": "table_01",
  "orders": [
    "...all orders today..."
  ]
}
```

Hoặc:

```json
{
  "_id": "customer_01",
  "orderHistory": [
    "...all historical orders..."
  ]
}
```

Vấn đề:

- Document lớn dần.
- Update bằng `$push` ngày càng tốn chi phí.
- Dễ chạm giới hạn 16MB.
- Nếu nhiều request update cùng document, dễ bị write contention.

Cách thiết kế tốt hơn:

1. Separate collection:

```json
{
  "_id": "order_001",
  "customerId": "customer_01",
  "tableId": "table_01",
  "storeId": "s_001",
  "status": "OPEN",
  "createdAt": "2026-06-29T10:00:00.000Z"
}
```

2. Bucket pattern cho dữ liệu log/time-series:

```json
{
  "_id": "kitchen_events_s001_2026_06_29_10",
  "storeId": "s_001",
  "hour": "2026-06-29T10:00:00.000Z",
  "events": [
    { "orderId": "o1", "type": "START_COOKING", "at": "..." }
  ],
  "count": 1
}
```

3. Capped history:

```js
db.tables.updateOne(
  { _id: tableId },
  {
    $push: {
      recentOrderIds: {
        $each: [orderId],
        $slice: -20
      }
    }
  }
)
```

Điểm middle cần nhớ:

- Embed khi dữ liệu nhỏ, đọc chung thường xuyên, và không tăng vô hạn.
- Reference khi dữ liệu lớn, tăng liên tục, hoặc có lifecycle riêng.
- Mảng embedded nên có giới hạn rõ ràng.

---

## 5. Document relocation: document lớn dần sẽ làm update đắt hơn

Khi document tăng kích thước nhiều lần, MongoDB có thể phải di chuyển document trong storage để có đủ chỗ lưu. Việc này làm tăng I/O và chi phí update index pointer.

Các pattern dễ gây vấn đề:

```js
// Order document bị append log liên tục
db.orders.updateOne(
  { _id: orderId },
  { $push: { statusLogs: newLog } }
)
```

```js
// Customer document bị nhồi toàn bộ lịch sử mua hàng
db.customers.updateOne(
  { _id: customerId },
  { $push: { orderHistory: orderSnapshot } }
)
```

Case F&B:

- Order status log: nên tách `order_events`.
- Payment retry log: nên tách `payment_attempts`.
- Kitchen event stream: nên tách hoặc bucket.
- Audit log thay đổi giá món: nên tách collection.

Thiết kế ổn hơn:

```json
{
  "_id": "event_001",
  "orderId": "order_001",
  "storeId": "s_001",
  "type": "ORDER_STATUS_CHANGED",
  "from": "CONFIRMED",
  "to": "COOKING",
  "createdAt": "2026-06-29T10:05:00.000Z"
}
```

Điểm middle cần nhớ:

- Log/event/history thường không nên nhúng vô hạn vào entity chính.
- Entity chính chỉ giữ trạng thái hiện tại và vài snapshot quan trọng.
- Lịch sử chi tiết đưa sang collection riêng để query theo thời gian.

---

## 6. Transaction trong MongoDB: dùng được, nhưng đừng lạm dụng

MongoDB mạnh nhất ở atomic update trên một document. Multi-document transaction có hỗ trợ, nhưng đắt hơn về latency, lock duration và replication overhead.

Nên dùng transaction khi nghiệp vụ thật sự cần all-or-nothing giữa nhiều document.

Ví dụ có thể cần transaction:

- Tạo order và tạo payment intent nội bộ cùng lúc.
- Chuyển bàn: update table cũ, table mới, order active.
- Hoàn tiền: update payment, order, ledger entry.

Không nên dùng transaction chỉ vì muốn code nhìn giống SQL.

Ví dụ có thể tránh transaction bằng single-document atomic update:

```js
db.orders.updateOne(
  {
    _id: orderId,
    status: "CONFIRMED"
  },
  {
    $set: { status: "COOKING", cookingStartedAt: new Date() }
  }
)
```

Nếu `matchedCount = 0`, nghĩa là order không còn ở trạng thái hợp lệ để chuyển.

Pattern thay thế transaction:

- Single-document aggregate: gom dữ liệu cần atomic vào cùng document nếu bounded.
- Optimistic concurrency: dùng `version`.
- Idempotency key: chống request payment/order bị retry tạo trùng.
- Outbox pattern: ghi event trong DB rồi worker publish ra message broker.

Ví dụ optimistic concurrency:

```js
db.orders.updateOne(
  { _id: orderId, version: currentVersion },
  {
    $set: { status: "PAID" },
    $inc: { version: 1 }
  }
)
```

Điểm middle cần nhớ:

- Transaction là công cụ, không phải default.
- Nếu transaction chạy lâu, nguy cơ conflict và timeout tăng.
- Với payment/order, idempotency thường quan trọng hơn transaction dài.

---

## 7. `$lookup` không phải JOIN miễn phí như SQL

`$lookup` giúp join giữa collections, nhưng MongoDB không tối ưu join như RDBMS. Dùng nhiều `$lookup` trong API realtime có thể làm latency tăng mạnh.

Ví dụ dễ nặng:

```js
db.orders.aggregate([
  { $match: { storeId, status: "OPEN" } },
  {
    $lookup: {
      from: "customers",
      localField: "customerId",
      foreignField: "_id",
      as: "customer"
    }
  },
  {
    $lookup: {
      from: "payments",
      localField: "_id",
      foreignField: "orderId",
      as: "payments"
    }
  }
])
```

Cách nghĩ tốt hơn:

- API order list cần field nào thì denormalize snapshot field đó vào order.
- Dữ liệu thay đổi ít như `customerName`, `tableName`, `storeName` có thể lưu snapshot.
- Dữ liệu nhạy cảm thay đổi thường xuyên thì reference và query riêng.

Ví dụ order snapshot:

```json
{
  "_id": "order_001",
  "storeId": "s_001",
  "customerId": "c_001",
  "customerName": "An",
  "tableId": "t_01",
  "tableName": "A1",
  "status": "OPEN",
  "total": 250000
}
```

Khi nào `$lookup` vẫn ổn:

- Admin report không realtime.
- Data set nhỏ sau `$match`.
- Foreign collection có index đúng trên `foreignField`.
- Pipeline đã được kiểm tra bằng `explain`.

Điểm middle cần nhớ:

- Denormalization trong MongoDB là thiết kế có chủ đích, không phải copy bừa.
- Snapshot field cần có rule: field nào update theo source, field nào giữ lịch sử tại thời điểm order.
- `$lookup` nên đứng sau `$match` càng lọc mạnh càng tốt.

---

## 8. Monotonic key và hotspot: không chỉ là chuyện sharding

Phần sharding chuyên sâu tạm bỏ qua. Nhưng vẫn cần hiểu vấn đề key tăng tuần tự vì nó ảnh hưởng index, write pattern và pagination.

Monotonic key là key tăng đều theo thời gian, ví dụ:

- `createdAt`
- auto-increment number
- `_id` ObjectId ở mức gần đúng theo thời gian

Với workload ghi cực lớn vào cùng một index range, phần cuối của index có thể trở thành điểm nóng.

Trong hệ F&B thông thường, đây chưa phải vấn đề đầu tiên cần lo. Nhưng cần biết để không thiết kế mọi thứ dựa vào một counter global:

```js
db.counters.updateOne(
  { _id: "global_order_no" },
  { $inc: { value: 1 } }
)
```

Thiết kế tốt hơn cho order number:

- Counter theo store + ngày.
- Không dùng counter global nếu không có yêu cầu nghiệp vụ.
- Nếu chỉ cần unique ID, dùng ObjectId/UUID.

Ví dụ:

```json
{
  "_id": "s001_20260629",
  "storeId": "s001",
  "date": "2026-06-29",
  "seq": 128
}
```

Điểm middle cần nhớ:

- Sequence đẹp cho con người đọc không nên là primary design cho database scale.
- Nếu cần order code như `A001`, scope nó theo store/shift/day.
- Đừng dùng một document counter cho toàn hệ thống nếu traffic cao.

---

## 9. ObjectId: tận dụng được, nhưng đừng hiểu quá tay

`ObjectId` chứa timestamp nên `_id` thường sortable theo thời gian tạo.

Có thể dùng cho cursor pagination:

```js
db.orders.find({
  storeId,
  _id: { $lt: lastId }
})
.sort({ _id: -1 })
.limit(20)
```

Có thể tạo ObjectId từ timestamp để query theo khoảng thời gian trong một số trường hợp:

```js
db.orders.find({
  _id: {
    $gte: ObjectId.createFromTime(fromUnix),
    $lt: ObjectId.createFromTime(toUnix)
  }
})
```

Nhưng trong business app, vẫn nên có `createdAt` rõ ràng nếu:

- Cần timezone/business day.
- Cần import dữ liệu cũ.
- Cần sửa thời điểm nghiệp vụ khác thời điểm insert.
- Cần index theo `storeId + createdAt`.

Case F&B:

- Business day có thể bắt đầu lúc 5:00 sáng, không phải 00:00.
- Order tạo offline rồi sync lên sau, `_id` timestamp có thể không phản ánh thời điểm bán hàng.
- Reporting nên dựa vào `orderedAt`, `paidAt`, `businessDate`, không chỉ `_id`.

Điểm middle cần nhớ:

- `_id` hữu ích cho pagination đơn giản.
- `createdAt`/`paidAt` vẫn cần cho nghiệp vụ rõ ràng.
- Khi sort bằng `_id`, đảm bảo query có index phù hợp với filter đi kèm.

---

## 10. Pagination: tránh `skip()` cho trang sâu

`skip()` nhìn giống SQL `OFFSET`, nhưng MongoDB vẫn phải đi qua các document bị skip.

Không nên cho list lớn:

```js
db.orders.find({ storeId })
  .sort({ createdAt: -1 })
  .skip(100000)
  .limit(20)
```

Nên dùng cursor-based pagination:

```js
db.orders.find({
  storeId,
  createdAt: { $lt: lastCreatedAt }
})
.sort({ createdAt: -1 })
.limit(20)
```

Nếu nhiều order có cùng `createdAt`, dùng tie-breaker `_id`:

```js
db.orders.find({
  storeId,
  $or: [
    { createdAt: { $lt: lastCreatedAt } },
    { createdAt: lastCreatedAt, _id: { $lt: lastId } }
  ]
})
.sort({ createdAt: -1, _id: -1 })
.limit(20)
```

Index tương ứng:

```js
db.orders.createIndex({ storeId: 1, createdAt: -1, _id: -1 })
```

Case F&B:

- Order history.
- Transaction list.
- Kitchen event log.
- Audit log.

Điểm middle cần nhớ:

- `skip()` ổn cho admin nhỏ hoặc page đầu.
- Cursor pagination tốt cho infinite scroll và data lớn.
- Cursor phải stable: sort field nên unique hoặc có tie-breaker.

---

## 11. Over-indexing: index giúp read nhưng làm write chậm

Mỗi index đều phải được cập nhật khi insert/update/delete. Collection càng nhiều index, write càng đắt.

Triệu chứng hay gặp:

- CPU không quá cao nhưng DB vẫn chậm.
- Insert/update latency tăng.
- Disk I/O cao.
- Replication lag tăng.
- Index size lớn hơn data size quá nhiều.

Case F&B write-heavy:

- Order tạo liên tục.
- Payment update liên tục.
- Kitchen status update liên tục.
- Event/audit log ghi nhiều.

Không nên tạo index theo kiểu thấy query nào cũng thêm index ngay. Cần gom theo access pattern.

Checklist trước khi tạo index:

- Query này có chạy thường xuyên không?
- Query này có nằm trong API realtime không?
- Collection có bao nhiêu document?
- Field có selective không?
- Index có support sort không?
- Index mới có làm write path chậm đi đáng kể không?

Điểm middle cần nhớ:

- Index là trade-off giữa read và write.
- Index ít nhưng đúng thường tốt hơn nhiều index rời rạc.
- Dùng slow query log/profiler để quyết định, không đoán bằng cảm giác.

---

## 12. Cách thiết kế index thực dụng

Index nên đi từ query pattern, không đi từ field.

Ví dụ API:

```js
db.orders.find({
  storeId,
  status: "OPEN",
  createdAt: { $gte: from, $lt: to }
})
.sort({ createdAt: -1 })
.limit(50)
```

Index hợp lý:

```js
db.orders.createIndex({
  storeId: 1,
  status: 1,
  createdAt: -1
})
```

Quy tắc quan trọng:

- Equality fields trước: `storeId`, `status`.
- Range/sort field sau: `createdAt`.
- Compound index dùng được tốt nhất theo prefix từ trái sang phải.

Ví dụ index:

```js
{ storeId: 1, createdAt: -1 }
```

Query dùng tốt:

```js
db.orders.find({ storeId }).sort({ createdAt: -1 })
```

Query dùng kém hoặc không đúng kỳ vọng:

```js
db.orders.find({ createdAt: { $gte: from } })
```

Vì query bỏ qua prefix `storeId`.

Các loại index nên biết ở level middle:

- Compound index: index nhiều field theo query pattern.
- Unique index: đảm bảo không trùng, ví dụ `paymentRequestId`.
- Partial index: chỉ index một phần document, rất hữu ích cho status.
- TTL index: cleanup data hết hạn.
- Text index: search cơ bản, không thay thế search engine chuyên dụng.

Ví dụ partial index:

```js
db.orders.createIndex(
  { storeId: 1, tableId: 1, createdAt: -1 },
  { partialFilterExpression: { status: "OPEN" } }
)
```

Phù hợp cho query:

```js
db.orders.find({
  storeId,
  tableId,
  status: "OPEN"
})
```

Ví dụ unique index cho idempotency:

```js
db.paymentRequests.createIndex(
  { storeId: 1, idempotencyKey: 1 },
  { unique: true }
)
```

Điểm middle cần nhớ:

- Index field order rất quan trọng.
- Index phải khớp filter + sort.
- Partial index giúp giảm index size cho data có trạng thái.
- Unique index là một cách bảo vệ business invariant ở DB level.

---

## 13. Aggregation pipeline: đừng để report kéo sập API

Pipeline dễ sai nhất khi xử lý quá nhiều document trước khi filter.

Không nên:

```js
db.orders.aggregate([
  { $sort: { createdAt: -1 } },
  { $match: { storeId, status: "PAID" } }
])
```

Nên:

```js
db.orders.aggregate([
  { $match: { storeId, status: "PAID" } },
  { $sort: { createdAt: -1 } },
  { $limit: 100 }
])
```

Checklist tối ưu aggregation:

- `$match` sớm.
- `$project` bỏ field lớn không cần dùng.
- `$sort` nên có index support hoặc sort trên tập nhỏ.
- `$lookup` sau khi đã filter mạnh.
- `$facet` cẩn thận vì có thể nhân chi phí.
- Chạy `explain("executionStats")`.

Ví dụ dùng `explain`:

```js
db.orders.explain("executionStats").aggregate([
  { $match: { storeId, status: "PAID" } },
  { $sort: { createdAt: -1 } },
  { $limit: 100 }
])
```

Cần nhìn:

- `totalDocsExamined`
- `totalKeysExamined`
- Có `COLLSCAN` không
- Execution time
- Stage nào tốn nhiều nhất

Case F&B:

- Dashboard realtime nên đọc từ read model/tổng hợp sẵn.
- Báo cáo cuối ngày có thể chạy async.
- Export Excel lớn nên chạy job và gửi link tải.

Điểm middle cần nhớ:

- Aggregation tốt khi data đã được lọc đúng.
- API request không nên chạy report quá nặng.
- Nếu business chấp nhận eventual consistency, precompute là lựa chọn tốt.

---

## 14. Hot document: nhiều request cùng update một document

MongoDB update atomic theo document. Nếu nhiều request cùng ghi vào một document, document đó thành bottleneck.

Ví dụ không tốt:

```json
{
  "_id": "store_1",
  "todayRevenue": 999999,
  "orderCount": 12345
}
```

Mỗi payment đều:

```js
db.storeStats.updateOne(
  { _id: "store_1" },
  { $inc: { todayRevenue: amount, orderCount: 1 } }
)
```

Khi peak hour, document `store_1` có thể bị write contention.

Cách tốt hơn:

1. Event source:

```json
{
  "_id": "rev_event_001",
  "storeId": "store_1",
  "orderId": "order_001",
  "amount": 250000,
  "createdAt": "2026-06-29T10:00:00.000Z"
}
```

2. Bucket counter:

```json
{
  "_id": "store_1_2026_06_29_10",
  "storeId": "store_1",
  "hour": "2026-06-29T10:00:00.000Z",
  "revenue": 12000000,
  "orderCount": 80
}
```

3. Async aggregation:

- Ghi order/payment trước.
- Worker cập nhật summary sau.
- Dashboard chấp nhận delay ngắn.

Điểm middle cần nhớ:

- Counter global là red flag.
- Counter theo store/day/hour thường thực tế hơn.
- Với revenue/payment, event log giúp audit tốt hơn update một field tổng.

---

## 15. Document model: embed hay reference?

MongoDB không có nghĩa là nhúng mọi thứ vào một document.

Embed phù hợp khi:

- Dữ liệu nhỏ.
- Luôn đọc chung với parent.
- Ít update riêng.
- Vòng đời phụ thuộc parent.
- Số lượng phần tử có giới hạn.

Reference phù hợp khi:

- Dữ liệu lớn hoặc tăng liên tục.
- Cần query riêng.
- Update độc lập.
- Có lifecycle riêng.
- Nhiều entity cùng tham chiếu.

Case F&B nên embed:

```json
{
  "_id": "order_001",
  "items": [
    {
      "productId": "p_001",
      "name": "Pho bo",
      "qty": 2,
      "unitPrice": 60000
    }
  ],
  "total": 120000
}
```

Vì order item là snapshot tại thời điểm bán. Nếu sau này món đổi tên hoặc đổi giá, order cũ vẫn phải giữ lịch sử đúng.

Case nên reference:

- Customer profile.
- Payment attempts.
- Order events/status history.
- Inventory movements.
- Audit logs.

Điểm middle cần nhớ:

- Data modeling bắt đầu từ read/write pattern.
- Denormalization là để phục vụ query cụ thể.
- Snapshot field cần phân biệt với source of truth.

---

## 16. Read concern, write concern, read preference

Ba khái niệm này rất quan trọng khi làm realtime app.

Write concern quyết định ghi tới mức nào thì MongoDB báo thành công.

```js
{ w: 1 }
```

Nhanh hơn, nhưng chỉ cần primary xác nhận.

```js
{ w: "majority" }
```

An toàn hơn, đợi đa số replica xác nhận, latency cao hơn.

Read preference quyết định đọc từ đâu:

- `primary`: đọc từ primary, dữ liệu mới nhất hơn.
- `secondary`: giảm tải primary nhưng có thể stale.
- `primaryPreferred`, `secondaryPreferred`: tùy tình huống.

Read concern quyết định mức consistency khi đọc:

- `local`: nhanh, có thể đọc dữ liệu chưa majority committed.
- `majority`: chỉ đọc dữ liệu đã được majority xác nhận.

Case F&B:

- Cashier vừa thanh toán xong mà kitchen screen đọc từ secondary có replication lag, có thể chưa thấy order mới.
- Customer app vừa đặt món mà order status đọc stale, UI báo sai.
- Payment success nên ưu tiên consistency hơn latency quá thấp.

Gợi ý thực dụng:

- Flow critical như payment/order status: đọc primary, write concern majority nếu cần độ an toàn cao.
- Dashboard/report: có thể đọc secondary nếu chấp nhận delay.
- Audit/payment ledger: ưu tiên correctness.
- Menu/product list: có thể cache và eventual consistency.

Điểm middle cần nhớ:

- Không phải query nào cũng cần consistency như nhau.
- Đọc từ secondary không miễn phí; nó đổi latency/load lấy khả năng stale data.
- Với POS realtime, stale vài giây cũng có thể là bug nghiệp vụ.

---

## 17. Schema governance: schema-less không có nghĩa là vô kỷ luật

MongoDB flexible schema giúp dev nhanh, nhưng dễ tạo data bẩn.

Ví dụ xấu:

```js
price: "100000"
price: 100000
price: null
```

Hậu quả:

- Aggregation phải cast phức tạp.
- Index kém hiệu quả.
- API phải handle nhiều shape cũ.
- Migration về sau khó và rủi ro.

Cách kiểm soát:

- DTO/schema validation ở application layer.
- MongoDB collection validator cho field quan trọng.
- Document versioning khi thay đổi schema lớn.
- Migration plan rõ ràng.
- Contract test cho API response quan trọng.

Ví dụ document version:

```json
{
  "_id": "order_001",
  "schemaVersion": 2,
  "storeId": "s_001",
  "total": 250000
}
```

Điểm middle cần nhớ:

- Flexible schema là quyền chủ động thiết kế, không phải bỏ schema.
- Field type phải nhất quán.
- Với money, tránh float; dùng integer minor unit như VND amount.

---

## 18. Migration production: updateMany lớn là nguy hiểm

Chạy một lệnh update hàng loạt trên collection lớn có thể gây:

- Replication lag.
- Oplog tăng mạnh.
- Lock/contention.
- CPU/I/O spike.
- API latency tăng.

Không nên:

```js
db.orders.updateMany(
  {},
  { $set: { schemaVersion: 2 } }
)
```

Quy trình an toàn hơn:

1. Code hỗ trợ cả schema cũ và mới.
2. Write path bắt đầu ghi schema mới.
3. Backfill theo batch nhỏ.
4. Theo dõi lag/latency/error.
5. Sau khi data ổn định, remove code hỗ trợ schema cũ.

Ví dụ batch migration:

```js
db.orders.find({
  schemaVersion: { $ne: 2 }
})
.sort({ _id: 1 })
.limit(500)
```

Sau đó update từng batch, nghỉ giữa các batch nếu DB tải cao.

Case F&B:

- Thêm `businessDate` vào orders cũ.
- Đổi payment status enum.
- Tách `customerInfo` thành snapshot field.
- Backfill `storeId` cho collection event cũ.

Điểm middle cần nhớ:

- Migration là rollout process, không chỉ là script.
- Code phải backward compatible trong giai đoạn chuyển đổi.
- Luôn có metric để biết migration có làm production chậm không.

---

## 19. Checklist ôn tập cho level middle

Khi review một MongoDB design, tự hỏi:

- Collection này phục vụ query pattern nào?
- Query quan trọng nhất có index chưa?
- Index có đúng thứ tự equality -> range/sort không?
- Có field nào type không nhất quán không?
- Có mảng nào tăng vô hạn không?
- Có document nào bị nhiều request cùng update không?
- API realtime có đọc từ secondary gây stale data không?
- Report nặng có đang chạy trực tiếp trong request không?
- Có cần transaction thật không, hay single-document atomic update đủ?
- Có idempotency key cho payment/order retry chưa?
- Migration có batch, monitor và rollback plan chưa?

Các case nên tự luyện:

1. Thiết kế collection `orders` cho POS.
2. Thiết kế index cho order list theo store/status/date.
3. Làm cursor pagination cho order history.
4. Thiết kế payment idempotency bằng unique index.
5. Tách order status history thành `order_events`.
6. Làm daily revenue summary bằng async aggregation.
7. Xử lý read/write concern cho payment success.
8. Viết migration thêm `businessDate` cho orders cũ.
9. Tối ưu aggregation top-selling products.
10. Phân biệt field snapshot và source of truth trong order.

---

## 20. Những phần tạm chưa cần học sâu

Các chủ đề dưới đây nên biết tên và khái niệm, nhưng chưa cần đào edge case nếu bạn chưa trực tiếp vận hành hệ lớn:

- Sharding và shard key design.
- Balancer/chunk migration.
- Cross-shard transaction.
- Change stream thay thế event bus ở scale lớn.
- Oplog retention tuning chuyên sâu.
- WiredTiger internal tuning.

Học MongoDB ở level sắp middle nên ưu tiên làm đúng các thứ xuất hiện hằng ngày: model dữ liệu, index, consistency, transaction vừa đủ, aggregation, migration và observability.
