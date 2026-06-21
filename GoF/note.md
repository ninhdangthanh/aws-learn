# 1. Phân biệt Strategy với Command

## Strategy

Payment:

```go
type PaymentStrategy interface {
	Pay(amount int)
}
```

Có:
*   CreditCard
*   Paypal
*   BankTransfer

Code:

```go
checkout.Pay()
```

Bạn đổi cách thanh toán.

## Command

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

Bạn đổi thời điểm / nơi thực thi.


# 2. Phân biệt Strategy với Template Method

## Strategy

Dùng khi cần thay thế toàn bộ hành vi hoặc thuật toán tại runtime.

> Strategy = "thay cả quy trình"

## Template Method

Dùng khi workflow tổng thể đã cố định, nhưng cho phép các implementation tùy chỉnh một vài bước bên trong workflow đó.

> Template Method = "giữ quy trình, thay một vài bước"

# 3. Phân biệt Strategy với State Pattern

State Pattern

Sử dụng khi một object có nhiều trạng thái và hành vi thay đổi theo từng trạng thái.

Thay vì dùng nhiều if/else hoặc switch theo state,
đóng gói hành vi của mỗi trạng thái vào một class riêng.

Strategy = thay thế thuật toán.

State = thay đổi hành vi dựa trên trạng thái hiện tại.

Strategy: client chọn implementation.

State: state hiện tại quyết định hành vi và có thể tự chuyển sang state khác.

Ví dụ:
Pending -> Paid -> Shipped -> Delivered

Strategy = "Tôi chọn cách làm"

State = "Trạng thái hiện tại quyết định cách làm"