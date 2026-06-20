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