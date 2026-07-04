# RabbitMQ Middle Backend Notes

File này gom kiến thức RabbitMQ ở mức Middle Backend: đủ để giải thích trong phỏng vấn, thiết kế worker production cơ bản, và biết các lỗi thường gặp khi dùng queue.

RabbitMQ không chỉ là "đẩy job vào queue". Cần hiểu message đi qua exchange, routing, queue, consumer, ack/nack, retry, DLQ, prefetch và idempotency.

---

## 1. RabbitMQ dùng để làm gì?

RabbitMQ là message broker. Producer publish message vào broker, consumer lấy message ra xử lý.

Use case phổ biến:

* gửi email/SMS/push notification async;
* xử lý ảnh/video/file sau upload;
* đồng bộ dữ liệu giữa service;
* webhook/event processing;
* POS/offline sync batch;
* report/export job;
* bảo vệ downstream bằng queue và worker limit.

RabbitMQ hợp với task queue, command/event async, routing linh hoạt và workload cần ack/retry rõ ràng. Nếu cần event log dài hạn, replay nhiều consumer group độc lập và throughput stream rất lớn, Kafka thường hợp hơn.

---

## 2. Thành phần cốt lõi

```text
Producer
  -> Exchange
      -> Binding + routing key
          -> Queue
              -> Consumer
```

| Thành phần | Ý nghĩa |
|---|---|
| Producer | App publish message |
| Exchange | Nhận message và quyết định route đi đâu |
| Queue | Lưu message chờ consumer xử lý |
| Binding | Quy tắc nối exchange với queue |
| Routing key | Key producer gắn vào message để exchange route |
| Consumer | Worker đọc message từ queue |
| Ack | Consumer báo xử lý thành công, broker xóa message |
| Nack/Reject | Consumer báo xử lý thất bại |

Điểm dễ nhầm:

* Producer thường publish vào exchange, không publish thẳng vào queue.
* Queue không tự biết nhận message nào nếu chưa bind với exchange.
* Một exchange có thể route tới nhiều queue.
* Một queue có thể nhận message từ nhiều binding.

---

## 3. Exchange Types

### Direct exchange

Route khi routing key match chính xác binding key.

```text
exchange: order.direct
binding: order.created -> queue order-created-worker
message routing_key=order.created -> vào order-created-worker
```

Hợp với task rõ loại:

* `email.send`
* `order.created`
* `payment.refund`

### Topic exchange

Route theo pattern.

Ký tự đặc biệt:

* `*` match đúng 1 word.
* `#` match 0 hoặc nhiều word.

Ví dụ:

```text
binding order.*       match order.created, order.cancelled
binding order.#       match order.created.vn.hcm
binding *.created     match order.created, payment.created
```

Hợp với domain event nhiều loại và cần subscribe theo nhóm.

### Fanout exchange

Broadcast message tới tất cả queue đã bind, bỏ qua routing key.

Hợp với:

* cache invalidation;
* notification nhiều worker khác nhau;
* một event cần nhiều service cùng biết.

Ví dụ:

```text
user.updated
  -> email-projection-queue
  -> search-index-queue
  -> analytics-queue
```

### Headers exchange

Route theo message headers thay vì routing key. Ít dùng hơn direct/topic/fanout vì phức tạp hơn và khó debug hơn.

---

## 4. Queue, Binding Và Routing Key

Ví dụ đặt tên dễ hiểu:

```text
exchange: order.events
type: topic

queue: payment-order-events
binding key: order.created

queue: analytics-order-events
binding key: order.#
```

Khi producer publish:

```text
routing_key = order.created
```

RabbitMQ route message tới:

* `payment-order-events`, vì match `order.created`;
* `analytics-order-events`, vì match `order.#`.

Middle backend cần phân biệt:

* Routing key quyết định message đi tới queue nào.
* Message id/event id dùng cho idempotency, không phải routing.
* Queue name là nơi consumer subscribe.

---

## 5. Ack, Nack, Reject Và At-Least-Once

RabbitMQ thường dùng manual ack cho worker production.

