Có thể tóm gọn như sau:

---

### Có 2 loại race condition

#### 1. Race khi ghi cùng một dữ liệu (technical conflict)

Ví dụ:

* 2 request cùng tạo GR đầu ngày → `E11000` (duplicate key)
* 2 request cùng update một document → `WriteConflict`

Đây chỉ là xung đột ở tầng database, **không làm thay đổi business result**. Sau khi retry, kết quả vẫn đúng như mong muốn.

**Cách xử lý:** Server tự retry.

| Tình huống                            | Lỗi             | Cách xử lý                                              |
| ------------------------------------- | --------------- | ------------------------------------------------------- |
| 2 transaction cùng update 1 document  | `WriteConflict` | MongoDB/driver tự retry                                 |
| 2 transaction cùng tạo mới 1 document | `E11000`        | Retry của application, lần sau sẽ đi nhánh update/merge |

---

#### 2. Race về business logic (business conflict)

Ví dụ:

PO còn **10** sản phẩm.

* Request A nhận **6**
* Request B nhận **6**

Nếu cùng chạy thì tổng thành **12**, vượt số lượng PO.

Đây **không phải lỗi kỹ thuật**, mà là **xung đột business**. Sau khi A thành công thì B chỉ còn được nhận **4**, và chỉ người dùng mới quyết định có nhận 4 hay hủy.

**Cách xử lý:** Trả **409 Conflict** để FE reload dữ liệu và người dùng quyết định.

---

### Idempotency có giải quyết được không?

**Không.**

Idempotency chỉ chống **một request bị thực hiện nhiều lần** (do timeout, retry, mất mạng...).

Nó **không giải quyết**:

* Race tạo/update GR (`E11000`, `WriteConflict`)
* Race vượt số lượng PO (10 nhưng 2 request cùng nhận 6)

Nó chỉ xử lý trường hợp như:

```
Client gửi request
↓
Transaction commit thành công
↓
Response bị mất
↓
Client retry
↓
Nếu không có idempotency → dữ liệu bị ghi thêm lần nữa
```

=> Idempotency giúp đảm bảo **một thao tác chỉ được áp dụng đúng một lần (exactly-once)** khi client retry, nhưng **không thay thế cơ chế retry hoặc kiểm tra business conflict**.
