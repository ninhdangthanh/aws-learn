package order

import (
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"gobreaker-demo/internal/payment"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // service log ồn, không cần trong test
	m.Run()
}

// trip đẩy breaker sang Open bằng 3 lỗi liên tiếp.
func trip(t *testing.T, svc *Service) {
	t.Helper()
	for i := 0; i < 3; i++ {
		if _, err := svc.Place("trip", 100); err == nil {
			t.Fatalf("lần %d: mong đợi lỗi khi gateway chết", i)
		}
	}
}

func TestOpenBreakerStopsCallingGateway(t *testing.T) {
	gw := payment.NewGateway()
	svc := NewService(gw)

	gw.SetHealthy(false)
	trip(t, svc)

	callsWhenOpen := gw.Calls()

	// Đây là điểm mấu chốt: breaker Open thì gateway không được chạm tới nữa.
	for i := 0; i < 10; i++ {
		_, err := svc.Place("blocked", 100)
		if !errors.Is(err, ErrPaymentUnavailable) {
			t.Fatalf("mong đợi ErrPaymentUnavailable, nhận %v", err)
		}
	}

	if got := gw.Calls(); got != callsWhenOpen {
		t.Errorf("gateway bị gọi thêm khi breaker Open: %d -> %d", callsWhenOpen, got)
	}
}

func TestBreakerRecoversAfterTimeout(t *testing.T) {
	gw := payment.NewGateway()
	svc := NewService(gw)

	gw.SetHealthy(false)
	trip(t, svc)

	gw.SetHealthy(true)

	// Trước Timeout vẫn bị chặn dù gateway đã khoẻ lại.
	if _, err := svc.Place("too-early", 100); !errors.Is(err, ErrPaymentUnavailable) {
		t.Fatalf("mong đợi vẫn bị chặn trước Timeout, nhận %v", err)
	}

	time.Sleep(2100 * time.Millisecond) // Timeout = 2s -> Half-open

	if _, err := svc.Place("probe", 100); err != nil {
		t.Fatalf("request thăm dò phải thành công, nhận %v", err)
	}

	if state := svc.State().String(); state != "closed" {
		t.Errorf("sau khi thăm dò OK breaker phải closed, nhận %s", state)
	}
}

func TestHealthyGatewayPassesThrough(t *testing.T) {
	gw := payment.NewGateway()
	svc := NewService(gw)

	txnID, err := svc.Place("order-1", 100)
	if err != nil {
		t.Fatalf("mong đợi thành công, nhận %v", err)
	}
	if txnID == "" {
		t.Error("mong đợi transaction ID không rỗng")
	}
	if len(svc.Queued()) != 0 {
		t.Errorf("không nên có đơn nào bị queue, có %d", len(svc.Queued()))
	}
}
