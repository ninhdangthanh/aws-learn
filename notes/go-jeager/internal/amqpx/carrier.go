package amqpx

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// HeaderCarrier là mấu chốt của toàn bộ lab này.
//
// HTTP và gRPC đều có thư viện instrumentation sẵn (otelhttp, otelgrpc) tự lo
// việc nhét traceparent vào request. RabbitMQ thì KHÔNG. Bạn phải tự bắc cầu
// giữa amqp.Table (map header của AMQP) và interface TextMapCarrier mà
// OpenTelemetry propagator yêu cầu.
//
// Quên bước này chính là nguyên nhân số một của hiện tượng "trace bị đứt làm
// đôi": producer nằm trong trace A, còn consumer tự sinh ra trace B mới toanh
// không liên quan gì.
//
// Interface propagation.TextMapCarrier chỉ đòi ba method: Get, Set, Keys.
type HeaderCarrier amqp.Table

// Get đọc một header. AMQP cho phép header mang kiểu bất kỳ (int, bool, []byte...)
// nên phải ép kiểu cẩn thận — traceparent luôn là chuỗi, nhưng một số client
// AMQP khác lại gửi dưới dạng []byte.
func (c HeaderCarrier) Get(key string) string {
	v, ok := c[key]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// Set ghi một header.
func (c HeaderCarrier) Set(key, value string) {
	c[key] = value
}

// Keys liệt kê toàn bộ header đang có.
func (c HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
