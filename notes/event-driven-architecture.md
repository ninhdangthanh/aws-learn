# Event-Driven Architecture (EDA) Middle Backend Notes

File này gom kiến thức Event-Driven Architecture đủ sâu cho phỏng vấn Middle Backend: bản chất event, các biến thể EDA, choreography vs orchestration, delivery guarantee, ordering, idempotent consumer, outbox, schema evolution, chọn broker và các failure mode production.

EDA không chỉ là "đẩy event vào queue". Cần hiểu rõ event khác command/message ở đâu, ai chịu trách nhiệm state, làm sao đảm bảo không mất và không xử lý trùng, và khi nào KHÔNG nên dùng.

Liên quan: [RabbitMQ Middle Notes](rabbitmq-middle-notes.md), [Backend Communication Roadmap](backend-communication-roadmap.md), [Notebook - Architecture](notebook.md) mục 5, và code idempotent consumer trong [`idempotency/`](../idempotency/README.md).

---

## 1. EDA là gì và khi nào dùng

Trong EDA, service giao tiếp bằng cách **phát ra sự kiện (event)** mô tả "một việc đã xảy ra", thay vì gọi trực tiếp nhau qua request/response. Producer không biết ai consume, khi nào consume.

Đặc trưng:

* **Loose coupling**: producer chỉ biết broker, không biết consumer. Thêm consumer mới không sửa producer.
* **Async by default**: producer phát event xong trả về ngay, không chờ downstream xử lý.
* **Chịu lỗi tốt hơn**: downstream chết thì event nằm trong broker chờ, tránh cascading failure như khi gọi sync dây chuyền.

Hợp dùng khi:

* Một hành động kéo theo nhiều side-effect độc lập (đặt hàng → trừ kho, gửi mail, tính điểm, ghi analytics).
* Cần decouple team/service, mỗi bên deploy độc lập.
* Cần buffer traffic spike (đẩy vào queue, worker xử lý theo nhịp).
* Cần audit/replay lịch sử thay đổi.

KHÔNG nên dùng khi:

* Cần kết quả ngay và cần biết thành công/thất bại trong cùng request (ví dụ trừ tiền và trả về balance mới cho user) → dùng sync (REST/gRPC).
* Luồng đơn giản 1-1, thêm broker chỉ tăng độ phức tạp vận hành và khó debug.
* Cần strong consistency tức thời; EDA về bản chất là eventual consistency.

Cái giá phải trả: eventual consistency, khó debug (luồng phân tán, không có stack trace xuyên service), phải tự lo delivery guarantee, ordering, idempotency, và observability (tracing/correlation id).

---

## 2. Event vs Command vs Message

Đây là phần dễ bị hỏi để phân biệt "hiểu bản chất" hay chỉ "biết dùng queue".

| Tiêu chí | Event | Command |
|---|---|---|
| Ý nghĩa | Một việc **đã xảy ra** trong quá khứ | Yêu cầu **làm một việc** trong tương lai |
| Tên | Quá khứ: `OrderPlaced`, `PaymentCaptured` | Mệnh lệnh: `PlaceOrder`, `CapturePayment` |
| Người nhận | 0..N consumer (fan-out) | Đúng 1 handler |
| Coupling | Producer không biết ai xử lý | Sender biết chính xác muốn ai làm |
| Có thể từ chối? | Không, đã xảy ra rồi | Có, handler có thể reject |

* **Message** là thuật ngữ chung cho cả hai (đơn vị dữ liệu đi qua broker).
* Command thực chất là "RPC bất đồng bộ" — vẫn là coupling về ý định, chỉ khác về thời gian.
* Event là hình thức decoupling mạnh nhất: producer chỉ thông báo, không ra lệnh.

Sai lầm hay gặp: đặt tên event kiểu mệnh lệnh (`SendEmail`) rồi coi như EDA. Đó thực ra là command trá hình → vẫn coupling chặt vào một consumer cụ thể.

---

## 3. Các biến thể EDA

### 3.1 Event Notification (thông báo mỏng)

Event chỉ mang **id + loại sự kiện**, ít/không mang state. Consumer nhận được thì tự gọi ngược lại producer (query API) để lấy chi tiết.

* Ưu: payload nhỏ, không lộ nhiều dữ liệu, producer là source of truth.
* Nhược: consumer phải call ngược → tăng coupling runtime và tải lên producer; dễ gặp race (event tới trước khi data đọc được, hoặc data đã đổi khi query).

### 3.2 Event-Carried State Transfer (mang theo state)

Event mang **đủ dữ liệu** consumer cần (`OrderPlaced` kèm items, total, customer...). Consumer không cần call ngược.

* Ưu: consumer tự chủ, không phụ thuộc producer lúc runtime, chịu lỗi tốt.
* Nhược: payload lớn, dữ liệu có thể trùng lặp/stale ở nhiều nơi, phải quản version schema chặt.

