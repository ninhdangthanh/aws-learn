# Lab Jaeger — bài học từng bước

Tài liệu này đi kèm 6 script trong `scripts/`. Mỗi phần gồm: chạy gì, nhìn vào
đâu, và điều cần rút ra.

Trước khi bắt đầu: `make up && make check`.

---

## Phần 0 — Hiểu bản đồ trước khi đọc trace

Bốn service, bốn loại I/O, một trace duy nhất:

```
curl ──HTTP──> gateway ──gRPC──> order-svc ──SQL──> Postgres
                                     │
                                     ├──gRPC──> inventory-svc ──SQL──> Postgres
                                     │
                                     └──AMQP──> RabbitMQ
                                                   │
                                                   └──> notification-svc ──HTTP──> gateway
```

Một request thành công sinh ra **23 span**. Nghe nhiều, nhưng phần lớn là span
Postgres do `otelpgx` tự sinh (`pool.acquire`, `prepare`, `query`).

**Ba khái niệm cần phân biệt cho rõ:**

| Khái niệm | Là gì | Ở đâu trong repo này |
|---|---|---|
| **Trace** | Toàn bộ hành trình của một request | Một dòng trong danh sách Jaeger |
| **Span** | Một đoạn công việc trong trace | Một thanh ngang trong waterfall |
| **Context propagation** | Cách trace ID đi từ service này sang service kia | `internal/amqpx/carrier.go` |

---

## Phần 1 — Trace đi qua biên service bằng cách nào

```bash
make happy
```

Mở link trace được in ra. Đây là bài học nền, đọc kỹ phần này thì bốn phần sau
sẽ nhẹ nhàng.

### Ba cơ chế truyền context, ba mức độ công sức

**HTTP — miễn phí.** `otelhttp.NewHandler` và `otelhttp.NewTransport` tự lo hết.

```go
// cmd/gateway/main.go
handler := otelhttp.NewHandler(faultMiddleware(mux), "gateway", ...)
```

**gRPC — gần như miễn phí.** Một dòng `StatsHandler` ở cả hai đầu:

```go
// server
grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
// client
grpc.NewClient(addr, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
```

> `StatsHandler` là API hiện hành. Nhiều blog cũ còn hướng dẫn dùng
> `UnaryInterceptor` — cách đó đã deprecated.

**RabbitMQ — phải tự viết.** Không có thư viện chính thức. Bạn phải bắc cầu
giữa `amqp.Table` và interface `TextMapCarrier`:

```go
// internal/amqpx/carrier.go — chỉ cần 3 method
type HeaderCarrier amqp.Table
func (c HeaderCarrier) Get(key string) string   { ... }
func (c HeaderCarrier) Set(key, value string)   { c[key] = value }
func (c HeaderCarrier) Keys() []string          { ... }
```

Rồi dùng ở hai đầu:

```go
// producer — internal/amqpx/amqpx.go
ctx, span := tracer.Start(ctx, "order.created publish", trace.WithSpanKind(trace.SpanKindProducer))
otel.GetTextMapPropagator().Inject(ctx, HeaderCarrier(headers))   // ← inject SAU khi mở span

// consumer
msgCtx := otel.GetTextMapPropagator().Extract(ctx, HeaderCarrier(d.Headers))
msgCtx, span := tracer.Start(msgCtx, "order.created process", trace.WithSpanKind(trace.SpanKindConsumer))
```

### Việc cần làm trong Jaeger UI

1. Bung span `order.created publish` → tab **Tags** → chép giá trị
   `lab.injected_traceparent`.
2. Bung span `order.created process` → tab **Tags** → so với
   `lab.received_traceparent`.
3. Hai chuỗi phải trùng nhau ở phần trace-id (32 ký tự hex sau `00-`).

Đó chính là sợi dây nối nhánh async vào trace gốc.

### Thứ tự inject rất quan trọng

Trong `Publish`, `Inject` được gọi **sau** `tracer.Start`. Nếu đảo lại,
consumer sẽ nối vào span cha thay vì span producer, và cây trace sai cấu trúc.
Sửa thử rồi chạy lại `make happy` để tự thấy khác biệt.

### Bài tập

Comment dòng `otel.GetTextMapPropagator().Inject(...)` trong
`internal/amqpx/amqpx.go`, chạy `make restart && make happy`. Bạn sẽ thấy
**hai trace riêng biệt** thay vì một. Đó là lỗi phổ biến nhất khi tự
instrument message queue.

---

## Phần 2 — Root cause không phải span đỏ đầu tiên bạn nhìn thấy

```bash
make oos
```

Request trả về HTTP 409. Trong trace có **4 span đỏ** trải trên 3 service:

