# Product Catalog Caching Design

## Overview

Product API dùng **Cache Aside** trên Redis cho toàn bộ catalog của một client.

Product được cấu hình ở **Client** và mọi Store thuộc Client dùng chung, nên cache tổ chức theo
Client thay vì Store. Mỗi client có đúng **một** key chứa cả categories, products và sizes — đã build
sẵn theo cấu trúc frontend cần.

Lý do chọn Cache Aside:

- Catalog mỗi client nhỏ (~50 product, 5–8 category).
- Write ít, read nhiều.
- Cache miss chấp nhận được.
- Rebuild cache rất rẻ — vài câu SELECT rồi marshal JSON.

---

## Cache Key

```text
client:{clientId}:catalog
```

Ví dụ:

```text
client:1001:catalog
```

### Payload

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

### Tại sao một key cho cả catalog?

Vì UI hoạt động đúng kiểu đó. Frontend mở trang là cần **toàn bộ** category + product để render trang
"All"; click sang category khác thì chỉ filter ở client, không gọi API nữa. Cache theo từng product
hay từng category sẽ tạo ra hàng chục round-trip cho một lần mở menu.

### Chỉ chứa data active

Catalog chỉ chứa **category active + product active**, frontend không phải filter gì thêm.

Món inactive không xuất hiện trong catalog, nhưng `POST /orders` vẫn đọc Postgres nên vẫn reject đúng
— đây là chỗ query path và command path tách nhau.

---

## Cache Strategy

- Pattern: **Cache Aside**
- Invalidation: **`DEL` key sau mỗi write**
- TTL: **1 hour**

TTL chỉ đóng vai trò:

- Giải phóng bộ nhớ Redis với key không còn được dùng.
- Là lưới an toàn nếu invalidation thất bại.

Không dùng TTL để đồng bộ dữ liệu.

**Không warm-up lúc start.** Redis rỗng cũng không sao — request đọc đầu tiên sẽ tự dựng cache.

---

## Read

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

### Pseudocode

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

### Redis down thì sao?

Coi như cache miss: đọc thẳng Postgres, log lỗi, request vẫn thành công.

Cache là optional — **hỏng thì chậm, không được sai**. Đây là điểm khác biệt so với mô hình read model,
nơi Redis là read path duy nhất nên Redis down phải trả 503.

Payload trong Redis bị hỏng cũng xử lý y hệt: xoá key, coi như miss.

---

## Write

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

### Pseudocode

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

## Order — command path không dùng cache

`POST /orders` chạy ba pha:

```text
Pha 1  Đọc size từ Postgres (JOIN product_sizes + products)
       Validate: size tồn tại, product cha thuộc đúng client, product cha active
          │
          ▼
Pha 2  So expected_price client gửi lên với giá DB
       Lệch -> 409 price_changed, KHÔNG tạo order
          │
          ▼
Pha 3  unit_price = product_sizes.price tại thời điểm checkout
```

### Cart gửi gì

```json
{ "items": [ { "size_id": 2, "quantity": 2, "expected_price": 40000 } ] }
```

`expected_price` là giá **client đang hiển thị cho khách**. Server không bao giờ tính tiền theo nó —
chỉ dùng để phát hiện giá đã đổi kể từ lúc khách xem menu. Tiền luôn tính theo `product_sizes.price`
đọc từ Postgres tại thời điểm checkout.

Trường này **optional**. Bỏ trống nghĩa là client chấp nhận mọi giá hiện hành và bỏ qua bước kiểm tra
— dành cho POS của nhân viên, nơi người bấm máy đang nhìn thẳng vào bảng giá của quán.

Không tin giá từ client, cũng không tin giá từ Redis. Với dữ liệu transaction như order/payment, luôn
lấy source of truth từ DB. Redis chỉ phục vụ read-heavy flow như browsing menu.

---

## Giá đổi giữa lúc xem menu và lúc đặt

Đây là vấn đề nêu trong `isssue.md`, và là lý do phải có pha 2.

```text
10:00  Khách mở menu     -> Trà Đào M = 40.000
10:05  Admin đổi giá     -> Trà Đào M = 99.000
10:10  Khách bấm đặt     -> tính tiền theo giá nào?
```

Hệ thống này theo **Option 1** của `isssue.md`: lấy giá tại thời điểm checkout, **và báo cho
khách biết** thay vì lặng lẽ tính giá mới.

### Response 409

