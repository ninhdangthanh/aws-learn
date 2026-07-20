package writemodel_test

import (
	"testing"

	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/writemodel"
)

func TestStore_SaveAndGet(t *testing.T) {
	s := writemodel.NewStore()
	s.Save(domain.Order{ID: "order-1", Amount: 10, Status: domain.OrderStatusPending})

	got, ok := s.Get("order-1")
	if !ok {
		t.Fatalf("expected order-1 to exist")
	}
	if got.Status != domain.OrderStatusPending {
		t.Fatalf("expected Pending, got %v", got.Status)
	}
}

func TestStore_UpdateStatus(t *testing.T) {
	s := writemodel.NewStore()
	s.Save(domain.Order{ID: "order-1", Amount: 10, Status: domain.OrderStatusPending})
	s.UpdateStatus("order-1", domain.OrderStatusConfirmed)

	got, _ := s.Get("order-1")
	if got.Status != domain.OrderStatusConfirmed {
		t.Fatalf("expected Confirmed, got %v", got.Status)
	}
}

func TestStore_UpdateStatusOnMissingOrderIsNoop(t *testing.T) {
	s := writemodel.NewStore()
	s.UpdateStatus("missing", domain.OrderStatusConfirmed)

	if _, ok := s.Get("missing"); ok {
		t.Fatalf("expected missing order to remain absent")
	}
}
