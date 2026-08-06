// inventory-svc: gRPC server quản lý tồn kho, đọc/ghi Postgres.
//
// Đây là service nằm SÂU NHẤT trong luồng, và cũng là nơi ba trong bốn fault
// mode được kích hoạt. Khi đọc trace trong Jaeger, span của service này chính
// là root cause thật sự của phần lớn kịch bản lỗi.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	inventoryv1 "github.com/ninhdang/go-jeager/gen/inventory/v1"
	"github.com/ninhdang/go-jeager/internal/config"
	"github.com/ninhdang/go-jeager/internal/dbx"
	"github.com/ninhdang/go-jeager/internal/faultx"
	"github.com/ninhdang/go-jeager/internal/otelx"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const serviceName = "inventory-svc"

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

	addr := config.Env("GRPC_ADDR", ":9092")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("lắng nghe thất bại", "addr", addr, "lỗi", err)
		os.Exit(1)
	}

	// StatsHandler là cách chuẩn hiện nay để instrument gRPC (interceptor đã
	// deprecated). Nó tự extract traceparent từ metadata và mở span SERVER.
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	inventoryv1.RegisterInventoryServiceServer(srv, &server{pool: pool})
	reflection.Register(srv) // để grpcurl soi được service khi bạn muốn gọi tay

	go func() {
		<-ctx.Done()
		slog.Info("nhận tín hiệu dừng, tắt gRPC server")
		srv.GracefulStop()
	}()

	slog.Info("inventory-svc đang chạy", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		slog.Error("gRPC server dừng", "lỗi", err)
	}
}

type server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	pool *pgxpool.Pool
}