```json
{
  "error": "price_changed",
  "message": "Giá đã thay đổi, vui lòng tải lại menu và xác nhận lại",
  "changed_items": [
    { "size_id": 2, "name": "Trà Đào Cam Sả (M)", "expected_price": 40000, "current_price": 99000 }
  ],
  "new_total": 99000,
  "catalog_refreshed": true
}
```

Order **không** được tạo. Mọi dòng lệch được gom vào một response để client sửa một lần, không phải
thử lại từng món.

Khách xem giá mới rồi xác nhận → client gửi lại với `expected_price` = giá mới → 201.

### catalog_refreshed — chữa cache, nhưng chỉ khi cache thật sự sai

Giá lệch **không** đồng nghĩa cache sai. Có hai tình huống hoàn toàn khác nhau:

| | Cache | DB | Client gửi | Kết luận | Hành động |
| --- | --- | --- | --- | --- | --- |
| Cache lệch thật | 40.000 | 99.000 | 40.000 | cache stale | `DEL` key, `catalog_refreshed: true` |
| Client cầm menu cũ | 40.000 | 40.000 | 30.000 | cache đang đúng | không đụng cache, `false` |

Cả hai đều trả 409 cho khách. Nhưng ở dòng thứ hai, khách chỉ mở tab từ lâu — cache hoàn toàn khoẻ.
Nếu cứ hễ lệch giá là `DEL` thì vài client cũ đủ sức làm cache bị xoá liên tục, mọi request đọc sau đó
đều miss vô ích. Vì vậy `reconcileStaleCache` đọc cache lên so với DB trước, lệch thật mới xoá.

Ba dấu hiệu được coi là cache stale:

| Cache | DB | Ý nghĩa |
| --- | --- | --- |
| có size, giá X | giá Y ≠ X | giá lệch |
| có size | product đã tắt | món tắt rồi mà cache vẫn còn |
| không có size | product đang bán | món mới mà cache chưa có |

Xoá bằng `DEL`, **không** rebuild — đúng tinh thần cache-aside, `GET /catalog` kế tiếp sẽ dựng lại.

Món bị tắt giữa chừng cũng đi qua đúng cơ chế này, trả `400 size N is not available` kèm
`catalog_refreshed`.

### Vì sao không giải quyết ở tầng cache

Bỏ Redis đi hoàn toàn thì race này **vẫn còn nguyên**: khách mở menu 10:00, admin đổi giá 10:05, khách
đặt 10:10. Cache chỉ *nới rộng* cửa sổ (tối đa hết TTL thay vì vài phút khách ngồi xem menu), không
phải nguyên nhân. Nên nó phải được xử lý ở tầng checkout, không phải bằng cách cố làm cache realtime.

### Phương án không chọn

`isssue.md` còn nêu Option 2: khoá giá lúc add to cart (`locked_price` + `expired_at`, kiểu hệ
thống booking). Không dùng vì F&B không cần — khách gọi món rồi thanh toán trong vài phút, và việc giữ
giá tạo ra state phải quản lý vòng đời.

---

## Cache Lifecycle

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


POST /orders
      │
      ▼
 Postgres trực tiếp, bỏ qua cache
      │
      ├── giá khớp ──────────────►  201, tính theo giá DB
      │
      └── giá lệch
             │
             ▼
      So cache với DB
             │
             ├── cache lệch  ──►  DEL key
             └── cache đúng  ──►  giữ nguyên
             │
             ▼
      409 price_changed
