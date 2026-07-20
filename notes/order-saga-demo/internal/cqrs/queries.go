package cqrs

import "order-saga-demo/internal/readmodel"

type GetOrderQuery struct {
	OrderID string
}

type QueryBus struct {
	readStore *readmodel.Store
}

func NewQueryBus(readStore *readmodel.Store) *QueryBus {
	return &QueryBus{readStore: readStore}
}

func (b *QueryBus) GetOrder(q GetOrderQuery) (readmodel.OrderView, bool) {
	return b.readStore.Get(q.OrderID)
}
