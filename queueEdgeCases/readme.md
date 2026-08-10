# Queue Edge Cases: Ordering, Idempotency, Retry/DLQ

Ghi chú cho các edge case hay gặp khi consume event qua RabbitMQ trong hệ
thống order: message đến sai thứ tự, duplicate delivery, và cách retry an
toàn bằng delay queue + DLQ. Phần lý thuyết RabbitMQ nền tảng (exchange,
ack/nack, prefetch, TTL+DLX, publisher confirm...) xem ở
[`../rabbitmq-middle-notes.md`](../rabbitmq-middle-notes.md), đặc biệt mục
7-11 (Retry/DLX, Poison Message, Idempotent Consumer, Ordering). File này
tập trung vào case cụ thể: **OrderCancelled đến trước OrderCreated**, và có
kèm lab Go để tái hiện retry 30s → 30m → DLQ bằng một retry queue duy nhất.

## 1. Vấn đề: event đến sai thứ tự hoặc duplicate

RabbitMQ không đảm bảo thứ tự tuyệt đối giữa các message thuộc cùng 1
business entity nếu chúng nằm ở các queue/consumer khác nhau, và có thể
deliver lại message nếu consumer xử lý xong nhưng chưa kịp ack, hoặc
consumer crash giữa chừng.

Ví dụ: consumer có thể nhận `OrderCancelled` trước `OrderCreated`.

Hai việc cần làm riêng biệt:

- **Duplicate delivery** → xử lý bằng idempotent handler.
- **Event thiếu context** (đến đúng 1 lần nhưng sai thứ tự) → xử lý bằng
  retry/defer, không phải idempotency.

## 2. Idempotent handler

Mỗi event nên có `event_id` để nhận diện duy nhất:

```json
{
  "event_id": "evt_abc",
  "order_id": "123",
  "event_type": "OrderCancelled",
  "version": 2
}
```

Consumer lưu bảng `processed_events(event_id)` và xử lý trong 1 transaction:

```text
BEGIN TX
  check event_id đã xử lý chưa -> nếu rồi: ACK, bỏ qua
  check version/state
  update order projection
  insert processed_events
COMMIT
ACK
```

Không ACK trước khi ghi DB xong — nếu ACK sớm rồi crash, message mất mà
side effect chưa chắc đã chạy.

## 3. Ordering: version/sequence + state machine

Retry queue chỉ giúp "chờ thêm một chút". Cái đảm bảo đúng nghiệp vụ là:

```text
expected_version = current_version + 1
```

Nếu event nhảy cóc (`event_version` > `expected_version`) → defer/retry.
Nếu sau nhiều lần vẫn thiếu version còn thiếu → đưa vào DLQ.

**Flow tổng thể của 1 consumer:**

```text
Consumer nhận event
  -> check event_id đã xử lý chưa (idempotency)
  -> load aggregate/projection hiện tại
  -> check version
       - version cũ         -> ACK, bỏ qua
       - version nhảy cóc   -> defer/retry
       - version đúng       -> tiếp tục
  -> check state machine
       - transition không hợp lệ -> defer hoặc DLQ
       - hợp lệ                  -> update DB
  -> insert processed_event
  -> commit transaction
  -> ACK RabbitMQ
```

**Ví dụ cụ thể:**

DB hiện tại: `order_id=123, current_version=0, status=NONE`.

1. Nhận `OrderCancelled version=2` → expected là 1 → thiếu `OrderCreated`
   → publish sang retry queue, ACK message hiện tại.
2. Nhận `OrderCreated version=1` → version đúng, `NONE -> CREATED` hợp lệ
   → `status=CREATED, current_version=1`, ACK.
3. Retry lại `OrderCancelled version=2` → version đúng,
   `CREATED -> CANCELLED` hợp lệ → `status=CANCELLED, current_version=2`,
   ACK.

## 4. Một queue có nên chỉ 1 consumer?

Nếu một queue có nhiều consumer chạy song song, 2 message của cùng 1
`order_id` có thể được xử lý đồng thời bởi 2 consumer khác nhau — mất thứ
tự dù đã có version check ở tầng app (race condition khi cả hai đọc cùng
version cũ trước khi cái nào update trước).

Muốn giữ thứ tự mạnh cho từng entity, nên route theo lane:

```text
order-events-0 -> consumer-0
order-events-1 -> consumer-1
order-events-2 -> consumer-2
order-events-3 -> consumer-3
```

hoặc tối thiểu set `prefetch=1` mỗi consumer, để nó xử lý tuần tự.

## 5. Retry có delay: ai quản lý gì

Retry có delay khi thiếu context (không phải lỗi hệ thống) cần một chỗ để
message "nằm chờ". Có 2 loại thông tin, và **RabbitMQ không quản lý business
logic**:

