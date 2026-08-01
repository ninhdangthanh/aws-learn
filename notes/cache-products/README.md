# Cache Products Demo

Demo này implement flow trong `docs.md`:

- Database được giả lập bằng in-memory struct trong code.
- Redis local dùng cache-aside cho `GET /clients/{clientID}/products`.
- RabbitMQ local publish event sau create/update/delete product vào exchange `product.events`.
- Queue `product-events.audit` được bind routing key `product.*` để dễ xem event.
- Checkout/order không đọc Redis, luôn đọc in-memory DB để lấy giá mới nhất.

## Run

```bash
go mod tidy
go run .
```

Default config:

```bash
REDIS_ADDR=127.0.0.1:6379
AMQP_URL=amqp://admin:admin@127.0.0.1:5672/
HTTP_ADDR=:8020
```

## Test nhanh

Lần đầu đọc DB và ghi Redis:

```bash
curl 127.0.0.1:8020/clients/1001/products
```

Lần hai đọc cache:

```bash
curl 127.0.0.1:8020/clients/1001/products
```

Update product, cache sẽ bị delete:

```bash
curl -X PUT 127.0.0.1:8020/clients/1001/products/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Pizza","price":120000,"active":true}'
```

Đọc lại sẽ miss cache và rebuild từ DB:

```bash
curl 127.0.0.1:8020/clients/1001/products
```

Checkout luôn lấy giá từ DB:

```bash
curl -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"product_id":1,"quantity":2}]}'
```
