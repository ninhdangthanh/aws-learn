package main

func main() {
	order := &Order{
		state: &PendingState{},
	}

	// Cannot ship unpaid order
	order.Ship()

	// Payment success
	order.Pay()

	// Order shipped
	order.Ship()
}