```
gateway    order.v1.OrderService/CreateOrder          ← đỏ (span client)
order-svc  order.v1.OrderService/CreateOrder          ← đỏ (span server)
order-svc  inventory.v1.InventoryService/Reserve      ← đỏ (span client)
inventory-svc  inventory.v1.InventoryService/Reserve  ← đỏ  ★ ROOT CAUSE
```

**Quy tắc: luôn lần xuống span đỏ SÂU NHẤT.** Các span đỏ phía trên chỉ đang
chuyển tiếp lỗi từ dưới lên.

### Chi tiết dễ bỏ sót

Span **gốc** `POST /orders` của gateway **không đỏ**, dù request trả 409.

`otelhttp` chỉ đánh dấu lỗi khi status ≥ 500. Mã 4xx được coi là "client sai",
không phải "server hỏng" — đúng theo đặc tả OpenTelemetry.

Hệ quả thực tế: lọc `Service=gateway` + `error=true` sẽ **không** tìm ra trace
này. Phải lọc theo service tầng dưới, hoặc dựa vào tag nghiệp vụ tự đặt
(`lab.http_status=409`).

### Vì sao nên gắn attribute nghiệp vụ

`inventory-svc` gắn `lab.sku`, `lab.qty_requested`, `lab.qty_available` lên
span. Nhờ vậy bạn tìm được trace theo ngôn ngữ nghiệp vụ:

```
Tags:  lab.sku=SKU-RARE
Tags:  error=true
```

Không có những tag này thì chỉ còn cách cuộn tay qua danh sách trace.

---

## Phần 3 — Đọc waterfall để tìm thủ phạm thật

```bash
make slow
```

Không có lỗi. Request trả 201. Chỉ là mất ~3 giây thay vì ~44ms.

Số đo thật từ lần chạy kiểm chứng:

| Span | Thời lượng | Vai trò |
|---|---|---|
| `POST /orders` (gateway) | 3037ms | đang chờ |
| `OrderService/CreateOrder` (order-svc) | 3036ms | đang chờ |
| `InventoryService/Reserve` (inventory-svc) | 3024ms | đang chờ |
| `inventory.slow_lookup` | 3013ms | đang chờ |
| **`query SELECT pg_sleep($1)`** | **3013ms** | ★ **thủ phạm** |

**Nguyên tắc: thời gian thật nằm ở span LÁ.** Span cha dài chỉ vì đang chờ con.

Chênh lệch 3037 − 3013 = **24ms** chính là chi phí thật của 3 chặng mạng cộng
với các query khác. Con số đó là baseline overhead của hệ thống.

### Ba công cụ của Jaeger nên tập dùng ở đây

- **Compare** (menu góc phải trên): dán trace nhanh và trace chậm, Jaeger tô
  màu chênh lệch.
- **Trace Statistics**: xếp hạng theo *self time* — thời gian span tự tiêu tốn,
  không tính thời gian chờ con. Đây mới là chỉ số chỉ đúng thủ phạm.
- **Min Duration** ở màn hình Search: gõ `2s` để lọc riêng request chậm.

---

## Phần 4 — Span thiếu cũng là tín hiệu chẩn đoán

```bash
make panic
```

`inventory-svc` gọi `panic()`. grpc-go không recover, cả process chết.

Trace thu được (số liệu kiểm chứng thật):

```
SERVICES: gateway, order-svc          ← chỉ 2, KHÔNG có inventory-svc
SPANS: 8                              ← thay vì 23

  gateway    POST /orders
  gateway    order.v1.OrderService/CreateOrder
  order-svc  order.v1.OrderService/CreateOrder
  order-svc  query INSERT INTO orders ... 'PENDING'
  order-svc  inventory.v1.InventoryService/Reserve   ← span CLIENT, không có server tương ứng
  order-svc  query UPDATE orders SET status = 'FAILED' ...
```

**Chữ ký của một service chết: có span client, không có span server.**

### Vì sao span biến mất

`BatchSpanProcessor` gom span vào buffer rồi gửi theo lô mỗi giây. Process chết
đột ngột thì buffer bay theo. Ba cách giảm thiểu:

1. `recover()` trong interceptor gRPC → span vẫn kịp ghi và flush.
2. `SimpleSpanProcessor` → gửi ngay từng span, an toàn nhưng rất tốn.
3. OTel Collector chạy cạnh service (sidecar/agent) → rút ngắn quãng mất mát.

### Phân biệt với các trường hợp khác

| Hiện tượng | grpc code | HTTP | Ý nghĩa |
|---|---|---|---|
| Có span server, đỏ | `FailedPrecondition` | 409 | lỗi nghiệp vụ |
| **Không có span server** | `Unavailable` | **503** | **service chết** |
| Có span server, rất dài | `DeadlineExceeded` | 504 | service quá chậm |