```

---

## So sánh với Read Model (phương án đã cân nhắc rồi bỏ)

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

## Advantages

- Implementation đơn giản, ít cơ chế nền, dễ suy luận.
- Redis down không làm chết API — chỉ chậm hơn.
- Không tốn tài nguyên duy trì cache cho client không ai đọc.
- Invalidation chỉ một dòng `DEL`, không có logic patch JSON.
- Không có tải nền định kỳ lên DB.

## Disadvantages

- Request đầu tiên sau mỗi write luôn là miss → chậm hơn.
- **Cache stampede**: N request đồng thời vào key vừa bị `DEL` sẽ cùng query DB. Fix bằng
  `singleflight` nếu cần — bản demo này không làm.
- **Race "DEL trước, SET sau"** khiến cache stale cả tiếng (xem dưới).
- Ai sửa thẳng dưới DB thì cache sai tới khi hết TTL, không có cơ chế tự chữa.
- Stale window giữa lúc client đọc menu và lúc đặt hàng — xử lý bằng cách order luôn đọc DB và trả
  409 `price_changed` khi giá lệch, không phải bằng cách cố làm cache realtime.

### Race "DEL trước, SET sau"

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

## Future Improvements

Thiết kế này hợp với catalog cỡ vài chục đến vài trăm product mỗi client. Ngưỡng cần xem lại:

- **Catalog lên hàng nghìn product** → payload một key quá lớn, cân nhắc chia
  `client:{id}:category:{catId}`.
- **Write trở nên thường xuyên** → miss liên tục, lúc đó rebuild-on-write (read model) đáng giá hơn.
- **Bắt buộc Redis luôn có đủ data** → quay lại read model có warm-up + resync.
- **Mỗi store có giá / tồn kho / khuyến mãi riêng** → tách thêm key theo store:

```text
client:{clientId}:catalog
store:{storeId}:inventory
store:{storeId}:promotion
```

---

## Hỏi & đáp — khi nào `reconcileStaleCache` trả `false`?

> Là chỉ có 1 case người dùng vào web quá lâu nhưng không order, cache lúc này vẫn đúng, chỉ có
> data người dùng gửi lên bị sai, nên mới lỗi 409 và không refresh lại cache?

Đúng, đó là case phổ biến nhất — nhưng không phải case duy nhất trả `false`. Có 5 nhánh.

### Cách nghĩ: hai trục độc lập

- **Trục A** — browser khách có cũ không? → quyết định có 409 hay không
- **Trục B** — cache có lệch DB không? → quyết định `reconcileStaleCache` trả `true`/`false`

Hai trục này độc lập nhau. 409 chỉ nói cho bạn biết trục A, không nói gì về trục B — nên mới phải đi
hỏi Redis.

| # | Tình huống | Cache | Return | Vì sao không xoá |
| --- | --- | --- | --- | --- |
| 1 | Browser mở lâu, admin sửa giá, write path đã `DEL` + ai đó nạp lại | đúng | `false` | Cache đúng, xoá là phá |
| 2 | Cache vừa hết TTL / vừa bị `DEL` bởi write path, chưa ai nạp lại | trống | `false` (`!hit`) | Không có key để xoá |
| 3 | Client gửi `expected_price` bịa/sai do bug hoặc script | đúng | `false` | Đây chính là thứ cần chặn |
| 4 | Redis down | không đọc được | `false` | `Get` fail → coi như miss (`redis.go:44-50`) |
| 5 | Write path `DEL` fail lúc Redis chập, hoặc sửa thẳng DB | lệch thật | `true` | Xoá đúng chỗ |

Case 1 là cái bạn mô tả. Nhưng case 2 và 4 cũng cho `false` mà lý do khác hẳn — cache không "đúng",
nó không tồn tại / không đọc được. Kết quả cuối vẫn giống nhau: chẳng có gì để chữa.

### Một chi tiết thú vị: `false` nhưng cache vẫn bị xoá

`redis.go:53-56` — nếu payload trong Redis hỏng (JSON không parse được), `Get` tự gọi `Del` rồi trả
`hit=false`. Lúc đó `reconcileStaleCache` trả `false` nhưng key đã bị dọn sạch. Nên
`catalog_refreshed` không phải chỉ báo chính xác 100% cho "cache có được đụng tới không" — nó chỉ nói
"nhánh reconcile có xoá không".

### Lỗ hổng ngược lại đáng chú ý hơn

Hàm này chỉ chạy khi có 409/400. Nghĩa là:

> Cache đang lệch DB, nhưng browser khách cũng lấy từ đúng cái cache lệch đó → `expected_price` khớp
> `size.Price`? **Không** — `size.Price` đọc từ DB (`order.go:42`), nên vẫn lệch → vẫn 409 → vẫn
> được phát hiện. ✅

Nhưng:

> Cache lệch DB, mà tất cả khách đang cầm giá mới (vừa reload trước khi cache lệch) → không ai bị 409
> → `reconcileStaleCache` không bao giờ được gọi → cache cứ lệch cho tới khi hết TTL.

Nên nhắc lại điểm ở câu trước: đây là **lưới an toàn cơ hội** (opportunistic), không phải cơ chế phát
hiện lệch chủ động. Muốn chủ động thì phải là write-path invalidation cho chắc
(`main.go:122/149/171/200`) + TTL làm chốt chặn cuối.

### Chốt lại

> "không refresh lại cache?"

Chính xác, và đúng là không cần. Thứ đang cũ là cái tab browser, không phải Redis. Refresh Redis
không sửa được browser. Cái sửa được browser là: client nhận 409 → gọi lại `GET /menu` → nhận giá
mới. Việc đó xảy ra bất kể `catalog_refreshed` là gì.
