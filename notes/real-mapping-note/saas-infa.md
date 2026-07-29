# Note luyện phỏng vấn — Map lý thuyết vào codebase SaaS này

> Toàn bộ số liệu dưới đây lấy từ việc quét thực tế repo `DIQIT-Code/saas` (30+ service, TypeScript + Go).
> Mỗi mục có: **lý thuyết** → **chỗ nó xuất hiện trong code** → **câu hỏi phỏng vấn hay hỏi**.

---

## 0. Bản đồ hạ tầng (trả lời nhanh khi bị hỏi "hệ thống của bạn dùng gì?")

| Thành phần | Có dùng? | Ghi chú |
|---|---|---|
| **MongoDB** | ✅ Chính | Mọi service `saas-management-*` + `saas-microservice-*`. Mongoose (TS) / mongo-driver (Go) |
| **PostgreSQL** | ✅ Phụ | Chỉ 2 service: `saas-management-reports` (TypeORM) và `saas-management-order` (driver `pg` thuần) |
| **RabbitMQ** | ✅ Chính | ~15 service. `amqplib` (TS) / `streadway/amqp` (Go) |
| **gRPC** | ✅ Chính | Giao tiếp đồng bộ giữa service (protobuf) |
| **Redis** | ❌ **KHÔNG dùng** | Xem mục 4 — đây là câu hỏi "tủ" cho bạn |
| **Materialized View** | ❌ **KHÔNG có** | Xem mục 3.4 — thay bằng bảng report tự build |
| **ClickHouse** | 🟡 Example | Chỉ có thư mục `clickhouse-db-example/`, chưa vào production |

**Kiến trúc tổng:** microservices, multi-tenant (mọi thứ khoá theo `client_uuid`),
đồng bộ = gRPC, bất đồng bộ = RabbitMQ, OLTP = MongoDB, OLAP/report = PostgreSQL.

---

## 1. MongoDB — Compound Index

### 1.1 Lý thuyết cần thuộc

**Compound index** = index trên nhiều field, lưu dưới dạng B-tree với key là *tuple* các field theo đúng thứ tự khai báo.

Ba luật phải nói được trong phỏng vấn:

**(a) Prefix rule** — index `{a:1, b:1, c:1}` dùng được cho query trên:
- `{a}` ✅
- `{a, b}` ✅
- `{a, b, c}` ✅
- `{b}` ❌ (không có prefix `a`)
- `{a, c}` 🟡 (dùng được `a` để seek, `c` phải filter thủ công — gọi là *index scan + residual filter*)

**(b) ESR rule** (Equality → Sort → Range) — thứ tự field tối ưu:
1. Field so sánh **bằng** (`=`) trước
2. Field dùng để **sort**
3. Field dùng **range** (`>`, `<`, `$in` nhiều giá trị, `BETWEEN`) sau cùng

Lý do: range làm "vỡ" tính liên tục của B-tree. Sau một field range, các field sau không còn sorted trong vùng scan → không dùng để sort được nữa.

**(c) Cardinality** — field lọc mạnh nhất (nhiều giá trị khác nhau) nên đứng trước, *trừ khi* ESR bắt phải khác.

### 1.2 Số liệu thật trong repo

**121 compound index** trên Mongo, phân bố:

| Service | Số compound index |
|---|---|
| `saas-management-inventory` | 37 |
| `saas-management-menu` | 36 |
| `saas-management-system` | 18 |
| `saas-management-order` | 12 |
| `saas-management-store` | 11 |
| `saas-management-payment` | 4 |
| `saas-management-promotion` | 2 |
| `saas-management-user` | 1 |

### 1.3 Ba nhóm pattern trong repo (nhớ để kể)

#### Nhóm A — "Tenant + soft-delete" (phổ biến nhất)

```ts
// saas-management-inventory/src/services/inventory/inventory.model.ts:81
InventorySchema.index({ client_uuid: 1, is_delete: 1 });
```

Vì **mọi** service function trong repo đều tự động thêm `is_delete: CONSTANT.NOT_DELETE`
(xem `CLAUDE.md` của inventory: *"every DB-reading function filters is_delete"*),
và mọi HTTP handler đều thêm `client_uuid: user.getClientUuid()`.

→ Query mặc định là `{ client_uuid: X, is_delete: 0, ...còn lại }` → index này phục vụ **mọi** endpoint list.

> ⚠️ **Điểm yếu để bạn nói ra khi phỏng vấn (rất ghi điểm):**
> `is_delete` chỉ có 2 giá trị (0/1) → **cardinality cực thấp**. Đưa nó vào index gần như
> không thu hẹp được gì; nó chỉ hữu ích khi làm *covered index* hoặc khi tỉ lệ deleted rất cao.
> Cách tốt hơn: dùng **partial index** — và thực tế repo có làm (xem nhóm C).

Còn tệ hơn nữa, có 14 chỗ khai báo index **chỉ có** `{is_delete, is_active}` — không có `client_uuid`:
```ts
// saas-management-inventory/src/services/masterUom/masterUom.model.ts:32
MasterUomSchema.index({ is_delete: 1, is_active: 1 });
```
Index này gần như vô dụng: 2 field boolean → selectivity ~1/4, planner nhiều khả năng bỏ qua và chọn COLLSCAN.

#### Nhóm B — Compound index có ích thật (đúng ESR)

```ts
// saas-management-order/src/services/order/order.model.ts:294
OrderSchema.index({ client_uuid: 1, store_uuid: 1, order_time: 1, is_delete: 1 });
OrderSchema.index({ client_uuid: 1, store_uuid: 1, collection_time: 1, is_delete: 1 });
```

**Phân tích ESR:**
- `client_uuid` = **E** (equality, tenant)
- `store_uuid` = **E** (equality, chọn cửa hàng)
- `order_time` = **R** (range — lọc theo khoảng thời gian) ✅ đúng vị trí
- `is_delete` — đứng **sau** range → **vô dụng cho việc seek**, chỉ còn tác dụng nếu covered

→ Nếu sửa lại đúng chuẩn ESR sẽ là `{client_uuid, store_uuid, is_delete, order_time}`,
hoặc tốt hơn: bỏ `is_delete` khỏi index và dùng partial index.

Đây là **câu trả lời hoàn hảo** cho câu hỏi *"Bạn tối ưu được index nào chưa?"*

```ts
// saas-management-inventory/src/services/inventoryLog/inventoryLog.model.ts:86
InventoryLogSchema.index({ client_uuid: 1, store_uuid: 1, material_uuid: 1, created_at: 1 });
```
→ 3 equality + 1 range/sort ở cuối. **Đây là compound index chuẩn nhất trong repo.**
Phục vụ query kiểu: "lấy log tồn kho của nguyên liệu X, tại cửa hàng Y, trong khoảng thời gian Z, sort theo thời gian".

#### Nhóm C — Unique compound + Partial filter (kỹ thuật đáng nói nhất)

```ts
// saas-management-menu/src/services/product/product.model.ts:295
ProductSchema.index(
  { client_uuid: 1, code: 1, is_delete: 1 },
  { unique: true, partialFilterExpression: { is_delete: 0 } }
);
```

**Đây là kỹ thuật giải quyết một bài toán kinh điển:** làm sao ép "mã sản phẩm là duy nhất
trong 1 tenant" mà vẫn cho phép **soft-delete rồi tạo lại mã cũ**?