Ánh xạ này nằm ở `writeGRPCError` trong `cmd/gateway/main.go`. Giữ nó chính xác
là điều kiện để đọc được trace.

### Bài tập

Thêm interceptor có `recover()` vào `cmd/inventory/main.go`, chạy lại
`make panic`, xem span của inventory-svc có xuất hiện không.

---

## Phần 5 — Lỗi mà monitoring tầng HTTP không nhìn thấy

```bash
make async
```

Đây là kịch bản nguy hiểm nhất, vì nó **vô hình** với client.

Số liệu kiểm chứng thật:

```
ROOT  POST /orders  status=201  error=false      ← client thấy mọi thứ hoàn hảo

--- span lỗi ---
  notification-svc  ->  notification.render_email     ← đỏ
  notification-svc  ->  order.created process         ← đỏ

--- baggage đi qua RabbitMQ ---
  lab.fail_mode = async_fail                     ← baggage xuyên qua được AMQP
```

Đơn hàng đã CONFIRMED, tồn kho đã trừ, client nhận 201. Nhưng email xác nhận
không bao giờ gửi. Tỉ lệ lỗi HTTP vẫn 0%.

**Distributed tracing bắt được vì span lỗi nằm cùng trace với request 201.**

### Lưu ý khi xem

Mở trace ngay lập tức thì trông như đã xong. **Chờ 5 giây rồi tải lại trang** —
span của `notification-svc` mới hiện ra, vì nó được ghi sau và gửi theo lô.

### Truy vấn nên gắn alert ở production

```
Service: notification-svc
Tags:    error=true
```

Ra đúng những đơn "thành công nhưng chưa gửi được email".

---

## Phần 6 — Tìm outlier giữa đám đông

```bash
make load          # 40 request
make load N=200    # nhiều hơn
```

Trộn ~70% thành công, ~15% hết hàng, ~10% chậm, ~5% hỏng async.

### Việc cần làm

1. Jaeger → `Service = gateway`, `Limit Results = 100` → **Find Traces**.
2. **Biểu đồ scatter phía trên**: trục dọc là latency. Chấm nằm cao chính là
   các request `slow_db`. Bấm thẳng vào chấm để mở trace — nhanh hơn nhiều so
   với cuộn danh sách.
3. Lọc dần:
   - `Tags: error=true` → chỉ trace hỏng
   - `Min Duration: 2s` → chỉ request chậm
   - `Tags: lab.sku=SKU-PHONE` → theo nghiệp vụ
   - Đổi `Service = notification-svc` + `error=true` → lỗi async ẩn

### System Architecture

Tab **System Architecture → DAG**: Jaeger tự vẽ sơ đồ phụ thuộc giữa 4 service
**từ chính dữ liệu trace**. Không ai khai báo tay sơ đồ này. Ở hệ thống thật
vài chục service, đây thường là bản đồ kiến trúc chính xác duy nhất bạn có.

---

## Tổng kết những cái bẫy đã gặp

Đây là các lỗi thật đã xảy ra khi dựng lab này, không phải giả định:

| Bẫy | Triệu chứng | Xử lý |
|---|---|---|
| `resource.Merge` xung đột schema URL | Service crash-loop: `conflicting Schema URL` | Dùng `resource.NewSchemaless(...)` — xem `internal/otelx/otelx.go` |
| Quên `COLLECTOR_OTLP_ENABLED=true` | `connection refused` tới cổng 4317 | Đặt env cho container Jaeger |
| Quên gọi shutdown khi thoát | Chạy xong mà Jaeger không có trace | `defer shutdownOtel(ctx)` để flush buffer |
| `RecordError` mà quên `SetStatus` | Span vẫn xanh dù có lỗi | Phải gọi cả hai — xem hàm `fail()` |
| Không `tr -d '\r'` khi đọc header | Link Jaeger hỏng | Header HTTP kết thúc bằng CRLF |
| Không inject traceparent vào AMQP | Trace đứt làm đôi | `internal/amqpx/carrier.go` |

---

## Đọc tiếp

Thứ tự đọc code đề xuất:

1. `internal/otelx/otelx.go` — khởi tạo, sampler, propagator
2. `internal/amqpx/carrier.go` — cầu nối AMQP ↔ OTel
3. `internal/amqpx/amqpx.go` — inject/extract ở hai đầu
4. `internal/faultx/faultx.go` — baggage
5. `cmd/inventory/main.go` — nơi ba fault mode được kích hoạt
6. `cmd/gateway/main.go` — thứ tự bọc middleware, ánh xạ lỗi
