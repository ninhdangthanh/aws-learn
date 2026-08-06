// notification-svc: worker tiêu thụ order.created rồi gọi HTTP ngược về gateway.
//
// Service này không có server nào cả — nó chỉ ngồi nghe RabbitMQ. Đây là nhánh
// ASYNC của lab, và là phần khó đọc nhất trong Jaeger:
//
//   - Span của nó nằm CÙNG trace với request HTTP gốc (nhờ traceparent được
//     nhét vào AMQP header), nhưng bắt đầu sau khi client đã nhận response.
//   - Nếu bạn mở Jaeger ngay lập tức, trace trông như đã xong. Chờ vài giây rồi
//     tải lại trang thì span của notification-svc mới hiện ra.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ninhdang/go-jeager/internal/amqpx"
	"github.com/ninhdang/go-jeager/internal/config"
	"github.com/ninhdang/go-jeager/internal/faultx"
	"github.com/ninhdang/go-jeager/internal/httpx"
	"github.com/ninhdang/go-jeager/internal/otelx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "notification-svc"

type orderCreatedEvent struct {
	OrderID  string `json:"order_id"`
	SKU      string `json:"sku"`
	Qty      int32  `json:"qty"`
	Customer string `json:"customer"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownOtel, err := otelx.Init(ctx, serviceName)
	if err != nil {
		slog.Error("khởi tạo OTel thất bại", "lỗi", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownOtel(context.Background()); err != nil {
			slog.Error("flush span thất bại", "lỗi", err)
		}
	}()

	mq, err := amqpx.Dial(config.Env("RABBITMQ_URL", "amqp://lab:lab@localhost:5672/"), serviceName)
	if err != nil {
		slog.Error("kết nối RabbitMQ thất bại", "lỗi", err)
		os.Exit(1)
	}
	defer func() { _ = mq.Close() }()

	w := &worker{
		client:      httpx.TracedClient(),
		gatewayURL:  config.Env("GATEWAY_URL", "http://localhost:8080"),
		tracer:      otelx.Tracer(serviceName),
		renderDelay: 150 * time.Millisecond,
	}

	slog.Info("notification-svc đang chạy", "gateway", w.gatewayURL)
	if err := mq.Consume(ctx, w.handle); err != nil && !isShutdown(ctx, err) {
		slog.Error("vòng lặp consume dừng", "lỗi", err)
		os.Exit(1)
	}
}

type worker struct {
	client      *http.Client
	gatewayURL  string
	tracer      trace.Tracer
	renderDelay time.Duration
}

// handle xử lý một sự kiện order.created.
//
// ctx truyền vào đây đã được amqpx.Consume nối vào trace gốc, nên MỌI span tạo
// từ nó — kể cả span HTTP client bên dưới — đều thuộc cùng một trace với request
// curl ban đầu.
func (w *worker) handle(ctx context.Context, body []byte) error {
	span := trace.SpanFromContext(ctx)

	var event orderCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("giải mã sự kiện: %w", err)
	}
	span.SetAttributes(
		attribute.String("lab.order_id", event.OrderID),
		attribute.String("lab.sku", event.SKU),
		attribute.String("lab.customer", event.Customer),
	)

	if mode := faultx.Mode(ctx); mode != "" {
		// Bằng chứng baggage đã đi qua được RabbitMQ, không chỉ qua HTTP/gRPC.
		span.SetAttributes(attribute.String("lab.fail_mode", mode))
		slog.Info("nhận fail mode qua baggage (đường AMQP)", "mode", mode, "order_id", event.OrderID)
	}

	if err := w.render(ctx, event); err != nil {
		return err
	}
	return w.notifyGateway(ctx, event)
}

// render mô phỏng bước dựng nội dung email, và là chỗ đặt fault async.
func (w *worker) render(ctx context.Context, event orderCreatedEvent) error {
	ctx, span := w.tracer.Start(ctx, "notification.render_email")
	defer span.End()
	span.SetAttributes(attribute.String("lab.template", "order_confirmation"))

	select {
	case <-time.After(w.renderDelay):
	case <-ctx.Done():
		return ctx.Err()
	}

	// ── FAULT 4: hỏng ở nhánh async ───────────────────────────────────────
	// Điểm mấu chốt: tới đây thì client đã nhận HTTP 201 từ lâu rồi. Request
	// gốc "thành công", nhưng công việc phía sau thì hỏng. Đúng loại sự cố mà
	// nếu không có distributed tracing thì gần như không thể phát hiện.
	if faultx.Is(ctx, faultx.AsyncFail) {
		err := fmt.Errorf("dựng email thất bại cho đơn %s (fail_mode=async_fail)", event.OrderID)
		span.RecordError(err)
		span.SetStatus(codes.Error, "render thất bại")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// notifyGateway gọi HTTP ngược về gateway. otelhttp.NewTransport lo phần span
// CLIENT và inject traceparent, nên nhánh async nối liền mạch về lại service
// khởi đầu trace.
func (w *worker) notifyGateway(ctx context.Context, event orderCreatedEvent) error {
	payload, err := json.Marshal(map[string]any{
		"order_id": event.OrderID,
		"sku":      event.SKU,
		"channel":  "email",
	})
	if err != nil {
		return fmt.Errorf("mã hoá payload callback: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost,
		w.gatewayURL+"/internal/callback", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("tạo request callback: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("gọi callback: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("callback trả về %s", resp.Status)
	}

	slog.Info("đã thông báo cho gateway", "order_id", event.OrderID,
		"trace_id", otelx.TraceIDFromContext(ctx))
	return nil
}

func isShutdown(ctx context.Context, err error) bool {
	return ctx.Err() != nil && err == ctx.Err()
}