Đây là dạng phổ biến nhất cho decoupling thực sự giữa các service.

### 3.3 Event Sourcing

State không lưu dạng "giá trị hiện tại" mà lưu **chuỗi event bất biến**. State hiện tại = fold/replay toàn bộ event từ đầu.

* Ưu: audit trail đầy đủ, replay được, dựng lại state tại bất kỳ thời điểm, hợp domain tài chính/kế toán.
* Nhược: phức tạp, query hiện trạng phải build **read model/projection** riêng, schema event thay đổi khó (event cũ bất biến, phải versioning/upcasting), cần snapshot để tránh replay quá dài.
* Thường đi kèm CQRS. Lưu ý: EDA ≠ event sourcing. Đa số hệ thống EDA chỉ publish integration event chứ không event-source.

### 3.4 CQRS + EDA

Tách **Command Model (write, normalized)** và **Query Model (read, denormalized)**. Khi write xong, phát integration event qua broker để cập nhật read model bất đồng bộ (eventual consistency).

* Hợp khi tỉ lệ đọc >> ghi, hoặc read/write có yêu cầu tối ưu rất khác nhau.
* Cái giá: có độ trễ giữa write và read; UI phải chấp nhận "vừa ghi xong chưa thấy ngay".

---

## 4. Choreography vs Orchestration

Hai cách điều phối một business flow trải qua nhiều service (ví dụ Saga cho luồng đặt hàng).

### Choreography (phân tán, hướng sự kiện)

Không có bộ điều phối trung tâm. Mỗi service lắng nghe event, làm phần của mình, rồi phát event tiếp theo.

```
OrderService   --OrderPlaced-->      (broker)
PaymentService --PaymentCaptured-->  (broker)
KitchenService --DishPrepared-->     (broker)
```

* Ưu: loose coupling tối đa, không single point, dễ thêm consumer.
* Nhược: luồng nghiệp vụ **ẩn** — không nơi nào nhìn thấy toàn bộ flow, khó debug/trace, dễ tạo phụ thuộc vòng, khó biết flow đang ở bước nào.

### Orchestration (tập trung, có trạng thái)

Một orchestrator (Saga coordinator, hoặc engine như Temporal) gọi từng bước và giữ trạng thái flow.

* Ưu: luồng nghiệp vụ **hiện rõ** ở một chỗ, dễ theo dõi, dễ xử lý compensation (rollback) khi một bước fail.
* Nhược: orchestrator là điểm tập trung coupling và có thể thành bottleneck/SPOF nếu thiết kế kém.

Rule thực dụng: flow đơn giản, ít bước, cần decouple → choreography. Flow dài, nhiều bước, cần compensation và visibility (order → payment → kitchen → delivery) → orchestration/Temporal. Xem thêm so sánh RabbitMQ vs Temporal trong [notebook.md](notebook.md) mục 7.

---

## 5. Delivery guarantee

| Mức | Cơ chế | Rủi ro |
|---|---|---|
| **At-most-once** | Commit/ack trước khi xử lý | Mất message nếu crash giữa chừng |
| **At-least-once** | Xử lý xong mới ack | Xử lý trùng nếu crash sau khi xử lý, trước khi ack |
| **Exactly-once** | Transaction/idempotency | Đắt, khó, thường là "hiệu ứng exactly-once" chứ không phải delivery thật |

Thực tế production gần như luôn chọn **at-least-once + idempotent consumer**. "Exactly-once" đúng nghĩa qua network là bài toán rất khó; cái ta đạt được là **at-least-once delivery + exactly-once processing** nhờ khử trùng ở consumer.

Câu chốt khi phỏng vấn:

> Tôi mặc định at-least-once vì mất message thường tệ hơn xử lý lại. Để chống trùng, consumer phải idempotent: dùng event id/business key làm khóa khử trùng, và làm side-effect an toàn với retry.

---

## 6. Ordering (thứ tự message)

Nhiều broker chỉ đảm bảo thứ tự **trong một partition/queue**, không đảm bảo toàn cục.

* **Kafka**: đảm bảo order trong 1 partition. Muốn giữ thứ tự theo entity → dùng cùng partition key (ví dụ `order_id`) để mọi event của cùng order rơi vào cùng partition, cùng 1 consumer xử lý tuần tự.
* **RabbitMQ**: order chỉ giữ khi 1 queue - 1 consumer - prefetch=1. Nhiều consumer song song hoặc requeue là mất thứ tự.

Các cách xử lý khi thứ tự quan trọng:

* Partition theo entity key để tuần tự hóa theo từng entity, vẫn song song giữa các entity khác nhau.
* Thiết kế event **giao hoán (commutative)** hoặc mang version/sequence number để consumer tự phát hiện và bỏ event cũ đến muộn.
* Chấp nhận out-of-order và reconcile bằng state (ví dụ so timestamp/version, chỉ apply nếu mới hơn).

