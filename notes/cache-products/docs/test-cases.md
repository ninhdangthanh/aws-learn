# Test Cases — cache-products

40 case, đã verify toàn bộ pass trên DB seed mới.

## Cách dùng

**Phải chạy tuần tự từ TC-01.** Nhiều case phụ thuộc state do case trước tạo ra — ví dụ TC-12 đổi giá
size 2 thành 45000, mọi case sau đó đều dựa trên giá này. Nhảy cóc sẽ ra kết quả khác.

Case nào làm thay đổi dữ liệu được đánh dấu **[mutate]**.

Cần `jq`. Đặt sẵn biến cho gọn:

```bash
B=127.0.0.1:8020
PG="docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab -q -t"
RD="docker exec redis redis-cli"
```

## Chuẩn bị — reset về trạng thái sạch

```bash
# 1. tắt app đang chạy (nếu có) — kiểm tra kỹ, pkill theo tên hay sót
lsof -nP -iTCP:8020 -sTCP:LISTEN
kill <PID>

# 2. xoá 4 bảng của demo này (KHÔNG đụng bảng của lab idempotency)
docker exec idempotency-postgres-1 psql -U postgres -d idempotency_lab \
  -c 'DROP TABLE IF EXISTS product_sizes, products, categories, clients CASCADE;'

# 3. xoá key cache
docker exec redis redis-cli --scan --pattern 'client:*:catalog' \
  | xargs -r -I{} docker exec redis redis-cli DEL {}

# 4. chạy app, giữ log lại để check
go run . > /tmp/app.log 2>&1 &
sleep 8

# 5. XÁC NHẬN app mới thật sự đang chạy, không phải instance cũ
grep -c 'bind: address already in use' /tmp/app.log   # phải là 0
lsof -nP -iTCP:8020 -sTCP:LISTEN | grep -c LISTEN     # phải là 1
```

> Bước 5 không được bỏ. Nếu một instance cũ còn giữ cổng 8020 thì app mới chết ngay lúc start và mọi
> request sẽ do binary cũ phục vụ — test vẫn "pass" nhưng không kiểm chứng được code hiện tại.

---

## Nhóm 1 — Startup: seed và không warm-up

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-01 | Seed đúng số lượng | `$PG -c "SELECT (SELECT count(*) FROM clients), (SELECT count(*) FROM categories), (SELECT count(*) FROM products), (SELECT count(*) FROM product_sizes);"` | `2 \| 7 \| 24 \| 42` |
| TC-02 | Redis rỗng lúc start | `$RD KEYS 'client:*:catalog'` | không có key nào |
| TC-03 | Không có warm-up / resync | `grep -cE 'warm-up\|resync' /tmp/app.log` | `0` |
| TC-04 | Health | `curl -s $B/health` | `{"status":"ok"}` |

TC-02 và TC-03 là điểm phân biệt cache-aside với read model: không dựng cache trước, không có goroutine nền.

---

## Nhóm 2 — Cache-aside

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-05 | Lần 1 miss → DB **[mutate]** | `curl -s $B/clients/1001/catalog \| jq -c '.source, .ttl'` | `"db"` rồi `"1h0m0s"` |
| TC-06 | Lần 2 hit → cache | `curl -s $B/clients/1001/catalog \| jq -c '.source'` | `"cache"` |
| TC-07 | TTL của key | `$RD TTL client:1001:catalog` | `3600` |
| TC-08 | Payload đúng shape | `curl -s $B/clients/1001/catalog \| jq -c '.catalog.categories[0].products[0]'` | xem dưới |
| TC-09 | Có `cached_at` | `curl -s $B/clients/1001/catalog \| jq '.catalog.cached_at'` | timestamp, không null |

TC-08:

```json
{"id":1,"name":"Trà Đào Cam Sả","sizes":[{"id":1,"name":"S","price":30000},{"id":2,"name":"M","price":40000},{"id":3,"name":"L","price":50000}]}
```

