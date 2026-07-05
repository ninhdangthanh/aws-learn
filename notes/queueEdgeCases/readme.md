# Queue Edge Cases: Ordering, Idempotency, Retry/DLQ

Ghi chú cho các edge case hay gặp khi consume event qua RabbitMQ trong hệ
thống order: message đến sai thứ tự, duplicate delivery, và cách retry an
toàn bằng delay queue + DLQ. Phần lý thuyết RabbitMQ nền tảng (exchange,
ack/nack, prefetch, TTL+DLX, publisher confirm...) xem ở
[`../rabbitmq-middle-notes.md`](../rabbitmq-middle-notes.md), đặc biệt mục
7-11 (Retry/DLX, Poison Message, Idempotent Consumer, Ordering). File này
tập trung vào case cụ thể: **OrderCancelled đến trước OrderCreated**, và có
kèm lab Go để tái hiện retry 10s/30s → DLQ.

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

`order-events.retry.5s`, `.retry.30s`, `.retry.5m`, `.dlq` là các queue phụ
để retry có delay khi thiếu context (không phải lỗi hệ thống).

Có 2 loại thông tin, và **RabbitMQ không quản lý business logic**:

| Ai quản lý | Thông tin |
|---|---|
| RabbitMQ | `x-message-ttl`, `x-dead-letter-exchange`, routing — chỉ biết "queue này chờ bao lâu, hết hạn thì đẩy đi đâu" |
| Application | `retry_count`, `event_id`, `event_version` — RabbitMQ không tự tăng, consumer phải tự đọc header, `+1`, rồi publish lại |

```text
Consumer đọc header.retry_count
  -> quyết định publish sang queue nào
  -> retry.5s (TTL=5s)
       -> RabbitMQ tự chờ hết TTL
       -> dead-letter về queue chính
       -> consumer đọc lại, retry_count đã +1
```

Có thể tự set `retry_count` trong header (dễ kiểm soát logic, cách team hay
dùng) hoặc dùng `x-death` mà RabbitMQ tự ghi lại lịch sử dead-letter. Cả
hai đều phổ biến trên production.

## 6. Nguyên tắc khi dùng retry queue trên production

- **Không retry vô hạn** — luôn có ngưỡng, ví dụ `retry_count >= 3 -> DLQ`,
  nếu không message lỗi quay vòng mãi và nghẽn hệ thống.
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

## 7. Lab: tái hiện retry 10s → 30s → DLQ (Go)

Lab này dựng lại đúng cơ chế TTL + dead-letter ở mục 5, rút gọn còn 2 tầng
delay (10s, 30s) để dễ quan sát, rồi in message ra ở DLQ để **manual check**
thay vì tự động xử lý lại — giống một bước "chờ người vận hành xem" trên
production trước khi quyết định bỏ/replay/report bug.

### Cấu trúc lab

```
docker-compose.yml   # RabbitMQ 3.13 management, cổng 5682/15682
go.mod
internal/mq/
  conn.go       # Dial() mở connection + channel + đảm bảo topology
  topology.go   # tên queue, TTL/dead-letter args, NextQueueForRetry()
cmd/
  producer/    # publish 1 message test vào order-events (retry_count=0)
  consumer/    # LUÔN coi message là thiếu context, escalate theo retry_count
  dlqwatcher/  # consume DLQ, in ra để manual check, rồi ACK
```

### Topology dùng default exchange, không cần exchange riêng

Publish thẳng vào queue qua default exchange (routing key = tên queue).
Queue retry tự dead-letter về lại `order-events` khi hết TTL nhờ
`x-dead-letter-exchange=""` + `x-dead-letter-routing-key=order-events`, và
`order-events` tự dead-letter sang `order-events.dlq` khi consumer `nack`:

```text
order-events              queue chính, DLX -> order-events.dlq (cho nack)
order-events.retry.10s    x-message-ttl=10000  -> dead-letter về order-events
order-events.retry.30s    x-message-ttl=30000  -> dead-letter về order-events
order-events.dlq          queue chứa message hết lượt retry
```

Logic escalate ở consumer (`retry_count` nằm trong header, do app tự tăng):

```text
retry_count=0 -> publish order-events.retry.10s (đợi 10s)
retry_count=1 -> publish order-events.retry.30s (đợi 30s)
retry_count=2 -> hết lượt retry -> nack(requeue=false) -> DLX đẩy về order-events.dlq
```

Hai chặng dùng hai cơ chế khác nhau có chủ đích:

- **Chặng retry** (`.retry.10s`/`.retry.30s`): **publish tay** vì đích thay đổi
  theo `retry_count`. `nack` chỉ đẩy được về một DLX cố định nên không route
  động nhiều tầng được.
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

Theo dõi terminal 1: message được nhận, publish sang `retry.10s`; đợi 10s,
RabbitMQ tự dead-letter về `order-events`, consumer nhận lại
(`retry_count=1`), publish sang `retry.30s`; đợi 30s, quay lại lần nữa
(`retry_count=2`), hết lượt retry → `nack(requeue=false)`, RabbitMQ tự
dead-letter về `order-events.dlq`.

Terminal 2 sẽ in ra message cuối cùng kèm headers (`retry_count=2`,
`event_id`, `event_type`, và `x-death` do broker tự ghi) — đúng bước
"manual check" mô phỏng người vận hành soi DLQ.

### Dọn dẹp

```bash
docker-compose down -v
```
