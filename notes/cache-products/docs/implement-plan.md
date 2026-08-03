# Implement Plan — chuyển cache-products từ Cache-Aside sang Read Model

Input: `docs/new-requirement-plan.md`. File này là bản **đã chốt**, dùng làm đầu vào để implement.

---

## Khoảng cách giữa plan và code hiện tại

| Hạng mục | Code hiện tại | Plan mới |
| --- | --- | --- |
| Data model | 1 bảng `products` (id, client_id, name, price, active) | thêm `clients`, `categories`, `product_sizes` — giá nằm ở size |
| Redis | cache-aside, key `client:X:products`, TTL 1h, `Del` khi write | read model, key `client:X:catalog`, JSON lồng category→product→size, `SET` rebuild khi write |
| Read path | miss → DB → set cache | đọc thẳng Redis |
| Warm-up | không có | app start: list client → load DB → SET |
| Resync | không có | ticker 5 phút, overwrite toàn bộ |
| Order API | đọc DB theo `product_id` | giữ lại, đổi sang `size_id` |

---

## Các quyết định đã chốt

Vòng 1:

| Câu hỏi | Chốt |
| --- | --- |
| Giá nằm ở đâu — product hay size? | Chỉ ở size, bỏ `price` khỏi product |
| Redis miss key hoặc Redis down thì sao? | Miss thì build ngay từ DB rồi SET (self-healing) |
| Phạm vi API? | GET catalog + CRUD product/size + giữ order |
| Code mới đặt ở đâu? | Refactor tại chỗ trong `cache-products/` |

Vòng 2:

| Câu hỏi | Chốt |
| --- | --- |
| Product/category inactive có vào Redis không? | Không — chỉ cache món active |
| TTL key catalog? | 24h (safety net) |
| Quy mô seed data? | 2 client, ~5 category, ~20 product, ~45 size |
| Kiểu `client_id`? | Giữ string |

---

## 1. Data model

Bỏ cột `price` khỏi `products`. Mọi product bắt buộc có ít nhất 1 size (món 1 giá thì tạo size `"Mặc định"`).

```text
clients(id VARCHAR PK, name, created_at)
categories(id BIGSERIAL PK, client_id, name, sort_order, active, timestamps)
products(id BIGSERIAL PK, client_id, category_id FK, name, active, timestamps)
product_sizes(id BIGSERIAL PK, product_id FK, name, price CHECK > 0, sort_order, timestamps)
```

`client_id` giữ kiểu **string** như code hiện tại (`"1001"`, `"1002"`).
Payload Redis vì vậy là `"client_id": "1001"`, không phải `1` như ví dụ trong `new-requirement-plan.md`.

---

## 2. Read path — self-healing khi miss

```text
GET /clients/:clientID/catalog

  Redis HIT   -> trả luôn                       source: redis
  Redis MISS  -> build từ DB + SET rồi trả      source: rebuilt
  Redis DOWN  -> 503 {"error":"cache_unavailable"}
  client không tồn tại -> 404 {"error":"client_not_found"}
```

Không fallback kiểu "đọc DB rồi trả thẳng" — như vậy là quay lại cache-aside.
Miss thì **rebuild**, tức là vẫn ghi lại Redis, cache tự lành, không phải chờ tới chu kỳ resync.

---

## 3. Cache chỉ chứa data active

Catalog trong Redis chỉ chứa **category active + product active**. Frontend không phải filter.

Hệ quả cần ý thức: câu *"cache luôn có đầy đủ data của source of truth"* phải hiểu chính xác hơn là
**"cache luôn có đầy đủ phần catalog đang bán của source of truth"**. Món inactive không xuất hiện
trong catalog, nhưng order API vẫn đọc DB nên vẫn reject đúng với lý do `product is not available`
— đây chính là chỗ chứng minh query path và command path tách nhau.

Payload không cần field `active`:

```json
{
  "client_id": "1001",
  "rebuilt_at": "2026-08-02T10:00:00Z",
  "categories": [
    {
      "id": 1,
      "name": "Đồ Uống",
      "products": [
        { "id": 100, "name": "Trà Đào", "sizes": [
            { "id": 1, "name": "S", "price": 30000 },
            { "id": 2, "name": "M", "price": 40000 }
        ]}
      ]
    }
  ]
}
```

