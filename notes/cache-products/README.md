# Cache Products Demo

Demo implement flow trong `docs.md`:

- API dùng **Gin**, source of truth là **Postgres** (qua **GORM**), cache là **Redis** (cache-aside).
- Một key duy nhất cho mỗi client: `client:{clientID}:catalog`, chứa cả categories → products → sizes.
- `GET /clients/{clientID}/catalog` đọc Redis trước, miss thì build từ Postgres rồi set lại cache (TTL 1h).
- Mọi write (product / size) đều **`DEL`** cache key của client đó, không rebuild ngay.
- Không warm-up lúc start — Redis rỗng là chuyện bình thường.
- Checkout/order **không** đọc Redis, luôn đọc Postgres để lấy giá mới nhất.

Giá nằm ở **size**, không nằm ở product. Món bán một giá vẫn có đúng một size tên `"Mặc định"`.

## Cấu trúc file

| File | Trách nhiệm |
| --- | --- |
| `main.go` | Config hard-code, Gin router, handler của query path + CRUD |
| `order.go` | Command path — checkout, phát hiện giá đổi, reconcile cache |
| `model.go` | Entity GORM, DTO `Catalog`, DTO order, validate |
| `postgres.go` | `CatalogRepo` — toàn bộ thao tác Postgres qua GORM |
| `redis.go` | `CatalogCache` — toàn bộ thao tác Redis |
| `seed.go` | Menu mẫu của 2 client, chỉ chạy khi bảng `clients` rỗng |

| Tài liệu | Nội dung |
| --- | --- |
| `docs.md` | Thiết kế cache-aside cho catalog |
| `docs/new-requirement-plan-1.md` → `docs/implement-plan-1.md` | Plan hiện tại |
| `docs/new-requirement-plan.md` → `docs/implement-plan.md` | Phương án read model đã cân nhắc rồi bỏ |
| `isssue.md` | Vấn đề stale price lúc checkout |

## Run

Postgres + Redis lấy từ compose của lab idempotency:

```bash
docker compose -f ../../idempotency/docker-compose.yml up -d postgres redis
```

Chạy app (GORM `AutoMigrate` tạo bảng, seed menu nếu bảng `clients` rỗng):

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

## Data mẫu

Seed tạo 2 client, 7 category, 24 product, 42 size:

```text
client 1001 — Trà Sữa Nhà Làm
  1 Đồ Uống : Trà Đào Cam Sả (S/M/L), Trà Sữa Trân Châu Đường Đen (M/L), Trà Vải Hoa Hồng,
              Hồng Trà Sữa, Sữa Tươi Trân Châu Đường Đen, Matcha Latte, Cà Phê Sữa Đá,
              Nước Ép Ổi, Trà Sữa Ngưng Bán  ← product active = false
  2 Topping : Trân Châu Đen, Thạch Phô Mai, Pudding Trứng, Kem Cheese
  3 Ăn Vặt  : Bánh Tráng Trộn, Khoai Tây Lắc Phô Mai, Gà Rán

client 1002 — Pizza Ba Miền
  4 Pizza        : Pizza Hải Sản (S/M/L), Pizza Bò Nướng Tiêu (S/M/L), Pizza Phô Mai (S/M/L)
  5 Mì Ý         : Mì Ý Sốt Bò Bằm, Mì Ý Sốt Kem Nấm
  6 Nước         : Coca-Cola (Lon/Chai), Nước Suối
  7 Khuyến Mãi Hè: Combo Hè Mát Lạnh   ← category active = false
```

Hai case cố tình cài sẵn để test filter: một **product** inactive và một **category** inactive.

Vài id hay dùng ở dưới:

| id | |
| --- | --- |
| size `1` / `2` / `3` | Trà Đào Cam Sả — S 30000 / M 40000 / L 50000 |
| size `18` | Trà Sữa Ngưng Bán — M (product inactive) |
| size `19` | Trân Châu Đen — Mặc định 8000 |
| product `1` | Trà Đào Cam Sả |
| category `1` | Đồ Uống (client 1001) |
| category `4` | Pizza (client 1002) |

## API

