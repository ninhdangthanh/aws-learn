package cqrs

import (
	"order-saga-demo/internal/domain"
	"order-saga-demo/internal/events"
	"order-saga-demo/internal/writemodel"
)

type CreateOrderCommand struct {
	OrderID string
	Items   []string
	Amount  float64
}

type CommandBus struct {
	writeStore *writemodel.Store
	eventBus   *events.Bus
}

func NewCommandBus(writeStore *writemodel.Store, eventBus *events.Bus) *CommandBus {
	return &CommandBus{writeStore: writeStore, eventBus: eventBus}
}

func (b *CommandBus) CreateOrder(cmd CreateOrderCommand) {
	order := domain.Order{
		ID:     cmd.OrderID,
		Items:  cmd.Items,
		Amount: cmd.Amount,
		Status: domain.OrderStatusPending,
	}
	b.writeStore.Save(order)
	b.eventBus.Publish(events.Event{Name: events.OrderCreated, OrderID: cmd.OrderID})
}
