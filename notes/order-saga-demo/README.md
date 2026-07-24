# Order Saga Demo — CQRS + Saga + Circuit Breaker

> **Muốn học từng pattern riêng lẻ trước?** Mỗi demo dưới đây dạy đúng một khái niệm, không cần biết hai cái còn lại:
>
> - [`../circuitbreaker-demo`](../circuitbreaker-demo) — ngừng gọi service đang chết. Không domain, không event.
> - [`../saga-demo`](../saga-demo) — 3 bước, fail giữa chừng thì bù trừ ngược. Không event bus, không CQRS.
> - [`../cqrs-demo`](../cqrs-demo) — tách đường ghi/đọc, nhiều projection từ một event. Không saga, không breaker.
>
> Đọc xong 3 cái đó rồi quay lại đây để thấy chúng **phối hợp** với nhau thế nào.

Demo Go nhỏ, chạy trong 1 process, toàn bộ dữ liệu in-memory — minh hoạ 3 pattern quen thuộc trong backend cùng lúc trên một luồng nghiệp vụ đặt hàng đơn giản (tạo order → giữ hàng trong kho → thu tiền → xác nhận).

Không có HTTP, không có DB thật, không docker-compose. Mục đích là đọc code + chạy `go run .` để thấy rõ luồng, không phải một service production.

## 1. CQRS (Command Query Responsibility Segregation)

Ý tưởng: **tách đường ghi (write) và đường đọc (read) thành hai model riêng**, không dùng chung một chỗ lưu trữ và chỉ nối với nhau qua event.

Trong code:

- `internal/cqrs/commands.go` — `CommandBus.CreateOrder(...)`: nhận lệnh, ghi `Order` vào `internal/writemodel` (map + mutex), rồi publish event `OrderCreated`. Đây là **nguồn sự thật (source of truth)**.
- `internal/cqrs/queries.go` — `QueryBus.GetOrder(...)`: chỉ đọc từ `internal/readmodel`, **không bao giờ** đọc `writemodel`.
- `internal/readmodel/store.go` — hàm `Project(bus, store)` subscribe vào mọi event và tự cập nhật `OrderView` (status + lịch sử các bước). Đây là "read model" được build lại từ event, không phải copy trực tiếp từ write model.

Vì sao tách: đường ghi cần validate/transaction chặt chẽ, đường đọc cần trả nhanh, dễ tối ưu hình dạng dữ liệu (denormalize) mà không ảnh hưởng đường ghi. Cái giá phải trả là **eventual consistency** — có test chứng minh điều này: `internal/cqrs/cqrs_test.go` (`TestQuery_DoesNotReflectDirectWriteStoreMutation`) sửa thẳng `writemodel` mà không bắn event thì `readmodel` **không đổi**, chứng minh hai bên thực sự tách biệt chứ không phải chung một map.

## 2. Saga (kiểu Orchestration)

Ý tưởng: một nghiệp vụ trải qua **nhiều bước độc lập** (ở đây là gọi 2 "service" giả lập Inventory và Payment), không có transaction chung xuyên suốt như DB. Nếu một bước giữa chừng fail, phải **chạy bù trừ (compensation)** cho các bước đã lỡ thành công trước đó, thay vì rollback như SQL transaction.

Trong code, `internal/saga/order_saga.go` (`OrderSaga`) là orchestrator — nó là nơi **duy nhất** biết toàn bộ thứ tự các bước:

```
OrderCreated → ReserveInventory → ChargePayment → ConfirmOrder
```

- `onOrderCreated`: gọi `InventoryService.Reserve`. Fail → bắn `InventoryReserveFailed`, đánh dấu order `Failed` (chưa có gì để bù trừ vì bước này còn chưa thành công).
- `onInventoryReserved`: gọi `PaymentService.Charge` (bọc qua circuit breaker, xem mục 3). Fail → bắn `PaymentFailed`.
- `onPaymentFailed`: đây là bước bù trừ — gọi `InventoryService.Release` để **trả lại hàng đã giữ ở bước trước**, rồi mới đánh dấu order `Failed`.
- `onPaymentCharged`: mọi bước đều ok → đánh dấu `Confirmed`.

Khác với **choreography saga** (mỗi service tự lắng nghe event của nhau, không ai biết toàn cục), ở đây chọn **orchestration** vì luồng chỉ có 2-3 bước, dễ đọc hơn khi có 1 nơi duy nhất quyết định "bước tiếp theo là gì" và "bù trừ cái gì khi fail ở đâu".

Test minh hoạ rõ nhất cho phần bù trừ: `internal/saga/order_saga_test.go` — `TestOrderSaga_PaymentFailureCompensatesInventoryReservation` và `TestOrderSaga_OpenCircuitFailsFastAndCompensates` (bù trừ ngay cả khi lý do fail là circuit breaker đang mở, không phải payment tự chối).

## 3. Circuit Breaker

Ý tưởng: nếu một service phụ thuộc (ở đây là `PaymentService`) liên tục lỗi, **ngừng gọi nó một thời gian** thay vì tiếp tục gọi và chờ timeout mỗi lần — tránh dồn tải lên service đang gặp sự cố và tránh caller bị treo theo.

Cài đặt tay (không dùng thư viện ngoài) tại `internal/circuitbreaker/breaker.go`, có 3 trạng thái:

| Trạng thái | Ý nghĩa | Chuyển sang trạng thái nào |
|---|---|---|
| **Closed** | Bình thường, gọi thẳng service | Đủ `failureThreshold` lần lỗi liên tiếp → **Open** |
| **Open** | Từ chối ngay (`ErrCircuitOpen`), **không gọi** service | Sau `resetTimeout` → **HalfOpen** |
| **HalfOpen** | Cho đúng 1 lệnh gọi thử | Thành công → **Closed**; thất bại → **Open** lại |

Điểm hay để ý trong code: `Execute()` nhả lock trước khi gọi `fn()` thật sự (không giữ lock trong lúc chờ I/O), rồi lock lại để cập nhật trạng thái dựa trên kết quả — nhờ vậy circuit breaker không block các goroutine khác trong lúc payment service đang xử lý chậm.

`main.go` dựng breaker với `failureThreshold=2, resetTimeout=200ms` rồi cho payment service fail 2 lần liên tiếp để breaker mở, gửi thêm 1 order để chứng minh **payment service không hề bị gọi** lúc breaker Open (đếm số lần gọi trước/sau, không đổi), rồi chờ qua `resetTimeout` để thấy breaker tự phục hồi.

## Chạy thử

```bash
go run .
```

In ra 4 kịch bản theo thứ tự: happy path → order confirmed; inventory hết hàng → có compensate + order failed; payment lỗi liên tiếp → breaker Closed → Open → fail-fast (không gọi payment nữa); chờ hết `resetTimeout` → breaker HalfOpen → Closed, order sau đó thành công. Cuối cùng in toàn bộ read model (`GetOrderQuery`) để thấy kết quả cuối của từng order.

## Chạy test

```bash
go test ./...
```

21 test trải trên các package `circuitbreaker`, `cqrs`, `events`, `readmodel`, `saga`, `services`, `writemodel` (`domain` chỉ có struct/const nên không có test riêng).
