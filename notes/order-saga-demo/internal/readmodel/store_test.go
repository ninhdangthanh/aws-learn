package readmodel_test

import (
	"testing"

	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/readmodel"
)

func TestProject_TracksStatusAndHistory(t *testing.T) {
	bus := events.NewBus()
	store := readmodel.NewStore()
	readmodel.Project(bus, store)

	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-1"})
	bus.Publish(events.Event{Name: events.InventoryReserved, OrderID: "order-1"})
	bus.Publish(events.Event{Name: events.PaymentCharged, OrderID: "order-1"})
	bus.Publish(events.Event{Name: events.OrderConfirmed, OrderID: "order-1"})

	view, ok := store.Get("order-1")
	if !ok {
		t.Fatalf("expected order-1 to exist in read model")
	}
	if view.Status != domain.OrderStatusConfirmed {
		t.Fatalf("expected Confirmed, got %v", view.Status)
	}
	wantHistory := []string{
		events.OrderCreated, events.InventoryReserved, events.PaymentCharged, events.OrderConfirmed,
	}
	if len(view.StepHistory) != len(wantHistory) {
		t.Fatalf("expected history %v, got %v", wantHistory, view.StepHistory)
	}
	for i := range wantHistory {
		if view.StepHistory[i] != wantHistory[i] {
			t.Fatalf("expected history %v, got %v", wantHistory, view.StepHistory)
		}
	}
}

func TestProject_FailurePathSetsFailedStatus(t *testing.T) {
	bus := events.NewBus()
	store := readmodel.NewStore()
	readmodel.Project(bus, store)

	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-2"})
	bus.Publish(events.Event{Name: events.InventoryReserveFailed, OrderID: "order-2", Reason: "no stock"})
	bus.Publish(events.Event{Name: events.OrderFailed, OrderID: "order-2", Reason: "no stock"})

	view, ok := store.Get("order-2")
	if !ok {
		t.Fatalf("expected order-2 to exist in read model")
	}
	if view.Status != domain.OrderStatusFailed {
		t.Fatalf("expected Failed, got %v", view.Status)
	}
}

func TestStore_AllReturnsEveryView(t *testing.T) {
	bus := events.NewBus()
	store := readmodel.NewStore()
	readmodel.Project(bus, store)

	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-1"})
	bus.Publish(events.Event{Name: events.OrderCreated, OrderID: "order-2"})

	if len(store.All()) != 2 {
		t.Fatalf("expected 2 views, got %d", len(store.All()))
	}
}