Đánh đổi kinh điển: **ordering vs throughput**. Ép tuần tự tuyệt đối (1 consumer) thì mất khả năng scale ngang.

---

## 7. Idempotent consumer

Vì at-least-once, consumer **phải** chịu được xử lý cùng event nhiều lần mà kết quả không đổi.

Cách làm:

1. **Dedup bằng khóa duy nhất**: mỗi event có `event_id` (hoặc business key như `payment_id`). Consumer lưu id đã xử lý vào bảng/Redis, gặp lại thì bỏ qua.
2. **Ràng buộc unique ở DB**: dùng `UNIQUE`/`INSERT ... ON CONFLICT DO NOTHING` để lần thứ hai không tạo bản ghi trùng — an toàn ngay cả khi có race giữa nhiều worker.
3. **Thao tác vốn idempotent**: `SET status = 'paid'` (không phải `balance = balance + 10`), hoặc upsert theo key.
4. **Atomic giữa side-effect và ghi dedup**: lý tưởng là xử lý nghiệp vụ và ghi "đã xử lý event này" trong **cùng một transaction**, để tránh trường hợp làm side-effect xong crash trước khi ghi dedup.

Chi tiết code Go về idempotency (Redis + Postgres unique constraint) nằm trong [`idempotency/`](../idempotency/README.md).

Lưu ý side-effect ngoài DB (gửi mail, gọi payment): những cái này khó rollback, nên cần dedup key trước khi gọi, hoặc dùng provider hỗ trợ idempotency key.

---

## 8. Dual-write problem & Outbox pattern

**Dual-write problem**: cần vừa ghi DB vừa publish event. Nếu làm hai thao tác tách rời:

* Ghi DB xong, publish fail → có state nhưng mất event (downstream không biết).
* Publish xong, commit DB fail → có event nhưng state không tồn tại (event ma).

Không có distributed transaction giữa DB và broker một cách rẻ và tin cậy, nên ta không ghi cả hai "cùng lúc".

**Transactional Outbox**:

1. Trong cùng transaction ghi nghiệp vụ, ghi thêm 1 dòng vào bảng `outbox` (event chờ gửi). Cùng commit → nguyên tử.
2. Một tiến trình riêng (relay/poller, hoặc CDC như Debezium đọc WAL) đọc bảng outbox và publish lên broker.
3. Publish thành công thì đánh dấu đã gửi/xóa dòng.

Vì relay có thể gửi lại khi retry → event là **at-least-once**, nên consumer vẫn phải idempotent (khớp với mục 7).

Biến thể: **Inbox pattern** ở phía consumer — lưu event id đã nhận vào bảng inbox trong cùng transaction xử lý để khử trùng.

---

## 9. Schema evolution & versioning

Event là **hợp đồng (contract)** giữa các service và tồn tại lâu (nhất là event sourcing — event cũ bất biến). Đổi schema ẩu là làm hỏng consumer.

Nguyên tắc:

* **Backward/forward compatible**: chỉ **thêm field optional**, không xóa/đổi nghĩa field cũ, không đổi kiểu. Consumer bỏ qua field lạ.
* **Không tái dùng** tên field cho ý nghĩa khác.
* Dùng schema có versioning: Protobuf (field number, tương thích tốt), Avro + Schema Registry (kiểm tra compatibility khi publish), hoặc gắn `schema_version` trong payload.
* Khi buộc phải breaking change: publish **version mới song song** (`OrderPlaced.v2`), cho consumer chuyển dần, rồi mới bỏ v1.
* Event sourcing: dùng **upcasting** — chuyển event cũ sang shape mới lúc đọc.

---

## 10. Chọn broker

| Broker | Bản chất | Hợp khi | Lưu ý |
|---|---|---|---|
| **RabbitMQ** | Message broker, routing linh hoạt | Task queue, command/event async, routing phức tạp, cần ack/retry/DLQ rõ ràng | Message tiêu thụ xong là mất; không hợp replay dài hạn |
| **Kafka** | Distributed event log | Event streaming throughput lớn, nhiều consumer group độc lập, replay, event sourcing | Vận hành nặng hơn; ordering theo partition |
| **Redis Streams** | Log nhẹ trong Redis | Realtime signal, throughput vừa, đã có sẵn Redis | Bền theo giới hạn memory/AOF; không mạnh bằng Kafka |
| **Redis Pub/Sub** | Fire-and-forget | Broadcast realtime, cache invalidation | **Không durable** — subscriber offline là mất message |
| **NATS / JetStream** | Messaging nhẹ, nhanh | Latency thấp, microservices; JetStream thêm persistence | Hệ sinh thái nhỏ hơn |
| **AWS SNS + SQS** | Pub/sub + queue managed | Fan-out trên AWS, ít phải vận hành | Coupling vào AWS |
| **AWS EventBridge** | Event bus có rule routing | Định tuyến event giữa nhiều service theo rule/filter, tích hợp AWS | Throughput/độ trễ khác Kafka; hợp integration hơn streaming |

