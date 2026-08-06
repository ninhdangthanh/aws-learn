// Package amqpx bọc RabbitMQ kèm instrumentation OpenTelemetry viết tay.
//
// Đọc file carrier.go trước để hiểu vì sao phải viết tay.
package amqpx

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tên exchange / queue / routing key dùng chung giữa order-svc và notification-svc.
const (
	ExchangeName = "lab.orders"
	QueueName    = "lab.order.created"
	RoutingKey   = "order.created"
)

// Conn gói connection và channel AMQP.
type Conn struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tracer  trace.Tracer
}

// Dial kết nối RabbitMQ và khai báo sẵn topology (exchange + queue + binding).
// Có retry vì lúc docker compose khởi động, RabbitMQ thường chậm hơn service.
func Dial(url string, tracerName string) (*Conn, error) {
	var conn *amqp.Connection
	var err error

	for attempt := 1; attempt <= 15; attempt++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		slog.Warn("chưa kết nối được RabbitMQ, thử lại", "lần", attempt, "lỗi", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("kết nối RabbitMQ sau 15 lần thử: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("mở channel: %w", err)
	}

	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("khai báo exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("khai báo queue: %w", err)
	}
	if err := ch.QueueBind(QueueName, RoutingKey, ExchangeName, false, nil); err != nil {
		return nil, fmt.Errorf("bind queue: %w", err)
	}

	slog.Info("RabbitMQ sẵn sàng", "exchange", ExchangeName, "queue", QueueName)
	return &Conn{conn: conn, channel: ch, tracer: otel.Tracer(tracerName)}, nil
}

// Close đóng channel và connection.
func (c *Conn) Close() error {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	return c.conn.Close()
}

// Publish gửi message kèm span PRODUCER và inject trace context vào header.
//
// Ba việc diễn ra theo đúng thứ tự, và thứ tự này quan trọng:
//  1. Mở span mới (SpanKindProducer) — span này thành cha của span consumer.
//  2. Inject context CỦA SPAN VỪA MỞ vào header. Inject trước khi mở span thì
//     consumer sẽ nối nhầm vào span cha, làm cây trace sai lệch.
//  3. Publish.
func (c *Conn) Publish(ctx context.Context, body []byte) error {
	ctx, span := c.tracer.Start(ctx,
		// Quy ước đặt tên span messaging của OTel: "<destination> <operation>".
		// Jaeger UI hiển thị đúng chuỗi này, nên đặt tên nhất quán rất đáng giá.
		fmt.Sprintf("%s publish", RoutingKey),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemRabbitmq,
			semconv.MessagingDestinationName(ExchangeName),
			semconv.MessagingOperationTypePublish,
			attribute.String("messaging.rabbitmq.routing_key", RoutingKey),
			attribute.Int("messaging.message.body.size", len(body)),
		),
	)
	defer span.End()

	headers := amqp.Table{}
	// Đây là dòng nối trace HTTP/gRPC ở trên với trace của consumer ở dưới.
	otel.GetTextMapPropagator().Inject(ctx, HeaderCarrier(headers))

	err := c.channel.PublishWithContext(ctx,
		ExchangeName, RoutingKey,
		false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			Headers:      headers,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish thất bại")
		return fmt.Errorf("publish: %w", err)
	}

	// Ghi lại header đã inject để bạn tự mắt nhìn thấy traceparent trong Jaeger.
	span.SetAttributes(attribute.String("lab.injected_traceparent", HeaderCarrier(headers).Get("traceparent")))
	return nil
}

// Handler xử lý một message. Trả về error thì message bị nack.
type Handler func(ctx context.Context, body []byte) error

// Consume chạy vòng lặp tiêu thụ message, mỗi message một span CONSUMER được
// nối vào trace gốc nhờ Extract từ header.
func (c *Conn) Consume(ctx context.Context, handler Handler) error {
	// Prefetch 1: xử lý tuần tự cho dễ đọc waterfall trong Jaeger.
	if err := c.channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("đặt QoS: %w", err)
	}

	deliveries, err := c.channel.Consume(QueueName, "notification-svc", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("đăng ký consume: %w", err)
	}

	slog.Info("bắt đầu tiêu thụ message", "queue", QueueName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("channel deliveries đã đóng")
			}
			c.handleDelivery(ctx, d, handler)
		}
	}
}

func (c *Conn) handleDelivery(ctx context.Context, d amqp.Delivery, handler Handler) {
	// Extract: dựng lại context của trace gốc từ header traceparent.
	// Nếu không có header, hàm này trả về context rỗng và span dưới đây sẽ
	// khởi tạo một trace hoàn toàn mới — chính là hiện tượng "trace đứt đôi".
	msgCtx := otel.GetTextMapPropagator().Extract(ctx, HeaderCarrier(d.Headers))

	msgCtx, span := c.tracer.Start(msgCtx,
		fmt.Sprintf("%s process", RoutingKey),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemRabbitmq,
			semconv.MessagingDestinationName(ExchangeName),
			// semconv 1.26 gọi thao tác phía consumer là "deliver"
			// (các bản trước đặt tên "process").
			semconv.MessagingOperationTypeDeliver,
			attribute.String("messaging.rabbitmq.routing_key", d.RoutingKey),
			attribute.String("lab.received_traceparent", HeaderCarrier(d.Headers).Get("traceparent")),
		),
	)
	defer span.End()

	if err := handler(msgCtx, d.Body); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Error("xử lý message thất bại", "lỗi", err, "trace_id", span.SpanContext().TraceID())

		// requeue=false: đẩy thẳng đi, không lặp vô hạn trong lab.
		// Production thì nên có dead-letter exchange.
		_ = d.Nack(false, false)
		return
	}

	span.SetStatus(codes.Ok, "")
	_ = d.Ack(false)
}