- Unique index thường: xoá mềm sản phẩm `code=A`, tạo lại `code=A` → **duplicate key error** vì bản ghi cũ vẫn nằm đó.
- **Partial index**: chỉ index những document thoả `is_delete: 0` → bản ghi đã xoá mềm không nằm trong index → tạo lại được. ✅

**18 index** trong repo dùng `partialFilterExpression`, tập trung ở `menu`, `inventory`, `payment`, `promotion`.

> Bonus: partial index còn **nhỏ hơn** index thường → ít RAM, ghi nhanh hơn.
> Đây chính là điều mình nói ở nhóm A: đáng lẽ nên dùng partial index thay vì nhét `is_delete` vào key.

#### Nhóm D — Compound index "quá dài" (anti-pattern để phê bình)

```ts
// saas-management-inventory/src/services/inventoryV2/inventoryV2.model.ts:52
InventoryV2Schema.index(
  { client_uuid:1, location_uuid:1, product_uuid:1, variant_uuid:1, barcode:1, expiration_date:1 },
  { unique: true }
);

// saas-management-inventory/src/services/lowStockAlert/lowStockAlert.model.ts:43
LowStockAlertSchema.index({ client_uuid:1, is_delete:1, status:1, product_uuid:1, variant_uuid:1, location_uuid:1 });
```

6 field. Điểm cần nói:
- Với **unique index** (inventoryV2) thì hợp lý — đó là **business key** thật (1 lô hàng = tenant + kho + sản phẩm + biến thể + barcode + hạn dùng).
- Với index thường (lowStockAlert) thì đáng ngờ: index càng dài, entry càng to → ít entry/page → nhiều page phải đọc → **write amplification** khi update.
- Và vì `is_delete` + `status` đứng ở vị trí 2-3 (cardinality thấp), 3 field cuối gần như không bao giờ được tận dụng.

### 1.4 Các loại index khác trong repo

**TTL index** (2 chỗ) — Mongo tự xoá document sau N giây:
```ts
// saas-management-customer/src/services/customerSyncLog/customerSyncLog.model.ts:54
CustomerSyncLogSchema.index(
  { updated_at: 1 },
  { expireAfterSeconds: 60 * 60 * 24 * 30 }  // giữ log 30 ngày
);
```
> **Đây là chỗ đáng lẽ người ta dùng Redis `EXPIRE`.** Repo này dùng TTL index của Mongo thay thế.
> Cơ chế: background thread của Mongo chạy mỗi 60s, quét index và xoá. → **không chính xác tuyệt đối về thời điểm**, có thể trễ tới 60s+. Nói được điều này là điểm cộng.

---

## 2. RabbitMQ

### 2.1 Lý thuyết cần thuộc

**Mô hình AMQP 0-9-1:**
```
Producer → [Exchange] --binding(routing key)--> [Queue] → Consumer
```
Producer **không bao giờ** gửi thẳng vào queue (trừ default exchange `""`). Nó gửi vào **exchange**,
exchange dựa vào **binding** để quyết định copy message vào queue nào.

**4 loại exchange:**

| Loại | Cách route | Dùng khi |
|---|---|---|
| `direct` | routing key **khớp chính xác** binding key | Point-to-point, phân loại đơn giản |
| `topic` | routing key khớp **pattern** (`*` = 1 từ, `#` = 0+ từ) | Pub/sub theo chủ đề, phân cấp |
| `fanout` | **bỏ qua** routing key, copy vào **mọi** queue đã bind | Broadcast thuần |
| `headers` | Khớp theo header thay vì routing key | Hiếm dùng |

**Điểm mấu chốt (chính là câu bạn hỏi):** một message **có thể** vào nhiều queue.
Đó là khi có **≥2 binding khác nhau cùng match** message đó. Exchange sẽ **copy** message
vào từng queue. Mỗi queue là một bản sao độc lập, ack riêng.

Ngược lại: nếu **nhiều consumer cùng nghe MỘT queue** → RabbitMQ **round-robin**, mỗi message chỉ 1 consumer nhận (competing consumers / work queue).

> Phân biệt được 2 cái này là ranh giới giữa junior và mid.

### 2.2 Topology thật trong repo

#### Exchange `audit_log_exchange` — **type `topic`** ⭐ (quan trọng nhất)

Khai báo tại `saas-microservice-system/service/queue/auditLogSetup.go:16-28`:

```go
const exchangeName = "audit_log_exchange"

var queue2routingKey = map[QueueName][]string{
    AuditLogOSSUser:  {"user.*"},
    AuditLogOSSAuth:  {"auth.*"},
    AuditLogOSSMenu:  {"menu.*"},
    AuditLogOSSStore: {"store.*"},
}
```

**Producer (6 service TypeScript):** middleware `src/middleware/auditLog.ts` ở
`menu`, `promotion`, `store`, `payment`, `auth`, `user`.

Routing key được build tại `auditLog.ts:169`:
```ts
const routingKey = `${scope}.${action}`;   // vd: "menu.create", "user.delete"
```
`scope` mặc định = tên service, `action` ∈ {`create`,`update`,`delete`}.

**Data vận chuyển** (`IAuditLog`, `auditLog.ts:12-24`):
```ts
{ ip, user_uuid, client_uuid, action, scope, status,
  url, method, target,
  metadata: { user_agent, device, browser, os, location },  // location từ geoip-lite
  created_at }
```
Message được publish trong `res.on('finish')` → **fire-and-forget, sau khi response đã trả về client** → không làm chậm API. Đây là ứng dụng đúng của async messaging.

**Consumer:** `saas-microservice-system/main.go:105-106`
```go
go queueService.ConsumeAuditLog(queue.AuditLogOSSUser,  10, auditLogCtl.AuditLogHandler)
go queueService.ConsumeAuditLog(queue.AuditLogOSSStore, 10, auditLogCtl.AuditLogHandler)
```

**Sơ đồ:**
```
saas-management-user     --"user.create"------┐
saas-management-auth     --"auth.update"----┐ │
saas-management-menu     --"menu.create"--┐ │ │
saas-management-store    --"store.*"----┐ │ │ │
saas-management-payment  --"payment.*"┐ │ │ │ │   ← không queue nào bind!
saas-management-promotion--"promotion.*"│ │ │ │   ← không queue nào bind!
                                      ↓ ↓ ↓ ↓ ↓
                         ┌────────────────────────────┐
                         │  audit_log_exchange (topic)│
                         └────────────────────────────┘
                    user.*   auth.*   menu.*   store.*
                      ↓        ↓        ↓        ↓
                 [OSS_USER][OSS_AUTH][OSS_MENU][OSS_STORE]
                      ↓        ✗        ✗        ↓
                 consumer   KHÔNG    KHÔNG   consumer
                            có       có
```

### 2.3 ⭐ Trả lời trực tiếp: "Có chỗ nào bắn vào exchange mà 2 queue cùng nhận không?"

**Câu trả lời trung thực: KHÔNG — nhưng hạ tầng đã sẵn sàng cho việc đó.**

Lý do: cả 4 binding pattern (`user.*`, `auth.*`, `menu.*`, `store.*`) đều **loại trừ lẫn nhau** — không có routing key nào khớp được 2 pattern. Nên mỗi message chỉ vào tối đa **1 queue**.

