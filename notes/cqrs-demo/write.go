package main

import "errors"

// ---------------------------------------------------------------------------
// THE WRITE SIDE
//
// Owns the truth. Validates. Publishes events. Never reads the read model.
// ---------------------------------------------------------------------------

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrOrderNotOpen  = errors.New("order is already paid")
	ErrEmptyOrder    = errors.New("cannot pay for an empty order")
)

type OrderLine struct {
	Item  string
	Price float64
}

// Order is the write model. It keeps every individual line, because that is
// what the business rules need in order to be checked.
type Order struct {
	ID    string
	Lines []OrderLine
	Paid  bool
}

func (o Order) Total() float64 {
	var sum float64
	for _, l := range o.Lines {
		sum += l.Price
	}
	return sum
}

// WriteStore is the source of truth. Only the commands below touch it.
type WriteStore struct {
	orders map[string]Order
}

func NewWriteStore() *WriteStore {
	return &WriteStore{orders: make(map[string]Order)}
}

func (s *WriteStore) Get(id string) (Order, bool) {
	o, ok := s.orders[id]
	return o, ok
}

func (s *WriteStore) Save(o Order) {
	s.orders[o.ID] = o
}

// CommandBus is the only legitimate way to change anything. Each command
// follows the same three beats: load & validate, save, publish.
type CommandBus struct {
	store *WriteStore
	bus   *Bus
}

func NewCommandBus(store *WriteStore, bus *Bus) *CommandBus {
	return &CommandBus{store: store, bus: bus}
}

func (c *CommandBus) PlaceOrder(orderID string) error {
	c.store.Save(Order{ID: orderID})
	c.bus.Publish(Event{Name: OrderPlaced, OrderID: orderID})
	return nil
}

func (c *CommandBus) AddItem(orderID, item string, price float64) error {
	order, err := c.openOrder(orderID)
	if err != nil {
		return err
	}

	order.Lines = append(order.Lines, OrderLine{Item: item, Price: price})
	c.store.Save(order)
	c.bus.Publish(Event{Name: ItemAdded, OrderID: orderID, Item: item, Price: price})
	return nil
}

func (c *CommandBus) PayOrder(orderID string) error {
	order, err := c.openOrder(orderID)
	if err != nil {
		return err
	}
	if len(order.Lines) == 0 {
		return ErrEmptyOrder
	}

	order.Paid = true
	c.store.Save(order)
	c.bus.Publish(Event{Name: OrderPaid, OrderID: orderID})
	return nil
}

// openOrder loads an order and enforces the shared rule: a paid order is
// frozen. Note it reads the WRITE store - validation must never trust the read
// model, which may be stale and would let an invalid write slip through.
func (c *CommandBus) openOrder(orderID string) (Order, error) {
	order, ok := c.store.Get(orderID)
	if !ok {
		return Order{}, ErrOrderNotFound
	}
	if order.Paid {
		return Order{}, ErrOrderNotOpen
	}
	return order, nil
}