`ttl` chỉ xuất hiện ở response miss (`source: db`), response hit không có field này.

---

## Nhóm 3 — Data inactive không vào catalog

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-10 | Product inactive bị loại | `curl -s $B/clients/1001/catalog \| jq -c '[.catalog.categories[].products[].name] \| any(. == "Trà Sữa Ngưng Bán")'` | `false` |
| TC-11 | Category inactive loại cả nhánh | `curl -s $B/clients/1002/catalog \| jq -c '[.catalog.categories[].name]'` | `["Pizza","Mì Ý","Nước"]` |

TC-11: `Khuyến Mãi Hè` biến mất dù product con `Combo Hè Mát Lạnh` vẫn `active = true`.

---

## Nhóm 4 — Write làm invalidate cache

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-12 | Đổi giá size **[mutate]** | `curl -s -X PUT $B/clients/1001/sizes/2 -H 'Content-Type: application/json' -d '{"name":"M","price":45000}' \| jq -c .` | `{"id":2,"product_id":1,"name":"M","price":45000,"sort_order":1}` |
| TC-13 | Key bị xoá sau write | `$RD EXISTS client:1001:catalog` | `0` |
| TC-14 | Đọc lại ra DB + giá mới | `curl -s $B/clients/1001/catalog \| jq -c '.source, .catalog.categories[0].products[0].sizes[1]'` | `"db"` rồi `{"id":2,"name":"M","price":45000}` |
| TC-15 | Tạo product **[mutate]** | xem dưới | id `25`, sizes `43`/`44` |
| TC-16 | Create cũng xoá cache | `$RD EXISTS client:1001:catalog` | `0` |

TC-15:

```bash
curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' \
  -d '{"category_id":1,"name":"Trà Ổi Hồng","sizes":[{"name":"M","price":42000},{"name":"L","price":52000}]}' | jq -c .
```

```json
{"id":25,"client_id":"1001","category_id":1,"name":"Trà Ổi Hồng","active":true,"sizes":[{"id":43,"product_id":25,"name":"M","price":42000,"sort_order":0},{"id":44,"product_id":25,"name":"L","price":52000,"sort_order":1}]}
```

Id chỉ đúng khi chạy trên DB vừa seed lần đầu. Đã từng tạo/xoá product thì sequence đã chạy tiếp, id sẽ lớn hơn.

---

## Nhóm 5 — Order đọc DB

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-17 | Order không gửi `expected_price` | xem dưới | total `98000`, `price_source: db` |

```bash
curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":2},{"size_id":19,"quantity":1}]}' | jq -c .
```

```json
{"lines":[{"size_id":2,"product_id":1,"name":"Trà Đào Cam Sả (M)","quantity":2,"unit_price":45000,"line_amount":90000},{"size_id":19,"product_id":10,"name":"Trân Châu Đen (Mặc định)","quantity":1,"unit_price":8000,"line_amount":8000}],"price_source":"db","total":98000}
```

Bỏ trống `expected_price` = client chấp nhận mọi giá hiện hành, không kiểm tra.

---

## Nhóm 6 — Giá đổi giữa chừng → 409

Trước nhóm này phải warm cache để nó khớp DB:

```bash
curl -s $B/clients/1001/catalog > /dev/null
```

### 6a. Cache đúng, chỉ client cầm menu cũ

| # | Case | Mong đợi |
|---|---|---|
| TC-18 | 409 khi giá lệch | `error: price_changed`, `catalog_refreshed: false` |
| TC-19 | Cache **không** bị xoá | `$RD EXISTS client:1001:catalog` → `1` |
| TC-20 | Xác nhận lại bằng giá mới → 201 | total `45000` |
| TC-21 | Gom nhiều dòng lệch vào 1 response | 2 dòng lệch, `new_total` `128000` |

TC-18:

```bash
curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":40000}]}' | jq -c .
```