**Cách trả lời hay trong phỏng vấn:**

> "Hệ thống em dùng topic exchange cho audit log. Hiện tại các binding key không giao nhau
> nên mỗi message chỉ vào 1 queue. Nhưng thiết kế này chọn `topic` thay vì `direct` chính là
> để mở đường: nếu mai này em muốn thêm một service Analytics nghe **toàn bộ** audit log,
> em chỉ cần khai báo thêm 1 queue bind với pattern `#` — không phải sửa một dòng code nào
> ở phía producer. Lúc đó 1 message sẽ vào 2 queue: queue theo scope và queue analytics."

Rồi bạn viết luôn ví dụ ra giấy:
```
routing key "menu.create":
  binding "menu.*"  → khớp → vào OSS_MENU
  binding "#"       → khớp → vào ANALYTICS      ← 1 message, 2 queue
```

**Hai bug thật để bạn kể tiếp (rất ấn tượng):**

1. **Message bị vứt im lặng.** `payment` và `promotion` publish routing key `payment.*` / `promotion.*`
   nhưng **không queue nào bind** 2 pattern này. Vì publish với `mandatory=false`,
   RabbitMQ **drop message không báo lỗi gì**. → Audit log của 2 service này mất trắng.
   *Cách phát hiện:* bật `mandatory=true` + handler `basic.return`, hoặc cắm **Alternate Exchange**.

2. **Queue phình rồi tự huỷ.** `OSS_AUTH` và `OSS_MENU` được declare + bind nhưng
   `main.go` chỉ start consumer cho `OSS_USER` và `OSS_STORE`. → 2 queue kia chỉ nhận vào, không ai lấy ra.
   Được cứu bởi `args` ở `auditLogSetup.go:35-40`:
   ```go
   "x-message-ttl": int32(24 * time.Hour / time.Millisecond),  // message sống 24h
   "x-expires":     int32(72 * time.Hour / time.Millisecond),  // queue tự xoá sau 72h không dùng
   ```
   → Đây là **backpressure/safety valve**, không phải là cái cớ để quên consumer.

### 2.4 Các exchange còn lại

| Exchange | Type | Publisher | Consumer (queue) | Routing key | Data |
|---|---|---|---|---|---|
| `OSS-PRODUCT` | topic | menu, customer, order | menu (`product-price-logging`), menu (`product-price-schedule`), reports (`pricing-tier-schedule`), customer (`sync-customer-smageri`), customer (`sync-points-smageri`), order (`sync-order-smageri`) | `PRICE_LOG_CREATE.CMD`, `PRICE_SCHEDULE.CMD`, `PRICING_TIER_SCHEDULE.CMD`, `SYNC_CUSTOMER_SCHEDULE.CMD`, `SYNC_POINTS_SCHEDULE.CMD`, `SYNC_ORDER_SCHEDULE.CMD` | Lệnh ghi log giá, lịch đổi giá, đồng bộ khách/điểm/đơn với Smaregi (POS Nhật) |
| `OSS-INVENTORY` | topic | inventory | inventory (`inventory-po-received`), inventory (`inventory-create-transfer-cmd`) | `PO.RECEIVED`, `TRANSFER.CREATE.CMD` | Phiếu nhập đã nhận hàng, lệnh tạo phiếu chuyển kho |
| `OSS-INVENTORY-V2-MOVEMENT` | topic | inventory | inventory (`inventory-v2-goods-receipt`), inventory (`inventory-v2-goods-return`) | `GOODSRECEIPT.RECEIVED`, `GOODSRETURN.RETURNED` | Biến động tồn kho v2: nhập hàng, trả hàng |
| `OSS-PURCHASE-ORDER-REQUEST` | topic | inventory | inventory (`purchase-order-create-cmd`) | `PO.CREATE.CMD` | Lệnh tạo đơn mua hàng |
| `OSS-NOTIFICATION-PURCHASE-ORDER-REQUEST` | topic | inventory | inventory (`purchase-order-request-notification`) | `PO.REQUEST.ACCEPTED` | Thông báo yêu cầu mua hàng được duyệt |
| `ex.out_of_stock.<client_uuid>` | **direct** | inventory | (POS/KDS client bên ngoài) | routing key = `store_uuid` | `{type: "PRODUCT"\|"TOPPING"\|"ITEM", data: {...}}` |
| `ORDER_POS` | direct + topic | `saas-microservice-order-pos` (Go) | (bên ngoài repo) | `CREATED`, `UPDATED`, `FINISH`, `CANCEL`, `REFUND...` | `{client_uuid, uuid, total, currency:"JPY", status, order_items[{product_uuid, product_name, quantity, price}], created_at, updated_at, updated_by}` |

**`ex.out_of_stock.<client_uuid>` — kỹ thuật đáng nói** (`saas-management-inventory/src/services/outOfStock/outOfStock.service.ts:15-26`):

```ts
const exchange = 'ex.out_of_stock.' + product.client_uuid;   // exchange PER TENANT
const routingKey = product.store_uuid;                       // routing key = cửa hàng
await publishToExchange(exchange, routingKey, JSON.stringify(message), 'direct',
  { persistent: true, expiration: 7200000 });                // TTL 2 giờ
```

- **Exchange động theo tenant** → cách ly hoàn toàn giữa các khách hàng ở tầng broker (không thể leak data chéo tenant).
- **Routing key = `store_uuid`** với `direct` → mỗi POS/KDS ở cửa hàng chỉ nhận đúng thông báo hết hàng của cửa hàng mình.
- **`expiration: 7200000`** (2h) → thông báo "hết hàng" mà cũ quá 2 tiếng thì vô nghĩa, tự bốc hơi. Đây là **per-message TTL** (khác `x-message-ttl` mức queue).
- **Trade-off cần nói:** exchange per-tenant = số exchange tăng tuyến tính theo số khách hàng. Với vài trăm tenant thì ổn, vài chục nghìn thì broker sẽ khổ (metadata trong Mnesia). Giải pháp thay thế: 1 topic exchange, routing key `<client_uuid>.<store_uuid>`.

### 2.5 Reliability — các kỹ thuật có trong repo

**(a) Prefetch / QoS** — giới hạn số message chưa ack mà broker đẩy cho 1 consumer:
```ts
await channel.prefetch(15);   // inventory, order, customer, store, payment, promotion, user
await channel.prefetch(5);    // menu, reports, system, auth
```
```go
ch.Qos(10, 0, false)   // order-pos, inventory, payment, customer, promotion, store
ch.Qos(2,  0, false)   // menu, handlequeue-menu
```
> **Nếu không set prefetch**, RabbitMQ đẩy *toàn bộ* queue cho consumer đầu tiên → consumer đó OOM,
> các consumer khác ngồi chơi → mất luôn tác dụng của việc scale ngang. Đây là câu hỏi phỏng vấn rất hay gặp.
> Prefetch thấp = cân bằng tải tốt, throughput thấp. Prefetch cao = ngược lại. `10-15` là vùng hợp lý.

