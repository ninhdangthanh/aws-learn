
# Cache Strategy

Sử dụng **Cache Aside**.

Lý do:

* Catalog của mỗi client nhỏ (~50 products, 5–8 categories).
* Write ít, read nhiều.
* Cache miss có thể chấp nhận được.
* Rebuild cache rất rẻ.

---

# Cache Key

Mỗi client chỉ cache **một catalog**.

```text
client:{clientID}:catalog
```

Ví dụ

```text
client:1001:catalog
```

Catalog gồm:

* Categories
* Products
* Product Sizes

đã được build theo đúng cấu trúc frontend cần.

---

# Read

```
GET catalog

↓

Redis

↓

HIT
    ↓
Return

MISS
    ↓
Load toàn bộ catalog từ Postgres

↓

Build catalog

↓

SET Redis (TTL 1h)

↓

Return
```

---

# Write

Mọi thao tác

* Create
* Update
* Delete

đều theo flow

```
Update Postgres

↓

Commit

↓

DEL client:{clientID}:catalog

↓

Return
```

Không rebuild ngay.

Request đọc đầu tiên sẽ tự rebuild cache.

---

# TTL

```
1 hour
```

TTL chỉ là cơ chế dọn dẹp.

Cache chủ yếu được invalidate sau mỗi write.

---

# Startup

Không warm-up.

Redis rỗng cũng không sao.

Cache sẽ được tạo khi có request đầu tiên.

---

# Cache Miss

Do catalog rất nhỏ nên cache miss chỉ cần

```
SELECT categories

+

SELECT products

+

SELECT product_sizes

↓

Build catalog

↓

SET Redis
```

Chi phí rất thấp.

---

# Order

Order không dùng cache.

Luôn đọc trực tiếp từ Postgres để lấy:

* Product
* Size
* Price

đảm bảo dữ liệu mới nhất.

---

Theo mình đây là plan hợp lý nhất cho bài toán của bạn. Nó đúng với tinh thần **Cache Aside**, chỉ khoảng 1/3 độ phức tạp so với plan "read-model replica", nhưng với quy mô **50 sản phẩm/client** thì hiệu năng gần như không khác biệt. Khi quy mô hoặc yêu cầu thay đổi (ví dụ hàng nghìn sản phẩm hoặc bắt buộc Redis luôn đầy đủ dữ liệu), lúc đó mới đáng cân nhắc chuyển sang mô hình rebuild-on-write hoặc read-model replica.
