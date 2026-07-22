package main

// Event is the only thing that crosses from the write side to the read side.
// It records what *happened*; the read side decides what to do with that fact.
type Event struct {
	Name    string
	OrderID string
	Item    string
	Price   float64
}

const (
	OrderPlaced = "OrderPlaced"
	ItemAdded   = "ItemAdded"
	OrderPaid   = "OrderPaid"
)

// Bus is a minimal in-process pub/sub. Real systems put Kafka or RabbitMQ
// here; the shape of the problem stays the same.
type Bus struct {
	handlers map[string][]func(Event)
}

func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]func(Event))}
}

func (b *Bus) Subscribe(name string, h func(Event)) {
	b.handlers[name] = append(b.handlers[name], h)
}

func (b *Bus) Publish(e Event) {
	for _, h := range b.handlers[e.Name] {
		h(e)
	}
}
