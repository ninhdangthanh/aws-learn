# Product API Caching Design

## Overview

Để giảm tải cho Database và cải thiện thời gian phản hồi của Customer API, Product API sử dụng **Redis Cache** theo mô hình **Cache Aside** kết hợp với **Cache Invalidation** và **TTL**.

Do Product được cấu hình ở **Client** và tất cả Store thuộc Client đều sử dụng chung Product, cache được tổ chức theo **Client** thay vì Store.

---

# Cache Key

```
client:{clientId}:products
```

Ví dụ:

```
client:1001:products
```

Cache chứa toàn bộ danh sách Product của Client.

---

# Cache Strategy

* Pattern: **Cache Aside**
* Invalidation: **Delete cache sau khi dữ liệu được cập nhật**
* TTL: **1 hour** (có thể điều chỉnh tùy traffic)

TTL chỉ đóng vai trò:

* Giải phóng bộ nhớ Redis đối với các key không còn được sử dụng.
* Là cơ chế fallback nếu cache invalidation thất bại.

Không sử dụng TTL để đồng bộ dữ liệu.

---

# Read Product

## Flow

```text
Client
    │
GET /products
    │
Backend
    │
GET client:{clientId}:products
    │
 ┌── Hit ───────────────┐
 │                      │
 │      Return Cache    │
 │                      │
 └──────────────────────┘
    │
Miss
    │
Database
    │
SET Redis (TTL = 1 hour)
    │
Return Response
```

## Pseudocode

```go
products := redis.Get(cacheKey)

if products != nil {
    return products
}

products = db.GetProducts(clientId)

redis.Set(cacheKey, products, time.Hour)

return products
```

---

# Create Product

## Flow

```text
Create Product
      │
      ▼
Database
      │
      ▼
Delete Redis Cache
      │
      ▼
Return Success
```

## Pseudocode

```go
db.CreateProduct(product)

redis.Del(cacheKey)

return success
```

Không ghi trực tiếp dữ liệu mới vào Redis.

Request đọc tiếp theo sẽ tự xây dựng lại cache.

---

# Update Product

## Flow

```text
Update Product
      │
      ▼
Database
      │
      ▼
Delete Redis Cache
      │
      ▼
Return Success
```

## Pseudocode

```go
db.UpdateProduct(product)

redis.Del(cacheKey)

return success
```

Không update cache trực tiếp để tránh đồng bộ nhiều logic giữa Database và Redis.

---

# Delete Product

## Flow

```text
Delete Product
      │
      ▼
Database
      │
      ▼
Delete Redis Cache
      │
      ▼
Return Success
```

## Pseudocode

```go
db.DeleteProduct(productId)

redis.Del(cacheKey)

return success
```

---

# Cache Lifecycle

```text
            GET Products
                 │
                 ▼
          Redis Hit ?
          /        \
       Yes          No
       │            │
       ▼            ▼
Return Cache      Read DB
                     │
                     ▼
          Cache Redis (TTL 1h)
                     │
                     ▼
             Return Response


Create / Update / Delete
           │
           ▼
      Update Database
           │
           ▼
 Delete client:{clientId}:products
           │
           ▼
Next Read Rebuild Cache
```

---

# Why Cache by Client?

Current business model:

```text
Client
    ├── Product A
    ├── Product B
    ├── Product C
    └── ...

Store 1
Store 2
Store 3
```

All stores of a client share the same product catalog.

Caching by Store would duplicate identical data:

```
store:1:products
store:2:products
store:3:products
```

Instead, cache once:

```
client:{clientId}:products
```

Benefits:

* Reduce Redis memory usage.
* Simpler cache invalidation.
* Only one cache rebuild per client.

---

# Advantages

* Fast response time for high-read APIs.
* Reduced database load.
* Simple implementation (Cache Aside).
* Easy cache invalidation.
* TTL automatically cleans up inactive cache entries.
* Avoid duplicated cache across stores.


# Disadvantages

* Big Issue: Có thể xảy ra stale data (ví dụ load lên rồi, người khác sửa, nhưng khách hàng add vào order rồi nhưng chưa place order)
* Cache Invalidation phức tạp
* Cache miss gây tăng tải Database
* Redis trở thành dependency quan trọng
* Tốn thêm memory
* Cache consistency khó khi có nhiều service

---

# Future Improvements

Nếu trong tương lai mỗi Store có:

* giá khác nhau,
* sản phẩm khác nhau,
* tồn kho khác nhau,
* chương trình khuyến mãi khác nhau,

thì cache key có thể được chuyển sang:

```
store:{storeId}:products
```

hoặc kết hợp nhiều lớp cache như:

```
client:{clientId}:products
store:{storeId}:inventory
store:{storeId}:promotion
```

để phản ánh dữ liệu riêng của từng Store mà vẫn tối ưu hiệu năng.