**(b) Manual ack** — `channel.consume(queue, "", false /* autoAck=false */, ...)`.
Consumer phải gọi `chan.ack(msg)` sau khi xử lý xong.
→ Đảm bảo **at-least-once delivery**: nếu consumer chết giữa chừng, message quay lại queue.
→ **Hệ quả bắt buộc: handler phải idempotent.** Chính vì thế các unique compound index ở mục 1.3-C
rất quan trọng — chúng chặn việc xử lý trùng ghi ra 2 bản ghi.

**(c) Persistent message + durable queue/exchange:**
```ts
producerChannel.publish(exchange, key, buf, { persistent: true });     // deliveryMode=2
await channel.assertQueue(queueName, { durable: true });
```
→ Message ghi xuống đĩa, sống sót qua restart broker.
> Cần nói thêm: `persistent` **không** = không mất. Có cửa sổ giữa lúc broker nhận và lúc fsync.
> Muốn chắc chắn phải bật **Publisher Confirms** — **repo này CHƯA có**. Đây là điểm cải tiến bạn có thể đề xuất.

**(d) Delayed message** (plugin `rabbitmq_delayed_message_exchange`) — 7 service Go dùng:
```go
// saas-microservice-order-pos/services/queue/amqp.go:306
ch.ExchangeDeclare(exchangeName, "x-delayed-message", true, false, false, false, args)
```
→ Retry có backoff / lên lịch job mà không cần Redis, không cần cron.
> **Đây là chỗ điển hình người ta dùng Redis Sorted Set (ZADD score=timestamp) hoặc BullMQ delayed job.** Repo đẩy trách nhiệm này sang RabbitMQ plugin.

**(e) Worker pool** — `saas-microservice-system/service/queue/auditLogSetup.go:90`:
```go
r.workerPool(numberWorker, string(queueName), msgs, handler)   // numberWorker = 10
```
Prefetch=10 + 10 goroutine worker → 10 message xử lý song song trên 1 channel.

**(f) Reconnect với retry** — mọi `rabbitMQ.lib.ts` đều có `_reconnectToRabbitMQ()` + handler `on('error')`, `on('close')`.
> ⚠️ Trong `saas-management-reports/src/lib/rabbitMQ.lib.ts:103` dòng `await connectToRabbitMQ()` đang **bị comment out** → service này thực chất KHÔNG tự reconnect được. Bug thật.

**(g) Tách channel producer / consumer:**
```ts
channel = await _createConsumerChannel();
producerChannel = await _createProducerChannel();
```
→ Đúng chuẩn. AMQP channel **không thread-safe**; và nếu 1 channel bị lỗi (vd. publish vào exchange không tồn tại), broker đóng cả channel → nếu dùng chung sẽ chết luôn consumer.

**(h) KHÔNG có Dead Letter Queue** — quét toàn repo: 0 chỗ dùng `x-dead-letter-exchange`.
> Đây là **lỗ hổng lớn nhất** trong thiết kế messaging của hệ thống này, và là câu trả lời tuyệt vời cho
> *"hệ thống bạn có điểm gì cần cải thiện?"*. Hiện tại: message lỗi → `nack` → requeue → lỗi lại → **poison message loop vô hạn**, hoặc bị `ack` mù và mất luôn.

---

## 3. PostgreSQL — ⭐ Tại sao dùng Postgres khi hệ thống chạy MongoDB?

### 3.1 Câu trả lời ngắn (30 giây)

> "MongoDB là **OLTP** store cho nghiệp vụ vận hành — ghi nhiều, đọc theo key, schema linh hoạt theo tenant.
> Nhưng báo cáo là **OLAP**: quét hàng triệu dòng order detail, GROUP BY nhiều chiều, tính tỉ lệ tích luỹ, window function.
> Aggregation Pipeline của Mongo làm được nhưng chậm, khó viết, và **quan trọng nhất: nó chạy chung cluster với luồng bán hàng** — một report nặng có thể làm nghẽn việc tạo đơn.
> Nên bọn em tách riêng: đơn hàng được ETL sang PostgreSQL dạng bảng phẳng (star schema), và toàn bộ report chạy trên đó."

### 3.2 Bằng chứng trong code

**Chỉ 2 service chạm Postgres:**

| Service | Cách kết nối | Mục đích |
|---|---|---|
| `saas-management-reports` | TypeORM `DataSource` (`src/lib/postgres.lib.ts`) | Toàn bộ báo cáo |
| `saas-management-order` | `pg` `Pool` thuần (`src/lib/postgres.lib.ts`, max 20 conn) | Chỉ 2 query đọc doanh số |

Quan trọng: **`saas-management-reports` kết nối CẢ HAI** (`src/lib/mongoose.lib.ts` + `src/lib/postgres.lib.ts`) — Mongo cho master data (sản phẩm, cửa hàng), Postgres cho fact data (đơn hàng).

**Schema Postgres = star schema kinh điển** (`src/models/order.postgre.ts`, 172 dòng, ~55 cột):

```ts
@Entity({ name: 'orders' })
export class Order {
  // ── Dimension keys (denormalized sẵn, không cần JOIN) ──
  client_uuid; client_name;              // tenant
  store_uuid; store_code; store_name; store_type;
  platform; brand; channel; order_type; order_device;

  // ── Date dimension "bung phẳng" — dấu hiệu nhận biết OLAP ──
  business_date;    // '2026-07-29'
  business_year;    // 2026
  business_month;   // 7
  business_day;     // 29
  business_quater;  // 'Q3'
  business_hour;    // 14

  // ── Measures ──
  transaction_value_with_tax; transaction_value_without_tax_delivery_fee;
  product_total_cost; tax; discount; delivery_fee;
  total_info8; total_info10; tax_info8; tax_info10;  // thuế 8%/10% (Nhật)
  used_point; earn_point;
}
```

**Vì sao `business_year/month/day/quater` là bằng chứng đây là kho phân tích?**
Trong OLTP bạn sẽ chỉ lưu 1 cột `timestamp` rồi `EXTRACT(YEAR FROM ...)` khi cần.
Ở đây họ **pre-compute** sẵn → tránh function call trên hàng triệu dòng, và cho phép index/partition trực tiếp trên các cột đó. Đây là kỹ thuật **denormalization for read** — đánh đổi dung lượng lấy tốc độ đọc.

### 3.3 Postgres làm được gì mà Mongo làm rất khổ — ví dụ ABC Report

`saas-management-reports/src/services/abcReport/abcReport.service.ts` — báo cáo phân tích ABC (Pareto 80/20 cho sản phẩm):

```sql
WITH product_revenue AS (
    SELECT product_code, product_name,
           COALESCE(NULLIF(product_category_code, ''), 'UNKNOWN') AS product_category_code,
           ROUND(price::numeric) AS unit_sub_price,
           SUM(quantity) AS quantity,
           SUM(ROUND(price::numeric) * quantity) AS revenue
    FROM order_details
    WHERE order_uuid = ANY($1::text[])
      AND ($2::text IS NULL OR product_category_code = $2::text)
    GROUP BY product_code, product_name, ..., unit_sub_price
),
total_calc AS (
    SELECT *,
           SUM(revenue) OVER (PARTITION BY product_code) AS group_revenue_by_product,
           SUM(revenue) OVER ()                          AS total_revenue
    FROM product_revenue
)
SELECT product_code, quantity, revenue,
       SUM(revenue) OVER (ORDER BY group_revenue_by_product DESC, product_code ASC)
         AS cumulative_revenue,                                    -- doanh thu tích luỹ
       ROUND((revenue / NULLIF(total_revenue,0)) * 100, 2) AS individual,
       ROUND((SUM(revenue) OVER (ORDER BY group_revenue_by_product DESC, ...)
              / NULLIF(total_revenue,0)) * 100, 2)         AS cumulative
FROM total_calc
ORDER BY group_revenue_by_product DESC, product_code ASC;
```

