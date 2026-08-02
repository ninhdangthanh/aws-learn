# Product Catalog Caching Design

## Overview

Product API dùng **Cache Aside** trên Redis cho toàn bộ catalog của một client.

Product được cấu hình ở **Client** và mọi Store thuộc Client dùng chung, nên cache tổ chức theo
Client thay vì Store. Mỗi client có đúng **một** key chứa cả categories, products và sizes — đã build
sẵn theo cấu trúc frontend cần.

Lý do chọn Cache Aside:

* Catalog mỗi client nhỏ (~50 product, 5–8 category).
* Write ít, read nhiều.
* Cache miss chấp nhận được.
* Rebuild cache rất rẻ — vài câu SELECT rồi marshal JSON.

---

# Cache Key

```
client:{clientId}:catalog
```

Ví dụ:

```
client:1001:catalog
```

## Payload

```json
{
  "client_id": "1001",
  "client_name": "Trà Sữa Nhà Làm",
  "cached_at": "2026-08-02T10:00:00Z",
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

`cached_at` cho biết entry được build lúc nào — nhìn vào đây biết cache đang cũ bao lâu.

## Tại sao một key cho cả catalog?

Vì UI hoạt động đúng kiểu đó. Frontend mở trang là cần **toàn bộ** category + product để render trang
"All"; click sang category khác thì chỉ filter ở client, không gọi API nữa. Cache theo từng product
hay từng category sẽ tạo ra hàng chục round-trip cho một lần mở menu.

## Chỉ chứa data active

Catalog chỉ chứa **category active + product active**, frontend không phải filter gì thêm.

Món inactive không xuất hiện trong catalog, nhưng `POST /orders` vẫn đọc Postgres nên vẫn reject đúng
— đây là chỗ query path và command path tách nhau.

---

# Cache Strategy

* Pattern: **Cache Aside**
* Invalidation: **`DEL` key sau mỗi write**
* TTL: **1 hour**

TTL chỉ đóng vai trò:

* Giải phóng bộ nhớ Redis với key không còn được dùng.
* Là lưới an toàn nếu invalidation thất bại.

Không dùng TTL để đồng bộ dữ liệu.

**Không warm-up lúc start.** Redis rỗng cũng không sao — request đọc đầu tiên sẽ tự dựng cache.

---

# Read

```text
Client
    │
GET /clients/{clientId}/catalog
    │
GET client:{clientId}:catalog
    │
 ┌── Hit ─────────────────┐
 │   Return  source: cache│
 └────────────────────────┘
    │
  Miss
    │
SELECT categories + products + product_sizes
    │
Build catalog
    │
SET client:{clientId}:catalog (TTL 1h)
    │
Return  source: db
```

## Pseudocode

```go
if catalog, hit := cache.Get(clientID); hit {
    return catalog                  // source: cache
}

catalog, err := repo.LoadCatalog(clientID)
if errors.Is(err, ErrClientNotFound) {
    return 404 client_not_found
}

cache.Set(clientID, catalog)

return catalog                      // source: db
```

## Redis down thì sao?

Coi như cache miss: đọc thẳng Postgres, log lỗi, request vẫn thành công.

Cache là optional — **hỏng thì chậm, không được sai**. Đây là điểm khác biệt so với mô hình read model,
nơi Redis là read path duy nhất nên Redis down phải trả 503.

Payload trong Redis bị hỏng cũng xử lý y hệt: xoá key, coi như miss.

---

# Write

Mọi thao tác — create, update, delete product, đổi giá size — đều theo cùng một flow:

```text
Update Postgres
      │
   Commit
      │
DEL client:{clientId}:catalog
      │
Return Success
```

## Pseudocode

```go
product, err := repo.UpdateProduct(clientID, productID, input)
if err != nil {
    return err
}

cache.Del(clientID)

return product
```

**Không rebuild ngay.** Request đọc đầu tiên sau đó sẽ tự dựng lại cache.

Lý do không ghi thẳng dữ liệu mới vào Redis: phải duy trì hai đường code cùng build ra một payload
(một ở nhánh read, một ở nhánh write), rất dễ lệch nhau. `DEL` thì chỉ có một đường build duy nhất.

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

Không tin giá từ client, cũng không tin giá từ Redis. Với dữ liệu transaction như order/payment, luôn
lấy source of truth từ DB. Redis chỉ phục vụ read-heavy flow như browsing menu.

---

# Cache Lifecycle

```text
            GET Catalog
                 │
                 ▼
            Redis Hit ?
            /        \
         Yes          No
          │            │
          ▼            ▼
   Return Cache      Load DB
                        │
                        ▼
              SET Redis (TTL 1h)
                        │
                        ▼
                Return Response


