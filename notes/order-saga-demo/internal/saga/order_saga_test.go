// internal/saga/order_saga_test.go
package saga_test

import (
	"errors"
	"testing"
	"time"

	"order-saga-demo/internal/circuitbreaker"
	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/saga"
	"order-saga-demo/internal/services"
	"order-saga-demo/internal/writemodel"
)

func TestOrderSaga_HappyPathConfirmsOrder(t *testing.T) {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()
	inventory := services.NewInventoryService(nil, nil)
	payment := services.NewPaymentService(nil)
	breaker := circuitbreaker.New(3, time.Second)
	saga.NewOrderSaga(bus, writeStore, inventory, payment, breaker)

	writeStore.Save(domain.Order{ID: "order-1", Amount: 100, Status: domain.OrderStatusPending})
	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-1"})

	order, ok := writeStore.Get("order-1")
	if !ok {
		t.Fatalf("expected order-1 to exist")
	}
	if order.Status != domain.OrderStatusConfirmed {
		t.Fatalf("expected Confirmed, got %v", order.Status)
	}
}

func TestOrderSaga_InventoryFailureMarksOrderFailed(t *testing.T) {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()
	inventory := services.NewInventoryService(func(string) bool { return true }, nil)
	payment := services.NewPaymentService(nil)
	breaker := circuitbreaker.New(3, time.Second)
	saga.NewOrderSaga(bus, writeStore, inventory, payment, breaker)

	writeStore.Save(domain.Order{ID: "order-2", Amount: 50, Status: domain.OrderStatusPending})
	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-2"})

	order, _ := writeStore.Get("order-2")
	if order.Status != domain.OrderStatusFailed {
		t.Fatalf("expected Failed, got %v", order.Status)
	}
}

func TestOrderSaga_PaymentFailureCompensatesInventoryReservation(t *testing.T) {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()

	var released string
	inventory := services.NewInventoryService(nil, func(orderID string) { released = orderID })
	payment := services.NewPaymentService(func(string) bool { return true })
	breaker := circuitbreaker.New(3, time.Second)
	saga.NewOrderSaga(bus, writeStore, inventory, payment, breaker)

	writeStore.Save(domain.Order{ID: "order-3", Amount: 50, Status: domain.OrderStatusPending})
	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-3"})

	order, _ := writeStore.Get("order-3")
	if order.Status != domain.OrderStatusFailed {
		t.Fatalf("expected Failed, got %v", order.Status)
	}
	if released != "order-3" {
		t.Fatalf("expected inventory to be released for order-3, got %q", released)
	}
}

func TestOrderSaga_OpenCircuitFailsFastAndCompensates(t *testing.T) {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()

	var released string
	inventory := services.NewInventoryService(nil, func(orderID string) { released = orderID })
	paymentCalls := 0
	payment := services.NewPaymentService(func(string) bool { paymentCalls++; return false })
	breaker := circuitbreaker.New(1, time.Hour)
	_ = breaker.Execute(func() error { return errors.New("boom") }) // force Open

	saga.NewOrderSaga(bus, writeStore, inventory, payment, breaker)

	writeStore.Save(domain.Order{ID: "order-4", Amount: 50, Status: domain.OrderStatusPending})
	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-4"})

	order, _ := writeStore.Get("order-4")
	if order.Status != domain.OrderStatusFailed {
		t.Fatalf("expected Failed, got %v", order.Status)
	}
	if paymentCalls != 0 {
		t.Fatalf("expected payment service to never be called while circuit is open, got %d calls", paymentCalls)
	}
	if released != "order-4" {
		t.Fatalf("expected inventory to be released for order-4, got %q", released)
	}
}