```text
Consumer nhận message
-> xử lý business
-> ghi DB/side effect thành công
-> Ack
-> RabbitMQ xóa message khỏi queue
```

Nếu consumer crash trước khi ack:

```text
Message đang unacked
-> connection/channel đóng
-> RabbitMQ redeliver message cho consumer khác hoặc consumer sau
```

Vì vậy RabbitMQ là **at-least-once delivery** trong flow manual ack: message có thể được xử lý hơn một lần.

### Message đang được consumer 1 xử lý thì consumer 2 có lấy được không?

Không, nếu consumer 1 đã nhận message và message đang ở trạng thái **unacked**, RabbitMQ sẽ không giao cùng message đó cho consumer 2 cùng lúc.

Flow:

```text
Queue có message A
-> RabbitMQ giao A cho consumer 1
-> A chuyển từ ready sang unacked
-> consumer 2 không lấy được A trong lúc A vẫn unacked
```

Consumer 2 chỉ có thể nhận message khác đang `ready` trong queue.

Message A chỉ được giao lại cho consumer khác khi:

* consumer 1 `ack` không xảy ra và connection/channel bị đóng;
* consumer 1 crash;
* consumer 1 `nack/reject` với `requeue=true`;
* broker quyết định redeliver sau một số cơ chế retry/delivery-limit tùy queue type/cấu hình.

Ví dụ:

```text
consumer 1 nhận A
consumer 1 xử lý xong DB nhưng crash trước ACK
RabbitMQ thấy connection mất
-> A quay lại queue/redeliver
-> consumer 2 có thể nhận A
```

Đây là lý do consumer vẫn phải idempotent. RabbitMQ không xử lý cùng một message song song ở 2 consumer trong trạng thái bình thường, nhưng message có thể được xử lý lại sau crash/nack/redelivery.

### Ack

Dùng khi xử lý thành công, hoặc khi message duplicate đã được xử lý trước đó.

### Nack/reject with requeue=true

Message quay lại queue. Cẩn thận vì có thể tạo loop:

```text
consumer lỗi
-> nack requeue
-> nhận lại ngay
-> lỗi tiếp
-> retry storm
```

### Nack/reject with requeue=false

Message bị drop hoặc đi vào DLX nếu queue có cấu hình dead-letter.

Rule thực tế:

* Lỗi transient: retry có delay/backoff, không requeue ngay vô hạn.
* Lỗi permanent/poison message: đưa vào DLQ.
* Duplicate message: ack và bỏ qua.

---

## 6. Prefetch Và Backpressure

Prefetch giới hạn số message unacked mà RabbitMQ giao cho một consumer.

Ví dụ:

```text
prefetch = 10
```

Nghĩa là RabbitMQ giao tối đa 10 message chưa ack cho consumer đó. Khi consumer ack bớt, broker mới giao thêm.

Tại sao cần prefetch:

* tránh một consumer ôm quá nhiều message;
* tạo backpressure khi worker chậm;
* giới hạn số job chạy song song;
* bảo vệ DB/API downstream khỏi bị worker bắn quá tải.

Trade-off:

| Prefetch | Ưu điểm | Rủi ro |
|---|---|---|
| 1 | Dễ giữ thứ tự hơn, ít overload | Throughput thấp |
| 10-100 | Throughput tốt hơn | Có thể tăng memory, xử lý song song làm ordering khó |
| Quá cao | Consumer nhận nhiều message | Unacked nhiều, shutdown/rebalance chậm |

Với job nặng hoặc cần thứ tự theo aggregate, bắt đầu với `prefetch=1` hoặc thấp. Với job nhẹ, tăng dần và đo DB/API latency.

---

## 7. Retry, DLX Và DLQ

### DLX là gì?

DLX là Dead Letter Exchange. Khi message bị dead-letter, RabbitMQ publish message đó sang DLX.

Message có thể dead-letter khi:

* consumer `nack/reject` với `requeue=false`;
* message hết TTL trong queue;
* queue vượt max length;
* quorum queue delivery limit bị vượt.

### DLQ là gì?

DLQ là Dead Letter Queue. Thường DLX route message lỗi vào DLQ để debug/manual retry.

