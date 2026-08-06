// Package faultx là cơ chế bơm lỗi có chủ đích của lab, và đồng thời là bài
// học về OpenTelemetry Baggage.
//
// Ý tưởng: client gửi header "X-Fail-Mode: slow_db" tới gateway. Gateway nhét
// giá trị đó vào Baggage. Từ đó trở đi, baggage tự động đi theo request qua
// gRPC (metadata) rồi qua RabbitMQ (AMQP header) mà KHÔNG service nào phải
// truyền tay tham số fail-mode. Service ở tận cùng chỉ cần hỏi faultx.Is(ctx,...)
// là biết mình có phải giả vờ hỏng hay không.
//
// Khác biệt cốt lõi cần nắm:
//
//	traceparent — dữ liệu của hệ thống trace, nối span cha với span con.
//	baggage     — dữ liệu của ỨNG DỤNG, đi kèm cùng đường với traceparent.
//
// Cảnh báo thực tế: baggage đi qua mọi biên service nên đừng bao giờ bỏ dữ
// liệu nhạy cảm (token, PII) vào đó.
package faultx

import (
	"context"

	"go.opentelemetry.io/otel/baggage"
)

// Key là tên thành viên baggage mang fail mode.
const Key = "lab.fail_mode"

// HeaderName là header HTTP mà gateway đọc để nạp fail mode vào baggage.
const HeaderName = "X-Fail-Mode"

// Các fail mode được hỗ trợ. Mỗi mode ứng với một script trong scripts/.
const (
	// OutOfStock ép inventory-svc từ chối giữ hàng bằng gRPC code
	// FAILED_PRECONDITION, dù kho vẫn còn hàng. Lỗi nghiệp vụ.
	OutOfStock = "out_of_stock"

	// SlowDB chèn pg_sleep vào truy vấn của inventory-svc. Lỗi hạ tầng:
	// request vẫn đúng, chỉ là chậm.
	SlowDB = "slow_db"

	// Panic làm inventory-svc panic giữa chừng và chết hẳn. Trace sẽ bị cụt.
	Panic = "panic"

	// AsyncFail làm notification-svc xử lý message thất bại. Request HTTP gốc
	// vẫn trả 200 — đây là điểm mấu chốt của bài học về lỗi async.
	AsyncFail = "async_fail"
)

// Inject đặt fail mode vào baggage của context. Chỉ gateway gọi hàm này.
// mode rỗng thì trả lại context nguyên vẹn.
func Inject(ctx context.Context, mode string) context.Context {
	if mode == "" {
		return ctx
	}
	member, err := baggage.NewMember(Key, mode)
	if err != nil {
		// Giá trị không hợp lệ (ký tự lạ) thì bỏ qua, không làm hỏng request.
		return ctx
	}
	bag, err := baggage.New(member)
	if err != nil {
		return ctx
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

// Mode đọc fail mode hiện tại ra khỏi baggage. Trả về "" nếu không có.
func Mode(ctx context.Context) string {
	return baggage.FromContext(ctx).Member(Key).Value()
}

// Is cho biết context có đang mang đúng fail mode đó không.
func Is(ctx context.Context, mode string) bool {
	return Mode(ctx) == mode
}