| Method | Path | Ghi chú |
| --- | --- | --- |
| GET | `/health` | health check |
| GET | `/clients/{clientID}/catalog` | cache-aside, trả kèm `source: cache\|db` |
| POST | `/clients/{clientID}/products` | tạo product + sizes, xoá cache |
| PUT | `/clients/{clientID}/products/{productID}` | update product, **thay toàn bộ sizes**, xoá cache |
| DELETE | `/clients/{clientID}/products/{productID}` | xoá product, xoá cache |
| PUT | `/clients/{clientID}/sizes/{sizeID}` | đổi tên/giá 1 size (giữ nguyên id), xoá cache |
| POST | `/clients/{clientID}/orders` | tính tiền, luôn đọc DB, 409 nếu giá đã đổi |

Client và category không có API tạo/sửa — seed sẵn trong DB.

> `PUT /products/{id}` xoá hết size cũ rồi insert lại nên **size id bị đổi**. Cart đang giữ `size_id`
> sẽ hỏng. Đổi giá thường ngày thì dùng `PUT /sizes/{id}`.

---

## Bộ curl test

### 0. Health

```bash
curl -s 127.0.0.1:8020/health
```

```json
{"status":"ok"}
```

### 1. Không warm-up — Redis rỗng lúc vừa start

```bash
docker exec redis redis-cli KEYS 'client:*:catalog'
```

Không có key nào. Cache-aside không dựng cache trước, request đọc đầu tiên mới dựng.

### 2. Cache-aside: lần 1 vào DB, lần 2 vào cache

```bash
# miss -> "source":"db", có kèm "ttl":"1h0m0s"
curl -s 127.0.0.1:8020/clients/1001/catalog | jq '.source, .ttl'

# hit -> "source":"cache"
curl -s 127.0.0.1:8020/clients/1001/catalog | jq '.source'
```

```
"db"
"1h0m0s"
"cache"
```

Xem key và TTL trong Redis:

```bash
docker exec redis redis-cli KEYS 'client:*:catalog'
docker exec redis redis-cli TTL client:1001:catalog     # 3600
```

Cấu trúc catalog:

```bash
curl -s 127.0.0.1:8020/clients/1001/catalog \
  | jq '.catalog.categories[0].products[0]'
```

```json
{
  "id": 1,
  "name": "Trà Đào Cam Sả",
  "sizes": [
    { "id": 1, "name": "S", "price": 30000 },
    { "id": 2, "name": "M", "price": 40000 },
    { "id": 3, "name": "L", "price": 50000 }
  ]
}
```

### 3. Data inactive không vào catalog

`Trà Sữa Ngưng Bán` (product inactive) không có trong `Đồ Uống`:

```bash
curl -s 127.0.0.1:8020/clients/1001/catalog \
  | jq '.catalog.categories[0].products[].name'
```

`Khuyến Mãi Hè` (category inactive) biến mất cả nhánh, dù product con vẫn active:

```bash
curl -s 127.0.0.1:8020/clients/1002/catalog | jq '.catalog.categories[].name'
```

```json
"Pizza"
"Mì Ý"
"Nước"
```

### 4. Write làm cache bị invalidate

```bash
# đổi giá Trà Đào size M: 40000 -> 45000
curl -s -X PUT 127.0.0.1:8020/clients/1001/sizes/2 \
  -H 'Content-Type: application/json' \
  -d '{"name":"M","price":45000}'

# key đã bị xoá -> 0
docker exec redis redis-cli EXISTS client:1001:catalog

# đọc lại -> "source":"db", giá mới
curl -s 127.0.0.1:8020/clients/1001/catalog \
  | jq '.source, .catalog.categories[0].products[0].sizes[1]'
```

```json
"db"
{ "id": 2, "name": "M", "price": 45000 }
```

Tạo product mới cũng xoá cache:

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/products \
  -H 'Content-Type: application/json' \
  -d '{"category_id":1,"name":"Trà Ổi Hồng","sizes":[{"name":"M","price":42000},{"name":"L","price":52000}]}'