**Các thứ lý thuyết xuất hiện ở đây — học thuộc để giải thích:**

| Kỹ thuật | Vai trò |
|---|---|
| **CTE** (`WITH ... AS`) | Chia query thành các bước đọc được. Từ PG12, CTE không còn là optimization fence mặc định (`NOT MATERIALIZED` là default nếu dùng 1 lần) |
| **Window function** `SUM() OVER (ORDER BY ...)` | **Running total** — tính doanh thu tích luỹ mà **không cần self-join**. Trong Mongo phải `$setWindowFields` (chỉ có từ 5.0) hoặc kéo hết về app tính bằng JS |
| **`SUM() OVER ()`** (frame rỗng) | Tổng toàn bảng đặt cạnh từng dòng — không cần query thứ 2 |
| **`PARTITION BY`** | Chia nhóm tính riêng trong cùng 1 lượt quét |
| **`NULLIF(x, 0)`** | Chống chia cho 0 → trả NULL thay vì lỗi |
| **`= ANY($1::text[])`** | Truyền mảng làm tham số — tránh build `IN (...)` bằng string concat |
| **Parameterized query** `$1, $2` | Chống SQL injection ✅ |

**So sánh trực tiếp để trả lời phỏng vấn:**

| | MongoDB Aggregation | PostgreSQL |
|---|---|---|
| Running total | `$setWindowFields` (MongoDB 5.0+), cú pháp rườm rà | `SUM() OVER (ORDER BY ...)` — 1 dòng |
| JOIN nhiều bảng | `$lookup` — chậm, không có hash join thực thụ | Hash/Merge/Nested-loop join, planner tự chọn |
| Giới hạn RAM | 100MB/stage, phải `allowDiskUse` | `work_mem` cấu hình được, spill ra đĩa tự động |
| Query planner | Hạn chế | Cost-based, `EXPLAIN ANALYZE` chi tiết |
| Ad-hoc analytics | Phải viết pipeline JS | SQL — BI tool nào cũng cắm được |

> Repo có dùng `allowDiskUse` **10 lần**, toàn bộ ở `saas-management-order` → đúng là họ đã đụng trần 100MB của Mongo aggregation. Đây là bằng chứng thực tế cho lý do chuyển sang Postgres.

### 3.4 Materialized View — KHÔNG dùng, nhưng có "phiên bản thủ công"

Quét toàn repo: **0 kết quả** cho `CREATE MATERIALIZED VIEW` / `REFRESH MATERIALIZED VIEW`.
Cũng không có file `.sql` nào (schema quản lý bên ngoài repo, `synchronize: false` trong TypeORM).

**Nhưng** — họ làm **materialization thủ công** bằng các bảng report được ghi sẵn.
Xem danh sách entity ở `src/lib/postgres.lib.ts:26-38`:

```ts
entities: [
  // ── Bảng FACT (dữ liệu thô, ETL từ Mongo sang) ──
  Order, OrderDetail, OrderPayment,

  // ── Bảng "materialized" (kết quả đã tính sẵn) ──
  DashboardOrderHourlyReport,   // doanh thu theo giờ
  MenuReport,                   // báo cáo menu
  OrderPaymentReport,           // báo cáo thanh toán
  OrderPromotionReport,         // báo cáo khuyến mãi
  CountPromotion,               // đếm lượt dùng KM
  StoreForecast,                // dự báo cửa hàng
  ProductQuantityForecast,      // dự báo số lượng sản phẩm
  PricingTier,                  // bậc giá
]
```

**Cách nói trong phỏng vấn:**

> "Bọn em không dùng `MATERIALIZED VIEW` của Postgres mà tự quản lý bảng tổng hợp.
> Lý do: `REFRESH MATERIALIZED VIEW` (không `CONCURRENTLY`) **khoá bảng ở mức ACCESS EXCLUSIVE** —
> report sẽ đứng hình trong lúc refresh. Còn `CONCURRENTLY` thì bắt buộc phải có unique index và chậm hơn nhiều.
> Quan trọng hơn: MV **refresh toàn bộ**, trong khi dữ liệu của bọn em là **append-only theo ngày** —
> em chỉ cần cập nhật phần tăng thêm. Nên bọn em dùng bảng thường + upsert incremental, được điều khiển bằng event RabbitMQ.
> Đánh đổi là em phải tự lo tính đúng đắn (consistency), thứ mà MV cho sẵn."

**Bằng chứng cơ chế cập nhật là event-driven** — `saas-management-reports/src/index.ts:54`:
```ts
consumeFromExchange(
  'OSS-PRODUCT', 'pricing-tier-schedule', 'PRICING_TIER_SCHEDULE.CMD',
  processPricingTierSchedule
);
```
→ Có sự kiện đổi giá → consumer cập nhật bảng `pricing_tier` trong Postgres. Đây chính là **incremental materialization**.

### 3.5 Compound index trong Postgres (2 cái, đều là unique)

```ts
// src/models/orderDetail.postgre.ts:6
@Index('idx_order_details_uniq',
       ['client_uuid', 'business_date', 'order_detail_uuid'],
       { unique: true })
```
```ts
// src/services/productSales/productSales.model.ts:19
@Index('uq_product_quantity_forecast',
       ['business_date', 'store_code', 'client_uuid', 'product_code'],
       { unique: true })
```

**Vai trò kép — nói được cả hai mới đủ điểm:**

1. **Idempotency key cho ETL.** Pipeline chạy lại (retry, backfill) sẽ không tạo bản ghi trùng — `ON CONFLICT` hoặc lỗi 23505 bắt được. Nhớ liên hệ với mục 2.5-(b): consumer at-least-once **bắt buộc** cần cái này.

2. **Index phục vụ query.** Thứ tự `(client_uuid, business_date, ...)` khớp chính xác với `WHERE` trong `abcReport.service.ts:47-51`:
   ```sql
   WHERE client_uuid = $1 AND store_uuid = $2
     AND business_date::date BETWEEN $3::date AND $4::date
   ```

> ⚠️ **Nhưng ở đây có một lỗi hiệu năng thật, rất đáng nói:**
> `business_date` khai báo là `string` (kiểu `text`/`varchar`), và query dùng `business_date::date BETWEEN ...`.
> **Ép kiểu trên cột index làm index vô hiệu** — Postgres không thể dùng B-tree trên `text`
> để đánh giá predicate trên `date` → **Seq Scan toàn bảng `orders`**.
>
> **Ba cách sửa (nói ra là ăn điểm to):**
> 1. Đổi cột sang kiểu `date` — đúng nhất.
> 2. Tạo **expression index**: `CREATE INDEX ON orders ((business_date::date));`
> 3. So sánh dạng string: `business_date BETWEEN '2026-07-01' AND '2026-07-31'` — chạy được vì format `YYYY-MM-DD` sort lexicographic = sort theo ngày.
>
> Đây gọi là **non-sargable predicate** (Search ARGument ABLE). Thuộc từ này.