// Reserve giữ hàng cho một đơn: đọc tồn kho với khoá FOR UPDATE rồi trừ đi.
func (s *server) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	// otelgrpc đã mở sẵn span SERVER; ta chỉ lấy ra để bổ sung attribute.
	// Đặt attribute nghiệp vụ (sku, qty, order_id) là thói quen rất đáng có:
	// nó cho phép bạn tìm trace theo tag trong Jaeger, ví dụ lab.sku=SKU-RARE.
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("lab.order_id", req.GetOrderId()),
		attribute.String("lab.sku", req.GetSku()),
		attribute.Int("lab.qty_requested", int(req.GetQty())),
	)

	if mode := faultx.Mode(ctx); mode != "" {
		// Ghi lại fail mode nhận được từ baggage — bằng chứng cho thấy baggage
		// đã đi xuyên qua HTTP rồi gRPC tới tận đây.
		span.SetAttributes(attribute.String("lab.fail_mode", mode))
		slog.Info("nhận fail mode qua baggage", "mode", mode, "trace_id", span.SpanContext().TraceID())
	}

	// ── FAULT 3: panic ────────────────────────────────────────────────────
	// grpc-go không recover panic, nên cả process chết. BatchSpanProcessor
	// chưa kịp flush ⇒ span này KHÔNG BAO GIỜ tới Jaeger. Trace sẽ cụt ngay
	// tại đây và bạn chỉ thấy span client bên order-svc báo Unavailable.
	if faultx.Is(ctx, faultx.Panic) {
		slog.Error("cố tình panic theo yêu cầu của lab", "trace_id", span.SpanContext().TraceID())
		panic("inventory-svc: panic có chủ đích (fail_mode=panic)")
	}

	// ── FAULT 2: truy vấn chậm ────────────────────────────────────────────
	// Không sai kết quả, chỉ chậm. Trong Jaeger bạn sẽ thấy một span DB duy
	// nhất nuốt gần hết thời gian của cả trace.
	if faultx.Is(ctx, faultx.SlowDB) {
		if err := s.slowQuery(ctx); err != nil {
			return nil, s.fail(span, grpccodes.Internal, "truy vấn chậm thất bại", err)
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, s.fail(span, grpccodes.Internal, "mở transaction thất bại", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var available int32
	err = tx.QueryRow(ctx,
		`SELECT qty FROM inventory WHERE sku = $1 FOR UPDATE`,
		req.GetSku(),
	).Scan(&available)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, s.fail(span, grpccodes.NotFound, "không có SKU này", fmt.Errorf("sku %q không tồn tại", req.GetSku()))
	}
	if err != nil {
		return nil, s.fail(span, grpccodes.Internal, "đọc tồn kho thất bại", err)
	}
	span.SetAttributes(attribute.Int("lab.qty_available", int(available)))

	// ── FAULT 1: hết hàng ─────────────────────────────────────────────────
	// Ép nhánh lỗi nghiệp vụ chạy dù kho vẫn còn hàng, để kịch bản test cho
	// kết quả ổn định không phụ thuộc dữ liệu seed.
	forced := faultx.Is(ctx, faultx.OutOfStock)
	if forced || available < req.GetQty() {
		span.SetAttributes(attribute.Bool("lab.out_of_stock_forced", forced))
		return nil, s.fail(span, grpccodes.FailedPrecondition, "không đủ hàng trong kho",
			fmt.Errorf("sku %s: cần %d, còn %d", req.GetSku(), req.GetQty(), available))
	}

	remaining := available - req.GetQty()
	if _, err := tx.Exec(ctx,
		`UPDATE inventory SET qty = $1, updated_at = now() WHERE sku = $2`,
		remaining, req.GetSku(),
	); err != nil {
		return nil, s.fail(span, grpccodes.Internal, "trừ tồn kho thất bại", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, s.fail(span, grpccodes.Internal, "commit thất bại", err)
	}

	span.SetAttributes(attribute.Int("lab.qty_remaining", int(remaining)))
	span.SetStatus(codes.Ok, "")
	return &inventoryv1.ReserveResponse{Sku: req.GetSku(), Remaining: remaining}, nil
}

// GetStock trả về tồn kho hiện tại của một SKU.
func (s *server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("lab.sku", req.GetSku()))

	var qty int32
	err := s.pool.QueryRow(ctx, `SELECT qty FROM inventory WHERE sku = $1`, req.GetSku()).Scan(&qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, s.fail(span, grpccodes.NotFound, "không có SKU này", err)
	}
	if err != nil {
		return nil, s.fail(span, grpccodes.Internal, "đọc tồn kho thất bại", err)
	}
	return &inventoryv1.GetStockResponse{Sku: req.GetSku(), Available: qty}, nil
}

// slowQuery mô phỏng một câu SQL nặng. pg_sleep chạy ở phía server Postgres nên
// span do otelpgx sinh ra sẽ thực sự dài đúng bằng thời gian ngủ — giống hệt
// một index bị thiếu hoặc một bảng bị lock.
func (s *server) slowQuery(ctx context.Context) error {
	seconds := 3
	ctx, span := otelx.Tracer(serviceName).Start(ctx, "inventory.slow_lookup")
	defer span.End()
	span.SetAttributes(
		attribute.Int("lab.injected_delay_seconds", seconds),
		attribute.String("lab.note", "độ trễ được bơm vào có chủ đích qua fail_mode=slow_db"),
	)

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds+5)*time.Second)
	defer cancel()

	_, err := s.pool.Exec(queryCtx, `SELECT pg_sleep($1)`, seconds)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "pg_sleep thất bại")
	}
	return err
}

// fail đánh dấu span lỗi rồi trả về gRPC status tương ứng.
//
// Hai việc phải làm cùng lúc và rất hay bị quên một nửa:
//   - span.RecordError: đưa chi tiết lỗi vào tab Logs của span trong Jaeger.
//   - span.SetStatus(codes.Error): tô ĐỎ span trong UI.
//
// Chỉ RecordError mà không SetStatus thì span vẫn hiện màu xanh, và bạn sẽ ngồi
// tìm mãi không ra chỗ hỏng.
func (s *server) fail(span trace.Span, code grpccodes.Code, msg string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, msg)
	span.SetAttributes(attribute.String("lab.error_kind", code.String()))
	slog.Error(msg, "lỗi", err, "trace_id", span.SpanContext().TraceID())
	return status.Errorf(code, "%s: %v", msg, err)
}