```

```json
{"id":25,"client_id":"1001","category_id":1,"name":"Trà Ổi Hồng","active":true,
 "sizes":[{"id":43,"product_id":25,"name":"M","price":42000,"sort_order":0},
          {"id":44,"product_id":25,"name":"L","price":52000,"sort_order":1}]}
```

> Các id ở đây (và mục 8) đúng khi chạy trên DB vừa seed lần đầu. Nếu bạn đã tạo/xoá product trước đó
> thì sequence đã chạy tiếp, id sẽ lớn hơn.

### 5. Order luôn lấy giá từ DB

Cart gửi `size_id` + `quantity`, và `expected_price` là giá client đang hiển thị. Server **không**
tính tiền theo `expected_price` — chỉ dùng nó để phát hiện giá đã đổi (xem mục 6). Bỏ trống thì bỏ
qua bước kiểm tra.

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":2},{"size_id":19,"quantity":1}]}'
```

```json
{"lines":[
  {"size_id":2,"product_id":1,"name":"Trà Đào Cam Sả (M)","quantity":2,"unit_price":45000,"line_amount":90000},
  {"size_id":19,"product_id":10,"name":"Trân Châu Đen (Mặc định)","quantity":1,"unit_price":8000,"line_amount":8000}
],"price_source":"db","total":98000}
```

Chứng minh order không ăn giá cũ trong cache — sửa giá thẳng dưới DB (không qua API nên cache **không**
bị xoá):

```bash
curl -s 127.0.0.1:8020/clients/1001/catalog > /dev/null   # warm cache, giá 45000

docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c "UPDATE product_sizes SET price = 999000 WHERE id = 2;"

# catalog vẫn "source":"cache", giá cũ 45000
curl -s 127.0.0.1:8020/clients/1001/catalog \
  | jq '.source, .catalog.categories[0].products[0].sizes[1].price'

# order đọc thẳng DB -> total = 999000
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1}]}' | jq '.total'
```

```
"cache"
45000
999000
```

Đây chính là điểm yếu đã biết của cache-aside: sửa thẳng DB thì cache sai tới hết TTL (1h), không có
cơ chế nào tự chữa. Đổi lại, đường tính tiền không bao giờ bị ảnh hưởng vì nó không đọc cache.

### 6. Giá đổi giữa lúc xem menu và lúc đặt → 409

Khách mở menu thấy Trà Đào M = 40000, admin đổi giá thành 45000 ở mục 4, giờ khách mới bấm đặt.
Client gửi lên giá nó đang hiển thị:

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":40000}]}'
```

```json
{"error":"price_changed",
 "message":"Giá đã thay đổi, vui lòng tải lại menu và xác nhận lại",
 "changed_items":[{"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":40000,"current_price":45000}],
 "new_total":45000,
 "catalog_refreshed":false}
```

Order **không** được tạo. Khách xem giá mới rồi xác nhận — client gửi lại với giá mới:

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' | jq '.total'
```

```
45000
```

Nhiều dòng lệch được gom vào một response, dòng nào đúng giá thì không xuất hiện:

```bash
curl -s -X POST 127.0.0.1:8020/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":1,"quantity":1,"expected_price":11},
                {"size_id":2,"quantity":2,"expected_price":22},
                {"size_id":19,"quantity":1,"expected_price":8000}]}' | jq '.changed_items, .new_total'
```

```json
[
  {"size_id":1,"name":"Trà Đào Cam Sả (S)","expected_price":11,"current_price":30000},
  {"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":22,"current_price":45000}
]
128000
```

#### `catalog_refreshed` — chỉ xoá cache khi cache thật sự sai

Ở hai lệnh trên `catalog_refreshed` đều là `false`: cache đang khớp DB, chỉ có client cầm giá cũ.
Xoá cache lúc đó là vô ích.

Giờ làm cho cache lệch thật — sửa thẳng DB nên cache không bị xoá:

```bash
curl -s 127.0.0.1:8020/clients/1001/catalog > /dev/null   # warm cache, giá 45000

docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c "UPDATE product_sizes SET price = 99000 WHERE id = 2;"

curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' \
  | jq '.error, .changed_items[0].current_price, .catalog_refreshed'
```