```text
main queue
  x-dead-letter-exchange = order.dlx

order.dlx
  -> order.dlq
```

DLX là exchange, DLQ là queue. Hai khái niệm này hay bị gọi lẫn.

---

## 8. Retry Có Delay Bằng TTL + DLX

> Có lab Go tái hiện đúng pattern này (retry 10s → 30s → DLQ, dùng
> default exchange, DLQ để manual check) ở
> [`queueEdgeCases/readme.md`](queueEdgeCases/readme.md#7-lab-tái-hiện-retry-10s--30s--dlq-go).

Không nên retry bằng cách `nack(requeue=true)` liên tục. Pattern phổ biến là dùng retry queue có TTL.

```text
order.events.queue
  -> consumer fail transient
  -> publish sang order.events.retry.5s
  -> ack message hiện tại

order.events.retry.5s
  x-message-ttl = 5000
  x-dead-letter-exchange = order.events.exchange
  x-dead-letter-routing-key = order.created

Sau 5s:
  RabbitMQ dead-letter message về exchange chính
  -> route lại vào order.events.queue
```

Retry nhiều tầng:

```text
order.events.queue
order.events.retry.5s
order.events.retry.30s
order.events.retry.5m
order.events.dlq
```

Consumer/app thường quản lý retry count trong header:

```json
{
  "x-retry-count": 2
}
```

Logic:

```text
retry_count = 0 -> publish retry.5s
retry_count = 1 -> publish retry.30s
retry_count = 2 -> publish retry.5m
retry_count >= 3 -> publish DLQ
```

RabbitMQ không tự tăng `retry_count` cho app. App phải đọc header, tăng count rồi publish sang retry queue phù hợp.

---

## 9. Delay / Scheduled Message

Section 8 dùng TTL + DLX để *retry*. Cùng cơ chế đó có thể dùng chủ động để **delay/schedule** một message: publish ngay bây giờ nhưng muốn consumer chỉ xử lý sau X thời gian.

Use case:

* hủy order chưa thanh toán sau 15 phút;
* gửi nhắc nhở sau 30 phút;
* retry webhook sau 1 giờ;
* cron-like task nhẹ không muốn dựng scheduler riêng.

Câu hỏi cốt lõi: delay nên đặt ở **header của message** hay **config của queue**? Phụ thuộc broker có plugin gì và delay cố định hay linh hoạt.

### Cách 1: Plugin x-delayed-message (delay theo từng message)

Nếu broker cài `rabbitmq_delayed_message_exchange`, tạo exchange type `x-delayed-message`, delay nằm ở header `x-delay` của **từng message**.

```text
exchange: delay-exchange
  type: x-delayed-message
  arguments: x-delayed-type = direct

publish message
  headers: x-delay = 30000   # 30s
  -> broker giữ message 30s rồi mới route như direct exchange
```

* Ưu: mỗi message một mức delay khác nhau, không cần tạo nhiều queue.
* Nhược: cần cài plugin (không phải managed broker nào cũng cho), message đang delay giữ trong broker nên delay rất dài + volume lớn thì tốn tài nguyên broker.

### Cách 2: TTL trên queue + DLX (delay cố định, không cần plugin)

Cách chuẩn RabbitMQ khi không có plugin. Tạo một "delay queue" có TTL cố định, **không có consumer**; hết TTL message dead-letter sang exchange/queue chính.

```text
producer
  -> delay_queue
       x-message-ttl = 30000        (không consumer)
       x-dead-letter-exchange = main_exchange
       x-dead-letter-routing-key = main
  -> sau 30s (TTL hết)
       main_exchange -> main_queue -> consumer
```

Ở đây delay là thuộc tính của **queue**: mọi message vào queue này đều trễ đúng 30s.

### Cách 3: TTL trên từng message + DLX

RabbitMQ cho set TTL riêng mỗi message qua field `expiration` (queue vẫn phải có `x-dead-letter-exchange`).

```text
publish -> delay_queue (có x-dead-letter-exchange)
  message.expiration = "30000"
```

Cảnh báo quan trọng (**head-of-line blocking**): TTL per-message chỉ được đánh giá khi message tới **đầu queue**. Nếu một message TTL ngắn nằm *sau* một message TTL dài, nó vẫn phải đợi message trước hết hạn/được xử lý mới thoát ra — delay bị trễ hơn mong đợi. Vì vậy per-message TTL + DLX chỉ hợp khi các delay xấp xỉ nhau; delay chênh lệch nhiều thì dùng nhiều delay queue cố định hoặc plugin.

### Chọn header hay queue?

| Nhu cầu | Cách |
|---|---|
| Delay cố định, mọi message trễ như nhau | TTL trên **queue** (`x-message-ttl`) + DLX |
| Vài mức delay cố định (30s, 5m, 1h) | Mỗi mức một delay queue riêng |
| Delay khác nhau mỗi message | Plugin `x-delayed-message` (ưu tiên) hoặc per-message `expiration` + DLX (lưu ý head-of-line) |

Tóm tắt thực tế:

* Không có plugin: TTL + DLX là lựa chọn ổn định, đúng chuẩn RabbitMQ — cùng cơ chế retry queue ở Section 8, chỉ khác mục đích.
* Có plugin: `x-delayed-message` linh hoạt hơn cho nhiều mức delay tùy message.
* Dù cách nào, sau khi hết delay message vẫn đi qua consumer bình thường, nên ack/retry/idempotency ở các section trên vẫn áp dụng nguyên vẹn.

---

## 10. Poison Message

Poison message là message sẽ luôn fail nếu retry lại.

Ví dụ:

* JSON sai schema;
* thiếu field bắt buộc;
* event version không còn hỗ trợ;
* business state không thể xử lý;
* data corrupt.

Không nên retry poison message vô hạn. Nên:

* validate message ở boundary consumer;
* lỗi schema rõ ràng thì đưa DLQ;
* log `event_id`, routing key, retry count, error;
* có tool/manual process để inspect và replay nếu cần.

---

## 11. Idempotent Consumer

Vì RabbitMQ at-least-once, consumer phải idempotent.

Pattern:

```text
BEGIN TX
  insert processed_events(event_id, consumer_name)
  nếu duplicate -> commit/rollback nhẹ, ack, bỏ qua

  xử lý side effect trong DB
  update business state
COMMIT
ACK
```

Schema mẫu:

```sql
CREATE TABLE processed_events (
    consumer_name TEXT NOT NULL,
    event_id TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer_name, event_id)
);
```

Điểm quan trọng:

* `event_id` phải stable khi retry.
* Dedup record nên ghi cùng transaction với side effect DB.
* Nếu side effect là external API, dùng idempotency key ở external provider nếu có.
* Duplicate message nên `ack`, không `nack`.

Xem lab thực hành: [Idempotency Lab](idempotency/README.md).

---

## 12. Ordering

RabbitMQ giữ thứ tự trong một queue theo điều kiện lý tưởng, nhưng production dễ phá ordering:

* nhiều consumer cùng đọc một queue;
* prefetch > 1;
* message fail rồi retry;
* consumer xử lý chậm/nhanh khác nhau;
* publish từ nhiều producer;
* redelivery sau crash.

Nếu cần giữ thứ tự theo `order_id`, thường dùng routing theo aggregate:

```text
hash(order_id) % N -> lane queue

order-events-0
order-events-1
order-events-2
order-events-3
```

Mỗi lane xử lý tuần tự hơn:

* một consumer active cho mỗi lane nếu cần ordering mạnh;
* `prefetch=1`;
* event có `version` hoặc `sequence`;
* handler có state machine;
* event out-of-order thì defer/retry, không update bừa.

RabbitMQ không thay thế business version/state machine.

### Vẫn cần nhiều consumer, nhưng vẫn phải tuần tự thì sao?

Câu hỏi phỏng vấn hay gặp: "dùng RabbitMQ xử lý tuần tự nhưng vẫn giữ nhiều consumer thì làm sao?" Có 2 kỹ thuật kết hợp:

**1. Lane pattern (đa consumer ở tầng hệ thống)**

Vẫn là routing theo `hash(order_id) % N` ở trên: toàn hệ thống có N consumer chạy song song (scale ngang bằng cách tăng N), nhưng mỗi lane chỉ có đúng 1 consumer xử lý tại một thời điểm nên thứ tự trong lane được giữ. "Nhiều consumer" ở đây là nhiều consumer *khác lane*, không phải nhiều consumer trên cùng 1 queue.

**2. Single Active Consumer (SAC) — đa consumer ngay trên cùng 1 queue**

RabbitMQ có tính năng built-in cho đúng case này: nhiều consumer được đăng ký trên **cùng một queue** để failover/HA, nhưng broker chỉ deliver cho **đúng 1 consumer active** tại một thời điểm. Consumer active mất kết nối, RabbitMQ tự chuyển sang consumer đứng chờ kế tiếp mà app không cần tự coordinate.

```text
queue: order-events-0
  x-single-active-consumer = true

consumer A (active)   <- đang nhận message
consumer B (standby)  <- đăng ký sẵn, không nhận message
consumer C (standby)  <- đăng ký sẵn, không nhận message

A mất kết nối
  -> RabbitMQ tự chuyển B thành active
  -> thứ tự vẫn được giữ, không cần app tự quản lý ai đang xử lý
```

Kết hợp cả hai: mỗi lane queue bật SAC, một pool consumer subscribe vào tất cả lane. Hệ thống có nhiều consumer đăng ký (dễ scale/deploy, có failover), nhưng mỗi lane luôn chỉ 1 consumer xử lý tại một thời điểm.

SAC/lane chỉ đảm bảo broker không phát song song trong cùng 1 lane. Crash + redelivery (message unacked quay lại) vẫn có thể tạo out-of-order thực tế ở tầng app, nên version/sequence + idempotent handler vẫn là lớp bảo vệ bắt buộc, không thể bỏ.

---

## 13. Durable, Persistent Và Reliability

Các khái niệm dễ nhầm:

| Khái niệm | Ý nghĩa |
|---|---|
| Durable exchange/queue | Exchange/queue sống qua broker restart |
| Persistent message | Message được ghi bền hơn qua restart |
| Publisher confirm | Producer biết broker đã nhận/ghi message |
| Manual ack | Consumer xác nhận xử lý xong |

Nếu queue durable nhưng message không persistent, broker restart vẫn có thể mất message.

Nếu producer publish mà không dùng publisher confirm, producer có thể tưởng publish thành công trong khi broker chưa chắc nhận bền.

Checklist cho message quan trọng:

* durable exchange/queue;
* persistent message;
* publisher confirm;
* manual ack;
* retry + DLQ;
* idempotent consumer.

---

## 14. Publisher Confirm

Publisher confirm giúp producer biết RabbitMQ đã nhận message.

Nếu không có confirm:

```text
producer publish
-> network/broker lỗi đúng lúc
-> producer không chắc message đã vào broker chưa
```

Với confirm:

```text
producer publish
-> chờ broker confirm
-> nếu confirm fail/timeout thì retry publish
```

Nhưng retry publish có thể tạo duplicate, nên message vẫn cần `event_id` và consumer idempotent.

---

## 15. Outbox Pattern

Vấn đề:

```text
DB commit thành công
publish RabbitMQ fail
```

hoặc:

```text
publish RabbitMQ thành công
DB rollback
```

Outbox pattern:

```text
BEGIN TX
  update orders
  insert outbox_events(event_id, event_type, payload)
COMMIT

outbox worker:
  read unpublished outbox events
  publish RabbitMQ với publisher confirm
  mark published
```

Đổi lại:

* tăng độ phức tạp;
* cần cleanup outbox;
* eventual consistency.

Nhưng nó giảm rủi ro mất event giữa DB và broker.

---

## 16. Consumer Concurrency

Các cách scale consumer:

* tăng số consumer process/pod;
* tăng goroutine/thread xử lý trong mỗi consumer;
* tăng prefetch;
* tăng số queue/lane theo partition key.

Không nên chỉ tăng consumer vô hạn. Downstream như PostgreSQL, Redis, third-party API có giới hạn.

Metrics cần nhìn:

* queue depth/backlog;
* ready messages;
* unacked messages;
* consumer count;
* processing latency;
* retry rate;
* DLQ count;
* DB/API downstream latency;
* error rate theo routing key/event type.

---

## 17. Graceful Shutdown Consumer

Khi deploy/restart worker:

```text
stop nhận message mới
đợi message đang xử lý xong trong timeout
ack/nack đúng trạng thái
close channel/connection
exit
```

Nếu process bị kill khi đang xử lý nhưng chưa ack, RabbitMQ sẽ redeliver message. Đây là lý do idempotent consumer bắt buộc.

---

## 18. RabbitMQ vs Kafka Nói Ngắn Gọn

| RabbitMQ | Kafka |
|---|---|
| Message broker/task queue | Distributed event log/stream |
| Queue + ack từng message | Consumer offset |
| Routing linh hoạt bằng exchange | Topic partition |
| Retry/DLQ thường thiết kế bằng queue | Replay theo offset dễ hơn |
| Hợp job async, workflow, command | Hợp event streaming, analytics, log, replay |

Câu trả lời tốt:

> Nếu tôi cần task queue với ack/retry/DLQ rõ, routing linh hoạt và worker xử lý job, RabbitMQ là lựa chọn đơn giản. Nếu tôi cần event log lưu dài hạn, replay nhiều consumer group và throughput stream lớn, Kafka hợp hơn.

---

## 19. Lỗi Thiết Kế Thường Gặp

* Ack trước khi side effect thành công.
* Retry bằng `nack(requeue=true)` vô hạn.
* Không có DLQ cho poison message.
* Không có idempotent consumer.
* Prefetch quá cao làm worker ôm nhiều message và overload DB.
* Nghĩ RabbitMQ đảm bảo exactly-once.
* Không có publisher confirm cho event quan trọng.
* Không dùng outbox khi cần nhất quán DB + publish event.
* Không monitor unacked/backlog/DLQ.
* Dùng một queue nhiều consumer rồi kỳ vọng ordering tuyệt đối.

---

## 20. Câu Trả Lời Phỏng Vấn Mẫu

### RabbitMQ hoạt động thế nào?

> Producer publish message vào exchange. Exchange route message vào queue dựa trên binding và routing key. Consumer đọc message từ queue và ack sau khi xử lý thành công. Với production, tôi dùng manual ack, prefetch để giới hạn unacked messages, retry có backoff, DLQ cho poison message và idempotent consumer vì RabbitMQ thường là at-least-once.

### Exchange type khác nhau thế nào?

> Direct match routing key chính xác, topic match pattern như `order.*`, fanout broadcast tới mọi queue bound, headers route theo headers. Với domain event tôi thường dùng topic exchange; với task cụ thể có thể dùng direct; với broadcast cache invalidation có thể dùng fanout.

### Prefetch để làm gì?

> Prefetch giới hạn số message unacked mà broker giao cho mỗi consumer. Nó giúp backpressure và tránh một worker ôm quá nhiều message. Prefetch thấp như 1 dễ kiểm soát ordering và downstream load hơn, prefetch cao tăng throughput nhưng có thể tăng unacked, memory và làm ordering khó hơn.

### Retry và DLQ thiết kế thế nào?

> Tôi tránh requeue ngay vô hạn. Với lỗi transient, consumer publish message sang retry queue có TTL như 5s, 30s, 5m rồi ack message hiện tại. Hết TTL, message dead-letter về exchange chính để xử lý lại. Retry count nằm trong header do app quản lý. Quá số lần hoặc lỗi permanent thì đưa vào DLQ để inspect/manual xử lý.

### Vì sao consumer phải idempotent?

> Vì consumer có thể xử lý xong nhưng crash trước khi ack, hoặc broker redeliver message. Do đó cùng event có thể đến nhiều lần. Tôi dùng event_id/dedup table hoặc unique constraint, ghi dedup record cùng transaction với side effect, duplicate thì ack và bỏ qua.
