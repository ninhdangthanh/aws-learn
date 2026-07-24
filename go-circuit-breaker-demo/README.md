# Circuit Breaker với `sony/gobreaker`

Demo tối giản: **order service** gọi sang **payment gateway**. Gateway sập rồi phục hồi, breaker đi qua đủ 3 trạng thái.

```bash
go run .     # chạy 6 scenario
go test ./...
```

## Vấn đề circuit breaker giải quyết

Payment gateway sập. Không có breaker:

- Mỗi đơn hàng vẫn gọi sang gateway → chờ timeout (thường 5–30s) rồi mới lỗi.
- Goroutine + connection của order service bị giữ suốt thời gian chờ đó.
- Traffic dồn lại, order service cạn tài nguyên và **sập theo** — dù bản thân nó không có lỗi gì.
- Gateway đang cố phục hồi thì bị chính ta dội request vào, không ngóc đầu lên được.

Đây là **cascading failure**: một service chết kéo cả hệ thống chết. Breaker cắt vòng lặp đó bằng cách *ngừng gọi* dependency đang chết.

## Ba trạng thái

| Trạng thái | Hành vi | Chuyển tiếp |
|---|---|---|
| **Closed** | Cho request đi qua, đếm lỗi | Đủ ngưỡng lỗi (`ReadyToTrip`) → Open |
| **Open** | **Fail-fast** — trả lỗi ngay, không chạm dependency | Hết `Timeout` → Half-open |
| **Half-open** | Cho tối đa `MaxRequests` request thăm dò | Thăm dò OK → Closed; lỗi → Open lại |

Half-open là phần quan trọng nhất: nó thử lại bằng **một** request thay vì cả luồng traffic. Nếu dependency vẫn chết, ta chỉ tốn đúng 1 request.

## Cấu hình trong `internal/order/service.go`

```go
gobreaker.NewCircuitBreaker[string](gobreaker.Settings{
    Name:        "payment-gateway",
    MaxRequests: 1,                // half-open: đúng 1 request thăm dò
    Timeout:     2 * time.Second,  // Open bao lâu thì thử lại
    ReadyToTrip: func(c gobreaker.Counts) bool {
        return c.ConsecutiveFailures >= 3
    },
    OnStateChange: func(name string, from, to gobreaker.State) { ... },
})
```

Generic `[string]` là kiểu trả về của hàm được bọc — ở đây là transaction ID (gobreaker v2 dùng generics, v1 thì trả `any`).

Gọi qua `Execute`:

```go
txnID, err := s.breaker.Execute(func() (string, error) {
    return s.gateway.Charge(orderID, amount)
})
```

## Phân biệt hai loại lỗi

`Execute` trả về lỗi trong hai tình huống khác hẳn nhau, và bạn cần tách chúng ra khi log/đo metric:

```go
errors.Is(err, gobreaker.ErrOpenState)       // breaker chặn, gateway KHÔNG bị gọi
errors.Is(err, gobreaker.ErrTooManyRequests) // half-open đã đủ request thăm dò
// còn lại → lỗi thật từ gateway
```

Trộn chung sẽ khiến dashboard hiển thị "gateway lỗi 10k lần" trong khi thực tế gateway chỉ bị gọi 3 lần.

## Fallback — phần hay bị bỏ quên

Breaker chỉ cho biết "đừng gọi nữa". Nó **không** trả lời "vậy giờ làm gì?" — đó là việc của bạn. Demo này đẩy đơn vào hàng đợi để retry:

```go
s.queued = append(s.queued, orderID)
return "", ErrPaymentUnavailable
```

Trong hệ thống thật, tuỳ nghiệp vụ mà chọn:

| Dependency | Fallback hợp lý |
|---|---|
| Payment | Nhận đơn, đưa vào queue, charge sau (đơn hàng vẫn được tạo) |
| Recommendation | Trả danh sách bán chạy tĩnh |
| Inventory | Dùng số tồn kho cache, chấp nhận oversell nhẹ |
| Đăng nhập | Không có fallback — báo lỗi thẳng còn hơn cho vào sai |

Nguyên tắc: fallback phải **rẻ và không phụ thuộc** vào chính cái đang chết.

## Output khi chạy

```
=== 2. Gateway sập — 3 lỗi liên tiếp làm breaker trip ===
  order-3: gateway lỗi thật: payment gateway unavailable
  order-4: gateway lỗi thật: payment gateway unavailable
  [breaker] payment-gateway: closed -> open
  order-5: gateway lỗi thật: payment gateway unavailable
  state=open | gateway calls=5 | queued=3

=== 3. Breaker Open — fail-fast, gateway không bị gọi thêm lần nào ===
  order-6: fail-fast, gateway KHÔNG bị gọi
  ...
  gateway calls: 5 -> 5 (đứng yên = fail-fast hoạt động)
```

`gateway calls` là con số cần nhìn: 4 đơn hàng đi qua mà gateway không bị chạm lần nào.

## Lưu ý khi lên production

- **Mỗi dependency một breaker.** Payment và inventory dùng chung một breaker thì inventory chết sẽ chặn luôn payment.
- **Ngưỡng nên theo tỉ lệ, không theo số tuyệt đối.** `ConsecutiveFailures >= 3` dễ hiểu nhưng trong demo thôi; production nên là "tỉ lệ lỗi > 50% trên tối thiểu 20 request" để tránh trip nhầm lúc traffic thấp:
  ```go
  ReadyToTrip: func(c gobreaker.Counts) bool {
      return c.Requests >= 20 && float64(c.TotalFailures)/float64(c.Requests) > 0.5
  }
  ```
- **Đừng đếm lỗi 4xx là failure.** Client gửi sai dữ liệu không có nghĩa gateway đang chết — lọc bằng `IsSuccessful`, và dùng `IsExcluded` để bỏ qua hẳn context cancellation.
- **Breaker là per-instance.** 10 pod = 10 breaker học độc lập. Cần chia sẻ trạng thái thì dùng `NewDistributedCircuitBreaker` với Redis.
- **Breaker không thay thế timeout.** Không có timeout ở HTTP client thì request treo mãi, breaker chẳng bao giờ thấy lỗi để mà trip. Timeout → retry (có backoff) → circuit breaker, theo thứ tự đó.
- **Gắn `OnStateChange` vào metric/alert.** Breaker mở là tín hiệu sự cố sớm nhất bạn có.

## Cấu trúc

```
main.go                        6 scenario minh hoạ vòng đời breaker
internal/payment/gateway.go    gateway giả, bật/tắt được, đếm số lần bị gọi
internal/order/service.go      order service + breaker + fallback
```

Xem thêm `../order-saga-demo/` — breaker tự viết tay đặt trong luồng saga/CQRS.