Khung chọn nhanh:

* Cần **replay + nhiều consumer group + throughput rất cao** → Kafka.
* Cần **routing linh hoạt + task queue + retry/DLQ** → RabbitMQ.
* Đã có Redis, cần **realtime nhẹ** → Redis Streams (durable) hoặc Pub/Sub (chấp nhận mất).
* Trên AWS, muốn **managed, ít vận hành** → SNS/SQS hoặc EventBridge.

---

## 11. Failure mode & edge case production

* **Message mất**: publish không confirm, broker chưa persist mà crash, Pub/Sub subscriber offline. → publisher confirm, persistent/durable queue, replication, outbox.
* **Xử lý trùng**: hệ quả của at-least-once/retry/redeliver. → idempotent consumer (mục 7).
* **Poison message**: message luôn fail, retry vô hạn làm nghẽn queue. → giới hạn retry + đẩy vào **DLQ**, alert và xử lý tay.
* **Out-of-order**: nhiều consumer song song, requeue. → partition theo key hoặc reconcile bằng version (mục 6).
* **Consumer lag / backpressure**: producer nhanh hơn consumer, queue phình. → scale consumer, tăng partition, giám sát lag, backpressure.
* **Retry storm**: downstream lỗi, tất cả retry đồng loạt. → exponential backoff + jitter, circuit breaker.
* **Event ma / dual-write**: xem outbox (mục 8).
* **Schema break**: consumer chết vì payload đổi. → versioning + compatibility (mục 9).
* **Khó debug/trace**: luồng phân tán không có stack trace. → gắn **correlation id / trace id** vào mọi event, distributed tracing, structured logging.
* **Eventual consistency lộ ra UI**: user vừa ghi chưa thấy ngay. → thiết kế UX chấp nhận, hoặc read-your-own-write có chủ đích.

---

## 12. Câu hỏi phỏng vấn thường gặp

**EDA khác request/response chỗ nào, khi nào không nên dùng?**
> EDA là async, decoupled, eventual consistency — producer phát event và không chờ. Hợp khi một hành động kéo theo nhiều side-effect độc lập, cần buffer traffic, cần decouple team. Không nên dùng khi cần kết quả ngay trong cùng request hoặc cần strong consistency; lúc đó dùng REST/gRPC sync. Cái giá của EDA là khó debug và phải tự lo delivery guarantee, ordering, idempotency.

**Event khác command thế nào?**
> Event mô tả việc đã xảy ra (quá khứ), fan-out tới nhiều consumer, producer không biết ai xử lý. Command là yêu cầu làm việc (mệnh lệnh), đúng một handler, vẫn coupling về ý định. Đặt tên event kiểu mệnh lệnh như `SendEmail` thực ra là command trá hình.

**Làm sao đảm bảo không mất và không trùng?**
> At-least-once để không mất (xử lý xong mới ack, publisher confirm, durable queue). Chống trùng bằng idempotent consumer: dedup theo event id/business key, unique constraint ở DB, thao tác idempotent. "Exactly-once" thực chất là at-least-once delivery + exactly-once processing.

**Vừa ghi DB vừa publish event, làm sao nhất quán?**
> Đây là dual-write problem. Dùng transactional outbox: ghi nghiệp vụ và ghi dòng outbox trong cùng transaction, một relay/CDC đọc outbox publish sau. Vì relay retry nên vẫn at-least-once, consumer vẫn phải idempotent.

**Đảm bảo thứ tự message thế nào?**
> Đa số broker chỉ đảm bảo order trong 1 partition/queue. Muốn giữ thứ tự theo entity thì partition theo entity key (mọi event cùng order vào cùng partition, một consumer xử lý tuần tự). Nếu cần scale thì thiết kế event mang version để consumer bỏ event cũ đến muộn, chấp nhận out-of-order rồi reconcile.

**Choreography hay orchestration?**
> Choreography: mỗi service nghe event tự làm rồi phát tiếp — loose coupling nhưng luồng bị ẩn, khó trace. Orchestration: một coordinator giữ trạng thái và gọi từng bước — luồng rõ, dễ compensation, nhưng tập trung coupling. Flow ngắn cần decouple → choreography; flow dài nhiều bước cần rollback và visibility → orchestration/Temporal.

**Poison message xử lý sao?**
> Giới hạn số lần retry, quá ngưỡng thì đẩy vào DLQ để không nghẽn queue, gắn alert và xử lý tay/replay sau khi fix. Kèm backoff + jitter để tránh retry storm.