**Và bảng `orders` thì hoàn toàn không có index nào được khai báo** (`order.postgre.ts` chỉ có `@PrimaryGeneratedColumn`), trong khi nó bị query bởi `client_uuid + store_uuid + business_date` ở khắp nơi. Index cần thêm:
```sql
CREATE INDEX idx_orders_tenant_store_date
  ON orders (client_uuid, store_uuid, business_date, order_status_code);
```
(Áp dụng đúng ESR: 2 equality → 1 range → 1 filter.)

### 3.6 🐛 Lỗ hổng SQL Injection thật trong repo

`saas-management-order/src/services/order_sales/orderSales.service.ts:13-27`:

```ts
const query = `
  SELECT store_uuid, business_date, total_sales, store_code
  FROM order_sales
  WHERE store_uuid = '${params.store_uuid}'          // ❌ string interpolation
    AND client_uuid = '${params.client_uuid}'        // ❌
    AND business_date >= '${params.start_date}'      // ❌
    AND business_date <= '${params.end_date}'        // ❌
`;
const result = await client.query(query);
```

So sánh với cách làm **đúng** ở `abcReport.service.ts` (cùng repo!):
```ts
await AppDataSource.query(ordersQuery, [client_uuid, payload.store_uuid, from, to]);  // ✅ $1,$2,$3,$4
```

> Nếu được hỏi *"bạn từng tìm ra lỗ hổng bảo mật nào chưa?"* → đây là câu trả lời.
> Nói thêm: nó **giảm nhẹ** vì input đi qua Zod validation ở middleware, nhưng đó là *defense in depth*
> chứ không phải fix. Prepared statement còn cho lợi ích phụ: **plan cache** ở phía Postgres.

### 3.7 Transaction — có dùng

`saas-management-reports/src/services/pricingTier/pricingTier.service.ts:177+`:
```ts
const queryRunner = AppDataSource.createQueryRunner();
await queryRunner.connect();
await queryRunner.startTransaction();
try {
  const repository = queryRunner.manager.getRepository(PricingTier);
  // 1. gỡ product khỏi tier cũ
  // 2. thêm product vào tier mới
  await queryRunner.commitTransaction();
} catch { await queryRunner.rollbackTransaction(); }
finally { await queryRunner.release(); }
```

Đây là bài toán **"chuyển sản phẩm giữa 2 bậc giá"** — phải atomic, nếu không sản phẩm sẽ nằm ở cả 2 tier hoặc biến mất khỏi cả hai. Kinh điển của ACID.

> Postgres mặc định **READ COMMITTED**. Với logic read-modify-write kiểu này (đọc tier cũ → sửa → ghi),
> READ COMMITTED **không** chống được lost update khi 2 request đồng thời.
> Cần `SELECT ... FOR UPDATE` (pessimistic lock) hoặc nâng lên `REPEATABLE READ` + retry khi serialization failure.
> **Repo chưa làm điều này** → có race condition. Nêu ra được là bạn đã tư duy ở mức senior.

Query này còn dùng **JSONB operator**:
```ts
.andWhere('pricing_tier.product_uuids::jsonb ? :productUuid', { productUuid })
```
Toán tử `?` = "JSONB có chứa key/element này không". Cần **GIN index** mới nhanh:
```sql
CREATE INDEX ON pricing_tier USING GIN (product_uuids jsonb_path_ops);
```

---

## 4. ⭐ Redis — KHÔNG dùng. Và đây là câu hỏi hay nhất của bạn.

### 4.1 Xác nhận bằng chứng

Đã kiểm tra:
- `package.json` của **cả 16 service Node**: không service nào có `redis` / `ioredis` / `cache-manager` / `bullmq`.
- `go.mod` của **cả 14 service Go**: không có `go-redis` / `redigo`.
- Không có dòng code nào import/gọi Redis.
- Duy nhất `saas-management-reports/package-lock.json` có chữ `ioredis` — nhưng đó là **optional peer dependency của TypeORM** (cho tính năng query cache của nó), **không được cài, không được dùng**.

### 4.2 Cách trả lời khi bị hỏi "sao không dùng Redis?"

**Đừng nói "vì chưa cần".** Hãy nói thế này:

> "Hệ thống hiện tại chưa có Redis. Mỗi nhu cầu mà thông thường người ta giải bằng Redis
> đều đang được giải bằng một component khác — và em nghĩ **một số chỗ là hợp lý, một số chỗ là nợ kỹ thuật**."

Rồi trình bày bảng này:

| Nhu cầu | Cách chuẩn (Redis) | Repo này làm gì | Đánh giá |
|---|---|---|---|
| Hết hạn dữ liệu tạm | `EXPIRE`, `SETEX` | Mongo **TTL index** `expireAfterSeconds` (`customerSyncLog`) | ✅ Hợp lý. Trade-off: Mongo TTL chạy background mỗi 60s → không chính xác tuyệt đối |
| Hết hạn message | Redis TTL | RabbitMQ `x-message-ttl` (24h), `expiration` per-message (2h) | ✅ Hợp lý — đúng chỗ hơn Redis |
| Delayed job / retry backoff | Sorted Set `ZADD score=ts`, BullMQ | RabbitMQ plugin `x-delayed-message` (7 service) | ✅ Hợp lý — bớt được 1 hạ tầng |
| Sinh số tuần tự (mã vạch, số phiếu) | `INCR` — atomic, ~µs | Mongo collection `counter` / `barcodeCounter` + `findOneAndUpdate` với `$inc` | 🟡 Chạy được (Mongo `$inc` cũng atomic) nhưng chậm hơn nhiều và tốn 1 round-trip DB |
| **Cache dữ liệu đọc nhiều** | `GET/SET` + TTL | **KHÔNG CÓ GÌ** | ❌ **Nợ kỹ thuật lớn nhất** |
| **Xác thực token** | Cache session/JWT trong Redis | **gRPC call sang user-service MỖI request** | ❌ **Nghiêm trọng nhất** |
| Rate limiting | `INCR` + `EXPIRE`, sliding window | Không có | ❌ Thiếu |
| Distributed lock | `SET NX PX` / Redlock | Không có (dựa vào unique index của DB) | 🟡 Unique index là "optimistic lock" — chấp nhận được, nhưng lỗi mới biết |
| Session store | Redis | JWT stateless + gRPC verify | 🟡 |

### 4.3 Chỗ Redis sẽ tạo tác động lớn nhất — `verifyToken`

Đọc `CLAUDE.md` của inventory, mục *Middleware stack*:

> "**`verifyToken`** (global, applied in `src/server.ts`) — decodes the `x-token` header
> **via a gRPC call to the `user` service**"

**Nghĩa là: MỌI request HTTP đến MỌI service đều phát sinh 1 gRPC call mạng sang `user-service`.**

Hệ quả — nói được đủ 4 ý này là rất mạnh:

1. **Latency cộng dồn.** Mỗi request +1 network hop. gRPC nội bộ ~2-5ms → p99 có thể +20ms.
2. **Single point of failure.** `user-service` chết → **toàn bộ hệ thống** ngừng xác thực. Không phải degrade, mà là sập.
3. **Amplification.** 16 service × QPS mỗi service → `user-service` gánh tổng QPS của cả hệ thống. Nó phải scale bằng tổng tất cả.
4. **Lãng phí.** Cùng một token được decode đi decode lại hàng nghìn lần trong vòng đời của nó.

