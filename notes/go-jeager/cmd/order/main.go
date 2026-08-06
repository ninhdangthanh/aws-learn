// order-svc: gRPC server điều phối việc tạo đơn.
//
// Đây là service trung tâm, chạm vào cả ba loại I/O còn lại:
//   - Postgres  (ghi bảng orders)
//   - gRPC      (gọi inventory-svc)
//   - RabbitMQ  (publish sự kiện order.created)
//
// Trong Jaeger, span của service này là chỗ bạn thấy rõ nhất một request được
// "rẽ nhánh" ra nhiều hệ thống phụ thuộc như thế nào.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	inventoryv1 "github.com/ninhdang/go-jeager/gen/inventory/v1"
	orderv1 "github.com/ninhdang/go-jeager/gen/order/v1"
	"github.com/ninhdang/go-jeager/internal/amqpx"
	"github.com/ninhdang/go-jeager/internal/config"
	"github.com/ninhdang/go-jeager/internal/dbx"
	"github.com/ninhdang/go-jeager/internal/otelx"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const serviceName = "order-svc"

// OrderCreatedEvent là payload đẩy sang RabbitMQ.
type OrderCreatedEvent struct {
	OrderID   string    `json:"order_id"`
	SKU       string    `json:"sku"`
	Qty       int32     `json:"qty"`
	Customer  string    `json:"customer"`
	CreatedAt time.Time `json:"created_at"`
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

	pool, err := dbx.Connect(ctx, config.Env("DATABASE_URL", "postgres://lab:lab@localhost:5432/lab?sslmode=disable"))
	if err != nil {
		slog.Error("kết nối Postgres thất bại", "lỗi", err)
		os.Exit(1)
	}
	defer pool.Close()

	mq, err := amqpx.Dial(config.Env("RABBITMQ_URL", "amqp://lab:lab@localhost:5672/"), serviceName)
	if err != nil {
		slog.Error("kết nối RabbitMQ thất bại", "lỗi", err)
		os.Exit(1)
	}
	defer func() { _ = mq.Close() }()

	// Client gRPC tới inventory-svc. StatsHandler ở đây lo việc mở span CLIENT
	// và inject traceparent + baggage vào metadata của mỗi lời gọi.
	invAddr := config.Env("INVENTORY_ADDR", "localhost:9092")
	invConn, err := grpc.NewClient(invAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("tạo client inventory thất bại", "addr", invAddr, "lỗi", err)
		os.Exit(1)
	}
	defer func() { _ = invConn.Close() }()

	addr := config.Env("GRPC_ADDR", ":9091")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("lắng nghe thất bại", "addr", addr, "lỗi", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	orderv1.RegisterOrderServiceServer(srv, &server{
		pool:      pool,
		mq:        mq,
		inventory: inventoryv1.NewInventoryServiceClient(invConn),
	})
	reflection.Register(srv)

	go func() {
		<-ctx.Done()
		slog.Info("nhận tín hiệu dừng, tắt gRPC server")
		srv.GracefulStop()
	}()

	slog.Info("order-svc đang chạy", "addr", addr, "inventory", invAddr)
	if err := srv.Serve(lis); err != nil {
		slog.Error("gRPC server dừng", "lỗi", err)
	}
}

type server struct {
	orderv1.UnimplementedOrderServiceServer
	pool      *pgxpool.Pool
	mq        *amqpx.Conn
	inventory inventoryv1.InventoryServiceClient
}

// CreateOrder chạy 5 bước, mỗi bước để lại dấu vết riêng trong trace:
//
//	1. INSERT đơn ở trạng thái PENDING        → span db
//	2. gọi inventory.Reserve                  → span gRPC client + server
//	3. cập nhật trạng thái CONFIRMED / FAILED → span db
//	4. publish order.created                  → span producer
//	5. trả kết quả
func (s *server) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	span := trace.SpanFromContext(ctx)
	orderID := uuid.NewString()
	span.SetAttributes(
		attribute.String("lab.order_id", orderID),
		attribute.String("lab.sku", req.GetSku()),
		attribute.Int("lab.qty", int(req.GetQty())),
		attribute.String("lab.customer", req.GetCustomer()),
	)

	// Bước 1 ────────────────────────────────────────────────────────────────
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO orders (id, sku, qty, customer, status) VALUES ($1, $2, $3, $4, 'PENDING')`,
		orderID, req.GetSku(), req.GetQty(), req.GetCustomer(),
	); err != nil {
		return nil, fail(span, grpccodes.Internal, "tạo đơn thất bại", err)
	}

	// Bước 2 ────────────────────────────────────────────────────────────────
	// Không cần truyền fail mode thủ công: nó nằm trong baggage của ctx và
	// otelgrpc tự đính vào metadata.
	reserved, err := s.inventory.Reserve(ctx, &inventoryv1.ReserveRequest{
		OrderId: orderID,
		Sku:     req.GetSku(),
		Qty:     req.GetQty(),
	})
	if err != nil {
		// Bước 3a: đánh dấu đơn hỏng. Dùng context.WithoutCancel để câu UPDATE
		// vẫn chạy được kể cả khi ctx gốc đã bị huỷ (client bỏ cuộc), nhưng vẫn
		// giữ nguyên trace context nên span này vẫn nằm trong cùng một trace.
		s.markFailed(context.WithoutCancel(ctx), orderID)

		// Chuyển tiếp nguyên vẹn gRPC code từ inventory-svc lên trên. Nhờ vậy
		// gateway phân biệt được 409 (hết hàng) với 503 (service chết).
		code := status.Code(err)
		return nil, fail(span, code, "giữ hàng thất bại", err)
	}

	// Bước 3b ───────────────────────────────────────────────────────────────
	if _, err := s.pool.Exec(ctx,
		`UPDATE orders SET status = 'CONFIRMED', updated_at = now() WHERE id = $1`,
		orderID,
	); err != nil {
		return nil, fail(span, grpccodes.Internal, "cập nhật trạng thái đơn thất bại", err)
	}

	// Bước 4 ────────────────────────────────────────────────────────────────
	// Publish lỗi thì KHÔNG làm hỏng đơn hàng: đơn đã CONFIRMED và tồn kho đã
	// trừ. Chỉ ghi lại sự cố lên span rồi đi tiếp. Trong Jaeger bạn sẽ thấy một
	// trace thành công nhưng có một span con màu đỏ — tình huống rất thật.
	event := OrderCreatedEvent{
		OrderID:   orderID,
		SKU:       req.GetSku(),
		Qty:       req.GetQty(),
		Customer:  req.GetCustomer(),
		CreatedAt: time.Now().UTC(),
	}
	payload, _ := json.Marshal(event)
	if err := s.mq.Publish(ctx, payload); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("lab.publish_failed", true))
		slog.Error("publish order.created thất bại", "lỗi", err, "order_id", orderID)
	}

	span.SetStatus(codes.Ok, "")
	return &orderv1.CreateOrderResponse{
		OrderId:        orderID,
		Status:         "CONFIRMED",
		RemainingStock: reserved.GetRemaining(),
	}, nil
}

// GetOrder đọc lại một đơn theo id.
func (s *server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("lab.order_id", req.GetOrderId()))

	// Kiểm tra định dạng trước khi chạm vào DB. Thiếu bước này thì một id sai
	// định dạng sẽ khiến Postgres báo lỗi 22P02, order-svc trả Internal, và
	// gateway dịch thành HTTP 500 — trong khi đây rõ ràng là lỗi phía client.
	//
	// Đây không chỉ là chuyện mã lỗi cho đẹp: otelhttp đánh dấu span lỗi từ 500
	// trở lên, nên nhầm 400 thành 500 sẽ làm nhiễu đúng bộ lọc error=true mà
	// bạn dùng để săn sự cố thật.
	if _, err := uuid.Parse(req.GetOrderId()); err != nil {
		return nil, fail(span, grpccodes.InvalidArgument, "order id không phải UUID hợp lệ", err)
	}

	var out orderv1.GetOrderResponse
	err := s.pool.QueryRow(ctx,
		`SELECT id::text, sku, qty, status, customer FROM orders WHERE id = $1`,
		req.GetOrderId(),
	).Scan(&out.OrderId, &out.Sku, &out.Qty, &out.Status, &out.Customer)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fail(span, grpccodes.NotFound, "không tìm thấy đơn", err)
	}
	if err != nil {
		return nil, fail(span, grpccodes.Internal, "đọc đơn thất bại", err)
	}
	return &out, nil
}

func (s *server) markFailed(ctx context.Context, orderID string) {
	if _, err := s.pool.Exec(ctx,
		`UPDATE orders SET status = 'FAILED', updated_at = now() WHERE id = $1`,
		orderID,
	); err != nil {
		slog.Error("đánh dấu đơn FAILED thất bại", "lỗi", err, "order_id", orderID)
	}
}

func fail(span trace.Span, code grpccodes.Code, msg string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, msg)
	span.SetAttributes(attribute.String("lab.error_kind", code.String()))
	slog.Error(msg, "lỗi", err, "trace_id", span.SpanContext().TraceID())
	return status.Errorf(code, "%s: %v", msg, err)
}
