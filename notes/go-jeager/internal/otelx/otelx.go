// Package otelx tập trung toàn bộ phần khởi tạo OpenTelemetry cho cả 4 service.
//
// Mọi service chỉ cần gọi otelx.Init(ctx, "tên-service") ở đầu main() và defer
// hàm shutdown trả về. Ba thứ quan trọng được cấu hình ở đây:
//
//  1. Exporter: đẩy span sang Jaeger qua OTLP/gRPC (cổng 4317).
//  2. Resource: gắn service.name — chính là tên bạn thấy ở dropdown Service
//     trong Jaeger UI. Sai chỗ này là trace hiện lên dưới tên "unknown_service".
//  3. Propagator: TraceContext (chuẩn W3C traceparent) + Baggage. Đây là thứ
//     nối span của service này với span của service kia. Nếu chỉ đăng ký
//     TraceContext mà quên Baggage thì fault-mode trong lab sẽ không chảy qua
//     được biên service.
package otelx

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Init dựng TracerProvider toàn cục và trả về hàm shutdown.
//
// Hàm shutdown PHẢI được gọi trước khi process thoát, nếu không những span còn
// nằm trong BatchSpanProcessor sẽ mất trắng — một lý do rất phổ biến khiến
// "chạy xong mà Jaeger không thấy trace nào".
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // lab nội bộ, không TLS
	)
	if err != nil {
		return nil, fmt.Errorf("tạo OTLP exporter: %w", err)
	}

	// Cái bẫy ở đây: resource.Merge TỪ CHỐI hợp nhất hai resource có schema URL
	// khác nhau và trả về lỗi "conflicting Schema URL". resource.Default() gắn
	// schema URL theo phiên bản semconv mà SDK dùng nội bộ, thường không trùng
	// với phiên bản semconv ta import ở trên.
	//
	// Dùng NewSchemaless để phần attribute của ta không mang schema URL, khi đó
	// Merge lấy schema URL của Default và mọi thứ hợp nhất trót lọt.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion("1.0.0"),
		semconv.DeploymentEnvironment("lab"),
		attribute.String("lab.owner", "jaeger-demo"),
	))
	if err != nil {
		return nil, fmt.Errorf("tạo resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			// Rút ngắn so với mặc định 5s để trace hiện lên Jaeger gần như tức thì.
			// Production thì nên để mặc định cho đỡ tốn network.
			sdktrace.WithBatchTimeout(time.Second),
		),
		sdktrace.WithResource(res),
		// Lab thì lấy 100% trace. Production đổi thành:
		//   sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.01)))
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// Composite propagator: vừa truyền traceparent, vừa truyền baggage.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("otel đã sẵn sàng", "service", serviceName, "otlp_endpoint", endpoint)

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}, nil
}

// Tracer trả về tracer đặt tên theo service, dùng để tạo span thủ công.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// TraceIDFromContext lấy trace ID dạng hex để trả về client qua header
// X-Trace-Id. Nhờ nó mà script test in ra được link Jaeger mở thẳng đúng trace.
// Trả về "" nếu context chưa có span nào đang ghi nhận.
func TraceIDFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