**Đề xuất (nói ra như một giải pháp có cân nhắc, không phải "cứ cắm Redis vào"):**

```
Cache key:  auth:token:<sha256(token)>
Value:      { user_uuid, client_uuid, permissions[] }
TTL:        min(60s, thời gian còn lại của token)
```

- TTL 60s → cân bằng giữa hiệu quả cache và độ trễ khi thu hồi quyền.
- Muốn thu hồi tức thì: publish event `auth.token.revoked` qua RabbitMQ (**hạ tầng đã có sẵn!**) → các service xoá cache. Đây là **cache invalidation qua pub/sub** — ghép được với mục 2.
- Rẻ hơn nữa mà không cần Redis: **in-process LRU cache** (`lru-cache`) TTL 30s ngay trong mỗi service. Không cần thêm hạ tầng, đổi lại mỗi pod có bản cache riêng.

> Việc bạn đưa ra được **cả phương án không-Redis** cho thấy bạn chọn công cụ theo bài toán, không theo trend. Đây là thứ interviewer senior tìm kiếm.

### 4.4 Chỗ thứ hai — cache report

Các báo cáo trong `saas-management-reports` (ABC report, hourly sales, cash count...) đều là:
- Query **nặng** (quét `order_details` với window function)
- Dữ liệu **quá khứ, bất biến** — báo cáo doanh thu tháng 6 sẽ không bao giờ đổi
- Được xem **lặp đi lặp lại** bởi nhiều người trong cùng tenant

→ Đây là **cache candidate hoàn hảo**: hit rate sẽ rất cao.

```
Key:  report:abc:<client_uuid>:<store_uuid>:<from>:<to>:<category>
TTL:  ngày quá khứ → 24h (thậm chí vô hạn)
      có chứa hôm nay → 5 phút
```

Chiến lược: **cache-aside (lazy loading)**. Nói được cả 3 pattern thì tốt:
- **Cache-aside**: app đọc cache, miss thì query DB rồi ghi cache. Phổ biến nhất, phù hợp ở đây.
- **Write-through**: ghi DB và cache cùng lúc. Consistency tốt, ghi chậm.
- **Write-behind**: ghi cache trước, flush DB sau. Nhanh nhất, rủi ro mất dữ liệu.

Và 3 vấn đề kinh điển của cache — chuẩn bị sẵn:
- **Cache stampede/avalanche**: nhiều request cùng miss 1 key → cùng đập vào DB. Fix: lock (`SET NX`) hoặc TTL jitter.
- **Cache penetration**: query key không tồn tại liên tục → luôn miss. Fix: cache cả giá trị null, hoặc Bloom filter.
- **Cache invalidation**: "một trong hai vấn đề khó nhất trong khoa học máy tính". Ở đây giải bằng event RabbitMQ.

---

## 5. Các kỹ thuật khác trong repo (bảng tra nhanh)

| Kỹ thuật | Số lần | Service | Lý thuyết đằng sau |
|---|---|---|---|
| **gRPC** | rất nhiều | Toàn bộ | RPC nhị phân trên HTTP/2, protobuf. Nhanh hơn REST/JSON, có contract chặt. Dùng cho **đồng bộ**; RabbitMQ cho **bất đồng bộ** |
| `.lean()` | 667 | Toàn bộ | Mongoose trả **plain JS object** thay vì Document → bỏ qua việc dựng getter/setter/change-tracking. Nhanh 3-5x, tốn ít RAM. Đổi lại: mất `.save()`, mất virtuals |
| `insertMany` | 45 | 6 service | Batch insert — 1 round-trip thay vì N. Giảm chi phí mạng, không giảm chi phí ghi đĩa |
| Aggregation Pipeline | 30 | 6 service | `$match → $group → $project`. **Luôn đặt `$match` đầu tiên** để dùng index; sau `$group` là index vô dụng |
| `upsert: true` | 29 | 6 service | Update-or-insert atomic. **Nền tảng của idempotency** cho consumer at-least-once |
| `bulkWrite` | 26 | 4 service | Gộp nhiều loại thao tác (insert/update/delete) trong 1 request. `ordered: false` cho phép chạy song song |
| `$lookup` | 19 | 5 service | "JOIN" của Mongo — thực chất là nested loop, không có hash join. **Đây chính là lý do report phải sang Postgres** |
| `partialFilterExpression` | 18 | 4 service | Partial index — xem mục 1.3-C |
| `prefetch`/`Qos` | ~20 | Hầu hết | Flow control của AMQP — xem mục 2.5-a |
| `x-delayed-message` | 7 | 7 service Go | Delayed exchange plugin |
| `allowDiskUse` | 10 | order | Aggregation vượt trần 100MB RAM → spill ra đĩa. **Dấu hiệu Mongo đang bị dùng sai mục đích** |
| `$facet` | 2 | inventory, system | Chạy nhiều pipeline song song trên cùng input — thường dùng để lấy `items` + `total` trong 1 query (pagination) |
| TTL index | 2 | customer, inventory | Xem mục 1.4 |
| Transaction | 1 | reports | Xem mục 3.7 |
| Soft delete | toàn bộ | Toàn bộ | `is_delete: 0/1` thay vì xoá thật. Giữ được audit trail, nhưng làm bảng phình và index kém hiệu quả |
| Multi-tenancy | toàn bộ | Toàn bộ | **Shared database, shared schema** — phân tách logic bằng `client_uuid`. Rẻ nhất, nhưng rủi ro leak cao nhất nếu quên `WHERE client_uuid` |
| Connection pooling | — | order | Mongo: `maxPoolSize: 50, minPoolSize: 10`. Postgres: `Pool({ max: 20 })` |

---

## 6. Bộ câu hỏi phỏng vấn + gợi ý trả lời

### Q1. "Compound index hoạt động thế nào? Thứ tự field có quan trọng không?"
→ Mục 1.1. Nói **prefix rule** + **ESR**. Rồi lấy ví dụ thật:
`{client_uuid, store_uuid, order_time, is_delete}` ở `order.model.ts:294` — chỉ ra rằng `is_delete`
đứng sau range field `order_time` nên **không dùng được để seek**, và đề xuất cách sửa.
> Đây là câu trả lời "biết code của mình", khác hẳn câu trả lời học thuộc.

### Q2. "Làm sao ép unique nhưng vẫn cho soft-delete rồi tạo lại?"
→ Mục 1.3-C. **Partial index**. Repo dùng 18 lần. Giải thích cơ chế: chỉ index document thoả điều kiện.
Bonus: index nhỏ hơn → ít RAM, ghi nhanh hơn.

### Q3. "Một message trong RabbitMQ có thể vào nhiều queue không? Khi nào?"
→ Mục 2.1 + 2.3. **Có** — khi ≥2 binding cùng khớp. Exchange **copy** message.
Phân biệt rõ với **nhiều consumer trên 1 queue** = round-robin, mỗi message 1 consumer.
Rồi kể `audit_log_exchange`: hiện tại binding không giao nhau nên chưa xảy ra, nhưng chọn `topic` là để mở đường.

