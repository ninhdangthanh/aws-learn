package saga

import (
	"order-saga-demo/internal/circuitbreaker"
	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/services"
	"order-saga-demo/internal/writemodel"
)

// OrderSaga orchestrates OrderCreated -> ReserveInventory -> ChargePayment ->
// ConfirmOrder, compensating already-completed steps if a later step fails.
type OrderSaga struct {
	bus        *events.Bus
	writeStore *writemodel.Store
	inventory  *services.InventoryService
	payment    *services.PaymentService
	breaker    *circuitbreaker.Breaker
}

func NewOrderSaga(
	bus *events.Bus,
	writeStore *writemodel.Store,
	inventory *services.InventoryService,
	payment *services.PaymentService,
	breaker *circuitbreaker.Breaker,
) *OrderSaga {
	s := &OrderSaga{
		bus:        bus,
		writeStore: writeStore,
		inventory:  inventory,
		payment:    payment,
		breaker:    breaker,
	}
	bus.Subscribe(events.OrderCreated, s.onOrderCreated)
	bus.Subscribe(events.InventoryReserved, s.onInventoryReserved)
	bus.Subscribe(events.InventoryReserveFailed, s.onInventoryReserveFailed)
	bus.Subscribe(events.PaymentCharged, s.onPaymentCharged)
	bus.Subscribe(events.PaymentFailed, s.onPaymentFailed)
	return s
}

func (s *OrderSaga) onOrderCreated(e events.Event) {
	if err := s.inventory.Reserve(e.OrderID); err != nil {
		s.bus.Publish(events.Event{Name: events.InventoryReserveFailed, OrderID: e.OrderID, Reason: err.Error()})
		return
	}
	s.bus.Publish(events.Event{Name: events.InventoryReserved, OrderID: e.OrderID})
}

func (s *OrderSaga) onInventoryReserved(e events.Event) {
	order, ok := s.writeStore.Get(e.OrderID)
	if !ok {
		return
	}
	err := s.breaker.Execute(func() error {
		return s.payment.Charge(e.OrderID, order.Amount)
	})
	if err != nil {
		s.bus.Publish(events.Event{Name: events.PaymentFailed, OrderID: e.OrderID, Reason: err.Error()})
		return
	}
	s.bus.Publish(events.Event{Name: events.PaymentCharged, OrderID: e.OrderID})
}

func (s *OrderSaga) onInventoryReserveFailed(e events.Event) {
	// Nothing to compensate - inventory reservation never succeeded.
	s.writeStore.UpdateStatus(e.OrderID, domain.OrderStatusFailed)
	s.bus.Publish(events.Event{Name: events.OrderFailed, OrderID: e.OrderID, Reason: e.Reason})
}

func (s *OrderSaga) onPaymentFailed(e events.Event) {
	// Compensate the completed ReserveInventory step.
	s.inventory.Release(e.OrderID)
	s.writeStore.UpdateStatus(e.OrderID, domain.OrderStatusFailed)
	s.bus.Publish(events.Event{Name: events.OrderFailed, OrderID: e.OrderID, Reason: e.Reason})
}

func (s *OrderSaga) onPaymentCharged(e events.Event) {
	s.writeStore.UpdateStatus(e.OrderID, domain.OrderStatusConfirmed)
	s.bus.Publish(events.Event{Name: events.OrderConfirmed, OrderID: e.OrderID})
}
