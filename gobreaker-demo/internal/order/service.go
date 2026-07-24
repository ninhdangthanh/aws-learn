// Package order là service đặt hàng — phía "của mình", gọi sang payment gateway.
// Toàn bộ giá trị của circuit breaker nằm ở đây: bảo vệ order service khỏi
// một dependency đang chết.
package order

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sony/gobreaker/v2"
	"gobreaker-demo/internal/payment"
)

// ErrPaymentUnavailable trả về cho client khi không thanh toán được — bất kể
// vì gateway lỗi thật hay vì breaker đang mở.
var ErrPaymentUnavailable = errors.New("payment temporarily unavailable, order queued")

type Service struct {
	gateway *payment.Gateway

	// breaker[string] vì hàm được bọc trả về transaction ID.
	breaker *gobreaker.CircuitBreaker[string]

	// queued giữ các đơn không charge được ngay, để retry sau. Trong hệ thống
	// thật đây là một message queue, không phải slice trong RAM.
	queued []string
}

func NewService(gw *payment.Gateway) *Service {
	s := &Service{gateway: gw}

	s.breaker = gobreaker.NewCircuitBreaker[string](gobreaker.Settings{
		Name: "payment-gateway",

		// Half-open cho đúng 1 request thăm dò. Nếu provider vẫn chết, ta chỉ
		// tốn 1 request chứ không dội cả traffic vào lại.
		MaxRequests: 1,

		// Sau 2s ở trạng thái Open thì tự chuyển sang Half-open để thử lại.
		Timeout: 2 * time.Second,

		// Mở mạch sau 3 lần lỗi liên tiếp. Ngưỡng thật nên dựa trên tỉ lệ lỗi
		// (vd: >50% trên tối thiểu 20 request) chứ không phải số tuyệt đối.
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= 3
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("  [breaker] %s: %s -> %s", name, from, to)
		},
	})

	return s
}

// Place xử lý một đơn hàng. Lời gọi payment được bọc trong breaker.
func (s *Service) Place(orderID string, amount int) (string, error) {
	txnID, err := s.breaker.Execute(func() (string, error) {
		return s.gateway.Charge(orderID, amount)
	})

	if err != nil {
		// gobreaker trả về ErrOpenState / ErrTooManyRequests khi nó chặn
		// request — phân biệt với lỗi thật từ gateway để log cho đúng.
		switch {
		case errors.Is(err, gobreaker.ErrOpenState):
			log.Printf("  %s: fail-fast, gateway KHÔNG bị gọi", orderID)
		case errors.Is(err, gobreaker.ErrTooManyRequests):
			log.Printf("  %s: half-open đã đủ request thăm dò, bị chặn", orderID)
		default:
			log.Printf("  %s: gateway lỗi thật: %v", orderID, err)
		}

		// Fallback: không làm sập luồng đặt hàng, đẩy đơn vào hàng đợi.
		s.queued = append(s.queued, orderID)
		return "", ErrPaymentUnavailable
	}

	log.Printf("  %s: thanh toán OK (%s)", orderID, txnID)
	return txnID, nil
}

// State trả về trạng thái hiện tại của breaker.
func (s *Service) State() gobreaker.State { return s.breaker.State() }

// Queued trả về các đơn đang chờ retry.
func (s *Service) Queued() []string { return s.queued }

// Summary in tình trạng hệ thống — tiện để so sánh giữa các scenario.
func (s *Service) Summary() string {
	return fmt.Sprintf("state=%s | gateway calls=%d | queued=%d",
		s.breaker.State(), s.gateway.Calls(), len(s.queued))
}
