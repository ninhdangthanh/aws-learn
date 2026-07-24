# CQRS (Command Query Responsibility Segregation)

Demo Go nhỏ, không saga, không circuit breaker, không database. Chỉ đúng một thứ: **đường ghi và đường đọc là hai model riêng biệt, nối nhau bằng event.**

```bash
go run .
go test ./...
```

## Đọc theo thứ tự này

Mỗi file là một phía của pattern:

| File | Vai trò |
|---|---|
| `write.go` | Đường ghi: `Order`, `WriteStore`, `CommandBus`. Nguồn sự thật. |
| `read.go` | Đường đọc: `OrderSummary`, `ReadStore`, `Project`, `QueryBus`. Dẫn xuất. |
| `events.go` | Cây cầu giữa hai bên. |
| `main.go` | 4 kịch bản chạy được. |

## Ý tưởng

```
Command  →  WriteStore  →  Event  →  ReadStore  →  Query
          (nguồn sự thật)         (dẫn xuất, dựng lại được)
```

Ba luật, và đó là toàn bộ pattern:

1. **Command** chỉ chạm write model.
2. **Query** chỉ chạm read model.
3. **Event là cây cầu duy nhất.**

Luật số 2 được ép bằng **kiểu dữ liệu**, không phải bằng quy ước: `QueryBus` chỉ giữ một `*ReadStore` — nó không có đường nào chạm tới write model, kể cả khi ai đó muốn.

## Hai model, hai hình dạng khác nhau

Đây là điểm hay bị bỏ sót. Nếu read model chỉ là bản sao y hệt write model, bạn chưa thu được gì — chỉ tốn thêm một chỗ lưu.

| | `Order` (write) | `OrderSummary` (read) |
|---|---|---|
| Lưu gì | Từng dòng hàng riêng lẻ | `ItemCount` + `Total` cộng sẵn |
| Tổng tiền | Tính lúc cần | Cộng dồn mỗi khi có event |
| Hình dạng theo | Luật nghiệp vụ phải kiểm tra | Màn hình phải hiển thị |

Scenario 1 in ra cùng một order dưới hai dạng:

```
write : paid=false lines=[{keyboard 120} {mouse 40}]
read  : status=open  items=2 total=160
```

Nguyên tắc rút ra: **write model có hình dạng theo bất biến (invariant) nó phải bảo vệ; read model có hình dạng theo câu hỏi nó phải trả lời.**

## Bằng chứng hai bên thực sự tách rời

Nói "tách rời" thì dễ. Scenario 4 chứng minh — ghi thẳng vào `WriteStore`, không qua `CommandBus`, nên **không có event nào được bắn**:

```
write : lines=[{keyboard 120} {mouse 40} {smuggled 500}]
read  : items=2 total=160
```

Hai bên bất đồng. Nếu chúng dùng chung một chỗ lưu, read model đã phải thấy 620. Nó thấy 160.

Nhìn có vẻ như bug. Nó không phải bug — nó là **định nghĩa** của pattern. Read model chỉ biết những gì event kể cho nó. Test `TestReadModel_OnlyLearnsAboutChangesThroughEvents` khẳng định chính xác điều này.

## Cái giá: eventual consistency

Ở demo này bus chạy đồng bộ trong cùng process, nên read model cập nhật tức thì. **Thực tế không như vậy** — event đi qua Kafka/RabbitMQ, projection chạy ở service khác, trễ từ vài ms tới vài giây:

```
POST /orders   -> 201 Created
GET  /orders/1 -> 404 Not Found     // projection chưa chạy kịp
```

Người dùng vừa tạo xong không thấy thứ mình vừa tạo. Cách xử lý thường gặp: trả ID ngay từ command để client khỏi query lại; hoặc cho user vừa ghi đọc tạm từ write model (read-your-own-writes); hoặc chấp nhận, vì với dashboard thì trễ 2 giây không ai quan tâm.

Điểm mấu chốt hay bị hỏi: **validate luôn đọc write model, không bao giờ đọc read model.** Trong `write.go`, `openOrder()` kiểm tra order đã thanh toán chưa bằng cách đọc `WriteStore`. Nếu nó đọc read model, một projection bị chậm có thể cho lọt một lệnh ghi sai — và đó là mất tính đúng đắn thật, không phải chỉ là hiển thị cũ.

## Vì sao đáng đánh đổi

Read model được xây từ event, nên **thêm một cách nhìn mới không đụng gì tới đường ghi**. Muốn có bảng "tổng chi tiêu theo khách hàng"? Thêm một map và một `bus.Subscribe(OrderPaid, ...)` trong `read.go`. `write.go` không đổi một dòng.

Câu hỏi kiểu "khách này đã tiêu bao nhiêu?" nếu hỏi write model thì phải quét toàn bộ order. Ở read model nó là một lần tra map, vì đã được cộng dồn sẵn từng bước. Đó là lý do người ta chịu đựng eventual consistency.

## Khi nào **không** nên dùng

- CRUD đơn giản, đọc và ghi cùng hình dạng — thêm CQRS chỉ tăng code
- Đọc/ghi cân bằng, không có truy vấn nào tốn kém
- Team chưa sẵn sàng debug "vì sao read model lệch"

Cân nhắc khi: đọc nhiều hơn ghi rất nhiều, truy vấn đọc phức tạp, hoặc cần nhiều cách nhìn khác nhau trên cùng dữ liệu.

## CQRS không phải Event Sourcing

- **CQRS** (demo này): write model lưu **trạng thái hiện tại**. Event chỉ để thông báo, không được lưu — xoá read model đi là không dựng lại được.
- **Event Sourcing**: event **chính là** nguồn sự thật, lưu vĩnh viễn. Trạng thái được tính bằng replay. Xoá projection lúc nào cũng dựng lại được.

Hai cái hợp nhau nhưng độc lập. Chú ý `WriteStore` ở đây lưu `Order`, không lưu danh sách event.

## Những gì demo này cố tình **không** làm

- **Event không được lưu**, phát rồi thôi. Không replay, không dựng lại projection được.
- **Bus đồng bộ, in-process, không lock.** Đủ cho demo một luồng; thật thì phải qua message broker và projection phải an toàn khi chạy song song.
- **Không xử lý event trùng lặp / sai thứ tự.** Broker thật giao at-least-once, projection phải idempotent. Xem `../idempotency`.
- **Chỉ một projection.** Xem mục "Vì sao đáng đánh đổi" ở trên để biết thêm cái thứ hai trông thế nào.

## Liên quan

- `../circuitbreaker-demo` — ngừng gọi service đang chết
- `../saga-demo` — nhiều bước, fail giữa chừng thì bù trừ
- `../order-saga-demo` — cả 3 pattern chạy chung trên một luồng đặt hàng
- `../event-driven-architecture.md` — ghi chú rộng hơn về kiến trúc event-driven