```json
{"catalog_refreshed":false,"changed_items":[{"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":40000,"current_price":45000}],"error":"price_changed","message":"Giá đã thay đổi, vui lòng tải lại menu và xác nhận lại","new_total":45000}
```

TC-20:

```bash
curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' | jq -c '.total'
```

TC-21 — dòng đúng giá (size 19) không xuất hiện trong `changed_items`:

```bash
curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":1,"quantity":1,"expected_price":11},
                {"size_id":2,"quantity":2,"expected_price":22},
                {"size_id":19,"quantity":1,"expected_price":8000}]}' | jq -c '.changed_items, .new_total'
```

```json
[{"size_id":1,"name":"Trà Đào Cam Sả (S)","expected_price":11,"current_price":30000},{"size_id":2,"name":"Trà Đào Cam Sả (M)","expected_price":22,"current_price":45000}]
128000
```

### 6b. Cache lệch thật → bị xoá

Sửa thẳng DB nên cache không bị invalidate:

```bash
$PG -c "UPDATE product_sizes SET price = 99000 WHERE id = 2;"    # [mutate]
```

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-22 | 409 + `catalog_refreshed: true` | xem dưới | `"price_changed"`, `99000`, `true` |
| TC-23 | Key đã bị xoá | `$RD EXISTS client:1001:catalog` | `0` |
| TC-24 | Log ghi rõ lý do | `grep 'lệch DB' /tmp/app.log` | xem dưới |
| TC-25 | GET lại ra DB | `curl -s $B/clients/1001/catalog \| jq -c '.source'` | `"db"` |

TC-22:

```bash
curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' \
  -d '{"items":[{"size_id":2,"quantity":1,"expected_price":45000}]}' \
  | jq -c '.error, .changed_items[0].current_price, .catalog_refreshed'
```

TC-24:

```
catalog cache lệch DB (client=1001 size=2: giá cache 45000 != DB 99000) -> xoá key
```

**So sánh TC-18 với TC-22 là trọng tâm của nhóm này.** Cả hai đều trả 409 cho khách, nhưng chỉ TC-22
xoá cache. Giá lệch không đồng nghĩa cache sai — nếu cứ hễ lệch là `DEL` thì vài client cầm menu cũ đủ
sức làm cache bị xoá liên tục.

### 6c. Món bị tắt mà cache vẫn còn

```bash
$PG -c "UPDATE products SET active = false WHERE id = 1;"    # [mutate]
```

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-26 | 400 + `catalog_refreshed: true` | `curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":1}]}' \| jq -c .` | `{"catalog_refreshed":true,"error":"size 2 is not available"}` |

`true` vì cache còn size 2 trong khi DB đã tắt product cha. Nếu trước đó cache đã kịp dựng lại (không
còn Trà Đào) thì ra `false` — lúc đó cache không sai.

Trả dữ liệu về trước khi sang nhóm 7:

```bash
$PG -c "UPDATE products SET active = true WHERE id = 1;" \
    -c "UPDATE product_sizes SET price = 45000 WHERE id = 2;"
```

---

## Nhóm 7 — Cache hỏng thì degrade, không lỗi

```bash
curl -s $B/clients/1001/catalog > /dev/null
$RD SET client:1001:catalog 'không phải JSON'    # [mutate]
```

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-27 | Payload hỏng vẫn trả 200 | `curl -s $B/clients/1001/catalog \| jq -c '.source'` | `"db"` |
| TC-28 | Log báo payload hỏng | `grep 'payload corrupted' /tmp/app.log` | `redis payload corrupted, dropping key: client:1001:catalog` |

Cache là optional — hỏng thì chậm, không được sai.

---

## Nhóm 8 — Delete

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-29 | Xoá product **[mutate]** | `curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/clients/1001/products/25` | `204` |
| TC-30 | Sizes con bị xoá theo | `$PG -c 'SELECT count(*) FROM product_sizes WHERE product_id = 25;'` | `0` |

---

## Nhóm 9 — Case lỗi

