package readmodel

import (
	"sync"

	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
)

type OrderView struct {
	OrderID     string
	Status      domain.OrderStatus
	StepHistory []string
}

type Store struct {
	mu    sync.Mutex
	views map[string]OrderView
}

func NewStore() *Store {
	return &Store{views: make(map[string]OrderView)}
}

func (s *Store) Get(orderID string) (OrderView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.views[orderID]
	return v, ok
}

func (s *Store) All() []OrderView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OrderView, 0, len(s.views))
	for _, v := range s.views {
		out = append(out, v)
	}
	return out
}

func (s *Store) apply(e events.Event, status domain.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.views[e.OrderID]
	v.OrderID = e.OrderID
	if status != "" {
		v.Status = status
	}
	v.StepHistory = append(v.StepHistory, e.Name)
	s.views[e.OrderID] = v
}

// Project subscribes store to bus so it stays in sync with every order
// event. store is the only read-model writer; queries only ever call Get/All.
func Project(bus *events.Bus, store *Store) {
	bus.Subscribe(events.OrderCreated, func(e events.Event) { store.apply(e, domain.OrderStatusPending) })
	bus.Subscribe(events.InventoryReserved, func(e events.Event) { store.apply(e, "") })
	bus.Subscribe(events.InventoryReserveFailed, func(e events.Event) { store.apply(e, "") })
	bus.Subscribe(events.PaymentCharged, func(e events.Event) { store.apply(e, "") })
	bus.Subscribe(events.PaymentFailed, func(e events.Event) { store.apply(e, "") })
	bus.Subscribe(events.OrderConfirmed, func(e events.Event) { store.apply(e, domain.OrderStatusConfirmed) })
	bus.Subscribe(events.OrderFailed, func(e events.Event) { store.apply(e, domain.OrderStatusFailed) })
}
