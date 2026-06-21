package main

func main() {
	order := &Order{ID: "ORD-100", Total: 450_000, state: &PendingState{}}

	// Invalid transition: an unpaid order cannot be shipped.
	order.Ship()

	// Valid lifecycle: pending -> paid -> shipped -> delivered.
	order.Pay()
	order.Ship()
	order.Deliver()

	// A different order follows the cancellation path.
	cancelledOrder := &Order{ID: "ORD-101", Total: 120_000, state: &PendingState{}}
	cancelledOrder.Pay()
	cancelledOrder.Cancel()
	cancelledOrder.Ship()
}
