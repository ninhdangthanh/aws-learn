// Package httpx chứa vài tiện ích HTTP dùng chung cho gateway và
// notification-svc.
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/ninhdang/go-jeager/internal/otelx"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TraceIDHeader là header phản hồi mang trace ID. Các script trong scripts/
// đọc header này để in ra link Jaeger mở thẳng trace vừa tạo — mẹo nhỏ nhưng
// tiết kiệm rất nhiều thời gian mò mẫm trong UI.
const TraceIDHeader = "X-Trace-Id"

// WriteJSON ghi response JSON và luôn đính kèm trace ID.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, payload any) {
	if id := otelx.TraceIDFromContext(r.Context()); id != "" {
		w.Header().Set(TraceIDHeader, id)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ErrorBody là hình dạng chung của mọi lỗi trả về từ gateway.
type ErrorBody struct {
	Error   string `json:"error"`
	Detail  string `json:"detail,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

// WriteError trả lỗi JSON kèm trace ID ngay trong body, để khi bạn thấy lỗi ở
// terminal là có thể tra ngược lên Jaeger ngay.
func WriteError(w http.ResponseWriter, r *http.Request, status int, msg, detail string) {
	WriteJSON(w, r, status, ErrorBody{
		Error:   msg,
		Detail:  detail,
		TraceID: otelx.TraceIDFromContext(r.Context()),
	})
}

// TracedClient trả về http.Client đã bọc otelhttp — mọi request đi ra sẽ tự
// sinh span CLIENT và tự inject header traceparent.
func TracedClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
