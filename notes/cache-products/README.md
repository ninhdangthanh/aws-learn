# Cache Products Demo

Demo này implement flow trong `docs.md`:

- API dùng **Gin**, database thật là **Postgres** (qua **GORM**), cache là **Redis** (cache-aside).
- `GET /clients/{clientID}/products` đọc Redis trước, miss thì đọc Postgres rồi set lại cache (TTL 1h).
- Create/update/delete product luôn xoá cache key của client đó.
- Checkout/order **không** đọc Redis, luôn đọc Postgres để lấy giá mới nhất.

## Cấu trúc file

| File | Trách nhiệm |
| --- | --- |
| `main.go` | Config hard-code, Gin router, handler |
| `model.go` | Struct `Product`, `ProductInput`, order DTO, validate |
| `postgres.go` | `ProductRepo` — toàn bộ thao tác Postgres qua GORM |
| `redis.go` | `ProductCache` — toàn bộ thao tác Redis |

## Run

Postgres + Redis lấy từ compose của lab idempotency:

```bash
docker compose -f ../../idempotency/docker-compose.yml up -d postgres redis
```

Chạy app (GORM `AutoMigrate` tạo bảng `products`, seed 3 product mẫu nếu bảng rỗng):

```bash
go mod tidy
go run .
```

Config hard-code trong `main.go`:

```go
postgresDSN = "postgres://postgres:postgres@127.0.0.1:5432/idempotency_lab?sslmode=disable"
redisAddr   = "127.0.0.1:6379"
httpAddr    = ":8020"
cacheTTL    = time.Hour
```

## API

| Method | Path | Ghi chú |
| --- | --- | --- |
| GET | `/health` | health check |
| GET | `/clients/{clientID}/products` | cache-aside, trả kèm `source: cache\|db` |
| POST | `/clients/{clientID}/products` | tạo product, xoá cache |
| PUT | `/clients/{clientID}/products/{productID}` | update full, xoá cache |
| DELETE | `/clients/{clientID}/products/{productID}` | xoá product, xoá cache |
| POST | `/clients/{clientID}/orders` | tính tiền, luôn đọc DB |

---

## Bộ curl test

### 0. Health

```bash
curl -s 127.0.0.1:8020/health
```

```json
{"status":"ok"}
```

### 1. Tạo list product ban đầu cho client 1001

App lúc start đã seed sẵn id `1` Pizza, `2` Burger (client 1001) và `3` Coffee (client 1002).
Chạy nguyên block này để tạo thêm một menu đầy đủ (được id `4`–`11`):

```bash
for p in \
  '{"name":"Pizza Hải Sản","price":180000}' \
  '{"name":"Pizza Bò","price":150000}' \
  '{"name":"Burger Gà","price":75000}' \
  '{"name":"Mì Ý Sốt Kem","price":95000}' \
  '{"name":"Salad Cá Ngừ","price":60000}' \
  '{"name":"Coca 330ml","price":20000}' \
  '{"name":"Trà Đào","price":35000}' \
  '{"name":"Món Ngưng Bán","price":50000,"active":false}' ; do
  curl -s -X POST 127.0.0.1:8020/clients/1001/products \
    -H 'Content-Type: application/json' \
    -d "$p"
  echo
done
```

Kết quả (lưu ý `Món Ngưng Bán` có `active:false` để test order bị reject):

```json
{"id":4,"client_id":"1001","name":"Pizza Hải Sản","price":180000,"active":true}
{"id":5,"client_id":"1001","name":"Pizza Bò","price":150000,"active":true}
{"id":6,"client_id":"1001","name":"Burger Gà","price":75000,"active":true}
{"id":7,"client_id":"1001","name":"Mì Ý Sốt Kem","price":95000,"active":true}
{"id":8,"client_id":"1001","name":"Salad Cá Ngừ","price":60000,"active":true}
{"id":9,"client_id":"1001","name":"Coca 330ml","price":20000,"active":true}
{"id":10,"client_id":"1001","name":"Trà Đào","price":35000,"active":true}
{"id":11,"client_id":"1001","name":"Món Ngưng Bán","price":50000,"active":false}
```

Tạo thêm cho client 1002 để thấy dữ liệu bị tách theo tenant:

```bash
curl -s -X POST 127.0.0.1:8020/clients/1002/products \
  -H 'Content-Type: application/json' \
  -d '{"name":"Cà Phê Sữa","price":45000}'
echo
```

### 2. Cache-aside: lần 1 vào DB, lần 2 vào cache

```bash
# miss -> "source":"db", có kèm "ttl":"1h0m0s"
curl -s 127.0.0.1:8020/clients/1001/products

# hit -> "source":"cache"
curl -s 127.0.0.1:8020/clients/1001/products
```

Xem key trong Redis:

```bash
docker exec redis redis-cli KEYS 'client:*:products'
docker exec redis redis-cli TTL client:1001:products
```

### 3. Update làm cache bị invalidate

```bash
curl -s -X PUT 127.0.0.1:8020/clients/1001/products/4 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Pizza Hải Sản","price":200000,"active":true}'

# đọc lại -> "source":"db" vì key đã bị xoá
curl -s 127.0.0.1:8020/clients/1001/products
```

### 4. Order luôn lấy giá từ DB

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"product_id":4,"quantity":2},{"product_id":6,"quantity":1}]}'
```

```json
{"lines":[
  {"product_id":4,"name":"Pizza Hải Sản","quantity":2,"unit_price":200000,"line_amount":400000},
  {"product_id":6,"name":"Burger Gà","quantity":1,"unit_price":75000,"line_amount":75000}
],"price_source":"db","total":475000}
```

Muốn chứng minh order không ăn giá cũ trong cache — sửa giá thẳng dưới DB
(không qua API nên cache không bị xoá):

```bash
curl -s 127.0.0.1:8020/clients/1001/products > /dev/null   # warm cache, giá 200000

docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c "UPDATE products SET price = 999000 WHERE id = 4;"

curl -s 127.0.0.1:8020/clients/1001/products   # "source":"cache", id 4 vẫn 200000

curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"product_id":4,"quantity":1}]}'   # total = 999000, đọc thẳng DB
```

### 5. Delete

```bash
curl -i -s -X DELETE 127.0.0.1:8020/clients/1001/products/10   # 204 No Content
```

### 6. Các case lỗi

```bash
# 400 {"error":"name is required and price must be positive"}
curl -s -X POST 127.0.0.1:8020/clients/1001/products \
  -H 'Content-Type: application/json' -d '{"name":"","price":0}'

# 404 {"error":"product not found"} - id không tồn tại
curl -s -X PUT 127.0.0.1:8020/clients/1001/products/999 \
  -H 'Content-Type: application/json' -d '{"name":"X","price":1000}'

# 404 {"error":"product not found"} - id 3 là của client 1002 (cross-tenant)
curl -s -X DELETE 127.0.0.1:8020/clients/1001/products/3

# 400 {"error":"product 11 is not available"} - product đang inactive
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' -d '{"items":[{"product_id":11,"quantity":1}]}'

# 400 {"error":"items are required"}
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' -d '{"items":[]}'
```

### 7. Reset dữ liệu

```bash
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab -c 'DROP TABLE IF EXISTS products;'
docker exec redis redis-cli FLUSHALL
# start lại app, AutoMigrate + seed sẽ chạy lại
```
