# Saga (Orchestration)

Demo Go nhỏ, không event bus, không CQRS, không database. Chỉ đúng một thứ: **một nghiệp vụ gồm nhiều bước ở nhiều service khác nhau, và cách dọn dẹp khi fail giữa chừng.**

```bash
go run .
go test ./...
```

## Vấn đề

Đặt hàng cần 3 việc, ở 3 service, 3 database khác nhau:

```
ReserveStock  →  ChargePayment  →  CreateShipment
 (Inventory)      (Payment)          (Shipping)
```

Trong một database, bạn bọc cả 3 trong `BEGIN ... COMMIT` và xong. Fail ở bước 3 thì bước 1, 2 tự động biến mất.

Nhưng ở đây **không có transaction nào bao được cả 3**. Payment service đã gọi Stripe và tiền đã rời tài khoản khách. Không có `ROLLBACK` nào lấy lại được. Bước 1 và 2 đã commit **thật**, ở nơi mà bạn không kiểm soát.

## Giải pháp

Với mỗi bước, định nghĩa thêm một hành động **triệt tiêu** nó. Fail ở bước N thì chạy ngược các hành động triệt tiêu của bước N-1, N-2, ...

```go
Step{
    Name:       "ChargePayment",
    Do:         pay.Charge,
    Compensate: pay.Refund,   // không phải rollback - là một giao dịch MỚI
}
```

Toàn bộ pattern nằm gọn trong `saga.go`, khoảng 40 dòng logic thật.

## Compensation không phải là rollback

Đây là điểm quan trọng nhất, và cũng là chỗ hay bị hiểu nhầm nhất.

| | Rollback (SQL) | Compensation (Saga) |
|---|---|---|
| Bản chất | Huỷ thay đổi chưa commit | Giao dịch **mới**, ngược chiều |
| Dấu vết để lại | Không có gì | Có — 2 dòng: charge + refund |
| Có thể fail không | Không | **Có** |
| Trạng thái trung gian | Không ai thấy | **Người khác thấy được** |

Ví dụ cụ thể: khách bị trừ 500k, 2 giây sau được hoàn 500k. Sao kê ngân hàng của họ có **2 dòng**, không phải 0 dòng. Và trong 2 giây đó, tiền thật sự không nằm trong tài khoản họ. Với rollback thì không có gì xảy ra cả.

Hệ quả thực tế: nếu nghiệp vụ của bạn không chịu được trạng thái trung gian bị nhìn thấy, saga không phải câu trả lời.

## Thứ tự bù trừ là ngược (LIFO)

Bước chạy `Reserve → Charge`, thì bù trừ chạy `Refund → Release`. Test `TestSaga_CompensationRunsInReverseOrder` khẳng định điều này.

Vì sao ngược? Bước sau có thể phụ thuộc bước trước. Nếu trả hàng về kho trước rồi mới hoàn tiền, sẽ có một khoảnh khắc mà hàng đã bán được cho người khác trong khi giao dịch của khách này vẫn còn treo. Cởi đúng thứ tự đã buộc thì không có khoảng hở đó.

## Bù trừ cũng có thể fail — và đây là kịch bản đáng sợ nhất

Chạy demo và nhìn Scenario 5:

```
> ReserveStock     ok
> ChargePayment    ok
x CreateShipment   failed: no courier available
! ChargePayment    COMPENSATION FAILED: refund service unavailable
< ReserveStock     compensated
```

Kết quả cuối:

```
order-refund-broken  stock_reserved=false  charged=true  shipment=false
```

**Khách đã trả tiền cho một đơn hàng sẽ không bao giờ được giao.** Hàng đã trả về kho. Không có retry nào trong process sửa được — refund service đang chết.

Đây không phải bug của demo, đây là **giới hạn thật của saga pattern**. Hệ thống thật xử lý bằng cách: đẩy compensation thất bại vào dead-letter queue, retry với backoff, và **báo động cho người thật** nếu vẫn không xong. Test `TestSaga_FailedCompensationLeavesTheSystemInconsistent` khẳng định chính xác trạng thái xấu này thay vì giả vờ nó không tồn tại.

Nếu bị hỏi "saga đảm bảo consistency đúng không?" — câu trả lời chính xác là: saga đảm bảo **eventual consistency trong điều kiện compensation thành công**. Nó không cho bạn tính atomic. Không có gì cho bạn tính atomic xuyên nhiều service.

## Orchestration vs Choreography

Demo này dùng **orchestration**: `Saga` là nơi duy nhất biết thứ tự các bước.

| | Orchestration (ở đây) | Choreography |
|---|---|---|
| Ai biết luồng | 1 orchestrator | Không ai — mỗi service tự nghe event |
| Đọc luồng ở đâu | Một chỗ, `buildSaga()` | Phải đọc hết mọi service rồi tự ghép |
| Coupling | Orchestrator biết mọi service | Các service chỉ biết event |
| Debug | Dễ — có một chỗ để log | Khó — luồng nằm rải rác |
| Hợp khi nào | Luồng phức tạp, nhiều nhánh | Ít bước, muốn service độc lập tối đa |

Ở quy mô 3 bước có compensation, orchestration gần như luôn thắng: bạn mở `buildSaga()` là thấy **toàn bộ** nghiệp vụ. Với choreography, câu hỏi "chuyện gì xảy ra khi shipping fail?" đòi bạn đọc code của cả 3 service.

Xem `../order-saga-demo` để thấy bản saga chạy trên event bus — vẫn là orchestration, nhưng các bước nối nhau qua event thay vì gọi tuần tự.

## Những gì demo này cố tình **không** làm

- **Không persist trạng thái saga.** Process chết giữa chừng là mất luồng, không ai bù trừ. Production phải ghi từng bước xuống DB (saga log) để khôi phục được sau restart.
- **Không idempotency.** Retry `Charge` hai lần sẽ trừ tiền hai lần. Thực tế mỗi bước cần idempotency key. Xem `../idempotency`.
- **Không timeout / retry.** Bước treo vĩnh viễn thì saga treo theo.
- **Không chạy song song.** Các bước độc lập nhau vốn có thể chạy đồng thời; ở đây tuần tự cho dễ đọc.

## Liên quan

- `../circuitbreaker-demo` — ngừng gọi service đang chết
- `../cqrs-demo` — tách đường ghi và đường đọc
- `../order-saga-demo` — cả 3 pattern chạy chung trên một luồng đặt hàng
