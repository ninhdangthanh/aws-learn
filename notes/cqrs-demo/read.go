package main

import "sort"

// ---------------------------------------------------------------------------
// THE READ SIDE
//
// Owns nothing. Everything here is derived from events. Never reads the write
// model, and has no way to reach it.
// ---------------------------------------------------------------------------

// OrderSummary deliberately does NOT look like Order. No individual lines -
// just the numbers a screen renders, added up ahead of time so a query never
// has to compute anything.
type OrderSummary struct {
	OrderID   string
	Status    string
	ItemCount int
	Total     float64
}

type ReadStore struct {
	summaries map[string]OrderSummary
}

func NewReadStore() *ReadStore {
	return &ReadStore{summaries: make(map[string]OrderSummary)}
}

// Project is the bridge. It is the ONLY writer of the read model - every field
// here is built up from events, never copied across from the write store.
func Project(bus *Bus, store *ReadStore) {
	bus.Subscribe(OrderPlaced, func(e Event) {
		store.summaries[e.OrderID] = OrderSummary{OrderID: e.OrderID, Status: "open"}
	})

	bus.Subscribe(ItemAdded, func(e Event) {
		v := store.summaries[e.OrderID]
		v.ItemCount++
		v.Total += e.Price
		store.summaries[e.OrderID] = v
	})

	bus.Subscribe(OrderPaid, func(e Event) {
		v := store.summaries[e.OrderID]
		v.Status = "paid"
		store.summaries[e.OrderID] = v
	})
}

// QueryBus holds only a *ReadStore. The separation is enforced by the type:
// there is no path from here to the write model even if someone wanted one.
type QueryBus struct {
	store *ReadStore
}

func NewQueryBus(store *ReadStore) *QueryBus {
	return &QueryBus{store: store}
}

func (q *QueryBus) GetSummary(orderID string) (OrderSummary, bool) {
	v, ok := q.store.summaries[orderID]
	return v, ok
}

func (q *QueryBus) ListSummaries() []OrderSummary {
	out := make([]OrderSummary, 0, len(q.store.summaries))
	for _, v := range q.store.summaries {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrderID < out[j].OrderID })
	return out
}