```json
"price_changed"
99000
true
```

Lần này `catalog_refreshed: true` — key đã bị xoá, log app in:

```
catalog cache lệch DB (client=1001 size=2: giá cache 45000 != DB 99000) -> xoá key
```

```bash
docker exec redis redis-cli EXISTS client:1001:catalog       # 0
curl -s 127.0.0.1:8020/clients/1001/catalog | jq '.source'   # "db"
```

Cùng cơ chế áp dụng cho món bị tắt giữa chừng. Lúc này cache đang giữ catalog **có** Trà Đào (dựng ở
lệnh `GET` ngay trên), rồi mới tắt món dưới DB:

```bash
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c "UPDATE products SET active = false WHERE id = 1;"

curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1}]}'
```

```json
{"error":"size 2 is not available","catalog_refreshed":true}
```

`true` vì cache vẫn còn size 2 trong khi DB đã tắt product cha — đúng dấu hiệu *"món tắt rồi mà cache
vẫn còn"*. Nếu trước đó cache đã kịp dựng lại (không còn Trà Đào) thì kết quả là `false`: lúc đó cache
không sai, chỉ có client cầm menu cũ.

Trả dữ liệu về:

```bash
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c "UPDATE products SET active = true WHERE id = 1;" \
  -c "UPDATE product_sizes SET price = 45000 WHERE id = 2;"
```

### 7. Cache hỏng thì degrade chứ không lỗi

Ghi payload rác vào key:

```bash
docker exec redis redis-cli SET client:1001:catalog 'không phải JSON'

# vẫn 200, "source":"db" — log app in "redis payload corrupted, dropping key"
curl -s 127.0.0.1:8020/clients/1001/catalog | jq '.source'
```

Redis down cũng đi đúng nhánh này: log lỗi, đọc thẳng Postgres, request vẫn thành công. Cache là
optional — hỏng thì chậm, không được sai.

### 8. Delete

```bash
curl -i -s -X DELETE 127.0.0.1:8020/clients/1001/products/25   # 204 No Content
```

### 9. Các case lỗi

```bash
# 404 {"error":"client_not_found"}
curl -s 127.0.0.1:8020/clients/9999/catalog

# 400 {"error":"at least one size is required"} - product bắt buộc có size
curl -s -X POST 127.0.0.1:8020/clients/1001/products \
  -H 'Content-Type: application/json' -d '{"category_id":1,"name":"X","sizes":[]}'

# 400 {"error":"category not found"} - category 4 là của client 1002 (cross-tenant)
curl -s -X POST 127.0.0.1:8020/clients/1001/products \
  -H 'Content-Type: application/json' -d '{"category_id":4,"name":"X","sizes":[{"name":"M","price":1000}]}'

# 404 {"error":"size not found"} - size 28 thuộc client 1002
curl -s -X PUT 127.0.0.1:8020/clients/1001/sizes/28 \
  -H 'Content-Type: application/json' -d '{"name":"S","price":1000}'

# 400 {"error":"size 18 is not available","catalog_refreshed":false} - product cha đang inactive
# catalog_refreshed = false vì cache cũng đang không có món này -> cache không sai
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' -d '{"items":[{"size_id":18,"quantity":1}]}'

# 404 - product 17 là của client 1002
curl -s -X DELETE 127.0.0.1:8020/clients/1001/products/17

# 400 {"error":"items are required"}
curl -s -X POST 127.0.0.1:8020/clients/1001/orders \
  -H 'Content-Type: application/json' -d '{"items":[]}'
```

### 10. Reset dữ liệu

```bash
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c 'DROP TABLE IF EXISTS product_sizes, products, categories, clients CASCADE;'

docker exec redis redis-cli --scan --pattern 'client:*:catalog' | xargs -r docker exec redis redis-cli DEL

# start lại app, AutoMigrate + seed sẽ chạy lại
```

Lưu ý: database `idempotency_lab` dùng chung với lab idempotency (`orders`, `payments`,
`idempotency_keys`, `notifications`, `processed_events`) — chỉ drop đúng 4 bảng ở trên.
