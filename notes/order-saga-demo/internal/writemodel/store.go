package writemodel

import (
	"sync"

	"order-saga-demo/internal/domain"
)

type Store struct {
	mu     sync.Mutex
	orders map[string]domain.Order
}

func NewStore() *Store {
	return &Store{orders: make(map[string]domain.Order)}
}

func (s *Store) Save(o domain.Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
}

func (s *Store) Get(id string) (domain.Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	return o, ok
}

func (s *Store) UpdateStatus(id string, status domain.OrderStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return
	}
	o.Status = status
	s.orders[id] = o
}
