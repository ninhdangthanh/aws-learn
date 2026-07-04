# Idempotency Lab

Lab này tách riêng từ phần `Idempotency` trong `notes/production-backend-concepts.md`.

Mục tiêu là nhìn được 2 nơi idempotency hay xuất hiện trong production backend:

1. **HTTP API có side effect**: client retry `POST /orders` nhưng không tạo duplicate order.
2. **Queue consumer at-least-once**: RabbitMQ redeliver message nhưng consumer không xử lý side effect lần hai.

Stack dùng đúng theo ghi chú:

* PostgreSQL: lưu idempotency key, request hash, response/result, dedup event và order thật.
* RabbitMQ: reproduce duplicate message ở consumer.
* Go: code ngắn, nhiều comment để học luồng.
* `time.Sleep`: cố tình làm request/worker chậm để dễ reproduce race condition, timeout và duplicate.

---

## Chạy nhanh

```bash
cd notes/idempotency
docker compose up -d
go mod tidy
go run ./cmd/api
```

API chạy ở:

```text
http://localhost:8080
```

RabbitMQ Management UI:

```text
http://localhost:15672
username: guest
password: guest
```

---

## Demo 1: retry cùng key trả lại cùng order

Request đầu tiên cố tình sleep 5 giây trước khi tạo order:

```bash
curl -i -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-order-001' \
  -d '{"user_id":"u1","sku":"coffee","quantity":2,"simulate_delay_seconds":5}'
```

Trong lúc request trên còn đang chờ, mở terminal khác gọi lại cùng key và cùng body:

```bash
curl -i -X POST 'http://localhost:8080/orders?wait=1' \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-order-001' \
  -d '{"user_id":"u1","sku":"coffee","quantity":2,"simulate_delay_seconds":5}'
```

Điều cần quan sát:

* request đầu tiên insert được idempotency record với `status=processing`;
* request thứ hai thấy key đang `processing`;
* nếu có `wait=1`, request thứ hai poll ngắn bằng `time.Sleep`;
* sau khi request đầu tiên thành công, retry nhận lại response cũ;
* bảng `orders` chỉ có 1 order cho key đó.

Kiểm tra DB:

```bash
docker compose exec postgres psql -U app -d idempotency_lab \
  -c "select id, user_id, sku, quantity, idempotency_key from orders order by id;"
```

---

## Demo 2: reuse key nhưng payload khác bị reject

Gọi lại cùng `Idempotency-Key` nhưng đổi `quantity`:

```bash
curl -i -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-order-001' \
  -d '{"user_id":"u1","sku":"coffee","quantity":99,"simulate_delay_seconds":0}'
```

Kết quả mong đợi:

```text
409 Conflict
```

Lý do: backend lưu `request_hash`. Cùng key nhưng payload khác là một bug phía client hoặc caller, không được trả response cũ một cách âm thầm.

---

## Demo 3: RabbitMQ duplicate message

Chạy worker:

```bash
go run ./cmd/worker
```

Publish một event:

```bash
go run ./cmd/publisher -event-id event-001 -order-id 1 -delay 4
```

Publish lại chính event đó:

```bash
go run ./cmd/publisher -event-id event-001 -order-id 1 -delay 4
```

Điều cần quan sát:

* worker dùng `event_id` làm dedup key;
* lần đầu insert được `processed_events`;
* lần duplicate bị unique constraint chặn;
* duplicate message vẫn được `ack`, vì nó đã được xử lý trước đó;
* worker chỉ ghi notification một lần.

Kiểm tra DB:

```bash
docker compose exec postgres psql -U app -d idempotency_lab \
  -c "select event_id, consumer_name, status from processed_events order by created_at;"

docker compose exec postgres psql -U app -d idempotency_lab \
  -c "select id, order_id, message from notifications order by id;"
```

---

## Mapping với lý thuyết

| Lý thuyết | Nằm ở đâu trong lab |
|---|---|
| Client gửi `Idempotency-Key` | `cmd/api/main.go` |
| Lưu key + request hash + response | bảng `idempotency_keys` |
| Retry cùng key trả response cũ | `handleExistingIdempotencyRecord` |
| Cùng key payload khác trả conflict | so sánh `request_hash` |
| Race condition cùng key | `INSERT ... ON CONFLICT DO NOTHING` |
| Side effect có unique constraint | unique index trên `orders(idempotency_scope, idempotency_key)` |
| Queue at-least-once | RabbitMQ trong `cmd/worker` |
| Dedup message | bảng `processed_events` |
| Không ack trước side effect | worker chỉ `Ack` sau transaction thành công |

---

## Dọn môi trường

```bash
docker compose down -v
```
