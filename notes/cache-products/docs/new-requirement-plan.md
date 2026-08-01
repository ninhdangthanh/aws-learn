Với bài toán của bạn, mình sẽ **không dùng Cache Aside nữa**.

Lý do không phải vì Cache Aside không tốt, mà vì **requirement đã thay đổi**.

Yêu cầu mới là:

> **"Lúc nào cache cũng phải có đầy đủ data của source of truth."**

Đây gần như phủ nhận triết lý của Cache Aside.

Cache Aside có bản chất là:

* Cache có thể không tồn tại.
* Cache chỉ chứa dữ liệu được truy cập.
* Cache miss là chuyện bình thường.

Trong khi requirement của bạn là:

* Redis luôn có toàn bộ catalog.
* Read gần như không bao giờ xuống DB.
* Redis là nguồn phục vụ read chính.

=> Redis không còn là cache "lazy" nữa mà trở thành **read model**.

---

# Với bài toán F&B này mình sẽ thiết kế như sau

NOTE: các data như client, category, products, size và giá, nhớ là như thật trong source of truth (Posgres) luôn

Source of truth

```text
Postgres
```

Read model

```text
Redis
```

Redis lưu toàn bộ catalog của từng client.

Ví dụ

```text
client:1:catalog
```

Payload

```json
{
  "client_id":1,
  "categories":[
    {
      "id":1,
      "name":"Drinks",
      "products":[
        {
          "id":100,
          "name":"Peach Tea",
          "sizes":[
            {
              "id":1,
              "name":"S",
              "price":30000
            },
            {
              "id":2,
              "name":"M",
              "price":40000
            }
          ]
        }
      ]
    }
  ]
}
```

---

## Tại sao cache nguyên catalog?

Vì UI của bạn hoạt động đúng kiểu đó.

User mở web.

Frontend cần

```text
ALL PRODUCTS

+

ALL CATEGORIES
```

Sau đó

Click

```text
Drinks
```

Frontend chỉ filter.

Không cần gọi API.

Thực tế React/Vue thường làm đúng vậy.

Load một lần.

Filter ở client.

---

## Read

```text
Client

↓

Redis

↓

Response
```

Không đụng DB.

---

## Write

Ví dụ

Update giá size M

```text
Peach Tea

M

40000

↓

45000
```

Flow

```text
Update DB

↓

Commit

↓

Rebuild catalog của client đó

↓

SET Redis
```

Chỉ rebuild

```text
client 1001
```

không rebuild toàn bộ hệ thống.

---

## Có cần rebuild toàn catalog không?

Theo mình:

**Có.**

Lý do là catalog của một client F&B không lớn.

Ví dụ

```
30 category

300 products

900 sizes
```

SELECT tất cả

*

marshal JSON

*

SET Redis

thường chỉ vài chục ms.

Trong khi đổi lại:

* code cực kỳ đơn giản
* không cần patch JSON
* không cần sửa từng node
* không sợ bug.

Đây là trade-off rất đáng giá.

---

# Warm-up

App start

↓

List client

↓

Load DB

↓

SET Redis

---

# Resync

5 phút

↓

Load DB

↓

Overwrite Redis

Mục đích:

* Redis restart
* ai sửa DB trực tiếp
* rebuild fail
* eviction

đều tự hồi phục.

---

# TTL

Mình sẽ **không phụ thuộc TTL**.

Có thể

```
TTL = 24h
```

hoặc

```
No Expire
```

Resync mới là cơ chế đồng bộ.

TTL chỉ là safety net.

---

# Có cần Write Through không?

Theo mình **không cần cố gọi nó là Write Through**.

Thực tế bạn đang làm:

```
DB

↓

Rebuild Read Model

↓

Redis
```

Nó giống CQRS projection hơn.

Nếu ghi trong design document mình sẽ viết:

> **Redis is used as a read-model replica of the product catalog. The cache is proactively synchronized after every successful write, warmed up during application startup, and periodically reconciled with PostgreSQL.**

Mô tả này đúng bản chất hơn là cố ép vào một thuật ngữ cache kinh điển.

---

# Mình sẽ chọn kiến trúc này

```
                 WRITE

        Client
           │
           ▼
      PostgreSQL
           │
        Commit
           │
           ▼
 Rebuild client catalog
           │
           ▼
         Redis


                  READ

        Client
           │
           ▼
         Redis
```

**Lý do mình chọn phương án này cho bài toán của bạn:**

* Catalog của mỗi client nhỏ (vài trăm đến vài nghìn sản phẩm), nên rebuild toàn bộ sau mỗi lần thay đổi là hoàn toàn chấp nhận được.
* UI luôn cần toàn bộ category và product để hiển thị trang "All" và lọc theo category ở phía frontend.
* Product có thêm nhiều `size` và `price theo size`, nên việc cố cập nhật từng phần trong một JSON lớn sẽ phức tạp và dễ lỗi hơn nhiều so với rebuild.
* Requirement "cache luôn có đầy đủ dữ liệu của source of truth" phù hợp với mô hình **read model được đồng bộ chủ động**, không phải cache-aside.

Với quy mô và yêu cầu hiện tại, đây là thiết kế vừa đơn giản, dễ bảo trì, vừa đáp ứng đúng hành vi mà frontend cần. Nếu sau này catalog tăng lên hàng chục nghìn sản phẩm hoặc write rất thường xuyên, khi đó mới cần cân nhắc chuyển sang cập nhật incremental hoặc chia nhỏ cache theo nhiều key.