| Ai quản lý | Thông tin |
|---|---|
| RabbitMQ | `x-message-ttl`, `x-dead-letter-exchange`, routing — chỉ biết "queue này chờ bao lâu, hết hạn thì đẩy đi đâu" |
| Application | `retry_tick`, `event_id`, `event_version` — RabbitMQ không tự tăng, consumer phải tự đọc header, `+1`, rồi publish lại |

Cách hay gặp là **mỗi mốc delay một queue** (`.retry.5s`, `.retry.30s`,
`.retry.5m`...), consumer đọc `retry_count` rồi chọn queue tương ứng. Dễ
hiểu, nhưng thêm một mốc delay là thêm một queue phải khai báo và maintain.

### Vì sao không dùng per-message TTL để gộp về 1 queue

Ý tưởng đầu tiên ai cũng nghĩ tới: một queue duy nhất, mỗi message tự mang
TTL riêng (`expiration`). **Không dùng được.** RabbitMQ chỉ kiểm tra hết hạn
ở **đầu queue**, nên một message TTL 30 phút nằm đầu sẽ chặn message TTL 30s
nằm sau nó — head-of-line blocking, message ngắn phải chờ message dài.

Cách gộp về 1 queue mà vẫn đúng là giữ **TTL cố định cho mọi message**, rồi
cho message **quay vòng** qua queue đó nhiều lần để tạo delay dài hơn. Cùng
TTL thì thứ tự hết hạn trùng thứ tự vào queue, không bao giờ kẹt. Đây là cơ
chế lab ở mục 7 dùng.

```text
Consumer đọc header.retry_tick
  -> chưa tới mốc? đẩy thẳng lại retry queue, tick+1
  -> tới mốc?      xử lý business; fail thì cũng đẩy lại, tick+1
       -> RabbitMQ tự chờ hết TTL (30s)
       -> dead-letter về queue chính
       -> consumer đọc lại, retry_tick đã +1
```

Có thể tự set counter trong header (dễ kiểm soát logic, cách team hay dùng)
hoặc dùng `x-death` mà RabbitMQ tự ghi lại lịch sử dead-letter. Cả hai đều
phổ biến trên production.

## 6. Nguyên tắc khi dùng retry queue trên production

- **Không retry vô hạn** — luôn có ngưỡng (số lần retry, hoặc tổng thời gian
  sống của message), nếu không message lỗi quay vòng mãi và nghẽn hệ thống.
- **Chỉ retry lỗi tạm thời**: thiếu context (`OrderCreated` chưa tới), DB
  timeout, dependency tạm lỗi, lock conflict, network issue. **Không** retry
  poison message (schema sai, `order_id` invalid, transition chắc chắn
  không hợp lệ) — đưa DLQ ngay.
- **Đừng `nack(requeue=true)` liên tục** cho lỗi thiếu context — dễ tạo
  vòng lặp `fail -> requeue -> consume lại ngay -> fail...`, làm CPU cao và
  nghẽn queue. Publish sang retry queue có delay rồi ACK message cũ.
- **Idempotency + version/sequence vẫn là bắt buộc** — retry/DLQ chỉ là cơ
  chế chờ và cách ly lỗi, không thay thế được state machine đúng.
- **Log + alert khi vào DLQ** — DLQ nên là nơi con người biết để nhìn vào,
  không phải nơi message biến mất im lặng.

## 7. Lab: một retry queue duy nhất, thang 30s → 30m → DLQ (Go)

Lab này dựng lại đúng cơ chế TTL + dead-letter ở mục 5, nhưng gộp về **một
retry queue duy nhất TTL 30s** thay vì mỗi mốc một queue, rồi in message ra
ở DLQ để **manual check** thay vì tự động xử lý lại — giống một bước "chờ
người vận hành xem" trên production trước khi quyết định bỏ/replay/report bug.

### Cấu trúc lab

```
docker-compose.yml   # RabbitMQ 3.13 management, cổng 5682/15682
go.mod
internal/mq/
  conn.go       # Dial() mở connection + channel + đảm bảo topology
  topology.go   # tên queue, TTL/dead-letter args, RetryTickMarks, ActionForTick()
cmd/
  producer/    # publish 1 message test vào order-events (retry_tick=0)
  consumer/    # LUÔN coi message là thiếu context, chạy thang retry theo retry_tick
  dlqwatcher/  # consume DLQ, in ra để manual check, rồi ACK
```

### Topology dùng default exchange, không cần exchange riêng

Publish thẳng vào queue qua default exchange (routing key = tên queue).
Queue retry tự dead-letter về lại `order-events` khi hết TTL nhờ
`x-dead-letter-exchange=""` + `x-dead-letter-routing-key=order-events`, và
`order-events` tự dead-letter sang `order-events.dlq` khi consumer `nack`:

