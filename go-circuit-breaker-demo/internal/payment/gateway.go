// Package payment mô phỏng một payment gateway bên ngoài (Stripe/VNPay/...).
// Đây là loại dependency mà ta KHÔNG kiểm soát được: nó có thể chậm, có thể sập.
package payment

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// ErrGatewayDown là lỗi khi gateway không phục vụ được request.
var ErrGatewayDown = errors.New("payment gateway unavailable")

// Gateway là client gọi sang payment provider.
type Gateway struct {
	// healthy=false mô phỏng provider đang sập. Dùng atomic vì demo gọi từ
	// nhiều goroutine ở scenario cuối.
	healthy atomic.Bool

	// calls đếm số lần thực sự chạm tới provider. Đây là con số quan trọng
	// nhất của demo: khi breaker mở, con số này phải đứng yên.
	calls atomic.Int64
}

func NewGateway() *Gateway {
	g := &Gateway{}
	g.healthy.Store(true)
	return g
}

// SetHealthy bật/tắt provider để mô phỏng sự cố và lúc phục hồi.
func (g *Gateway) SetHealthy(ok bool) { g.healthy.Store(ok) }

// Calls trả về tổng số lần gateway thật sự bị gọi.
func (g *Gateway) Calls() int64 { return g.calls.Load() }

// Charge trừ tiền cho một đơn hàng.
func (g *Gateway) Charge(orderID string, amount int) (string, error) {
	g.calls.Add(1)

	if !g.healthy.Load() {
		return "", ErrGatewayDown
	}
	return fmt.Sprintf("txn-%s-%d", orderID, amount), nil
}
