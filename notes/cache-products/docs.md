# Product Catalog Read Model Design

## Overview

Redis **không** còn được dùng như cache-aside. Nó là **read model** của product catalog:

> Redis is used as a read-model replica of the product catalog. The cache is proactively
> synchronized after every successful write, warmed up during application startup, and
> periodically reconciled with PostgreSQL.

Lý do đổi: requirement mới là *"lúc nào cache cũng phải có đầy đủ data của source of truth"*.
Cache-aside có bản chất ngược lại — cache có thể không tồn tại, chỉ chứa dữ liệu được truy cập,
và cache miss là chuyện bình thường.

| | Cache Aside (bản cũ) | Read Model (bản này) |
| --- | --- | --- |
| Redis chứa gì | phần dữ liệu vừa được đọc | toàn bộ catalog đang bán của mọi client |
| Sau khi write | `DEL` key, chờ read sau dựng lại | `SET` lại ngay bằng bản build từ DB |
| Cache miss | bình thường, xảy ra thường xuyên | bất thường, rebuild ngay và ghi lại Redis |
| Đồng bộ bằng gì | TTL + invalidation | rebuild sau write + warm-up + resync định kỳ |
| TTL | cơ chế fallback | chỉ để dọn key rác |

Product được cấu hình ở **Client** và mọi Store thuộc Client dùng chung, nên read model
tổ chức theo Client thay vì Store.

---

# Cache Key

```
client:{clientId}:catalog
```

Ví dụ:

```
client:1001:catalog
```

Một key duy nhất chứa toàn bộ catalog của client — đúng shape frontend cần: load một lần rồi
filter theo category ở phía client, click sang category khác không phải gọi API.

## Payload

```json
{
  "client_id": "1001",
  "client_name": "Trà Sữa Nhà Làm",
  "rebuilt_at": "2026-08-02T10:00:00Z",
  "categories": [
    {
      "id": 1,
      "name": "Đồ Uống",
      "products": [
        {
          "id": 1,
          "name": "Trà Đào Cam Sả",
          "sizes": [
            { "id": 1, "name": "S", "price": 30000 },
            { "id": 2, "name": "M", "price": 40000 },
            { "id": 3, "name": "L", "price": 50000 }
          ]
        }
      ]
    }
  ]
}
```

Giá nằm ở **size**, không nằm ở product. Món bán một giá vẫn có đúng một size tên `"Mặc định"`.

## Chỉ chứa data active

Read model chỉ chứa **category active + product active**. Frontend không phải filter gì thêm.

Nghĩa là câu *"cache luôn có đầy đủ data của source of truth"* phải hiểu chính xác là
**"cache luôn có đầy đủ phần catalog đang bán của source of truth"**. Món inactive không xuất
hiện trong catalog, nhưng `POST /orders` vẫn đọc Postgres nên vẫn reject đúng — đây chính là
chỗ query path và command path tách nhau.

---

# TTL

```
24 hours
```

TTL **không** phải cơ chế đồng bộ. Nó chỉ để dọn key của client đã bị xoá. Cơ chế đồng bộ thật sự
là resync mỗi 5 phút.

---

# Read

```text
Client
    │
GET /clients/{clientId}/catalog
    │
GET client:{clientId}:catalog
    │
 ┌── Hit ────────────────────┐
 │   Return  source: redis   │
 └───────────────────────────┘
    │
  Miss
    │
Load catalog từ Postgres
    │
SET client:{clientId}:catalog (TTL 24h)
    │
Return  source: rebuilt
```

## Pseudocode

```go
catalog, hit, err := cache.Get(clientID)

if err != nil {
    return 503 cache_unavailable   // Redis down
}
if hit {
    return catalog                 // source: redis
}

catalog, err = service.Rebuild(clientID)   // load DB + SET Redis
if errors.Is(err, ErrClientNotFound) {
    return 404 client_not_found
}

return catalog                     // source: rebuilt
```

Miss **không** fallback kiểu "đọc DB rồi trả thẳng" — như vậy là quay lại cache-aside.
Miss thì rebuild, tức là vẫn ghi lại Redis, cache tự lành ngay chứ không chờ chu kỳ resync.

Ba trạng thái được phân biệt rõ ở tầng `CatalogCache.Get`:

| Trạng thái | Ý nghĩa | Xử lý |
| --- | --- | --- |
| `hit = true` | key có sẵn | trả luôn |
| `hit = false`, `err = nil` | miss / payload hỏng | rebuild từ Postgres |
| `err != nil` | Redis down | 503 |

---

# Write

Mọi write đều theo cùng một flow, chỉ khác thao tác DB ở bước đầu.

```text
Update Postgres
      │
   Commit
      │
Rebuild catalog của đúng client đó
      │
SET client:{clientId}:catalog
      │
Return Success
```

## Pseudocode