### Q4. "Hệ thống bạn đảm bảo message không mất thế nào?"
→ Mục 2.5. Kể đủ chuỗi: `durable` exchange/queue → `persistent` message → `prefetch` → manual `ack`.
Rồi **tự nêu 2 lỗ hổng**: chưa có Publisher Confirms, chưa có DLQ.
> Tự chỉ ra điểm yếu của hệ thống mình = tín hiệu senior mạnh nhất.

### Q5. "Tại sao dùng 2 database? Không phức tạp quá sao?"
→ Mục 3.1-3.3. **Polyglot persistence**: OLTP (Mongo) vs OLAP (Postgres).
Nhấn mạnh lý do **cách ly tải**: report nặng không được phép làm chậm luồng bán hàng.
Bằng chứng: `allowDiskUse` 10 lần → đã chạm trần 100MB của Mongo aggregation.
Thừa nhận cái giá phải trả: eventual consistency giữa 2 store, phải build ETL, phải vận hành 2 hệ.

### Q6. "Materialized view là gì? Sao không dùng?"
→ Mục 3.4. Định nghĩa: view lưu kết quả vật lý xuống đĩa, phải `REFRESH` mới cập nhật.
Lý do không dùng: `REFRESH` khoá ACCESS EXCLUSIVE; `CONCURRENTLY` cần unique index và chậm; MV refresh **toàn bộ** trong khi dữ liệu ở đây là append-only theo ngày → chỉ cần incremental.
Cách thay thế: bảng tổng hợp tự quản, cập nhật bằng event RabbitMQ.

### Q7. "Redis dùng để làm gì? Hệ thống bạn dùng chưa?"
→ Mục 4. Trả lời trung thực **chưa dùng**, nhưng chỉ ra từng nhu cầu đang được giải bằng gì
(TTL index, x-delayed-message, x-message-ttl), rồi chỉ ra 2 chỗ **nên** có Redis:
`verifyToken` (gRPC mỗi request) và cache report.
Kết bằng phương án không-Redis (in-process LRU) để thể hiện tư duy đánh đổi.

### Q8. "Index nào trong hệ thống bạn thấy cần tối ưu?"
→ Ba câu trả lời sẵn:
1. `{is_delete, is_active}` (14 chỗ) — cardinality quá thấp, gần như vô dụng.
2. `is_delete` đặt sau range field trong `order.model.ts` — sai ESR.
3. Bảng `orders` trong Postgres **không có index nào**, mà `business_date::date` lại non-sargable → Seq Scan. Đưa ra câu `CREATE INDEX` cụ thể.

### Q9. "At-least-once delivery nghĩa là gì? Xử lý thế nào?"
→ Manual ack + `persistent` → message có thể được giao **lại** nếu consumer chết trước khi ack.
→ Hệ quả: **handler phải idempotent**.
→ Repo giải bằng: unique compound index (`idx_order_details_uniq`, `uq_product_quantity_forecast`) + `upsert: true` (29 chỗ).
Nối luôn sang exactly-once: "exactly-once *delivery* là bất khả thi trong hệ phân tán; cái đạt được là exactly-once *processing* = at-least-once + idempotency."

### Q10. "Multi-tenant các bạn tách dữ liệu thế nào?"
→ **Shared DB, shared schema**, phân tách bằng `client_uuid` ở mọi query.
Rủi ro: quên 1 chỗ `WHERE client_uuid` là leak chéo tenant.
Repo giảm thiểu bằng: `client_uuid` là field **đầu tiên** trong hầu hết compound index (ép developer nghĩ về nó), và convention `user.getClientUuid()` ở mọi HTTP handler.
Điểm sáng: exchange `ex.out_of_stock.<client_uuid>` — cách ly tenant ngay ở tầng broker.

---

## 7. Sơ đồ tổng để vẽ lên bảng trắng

```
                          ┌──────────────┐
   HTTP  ───────────────► │ saas-        │
                          │ management-* │
                          │ (Express/TS) │
                          └──┬────┬───┬──┘
             gRPC (đồng bộ)  │    │   │  AMQP (bất đồng bộ)
        ┌──────────────────◄─┘    │   └─────────────────┐
        ▼                         ▼                     ▼
  ┌───────────┐            ┌───────────┐        ┌──────────────┐
  │  user-svc │            │  MongoDB  │        │  RabbitMQ    │
  │ verifyTkn │            │  (OLTP)   │        │              │
  │           │            │           │        │ audit_log_ex │
  │ ❌ gọi    │            │ 121 compound│      │ OSS-PRODUCT  │
  │ MỖI req   │            │ index      │       │ OSS-INVENTORY│
  │ → nên     │            │ TTL index  │       │ ex.out_of_   │
  │   cache!  │            │ partial idx│       │  stock.<tid> │
  └───────────┘            └─────┬─────┘        └───┬──────┬───┘
                                 │                  │      │
                          ETL ───┘                  │      │
                                 ▼                  ▼      ▼
                          ┌──────────────┐    ┌─────────┐ ┌──────────┐
                          │  PostgreSQL  │    │ Go      │ │ POS/KDS  │
                          │  (OLAP)      │◄───┤ micro-  │ │ (client) │
                          │              │    │ services│ └──────────┘
                          │ orders       │    └─────────┘
                          │ order_details│
                          │ + 8 bảng     │
                          │  "materialized"│
                          │ window func  │
                          │ CTE, JSONB   │
                          └──────────────┘

  ❌ KHÔNG CÓ: Redis · Dead Letter Queue · Publisher Confirms · Materialized View
```

---

## 8. Danh sách file cần mở lại khi ôn

| Chủ đề | File |
|---|---|
| Compound + partial index đẹp nhất | `saas-management-menu/src/services/product/product.model.ts:295-297` |
| Compound index đúng ESR nhất | `saas-management-inventory/src/services/inventoryLog/inventoryLog.model.ts:86` |
| Compound index sai ESR (để phê bình) | `saas-management-order/src/services/order/order.model.ts:294` |
| TTL index | `saas-management-customer/src/services/customerSyncLog/customerSyncLog.model.ts:54` |
| **Topic exchange + 4 binding** | `saas-microservice-system/service/queue/auditLogSetup.go:16-75` |
| Producer audit log + payload | `saas-management-menu/src/middleware/auditLog.ts:129-185` |
| Consumer chỉ start 2/4 queue | `saas-microservice-system/main.go:98-107` |
| Exchange per-tenant + per-msg TTL | `saas-management-inventory/src/services/outOfStock/outOfStock.service.ts:15-26` |
| Wrapper RabbitMQ (prefetch, 2 channel) | `saas-management-reports/src/lib/rabbitMQ.lib.ts` |
| Đăng ký 6 consumer | `saas-management-inventory/src/index.ts:61-104` |
| Publisher Go + payload order | `saas-microservice-order-pos/services/orderIntegrate/publishMessage.go` |
| **SQL window function + CTE** | `saas-management-reports/src/services/abcReport/abcReport.service.ts:66-115` |
| Star schema Postgres | `saas-management-reports/src/models/order.postgre.ts` |
| Unique index cho idempotency | `saas-management-reports/src/models/orderDetail.postgre.ts:6-10` |
| Transaction + JSONB | `saas-management-reports/src/services/pricingTier/pricingTier.service.ts:177+` |
| 🐛 SQL injection | `saas-management-order/src/services/order_sales/orderSales.service.ts:13-27` |