---

## 4. Redis key & TTL

```text
key  : client:{clientID}:catalog
value: JSON string ở trên
TTL  : 24h — chỉ là safety net để dọn key của client đã xoá
```

Resync 5 phút mới là cơ chế đồng bộ chính. Response `GET /catalog` trả kèm `ttl` để demo nhìn thấy.

---

## 5. API scope

| Method | Path | Ghi chú |
| --- | --- | --- |
| GET | `/health` | |
| GET | `/clients/:clientID/catalog` | đọc Redis, miss thì rebuild |
| POST | `/clients/:clientID/products` | tạo product + sizes trong 1 transaction → rebuild |
| PUT | `/clients/:clientID/products/:productID` | update product + replace sizes → rebuild |
| DELETE | `/clients/:clientID/products/:productID` | xoá product (cascade sizes) → rebuild |
| PUT | `/clients/:clientID/sizes/:sizeID` | đổi tên/giá 1 size → rebuild |
| POST | `/clients/:clientID/orders` | **đọc thẳng Postgres**, không đụng Redis |

Client và category **không có API tạo/sửa** — seed sẵn trong DB.

Order item đổi từ `product_id` sang `size_id`:

```json
{ "items": [ { "size_id": 1, "quantity": 2 } ] }
```

Validate khi order: size tồn tại, product cha thuộc đúng client, product cha `active = true`.
`unit_price` lấy từ `product_sizes.price` tại thời điểm checkout. Response giữ `price_source: "db"`.

---

## 6. Rebuild / warm-up / resync

- **Sau mỗi write**: commit DB xong → build lại catalog của **đúng client đó** → `SET`. Không đụng client khác.
- **SET fail**: chỉ `log`, API vẫn trả 2xx, không rollback DB. Resync sẽ chữa.
- **Warm-up**: lúc app start, `SELECT id FROM clients` → build + SET từng client. Một client fail thì log và đi tiếp, không chặn app start.
- **Resync**: `time.Ticker` 5 phút trong goroutine, lặp lại đúng logic warm-up, overwrite toàn bộ. Dừng theo context khi shutdown.

---

## 7. Điểm đã biết trước là chưa hoàn hảo (chấp nhận cho demo)

Hai write song song trên cùng một client có thể rebuild race — goroutine chậm hơn ghi đè bản mới hơn
lên Redis. Demo chấp nhận, resync 5 phút sẽ chữa. Nếu về sau cần chặt hơn: per-client mutex, hoặc
gắn version/`updated_at` vào payload rồi so sánh trước khi SET.

---

## 8. Code layout

Refactor **tại chỗ** trong `cache-products/`, ghi đè bản cache-aside cũ (vẫn còn trong git history).

```text
main.go      config, Gin router, handler
model.go     Client/Category/Product/ProductSize, DTO catalog, DTO order, validate
postgres.go  CatalogRepo — mọi thao tác Postgres qua GORM (gồm LoadCatalog cho 1 client)
redis.go     CatalogCache — Get/Set/Del key client:X:catalog
catalog.go   Service: Rebuild(clientID), WarmUp(), StartResync(ctx)
```

`README.md` và `docs.md` phải viết lại theo kiến trúc read model, kèm bộ curl test mới.

---

## 9. Seed data (menu thật, tiếng Việt)

2 client, ~5 category, ~20 product, ~45 size — đủ nhìn ra cấu trúc lồng nhau mà curl vẫn đọc được bằng mắt.

```text
client 1001 — Trà Sữa Nhà Làm
  Đồ Uống : Trà Đào (S/M/L), Trà Sữa Trân Châu (M/L), ...
  Topping : Trân Châu Đen, Thạch Phô Mai, ... (size "Mặc định")
  Ăn Vặt  : ...

client 1002 — Pizza Ba Miền
  Pizza   : Pizza Hải Sản (S/M/L), Pizza Bò (S/M/L), ...
  Mì Ý    : ...
  Nước    : ...
```

Seed phải có ít nhất **1 product `active = false`** để test order bị reject và để thấy nó không xuất
hiện trong catalog.
