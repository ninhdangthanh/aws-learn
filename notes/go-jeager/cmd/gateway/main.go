// gateway: cửa ngõ HTTP công khai của lab.
//
// Vai trò trong bài học: đây là nơi trace BẮT ĐẦU. otelhttp tạo span gốc, và
// mọi span khác trong hệ thống đều là con cháu của span này. Gateway cũng là
// nơi header X-Fail-Mode được chuyển hoá thành Baggage để đi khắp hệ thống.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	orderv1 "github.com/ninhdang/go-jeager/gen/order/v1"
	"github.com/ninhdang/go-jeager/internal/config"
	"github.com/ninhdang/go-jeager/internal/faultx"
	"github.com/ninhdang/go-jeager/internal/httpx"
	"github.com/ninhdang/go-jeager/internal/otelx"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const serviceName = "gateway"

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

	orderAddr := config.Env("ORDER_ADDR", "localhost:9091")
	orderConn, err := grpc.NewClient(orderAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		slog.Error("tạo client order thất bại", "addr", orderAddr, "lỗi", err)
		os.Exit(1)
	}
	defer func() { _ = orderConn.Close() }()

	app := &gateway{orders: orderv1.NewOrderServiceClient(orderConn)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", app.createOrder)
	mux.HandleFunc("GET /orders/{id}", app.getOrder)
	mux.HandleFunc("POST /internal/callback", app.callback)
	mux.HandleFunc("GET /healthz", app.healthz)
	mux.HandleFunc("GET /stats", app.stats)

	// Thứ tự bọc handler rất quan trọng:
	//
	//   otelhttp  ←  ngoài cùng: tạo span SERVER, extract traceparent đến từ client
	//     └ faultMiddleware  ←  cần span đã tồn tại để gắn attribute
	//         └ mux          ←  handler nghiệp vụ, ctx đã có đủ span + baggage
	//
	// Đảo ngược lại thì faultMiddleware sẽ chạy khi chưa có span, và baggage nó
	// đặt vào sẽ không đến được handler.
	handler := otelhttp.NewHandler(
		faultMiddleware(mux),
		"gateway",
		// Mặc định otelhttp đặt tên span theo pattern chung, khiến mọi request
		// trông giống nhau trong Jaeger. Đổi thành "METHOD /path" cho dễ đọc.
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)

	addr := config.Env("HTTP_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("nhận tín hiệu dừng, tắt HTTP server")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	slog.Info("gateway đang chạy", "addr", addr, "order", orderAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server dừng", "lỗi", err)
	}
}

// faultMiddleware đọc header X-Fail-Mode và nạp vào Baggage.
//
// Đây là toàn bộ "phép thuật" của cơ chế bơm lỗi: sau dòng này, không service
// nào phải biết gì về X-Fail-Mode nữa — baggage tự đi theo request qua gRPC và
// qua cả RabbitMQ.
func faultMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := r.Header.Get(faultx.HeaderName)
		if mode == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := faultx.Inject(r.Context(), mode)
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("lab.fail_mode", mode))
		slog.Info("kích hoạt fail mode", "mode", mode, "path", r.URL.Path)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type gateway struct {
	orders        orderv1.OrderServiceClient
	callbackCount atomic.Int64
}

type createOrderRequest struct {
	SKU      string `json:"sku"`
	Qty      int32  `json:"qty"`
	Customer string `json:"customer"`
}

func (g *gateway) createOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, http.StatusBadRequest, "body JSON không hợp lệ", err.Error())
		return
	}
	if req.SKU == "" || req.Qty <= 0 {
		httpx.WriteError(w, r, http.StatusBadRequest, "thiếu tham số", "sku bắt buộc và qty phải > 0")
		return
	}
	if req.Customer == "" {
		req.Customer = "khách vãng lai"
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("lab.sku", req.SKU),
		attribute.Int("lab.qty", int(req.Qty)),
	)

	resp, err := g.orders.CreateOrder(r.Context(), &orderv1.CreateOrderRequest{
		Sku:      req.SKU,
		Qty:      req.Qty,
		Customer: req.Customer,
	})
	if err != nil {
		writeGRPCError(w, r, err)
		return
	}

	httpx.WriteJSON(w, r, http.StatusCreated, map[string]any{
		"order_id":        resp.GetOrderId(),
		"status":          resp.GetStatus(),
		"remaining_stock": resp.GetRemainingStock(),
		"trace_id":        otelx.TraceIDFromContext(r.Context()),
	})
}

func (g *gateway) getOrder(w http.ResponseWriter, r *http.Request) {
	resp, err := g.orders.GetOrder(r.Context(), &orderv1.GetOrderRequest{
		OrderId: r.PathValue("id"),
	})
	if err != nil {
		writeGRPCError(w, r, err)
		return
	}
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"order_id": resp.GetOrderId(),
		"sku":      resp.GetSku(),
		"qty":      resp.GetQty(),
		"status":   resp.GetStatus(),
		"customer": resp.GetCustomer(),
	})
}

// callback là điểm đến của notification-svc.
//
// Nó khép kín vòng tròn: HTTP → gRPC → gRPC → RabbitMQ → HTTP quay lại gateway.
// Trong Jaeger, span này xuất hiện MUỘN hơn hẳn so với span gốc — bằng chứng
// trực quan cho thấy nhánh async chạy sau khi client đã nhận response từ lâu.
func (g *gateway) callback(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)

	n := g.callbackCount.Add(1)
	trace.SpanFromContext(r.Context()).SetAttributes(
		attribute.String("lab.callback_order_id", asString(body["order_id"])),
		attribute.Int64("lab.callback_total", n),
	)
	slog.Info("nhận callback từ notification-svc", "order_id", body["order_id"], "tổng", n)

	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{"received": true})
}

func (g *gateway) healthz(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{"status": "ok", "service": serviceName})
}

func (g *gateway) stats(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, r, http.StatusOK, map[string]any{
		"callbacks_received": g.callbackCount.Load(),
	})
}

// writeGRPCError dịch gRPC status code sang HTTP status code.
//
// Giữ đúng ánh xạ này là điều kiện để bạn đọc được trace: nhìn 409 là biết lỗi
// nghiệp vụ, nhìn 503 là biết service phía dưới đã chết.
func writeGRPCError(w http.ResponseWriter, r *http.Request, err error) {
	st := status.Convert(err)

	httpStatus := http.StatusInternalServerError
	label := "lỗi nội bộ"

	switch st.Code() {
	case grpccodes.FailedPrecondition:
		httpStatus, label = http.StatusConflict, "không đủ hàng trong kho"
	case grpccodes.NotFound:
		httpStatus, label = http.StatusNotFound, "không tìm thấy"
	case grpccodes.InvalidArgument:
		httpStatus, label = http.StatusBadRequest, "tham số không hợp lệ"
	case grpccodes.Unavailable:
		httpStatus, label = http.StatusServiceUnavailable, "service phía dưới không phản hồi"
	case grpccodes.DeadlineExceeded:
		httpStatus, label = http.StatusGatewayTimeout, "quá hạn chờ"
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("lab.grpc_code", st.Code().String()),
		attribute.Int("lab.http_status", httpStatus),
	)
	slog.Error("gọi service phía dưới thất bại",
		"grpc_code", st.Code(), "http_status", httpStatus,
		"trace_id", otelx.TraceIDFromContext(r.Context()))

	httpx.WriteError(w, r, httpStatus, label, st.Message())
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
