Đây là một vấn đề **khác với stale cache thông thường**. Nó liên quan đến **data consistency trong quá trình checkout/order**.

Ví dụ flow:

```
10:00 Customer mở menu
        |
        v
GET /products
        |
        v
Redis trả:
Pizza = 100.000

10:05 Admin sửa:
Pizza = 120.000

10:10 Customer checkout
```

Lúc này customer đang giữ:

```json
{
  "product_id": 1,
  "name": "Pizza",
  "price": 100000
}
```

Trong cart.

Câu hỏi là: **order có lấy giá 100.000 hay 120.000?**

---

## Cách xử lý chuẩn trong hệ thống e-commerce/F&B

### Không tin dữ liệu từ cart/client

Cart chỉ nên lưu:

```json
{
  "product_id": 1,
  "quantity": 2
}
```

Không nên lưu:

```json
{
  "product_id": 1,
  "price": 100000
}
```

vì price có thể đã thay đổi.

---

## Khi Place Order → validate lại Product

Flow:

```
Add to cart
      |
      v
Store:
product_id + quantity


Place Order
      |
      v
Backend lấy product hiện tại
      |
      v
Validate
      |
      v
Create Order
```

Ví dụ:

Cart:

```json
{
  "items": [
    {
      "product_id": 100,
      "quantity": 2
    }
  ]
}
```

Khi checkout:

Backend:

```
GET product:100
```

Database/Redis:

```json
{
  "id":100,
  "name":"Pizza",
  "price":120000
}
```

Tạo order:

```json
{
  "product_id":100,
  "quantity":2,
  "unit_price":120000
}
```

---

## Nhưng nếu Redis vẫn stale thì sao?

Đây mới là phần quan trọng.

Ví dụ:

```
Redis:
Pizza = 100k

DB:
Pizza = 120k
```

Nếu checkout đọc Redis:

=> vẫn sai.

Vì vậy các hệ thống thường **không dùng cache cho bước quan trọng như payment/order confirmation**.

---

## Tách 2 loại API

### 1. Read API (customer browsing)

Ví dụ:

```
GET /menu
GET /products
```

Dùng Redis:

```
Redis
 |
return menu
```

Mục tiêu:

* nhanh
* giảm DB load

Chấp nhận stale vài phút.

---

### 2. Transaction API (checkout)

Ví dụ:

```
POST /orders
```

Không tin cache.

Flow:

```
Create Order
      |
      v
Product Service
      |
      v
Database
      |
      v
Get latest price
      |
      v
Create order
```

---

## Nếu product service dùng microservice

Kiến trúc thường:

```
Customer API

GET /products
       |
       v
Product Service
       |
       v
Redis


POST /orders
       |
       v
Order Service
       |
       v
Product Service
       |
       v
Database
```

Order Service gọi Product Service để lấy giá mới.

---

## Một vấn đề khác: giá thay đổi sau khi add cart

Ví dụ:

```
Customer:
Add Pizza 100k

Admin:
Update Pizza 120k

Customer:
Checkout
```

Có hai business rule:

### Option 1: Giá tại thời điểm checkout

Phổ biến:

```
Order price = current product price
```

Customer thấy thông báo:

> Price has changed.

---

### Option 2: Lock price khi add cart

Một số hệ thống:

```
Cart Item:

product_id
quantity
locked_price
expired_at
```

Ví dụ:

```
Pizza:
100k
valid until 10 minutes
```

Giống các hệ thống booking.

Nhưng F&B thường không cần.

---

## Với hệ thống F&B POS của bạn, mình sẽ thiết kế:

### Product cache

```
client:{clientId}:products
```

cho:

```
GET /menu
```

---

### Cart

MongoDB/Redis:

```json
{
  "cart_id": "xxx",
  "items": [
    {
      "product_id": "p1",
      "qty": 2
    }
  ]
}
```

Không lưu price.

---

### Checkout

```
POST /orders
        |
        v
Product Service
        |
        v
Get latest product from DB
        |
        v
Validate:
- product exists
- active?
- price
- availability
        |
        v
Create order
```

---

Tóm lại:

> Redis stale data không nên được giải quyết bằng cách cố làm Redis luôn realtime. Với dữ liệu transaction như order/payment, hãy bypass cache và lấy source of truth từ DB. Redis chỉ phục vụ read-heavy flow như browsing menu.

Đây cũng là lý do các hệ thống lớn thường phân biệt rất rõ:

* **Query path** → có cache.
* **Command/transaction path** → dùng database chính xác.

---

# Đã implement (xem `order.go`)

Chọn **Option 1** — giá tại thời điểm checkout, và **có báo cho khách** thay vì lặng lẽ tính giá mới.

Cart gửi kèm `expected_price` là giá client đang hiển thị. Server không bao giờ tính tiền theo nó,
chỉ dùng để so với `product_sizes.price` trong Postgres:

* khớp → tạo order theo giá DB
* lệch → `409 price_changed` kèm `changed_items` (expected vs current) và `new_total`, **không** tạo
  order; khách xác nhận rồi client gửi lại với giá mới

Trường này optional — bỏ trống thì không kiểm tra, dành cho POS của nhân viên.

**Ngoài ra khi phát hiện lệch, server còn tự chữa cache** (`reconcileStaleCache`): đọc catalog trong
Redis lên so với DB, chỉ `DEL` key khi cache thật sự sai. Giá lệch không đồng nghĩa cache sai — khách
mở menu từ lâu thì cache vẫn đang đúng, xoá lúc đó là vô ích và tạo đường cho client cũ làm cache bị
xoá liên tục. Response trả `catalog_refreshed` để client biết có cần tải lại menu không.

Không chọn Option 2 (lock price khi add cart) vì F&B không cần và nó tạo thêm state phải quản lý vòng đời.

Chi tiết flow: `docs.md`, mục *"Giá đổi giữa lúc xem menu và lúc đặt"*.