Create / Update / Delete / Đổi giá size
           │
           ▼
      Update Postgres
           │
           ▼
 DEL client:{clientId}:catalog
           │
           ▼
   Next Read Rebuild Cache


POST /orders  ──►  Postgres trực tiếp, bỏ qua cache
```

---

# So sánh với Read Model (phương án đã cân nhắc rồi bỏ)

Phương án còn lại là biến Redis thành read model: warm-up lúc start, rebuild + `SET` sau mỗi write,
resync định kỳ 5 phút. Xem `docs/implement-plan.md`.

| | Cache Aside (bản này) | Read Model |
| --- | --- | --- |
| Điền cache bằng gì | chỉ read miss | warm-up + rebuild sau write + resync |
| Write path làm gì | `DEL` | `SET` bản mới |
| Cache chứa gì | phần đã có người đọc | toàn bộ catalog mọi client |
| Redis down | fallback đọc DB | 503 |
| Ai sửa thẳng DB | cache sai tới hết TTL | resync chữa trong ≤ 5 phút |
| Độ phức tạp | ~1/3 | cao hơn |

Chọn Cache Aside vì với ~50 product/client thì hiệu năng gần như không khác biệt, trong khi code đơn
giản hơn hẳn. Cái phải từ bỏ là requirement *"lúc nào cache cũng phải có toàn bộ data của source of
truth"* — đó là một đánh đổi có ý thức, không phải bỏ sót.

---

# Advantages

* Implementation đơn giản, ít cơ chế nền, dễ suy luận.
* Redis down không làm chết API — chỉ chậm hơn.
* Không tốn tài nguyên duy trì cache cho client không ai đọc.
* Invalidation chỉ một dòng `DEL`, không có logic patch JSON.
* Không có tải nền định kỳ lên DB.

# Disadvantages

* Request đầu tiên sau mỗi write luôn là miss → chậm hơn.
* **Cache stampede**: N request đồng thời vào key vừa bị `DEL` sẽ cùng query DB. Fix bằng
  `singleflight` nếu cần — bản demo này không làm.
* **Race "DEL trước, SET sau"** khiến cache stale cả tiếng (xem dưới).
* Ai sửa thẳng dưới DB thì cache sai tới khi hết TTL, không có cơ chế tự chữa.
* Stale window giữa lúc client đọc menu và lúc đặt hàng — được xử lý bằng cách order luôn đọc DB.

## Race "DEL trước, SET sau"

Điểm yếu kinh điển của Cache Aside:

```text
T1 (read)  : miss, SELECT DB xong -> đang giữ catalog giá 40k
T2 (write) : UPDATE giá 45k, commit, DEL key  (key đang trống, no-op)
T1 (read)  : SET key = catalog giá 40k, TTL 1h
=> Redis giữ giá cũ tới 1 tiếng, không có cơ chế nào tự chữa
```

Xác suất thấp vì cửa sổ race hẹp và write ít, nhưng hậu quả kéo dài hết TTL.

Mô hình read model không có lỗi này vì write luôn `SET` bản mới **sau** commit.

Giảm nhẹ nếu cần: TTL ngắn hơn, delayed double-delete (`DEL` lần hai sau vài trăm ms), hoặc versioned
key. Bản demo chấp nhận và dựa vào TTL.

---

# Future Improvements

Thiết kế này hợp với catalog cỡ vài chục đến vài trăm product mỗi client. Ngưỡng cần xem lại:

* **Catalog lên hàng nghìn product** → payload một key quá lớn, cân nhắc chia
  `client:{id}:category:{catId}`.
* **Write trở nên thường xuyên** → miss liên tục, lúc đó rebuild-on-write (read model) đáng giá hơn.
* **Bắt buộc Redis luôn có đủ data** → quay lại read model có warm-up + resync.
* **Mỗi store có giá / tồn kho / khuyến mãi riêng** → tách thêm key theo store:

```
client:{clientId}:catalog
store:{storeId}:inventory
store:{storeId}:promotion
```
