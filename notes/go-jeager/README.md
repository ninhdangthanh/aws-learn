# Lab học Jaeger + OpenTelemetry

Bốn microservice Go nối với nhau qua **HTTP, gRPC, PostgreSQL và RabbitMQ**, tất
cả instrument bằng OpenTelemetry và đẩy trace về Jaeger. Kèm 6 kịch bản test,
trong đó 4 kịch bản cố ý gây lỗi ở những chỗ khác nhau để tập đọc trace.

## Chạy nhanh

```bash
make up       # dựng toàn bộ (Jaeger + Postgres + RabbitMQ + 4 service)
make check    # kiểm tra mọi thứ đã sẵn sàng
make all      # chạy tuần tự 6 kịch bản, dừng giữa mỗi bước để bạn xem Jaeger
```

Mở http://localhost:16686

> **Bài học chính nằm ở [`docs/LAB.md`](docs/LAB.md)** — chạy script xong thì đọc file đó.
>
> Bảng test case đầy đủ: [`docs/TEST-CASES.md`](docs/TEST-CASES.md) — dùng làm checklist hồi quy.

## Kiến trúc

```
curl ──HTTP──> gateway ──gRPC──> order-svc ──SQL──> Postgres
                                     │
                                     ├──gRPC──> inventory-svc ──SQL──> Postgres
                                     │
                                     └──AMQP──> RabbitMQ
                                                   │
                                                   └──> notification-svc ──HTTP──> gateway
```

Một request thành công sinh ra **23 span trải trên 4 service**.

| Service | Vai trò | I/O |
|---|---|---|
| `gateway` | REST công khai `:8080` | HTTP server, gRPC client, nhận HTTP callback |
| `order-svc` | Điều phối `:9091` | gRPC server/client, Postgres, AMQP publisher |
| `inventory-svc` | Tồn kho `:9092` | gRPC server, Postgres |
| `notification-svc` | Worker async | AMQP consumer, HTTP client |

## Các kịch bản

| Lệnh | Kịch bản | Kết quả | Bài học |
|---|---|---|---|
| `make happy` | Luồng thành công | 201 | Context propagation qua 3 tầng khác nhau |
| `make oos` | Hết hàng | 409 | Root cause là span đỏ SÂU NHẤT, không phải span đỏ đầu tiên |
| `make slow` | `pg_sleep(3)` | 201 (~3s) | Thời gian thật nằm ở span lá, không phải span cha |
| `make panic` | Service chết | 503 | Span THIẾU cũng là tín hiệu chẩn đoán |
| `make async` | Consumer hỏng | **201** | Request thành công ≠ công việc đã xong |
| `make load` | 40 request hỗn hợp | — | Tìm outlier bằng scatter plot và bộ lọc tag |

## Cơ chế bơm lỗi

Gửi header `X-Fail-Mode` tới gateway. Gateway nạp nó vào **OpenTelemetry
Baggage**, và baggage tự chảy xuống mọi service qua cả gRPC lẫn RabbitMQ mà
không service nào phải truyền tay tham số:

```bash
curl -X POST localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Fail-Mode: slow_db' \
  -d '{"sku":"SKU-LAPTOP","qty":1}' -i
```

Bốn giá trị hợp lệ: `out_of_stock`, `slow_db`, `panic`, `async_fail`.

Mọi response đều mang header `X-Trace-Id`, nên script in được link mở thẳng
đúng trace:

```
Trace : http://localhost:16686/trace/843da6a19615852e3a6105ef6c9c5ef1
```

## Cổng

Cổng host được dời khỏi giá trị chuẩn để không đụng các stack khác trên máy.
Bên trong mạng compose, các service vẫn dùng cổng chuẩn.

| Dịch vụ | URL | Ghi chú |
|---|---|---|
| Jaeger UI | http://localhost:16686 | |
| RabbitMQ UI | http://localhost:15673 | `lab` / `lab` |
| Gateway | http://localhost:8080 | |
| Postgres | `localhost:5442` | `lab` / `lab` / db `lab` |
| OTLP gRPC | `localhost:4317` | các service đẩy span vào đây |
| order-svc gRPC | `localhost:9091` | mở sẵn để gọi bằng `grpcurl` |
| inventory-svc gRPC | `localhost:9092` | |

Muốn đổi, sửa phần `ports:` trong `docker-compose.yml`.

## API

```bash
POST /orders              {"sku":"SKU-LAPTOP","qty":1,"customer":"tên"}
GET  /orders/{id}
GET  /stats               # đếm callback từ notification-svc
GET  /healthz
POST /internal/callback   # notification-svc gọi vào, không dùng trực tiếp
```

SKU có sẵn: `SKU-LAPTOP` (50), `SKU-PHONE` (100), `SKU-MOUSE` (30),
`SKU-RARE` (2 — đặt nhiều hơn 2 là hết hàng thật).

## Lệnh khác

```bash
make logs     # log 4 service
make ps       # trạng thái container
make psql     # vào psql
make reset    # xoá sạch dữ liệu Postgres rồi dựng lại
make proto    # sinh lại code gRPC sau khi sửa file .proto
make down     # dừng lab
```

## Cấu trúc

```
cmd/                     4 service, mỗi thư mục một main.go
internal/
  otelx/                 khởi tạo OTel — sampler, propagator, exporter
  amqpx/carrier.go       cầu nối AMQP header ↔ OTel TextMapCarrier  ★
  amqpx/amqpx.go         publish/consume có inject/extract trace context
  faultx/                bơm lỗi qua Baggage
  dbx/                   pgx pool gắn otelpgx
  httpx/                 helper HTTP, header X-Trace-Id
proto/                   định nghĩa gRPC
gen/                     code sinh từ protoc (đã commit)
scripts/                 6 kịch bản test
docs/LAB.md              bài học từng bước  ★
```

Hai file đánh dấu ★ là phần đáng đọc nhất.

## Yêu cầu

Docker + Docker Compose. Go và `buf` chỉ cần khi muốn build tại máy hoặc sửa
file `.proto`.