| # | Case | Lệnh | Mong đợi |
|---|---|---|---|
| TC-31 | Client không tồn tại | `curl -s $B/clients/9999/catalog` | `{"error":"client_not_found"}` |
| TC-32 | Product không có size | `curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' -d '{"category_id":1,"name":"X","sizes":[]}'` | `{"error":"at least one size is required"}` |
| TC-33 | Category cross-tenant | `curl -s -X POST $B/clients/1001/products -H 'Content-Type: application/json' -d '{"category_id":4,"name":"X","sizes":[{"name":"M","price":1000}]}'` | `{"error":"category not found"}` |
| TC-34 | Size cross-tenant | `curl -s -X PUT $B/clients/1001/sizes/28 -H 'Content-Type: application/json' -d '{"name":"S","price":1000}'` | `{"error":"size not found"}` |
| TC-35 | Order món inactive | `curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":18,"quantity":1}]}'` | `{"catalog_refreshed":false,"error":"size 18 is not available"}` |
| TC-36 | Delete product cross-tenant | `curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/clients/1001/products/17` | `404` |
| TC-37 | Order rỗng | `curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[]}'` | `{"error":"items are required"}` |
| TC-38 | Quantity âm | `curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d '{"items":[{"size_id":2,"quantity":-1}]}'` | `{"error":"size_id and quantity must be positive"}` |
| TC-39 | JSON hỏng | `curl -s -X POST $B/clients/1001/orders -H 'Content-Type: application/json' -d 'not json'` | `{"error":"invalid_json"}` |
| TC-40 | Id không hợp lệ | `curl -s -X PUT $B/clients/1001/sizes/abc -H 'Content-Type: application/json' -d '{"name":"S","price":1000}'` | `{"error":"invalid sizeID"}` |

TC-33 và TC-34 là hai case cross-tenant quan trọng: category 4 và size 28 thuộc client 1002, client
1001 không được đụng tới.

TC-35 trả `catalog_refreshed: false` vì cache cũng đang không có món này — cache không sai, đúng như
cơ chế ở TC-18.

---

## Dọn dẹp sau khi test

```bash
$PG -c "UPDATE product_sizes SET price = 40000 WHERE id = 2;"

docker exec redis redis-cli --scan --pattern 'client:*:catalog' \
  | xargs -r -I{} docker exec redis redis-cli DEL {}

# kiểm tra đã về seed
$PG -c "SELECT (SELECT count(*) FROM products), (SELECT count(*) FROM product_sizes), (SELECT price FROM product_sizes WHERE id=2), (SELECT count(*) FROM products WHERE active=false);"
# 24 | 42 | 40000 | 1
```

---

## Chưa test được bằng bộ này

| Nhánh | Vì sao | Cách test nếu cần |
|---|---|---|
| **Redis down thật** | container Redis dùng chung với lab idempotency và service khác | `docker stop redis` rồi `GET /catalog` — mong đợi vẫn 200 `source: db`; nhớ `docker start redis` sau đó |
| **Race "DEL trước, SET sau"** | cần chạy đồng thời, curl tuần tự không tái hiện được | chạy song song một vòng lặp `GET /catalog` và một vòng lặp `PUT /sizes/2`, sau đó so cache với DB |
| **Cache stampede** | như trên | `for i in {1..50}; do curl -s $B/clients/1001/catalog & done` ngay sau khi `DEL` key, đếm số câu SELECT trong log GORM |

Hai cái sau là điểm yếu đã biết của cache-aside, ghi ở `docs.md` mục *Disadvantages*. Bản demo chấp
nhận, không fix.

---

## Chạy tự động

Script bash chạy cả 40 case và in PASS/FAIL: `test.sh` ở thư mục gốc.

```bash
go run . > /tmp/app.log 2>&1 &
sleep 8
LOG=/tmp/app.log bash test.sh
```

Script tự so khớp chuỗi chính xác, không chỉ check status code. Nó **không** tự reset DB — phải làm
bước Chuẩn bị ở trên trước.
