# GoF Design Patterns — Notes

## 1. Phân biệt Strategy với Command

### Strategy

Payment:

```go
type PaymentStrategy interface {
	Pay(amount int)
}
```

Có các implementation:

- `CreditCard`
- `Paypal`
- `BankTransfer`

Code:

```go
checkout.Pay()
```

Bạn đổi cách thanh toán.

### Command

API:

```
POST /order
```

Tạo:

```go
CreateOrderCommand{
	UserID: 10,
	ItemID: 20,
}
```

Đẩy vào queue:

```go
queue.Push(command)
```

Worker:

```go
command.Execute()
```

Bạn đóng gói một yêu cầu để có thể đổi **thời điểm** hoặc **nơi thực thi**.

## 2. Phân biệt Strategy với Template Method

### Strategy

Dùng khi cần thay thế toàn bộ hành vi hoặc thuật toán tại runtime.

> Strategy = "thay cả quy trình"

### Template Method

Dùng khi workflow tổng thể đã cố định, nhưng cho phép các implementation tùy chỉnh một vài bước bên trong workflow đó.

> Template Method = "giữ quy trình, thay một vài bước"

Ví dụ báo cáo có luồng cố định: tải dữ liệu → kiểm tra dữ liệu → định dạng → xuất file → gửi thông báo. PDF và Excel chỉ thay đổi các bước định dạng, xuất file và thông báo.

## 3. Phân biệt Strategy với State Pattern

### State Pattern

Sử dụng khi một object có nhiều trạng thái và hành vi thay đổi theo trạng thái hiện tại.

Thay vì dùng nhiều `if/else` hoặc `switch` theo state, đóng gói hành vi của mỗi trạng thái vào một type riêng.

- Strategy: thay thế thuật toán.
- State: thay đổi hành vi dựa trên trạng thái hiện tại.

- Strategy: client chọn implementation.
- State: state hiện tại quyết định hành vi và có thể tự chuyển sang state khác.

Ví dụ vòng đời đơn hàng:

```text
Pending → Paid → Shipped → Delivered
   │       │
   └───────┴──→ Cancelled
```

Ví dụ cho thấy `Ship()` bị từ chối ở `PendingState`, nhưng ở `PaidState` nó tự chuyển đơn hàng sang `ShippedState`. Đơn `Pending` có thể hủy trực tiếp; đơn `Paid` được hoàn tiền rồi chuyển sang `Cancelled`. Mỗi state chỉ chứa các luật hợp lệ của chính nó.

> Strategy = "Tôi chọn cách làm"

> State = "Trạng thái hiện tại quyết định cách làm"