```go
product, err := repo.UpdateProduct(clientID, productID, input)
if err != nil {
    return err
}

catalog.RebuildAfterWrite(clientID)   // rebuild fail chỉ log

return product
```

Chỉ rebuild client bị ảnh hưởng, không rebuild toàn hệ thống.

## Tại sao rebuild nguyên catalog thay vì patch JSON?

Catalog một client F&B rất nhỏ — cỡ 30 category / 300 product / 900 size thì `SELECT` toàn bộ,
marshal JSON và `SET` Redis chỉ mất vài chục ms. Đổi lại:

* code cực kỳ đơn giản
* không cần patch từng node trong JSON lồng nhau
* không sợ bug lệch dữ liệu

Với product có nhiều size và giá theo size, patch từng phần trong một JSON lớn phức tạp và dễ lỗi
hơn rebuild rất nhiều. Đây là trade-off đáng giá.

## Rebuild fail thì sao?

Chỉ `log`, API vẫn trả 2xx, **không** rollback DB. Dữ liệu đã nằm trong source of truth rồi;
resync sẽ chữa cache trong tối đa 5 phút.

---

# Warm-up

```text
App start
    │
SELECT id FROM clients
    │
Load catalog từng client
    │
SET Redis
```

Một client fail thì log và đi tiếp, không chặn app start.

---

# Resync

```text
Mỗi 5 phút
    │
Load DB
    │
Overwrite toàn bộ key catalog
```

Đây mới là cơ chế đồng bộ chính. Nó làm những trường hợp sau **tự hồi phục**:

* Redis restart / mất data
* ai đó sửa thẳng dưới database, không qua API
* rebuild sau write bị fail
* key bị eviction

---

# Order — command path không dùng cache

```text
POST /orders
      │
      ▼
Postgres (JOIN product_sizes + products)
      │
      ▼
Validate: size tồn tại, product cha thuộc đúng client, product cha active
      │
      ▼
unit_price = product_sizes.price tại thời điểm checkout
```

Cart chỉ gửi lên `size_id` + `quantity`, **không** gửi price:

```json
{ "items": [ { "size_id": 2, "quantity": 2 } ] }
```

Không tin giá từ client, cũng không tin giá từ Redis. Với dữ liệu transaction như order/payment,
luôn lấy source of truth từ DB. Redis chỉ phục vụ read-heavy flow như browsing menu.

---

# Kiến trúc tổng thể

```text
                 WRITE                                  READ

        Client                                   Client
           │                                        │
           ▼                                        ▼
      PostgreSQL                                  Redis
           │                                        │
        Commit                                   Response
           │
           ▼
 Rebuild client catalog
           │
           ▼
         Redis


                              ORDER

        Client  ──►  PostgreSQL  ──►  giá tại thời điểm checkout
```

---

# Advantages

* Read gần như không bao giờ xuống DB — Redis luôn có sẵn toàn bộ catalog.
* Không có "cold start penalty": warm-up đã dựng cache trước khi nhận request đầu tiên.
* Rebuild toàn bộ nên không có logic patch phức tạp, không sợ lệch dữ liệu.
* Resync làm mọi sai lệch tự hồi phục, kể cả khi có người sửa thẳng dưới DB.
* Payload đúng shape frontend cần, không phải gọi nhiều API rồi ghép ở client.

# Disadvantages

* Mỗi write tốn thêm một lần `SELECT` toàn bộ catalog + `SET` Redis.
* Redis là dependency cứng của read path — Redis down thì `GET /catalog` trả 503.
* Tốn memory hơn cache-aside vì cache cả những phần chưa ai đọc.
* Rebuild race: hai write song song trên cùng client có thể ghi đè nhau (xem dưới).
* Resync định kỳ tạo tải nền đều đặn lên DB, tỉ lệ thuận với số client.

## Rebuild race

Hai write song song trên cùng một client có thể rebuild lệch nhau — goroutine chậm hơn ghi đè
bản mới hơn lên Redis. Demo này chấp nhận, resync 5 phút sẽ chữa.

Muốn chặt hơn:

* per-client mutex quanh đoạn rebuild, hoặc
* gắn version / `updated_at` vào payload rồi so sánh trước khi `SET`.

---

# Future Improvements

Thiết kế này hợp với catalog cỡ vài trăm đến vài nghìn product mỗi client. Nếu sau này:

* catalog lên hàng chục nghìn product, hoặc
* write rất thường xuyên,

thì cân nhắc:

* cập nhật incremental thay vì rebuild toàn bộ,
* chia nhỏ read model thành nhiều key (`client:{id}:category:{catId}`),
* đẩy việc rebuild sang worker qua message queue thay vì làm inline trong request,
* tách theo store nếu mỗi store có giá / tồn kho / khuyến mãi riêng:

```
client:{clientId}:catalog
store:{storeId}:inventory
store:{storeId}:promotion
```
