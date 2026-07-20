// internal/cqrs/cqrs_test.go
package cqrs_test

import (
	"testing"

	"order-saga-demo/internal/cqrs"
	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/readmodel"
	"order-saga-demo/internal/writemodel"
)

func TestCommandThenQuery_GoesThroughReadModel(t *testing.T) {
	bus := events.NewBus()
	writeStore := writemodel.NewStore()
	readStore := readmodel.NewStore()
	readmodel.Project(bus, readStore)

	cmdBus := cqrs.NewCommandBus(writeStore, bus)
	queryBus := cqrs.NewQueryBus(readStore)

	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-9", Items: []string{"widget"}, Amount: 42})

	view, ok := queryBus.GetOrder(cqrs.GetOrderQuery{OrderID: "order-9"})
	if !ok {
		t.Fatalf("expected read model to have order-9")
	}
	if view.Status != domain.OrderStatusPending {
		t.Fatalf("expected Pending, got %v", view.Status)
	}
}

func TestQuery_DoesNotReflectDirectWriteStoreMutation(t *testing.T) {
	// The read model only changes via events. Mutating the write store
	// directly must NOT change what queries see - that is the CQRS boundary
	// this demo exists to show.
	bus := events.NewBus()
	writeStore := writemodel.NewStore()
	readStore := readmodel.NewStore()
	readmodel.Project(bus, readStore)

	cmdBus := cqrs.NewCommandBus(writeStore, bus)
	queryBus := cqrs.NewQueryBus(readStore)

	cmdBus.CreateOrder(cqrs.CreateOrderCommand{OrderID: "order-9", Items: []string{"widget"}, Amount: 42})
	writeStore.UpdateStatus("order-9", domain.OrderStatusConfirmed)

	view, _ := queryBus.GetOrder(cqrs.GetOrderQuery{OrderID: "order-9"})
	if view.Status != domain.OrderStatusPending {
		t.Fatalf("expected read model to remain Pending until an event updates it, got %v", view.Status)
	}
}