```text
order-events          queue chính, DLX -> order-events.dlq (cho nack)
order-events.retry    x-message-ttl=30000  -> dead-letter về order-events
order-events.dlq      queue chứa message hết lượt retry
```

Thêm mốc delay mới **không cần thêm queue** — chỉ sửa một mảng.

### Thang retry: mốc tích luỹ, một biến đếm

`retry_tick` là số vòng 30s message đã đi qua kể từ lần fail đầu tiên.
`RetryTickMarks` là các **mốc tích luỹ** mà tại đó consumer được phép xử lý lại:

```go
var RetryTickMarks = []int{1, 2, 4, 10, 20, 60} // 30s, 1m, 2m, 5m, 10m, 30m
```

Consumer chỉ cần đọc một biến đó rồi rẽ 3 nhánh:

```text
tick=0                  -> message mới từ producer, xử lý luôn
tick ∈ {1,2,4,10,20,60} -> tới mốc, xử lý lại
tick ở giữa hai mốc     -> bỏ qua business, đẩy thẳng lại order-events.retry
tick > 60               -> hết lượt -> nack(requeue=false) -> DLX đẩy về order-events.dlq
```

Message vẫn quay vòng ở **mọi** tick (3, 5, 6, 7...), consumer chỉ nhìn header
rồi đẩy nó trở lại. Tổng vòng đời tối đa = mốc cuối = 60 tick = **30 phút**,
đổi lại **60 lượt hop** qua broker nhưng chỉ **6 lần xử lý thật**.

Đánh đổi có chủ ý: logic milestone nằm ở chính consumer chính, nên một message
chờ 30 phút sẽ đi qua consumer 60 lần và bị bỏ qua 54 lần (với `prefetch=1`,
mỗi lượt no-op vẫn chiếm slot). Đổi lại không cần scheduler process riêng.
Nếu lưu lượng fail lớn, cách giảm hop là thêm **tầng thứ hai** — một retry
queue TTL 5m cho các mốc dài — vẫn không phải mỗi mốc một queue.

Hai chặng dùng hai cơ chế khác nhau có chủ đích:

- **Chặng retry** (`order-events.retry`): **publish tay** vì `nack` chỉ đẩy được
  về một DLX cố định của main queue (là DLQ), không route sang retry queue được.
- **Chặng cuối vào DLQ**: **`nack(requeue=false)`**, để RabbitMQ tự dead-letter
  về `order-events.dlq` qua DLX của main queue. Atomic (không có khe hở
  publish-rồi-ack có thể nhân đôi message), và broker tự ghi `x-death`
  (reason=`rejected`, count, queue gốc) — đúng thứ bước manual check cần.

### Chạy lab

```bash
docker-compose up -d   # RabbitMQ ở localhost:5682 (management UI: localhost:15682, guest/guest)
```

Mở 2 terminal chạy nền:

```bash
go run ./cmd/consumer    # terminal 1: xử lý order-events, tự escalate retry
go run ./cmd/dlqwatcher  # terminal 2: theo dõi DLQ, in ra để manual check
```

Terminal 3 — bắn 1 message test:

```bash
go run ./cmd/producer
```

Theo dõi terminal 1: message được nhận (`tick=0`), park sang
`order-events.retry`; đợi 30s, RabbitMQ tự dead-letter về `order-events`,
consumer nhận lại ở `tick=1` — đúng mốc 30s nên xử lý lại, fail, park tiếp.
`tick=2` là mốc 1m, xử lý lại. `tick=3` chưa tới mốc nên bị đẩy thẳng lại
không xử lý. Cứ thế tới `tick=60` (mốc 30m) là lần xử lý cuối; `tick=61`
vượt mốc cuối → `nack(requeue=false)`, RabbitMQ tự dead-letter về
`order-events.dlq`.

Chạy hết thang mất **30 phút**. Muốn xem nhanh thì tạm hạ `RetryTick` xuống
`2 * time.Second` trong `internal/mq/topology.go`, hoặc rút ngắn
`RetryTickMarks` — nhớ **xoá queue `order-events.retry`** ở management UI
trước khi chạy lại, vì đổi `x-message-ttl` của queue đã tồn tại sẽ làm
`QueueDeclare` fail với `PRECONDITION_FAILED`.

Terminal 2 sẽ in ra message cuối cùng kèm headers (`retry_tick=61`,
`event_id`, `event_type`, và `x-death` do broker tự ghi) — đúng bước
"manual check" mô phỏng người vận hành soi DLQ.

### Dọn dẹp

```bash
docker-compose down -v
```
